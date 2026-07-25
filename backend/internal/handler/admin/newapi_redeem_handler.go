package admin

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// NewAPIRedeemHandler 承载单平台 NewAPI 账号与兑换码工具的管理端接口。
type NewAPIRedeemHandler struct {
	redeemService *service.NewAPIRedeemService
}

// NewNewAPIRedeemHandler 创建 NewAPI 兑换工具 handler。
func NewNewAPIRedeemHandler(redeemService *service.NewAPIRedeemService) *NewAPIRedeemHandler {
	return &NewAPIRedeemHandler{redeemService: redeemService}
}

// newAPIRedeemImportRequest 是批量导入 API Key 模式账号的请求体。
type newAPIRedeemImportRequest struct {
	// Accounts 是需要新增或更新的账号列表。
	Accounts []service.NewAPIRedeemAccountInput `json:"accounts"`
}

// newAPIRedeemCreateKeyRequest 是创建上游 API Key 的请求体。
type newAPIRedeemCreateKeyRequest struct {
	// Name 是新 API Key 的显示名称。
	Name string `json:"name"`
	// Group 是可选的目标分组。
	Group string `json:"group"`
}

// newAPIRedeemUpdateKeyGroupRequest 是修改单个上游 API Key 分组的请求体。
type newAPIRedeemUpdateKeyGroupRequest struct {
	// Group 是目标分组名称。
	Group string `json:"group"`
}

// newAPIRedeemLinkAPIKeyRequest 是将兑换 Key 写入 Sub2API 账号的请求体。
type newAPIRedeemLinkAPIKeyRequest struct {
	// TargetAccountID 是目标 OpenAI API Key 账号 ID。
	TargetAccountID int64 `json:"target_account_id"`
	// Operation 为 append 或 replace。
	Operation string `json:"operation"`
}

const newAPIRedeemManualLogLimit = 100

// Overview GET /admin/newapi-redeem/overview
func (h *NewAPIRedeemHandler) Overview(c *gin.Context) {
	data, err := h.redeemService.Overview(c.Request.Context())
	if err == nil {
		includeLogs := newAPIRedeemIncludeLogs(c)
		for index := range data.Runs {
			data.Runs[index] = projectNewAPIRedeemRunLogs(data.Runs[index], includeLogs, 0)
		}
	}
	respondNewAPIRedeem(c, data, err)
}

// ImportAccounts POST /admin/newapi-redeem/accounts/import
func (h *NewAPIRedeemHandler) ImportAccounts(c *gin.Context) {
	var request newAPIRedeemImportRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "请求体格式无效: "+err.Error())
		return
	}
	data, err := h.redeemService.ImportAccounts(c.Request.Context(), request.Accounts)
	respondNewAPIRedeem(c, data, err)
}

// DeleteAccount DELETE /admin/newapi-redeem/accounts/:account_id
func (h *NewAPIRedeemHandler) DeleteAccount(c *gin.Context) {
	err := h.redeemService.DeleteAccount(c.Request.Context(), c.Param("account_id"))
	respondNewAPIRedeem(c, gin.H{"deleted": true}, err)
}

// RefreshAccount POST /admin/newapi-redeem/accounts/:account_id/refresh
func (h *NewAPIRedeemHandler) RefreshAccount(c *gin.Context) {
	data, err := h.redeemService.RefreshAccount(c.Request.Context(), c.Param("account_id"))
	respondNewAPIRedeem(c, data, err)
}

// CreateAPIKey POST /admin/newapi-redeem/accounts/:account_id/api-keys
func (h *NewAPIRedeemHandler) CreateAPIKey(c *gin.Context) {
	var request newAPIRedeemCreateKeyRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "请求体格式无效: "+err.Error())
		return
	}
	data, err := h.redeemService.CreateAPIKey(c.Request.Context(), c.Param("account_id"), request.Name, request.Group)
	respondNewAPIRedeem(c, data, err)
}

// UpdateAPIKeyGroup PUT /admin/newapi-redeem/accounts/:account_id/api-keys/:api_key_id/group
func (h *NewAPIRedeemHandler) UpdateAPIKeyGroup(c *gin.Context) {
	apiKeyID, ok := newAPIRedeemAPIKeyID(c)
	if !ok {
		return
	}
	var request newAPIRedeemUpdateKeyGroupRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "请求体格式无效: "+err.Error())
		return
	}
	data, err := h.redeemService.UpdateAPIKeyGroup(c.Request.Context(), c.Param("account_id"), apiKeyID, request.Group)
	respondNewAPIRedeem(c, data, err)
}

// RevealAPIKey POST /admin/newapi-redeem/accounts/:account_id/api-keys/:api_key_id/reveal
func (h *NewAPIRedeemHandler) RevealAPIKey(c *gin.Context) {
	apiKeyID, ok := newAPIRedeemAPIKeyID(c)
	if !ok {
		return
	}
	data, err := h.redeemService.RevealAPIKey(c.Request.Context(), c.Param("account_id"), apiKeyID)
	respondNewAPIRedeem(c, data, err)
}

// LinkAPIKey POST /admin/newapi-redeem/accounts/:account_id/api-keys/:api_key_id/link
func (h *NewAPIRedeemHandler) LinkAPIKey(c *gin.Context) {
	apiKeyID, ok := newAPIRedeemAPIKeyID(c)
	if !ok {
		return
	}
	var request newAPIRedeemLinkAPIKeyRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "请求体格式无效: "+err.Error())
		return
	}
	if request.TargetAccountID <= 0 {
		response.BadRequest(c, "target_account_id 必须是正整数")
		return
	}
	data, err := h.redeemService.LinkAPIKeyToAccount(c.Request.Context(), c.Param("account_id"), apiKeyID, request.TargetAccountID, request.Operation)
	respondNewAPIRedeem(c, data, err)
}

// UploadFiles POST /admin/newapi-redeem/files 使用 multipart 字段 files 上传多个文本文件。
func (h *NewAPIRedeemHandler) UploadFiles(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		response.BadRequest(c, "读取上传文件失败: "+err.Error())
		return
	}
	files := form.File["files"]
	if len(files) == 0 {
		response.BadRequest(c, "请使用 files 字段上传至少一个文件")
		return
	}
	data, err := h.redeemService.SaveVoucherFiles(c.Request.Context(), files)
	respondNewAPIRedeem(c, data, err)
}

// DeleteFile DELETE /admin/newapi-redeem/files/:file_id
func (h *NewAPIRedeemHandler) DeleteFile(c *gin.Context) {
	err := h.redeemService.DeleteVoucherFile(c.Request.Context(), c.Param("file_id"))
	respondNewAPIRedeem(c, gin.H{"deleted": true}, err)
}

// StartRun POST /admin/newapi-redeem/runs
func (h *NewAPIRedeemHandler) StartRun(c *gin.Context) {
	var request service.NewAPIRedeemStartRunRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "请求体格式无效: "+err.Error())
		return
	}
	data, err := h.redeemService.StartRedemption(c.Request.Context(), request)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Accepted(c, data)
}

// GetRun GET /admin/newapi-redeem/runs/:run_id
func (h *NewAPIRedeemHandler) GetRun(c *gin.Context) {
	data, err := h.redeemService.GetRun(c.Request.Context(), c.Param("run_id"))
	if err == nil {
		data = projectNewAPIRedeemRunLogs(data, newAPIRedeemIncludeLogs(c), newAPIRedeemLogLimit(c))
	}
	respondNewAPIRedeem(c, data, err)
}

// CancelRun POST /admin/newapi-redeem/runs/:run_id/cancel
func (h *NewAPIRedeemHandler) CancelRun(c *gin.Context) {
	data, err := h.redeemService.CancelRun(c.Request.Context(), c.Param("run_id"))
	respondNewAPIRedeem(c, data, err)
}

func newAPIRedeemAPIKeyID(c *gin.Context) (int64, bool) {
	value, err := strconv.ParseInt(strings.TrimSpace(c.Param("api_key_id")), 10, 64)
	if err != nil || value <= 0 {
		response.BadRequest(c, "api_key_id 必须是正整数")
		return 0, false
	}
	return value, true
}

// newAPIRedeemIncludeLogs 保持旧调用方默认返回日志，仅允许页面轮询显式关闭日志。
func newAPIRedeemIncludeLogs(c *gin.Context) bool {
	return !strings.EqualFold(strings.TrimSpace(c.Query("include_logs")), "false")
}

// newAPIRedeemLogLimit 解析手动日志快照上限，并限制单次最多返回 100 条。
func newAPIRedeemLogLimit(c *gin.Context) int {
	value, err := strconv.Atoi(strings.TrimSpace(c.Query("log_limit")))
	if err != nil || value <= 0 {
		return 0
	}
	if value > newAPIRedeemManualLogLimit {
		return newAPIRedeemManualLogLimit
	}
	return value
}

// projectNewAPIRedeemRunLogs 返回独立日志切片；限量时保留最近的请求记录。
func projectNewAPIRedeemRunLogs(run service.NewAPIRedeemRun, includeLogs bool, limit int) service.NewAPIRedeemRun {
	if !includeLogs {
		run.Logs = []service.NewAPIRedeemRequestLog{}
		return run
	}
	logs := run.Logs
	if limit > 0 && len(logs) > limit {
		logs = logs[len(logs)-limit:]
	}
	run.Logs = append([]service.NewAPIRedeemRequestLog{}, logs...)
	return run
}

func respondNewAPIRedeem(c *gin.Context, data any, err error) {
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, data)
}
