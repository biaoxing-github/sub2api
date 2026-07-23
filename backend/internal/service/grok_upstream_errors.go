package service

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// isGrokContentPolicyRejection 只识别由当前请求内容触发的 xAI 403。
// 账号订阅、权限和停用消息可能也包含 policy 字样，必须继续走账号级冷却和切号路径。
func isGrokContentPolicyRejection(statusCode int, responseBody []byte) bool {
	if statusCode != http.StatusForbidden || len(responseBody) == 0 {
		return false
	}
	if grokAccountAccessMessage(string(responseBody)) {
		return false
	}

	var payload any
	if json.Unmarshal(responseBody, &payload) == nil {
		if grokStructuredAccountAccessMarker(payload) {
			return false
		}
		if grokStructuredContentPolicyMarker(payload) {
			return true
		}
	}

	return grokContentPolicyMessage(string(responseBody))
}

// grokStructuredAccountAccessMarker 递归识别账号访问状态，账号状态优先于文本中的策略词。
func grokStructuredAccountAccessMarker(value any) bool {
	switch node := value.(type) {
	case map[string]any:
		for key, child := range node {
			switch normalizeGrokErrorMarker(key) {
			case "code", "error_code", "type", "category", "reason":
				if marker, ok := child.(string); ok && isGrokAccountAccessCode(marker) {
					return true
				}
			}
			if grokStructuredAccountAccessMarker(child) {
				return true
			}
		}
	case []any:
		for _, child := range node {
			if grokStructuredAccountAccessMarker(child) {
				return true
			}
		}
	}
	return false
}

// grokStructuredContentPolicyMarker 递归识别 xAI 返回的明确请求内容策略码。
func grokStructuredContentPolicyMarker(value any) bool {
	switch node := value.(type) {
	case map[string]any:
		for key, child := range node {
			switch normalizeGrokErrorMarker(key) {
			case "code", "error_code", "type", "category", "reason":
				if marker, ok := child.(string); ok && isGrokContentPolicyCode(marker) {
					return true
				}
			}
			if grokStructuredContentPolicyMarker(child) {
				return true
			}
		}
	case []any:
		for _, child := range node {
			if grokStructuredContentPolicyMarker(child) {
				return true
			}
		}
	}
	return false
}

// normalizeGrokErrorMarker 统一错误码的大小写、空格和连字符形式。
func normalizeGrokErrorMarker(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.ReplaceAll(value, " ", "_")
	return value
}

// isGrokContentPolicyCode 判断结构化错误码是否明确归因于本次请求内容。
func isGrokContentPolicyCode(value string) bool {
	switch normalizeGrokErrorMarker(value) {
	case "content_filter",
		"content_policy",
		"content_policy_violation",
		"content_moderation",
		"cyber_policy",
		"new_sensitive":
		return true
	default:
		return false
	}
}

// isGrokAccountAccessCode 判断结构化错误码是否表示账号权限或订阅状态。
func isGrokAccountAccessCode(value string) bool {
	switch normalizeGrokErrorMarker(value) {
	case "account_suspended",
		"account_disabled",
		"user_suspended",
		"user_disabled",
		"subscription_required",
		"entitlement_required",
		"not_entitled",
		"plan_required",
		"permission_denied":
		return true
	default:
		return false
	}
}

// grokAccountAccessMessage 识别没有结构化错误码时仍明确指向账号访问状态的文本。
func grokAccountAccessMessage(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	for _, phrase := range []string{
		"account suspended",
		"account has been suspended",
		"account disabled",
		"account has been disabled",
		"user suspended",
		"user has been suspended",
		"subscription required",
		"entitlement required",
		"not entitled",
	} {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

// grokContentPolicyMessage 使用窄文本集合补充官方媒体安全响应，避免泛化匹配 policy。
func grokContentPolicyMessage(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	if lower == "" {
		return false
	}
	for _, phrase := range []string{
		"the moderation feature is not available",
		"image is sensitive",
		"text is sensitive",
		"prohibited content",
		"forbidden content",
		"content policy violation",
		"content policy rejection",
		"content policy rejected",
		"content moderation rejection",
		"content moderation rejected",
		"content moderation blocked",
		"request blocked by content moderation",
		"request rejected by content moderation",
		"request blocked by policy",
		"request rejected by policy",
		"request violates policy",
		"prompt violates content policy",
		"prompt violates policy",
		"input violates content policy",
		"input violates policy",
	} {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

// grokContentPolicyClientMessage 提取可安全返回给客户端的上游内容策略说明。
func grokContentPolicyClientMessage(responseBody []byte) string {
	message := sanitizeUpstreamErrorMessage(strings.TrimSpace(extractUpstreamErrorMessage(responseBody)))
	if message == "" {
		return "Request blocked by upstream content policy"
	}
	return message
}

// shouldFailoverGrokUpstreamError 将请求级内容拒绝从状态码切号策略中剔除。
func (s *OpenAIGatewayService) shouldFailoverGrokUpstreamError(statusCode int, responseBody []byte) bool {
	if isGrokContentPolicyRejection(statusCode, responseBody) {
		return false
	}
	return s.shouldFailoverUpstreamError(statusCode)
}

// recordGrokContentPolicyRejection 仅记录请求诊断并标记响应已提交，绝不修改账号或配额状态。
func (s *OpenAIGatewayService) recordGrokContentPolicyRejection(c *gin.Context, account *Account, resp *http.Response, responseBody []byte) string {
	message := grokContentPolicyClientMessage(responseBody)
	if c == nil {
		return message
	}

	statusCode := http.StatusForbidden
	requestID := ""
	if resp != nil {
		if resp.StatusCode > 0 {
			statusCode = resp.StatusCode
		}
		requestID = firstNonEmpty(resp.Header.Get("x-request-id"), resp.Header.Get("xai-request-id"))
	}
	detail := ""
	if s != nil && s.cfg != nil && s.cfg.Gateway.LogUpstreamErrorBody {
		maxBytes := s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes
		if maxBytes <= 0 {
			maxBytes = 2048
		}
		detail = truncateString(string(responseBody), maxBytes)
	}
	setOpsUpstreamError(c, statusCode, message, detail)
	if account != nil {
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:           account.Platform,
			AccountID:          account.ID,
			AccountName:        account.Name,
			UpstreamStatusCode: statusCode,
			UpstreamRequestID:  requestID,
			Kind:               "http_error",
			Message:            message,
			Detail:             detail,
		})
	}
	MarkResponseCommitted(c)
	return message
}

// applyGrokForbiddenPolicy 保留管理员已配置的非内容策略 403 临时不可调度规则。
func (s *OpenAIGatewayService) applyGrokForbiddenPolicy(ctx context.Context, account *Account, responseBody []byte) bool {
	if account == nil || !account.IsTempUnschedulableEnabled() {
		return false
	}

	matches := matchTempUnschedulableRules(account, http.StatusForbidden, responseBody)
	if len(matches) == 0 {
		return false
	}

	match := matches[0]
	if s != nil && s.rateLimitService != nil && s.rateLimitService.accountRepo != nil {
		stateCtx, cancel := openAIAccountStateContext(ctx)
		handled := s.rateLimitService.tryTempUnschedulable(
			stateCtx,
			account,
			http.StatusForbidden,
			responseBody,
		)
		cancel()
		if handled {
			return true
		}
	}

	cooldown := time.Duration(match.rule.DurationMinutes) * time.Minute
	if cooldown > 0 {
		s.tempUnscheduleGrok(ctx, account, cooldown, "grok configured forbidden rule")
	}
	return true
}
