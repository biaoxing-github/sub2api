package service

import "github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"

// openAIUpstreamTLSProfile 让管理端人工测试复用正式 OpenAI 网关的 TLS 选择规则。
//
// 人工测试是管理员验证账号的正常流程，账号级 Codex CLI 模拟不能只停留在
// header 构造层；这里确保 Responses、compact、chat completions 与 image
// 测试请求都会使用同一套 OpenAI/Codex TLS profile 解析。
func (s *AccountTestService) openAIUpstreamTLSProfile(account *Account) *tlsfingerprint.Profile {
	if s == nil {
		return resolveOpenAIUpstreamTLSProfile(nil, nil, account)
	}
	return resolveOpenAIUpstreamTLSProfile(s.cfg, s.tlsFPProfileService, account)
}
