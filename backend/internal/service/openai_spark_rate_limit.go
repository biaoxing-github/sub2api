package service

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

type openAIOAuth429Disposition uint8

const (
	openAIOAuth429Transient  openAIOAuth429Disposition = iota // 未声明配额窗口的瞬时限流。
	openAIOAuth429Quota5h                                     // 明确耗尽五小时窗口。
	openAIOAuth429Quota7d                                     // 明确耗尽七天窗口。
	openAIOAuth429QuotaReset                                  // 只有 reset，不能证明 Spark 窗口耗尽。
)

// classifyOpenAIOAuth429 保留已耗尽窗口的身份，缺少其 reset 时不借用另一窗口。
func classifyOpenAIOAuth429(headers http.Header, responseBody []byte) (openAIOAuth429Disposition, *time.Time) {
	if snapshot := ParseCodexRateLimitHeaders(headers); snapshot != nil {
		if normalized := snapshot.Normalize(); normalized != nil {
			if normalized.Used7dPercent != nil && *normalized.Used7dPercent >= 100 {
				if normalized.Reset7dSeconds != nil {
					reset := time.Now().Add(time.Duration(*normalized.Reset7dSeconds) * time.Second)
					return openAIOAuth429Quota7d, &reset
				}
				return openAIOAuth429Quota7d, nil
			}
			if normalized.Used5hPercent != nil && *normalized.Used5hPercent >= 100 {
				if normalized.Reset5hSeconds != nil {
					reset := time.Now().Add(time.Duration(*normalized.Reset5hSeconds) * time.Second)
					return openAIOAuth429Quota5h, &reset
				}
				return openAIOAuth429Quota5h, nil
			}
		}
	}
	if reset := calculateOpenAI429ResetTime(headers); reset != nil {
		return openAIOAuth429QuotaReset, reset
	}
	if resetUnix := parseOpenAIRateLimitResetTime(responseBody); resetUnix != nil {
		reset := time.Unix(*resetUnix, 0)
		return openAIOAuth429QuotaReset, &reset
	}
	return openAIOAuth429Transient, nil
}

// HandleOpenAICodexSparkRateLimit 只冷却 Spark，不写入账号级配额或全局使用率快照。
func (s *RateLimitService) HandleOpenAICodexSparkRateLimit(ctx context.Context, account *Account, requestedModel string, statusCode int, headers http.Header, responseBody []byte) bool {
	if s == nil || s.accountRepo == nil || !isOpenAIOAuthAccount(account) || statusCode != http.StatusTooManyRequests || !account.ShouldHandleErrorCode(statusCode) {
		return false
	}
	modelKey := normalizeCodexModel(resolveOpenAIModelRateLimitKey(account, requestedModel))
	if !isCodexSparkModel(modelKey) {
		return false
	}
	disposition, resetAt := classifyOpenAIOAuth429(headers, responseBody)
	if disposition != openAIOAuth429Quota5h && disposition != openAIOAuth429Quota7d {
		resetAt = nil
	}
	if resetAt == nil || !resetAt.After(time.Now()) {
		cooldown, enabled := s.get429FallbackCooldown(ctx, account)
		if !enabled || cooldown <= 0 {
			cooldown = time.Duration(defaultRateLimit429CooldownSeconds) * time.Second
		}
		reset := time.Now().Add(cooldown)
		resetAt = &reset
	}
	if err := s.accountRepo.SetModelRateLimit(ctx, account.ID, modelKey, *resetAt); err != nil {
		slog.Warn("openai_spark_model_rate_limit_set_failed", "account_id", account.ID, "model", modelKey, "error", err)
	}
	updateAccountModelRateLimitExtra(account, modelKey, *resetAt)
	return true
}

// openAIWSSemantic429Headers 只允许 Spark 语义限流使用成功握手或 HTTP 200 的配额头。
func openAIWSSemantic429Headers(account *Account, model string, headers http.Header) http.Header {
	if isOpenAIOAuthAccount(account) && isCodexSparkModel(resolveOpenAIModelRateLimitKey(account, model)) {
		return headers
	}
	return nil
}

// openAIModelRateLimitActionMetadata 告知 handler 模型冷却已完成，防止其通用 429/502 冷却扩大范围。
func openAIModelRateLimitActionMetadata(account *Account, model string, metadata map[string]string) map[string]string {
	model = normalizeCodexModel(resolveOpenAIModelRateLimitKey(account, model))
	if isOpenAIOAuthAccount(account) && isCodexSparkModel(model) && account.isRateLimitActiveForKey(model) {
		return mergeOpenAIStreamActionMetadata(metadata, map[string]string{"model_rate_limit": model})
	}
	return metadata
}

// openAICodexSnapshotHeaders 转换当前 WS 配额事件，避免借用旧握手快照。
func openAICodexSnapshotHeaders(snapshot *OpenAICodexUsageSnapshot) http.Header {
	headers := make(http.Header)
	if snapshot == nil {
		return headers
	}
	normalized := snapshot.Normalize()
	if normalized == nil {
		return headers
	}
	for _, window := range []struct {
		prefix  string   // Codex 响应头窗口前缀。
		used    *float64 // 当前窗口使用百分比。
		reset   *int     // 当前窗口剩余秒数。
		minutes string   // 归一化窗口长度。
	}{
		{"x-codex-primary-", normalized.Used5hPercent, normalized.Reset5hSeconds, "300"},
		{"x-codex-secondary-", normalized.Used7dPercent, normalized.Reset7dSeconds, "10080"},
	} {
		headers.Set(window.prefix+"window-minutes", window.minutes)
		if window.used != nil {
			headers.Set(window.prefix+"used-percent", strconv.FormatFloat(*window.used, 'f', -1, 64))
		}
		if window.reset != nil {
			headers.Set(window.prefix+"reset-after-seconds", strconv.Itoa(*window.reset))
		}
	}
	return headers
}

// handleOpenAIStreamRateLimit 从流内错误提取限流语义；返回 true 表示已处理，不能再整账号避让。
func (s *OpenAIGatewayService) handleOpenAIStreamRateLimit(ctx context.Context, account *Account, payload []byte, model string, headers http.Header) bool {
	// API Key 继续走既有流错误停调；此路径只接管 OAuth 的独立配额窗口。
	if !isOpenAIOAuthAccount(account) {
		return false
	}
	code := firstNonEmptyString(gjson.GetBytes(payload, "response.error.code").String(), gjson.GetBytes(payload, "error.code").String())
	errType := firstNonEmptyString(gjson.GetBytes(payload, "response.error.type").String(), gjson.GetBytes(payload, "error.type").String())
	message := extractOpenAISSEErrorMessage(payload)
	if !isOpenAIWSRateLimitError(code, errType, message) {
		return false
	}
	if strings.TrimSpace(model) == "" {
		model = firstNonEmptyString(gjson.GetBytes(payload, "response.model").String(), gjson.GetBytes(payload, "model").String())
	}
	s.handleOpenAIAccountUpstreamError(ctx, account, http.StatusTooManyRequests, openAIWSSemantic429Headers(account, model, headers), payload, model)
	return true
}
