package service

import (
	"net/http"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

const openAIModelNotFoundCooldown = 30 * time.Minute

// isOpenAIModelNotFoundError 识别上游“模型不存在”错误。
// OpenAI 兼容上游有时用 400，有时用 404；只有消息或错误码明确指向模型不存在时才精细冷却。
func isOpenAIModelNotFoundError(statusCode int, body []byte) bool {
	if statusCode != http.StatusBadRequest && statusCode != http.StatusNotFound {
		return false
	}
	code := strings.ToLower(strings.TrimSpace(firstNonEmptyString(
		gjson.GetBytes(body, "error.code").String(),
		gjson.GetBytes(body, "code").String(),
	)))
	errType := strings.ToLower(strings.TrimSpace(firstNonEmptyString(
		gjson.GetBytes(body, "error.type").String(),
		gjson.GetBytes(body, "type").String(),
	)))
	message := strings.ToLower(strings.TrimSpace(extractUpstreamErrorMessage(body)))
	if message == "" {
		message = strings.ToLower(string(body))
	}
	if strings.Contains(code, "model_not_found") || strings.Contains(code, "unknown_model") {
		return true
	}
	if strings.Contains(errType, "not_found") && strings.Contains(message, "model") {
		return true
	}
	if strings.Contains(message, "model not found") ||
		strings.Contains(message, "unknown model") ||
		(strings.Contains(message, "model") && strings.Contains(message, "does not exist")) ||
		(strings.Contains(message, "model") && strings.Contains(message, "not found")) {
		return true
	}
	return false
}

// openAIRequestModelFromBody 从 OpenAI/Anthropic 兼容请求体中提取调度模型。
func openAIRequestModelFromBody(body []byte) string {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return ""
	}
	return strings.TrimSpace(gjson.GetBytes(body, "model").String())
}

func resolveOpenAIModelRateLimitKey(account *Account, requestedModel string) string {
	model := strings.TrimSpace(requestedModel)
	if model == "" {
		return ""
	}
	if account != nil {
		model = strings.TrimSpace(account.GetMappedModel(model))
	}
	return model
}

func isOpenAIPoolModeRetryableOnSameAccount(account *Account, statusCode int, upstreamMsg string, body []byte) bool {
	if account == nil || !account.IsPoolMode() {
		return false
	}
	if isOpenAIModelNotFoundError(statusCode, body) {
		return false
	}
	return account.IsPoolModeRetryableStatus(statusCode) || isOpenAITransientProcessingError(statusCode, upstreamMsg, body)
}
