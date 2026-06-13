package service

import (
	"errors"
	"net/http"
	"testing"
)

func TestClassifyUpstreamErrorCoversSharedCategories(t *testing.T) {
	cases := []struct {
		name             string
		statusCode       int
		message          string
		body             []byte
		err              error
		want             string
		wantRetryable    bool
		wantInvalid      bool
		wantRateLimited  bool
		wantLineDegraded bool
		wantPathReason   string
	}{
		{name: "401", statusCode: http.StatusUnauthorized, message: "bad token", want: UpstreamErrorCategoryUnauthorized, wantInvalid: true, wantPathReason: OpenAIPathFailureHTTP401},
		{name: "429", statusCode: http.StatusTooManyRequests, message: "rate limit exceeded", want: UpstreamErrorCategoryRateLimited, wantRetryable: true, wantRateLimited: true, wantPathReason: OpenAIPathFailureHTTP429},
		{name: "cloudflare", statusCode: http.StatusForbidden, body: []byte(`<html><title>Just a moment...</title><center>cloudflare</center>`), want: UpstreamErrorCategoryCloudflareWAF, wantRetryable: true, wantLineDegraded: true, wantPathReason: OpenAIPathFailureOther},
		{name: "client ip circuit", statusCode: http.StatusTooManyRequests, message: "client_ip_error_circuit_open", want: UpstreamErrorCategoryClientIPCircuitOpen, wantRetryable: true, wantRateLimited: true, wantLineDegraded: true, wantPathReason: OpenAIPathFailureHTTP429},
		{name: "unexpected eof", err: errors.New("unexpected EOF"), want: UpstreamErrorCategoryUnexpectedEOF, wantRetryable: true, wantLineDegraded: true, wantPathReason: OpenAIPathFailureEOF},
		{name: "header timeout", err: errors.New("timed out waiting for OpenAI upstream response headers after 20s"), want: UpstreamErrorCategoryHeaderTimeout, wantRetryable: true, wantLineDegraded: true, wantPathReason: OpenAIPathFailureHeaderTimeout},
		{name: "previous response", statusCode: http.StatusBadRequest, message: "previous response not found", want: UpstreamErrorCategoryPreviousResponseNotFound},
		{name: "5xx", statusCode: http.StatusBadGateway, message: "upstream failed", want: UpstreamErrorCategoryUpstream5xx, wantRetryable: true, wantLineDegraded: true, wantPathReason: OpenAIPathFailureOther},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyUpstreamError(UpstreamErrorInput{
				StatusCode: tc.statusCode,
				Message:    tc.message,
				Body:       tc.body,
				Err:        tc.err,
			})
			if got.Category != tc.want {
				t.Fatalf("Category = %q, want %q; got=%+v", got.Category, tc.want, got)
			}
			if got.Retryable != tc.wantRetryable {
				t.Fatalf("Retryable = %v, want %v; got=%+v", got.Retryable, tc.wantRetryable, got)
			}
			if got.AccountInvalid != tc.wantInvalid {
				t.Fatalf("AccountInvalid = %v, want %v; got=%+v", got.AccountInvalid, tc.wantInvalid, got)
			}
			if got.RateLimited != tc.wantRateLimited {
				t.Fatalf("RateLimited = %v, want %v; got=%+v", got.RateLimited, tc.wantRateLimited, got)
			}
			if got.LineDegraded != tc.wantLineDegraded {
				t.Fatalf("LineDegraded = %v, want %v; got=%+v", got.LineDegraded, tc.wantLineDegraded, got)
			}
			if got.PathHealthReason != tc.wantPathReason {
				t.Fatalf("PathHealthReason = %q, want %q; got=%+v", got.PathHealthReason, tc.wantPathReason, got)
			}
		})
	}
}

func TestClassifyUpstreamErrorBusinessLimitsDoNotPollutePathHealth(t *testing.T) {
	cases := []struct {
		name    string
		message string
		want    string
	}{
		{name: "quota", message: "insufficient_quota", want: UpstreamErrorCategoryQuota},
		{name: "balance", message: "Insufficient account balance", want: UpstreamErrorCategoryQuota},
		{name: "subscription gate", message: "No active subscription found for this group", want: UpstreamErrorCategoryBusinessLimited},
		{name: "platform gate", message: "API key group platform is not gemini", want: UpstreamErrorCategoryBusinessLimited},
		{name: "whitelist denial", message: "model claude-opus is not in whitelist", want: UpstreamErrorCategoryBusinessLimited},
		{name: "count tokens gate", message: "count_tokens is not enabled for this group", want: UpstreamErrorCategoryBusinessLimited},
		{name: "local policy denial", message: "request denied by local policy", want: UpstreamErrorCategoryBusinessLimited},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyUpstreamError(UpstreamErrorInput{Message: tc.message})
			if got.Category != tc.want {
				t.Fatalf("Category = %q, want %q; got=%+v", got.Category, tc.want, got)
			}
			if got.PathHealthReason != "" {
				t.Fatalf("PathHealthReason = %q, want empty; got=%+v", got.PathHealthReason, got)
			}
			if got.Retryable || got.LineDegraded {
				t.Fatalf("business limit should not pollute path health (Retryable/LineDegraded); got=%+v", got)
			}
		})
	}
}

func TestClassifyUpstreamErrorDoesNotMaskRealUpstreamFailuresAsBusinessLimit(t *testing.T) {
	cases := []struct {
		name       string
		statusCode int
		message    string
		want       string
	}{
		{name: "401 stays unauthorized", statusCode: http.StatusUnauthorized, message: "invalid api key", want: UpstreamErrorCategoryUnauthorized},
		{name: "429 stays rate limited", statusCode: http.StatusTooManyRequests, message: "daily usage limit exceeded", want: UpstreamErrorCategoryRateLimited},
		{name: "5xx stays upstream", statusCode: http.StatusBadGateway, message: "request denied by local policy", want: UpstreamErrorCategoryUpstream5xx},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyUpstreamError(UpstreamErrorInput{StatusCode: tc.statusCode, Message: tc.message})
			if got.Category != tc.want {
				t.Fatalf("Category = %q, want %q; got=%+v", got.Category, tc.want, got)
			}
		})
	}
}

func TestRetryableSchedulerExhaustionStatusUsesSharedClassifier(t *testing.T) {
	cases := []struct {
		name       string
		statusCode int
		want       bool
	}{
		{name: "429", statusCode: http.StatusTooManyRequests, want: true},
		{name: "502", statusCode: http.StatusBadGateway, want: true},
		{name: "503", statusCode: http.StatusServiceUnavailable, want: true},
		{name: "504", statusCode: http.StatusGatewayTimeout, want: true},
		{name: "500", statusCode: http.StatusInternalServerError, want: false},
		{name: "401", statusCode: http.StatusUnauthorized, want: false},
		{name: "400", statusCode: http.StatusBadRequest, want: false},
		{name: "empty", statusCode: 0, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsRetryableSchedulerExhaustionStatus(tc.statusCode)
			if got != tc.want {
				t.Fatalf("IsRetryableSchedulerExhaustionStatus(%d) = %v, want %v", tc.statusCode, got, tc.want)
			}
		})
	}
}
