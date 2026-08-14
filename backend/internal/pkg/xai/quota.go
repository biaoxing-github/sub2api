package xai

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

type QuotaWindow struct {
	Limit     *int64 `json:"limit,omitempty"`
	Remaining *int64 `json:"remaining,omitempty"`
	ResetUnix *int64 `json:"reset_unix,omitempty"`
	ResetAt   string `json:"reset_at,omitempty"`
}

type QuotaSnapshot struct {
	Requests          *QuotaWindow      `json:"requests,omitempty"`
	Tokens            *QuotaWindow      `json:"tokens,omitempty"`
	RetryAfterSeconds *int              `json:"retry_after_seconds,omitempty"`
	SubscriptionTier  string            `json:"subscription_tier,omitempty"`
	EntitlementStatus string            `json:"entitlement_status,omitempty"`
	StatusCode        int               `json:"status_code,omitempty"`
	Headers           map[string]string `json:"headers,omitempty"`
	HeadersObserved   bool              `json:"headers_observed"`
	ObservationSource string            `json:"observation_source,omitempty"`
	LastProbeAt       string            `json:"last_probe_at,omitempty"`
	LastHeadersSeenAt string            `json:"last_headers_seen_at,omitempty"`
	UpdatedAt         string            `json:"updated_at"`
	// Model 是产生当前限流窗口的上游模型，用于区分 SuperGrok 与 Heavy。
	Model string `json:"model,omitempty"`
	// PlanFrom45Responses 保存 grok-4.5 Responses 窗口推断出的档位，避免后续其他模型覆盖。
	PlanFrom45Responses   string `json:"plan_from_45_responses,omitempty"`
	PlanFrom45ResponsesAt string `json:"plan_from_45_responses_at,omitempty"`
}

// HasObservedHeaders 判断快照是否包含任一可用的 xAI 配额响应头。
func (s *QuotaSnapshot) HasObservedHeaders() bool {
	if s == nil {
		return false
	}
	return s.HeadersObserved || s.Requests != nil || s.Tokens != nil ||
		s.RetryAfterSeconds != nil || s.SubscriptionTier != "" ||
		s.EntitlementStatus != "" || len(s.Headers) > 0
}

func ParseQuotaHeaders(headers http.Header, statusCode int) *QuotaSnapshot {
	if headers == nil {
		return nil
	}

	snapshot := &QuotaSnapshot{
		Requests:   parseQuotaWindow(headers, "requests"),
		Tokens:     parseQuotaWindow(headers, "tokens"),
		StatusCode: statusCode,
		Headers:    map[string]string{},
		UpdatedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	if retryAfter := parseRetryAfter(headers.Get("retry-after")); retryAfter != nil {
		snapshot.RetryAfterSeconds = retryAfter
	}
	snapshot.SubscriptionTier = firstHeader(headers, "xai-subscription-tier", "x-subscription-tier")
	snapshot.EntitlementStatus = firstHeader(headers, "xai-entitlement-status", "x-entitlement-status")
	if snapshot.Requests == nil && snapshot.Tokens == nil && snapshot.RetryAfterSeconds == nil && snapshot.SubscriptionTier == "" && snapshot.EntitlementStatus == "" {
		return nil
	}
	snapshot.HeadersObserved = true
	snapshot.LastHeadersSeenAt = snapshot.UpdatedAt
	return snapshot
}

func parseQuotaWindow(headers http.Header, dimension string) *QuotaWindow {
	window := &QuotaWindow{
		Limit:     parseInt64Ptr(headers.Get("x-ratelimit-limit-" + dimension)),
		Remaining: parseInt64Ptr(headers.Get("x-ratelimit-remaining-" + dimension)),
	}
	if reset := parseResetHeader(headers.Get("x-ratelimit-reset-" + dimension)); reset != nil {
		window.ResetUnix = reset
		window.ResetAt = time.Unix(*reset, 0).UTC().Format(time.RFC3339)
	}
	if window.Limit == nil && window.Remaining == nil && window.ResetUnix == nil {
		return nil
	}
	return window
}

func parseResetHeader(raw string) *int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if value, err := strconv.ParseInt(raw, 10, 64); err == nil {
		if value > 1_000_000_000_000 {
			value /= 1000
		}
		return &value
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		value := t.Unix()
		return &value
	}
	return nil
}

func parseRetryAfter(raw string) *int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if value, err := strconv.Atoi(raw); err == nil {
		return &value
	}
	if t, err := http.ParseTime(raw); err == nil {
		seconds := int(time.Until(t).Seconds())
		if seconds < 0 {
			seconds = 0
		}
		return &seconds
	}
	return nil
}

func parseInt64Ptr(raw string) *int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil
	}
	return &value
}

func firstHeader(headers http.Header, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(headers.Get(name)); value != "" {
			return value
		}
	}
	return ""
}
