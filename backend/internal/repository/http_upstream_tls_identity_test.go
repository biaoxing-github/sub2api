package repository

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

// TestBuildOpenAITransportCacheKeyIsolatesAccountProfileAndAuthorization 验证 HTTP 客户端不会跨身份复用。
func TestBuildOpenAITransportCacheKeyIsolatesAccountProfileAndAuthorization(t *testing.T) {
	profileA := &tlsfingerprint.Profile{Name: "Chrome 100", Preset: tlsfingerprint.ClientHelloPresetChrome100}
	profileACopy := &tlsfingerprint.Profile{Name: "Chrome 100", Preset: tlsfingerprint.ClientHelloPresetChrome100}
	profileB := &tlsfingerprint.Profile{Name: "Firefox 105", Preset: tlsfingerprint.ClientHelloPresetFirefox105}

	keyA := buildOpenAITransportCacheKey("base", 1, profileA, "Bearer sk-team-a")
	require.Equal(t, keyA, buildOpenAITransportCacheKey("base", 1, profileACopy, "Bearer sk-team-a"))
	require.NotEqual(t, keyA, buildOpenAITransportCacheKey("base", 2, profileA, "Bearer sk-team-a"))
	require.NotEqual(t, keyA, buildOpenAITransportCacheKey("base", 1, profileB, "Bearer sk-team-a"))
	require.NotEqual(t, keyA, buildOpenAITransportCacheKey("base", 1, profileA, "Bearer sk-team-b"))
	require.NotContains(t, keyA, "sk-team-a")
	require.NotContains(t, keyA, "Bearer")
}
