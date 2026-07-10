package service

import "strings"

// resolveOpenAIForwardModel 解析 OpenAI 兼容转发使用的模型。
// defaultMappedModel 只服务于 /v1/messages 的 Claude 系列显式调度映射，
// 不作为普通 OpenAI 请求的未知模型兜底。
func resolveOpenAIForwardModel(account *Account, requestedModel, defaultMappedModel string) string {
	if account == nil {
		if defaultMappedModel != "" && claudeMessagesDispatchFamily(requestedModel) != "" {
			return defaultMappedModel
		}
		return requestedModel
	}

	mappedModel, matched := account.ResolveMappedModel(requestedModel)
	if !matched && defaultMappedModel != "" && claudeMessagesDispatchFamily(requestedModel) != "" {
		return defaultMappedModel
	}
	return mappedModel
}

// isOpenAIOAuthServableModel 判断空 model_mapping 的 OpenAI OAuth 账号能否服务请求模型。
// 判定与转发归一化保持一致：已知 Codex 模型、图像模型、推理后缀变体，
// 以及 /v1/messages 可按 Claude 家族默认映射的模型才允许进入该账号。
func isOpenAIOAuthServableModel(requestedModel string) bool {
	model := strings.TrimSpace(requestedModel)
	if model == "" {
		return true
	}
	if claudeMessagesDispatchFamily(model) != "" {
		return true
	}
	if _, ok := normalizeKnownCodexModel(model); ok {
		return true
	}
	if normalized := NormalizeOpenAICompatRequestedModel(model); normalized != model {
		if _, ok := normalizeKnownCodexModel(normalized); ok {
			return true
		}
	}
	return false
}

// resolveOpenAICompactForwardModel determines the compact-only upstream model
// for /responses/compact requests. It never affects normal /responses traffic.
// When no compact-specific mapping matches, the input model is returned as-is.
func resolveOpenAICompactForwardModel(account *Account, model string) string {
	trimmedModel := strings.TrimSpace(model)
	if trimmedModel == "" || account == nil {
		return trimmedModel
	}

	mappedModel, matched := account.ResolveCompactMappedModel(trimmedModel)
	if !matched {
		return trimmedModel
	}
	if trimmedMapped := strings.TrimSpace(mappedModel); trimmedMapped != "" {
		return trimmedMapped
	}
	return trimmedModel
}
