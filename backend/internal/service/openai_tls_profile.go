package service

import (
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

// resolveOpenAIUpstreamTLSProfile 返回 OpenAI/Codex 上游请求应使用的 TLS 指纹模板。
// 触发范围限定在账号级 Codex CLI 模拟，避免全局 OAuth 兼容模式覆盖所有 OpenAI 账号。
func resolveOpenAIUpstreamTLSProfile(cfg *config.Config, profileService *TLSFingerprintProfileService, account *Account) *tlsfingerprint.Profile {
	if !shouldUseOpenAICodexTLSProfile(cfg, account) {
		return nil
	}

	profileID := int64(0)
	if cfg != nil {
		profileID = cfg.Gateway.OpenAICodexDirectTLSFingerprintProfileID
	}
	if profileID == 0 {
		return builtInOpenAICodexTLSFingerprintProfile(account.ID)
	}
	if profileService != nil {
		return profileService.ResolveProfileID(profileID)
	}
	if profileID > 0 {
		return nil
	}
	return BuiltInDefaultTLSFingerprintProfile()
}

// builtInOpenAICodexTLSFingerprintProfiles 保存按账号轮转的十套固定 uTLS 模板。
var builtInOpenAICodexTLSFingerprintProfiles = [...]tlsfingerprint.Profile{
	{Name: "Chrome 100", Preset: tlsfingerprint.ClientHelloPresetChrome100},
	{Name: "iOS 12.1", Preset: tlsfingerprint.ClientHelloPresetIOS121},
	{Name: "Firefox 105", Preset: tlsfingerprint.ClientHelloPresetFirefox105},
	{Name: "Firefox 120", Preset: tlsfingerprint.ClientHelloPresetFirefox120},
	{Name: "Safari 16.0", Preset: tlsfingerprint.ClientHelloPresetSafari160},
	{Name: "iOS 14", Preset: tlsfingerprint.ClientHelloPresetIOS14},
	{Name: "Android 11 OkHttp", Preset: tlsfingerprint.ClientHelloPresetAndroid11},
	{Name: "Edge 85", Preset: tlsfingerprint.ClientHelloPresetEdge85},
	{Name: "360 Browser 7.5", Preset: tlsfingerprint.ClientHelloPreset360Browser75},
	{Name: "QQ Browser 11.1", Preset: tlsfingerprint.ClientHelloPresetQQBrowser111},
}

// builtInOpenAICodexTLSFingerprintProfile 按 sub2api 账号 ID 稳定轮转十套内置指纹。
func builtInOpenAICodexTLSFingerprintProfile(accountID int64) *tlsfingerprint.Profile {
	index := 0
	if accountID > 0 {
		index = int((uint64(accountID) - 1) % uint64(len(builtInOpenAICodexTLSFingerprintProfiles)))
	}
	profile := builtInOpenAICodexTLSFingerprintProfiles[index]
	return &profile
}

func shouldUseOpenAICodexTLSProfile(cfg *config.Config, account *Account) bool {
	return account != nil && account.IsOpenAICodexCLISimulationEnabled()
}
