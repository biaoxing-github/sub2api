package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

const (
	newAPICheckinDefaultPath       = "/api/user/checkin"
	newAPICheckinConfigFile        = "config"
	newAPICheckinLatestFile        = "latest-run"
	newAPICheckinBalancesFile      = "balances"
	newAPICheckinHistoryFile       = "history"
	newAPICheckinMonthlyFile       = "monthly"
	newAPICheckinDefaultQuotaUnit  = int64(500000)
	newAPICheckinDefaultSymbol     = "$"
	newAPICheckinDateLayout        = "2006-01-02"
	newAPICheckinDateTimeLayout    = "2006-01-02 15:04:05"
	newAPICheckinMonthlySourceSync = "manual-refresh"
	newAPICheckinAPIKeyConcurrency = 6
	newAPICheckinProviderNewAPI    = "newapi"
	newAPICheckinProviderSub2API   = "sub2api"
)

// NewAPICheckinOptions 描述 NewApi 签到迁移功能的运行选项。
type NewAPICheckinOptions struct {
	// Repository 是 NewApi 签到配置、缓存、历史和月度记录的 SQL 存储端口。
	Repository NewAPICheckinRepository
	// AccountRepository 用于按站点地址关联主平台的 OpenAI API Key 账号。
	AccountRepository NewAPICheckinAccountRepository
	// HTTPClient 是访问各 NewApi 站点的客户端；为空时使用带超时的默认客户端。
	HTTPClient *http.Client
	// Now 返回当前时间，测试中用于固定时间。
	Now func() time.Time
}

// NewAPICheckinService 承载 NewApi 多站点签到 dashboard 的业务逻辑。
type NewAPICheckinService struct {
	repo        NewAPICheckinRepository
	accountRepo NewAPICheckinAccountRepository
	httpClient  *http.Client
	now         func() time.Time

	mu               sync.Mutex
	checkinJobState  NewAPICheckinJobState
	monthlySyncState NewAPICheckinMonthlySyncState
}

// NewAPICheckinAccountRepository 是签到工具关联主平台账号所需的最小持久化端口。
type NewAPICheckinAccountRepository interface {
	ListByPlatform(ctx context.Context, platform string) ([]Account, error)
	GetByID(ctx context.Context, id int64) (*Account, error)
	Update(ctx context.Context, account *Account) error
}

// NewAPICheckinRepository 定义 NewApi 签到功能的 SQL 持久化端口。
type NewAPICheckinRepository interface {
	// LoadConfig 读取平台和账号配置。
	LoadConfig(ctx context.Context) (NewAPICheckinConfig, error)
	// SaveConfig 保存平台和账号配置。
	SaveConfig(ctx context.Context, cfg NewAPICheckinConfig) error
	// LoadLatestReport 读取最近一次签到执行报告。
	LoadLatestReport(ctx context.Context) (NewAPICheckinReport, error)
	// SaveLatestReport 保存最近一次签到执行报告。
	SaveLatestReport(ctx context.Context, report NewAPICheckinReport) error
	// LoadBalanceCache 读取实时余额缓存。
	LoadBalanceCache(ctx context.Context) (NewAPICheckinBalancePayload, error)
	// SaveBalanceCache 保存实时余额缓存。
	SaveBalanceCache(ctx context.Context, cache NewAPICheckinBalancePayload) error
	// LoadHistory 读取签到历史。
	LoadHistory(ctx context.Context) (NewAPICheckinHistoryPayload, error)
	// SaveHistory 保存签到历史。
	SaveHistory(ctx context.Context, payload NewAPICheckinHistoryPayload) error
	// LoadMonthlyRecords 读取月度签到记录。
	LoadMonthlyRecords(ctx context.Context) ([]NewAPICheckinMonthlyRecord, error)
	// SaveMonthlyRecords 保存月度签到记录。
	SaveMonthlyRecords(ctx context.Context, records []NewAPICheckinMonthlyRecord) error
	// LoadAPIKeyCache 读取 API Key 与分组的账号级数据库缓存。
	LoadAPIKeyCache(ctx context.Context) ([]NewAPICheckinAPIKeyCacheEntry, error)
	// SaveAPIKeyCache 保存本次成功读取的 API Key 与分组账号级缓存。
	SaveAPIKeyCache(ctx context.Context, entries []NewAPICheckinAPIKeyCacheEntry) error
	// StorageLabel 返回兼容页面展示的存储位置说明。
	StorageLabel() string
}

// NewAPICheckinConfig 是运行时保存的 NewApi 多站点配置。
type NewAPICheckinConfig struct {
	// DefaultCheckinPath 是站点未显式配置时使用的签到路径。
	DefaultCheckinPath string `json:"default_checkin_path"`
	// DelayBetweenCheckinsSec 是全量签到账号之间的等待秒数。
	DelayBetweenCheckinsSec int `json:"delay_between_checkins_sec"`
	// RouteSwitchWaitSec 是保留给线路切换后的等待秒数。
	RouteSwitchWaitSec int `json:"route_switch_wait_sec"`
	// NotifyFeishu 表示是否沿用原脚本的飞书通知开关。
	NotifyFeishu bool `json:"notify_feishu"`
	// Sites 是参与 dashboard 展示和刷新查询的平台列表。
	Sites []NewAPICheckinSite `json:"sites"`
}

// NewAPICheckinSite 是单个 NewApi 平台的配置。
type NewAPICheckinSite struct {
	// Name 是 dashboard 中展示和过滤使用的平台名。
	Name string `json:"name"`
	// Provider 是站点协议类型，newapi 支持签到，sub2api 仅支持只读用量查询。
	Provider string `json:"provider"`
	// Enabled 控制自动/手动签到，禁用后仍保留余额和月度查询能力。
	Enabled bool `json:"enabled"`
	// DisabledReason 是禁用签到时展示给用户的原因。
	DisabledReason string `json:"disabled_reason,omitempty"`
	// BackgroundCheckinEnabled 保留原配置里的后台签到开关。
	BackgroundCheckinEnabled bool `json:"background_checkin_enabled,omitempty"`
	// BaseURL 是 NewApi 站点根地址。
	BaseURL string `json:"base_url"`
	// CheckinPath 是站点自定义签到路径，空值使用默认路径。
	CheckinPath string `json:"checkin_path,omitempty"`
	// SiteStatus 是可选的额度折算缓存或默认值。
	SiteStatus NewAPICheckinSiteStatus `json:"site_status,omitempty"`
	// Accounts 是该平台下的访问 key 账号列表。
	Accounts []NewAPICheckinAccount `json:"accounts"`
}

// NewAPICheckinAccount 是单个 NewApi 用户 access key 配置。
type NewAPICheckinAccount struct {
	// Name 是本地配置中的账号名。
	Name string `json:"name"`
	// Username 是上游 /api/user/self 返回的用户名缓存。
	Username string `json:"username,omitempty"`
	// DisplayName 是上游 /api/user/self 返回的展示名缓存。
	DisplayName string `json:"display_name,omitempty"`
	// UserID 是 New-Api-User 头使用的上游用户 ID。
	UserID string `json:"user_id"`
	// AccessKey 是 Authorization Bearer 使用的明文 access key，持久化在 NewApi 签到 SQL 配置表。
	AccessKey string `json:"access_key"`
	// LoginUsername 是上游用户登录账号，用于临时换取用户 JWT。
	LoginUsername string `json:"login_username,omitempty"`
	// LoginPassword 是上游用户登录密码，仅持久化使用，任何页面摘要都不得返回。
	LoginPassword string `json:"login_password,omitempty"`
	// IPProfile 是原脚本用于线路轮换的本地标记。
	IPProfile string `json:"ip_profile,omitempty"`
	// Enabled 控制账号是否参与查询和签到；缺省为 true。
	Enabled bool `json:"enabled"`
	// DisabledReason 是账号禁用时的本地原因。
	DisabledReason string `json:"disabled_reason,omitempty"`
}

// NewAPICheckinSiteStatus 描述上游站点额度折算规则。
type NewAPICheckinSiteStatus struct {
	// OK 表示站点状态接口是否成功。
	OK bool `json:"ok"`
	// Message 是站点状态接口的提示文本。
	Message string `json:"message"`
	// QuotaDisplayType 是上游返回的额度展示类型，例如 USD。
	QuotaDisplayType string `json:"quota_display_type"`
	// QuotaPerUnit 是原始 quota 折算成展示金额的分母。
	QuotaPerUnit int64 `json:"quota_per_unit"`
	// CustomCurrencySymbol 是自定义货币符号。
	CustomCurrencySymbol string `json:"custom_currency_symbol"`
}

// NewAPICheckinConfigSummary 是 /api/config 页面契约。
type NewAPICheckinConfigSummary struct {
	// GeneratedAt 是摘要生成时间。
	GeneratedAt string `json:"generated_at"`
	// AllSiteCount 是配置中的全部站点数。
	AllSiteCount int `json:"all_site_count"`
	// EnabledSiteCount 是启用签到的站点数。
	EnabledSiteCount int `json:"enabled_site_count"`
	// AllAccountCount 是配置中的全部账号数。
	AllAccountCount int `json:"all_account_count"`
	// EnabledAccountCount 是启用站点下可用账号数。
	EnabledAccountCount int `json:"enabled_account_count"`
	// Sites 是脱敏后的站点和账号摘要。
	Sites []NewAPICheckinConfigSiteSummary `json:"sites"`
}

// NewAPICheckinConfigSiteSummary 是页面平台目录中的站点摘要。
type NewAPICheckinConfigSiteSummary struct {
	// Name 是站点名。
	Name string `json:"name"`
	// Provider 是页面区分可签到站点和只读数据源的协议类型。
	Provider string `json:"provider"`
	// Enabled 表示站点是否参与签到。
	Enabled bool `json:"enabled"`
	// DisabledReason 是站点禁用原因。
	DisabledReason string `json:"disabled_reason"`
	// BaseURL 是站点根地址。
	BaseURL string `json:"base_url"`
	// AccountCount 是站点下账号数。
	AccountCount int `json:"account_count"`
	// Accounts 是脱敏账号摘要。
	Accounts []NewAPICheckinConfigAccountSummary `json:"accounts"`
}

// NewAPICheckinConfigAccountSummary 是页面平台目录中的账号摘要。
type NewAPICheckinConfigAccountSummary struct {
	// Name 是账号配置名。
	Name string `json:"name"`
	// Username 是上游用户名缓存。
	Username string `json:"username"`
	// DisplayName 是上游展示名缓存。
	DisplayName string `json:"display_name"`
	// Label 是页面优先展示标签。
	Label string `json:"label"`
	// UserID 是 NewApi 用户 ID。
	UserID string `json:"user_id"`
	// IPProfile 是线路标记。
	IPProfile string `json:"ip_profile"`
	// AccessKey 是受保护管理页面按需展示的完整配置访问凭据。
	AccessKey string `json:"access_key,omitempty"`
	// AccessKeyMasked 是配置访问凭据的脱敏展示值。
	AccessKeyMasked string `json:"access_key_masked,omitempty"`
	// LoginUsername 是上游登录账号，允许管理员在平台目录中核对。
	LoginUsername string `json:"login_username,omitempty"`
	// HasLoginPassword 仅说明密码是否已保存，不返回密码内容。
	HasLoginPassword bool `json:"has_login_password"`
}

// NewAPICheckinGroupOption 是上游可用分组的稳定 ID、名称和倍率。
type NewAPICheckinGroupOption struct {
	// ID 是 sub2api 分组 ID；NewAPI 只有名称时为 0。
	ID int64 `json:"id"`
	// Name 是页面展示的分组名称。
	Name string `json:"name"`
	// RateMultiplier 是上游返回的分组倍率；站点未提供时保持为空。
	RateMultiplier *float64 `json:"rate_multiplier,omitempty"`
}

// NewAPICheckinAPIKeySummary 是允许返回管理页面的上游 API Key 脱敏摘要。
type NewAPICheckinAPIKeySummary struct {
	// ID 是上游 token 的稳定标识，用于按需显示和更新分组。
	ID int64 `json:"id"`
	// Name 是上游 API Key 名称。
	Name string `json:"name"`
	// MaskedKey 固定使用 sk-前2位***后4位格式，不包含完整凭据。
	MaskedKey string `json:"masked_key"`
	// Group 是该 API Key 当前使用的上游分组。
	Group string `json:"group"`
	// GroupID 是 sub2api API Key 当前绑定的分组 ID。
	GroupID int64 `json:"group_id,omitempty"`
	// ReferencedAccounts 是当前已经引用该 Key 的主平台账号。
	ReferencedAccounts []NewAPICheckinAccountReference `json:"referenced_accounts,omitempty"`
	// TargetAccounts 是同站点下可执行追加或替换的主平台账号。
	TargetAccounts []NewAPICheckinAccountReference `json:"target_accounts,omitempty"`
	// MatchKey 仅在服务端保存完整或上游脱敏 Key，用于数据库引用匹配，不写入接口 JSON。
	MatchKey string `json:"-"`
}

// NewAPICheckinAPIKeyCacheEntry 是一个签到账号的 API Key 与分组数据库缓存。
type NewAPICheckinAPIKeyCacheEntry struct {
	// Site 是签到站点名称。
	Site string `json:"site"`
	// UserID 是签到账号在站点内的稳定标识。
	UserID string `json:"user_id"`
	// RefreshedAt 是最近一次成功访问上游的时间。
	RefreshedAt string `json:"refreshed_at"`
	// Summary 是不包含主平台引用派生信息的上游摘要。
	Summary NewAPICheckinAccountAPIKeySummary `json:"summary"`
}

// NewAPICheckinAccountReference 描述与签到站点 URL 匹配的主平台账号。
type NewAPICheckinAccountReference struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Referenced bool   `json:"referenced"`
}

// NewAPICheckinLinkAPIKeyResult 是签到 Key 写入主平台账号后的结果。
type NewAPICheckinLinkAPIKeyResult struct {
	Site              string `json:"site"`
	UserID            string `json:"user_id"`
	APIKeyID          int64  `json:"api_key_id"`
	TargetAccountID   int64  `json:"target_account_id"`
	TargetAccountName string `json:"target_account_name"`
	Operation         string `json:"operation"`
	KeyCount          int    `json:"key_count"`
	Message           string `json:"message"`
}

// NewAPICheckinAccountAPIKeySummary 是单个签到账号的 API Key 查询结果。
type NewAPICheckinAccountAPIKeySummary struct {
	// Site 是账号所属站点名。
	Site string `json:"site"`
	// Provider 是站点协议类型。
	Provider string `json:"provider"`
	// UserID 是 NewApi 用户 ID。
	UserID string `json:"user_id"`
	// Status 是 ready、missing、unsupported 或 error。
	Status string `json:"status"`
	// Message 是读取失败时的简短原因。
	Message string `json:"message,omitempty"`
	// GroupStatus 是 ready、partial、unsupported 或 error。
	GroupStatus string `json:"group_status"`
	// GroupMessage 说明完整分组读取能力或失败原因。
	GroupMessage string `json:"group_message,omitempty"`
	// AvailableGroups 是当前账号能够读取到的分组名称。
	AvailableGroups []string `json:"available_groups"`
	// AvailableGroupOptions 是包含稳定 ID 和可选倍率的分组选项。
	AvailableGroupOptions []NewAPICheckinGroupOption `json:"available_group_options,omitempty"`
	// APIKeys 是该账号已生成的脱敏 API Key 列表。
	APIKeys []NewAPICheckinAPIKeySummary `json:"api_keys"`
}

// NewAPICheckinAPIKeyRevealResult 是管理员按需查看完整上游 API Key 的结果。
type NewAPICheckinAPIKeyRevealResult struct {
	// Site 是账号所属站点名。
	Site string `json:"site"`
	// UserID 是账号在站点中的用户 ID。
	UserID string `json:"user_id"`
	// APIKeyID 是上游 token ID。
	APIKeyID int64 `json:"api_key_id"`
	// Name 是上游 token 名称。
	Name string `json:"name"`
	// Key 是按需返回的完整 sk- API Key。
	Key string `json:"key"`
	// MaskedKey 是默认展示的脱敏值。
	MaskedKey string `json:"masked_key"`
	// Group 是 token 当前分组。
	Group string `json:"group"`
}

// NewAPICheckinAPIKeyGroupResult 是更新上游 token 分组后的结果。
type NewAPICheckinAPIKeyGroupResult struct {
	// Site 是账号所属站点名。
	Site string `json:"site"`
	// UserID 是账号在站点中的用户 ID。
	UserID string `json:"user_id"`
	// APIKeyID 是上游 token ID。
	APIKeyID int64 `json:"api_key_id"`
	// Name 是上游 token 名称。
	Name string `json:"name"`
	// Group 是更新后的分组。
	Group string `json:"group"`
	// GroupID 是更新后的 sub2api 分组 ID。
	GroupID int64 `json:"group_id,omitempty"`
	// Message 是上游成功消息。
	Message string `json:"message"`
}

// NewAPICheckinLoginCredentialResult 是登录凭据保存或测试后的非敏感结果。
type NewAPICheckinLoginCredentialResult struct {
	// Site 是账号所属站点名。
	Site string `json:"site"`
	// UserID 是签到工具中的稳定账号标识。
	UserID string `json:"user_id"`
	// LoginUsername 是本次验证使用的登录账号。
	LoginUsername string `json:"login_username"`
	// HasLoginPassword 表示账号已经保存登录密码。
	HasLoginPassword bool `json:"has_login_password"`
	// LoginOK 表示上游登录是否成功。
	LoginOK bool `json:"login_ok"`
	// APIKeyCount 是登录后可见的 API Key 数量。
	APIKeyCount int `json:"api_key_count"`
	// GroupCount 是登录后可见的分组数量。
	GroupCount int `json:"group_count"`
	// Message 是非敏感结果说明。
	Message string `json:"message"`
	// Config 是保存成功后的最新脱敏配置摘要。
	Config *NewAPICheckinConfigSummary `json:"config,omitempty"`
}

// NewAPICheckinAPIKeyPayload 是签到页面 API Key 列的只读载荷。
type NewAPICheckinAPIKeyPayload struct {
	// GeneratedAt 是本次读取完成时间。
	GeneratedAt string `json:"generated_at"`
	// AccountCount 是参与读取的启用账号数。
	AccountCount int `json:"account_count"`
	// AvailableCount 是至少有一个 API Key 的账号数。
	AvailableCount int `json:"available_count"`
	// MissingCount 是尚未生成 API Key 的账号数。
	MissingCount int `json:"missing_count"`
	// ErrorCount 是上游读取失败的账号数。
	ErrorCount int `json:"error_count"`
	// UnsupportedCount 是仅有网关 Key、缺少账号登录态的账号数。
	UnsupportedCount int `json:"unsupported_count"`
	// CachedCount 是本次直接从数据库缓存返回的账号数。
	CachedCount int `json:"cached_count"`
	// RefreshedCount 是本次实时访问上游并更新缓存的账号数。
	RefreshedCount int `json:"refreshed_count"`
	// Accounts 按站点配置顺序返回每个账号的结果。
	Accounts []NewAPICheckinAccountAPIKeySummary `json:"accounts"`
}

// NewAPICheckinSiteEnabledResult 是站点启停接口的返回值。
type NewAPICheckinSiteEnabledResult struct {
	// GeneratedAt 是修改时间。
	GeneratedAt string `json:"generated_at"`
	// ConfigPath 是运行时配置路径。
	ConfigPath string `json:"config_path"`
	// Site 是被修改的站点名。
	Site string `json:"site"`
	// Enabled 是修改后的启用状态。
	Enabled bool `json:"enabled"`
	// DisabledReason 是修改后的禁用原因。
	DisabledReason string `json:"disabled_reason"`
	// Config 是修改后的脱敏配置摘要。
	Config NewAPICheckinConfigSummary `json:"config"`
	// Balances 是修改后的余额缓存视图。
	Balances NewAPICheckinBalancePayload `json:"balances"`
}

// NewAPICheckinAddConfigResult 是配置写入接口的返回值。
type NewAPICheckinAddConfigResult struct {
	// GeneratedAt 是写入时间。
	GeneratedAt string `json:"generated_at"`
	// ConfigPath 是运行时配置路径。
	ConfigPath string `json:"config_path"`
	// SiteCount 是本次提交的平台数。
	SiteCount int `json:"site_count"`
	// AddedAccounts 是新增账号数。
	AddedAccounts int `json:"added_accounts"`
	// UpdatedAccounts 是更新账号数。
	UpdatedAccounts int `json:"updated_accounts"`
	// Sites 是每个平台的写入结果。
	Sites []map[string]any `json:"sites"`
	// Config 是写入后的脱敏配置摘要。
	Config NewAPICheckinConfigSummary `json:"config"`
	// Balances 是写入后的余额缓存视图。
	Balances NewAPICheckinBalancePayload `json:"balances"`
}

// NewAPICheckinDeleteResult 是删除站点或账号后的页面返回值。
type NewAPICheckinDeleteResult struct {
	// GeneratedAt 是删除时间。
	GeneratedAt string `json:"generated_at"`
	// Site 是删除目标站点。
	Site string `json:"site"`
	// UserID 是删除目标账号，删除站点时为空。
	UserID string `json:"user_id,omitempty"`
	// RemovedAccounts 是从配置中删除的账号数。
	RemovedAccounts int `json:"removed_accounts"`
	// RemovedCacheAccounts 是从余额缓存中删除的账号数。
	RemovedCacheAccounts int `json:"removed_cache_accounts"`
	// RemovedHistoryEntries 是从历史记录中删除的条数。
	RemovedHistoryEntries int `json:"removed_history_entries"`
	// RemovedMonthlyEntries 是从月度记录中删除的条数。
	RemovedMonthlyEntries int `json:"removed_monthly_entries"`
	// Config 是删除后的脱敏配置摘要。
	Config NewAPICheckinConfigSummary `json:"config"`
	// Balances 是删除后的余额缓存视图。
	Balances NewAPICheckinBalancePayload `json:"balances"`
	// History 是删除后的签到历史视图。
	History NewAPICheckinHistoryPayload `json:"history"`
	// LastRun 是删除后的最近执行视图。
	LastRun NewAPICheckinReport `json:"last_run"`
}

// NewAPICheckinReport 是最近一次签到执行报告。
type NewAPICheckinReport struct {
	// StartedAt 是执行开始时间。
	StartedAt string `json:"started_at"`
	// EndedAt 是执行结束时间。
	EndedAt string `json:"ended_at"`
	// Source 是报告来源。
	Source string `json:"source,omitempty"`
	// SiteCount 是本次涉及站点数。
	SiteCount int `json:"site_count"`
	// TaskCount 是本次账号任务数。
	TaskCount int `json:"task_count"`
	// SuccessCount 是签到成功账号数。
	SuccessCount int `json:"success_count"`
	// AlreadyDoneCount 是今日已签到账号数。
	AlreadyDoneCount int `json:"already_done_count"`
	// FailedCount 是失败账号数。
	FailedCount int `json:"failed_count"`
	// QuotaAwardedTotal 是本次奖励原始额度总和。
	QuotaAwardedTotal int64 `json:"quota_awarded_total"`
	// QuotaAwardedDisplay 是本次奖励展示金额总和。
	QuotaAwardedDisplay string `json:"quota_awarded_display"`
	// QuotaDisplaySymbol 是展示金额符号。
	QuotaDisplaySymbol string `json:"quota_display_symbol"`
	// SiteSummaries 是按站点聚合的执行结果。
	SiteSummaries []NewAPICheckinSiteRunSummary `json:"site_summaries"`
	// AccountResults 是每个账号的执行明细。
	AccountResults []NewAPICheckinAccountResult `json:"account_results"`
	// SummaryText 是 Markdown 摘要文本。
	SummaryText string `json:"summary_text"`
}

// NewAPICheckinSiteRunSummary 是最近一次执行的站点聚合行。
type NewAPICheckinSiteRunSummary struct {
	// Site 是站点名。
	Site string `json:"site"`
	// TaskCount 是站点任务数。
	TaskCount int `json:"task_count"`
	// SuccessCount 是成功数。
	SuccessCount int `json:"success_count"`
	// AlreadyDoneCount 是已签到数。
	AlreadyDoneCount int `json:"already_done_count"`
	// FailedCount 是失败数。
	FailedCount int `json:"failed_count"`
	// QuotaAwardedTotal 是原始奖励额度。
	QuotaAwardedTotal int64 `json:"quota_awarded_total"`
	// QuotaDisplayTotal 是展示奖励额度。
	QuotaDisplayTotal string `json:"quota_display_total"`
	// DisplaySymbol 是展示符号。
	DisplaySymbol string `json:"display_symbol"`
}

// NewAPICheckinAccountResult 是签到执行账号明细。
type NewAPICheckinAccountResult struct {
	// Site 是站点名。
	Site string `json:"site"`
	// Account 是本地账号名。
	Account string `json:"account"`
	// UserID 是 NewApi 用户 ID。
	UserID string `json:"user_id"`
	// IPProfile 是线路标记。
	IPProfile string `json:"ip_profile"`
	// OK 表示签到查询是否可视作完成。
	OK bool `json:"ok"`
	// Success 表示本次直签是否成功。
	Success bool `json:"success"`
	// Message 是上游或本地状态消息。
	Message string `json:"message"`
	// CheckinDate 是签到日期。
	CheckinDate string `json:"checkin_date"`
	// QuotaAwarded 是本次奖励原始额度。
	QuotaAwarded *int64 `json:"quota_awarded"`
	// QuotaAwardedDisplay 是本次奖励展示额度。
	QuotaAwardedDisplay string `json:"quota_awarded_display"`
	// QuotaAwardedDisplayValue 是本次奖励展示数值。
	QuotaAwardedDisplayValue float64 `json:"quota_awarded_display_value"`
	// RemainingQuota 是当前剩余额度。
	RemainingQuota *int64 `json:"remaining_quota,omitempty"`
	// RemainingQuotaDisplay 是当前剩余额度展示值。
	RemainingQuotaDisplay string `json:"remaining_quota_display,omitempty"`
	// UsedQuota 是当前已用额度。
	UsedQuota *int64 `json:"used_quota,omitempty"`
	// UsedQuotaDisplay 是当前已用额度展示值。
	UsedQuotaDisplay string `json:"used_quota_display,omitempty"`
	// Username 是上游用户名。
	Username string `json:"username,omitempty"`
	// DisplayName 是上游展示名。
	DisplayName string `json:"display_name,omitempty"`
	// Status 是页面状态列文本。
	Status string `json:"status"`
	// CheckinStatus 是签到状态文本。
	CheckinStatus string `json:"checkin_status"`
	// CheckinStatusTone 是签到状态色调。
	CheckinStatusTone string `json:"checkin_status_tone"`
}

// NewAPICheckinBalancePayload 是实时余额页的数据契约。
type NewAPICheckinBalancePayload struct {
	// GeneratedAt 是缓存生成时间。
	GeneratedAt string `json:"generated_at"`
	// Source 是缓存来源。
	Source string `json:"source"`
	// SiteCount 是站点数。
	SiteCount int `json:"site_count"`
	// AccountCount 是账号数。
	AccountCount int `json:"account_count"`
	// Overall 是跨站点汇总。
	Overall NewAPICheckinBalanceOverall `json:"overall"`
	// SiteSummaries 是站点余额汇总。
	SiteSummaries []NewAPICheckinBalanceSiteSummary `json:"site_summaries"`
	// Accounts 是账号余额明细。
	Accounts []NewAPICheckinBalanceAccount `json:"accounts"`
	// SiteStatuses 是站点额度折算状态缓存。
	SiteStatuses map[string]NewAPICheckinSiteStatus `json:"site_statuses"`
}

// NewAPICheckinBalanceOverall 是实时余额总览。
type NewAPICheckinBalanceOverall struct {
	// QuotaTotal 是全部账号剩余额度总和。
	QuotaTotal int64 `json:"quota_total"`
	// UsedQuotaTotal 是全部账号已用额度总和。
	UsedQuotaTotal int64 `json:"used_quota_total"`
	// EnabledQuotaTotal 是启用站点账号剩余额度总和。
	EnabledQuotaTotal int64 `json:"enabled_quota_total"`
	// EnabledUsedQuotaTotal 是启用站点账号已用额度总和。
	EnabledUsedQuotaTotal int64 `json:"enabled_used_quota_total"`
	// DisplayTotals 是按货币符号折算后的汇总。
	DisplayTotals []map[string]any `json:"display_totals"`
}

// NewAPICheckinBalanceSiteSummary 是实时余额站点汇总。
type NewAPICheckinBalanceSiteSummary struct {
	// Site 是站点名。
	Site string `json:"site"`
	// Enabled 表示站点是否参与签到。
	Enabled bool `json:"enabled"`
	// AccountCount 是站点账号数。
	AccountCount int `json:"account_count"`
	// QuotaTotal 是剩余额度总和。
	QuotaTotal int64 `json:"quota_total"`
	// QuotaDisplayTotal 是剩余额度展示值。
	QuotaDisplayTotal string `json:"quota_display_total"`
	// UsedQuotaTotal 是已用额度总和。
	UsedQuotaTotal int64 `json:"used_quota_total"`
	// UsedDisplayTotal 是已用额度展示值。
	UsedDisplayTotal string `json:"used_display_total"`
}

// NewAPICheckinBalanceAccount 是实时余额账号行。
type NewAPICheckinBalanceAccount struct {
	// Site 是站点名。
	Site string `json:"site"`
	// Provider 是账号所属站点的协议类型。
	Provider string `json:"provider"`
	// ProviderData 保存协议特有的只读套餐、用量和模型信息。
	ProviderData map[string]any `json:"provider_data,omitempty"`
	// Enabled 表示所在站点是否参与签到。
	Enabled bool `json:"enabled"`
	// Account 是本地账号名。
	Account string `json:"account"`
	// Username 是上游用户名。
	Username string `json:"username"`
	// DisplayName 是上游展示名。
	DisplayName string `json:"display_name"`
	// Label 是页面展示标签。
	Label string `json:"label"`
	// UserID 是 NewApi 用户 ID。
	UserID string `json:"user_id"`
	// IPProfile 是线路标记。
	IPProfile string `json:"ip_profile"`
	// Status 是页面状态文本。
	Status string `json:"status"`
	// Message 是状态消息。
	Message string `json:"message"`
	// CheckinOK 表示签到状态查询是否可视作完成。
	CheckinOK any `json:"checkin_ok,omitempty"`
	// CheckinSuccess 表示直签是否成功。
	CheckinSuccess any `json:"checkin_success,omitempty"`
	// CheckedInToday 表示今日是否已签到。
	CheckedInToday any `json:"checked_in_today,omitempty"`
	// CheckinStatus 是签到状态文本。
	CheckinStatus string `json:"checkin_status,omitempty"`
	// CheckinStatusTone 是签到状态色调。
	CheckinStatusTone string `json:"checkin_status_tone,omitempty"`
	// CheckinMessage 是签到消息。
	CheckinMessage string `json:"checkin_message,omitempty"`
	// CheckinDate 是签到日期。
	CheckinDate string `json:"checkin_date,omitempty"`
	// Quota 是剩余额度。
	Quota *int64 `json:"quota"`
	// QuotaDisplay 是剩余额度展示值。
	QuotaDisplay string `json:"quota_display"`
	// UsedQuota 是已用额度。
	UsedQuota *int64 `json:"used_quota"`
	// UsedQuotaDisplay 是已用额度展示值。
	UsedQuotaDisplay string `json:"used_quota_display"`
	// QuotaAwarded 是签到奖励额度。
	QuotaAwarded *int64 `json:"quota_awarded,omitempty"`
	// QuotaAwardedDisplay 是签到奖励展示值。
	QuotaAwardedDisplay string `json:"quota_awarded_display,omitempty"`
	// LastRefreshedAt 是最后刷新时间。
	LastRefreshedAt string `json:"last_refreshed_at"`
}

// NewAPICheckinHistoryPayload 是签到记录和趋势页的数据契约。
type NewAPICheckinHistoryPayload struct {
	// GeneratedAt 是生成时间。
	GeneratedAt string `json:"generated_at"`
	// Entries 是历史明细。
	Entries []NewAPICheckinHistoryEntry `json:"entries"`
	// DailySummaries 是全局每日趋势。
	DailySummaries []map[string]any `json:"daily_summaries"`
	// SiteSummaries 是站点每日趋势。
	SiteSummaries []map[string]any `json:"site_summaries"`
	// AccountSummaries 是账号每日趋势。
	AccountSummaries []map[string]any `json:"account_summaries"`
}

// NewAPICheckinHistoryEntry 是签到历史明细行。
type NewAPICheckinHistoryEntry struct {
	// Date 是记录日期。
	Date string `json:"date"`
	// RecordedAt 是写入时间。
	RecordedAt string `json:"recorded_at"`
	// Source 是记录来源。
	Source string `json:"source"`
	// Site 是站点名。
	Site string `json:"site"`
	// Account 是账号展示名。
	Account string `json:"account"`
	// UserID 是 NewApi 用户 ID。
	UserID string `json:"user_id"`
	// IPProfile 是线路标记。
	IPProfile string `json:"ip_profile"`
	// CheckinStatus 是签到状态文本。
	CheckinStatus string `json:"checkin_status"`
	// CheckinStatusTone 是签到状态色调。
	CheckinStatusTone string `json:"checkin_status_tone"`
	// CheckinMessage 是签到消息。
	CheckinMessage string `json:"checkin_message"`
	// CheckinSuccess 表示直签是否成功。
	CheckinSuccess bool `json:"checkin_success"`
	// CheckedInToday 表示今日是否已签到。
	CheckedInToday bool `json:"checked_in_today"`
	// QuotaAwarded 是奖励原始额度。
	QuotaAwarded *int64 `json:"quota_awarded"`
	// QuotaAwardedDisplay 是奖励展示值。
	QuotaAwardedDisplay string `json:"quota_awarded_display"`
	// QuotaAwardedDisplayValue 是奖励展示数值。
	QuotaAwardedDisplayValue float64 `json:"quota_awarded_display_value"`
	// Balance 是剩余额度。
	Balance *int64 `json:"balance"`
	// BalanceDisplay 是剩余额度展示值。
	BalanceDisplay string `json:"balance_display"`
	// BalanceDisplayValue 是剩余额度展示数值。
	BalanceDisplayValue float64 `json:"balance_display_value"`
	// UsedQuota 是已用额度。
	UsedQuota *int64 `json:"used_quota"`
	// UsedDisplay 是已用额度展示值。
	UsedDisplay string `json:"used_display"`
	// UsedDisplayValue 是已用额度展示数值。
	UsedDisplayValue float64 `json:"used_display_value"`
}

// NewAPICheckinMonthlyPayload 是月度历史页的数据契约。
type NewAPICheckinMonthlyPayload struct {
	// GeneratedAt 是生成时间。
	GeneratedAt string `json:"generated_at"`
	// DBPath 是兼容原页面的 SQL 存储位置说明。
	DBPath string `json:"db_path"`
	// Initialized 表示本地月度缓存是否初始化。
	Initialized bool `json:"initialized"`
	// SelectedMonth 是当前选择月份。
	SelectedMonth string `json:"selected_month"`
	// AvailableMonths 是已有月度记录月份。
	AvailableMonths []string `json:"available_months"`
	// SyncState 是月度同步任务状态。
	SyncState NewAPICheckinMonthlySyncState `json:"sync_state"`
	// Records 是月度签到明细。
	Records []NewAPICheckinMonthlyRecord `json:"records"`
	// SiteSummaries 是站点月度汇总。
	SiteSummaries []map[string]any `json:"site_summaries"`
	// AccountSummaries 是账号月度汇总。
	AccountSummaries []map[string]any `json:"account_summaries"`
	// SiteDailySummaries 是站点每日月度趋势。
	SiteDailySummaries []map[string]any `json:"site_daily_summaries"`
	// AccountDailySummaries 是账号每日月度趋势。
	AccountDailySummaries []map[string]any `json:"account_daily_summaries"`
}

// NewAPICheckinMonthlyRecord 是单条月度签到记录。
type NewAPICheckinMonthlyRecord struct {
	// Site 是站点名。
	Site string `json:"site"`
	// UserID 是 NewApi 用户 ID。
	UserID string `json:"user_id"`
	// AccountName 是本地账号名。
	AccountName string `json:"account_name"`
	// Username 是上游用户名。
	Username string `json:"username"`
	// DisplayName 是上游展示名。
	DisplayName string `json:"display_name"`
	// IPProfile 是线路标记。
	IPProfile string `json:"ip_profile"`
	// Month 是 YYYY-MM 月份。
	Month string `json:"month"`
	// CheckinDate 是签到日期。
	CheckinDate string `json:"checkin_date"`
	// QuotaAwarded 是奖励原始额度。
	QuotaAwarded *int64 `json:"quota_awarded"`
	// QuotaAwardedDisplay 是奖励展示值。
	QuotaAwardedDisplay string `json:"quota_awarded_display"`
	// QuotaAwardedDisplayValue 是奖励展示数值。
	QuotaAwardedDisplayValue float64 `json:"quota_awarded_display_value"`
	// FetchedAt 是拉取时间。
	FetchedAt string `json:"fetched_at"`
	// Source 是记录来源。
	Source string `json:"source"`
}

// NewAPICheckinMonthlySyncState 描述月度同步后台任务状态。
type NewAPICheckinMonthlySyncState struct {
	// Running 表示任务是否运行中。
	Running bool `json:"running"`
	// Mode 是任务来源。
	Mode string `json:"mode"`
	// Scope 是同步范围。
	Scope string `json:"scope"`
	// Month 是同步月份。
	Month string `json:"month"`
	// StartedAt 是开始时间。
	StartedAt string `json:"started_at"`
	// EndedAt 是结束时间。
	EndedAt string `json:"ended_at"`
	// Message 是任务提示。
	Message string `json:"message"`
	// Processed 是已处理账号数。
	Processed int `json:"processed"`
	// Total 是总账号数。
	Total int `json:"total"`
	// LastSite 是最近处理站点。
	LastSite string `json:"last_site"`
	// LastUserID 是最近处理账号。
	LastUserID string `json:"last_user_id"`
	// Error 是错误文本。
	Error string `json:"error"`
}

// NewAPICheckinJobState 描述全量签到后台任务状态。
type NewAPICheckinJobState struct {
	// Running 表示任务是否运行中。
	Running bool `json:"running"`
	// StartedAt 是开始时间。
	StartedAt string `json:"started_at"`
	// EndedAt 是结束时间。
	EndedAt string `json:"ended_at"`
	// Message 是任务提示。
	Message string `json:"message"`
	// ExitCode 是兼容原脚本的退出码。
	ExitCode *int `json:"exit_code"`
	// Stdout 是兼容原脚本的标准输出。
	Stdout string `json:"stdout"`
	// Stderr 是错误输出。
	Stderr string `json:"stderr"`
	// Report 是最近任务报告。
	Report NewAPICheckinReport `json:"report"`
}

// NewAPICheckinStartJobResult 是后台任务启动接口返回值。
type NewAPICheckinStartJobResult struct {
	// Started 表示本次是否新启动任务。
	Started bool `json:"started"`
	// Job 是启动后的任务状态。
	Job NewAPICheckinJobState `json:"job"`
}

// NewAPICheckinStartMonthlyResult 是月度同步启动接口返回值。
type NewAPICheckinStartMonthlyResult struct {
	// Started 表示本次是否新启动任务。
	Started bool `json:"started"`
	// SyncState 是启动后的同步状态。
	SyncState NewAPICheckinMonthlySyncState `json:"sync_state"`
}

// NewAPICheckinRefreshAccountResult 是单账号余额/签到刷新结果。
type NewAPICheckinRefreshAccountResult struct {
	// GeneratedAt 是刷新时间。
	GeneratedAt string `json:"generated_at"`
	// Site 是刷新站点。
	Site string `json:"site"`
	// UserID 是刷新账号。
	UserID string `json:"user_id"`
	// OK 表示余额与签到状态都可视作完成。
	OK bool `json:"ok"`
	// Message 是刷新消息。
	Message string `json:"message"`
	// Checkin 是签到查询结果。
	Checkin NewAPICheckinStatusResult `json:"checkin"`
	// Account 是余额缓存账号行。
	Account NewAPICheckinBalanceAccount `json:"account"`
	// Balances 是刷新后的余额缓存视图。
	Balances NewAPICheckinBalancePayload `json:"balances"`
	// History 是刷新后的历史视图。
	History NewAPICheckinHistoryPayload `json:"history"`
	// LastRun 是刷新后合并的最近执行视图。
	LastRun NewAPICheckinReport `json:"last_run"`
	// Monthly 是刷新后的月度视图。
	Monthly NewAPICheckinMonthlyPayload `json:"monthly"`
	// Changed 表示同步名称时配置是否发生变化。
	Changed bool `json:"changed,omitempty"`
}

// NewAPICheckinRefreshSiteResult 是站点余额/签到刷新结果。
type NewAPICheckinRefreshSiteResult struct {
	// GeneratedAt 是刷新时间。
	GeneratedAt string `json:"generated_at"`
	// Site 是刷新站点。
	Site string `json:"site"`
	// AccountCount 是刷新账号数。
	AccountCount int `json:"account_count"`
	// SuccessCount 是直签成功数。
	SuccessCount int `json:"success_count"`
	// AlreadyDoneCount 是今日已签到数。
	AlreadyDoneCount int `json:"already_done_count"`
	// FailedCount 是失败数。
	FailedCount int `json:"failed_count"`
	// Accounts 是账号刷新行。
	Accounts []NewAPICheckinBalanceAccount `json:"accounts"`
	// Balances 是刷新后的余额缓存。
	Balances NewAPICheckinBalancePayload `json:"balances"`
	// History 是刷新后的历史视图。
	History NewAPICheckinHistoryPayload `json:"history"`
	// LastRun 是刷新后合并的最近执行视图。
	LastRun NewAPICheckinReport `json:"last_run"`
	// Monthly 是刷新后的月度视图。
	Monthly NewAPICheckinMonthlyPayload `json:"monthly"`
	// Changed 表示同步名称时配置是否发生变化。
	Changed bool `json:"changed,omitempty"`
	// UpdatedAccounts 是同步名称接口的逐账号结果。
	UpdatedAccounts []map[string]any `json:"updated_accounts,omitempty"`
}

// NewAPICheckinStatusResult 是账号签到状态查询结果。
type NewAPICheckinStatusResult struct {
	// CheckinOK 表示签到流程是否可视作完成。
	CheckinOK bool `json:"checkin_ok"`
	// CheckinSuccess 表示直签是否成功。
	CheckinSuccess bool `json:"checkin_success"`
	// CheckedInToday 表示今日是否已签到。
	CheckedInToday bool `json:"checked_in_today"`
	// CheckinStatus 是状态文本。
	CheckinStatus string `json:"checkin_status"`
	// CheckinStatusTone 是状态色调。
	CheckinStatusTone string `json:"checkin_status_tone"`
	// CheckinMessage 是上游或本地消息。
	CheckinMessage string `json:"checkin_message"`
	// CheckinDate 是签到日期。
	CheckinDate string `json:"checkin_date"`
	// QuotaAwarded 是奖励原始额度。
	QuotaAwarded *int64 `json:"quota_awarded"`
	// QuotaAwardedDisplay 是奖励展示值。
	QuotaAwardedDisplay string `json:"quota_awarded_display"`
	// QuotaAwardedDisplayValue 是奖励展示数值。
	QuotaAwardedDisplayValue float64 `json:"quota_awarded_display_value"`
}

type newAPICheckinAPIResult struct {
	ok         bool
	statusCode int
	message    string
	payload    map[string]any
}

type newAPICheckinSub2Session struct {
	accessToken string
	keys        []any
	groups      []any
}

type newAPICheckinTask struct {
	site    NewAPICheckinSite
	account NewAPICheckinAccount
}

type NewAPICheckinMonthlyStore struct {
	Records []NewAPICheckinMonthlyRecord `json:"records"`
}

// NewNewAPICheckinService 创建 NewApi 签到迁移服务。
func NewNewAPICheckinService(options NewAPICheckinOptions) *NewAPICheckinService {
	client := options.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &NewAPICheckinService{
		repo:        options.Repository,
		accountRepo: options.AccountRepository,
		httpClient:  client,
		now:         now,
		checkinJobState: NewAPICheckinJobState{
			Message: "空闲",
			Report:  NewAPICheckinReport{},
		},
		monthlySyncState: NewAPICheckinMonthlySyncState{
			Message: "空闲",
		},
	}
}

// ProvideNewAPICheckinService 使用 SQL 仓储创建 NewApi 签到服务。
func ProvideNewAPICheckinService(repo NewAPICheckinRepository, accountRepo AccountRepository) *NewAPICheckinService {
	return NewNewAPICheckinService(NewAPICheckinOptions{Repository: repo, AccountRepository: accountRepo})
}

// ConfigSummary 返回脱敏后的平台目录和启用数量。
func (s *NewAPICheckinService) ConfigSummary(ctx context.Context) (NewAPICheckinConfigSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg, err := s.loadConfigLocked(ctx)
	if err != nil {
		return NewAPICheckinConfigSummary{}, err
	}
	return s.configSummaryLocked(cfg), nil
}

// APIKeys 从数据库缓存读取 Key 与分组，仅对缺少缓存或已被主平台引用的账号实时刷新。
func (s *NewAPICheckinService) APIKeys(ctx context.Context) (NewAPICheckinAPIKeyPayload, error) {
	return s.loadAPIKeys(ctx, false)
}

// SyncAPIKeys 强制从全部启用账号的上游同步最新 Key 与分组并更新数据库缓存。
func (s *NewAPICheckinService) SyncAPIKeys(ctx context.Context) (NewAPICheckinAPIKeyPayload, error) {
	return s.loadAPIKeys(ctx, true)
}

// loadAPIKeys 按页面普通读取或管理员手动同步策略组装 API Key 载荷。
func (s *NewAPICheckinService) loadAPIKeys(ctx context.Context, forceRefresh bool) (NewAPICheckinAPIKeyPayload, error) {
	s.mu.Lock()
	cfg, err := s.loadConfigLocked(ctx)
	s.mu.Unlock()
	if err != nil {
		return NewAPICheckinAPIKeyPayload{}, err
	}
	cacheEntries, err := s.repo.LoadAPIKeyCache(ctx)
	if err != nil {
		return NewAPICheckinAPIKeyPayload{}, fmt.Errorf("读取 API Key 数据库缓存失败: %w", err)
	}
	cacheByAccount := make(map[string]NewAPICheckinAPIKeyCacheEntry, len(cacheEntries))
	for _, entry := range cacheEntries {
		cacheByAccount[newAPICheckinCacheKey(entry.Site, entry.UserID)] = entry
	}
	mainAccounts := []Account{}
	if s.accountRepo != nil {
		mainAccounts, err = s.accountRepo.ListByPlatform(ctx, PlatformOpenAI)
		if err != nil {
			return NewAPICheckinAPIKeyPayload{}, fmt.Errorf("读取主平台账号失败: %w", err)
		}
	}

	type target struct {
		site    NewAPICheckinSite
		account NewAPICheckinAccount
	}
	targets := make([]target, 0)
	for _, site := range cfg.Sites {
		for _, account := range site.Accounts {
			if account.Enabled {
				targets = append(targets, target{site: site, account: account})
			}
		}
	}

	accounts := make([]NewAPICheckinAccountAPIKeySummary, len(targets))
	refresh := make([]bool, len(targets))
	cachedCount := 0
	for index, item := range targets {
		entry, ok := cacheByAccount[newAPICheckinCacheKey(item.site.Name, item.account.UserID)]
		if !ok {
			refresh[index] = true
			continue
		}
		accounts[index] = entry.Summary
		if forceRefresh {
			refresh[index] = true
			continue
		}
		if newAPICheckinAccountHasMainReference(item.account, mainAccounts) {
			refresh[index] = true
			continue
		}
		probe := cloneNewAPICheckinAPIKeySummary(entry.Summary)
		s.attachMainAccountReferences(&probe, item.site, mainAccounts)
		refresh[index] = newAPICheckinSummaryHasReferences(probe)
		if !refresh[index] {
			cachedCount++
		}
	}

	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(newAPICheckinAPIKeyConcurrency)
	for index, item := range targets {
		index, item := index, item
		if !refresh[index] {
			continue
		}
		group.Go(func() error {
			accounts[index] = s.queryAPIKeysLocked(groupCtx, item.site, item.account)
			return nil
		})
	}
	_ = group.Wait()
	refreshedEntries := make([]NewAPICheckinAPIKeyCacheEntry, 0)
	for index := range accounts {
		if refresh[index] && accounts[index].Status != "error" {
			refreshedEntries = append(refreshedEntries, NewAPICheckinAPIKeyCacheEntry{
				Site: targets[index].site.Name, UserID: targets[index].account.UserID,
				RefreshedAt: s.nowText(), Summary: cloneNewAPICheckinAPIKeySummary(accounts[index]),
			})
		}
	}
	if err := s.repo.SaveAPIKeyCache(ctx, refreshedEntries); err != nil {
		return NewAPICheckinAPIKeyPayload{}, fmt.Errorf("保存 API Key 数据库缓存失败: %w", err)
	}
	for index := range accounts {
		s.attachMainAccountReferences(&accounts[index], targets[index].site, mainAccounts)
	}
	sort.SliceStable(accounts, func(left, right int) bool {
		return newAPICheckinSummaryHasReferences(accounts[left]) && !newAPICheckinSummaryHasReferences(accounts[right])
	})

	payload := NewAPICheckinAPIKeyPayload{
		GeneratedAt: s.nowText(), AccountCount: len(accounts), Accounts: accounts,
		CachedCount: cachedCount, RefreshedCount: len(targets) - cachedCount,
	}
	for _, account := range accounts {
		switch account.Status {
		case "ready":
			payload.AvailableCount++
		case "missing":
			payload.MissingCount++
		case "unsupported":
			payload.UnsupportedCount++
		default:
			payload.ErrorCount++
		}
	}
	return payload, nil
}

// newAPICheckinCacheKey 返回站点与账号组成的稳定缓存索引。
func newAPICheckinCacheKey(site, userID string) string {
	return strings.TrimSpace(site) + "\x00" + strings.TrimSpace(userID)
}

// cloneNewAPICheckinAPIKeySummary 复制摘要并移除由主平台账号动态派生的引用列表。
func cloneNewAPICheckinAPIKeySummary(summary NewAPICheckinAccountAPIKeySummary) NewAPICheckinAccountAPIKeySummary {
	cloned := summary
	cloned.AvailableGroups = append([]string(nil), summary.AvailableGroups...)
	cloned.AvailableGroupOptions = append([]NewAPICheckinGroupOption(nil), summary.AvailableGroupOptions...)
	cloned.APIKeys = append([]NewAPICheckinAPIKeySummary(nil), summary.APIKeys...)
	for index := range cloned.APIKeys {
		cloned.APIKeys[index].ReferencedAccounts = nil
		cloned.APIKeys[index].TargetAccounts = nil
	}
	return cloned
}

// LinkAPIKeyToAccount 将签到工具生成的 Key 追加或替换到同站点的主平台账号。
func (s *NewAPICheckinService) LinkAPIKeyToAccount(ctx context.Context, siteName, userID string, apiKeyID, targetAccountID int64, operation string) (NewAPICheckinLinkAPIKeyResult, error) {
	if s.accountRepo == nil {
		return NewAPICheckinLinkAPIKeyResult{}, fmt.Errorf("主平台账号关联服务不可用")
	}
	operation = strings.ToLower(strings.TrimSpace(operation))
	if operation != "append" && operation != "replace" {
		return NewAPICheckinLinkAPIKeyResult{}, fmt.Errorf("operation 必须是 append 或 replace")
	}
	revealed, err := s.RevealAPIKey(ctx, siteName, userID, apiKeyID)
	if err != nil {
		return NewAPICheckinLinkAPIKeyResult{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	site, _, err := s.findConfiguredAccountLocked(ctx, siteName, userID)
	if err != nil {
		return NewAPICheckinLinkAPIKeyResult{}, err
	}
	target, err := s.accountRepo.GetByID(ctx, targetAccountID)
	if err != nil {
		return NewAPICheckinLinkAPIKeyResult{}, err
	}
	if target.Platform != PlatformOpenAI || target.Type != AccountTypeAPIKey || !newAPIAccountBaseURLsMatch(target.GetCredential("base_url"), site.BaseURL) {
		return NewAPICheckinLinkAPIKeyResult{}, fmt.Errorf("目标账号与签到站点 URL 不匹配")
	}
	// Key 关联是局部更新；先复制完整凭据，避免全量表单合并语义删除 base_url 等未显式提交字段。
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
		return NewAPICheckinLinkAPIKeyResult{}, err
	}
	return NewAPICheckinLinkAPIKeyResult{Site: site.Name, UserID: userID, APIKeyID: apiKeyID, TargetAccountID: target.ID, TargetAccountName: target.Name, Operation: operation, KeyCount: len(target.GetAPIKeys()), Message: "主平台账号 Key 已更新"}, nil
}

// attachMainAccountReferences 从数据库 Key 标注引用关系，并按兼容站点 URL 提供可操作账号。
func (s *NewAPICheckinService) attachMainAccountReferences(summary *NewAPICheckinAccountAPIKeySummary, site NewAPICheckinSite, accounts []Account) {
	for keyIndex := range summary.APIKeys {
		key := &summary.APIKeys[keyIndex]
		for index := range accounts {
			account := &accounts[index]
			if account.Type != AccountTypeAPIKey {
				continue
			}
			referenced := false
			for _, existing := range storedAccountAPIKeys(account.Credentials) {
				if newAPIGeneratedKeyMatchesStored(key.MatchKey, existing) {
					referenced = true
					break
				}
			}
			ref := NewAPICheckinAccountReference{ID: account.ID, Name: account.Name, Referenced: referenced}
			if referenced {
				key.ReferencedAccounts = append(key.ReferencedAccounts, ref)
			}
			if newAPIAccountBaseURLsMatch(account.GetCredential("base_url"), site.BaseURL) {
				key.TargetAccounts = append(key.TargetAccounts, ref)
			}
		}
		sort.SliceStable(key.TargetAccounts, func(left, right int) bool {
			return key.TargetAccounts[left].Referenced && !key.TargetAccounts[right].Referenced
		})
	}
	sort.SliceStable(summary.APIKeys, func(left, right int) bool {
		return len(summary.APIKeys[left].ReferencedAccounts) > 0 && len(summary.APIKeys[right].ReferencedAccounts) == 0
	})
}

// newAPICheckinSummaryHasReferences 判断签到账号是否包含数据库已引用 Key，用于账号级置顶。
func newAPICheckinSummaryHasReferences(summary NewAPICheckinAccountAPIKeySummary) bool {
	for _, key := range summary.APIKeys {
		if len(key.ReferencedAccounts) > 0 {
			return true
		}
	}
	return false
}

// newAPICheckinAccountHasMainReference 使用签到账号自身的 access key 判断是否已被主平台引用。
// 该判断不依赖旧缓存，确保 unsupported 等无 Key 缓存也能在保存凭据后实时恢复。
func newAPICheckinAccountHasMainReference(checkinAccount NewAPICheckinAccount, accounts []Account) bool {
	for index := range accounts {
		account := &accounts[index]
		if account.Type != AccountTypeAPIKey {
			continue
		}
		for _, existing := range storedAccountAPIKeys(account.Credentials) {
			if newAPIGeneratedKeyMatchesStored(checkinAccount.AccessKey, existing) {
				return true
			}
		}
	}
	return false
}

// newAPIGeneratedKeyMatchesStored 使用 NewAPI 固定的前4位、后4位脱敏格式匹配数据库完整 Key。
func newAPIGeneratedKeyMatchesStored(generated, stored string) bool {
	generated = normalizeNewAPIFullKey(generated)
	stored = normalizeNewAPIFullKey(stored)
	if generated == "" || stored == "" {
		return false
	}
	if generated == stored {
		return true
	}
	firstMask := strings.IndexByte(generated, '*')
	lastMask := strings.LastIndexByte(generated, '*')
	if firstMask <= len("sk-") || lastMask < firstMask || lastMask+1 >= len(generated) {
		return false
	}
	prefix := generated[:firstMask]
	suffix := generated[lastMask+1:]
	return len(suffix) >= 4 && strings.HasPrefix(stored, prefix) && strings.HasSuffix(stored, suffix)
}

// newAPIAccountBaseURLsMatch 兼容根域名与 api. 子域名，以及常见 /v1 后缀差异。
func newAPIAccountBaseURLsMatch(left, right string) bool {
	left = normalizeNewAPIAccountBaseURL(left)
	right = normalizeNewAPIAccountBaseURL(right)
	if left == right {
		return true
	}
	leftURL, leftErr := url.Parse(left)
	rightURL, rightErr := url.Parse(right)
	if leftErr != nil || rightErr != nil || leftURL.Host == "" || rightURL.Host == "" {
		return false
	}
	leftHost := strings.TrimPrefix(strings.ToLower(leftURL.Hostname()), "api.")
	rightHost := strings.TrimPrefix(strings.ToLower(rightURL.Hostname()), "api.")
	return leftURL.Scheme == rightURL.Scheme && leftHost == rightHost && leftURL.Port() == rightURL.Port() && leftURL.Path == rightURL.Path
}

// resolveStoredFullAPIKey 将上游脱敏 Key 映射为数据库中唯一的完整 Key。
func (s *NewAPICheckinService) resolveStoredFullAPIKey(ctx context.Context, generated string) (string, error) {
	generated = normalizeNewAPIFullKey(generated)
	if generated == "" {
		return "", fmt.Errorf("上游未返回 API Key")
	}
	if !strings.Contains(generated, "*") {
		return generated, nil
	}
	if s.accountRepo == nil {
		return "", fmt.Errorf("上游仅返回脱敏 Key，主平台账号数据库不可用")
	}
	accounts, err := s.accountRepo.ListByPlatform(ctx, PlatformOpenAI)
	if err != nil {
		return "", fmt.Errorf("读取主平台账号失败: %w", err)
	}
	matches := map[string]struct{}{}
	for index := range accounts {
		if accounts[index].Type != AccountTypeAPIKey {
			continue
		}
		for _, stored := range storedAccountAPIKeys(accounts[index].Credentials) {
			stored = normalizeNewAPIFullKey(stored)
			if newAPIGeneratedKeyMatchesStored(generated, stored) {
				matches[stored] = struct{}{}
			}
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("上游仅返回脱敏 Key，主平台数据库未找到对应完整 Key")
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("脱敏 Key 匹配到多个不同完整 Key，无法安全关联")
	}
	for key := range matches {
		return key, nil
	}
	return "", fmt.Errorf("主平台数据库未找到对应完整 Key")
}

// storedAccountAPIKeys 返回账号保存的全部 Key，包括暂时禁用的 Key，用于准确标记引用关系。
func storedAccountAPIKeys(credentials map[string]any) []string {
	keys := normalizeAPIKeys(credentials["api_keys"])
	if len(keys) == 0 {
		if legacy, ok := credentials["api_key"].(string); ok && strings.TrimSpace(legacy) != "" {
			keys = []string{strings.TrimSpace(legacy)}
		}
	}
	return keys
}

// normalizeNewAPIAccountBaseURL 统一协议、主机、末尾斜杠和常见 /v1 后缀用于站点关联。
func normalizeNewAPIAccountBaseURL(value string) string {
	value = strings.TrimSpace(value)
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return strings.ToLower(strings.TrimRight(value, "/"))
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	if strings.EqualFold(parsed.Path, "/v1") {
		parsed.Path = ""
	}
	return strings.TrimRight(parsed.String(), "/")
}

// RevealAPIKey 实时读取账号最新 Key 与分组、更新缓存，再按 token ID 返回完整 Key。
func (s *NewAPICheckinService) RevealAPIKey(ctx context.Context, siteName, userID string, apiKeyID int64) (NewAPICheckinAPIKeyRevealResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	site, account, err := s.findConfiguredAccountLocked(ctx, siteName, userID)
	if err != nil {
		return NewAPICheckinAPIKeyRevealResult{}, err
	}
	summary := s.queryAPIKeysLocked(ctx, site, account)
	if summary.Status == "error" || summary.Status == "unsupported" {
		return NewAPICheckinAPIKeyRevealResult{}, fmt.Errorf("实时读取 API Key 失败: %s", firstNonEmpty(summary.Message, summary.GroupMessage))
	}
	if err := s.saveAPIKeySummaryCacheLocked(ctx, site, account, summary); err != nil {
		return NewAPICheckinAPIKeyRevealResult{}, err
	}
	var selected *NewAPICheckinAPIKeySummary
	for index := range summary.APIKeys {
		if summary.APIKeys[index].ID == apiKeyID {
			selected = &summary.APIKeys[index]
			break
		}
	}
	if selected == nil {
		return NewAPICheckinAPIKeyRevealResult{}, fmt.Errorf("未找到 API Key %d", apiKeyID)
	}
	key := selected.MatchKey
	if !isSub2APISite(site) {
		key, err = s.resolveStoredFullAPIKey(ctx, key)
	}
	if err != nil {
		return NewAPICheckinAPIKeyRevealResult{}, fmt.Errorf("读取 token %d 完整 API Key 失败: %w", apiKeyID, err)
	}
	if key == "" || strings.Contains(key, "*") {
		return NewAPICheckinAPIKeyRevealResult{}, fmt.Errorf("上游 Key %d 未返回完整 API Key", apiKeyID)
	}
	return NewAPICheckinAPIKeyRevealResult{
		Site: site.Name, UserID: account.UserID, APIKeyID: apiKeyID,
		Name: selected.Name, Key: key, MaskedKey: maskNewAPIKey(key), Group: selected.Group,
	}, nil
}

// saveAPIKeySummaryCacheLocked 保存单个账号刚从上游读取的 Key 与分组摘要。
func (s *NewAPICheckinService) saveAPIKeySummaryCacheLocked(ctx context.Context, site NewAPICheckinSite, account NewAPICheckinAccount, summary NewAPICheckinAccountAPIKeySummary) error {
	entry := NewAPICheckinAPIKeyCacheEntry{
		Site: site.Name, UserID: account.UserID, RefreshedAt: s.nowText(),
		Summary: cloneNewAPICheckinAPIKeySummary(summary),
	}
	if err := s.repo.SaveAPIKeyCache(ctx, []NewAPICheckinAPIKeyCacheEntry{entry}); err != nil {
		return fmt.Errorf("保存 API Key 数据库缓存失败: %w", err)
	}
	return nil
}

// UpdateAPIKeyGroup 保留上游 token 的完整字段，只替换 group 后提交更新。
func (s *NewAPICheckinService) UpdateAPIKeyGroup(ctx context.Context, siteName, userID string, apiKeyID int64, groupName string, groupID int64) (NewAPICheckinAPIKeyGroupResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	groupName = cleanNewAPIText(groupName)
	site, account, err := s.findConfiguredAccountLocked(ctx, siteName, userID)
	if err != nil {
		return NewAPICheckinAPIKeyGroupResult{}, err
	}
	if isSub2APISite(site) {
		if groupID <= 0 {
			return NewAPICheckinAPIKeyGroupResult{}, fmt.Errorf("sub2api 分组 ID 必须是正整数")
		}
		session, err := s.loginSub2AccountLocked(ctx, site, account)
		if err != nil {
			return NewAPICheckinAPIKeyGroupResult{}, err
		}
		item, err := findSub2APIKey(session.keys, apiKeyID)
		if err != nil {
			return NewAPICheckinAPIKeyGroupResult{}, err
		}
		groupName = firstNonEmpty(groupName, findSub2GroupName(session.groups, groupID))
		result := s.requestJSONBodyLocked(ctx, http.MethodPut, joinNewAPIURL(site.BaseURL, fmt.Sprintf("/api/v1/keys/%d", apiKeyID)), site.BaseURL, sub2BearerHeaders(session.accessToken), map[string]any{"group_id": groupID})
		if !result.ok {
			return NewAPICheckinAPIKeyGroupResult{}, fmt.Errorf("更新 API Key 分组失败: %s", firstNonEmpty(result.message, "上游拒绝请求"))
		}
		refreshed := s.queryAPIKeysLocked(ctx, site, account)
		if refreshed.Status == "error" {
			return NewAPICheckinAPIKeyGroupResult{}, fmt.Errorf("分组已更新，但重新读取最新 Key 与分组失败: %s", refreshed.Message)
		}
		if err := s.saveAPIKeySummaryCacheLocked(ctx, site, account, refreshed); err != nil {
			return NewAPICheckinAPIKeyGroupResult{}, err
		}
		return NewAPICheckinAPIKeyGroupResult{
			Site: site.Name, UserID: account.UserID, APIKeyID: apiKeyID,
			Name: firstNonEmpty(cleanNewAPIText(item["name"]), "未命名"), Group: groupName, GroupID: groupID,
			Message: firstNonEmpty(result.message, "分组已更新"),
		}, nil
	}
	if groupName == "" {
		return NewAPICheckinAPIKeyGroupResult{}, fmt.Errorf("分组不能为空")
	}
	item, err := s.findNewAPITokenLocked(ctx, site, account, apiKeyID)
	if err != nil {
		return NewAPICheckinAPIKeyGroupResult{}, err
	}
	item["group"] = groupName
	headers := newAPICheckinAccountHeaders(account)
	result := s.requestJSONBodyLocked(ctx, http.MethodPut, joinNewAPIURL(site.BaseURL, "/api/token/"), site.BaseURL, headers, item)
	if !result.ok {
		return NewAPICheckinAPIKeyGroupResult{}, fmt.Errorf("更新 API Key 分组失败: %s", firstNonEmpty(result.message, "上游拒绝请求"))
	}
	refreshed := s.queryAPIKeysLocked(ctx, site, account)
	if refreshed.Status == "error" {
		return NewAPICheckinAPIKeyGroupResult{}, fmt.Errorf("分组已更新，但重新读取最新 Key 与分组失败: %s", refreshed.Message)
	}
	if err := s.saveAPIKeySummaryCacheLocked(ctx, site, account, refreshed); err != nil {
		return NewAPICheckinAPIKeyGroupResult{}, err
	}
	return NewAPICheckinAPIKeyGroupResult{
		Site: site.Name, UserID: account.UserID, APIKeyID: apiKeyID,
		Name: firstNonEmpty(cleanNewAPIText(item["name"]), "未命名"), Group: groupName,
		Message: firstNonEmpty(result.message, "分组已更新"),
	}, nil
}

// LastRun 读取最近一次非空签到报告。
func (s *NewAPICheckinService) LastRun(ctx context.Context) (NewAPICheckinReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	report, err := s.loadReportLocked(ctx)
	if err != nil {
		return NewAPICheckinReport{}, err
	}
	return s.rebuildReportLocked(report), nil
}

// Balances 读取并重建实时余额缓存视图。
func (s *NewAPICheckinService) Balances(ctx context.Context) (NewAPICheckinBalancePayload, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg, err := s.loadConfigLocked(ctx)
	if err != nil {
		return NewAPICheckinBalancePayload{}, err
	}
	cache, err := s.loadBalanceCacheLocked(ctx)
	if err != nil {
		return NewAPICheckinBalancePayload{}, err
	}
	cache = s.mergeBalanceCacheWithConfigLocked(cache, cfg)
	return s.rebuildBalancePayloadLocked(cache), nil
}

// History 读取签到历史并返回趋势聚合。
func (s *NewAPICheckinService) History(ctx context.Context) (NewAPICheckinHistoryPayload, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buildHistoryPayloadLocked(ctx)
}

// BalanceHistory 返回实时余额趋势数据。
func (s *NewAPICheckinService) BalanceHistory(ctx context.Context, month, siteName, userID string) (NewAPICheckinHistoryPayload, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	payload, err := s.buildHistoryPayloadLocked(ctx)
	if err != nil {
		return NewAPICheckinHistoryPayload{}, err
	}
	month = normalizeNewAPIMonth(month, s.currentMonth())
	siteName = cleanNewAPIText(siteName)
	userID = cleanNewAPIText(userID)
	entries := payload.Entries[:0]
	for _, entry := range payload.Entries {
		if month != "" && !strings.HasPrefix(entry.Date, month) {
			continue
		}
		if siteName != "" && entry.Site != siteName {
			continue
		}
		if userID != "" && entry.UserID != userID {
			continue
		}
		entries = append(entries, entry)
	}
	return s.buildHistoryPayloadFromEntriesLocked(entries), nil
}

// Monthly 返回月度签到记录和趋势聚合。
func (s *NewAPICheckinService) Monthly(ctx context.Context, month, siteName, userID string) (NewAPICheckinMonthlyPayload, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buildMonthlyPayloadLocked(ctx, month, siteName, userID)
}

// MonthlySyncStatus 返回月度同步任务状态。
func (s *NewAPICheckinService) MonthlySyncStatus() NewAPICheckinMonthlySyncState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.monthlySyncState
}

// CheckinJobStatus 返回全量签到后台任务状态。
func (s *NewAPICheckinService) CheckinJobStatus() NewAPICheckinJobState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.checkinJobState
}

// SetSiteEnabled 修改站点签到开关，禁用站点仍保留在页面和查询范围内。
func (s *NewAPICheckinService) SetSiteEnabled(ctx context.Context, siteName string, enabled bool, disabledReason string) (NewAPICheckinSiteEnabledResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	siteName = cleanNewAPIText(siteName)
	if siteName == "" {
		return NewAPICheckinSiteEnabledResult{}, errors.New("缺少 site")
	}
	cfg, err := s.loadConfigLocked(ctx)
	if err != nil {
		return NewAPICheckinSiteEnabledResult{}, err
	}
	site, idx := findNewAPISite(cfg, siteName)
	if idx < 0 {
		return NewAPICheckinSiteEnabledResult{}, fmt.Errorf("未找到站点: %s", siteName)
	}
	site.Enabled = enabled
	if enabled {
		if cleanNewAPIText(disabledReason) != "" {
			site.DisabledReason = cleanNewAPIText(disabledReason)
		} else {
			site.DisabledReason = ""
		}
	} else {
		site.DisabledReason = cleanNewAPIText(disabledReason)
		if site.DisabledReason == "" {
			site.DisabledReason = "页面禁用签到"
		}
	}
	cfg.Sites[idx] = site
	if err := s.saveConfigLocked(ctx, cfg); err != nil {
		return NewAPICheckinSiteEnabledResult{}, err
	}
	cache, err := s.loadBalanceCacheLocked(ctx)
	if err != nil {
		return NewAPICheckinSiteEnabledResult{}, err
	}
	cache = s.mergeBalanceCacheWithConfigLocked(cache, cfg)
	balances := s.rebuildBalancePayloadLocked(cache)
	return NewAPICheckinSiteEnabledResult{
		GeneratedAt:    s.nowText(),
		ConfigPath:     s.path(newAPICheckinConfigFile),
		Site:           siteName,
		Enabled:        site.Enabled,
		DisabledReason: site.DisabledReason,
		Config:         s.configSummaryLocked(cfg),
		Balances:       balances,
	}, nil
}

// AddOrMergeSites 写入单个平台或 sites 数组，按站点名和 user_id 合并。
func (s *NewAPICheckinService) AddOrMergeSites(ctx context.Context, payload map[string]any) (NewAPICheckinAddConfigResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	incoming, err := normalizeIncomingNewAPISites(payload)
	if err != nil {
		return NewAPICheckinAddConfigResult{}, err
	}
	cfg, err := s.loadConfigLocked(ctx)
	if err != nil {
		return NewAPICheckinAddConfigResult{}, err
	}
	changedSites := make([]map[string]any, 0, len(incoming))
	addedAccounts := 0
	updatedAccounts := 0
	for _, next := range incoming {
		existing, idx := findNewAPISite(cfg, next.Name)
		if idx < 0 {
			cfg.Sites = append(cfg.Sites, next)
			addedAccounts += len(next.Accounts)
			changedSites = append(changedSites, map[string]any{"site": next.Name, "mode": "created", "account_count": len(next.Accounts)})
			continue
		}
		nextAccounts := next.Accounts
		next.Accounts = existing.Accounts
		existingByUser := map[string]int{}
		for i, account := range next.Accounts {
			existingByUser[account.UserID] = i
		}
		siteAdded := 0
		siteUpdated := 0
		for _, account := range nextAccounts {
			if accountIdx, ok := existingByUser[account.UserID]; ok {
				// JSON 合并未携带登录凭据时保留数据库中的原值，避免普通账号维护清空密码。
				if account.LoginUsername == "" {
					account.LoginUsername = next.Accounts[accountIdx].LoginUsername
				}
				if account.LoginPassword == "" {
					account.LoginPassword = next.Accounts[accountIdx].LoginPassword
				}
				next.Accounts[accountIdx] = account
				siteUpdated++
				updatedAccounts++
				continue
			}
			next.Accounts = append(next.Accounts, account)
			siteAdded++
			addedAccounts++
		}
		cfg.Sites[idx] = next
		changedSites = append(changedSites, map[string]any{
			"site":             next.Name,
			"mode":             "merged",
			"account_count":    len(nextAccounts),
			"added_accounts":   siteAdded,
			"updated_accounts": siteUpdated,
		})
	}
	if err := s.saveConfigLocked(ctx, cfg); err != nil {
		return NewAPICheckinAddConfigResult{}, err
	}
	cache, err := s.loadBalanceCacheLocked(ctx)
	if err != nil {
		return NewAPICheckinAddConfigResult{}, err
	}
	cache = s.mergeBalanceCacheWithConfigLocked(cache, cfg)
	balances := s.rebuildBalancePayloadLocked(cache)
	return NewAPICheckinAddConfigResult{
		GeneratedAt:     s.nowText(),
		ConfigPath:      s.path(newAPICheckinConfigFile),
		SiteCount:       len(incoming),
		AddedAccounts:   addedAccounts,
		UpdatedAccounts: updatedAccounts,
		Sites:           changedSites,
		Config:          s.configSummaryLocked(cfg),
		Balances:        balances,
	}, nil
}

// DeleteSite 删除站点及其缓存、历史和月度记录。
func (s *NewAPICheckinService) DeleteSite(ctx context.Context, siteName string) (NewAPICheckinDeleteResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	siteName = cleanNewAPIText(siteName)
	if siteName == "" {
		return NewAPICheckinDeleteResult{}, errors.New("缺少 site")
	}
	cfg, err := s.loadConfigLocked(ctx)
	if err != nil {
		return NewAPICheckinDeleteResult{}, err
	}
	site, idx := findNewAPISite(cfg, siteName)
	if idx < 0 {
		return NewAPICheckinDeleteResult{}, fmt.Errorf("未找到站点: %s", siteName)
	}
	cfg.Sites = append(cfg.Sites[:idx], cfg.Sites[idx+1:]...)
	if err := s.saveConfigLocked(ctx, cfg); err != nil {
		return NewAPICheckinDeleteResult{}, err
	}
	removedCache, err := s.pruneBalanceCacheLocked(ctx, siteName, "")
	if err != nil {
		return NewAPICheckinDeleteResult{}, err
	}
	removedHistory, err := s.pruneHistoryLocked(ctx, siteName, "")
	if err != nil {
		return NewAPICheckinDeleteResult{}, err
	}
	removedMonthly, err := s.pruneMonthlyLocked(ctx, siteName, "")
	if err != nil {
		return NewAPICheckinDeleteResult{}, err
	}
	lastRun, err := s.pruneLastRunLocked(ctx, siteName, "")
	if err != nil {
		return NewAPICheckinDeleteResult{}, err
	}
	return s.deleteResultLocked(ctx, cfg, siteName, "", len(site.Accounts), removedCache, removedHistory, removedMonthly, lastRun)
}

// DeleteAccount 删除指定站点账号及其缓存、历史和月度记录。
func (s *NewAPICheckinService) DeleteAccount(ctx context.Context, siteName, userID string) (NewAPICheckinDeleteResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	siteName = cleanNewAPIText(siteName)
	userID = cleanNewAPIText(userID)
	if siteName == "" || userID == "" {
		return NewAPICheckinDeleteResult{}, errors.New("缺少 site 或 user_id")
	}
	cfg, err := s.loadConfigLocked(ctx)
	if err != nil {
		return NewAPICheckinDeleteResult{}, err
	}
	site, siteIdx := findNewAPISite(cfg, siteName)
	if siteIdx < 0 {
		return NewAPICheckinDeleteResult{}, fmt.Errorf("未找到站点: %s", siteName)
	}
	_, accountIdx := findNewAPIAccount(site, userID)
	if accountIdx < 0 {
		return NewAPICheckinDeleteResult{}, fmt.Errorf("未找到账号: %s/%s", siteName, userID)
	}
	site.Accounts = append(site.Accounts[:accountIdx], site.Accounts[accountIdx+1:]...)
	cfg.Sites[siteIdx] = site
	if err := s.saveConfigLocked(ctx, cfg); err != nil {
		return NewAPICheckinDeleteResult{}, err
	}
	removedCache, err := s.pruneBalanceCacheLocked(ctx, siteName, userID)
	if err != nil {
		return NewAPICheckinDeleteResult{}, err
	}
	removedHistory, err := s.pruneHistoryLocked(ctx, siteName, userID)
	if err != nil {
		return NewAPICheckinDeleteResult{}, err
	}
	removedMonthly, err := s.pruneMonthlyLocked(ctx, siteName, userID)
	if err != nil {
		return NewAPICheckinDeleteResult{}, err
	}
	lastRun, err := s.pruneLastRunLocked(ctx, siteName, userID)
	if err != nil {
		return NewAPICheckinDeleteResult{}, err
	}
	return s.deleteResultLocked(ctx, cfg, siteName, userID, 1, removedCache, removedHistory, removedMonthly, lastRun)
}

// SetAccountDisplayName 保存管理员手工填写的用户名或邮箱标识，不读取远端账号凭据。
func (s *NewAPICheckinService) SetAccountDisplayName(ctx context.Context, siteName, userID, displayName string) (NewAPICheckinConfigSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	siteName = cleanNewAPIText(siteName)
	userID = cleanNewAPIText(userID)
	displayName = cleanNewAPIText(displayName)
	if siteName == "" || userID == "" || displayName == "" {
		return NewAPICheckinConfigSummary{}, errors.New("site、user_id 和 display_name 均不能为空")
	}
	cfg, err := s.loadConfigLocked(ctx)
	if err != nil {
		return NewAPICheckinConfigSummary{}, err
	}
	site, siteIdx := findNewAPISite(cfg, siteName)
	if siteIdx < 0 {
		return NewAPICheckinConfigSummary{}, fmt.Errorf("未找到站点: %s", siteName)
	}
	account, accountIdx := findNewAPIAccount(site, userID)
	if accountIdx < 0 {
		return NewAPICheckinConfigSummary{}, fmt.Errorf("未找到账号: %s/%s", siteName, userID)
	}
	account.DisplayName = displayName
	site.Accounts[accountIdx] = account
	cfg.Sites[siteIdx] = site
	if err := s.saveConfigLocked(ctx, cfg); err != nil {
		return NewAPICheckinConfigSummary{}, err
	}
	return s.configSummaryLocked(cfg), nil
}

// SetAccountLoginCredentials 验证 sub2api 登录凭据后写入账号配置，密码不会进入返回摘要。
func (s *NewAPICheckinService) SetAccountLoginCredentials(ctx context.Context, siteName, userID, loginUsername, loginPassword string) (NewAPICheckinLoginCredentialResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg, site, account, siteIdx, accountIdx, err := s.configuredAccountForUpdateLocked(ctx, siteName, userID)
	if err != nil {
		return NewAPICheckinLoginCredentialResult{}, err
	}
	loginUsername, loginPassword, err = resolveNewAPILoginCredentials(account, loginUsername, loginPassword)
	if err != nil {
		return NewAPICheckinLoginCredentialResult{}, err
	}
	probe, refreshed, err := s.probeSub2LoginLocked(ctx, site, account, loginUsername, loginPassword)
	if err != nil {
		return NewAPICheckinLoginCredentialResult{}, err
	}
	account.LoginUsername = loginUsername
	account.LoginPassword = loginPassword
	site.Accounts[accountIdx] = account
	cfg.Sites[siteIdx] = site
	if err := s.saveConfigLocked(ctx, cfg); err != nil {
		return NewAPICheckinLoginCredentialResult{}, err
	}
	if err := s.saveAPIKeySummaryCacheLocked(ctx, site, account, refreshed); err != nil {
		return NewAPICheckinLoginCredentialResult{}, err
	}
	summary := s.configSummaryLocked(cfg)
	probe.Config = &summary
	probe.HasLoginPassword = true
	probe.Message = "登录验证成功，凭据已保存"
	return probe, nil
}

// TestAccountLoginCredentials 使用提交值或已保存值登录 sub2api，仅返回 Key/分组数量，不保存配置。
func (s *NewAPICheckinService) TestAccountLoginCredentials(ctx context.Context, siteName, userID, loginUsername, loginPassword string) (NewAPICheckinLoginCredentialResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	site, account, err := s.findConfiguredAccountLocked(ctx, siteName, userID)
	if err != nil {
		return NewAPICheckinLoginCredentialResult{}, err
	}
	loginUsername, loginPassword, err = resolveNewAPILoginCredentials(account, loginUsername, loginPassword)
	if err != nil {
		return NewAPICheckinLoginCredentialResult{}, err
	}
	probe, _, err := s.probeSub2LoginLocked(ctx, site, account, loginUsername, loginPassword)
	return probe, err
}

func (s *NewAPICheckinService) configuredAccountForUpdateLocked(ctx context.Context, siteName, userID string) (NewAPICheckinConfig, NewAPICheckinSite, NewAPICheckinAccount, int, int, error) {
	cfg, err := s.loadConfigLocked(ctx)
	if err != nil {
		return NewAPICheckinConfig{}, NewAPICheckinSite{}, NewAPICheckinAccount{}, -1, -1, err
	}
	site, siteIdx := findNewAPISite(cfg, siteName)
	if siteIdx < 0 {
		return NewAPICheckinConfig{}, NewAPICheckinSite{}, NewAPICheckinAccount{}, -1, -1, fmt.Errorf("站点不存在: %s", siteName)
	}
	account, accountIdx := findNewAPIAccount(site, userID)
	if accountIdx < 0 {
		return NewAPICheckinConfig{}, NewAPICheckinSite{}, NewAPICheckinAccount{}, -1, -1, fmt.Errorf("账号不存在: %s / %s", siteName, userID)
	}
	return cfg, site, account, siteIdx, accountIdx, nil
}

// RefreshAccountBalance 刷新单个账号的签到状态和余额。
func (s *NewAPICheckinService) RefreshAccountBalance(ctx context.Context, siteName, userID string) (NewAPICheckinRefreshAccountResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.refreshAccountBalanceLocked(ctx, siteName, userID, false, "account-refresh")
}

// RefreshSiteBalances 刷新站点下所有账号的签到状态和余额。
func (s *NewAPICheckinService) RefreshSiteBalances(ctx context.Context, siteName string) (NewAPICheckinRefreshSiteResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.refreshSiteBalancesLocked(ctx, siteName, true, "site-refresh")
}

// SyncAccountName 刷新账号并把上游名称同步回配置。
func (s *NewAPICheckinService) SyncAccountName(ctx context.Context, siteName, userID string) (NewAPICheckinRefreshAccountResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg, err := s.loadConfigLocked(ctx)
	if err != nil {
		return NewAPICheckinRefreshAccountResult{}, err
	}
	configuredSite, configuredSiteIdx := findNewAPISite(cfg, siteName)
	if configuredSiteIdx < 0 {
		return NewAPICheckinRefreshAccountResult{}, fmt.Errorf("未找到站点: %s", siteName)
	}
	if isSub2APISite(configuredSite) {
		return NewAPICheckinRefreshAccountResult{}, fmt.Errorf("%s 是只读数据源，不支持名称同步", siteName)
	}
	result, err := s.refreshAccountBalanceLocked(ctx, siteName, userID, false, "account-name-sync")
	if err != nil {
		return NewAPICheckinRefreshAccountResult{}, err
	}
	cfg, err = s.loadConfigLocked(ctx)
	if err != nil {
		return NewAPICheckinRefreshAccountResult{}, err
	}
	site, siteIdx := findNewAPISite(cfg, siteName)
	account, accountIdx := findNewAPIAccount(site, userID)
	before := []string{account.Name, account.Username, account.DisplayName}
	if result.Account.Username != "" {
		account.Username = result.Account.Username
	}
	if result.Account.DisplayName != "" {
		account.DisplayName = result.Account.DisplayName
	}
	if label := resolveNewAPIAccountLabel(account, result.Account); label != "" {
		account.Name = label
	}
	after := []string{account.Name, account.Username, account.DisplayName}
	result.Changed = strings.Join(before, "\x00") != strings.Join(after, "\x00")
	if result.Changed {
		site.Accounts[accountIdx] = account
		cfg.Sites[siteIdx] = site
		if err := s.saveConfigLocked(ctx, cfg); err != nil {
			return NewAPICheckinRefreshAccountResult{}, err
		}
	}
	return result, nil
}

// SyncSiteNames 刷新站点并把上游账号名称同步回配置。
func (s *NewAPICheckinService) SyncSiteNames(ctx context.Context, siteName string) (NewAPICheckinRefreshSiteResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg, err := s.loadConfigLocked(ctx)
	if err != nil {
		return NewAPICheckinRefreshSiteResult{}, err
	}
	configuredSite, configuredSiteIdx := findNewAPISite(cfg, siteName)
	if configuredSiteIdx < 0 {
		return NewAPICheckinRefreshSiteResult{}, fmt.Errorf("未找到站点: %s", siteName)
	}
	if isSub2APISite(configuredSite) {
		return NewAPICheckinRefreshSiteResult{}, fmt.Errorf("%s 是只读数据源，不支持名称同步", siteName)
	}
	result, err := s.refreshSiteBalancesLocked(ctx, siteName, true, "site-name-sync")
	if err != nil {
		return NewAPICheckinRefreshSiteResult{}, err
	}
	cfg, err = s.loadConfigLocked(ctx)
	if err != nil {
		return NewAPICheckinRefreshSiteResult{}, err
	}
	site, siteIdx := findNewAPISite(cfg, siteName)
	rows := map[string]NewAPICheckinBalanceAccount{}
	for _, row := range result.Accounts {
		rows[row.UserID] = row
	}
	changed := false
	updates := []map[string]any{}
	for idx, account := range site.Accounts {
		row := rows[account.UserID]
		before := []string{account.Name, account.Username, account.DisplayName}
		if row.Username != "" {
			account.Username = row.Username
		}
		if row.DisplayName != "" {
			account.DisplayName = row.DisplayName
		}
		if label := resolveNewAPIAccountLabel(account, row); label != "" {
			account.Name = label
		}
		after := []string{account.Name, account.Username, account.DisplayName}
		accountChanged := strings.Join(before, "\x00") != strings.Join(after, "\x00")
		changed = changed || accountChanged
		site.Accounts[idx] = account
		updates = append(updates, map[string]any{
			"site":    siteName,
			"user_id": account.UserID,
			"label":   resolveNewAPIAccountLabel(account, row),
			"updated": accountChanged,
			"message": map[bool]string{true: "已同步", false: "无需更新"}[accountChanged],
		})
	}
	result.Changed = changed
	result.UpdatedAccounts = updates
	if changed {
		cfg.Sites[siteIdx] = site
		if err := s.saveConfigLocked(ctx, cfg); err != nil {
			return NewAPICheckinRefreshSiteResult{}, err
		}
	}
	return result, nil
}

// RunFullCheckin 同步执行一次全量签到，跳过禁用站点并保留旧 latest。
func (s *NewAPICheckinService) RunFullCheckin(ctx context.Context) (NewAPICheckinReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg, err := s.loadConfigLocked(ctx)
	if err != nil {
		return NewAPICheckinReport{}, err
	}
	tasks := buildNewAPICheckinTasks(cfg, false)
	report := NewAPICheckinReport{
		StartedAt: s.nowText(),
		Source:    "page-full-checkin",
	}
	cache, err := s.loadBalanceCacheLocked(ctx)
	if err != nil {
		return NewAPICheckinReport{}, err
	}
	siteStatuses := cache.SiteStatuses
	if siteStatuses == nil {
		siteStatuses = map[string]NewAPICheckinSiteStatus{}
	}
	refreshedRows := make([]NewAPICheckinBalanceAccount, 0, len(tasks))
	for idx, task := range tasks {
		status := normalizeNewAPISiteStatus(siteStatuses[task.site.Name])
		if status.QuotaPerUnit == 0 || status.Message == "本地默认折算" {
			status = s.querySiteStatusLocked(ctx, task.site)
			siteStatuses[task.site.Name] = status
		}
		checkin := s.queryCheckinStatusLocked(ctx, task.site, task.account, status)
		checkin, err = s.applyMonthlyAwardToCheckinLocked(ctx, task.site, task.account, checkin, status)
		if err != nil {
			return NewAPICheckinReport{}, err
		}
		self := s.queryAccountSelfLocked(ctx, task.site, task.account, status)
		row := s.buildBalanceAccount(task.site, task.account, self, checkin, status, s.nowText())
		refreshedRows = append(refreshedRows, row)
		report.AccountResults = append(report.AccountResults, s.accountResultFromBalanceRow(row, checkin, self, status))
		if idx < len(tasks)-1 && cfg.DelayBetweenCheckinsSec > 0 {
			s.mu.Unlock()
			select {
			case <-ctx.Done():
				s.mu.Lock()
				return NewAPICheckinReport{}, ctx.Err()
			case <-time.After(time.Duration(cfg.DelayBetweenCheckinsSec) * time.Second):
			}
			s.mu.Lock()
		}
	}
	report.EndedAt = s.nowText()
	report = s.rebuildReportLocked(report)
	if len(report.AccountResults) == 0 {
		return report, nil
	}
	cache.SiteStatuses = siteStatuses
	cache.Accounts = replaceNewAPIBalanceRows(cache.Accounts, refreshedRows)
	cache.GeneratedAt = report.EndedAt
	cache.Source = "page-full-checkin"
	cache = s.rebuildBalancePayloadLocked(cache)
	if err := s.saveBalanceCacheLocked(ctx, cache); err != nil {
		return NewAPICheckinReport{}, err
	}
	if err := s.saveReportLocked(ctx, report); err != nil {
		return NewAPICheckinReport{}, err
	}
	if err := s.upsertHistoryEntriesLocked(ctx, s.historyEntriesFromReport(report)); err != nil {
		return NewAPICheckinReport{}, err
	}
	if err := s.upsertMonthlyFromReportLocked(ctx, report, "page-full-checkin"); err != nil {
		return NewAPICheckinReport{}, err
	}
	return report, nil
}

// StartFullCheckinJob 后台启动全量签到任务。
func (s *NewAPICheckinService) StartFullCheckinJob(ctx context.Context) NewAPICheckinStartJobResult {
	s.mu.Lock()
	if s.checkinJobState.Running {
		job := s.checkinJobState
		s.mu.Unlock()
		return NewAPICheckinStartJobResult{Started: false, Job: job}
	}
	s.checkinJobState = NewAPICheckinJobState{
		Running:   true,
		StartedAt: s.nowText(),
		Message:   "全量签到已进入后台队列",
	}
	job := s.checkinJobState
	s.mu.Unlock()

	jobCtx := context.WithoutCancel(ctx)
	go func() {
		report, err := s.RunFullCheckin(jobCtx)
		s.mu.Lock()
		defer s.mu.Unlock()
		exitCode := 0
		if err != nil {
			exitCode = -1
			s.checkinJobState.Stderr = err.Error()
			s.checkinJobState.Message = "全量签到失败"
		} else if report.FailedCount > 0 {
			s.checkinJobState.Message = "全量签到完成但存在失败"
		} else {
			s.checkinJobState.Message = "全量签到完成"
		}
		s.checkinJobState.Running = false
		s.checkinJobState.EndedAt = s.nowText()
		s.checkinJobState.ExitCode = &exitCode
		s.checkinJobState.Report = report
	}()
	return NewAPICheckinStartJobResult{Started: true, Job: job}
}

// RunSingleCheckin 同步执行单账号补签。
func (s *NewAPICheckinService) RunSingleCheckin(ctx context.Context, siteName, userID string) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg, err := s.loadConfigLocked(ctx)
	if err != nil {
		return nil, err
	}
	site, siteIdx := findNewAPISite(cfg, siteName)
	if siteIdx < 0 {
		return nil, fmt.Errorf("未找到站点: %s", siteName)
	}
	if isSub2APISite(site) {
		return nil, fmt.Errorf("%s 是只读数据源，不支持签到", siteName)
	}
	if !site.Enabled {
		reason := cleanNewAPIText(site.DisabledReason)
		if reason == "" {
			reason = "站点已禁用签到"
		}
		return nil, fmt.Errorf("%s 已禁用签到: %s", siteName, reason)
	}
	account, accountIdx := findNewAPIAccount(site, userID)
	if accountIdx < 0 {
		return nil, fmt.Errorf("未找到账号: %s/%s", siteName, userID)
	}
	status := s.querySiteStatusLocked(ctx, site)
	checkin := s.queryCheckinStatusLocked(ctx, site, account, status)
	self := s.queryAccountSelfLocked(ctx, site, account, status)
	row := s.buildBalanceAccount(site, account, self, checkin, status, s.nowText())
	report := s.rebuildReportLocked(NewAPICheckinReport{
		StartedAt:      s.nowText(),
		EndedAt:        s.nowText(),
		Source:         "page-single-checkin",
		AccountResults: []NewAPICheckinAccountResult{s.accountResultFromBalanceRow(row, checkin, self, status)},
	})
	cache, err := s.loadBalanceCacheLocked(ctx)
	if err != nil {
		return nil, err
	}
	cache.SiteStatuses[site.Name] = status
	cache.Accounts = replaceNewAPIBalanceRows(cache.Accounts, []NewAPICheckinBalanceAccount{row})
	cache.GeneratedAt = s.nowText()
	cache.Source = "single-checkin"
	balances := s.rebuildBalancePayloadLocked(cache)
	if err := s.saveBalanceCacheLocked(ctx, balances); err != nil {
		return nil, err
	}
	lastRun, err := s.mergeReportIntoLastRunLocked(ctx, report)
	if err != nil {
		return nil, err
	}
	if err := s.upsertHistoryEntriesLocked(ctx, s.historyEntriesFromReport(report)); err != nil {
		return nil, err
	}
	if err := s.upsertMonthlyFromReportLocked(ctx, report, "page-single-checkin"); err != nil {
		return nil, err
	}
	history, err := s.buildHistoryPayloadLocked(ctx)
	if err != nil {
		return nil, err
	}
	monthly, err := s.buildMonthlyPayloadLocked(ctx, s.currentMonth(), siteName, userID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"generated_at": s.nowText(),
		"exit_code":    0,
		"stdout":       "",
		"stderr":       "",
		"log_path":     "",
		"report":       report,
		"history":      history,
		"balances":     balances,
		"last_run":     lastRun,
		"monthly":      monthly,
	}, nil
}

// StartMonthlySyncJob 后台启动月度记录同步。
func (s *NewAPICheckinService) StartMonthlySyncJob(ctx context.Context, month, siteName, userID string) NewAPICheckinStartMonthlyResult {
	s.mu.Lock()
	if s.monthlySyncState.Running {
		state := s.monthlySyncState
		s.mu.Unlock()
		return NewAPICheckinStartMonthlyResult{Started: false, SyncState: state}
	}
	month = normalizeNewAPIMonth(month, s.currentMonth())
	s.monthlySyncState = NewAPICheckinMonthlySyncState{
		Running:   true,
		Mode:      newAPICheckinMonthlySourceSync,
		Scope:     siteName,
		Month:     month,
		StartedAt: s.nowText(),
		Message:   "正在同步月度签到历史",
	}
	state := s.monthlySyncState
	s.mu.Unlock()

	jobCtx := context.WithoutCancel(ctx)
	go func() {
		_, err := s.syncMonthlyRecordsForScope(jobCtx, month, siteName, userID, newAPICheckinMonthlySourceSync)
		s.mu.Lock()
		defer s.mu.Unlock()
		s.monthlySyncState.Running = false
		s.monthlySyncState.EndedAt = s.nowText()
		if err != nil {
			s.monthlySyncState.Message = "月度同步失败"
			s.monthlySyncState.Error = err.Error()
		} else {
			s.monthlySyncState.Message = fmt.Sprintf("%s 同步完成", month)
		}
	}()
	return NewAPICheckinStartMonthlyResult{Started: true, SyncState: state}
}

func (s *NewAPICheckinService) refreshAccountBalanceLocked(ctx context.Context, siteName, userID string, refreshSiteStatus bool, source string) (NewAPICheckinRefreshAccountResult, error) {
	cfg, err := s.loadConfigLocked(ctx)
	if err != nil {
		return NewAPICheckinRefreshAccountResult{}, err
	}
	site, siteIdx := findNewAPISite(cfg, siteName)
	if siteIdx < 0 {
		return NewAPICheckinRefreshAccountResult{}, fmt.Errorf("未找到站点: %s", siteName)
	}
	account, accountIdx := findNewAPIAccount(site, userID)
	if accountIdx < 0 {
		return NewAPICheckinRefreshAccountResult{}, fmt.Errorf("未找到账号: %s/%s", siteName, userID)
	}
	_ = accountIdx
	cache, err := s.loadBalanceCacheLocked(ctx)
	if err != nil {
		return NewAPICheckinRefreshAccountResult{}, err
	}
	if isSub2APISite(site) {
		return s.refreshSub2AccountBalanceLocked(ctx, site, account, cache, source)
	}
	status := normalizeNewAPISiteStatus(cache.SiteStatuses[site.Name])
	if refreshSiteStatus || status.Message == "本地默认折算" {
		status = s.querySiteStatusLocked(ctx, site)
		cache.SiteStatuses[site.Name] = status
	}
	checkin := s.queryCheckinStatusLocked(ctx, site, account, status)
	checkin, err = s.applyMonthlyAwardToCheckinLocked(ctx, site, account, checkin, status)
	if err != nil {
		return NewAPICheckinRefreshAccountResult{}, err
	}
	self := s.queryAccountSelfLocked(ctx, site, account, status)
	row := s.buildBalanceAccount(site, account, self, checkin, status, s.nowText())
	cache.Accounts = replaceNewAPIBalanceRows(cache.Accounts, []NewAPICheckinBalanceAccount{row})
	cache.GeneratedAt = s.nowText()
	cache.Source = fmt.Sprintf("%s:%s/%s", source, site.Name, account.UserID)
	balances := s.rebuildBalancePayloadLocked(cache)
	if err := s.saveBalanceCacheLocked(ctx, balances); err != nil {
		return NewAPICheckinRefreshAccountResult{}, err
	}
	report := s.rebuildReportLocked(NewAPICheckinReport{
		StartedAt:      s.nowText(),
		EndedAt:        s.nowText(),
		Source:         cache.Source,
		AccountResults: []NewAPICheckinAccountResult{s.accountResultFromBalanceRow(row, checkin, self, status)},
	})
	lastRun, err := s.mergeReportIntoLastRunLocked(ctx, report)
	if err != nil {
		return NewAPICheckinRefreshAccountResult{}, err
	}
	if err := s.upsertHistoryEntriesLocked(ctx, []NewAPICheckinHistoryEntry{s.historyEntryFromBalanceRow(row, s.nowText(), source)}); err != nil {
		return NewAPICheckinRefreshAccountResult{}, err
	}
	monthly, err := s.syncCurrentMonthWhenIdleLocked(ctx, site.Name, account.UserID, source)
	if err != nil {
		return NewAPICheckinRefreshAccountResult{}, err
	}
	history, err := s.buildHistoryPayloadLocked(ctx)
	if err != nil {
		return NewAPICheckinRefreshAccountResult{}, err
	}
	return NewAPICheckinRefreshAccountResult{
		GeneratedAt: s.nowText(),
		Site:        site.Name,
		UserID:      account.UserID,
		OK:          self.ok && checkin.CheckinOK,
		Message:     firstNonEmpty(checkin.CheckinMessage, self.message),
		Checkin:     checkin,
		Account:     row,
		Balances:    balances,
		History:     history,
		LastRun:     lastRun,
		Monthly:     monthly,
	}, nil
}

func (s *NewAPICheckinService) refreshSiteBalancesLocked(ctx context.Context, siteName string, refreshSiteStatus bool, source string) (NewAPICheckinRefreshSiteResult, error) {
	cfg, err := s.loadConfigLocked(ctx)
	if err != nil {
		return NewAPICheckinRefreshSiteResult{}, err
	}
	site, siteIdx := findNewAPISite(cfg, siteName)
	if siteIdx < 0 {
		return NewAPICheckinRefreshSiteResult{}, fmt.Errorf("未找到站点: %s", siteName)
	}
	cache, err := s.loadBalanceCacheLocked(ctx)
	if err != nil {
		return NewAPICheckinRefreshSiteResult{}, err
	}
	if isSub2APISite(site) {
		return s.refreshSub2SiteBalancesLocked(ctx, site, cache, source)
	}
	status := normalizeNewAPISiteStatus(cache.SiteStatuses[site.Name])
	if refreshSiteStatus || status.Message == "本地默认折算" {
		status = s.querySiteStatusLocked(ctx, site)
		cache.SiteStatuses[site.Name] = status
	}
	refreshed := []NewAPICheckinBalanceAccount{}
	reportRows := []NewAPICheckinAccountResult{}
	successCount := 0
	alreadyDoneCount := 0
	failedCount := 0
	for _, account := range site.Accounts {
		if !account.Enabled {
			continue
		}
		checkin := s.queryCheckinStatusLocked(ctx, site, account, status)
		checkin, err = s.applyMonthlyAwardToCheckinLocked(ctx, site, account, checkin, status)
		if err != nil {
			return NewAPICheckinRefreshSiteResult{}, err
		}
		self := s.queryAccountSelfLocked(ctx, site, account, status)
		row := s.buildBalanceAccount(site, account, self, checkin, status, s.nowText())
		refreshed = append(refreshed, row)
		reportRows = append(reportRows, s.accountResultFromBalanceRow(row, checkin, self, status))
		if checkin.CheckinSuccess {
			successCount++
		} else if checkin.CheckedInToday {
			alreadyDoneCount++
		}
		if !self.ok || !checkin.CheckinOK {
			failedCount++
		}
	}
	cache.Accounts = replaceNewAPIBalanceRows(removeNewAPIBalanceRows(cache.Accounts, site.Name, ""), refreshed)
	cache.GeneratedAt = s.nowText()
	cache.Source = fmt.Sprintf("%s:%s", source, site.Name)
	balances := s.rebuildBalancePayloadLocked(cache)
	if err := s.saveBalanceCacheLocked(ctx, balances); err != nil {
		return NewAPICheckinRefreshSiteResult{}, err
	}
	report := s.rebuildReportLocked(NewAPICheckinReport{
		StartedAt:      s.nowText(),
		EndedAt:        s.nowText(),
		Source:         cache.Source,
		AccountResults: reportRows,
	})
	lastRun, err := s.mergeReportIntoLastRunLocked(ctx, report)
	if err != nil {
		return NewAPICheckinRefreshSiteResult{}, err
	}
	entries := make([]NewAPICheckinHistoryEntry, 0, len(refreshed))
	for _, row := range refreshed {
		entries = append(entries, s.historyEntryFromBalanceRow(row, s.nowText(), source))
	}
	if err := s.upsertHistoryEntriesLocked(ctx, entries); err != nil {
		return NewAPICheckinRefreshSiteResult{}, err
	}
	monthly, err := s.syncCurrentMonthWhenIdleLocked(ctx, site.Name, "", source)
	if err != nil {
		return NewAPICheckinRefreshSiteResult{}, err
	}
	history, err := s.buildHistoryPayloadLocked(ctx)
	if err != nil {
		return NewAPICheckinRefreshSiteResult{}, err
	}
	return NewAPICheckinRefreshSiteResult{
		GeneratedAt:      s.nowText(),
		Site:             site.Name,
		AccountCount:     len(refreshed),
		SuccessCount:     successCount,
		AlreadyDoneCount: alreadyDoneCount,
		FailedCount:      failedCount,
		Accounts:         refreshed,
		Balances:         balances,
		History:          history,
		LastRun:          lastRun,
		Monthly:          monthly,
	}, nil
}

// refreshSub2AccountBalanceLocked 只读取 sub2api 的用量和模型，不写入任何签到记录。
func (s *NewAPICheckinService) refreshSub2AccountBalanceLocked(ctx context.Context, site NewAPICheckinSite, account NewAPICheckinAccount, cache NewAPICheckinBalancePayload, source string) (NewAPICheckinRefreshAccountResult, error) {
	row, ok := s.querySub2AccountLocked(ctx, site, account)
	cache.SiteStatuses[site.Name] = normalizeNewAPISiteStatus(site.SiteStatus)
	cache.Accounts = replaceNewAPIBalanceRows(cache.Accounts, []NewAPICheckinBalanceAccount{row})
	cache.GeneratedAt = s.nowText()
	cache.Source = fmt.Sprintf("%s:%s/%s", source, site.Name, account.UserID)
	balances := s.rebuildBalancePayloadLocked(cache)
	if err := s.saveBalanceCacheLocked(ctx, balances); err != nil {
		return NewAPICheckinRefreshAccountResult{}, err
	}
	history, err := s.buildHistoryPayloadLocked(ctx)
	if err != nil {
		return NewAPICheckinRefreshAccountResult{}, err
	}
	lastRun, err := s.loadReportLocked(ctx)
	if err != nil {
		return NewAPICheckinRefreshAccountResult{}, err
	}
	monthly, err := s.buildMonthlyPayloadLocked(ctx, s.currentMonth(), site.Name, account.UserID)
	if err != nil {
		return NewAPICheckinRefreshAccountResult{}, err
	}
	return NewAPICheckinRefreshAccountResult{
		GeneratedAt: s.nowText(),
		Site:        site.Name,
		UserID:      account.UserID,
		OK:          ok,
		Message:     row.Message,
		Account:     row,
		Balances:    balances,
		History:     history,
		LastRun:     s.rebuildReportLocked(lastRun),
		Monthly:     monthly,
	}, nil
}

// refreshSub2SiteBalancesLocked 批量刷新 sub2api 账号，同时保持签到历史和月度缓存不变。
func (s *NewAPICheckinService) refreshSub2SiteBalancesLocked(ctx context.Context, site NewAPICheckinSite, cache NewAPICheckinBalancePayload, source string) (NewAPICheckinRefreshSiteResult, error) {
	refreshed := make([]NewAPICheckinBalanceAccount, 0, len(site.Accounts))
	failedCount := 0
	for _, account := range site.Accounts {
		if !account.Enabled {
			continue
		}
		row, ok := s.querySub2AccountLocked(ctx, site, account)
		if !ok {
			failedCount++
		}
		refreshed = append(refreshed, row)
	}
	cache.SiteStatuses[site.Name] = normalizeNewAPISiteStatus(site.SiteStatus)
	cache.Accounts = replaceNewAPIBalanceRows(removeNewAPIBalanceRows(cache.Accounts, site.Name, ""), refreshed)
	cache.GeneratedAt = s.nowText()
	cache.Source = fmt.Sprintf("%s:%s", source, site.Name)
	balances := s.rebuildBalancePayloadLocked(cache)
	if err := s.saveBalanceCacheLocked(ctx, balances); err != nil {
		return NewAPICheckinRefreshSiteResult{}, err
	}
	history, err := s.buildHistoryPayloadLocked(ctx)
	if err != nil {
		return NewAPICheckinRefreshSiteResult{}, err
	}
	lastRun, err := s.loadReportLocked(ctx)
	if err != nil {
		return NewAPICheckinRefreshSiteResult{}, err
	}
	monthly, err := s.buildMonthlyPayloadLocked(ctx, s.currentMonth(), site.Name, "")
	if err != nil {
		return NewAPICheckinRefreshSiteResult{}, err
	}
	return NewAPICheckinRefreshSiteResult{
		GeneratedAt:  s.nowText(),
		Site:         site.Name,
		AccountCount: len(refreshed),
		FailedCount:  failedCount,
		Accounts:     refreshed,
		Balances:     balances,
		History:      history,
		LastRun:      s.rebuildReportLocked(lastRun),
		Monthly:      monthly,
	}, nil
}

// querySub2AccountLocked 合并 sub2api 的订阅用量和 Key 可用模型两个只读接口。
func (s *NewAPICheckinService) querySub2AccountLocked(ctx context.Context, site NewAPICheckinSite, account NewAPICheckinAccount) (NewAPICheckinBalanceAccount, bool) {
	headers := map[string]string{"Authorization": fmt.Sprintf("Bearer %s", account.AccessKey)}
	usage := s.requestJSONLocked(ctx, http.MethodGet, joinNewAPIURL(site.BaseURL, "/v1/usage?days=30"), site.BaseURL, headers)
	modelsResult := s.requestJSONLocked(ctx, http.MethodGet, joinNewAPIURL(site.BaseURL, "/v1/models"), site.BaseURL, headers)

	models := make([]any, 0)
	for _, raw := range sliceFromAny(modelsResult.payload["data"]) {
		if modelID := cleanNewAPIText(mapFromAny(raw)["id"]); modelID != "" {
			models = append(models, modelID)
		}
	}
	planName := cleanNewAPIText(usage.payload["planName"])
	remaining, hasRemaining := optionalFloat64FromAny(usage.payload["remaining"])
	usageSummary := mapFromAny(usage.payload["usage"])
	totalUsage := mapFromAny(usageSummary["total"])
	used, hasUsed := optionalFloat64FromAny(totalUsage["cost"])
	providerData := map[string]any{
		"plan_name":    planName,
		"mode":         cleanNewAPIText(usage.payload["mode"]),
		"unit":         firstNonEmpty(cleanNewAPIText(usage.payload["unit"]), "USD"),
		"is_valid":     boolFromAny(usage.payload["isValid"], false),
		"subscription": mapFromAny(usage.payload["subscription"]),
		"usage":        usageSummary,
		"daily_usage":  sliceFromAny(usage.payload["daily_usage"]),
		"model_stats":  sliceFromAny(usage.payload["model_stats"]),
		"model_count":  float64(len(models)),
		"models":       models,
	}
	providerData["expires_at"] = cleanNewAPIText(mapFromAny(providerData["subscription"])["expires_at"])
	ok := usage.ok && modelsResult.ok && boolFromAny(usage.payload["isValid"], true)
	message := "sub2api 套餐、用量和模型读取成功"
	if !ok {
		message = firstNonEmpty(usage.message, modelsResult.message, "sub2api 数据读取失败")
	}
	row := NewAPICheckinBalanceAccount{
		Site:            site.Name,
		Provider:        newAPICheckinProviderSub2API,
		ProviderData:    providerData,
		Enabled:         site.Enabled,
		Account:         account.Name,
		Label:           firstNonEmpty(account.Name, account.UserID),
		UserID:          account.UserID,
		IPProfile:       account.IPProfile,
		Status:          map[bool]string{true: "只读数据正常", false: "只读数据失败"}[ok],
		Message:         message,
		LastRefreshedAt: s.nowText(),
	}
	status := normalizeNewAPISiteStatus(site.SiteStatus)
	if hasRemaining {
		quota := sub2AmountToQuota(remaining, status)
		row.Quota = &quota
		row.QuotaDisplay = formatSub2Amount(remaining, status)
	}
	if hasUsed {
		usedQuota := sub2AmountToQuota(used, status)
		row.UsedQuota = &usedQuota
		row.UsedQuotaDisplay = formatSub2Amount(used, status)
	}
	return row, ok
}

func (s *NewAPICheckinService) syncMonthlyRecordsForScope(ctx context.Context, month, siteName, userID, source string) (NewAPICheckinMonthlyPayload, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	month = normalizeNewAPIMonth(month, s.currentMonth())
	cfg, err := s.loadConfigLocked(ctx)
	if err != nil {
		return NewAPICheckinMonthlyPayload{}, err
	}
	targets := buildNewAPIMonthlyTasks(cfg, siteName, userID)
	s.monthlySyncState.Running = true
	s.monthlySyncState.Month = month
	s.monthlySyncState.Total = len(targets)
	inserted := 0
	for idx, task := range targets {
		status := normalizeNewAPISiteStatus(task.site.SiteStatus)
		records := s.queryMonthlyRecordsLocked(ctx, task.site, task.account, month, status, source)
		if err := s.upsertMonthlyRecordsLocked(ctx, records); err != nil {
			return NewAPICheckinMonthlyPayload{}, err
		}
		inserted += len(records)
		s.monthlySyncState.Processed = idx + 1
		s.monthlySyncState.LastSite = task.site.Name
		s.monthlySyncState.LastUserID = task.account.UserID
		s.monthlySyncState.Message = fmt.Sprintf("正在同步 %s / %s 的 %s", task.site.Name, task.account.UserID, month)
	}
	s.monthlySyncState.Message = fmt.Sprintf("%s 同步完成，写入 %d 条月度记录", month, inserted)
	return s.buildMonthlyPayloadLocked(ctx, month, siteName, userID)
}

func (s *NewAPICheckinService) syncCurrentMonthWhenIdleLocked(ctx context.Context, siteName, userID, source string) (NewAPICheckinMonthlyPayload, error) {
	if s.monthlySyncState.Running {
		return s.buildMonthlyPayloadLocked(ctx, s.currentMonth(), siteName, userID)
	}
	return s.buildMonthlyPayloadLocked(ctx, s.currentMonth(), siteName, userID)
}

func (s *NewAPICheckinService) querySiteStatusLocked(ctx context.Context, site NewAPICheckinSite) NewAPICheckinSiteStatus {
	result := s.requestJSONLocked(ctx, http.MethodGet, joinNewAPIURL(site.BaseURL, "/api/status"), site.BaseURL, nil)
	status := normalizeNewAPISiteStatus(site.SiteStatus)
	if result.ok {
		data := mapFromAny(result.payload["data"])
		status.OK = true
		status.Message = firstNonEmpty(result.message, "站点状态已刷新")
		if v := cleanNewAPIText(data["quota_display_type"]); v != "" {
			status.QuotaDisplayType = v
		}
		if v := int64FromAny(data["quota_per_unit"]); v > 0 {
			status.QuotaPerUnit = v
		}
		if v := cleanNewAPIText(data["custom_currency_symbol"]); v != "" {
			status.CustomCurrencySymbol = v
		}
		return status
	}
	status.OK = false
	status.Message = firstNonEmpty(result.message, "站点状态查询失败")
	return status
}

func (s *NewAPICheckinService) queryAccountSelfLocked(ctx context.Context, site NewAPICheckinSite, account NewAPICheckinAccount, status NewAPICheckinSiteStatus) newAPICheckinAPIResult {
	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", account.AccessKey),
		"New-Api-User":  account.UserID,
	}
	result := s.requestJSONLocked(ctx, http.MethodGet, joinNewAPIURL(site.BaseURL, "/api/user/self"), site.BaseURL, headers)
	if result.payload == nil {
		result.payload = map[string]any{}
	}
	data := mapFromAny(result.payload["data"])
	result.payload["username"] = cleanNewAPIText(data["username"])
	result.payload["display_name"] = cleanNewAPIText(data["display_name"])
	result.payload["email"] = cleanNewAPIText(data["email"])
	if quota, ok := optionalInt64FromAny(data["quota"]); ok {
		result.payload["quota"] = quota
		result.payload["quota_display"] = formatNewAPIDisplayAmount(&quota, status)
	}
	if used, ok := optionalInt64FromAny(data["used_quota"]); ok {
		result.payload["used_quota"] = used
		result.payload["used_quota_display"] = formatNewAPIDisplayAmount(&used, status)
	}
	return result
}

func (s *NewAPICheckinService) queryAPIKeysLocked(ctx context.Context, site NewAPICheckinSite, account NewAPICheckinAccount) NewAPICheckinAccountAPIKeySummary {
	summary := NewAPICheckinAccountAPIKeySummary{
		Site:            site.Name,
		Provider:        normalizeNewAPIProvider(site.Provider),
		UserID:          account.UserID,
		Status:          "missing",
		GroupStatus:     "error",
		AvailableGroups: []string{},
		APIKeys:         []NewAPICheckinAPIKeySummary{},
	}
	if isSub2APISite(site) {
		if cleanNewAPIText(account.LoginUsername) == "" || account.LoginPassword == "" {
			summary.Status = "unsupported"
			summary.Message = "尚未保存 sub2api 登录凭据"
			summary.GroupStatus = "unsupported"
			summary.GroupMessage = "填写登录账号和密码后可读取 Key 与全部可用分组"
			return summary
		}
		session, err := s.loginSub2AccountLocked(ctx, site, account)
		if err != nil {
			summary.Status = "error"
			summary.Message = err.Error()
			summary.GroupMessage = err.Error()
			return summary
		}
		return buildSub2APIKeySummary(site, account, session)
	}
	headers := newAPICheckinAccountHeaders(account)
	result := s.requestJSONLocked(ctx, http.MethodGet, joinNewAPIURL(site.BaseURL, "/api/token/?p=1&size=100"), site.BaseURL, headers)
	if !result.ok {
		summary.Status = "error"
		summary.Message = firstNonEmpty(result.message, "API Key 读取失败")
		return summary
	}

	groupOptions := map[string]*float64{}
	data := mapFromAny(result.payload["data"])
	for _, raw := range sliceFromAny(data["items"]) {
		item := mapFromAny(raw)
		masked := maskNewAPIKey(cleanNewAPIText(item["key"]))
		if masked == "" {
			continue
		}
		groupName := cleanNewAPIText(item["group"])
		addNewAPIGroupOption(groupOptions, groupName, nil)
		summary.APIKeys = append(summary.APIKeys, NewAPICheckinAPIKeySummary{
			ID:        int64FromAny(item["id"]),
			Name:      firstNonEmpty(cleanNewAPIText(item["name"]), "未命名"),
			MaskedKey: masked,
			Group:     groupName,
			MatchKey:  normalizeNewAPIFullKey(cleanNewAPIText(item["key"])),
		})
	}
	self := s.requestJSONLocked(ctx, http.MethodGet, joinNewAPIURL(site.BaseURL, "/api/user/self"), site.BaseURL, headers)
	if self.ok {
		selfData := mapFromAny(self.payload["data"])
		if selfData == nil {
			selfData = self.payload
		}
		addNewAPIGroupOption(groupOptions, cleanNewAPIText(selfData["group"]), nil)
	}
	groupLoaded := false
	groupMessage := ""
	// NewAPI 新版从 self/groups 返回用户可用分组及倍率，旧版和衍生站点可能只开放其余端点。
	for _, endpoint := range []string{"/api/user/self/groups", "/api/user/groups", "/api/pricing", "/api/user/available_groups"} {
		groups := s.requestJSONLocked(ctx, http.MethodGet, joinNewAPIURL(site.BaseURL, endpoint), site.BaseURL, headers)
		if !groups.ok {
			groupMessage = firstNonEmpty(groupMessage, groups.message)
			continue
		}
		groupLoaded = true
		collectNewAPIGroupOptions(groups.payload["data"], groupOptions)
		collectNewAPIGroupOptions(groups.payload["group_ratio"], groupOptions)
	}
	if groupLoaded {
		summary.GroupStatus = "ready"
		summary.GroupMessage = "已读取上游完整可用分组"
	} else if len(groupOptions) > 0 {
		summary.GroupStatus = "partial"
		summary.GroupMessage = "上游完整分组接口未授权，当前仅展示账号和 token 已知分组"
	} else {
		summary.GroupMessage = firstNonEmpty(groupMessage, "上游分组读取失败")
	}
	summary.AvailableGroupOptions = sortedNewAPIGroupOptions(groupOptions)
	for _, option := range summary.AvailableGroupOptions {
		summary.AvailableGroups = append(summary.AvailableGroups, option.Name)
	}
	if len(summary.APIKeys) > 0 {
		summary.Status = "ready"
	}
	return summary
}

// probeSub2LoginLocked 验证登录后同时返回数量和可持久化的最新 Key/分组摘要，响应中不包含 JWT 或密码。
func (s *NewAPICheckinService) probeSub2LoginLocked(ctx context.Context, site NewAPICheckinSite, account NewAPICheckinAccount, loginUsername, loginPassword string) (NewAPICheckinLoginCredentialResult, NewAPICheckinAccountAPIKeySummary, error) {
	if !isSub2APISite(site) {
		return NewAPICheckinLoginCredentialResult{}, NewAPICheckinAccountAPIKeySummary{}, fmt.Errorf("%s 不是 sub2api 站点", site.Name)
	}
	account.LoginUsername = loginUsername
	account.LoginPassword = loginPassword
	session, err := s.loginSub2AccountLocked(ctx, site, account)
	if err != nil {
		return NewAPICheckinLoginCredentialResult{}, NewAPICheckinAccountAPIKeySummary{}, err
	}
	result := NewAPICheckinLoginCredentialResult{
		Site: site.Name, UserID: account.UserID, LoginUsername: loginUsername,
		HasLoginPassword: loginPassword != "", LoginOK: true,
		APIKeyCount: len(session.keys), GroupCount: len(sub2GroupOptions(session.groups)), Message: "登录验证成功",
	}
	return result, buildSub2APIKeySummary(site, account, session), nil
}

// buildSub2APIKeySummary 把一次登录读取到的 Key 与分组转换成页面和数据库共用的账号摘要。
func buildSub2APIKeySummary(site NewAPICheckinSite, account NewAPICheckinAccount, session newAPICheckinSub2Session) NewAPICheckinAccountAPIKeySummary {
	summary := NewAPICheckinAccountAPIKeySummary{
		Site: site.Name, Provider: normalizeNewAPIProvider(site.Provider), UserID: account.UserID,
		Status: "missing", GroupStatus: "ready", AvailableGroups: []string{}, APIKeys: []NewAPICheckinAPIKeySummary{},
	}
	options := sub2GroupOptions(session.groups)
	for _, option := range options {
		summary.AvailableGroups = append(summary.AvailableGroups, option.Name)
	}
	summary.AvailableGroupOptions = options
	for _, raw := range session.keys {
		item := mapFromAny(raw)
		key := normalizeNewAPIFullKey(cleanNewAPIText(item["key"]))
		if key == "" {
			continue
		}
		groupID := int64FromAny(item["group_id"])
		summary.APIKeys = append(summary.APIKeys, NewAPICheckinAPIKeySummary{
			ID: int64FromAny(item["id"]), Name: firstNonEmpty(cleanNewAPIText(item["name"]), "未命名"),
			MaskedKey: maskNewAPIKey(key), Group: firstNonEmpty(sub2GroupNameFromKey(item), findSub2GroupName(session.groups, groupID)), GroupID: groupID,
			MatchKey: key,
		})
	}
	if len(summary.APIKeys) > 0 {
		summary.Status = "ready"
	}
	summary.GroupMessage = fmt.Sprintf("登录成功，已读取 %d 个可用分组", len(options))
	return summary
}

// loginSub2AccountLocked 使用账号密码临时换取 JWT，并读取当前用户的 Key 和可用分组。
func (s *NewAPICheckinService) loginSub2AccountLocked(ctx context.Context, site NewAPICheckinSite, account NewAPICheckinAccount) (newAPICheckinSub2Session, error) {
	loginUsername := cleanNewAPIText(account.LoginUsername)
	if loginUsername == "" || account.LoginPassword == "" {
		return newAPICheckinSub2Session{}, fmt.Errorf("未保存登录账号或密码")
	}
	login := s.requestJSONBodyLocked(ctx, http.MethodPost, joinNewAPIURL(site.BaseURL, "/api/v1/auth/login"), site.BaseURL, nil, map[string]any{
		"email": loginUsername, "password": account.LoginPassword,
	})
	if !login.ok {
		return newAPICheckinSub2Session{}, fmt.Errorf("登录失败: %s", firstNonEmpty(login.message, "上游拒绝登录"))
	}
	loginData := mapFromAny(login.payload["data"])
	if loginData == nil {
		loginData = login.payload
	}
	accessToken := cleanNewAPIText(loginData["access_token"])
	if accessToken == "" {
		return newAPICheckinSub2Session{}, fmt.Errorf("登录成功但上游未返回 access_token")
	}
	headers := sub2BearerHeaders(accessToken)
	keysResult := s.requestJSONLocked(ctx, http.MethodGet, joinNewAPIURL(site.BaseURL, "/api/v1/keys"), site.BaseURL, headers)
	if !keysResult.ok {
		return newAPICheckinSub2Session{}, fmt.Errorf("读取 API Key 失败: %s", firstNonEmpty(keysResult.message, "上游拒绝请求"))
	}
	keysData := mapFromAny(keysResult.payload["data"])
	keys := sliceFromAny(keysData["items"])
	groupsResult := s.requestJSONLocked(ctx, http.MethodGet, joinNewAPIURL(site.BaseURL, "/api/v1/groups/available"), site.BaseURL, headers)
	if !groupsResult.ok {
		return newAPICheckinSub2Session{}, fmt.Errorf("读取可用分组失败: %s", firstNonEmpty(groupsResult.message, "上游拒绝请求"))
	}
	groups := sliceFromAny(groupsResult.payload["data"])
	return newAPICheckinSub2Session{accessToken: accessToken, keys: keys, groups: groups}, nil
}

func resolveNewAPILoginCredentials(account NewAPICheckinAccount, loginUsername, loginPassword string) (string, string, error) {
	loginUsername = firstNonEmpty(cleanNewAPIText(loginUsername), cleanNewAPIText(account.LoginUsername))
	if loginPassword == "" {
		loginPassword = account.LoginPassword
	}
	if loginUsername == "" || loginPassword == "" {
		return "", "", fmt.Errorf("登录账号和密码不能为空")
	}
	return loginUsername, loginPassword, nil
}

func sub2BearerHeaders(accessToken string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + accessToken}
}

func sub2GroupOptions(groups []any) []NewAPICheckinGroupOption {
	options := make([]NewAPICheckinGroupOption, 0, len(groups))
	seen := map[int64]struct{}{}
	for _, raw := range groups {
		group := mapFromAny(raw)
		id := int64FromAny(group["id"])
		name := cleanNewAPIText(group["name"])
		if id <= 0 || name == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		options = append(options, NewAPICheckinGroupOption{
			ID:             id,
			Name:           name,
			RateMultiplier: newAPIGroupRateMultiplier(group),
		})
	}
	return options
}

func findSub2GroupName(groups []any, groupID int64) string {
	for _, option := range sub2GroupOptions(groups) {
		if option.ID == groupID {
			return option.Name
		}
	}
	return ""
}

func sub2GroupNameFromKey(item map[string]any) string {
	if group := mapFromAny(item["group"]); group != nil {
		return cleanNewAPIText(group["name"])
	}
	return cleanNewAPIText(item["group"])
}

func findSub2APIKey(keys []any, apiKeyID int64) (map[string]any, error) {
	for _, raw := range keys {
		item := mapFromAny(raw)
		if int64FromAny(item["id"]) == apiKeyID {
			return item, nil
		}
	}
	return nil, fmt.Errorf("未找到上游 API Key: %d", apiKeyID)
}

func (s *NewAPICheckinService) findConfiguredAccountLocked(ctx context.Context, siteName, userID string) (NewAPICheckinSite, NewAPICheckinAccount, error) {
	cfg, err := s.loadConfigLocked(ctx)
	if err != nil {
		return NewAPICheckinSite{}, NewAPICheckinAccount{}, err
	}
	site, siteIdx := findNewAPISite(cfg, siteName)
	if siteIdx < 0 {
		return NewAPICheckinSite{}, NewAPICheckinAccount{}, fmt.Errorf("站点不存在: %s", siteName)
	}
	account, accountIdx := findNewAPIAccount(site, userID)
	if accountIdx < 0 {
		return NewAPICheckinSite{}, NewAPICheckinAccount{}, fmt.Errorf("账号不存在: %s / %s", siteName, userID)
	}
	return site, account, nil
}

func (s *NewAPICheckinService) findNewAPITokenLocked(ctx context.Context, site NewAPICheckinSite, account NewAPICheckinAccount, apiKeyID int64) (map[string]any, error) {
	result := s.requestJSONLocked(ctx, http.MethodGet, joinNewAPIURL(site.BaseURL, "/api/token/?p=1&size=100"), site.BaseURL, newAPICheckinAccountHeaders(account))
	if !result.ok {
		return nil, fmt.Errorf("API Key 读取失败: %s", firstNonEmpty(result.message, "上游拒绝请求"))
	}
	for _, raw := range sliceFromAny(mapFromAny(result.payload["data"])["items"]) {
		item := mapFromAny(raw)
		if int64FromAny(item["id"]) == apiKeyID {
			return item, nil
		}
	}
	return nil, fmt.Errorf("未找到上游 token: %d", apiKeyID)
}

func newAPICheckinAccountHeaders(account NewAPICheckinAccount) map[string]string {
	return map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", account.AccessKey),
		"New-Api-User":  account.UserID,
	}
}

func normalizeNewAPIFullKey(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "sk-") {
		return value
	}
	return "sk-" + value
}

func addNewAPIGroupOption(groups map[string]*float64, value string, rateMultiplier *float64) {
	value = cleanNewAPIText(value)
	if value == "" {
		return
	}
	if current, exists := groups[value]; !exists || (current == nil && rateMultiplier != nil) {
		groups[value] = rateMultiplier
	}
}

// collectNewAPIGroupOptions 兼容 NewAPI 的名称数组、分组对象数组和 group_ratio 映射。
func collectNewAPIGroupOptions(value any, groups map[string]*float64) {
	switch typed := value.(type) {
	case string:
		addNewAPIGroupOption(groups, typed, nil)
	case []any:
		for _, item := range typed {
			if row := mapFromAny(item); row != nil {
				addNewAPIGroupOption(
					groups,
					firstNonEmpty(cleanNewAPIText(row["name"]), cleanNewAPIText(row["group"]), cleanNewAPIText(row["value"])),
					newAPIGroupRateMultiplier(row),
				)
				continue
			}
			collectNewAPIGroupOptions(item, groups)
		}
	case map[string]any:
		for _, wrapper := range []string{"groups", "items", "available_groups", "group_ratio"} {
			if nested, ok := typed[wrapper]; ok {
				collectNewAPIGroupOptions(nested, groups)
				return
			}
		}
		if name := firstNonEmpty(cleanNewAPIText(typed["name"]), cleanNewAPIText(typed["group"]), cleanNewAPIText(typed["value"])); name != "" {
			addNewAPIGroupOption(groups, name, newAPIGroupRateMultiplier(typed))
			return
		}
		for name, rawRate := range typed {
			addNewAPIGroupOption(groups, name, newAPIGroupRateMultiplier(rawRate))
		}
	}
}

func sortedNewAPIGroupOptions(groups map[string]*float64) []NewAPICheckinGroupOption {
	out := make([]NewAPICheckinGroupOption, 0, len(groups))
	for name, rateMultiplier := range groups {
		out = append(out, NewAPICheckinGroupOption{Name: name, RateMultiplier: rateMultiplier})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func newAPIGroupRateMultiplier(value any) *float64 {
	if row := mapFromAny(value); row != nil {
		for _, field := range []string{"rate_multiplier", "ratio", "group_ratio"} {
			if rateMultiplier := newAPIGroupRateMultiplier(row[field]); rateMultiplier != nil {
				return rateMultiplier
			}
		}
		return nil
	}
	rateMultiplier, ok := optionalFloat64FromAny(value)
	if !ok || rateMultiplier < 0 {
		return nil
	}
	return &rateMultiplier
}

func maskNewAPIKey(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "sk-")
	if len(value) < 7 {
		return ""
	}
	return "sk-" + value[:2] + "***" + value[len(value)-4:]
}

func (s *NewAPICheckinService) queryCheckinStatusLocked(ctx context.Context, site NewAPICheckinSite, account NewAPICheckinAccount, status NewAPICheckinSiteStatus) NewAPICheckinStatusResult {
	if !site.Enabled {
		reason := firstNonEmpty(site.DisabledReason, "站点已禁用签到")
		return NewAPICheckinStatusResult{
			CheckinOK:         true,
			CheckinSuccess:    false,
			CheckedInToday:    false,
			CheckinStatus:     "站点已禁用签到",
			CheckinStatusTone: "warn",
			CheckinMessage:    reason,
			CheckinDate:       s.todayText(),
		}
	}
	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", account.AccessKey),
		"New-Api-User":  account.UserID,
	}
	path := cleanNewAPIText(site.CheckinPath)
	if path == "" {
		path = newAPICheckinDefaultPath
	}
	result := s.requestJSONLocked(ctx, http.MethodPost, joinNewAPIURL(site.BaseURL, path), site.BaseURL, headers)
	data := mapFromAny(result.payload["data"])
	message := firstNonEmpty(result.message, extractNewAPIMessage(result.payload))
	success := boolFromAny(result.payload["success"], false)
	alreadyDone := strings.Contains(message, "已签到") || strings.Contains(strings.ToLower(message), "already")
	quota, hasQuota := optionalInt64FromAny(data["quota_awarded"])
	var quotaPtr *int64
	if hasQuota {
		quotaPtr = &quota
	}
	checkinDate := firstNonEmpty(cleanNewAPIText(data["checkin_date"]), s.todayText())
	statusText := "签到查询失败"
	tone := "danger"
	checked := false
	if success {
		statusText = "签到成功"
		tone = "ok"
		checked = true
	} else if alreadyDone {
		statusText = "今日已签到"
		tone = "warn"
		checked = true
	}
	display := formatNewAPIDisplayAmount(quotaPtr, status)
	return NewAPICheckinStatusResult{
		CheckinOK:                success || alreadyDone,
		CheckinSuccess:           success,
		CheckedInToday:           checked,
		CheckinStatus:            statusText,
		CheckinStatusTone:        tone,
		CheckinMessage:           firstNonEmpty(message, statusText),
		CheckinDate:              checkinDate,
		QuotaAwarded:             quotaPtr,
		QuotaAwardedDisplay:      display,
		QuotaAwardedDisplayValue: displayAmountValue(display),
	}
}

func (s *NewAPICheckinService) queryMonthlyRecordsLocked(ctx context.Context, site NewAPICheckinSite, account NewAPICheckinAccount, month string, status NewAPICheckinSiteStatus, source string) []NewAPICheckinMonthlyRecord {
	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", account.AccessKey),
		"New-Api-User":  account.UserID,
	}
	path := cleanNewAPIText(site.CheckinPath)
	if path == "" {
		path = newAPICheckinDefaultPath
	}
	result := s.requestJSONLocked(ctx, http.MethodGet, joinNewAPIURL(site.BaseURL, path)+"?month="+month, site.BaseURL, headers)
	data := mapFromAny(result.payload["data"])
	stats := mapFromAny(data["stats"])
	rawRecords := sliceFromAny(stats["records"])
	records := []NewAPICheckinMonthlyRecord{}
	for _, raw := range rawRecords {
		item := mapFromAny(raw)
		date := cleanNewAPIText(item["checkin_date"])
		if date == "" {
			continue
		}
		quota, hasQuota := optionalInt64FromAny(item["quota_awarded"])
		var quotaPtr *int64
		if hasQuota {
			quotaPtr = &quota
		}
		display := formatNewAPIDisplayAmount(quotaPtr, status)
		records = append(records, NewAPICheckinMonthlyRecord{
			Site:                     site.Name,
			UserID:                   account.UserID,
			AccountName:              account.Name,
			Username:                 account.Username,
			DisplayName:              account.DisplayName,
			IPProfile:                account.IPProfile,
			Month:                    month,
			CheckinDate:              date,
			QuotaAwarded:             quotaPtr,
			QuotaAwardedDisplay:      display,
			QuotaAwardedDisplayValue: displayAmountValue(display),
			FetchedAt:                s.nowText(),
			Source:                   source,
		})
	}
	return records
}

func (s *NewAPICheckinService) applyMonthlyAwardToCheckinLocked(ctx context.Context, site NewAPICheckinSite, account NewAPICheckinAccount, checkin NewAPICheckinStatusResult, status NewAPICheckinSiteStatus) (NewAPICheckinStatusResult, error) {
	if checkin.QuotaAwarded != nil && *checkin.QuotaAwarded != 0 {
		return checkin, nil
	}
	record, ok, err := s.monthlyRecordForCheckinLocked(ctx, site.Name, account.UserID, firstNonEmpty(checkin.CheckinDate, s.todayText()))
	if err != nil || !ok || record.QuotaAwarded == nil {
		return checkin, err
	}
	checkin.QuotaAwarded = record.QuotaAwarded
	checkin.QuotaAwardedDisplay = firstNonEmpty(normalizeNewAPIDisplayText(record.QuotaAwardedDisplay), formatNewAPIDisplayAmount(record.QuotaAwarded, status))
	checkin.CheckinOK = true
	checkin.CheckedInToday = true
	if checkin.CheckinStatus == "" || checkin.CheckinStatus == "签到查询失败" {
		checkin.CheckinStatus = "今日已签到"
		checkin.CheckinStatusTone = "warn"
	}
	if checkin.CheckinMessage == "" || checkin.CheckinMessage == "签到查询失败" {
		checkin.CheckinMessage = "月度记录已确认今日签到"
	}
	return checkin, nil
}

func (s *NewAPICheckinService) monthlyRecordForCheckinLocked(ctx context.Context, siteName, userID, checkinDate string) (NewAPICheckinMonthlyRecord, bool, error) {
	store, err := s.loadMonthlyStoreLocked(ctx)
	if err != nil {
		return NewAPICheckinMonthlyRecord{}, false, err
	}
	for _, record := range store.Records {
		if record.Site == siteName && record.UserID == userID && record.CheckinDate == checkinDate {
			return record, true, nil
		}
	}
	return NewAPICheckinMonthlyRecord{}, false, nil
}

func applyMonthlyAwardsToHistoryEntries(entries []NewAPICheckinHistoryEntry, records []NewAPICheckinMonthlyRecord) []NewAPICheckinHistoryEntry {
	monthlyByKey := map[string]NewAPICheckinMonthlyRecord{}
	for _, record := range records {
		monthlyByKey[record.Site+"\x00"+record.UserID+"\x00"+record.CheckinDate] = record
	}
	for idx := range entries {
		entry := &entries[idx]
		record, ok := monthlyByKey[entry.Site+"\x00"+entry.UserID+"\x00"+entry.Date]
		if !ok || record.QuotaAwarded == nil {
			continue
		}
		if entry.QuotaAwarded == nil || *entry.QuotaAwarded == 0 {
			entry.QuotaAwarded = record.QuotaAwarded
			entry.QuotaAwardedDisplay = firstNonEmpty(normalizeNewAPIDisplayText(record.QuotaAwardedDisplay), entry.QuotaAwardedDisplay)
			entry.QuotaAwardedDisplayValue = firstNonZeroFloat64(record.QuotaAwardedDisplayValue, displayAmountValue(entry.QuotaAwardedDisplay))
			entry.CheckedInToday = true
			if entry.CheckinStatus == "" || entry.CheckinStatus == "签到查询失败" {
				entry.CheckinStatus = "今日已签到"
				entry.CheckinStatusTone = "warn"
			}
			if entry.CheckinMessage == "" || entry.CheckinMessage == "签到查询失败" {
				entry.CheckinMessage = "月度记录已确认今日签到"
			}
		}
	}
	return entries
}

func firstNonZeroFloat64(values ...float64) float64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func (s *NewAPICheckinService) requestJSONLocked(ctx context.Context, method, url, baseURL string, headers map[string]string) newAPICheckinAPIResult {
	var body io.Reader
	if method == http.MethodPost {
		body = bytes.NewReader([]byte("{}"))
	}
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return newAPICheckinAPIResult{ok: false, message: err.Error(), payload: map[string]any{}}
	}
	req.Header.Set("User-Agent", "Sub2API-NewApi-Checkin/1.0")
	req.Header.Set("Accept", "application/json")
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	if baseURL != "" {
		req.Header.Set("Referer", strings.TrimRight(baseURL, "/")+"/")
		req.Header.Set("Origin", strings.TrimRight(baseURL, "/"))
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return newAPICheckinAPIResult{ok: false, message: err.Error(), payload: map[string]any{}}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	payload := map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &payload); err != nil {
			payload["message"] = string(raw)
		}
	}
	message := extractNewAPIMessage(payload)
	if resp.StatusCode >= 400 {
		return newAPICheckinAPIResult{ok: false, statusCode: resp.StatusCode, message: firstNonEmpty(message, string(raw), resp.Status), payload: payload}
	}
	if success, ok := payload["success"].(bool); ok && !success {
		return newAPICheckinAPIResult{ok: false, statusCode: resp.StatusCode, message: message, payload: payload}
	}
	return newAPICheckinAPIResult{ok: true, statusCode: resp.StatusCode, message: message, payload: payload}
}

// requestJSONBodyLocked 发送带 JSON 请求体的上游请求，并复用统一响应解析。
func (s *NewAPICheckinService) requestJSONBodyLocked(ctx context.Context, method, url, baseURL string, headers map[string]string, value any) newAPICheckinAPIResult {
	raw, err := json.Marshal(value)
	if err != nil {
		return newAPICheckinAPIResult{ok: false, message: err.Error(), payload: map[string]any{}}
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(raw))
	if err != nil {
		return newAPICheckinAPIResult{ok: false, message: err.Error(), payload: map[string]any{}}
	}
	req.Header.Set("User-Agent", "Sub2API-NewApi-Checkin/1.0")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	if baseURL != "" {
		req.Header.Set("Referer", strings.TrimRight(baseURL, "/")+"/")
		req.Header.Set("Origin", strings.TrimRight(baseURL, "/"))
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return newAPICheckinAPIResult{ok: false, message: err.Error(), payload: map[string]any{}}
	}
	defer resp.Body.Close()
	responseRaw, _ := io.ReadAll(resp.Body)
	payload := map[string]any{}
	if len(responseRaw) > 0 {
		if err := json.Unmarshal(responseRaw, &payload); err != nil {
			payload["message"] = string(responseRaw)
		}
	}
	message := extractNewAPIMessage(payload)
	if resp.StatusCode >= 400 {
		return newAPICheckinAPIResult{ok: false, statusCode: resp.StatusCode, message: firstNonEmpty(message, string(responseRaw), resp.Status), payload: payload}
	}
	if success, ok := payload["success"].(bool); ok && !success {
		return newAPICheckinAPIResult{ok: false, statusCode: resp.StatusCode, message: message, payload: payload}
	}
	return newAPICheckinAPIResult{ok: true, statusCode: resp.StatusCode, message: message, payload: payload}
}

func (s *NewAPICheckinService) buildBalanceAccount(site NewAPICheckinSite, account NewAPICheckinAccount, self newAPICheckinAPIResult, checkin NewAPICheckinStatusResult, status NewAPICheckinSiteStatus, ts string) NewAPICheckinBalanceAccount {
	quota, _ := optionalInt64FromAny(self.payload["quota"])
	used, _ := optionalInt64FromAny(self.payload["used_quota"])
	quotaPtr := pointerIfPresent(self.payload, "quota", quota)
	usedPtr := pointerIfPresent(self.payload, "used_quota", used)
	username := firstNonEmpty(cleanNewAPIText(self.payload["username"]), account.Username)
	displayName := firstNonEmpty(cleanNewAPIText(self.payload["display_name"]), account.DisplayName)
	row := NewAPICheckinBalanceAccount{
		Site:                site.Name,
		Enabled:             site.Enabled,
		Account:             account.Name,
		Username:            username,
		DisplayName:         displayName,
		UserID:              account.UserID,
		IPProfile:           account.IPProfile,
		Status:              firstNonEmpty(checkin.CheckinStatus, map[bool]string{true: "余额正常", false: "未刷新"}[self.ok]),
		Message:             firstNonEmpty(checkin.CheckinMessage, self.message, "本地缓存"),
		CheckinOK:           checkin.CheckinOK,
		CheckinSuccess:      checkin.CheckinSuccess,
		CheckedInToday:      checkin.CheckedInToday,
		CheckinStatus:       checkin.CheckinStatus,
		CheckinStatusTone:   checkin.CheckinStatusTone,
		CheckinMessage:      checkin.CheckinMessage,
		CheckinDate:         checkin.CheckinDate,
		Quota:               quotaPtr,
		QuotaDisplay:        formatNewAPIDisplayAmount(quotaPtr, status),
		UsedQuota:           usedPtr,
		UsedQuotaDisplay:    formatNewAPIDisplayAmount(usedPtr, status),
		QuotaAwarded:        checkin.QuotaAwarded,
		QuotaAwardedDisplay: checkin.QuotaAwardedDisplay,
		LastRefreshedAt:     ts,
	}
	row.Label = resolveNewAPIAccountLabel(account, row)
	return row
}

func (s *NewAPICheckinService) accountResultFromBalanceRow(row NewAPICheckinBalanceAccount, checkin NewAPICheckinStatusResult, self newAPICheckinAPIResult, status NewAPICheckinSiteStatus) NewAPICheckinAccountResult {
	return NewAPICheckinAccountResult{
		Site:                     row.Site,
		Account:                  row.Account,
		UserID:                   row.UserID,
		IPProfile:                row.IPProfile,
		OK:                       checkin.CheckinOK,
		Success:                  checkin.CheckinSuccess,
		Message:                  checkin.CheckinMessage,
		CheckinDate:              checkin.CheckinDate,
		QuotaAwarded:             checkin.QuotaAwarded,
		QuotaAwardedDisplay:      checkin.QuotaAwardedDisplay,
		QuotaAwardedDisplayValue: displayAmountValue(checkin.QuotaAwardedDisplay),
		RemainingQuota:           row.Quota,
		RemainingQuotaDisplay:    row.QuotaDisplay,
		UsedQuota:                row.UsedQuota,
		UsedQuotaDisplay:         row.UsedQuotaDisplay,
		Username:                 row.Username,
		DisplayName:              row.DisplayName,
		Status:                   checkin.CheckinStatus,
		CheckinStatus:            checkin.CheckinStatus,
		CheckinStatusTone:        checkin.CheckinStatusTone,
	}
}

func (s *NewAPICheckinService) historyEntryFromBalanceRow(row NewAPICheckinBalanceAccount, recordedAt, source string) NewAPICheckinHistoryEntry {
	return NewAPICheckinHistoryEntry{
		Date:                     firstNonEmpty(row.CheckinDate, datePart(recordedAt)),
		RecordedAt:               recordedAt,
		Source:                   source,
		Site:                     row.Site,
		Account:                  row.Label,
		UserID:                   row.UserID,
		IPProfile:                row.IPProfile,
		CheckinStatus:            row.CheckinStatus,
		CheckinStatusTone:        row.CheckinStatusTone,
		CheckinMessage:           row.CheckinMessage,
		CheckinSuccess:           boolFromAny(row.CheckinSuccess, false),
		CheckedInToday:           boolFromAny(row.CheckedInToday, false),
		QuotaAwarded:             row.QuotaAwarded,
		QuotaAwardedDisplay:      row.QuotaAwardedDisplay,
		QuotaAwardedDisplayValue: displayAmountValue(row.QuotaAwardedDisplay),
		Balance:                  row.Quota,
		BalanceDisplay:           row.QuotaDisplay,
		BalanceDisplayValue:      displayAmountValue(row.QuotaDisplay),
		UsedQuota:                row.UsedQuota,
		UsedDisplay:              row.UsedQuotaDisplay,
		UsedDisplayValue:         displayAmountValue(row.UsedQuotaDisplay),
	}
}

func (s *NewAPICheckinService) historyEntriesFromReport(report NewAPICheckinReport) []NewAPICheckinHistoryEntry {
	entries := make([]NewAPICheckinHistoryEntry, 0, len(report.AccountResults))
	recordedAt := firstNonEmpty(report.EndedAt, s.nowText())
	for _, row := range report.AccountResults {
		entries = append(entries, NewAPICheckinHistoryEntry{
			Date:                     firstNonEmpty(row.CheckinDate, datePart(recordedAt)),
			RecordedAt:               recordedAt,
			Source:                   firstNonEmpty(report.Source, "last-run"),
			Site:                     row.Site,
			Account:                  row.Account,
			UserID:                   row.UserID,
			IPProfile:                row.IPProfile,
			CheckinStatus:            row.CheckinStatus,
			CheckinStatusTone:        row.CheckinStatusTone,
			CheckinMessage:           row.Message,
			CheckinSuccess:           row.Success,
			CheckedInToday:           row.Success || strings.Contains(row.Message, "已签到"),
			QuotaAwarded:             row.QuotaAwarded,
			QuotaAwardedDisplay:      row.QuotaAwardedDisplay,
			QuotaAwardedDisplayValue: row.QuotaAwardedDisplayValue,
			Balance:                  row.RemainingQuota,
			BalanceDisplay:           row.RemainingQuotaDisplay,
			BalanceDisplayValue:      displayAmountValue(row.RemainingQuotaDisplay),
			UsedQuota:                row.UsedQuota,
			UsedDisplay:              row.UsedQuotaDisplay,
			UsedDisplayValue:         displayAmountValue(row.UsedQuotaDisplay),
		})
	}
	return entries
}

func (s *NewAPICheckinService) configSummaryLocked(cfg NewAPICheckinConfig) NewAPICheckinConfigSummary {
	sites := make([]NewAPICheckinConfigSiteSummary, 0, len(cfg.Sites))
	enabledSites := 0
	enabledAccounts := 0
	totalAccounts := 0
	for _, site := range cfg.Sites {
		accounts := make([]NewAPICheckinConfigAccountSummary, 0, len(site.Accounts))
		for _, account := range site.Accounts {
			if !account.Enabled {
				continue
			}
			accounts = append(accounts, NewAPICheckinConfigAccountSummary{
				Name:             account.Name,
				Username:         account.Username,
				DisplayName:      account.DisplayName,
				Label:            resolveNewAPIAccountLabel(account, NewAPICheckinBalanceAccount{}),
				UserID:           account.UserID,
				IPProfile:        account.IPProfile,
				AccessKey:        account.AccessKey,
				AccessKeyMasked:  maskNewAPIKey(account.AccessKey),
				LoginUsername:    account.LoginUsername,
				HasLoginPassword: account.LoginPassword != "",
			})
		}
		totalAccounts += len(accounts)
		if site.Enabled {
			enabledSites++
			enabledAccounts += len(accounts)
		}
		sites = append(sites, NewAPICheckinConfigSiteSummary{
			Name:           site.Name,
			Provider:       normalizeNewAPIProvider(site.Provider),
			Enabled:        site.Enabled,
			DisabledReason: site.DisabledReason,
			BaseURL:        site.BaseURL,
			AccountCount:   len(accounts),
			Accounts:       accounts,
		})
	}
	return NewAPICheckinConfigSummary{
		GeneratedAt:         s.nowText(),
		AllSiteCount:        len(sites),
		EnabledSiteCount:    enabledSites,
		AllAccountCount:     totalAccounts,
		EnabledAccountCount: enabledAccounts,
		Sites:               sites,
	}
}

func (s *NewAPICheckinService) rebuildReportLocked(report NewAPICheckinReport) NewAPICheckinReport {
	siteMap := map[string]*NewAPICheckinSiteRunSummary{}
	siteOrder := []string{}
	successCount := 0
	alreadyDoneCount := 0
	failedCount := 0
	var quotaTotal int64
	displayTotal := 0.0
	symbol := report.QuotaDisplaySymbol
	if symbol == "" {
		symbol = newAPICheckinDefaultSymbol
	}
	for idx := range report.AccountResults {
		row := &report.AccountResults[idx]
		row.QuotaAwardedDisplay = normalizeNewAPIDisplayText(row.QuotaAwardedDisplay)
		row.RemainingQuotaDisplay = normalizeNewAPIDisplayText(row.RemainingQuotaDisplay)
		row.UsedQuotaDisplay = normalizeNewAPIDisplayText(row.UsedQuotaDisplay)
		if row.CheckinStatus == "" {
			row.CheckinStatus = classifyNewAPICheckinStatus(*row).status
			row.CheckinStatusTone = classifyNewAPICheckinStatus(*row).tone
		}
		if row.Status == "" {
			row.Status = row.CheckinStatus
		}
		if _, ok := siteMap[row.Site]; !ok {
			siteOrder = append(siteOrder, row.Site)
			siteMap[row.Site] = &NewAPICheckinSiteRunSummary{Site: row.Site, DisplaySymbol: symbol}
		}
		summary := siteMap[row.Site]
		summary.TaskCount++
		if row.Success {
			successCount++
			summary.SuccessCount++
		} else if strings.Contains(row.Message, "已签到") || row.CheckinStatus == "今日已签到" {
			alreadyDoneCount++
			summary.AlreadyDoneCount++
		} else {
			failedCount++
			summary.FailedCount++
		}
		if row.QuotaAwarded != nil {
			quotaTotal += *row.QuotaAwarded
			summary.QuotaAwardedTotal += *row.QuotaAwarded
		}
		displayValue := row.QuotaAwardedDisplayValue
		if displayValue == 0 {
			displayValue = displayAmountValue(row.QuotaAwardedDisplay)
		}
		displayTotal += displayValue
		current := displayAmountValue(summary.QuotaDisplayTotal)
		summary.QuotaDisplayTotal = formatDisplayTotal(current+displayValue, symbol)
	}
	report.SiteCount = len(siteOrder)
	report.TaskCount = len(report.AccountResults)
	report.SuccessCount = successCount
	report.AlreadyDoneCount = alreadyDoneCount
	report.FailedCount = failedCount
	report.QuotaAwardedTotal = quotaTotal
	report.QuotaDisplaySymbol = symbol
	report.QuotaAwardedDisplay = formatDisplayTotal(displayTotal, symbol)
	report.SiteSummaries = make([]NewAPICheckinSiteRunSummary, 0, len(siteOrder))
	for _, site := range siteOrder {
		report.SiteSummaries = append(report.SiteSummaries, *siteMap[site])
	}
	report.SummaryText = buildNewAPISummaryText(report)
	return report
}

func (s *NewAPICheckinService) rebuildBalancePayloadLocked(cache NewAPICheckinBalancePayload) NewAPICheckinBalancePayload {
	siteMap := map[string]*NewAPICheckinBalanceSiteSummary{}
	siteOrder := []string{}
	displayTotals := map[string]map[string]any{}
	var quotaTotal, usedTotal, enabledQuota, enabledUsed int64
	for idx := range cache.Accounts {
		cache.Accounts[idx].QuotaDisplay = normalizeNewAPIDisplayText(cache.Accounts[idx].QuotaDisplay)
		cache.Accounts[idx].UsedQuotaDisplay = normalizeNewAPIDisplayText(cache.Accounts[idx].UsedQuotaDisplay)
		cache.Accounts[idx].QuotaAwardedDisplay = normalizeNewAPIDisplayText(cache.Accounts[idx].QuotaAwardedDisplay)
	}
	for _, row := range cache.Accounts {
		if _, ok := siteMap[row.Site]; !ok {
			siteOrder = append(siteOrder, row.Site)
			siteMap[row.Site] = &NewAPICheckinBalanceSiteSummary{Site: row.Site, Enabled: row.Enabled}
		}
		summary := siteMap[row.Site]
		summary.AccountCount++
		symbol := detectDisplaySymbol(row.QuotaDisplay, row.UsedQuotaDisplay)
		if symbol == "" {
			symbol = newAPICheckinDefaultSymbol
		}
		if _, ok := displayTotals[symbol]; !ok {
			displayTotals[symbol] = map[string]any{
				"symbol":              symbol,
				"quota_display_value": 0.0,
				"used_display_value":  0.0,
				"site_count":          0,
				"enabled_site_count":  0,
				"seen_sites":          map[string]bool{},
				"seen_enabled_sites":  map[string]bool{},
			}
		}
		total := displayTotals[symbol]
		seen := total["seen_sites"].(map[string]bool)
		if !seen[row.Site] {
			seen[row.Site] = true
			total["site_count"] = total["site_count"].(int) + 1
		}
		seenEnabled := total["seen_enabled_sites"].(map[string]bool)
		if row.Enabled && !seenEnabled[row.Site] {
			seenEnabled[row.Site] = true
			total["enabled_site_count"] = total["enabled_site_count"].(int) + 1
		}
		if row.Quota != nil {
			summary.QuotaTotal += *row.Quota
			quotaTotal += *row.Quota
			if row.Enabled {
				enabledQuota += *row.Quota
			}
			total["quota_display_value"] = total["quota_display_value"].(float64) + displayAmountValue(row.QuotaDisplay)
		}
		if row.UsedQuota != nil {
			summary.UsedQuotaTotal += *row.UsedQuota
			usedTotal += *row.UsedQuota
			if row.Enabled {
				enabledUsed += *row.UsedQuota
			}
			total["used_display_value"] = total["used_display_value"].(float64) + displayAmountValue(row.UsedQuotaDisplay)
		}
	}
	cache.SiteSummaries = make([]NewAPICheckinBalanceSiteSummary, 0, len(siteOrder))
	for _, site := range siteOrder {
		summary := siteMap[site]
		symbol := newAPICheckinDefaultSymbol
		for _, row := range cache.Accounts {
			if row.Site == site {
				symbol = firstNonEmpty(detectDisplaySymbol(row.QuotaDisplay, row.UsedQuotaDisplay), symbol)
				break
			}
		}
		summary.QuotaDisplayTotal = formatDisplayTotal(sumDisplayForSite(cache.Accounts, site, true), symbol)
		summary.UsedDisplayTotal = formatDisplayTotal(sumDisplayForSite(cache.Accounts, site, false), symbol)
		cache.SiteSummaries = append(cache.SiteSummaries, *summary)
	}
	displayRows := []map[string]any{}
	for symbol, item := range displayTotals {
		displayRows = append(displayRows, map[string]any{
			"symbol":              symbol,
			"quota_display_total": formatDisplayTotal(item["quota_display_value"].(float64), symbol),
			"used_display_total":  formatDisplayTotal(item["used_display_value"].(float64), symbol),
			"site_count":          item["site_count"],
			"enabled_site_count":  item["enabled_site_count"],
		})
	}
	sort.Slice(displayRows, func(i, j int) bool {
		return cleanNewAPIText(displayRows[i]["symbol"]) < cleanNewAPIText(displayRows[j]["symbol"])
	})
	cache.SiteCount = len(cache.SiteSummaries)
	cache.AccountCount = len(cache.Accounts)
	cache.Overall = NewAPICheckinBalanceOverall{
		QuotaTotal:            quotaTotal,
		UsedQuotaTotal:        usedTotal,
		EnabledQuotaTotal:     enabledQuota,
		EnabledUsedQuotaTotal: enabledUsed,
		DisplayTotals:         displayRows,
	}
	if cache.SiteStatuses == nil {
		cache.SiteStatuses = map[string]NewAPICheckinSiteStatus{}
	}
	if cache.GeneratedAt == "" {
		cache.GeneratedAt = s.nowText()
	}
	if cache.Source == "" {
		cache.Source = "config-seed"
	}
	return cache
}

func (s *NewAPICheckinService) mergeBalanceCacheWithConfigLocked(cache NewAPICheckinBalancePayload, cfg NewAPICheckinConfig) NewAPICheckinBalancePayload {
	configRows := map[string]NewAPICheckinBalanceAccount{}
	siteStatuses := cache.SiteStatuses
	if siteStatuses == nil {
		siteStatuses = map[string]NewAPICheckinSiteStatus{}
	}
	for _, site := range cfg.Sites {
		if _, ok := siteStatuses[site.Name]; !ok {
			siteStatuses[site.Name] = normalizeNewAPISiteStatus(site.SiteStatus)
		}
		for _, account := range site.Accounts {
			if !account.Enabled {
				continue
			}
			label := resolveNewAPIAccountLabel(account, NewAPICheckinBalanceAccount{})
			configRows[site.Name+"\x00"+account.UserID] = NewAPICheckinBalanceAccount{
				Site:             site.Name,
				Provider:         normalizeNewAPIProvider(site.Provider),
				Enabled:          site.Enabled,
				Account:          account.Name,
				Username:         account.Username,
				DisplayName:      account.DisplayName,
				Label:            label,
				UserID:           account.UserID,
				IPProfile:        account.IPProfile,
				Status:           "未刷新",
				Message:          "等待余额刷新",
				Quota:            nil,
				QuotaDisplay:     "",
				UsedQuota:        nil,
				UsedQuotaDisplay: "",
			}
		}
	}

	merged := make([]NewAPICheckinBalanceAccount, 0, len(configRows))
	seen := map[string]bool{}
	for _, row := range cache.Accounts {
		key := row.Site + "\x00" + row.UserID
		configRow, ok := configRows[key]
		if !ok {
			continue
		}
		row.Enabled = configRow.Enabled
		row.Provider = configRow.Provider
		row.Account = firstNonEmpty(configRow.Account, row.Account)
		row.Username = firstNonEmpty(row.Username, configRow.Username)
		row.DisplayName = firstNonEmpty(configRow.DisplayName, row.DisplayName)
		row.Label = firstNonEmpty(resolveNewAPIAccountLabel(NewAPICheckinAccount{
			Name:        row.Account,
			Username:    row.Username,
			DisplayName: row.DisplayName,
			UserID:      row.UserID,
		}, row), configRow.Label)
		row.IPProfile = firstNonEmpty(configRow.IPProfile, row.IPProfile)
		if row.Status == "" {
			row.Status = configRow.Status
		}
		if row.Message == "" {
			row.Message = configRow.Message
		}
		merged = append(merged, row)
		seen[key] = true
	}
	for key, row := range configRows {
		if !seen[key] {
			merged = append(merged, row)
		}
	}
	sort.Slice(merged, func(i, j int) bool {
		if merged[i].Site == merged[j].Site {
			return merged[i].UserID < merged[j].UserID
		}
		return merged[i].Site < merged[j].Site
	})
	cache.Accounts = merged
	cache.SiteStatuses = siteStatuses
	return cache
}

func (s *NewAPICheckinService) pruneBalanceCacheLocked(ctx context.Context, siteName, userID string) (int, error) {
	cache, err := s.loadBalanceCacheLocked(ctx)
	if err != nil {
		return 0, err
	}
	kept := make([]NewAPICheckinBalanceAccount, 0, len(cache.Accounts))
	removed := 0
	for _, row := range cache.Accounts {
		if newAPIMatchesScope(row.Site, row.UserID, siteName, userID) {
			removed++
			continue
		}
		kept = append(kept, row)
	}
	cache.Accounts = kept
	if userID == "" && cache.SiteStatuses != nil {
		delete(cache.SiteStatuses, siteName)
	}
	cache.GeneratedAt = s.nowText()
	cache.Source = "config-delete"
	if err := s.saveBalanceCacheLocked(ctx, s.rebuildBalancePayloadLocked(cache)); err != nil {
		return 0, err
	}
	return removed, nil
}

func (s *NewAPICheckinService) pruneHistoryLocked(ctx context.Context, siteName, userID string) (int, error) {
	payload, err := s.buildHistoryPayloadLocked(ctx)
	if err != nil {
		return 0, err
	}
	kept := make([]NewAPICheckinHistoryEntry, 0, len(payload.Entries))
	removed := 0
	for _, entry := range payload.Entries {
		if newAPIMatchesScope(entry.Site, entry.UserID, siteName, userID) {
			removed++
			continue
		}
		kept = append(kept, entry)
	}
	if s.repo == nil {
		return 0, errors.New("newapi checkin repository is not configured")
	}
	if err := s.repo.SaveHistory(ctx, s.buildHistoryPayloadFromEntriesLocked(kept)); err != nil {
		return 0, err
	}
	return removed, nil
}

func (s *NewAPICheckinService) pruneMonthlyLocked(ctx context.Context, siteName, userID string) (int, error) {
	store, err := s.loadMonthlyStoreLocked(ctx)
	if err != nil {
		return 0, err
	}
	kept := make([]NewAPICheckinMonthlyRecord, 0, len(store.Records))
	removed := 0
	for _, record := range store.Records {
		if newAPIMatchesScope(record.Site, record.UserID, siteName, userID) {
			removed++
			continue
		}
		kept = append(kept, record)
	}
	store.Records = kept
	if s.repo == nil {
		return 0, errors.New("newapi checkin repository is not configured")
	}
	if err := s.repo.SaveMonthlyRecords(ctx, store.Records); err != nil {
		return 0, err
	}
	return removed, nil
}

func (s *NewAPICheckinService) pruneLastRunLocked(ctx context.Context, siteName, userID string) (NewAPICheckinReport, error) {
	report, err := s.loadReportLocked(ctx)
	if err != nil {
		return NewAPICheckinReport{}, err
	}
	kept := make([]NewAPICheckinAccountResult, 0, len(report.AccountResults))
	for _, row := range report.AccountResults {
		if !newAPIMatchesScope(row.Site, row.UserID, siteName, userID) {
			kept = append(kept, row)
		}
	}
	report.AccountResults = kept
	report.EndedAt = firstNonEmpty(report.EndedAt, s.nowText())
	report = s.rebuildReportLocked(report)
	if err := s.saveReportLocked(ctx, report); err != nil {
		return NewAPICheckinReport{}, err
	}
	return report, nil
}

func (s *NewAPICheckinService) deleteResultLocked(ctx context.Context, cfg NewAPICheckinConfig, siteName, userID string, removedAccounts, removedCache, removedHistory, removedMonthly int, lastRun NewAPICheckinReport) (NewAPICheckinDeleteResult, error) {
	cache, err := s.loadBalanceCacheLocked(ctx)
	if err != nil {
		return NewAPICheckinDeleteResult{}, err
	}
	balances := s.rebuildBalancePayloadLocked(s.mergeBalanceCacheWithConfigLocked(cache, cfg))
	history, err := s.buildHistoryPayloadLocked(ctx)
	if err != nil {
		return NewAPICheckinDeleteResult{}, err
	}
	return NewAPICheckinDeleteResult{
		GeneratedAt:           s.nowText(),
		Site:                  siteName,
		UserID:                userID,
		RemovedAccounts:       removedAccounts,
		RemovedCacheAccounts:  removedCache,
		RemovedHistoryEntries: removedHistory,
		RemovedMonthlyEntries: removedMonthly,
		Config:                s.configSummaryLocked(cfg),
		Balances:              balances,
		History:               history,
		LastRun:               lastRun,
	}, nil
}

func (s *NewAPICheckinService) buildHistoryPayloadLocked(ctx context.Context) (NewAPICheckinHistoryPayload, error) {
	if s.repo == nil {
		return NewAPICheckinHistoryPayload{}, errors.New("newapi checkin repository is not configured")
	}
	payload, err := s.repo.LoadHistory(ctx)
	if err != nil {
		return NewAPICheckinHistoryPayload{}, err
	}
	monthlyStore, err := s.loadMonthlyStoreLocked(ctx)
	if err != nil {
		return NewAPICheckinHistoryPayload{}, err
	}
	payload.Entries = applyMonthlyAwardsToHistoryEntries(payload.Entries, monthlyStore.Records)
	return s.buildHistoryPayloadFromEntriesLocked(payload.Entries), nil
}

func (s *NewAPICheckinService) buildHistoryPayloadFromEntriesLocked(entries []NewAPICheckinHistoryEntry) NewAPICheckinHistoryPayload {
	for idx := range entries {
		entries[idx].QuotaAwardedDisplay = normalizeNewAPIDisplayText(entries[idx].QuotaAwardedDisplay)
		entries[idx].BalanceDisplay = normalizeNewAPIDisplayText(entries[idx].BalanceDisplay)
		entries[idx].UsedDisplay = normalizeNewAPIDisplayText(entries[idx].UsedDisplay)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Date == entries[j].Date {
			if entries[i].Site == entries[j].Site {
				return entries[i].UserID < entries[j].UserID
			}
			return entries[i].Site < entries[j].Site
		}
		return entries[i].Date < entries[j].Date
	})
	daily := aggregateHistory(entries, func(e NewAPICheckinHistoryEntry) string { return e.Date }, false)
	site := aggregateHistory(entries, func(e NewAPICheckinHistoryEntry) string { return e.Date + "\x00" + e.Site }, true)
	account := aggregateHistory(entries, func(e NewAPICheckinHistoryEntry) string { return e.Date + "\x00" + e.Site + "\x00" + e.UserID }, true)
	return NewAPICheckinHistoryPayload{
		GeneratedAt:      s.nowText(),
		Entries:          entries,
		DailySummaries:   daily,
		SiteSummaries:    site,
		AccountSummaries: account,
	}
}

func (s *NewAPICheckinService) buildMonthlyPayloadLocked(ctx context.Context, month, siteName, userID string) (NewAPICheckinMonthlyPayload, error) {
	store, err := s.loadMonthlyStoreLocked(ctx)
	if err != nil {
		return NewAPICheckinMonthlyPayload{}, err
	}
	month = normalizeNewAPIMonth(month, s.currentMonth())
	siteName = cleanNewAPIText(siteName)
	userID = cleanNewAPIText(userID)
	records := []NewAPICheckinMonthlyRecord{}
	monthSet := map[string]bool{}
	for _, record := range store.Records {
		if record.Month != "" {
			monthSet[record.Month] = true
		}
		if month != "" && record.Month != month {
			continue
		}
		if siteName != "" && record.Site != siteName {
			continue
		}
		if userID != "" && record.UserID != userID {
			continue
		}
		record.QuotaAwardedDisplay = normalizeNewAPIDisplayText(record.QuotaAwardedDisplay)
		records = append(records, record)
	}
	months := make([]string, 0, len(monthSet))
	for value := range monthSet {
		months = append(months, value)
	}
	sort.Strings(months)
	siteSummaries, accountSummaries, siteDaily, accountDaily := aggregateMonthly(records)
	return NewAPICheckinMonthlyPayload{
		GeneratedAt:           s.nowText(),
		DBPath:                s.path(newAPICheckinMonthlyFile),
		Initialized:           true,
		SelectedMonth:         month,
		AvailableMonths:       months,
		SyncState:             s.monthlySyncState,
		Records:               records,
		SiteSummaries:         siteSummaries,
		AccountSummaries:      accountSummaries,
		SiteDailySummaries:    siteDaily,
		AccountDailySummaries: accountDaily,
	}, nil
}

func (s *NewAPICheckinService) upsertHistoryEntriesLocked(ctx context.Context, entries []NewAPICheckinHistoryEntry) error {
	payload, err := s.buildHistoryPayloadLocked(ctx)
	if err != nil {
		return err
	}
	byKey := map[string]NewAPICheckinHistoryEntry{}
	for _, entry := range payload.Entries {
		byKey[entry.Date+"\x00"+entry.Site+"\x00"+entry.UserID] = entry
	}
	for _, entry := range entries {
		byKey[entry.Date+"\x00"+entry.Site+"\x00"+entry.UserID] = entry
	}
	next := make([]NewAPICheckinHistoryEntry, 0, len(byKey))
	for _, entry := range byKey {
		next = append(next, entry)
	}
	payload = s.buildHistoryPayloadFromEntriesLocked(next)
	if s.repo == nil {
		return errors.New("newapi checkin repository is not configured")
	}
	return s.repo.SaveHistory(ctx, payload)
}

func (s *NewAPICheckinService) upsertMonthlyFromReportLocked(ctx context.Context, report NewAPICheckinReport, source string) error {
	records := []NewAPICheckinMonthlyRecord{}
	cfg, err := s.loadConfigLocked(ctx)
	if err != nil {
		return err
	}
	for _, row := range report.AccountResults {
		if row.QuotaAwarded == nil || row.CheckinDate == "" {
			continue
		}
		site, _ := findNewAPISite(cfg, row.Site)
		account, _ := findNewAPIAccount(site, row.UserID)
		records = append(records, NewAPICheckinMonthlyRecord{
			Site:                     row.Site,
			UserID:                   row.UserID,
			AccountName:              row.Account,
			Username:                 row.Username,
			DisplayName:              row.DisplayName,
			IPProfile:                row.IPProfile,
			Month:                    monthFromDate(row.CheckinDate, s.currentMonth()),
			CheckinDate:              row.CheckinDate,
			QuotaAwarded:             row.QuotaAwarded,
			QuotaAwardedDisplay:      row.QuotaAwardedDisplay,
			QuotaAwardedDisplayValue: row.QuotaAwardedDisplayValue,
			FetchedAt:                firstNonEmpty(report.EndedAt, s.nowText()),
			Source:                   source,
		})
		if records[len(records)-1].AccountName == "" {
			records[len(records)-1].AccountName = account.Name
		}
	}
	return s.upsertMonthlyRecordsLocked(ctx, records)
}

func (s *NewAPICheckinService) upsertMonthlyRecordsLocked(ctx context.Context, records []NewAPICheckinMonthlyRecord) error {
	store, err := s.loadMonthlyStoreLocked(ctx)
	if err != nil {
		return err
	}
	byKey := map[string]NewAPICheckinMonthlyRecord{}
	for _, record := range store.Records {
		byKey[record.Site+"\x00"+record.UserID+"\x00"+record.CheckinDate] = record
	}
	for _, record := range records {
		byKey[record.Site+"\x00"+record.UserID+"\x00"+record.CheckinDate] = record
	}
	store.Records = store.Records[:0]
	for _, record := range byKey {
		store.Records = append(store.Records, record)
	}
	sort.Slice(store.Records, func(i, j int) bool {
		if store.Records[i].CheckinDate == store.Records[j].CheckinDate {
			if store.Records[i].Site == store.Records[j].Site {
				return store.Records[i].UserID < store.Records[j].UserID
			}
			return store.Records[i].Site < store.Records[j].Site
		}
		return store.Records[i].CheckinDate < store.Records[j].CheckinDate
	})
	if s.repo == nil {
		return errors.New("newapi checkin repository is not configured")
	}
	return s.repo.SaveMonthlyRecords(ctx, store.Records)
}

func (s *NewAPICheckinService) mergeReportIntoLastRunLocked(ctx context.Context, report NewAPICheckinReport) (NewAPICheckinReport, error) {
	current, err := s.loadReportLocked(ctx)
	if err != nil {
		return NewAPICheckinReport{}, err
	}
	byKey := map[string]NewAPICheckinAccountResult{}
	for _, row := range current.AccountResults {
		byKey[row.Site+"\x00"+row.UserID] = row
	}
	for _, row := range report.AccountResults {
		byKey[row.Site+"\x00"+row.UserID] = row
	}
	current.AccountResults = current.AccountResults[:0]
	for _, row := range byKey {
		current.AccountResults = append(current.AccountResults, row)
	}
	sort.Slice(current.AccountResults, func(i, j int) bool {
		if current.AccountResults[i].Site == current.AccountResults[j].Site {
			return current.AccountResults[i].UserID < current.AccountResults[j].UserID
		}
		return current.AccountResults[i].Site < current.AccountResults[j].Site
	})
	current.StartedAt = firstNonEmpty(report.StartedAt, current.StartedAt)
	current.EndedAt = firstNonEmpty(report.EndedAt, s.nowText())
	current.Source = firstNonEmpty(report.Source, current.Source)
	current = s.rebuildReportLocked(current)
	if err := s.saveReportLocked(ctx, current); err != nil {
		return NewAPICheckinReport{}, err
	}
	return current, nil
}

func (s *NewAPICheckinService) loadConfigLocked(ctx context.Context) (NewAPICheckinConfig, error) {
	if s.repo == nil {
		return NewAPICheckinConfig{}, errors.New("newapi checkin repository is not configured")
	}
	return s.repo.LoadConfig(ctx)
}

func (s *NewAPICheckinService) saveConfigLocked(ctx context.Context, cfg NewAPICheckinConfig) error {
	if s.repo == nil {
		return errors.New("newapi checkin repository is not configured")
	}
	return s.repo.SaveConfig(ctx, cfg)
}

func (s *NewAPICheckinService) loadReportLocked(ctx context.Context) (NewAPICheckinReport, error) {
	if s.repo == nil {
		return NewAPICheckinReport{}, errors.New("newapi checkin repository is not configured")
	}
	return s.repo.LoadLatestReport(ctx)
}

func (s *NewAPICheckinService) saveReportLocked(ctx context.Context, report NewAPICheckinReport) error {
	if s.repo == nil {
		return errors.New("newapi checkin repository is not configured")
	}
	return s.repo.SaveLatestReport(ctx, report)
}

func (s *NewAPICheckinService) loadBalanceCacheLocked(ctx context.Context) (NewAPICheckinBalancePayload, error) {
	if s.repo == nil {
		return NewAPICheckinBalancePayload{}, errors.New("newapi checkin repository is not configured")
	}
	cache, err := s.repo.LoadBalanceCache(ctx)
	if err != nil {
		return NewAPICheckinBalancePayload{}, err
	}
	if cache.SiteStatuses == nil {
		cache.SiteStatuses = map[string]NewAPICheckinSiteStatus{}
	}
	return cache, nil
}

func (s *NewAPICheckinService) saveBalanceCacheLocked(ctx context.Context, cache NewAPICheckinBalancePayload) error {
	if s.repo == nil {
		return errors.New("newapi checkin repository is not configured")
	}
	return s.repo.SaveBalanceCache(ctx, cache)
}

func (s *NewAPICheckinService) loadMonthlyStoreLocked(ctx context.Context) (NewAPICheckinMonthlyStore, error) {
	if s.repo == nil {
		return NewAPICheckinMonthlyStore{}, errors.New("newapi checkin repository is not configured")
	}
	records, err := s.repo.LoadMonthlyRecords(ctx)
	if err != nil {
		return NewAPICheckinMonthlyStore{}, err
	}
	return NewAPICheckinMonthlyStore{Records: records}, nil
}

func (s *NewAPICheckinService) path(name string) string {
	if s.repo == nil {
		return "sql:newapi-checkin/" + name
	}
	return strings.TrimRight(s.repo.StorageLabel(), "/") + "/" + name
}

func (s *NewAPICheckinService) nowText() string {
	return s.now().Format(newAPICheckinDateTimeLayout)
}

func (s *NewAPICheckinService) todayText() string {
	return s.now().Format(newAPICheckinDateLayout)
}

func (s *NewAPICheckinService) currentMonth() string {
	return s.now().Format("2006-01")
}

func decodeNewAPIConfigMap(raw map[string]any) (NewAPICheckinConfig, error) {
	cfg := NewAPICheckinConfig{
		DefaultCheckinPath: newAPICheckinDefaultPath,
		Sites:              []NewAPICheckinSite{},
	}
	if raw == nil {
		return cfg, nil
	}
	if v := cleanNewAPIText(raw["default_checkin_path"]); v != "" {
		cfg.DefaultCheckinPath = v
	}
	cfg.DelayBetweenCheckinsSec = intFromAny(raw["delay_between_checkins_sec"])
	cfg.RouteSwitchWaitSec = intFromAny(raw["route_switch_wait_sec"])
	cfg.NotifyFeishu = boolFromAny(raw["notify_feishu"], false)
	for _, item := range sliceFromAny(raw["sites"]) {
		site, err := normalizeNewAPISite(mapFromAny(item))
		if err != nil {
			return cfg, err
		}
		cfg.Sites = append(cfg.Sites, site)
	}
	return cfg, nil
}

func normalizeIncomingNewAPISites(payload map[string]any) ([]NewAPICheckinSite, error) {
	rawSites := payload["sites"]
	if rawSites == nil {
		if rawSite, ok := payload["site"]; ok {
			rawSites = []any{rawSite}
		} else {
			rawSites = []any{payload}
		}
	}
	items := sliceFromAny(rawSites)
	if len(items) == 0 {
		return nil, errors.New("sites 必须是数组")
	}
	sites := make([]NewAPICheckinSite, 0, len(items))
	for _, item := range items {
		site, err := normalizeNewAPISite(mapFromAny(item))
		if err != nil {
			return nil, err
		}
		sites = append(sites, site)
	}
	return sites, nil
}

func normalizeNewAPISite(raw map[string]any) (NewAPICheckinSite, error) {
	name := cleanNewAPIText(raw["name"])
	baseURL := strings.TrimRight(cleanNewAPIText(raw["base_url"]), "/")
	if name == "" || baseURL == "" {
		return NewAPICheckinSite{}, errors.New("平台必须包含 name 和 base_url")
	}
	accountItems := sliceFromAny(raw["accounts"])
	if len(accountItems) == 0 {
		return NewAPICheckinSite{}, errors.New("平台必须包含 accounts 数组")
	}
	accounts := make([]NewAPICheckinAccount, 0, len(accountItems))
	for _, item := range accountItems {
		account, err := normalizeNewAPIAccount(mapFromAny(item))
		if err != nil {
			return NewAPICheckinSite{}, err
		}
		accounts = append(accounts, account)
	}
	site := NewAPICheckinSite{
		Name:                     name,
		Provider:                 normalizeNewAPIProvider(cleanNewAPIText(raw["provider"])),
		Enabled:                  boolFromAny(raw["enabled"], true),
		DisabledReason:           cleanNewAPIText(raw["disabled_reason"]),
		BackgroundCheckinEnabled: boolFromAny(raw["background_checkin_enabled"], true),
		BaseURL:                  baseURL,
		CheckinPath:              cleanNewAPIText(raw["checkin_path"]),
		SiteStatus:               normalizeNewAPISiteStatusFromMap(mapFromAny(raw["site_status"])),
		Accounts:                 accounts,
	}
	return site, nil
}

func normalizeNewAPIAccount(raw map[string]any) (NewAPICheckinAccount, error) {
	userID := cleanNewAPIText(raw["user_id"])
	accessKey := cleanNewAPIText(raw["access_key"])
	if userID == "" || accessKey == "" {
		return NewAPICheckinAccount{}, errors.New("账号必须包含 user_id 和 access_key")
	}
	account := NewAPICheckinAccount{
		Name:           firstNonEmpty(cleanNewAPIText(raw["name"]), userID),
		Username:       cleanNewAPIText(raw["username"]),
		DisplayName:    cleanNewAPIText(raw["display_name"]),
		UserID:         userID,
		AccessKey:      accessKey,
		LoginUsername:  cleanNewAPIText(raw["login_username"]),
		LoginPassword:  cleanNewAPIText(raw["login_password"]),
		IPProfile:      firstNonEmpty(cleanNewAPIText(raw["ip_profile"]), "ip-slot-manual"),
		Enabled:        boolFromAny(raw["enabled"], true),
		DisabledReason: cleanNewAPIText(raw["disabled_reason"]),
	}
	return account, nil
}

func normalizeNewAPISiteStatusFromMap(raw map[string]any) NewAPICheckinSiteStatus {
	status := defaultNewAPISiteStatus()
	if raw == nil {
		return status
	}
	status.OK = boolFromAny(raw["ok"], status.OK)
	status.Message = firstNonEmpty(cleanNewAPIText(raw["message"]), status.Message)
	status.QuotaDisplayType = firstNonEmpty(cleanNewAPIText(raw["quota_display_type"]), status.QuotaDisplayType)
	if unit := int64FromAny(raw["quota_per_unit"]); unit > 0 {
		status.QuotaPerUnit = unit
	}
	status.CustomCurrencySymbol = cleanNewAPIText(raw["custom_currency_symbol"])
	return status
}

func normalizeNewAPISiteStatus(status NewAPICheckinSiteStatus) NewAPICheckinSiteStatus {
	base := defaultNewAPISiteStatus()
	if status.Message != "" {
		base.Message = status.Message
	}
	if status.QuotaDisplayType != "" {
		base.QuotaDisplayType = status.QuotaDisplayType
	}
	if status.QuotaPerUnit > 0 {
		base.QuotaPerUnit = status.QuotaPerUnit
	}
	if status.CustomCurrencySymbol != "" {
		base.CustomCurrencySymbol = status.CustomCurrencySymbol
	}
	base.OK = status.OK || status.Message == ""
	return base
}

func defaultNewAPISiteStatus() NewAPICheckinSiteStatus {
	return NewAPICheckinSiteStatus{
		OK:               true,
		Message:          "本地默认折算",
		QuotaDisplayType: "USD",
		QuotaPerUnit:     newAPICheckinDefaultQuotaUnit,
	}
}

func findNewAPISite(cfg NewAPICheckinConfig, siteName string) (NewAPICheckinSite, int) {
	siteName = cleanNewAPIText(siteName)
	for idx, site := range cfg.Sites {
		if site.Name == siteName {
			return site, idx
		}
	}
	return NewAPICheckinSite{}, -1
}

func findNewAPIAccount(site NewAPICheckinSite, userID string) (NewAPICheckinAccount, int) {
	userID = cleanNewAPIText(userID)
	for idx, account := range site.Accounts {
		if account.UserID == userID {
			return account, idx
		}
	}
	return NewAPICheckinAccount{}, -1
}

func buildNewAPICheckinTasks(cfg NewAPICheckinConfig, includeDisabledSites bool) []newAPICheckinTask {
	maxAccounts := 0
	for _, site := range cfg.Sites {
		if isSub2APISite(site) {
			continue
		}
		if !includeDisabledSites && !site.Enabled {
			continue
		}
		if len(site.Accounts) > maxAccounts {
			maxAccounts = len(site.Accounts)
		}
	}
	tasks := []newAPICheckinTask{}
	for accountIdx := 0; accountIdx < maxAccounts; accountIdx++ {
		for _, site := range cfg.Sites {
			if isSub2APISite(site) {
				continue
			}
			if !includeDisabledSites && !site.Enabled {
				continue
			}
			if accountIdx >= len(site.Accounts) {
				continue
			}
			account := site.Accounts[accountIdx]
			if !account.Enabled {
				continue
			}
			tasks = append(tasks, newAPICheckinTask{site: site, account: account})
		}
	}
	return tasks
}

func buildNewAPIMonthlyTasks(cfg NewAPICheckinConfig, siteName, userID string) []newAPICheckinTask {
	siteName = cleanNewAPIText(siteName)
	userID = cleanNewAPIText(userID)
	tasks := []newAPICheckinTask{}
	for _, site := range cfg.Sites {
		if isSub2APISite(site) {
			continue
		}
		if siteName != "" && site.Name != siteName {
			continue
		}
		for _, account := range site.Accounts {
			if userID != "" && account.UserID != userID {
				continue
			}
			if !account.Enabled {
				continue
			}
			tasks = append(tasks, newAPICheckinTask{site: site, account: account})
		}
	}
	return tasks
}

func joinNewAPIURL(baseURL, path string) string {
	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(path, "/")
}

func normalizeNewAPIProvider(provider string) string {
	if strings.EqualFold(strings.TrimSpace(provider), newAPICheckinProviderSub2API) {
		return newAPICheckinProviderSub2API
	}
	return newAPICheckinProviderNewAPI
}

func isSub2APISite(site NewAPICheckinSite) bool {
	return normalizeNewAPIProvider(site.Provider) == newAPICheckinProviderSub2API
}

func optionalFloat64FromAny(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func sub2AmountToQuota(amount float64, status NewAPICheckinSiteStatus) int64 {
	return int64(amount*float64(status.QuotaPerUnit) + 0.5)
}

func formatSub2Amount(amount float64, status NewAPICheckinSiteStatus) string {
	symbol := firstNonEmpty(status.CustomCurrencySymbol, newAPICheckinDefaultSymbol)
	return symbol + strconv.FormatFloat(amount, 'f', -1, 64)
}

func cleanNewAPIText(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func boolFromAny(value any, defaultValue bool) bool {
	if value == nil {
		return defaultValue
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		if typed == "" {
			return defaultValue
		}
		return typed == "true" || typed == "1" || typed == "yes"
	default:
		return defaultValue
	}
}

func intFromAny(value any) int {
	return int(int64FromAny(value))
}

func int64FromAny(value any) int64 {
	v, _ := optionalInt64FromAny(value)
	return v
}

func optionalInt64FromAny(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int64:
		return typed, true
	case float64:
		return int64(typed), true
	case json.Number:
		v, err := typed.Int64()
		return v, err == nil
	case string:
		var v int64
		_, err := fmt.Sscanf(strings.TrimSpace(typed), "%d", &v)
		return v, err == nil
	default:
		return 0, false
	}
}

func pointerIfPresent(payload map[string]any, key string, value int64) *int64 {
	if _, ok := optionalInt64FromAny(payload[key]); !ok {
		return nil
	}
	return &value
}

func mapFromAny(value any) map[string]any {
	if value == nil {
		return nil
	}
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	body, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var out map[string]any
	_ = json.Unmarshal(body, &out)
	return out
}

func sliceFromAny(value any) []any {
	switch typed := value.(type) {
	case []any:
		return typed
	case []map[string]any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, item)
		}
		return out
	default:
		return nil
	}
}

func extractNewAPIMessage(payload map[string]any) string {
	for _, key := range []string{"message", "msg", "error", "reason", "status"} {
		if value := cleanNewAPIText(payload[key]); value != "" {
			return value
		}
	}
	return ""
}

func formatNewAPIDisplayAmount(quota *int64, status NewAPICheckinSiteStatus) string {
	if quota == nil {
		return ""
	}
	status = normalizeNewAPISiteStatus(status)
	if status.QuotaPerUnit <= 0 {
		return ""
	}
	return formatDisplayTotal(float64(*quota)/float64(status.QuotaPerUnit), getNewAPIDisplaySymbol(status))
}

func getNewAPIDisplaySymbol(status NewAPICheckinSiteStatus) string {
	customSymbol := strings.TrimSpace(status.CustomCurrencySymbol)
	if isNewAPIGenericCurrencyPlaceholder(customSymbol) {
		customSymbol = ""
	}
	if customSymbol != "" {
		return customSymbol
	}
	if strings.EqualFold(status.QuotaDisplayType, "USD") || status.QuotaDisplayType == "" {
		return newAPICheckinDefaultSymbol
	}
	return status.QuotaDisplayType
}

func isNewAPIGenericCurrencyPlaceholder(symbol string) bool {
	return symbol == "¤" || symbol == "�"
}

func normalizeNewAPIDisplayText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for _, symbol := range []string{"¤", "�"} {
		if strings.HasPrefix(value, symbol) {
			return newAPICheckinDefaultSymbol + strings.TrimSpace(strings.TrimPrefix(value, symbol))
		}
	}
	return value
}

func formatDisplayTotal(value float64, symbol string) string {
	if symbol == "" {
		symbol = newAPICheckinDefaultSymbol
	}
	text := fmt.Sprintf("%s%.6f", symbol, value)
	text = strings.TrimRight(strings.TrimRight(text, "0"), ".")
	if text == symbol {
		return symbol + "0"
	}
	return text
}

func displayAmountValue(value string) float64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	start := 0
	for start < len(value) {
		ch := value[start]
		if (ch >= '0' && ch <= '9') || ch == '-' || ch == '+' || ch == '.' {
			break
		}
		start++
	}
	var out float64
	_, _ = fmt.Sscanf(value[start:], "%f", &out)
	return out
}

func detectDisplaySymbol(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		prefix := ""
		for _, ch := range value {
			if (ch >= '0' && ch <= '9') || ch == '-' || ch == '+' || ch == '.' {
				break
			}
			prefix += string(ch)
		}
		if prefix != "" {
			return prefix
		}
	}
	return ""
}

func resolveNewAPIAccountLabel(account NewAPICheckinAccount, row any) string {
	switch typed := row.(type) {
	case NewAPICheckinBalanceAccount:
		return firstNonEmpty(typed.DisplayName, typed.Username, account.DisplayName, account.Username, account.Name, account.UserID)
	default:
		return firstNonEmpty(account.DisplayName, account.Username, account.Name, account.UserID)
	}
}

func classifyNewAPICheckinStatus(row NewAPICheckinAccountResult) struct{ status, tone string } {
	if row.Success {
		return struct{ status, tone string }{"签到成功", "ok"}
	}
	if strings.Contains(row.Message, "已签到") {
		return struct{ status, tone string }{"今日已签到", "warn"}
	}
	return struct{ status, tone string }{"签到查询失败", "danger"}
}

func buildNewAPISummaryText(report NewAPICheckinReport) string {
	if len(report.AccountResults) == 0 {
		return "暂无签到任务。"
	}
	lines := []string{
		fmt.Sprintf("开始: %s", report.StartedAt),
		fmt.Sprintf("结束: %s", report.EndedAt),
		fmt.Sprintf("任务: %d，成功: %d，已签: %d，失败: %d，奖励: %s", report.TaskCount, report.SuccessCount, report.AlreadyDoneCount, report.FailedCount, report.QuotaAwardedDisplay),
	}
	return strings.Join(lines, "\n")
}

func datePart(value string) string {
	if len(value) >= 10 {
		return value[:10]
	}
	return value
}

func monthFromDate(value, fallback string) string {
	if len(value) >= 7 {
		return value[:7]
	}
	return fallback
}

func normalizeNewAPIMonth(value, fallback string) string {
	value = cleanNewAPIText(value)
	if len(value) >= 7 {
		return value[:7]
	}
	return fallback
}

func removeNewAPIBalanceRows(rows []NewAPICheckinBalanceAccount, siteName, userID string) []NewAPICheckinBalanceAccount {
	out := rows[:0]
	for _, row := range rows {
		if row.Site != siteName {
			out = append(out, row)
			continue
		}
		if userID != "" && row.UserID != userID {
			out = append(out, row)
			continue
		}
	}
	return out
}

func newAPIMatchesScope(site, rowUserID, targetSite, targetUserID string) bool {
	if site != targetSite {
		return false
	}
	if targetUserID == "" {
		return true
	}
	return rowUserID == targetUserID
}

func replaceNewAPIBalanceRows(existing []NewAPICheckinBalanceAccount, updated []NewAPICheckinBalanceAccount) []NewAPICheckinBalanceAccount {
	updatedKeys := map[string]bool{}
	for _, row := range updated {
		updatedKeys[row.Site+"\x00"+row.UserID] = true
	}
	out := make([]NewAPICheckinBalanceAccount, 0, len(existing)+len(updated))
	for _, row := range existing {
		if !updatedKeys[row.Site+"\x00"+row.UserID] {
			out = append(out, row)
		}
	}
	out = append(out, updated...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Site == out[j].Site {
			return out[i].UserID < out[j].UserID
		}
		return out[i].Site < out[j].Site
	})
	return out
}

func sumDisplayForSite(rows []NewAPICheckinBalanceAccount, site string, quota bool) float64 {
	total := 0.0
	for _, row := range rows {
		if row.Site != site {
			continue
		}
		if quota {
			total += displayAmountValue(row.QuotaDisplay)
		} else {
			total += displayAmountValue(row.UsedQuotaDisplay)
		}
	}
	return total
}

func aggregateHistory(entries []NewAPICheckinHistoryEntry, keyFn func(NewAPICheckinHistoryEntry) string, includeScope bool) []map[string]any {
	type bucket struct {
		date         string
		site         string
		userID       string
		account      string
		count        int
		success      int
		already      int
		failed       int
		quotaDisplay float64
		balance      float64
	}
	buckets := map[string]*bucket{}
	keys := []string{}
	for _, entry := range entries {
		key := keyFn(entry)
		if _, ok := buckets[key]; !ok {
			parts := strings.Split(key, "\x00")
			b := &bucket{date: parts[0]}
			if len(parts) > 1 {
				b.site = parts[1]
			}
			if len(parts) > 2 {
				b.userID = parts[2]
				b.account = entry.Account
			}
			buckets[key] = b
			keys = append(keys, key)
		}
		b := buckets[key]
		b.count++
		if entry.CheckinSuccess {
			b.success++
		} else if entry.CheckedInToday {
			b.already++
		} else {
			b.failed++
		}
		b.quotaDisplay += entry.QuotaAwardedDisplayValue
		b.balance += entry.BalanceDisplayValue
	}
	sort.Strings(keys)
	out := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		b := buckets[key]
		row := map[string]any{
			"date":                        b.date,
			"checkin_count":               b.count,
			"success_count":               b.success,
			"already_done_count":          b.already,
			"failed_count":                b.failed,
			"quota_awarded_display_value": b.quotaDisplay,
			"quota_awarded_display":       formatDisplayTotal(b.quotaDisplay, newAPICheckinDefaultSymbol),
			"balance_display_value":       b.balance,
			"balance_display":             formatDisplayTotal(b.balance, newAPICheckinDefaultSymbol),
		}
		if includeScope && b.site != "" {
			row["site"] = b.site
		}
		if includeScope && b.userID != "" {
			row["user_id"] = b.userID
			row["account"] = b.account
		}
		out = append(out, row)
	}
	return out
}

type newAPICheckinMonthlyBucket struct {
	site     string
	userID   string
	account  string
	date     string
	count    int
	accounts map[string]bool
	total    float64
}

func aggregateMonthly(records []NewAPICheckinMonthlyRecord) ([]map[string]any, []map[string]any, []map[string]any, []map[string]any) {
	siteMonth := map[string]*newAPICheckinMonthlyBucket{}
	accountMonth := map[string]*newAPICheckinMonthlyBucket{}
	siteDaily := map[string]*newAPICheckinMonthlyBucket{}
	accountDaily := map[string]*newAPICheckinMonthlyBucket{}
	// 月度刷新可能重复返回同一账号同一天的数据；汇总前按持久化唯一键收敛，确保次数和奖励只计算一次。
	uniqueRecords := make(map[string]NewAPICheckinMonthlyRecord, len(records))
	for _, record := range records {
		key := record.Site + "\x00" + record.UserID + "\x00" + record.CheckinDate
		uniqueRecords[key] = record
	}
	for _, record := range uniqueRecords {
		value := record.QuotaAwardedDisplayValue
		accountLabel := firstNonEmpty(record.DisplayName, record.Username, record.AccountName, record.UserID)
		if _, ok := siteMonth[record.Site]; !ok {
			siteMonth[record.Site] = &newAPICheckinMonthlyBucket{site: record.Site, accounts: map[string]bool{}}
		}
		siteMonth[record.Site].count++
		siteMonth[record.Site].total += value
		siteMonth[record.Site].accounts[record.UserID] = true
		accountKey := record.Site + "\x00" + record.UserID
		if _, ok := accountMonth[accountKey]; !ok {
			accountMonth[accountKey] = &newAPICheckinMonthlyBucket{site: record.Site, userID: record.UserID, account: accountLabel}
		}
		accountMonth[accountKey].count++
		accountMonth[accountKey].total += value
		siteDayKey := record.CheckinDate + "\x00" + record.Site
		if _, ok := siteDaily[siteDayKey]; !ok {
			siteDaily[siteDayKey] = &newAPICheckinMonthlyBucket{date: record.CheckinDate, site: record.Site}
		}
		siteDaily[siteDayKey].count++
		siteDaily[siteDayKey].total += value
		accountDayKey := record.CheckinDate + "\x00" + record.Site + "\x00" + record.UserID
		if _, ok := accountDaily[accountDayKey]; !ok {
			accountDaily[accountDayKey] = &newAPICheckinMonthlyBucket{date: record.CheckinDate, site: record.Site, userID: record.UserID, account: accountLabel}
		}
		accountDaily[accountDayKey].count++
		accountDaily[accountDayKey].total += value
	}
	return monthlyRows(siteMonth, false), monthlyRows(accountMonth, true), monthlyRows(siteDaily, false), monthlyRows(accountDaily, true)
}

func monthlyRows(buckets map[string]*newAPICheckinMonthlyBucket, includeAccount bool) []map[string]any {
	keys := make([]string, 0, len(buckets))
	for key := range buckets {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		bucket := buckets[key]
		row := map[string]any{
			"site":                        bucket.site,
			"checkin_count":               bucket.count,
			"quota_awarded_display_value": bucket.total,
			"quota_awarded_display":       formatDisplayTotal(bucket.total, newAPICheckinDefaultSymbol),
		}
		if bucket.date != "" {
			row["date"] = bucket.date
		}
		if len(bucket.accounts) > 0 {
			row["account_count"] = len(bucket.accounts)
		}
		if includeAccount {
			row["user_id"] = bucket.userID
			row["account"] = bucket.account
		}
		out = append(out, row)
	}
	return out
}
