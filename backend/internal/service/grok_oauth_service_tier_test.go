//go:build unit

package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/stretchr/testify/require"
)

type grokOAuthTierClient struct {
	refreshResponse *xai.TokenResponse
}

func (c *grokOAuthTierClient) ExchangeCode(context.Context, string, string, string, string, string, string) (*xai.TokenResponse, error) {
	return nil, nil
}

func (c *grokOAuthTierClient) RefreshToken(context.Context, string, string, string) (*xai.TokenResponse, error) {
	return c.refreshResponse, nil
}

func TestGrokOAuthRefreshOverwritesStaleTierFromAccessToken(t *testing.T) {
	svc := NewGrokOAuthService(nil, &grokOAuthTierClient{refreshResponse: &xai.TokenResponse{
		AccessToken: grokOAuthJWTWithClaims(t, map[string]any{"tier": 0}),
		ExpiresIn:   3600,
	}})
	defer svc.Stop()
	account := &Account{Platform: PlatformGrok, Type: AccountTypeOAuth, Credentials: map[string]any{
		"refresh_token": "refresh-token", "subscription_tier": "supergrok_heavy",
	}}

	info, err := svc.RefreshAccountToken(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, "free", info.SubscriptionTier)
	require.Equal(t, "free", svc.BuildAccountCredentials(info)["subscription_tier"])
}

func TestGrokOAuthRefreshKeepsStoredTierWithoutAccessTokenClaim(t *testing.T) {
	svc := NewGrokOAuthService(nil, &grokOAuthTierClient{refreshResponse: &xai.TokenResponse{
		AccessToken: "opaque-access-token",
		IDToken:     grokOAuthJWTWithClaims(t, map[string]any{"tier": 5}),
		ExpiresIn:   3600,
	}})
	defer svc.Stop()
	account := &Account{Platform: PlatformGrok, Type: AccountTypeOAuth, Credentials: map[string]any{
		"refresh_token": "refresh-token", "subscription_tier": "supergrok_lite",
	}}

	info, err := svc.RefreshAccountToken(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, "supergrok_lite", info.SubscriptionTier)
}

func grokOAuthJWTWithClaims(t *testing.T, claims map[string]any) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	require.NoError(t, err)
	return "header." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}
