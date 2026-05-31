package service

import (
	"errors"
	"io"
	"net/http"
	"strings"
)

const (
	UpstreamErrorCategoryOK                       = "ok"
	UpstreamErrorCategoryUnauthorized             = "unauthorized"
	UpstreamErrorCategoryRateLimited              = "rate_limited"
	UpstreamErrorCategoryCloudflareWAF            = "cloudflare_waf"
	UpstreamErrorCategoryClientIPCircuitOpen      = "client_ip_circuit_open"
	UpstreamErrorCategoryUnexpectedEOF            = "unexpected_eof"
	UpstreamErrorCategoryHeaderTimeout            = "header_timeout"
	UpstreamErrorCategoryPreviousResponseNotFound = "previous_response_not_found"
	UpstreamErrorCategoryUpstream5xx              = "upstream_5xx"
	UpstreamErrorCategoryRequestTooLarge          = "request_too_large"
	UpstreamErrorCategoryTimeout                  = "timeout"
	UpstreamErrorCategoryQuota                    = "quota"
	UpstreamErrorCategoryBusinessLimited          = "business_limited"
	UpstreamErrorCategoryReauthRequired           = "reauth_required"
	UpstreamErrorCategoryUpstreamError            = "upstream_error"
)

type UpstreamErrorInput struct {
	StatusCode int
	Message    string
	Body       []byte
	Err        error
}

type UpstreamErrorClass struct {
	Category         string
	Label            string
	PathHealthReason string
	Retryable        bool
	AccountInvalid   bool
	RateLimited      bool
	LineDegraded     bool
}

func ClassifyUpstreamError(input UpstreamErrorInput) UpstreamErrorClass {
	msg := strings.TrimSpace(input.Message)
	if msg == "" && len(input.Body) > 0 {
		msg = strings.TrimSpace(extractUpstreamErrorMessage(input.Body))
	}
	raw := strings.TrimSpace(msg + " " + string(input.Body))
	if input.Err != nil {
		raw = strings.TrimSpace(raw + " " + input.Err.Error())
	}
	lower := strings.ToLower(raw)

	switch {
	case input.Err == nil && input.StatusCode >= 200 && input.StatusCode < 400 && strings.TrimSpace(raw) == "":
		return upstreamErrorClass(UpstreamErrorCategoryOK, "正常", "", false, false, false, false)
	case input.StatusCode == http.StatusRequestEntityTooLarge || strings.Contains(lower, "413 request entity too large"):
		return upstreamErrorClass(UpstreamErrorCategoryRequestTooLarge, "请求体过大/413", "", false, false, false, false)
	case input.StatusCode == http.StatusUnauthorized ||
		strings.Contains(lower, "401") ||
		strings.Contains(lower, "unauthorized") ||
		strings.Contains(lower, "authentication failed") ||
		strings.Contains(lower, "invalid api key") ||
		strings.Contains(lower, "token invalid"):
		return upstreamErrorClass(UpstreamErrorCategoryUnauthorized, "认证失败/401", OpenAIPathFailureHTTP401, false, true, false, false)
	case strings.Contains(lower, "client_ip_error_circuit_open") ||
		strings.Contains(lower, "当前来源短时间错误过多"):
		return upstreamErrorClass(UpstreamErrorCategoryClientIPCircuitOpen, "来源 IP 熔断", OpenAIPathFailureHTTP429, true, false, true, true)
	case input.StatusCode == http.StatusTooManyRequests ||
		strings.Contains(lower, "429") ||
		strings.Contains(lower, "rate limit") ||
		strings.Contains(lower, "rate_limit") ||
		strings.Contains(lower, "too many requests"):
		return upstreamErrorClass(UpstreamErrorCategoryRateLimited, "429 限流", OpenAIPathFailureHTTP429, true, false, true, false)
	case strings.Contains(lower, "previous_response_not_found") ||
		strings.Contains(lower, "previous response not found"):
		return upstreamErrorClass(UpstreamErrorCategoryPreviousResponseNotFound, "previous_response_not_found", "", false, false, false, false)
	case strings.Contains(lower, "cloudflare") ||
		strings.Contains(lower, "cf-ray") ||
		strings.Contains(lower, "error 522") ||
		strings.Contains(lower, "error 524") ||
		strings.Contains(lower, "just a moment"):
		return upstreamErrorClass(UpstreamErrorCategoryCloudflareWAF, "Cloudflare/WAF 拦截", OpenAIPathFailureOther, true, false, false, true)
	case errors.Is(input.Err, io.ErrUnexpectedEOF) ||
		errors.Is(input.Err, io.EOF) ||
		strings.Contains(lower, "unexpected eof") ||
		strings.Contains(lower, "stream error"):
		return upstreamErrorClass(UpstreamErrorCategoryUnexpectedEOF, "unexpected EOF", OpenAIPathFailureEOF, true, false, false, true)
	case strings.Contains(lower, "timeout awaiting response headers") ||
		strings.Contains(lower, "timed out waiting for openai upstream response headers") ||
		strings.Contains(lower, "header timeout"):
		return upstreamErrorClass(UpstreamErrorCategoryHeaderTimeout, "响应头超时", OpenAIPathFailureHeaderTimeout, true, false, false, true)
	case strings.Contains(lower, "context deadline exceeded") ||
		strings.Contains(lower, "timeout"):
		return upstreamErrorClass(UpstreamErrorCategoryTimeout, "请求超时", OpenAIPathFailureHeaderTimeout, true, false, false, true)
	case input.StatusCode >= 500 || strings.Contains(lower, "upstream request failed"):
		return upstreamErrorClass(UpstreamErrorCategoryUpstream5xx, "上游 5xx/网关错误", OpenAIPathFailureOther, true, false, false, true)
	case strings.Contains(lower, "quota") ||
		strings.Contains(lower, "insufficient_quota") ||
		strings.Contains(lower, "insufficient balance") ||
		strings.Contains(lower, "insufficient account balance") ||
		strings.Contains(lower, "usage_limit"):
		return upstreamErrorClass(UpstreamErrorCategoryQuota, "额度不足", "", false, false, false, false)
	case isUpstreamBusinessLimitMessage(lower):
		return upstreamErrorClass(UpstreamErrorCategoryBusinessLimited, "业务限制/策略拒绝", "", false, false, false, false)
	case strings.Contains(lower, "no access token") || strings.Contains(lower, "no refresh token") || strings.Contains(lower, "invalid_grant"):
		return upstreamErrorClass(UpstreamErrorCategoryReauthRequired, "需要重新授权", "", false, true, false, false)
	default:
		return upstreamErrorClass(UpstreamErrorCategoryUpstreamError, "上游错误", OpenAIPathFailureOther, true, false, false, false)
	}
}

func isUpstreamBusinessLimitMessage(lower string) bool {
	lower = strings.TrimSpace(lower)
	if lower == "" {
		return false
	}
	if strings.Contains(lower, "no active subscription found for this group") ||
		strings.Contains(lower, "api key in query parameter is deprecated") ||
		strings.Contains(lower, "query parameter api_key is deprecated") ||
		strings.Contains(lower, "api key group platform is not") ||
		strings.Contains(lower, "daily usage limit exceeded") ||
		strings.Contains(lower, "weekly usage limit exceeded") ||
		strings.Contains(lower, "monthly usage limit exceeded") ||
		strings.Contains(lower, "requests-per-minute limit exceeded") {
		return true
	}
	if strings.Contains(lower, "count_tokens") &&
		containsAnyUpstreamErrorText(lower, "not enabled", "disabled", "not supported", "unsupported", "denied", "forbidden", "not allowed") {
		return true
	}
	if (strings.Contains(lower, "whitelist") || strings.Contains(lower, "white list")) &&
		containsAnyUpstreamErrorText(lower, "not in", "denied", "forbidden", "not allowed", "disallowed", "blocked") {
		return true
	}
	if strings.Contains(lower, "policy") &&
		containsAnyUpstreamErrorText(lower, "denied", "forbidden", "not allowed", "disallowed", "blocked") {
		return true
	}
	return false
}

func containsAnyUpstreamErrorText(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func upstreamErrorClass(category, label, pathHealthReason string, retryable, accountInvalid, rateLimited, lineDegraded bool) UpstreamErrorClass {
	return UpstreamErrorClass{
		Category:         category,
		Label:            label,
		PathHealthReason: pathHealthReason,
		Retryable:        retryable,
		AccountInvalid:   accountInvalid,
		RateLimited:      rateLimited,
		LineDegraded:     lineDegraded,
	}
}
