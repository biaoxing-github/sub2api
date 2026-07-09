package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
)

const grokRefreshWindow = 15 * time.Minute

// GrokTokenRefresher 实现 Grok OAuth 账号后台 token 刷新。
type GrokTokenRefresher struct {
	grokOAuthService *GrokOAuthService
}

// NewGrokTokenRefresher 创建 Grok token 刷新器。
func NewGrokTokenRefresher(grokOAuthService *GrokOAuthService) *GrokTokenRefresher {
	return &GrokTokenRefresher{grokOAuthService: grokOAuthService}
}

// CacheKey 返回 Grok OAuth 账号刷新锁使用的缓存键。
func (r *GrokTokenRefresher) CacheKey(account *Account) string {
	if account == nil {
		return "grok:account:0"
	}
	clientID := strings.TrimSpace(account.GetCredential("client_id"))
	refreshToken := strings.TrimSpace(account.GetCredential("refresh_token"))
	if clientID != "" && refreshToken != "" {
		sum := sha256.Sum256([]byte(clientID + ":" + refreshToken))
		return "grok:oauth:" + hex.EncodeToString(sum[:12])
	}
	return "grok:account:" + strconv.FormatInt(account.ID, 10)
}

// CanRefresh 判断账号是否属于 Grok OAuth。
func (r *GrokTokenRefresher) CanRefresh(account *Account) bool {
	return account != nil && account.Platform == PlatformGrok && account.Type == AccountTypeOAuth
}

// NeedsRefresh 检查 Grok access_token 是否进入刷新窗口。
func (r *GrokTokenRefresher) NeedsRefresh(account *Account, _ time.Duration) bool {
	if !r.CanRefresh(account) {
		return false
	}
	expiresAt := account.GetCredentialAsTime("expires_at")
	if expiresAt == nil {
		return false
	}
	return time.Until(*expiresAt) < grokRefreshWindow
}

// Refresh 调用 Grok OAuth 服务刷新 token 并合并旧 credentials。
func (r *GrokTokenRefresher) Refresh(ctx context.Context, account *Account) (map[string]any, error) {
	tokenInfo, err := r.grokOAuthService.RefreshAccountToken(ctx, account)
	if err != nil {
		return nil, err
	}
	newCredentials := r.grokOAuthService.BuildAccountCredentials(tokenInfo)
	return MergeCredentials(account.Credentials, newCredentials), nil
}
