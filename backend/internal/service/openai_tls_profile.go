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
	if profileService != nil {
		return profileService.ResolveProfileID(profileID)
	}
	if profileID > 0 {
		return nil
	}
	return BuiltInDefaultTLSFingerprintProfile()
}

func shouldUseOpenAICodexTLSProfile(cfg *config.Config, account *Account) bool {
	return account != nil && account.IsOpenAICodexCLISimulationEnabled()
}
