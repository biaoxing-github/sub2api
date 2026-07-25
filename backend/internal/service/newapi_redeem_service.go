package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	newAPIRedeemDefaultBaseURL  = "https://www.cun.ai"
	newAPIRedeemStateFileName   = "state.json"
	newAPIRedeemUploadsDirName  = "vouchers"
	newAPIRedeemMaxUploadBytes  = 5 * 1024 * 1024
	newAPIRedeemDefaultInterval = 2 * time.Second
	newAPIRedeemRetryDelay      = 5 * time.Minute
	// 所有任务合计只保留最近明细，并按固定间隔落盘，避免历史任务持续放大状态文件。
	newAPIRedeemMaxStoredLogs      = 100
	newAPIRedeemCheckpointInterval = 25
	newAPIRedeemFileCompactBatch   = 100
)

var errNewAPIRedeemNoAvailableExit = errors.New("所有兑换出口暂不可用")

// NewAPIRedeemOptions 是创建兑换工具服务的可测试配置。
type NewAPIRedeemOptions struct {
	// RootDir 是本地账号、文件和兑换进度的持久化目录。
	RootDir string
	// BaseURL 是唯一 NewAPI 平台的根地址。
	BaseURL string
	// HTTPClient 负责调用 NewAPI 的账户和兑换接口。
	HTTPClient *http.Client
	// Now 为测试提供可控时间来源。
	Now func() time.Time
	// RequestInterval 控制相邻兑换码请求的最小间隔。
	RequestInterval time.Duration
	// NetworkRetryDelay 控制所有出口耗尽后重试当前文件前的共享等待时间。
	NetworkRetryDelay time.Duration
	// MihomoControllerURL 是 Clash Verge Mihomo HTTP Controller 地址。
	MihomoControllerURL string
	// MihomoControllerSecret 是 Mihomo Controller 的 Bearer 密钥。
	MihomoControllerSecret string
	// MihomoSelectorGroup 是实际承载 cun.ai 流量的选择器分组。
	MihomoSelectorGroup string
	// MihomoDelayURL 是切换前用于节点延迟测试的目标地址。
	MihomoDelayURL string
	// ExitIPURL 返回当前容器请求看到的公网出口 IP。
	ExitIPURL string
	// MihomoHTTPClient 允许测试替换 Controller 和出口 IP 请求客户端。
	MihomoHTTPClient *http.Client
	// MihomoSwitchPollInterval 控制切换后轮询出口 IP 的间隔。
	MihomoSwitchPollInterval time.Duration
	// MihomoSwitchPollAttempts 控制单节点等待出口 IP 生效的次数。
	MihomoSwitchPollAttempts int
	// AccountRepository 用于展示和更新 Sub2API OpenAI API Key 账号的引用关系。
	AccountRepository NewAPICheckinAccountRepository
}

// NewAPIRedeemService 管理单平台 NewAPI 账号、兑换码文件与异步兑换任务。
type NewAPIRedeemService struct {
	rootDir           string
	statePath         string
	uploadsDir        string
	baseURL           string
	httpClient        *http.Client
	mihomoClient      *newAPIRedeemMihomoClient
	now               func() time.Time
	requestInterval   time.Duration
	networkRetryDelay time.Duration
	accountRepo       NewAPICheckinAccountRepository
	clientMu          sync.Mutex
	accountClients    map[string]*http.Client

	mu          sync.Mutex
	initialized bool
	state       newAPIRedeemState
	runCancels  map[string]context.CancelFunc
}

// NewAPIRedeemAccountInput 是新增或更新本地账号时需要的 API Key 登录信息。
type NewAPIRedeemAccountInput struct {
	// UserID 对应上游 New-Api-User 请求头。
	UserID string `json:"user_id"`
	// AccessKey 对应上游 Authorization Bearer 凭据，只写入本地私有状态文件。
	AccessKey string `json:"access_key"`
}

// NewAPIRedeemAccount 是不会泄露访问凭据的账户展示模型。
type NewAPIRedeemAccount struct {
	ID              string                          `json:"id"`
	UserID          string                          `json:"user_id"`
	AccessKeyMasked string                          `json:"access_key_masked"`
	Username        string                          `json:"username,omitempty"`
	Email           string                          `json:"email,omitempty"`
	Group           string                          `json:"group,omitempty"`
	Quota           string                          `json:"quota,omitempty"`
	UsedQuota       string                          `json:"used_quota,omitempty"`
	APIKeys         []NewAPIRedeemAPIKey            `json:"api_keys"`
	AvailableGroups []string                        `json:"available_groups"`
	RedeemedCodes   []NewAPIRedeemAccountRedemption `json:"redeemed_codes"`
	Status          string                          `json:"status"`
	Message         string                          `json:"message,omitempty"`
	RefreshedAt     string                          `json:"refreshed_at,omitempty"`
	// BrowserFingerprint 是该账号稳定绑定的 TLS ClientHello 与 HTTP 头身份。
	BrowserFingerprint string `json:"browser_fingerprint"`
}

// NewAPIRedeemAccountRedemption 展示账号在指定兑换码文件上的唯一成功标记。
type NewAPIRedeemAccountRedemption struct {
	FileID     string `json:"file_id"`
	FileName   string `json:"file_name"`
	Result     string `json:"result"`
	RedeemedAt string `json:"redeemed_at"`
}

// NewAPIRedeemAPIKey 是上游生成 API Key 的脱敏展示模型。
type NewAPIRedeemAPIKey struct {
	ID                 int64                          `json:"id"`
	Name               string                         `json:"name"`
	Group              string                         `json:"group"`
	MaskedKey          string                         `json:"masked_key"`
	ReferencedAccounts []NewAPIRedeemAccountReference `json:"referenced_accounts"`
	TargetAccounts     []NewAPIRedeemAccountReference `json:"target_accounts"`
}

// NewAPIRedeemAccountReference 是一个可展示且可操作的 Sub2API API Key 账号引用。
type NewAPIRedeemAccountReference struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Referenced bool   `json:"referenced"`
}

// NewAPIRedeemLinkAPIKeyResult 是兑换工具 Key 写入 Sub2API 账号后的结果。
type NewAPIRedeemLinkAPIKeyResult struct {
	AccountID        string `json:"account_id"`
	APIKeyID         int64  `json:"api_key_id"`
	TargetAccountID  int64  `json:"target_account_id"`
	TargetAccountName string `json:"target_account_name"`
	Operation        string `json:"operation"`
	KeyCount         int    `json:"key_count"`
	Message          string `json:"message"`
}

// NewAPIRedeemAPIKeySecret 仅由显式查看 Key 的管理接口返回。
type NewAPIRedeemAPIKeySecret struct {
	AccountID string `json:"account_id"`
	APIKeyID  int64  `json:"api_key_id"`
	Key       string `json:"key"`
}

// NewAPIRedeemVoucherFile 是本地已上传兑换码文件的展示模型。
type NewAPIRedeemVoucherFile struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	CodeCount  int    `json:"code_count"`
	UploadedAt string `json:"uploaded_at"`
}

// NewAPIRedeemRequestLog 记录每次兑换请求的上游结果，便于发现限流后切换出口 IP。
type NewAPIRedeemRequestLog struct {
	At                 string `json:"at"`
	FileID             string `json:"file_id"`
	FileName           string `json:"file_name"`
	AccountID          string `json:"account_id"`
	UserID             string `json:"user_id"`
	BrowserFingerprint string `json:"browser_fingerprint"`
	Code               string `json:"code"`
	Result             string `json:"result"`
	Message            string `json:"message,omitempty"`
	StatusCode         int    `json:"status_code,omitempty"`
	RemovedFromFile    bool   `json:"removed_from_file,omitempty"`
	Node               string `json:"node,omitempty"`
	ExitIP             string `json:"exit_ip,omitempty"`
	SwitchedNode       string `json:"switched_node,omitempty"`
	SwitchedExitIP     string `json:"switched_exit_ip,omitempty"`
	SwitchMessage      string `json:"switch_message,omitempty"`
}

// NewAPIRedeemSuccessLog 保留旧名称，兼容已存在的服务测试和状态文件调用方。
type NewAPIRedeemSuccessLog = NewAPIRedeemRequestLog

// NewAPIRedeemRun 是一个后台兑换任务的可轮询状态。
type NewAPIRedeemRun struct {
	ID              string                   `json:"id"`
	Status          string                   `json:"status"`
	StartedAt       string                   `json:"started_at"`
	FinishedAt      string                   `json:"finished_at,omitempty"`
	TotalPairs      int                      `json:"total_pairs"`
	CompletedPairs  int                      `json:"completed_pairs"`
	Attempts        int                      `json:"attempts"`
	Successes       int                      `json:"successes"`
	Message         string                   `json:"message,omitempty"`
	CurrentFileID   string                   `json:"current_file_id,omitempty"`
	CurrentFileName string                   `json:"current_file_name,omitempty"`
	CurrentNode     string                   `json:"current_node,omitempty"`
	CurrentExitIP   string                   `json:"current_exit_ip,omitempty"`
	SwitchCount     int                      `json:"switch_count"`
	NetworkMessage  string                   `json:"network_message,omitempty"`
	Logs            []NewAPIRedeemRequestLog `json:"logs"`
}

// NewAPIRedeemStartRunRequest 指定需要一起处理的兑换码文件和账号。
type NewAPIRedeemStartRunRequest struct {
	FileIDs    []string `json:"file_ids"`
	AccountIDs []string `json:"account_ids"`
}

// NewAPIRedeemOverview 聚合页面首屏所需的本地状态。
type NewAPIRedeemOverview struct {
	BaseURL  string                    `json:"base_url"`
	Accounts []NewAPIRedeemAccount     `json:"accounts"`
	Files    []NewAPIRedeemVoucherFile `json:"files"`
	Runs     []NewAPIRedeemRun         `json:"runs"`
}

type newAPIRedeemStoredAccount struct {
	ID               string              `json:"id"`
	UserID           string              `json:"user_id"`
	AccessKey        string              `json:"access_key"`
	BrowserProfileID string              `json:"browser_profile_id"`
	Snapshot         NewAPIRedeemAccount `json:"snapshot"`
}

type newAPIRedeemStoredFile struct {
	NewAPIRedeemVoucherFile
	StoredName string `json:"stored_name"`
}

type newAPIRedeemRedemption struct {
	FileID     string `json:"file_id"`
	AccountID  string `json:"account_id"`
	Code       string `json:"code,omitempty"`
	CodeHash   string `json:"code_hash"`
	Result     string `json:"result"`
	RedeemedAt string `json:"redeemed_at"`
}

type newAPIRedeemState struct {
	Version             int                         `json:"version"`
	Accounts            []newAPIRedeemStoredAccount `json:"accounts"`
	Files               []newAPIRedeemStoredFile    `json:"files"`
	Redemptions         []newAPIRedeemRedemption    `json:"redemptions"`
	Runs                []NewAPIRedeemRun           `json:"runs"`
	PendingCodeRemovals map[string][]string         `json:"pending_code_removals,omitempty"`
}

type newAPIRedeemUpstreamResponse struct {
	OK         bool
	StatusCode int
	Message    string
	Payload    map[string]any
}

// NewNewAPIRedeemService 创建本地文件持久化的兑换工具服务。
func NewNewAPIRedeemService(options NewAPIRedeemOptions) *NewAPIRedeemService {
	rootDir := strings.TrimSpace(options.RootDir)
	if rootDir == "" {
		dataDir := strings.TrimSpace(os.Getenv("DATA_DIR"))
		if dataDir == "" {
			dataDir = "./data"
		}
		rootDir = filepath.Join(dataDir, "newapi-redeem")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(options.BaseURL), "/")
	if baseURL == "" {
		baseURL = newAPIRedeemDefaultBaseURL
	}
	client := options.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	interval := options.RequestInterval
	if interval <= 0 {
		interval = newAPIRedeemDefaultInterval
	}
	retryDelay := options.NetworkRetryDelay
	if retryDelay <= 0 {
		retryDelay = newAPIRedeemRetryDelay
	}
	service := &NewAPIRedeemService{
		rootDir:           rootDir,
		statePath:         filepath.Join(rootDir, newAPIRedeemStateFileName),
		uploadsDir:        filepath.Join(rootDir, newAPIRedeemUploadsDirName),
		baseURL:           baseURL,
		httpClient:        client,
		now:               now,
		requestInterval:   interval,
		networkRetryDelay: retryDelay,
		accountRepo:       options.AccountRepository,
		accountClients:    map[string]*http.Client{},
		runCancels:        map[string]context.CancelFunc{},
	}
	service.mihomoClient = newNewAPIRedeemMihomoClient(options, client)
	return service
}

// ProvideNewAPIRedeemService 为 Wire 提供默认运行时配置。
func ProvideNewAPIRedeemService(accountRepo NewAPICheckinAccountRepository) *NewAPIRedeemService {
	return NewNewAPIRedeemService(NewAPIRedeemOptions{
		MihomoControllerURL:    firstNewAPIRedeemText(os.Getenv("MIHOMO_CONTROLLER_URL"), newAPIRedeemDefaultMihomoControllerURL),
		MihomoControllerSecret: os.Getenv("MIHOMO_CONTROLLER_SECRET"),
		MihomoSelectorGroup:    os.Getenv("MIHOMO_SELECTOR_GROUP"),
		MihomoDelayURL:         os.Getenv("MIHOMO_DELAY_URL"),
		ExitIPURL:              os.Getenv("NEWAPI_REDEEM_EXIT_IP_URL"),
		AccountRepository:      accountRepo,
	})
}

// Overview 返回所有可展示的本地状态，绝不包含原始访问 Key。
func (s *NewAPIRedeemService) Overview(ctx context.Context) (NewAPIRedeemOverview, error) {
	s.mu.Lock()
	if err := s.ensureStateLocked(ctx); err != nil {
		s.mu.Unlock()
		return NewAPIRedeemOverview{}, err
	}
	overview := NewAPIRedeemOverview{
		BaseURL:  s.baseURL,
		Accounts: make([]NewAPIRedeemAccount, 0, len(s.state.Accounts)),
		Files:    make([]NewAPIRedeemVoucherFile, 0, len(s.state.Files)),
		Runs:     make([]NewAPIRedeemRun, 0, len(s.state.Runs)),
	}
	for _, account := range s.state.Accounts {
		projected := projectNewAPIRedeemAccount(account)
		projected.RedeemedCodes = s.projectNewAPIRedeemAccountRedemptions(account.ID)
		overview.Accounts = append(overview.Accounts, projected)
	}
	for _, file := range s.state.Files {
		overview.Files = append(overview.Files, file.NewAPIRedeemVoucherFile)
	}
	for _, run := range s.state.Runs {
		overview.Runs = append(overview.Runs, cloneNewAPIRedeemRun(run))
	}
	sort.Slice(overview.Accounts, func(i, j int) bool { return overview.Accounts[i].UserID < overview.Accounts[j].UserID })
	sort.Slice(overview.Files, func(i, j int) bool { return overview.Files[i].UploadedAt > overview.Files[j].UploadedAt })
	sort.Slice(overview.Runs, func(i, j int) bool { return overview.Runs[i].StartedAt > overview.Runs[j].StartedAt })
	if len(overview.Runs) > 20 {
		overview.Runs = overview.Runs[:20]
	}
	s.mu.Unlock()
	if s.accountRepo != nil {
		accounts, err := s.accountRepo.ListByPlatform(ctx, PlatformOpenAI)
		if err != nil {
			return NewAPIRedeemOverview{}, err
		}
		attachNewAPIRedeemAccountReferences(overview.Accounts, accounts, s.baseURL)
	}
	return overview, nil
}

// LinkAPIKeyToAccount 将兑换工具生成的 Key 追加或替换到同站点的主平台账号。
func (s *NewAPIRedeemService) LinkAPIKeyToAccount(ctx context.Context, accountID string, apiKeyID, targetAccountID int64, operation string) (NewAPIRedeemLinkAPIKeyResult, error) {
	if s.accountRepo == nil {
		return NewAPIRedeemLinkAPIKeyResult{}, fmt.Errorf("主平台账号关联服务不可用")
	}
	operation = strings.ToLower(strings.TrimSpace(operation))
	if operation != "append" && operation != "replace" {
		return NewAPIRedeemLinkAPIKeyResult{}, fmt.Errorf("operation 必须是 append 或 replace")
	}
	revealed, err := s.RevealAPIKey(ctx, accountID, apiKeyID)
	if err != nil {
		return NewAPIRedeemLinkAPIKeyResult{}, err
	}
	target, err := s.accountRepo.GetByID(ctx, targetAccountID)
	if err != nil {
		return NewAPIRedeemLinkAPIKeyResult{}, err
	}
	if target == nil || target.Platform != PlatformOpenAI || target.Type != AccountTypeAPIKey || !newAPIAccountBaseURLsMatch(target.GetCredential("base_url"), s.baseURL) {
		return NewAPIRedeemLinkAPIKeyResult{}, fmt.Errorf("目标账号与兑换平台 URL 不匹配")
	}
	// Key 关联是局部更新，保留 base_url、模型映射和其他凭据。
	credentials := cloneCredentials(target.Credentials)
	if operation == "append" {
		credentials["api_keys_append"] = []string{revealed.Key}
	} else {
		credentials["api_keys"] = []string{revealed.Key}
	}
	target.Credentials = MergeAccountCredentialsForUpdate(target.Credentials, credentials)
	if operation == "replace" {
		delete(target.Credentials, CredentialAPIKeysDisabled)
	}
	if err := s.accountRepo.Update(ctx, target); err != nil {
		return NewAPIRedeemLinkAPIKeyResult{}, err
	}
	return NewAPIRedeemLinkAPIKeyResult{AccountID: accountID, APIKeyID: apiKeyID, TargetAccountID: target.ID, TargetAccountName: target.Name, Operation: operation, KeyCount: len(target.GetAPIKeys()), Message: "主平台账号 Key 已更新"}, nil
}

// ImportAccounts 新增或更新一组 API Key 模式账号，并返回脱敏后的账户列表。
func (s *NewAPIRedeemService) ImportAccounts(ctx context.Context, inputs []NewAPIRedeemAccountInput) ([]NewAPIRedeemAccount, error) {
	if len(inputs) == 0 {
		return nil, fmt.Errorf("至少需要一个账号")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureStateLocked(ctx); err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(inputs))
	changed := make([]NewAPIRedeemAccount, 0, len(inputs))
	for _, input := range inputs {
		userID := strings.TrimSpace(input.UserID)
		accessKey := strings.TrimSpace(input.AccessKey)
		if userID == "" || accessKey == "" {
			return nil, fmt.Errorf("user_id 和 access_key 均不能为空")
		}
		if _, duplicate := seen[userID]; duplicate {
			return nil, fmt.Errorf("导入列表包含重复账号: %s", userID)
		}
		seen[userID] = struct{}{}
		index := findNewAPIRedeemAccountByUserID(s.state.Accounts, userID)
		if index < 0 {
			stored := newAPIRedeemStoredAccount{
				ID:               uuid.NewString(),
				UserID:           userID,
				AccessKey:        accessKey,
				BrowserProfileID: allocateNewAPIRedeemBrowserProfileID(s.state.Accounts),
				Snapshot: NewAPIRedeemAccount{
					UserID:          userID,
					APIKeys:         []NewAPIRedeemAPIKey{},
					AvailableGroups: []string{},
					Status:          "pending",
					Message:         "等待刷新上游信息",
				},
			}
			s.state.Accounts = append(s.state.Accounts, stored)
			changed = append(changed, projectNewAPIRedeemAccount(stored))
			continue
		}
		s.state.Accounts[index].AccessKey = accessKey
		s.state.Accounts[index].Snapshot.Message = "访问 Key 已更新，等待刷新上游信息"
		s.state.Accounts[index].Snapshot.Status = "pending"
		changed = append(changed, projectNewAPIRedeemAccount(s.state.Accounts[index]))
	}
	if err := s.saveStateLocked(); err != nil {
		return nil, err
	}
	return changed, nil
}

// DeleteAccount 删除本地账户及其已记录的兑换成功标记。
func (s *NewAPIRedeemService) DeleteAccount(ctx context.Context, accountID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureStateLocked(ctx); err != nil {
		return err
	}
	if s.hasRunningRunLocked() {
		return fmt.Errorf("存在进行中的兑换任务，暂不能删除账号")
	}
	index := findNewAPIRedeemAccountByID(s.state.Accounts, strings.TrimSpace(accountID))
	if index < 0 {
		return fmt.Errorf("未找到账号")
	}
	s.state.Accounts = append(s.state.Accounts[:index], s.state.Accounts[index+1:]...)
	redemptions := s.state.Redemptions[:0]
	for _, redemption := range s.state.Redemptions {
		if redemption.AccountID != accountID {
			redemptions = append(redemptions, redemption)
		}
	}
	s.state.Redemptions = redemptions
	return s.saveStateLocked()
}

// RefreshAccount 从 NewAPI 读取账户、余额、分组和生成 Key 的最新摘要。
func (s *NewAPIRedeemService) RefreshAccount(ctx context.Context, accountID string) (NewAPIRedeemAccount, error) {
	s.mu.Lock()
	if err := s.ensureStateLocked(ctx); err != nil {
		s.mu.Unlock()
		return NewAPIRedeemAccount{}, err
	}
	index := findNewAPIRedeemAccountByID(s.state.Accounts, strings.TrimSpace(accountID))
	if index < 0 {
		s.mu.Unlock()
		return NewAPIRedeemAccount{}, fmt.Errorf("未找到账号")
	}
	stored := s.state.Accounts[index]
	s.mu.Unlock()

	snapshot, err := s.fetchNewAPIRedeemAccount(ctx, stored)
	s.mu.Lock()
	defer s.mu.Unlock()
	if ensureErr := s.ensureStateLocked(ctx); ensureErr != nil {
		return NewAPIRedeemAccount{}, ensureErr
	}
	index = findNewAPIRedeemAccountByID(s.state.Accounts, stored.ID)
	if index < 0 {
		return NewAPIRedeemAccount{}, fmt.Errorf("账号在刷新期间已删除")
	}
	if err != nil {
		s.state.Accounts[index].Snapshot.Status = "error"
		s.state.Accounts[index].Snapshot.Message = err.Error()
		_ = s.saveStateLocked()
		return projectNewAPIRedeemAccount(s.state.Accounts[index]), err
	}
	s.state.Accounts[index].Snapshot = snapshot
	if err := s.saveStateLocked(); err != nil {
		return NewAPIRedeemAccount{}, err
	}
	return projectNewAPIRedeemAccount(s.state.Accounts[index]), nil
}

// CreateAPIKey 在指定上游账号创建一个新的 API Key 后刷新本地摘要。
func (s *NewAPIRedeemService) CreateAPIKey(ctx context.Context, accountID, name, group string) (NewAPIRedeemAccount, error) {
	name = strings.TrimSpace(name)
	group = strings.TrimSpace(group)
	if name == "" {
		return NewAPIRedeemAccount{}, fmt.Errorf("Key 名称不能为空")
	}
	account, err := s.storedAccount(ctx, accountID)
	if err != nil {
		return NewAPIRedeemAccount{}, err
	}
	body := map[string]any{"name": name}
	if group != "" {
		body["group"] = group
	}
	result, err := s.callNewAPI(ctx, http.MethodPost, "/api/token/", account, body)
	if err != nil {
		return NewAPIRedeemAccount{}, err
	}
	if !result.OK {
		return NewAPIRedeemAccount{}, fmt.Errorf("创建 API Key 失败: %s", firstNewAPIRedeemText(result.Message, "上游拒绝请求"))
	}
	return s.RefreshAccount(ctx, accountID)
}

// UpdateAPIKeyGroup 读取完整 token 后仅替换其 group 字段并提交上游更新。
func (s *NewAPIRedeemService) UpdateAPIKeyGroup(ctx context.Context, accountID string, apiKeyID int64, group string) (NewAPIRedeemAccount, error) {
	group = strings.TrimSpace(group)
	if apiKeyID <= 0 || group == "" {
		return NewAPIRedeemAccount{}, fmt.Errorf("api_key_id 和 group 均不能为空")
	}
	account, err := s.storedAccount(ctx, accountID)
	if err != nil {
		return NewAPIRedeemAccount{}, err
	}
	token, err := s.fetchNewAPIRedeemToken(ctx, account, apiKeyID)
	if err != nil {
		return NewAPIRedeemAccount{}, err
	}
	token["group"] = group
	result, err := s.callNewAPI(ctx, http.MethodPut, "/api/token/", account, token)
	if err != nil {
		return NewAPIRedeemAccount{}, err
	}
	if !result.OK {
		return NewAPIRedeemAccount{}, fmt.Errorf("更新 API Key 分组失败: %s", firstNewAPIRedeemText(result.Message, "上游拒绝请求"))
	}
	return s.RefreshAccount(ctx, accountID)
}

// RevealAPIKey 通过上游单个 token Key 接口返回完整生成 Key。
func (s *NewAPIRedeemService) RevealAPIKey(ctx context.Context, accountID string, apiKeyID int64) (NewAPIRedeemAPIKeySecret, error) {
	if apiKeyID <= 0 {
		return NewAPIRedeemAPIKeySecret{}, fmt.Errorf("api_key_id 必须是正整数")
	}
	account, err := s.storedAccount(ctx, accountID)
	if err != nil {
		return NewAPIRedeemAPIKeySecret{}, err
	}
	key, err := s.fetchNewAPIRedeemFullKey(ctx, account, apiKeyID)
	if err != nil {
		return NewAPIRedeemAPIKeySecret{}, err
	}
	return NewAPIRedeemAPIKeySecret{AccountID: accountID, APIKeyID: apiKeyID, Key: key}, nil
}

// SaveVoucherFiles 保存一批文本兑换码文件，并返回每个文件的去重后兑换码数量。
func (s *NewAPIRedeemService) SaveVoucherFiles(ctx context.Context, files []*multipart.FileHeader) ([]NewAPIRedeemVoucherFile, error) {
	if len(files) == 0 {
		return nil, fmt.Errorf("请选择至少一个兑换码文件")
	}
	saved := make([]NewAPIRedeemVoucherFile, 0, len(files))
	for _, header := range files {
		if header == nil {
			return nil, fmt.Errorf("上传文件无效")
		}
		if header.Size > newAPIRedeemMaxUploadBytes {
			return nil, fmt.Errorf("文件 %s 超过 5 MiB 限制", header.Filename)
		}
		file, err := header.Open()
		if err != nil {
			return nil, fmt.Errorf("读取文件 %s: %w", header.Filename, err)
		}
		content, readErr := io.ReadAll(io.LimitReader(file, newAPIRedeemMaxUploadBytes+1))
		closeErr := file.Close()
		if readErr != nil {
			return nil, fmt.Errorf("读取文件 %s: %w", header.Filename, readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("关闭文件 %s: %w", header.Filename, closeErr)
		}
		if len(content) > newAPIRedeemMaxUploadBytes {
			return nil, fmt.Errorf("文件 %s 超过 5 MiB 限制", header.Filename)
		}
		voucher, err := s.SaveVoucherContent(ctx, header.Filename, content)
		if err != nil {
			return nil, err
		}
		saved = append(saved, voucher)
	}
	return saved, nil
}

// SaveVoucherContent 保存文本内容，供 HTTP 上传和服务测试共用。
func (s *NewAPIRedeemService) SaveVoucherContent(ctx context.Context, name string, content []byte) (NewAPIRedeemVoucherFile, error) {
	codes := parseNewAPIRedeemCodes(content)
	if len(codes) == 0 {
		return NewAPIRedeemVoucherFile{}, fmt.Errorf("文件中没有可用兑换码")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureStateLocked(ctx); err != nil {
		return NewAPIRedeemVoucherFile{}, err
	}
	fileID := uuid.NewString()
	storedName := fileID + ".txt"
	if err := os.WriteFile(filepath.Join(s.uploadsDir, storedName), content, 0o600); err != nil {
		return NewAPIRedeemVoucherFile{}, fmt.Errorf("保存兑换码文件: %w", err)
	}
	voucher := NewAPIRedeemVoucherFile{
		ID:         fileID,
		Name:       sanitizeNewAPIRedeemFileName(name),
		CodeCount:  len(codes),
		UploadedAt: s.now().UTC().Format(time.RFC3339),
	}
	s.state.Files = append(s.state.Files, newAPIRedeemStoredFile{NewAPIRedeemVoucherFile: voucher, StoredName: storedName})
	if err := s.saveStateLocked(); err != nil {
		_ = os.Remove(filepath.Join(s.uploadsDir, storedName))
		return NewAPIRedeemVoucherFile{}, err
	}
	return voucher, nil
}

// DeleteVoucherFile 删除本地文件和它在所有账号中的已兑换标记。
func (s *NewAPIRedeemService) DeleteVoucherFile(ctx context.Context, fileID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureStateLocked(ctx); err != nil {
		return err
	}
	if s.hasRunningRunLocked() {
		return fmt.Errorf("存在进行中的兑换任务，暂不能删除文件")
	}
	index := findNewAPIRedeemFileByID(s.state.Files, strings.TrimSpace(fileID))
	if index < 0 {
		return fmt.Errorf("未找到兑换码文件")
	}
	file := s.state.Files[index]
	s.state.Files = append(s.state.Files[:index], s.state.Files[index+1:]...)
	delete(s.state.PendingCodeRemovals, fileID)
	redemptions := s.state.Redemptions[:0]
	for _, redemption := range s.state.Redemptions {
		if redemption.FileID != fileID {
			redemptions = append(redemptions, redemption)
		}
	}
	s.state.Redemptions = redemptions
	if err := s.saveStateLocked(); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(s.uploadsDir, file.StoredName)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("删除兑换码文件: %w", err)
	}
	return nil
}

// StartRedemption 启动单个多账号并行后台任务，每个兑换码只交给一个账号验证。
func (s *NewAPIRedeemService) StartRedemption(ctx context.Context, request NewAPIRedeemStartRunRequest) (NewAPIRedeemRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureStateLocked(ctx); err != nil {
		return NewAPIRedeemRun{}, err
	}
	if s.hasRunningRunLocked() {
		return NewAPIRedeemRun{}, fmt.Errorf("已有兑换任务正在运行")
	}
	fileIDs := uniqueNewAPIRedeemIDs(request.FileIDs)
	accountIDs := uniqueNewAPIRedeemIDs(request.AccountIDs)
	if len(fileIDs) == 0 || len(accountIDs) == 0 {
		return NewAPIRedeemRun{}, fmt.Errorf("至少选择一个文件和一个账号")
	}
	totalCodes := 0
	for _, fileID := range fileIDs {
		fileIndex := findNewAPIRedeemFileByID(s.state.Files, fileID)
		if fileIndex < 0 {
			return NewAPIRedeemRun{}, fmt.Errorf("未找到兑换码文件")
		}
		totalCodes += s.state.Files[fileIndex].CodeCount
	}
	for _, accountID := range accountIDs {
		if findNewAPIRedeemAccountByID(s.state.Accounts, accountID) < 0 {
			return NewAPIRedeemRun{}, fmt.Errorf("未找到账号")
		}
	}
	run := NewAPIRedeemRun{
		ID:        uuid.NewString(),
		Status:    "running",
		StartedAt: s.now().UTC().Format(time.RFC3339),
		// 字段名为兼容既有状态文件保留，数值表示本次需要验证的唯一兑换码数。
		TotalPairs: totalCodes,
		Message:    fmt.Sprintf("正在使用 %d 个账号并行验证兑换码", len(accountIDs)),
		Logs:       []NewAPIRedeemRequestLog{},
	}
	s.state.Runs = append(s.state.Runs, run)
	if err := s.saveStateLocked(); err != nil {
		return NewAPIRedeemRun{}, err
	}
	runCtx, cancel := context.WithCancel(context.Background())
	s.runCancels[run.ID] = cancel
	go s.executeNewAPIRedeemRun(runCtx, run.ID, fileIDs, accountIDs)
	return cloneNewAPIRedeemRun(run), nil
}

// GetRun 返回单个异步兑换任务状态。
func (s *NewAPIRedeemService) GetRun(ctx context.Context, runID string) (NewAPIRedeemRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureStateLocked(ctx); err != nil {
		return NewAPIRedeemRun{}, err
	}
	index := findNewAPIRedeemRunByID(s.state.Runs, strings.TrimSpace(runID))
	if index < 0 {
		return NewAPIRedeemRun{}, fmt.Errorf("未找到兑换任务")
	}
	return cloneNewAPIRedeemRun(s.state.Runs[index]), nil
}

// CancelRun 请求停止仍未开始的兑换码请求。
func (s *NewAPIRedeemService) CancelRun(ctx context.Context, runID string) (NewAPIRedeemRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureStateLocked(ctx); err != nil {
		return NewAPIRedeemRun{}, err
	}
	index := findNewAPIRedeemRunByID(s.state.Runs, strings.TrimSpace(runID))
	if index < 0 {
		return NewAPIRedeemRun{}, fmt.Errorf("未找到兑换任务")
	}
	if s.state.Runs[index].Status != "running" {
		return cloneNewAPIRedeemRun(s.state.Runs[index]), nil
	}
	if cancel, ok := s.runCancels[runID]; ok {
		cancel()
	}
	s.state.Runs[index].Status = "cancelling"
	s.state.Runs[index].Message = "正在停止当前兑换请求"
	if err := s.saveStateLocked(); err != nil {
		return NewAPIRedeemRun{}, err
	}
	return cloneNewAPIRedeemRun(s.state.Runs[index]), nil
}

func (s *NewAPIRedeemService) executeNewAPIRedeemRun(ctx context.Context, runID string, fileIDs, accountIDs []string) {
	defer func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.runCancels, runID)
	}()
	accounts := make([]newAPIRedeemStoredAccount, 0, len(accountIDs))
	for _, accountID := range accountIDs {
		account, err := s.storedAccount(context.Background(), accountID)
		if err != nil {
			s.finishNewAPIRedeemRun(runID, "failed", err.Error())
			return
		}
		accounts = append(accounts, account)
	}
	runNetwork := newNewAPIRedeemRunNetwork(s, runID, len(accounts))
	runNetwork.inspect(ctx)
	processedFiles := 0
filesLoop:
	for _, fileID := range fileIDs {
		fileCounted := false
		for {
			if ctx.Err() != nil {
				break filesLoop
			}
			file, codes, err := s.loadNewAPIRedeemRunFile(fileID)
			if err != nil {
				s.finishNewAPIRedeemRun(runID, "failed", err.Error())
				return
			}
			pendingAccounts := make([]newAPIRedeemStoredAccount, 0, len(accounts))
			for _, account := range accounts {
				if !s.hasNewAPIRedeemSuccess(file.ID, account.ID) {
					pendingAccounts = append(pendingAccounts, account)
				}
			}
			if len(pendingAccounts) == 0 {
				break
			}
			if !fileCounted {
				processedFiles++
				fileCounted = true
			}
			if len(codes) == 0 {
				break
			}
			if err := s.executeNewAPIRedeemFile(ctx, runID, file, codes, pendingAccounts, runNetwork); err != nil {
				if !errors.Is(err, errNewAPIRedeemNoAvailableExit) {
					s.finishNewAPIRedeemRun(runID, "failed", err.Error())
					return
				}
				if !s.waitNewAPIRedeemNetworkRetry(ctx, runID, err) {
					break filesLoop
				}
				runNetwork = newNewAPIRedeemRunNetwork(s, runID, len(accounts))
				runNetwork.inspect(ctx)
				continue
			}
			break
		}
	}
	if err := s.flushAllNewAPIRedeemPendingRemovals(); err != nil {
		s.finishNewAPIRedeemRun(runID, "failed", err.Error())
		return
	}
	if ctx.Err() != nil {
		s.finishNewAPIRedeemRun(runID, "cancelled", "兑换任务已停止")
		return
	}
	if processedFiles == 0 {
		s.finishNewAPIRedeemRun(runID, "completed", "所选账号和文件均已标记，已全部跳过")
		return
	}
	s.finishNewAPIRedeemRun(runID, "completed", "兑换任务已完成")
}

// executeNewAPIRedeemFile 只处理一个文件；该文件完成后才允许进入下一个文件。
func (s *NewAPIRedeemService) executeNewAPIRedeemFile(ctx context.Context, runID string, file newAPIRedeemStoredFile, codes []string, accounts []newAPIRedeemStoredAccount, network *newAPIRedeemRunNetwork) error {
	workerCtx, cancelWorkers := context.WithCancel(ctx)
	defer cancelWorkers()
	jobs := make(chan newAPIRedeemWorkItem)
	results := make(chan newAPIRedeemWorkResult, len(accounts))
	orderedCodes := s.prioritizeNewAPIRedeemCodes(file.ID, codes)
	queue := make([]newAPIRedeemWorkItem, 0, len(orderedCodes))
	for _, code := range orderedCodes {
		queue = append(queue, newAPIRedeemWorkItem{File: file, Code: code})
	}

	var workers sync.WaitGroup
	for _, account := range accounts {
		account := account
		workers.Add(1)
		go func() {
			defer workers.Done()
			s.executeNewAPIRedeemWorker(workerCtx, runID, account, jobs, results, network)
		}()
	}
	activeWorkers := len(accounts)
	inFlight := 0
	for activeWorkers > 0 && (len(queue) > 0 || inFlight > 0) {
		var nextJob chan<- newAPIRedeemWorkItem
		var item newAPIRedeemWorkItem
		if len(queue) > 0 {
			nextJob = jobs
			item = queue[0]
		}
		select {
		case <-ctx.Done():
			cancelWorkers()
			workers.Wait()
			return nil
		case nextJob <- item:
			queue = queue[1:]
			inFlight++
		case result := <-results:
			inFlight--
			terminalAccount := result.Err != nil || result.Outcome == "redeemed" || result.Outcome == "already_redeemed"
			if terminalAccount {
				activeWorkers--
			}
			if result.Err != nil {
				cancelWorkers()
				workers.Wait()
				return result.Err
			}
			if result.Outcome == "already_redeemed" && activeWorkers > 0 {
				// 已兑换码对其他未标记账号仍可能有效，放回队首优先消费。
				queue = append([]newAPIRedeemWorkItem{result.Item}, queue...)
			}
		}
	}
	cancelWorkers()
	workers.Wait()
	return nil
}

// prioritizeNewAPIRedeemCodes 把历史上仅标记为已兑换、但仍保留在文件中的码移动到队首。
func (s *NewAPIRedeemService) prioritizeNewAPIRedeemCodes(fileID string, codes []string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	priority := make(map[string]struct{})
	for _, redemption := range s.state.Redemptions {
		if redemption.FileID == fileID && redemption.Result == "already_redeemed" && redemption.Code != "" {
			priority[redemption.Code] = struct{}{}
		}
	}
	if len(priority) == 0 {
		return append([]string{}, codes...)
	}
	ordered := make([]string, 0, len(codes))
	for _, code := range codes {
		if _, ok := priority[code]; ok {
			ordered = append(ordered, code)
		}
	}
	for _, code := range codes {
		if _, ok := priority[code]; !ok {
			ordered = append(ordered, code)
		}
	}
	return ordered
}

func (s *NewAPIRedeemService) fetchNewAPIRedeemAccount(ctx context.Context, account newAPIRedeemStoredAccount) (NewAPIRedeemAccount, error) {
	self, err := s.callNewAPI(ctx, http.MethodGet, "/api/user/self", account, nil)
	if err != nil {
		return NewAPIRedeemAccount{}, err
	}
	if !self.OK {
		return NewAPIRedeemAccount{}, fmt.Errorf("读取账号信息失败: %s", firstNewAPIRedeemText(self.Message, "上游拒绝请求"))
	}
	data := newAPIRedeemMap(self.Payload["data"])
	if len(data) == 0 {
		data = self.Payload
	}
	snapshot := NewAPIRedeemAccount{
		ID:              account.ID,
		UserID:          account.UserID,
		Username:        newAPIRedeemString(data["username"]),
		Email:           newAPIRedeemString(data["email"]),
		Group:           newAPIRedeemString(data["group"]),
		Quota:           newAPIRedeemNumber(data["quota"]),
		UsedQuota:       newAPIRedeemNumber(data["used_quota"]),
		APIKeys:         []NewAPIRedeemAPIKey{},
		AvailableGroups: []string{},
		Status:          "ready",
		Message:         "已刷新上游账号信息",
		RefreshedAt:     s.now().UTC().Format(time.RFC3339),
	}
	groups := map[string]struct{}{}
	if snapshot.Group != "" {
		groups[snapshot.Group] = struct{}{}
	}
	tokens, err := s.callNewAPI(ctx, http.MethodGet, "/api/token/?p=1&size=100", account, nil)
	if err != nil {
		return NewAPIRedeemAccount{}, err
	}
	if !tokens.OK {
		return NewAPIRedeemAccount{}, fmt.Errorf("读取 API Key 失败: %s", firstNewAPIRedeemText(tokens.Message, "上游拒绝请求"))
	}
	items := newAPIRedeemSlice(newAPIRedeemMap(tokens.Payload["data"])["items"])
	for _, raw := range items {
		item := newAPIRedeemMap(raw)
		keyID := newAPIRedeemInt64(item["id"])
		if keyID <= 0 {
			continue
		}
		group := newAPIRedeemString(item["group"])
		if group != "" {
			groups[group] = struct{}{}
		}
		// 刷新只依赖列表接口返回的部分 key 做脱敏展示；完整明文仅在显式 reveal 时拉取。
		masked := newAPIRedeemMask(newAPIRedeemString(item["key"]))
		snapshot.APIKeys = append(snapshot.APIKeys, NewAPIRedeemAPIKey{
			ID:        keyID,
			Name:      firstNewAPIRedeemText(newAPIRedeemString(item["name"]), "未命名"),
			Group:     group,
			MaskedKey: masked,
		})
	}
	for _, endpoint := range []string{"/api/user/self/groups", "/api/user/groups", "/api/pricing", "/api/user/available_groups"} {
		result, queryErr := s.callNewAPI(ctx, http.MethodGet, endpoint, account, nil)
		if queryErr != nil || !result.OK {
			continue
		}
		collectNewAPIRedeemGroups(result.Payload["data"], groups)
		collectNewAPIRedeemGroups(result.Payload["group_ratio"], groups)
	}
	for group := range groups {
		snapshot.AvailableGroups = append(snapshot.AvailableGroups, group)
	}
	sort.Strings(snapshot.AvailableGroups)
	sort.Slice(snapshot.APIKeys, func(i, j int) bool { return snapshot.APIKeys[i].ID < snapshot.APIKeys[j].ID })
	return snapshot, nil
}

func (s *NewAPIRedeemService) storedAccount(ctx context.Context, accountID string) (newAPIRedeemStoredAccount, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureStateLocked(ctx); err != nil {
		return newAPIRedeemStoredAccount{}, err
	}
	index := findNewAPIRedeemAccountByID(s.state.Accounts, strings.TrimSpace(accountID))
	if index < 0 {
		return newAPIRedeemStoredAccount{}, fmt.Errorf("未找到账号")
	}
	return s.state.Accounts[index], nil
}

func (s *NewAPIRedeemService) fetchNewAPIRedeemToken(ctx context.Context, account newAPIRedeemStoredAccount, apiKeyID int64) (map[string]any, error) {
	result, err := s.callNewAPI(ctx, http.MethodGet, "/api/token/?p=1&size=100", account, nil)
	if err != nil {
		return nil, err
	}
	if !result.OK {
		return nil, fmt.Errorf("读取 API Key 失败: %s", firstNewAPIRedeemText(result.Message, "上游拒绝请求"))
	}
	for _, raw := range newAPIRedeemSlice(newAPIRedeemMap(result.Payload["data"])["items"]) {
		item := newAPIRedeemMap(raw)
		if newAPIRedeemInt64(item["id"]) == apiKeyID {
			return item, nil
		}
	}
	return nil, fmt.Errorf("未找到上游 API Key: %d", apiKeyID)
}

func (s *NewAPIRedeemService) fetchNewAPIRedeemFullKey(ctx context.Context, account newAPIRedeemStoredAccount, apiKeyID int64) (string, error) {
	result, err := s.callNewAPI(ctx, http.MethodPost, fmt.Sprintf("/api/token/%d/key", apiKeyID), account, nil)
	if err != nil {
		return "", err
	}
	if !result.OK {
		return "", fmt.Errorf("读取完整 API Key 失败: %s", firstNewAPIRedeemText(result.Message, "上游拒绝请求"))
	}
	key := newAPIRedeemString(newAPIRedeemMap(result.Payload["data"])["key"])
	if key == "" || strings.Contains(key, "*") {
		return "", fmt.Errorf("上游响应未包含完整 API Key")
	}
	return key, nil
}

func (s *NewAPIRedeemService) callNewAPI(ctx context.Context, method, path string, account newAPIRedeemStoredAccount, body any) (newAPIRedeemUpstreamResponse, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return newAPIRedeemUpstreamResponse{}, fmt.Errorf("编码上游请求: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.baseURL+path, reader)
	if err != nil {
		return newAPIRedeemUpstreamResponse{}, fmt.Errorf("创建上游请求: %w", err)
	}
	// 同账号复用自己的连接池；节点切换闸门会统一关闭旧出口上的空闲连接。
	client, browserProfile, err := s.newAPIRedeemAccountHTTPClient(account)
	if err != nil {
		return newAPIRedeemUpstreamResponse{}, err
	}
	applyNewAPIRedeemBrowserHeaders(req, s.baseURL, browserProfile)
	req.Header.Set("Authorization", "Bearer "+account.AccessKey)
	req.Header.Set("New-Api-User", account.UserID)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := client.Do(req)
	if err != nil {
		return newAPIRedeemUpstreamResponse{}, fmt.Errorf("请求 NewAPI 失败: %w", err)
	}
	defer response.Body.Close()
	content, err := io.ReadAll(io.LimitReader(response.Body, 2*1024*1024))
	if err != nil {
		return newAPIRedeemUpstreamResponse{}, fmt.Errorf("读取 NewAPI 响应: %w", err)
	}
	payload := map[string]any{}
	if len(content) > 0 {
		if err := json.Unmarshal(content, &payload); err != nil {
			return newAPIRedeemUpstreamResponse{}, fmt.Errorf("解析 NewAPI 响应: %w", err)
		}
	}
	message := firstNewAPIRedeemText(newAPIRedeemString(payload["message"]), newAPIRedeemString(payload["error"]))
	if message == "" {
		message = newAPIRedeemString(payload["data"])
	}
	ok := response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices
	if success, exists := payload["success"].(bool); exists {
		ok = ok && success
	}
	return newAPIRedeemUpstreamResponse{OK: ok, StatusCode: response.StatusCode, Message: message, Payload: payload}, nil
}

func (s *NewAPIRedeemService) loadNewAPIRedeemRunFile(fileID string) (newAPIRedeemStoredFile, []string, error) {
	s.mu.Lock()
	if err := s.ensureStateLocked(context.Background()); err != nil {
		s.mu.Unlock()
		return newAPIRedeemStoredFile{}, nil, err
	}
	index := findNewAPIRedeemFileByID(s.state.Files, fileID)
	if index < 0 {
		s.mu.Unlock()
		return newAPIRedeemStoredFile{}, nil, fmt.Errorf("未找到兑换码文件")
	}
	file := s.state.Files[index]
	s.mu.Unlock()
	content, err := os.ReadFile(filepath.Join(s.uploadsDir, file.StoredName))
	if err != nil {
		return newAPIRedeemStoredFile{}, nil, fmt.Errorf("读取兑换码文件: %w", err)
	}
	return file, parseNewAPIRedeemCodes(content), nil
}

// stageNewAPIRedeemCodeRemoval 先记录待删除码；批量压缩会在阈值或任务结束时原子更新源文件。
func (s *NewAPIRedeemService) stageNewAPIRedeemCodeRemoval(fileID, code string) (bool, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureStateLocked(context.Background()); err != nil {
		return false, 0, err
	}
	index := findNewAPIRedeemFileByID(s.state.Files, fileID)
	if index < 0 {
		return false, 0, fmt.Errorf("未找到兑换码文件")
	}
	if s.state.PendingCodeRemovals == nil {
		s.state.PendingCodeRemovals = map[string][]string{}
	}
	for _, pendingCode := range s.state.PendingCodeRemovals[fileID] {
		if pendingCode == code {
			return false, len(s.state.PendingCodeRemovals[fileID]), nil
		}
	}
	s.state.PendingCodeRemovals[fileID] = append(s.state.PendingCodeRemovals[fileID], code)
	if s.state.Files[index].CodeCount > 0 {
		s.state.Files[index].CodeCount--
	}
	return true, len(s.state.PendingCodeRemovals[fileID]), nil
}

// flushNewAPIRedeemPendingRemovals 将一个文件积累的待删除码一次性压缩到磁盘。
func (s *NewAPIRedeemService) flushNewAPIRedeemPendingRemovals(fileID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureStateLocked(context.Background()); err != nil {
		return err
	}
	return s.flushNewAPIRedeemPendingRemovalsLocked(fileID)
}

// flushAllNewAPIRedeemPendingRemovals 在任务结束、取消或重启恢复时强制收敛所有源文件。
func (s *NewAPIRedeemService) flushAllNewAPIRedeemPendingRemovals() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureStateLocked(context.Background()); err != nil {
		return err
	}
	fileIDs := make([]string, 0, len(s.state.PendingCodeRemovals))
	for fileID := range s.state.PendingCodeRemovals {
		fileIDs = append(fileIDs, fileID)
	}
	for _, fileID := range fileIDs {
		if err := s.flushNewAPIRedeemPendingRemovalsLocked(fileID); err != nil {
			return err
		}
	}
	return nil
}

func (s *NewAPIRedeemService) flushNewAPIRedeemPendingRemovalsLocked(fileID string) error {
	pending := append([]string{}, s.state.PendingCodeRemovals[fileID]...)
	if len(pending) == 0 {
		delete(s.state.PendingCodeRemovals, fileID)
		return nil
	}
	index := findNewAPIRedeemFileByID(s.state.Files, fileID)
	if index < 0 {
		return fmt.Errorf("未找到待压缩的兑换码文件")
	}
	path := filepath.Join(s.uploadsDir, s.state.Files[index].StoredName)
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取兑换码文件: %w", err)
	}
	removeSet := make(map[string]struct{}, len(pending))
	for _, code := range pending {
		removeSet[code] = struct{}{}
	}
	remaining, _ := removeNewAPIRedeemCodes(content, removeSet)
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, remaining, 0o600); err != nil {
		return fmt.Errorf("写入兑换码文件: %w", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		return fmt.Errorf("提交兑换码文件: %w", err)
	}
	delete(s.state.PendingCodeRemovals, fileID)
	s.state.Files[index].CodeCount = len(parseNewAPIRedeemCodes(remaining))
	if err := s.saveStateLocked(); err != nil {
		s.state.PendingCodeRemovals[fileID] = pending
		return fmt.Errorf("同步兑换码文件状态: %w", err)
	}
	return nil
}

func (s *NewAPIRedeemService) hasNewAPIRedeemSuccess(fileID, accountID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, redemption := range s.state.Redemptions {
		if redemption.FileID == fileID && redemption.AccountID == accountID && (redemption.Result == "redeemed" || redemption.Result == "already_redeemed") {
			return true
		}
	}
	return false
}

// recordNewAPIRedeemCurrentRequest 让轮询页面立即显示当前正在处理的兑换码文件。
func (s *NewAPIRedeemService) recordNewAPIRedeemCurrentRequest(runID string, file newAPIRedeemStoredFile) {
	s.mutateNewAPIRedeemRunInMemory(runID, func(run *NewAPIRedeemRun) {
		run.CurrentFileID = file.ID
		run.CurrentFileName = file.Name
	})
}

// recordNewAPIRedeemRequest 持久化每次请求及其节点信息，并仅对成功类结果建立账号兑换标记。
func (s *NewAPIRedeemService) recordNewAPIRedeemRequest(runID string, file newAPIRedeemStoredFile, account newAPIRedeemStoredAccount, code, result, message string, statusCode int, removedFromFile bool, network newAPIRedeemNetworkIdentity, switched *newAPIRedeemNetworkSwitch, completedMessage string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	index := findNewAPIRedeemRunByID(s.state.Runs, runID)
	if index < 0 {
		return nil
	}
	log := NewAPIRedeemRequestLog{
		At:        s.now().UTC().Format(time.RFC3339),
		FileID:    file.ID,
		FileName:  file.Name,
		AccountID: account.ID,
		UserID:    account.UserID,
		BrowserFingerprint: func() string {
			if profile, ok := newAPIRedeemBrowserProfileByID(account.BrowserProfileID); ok {
				return profile.Name
			}
			return ""
		}(),
		// 管理端兑换日志需要完整码以便核对，不脱敏。
		Code:            code,
		Result:          result,
		Message:         message,
		StatusCode:      statusCode,
		RemovedFromFile: removedFromFile,
		Node:            network.Node,
		ExitIP:          network.ExitIP,
	}
	if switched != nil {
		log.SwitchedNode = switched.ToNode
		log.SwitchedExitIP = switched.ToIP
		log.SwitchMessage = switched.Message
	}
	s.state.Runs[index].Attempts++
	s.state.Runs[index].Logs = append(s.state.Runs[index].Logs, log)
	trimNewAPIRedeemStoredLogs(s.state.Runs, newAPIRedeemMaxStoredLogs)
	if completedMessage != "" {
		s.state.Runs[index].CompletedPairs++
		s.state.Runs[index].Message = completedMessage
	}
	if result == "redeemed" || result == "already_redeemed" {
		s.state.Runs[index].Successes++
		found := false
		for redemptionIndex := range s.state.Redemptions {
			redemption := &s.state.Redemptions[redemptionIndex]
			if redemption.FileID == file.ID && redemption.AccountID == account.ID {
				redemption.Code = code
				redemption.CodeHash = newAPIRedeemCodeHash(file.ID, code)
				redemption.Result = result
				redemption.RedeemedAt = log.At
				found = true
				break
			}
		}
		if !found {
			s.state.Redemptions = append(s.state.Redemptions, newAPIRedeemRedemption{
				FileID: file.ID, AccountID: account.ID, Code: code, CodeHash: newAPIRedeemCodeHash(file.ID, code), Result: result, RedeemedAt: log.At,
			})
		}
	}
	if result == "rate_limited" || s.state.Runs[index].Attempts%newAPIRedeemCheckpointInterval == 0 {
		return s.saveStateLocked()
	}
	return nil
}

// recordNewAPIRedeemNetwork 更新任务当前节点、出口 IP 和最近一次切换结果。
func (s *NewAPIRedeemService) recordNewAPIRedeemNetwork(runID string, identity newAPIRedeemNetworkIdentity, switchIncrement int, message string) {
	s.mutateNewAPIRedeemRun(runID, func(run *NewAPIRedeemRun) {
		if identity.Node != "" {
			run.CurrentNode = identity.Node
		}
		if identity.ExitIP != "" {
			run.CurrentExitIP = identity.ExitIP
		}
		run.SwitchCount += switchIncrement
		run.NetworkMessage = message
	})
}

func (s *NewAPIRedeemService) completeNewAPIRedeemPair(runID, message string) {
	s.mutateNewAPIRedeemRun(runID, func(run *NewAPIRedeemRun) {
		run.CompletedPairs++
		run.Message = message
	})
}

func (s *NewAPIRedeemService) mutateNewAPIRedeemRunInMemory(runID string, mutate func(*NewAPIRedeemRun)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	index := findNewAPIRedeemRunByID(s.state.Runs, runID)
	if index >= 0 {
		mutate(&s.state.Runs[index])
	}
}

func (s *NewAPIRedeemService) finishNewAPIRedeemRun(runID, status, message string) {
	s.mutateNewAPIRedeemRun(runID, func(run *NewAPIRedeemRun) {
		run.Status = status
		run.Message = message
		run.FinishedAt = s.now().UTC().Format(time.RFC3339)
	})
}

func (s *NewAPIRedeemService) mutateNewAPIRedeemRun(runID string, mutate func(*NewAPIRedeemRun)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	index := findNewAPIRedeemRunByID(s.state.Runs, runID)
	if index < 0 {
		return
	}
	mutate(&s.state.Runs[index])
	_ = s.saveStateLocked()
}

func (s *NewAPIRedeemService) waitNewAPIRedeemInterval(ctx context.Context) bool {
	timer := time.NewTimer(s.requestInterval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// waitNewAPIRedeemNetworkRetry 在所有出口耗尽时让整轮任务共享等待，并允许取消立即打断等待。
func (s *NewAPIRedeemService) waitNewAPIRedeemNetworkRetry(ctx context.Context, runID string, cause error) bool {
	delayText := s.networkRetryDelay.String()
	if s.networkRetryDelay%time.Minute == 0 {
		delayText = fmt.Sprintf("%d 分钟", s.networkRetryDelay/time.Minute)
	}
	s.mutateNewAPIRedeemRun(runID, func(run *NewAPIRedeemRun) {
		run.Message = fmt.Sprintf("所有出口暂不可用，暂停 %s 后重试当前文件", delayText)
		run.NetworkMessage = cause.Error()
	})
	timer := time.NewTimer(s.networkRetryDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		s.mutateNewAPIRedeemRun(runID, func(run *NewAPIRedeemRun) {
			run.Message = "暂停结束，正在重新初始化出口并重试当前文件"
		})
		return true
	}
}

func (s *NewAPIRedeemService) ensureStateLocked(context.Context) error {
	if s.initialized {
		return nil
	}
	if err := os.MkdirAll(s.uploadsDir, 0o700); err != nil {
		return fmt.Errorf("创建兑换码目录: %w", err)
	}
	content, err := os.ReadFile(s.statePath)
	if errors.Is(err, os.ErrNotExist) {
		s.state = newAPIRedeemState{Version: 1, Accounts: []newAPIRedeemStoredAccount{}, Files: []newAPIRedeemStoredFile{}, Redemptions: []newAPIRedeemRedemption{}, Runs: []NewAPIRedeemRun{}, PendingCodeRemovals: map[string][]string{}}
		s.initialized = true
		return nil
	}
	if err != nil {
		return fmt.Errorf("读取兑换工具状态: %w", err)
	}
	if err := json.Unmarshal(content, &s.state); err != nil {
		return fmt.Errorf("解析兑换工具状态: %w", err)
	}
	if s.state.Version != 1 {
		return fmt.Errorf("不支持的兑换工具状态版本: %d", s.state.Version)
	}
	if s.state.Accounts == nil {
		s.state.Accounts = []newAPIRedeemStoredAccount{}
	}
	if s.state.Files == nil {
		s.state.Files = []newAPIRedeemStoredFile{}
	}
	if s.state.Redemptions == nil {
		s.state.Redemptions = []newAPIRedeemRedemption{}
	}
	if s.state.Runs == nil {
		s.state.Runs = []NewAPIRedeemRun{}
	}
	if s.state.PendingCodeRemovals == nil {
		s.state.PendingCodeRemovals = map[string][]string{}
	}
	profilesChanged := ensureNewAPIRedeemBrowserProfileAssignments(s.state.Accounts)
	stateChanged := profilesChanged || trimNewAPIRedeemStoredLogs(s.state.Runs, newAPIRedeemMaxStoredLogs)
	for index := range s.state.Runs {
		run := &s.state.Runs[index]
		if run.Status != "running" && run.Status != "cancelling" {
			continue
		}
		run.Status = "interrupted"
		run.Message = "服务重启，兑换任务已中断"
		run.FinishedAt = s.now().UTC().Format(time.RFC3339)
		stateChanged = true
	}
	fileIDs := make([]string, 0, len(s.state.PendingCodeRemovals))
	for fileID := range s.state.PendingCodeRemovals {
		fileIDs = append(fileIDs, fileID)
	}
	for _, fileID := range fileIDs {
		if err := s.flushNewAPIRedeemPendingRemovalsLocked(fileID); err != nil {
			return fmt.Errorf("恢复兑换码文件: %w", err)
		}
		stateChanged = true
	}
	if stateChanged {
		if err := s.saveStateLocked(); err != nil {
			return fmt.Errorf("更新兑换工具状态: %w", err)
		}
	}
	s.initialized = true
	return nil
}

func (s *NewAPIRedeemService) saveStateLocked() error {
	content, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return fmt.Errorf("编码兑换工具状态: %w", err)
	}
	temporary := s.statePath + ".tmp"
	if err := os.WriteFile(temporary, content, 0o600); err != nil {
		return fmt.Errorf("写入兑换工具状态: %w", err)
	}
	if err := os.Rename(temporary, s.statePath); err != nil {
		return fmt.Errorf("提交兑换工具状态: %w", err)
	}
	return nil
}

func (s *NewAPIRedeemService) hasRunningRunLocked() bool {
	for _, run := range s.state.Runs {
		if run.Status == "running" || run.Status == "cancelling" {
			return true
		}
	}
	return false
}

func projectNewAPIRedeemAccount(account newAPIRedeemStoredAccount) NewAPIRedeemAccount {
	projected := account.Snapshot
	projected.APIKeys = append([]NewAPIRedeemAPIKey(nil), account.Snapshot.APIKeys...)
	for index := range projected.APIKeys {
		projected.APIKeys[index].ReferencedAccounts = nil
		projected.APIKeys[index].TargetAccounts = nil
	}
	projected.AvailableGroups = append([]string(nil), account.Snapshot.AvailableGroups...)
	projected.ID = account.ID
	projected.UserID = account.UserID
	projected.AccessKeyMasked = newAPIRedeemMask(account.AccessKey)
	if profile, ok := newAPIRedeemBrowserProfileByID(account.BrowserProfileID); ok {
		projected.BrowserFingerprint = profile.Name
	}
	if projected.APIKeys == nil {
		projected.APIKeys = []NewAPIRedeemAPIKey{}
	}
	if projected.AvailableGroups == nil {
		projected.AvailableGroups = []string{}
	}
	projected.RedeemedCodes = []NewAPIRedeemAccountRedemption{}
	return projected
}

// attachNewAPIRedeemAccountReferences 为兑换 Key 附加数据库引用和同 base_url 可关联账号。
func attachNewAPIRedeemAccountReferences(redeemAccounts []NewAPIRedeemAccount, accounts []Account, baseURL string) {
	for accountIndex := range redeemAccounts {
		for keyIndex := range redeemAccounts[accountIndex].APIKeys {
			key := &redeemAccounts[accountIndex].APIKeys[keyIndex]
			for index := range accounts {
				account := &accounts[index]
				if account.Type != AccountTypeAPIKey {
					continue
				}
				referenced := false
				for _, existing := range storedAccountAPIKeys(account.Credentials) {
					if newAPIGeneratedKeyMatchesStored(key.MaskedKey, existing) {
						referenced = true
						break
					}
				}
				ref := NewAPIRedeemAccountReference{ID: account.ID, Name: account.Name, Referenced: referenced}
				if referenced {
					key.ReferencedAccounts = append(key.ReferencedAccounts, ref)
				}
				if newAPIAccountBaseURLsMatch(account.GetCredential("base_url"), baseURL) {
					key.TargetAccounts = append(key.TargetAccounts, ref)
				}
			}
			sort.SliceStable(key.TargetAccounts, func(left, right int) bool {
				return key.TargetAccounts[left].Referenced && !key.TargetAccounts[right].Referenced
			})
		}
		sort.SliceStable(redeemAccounts[accountIndex].APIKeys, func(left, right int) bool {
			return len(redeemAccounts[accountIndex].APIKeys[left].ReferencedAccounts) > 0 && len(redeemAccounts[accountIndex].APIKeys[right].ReferencedAccounts) == 0
		})
	}
}

// projectNewAPIRedeemAccountRedemptions 返回账号的文件级唯一标记，并合并旧状态中的兑换码级重复记录。
func (s *NewAPIRedeemService) projectNewAPIRedeemAccountRedemptions(accountID string) []NewAPIRedeemAccountRedemption {
	fileNames := make(map[string]string, len(s.state.Files))
	for _, file := range s.state.Files {
		fileNames[file.ID] = file.Name
	}
	byFile := make(map[string]NewAPIRedeemAccountRedemption)
	for _, redemption := range s.state.Redemptions {
		if redemption.AccountID != accountID || (redemption.Result != "redeemed" && redemption.Result != "already_redeemed") {
			continue
		}
		marker := NewAPIRedeemAccountRedemption{
			FileID: redemption.FileID, FileName: fileNames[redemption.FileID], Result: redemption.Result, RedeemedAt: redemption.RedeemedAt,
		}
		if marker.FileName == "" {
			for runIndex := len(s.state.Runs) - 1; runIndex >= 0 && marker.FileName == ""; runIndex-- {
				for logIndex := len(s.state.Runs[runIndex].Logs) - 1; logIndex >= 0; logIndex-- {
					log := s.state.Runs[runIndex].Logs[logIndex]
					if log.AccountID == accountID && log.FileID == redemption.FileID {
						marker.FileName = log.FileName
						break
					}
				}
			}
		}
		if current, exists := byFile[redemption.FileID]; !exists || marker.RedeemedAt > current.RedeemedAt {
			byFile[redemption.FileID] = marker
		}
	}
	items := make([]NewAPIRedeemAccountRedemption, 0, len(byFile))
	for _, marker := range byFile {
		items = append(items, marker)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].RedeemedAt > items[j].RedeemedAt })
	return items
}

func cloneNewAPIRedeemRun(run NewAPIRedeemRun) NewAPIRedeemRun {
	clone := run
	clone.Logs = append([]NewAPIRedeemRequestLog{}, run.Logs...)
	return clone
}

// trimNewAPIRedeemStoredLogs 从最新任务向前保留日志，确保历史任务合计不超过指定上限。
func trimNewAPIRedeemStoredLogs(runs []NewAPIRedeemRun, limit int) bool {
	remaining := limit
	changed := false
	for index := len(runs) - 1; index >= 0; index-- {
		logCount := len(runs[index].Logs)
		if remaining <= 0 {
			if logCount > 0 {
				runs[index].Logs = []NewAPIRedeemRequestLog{}
				changed = true
			}
			continue
		}
		if logCount > remaining {
			runs[index].Logs = append([]NewAPIRedeemRequestLog{}, runs[index].Logs[logCount-remaining:]...)
			logCount = remaining
			changed = true
		}
		remaining -= logCount
	}
	return changed
}

func parseNewAPIRedeemCodes(content []byte) []string {
	seen := map[string]struct{}{}
	codes := []string{}
	for _, raw := range strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n") {
		code := strings.TrimSpace(strings.TrimPrefix(raw, "\ufeff"))
		if code == "" {
			continue
		}
		if _, exists := seen[code]; exists {
			continue
		}
		seen[code] = struct{}{}
		codes = append(codes, code)
	}
	return codes
}

// removeNewAPIRedeemCode 移除与指定兑换码相同的所有文本行，保留其余文件内容顺序。
func removeNewAPIRedeemCode(content []byte, code string) ([]byte, bool) {
	return removeNewAPIRedeemCodes(content, map[string]struct{}{code: {}})
}

func removeNewAPIRedeemCodes(content []byte, codes map[string]struct{}) ([]byte, bool) {
	lines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
	remaining := make([]string, 0, len(lines))
	removed := false
	for _, line := range lines {
		candidate := strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
		if _, shouldRemove := codes[candidate]; shouldRemove {
			removed = true
			continue
		}
		remaining = append(remaining, line)
	}
	if !removed {
		return content, false
	}
	return []byte(strings.Join(remaining, "\n")), true
}

func sanitizeNewAPIRedeemFileName(name string) string {
	clean := strings.TrimSpace(filepath.Base(name))
	if clean == "" || clean == "." {
		return "codes.txt"
	}
	return clean
}

func uniqueNewAPIRedeemIDs(ids []string) []string {
	seen := map[string]struct{}{}
	unique := []string{}
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique
}

func findNewAPIRedeemAccountByID(accounts []newAPIRedeemStoredAccount, id string) int {
	for index := range accounts {
		if accounts[index].ID == id {
			return index
		}
	}
	return -1
}

func findNewAPIRedeemAccountByUserID(accounts []newAPIRedeemStoredAccount, userID string) int {
	for index := range accounts {
		if accounts[index].UserID == userID {
			return index
		}
	}
	return -1
}

func findNewAPIRedeemFileByID(files []newAPIRedeemStoredFile, id string) int {
	for index := range files {
		if files[index].ID == id {
			return index
		}
	}
	return -1
}

func findNewAPIRedeemRunByID(runs []NewAPIRedeemRun, id string) int {
	for index := range runs {
		if runs[index].ID == id {
			return index
		}
	}
	return -1
}

func newAPIRedeemCodeHash(fileID, code string) string {
	digest := sha256.Sum256([]byte(fileID + "\x00" + code))
	return hex.EncodeToString(digest[:])
}

func newAPIRedeemMask(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= 8 {
		return "***"
	}
	return value[:4] + "***" + value[len(value)-4:]
}

func isNewAPIRedeemAlreadyRedeemed(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	for _, phrase := range []string{"该分组兑换码每人限兑 1 次，您已达到上限", "already redeemed", "already used", "already top up", "已兑换", "已经兑换", "兑换过", "已使用", "已经使用"} {
		if strings.Contains(text, phrase) {
			return true
		}
	}
	return false
}

// isNewAPIRedeemRateLimited 将 HTTP 429 和常见限流消息标为可重试，保留兑换码文件内容。
func isNewAPIRedeemRateLimited(statusCode int, message string) bool {
	if statusCode == http.StatusTooManyRequests {
		return true
	}
	text := strings.ToLower(strings.TrimSpace(message))
	for _, phrase := range []string{"rate limit", "too many requests", "请求过于频繁", "请求频繁", "访问频繁", "限流", "频率过高"} {
		if strings.Contains(text, phrase) {
			return true
		}
	}
	return false
}

// isNewAPIRedeemInvalidCode 识别上游确认已无法再兑换的码，包括 cun.ai 用于已使用兑换码的通用失败文案。
func isNewAPIRedeemInvalidCode(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	for _, phrase := range []string{"兑换失败，请稍后重试", "redemption failed, please try again later", "兑换码无效", "无效兑换码", "兑换码不存在", "兑换码已失效", "兑换码已过期", "兑换码错误", "invalid code", "invalid voucher", "voucher invalid", "code not found", "voucher not found", "expired code", "voucher expired"} {
		if strings.Contains(text, phrase) {
			return true
		}
	}
	return false
}

func firstNewAPIRedeemText(values ...string) string {
	for _, value := range values {
		if clean := strings.TrimSpace(value); clean != "" {
			return clean
		}
	}
	return ""
}

func newAPIRedeemMap(value any) map[string]any {
	if mapped, ok := value.(map[string]any); ok {
		return mapped
	}
	return map[string]any{}
}

func newAPIRedeemSlice(value any) []any {
	if values, ok := value.([]any); ok {
		return values
	}
	return []any{}
}

func newAPIRedeemString(value any) string {
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func newAPIRedeemNumber(value any) string {
	switch typed := value.(type) {
	case float64:
		return fmt.Sprintf("%g", typed)
	case float32:
		return fmt.Sprintf("%g", typed)
	case int:
		return fmt.Sprintf("%d", typed)
	case int64:
		return fmt.Sprintf("%d", typed)
	case json.Number:
		return typed.String()
	case string:
		return strings.TrimSpace(typed)
	default:
		return ""
	}
}

func newAPIRedeemInt64(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case float32:
		return int64(typed)
	case int:
		return int64(typed)
	case int64:
		return typed
	case json.Number:
		parsed, _ := typed.Int64()
		return parsed
	case string:
		var parsed int64
		_, _ = fmt.Sscan(strings.TrimSpace(typed), &parsed)
		return parsed
	default:
		return 0
	}
}

func collectNewAPIRedeemGroups(value any, groups map[string]struct{}) {
	switch typed := value.(type) {
	case string:
		if group := strings.TrimSpace(typed); group != "" {
			groups[group] = struct{}{}
		}
	case []any:
		for _, item := range typed {
			collectNewAPIRedeemGroups(item, groups)
		}
	case map[string]any:
		for _, key := range []string{"groups", "items", "available_groups", "group_ratio"} {
			if nested, exists := typed[key]; exists {
				collectNewAPIRedeemGroups(nested, groups)
				return
			}
		}
		if group := firstNewAPIRedeemText(newAPIRedeemString(typed["name"]), newAPIRedeemString(typed["group"]), newAPIRedeemString(typed["value"])); group != "" {
			groups[group] = struct{}{}
			return
		}
		for key := range typed {
			if clean := strings.TrimSpace(key); clean != "" {
				groups[clean] = struct{}{}
			}
		}
	}
}
