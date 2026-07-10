//go:build unit

package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestGatewayServiceForward_NilGinContextDoesNotPanicBeforeUpstreamError(t *testing.T) {
	body := []byte(`{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`)
	parsed := &ParsedRequest{Body: NewRequestBodyRef(body), Model: "claude-sonnet-4-20250514"}
	upstream := &anthropicHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusTeapot,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"upstream rejected request"}}`)),
		},
	}
	svc := &GatewayService{
		cfg:          &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          14701,
		Name:        "nil-context-forward",
		Platform:    PlatformAnthropic,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "upstream-key",
			"base_url": "https://api.anthropic.com",
		},
		Status:      StatusActive,
		Schedulable: true,
	}

	require.NotPanics(t, func() {
		result, err := svc.Forward(context.Background(), nil, account, parsed)
		require.Nil(t, result)
		require.Error(t, err)
	})
}

func TestGatewayServiceHandleErrorResponse_NilGinContextBadRequestDoesNotPanic(t *testing.T) {
	svc := &GatewayService{cfg: &config.Config{}}
	account := &Account{ID: 14703, Name: "nil-context-bad-request", Platform: PlatformAnthropic}
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"bad request"}}`)),
	}

	require.NotPanics(t, func() {
		result, err := svc.handleErrorResponse(context.Background(), resp, nil, account)
		require.Nil(t, result)
		require.ErrorContains(t, err, "upstream error: 400")
	})
}

func TestGatewayServiceForwardCountTokens_NilGinContextReturnsUpstreamError(t *testing.T) {
	body := []byte(`{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`)
	parsed := &ParsedRequest{Body: NewRequestBodyRef(body), Model: "claude-sonnet-4-20250514"}
	svc := &GatewayService{
		cfg: &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
		httpUpstream: &anthropicHTTPUpstreamRecorder{
			err: errors.New("dial refused"),
		},
	}
	account := &Account{
		ID:          14702,
		Name:        "nil-context-count-tokens",
		Platform:    PlatformAnthropic,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "upstream-key",
			"base_url": "https://api.anthropic.com",
		},
		Status:      StatusActive,
		Schedulable: true,
	}

	require.NotPanics(t, func() {
		err := svc.ForwardCountTokens(context.Background(), nil, account, parsed)
		require.ErrorContains(t, err, "upstream request failed")
	})
}
