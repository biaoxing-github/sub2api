// Package admin provides HTTP handlers for administrative operations.
package admin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/antigravity"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
)

const accountProbeBatchConcurrency = 2
const accountProbeRunTimeout = 45 * time.Minute

const (
	batchTestNonAPIKeyGlobalConcurrency = 10
	batchTestNonAPIKeyGroupConcurrency  = 3
	batchTestNonAPIKeyAdaptiveWindow    = 10 * time.Second
	batchTestNonAPIKeyPauseDuration     = 30 * time.Second
)

var nonAPIKeyBatchTestLimiter = newAccountBatchTestLimiter(
	batchTestNonAPIKeyGlobalConcurrency,
	batchTestNonAPIKeyGroupConcurrency,
	batchTestNonAPIKeyAdaptiveWindow,
	batchTestNonAPIKeyPauseDuration,
)

// OAuthHandler handles OAuth-related operations for accounts
type OAuthHandler struct {
	oauthService *service.OAuthService
}

// NewOAuthHandler creates a new OAuth handler
func NewOAuthHandler(oauthService *service.OAuthService) *OAuthHandler {
	return &OAuthHandler{
		oauthService: oauthService,
	}
}

// AccountHandler handles admin account management
type AccountHandler struct {
	adminService                      service.AdminService
	oauthService                      *service.OAuthService
	openaiOAuthService                *service.OpenAIOAuthService
	geminiOAuthService                *service.GeminiOAuthService
	antigravityOAuthService           *service.AntigravityOAuthService
	rateLimitService                  *service.RateLimitService
	accountUsageService               *service.AccountUsageService
	accountTestService                *service.AccountTestService
	usageService                      *service.UsageService
	batchAccountTester                batchAccountTester
	accountBatchTestRepo              service.AccountBatchTestRepository
	accountProbeService               accountProbeRunner
	upstreamBalanceService            *service.UpstreamBalanceService
	concurrencyService                *service.ConcurrencyService
	crsSyncService                    *service.CRSSyncService
	sessionLimitCache                 service.SessionLimitCache
	rpmCache                          service.RPMCache
	tokenCacheInvalidator             service.TokenCacheInvalidator
	openAIPathHealthReader            accountPathHealthReader
	openAIAccountSchedulingPoolReader accountSchedulingPoolReader
}

type accountProbeRunner interface {
	Start(ctx context.Context, req service.AccountProbeRunRequest) (service.AccountProbeResult, error)
	RunExisting(ctx context.Context, run service.AccountProbeResult, req service.AccountProbeRunRequest) (service.AccountProbeResult, error)
	StartBazaarLink(ctx context.Context, req service.BazaarLinkProbeRunRequest) (service.AccountProbeResult, error)
	RunBazaarLinkExisting(ctx context.Context, run service.AccountProbeResult, req service.BazaarLinkProbeRunRequest) (service.AccountProbeResult, error)
	List(ctx context.Context, filter service.AccountProbeHistoryFilter) ([]service.AccountProbeResult, error)
	Get(ctx context.Context, accountID, runID int64) (*service.AccountProbeResult, error)
	ListReports(ctx context.Context, filter service.AccountProbeReportFilter) (service.AccountProbeReportPage, error)
	GetReport(ctx context.Context, runID int64) (*service.AccountProbeReportItem, error)
	DeleteReports(ctx context.Context, runIDs []int64) (service.AccountProbeReportDeleteResult, error)
	ListRanking(ctx context.Context, limit int) ([]service.AccountProbeRankingItem, error)
}

type accountPathHealthReader interface {
	SnapshotOpenAIPathHealthForAccount(account *service.Account, transport service.OpenAIUpstreamTransport) (service.OpenAIPathHealthRecord, bool)
}

type accountSchedulingPoolReader interface {
	ListOpenAIAccountSchedulingPool(ctx context.Context, filter service.OpenAIAccountSchedulingPoolFilter, now time.Time) (service.OpenAIAccountSchedulingPoolSnapshot, error)
}

type batchAccountTester interface {
	RunTestBackground(ctx context.Context, accountID int64, modelID string) (*service.ScheduledTestResult, error)
}

// NewAccountHandler creates a new admin account handler
func NewAccountHandler(
	adminService service.AdminService,
	oauthService *service.OAuthService,
	openaiOAuthService *service.OpenAIOAuthService,
	geminiOAuthService *service.GeminiOAuthService,
	antigravityOAuthService *service.AntigravityOAuthService,
	rateLimitService *service.RateLimitService,
	accountUsageService *service.AccountUsageService,
	accountTestService *service.AccountTestService,
	upstreamBalanceService *service.UpstreamBalanceService,
	concurrencyService *service.ConcurrencyService,
	crsSyncService *service.CRSSyncService,
	sessionLimitCache service.SessionLimitCache,
	rpmCache service.RPMCache,
	tokenCacheInvalidator service.TokenCacheInvalidator,
) *AccountHandler {
	return &AccountHandler{
		adminService:            adminService,
		oauthService:            oauthService,
		openaiOAuthService:      openaiOAuthService,
		geminiOAuthService:      geminiOAuthService,
		antigravityOAuthService: antigravityOAuthService,
		rateLimitService:        rateLimitService,
		accountUsageService:     accountUsageService,
		accountTestService:      accountTestService,
		batchAccountTester:      accountTestService,
		upstreamBalanceService:  upstreamBalanceService,
		concurrencyService:      concurrencyService,
		crsSyncService:          crsSyncService,
		sessionLimitCache:       sessionLimitCache,
		rpmCache:                rpmCache,
		tokenCacheInvalidator:   tokenCacheInvalidator,
	}
}

// SetUsageService 注入人工探测使用记录服务。
func (h *AccountHandler) SetUsageService(usageService *service.UsageService) {
	h.usageService = usageService
}

func (h *AccountHandler) SetAccountProbeService(accountProbeService accountProbeRunner) {
	h.accountProbeService = accountProbeService
}

func (h *AccountHandler) SetAccountBatchTestRepository(repo service.AccountBatchTestRepository) {
	h.accountBatchTestRepo = repo
}

func (h *AccountHandler) SetOpenAIPathHealthReader(reader accountPathHealthReader) {
	h.openAIPathHealthReader = reader
}

func (h *AccountHandler) SetOpenAIAccountSchedulingPoolReader(reader accountSchedulingPoolReader) {
	h.openAIAccountSchedulingPoolReader = reader
}

// CreateAccountRequest represents create account request
type CreateAccountRequest struct {
	Name                    string         `json:"name" binding:"required"`
	Notes                   *string        `json:"notes"`
	Platform                string         `json:"platform" binding:"required"`
	Type                    string         `json:"type" binding:"required,oneof=oauth setup-token apikey upstream bedrock service_account"`
	Credentials             map[string]any `json:"credentials" binding:"required"`
	Extra                   map[string]any `json:"extra"`
	ProxyID                 *int64         `json:"proxy_id"`
	Concurrency             int            `json:"concurrency"`
	Priority                int            `json:"priority"`
	RateMultiplier          *float64       `json:"rate_multiplier"`
	LoadFactor              *int           `json:"load_factor"`
	GroupIDs                []int64        `json:"group_ids"`
	ExpiresAt               *int64         `json:"expires_at"`
	AutoPauseOnExpired      *bool          `json:"auto_pause_on_expired"`
	ConfirmMixedChannelRisk *bool          `json:"confirm_mixed_channel_risk"` // 用户确认混合渠道风险
}

// UpdateAccountRequest represents update account request
// 使用指针类型来区分"未提供"和"设置为0"
type UpdateAccountRequest struct {
	Name                    string         `json:"name"`
	Notes                   *string        `json:"notes"`
	Platform                string         `json:"platform"`
	Type                    string         `json:"type" binding:"omitempty,oneof=oauth setup-token apikey upstream bedrock service_account"`
	Credentials             map[string]any `json:"credentials"`
	Extra                   map[string]any `json:"extra"`
	ProxyID                 *int64         `json:"proxy_id"`
	Concurrency             *int           `json:"concurrency"`
	Priority                *int           `json:"priority"`
	RateMultiplier          *float64       `json:"rate_multiplier"`
	LoadFactor              *int           `json:"load_factor"`
	Status                  string         `json:"status" binding:"omitempty,oneof=active inactive error"`
	GroupIDs                *[]int64       `json:"group_ids"`
	ExpiresAt               *int64         `json:"expires_at"`
	AutoPauseOnExpired      *bool          `json:"auto_pause_on_expired"`
	ConfirmMixedChannelRisk *bool          `json:"confirm_mixed_channel_risk"` // 用户确认混合渠道风险
}

// BulkUpdateAccountsRequest represents the payload for bulk editing accounts
type BulkUpdateAccountsRequest struct {
	AccountIDs              []int64                   `json:"account_ids"`
	Filters                 *BulkUpdateAccountFilters `json:"filters"`
	Name                    string                    `json:"name"`
	ProxyID                 *int64                    `json:"proxy_id"`
	Concurrency             *int                      `json:"concurrency"`
	Priority                *int                      `json:"priority"`
	RateMultiplier          *float64                  `json:"rate_multiplier"`
	LoadFactor              *int                      `json:"load_factor"`
	Status                  string                    `json:"status" binding:"omitempty,oneof=active inactive error"`
	Schedulable             *bool                     `json:"schedulable"`
	GroupIDs                *[]int64                  `json:"group_ids"`
	Credentials             map[string]any            `json:"credentials"`
	Extra                   map[string]any            `json:"extra"`
	ConfirmMixedChannelRisk *bool                     `json:"confirm_mixed_channel_risk"` // 用户确认混合渠道风险
}

type BulkUpdateAccountFilters struct {
	Platform    string `json:"platform"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	Group       string `json:"group"`
	Search      string `json:"search"`
	PrivacyMode string `json:"privacy_mode"`
	PlanType    string `json:"plan_type"`
}

// CheckMixedChannelRequest represents check mixed channel risk request
type CheckMixedChannelRequest struct {
	Platform  string  `json:"platform" binding:"required"`
	GroupIDs  []int64 `json:"group_ids"`
	AccountID *int64  `json:"account_id"`
}

// AccountWithConcurrency extends Account with real-time concurrency info
type AccountWithConcurrency struct {
	*dto.Account
	CurrentConcurrency int `json:"current_concurrency"`
	// 以下字段仅对 Anthropic OAuth/SetupToken 账号有效，且仅在启用相应功能时返回
	CurrentWindowCost *float64 `json:"current_window_cost,omitempty"` // 当前窗口费用
	ActiveSessions    *int     `json:"active_sessions,omitempty"`     // 当前活跃会话数
	CurrentRPM        *int     `json:"current_rpm,omitempty"`         // 当前分钟 RPM 计数
}

// AccountSchedulingPoolResponse 表示管理端看到的协议调度池快照。
type AccountSchedulingPoolResponse struct {
	Items            []AccountSchedulingPoolItem `json:"items"`
	Total            int                         `json:"total"`
	SchedulableCount int                         `json:"schedulable_count"`
	DegradedCount    int                         `json:"degraded_count"`
	BlockedCount     int                         `json:"blocked_count"`
	FilteredCount    int                         `json:"filtered_count"`
	GeneratedAt      time.Time                   `json:"generated_at"`
	GroupID          *int64                      `json:"group_id,omitempty"`
	Platform         string                      `json:"platform,omitempty"`
	Model            string                      `json:"model,omitempty"`
	Endpoint         string                      `json:"endpoint,omitempty"`
	Transport        string                      `json:"transport,omitempty"`
	ImageCapability  string                      `json:"image_capability,omitempty"`
}

// AccountSchedulingPoolItem 表示一个账号在当前调度约束下的池内状态。
type AccountSchedulingPoolItem struct {
	Account             *dto.Account                               `json:"account"`
	PoolStatus          string                                     `json:"pool_status"`
	PoolReasons         []string                                   `json:"pool_reasons,omitempty"`
	NextScheduledAt     *time.Time                                 `json:"next_scheduled_at,omitempty"`
	NextScheduledReason string                                     `json:"next_scheduled_reason,omitempty"`
	RuntimeBlock        *service.OpenAIAccountRuntimeBlockSnapshot `json:"runtime_block,omitempty"`
	PathHealth          service.OpenAIPathHealthRecord             `json:"path_health"`
	PathHealthAvailable bool                                       `json:"path_health_available"`
	DerivedHealth       *dto.AccountDerivedHealth                  `json:"derived_health,omitempty"`
	EffectiveLoadFactor int                                        `json:"effective_load_factor"`
}

const accountListGroupUngroupedQueryValue = "ungrouped"

func (h *AccountHandler) buildAccountResponseWithRuntime(ctx context.Context, account *service.Account) AccountWithConcurrency {
	item := AccountWithConcurrency{
		Account:            dto.AccountFromService(account),
		CurrentConcurrency: 0,
	}
	if account == nil {
		return item
	}
	h.attachAccountLoadFactorAdvice(item.Account, account)

	if h.concurrencyService != nil {
		if counts, err := h.concurrencyService.GetAccountConcurrencyBatch(ctx, []int64{account.ID}); err == nil {
			item.CurrentConcurrency = counts[account.ID]
		}
	}

	if account.IsAnthropicOAuthOrSetupToken() {
		if h.accountUsageService != nil && account.GetWindowCostLimit() > 0 {
			startTime := account.GetCurrentWindowStartTime()
			if stats, err := h.accountUsageService.GetAccountWindowStats(ctx, account.ID, startTime); err == nil && stats != nil {
				cost := stats.StandardCost
				item.CurrentWindowCost = &cost
			}
		}

		if h.sessionLimitCache != nil && account.GetMaxSessions() > 0 {
			idleTimeout := time.Duration(account.GetSessionIdleTimeoutMinutes()) * time.Minute
			idleTimeouts := map[int64]time.Duration{account.ID: idleTimeout}
			if sessions, err := h.sessionLimitCache.GetActiveSessionCountBatch(ctx, []int64{account.ID}, idleTimeouts); err == nil {
				if count, ok := sessions[account.ID]; ok {
					item.ActiveSessions = &count
				}
			}
		}

		if h.rpmCache != nil && account.GetBaseRPM() > 0 {
			if rpm, err := h.rpmCache.GetRPM(ctx, account.ID); err == nil {
				item.CurrentRPM = &rpm
			}
		}
	}

	return item
}

// List handles listing all accounts with pagination
// GET /api/v1/admin/accounts
func (h *AccountHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	platform := c.Query("platform")
	accountType := c.Query("type")
	status := c.Query("status")
	search := c.Query("search")
	privacyMode := strings.TrimSpace(c.Query("privacy_mode"))
	planType := strings.ToLower(strings.TrimSpace(c.Query("plan_type")))
	sortBy := c.DefaultQuery("sort_by", "name")
	sortOrder := c.DefaultQuery("sort_order", "asc")
	// 标准化和验证 search 参数
	search = strings.TrimSpace(search)
	if len(search) > 100 {
		search = search[:100]
	}
	lite := parseBoolQueryWithDefault(c.Query("lite"), false)

	var groupID int64
	if groupIDStr := c.Query("group"); groupIDStr != "" {
		if groupIDStr == accountListGroupUngroupedQueryValue {
			groupID = service.AccountListGroupUngrouped
		} else {
			parsedGroupID, parseErr := strconv.ParseInt(groupIDStr, 10, 64)
			if parseErr != nil {
				response.ErrorFrom(c, infraerrors.BadRequest("INVALID_GROUP_FILTER", "invalid group filter"))
				return
			}
			if parsedGroupID < 0 {
				response.ErrorFrom(c, infraerrors.BadRequest("INVALID_GROUP_FILTER", "invalid group filter"))
				return
			}
			groupID = parsedGroupID
		}
	}

	accounts, total, err := h.adminService.ListAccounts(c.Request.Context(), page, pageSize, platform, accountType, status, search, groupID, privacyMode, planType, sortBy, sortOrder)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Get current concurrency counts for all accounts
	accountIDs := make([]int64, len(accounts))
	for i, acc := range accounts {
		accountIDs[i] = acc.ID
	}

	concurrencyCounts := make(map[int64]int)
	var windowCosts map[int64]float64
	var activeSessions map[int64]int
	var rpmCounts map[int64]int

	// 始终获取并发数（Redis ZCARD，极低开销）
	if h.concurrencyService != nil {
		if cc, ccErr := h.concurrencyService.GetAccountConcurrencyBatch(c.Request.Context(), accountIDs); ccErr == nil && cc != nil {
			concurrencyCounts = cc
		}
	}

	// 识别需要查询窗口费用、会话数和 RPM 的账号（Anthropic OAuth/SetupToken 且启用了相应功能）
	windowCostAccountIDs := make([]int64, 0)
	sessionLimitAccountIDs := make([]int64, 0)
	rpmAccountIDs := make([]int64, 0)
	sessionIdleTimeouts := make(map[int64]time.Duration) // 各账号的会话空闲超时配置
	for i := range accounts {
		acc := &accounts[i]
		if acc.IsAnthropicOAuthOrSetupToken() {
			if acc.GetWindowCostLimit() > 0 {
				windowCostAccountIDs = append(windowCostAccountIDs, acc.ID)
			}
			if acc.GetMaxSessions() > 0 {
				sessionLimitAccountIDs = append(sessionLimitAccountIDs, acc.ID)
				sessionIdleTimeouts[acc.ID] = time.Duration(acc.GetSessionIdleTimeoutMinutes()) * time.Minute
			}
			if acc.GetBaseRPM() > 0 {
				rpmAccountIDs = append(rpmAccountIDs, acc.ID)
			}
		}
	}

	// 始终获取 RPM 计数（Redis GET，极低开销）
	if len(rpmAccountIDs) > 0 && h.rpmCache != nil {
		rpmCounts, _ = h.rpmCache.GetRPMBatch(c.Request.Context(), rpmAccountIDs)
		if rpmCounts == nil {
			rpmCounts = make(map[int64]int)
		}
	}

	// 始终获取活跃会话数（Redis ZCARD，低开销）
	if len(sessionLimitAccountIDs) > 0 && h.sessionLimitCache != nil {
		activeSessions, _ = h.sessionLimitCache.GetActiveSessionCountBatch(c.Request.Context(), sessionLimitAccountIDs, sessionIdleTimeouts)
		if activeSessions == nil {
			activeSessions = make(map[int64]int)
		}
	}

	// 始终获取窗口费用（PostgreSQL 聚合查询）
	if len(windowCostAccountIDs) > 0 {
		windowCosts = make(map[int64]float64)
		var mu sync.Mutex
		g, gctx := errgroup.WithContext(c.Request.Context())
		g.SetLimit(10) // 限制并发数

		for i := range accounts {
			acc := &accounts[i]
			if !acc.IsAnthropicOAuthOrSetupToken() || acc.GetWindowCostLimit() <= 0 {
				continue
			}
			accCopy := acc // 闭包捕获
			g.Go(func() error {
				// 使用统一的窗口开始时间计算逻辑（考虑窗口过期情况）
				startTime := accCopy.GetCurrentWindowStartTime()
				stats, err := h.accountUsageService.GetAccountWindowStats(gctx, accCopy.ID, startTime)
				if err == nil && stats != nil {
					mu.Lock()
					windowCosts[accCopy.ID] = stats.StandardCost // 使用标准费用
					mu.Unlock()
				}
				return nil // 不返回错误，允许部分失败
			})
		}
		_ = g.Wait()
	}

	// Build response with concurrency info
	result := make([]AccountWithConcurrency, len(accounts))
	for i := range accounts {
		acc := &accounts[i]
		item := AccountWithConcurrency{
			Account:            dto.AccountFromService(acc),
			CurrentConcurrency: concurrencyCounts[acc.ID],
		}
		h.attachAccountLoadFactorAdvice(item.Account, acc)

		// 添加窗口费用（仅当启用时）
		if windowCosts != nil {
			if cost, ok := windowCosts[acc.ID]; ok {
				item.CurrentWindowCost = &cost
			}
		}

		// 添加活跃会话数（仅当启用时）
		if activeSessions != nil {
			if count, ok := activeSessions[acc.ID]; ok {
				item.ActiveSessions = &count
			}
		}

		// 添加 RPM 计数（仅当启用时）
		if rpmCounts != nil {
			if rpm, ok := rpmCounts[acc.ID]; ok {
				item.CurrentRPM = &rpm
			}
		}

		result[i] = item
	}

	etag := buildAccountsListETag(result, total, page, pageSize, platform, accountType, status, search, lite)
	if etag != "" {
		c.Header("ETag", etag)
		c.Header("Vary", "If-None-Match")
		if ifNoneMatchMatched(c.GetHeader("If-None-Match"), etag) {
			c.Status(http.StatusNotModified)
			return
		}
	}

	response.Paginated(c, result, total, page, pageSize)
}

// ListSchedulingPool handles protocol scheduling pool visualization.
// GET /api/v1/admin/accounts/scheduling-pool
func (h *AccountHandler) ListSchedulingPool(c *gin.Context) {
	if h.openAIAccountSchedulingPoolReader == nil {
		response.InternalError(c, "Scheduling pool reader is not configured")
		return
	}
	filter, ok := parseAccountSchedulingPoolFilter(c)
	if !ok {
		return
	}
	snapshot, err := h.openAIAccountSchedulingPoolReader.ListOpenAIAccountSchedulingPool(c.Request.Context(), filter, time.Now())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, accountSchedulingPoolResponseFromService(snapshot))
}

func (h *AccountHandler) attachAccountLoadFactorAdvice(out *dto.Account, account *service.Account) {
	if out == nil || account == nil {
		return
	}
	var health service.OpenAIPathHealthRecord
	if account.IsOpenAI() && h.openAIPathHealthReader != nil {
		if snapshot, ok := h.openAIPathHealthReader.SnapshotOpenAIPathHealthForAccount(account, service.OpenAIUpstreamTransportHTTPSSE); ok {
			health = snapshot
		}
	}
	out.DerivedHealth = dto.AccountDerivedHealthFromService(service.DeriveAccountHealthState(account, health, time.Now()))
	if !account.IsOpenAI() {
		return
	}
	advice := service.NewAccountLoadFactorAdvisor(service.AccountLoadFactorAdvisorOptions{}).Advise(account, health)
	out.LoadFactorAdvice = dto.AccountLoadFactorAdviceFromService(advice)
}

func parseAccountSchedulingPoolFilter(c *gin.Context) (service.OpenAIAccountSchedulingPoolFilter, bool) {
	var filter service.OpenAIAccountSchedulingPoolFilter
	if groupText := strings.TrimSpace(c.Query("group")); groupText != "" {
		groupID, err := strconv.ParseInt(groupText, 10, 64)
		if err != nil || groupID < 0 {
			response.BadRequest(c, "Invalid group filter")
			return filter, false
		}
		filter.GroupID = &groupID
	}
	filter.Model = strings.TrimSpace(c.Query("model"))
	filter.Search = strings.TrimSpace(c.Query("search"))
	if len(filter.Search) > 100 {
		filter.Search = filter.Search[:100]
	}
	platform, ok := parseAccountSchedulingPoolPlatform(c)
	if !ok {
		return filter, false
	}
	filter.Platform = platform

	endpoint, ok := parseOpenAIEndpointCapabilityQuery(c, "endpoint")
	if !ok {
		return filter, false
	}
	transport, ok := parseOpenAIUpstreamTransportQuery(c, "transport")
	if !ok {
		return filter, false
	}
	imageCapability, ok := parseOpenAIImagesCapabilityQuery(c, "image_capability")
	if !ok {
		return filter, false
	}
	filter.Endpoint = endpoint
	filter.Transport = transport
	filter.ImageCapability = imageCapability
	return filter, true
}

func parseAccountSchedulingPoolPlatform(c *gin.Context) (string, bool) {
	value := strings.ToLower(strings.TrimSpace(c.DefaultQuery("platform", service.PlatformOpenAI)))
	switch value {
	case service.PlatformOpenAI, service.PlatformAnthropic:
		return value, true
	default:
		response.BadRequest(c, "Invalid scheduling pool platform")
		return "", false
	}
}

func parseOpenAIEndpointCapabilityQuery(c *gin.Context, key string) (service.OpenAIEndpointCapability, bool) {
	value := strings.TrimSpace(c.Query(key))
	switch service.OpenAIEndpointCapability(value) {
	case "", service.OpenAIEndpointCapabilityResponses, service.OpenAIEndpointCapabilityChatCompletions, service.OpenAIEndpointCapabilityEmbeddings:
		return service.OpenAIEndpointCapability(value), true
	default:
		response.BadRequest(c, "Invalid endpoint capability")
		return "", false
	}
}

func parseOpenAIUpstreamTransportQuery(c *gin.Context, key string) (service.OpenAIUpstreamTransport, bool) {
	value := strings.TrimSpace(c.Query(key))
	switch service.OpenAIUpstreamTransport(value) {
	case service.OpenAIUpstreamTransportAny, service.OpenAIUpstreamTransportHTTPSSE, service.OpenAIUpstreamTransportResponsesWebsocket, service.OpenAIUpstreamTransportResponsesWebsocketV2:
		return service.OpenAIUpstreamTransport(value), true
	default:
		response.BadRequest(c, "Invalid OpenAI upstream transport")
		return "", false
	}
}

func parseOpenAIImagesCapabilityQuery(c *gin.Context, key string) (service.OpenAIImagesCapability, bool) {
	value := strings.TrimSpace(c.Query(key))
	switch service.OpenAIImagesCapability(value) {
	case "", service.OpenAIImagesCapabilityBasic, service.OpenAIImagesCapabilityNative:
		return service.OpenAIImagesCapability(value), true
	default:
		response.BadRequest(c, "Invalid OpenAI image capability")
		return "", false
	}
}

func accountSchedulingPoolResponseFromService(snapshot service.OpenAIAccountSchedulingPoolSnapshot) AccountSchedulingPoolResponse {
	items := make([]AccountSchedulingPoolItem, 0, len(snapshot.Items))
	for i := range snapshot.Items {
		items = append(items, accountSchedulingPoolItemFromService(snapshot.Items[i]))
	}
	return AccountSchedulingPoolResponse{
		Items:            items,
		Total:            snapshot.Total,
		SchedulableCount: snapshot.SchedulableCount,
		DegradedCount:    snapshot.DegradedCount,
		BlockedCount:     snapshot.BlockedCount,
		FilteredCount:    snapshot.FilteredCount,
		GeneratedAt:      snapshot.GeneratedAt,
		GroupID:          cloneAccountSchedulingPoolInt64Ptr(snapshot.GroupID),
		Platform:         snapshot.Platform,
		Model:            snapshot.Model,
		Endpoint:         snapshot.Endpoint,
		Transport:        snapshot.Transport,
		ImageCapability:  snapshot.ImageCapability,
	}
}

func cloneAccountSchedulingPoolInt64Ptr(value *int64) *int64 {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func accountSchedulingPoolItemFromService(item service.OpenAIAccountSchedulingPoolItem) AccountSchedulingPoolItem {
	account := dto.AccountFromServiceShallow(&item.Account)
	if account != nil {
		account.DerivedHealth = dto.AccountDerivedHealthFromService(item.DerivedHealth)
		advice := service.NewAccountLoadFactorAdvisor(service.AccountLoadFactorAdvisorOptions{}).Advise(&item.Account, item.PathHealth)
		account.LoadFactorAdvice = dto.AccountLoadFactorAdviceFromService(advice)
	}
	return AccountSchedulingPoolItem{
		Account:             account,
		PoolStatus:          item.PoolStatus,
		PoolReasons:         item.PoolReasons,
		NextScheduledAt:     item.NextScheduledAt,
		NextScheduledReason: item.NextScheduledReason,
		RuntimeBlock:        item.RuntimeBlock,
		PathHealth:          item.PathHealth,
		PathHealthAvailable: item.PathHealthAvailable,
		DerivedHealth:       dto.AccountDerivedHealthFromService(item.DerivedHealth),
		EffectiveLoadFactor: item.EffectiveLoadFactor,
	}
}

// GetUsageSummary handles aggregated account-pool usage for the current filters.
// GET /api/v1/admin/accounts/usage-summary
func (h *AccountHandler) GetUsageSummary(c *gin.Context) {
	if h.accountUsageService == nil {
		response.InternalError(c, "Account usage service is not configured")
		return
	}

	platform := strings.TrimSpace(c.Query("platform"))
	accountType := c.Query("type")
	status := c.Query("status")
	search := strings.TrimSpace(c.Query("search"))
	privacyMode := strings.TrimSpace(c.Query("privacy_mode"))
	planType := strings.ToLower(strings.TrimSpace(c.Query("plan_type")))
	sortBy := c.DefaultQuery("sort_by", "name")
	sortOrder := c.DefaultQuery("sort_order", "asc")
	if len(search) > 100 {
		search = search[:100]
	}

	groupID, ok := parseAccountListGroupFilter(c)
	if !ok {
		return
	}

	accounts, err := h.listAccountsFiltered(c.Request.Context(), platform, accountType, status, search, groupID, privacyMode, planType, sortBy, sortOrder)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	summary, err := h.accountUsageService.GetAccountUsageSummary(c.Request.Context(), accounts)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, summary)
}

// GetDashboardSummary handles first-screen account dashboard aggregates.
// GET /api/v1/admin/accounts/dashboard-summary
func (h *AccountHandler) GetDashboardSummary(c *gin.Context) {
	accounts, err := h.listAccountsForCurrentFilters(c)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	statusSummaryAccounts := accounts
	if strings.TrimSpace(c.Query("status")) != "" {
		var statusSummaryErr error
		statusSummaryAccounts, statusSummaryErr = h.listAccountsForCurrentFiltersWithoutStatus(c)
		if statusSummaryErr != nil {
			response.ErrorFrom(c, statusSummaryErr)
			return
		}
	}

	var usageSummary *service.AccountUsageSummary
	var usageErr string
	if h.accountUsageService != nil {
		summary, err := h.accountUsageService.GetAccountUsageSummary(c.Request.Context(), accounts)
		if err != nil {
			usageErr = err.Error()
		} else {
			usageSummary = summary
		}
	} else {
		usageErr = "account usage service is not configured"
	}

	now := time.Now()
	result := service.BuildAccountDashboardSummary(accounts, usageSummary, now)
	if strings.TrimSpace(c.Query("status")) != "" {
		result.StatusSummary = service.BuildAccountDashboardSummary(statusSummaryAccounts, nil, now).StatusSummary
	}
	result.UsageSummaryError = usageErr
	response.Success(c, result)
}

// GetActionItems handles account action-item queue for the current filters.
// GET /api/v1/admin/accounts/action-items
func (h *AccountHandler) GetActionItems(c *gin.Context) {
	accounts, err := h.listAccountsForCurrentFilters(c)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, service.BuildAccountActionItems(accounts, time.Now()))
}

// RefreshUpstreamBalances refreshes upstream API-key balances for the current account filters.
// POST /api/v1/admin/accounts/refresh-upstream-balances
func (h *AccountHandler) RefreshUpstreamBalances(c *gin.Context) {
	if h.upstreamBalanceService == nil {
		response.InternalError(c, "Upstream balance service is not configured")
		return
	}

	result, err := h.upstreamBalanceService.RefreshAll(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// RefreshUpstreamBalance refreshes one upstream API-key account balance.
// POST /api/v1/admin/accounts/:id/refresh-upstream-balance
func (h *AccountHandler) RefreshUpstreamBalance(c *gin.Context) {
	if h.upstreamBalanceService == nil {
		response.InternalError(c, "Upstream balance service is not configured")
		return
	}

	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	if _, err := h.upstreamBalanceService.RefreshOne(c.Request.Context(), accountID); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	account, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, h.buildAccountResponseWithRuntime(c.Request.Context(), account))
}

func parseAccountListGroupFilter(c *gin.Context) (int64, bool) {
	groupID, err := parseAccountListGroupFilterValue(c.Query("group"))
	if err != nil {
		response.ErrorFrom(c, err)
		return 0, false
	}
	return groupID, true
}

func buildAccountsListETag(
	items []AccountWithConcurrency,
	total int64,
	page, pageSize int,
	platform, accountType, status, search string,
	lite bool,
) string {
	payload := struct {
		Total       int64                    `json:"total"`
		Page        int                      `json:"page"`
		PageSize    int                      `json:"page_size"`
		Platform    string                   `json:"platform"`
		AccountType string                   `json:"type"`
		Status      string                   `json:"status"`
		Search      string                   `json:"search"`
		Lite        bool                     `json:"lite"`
		Items       []AccountWithConcurrency `json:"items"`
	}{
		Total:       total,
		Page:        page,
		PageSize:    pageSize,
		Platform:    platform,
		AccountType: accountType,
		Status:      status,
		Search:      search,
		Lite:        lite,
		Items:       items,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return "\"" + hex.EncodeToString(sum[:]) + "\""
}

func ifNoneMatchMatched(ifNoneMatch, etag string) bool {
	if etag == "" || ifNoneMatch == "" {
		return false
	}
	for _, token := range strings.Split(ifNoneMatch, ",") {
		candidate := strings.TrimSpace(token)
		if candidate == "*" {
			return true
		}
		if candidate == etag {
			return true
		}
		if strings.HasPrefix(candidate, "W/") && strings.TrimPrefix(candidate, "W/") == etag {
			return true
		}
	}
	return false
}

// GetByID handles getting an account by ID
// GET /api/v1/admin/accounts/:id
func (h *AccountHandler) GetByID(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	account, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, h.buildAccountResponseWithRuntime(c.Request.Context(), account))
}

// CheckMixedChannel handles checking mixed channel risk for account-group binding.
// POST /api/v1/admin/accounts/check-mixed-channel
func (h *AccountHandler) CheckMixedChannel(c *gin.Context) {
	var req CheckMixedChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if len(req.GroupIDs) == 0 {
		response.Success(c, gin.H{"has_risk": false})
		return
	}

	accountID := int64(0)
	if req.AccountID != nil {
		accountID = *req.AccountID
	}

	err := h.adminService.CheckMixedChannelRisk(c.Request.Context(), accountID, req.Platform, req.GroupIDs)
	if err != nil {
		var mixedErr *service.MixedChannelError
		if errors.As(err, &mixedErr) {
			response.Success(c, gin.H{
				"has_risk": true,
				"error":    "mixed_channel_warning",
				"message":  mixedErr.Error(),
				"details": gin.H{
					"group_id":         mixedErr.GroupID,
					"group_name":       mixedErr.GroupName,
					"current_platform": mixedErr.CurrentPlatform,
					"other_platform":   mixedErr.OtherPlatform,
				},
			})
			return
		}

		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"has_risk": false})
}

// Create handles creating a new account
// POST /api/v1/admin/accounts
func (h *AccountHandler) Create(c *gin.Context) {
	var req CreateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.RateMultiplier != nil && *req.RateMultiplier < 0 {
		response.BadRequest(c, "rate_multiplier must be >= 0")
		return
	}
	// base_rpm 输入校验：负值归零，超过 10000 截断
	sanitizeExtraBaseRPM(req.Extra)

	// 确定是否跳过混合渠道检查
	skipCheck := req.ConfirmMixedChannelRisk != nil && *req.ConfirmMixedChannelRisk

	// 捕获闭包内创建的账号引用，用于创建成功后触发异步探测。
	// 幂等重放时闭包不会执行 → createdAccount 为 nil → 不重复调度。
	var createdAccount *service.Account

	result, err := executeAdminIdempotent(c, "admin.accounts.create", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		account, execErr := h.adminService.CreateAccount(ctx, &service.CreateAccountInput{
			Name:                  req.Name,
			Notes:                 req.Notes,
			Platform:              req.Platform,
			Type:                  req.Type,
			Credentials:           req.Credentials,
			Extra:                 req.Extra,
			ProxyID:               req.ProxyID,
			Concurrency:           req.Concurrency,
			Priority:              req.Priority,
			RateMultiplier:        req.RateMultiplier,
			LoadFactor:            req.LoadFactor,
			GroupIDs:              req.GroupIDs,
			ExpiresAt:             req.ExpiresAt,
			AutoPauseOnExpired:    req.AutoPauseOnExpired,
			SkipMixedChannelCheck: skipCheck,
		})
		if execErr != nil {
			return nil, execErr
		}
		createdAccount = account
		// Antigravity OAuth: 新账号直接设置隐私
		h.adminService.ForceAntigravityPrivacy(ctx, account)
		// OpenAI OAuth: 新账号直接设置隐私
		h.adminService.ForceOpenAIPrivacy(ctx, account)
		return h.buildAccountResponseWithRuntime(ctx, account), nil
	})
	if err != nil {
		// 检查是否为混合渠道错误
		var mixedErr *service.MixedChannelError
		if errors.As(err, &mixedErr) {
			// 创建接口仅返回最小必要字段，详细信息由专门检查接口提供
			c.JSON(409, gin.H{
				"error":   "mixed_channel_warning",
				"message": mixedErr.Error(),
			})
			return
		}

		if retryAfter := service.RetryAfterSecondsFromError(err); retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		response.ErrorFrom(c, err)
		return
	}

	if result != nil && result.Replayed {
		c.Header("X-Idempotency-Replayed", "true")
	}
	// OpenAI APIKey 账号创建后异步探测上游 /v1/responses 能力。
	// 探测失败不影响账号创建响应。
	h.scheduleOpenAIResponsesProbe(createdAccount)
	response.Success(c, result.Data)
}

// Update handles updating an account
// PUT /api/v1/admin/accounts/:id
func (h *AccountHandler) Update(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	var req UpdateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.RateMultiplier != nil && *req.RateMultiplier < 0 {
		response.BadRequest(c, "rate_multiplier must be >= 0")
		return
	}
	// base_rpm 输入校验：负值归零，超过 10000 截断
	sanitizeExtraBaseRPM(req.Extra)

	// 确定是否跳过混合渠道检查
	skipCheck := req.ConfirmMixedChannelRisk != nil && *req.ConfirmMixedChannelRisk

	account, err := h.adminService.UpdateAccount(c.Request.Context(), accountID, &service.UpdateAccountInput{
		Name:                  req.Name,
		Notes:                 req.Notes,
		Platform:              req.Platform,
		Type:                  req.Type,
		Credentials:           req.Credentials,
		Extra:                 req.Extra,
		ProxyID:               req.ProxyID,
		Concurrency:           req.Concurrency, // 指针类型，nil 表示未提供
		Priority:              req.Priority,    // 指针类型，nil 表示未提供
		RateMultiplier:        req.RateMultiplier,
		LoadFactor:            req.LoadFactor,
		Status:                req.Status,
		GroupIDs:              req.GroupIDs,
		ExpiresAt:             req.ExpiresAt,
		AutoPauseOnExpired:    req.AutoPauseOnExpired,
		SkipMixedChannelCheck: skipCheck,
	})
	if err != nil {
		// 检查是否为混合渠道错误
		var mixedErr *service.MixedChannelError
		if errors.As(err, &mixedErr) {
			// 更新接口仅返回最小必要字段，详细信息由专门检查接口提供
			c.JSON(409, gin.H{
				"error":   "mixed_channel_warning",
				"message": mixedErr.Error(),
			})
			return
		}

		response.ErrorFrom(c, err)
		return
	}

	// OpenAI APIKey: credentials 修改后重新探测上游能力（base_url/api_key 可能变更）。
	// 异步执行，探测失败不影响账号更新响应。
	if len(req.Credentials) > 0 {
		h.scheduleOpenAIResponsesProbe(account)
	}

	response.Success(c, h.buildAccountResponseWithRuntime(c.Request.Context(), account))
}

// scheduleOpenAIResponsesProbe 异步触发 OpenAI APIKey 账号的 Responses API 能力探测。
//
// 仅对 platform=openai && type=apikey 账号生效；其他账号无操作。
// 探测本身在 goroutine 中执行（会发一次 HTTP 请求到上游），不会阻塞
// 当前请求。探测错误仅记录日志，不向上下文传播：探测失败时标记保持缺失，
// 网关会按"现状即证据"默认走 Responses。
func (h *AccountHandler) scheduleOpenAIResponsesProbe(account *service.Account) {
	if account == nil || account.Platform != service.PlatformOpenAI || account.Type != service.AccountTypeAPIKey {
		return
	}
	if h.accountTestService == nil {
		return
	}
	accountID := account.ID
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("openai_responses_probe_panic", "account_id", accountID, "recover", r)
			}
		}()
		h.accountTestService.ProbeOpenAIAPIKeyResponsesSupport(context.Background(), accountID)
	}()
}

// Delete handles deleting an account
// DELETE /api/v1/admin/accounts/:id
func (h *AccountHandler) Delete(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	err = h.adminService.DeleteAccount(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "Account deleted successfully"})
}

// DeleteAPIKey 删除账号中指定指纹的单个已保存 API Key。
// DELETE /api/v1/admin/accounts/:id/api-keys/:fingerprint
func (h *AccountHandler) DeleteAPIKey(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	fingerprint := strings.TrimSpace(c.Param("fingerprint"))
	if fingerprint == "" {
		response.BadRequest(c, "Invalid API key fingerprint")
		return
	}

	account, err := h.adminService.DeleteAccountAPIKey(c.Request.Context(), accountID, fingerprint)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, h.buildAccountResponseWithRuntime(c.Request.Context(), account))
}

// RestoreAPIKeyState 恢复账号中指定指纹的单个已停用 API Key。
// POST /api/v1/admin/accounts/:id/api-keys/:fingerprint/restore-state
func (h *AccountHandler) RestoreAPIKeyState(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	fingerprint := strings.TrimSpace(c.Param("fingerprint"))
	if fingerprint == "" {
		response.BadRequest(c, "Invalid API key fingerprint")
		return
	}

	account, err := h.adminService.RestoreAccountAPIKeyState(c.Request.Context(), accountID, fingerprint)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, h.buildAccountResponseWithRuntime(c.Request.Context(), account))
}

// TestAccountRequest represents the request body for testing an account
type TestAccountRequest struct {
	ModelID string `json:"model_id"`
	Prompt  string `json:"prompt"`
	Mode    string `json:"mode"`
}

type BatchTestNonAPIKeyAccountsRequest struct {
	ModelID     string  `json:"model_id"`
	Platform    string  `json:"platform"`
	Status      string  `json:"status"`
	Search      string  `json:"search"`
	Group       string  `json:"group"`
	PlanType    string  `json:"plan_type"`
	AccountIDs  []int64 `json:"account_ids"`
	Concurrency int     `json:"concurrency"`
	Limit       int     `json:"limit"`
}

const defaultBatchTestNonAPIKeyModelID = openai.DefaultTestModel

type CreateAccountProbeRunRequest struct {
	Mode                  string `json:"mode"`
	Model                 string `json:"model"`
	IncludeCodexStability bool   `json:"include_codex_stability"`
	IncludeLongContext    bool   `json:"include_long_context"`
	CodexStability        bool   `json:"codex_stability"`
	LongContext           bool   `json:"long_context"`
	RequestMode           string `json:"request_mode"`
}

type BatchCreateAccountProbeRunsRequest struct {
	AccountIDs            []int64 `json:"account_ids"`
	Mode                  string  `json:"mode"`
	Model                 string  `json:"model"`
	IncludeCodexStability bool    `json:"include_codex_stability"`
	IncludeLongContext    bool    `json:"include_long_context"`
	CodexStability        bool    `json:"codex_stability"`
	LongContext           bool    `json:"long_context"`
	RequestMode           string  `json:"request_mode"`
}

type CreateAccountModelProbeRunRequest struct {
	AccountID           int64  `json:"account_id" binding:"required"`
	Model               string `json:"model"`
	RequestMode         string `json:"request_mode"`
	TrustedComparisonID int64  `json:"trusted_comparison_account_id"`
}

type CreateBazaarLinkModelProbeRunRequest struct {
	AccountID int64  `json:"account_id" binding:"required"`
	Model     string `json:"model"`
	Mode      string `json:"mode"`
}

type BatchCreateAccountModelProbeRunsRequest struct {
	AccountIDs          []int64 `json:"account_ids"`
	Model               string  `json:"model"`
	RequestMode         string  `json:"request_mode"`
	TrustedComparisonID int64   `json:"trusted_comparison_account_id"`
}

type batchAccountProbeSkip struct {
	AccountID int64  `json:"account_id"`
	Message   string `json:"message"`
}

type DeleteAccountProbeRunsRequest struct {
	RunIDs []int64 `json:"run_ids"`
}

type SyncFromCRSRequest struct {
	BaseURL            string   `json:"base_url" binding:"required"`
	Username           string   `json:"username" binding:"required"`
	Password           string   `json:"password" binding:"required"`
	SyncProxies        *bool    `json:"sync_proxies"`
	SelectedAccountIDs []string `json:"selected_account_ids"`
}

type PreviewFromCRSRequest struct {
	BaseURL  string `json:"base_url" binding:"required"`
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Test handles testing account connectivity with SSE streaming
// POST /api/v1/admin/accounts/:id/test
func (h *AccountHandler) Test(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	var req TestAccountRequest
	// Allow empty body, model_id is optional
	_ = c.ShouldBindJSON(&req)

	result, testErr := h.accountTestService.TestAccountConnectionWithResult(c, accountID, req.ModelID, req.Prompt, req.Mode)
	if h.rateLimitService != nil && result != nil {
		transition, err := h.rateLimitService.RecordAccountProbeOutcome(c.Request.Context(), service.AccountProbeOutcome{
			AccountID:    accountID,
			Source:       service.AccountProbeOutcomeSourceManualTest,
			Success:      result.Success,
			ErrorMessage: result.ErrorMessage,
			HTTPStatus:   result.HTTPStatus,
			Reason:       result.Reason,
			LatencyMs:    result.LatencyMs,
			FirstTokenMs: result.FirstTokenMs,
			ObservedAt:   result.FinishedAt,
		})
		if err != nil {
			_ = c.Error(err)
		} else {
			h.sendAccountTestTransitionEvent(c, transition)
		}
	}
	if testErr != nil {
		// Error already sent via SSE; state transition has been recorded above.
		return
	}
}

func (h *AccountHandler) sendAccountTestTransitionEvent(c *gin.Context, transition *service.AccountProbeTransitionResult) {
	if h == nil || h.accountTestService == nil || c == nil || transition == nil {
		return
	}
	before := service.AccountProbeHealthLabel(transition.PreviousLevel)
	after := service.AccountProbeHealthLabel(transition.NextLevel)
	text := fmt.Sprintf("账号状态：%s -> %s", before, after)
	if !transition.StateChanged {
		text = fmt.Sprintf("账号状态保持：%s", after)
	}
	if transition.Reason != "" {
		text = fmt.Sprintf("%s（%s）", text, transition.Reason)
	}
	h.accountTestService.SendTestEvent(c, service.TestEvent{
		Type:         "status",
		Text:         text,
		StateBefore:  transition.PreviousLevel,
		StateAfter:   transition.NextLevel,
		StateChanged: transition.StateChanged,
		StateReason:  transition.Reason,
		BlockedUntil: transition.BlockedUntil,
	})
}

type ManualProbeRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Mode   string `json:"mode"`
}

// ManualProbeResultResponse 是调度池人工探测给前端消费的稳定 JSON 契约。
type ManualProbeResultResponse struct {
	Success      bool   `json:"success"`
	Message      string `json:"message,omitempty"`
	Error        string `json:"error,omitempty"`
	LatencyMS    *int   `json:"latency_ms,omitempty"`
	FirstTokenMS *int   `json:"first_token_ms,omitempty"`
	HTTPStatus   int    `json:"http_status,omitempty"`
	Reason       string `json:"reason,omitempty"`
}

type ManualProbeResponse struct {
	Success bool                       `json:"success"`
	Result  *ManualProbeResultResponse `json:"result,omitempty"`
	Account *dto.Account               `json:"account,omitempty"`
}

// manualProbeResultResponseFromService 把账号测试内部结构转换为前端读取的小写字段。
func manualProbeResultResponseFromService(result *service.AccountTestConnectionResult) *ManualProbeResultResponse {
	if result == nil {
		return nil
	}
	resp := &ManualProbeResultResponse{
		Success:      result.Success,
		LatencyMS:    result.LatencyMs,
		FirstTokenMS: result.FirstTokenMs,
		HTTPStatus:   result.HTTPStatus,
		Reason:       result.Reason,
	}
	if result.ErrorMessage != "" {
		resp.Message = result.ErrorMessage
		resp.Error = result.ErrorMessage
	}
	return resp
}

// ManualProbe handles manual probe requests from frontend
// POST /api/v1/admin/accounts/:id/manual-probe
func (h *AccountHandler) ManualProbe(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	var req ManualProbeRequest
	_ = c.ShouldBindJSON(&req)

	if req.Prompt == "" {
		req.Prompt = service.RandomQuickValidationPrompt()
	}

	account, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		response.InternalError(c, "Failed to get account")
		return
	}

	result, testErr := h.accountTestService.TestAccountConnectionWithResultBackground(c.Request.Context(), accountID, req.Model, req.Prompt, req.Mode)
	if testErr != nil && result == nil {
		response.InternalError(c, "Failed to test account")
		return
	}
	if subject, ok := middleware.GetAuthSubjectFromContext(c); ok && h.usageService != nil {
		model := strings.TrimSpace(result.Model)
		if model == "" {
			model = strings.TrimSpace(req.Model)
		}
		if model == "" {
			model = "manual-probe"
		}
		durationMs := result.LatencyMs
		inboundEndpoint := "/api/v1/admin/accounts/:id/manual-probe"
		upstreamEndpoint := manualProbeUpstreamEndpoint(account)
		_, err := h.usageService.Create(c.Request.Context(), service.CreateUsageLogRequest{
			UserID: subject.UserID, AccountID: accountID, RequestID: fmt.Sprintf("manual-probe:%d:%d", subject.UserID, result.StartedAt.UnixNano()),
			Model: model, InputTokens: result.InputTokens, OutputTokens: result.OutputTokens, Stream: result.Stream,
			DurationMs: durationMs, FirstTokenMs: result.FirstTokenMs, InboundEndpoint: &inboundEndpoint, UpstreamEndpoint: &upstreamEndpoint,
		})
		if err != nil {
			slog.Error("记录人工上游探测使用日志失败", "user_id", subject.UserID, "account_id", accountID, "error", err)
		}
	}

	var updatedAccount *service.Account
	if result.Success && h.rateLimitService != nil {
		if _, err := h.rateLimitService.RecordAccountProbeOutcome(c.Request.Context(), service.AccountProbeOutcome{
			AccountID:    accountID,
			Account:      account,
			Source:       service.AccountProbeOutcomeSourceManualTest,
			Success:      true,
			ErrorMessage: result.ErrorMessage,
			HTTPStatus:   result.HTTPStatus,
			Reason:       result.Reason,
			LatencyMs:    result.LatencyMs,
			FirstTokenMs: result.FirstTokenMs,
			ObservedAt:   result.FinishedAt,
		}); err != nil {
			response.InternalError(c, "Failed to recover account state")
			return
		}
	}
	updatedAccount, err = h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		response.InternalError(c, "Failed to get account")
		return
	}

	resp := ManualProbeResponse{
		Success: result.Success,
		Result:  manualProbeResultResponseFromService(result),
		Account: dto.AccountFromService(updatedAccount),
	}

	c.JSON(http.StatusOK, resp)
}

// manualProbeUpstreamEndpoint 返回人工探测使用的标准上游端点。
func manualProbeUpstreamEndpoint(account *service.Account) string {
	if account != nil && account.IsOpenAI() {
		return "/v1/responses"
	}
	if account != nil && account.IsGemini() {
		return "/v1beta/models"
	}
	return "/v1/messages"
}

// BatchTestNonAPIKey tests all non-api-key accounts matching optional filters.
// POST /api/v1/admin/accounts/batch-test-non-apikey
func (h *AccountHandler) BatchTestNonAPIKey(c *gin.Context) {
	if h.batchAccountTester == nil {
		response.InternalError(c, "Account test service is not configured")
		return
	}
	if h.accountBatchTestRepo == nil {
		response.InternalError(c, "Account batch test repository is not configured")
		return
	}
	var req BatchTestNonAPIKeyAccountsRequest
	_ = c.ShouldBindJSON(&req)
	platform := strings.TrimSpace(req.Platform)
	status := strings.TrimSpace(req.Status)
	search := strings.TrimSpace(req.Search)
	limit := req.Limit
	if limit <= 0 || limit > 500 {
		limit = 500
	}
	concurrency := normalizeBatchTestNonAPIKeyConcurrency(req.Concurrency)
	modelID := normalizeBatchTestNonAPIKeyModelID(req.ModelID)

	targets, err := h.resolveBatchTestNonAPIKeyTargets(c.Request.Context(), req, limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	now := time.Now()
	run := service.AccountBatchTestRun{
		Status:       service.AccountBatchTestStatusRunning,
		ModelID:      modelID,
		Platform:     platform,
		StatusFilter: status,
		Search:       search,
		Concurrency:  concurrency,
		Limit:        limit,
		Total:        len(targets),
		CreatedAt:    now,
		StartedAt:    &now,
	}
	if err := h.accountBatchTestRepo.CreateAccountBatchTestRun(c.Request.Context(), &run); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	items := make([]service.AccountBatchTestItem, len(targets))
	for i, account := range targets {
		items[i] = service.AccountBatchTestItem{
			RunID:       run.ID,
			AccountID:   account.ID,
			AccountName: account.Name,
			Platform:    account.Platform,
			Type:        account.Type,
			GroupID:     firstAccountGroupID(account),
			Status:      service.AccountBatchTestItemStatusPending,
			CreatedAt:   now,
		}
	}
	if err := h.accountBatchTestRepo.CreateAccountBatchTestItems(c.Request.Context(), run.ID, items); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	go h.runBatchTestNonAPIKeyBackground(run, items)
	response.Accepted(c, run)
}

func (h *AccountHandler) resolveBatchTestNonAPIKeyTargets(ctx context.Context, req BatchTestNonAPIKeyAccountsRequest, limit int) ([]service.Account, error) {
	if len(req.AccountIDs) > 0 {
		ids := dedupePositiveAccountIDs(req.AccountIDs)
		accounts, err := h.adminService.GetAccountsByIDs(ctx, ids)
		if err != nil {
			return nil, err
		}
		targets := make([]service.Account, 0, len(accounts))
		for _, account := range accounts {
			if account == nil || account.Type == service.AccountTypeAPIKey {
				continue
			}
			targets = append(targets, *account)
		}
		return targets, nil
	}

	groupID, err := parseAccountListGroupFilterValue(strings.TrimSpace(req.Group))
	if err != nil {
		return nil, err
	}
	accounts, _, err := h.adminService.ListAccounts(
		ctx,
		1,
		limit,
		strings.TrimSpace(req.Platform),
		"",
		strings.TrimSpace(req.Status),
		strings.TrimSpace(req.Search),
		groupID,
		"",
		strings.ToLower(strings.TrimSpace(req.PlanType)),
		"name",
		"asc",
	)
	if err != nil {
		return nil, err
	}
	targets := make([]service.Account, 0, len(accounts))
	for _, account := range accounts {
		if account.Type == service.AccountTypeAPIKey {
			continue
		}
		targets = append(targets, account)
	}
	return targets, nil
}

func normalizeBatchTestNonAPIKeyConcurrency(concurrency int) int {
	if concurrency <= 0 {
		return 5
	}
	if concurrency > 20 {
		return 20
	}
	return concurrency
}

func normalizeBatchTestNonAPIKeyModelID(modelID string) string {
	normalized := strings.TrimSpace(modelID)
	if normalized == "" {
		return defaultBatchTestNonAPIKeyModelID
	}
	return normalized
}

func dedupePositiveAccountIDs(ids []int64) []int64 {
	out := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func parseAccountListGroupFilterValue(groupIDStr string) (int64, error) {
	if groupIDStr == "" {
		return 0, nil
	}
	if groupIDStr == accountListGroupUngroupedQueryValue {
		return service.AccountListGroupUngrouped, nil
	}
	parsedGroupID, parseErr := strconv.ParseInt(groupIDStr, 10, 64)
	if parseErr != nil || parsedGroupID < 0 {
		return 0, infraerrors.BadRequest("INVALID_GROUP_FILTER", "invalid group filter")
	}
	return parsedGroupID, nil
}

func (h *AccountHandler) runBatchTestNonAPIKeyBackground(run service.AccountBatchTestRun, items []service.AccountBatchTestItem) {
	if h.batchAccountTester == nil || h.accountBatchTestRepo == nil {
		return
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			finishedAt := time.Now()
			run.Status = service.AccountBatchTestStatusFailed
			run.ErrorMessage = fmt.Sprint(recovered)
			run.FinishedAt = &finishedAt
			_ = h.accountBatchTestRepo.UpdateAccountBatchTestRun(context.Background(), &run)
			slog.Error("account_batch_test_background_panic", "run_id", run.ID, "recover", recovered)
		}
	}()
	concurrency := run.Concurrency
	concurrency = normalizeBatchTestNonAPIKeyConcurrency(concurrency)
	modelID := normalizeBatchTestNonAPIKeyModelID(run.ModelID)
	limiter := h.accountBatchTestLimiter()
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i := range items {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			startedAt := time.Now()
			items[i].Status = service.AccountBatchTestItemStatusRunning
			items[i].StartedAt = &startedAt
			_ = h.accountBatchTestRepo.UpdateAccountBatchTestItem(context.Background(), items[i])

			if pauseReason := h.batchTestLiveTrafficPauseReason(context.Background(), items[i]); pauseReason != "" {
				finishedAt := time.Now()
				items[i].FinishedAt = &finishedAt
				items[i].Status = service.AccountBatchTestItemStatusFailed
				items[i].Category = "batch_paused"
				items[i].ErrorMessage = pauseReason
				_ = h.accountBatchTestRepo.UpdateAccountBatchTestItem(context.Background(), items[i])
				return
			}

			release, acquireErr := limiter.Acquire(context.Background(), batchTestNonAPIKeyLimiterKey(items[i]))
			if acquireErr != nil {
				finishedAt := time.Now()
				items[i].FinishedAt = &finishedAt
				items[i].Status = service.AccountBatchTestItemStatusFailed
				items[i].Category = "batch_paused"
				items[i].ErrorMessage = acquireErr.Error()
				_ = h.accountBatchTestRepo.UpdateAccountBatchTestItem(context.Background(), items[i])
				return
			}
			defer release()

			result, runErr := h.batchAccountTester.RunTestBackground(context.Background(), items[i].AccountID, modelID)
			finishedAt := time.Now()
			items[i].FinishedAt = &finishedAt
			if runErr != nil {
				items[i].Status = service.AccountBatchTestItemStatusFailed
				items[i].Category = classifyAccountTestError(runErr.Error())
				items[i].ErrorMessage = runErr.Error()
				limiter.RecordResult(batchTestNonAPIKeyLimiterKey(items[i]), items[i].Category)
				_ = h.accountBatchTestRepo.UpdateAccountBatchTestItem(context.Background(), items[i])
				return
			}
			if result == nil {
				items[i].Status = service.AccountBatchTestItemStatusFailed
				items[i].Category = "error"
				items[i].ErrorMessage = "empty test result"
				limiter.RecordResult(batchTestNonAPIKeyLimiterKey(items[i]), items[i].Category)
				_ = h.accountBatchTestRepo.UpdateAccountBatchTestItem(context.Background(), items[i])
				return
			}
			items[i].LatencyMs = int(result.LatencyMs)
			items[i].FirstTokenMs = result.FirstTokenMs
			if result.Status == service.AccountBatchTestItemStatusSuccess {
				items[i].Status = service.AccountBatchTestItemStatusSuccess
				items[i].Category = "ok"
				items[i].Message = result.ResponseText
				limiter.RecordResult(batchTestNonAPIKeyLimiterKey(items[i]), items[i].Category)
				_ = h.accountBatchTestRepo.UpdateAccountBatchTestItem(context.Background(), items[i])
				return
			}
			items[i].Status = service.AccountBatchTestItemStatusFailed
			items[i].Category = classifyAccountTestError(result.ErrorMessage)
			items[i].ErrorMessage = result.ErrorMessage
			limiter.RecordResult(batchTestNonAPIKeyLimiterKey(items[i]), items[i].Category)
			_ = h.accountBatchTestRepo.UpdateAccountBatchTestItem(context.Background(), items[i])
		}()
	}
	wg.Wait()

	run.Total = len(items)
	for _, item := range items {
		if item.Status == service.AccountBatchTestItemStatusSuccess {
			run.SuccessCount++
			continue
		}
		run.FailedCount++
		if item.Category == "unauthorized" {
			run.UnauthorizedCount++
		}
		if item.Category == "rate_limited" {
			run.RateLimitedCount++
		}
	}
	if run.FailedCount == 0 {
		run.Status = service.AccountBatchTestStatusSuccess
	} else if run.SuccessCount == 0 {
		run.Status = service.AccountBatchTestStatusFailed
	} else {
		run.Status = service.AccountBatchTestStatusPartial
	}
	finishedAt := time.Now()
	run.FinishedAt = &finishedAt
	_ = h.accountBatchTestRepo.UpdateAccountBatchTestRun(context.Background(), &run)
}

func (h *AccountHandler) accountBatchTestLimiter() *accountBatchTestLimiter {
	return nonAPIKeyBatchTestLimiter
}

func (h *AccountHandler) batchTestLiveTrafficPauseReason(ctx context.Context, item service.AccountBatchTestItem) string {
	if h == nil || h.concurrencyService == nil || item.AccountID <= 0 {
		return ""
	}
	loadMap, err := h.concurrencyService.GetAccountsLoadBatchFresh(ctx, []service.AccountWithConcurrency{
		{ID: item.AccountID, MaxConcurrency: 1},
	})
	if err != nil {
		slog.Warn("account_batch_test_live_pressure_check_failed", "account_id", item.AccountID, "err", err)
		return ""
	}
	load := loadMap[item.AccountID]
	if load == nil {
		return ""
	}
	if load.CurrentConcurrency <= 0 && load.WaitingCount <= 0 {
		return ""
	}
	return fmt.Sprintf("batch test paused because live traffic is using account %d (active=%d waiting=%d)", item.AccountID, load.CurrentConcurrency, load.WaitingCount)
}

func batchTestNonAPIKeyLimiterKey(item service.AccountBatchTestItem) string {
	platform := strings.ToLower(strings.TrimSpace(item.Platform))
	if platform == "" {
		platform = "unknown"
	}
	accountType := strings.ToLower(strings.TrimSpace(item.Type))
	if accountType == "" {
		accountType = "unknown"
	}
	group := "ungrouped"
	if item.GroupID > 0 {
		group = strconv.FormatInt(item.GroupID, 10)
	}
	return platform + ":" + accountType + ":group:" + group
}

func firstAccountGroupID(account service.Account) int64 {
	if len(account.GroupIDs) > 0 {
		return account.GroupIDs[0]
	}
	if len(account.AccountGroups) > 0 {
		return account.AccountGroups[0].GroupID
	}
	if len(account.Groups) > 0 && account.Groups[0] != nil {
		return account.Groups[0].ID
	}
	return 0
}

func (h *AccountHandler) ListBatchTestRuns(c *gin.Context) {
	if h.accountBatchTestRepo == nil {
		response.InternalError(c, "Account batch test repository is not configured")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	items, total, err := h.accountBatchTestRepo.ListAccountBatchTestRuns(c.Request.Context(), service.AccountBatchTestRunFilter{
		Page:     page,
		PageSize: pageSize,
		Status:   c.Query("status"),
		Keyword:  c.Query("keyword"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, service.AccountBatchTestRunPage{Items: items, Total: total, Page: page, PageSize: pageSize})
}

func (h *AccountHandler) GetBatchTestRun(c *gin.Context) {
	if h.accountBatchTestRepo == nil {
		response.InternalError(c, "Account batch test repository is not configured")
		return
	}
	runID, err := strconv.ParseInt(c.Param("run_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid run ID")
		return
	}
	run, items, err := h.accountBatchTestRepo.GetAccountBatchTestRun(c.Request.Context(), runID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	category := strings.TrimSpace(c.Query("category"))
	if category != "" {
		filtered := make([]service.AccountBatchTestItem, 0, len(items))
		for _, item := range items {
			if accountBatchTestItemMatchesCategory(item, category) {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	response.Success(c, service.AccountBatchTestRunDetail{AccountBatchTestRun: *run, Items: items})
}

func classifyAccountTestError(message string) string {
	classification := service.ClassifyUpstreamError(service.UpstreamErrorInput{Message: message})
	if classification.Category == service.UpstreamErrorCategoryOK {
		return "error"
	}
	return classification.Category
}

func accountBatchTestItemMatchesCategory(item service.AccountBatchTestItem, category string) bool {
	if category == "" {
		return true
	}
	if category == "ok" && item.Category != "ok" && item.Status == service.AccountBatchTestItemStatusSuccess {
		return true
	}
	if category == "rate_limited" && accountBatchTestItemIsRateLimited(item) {
		return true
	}
	if category == "unauthorized" && accountBatchTestItemIsUnauthorized(item) {
		return true
	}
	return item.Category == category
}

func accountBatchTestItemIsRateLimited(item service.AccountBatchTestItem) bool {
	classification := service.ClassifyUpstreamError(service.UpstreamErrorInput{Message: firstNonEmptyBatchTestMessage(item)})
	return item.Category == service.UpstreamErrorCategoryRateLimited ||
		item.Category == service.UpstreamErrorCategoryClientIPCircuitOpen ||
		classification.RateLimited
}

func accountBatchTestItemIsUnauthorized(item service.AccountBatchTestItem) bool {
	classification := service.ClassifyUpstreamError(service.UpstreamErrorInput{Message: firstNonEmptyBatchTestMessage(item)})
	return item.Category == service.UpstreamErrorCategoryUnauthorized || classification.AccountInvalid
}

func firstNonEmptyBatchTestMessage(item service.AccountBatchTestItem) string {
	if msg := strings.TrimSpace(item.ErrorMessage); msg != "" {
		return msg
	}
	return strings.TrimSpace(item.Message)
}

// CreateProbeRun runs and persists an upstream account probe.
// POST /api/v1/admin/accounts/:id/probe-runs
func (h *AccountHandler) CreateProbeRun(c *gin.Context) {
	if h.accountProbeService == nil {
		response.InternalError(c, "Account probe service is not configured")
		return
	}
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	var req CreateAccountProbeRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = service.AccountProbeProfileStandard
	}
	probeReq := service.AccountProbeRunRequest{
		AccountID:          accountID,
		Profile:            mode,
		Model:              req.Model,
		IncludeLongContext: req.IncludeLongContext || req.LongContext,
		RequestMode:        req.RequestMode,
	}
	result, err := h.accountProbeService.Start(c.Request.Context(), probeReq)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	go h.runAccountProbeBackground(result, probeReq)
	response.Accepted(c, result)
}

// CreateModelProbeRun 手动执行账号模型行为探针。
// POST /api/v1/admin/account-model-probe-runs
func (h *AccountHandler) CreateModelProbeRun(c *gin.Context) {
	if h.accountProbeService == nil {
		response.InternalError(c, "Account probe service is not configured")
		return
	}
	var req CreateAccountModelProbeRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	probeReq := service.AccountProbeRunRequest{
		AccountID:           req.AccountID,
		Profile:             service.AccountProbeProfileModelValidation,
		Model:               req.Model,
		RequestMode:         req.RequestMode,
		TrustedComparisonID: req.TrustedComparisonID,
		ModelValidationOnly: true,
	}
	result, err := h.accountProbeService.Start(c.Request.Context(), probeReq)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	go h.runAccountProbeBackground(result, probeReq)
	response.Accepted(c, result)
}

// BatchCreateModelProbeRuns 手动批量执行账号模型行为探针。
// POST /api/v1/admin/account-model-probe-runs/batch
func (h *AccountHandler) BatchCreateModelProbeRuns(c *gin.Context) {
	if h.accountProbeService == nil {
		response.InternalError(c, "Account probe service is not configured")
		return
	}
	var req BatchCreateAccountModelProbeRunsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	accountIDs := uniquePositiveInt64s(req.AccountIDs)
	if len(accountIDs) == 0 {
		response.BadRequest(c, "account_ids is required")
		return
	}
	if len(accountIDs) > 50 {
		response.BadRequest(c, "account_ids cannot exceed 50")
		return
	}
	runs := make([]service.AccountProbeResult, 0, len(accountIDs))
	skipped := make([]batchAccountProbeSkip, 0)
	for _, accountID := range accountIDs {
		probeReq := service.AccountProbeRunRequest{
			AccountID:           accountID,
			Profile:             service.AccountProbeProfileModelValidation,
			Model:               req.Model,
			RequestMode:         req.RequestMode,
			TrustedComparisonID: req.TrustedComparisonID,
			ModelValidationOnly: true,
		}
		run, err := h.accountProbeService.Start(c.Request.Context(), probeReq)
		if err != nil {
			skipped = append(skipped, batchAccountProbeSkip{AccountID: accountID, Message: err.Error()})
			continue
		}
		runs = append(runs, run)
	}
	if len(runs) == 0 {
		if len(skipped) > 0 {
			response.InternalError(c, skipped[0].Message)
			return
		}
		response.BadRequest(c, "No account model probe runs accepted")
		return
	}
	go h.runAccountModelProbeBatchBackground(runs, req)
	response.Accepted(c, gin.H{
		"accepted_count": len(runs),
		"skipped_count":  len(skipped),
		"skipped":        skipped,
		"runs":           runs,
	})
}

// CreateBazaarLinkModelProbeRun 创建 BazaarLink 外部 API 模型验证后台任务。
// POST /api/v1/admin/account-model-probe-runs/bazaarlink
func (h *AccountHandler) CreateBazaarLinkModelProbeRun(c *gin.Context) {
	if h.accountProbeService == nil {
		response.InternalError(c, "Account probe service is not configured")
		return
	}
	var req CreateBazaarLinkModelProbeRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	probeReq := service.BazaarLinkProbeRunRequest{
		AccountID: req.AccountID,
		Model:     req.Model,
		Mode:      service.BazaarLinkProbeMode(req.Mode),
	}
	result, err := h.accountProbeService.StartBazaarLink(c.Request.Context(), probeReq)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	go h.runBazaarLinkProbeBackground(result, probeReq)
	response.Accepted(c, result)
}

func (h *AccountHandler) runAccountProbeBackground(run service.AccountProbeResult, req service.AccountProbeRunRequest) {
	if h == nil || h.accountProbeService == nil {
		return
	}
	bgCtx, cancel := context.WithTimeout(context.Background(), accountProbeBackgroundTimeout(req))
	defer cancel()
	if _, err := h.accountProbeService.RunExisting(bgCtx, run, req); err != nil {
		slog.Warn("account probe background run failed",
			"account_id", req.AccountID,
			"run_id", run.ID,
			"error", err.Error(),
		)
	}
}

func (h *AccountHandler) runBazaarLinkProbeBackground(run service.AccountProbeResult, req service.BazaarLinkProbeRunRequest) {
	if h == nil || h.accountProbeService == nil {
		return
	}
	bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if _, err := h.accountProbeService.RunBazaarLinkExisting(bgCtx, run, req); err != nil {
		slog.Warn("bazaarlink account probe background run failed",
			"account_id", req.AccountID,
			"run_id", run.ID,
			"error", err.Error(),
		)
	}
}

func accountProbeBackgroundTimeout(req service.AccountProbeRunRequest) time.Duration {
	timeout := 2 * time.Minute
	if req.ModelValidationOnly {
		timeout = 6 * time.Minute
	}
	if req.ModelValidationOnly && req.TrustedComparisonID > 0 {
		timeout = accountProbeRunTimeout
	}
	if strings.EqualFold(strings.TrimSpace(req.Profile), service.AccountProbeProfileStandard) {
		timeout = 4 * time.Minute
	}
	if req.IncludeLongContext {
		timeout += 4 * time.Minute
	}
	if timeout > accountProbeRunTimeout {
		timeout = accountProbeRunTimeout
	}
	return timeout
}

// ListProbeRuns returns persisted probe history for an upstream account.
// GET /api/v1/admin/accounts/:id/probe-runs
func (h *AccountHandler) ListProbeRuns(c *gin.Context) {
	if h.accountProbeService == nil {
		response.InternalError(c, "Account probe service is not configured")
		return
	}
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	result, err := h.accountProbeService.List(c.Request.Context(), service.AccountProbeHistoryFilter{
		AccountID: accountID,
		Limit:     limit,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// GetProbeRun returns one persisted probe run with samples.
// GET /api/v1/admin/accounts/:id/probe-runs/:run_id
func (h *AccountHandler) GetProbeRun(c *gin.Context) {
	if h.accountProbeService == nil {
		response.InternalError(c, "Account probe service is not configured")
		return
	}
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	runID, err := strconv.ParseInt(c.Param("run_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid probe run ID")
		return
	}
	result, err := h.accountProbeService.Get(c.Request.Context(), accountID, runID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// ListProbeReportRuns returns all upstream account probe runs for the report page.
// GET /api/v1/admin/account-probe-runs
func (h *AccountHandler) ListProbeReportRuns(c *gin.Context) {
	if h.accountProbeService == nil {
		response.InternalError(c, "Account probe service is not configured")
		return
	}
	filter, err := parseAccountProbeReportFilter(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	result, err := h.accountProbeService.ListReports(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// ListProbeReportRanking returns aggregate upstream probe ranking.
// GET /api/v1/admin/account-probe-runs/ranking
func (h *AccountHandler) ListProbeReportRanking(c *gin.Context) {
	if h.accountProbeService == nil {
		response.InternalError(c, "Account probe service is not configured")
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	items, err := h.accountProbeService.ListRanking(c.Request.Context(), limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

// GetProbeReportRun returns one upstream account probe run for the report page.
// GET /api/v1/admin/account-probe-runs/:run_id
func (h *AccountHandler) GetProbeReportRun(c *gin.Context) {
	if h.accountProbeService == nil {
		response.InternalError(c, "Account probe service is not configured")
		return
	}
	runID, err := strconv.ParseInt(c.Param("run_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid probe run ID")
		return
	}
	result, err := h.accountProbeService.GetReport(c.Request.Context(), runID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// DeleteProbeReportRuns deletes selected finished upstream probe reports.
// DELETE /api/v1/admin/account-probe-runs
func (h *AccountHandler) DeleteProbeReportRuns(c *gin.Context) {
	if h.accountProbeService == nil {
		response.InternalError(c, "Account probe service is not configured")
		return
	}
	var req DeleteAccountProbeRunsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	runIDs := uniquePositiveInt64s(req.RunIDs)
	if len(runIDs) == 0 {
		response.BadRequest(c, "run_ids is required")
		return
	}
	if len(runIDs) > 200 {
		response.BadRequest(c, "run_ids cannot exceed 200")
		return
	}
	result, err := h.accountProbeService.DeleteReports(c.Request.Context(), runIDs)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// BatchCreateProbeReportRuns starts background probe runs for selected upstream accounts.
// POST /api/v1/admin/account-probe-runs/batch
func (h *AccountHandler) BatchCreateProbeReportRuns(c *gin.Context) {
	if h.accountProbeService == nil {
		response.InternalError(c, "Account probe service is not configured")
		return
	}
	var req BatchCreateAccountProbeRunsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	accountIDs := uniquePositiveInt64s(req.AccountIDs)
	if len(accountIDs) == 0 {
		response.BadRequest(c, "account_ids is required")
		return
	}
	if len(accountIDs) > 50 {
		response.BadRequest(c, "account_ids cannot exceed 50")
		return
	}
	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = service.AccountProbeProfileQuick
	}
	runs := make([]service.AccountProbeResult, 0, len(accountIDs))
	for _, accountID := range accountIDs {
		probeReq := service.AccountProbeRunRequest{
			AccountID:          accountID,
			Profile:            mode,
			Model:              req.Model,
			IncludeLongContext: req.IncludeLongContext || req.LongContext,
			RequestMode:        req.RequestMode,
		}
		run, err := h.accountProbeService.Start(c.Request.Context(), probeReq)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		runs = append(runs, run)
	}
	go h.runAccountProbeBatchBackground(runs, req)
	response.Accepted(c, gin.H{
		"accepted_count": len(runs),
		"runs":           runs,
	})
}

func (h *AccountHandler) runAccountProbeBatchBackground(runs []service.AccountProbeResult, req BatchCreateAccountProbeRunsRequest) {
	if h == nil || h.accountProbeService == nil || len(runs) == 0 {
		return
	}
	limit := accountProbeBatchConcurrency
	if limit <= 0 {
		limit = 1
	}
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for _, run := range runs {
		run := run
		probeReq := service.AccountProbeRunRequest{
			AccountID:          run.AccountID,
			Profile:            run.Profile,
			Model:              run.Model,
			IncludeLongContext: run.IncludeLongContext || req.IncludeLongContext || req.LongContext,
			RequestMode:        run.RequestMode,
		}
		if strings.TrimSpace(probeReq.Profile) == "" {
			probeReq.Profile = req.Mode
		}
		if strings.TrimSpace(probeReq.Model) == "" {
			probeReq.Model = req.Model
		}
		if strings.TrimSpace(probeReq.RequestMode) == "" {
			probeReq.RequestMode = req.RequestMode
		}
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			h.runAccountProbeBackground(run, probeReq)
		}()
	}
	wg.Wait()
}

func (h *AccountHandler) runAccountModelProbeBatchBackground(runs []service.AccountProbeResult, req BatchCreateAccountModelProbeRunsRequest) {
	if h == nil || h.accountProbeService == nil || len(runs) == 0 {
		return
	}
	limit := accountProbeBatchConcurrency
	if limit <= 0 {
		limit = 1
	}
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for _, run := range runs {
		run := run
		probeReq := service.AccountProbeRunRequest{
			AccountID:           run.AccountID,
			Profile:             service.AccountProbeProfileModelValidation,
			Model:               run.Model,
			RequestMode:         run.RequestMode,
			TrustedComparisonID: req.TrustedComparisonID,
			ModelValidationOnly: true,
		}
		if strings.TrimSpace(probeReq.Model) == "" {
			probeReq.Model = req.Model
		}
		if strings.TrimSpace(probeReq.RequestMode) == "" {
			probeReq.RequestMode = req.RequestMode
		}
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			h.runAccountProbeBackground(run, probeReq)
		}()
	}
	wg.Wait()
}

func parseAccountProbeReportFilter(c *gin.Context) (service.AccountProbeReportFilter, error) {
	var filter service.AccountProbeReportFilter
	if raw := strings.TrimSpace(c.Query("account_id")); raw != "" {
		accountID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return filter, fmt.Errorf("Invalid account ID")
		}
		filter.AccountID = accountID
	}
	filter.Status = c.Query("status")
	filter.Profile = c.Query("mode")
	filter.RequestMode = c.Query("request_mode")
	filter.Model = c.Query("model")
	filter.Keyword = c.Query("keyword")
	filter.Sort = firstQueryValue(c, service.AccountProbeReportSortCreatedAt, "sort", "sort_by")
	filter.Order = firstQueryValue(c, "desc", "order", "sort_order")
	filter.Page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	filter.PageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if raw := strings.TrimSpace(firstQueryValue(c, "", "from", "start_time")); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return filter, fmt.Errorf("Invalid from time")
		}
		filter.From = &t
	}
	if raw := strings.TrimSpace(firstQueryValue(c, "", "to", "end_time")); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return filter, fmt.Errorf("Invalid to time")
		}
		filter.To = &t
	}
	return filter, nil
}

func uniquePositiveInt64s(values []int64) []int64 {
	seen := make(map[int64]bool, len(values))
	out := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func firstQueryValue(c *gin.Context, fallback string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(c.Query(key)); value != "" {
			return value
		}
	}
	return fallback
}

// RecoverState handles unified recovery of recoverable account runtime state.
// POST /api/v1/admin/accounts/:id/recover-state
func (h *AccountHandler) RecoverState(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	if h.rateLimitService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Rate limit service unavailable")
		return
	}

	if _, err := h.rateLimitService.RecoverAccountState(c.Request.Context(), accountID, service.AccountRecoveryOptions{
		InvalidateToken: true,
	}); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	account, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, h.buildAccountResponseWithRuntime(c.Request.Context(), account))
}

// SyncFromCRS handles syncing accounts from claude-relay-service (CRS)
// POST /api/v1/admin/accounts/sync/crs
func (h *AccountHandler) SyncFromCRS(c *gin.Context) {
	var req SyncFromCRSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Default to syncing proxies (can be disabled by explicitly setting false)
	syncProxies := true
	if req.SyncProxies != nil {
		syncProxies = *req.SyncProxies
	}

	result, err := h.crsSyncService.SyncFromCRS(c.Request.Context(), service.SyncFromCRSInput{
		BaseURL:            req.BaseURL,
		Username:           req.Username,
		Password:           req.Password,
		SyncProxies:        syncProxies,
		SelectedAccountIDs: req.SelectedAccountIDs,
	})
	if err != nil {
		// Provide detailed error message for CRS sync failures
		response.InternalError(c, "CRS sync failed: "+err.Error())
		return
	}

	response.Success(c, result)
}

// PreviewFromCRS handles previewing accounts from CRS before sync
// POST /api/v1/admin/accounts/sync/crs/preview
func (h *AccountHandler) PreviewFromCRS(c *gin.Context) {
	var req PreviewFromCRSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	result, err := h.crsSyncService.PreviewFromCRS(c.Request.Context(), service.SyncFromCRSInput{
		BaseURL:  req.BaseURL,
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		response.InternalError(c, "CRS preview failed: "+err.Error())
		return
	}

	response.Success(c, result)
}

// refreshSingleAccount refreshes credentials for a single OAuth account.
// Returns (updatedAccount, warning, error) where warning is used for Antigravity ProjectIDMissing scenario.
func (h *AccountHandler) refreshSingleAccount(ctx context.Context, account *service.Account) (*service.Account, string, error) {
	if !account.IsOAuth() {
		return nil, "", infraerrors.BadRequest("NOT_OAUTH", "cannot refresh non-OAuth account")
	}

	var newCredentials map[string]any

	if account.IsOpenAI() {
		tokenInfo, err := h.openaiOAuthService.RefreshAccountToken(ctx, account)
		if err != nil {
			// 刷新失败但 access_token 可能仍有效，尝试设置隐私
			h.adminService.EnsureOpenAIPrivacy(ctx, account)
			return h.markAccountRefreshFailure(ctx, account, err), "", err
		}

		newCredentials = h.openaiOAuthService.BuildAccountCredentials(tokenInfo)
		for k, v := range account.Credentials {
			if _, exists := newCredentials[k]; !exists {
				newCredentials[k] = v
			}
		}
	} else if account.Platform == service.PlatformGemini {
		tokenInfo, err := h.geminiOAuthService.RefreshAccountToken(ctx, account)
		if err != nil {
			wrappedErr := fmt.Errorf("failed to refresh credentials: %w", err)
			return h.markAccountRefreshFailure(ctx, account, wrappedErr), "", wrappedErr
		}

		newCredentials = h.geminiOAuthService.BuildAccountCredentials(tokenInfo)
		for k, v := range account.Credentials {
			if _, exists := newCredentials[k]; !exists {
				newCredentials[k] = v
			}
		}
	} else if account.Platform == service.PlatformAntigravity {
		tokenInfo, err := h.antigravityOAuthService.RefreshAccountToken(ctx, account)
		if err != nil {
			return h.markAccountRefreshFailure(ctx, account, err), "", err
		}

		newCredentials = h.antigravityOAuthService.BuildAccountCredentials(tokenInfo)
		for k, v := range account.Credentials {
			if _, exists := newCredentials[k]; !exists {
				newCredentials[k] = v
			}
		}

		// 特殊处理 project_id：如果新值为空但旧值非空，保留旧值
		// 这确保了即使 LoadCodeAssist 失败，project_id 也不会丢失
		if newProjectID, _ := newCredentials["project_id"].(string); newProjectID == "" {
			if oldProjectID := strings.TrimSpace(account.GetCredential("project_id")); oldProjectID != "" {
				newCredentials["project_id"] = oldProjectID
			}
		}

		// 如果 project_id 获取失败，更新凭证但不标记为 error
		if tokenInfo.ProjectIDMissing {
			updatedAccount, updateErr := h.adminService.UpdateAccount(ctx, account.ID, &service.UpdateAccountInput{
				Credentials: newCredentials,
			})
			if updateErr != nil {
				return nil, "", fmt.Errorf("failed to update credentials: %w", updateErr)
			}
			updatedAccount = h.markAccountRefreshSuccess(ctx, account, updatedAccount)
			h.adminService.EnsureAntigravityPrivacy(ctx, updatedAccount)
			return updatedAccount, "missing_project_id_temporary", nil
		}

		// 成功获取到 project_id，如果之前是 missing_project_id 错误则清除
		if account.Status == service.StatusError && strings.Contains(account.ErrorMessage, "missing_project_id:") {
			if _, clearErr := h.adminService.ClearAccountError(ctx, account.ID); clearErr != nil {
				return nil, "", fmt.Errorf("failed to clear account error: %w", clearErr)
			}
		}
	} else {
		// Use Anthropic/Claude OAuth service to refresh token
		tokenInfo, err := h.oauthService.RefreshAccountToken(ctx, account)
		if err != nil {
			return h.markAccountRefreshFailure(ctx, account, err), "", err
		}

		// Copy existing credentials to preserve non-token settings (e.g., intercept_warmup_requests)
		newCredentials = make(map[string]any)
		for k, v := range account.Credentials {
			newCredentials[k] = v
		}

		// Update token-related fields
		newCredentials["access_token"] = tokenInfo.AccessToken
		newCredentials["token_type"] = tokenInfo.TokenType
		newCredentials["expires_in"] = strconv.FormatInt(tokenInfo.ExpiresIn, 10)
		newCredentials["expires_at"] = strconv.FormatInt(tokenInfo.ExpiresAt, 10)
		if strings.TrimSpace(tokenInfo.RefreshToken) != "" {
			newCredentials["refresh_token"] = tokenInfo.RefreshToken
		}
		if strings.TrimSpace(tokenInfo.Scope) != "" {
			newCredentials["scope"] = tokenInfo.Scope
		}
	}

	updatedAccount, err := h.adminService.UpdateAccount(ctx, account.ID, &service.UpdateAccountInput{
		Credentials: newCredentials,
	})
	if err != nil {
		return nil, "", err
	}
	updatedAccount = h.markAccountRefreshSuccess(ctx, account, updatedAccount)

	// 刷新成功后，清除 token 缓存，确保下次请求使用新 token
	if h.tokenCacheInvalidator != nil {
		if invalidateErr := h.tokenCacheInvalidator.InvalidateToken(ctx, updatedAccount); invalidateErr != nil {
			log.Printf("[WARN] Failed to invalidate token cache for account %d: %v", updatedAccount.ID, invalidateErr)
		}
	}

	// OpenAI OAuth: 刷新成功后检查并设置 privacy_mode
	h.adminService.EnsureOpenAIPrivacy(ctx, updatedAccount)
	// Antigravity OAuth: 刷新成功后检查并设置 privacy_mode
	h.adminService.EnsureAntigravityPrivacy(ctx, updatedAccount)

	return updatedAccount, "", nil
}

func (h *AccountHandler) markAccountRefreshSuccess(ctx context.Context, previousAccount *service.Account, updatedAccount *service.Account) *service.Account {
	if h == nil || h.adminService == nil {
		return updatedAccount
	}
	accountID := int64(0)
	if updatedAccount != nil {
		accountID = updatedAccount.ID
	}
	if accountID == 0 && previousAccount != nil {
		accountID = previousAccount.ID
	}
	if accountID == 0 {
		return updatedAccount
	}

	needsClear := accountNeedsRefreshRecovery(previousAccount) || accountNeedsRefreshRecovery(updatedAccount)
	if needsClear {
		if recovered, err := h.adminService.ClearAccountError(ctx, accountID); err != nil {
			slog.Warn("account_refresh_clear_error_failed", "account_id", accountID, "error", err)
		} else if recovered != nil {
			updatedAccount = recovered
		}
	}

	if accountNeedsSchedulableRecovery(previousAccount) || accountNeedsSchedulableRecovery(updatedAccount) {
		if recovered, err := h.adminService.SetAccountSchedulable(ctx, accountID, true); err != nil {
			slog.Warn("account_refresh_set_schedulable_failed", "account_id", accountID, "error", err)
		} else if recovered != nil {
			updatedAccount = recovered
		}
	}

	return updatedAccount
}

func accountNeedsRefreshRecovery(account *service.Account) bool {
	if account == nil {
		return false
	}
	return account.Status == service.StatusError ||
		strings.TrimSpace(account.ErrorMessage) != "" ||
		account.IsRateLimited() ||
		account.TempUnschedulableUntil != nil
}

func accountNeedsSchedulableRecovery(account *service.Account) bool {
	return account != nil && !account.Schedulable
}

func (h *AccountHandler) markAccountRefreshFailure(ctx context.Context, account *service.Account, refreshErr error) *service.Account {
	if h == nil || h.adminService == nil || account == nil || refreshErr == nil {
		return nil
	}
	errorMessage := strings.TrimSpace(refreshErr.Error())
	if errorMessage == "" {
		errorMessage = "token refresh failed"
	}
	if err := h.adminService.SetAccountError(ctx, account.ID, errorMessage); err != nil {
		slog.Warn("account_refresh_set_error_failed", "account_id", account.ID, "error", err)
		return nil
	}
	updated, err := h.adminService.GetAccount(ctx, account.ID)
	if err != nil {
		slog.Warn("account_refresh_get_error_account_failed", "account_id", account.ID, "error", err)
		return nil
	}
	return updated
}

// Refresh handles refreshing account credentials
// POST /api/v1/admin/accounts/:id/refresh
func (h *AccountHandler) Refresh(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	// Get account
	account, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		response.NotFound(c, "Account not found")
		return
	}

	updatedAccount, warning, err := h.refreshSingleAccount(c.Request.Context(), account)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	if warning == "missing_project_id_temporary" {
		response.Success(c, gin.H{
			"message": "Token refreshed successfully, but project_id could not be retrieved (will retry automatically)",
			"warning": "missing_project_id_temporary",
		})
		return
	}

	response.Success(c, h.buildAccountResponseWithRuntime(c.Request.Context(), updatedAccount))
}

// GetStats handles getting account statistics
// GET /api/v1/admin/accounts/:id/stats
func (h *AccountHandler) GetStats(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	// Parse days parameter (default 30)
	days := 30
	if daysStr := c.Query("days"); daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 && d <= 90 {
			days = d
		}
	}

	// Calculate time range
	now := timezone.Now()
	endTime := timezone.StartOfDay(now.AddDate(0, 0, 1))
	startTime := timezone.StartOfDay(now.AddDate(0, 0, -days+1))

	stats, err := h.accountUsageService.GetAccountUsageStats(c.Request.Context(), accountID, startTime, endTime)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, stats)
}

// ClearError handles clearing account error
// POST /api/v1/admin/accounts/:id/clear-error
func (h *AccountHandler) ClearError(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	account, err := h.adminService.ClearAccountError(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// 清除错误后，同时清除 token 缓存，确保下次请求会获取最新的 token（触发刷新或从 DB 读取）
	// 这解决了管理员重置账号状态后，旧的失效 token 仍在缓存中导致立即再次 401 的问题
	if h.tokenCacheInvalidator != nil && account.IsOAuth() {
		if invalidateErr := h.tokenCacheInvalidator.InvalidateToken(c.Request.Context(), account); invalidateErr != nil {
			log.Printf("[WARN] Failed to invalidate token cache for account %d: %v", accountID, invalidateErr)
		}
	}

	response.Success(c, h.buildAccountResponseWithRuntime(c.Request.Context(), account))
}

// BatchClearError handles batch clearing account errors
// POST /api/v1/admin/accounts/batch-clear-error
func (h *AccountHandler) BatchClearError(c *gin.Context) {
	var req struct {
		AccountIDs []int64 `json:"account_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if len(req.AccountIDs) == 0 {
		response.BadRequest(c, "account_ids is required")
		return
	}

	ctx := c.Request.Context()

	const maxConcurrency = 10
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrency)

	var mu sync.Mutex
	var successCount, failedCount int
	var errors []gin.H

	// 注意：所有 goroutine 必须 return nil，避免 errgroup cancel 其他并发任务
	for _, id := range req.AccountIDs {
		accountID := id // 闭包捕获
		g.Go(func() error {
			account, err := h.adminService.ClearAccountError(gctx, accountID)
			if err != nil {
				mu.Lock()
				failedCount++
				errors = append(errors, gin.H{
					"account_id": accountID,
					"error":      err.Error(),
				})
				mu.Unlock()
				return nil
			}

			// 清除错误后，同时清除 token 缓存
			if h.tokenCacheInvalidator != nil && account.IsOAuth() {
				if invalidateErr := h.tokenCacheInvalidator.InvalidateToken(gctx, account); invalidateErr != nil {
					log.Printf("[WARN] Failed to invalidate token cache for account %d: %v", accountID, invalidateErr)
				}
			}

			mu.Lock()
			successCount++
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"total":   len(req.AccountIDs),
		"success": successCount,
		"failed":  failedCount,
		"errors":  errors,
	})
}

// BatchRefresh handles batch refreshing account credentials
// POST /api/v1/admin/accounts/batch-refresh
func (h *AccountHandler) BatchRefresh(c *gin.Context) {
	var req struct {
		AccountIDs []int64 `json:"account_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if len(req.AccountIDs) == 0 {
		response.BadRequest(c, "account_ids is required")
		return
	}

	ctx := c.Request.Context()

	accounts, err := h.adminService.GetAccountsByIDs(ctx, req.AccountIDs)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// 建立已获取账号的 ID 集合，检测缺失的 ID
	foundIDs := make(map[int64]bool, len(accounts))
	for _, acc := range accounts {
		if acc != nil {
			foundIDs[acc.ID] = true
		}
	}

	const maxConcurrency = 10
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrency)

	var mu sync.Mutex
	var successCount, failedCount int
	var errors []gin.H
	var warnings []gin.H
	var refreshedAccounts []AccountWithConcurrency

	// 将不存在的账号 ID 标记为失败
	for _, id := range req.AccountIDs {
		if !foundIDs[id] {
			failedCount++
			errors = append(errors, gin.H{
				"account_id": id,
				"error":      "account not found",
			})
		}
	}

	// 注意：所有 goroutine 必须 return nil，避免 errgroup cancel 其他并发任务
	for _, account := range accounts {
		acc := account // 闭包捕获
		if acc == nil {
			continue
		}
		g.Go(func() error {
			updatedAccount, warning, err := h.refreshSingleAccount(gctx, acc)
			if updatedAccount == nil {
				if latest, getErr := h.adminService.GetAccount(gctx, acc.ID); getErr == nil {
					updatedAccount = latest
				}
			}
			var updatedResponse *AccountWithConcurrency
			if updatedAccount != nil {
				responseAccount := h.buildAccountResponseWithRuntime(gctx, updatedAccount)
				updatedResponse = &responseAccount
			}

			mu.Lock()
			if err != nil {
				failedCount++
				errors = append(errors, gin.H{
					"account_id": acc.ID,
					"error":      err.Error(),
				})
			} else {
				successCount++
				if warning != "" {
					warnings = append(warnings, gin.H{
						"account_id": acc.ID,
						"warning":    warning,
					})
				}
			}
			if updatedResponse != nil {
				refreshedAccounts = append(refreshedAccounts, *updatedResponse)
			}
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"total":    len(req.AccountIDs),
		"success":  successCount,
		"failed":   failedCount,
		"errors":   errors,
		"warnings": warnings,
		"accounts": refreshedAccounts,
	})
}

// BatchCreate handles batch creating accounts
// POST /api/v1/admin/accounts/batch
func (h *AccountHandler) BatchCreate(c *gin.Context) {
	var req struct {
		Accounts []CreateAccountRequest `json:"accounts" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	executeAdminIdempotentJSON(c, "admin.accounts.batch_create", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		success := 0
		failed := 0
		results := make([]gin.H, 0, len(req.Accounts))
		// 收集需要异步设置隐私的 OAuth 账号
		var antigravityPrivacyAccounts []*service.Account
		var openaiPrivacyAccounts []*service.Account

		for _, item := range req.Accounts {
			if item.RateMultiplier != nil && *item.RateMultiplier < 0 {
				failed++
				results = append(results, gin.H{
					"name":    item.Name,
					"success": false,
					"error":   "rate_multiplier must be >= 0",
				})
				continue
			}

			// base_rpm 输入校验：负值归零，超过 10000 截断
			sanitizeExtraBaseRPM(item.Extra)

			skipCheck := item.ConfirmMixedChannelRisk != nil && *item.ConfirmMixedChannelRisk

			account, err := h.adminService.CreateAccount(ctx, &service.CreateAccountInput{
				Name:                  item.Name,
				Notes:                 item.Notes,
				Platform:              item.Platform,
				Type:                  item.Type,
				Credentials:           item.Credentials,
				Extra:                 item.Extra,
				ProxyID:               item.ProxyID,
				Concurrency:           item.Concurrency,
				Priority:              item.Priority,
				RateMultiplier:        item.RateMultiplier,
				GroupIDs:              item.GroupIDs,
				ExpiresAt:             item.ExpiresAt,
				AutoPauseOnExpired:    item.AutoPauseOnExpired,
				SkipMixedChannelCheck: skipCheck,
			})
			if err != nil {
				failed++
				results = append(results, gin.H{
					"name":    item.Name,
					"success": false,
					"error":   err.Error(),
				})
				continue
			}
			// 收集需要异步设置隐私的 OAuth 账号
			if account.Type == service.AccountTypeOAuth {
				switch account.Platform {
				case service.PlatformAntigravity:
					antigravityPrivacyAccounts = append(antigravityPrivacyAccounts, account)
				case service.PlatformOpenAI:
					openaiPrivacyAccounts = append(openaiPrivacyAccounts, account)
				}
			}
			// OpenAI APIKey 账号异步探测 /v1/responses 能力。
			h.scheduleOpenAIResponsesProbe(account)
			success++
			results = append(results, gin.H{
				"name":    item.Name,
				"id":      account.ID,
				"success": true,
			})
		}

		// 异步设置隐私，避免批量创建时阻塞请求
		adminSvc := h.adminService
		if len(antigravityPrivacyAccounts) > 0 {
			accounts := antigravityPrivacyAccounts
			go func() {
				defer func() {
					if r := recover(); r != nil {
						slog.Error("batch_create_antigravity_privacy_panic", "recover", r)
					}
				}()
				bgCtx := context.Background()
				for _, acc := range accounts {
					adminSvc.ForceAntigravityPrivacy(bgCtx, acc)
				}
			}()
		}
		if len(openaiPrivacyAccounts) > 0 {
			accounts := openaiPrivacyAccounts
			go func() {
				defer func() {
					if r := recover(); r != nil {
						slog.Error("batch_create_openai_privacy_panic", "recover", r)
					}
				}()
				bgCtx := context.Background()
				for _, acc := range accounts {
					adminSvc.ForceOpenAIPrivacy(bgCtx, acc)
				}
			}()
		}

		return gin.H{
			"success": success,
			"failed":  failed,
			"results": results,
		}, nil
	})
}

// BatchUpdateCredentialsRequest represents batch credentials update request
type BatchUpdateCredentialsRequest struct {
	AccountIDs []int64 `json:"account_ids" binding:"required,min=1"`
	Field      string  `json:"field" binding:"required,oneof=account_uuid org_uuid intercept_warmup_requests"`
	Value      any     `json:"value"`
}

// BatchUpdateCredentials handles batch updating credentials fields
// POST /api/v1/admin/accounts/batch-update-credentials
func (h *AccountHandler) BatchUpdateCredentials(c *gin.Context) {
	var req BatchUpdateCredentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Validate value type based on field
	if req.Field == "intercept_warmup_requests" {
		// Must be boolean
		if _, ok := req.Value.(bool); !ok {
			response.BadRequest(c, "intercept_warmup_requests must be boolean")
			return
		}
	} else {
		// account_uuid and org_uuid can be string or null
		if req.Value != nil {
			if _, ok := req.Value.(string); !ok {
				response.BadRequest(c, req.Field+" must be string or null")
				return
			}
		}
	}

	ctx := c.Request.Context()

	// 阶段一：预验证所有账号存在，收集 credentials
	type accountUpdate struct {
		ID          int64
		Credentials map[string]any
	}
	updates := make([]accountUpdate, 0, len(req.AccountIDs))
	for _, accountID := range req.AccountIDs {
		account, err := h.adminService.GetAccount(ctx, accountID)
		if err != nil {
			response.Error(c, 404, fmt.Sprintf("Account %d not found", accountID))
			return
		}
		if account.Credentials == nil {
			account.Credentials = make(map[string]any)
		}
		account.Credentials[req.Field] = req.Value
		updates = append(updates, accountUpdate{ID: accountID, Credentials: account.Credentials})
	}

	// 阶段二：依次更新，返回每个账号的成功/失败明细，便于调用方重试
	success := 0
	failed := 0
	successIDs := make([]int64, 0, len(updates))
	failedIDs := make([]int64, 0, len(updates))
	results := make([]gin.H, 0, len(updates))
	for _, u := range updates {
		updateInput := &service.UpdateAccountInput{Credentials: u.Credentials}
		if _, err := h.adminService.UpdateAccount(ctx, u.ID, updateInput); err != nil {
			failed++
			failedIDs = append(failedIDs, u.ID)
			results = append(results, gin.H{
				"account_id": u.ID,
				"success":    false,
				"error":      err.Error(),
			})
			continue
		}
		success++
		successIDs = append(successIDs, u.ID)
		results = append(results, gin.H{
			"account_id": u.ID,
			"success":    true,
		})
	}

	response.Success(c, gin.H{
		"success":     success,
		"failed":      failed,
		"success_ids": successIDs,
		"failed_ids":  failedIDs,
		"results":     results,
	})
}

// BulkUpdate handles bulk updating accounts with selected fields/credentials.
// POST /api/v1/admin/accounts/bulk-update
func (h *AccountHandler) BulkUpdate(c *gin.Context) {
	var req BulkUpdateAccountsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.RateMultiplier != nil && *req.RateMultiplier < 0 {
		response.BadRequest(c, "rate_multiplier must be >= 0")
		return
	}
	if len(req.AccountIDs) == 0 && req.Filters == nil {
		response.BadRequest(c, "account_ids or filters is required")
		return
	}
	// base_rpm 输入校验：负值归零，超过 10000 截断
	sanitizeExtraBaseRPM(req.Extra)

	// 确定是否跳过混合渠道检查
	skipCheck := req.ConfirmMixedChannelRisk != nil && *req.ConfirmMixedChannelRisk

	hasUpdates := req.Name != "" ||
		req.ProxyID != nil ||
		req.Concurrency != nil ||
		req.Priority != nil ||
		req.RateMultiplier != nil ||
		req.LoadFactor != nil ||
		req.Status != "" ||
		req.Schedulable != nil ||
		req.GroupIDs != nil ||
		len(req.Credentials) > 0 ||
		len(req.Extra) > 0

	if !hasUpdates {
		response.BadRequest(c, "No updates provided")
		return
	}

	result, err := h.adminService.BulkUpdateAccounts(c.Request.Context(), &service.BulkUpdateAccountsInput{
		AccountIDs:            req.AccountIDs,
		Filters:               toServiceBulkUpdateAccountFilters(req.Filters),
		Name:                  req.Name,
		ProxyID:               req.ProxyID,
		Concurrency:           req.Concurrency,
		Priority:              req.Priority,
		RateMultiplier:        req.RateMultiplier,
		LoadFactor:            req.LoadFactor,
		Status:                req.Status,
		Schedulable:           req.Schedulable,
		GroupIDs:              req.GroupIDs,
		Credentials:           req.Credentials,
		Extra:                 req.Extra,
		SkipMixedChannelCheck: skipCheck,
	})
	if err != nil {
		var mixedErr *service.MixedChannelError
		if errors.As(err, &mixedErr) {
			c.JSON(409, gin.H{
				"error":   "mixed_channel_warning",
				"message": mixedErr.Error(),
				"details": gin.H{
					"group_id":         mixedErr.GroupID,
					"group_name":       mixedErr.GroupName,
					"current_platform": mixedErr.CurrentPlatform,
					"other_platform":   mixedErr.OtherPlatform,
				},
			})
			return
		}
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, result)
}

func toServiceBulkUpdateAccountFilters(filters *BulkUpdateAccountFilters) *service.BulkUpdateAccountFilters {
	if filters == nil {
		return nil
	}
	return &service.BulkUpdateAccountFilters{
		Platform:    filters.Platform,
		Type:        filters.Type,
		Status:      filters.Status,
		Group:       filters.Group,
		Search:      filters.Search,
		PrivacyMode: filters.PrivacyMode,
		PlanType:    filters.PlanType,
	}
}

// ========== OAuth Handlers ==========

// GenerateAuthURLRequest represents the request for generating auth URL
type GenerateAuthURLRequest struct {
	ProxyID *int64 `json:"proxy_id"`
}

// GenerateAuthURL generates OAuth authorization URL with full scope
// POST /api/v1/admin/accounts/generate-auth-url
func (h *OAuthHandler) GenerateAuthURL(c *gin.Context) {
	var req GenerateAuthURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Allow empty body
		req = GenerateAuthURLRequest{}
	}

	result, err := h.oauthService.GenerateAuthURL(c.Request.Context(), req.ProxyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, result)
}

// GenerateSetupTokenURL generates OAuth authorization URL for setup token (inference only)
// POST /api/v1/admin/accounts/generate-setup-token-url
func (h *OAuthHandler) GenerateSetupTokenURL(c *gin.Context) {
	var req GenerateAuthURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Allow empty body
		req = GenerateAuthURLRequest{}
	}

	result, err := h.oauthService.GenerateSetupTokenURL(c.Request.Context(), req.ProxyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, result)
}

// ExchangeCodeRequest represents the request for exchanging auth code
type ExchangeCodeRequest struct {
	SessionID string `json:"session_id" binding:"required"`
	Code      string `json:"code" binding:"required"`
	ProxyID   *int64 `json:"proxy_id"`
}

// ExchangeCode exchanges authorization code for tokens
// POST /api/v1/admin/accounts/exchange-code
func (h *OAuthHandler) ExchangeCode(c *gin.Context) {
	var req ExchangeCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	tokenInfo, err := h.oauthService.ExchangeCode(c.Request.Context(), &service.ExchangeCodeInput{
		SessionID: req.SessionID,
		Code:      req.Code,
		ProxyID:   req.ProxyID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, tokenInfo)
}

// ExchangeSetupTokenCode exchanges authorization code for setup token
// POST /api/v1/admin/accounts/exchange-setup-token-code
func (h *OAuthHandler) ExchangeSetupTokenCode(c *gin.Context) {
	var req ExchangeCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	tokenInfo, err := h.oauthService.ExchangeCode(c.Request.Context(), &service.ExchangeCodeInput{
		SessionID: req.SessionID,
		Code:      req.Code,
		ProxyID:   req.ProxyID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, tokenInfo)
}

// CookieAuthRequest represents the request for cookie-based authentication
type CookieAuthRequest struct {
	SessionKey string `json:"code" binding:"required"` // Using 'code' field as sessionKey (frontend sends it this way)
	ProxyID    *int64 `json:"proxy_id"`
}

// CookieAuth performs OAuth using sessionKey (cookie-based auto-auth)
// POST /api/v1/admin/accounts/cookie-auth
func (h *OAuthHandler) CookieAuth(c *gin.Context) {
	var req CookieAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	tokenInfo, err := h.oauthService.CookieAuth(c.Request.Context(), &service.CookieAuthInput{
		SessionKey: req.SessionKey,
		ProxyID:    req.ProxyID,
		Scope:      "full",
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, tokenInfo)
}

// SetupTokenCookieAuth performs OAuth using sessionKey for setup token (inference only)
// POST /api/v1/admin/accounts/setup-token-cookie-auth
func (h *OAuthHandler) SetupTokenCookieAuth(c *gin.Context) {
	var req CookieAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	tokenInfo, err := h.oauthService.CookieAuth(c.Request.Context(), &service.CookieAuthInput{
		SessionKey: req.SessionKey,
		ProxyID:    req.ProxyID,
		Scope:      "inference",
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, tokenInfo)
}

// GetUsage handles getting account usage information
// GET /api/v1/admin/accounts/:id/usage?source=passive|active&force=true
func (h *AccountHandler) GetUsage(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	source := c.DefaultQuery("source", "active")
	force := c.Query("force") == "true"

	var usage *service.UsageInfo
	if source == "passive" {
		usage, err = h.accountUsageService.GetPassiveUsage(c.Request.Context(), accountID)
	} else {
		usage, err = h.accountUsageService.GetUsage(c.Request.Context(), accountID, force)
	}
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, usage)
}

// ClearRateLimit handles clearing account rate limit status
// POST /api/v1/admin/accounts/:id/clear-rate-limit
func (h *AccountHandler) ClearRateLimit(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	err = h.rateLimitService.ClearRateLimit(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	account, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, h.buildAccountResponseWithRuntime(c.Request.Context(), account))
}

// ResetQuota handles resetting account quota usage
// POST /api/v1/admin/accounts/:id/reset-quota
func (h *AccountHandler) ResetQuota(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	if err := h.adminService.ResetAccountQuota(c.Request.Context(), accountID); err != nil {
		response.InternalError(c, "Failed to reset account quota: "+err.Error())
		return
	}

	account, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, h.buildAccountResponseWithRuntime(c.Request.Context(), account))
}

// GetTempUnschedulable handles getting temporary unschedulable status
// GET /api/v1/admin/accounts/:id/temp-unschedulable
func (h *AccountHandler) GetTempUnschedulable(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	state, err := h.rateLimitService.GetTempUnschedStatus(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	if state == nil || state.UntilUnix <= time.Now().Unix() {
		response.Success(c, gin.H{"active": false})
		return
	}

	response.Success(c, gin.H{
		"active": true,
		"state":  state,
	})
}

// ClearTempUnschedulable handles clearing temporary unschedulable status
// DELETE /api/v1/admin/accounts/:id/temp-unschedulable
func (h *AccountHandler) ClearTempUnschedulable(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	if err := h.rateLimitService.ClearTempUnschedulable(c.Request.Context(), accountID); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "Temp unschedulable cleared successfully"})
}

// GetTodayStats handles getting account today statistics
// GET /api/v1/admin/accounts/:id/today-stats
func (h *AccountHandler) GetTodayStats(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	stats, err := h.accountUsageService.GetTodayStats(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, stats)
}

// BatchTodayStatsRequest 批量今日统计请求体。
type BatchTodayStatsRequest struct {
	AccountIDs []int64 `json:"account_ids" binding:"required"`
}

// GetBatchTodayStats 批量获取多个账号的今日统计。
// POST /api/v1/admin/accounts/today-stats/batch
func (h *AccountHandler) GetBatchTodayStats(c *gin.Context) {
	var req BatchTodayStatsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	accountIDs := normalizeInt64IDList(req.AccountIDs)
	if len(accountIDs) == 0 {
		response.Success(c, gin.H{"stats": map[string]any{}})
		return
	}

	cacheKey := buildAccountTodayStatsBatchCacheKey(accountIDs)
	if cached, ok := accountTodayStatsBatchCache.Get(cacheKey); ok {
		if cached.ETag != "" {
			c.Header("ETag", cached.ETag)
			c.Header("Vary", "If-None-Match")
			if ifNoneMatchMatched(c.GetHeader("If-None-Match"), cached.ETag) {
				c.Status(http.StatusNotModified)
				return
			}
		}
		c.Header("X-Snapshot-Cache", "hit")
		response.Success(c, cached.Payload)
		return
	}

	stats, err := h.accountUsageService.GetTodayStatsBatch(c.Request.Context(), accountIDs)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	payload := gin.H{"stats": stats}
	cached := accountTodayStatsBatchCache.Set(cacheKey, payload)
	if cached.ETag != "" {
		c.Header("ETag", cached.ETag)
		c.Header("Vary", "If-None-Match")
	}
	c.Header("X-Snapshot-Cache", "miss")
	response.Success(c, payload)
}

// SetSchedulableRequest represents the request body for setting schedulable status
type SetSchedulableRequest struct {
	Schedulable bool `json:"schedulable"`
}

// SetSchedulable handles toggling account schedulable status
// POST /api/v1/admin/accounts/:id/schedulable
func (h *AccountHandler) SetSchedulable(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	var req SetSchedulableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	account, err := h.adminService.SetAccountSchedulable(c.Request.Context(), accountID, req.Schedulable)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, h.buildAccountResponseWithRuntime(c.Request.Context(), account))
}

// GetAvailableModels handles getting available models for an account
// GET /api/v1/admin/accounts/:id/models
func (h *AccountHandler) GetAvailableModels(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	account, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		response.NotFound(c, "Account not found")
		return
	}

	// Handle OpenAI accounts
	if account.IsOpenAI() {
		// OpenAI 自动透传会绕过常规模型改写，测试/模型列表也应回落到默认模型集。
		if account.IsOpenAIPassthroughEnabled() {
			response.Success(c, openai.DefaultModels)
			return
		}

		mapping := account.GetModelMapping()
		if len(mapping) == 0 {
			response.Success(c, openai.DefaultModels)
			return
		}

		// Return mapped models
		var models []openai.Model
		for requestedModel := range mapping {
			var found bool
			for _, dm := range openai.DefaultModels {
				if dm.ID == requestedModel {
					models = append(models, dm)
					found = true
					break
				}
			}
			if !found {
				models = append(models, openai.Model{
					ID:          requestedModel,
					Object:      "model",
					Type:        "model",
					DisplayName: requestedModel,
				})
			}
		}
		response.Success(c, models)
		return
	}

	// Handle Gemini accounts
	if account.IsGemini() {
		// Consumer Google One OAuth still uses the legacy Gemini CLI / Code
		// Assist channel. Do not advertise newer 3.x or image models that the
		// channel cannot serve.
		if account.IsOAuth() {
			if account.IsGeminiGoogleOne() {
				response.Success(c, geminicli.GoogleOneModels)
				return
			}
			response.Success(c, geminicli.DefaultModels)
			return
		}

		// For API Key accounts: return models based on model_mapping
		mapping := account.GetModelMapping()
		if len(mapping) == 0 {
			response.Success(c, geminicli.DefaultModels)
			return
		}

		var models []geminicli.Model
		for requestedModel := range mapping {
			var found bool
			for _, dm := range geminicli.DefaultModels {
				if dm.ID == requestedModel {
					models = append(models, dm)
					found = true
					break
				}
			}
			if !found {
				models = append(models, geminicli.Model{
					ID:          requestedModel,
					Type:        "model",
					DisplayName: requestedModel,
					CreatedAt:   "",
				})
			}
		}
		response.Success(c, models)
		return
	}

	// Handle Antigravity accounts: return Claude + Gemini models
	if account.Platform == service.PlatformAntigravity {
		// 直接复用 antigravity.DefaultModels()，与 /v1/models 端点保持同步
		response.Success(c, antigravity.DefaultModels())
		return
	}

	// Handle Claude/Anthropic accounts
	// For OAuth and Setup-Token accounts: return default models
	if account.IsOAuth() {
		response.Success(c, claude.DefaultModels)
		return
	}

	// For API Key accounts: return models based on model_mapping
	mapping := account.GetModelMapping()
	if len(mapping) == 0 {
		// No mapping configured, return default models
		response.Success(c, claude.DefaultModels)
		return
	}

	// Return mapped models (keys of the mapping are the available model IDs)
	var models []claude.Model
	for requestedModel := range mapping {
		// Try to find display info from default models
		var found bool
		for _, dm := range claude.DefaultModels {
			if dm.ID == requestedModel {
				models = append(models, dm)
				found = true
				break
			}
		}
		// If not found in defaults, create a basic entry
		if !found {
			models = append(models, claude.Model{
				ID:          requestedModel,
				Type:        "model",
				DisplayName: requestedModel,
				CreatedAt:   "",
			})
		}
	}

	response.Success(c, models)
}

// SyncUpstreamModels handles syncing live supported models from an account's upstream.
// POST /api/v1/admin/accounts/:id/models/sync-upstream
func (h *AccountHandler) SyncUpstreamModels(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	account, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		response.NotFound(c, "Account not found")
		return
	}

	if h.accountTestService == nil {
		response.InternalError(c, "Account test service is not configured")
		return
	}

	models, err := h.accountTestService.FetchUpstreamSupportedModels(c.Request.Context(), account)
	if err != nil {
		var syncErr *service.UpstreamModelSyncError
		if errors.As(err, &syncErr) {
			switch syncErr.Kind {
			case service.UpstreamModelSyncErrorConfiguration, service.UpstreamModelSyncErrorUnsupported:
				response.BadRequest(c, syncErr.SafeMessage())
			default:
				slog.Warn("sync_upstream_models_failed", "account_id", accountID, "kind", syncErr.Kind)
				response.Error(c, http.StatusBadGateway, syncErr.SafeMessage())
			}
			return
		}

		slog.Warn("sync_upstream_models_failed", "account_id", accountID)
		response.Error(c, http.StatusBadGateway, "Failed to sync upstream models from upstream")
		return
	}

	response.Success(c, gin.H{"models": models})
}

// SetPrivacy handles setting privacy for a single OpenAI/Antigravity OAuth account
// POST /api/v1/admin/accounts/:id/set-privacy
func (h *AccountHandler) SetPrivacy(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	account, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		response.NotFound(c, "Account not found")
		return
	}
	if account.Type != service.AccountTypeOAuth {
		response.BadRequest(c, "Only OAuth accounts support privacy setting")
		return
	}
	var mode string
	switch account.Platform {
	case service.PlatformOpenAI:
		mode = h.adminService.ForceOpenAIPrivacy(c.Request.Context(), account)
	case service.PlatformAntigravity:
		mode = h.adminService.ForceAntigravityPrivacy(c.Request.Context(), account)
	default:
		response.BadRequest(c, "Only OpenAI and Antigravity OAuth accounts support privacy setting")
		return
	}
	if mode == "" {
		response.BadRequest(c, "Cannot set privacy: missing access_token")
		return
	}
	// 从 DB 重新读取以确保返回最新状态
	updated, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		// 隐私已设置成功但读取失败，回退到内存更新
		if account.Extra == nil {
			account.Extra = make(map[string]any)
		}
		account.Extra["privacy_mode"] = mode
		response.Success(c, h.buildAccountResponseWithRuntime(c.Request.Context(), account))
		return
	}
	response.Success(c, h.buildAccountResponseWithRuntime(c.Request.Context(), updated))
}

// RefreshTier handles refreshing Google One tier for a single account
// POST /api/v1/admin/accounts/:id/refresh-tier
func (h *AccountHandler) RefreshTier(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	ctx := c.Request.Context()
	account, err := h.adminService.GetAccount(ctx, accountID)
	if err != nil {
		response.NotFound(c, "Account not found")
		return
	}

	if account.Platform != service.PlatformGemini || account.Type != service.AccountTypeOAuth {
		response.BadRequest(c, "Only Gemini OAuth accounts support tier refresh")
		return
	}

	oauthType, _ := account.Credentials["oauth_type"].(string)
	if oauthType != "google_one" {
		response.BadRequest(c, "Only google_one OAuth accounts support tier refresh")
		return
	}

	tierID, extra, creds, err := h.geminiOAuthService.RefreshAccountGoogleOneTier(ctx, account)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	_, updateErr := h.adminService.UpdateAccount(ctx, accountID, &service.UpdateAccountInput{
		Credentials: creds,
		Extra:       extra,
	})
	if updateErr != nil {
		response.ErrorFrom(c, updateErr)
		return
	}

	response.Success(c, gin.H{
		"tier_id":             tierID,
		"storage_info":        extra,
		"drive_storage_limit": extra["drive_storage_limit"],
		"drive_storage_usage": extra["drive_storage_usage"],
		"updated_at":          extra["drive_tier_updated_at"],
	})
}

// BatchRefreshTierRequest represents batch tier refresh request
type BatchRefreshTierRequest struct {
	AccountIDs []int64 `json:"account_ids"`
}

// BatchRefreshTier handles batch refreshing Google One tier
// POST /api/v1/admin/accounts/batch-refresh-tier
func (h *AccountHandler) BatchRefreshTier(c *gin.Context) {
	var req BatchRefreshTierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req = BatchRefreshTierRequest{}
	}

	ctx := c.Request.Context()
	accounts := make([]*service.Account, 0)

	if len(req.AccountIDs) == 0 {
		allAccounts, _, err := h.adminService.ListAccounts(ctx, 1, 10000, "gemini", "oauth", "", "", 0, "", "", "name", "asc")
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		for i := range allAccounts {
			acc := &allAccounts[i]
			oauthType, _ := acc.Credentials["oauth_type"].(string)
			if oauthType == "google_one" {
				accounts = append(accounts, acc)
			}
		}
	} else {
		fetched, err := h.adminService.GetAccountsByIDs(ctx, req.AccountIDs)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}

		for _, acc := range fetched {
			if acc == nil {
				continue
			}
			if acc.Platform != service.PlatformGemini || acc.Type != service.AccountTypeOAuth {
				continue
			}
			oauthType, _ := acc.Credentials["oauth_type"].(string)
			if oauthType != "google_one" {
				continue
			}
			accounts = append(accounts, acc)
		}
	}

	const maxConcurrency = 10
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrency)

	var mu sync.Mutex
	var successCount, failedCount int
	var errors []gin.H

	for _, account := range accounts {
		acc := account // 闭包捕获
		g.Go(func() error {
			_, extra, creds, err := h.geminiOAuthService.RefreshAccountGoogleOneTier(gctx, acc)
			if err != nil {
				mu.Lock()
				failedCount++
				errors = append(errors, gin.H{
					"account_id": acc.ID,
					"error":      err.Error(),
				})
				mu.Unlock()
				return nil
			}

			_, updateErr := h.adminService.UpdateAccount(gctx, acc.ID, &service.UpdateAccountInput{
				Credentials: creds,
				Extra:       extra,
			})

			mu.Lock()
			if updateErr != nil {
				failedCount++
				errors = append(errors, gin.H{
					"account_id": acc.ID,
					"error":      updateErr.Error(),
				})
			} else {
				successCount++
			}
			mu.Unlock()

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	results := gin.H{
		"total":   len(accounts),
		"success": successCount,
		"failed":  failedCount,
		"errors":  errors,
	}

	response.Success(c, results)
}

// GetAntigravityDefaultModelMapping 获取 Antigravity 平台的默认模型映射
// GET /api/v1/admin/accounts/antigravity/default-model-mapping
func (h *AccountHandler) GetAntigravityDefaultModelMapping(c *gin.Context) {
	response.Success(c, domain.DefaultAntigravityModelMapping)
}

// sanitizeExtraBaseRPM 对 extra map 中的 base_rpm 值进行范围校验和归一化。
// 负值归零，超过 10000 截断为 10000。extra 为 nil 或不含 base_rpm 时无操作。
func sanitizeExtraBaseRPM(extra map[string]any) {
	if extra == nil {
		return
	}
	raw, ok := extra["base_rpm"]
	if !ok {
		return
	}
	v := service.ParseExtraInt(raw)
	if v < 0 {
		v = 0
	} else if v > 10000 {
		v = 10000
	}
	extra["base_rpm"] = v
}
