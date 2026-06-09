package service

import (
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

// resolveOpenAIUpstreamTLSProfile 返回 OpenAI/Codex 上游请求应使用的 TLS 指纹模板。
//
// 全局 codex_direct 与账号级 Codex CLI 模拟都表示本次 OpenAI 请求需要贴近
// Codex CLI 出口特征；二者共用 gateway.openai_codex_direct_tls_fingerprint_profile_id。
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
	if cfg != nil && cfg.Gateway.OpenAIOAuthCompatMode == config.GatewayOpenAIOAuthCompatModeCodexDirect {
		return true
	}
	return account != nil && account.IsOpenAICodexCLISimulationEnabled()
}
