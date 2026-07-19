package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

// TestResolveOpenAIUpstreamTLSProfileStablePerAccount 验证同一账号稳定、前十个账号完整覆盖十套模板。
func TestResolveOpenAIUpstreamTLSProfileStablePerAccount(t *testing.T) {
	cfg := &config.Config{}
	seen := make(map[tlsfingerprint.ClientHelloPreset]struct{}, 10)

	for accountID := int64(1); accountID <= 10; accountID++ {
		account := openAICodexTLSProfileTestAccount(accountID)
		profile := resolveOpenAIUpstreamTLSProfile(cfg, nil, account)
		require.NotNil(t, profile)
		require.NotEmpty(t, profile.Preset)
		seen[profile.Preset] = struct{}{}
	}
	require.Len(t, seen, 10)

	accountA := openAICodexTLSProfileTestAccount(1)
	accountB := openAICodexTLSProfileTestAccount(2)
	profileA1 := resolveOpenAIUpstreamTLSProfile(cfg, nil, accountA)
	profileB := resolveOpenAIUpstreamTLSProfile(cfg, nil, accountB)
	profileA2 := resolveOpenAIUpstreamTLSProfile(cfg, nil, accountA)
	require.Equal(t, profileA1.Preset, profileA2.Preset)
	require.Equal(t, tlsfingerprint.ProfileCacheKey(profileA1), tlsfingerprint.ProfileCacheKey(profileA2))
	require.NotEqual(t, profileA1.Preset, profileB.Preset)
	require.Equal(t, profileA1.Preset, resolveOpenAIUpstreamTLSProfile(cfg, nil, openAICodexTLSProfileTestAccount(11)).Preset)
}

// TestResolveOpenAIUpstreamTLSProfileExplicitProfileWins 验证显式数据库模板继续覆盖自动映射。
func TestResolveOpenAIUpstreamTLSProfileExplicitProfileWins(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAICodexDirectTLSFingerprintProfileID = 42
	profileService := &TLSFingerprintProfileService{
		localCache: map[int64]*model.TLSFingerprintProfile{
			42: {
				ID:           42,
				Name:         "custom-profile",
				CipherSuites: []uint16{0x1301, 0x1302},
			},
		},
	}

	profile := resolveOpenAIUpstreamTLSProfile(cfg, profileService, openAICodexTLSProfileTestAccount(1))
	require.NotNil(t, profile)
	require.Equal(t, "custom-profile", profile.Name)
	require.Empty(t, profile.Preset)
	require.Equal(t, []uint16{0x1301, 0x1302}, profile.CipherSuites)
}

// TestResolveOpenAIUpstreamTLSProfileDisabledAccount 验证未开启 Codex 模拟的账号保持原有 TLS 行为。
func TestResolveOpenAIUpstreamTLSProfileDisabledAccount(t *testing.T) {
	account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	require.Nil(t, resolveOpenAIUpstreamTLSProfile(&config.Config{}, nil, account))
}

// openAICodexTLSProfileTestAccount 构造开启账号级 Codex 模拟的测试账号。
func openAICodexTLSProfileTestAccount(accountID int64) *Account {
	return &Account{
		ID:       accountID,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Extra: map[string]any{
			OpenAICodexCLISimulationEnabledExtraKey: true,
		},
	}
}
