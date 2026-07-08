package admin

import (
	"encoding/json"
	"html"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// TokenCostHandler 承载 token 成本计算器旧页面兼容接口。
type TokenCostHandler struct {
	tokenCostService *service.TokenCostService
}

// NewTokenCostHandler 创建 token 成本计算器管理端 handler。
func NewTokenCostHandler(tokenCostService *service.TokenCostService) *TokenCostHandler {
	return &TokenCostHandler{tokenCostService: tokenCostService}
}

type tokenCostStateRequest struct {
	// State 是旧页面提交的完整计算器状态。
	State service.TokenCostState `json:"state"`
}

// Health GET /admin/token-cost/health
func (h *TokenCostHandler) Health(c *gin.Context) {
	if h == nil || h.tokenCostService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"ok": false, "error": "token cost service is not configured"})
		return
	}
	c.JSON(http.StatusOK, h.tokenCostService.Health())
}

// GetState GET /admin/token-cost/state
func (h *TokenCostHandler) GetState(c *gin.Context) {
	if h == nil || h.tokenCostService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"ok": false, "error": "token cost service is not configured"})
		return
	}
	state, err := h.tokenCostService.GetState(c.Request.Context())
	if err != nil {
		respondTokenCostError(c, err)
		return
	}
	if c.Query("view") == "page" {
		respondTokenCostStatePage(c, state)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "state": state})
}

// SaveState POST /admin/token-cost/state
func (h *TokenCostHandler) SaveState(c *gin.Context) {
	if h == nil || h.tokenCostService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"ok": false, "error": "token cost service is not configured"})
		return
	}
	var req tokenCostStateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "Invalid request: " + err.Error()})
		return
	}
	if _, err := h.tokenCostService.SaveState(c.Request.Context(), req.State); err != nil {
		respondTokenCostError(c, err)
		return
	}
	health := h.tokenCostService.Health()
	c.JSON(http.StatusOK, gin.H{"ok": true, "state": health.State, "storage": health.Storage})
}

func respondTokenCostError(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
}

func respondTokenCostStatePage(c *gin.Context, state service.TokenCostState) {
	raw, err := json.Marshal(state)
	if err != nil {
		respondTokenCostError(c, err)
		return
	}
	page := "<!doctype html><html><body data-state='" + html.EscapeString(string(raw)) + "'></body></html>"
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(page))
}
