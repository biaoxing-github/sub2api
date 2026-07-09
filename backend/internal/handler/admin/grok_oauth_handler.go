package admin

import (
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// GrokOAuthHandler 承接管理端 Grok OAuth 授权、授权码兑换和 RT 验证请求。
type GrokOAuthHandler struct {
	grokOAuthService *service.GrokOAuthService
}

// NewGrokOAuthHandler 创建 Grok OAuth 管理端 handler。
func NewGrokOAuthHandler(grokOAuthService *service.GrokOAuthService) *GrokOAuthHandler {
	return &GrokOAuthHandler{grokOAuthService: grokOAuthService}
}

// GrokGenerateAuthURLRequest 是生成 Grok 授权链接的请求体。
type GrokGenerateAuthURLRequest struct {
	ProxyID     *int64 `json:"proxy_id"`
	RedirectURI string `json:"redirect_uri"`
}

// GenerateAuthURL 生成 xAI/Grok OAuth 授权链接。
// POST /api/v1/admin/grok/oauth/auth-url
func (h *GrokOAuthHandler) GenerateAuthURL(c *gin.Context) {
	var req GrokGenerateAuthURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求无效: "+err.Error())
		return
	}

	redirectURI := strings.TrimSpace(req.RedirectURI)
	if redirectURI == "" {
		redirectURI = deriveGrokRedirectURI(c)
	}
	result, err := h.grokOAuthService.GenerateAuthURL(c.Request.Context(), req.ProxyID, redirectURI)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// GrokExchangeCodeRequest 是 Grok OAuth 授权码兑换请求体。
type GrokExchangeCodeRequest struct {
	SessionID   string `json:"session_id" binding:"required"`
	State       string `json:"state"`
	Code        string `json:"code" binding:"required"`
	RedirectURI string `json:"redirect_uri"`
	ProxyID     *int64 `json:"proxy_id"`
}

// ExchangeCode 用授权码换取 Grok token。
// POST /api/v1/admin/grok/oauth/exchange-code
func (h *GrokOAuthHandler) ExchangeCode(c *gin.Context) {
	var req GrokExchangeCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求无效: "+err.Error())
		return
	}
	tokenInfo, err := h.grokOAuthService.ExchangeCode(c.Request.Context(), &service.GrokExchangeCodeInput{
		SessionID:   req.SessionID,
		State:       req.State,
		Code:        req.Code,
		RedirectURI: req.RedirectURI,
		ProxyID:     req.ProxyID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, tokenInfo)
}

// GrokRefreshTokenRequest 是手动 refresh_token 验证请求体。
type GrokRefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
	ProxyID      *int64 `json:"proxy_id"`
}

// RefreshToken 验证 Grok refresh_token 并返回可创建账号的 token 信息。
// POST /api/v1/admin/grok/oauth/refresh-token
func (h *GrokOAuthHandler) RefreshToken(c *gin.Context) {
	var req GrokRefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求无效: "+err.Error())
		return
	}
	tokenInfo, err := h.grokOAuthService.ValidateRefreshToken(c.Request.Context(), req.RefreshToken, req.ProxyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, tokenInfo)
}

// deriveGrokRedirectURI 根据请求头生成管理端本地 callback 地址。
func deriveGrokRedirectURI(c *gin.Context) string {
	origin := strings.TrimSpace(c.GetHeader("Origin"))
	if origin != "" {
		return strings.TrimRight(origin, "/") + "/auth/callback"
	}

	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if xfProto := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")); xfProto != "" {
		scheme = strings.TrimSpace(strings.Split(xfProto, ",")[0])
	}

	host := strings.TrimSpace(c.Request.Host)
	if xfHost := strings.TrimSpace(c.GetHeader("X-Forwarded-Host")); xfHost != "" {
		host = strings.TrimSpace(strings.Split(xfHost, ",")[0])
	}
	return fmt.Sprintf("%s://%s/auth/callback", scheme, host)
}
