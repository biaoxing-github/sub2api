package service

const grokMissingUsageMessage = "Grok upstream returned a successful response without billable usage"

// hasBillableOpenAIUsage 判断上游是否返回了可用于计费的正向用量。
// Grok 成功响应缺失 usage 时不能按零费用放行，调用方必须在写回客户端前切号。
func hasBillableOpenAIUsage(usage OpenAIUsage) bool {
	return usage.InputTokens > 0 ||
		usage.OutputTokens > 0 ||
		usage.CacheCreationInputTokens > 0 ||
		usage.CacheReadInputTokens > 0 ||
		usage.ImageOutputTokens > 0
}

// requiresBillableGrokChatUsage 同时按账号平台和模型身份识别 Grok 请求。
// Grok 模型可能由通用 OpenAI 兼容账号提供，不能只依赖账号的平台字段。
func requiresBillableGrokChatUsage(account *Account, models ...string) bool {
	if account != nil && account.Platform == PlatformGrok {
		return true
	}
	for _, model := range models {
		if platform, ok := DetectModelPlatform(model); ok && platform == PlatformGrok {
			return true
		}
	}
	return false
}
