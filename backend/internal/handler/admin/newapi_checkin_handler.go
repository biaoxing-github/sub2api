package admin

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// NewAPICheckinHandler 承载 NewApi 多站点签到 dashboard 的管理端 HTTP 接口。
type NewAPICheckinHandler struct {
	checkinService *service.NewAPICheckinService
}

// NewNewAPICheckinHandler 创建 NewApi 签到 dashboard handler。
func NewNewAPICheckinHandler(checkinService *service.NewAPICheckinService) *NewAPICheckinHandler {
	return &NewAPICheckinHandler{checkinService: checkinService}
}

// newAPICheckinSiteRequest 是只定位站点的请求体。
type newAPICheckinSiteRequest struct {
	// Site 是 NewApi 站点名。
	Site string `json:"site"`
}

// newAPICheckinAccountRequest 是定位站点账号的请求体。
type newAPICheckinAccountRequest struct {
	// Site 是 NewApi 站点名。
	Site string `json:"site"`
	// UserID 是 New-Api-User 对应的上游用户 ID。
	UserID string `json:"user_id"`
}

// newAPICheckinAccountDisplayNameRequest 是管理员手工维护账号标识的请求体。
type newAPICheckinAccountDisplayNameRequest struct {
	// Site 是账号所属站点名。
	Site string `json:"site"`
	// UserID 是账号在签到工具配置中的稳定标识。
	UserID string `json:"user_id"`
	// DisplayName 是管理员填写的用户名或邮箱。
	DisplayName string `json:"display_name"`
}

// newAPICheckinAPIKeyRequest 定位账号下的上游 token。
type newAPICheckinAPIKeyRequest struct {
	// Site 是账号所属站点名。
	Site string `json:"site"`
	// UserID 是账号在签到工具中的稳定标识。
	UserID string `json:"user_id"`
	// APIKeyID 是上游 token ID。
	APIKeyID int64 `json:"api_key_id"`
}

// newAPICheckinAPIKeyGroupRequest 是更新上游 token 分组的请求体。
type newAPICheckinAPIKeyGroupRequest struct {
	newAPICheckinAPIKeyRequest
	// Group 是目标分组名称。
	Group string `json:"group"`
	// GroupID 是 sub2api 的目标分组 ID。
	GroupID int64 `json:"group_id"`
}

// newAPICheckinLinkAPIKeyRequest 描述签到 Key 写入主平台账号的目标与操作。
type newAPICheckinLinkAPIKeyRequest struct {
	newAPICheckinAPIKeyRequest
	// TargetAccountID 是与站点 URL 匹配的主平台账号 ID。
	TargetAccountID int64 `json:"target_account_id"`
	// Operation 是 append 或 replace。
	Operation string `json:"operation"`
}

// newAPICheckinLoginCredentialRequest 是账号登录凭据保存和测试请求体。
type newAPICheckinLoginCredentialRequest struct {
	newAPICheckinAccountRequest
	// LoginUsername 是上游登录账号或邮箱。
	LoginUsername string `json:"login_username"`
	// LoginPassword 是新密码；测试时为空会使用已保存密码。
	LoginPassword string `json:"login_password"`
}

// newAPICheckinSiteEnabledRequest 是站点签到启停请求体。
type newAPICheckinSiteEnabledRequest struct {
	// Site 是 NewApi 站点名。
	Site string `json:"site"`
	// Enabled 表示是否允许自动/手动签到。
	Enabled bool `json:"enabled"`
	// DisabledReason 是关闭签到时展示给页面的原因。
	DisabledReason string `json:"disabled_reason"`
}

// newAPICheckinMonthlyRequest 是月度同步请求体。
type newAPICheckinMonthlyRequest struct {
	// Month 是 YYYY-MM 月份，空值表示当前月份。
	Month string `json:"month"`
	// Site 是可选站点范围。
	Site string `json:"site"`
	// UserID 是可选账号范围。
	UserID string `json:"user_id"`
}

// GetConfig GET /admin/newapi-checkin/config
func (h *NewAPICheckinHandler) GetConfig(c *gin.Context) {
	data, err := h.checkinService.ConfigSummary(c.Request.Context())
	respondNewAPICheckin(c, data, err)
}

// GetAPIKeys GET /admin/newapi-checkin/api-keys
func (h *NewAPICheckinHandler) GetAPIKeys(c *gin.Context) {
	data, err := h.checkinService.APIKeys(c.Request.Context())
	respondNewAPICheckin(c, data, err)
}

// RevealAPIKey POST /admin/newapi-checkin/reveal-api-key
func (h *NewAPICheckinHandler) RevealAPIKey(c *gin.Context) {
	var req newAPICheckinAPIKeyRequest
	if !bindNewAPICheckinJSON(c, &req) {
		return
	}
	if req.APIKeyID <= 0 {
		response.BadRequest(c, "api_key_id 必须是正整数")
		return
	}
	data, err := h.checkinService.RevealAPIKey(c.Request.Context(), strings.TrimSpace(req.Site), strings.TrimSpace(req.UserID), req.APIKeyID)
	respondNewAPICheckin(c, data, err)
}

// UpdateAPIKeyGroup POST /admin/newapi-checkin/api-key-group
func (h *NewAPICheckinHandler) UpdateAPIKeyGroup(c *gin.Context) {
	var req newAPICheckinAPIKeyGroupRequest
	if !bindNewAPICheckinJSON(c, &req) {
		return
	}
	if req.APIKeyID <= 0 {
		response.BadRequest(c, "api_key_id 必须是正整数")
		return
	}
	data, err := h.checkinService.UpdateAPIKeyGroup(
		c.Request.Context(), strings.TrimSpace(req.Site), strings.TrimSpace(req.UserID), req.APIKeyID, strings.TrimSpace(req.Group), req.GroupID,
	)
	respondNewAPICheckin(c, data, err)
}

// LinkAPIKeyToAccount POST /admin/newapi-checkin/link-api-key 将签到 Key 写入主平台账号。
func (h *NewAPICheckinHandler) LinkAPIKeyToAccount(c *gin.Context) {
	var req newAPICheckinLinkAPIKeyRequest
	if !bindNewAPICheckinJSON(c, &req) {
		return
	}
	if req.APIKeyID <= 0 || req.TargetAccountID <= 0 {
		response.BadRequest(c, "api_key_id 和 target_account_id 必须是正整数")
		return
	}
	data, err := h.checkinService.LinkAPIKeyToAccount(c.Request.Context(), strings.TrimSpace(req.Site), strings.TrimSpace(req.UserID), req.APIKeyID, req.TargetAccountID, strings.TrimSpace(req.Operation))
	respondNewAPICheckin(c, data, err)
}

// SaveLoginCredentials POST /admin/newapi-checkin/login-credentials
func (h *NewAPICheckinHandler) SaveLoginCredentials(c *gin.Context) {
	var req newAPICheckinLoginCredentialRequest
	if !bindNewAPICheckinJSON(c, &req) {
		return
	}
	data, err := h.checkinService.SetAccountLoginCredentials(
		c.Request.Context(), strings.TrimSpace(req.Site), strings.TrimSpace(req.UserID),
		strings.TrimSpace(req.LoginUsername), req.LoginPassword,
	)
	respondNewAPICheckin(c, data, err)
}

// TestLoginCredentials POST /admin/newapi-checkin/test-login-credentials
func (h *NewAPICheckinHandler) TestLoginCredentials(c *gin.Context) {
	var req newAPICheckinLoginCredentialRequest
	if !bindNewAPICheckinJSON(c, &req) {
		return
	}
	data, err := h.checkinService.TestAccountLoginCredentials(
		c.Request.Context(), strings.TrimSpace(req.Site), strings.TrimSpace(req.UserID),
		strings.TrimSpace(req.LoginUsername), req.LoginPassword,
	)
	respondNewAPICheckin(c, data, err)
}

// GetLastRun GET /admin/newapi-checkin/last-run
func (h *NewAPICheckinHandler) GetLastRun(c *gin.Context) {
	data, err := h.checkinService.LastRun(c.Request.Context())
	respondNewAPICheckin(c, data, err)
}

// GetBalances GET /admin/newapi-checkin/balances
func (h *NewAPICheckinHandler) GetBalances(c *gin.Context) {
	data, err := h.checkinService.Balances(c.Request.Context())
	respondNewAPICheckin(c, data, err)
}

// GetHistory GET /admin/newapi-checkin/history
func (h *NewAPICheckinHandler) GetHistory(c *gin.Context) {
	data, err := h.checkinService.History(c.Request.Context())
	respondNewAPICheckin(c, data, err)
}

// GetBalanceHistory GET /admin/newapi-checkin/balance-history
func (h *NewAPICheckinHandler) GetBalanceHistory(c *gin.Context) {
	data, err := h.checkinService.BalanceHistory(
		c.Request.Context(),
		strings.TrimSpace(c.Query("month")),
		strings.TrimSpace(c.Query("site")),
		strings.TrimSpace(c.Query("user_id")),
	)
	respondNewAPICheckin(c, data, err)
}

// GetMonthly GET /admin/newapi-checkin/monthly
func (h *NewAPICheckinHandler) GetMonthly(c *gin.Context) {
	data, err := h.checkinService.Monthly(
		c.Request.Context(),
		strings.TrimSpace(c.Query("month")),
		strings.TrimSpace(c.Query("site")),
		strings.TrimSpace(c.Query("user_id")),
	)
	respondNewAPICheckin(c, data, err)
}

// GetMonthlySyncStatus GET /admin/newapi-checkin/monthly-sync-status
func (h *NewAPICheckinHandler) GetMonthlySyncStatus(c *gin.Context) {
	response.Success(c, h.checkinService.MonthlySyncStatus())
}

// GetCheckinJobStatus GET /admin/newapi-checkin/checkin-job-status
func (h *NewAPICheckinHandler) GetCheckinJobStatus(c *gin.Context) {
	response.Success(c, h.checkinService.CheckinJobStatus())
}

// SyncUsernames POST /admin/newapi-checkin/sync-usernames
func (h *NewAPICheckinHandler) SyncUsernames(c *gin.Context) {
	response.BadRequest(c, "全局名称同步已关闭，请使用站点或账号级同步。")
}

// AddOrMergeSite POST /admin/newapi-checkin/config-site
func (h *NewAPICheckinHandler) AddOrMergeSite(c *gin.Context) {
	var payload map[string]any
	if !bindNewAPICheckinJSON(c, &payload) {
		return
	}
	data, err := h.checkinService.AddOrMergeSites(c.Request.Context(), payload)
	respondNewAPICheckin(c, data, err)
}

// DeleteSite POST /admin/newapi-checkin/delete-site
func (h *NewAPICheckinHandler) DeleteSite(c *gin.Context) {
	var req newAPICheckinSiteRequest
	if !bindNewAPICheckinJSON(c, &req) {
		return
	}
	data, err := h.checkinService.DeleteSite(c.Request.Context(), strings.TrimSpace(req.Site))
	respondNewAPICheckin(c, data, err)
}

// DeleteAccount POST /admin/newapi-checkin/delete-account
func (h *NewAPICheckinHandler) DeleteAccount(c *gin.Context) {
	req, ok := bindNewAPICheckinAccountRequest(c)
	if !ok {
		return
	}
	data, err := h.checkinService.DeleteAccount(c.Request.Context(), req.Site, req.UserID)
	respondNewAPICheckin(c, data, err)
}

// SetAccountDisplayName POST /admin/newapi-checkin/account-display-name
func (h *NewAPICheckinHandler) SetAccountDisplayName(c *gin.Context) {
	var req newAPICheckinAccountDisplayNameRequest
	if !bindNewAPICheckinJSON(c, &req) {
		return
	}
	data, err := h.checkinService.SetAccountDisplayName(
		c.Request.Context(),
		strings.TrimSpace(req.Site),
		strings.TrimSpace(req.UserID),
		strings.TrimSpace(req.DisplayName),
	)
	respondNewAPICheckin(c, data, err)
}

// SetSiteEnabled POST /admin/newapi-checkin/site-enabled
func (h *NewAPICheckinHandler) SetSiteEnabled(c *gin.Context) {
	var req newAPICheckinSiteEnabledRequest
	if !bindNewAPICheckinJSON(c, &req) {
		return
	}
	data, err := h.checkinService.SetSiteEnabled(
		c.Request.Context(),
		strings.TrimSpace(req.Site),
		req.Enabled,
		strings.TrimSpace(req.DisabledReason),
	)
	respondNewAPICheckin(c, data, err)
}

// RefreshSiteBalances POST /admin/newapi-checkin/refresh-site-balances
func (h *NewAPICheckinHandler) RefreshSiteBalances(c *gin.Context) {
	var req newAPICheckinSiteRequest
	if !bindNewAPICheckinJSON(c, &req) {
		return
	}
	data, err := h.checkinService.RefreshSiteBalances(c.Request.Context(), strings.TrimSpace(req.Site))
	respondNewAPICheckin(c, data, err)
}

// RefreshAccountBalance POST /admin/newapi-checkin/refresh-account-balance
func (h *NewAPICheckinHandler) RefreshAccountBalance(c *gin.Context) {
	req, ok := bindNewAPICheckinAccountRequest(c)
	if !ok {
		return
	}
	data, err := h.checkinService.RefreshAccountBalance(c.Request.Context(), req.Site, req.UserID)
	respondNewAPICheckin(c, data, err)
}

// SyncSiteNames POST /admin/newapi-checkin/sync-site-names
func (h *NewAPICheckinHandler) SyncSiteNames(c *gin.Context) {
	var req newAPICheckinSiteRequest
	if !bindNewAPICheckinJSON(c, &req) {
		return
	}
	data, err := h.checkinService.SyncSiteNames(c.Request.Context(), strings.TrimSpace(req.Site))
	respondNewAPICheckin(c, data, err)
}

// SyncAccountName POST /admin/newapi-checkin/sync-account-name
func (h *NewAPICheckinHandler) SyncAccountName(c *gin.Context) {
	req, ok := bindNewAPICheckinAccountRequest(c)
	if !ok {
		return
	}
	data, err := h.checkinService.SyncAccountName(c.Request.Context(), req.Site, req.UserID)
	respondNewAPICheckin(c, data, err)
}

// SyncMonthly POST /admin/newapi-checkin/sync-monthly
func (h *NewAPICheckinHandler) SyncMonthly(c *gin.Context) {
	var req newAPICheckinMonthlyRequest
	if !bindNewAPICheckinJSON(c, &req) {
		return
	}
	response.Success(c, h.checkinService.StartMonthlySyncJob(
		c.Request.Context(),
		strings.TrimSpace(req.Month),
		strings.TrimSpace(req.Site),
		strings.TrimSpace(req.UserID),
	))
}

// RunFullCheckin POST /admin/newapi-checkin/run-full-checkin
func (h *NewAPICheckinHandler) RunFullCheckin(c *gin.Context) {
	response.Success(c, h.checkinService.StartFullCheckinJob(c.Request.Context()))
}

// CheckinSingle POST /admin/newapi-checkin/checkin-single
func (h *NewAPICheckinHandler) CheckinSingle(c *gin.Context) {
	req, ok := bindNewAPICheckinAccountRequest(c)
	if !ok {
		return
	}
	data, err := h.checkinService.RunSingleCheckin(c.Request.Context(), req.Site, req.UserID)
	respondNewAPICheckin(c, data, err)
}

func bindNewAPICheckinAccountRequest(c *gin.Context) (newAPICheckinAccountRequest, bool) {
	var req newAPICheckinAccountRequest
	if !bindNewAPICheckinJSON(c, &req) {
		return req, false
	}
	req.Site = strings.TrimSpace(req.Site)
	req.UserID = strings.TrimSpace(req.UserID)
	if req.Site == "" || req.UserID == "" {
		response.BadRequest(c, "site 和 user_id 不能为空")
		return req, false
	}
	return req, true
}

func bindNewAPICheckinJSON(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return false
	}
	return true
}

func respondNewAPICheckin(c *gin.Context, data any, err error) {
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, data)
}
