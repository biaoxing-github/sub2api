package service

import (
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

// resolveOpenAIUpstreamTLSProfile 返回 OpenAI/Codex 上游请求应使用的 TLS 指纹模板。
//
// 全局 codex_direct 代表整条 OpenAI 上游链路需要进入 Codex 直连传输模式，
// 因此继续使用 gateway.openai_codex_direct_tls_fingerprint_profile_id。
// 账号级 Codex CLI 模拟只对齐 APIKey 请求的 URL、headers 和 body；真实
// Codex CLI 直连 free5 已证明不需要套用这里的 Node.js 24.x TLS 指纹。
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
	return false
}
