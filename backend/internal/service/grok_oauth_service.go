package service

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
)

const grokDefaultAccessTokenTTL = 6 * time.Hour

// GrokOAuthClient 定义 xAI OAuth token endpoint 的最小客户端能力。
type GrokOAuthClient interface {
	ExchangeCode(ctx context.Context, code, codeVerifier, codeChallenge, redirectURI, proxyURL, clientID string) (*xai.TokenResponse, error)
	RefreshToken(ctx context.Context, refreshToken, proxyURL, clientID string) (*xai.TokenResponse, error)
}

// GrokOAuthService 管理 Grok OAuth 会话、授权码兑换与 refresh_token 刷新。
type GrokOAuthService struct {
	sessionStore *xai.SessionStore
	proxyRepo    ProxyRepository
	oauthClient  GrokOAuthClient
}

// NewGrokOAuthService 创建 Grok OAuth 服务并启动内存会话存储。
func NewGrokOAuthService(proxyRepo ProxyRepository, oauthClient GrokOAuthClient) *GrokOAuthService {
	return &GrokOAuthService{
		sessionStore: xai.NewSessionStore(),
		proxyRepo:    proxyRepo,
		oauthClient:  oauthClient,
	}
}

// GrokAuthURLResult 是生成授权链接后返回给管理前端的会话信息。
type GrokAuthURLResult struct {
	AuthURL   string `json:"auth_url"`
	SessionID string `json:"session_id"`
	State     string `json:"state"`
}

// GenerateAuthURL 生成带 PKCE 的 xAI OAuth 授权链接并保存临时会话。
func (s *GrokOAuthService) GenerateAuthURL(ctx context.Context, proxyID *int64, redirectURI string) (*GrokAuthURLResult, error) {
	state, err := xai.GenerateState()
	if err != nil {
		return nil, infraerrors.Newf(http.StatusInternalServerError, "GROK_OAUTH_STATE_FAILED", "failed to generate state: %v", err)
	}
	nonce, err := xai.GenerateNonce()
	if err != nil {
		return nil, infraerrors.Newf(http.StatusInternalServerError, "GROK_OAUTH_NONCE_FAILED", "failed to generate nonce: %v", err)
	}
	codeVerifier, err := xai.GenerateCodeVerifier()
	if err != nil {
		return nil, infraerrors.Newf(http.StatusInternalServerError, "GROK_OAUTH_VERIFIER_FAILED", "failed to generate code verifier: %v", err)
	}
	sessionID, err := xai.GenerateSessionID()
	if err != nil {
		return nil, infraerrors.Newf(http.StatusInternalServerError, "GROK_OAUTH_SESSION_FAILED", "failed to generate session ID: %v", err)
	}

	proxyURL, err := s.proxyURL(ctx, proxyID)
	if err != nil {
		return nil, err
	}
	redirectURI = xai.EffectiveRedirectURI(redirectURI)
	codeChallenge := xai.GenerateCodeChallenge(codeVerifier)

	s.sessionStore.Set(sessionID, &xai.OAuthSession{
		State:         state,
		CodeVerifier:  codeVerifier,
		CodeChallenge: codeChallenge,
		ClientID:      xai.EffectiveClientID(),
		Scope:         xai.EffectiveScope(),
		ProxyURL:      proxyURL,
		RedirectURI:   redirectURI,
		CreatedAt:     time.Now(),
	})

	authURL, err := xai.BuildAuthorizationURL(state, codeChallenge, redirectURI, nonce)
	if err != nil {
		return nil, err
	}
	return &GrokAuthURLResult{
		AuthURL:   authURL,
		SessionID: sessionID,
		State:     state,
	}, nil
}

// GrokExchangeCodeInput 是管理前端提交授权码兑换时的输入。
type GrokExchangeCodeInput struct {
	SessionID   string
	Code        string
	State       string
	RedirectURI string
	ProxyID     *int64
}

// GrokTokenInfo 是前端创建账号所需的 Grok token 与订阅信息。
type GrokTokenInfo struct {
	AccessToken       string `json:"access_token"`
	RefreshToken      string `json:"refresh_token,omitempty"`
	IDToken           string `json:"id_token,omitempty"`
	TokenType         string `json:"token_type,omitempty"`
	ExpiresIn         int64  `json:"expires_in"`
	ExpiresAt         int64  `json:"expires_at"`
	ClientID          string `json:"client_id,omitempty"`
	Scope             string `json:"scope,omitempty"`
	Email             string `json:"email,omitempty"`
	SubscriptionTier  string `json:"subscription_tier,omitempty"`
	EntitlementStatus string `json:"entitlement_status,omitempty"`
}

// ExchangeCode 校验 OAuth state 并把授权码兑换为 Grok token。
func (s *GrokOAuthService) ExchangeCode(ctx context.Context, input *GrokExchangeCodeInput) (*GrokTokenInfo, error) {
	if input == nil {
		return nil, infraerrors.New(http.StatusBadRequest, "GROK_OAUTH_INVALID_INPUT", "input is required")
	}
	session, ok := s.sessionStore.Get(input.SessionID)
	if !ok {
		return nil, infraerrors.New(http.StatusBadRequest, "GROK_OAUTH_SESSION_NOT_FOUND", "session not found or expired")
	}

	parsed := xai.ParseAuthorizationInput(input.Code)
	code := strings.TrimSpace(parsed.Code)
	if code == "" {
		return nil, infraerrors.New(http.StatusBadRequest, "GROK_OAUTH_CODE_REQUIRED", "authorization code is required")
	}
	state := strings.TrimSpace(input.State)
	if state == "" {
		state = strings.TrimSpace(parsed.State)
	}
	if state != "" && subtle.ConstantTimeCompare([]byte(state), []byte(session.State)) != 1 {
		return nil, infraerrors.New(http.StatusBadRequest, "GROK_OAUTH_INVALID_STATE", "invalid oauth state")
	}

	proxyURL := session.ProxyURL
	if input.ProxyID != nil {
		var err error
		proxyURL, err = s.proxyURL(ctx, input.ProxyID)
		if err != nil {
			return nil, err
		}
	}
	redirectURI := session.RedirectURI
	if strings.TrimSpace(input.RedirectURI) != "" {
		redirectURI = input.RedirectURI
	}

	tokenResp, err := s.oauthClient.ExchangeCode(ctx, code, session.CodeVerifier, session.CodeChallenge, redirectURI, proxyURL, session.ClientID)
	if err != nil {
		return nil, err
	}
	s.sessionStore.Delete(input.SessionID)
	return s.tokenInfoFromResponse(tokenResp, session.ClientID, nil), nil
}

// RefreshToken 使用指定 refresh_token 和可选 client_id 刷新 Grok token。
func (s *GrokOAuthService) RefreshToken(ctx context.Context, refreshToken, proxyURL, clientID string) (*GrokTokenInfo, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil, infraerrors.New(http.StatusBadRequest, "GROK_OAUTH_NO_REFRESH_TOKEN", "refresh_token is required")
	}
	tokenResp, err := s.oauthClient.RefreshToken(ctx, refreshToken, proxyURL, clientID)
	if err != nil {
		return nil, err
	}
	return s.tokenInfoFromResponse(tokenResp, clientID, nil), nil
}

// ValidateRefreshToken 验证管理端手动粘贴的 Grok refresh_token。
func (s *GrokOAuthService) ValidateRefreshToken(ctx context.Context, refreshToken string, proxyID *int64) (*GrokTokenInfo, error) {
	proxyURL, err := s.proxyURL(ctx, proxyID)
	if err != nil {
		return nil, err
	}
	return s.RefreshToken(ctx, refreshToken, proxyURL, xai.EffectiveClientID())
}

// RefreshAccountToken 刷新已保存的 Grok OAuth 账号 token。
func (s *GrokOAuthService) RefreshAccountToken(ctx context.Context, account *Account) (*GrokTokenInfo, error) {
	if account == nil || account.Platform != PlatformGrok {
		return nil, infraerrors.New(http.StatusBadRequest, "GROK_OAUTH_INVALID_ACCOUNT", "account is not a Grok account")
	}
	if account.Type != AccountTypeOAuth {
		return nil, infraerrors.New(http.StatusBadRequest, "GROK_OAUTH_INVALID_ACCOUNT_TYPE", "account is not an OAuth account")
	}

	proxyURL, err := s.proxyURL(ctx, account.ProxyID)
	if err != nil {
		return nil, err
	}
	refreshToken := account.GetCredential("refresh_token")
	if strings.TrimSpace(refreshToken) == "" {
		return nil, infraerrors.New(http.StatusBadRequest, "GROK_OAUTH_NO_REFRESH_TOKEN", "no refresh token available")
	}

	clientID := account.GetCredential("client_id")
	tokenInfo, err := s.RefreshToken(ctx, refreshToken, proxyURL, clientID)
	if err != nil {
		return nil, err
	}
	tokenInfo.SubscriptionTier = account.GetCredential("subscription_tier")
	tokenInfo.EntitlementStatus = account.GetCredential("entitlement_status")
	return tokenInfo, nil
}

// BuildAccountCredentials 将 Grok token 信息转换为账号 credentials 存储结构。
func (s *GrokOAuthService) BuildAccountCredentials(tokenInfo *GrokTokenInfo) map[string]any {
	if tokenInfo == nil {
		return nil
	}
	expiresAt := time.Unix(tokenInfo.ExpiresAt, 0).UTC().Format(time.RFC3339)
	creds := map[string]any{
		"access_token": tokenInfo.AccessToken,
		"expires_at":   expiresAt,
	}
	if tokenInfo.RefreshToken != "" {
		creds["refresh_token"] = tokenInfo.RefreshToken
	}
	if tokenInfo.TokenType != "" {
		creds["token_type"] = tokenInfo.TokenType
	}
	if tokenInfo.IDToken != "" {
		creds["id_token"] = tokenInfo.IDToken
	}
	if tokenInfo.ClientID != "" {
		creds["client_id"] = tokenInfo.ClientID
	}
	if tokenInfo.Scope != "" {
		creds["scope"] = tokenInfo.Scope
	}
	if tokenInfo.Email != "" {
		creds["email"] = tokenInfo.Email
	}
	if tokenInfo.SubscriptionTier != "" {
		creds["subscription_tier"] = tokenInfo.SubscriptionTier
	}
	if tokenInfo.EntitlementStatus != "" {
		creds["entitlement_status"] = tokenInfo.EntitlementStatus
	}
	creds["base_url"] = xai.DefaultCLIBaseURL
	return creds
}

// Stop 停止 Grok OAuth 临时会话清理器。
func (s *GrokOAuthService) Stop() {
	s.sessionStore.Stop()
}

// tokenInfoFromResponse 规范化 xAI token 响应，补齐默认过期时间和邮箱。
func (s *GrokOAuthService) tokenInfoFromResponse(tokenResp *xai.TokenResponse, clientID string, existing map[string]any) *GrokTokenInfo {
	now := time.Now()
	expiresIn := tokenResp.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = int64(grokDefaultAccessTokenTTL.Seconds())
	}
	info := &GrokTokenInfo{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		IDToken:      tokenResp.IDToken,
		TokenType:    tokenResp.TokenType,
		ExpiresIn:    expiresIn,
		ExpiresAt:    now.Add(time.Duration(expiresIn) * time.Second).Unix(),
		ClientID:     strings.TrimSpace(clientID),
		Scope:        tokenResp.Scope,
	}
	if info.ClientID == "" {
		info.ClientID = xai.EffectiveClientID()
	}
	if info.TokenType == "" {
		info.TokenType = "Bearer"
	}
	if email := parseJWTEmailClaim(tokenResp.IDToken); email != "" {
		info.Email = email
	}
	if info.Email == "" && existing != nil {
		if email, _ := existing["email"].(string); email != "" {
			info.Email = email
		}
	}
	return info
}

// proxyURL 按代理 ID 返回可用于 OAuth 请求的代理地址。
func (s *GrokOAuthService) proxyURL(ctx context.Context, proxyID *int64) (string, error) {
	if proxyID == nil {
		return "", nil
	}
	if s.proxyRepo == nil {
		return "", infraerrors.New(http.StatusBadRequest, "GROK_OAUTH_PROXY_NOT_AVAILABLE", "proxy repository is not available")
	}
	proxy, err := s.proxyRepo.GetByID(ctx, *proxyID)
	if err != nil {
		return "", infraerrors.Newf(http.StatusBadRequest, "GROK_OAUTH_PROXY_NOT_FOUND", "proxy not found: %v", err)
	}
	if proxy == nil {
		return "", nil
	}
	return proxy.URL(), nil
}

// parseJWTEmailClaim 从 id_token payload 中读取 email claim。
func parseJWTEmailClaim(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	return strings.TrimSpace(claims.Email)
}
