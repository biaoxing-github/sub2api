//go:build unit

package xai

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMapJWTSubscriptionTierNumber(t *testing.T) {
	t.Parallel()
	require.Equal(t, "free", MapJWTSubscriptionTier(0))
	require.Equal(t, "supergrok", MapJWTSubscriptionTier(1))
	require.Equal(t, "supergrok_heavy", MapJWTSubscriptionTier(5))
	require.Equal(t, "supergrok_lite", MapJWTSubscriptionTier(6))
	require.Equal(t, "9", MapJWTSubscriptionTier(9))
}

func TestNormalizeSubscriptionTierAliases(t *testing.T) {
	t.Parallel()
	require.Equal(t, "free", NormalizeSubscriptionTier("free-tier"))
	require.Equal(t, "supergrok_pro", NormalizeSubscriptionTier("SuperGrokPro"))
	require.Equal(t, "supergrok_heavy", NormalizeSubscriptionTier("SuperGrok Heavy"))
	require.Equal(t, "supergrok_lite", NormalizeSubscriptionTier("SuperGrok Lite"))
}

func TestSubscriptionTierFromJWTUsesNumericClaim(t *testing.T) {
	t.Parallel()
	require.Equal(t, "supergrok_heavy", SubscriptionTierFromJWT(jwtWithTierClaims(t, map[string]any{"tier": 5})))
	require.Equal(t, "free", SubscriptionTierFromJWT(jwtWithTierClaims(t, map[string]any{"tier": "0"})))
	require.Equal(t, "supergrok_lite", SubscriptionTierFromJWT(jwtWithTierClaims(t, map[string]any{"tier": 6})))
	require.Empty(t, SubscriptionTierFromJWT(jwtWithTierClaims(t, map[string]any{"sub": "user"})))
	require.Empty(t, SubscriptionTierFromJWT("not-a-jwt"))
}

func jwtWithTierClaims(t *testing.T, claims map[string]any) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	require.NoError(t, err)
	return "header." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}
