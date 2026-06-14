//go:build unit

package service

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestAccountTestService_AnthropicAPIKeyContext1MAddsBetaHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	account := Account{
		ID:          444,
		Name:        "anyrouter",
		Platform:    PlatformAnthropic,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "upstream-anthropic-key",
			"base_url": "https://anyrouter.example",
			"model_mapping": map[string]any{
				"claude-opus-4-7": "claude-opus-4-7[1m]",
			},
		},
		Extra: map[string]any{
			AnthropicContext1MEnabledExtraKey: true,
		},
	}
	upstream := &anthropicHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body:       io.NopCloser(strings.NewReader("data: {\"type\":\"message_stop\"}\n\n")),
		},
	}
	svc := &AccountTestService{
		accountRepo:  stubOpenAIAccountRepo{accounts: []Account{account}},
		httpUpstream: upstream,
		cfg:          &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/444/test", bytes.NewReader(nil))

	err := svc.TestAccountConnection(c, account.ID, "claude-opus-4-7", "", AccountTestModeDefault)
	require.NoError(t, err)

	require.Equal(t, "https://anyrouter.example/v1/messages?beta=true", upstream.lastReq.URL.String())
	require.Equal(t, "claude-opus-4-7[1m]", gjson.GetBytes(upstream.lastBody, "model").String())
	beta := upstream.lastReq.Header.Get("anthropic-beta")
	require.Contains(t, beta, claude.BetaContext1M)
	require.Contains(t, beta, claude.APIKeyBetaHeader)
	require.Contains(t, rec.Body.String(), `"type":"test_complete"`)
}

func TestAccountTestService_AnthroDefaultModel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	account := Account{
		ID:          555,
		Name:        "test-anthropic",
		Platform:    PlatformAnthropic,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-ant-test-key",
			"base_url": "https://api.anthropic.com",
		},
	}
	upstream := &anthropicHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body:       io.NopCloser(strings.NewReader("data: {\"type\":\"message_stop\"}\n\n")),
		},
	}
	svc := &AccountTestService{
		accountRepo:  stubOpenAIAccountRepo{accounts: []Account{account}},
		httpUpstream: upstream,
		cfg:          &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/555/test", bytes.NewReader(nil))

	// Call with empty modelID - should default to "claude-opus-4-8"
	err := svc.TestAccountConnection(c, account.ID, "", "", AccountTestModeDefault)
	require.NoError(t, err)

	// Verify the model sent to upstream is "claude-opus-4-8"
	actualModel := gjson.GetBytes(upstream.lastBody, "model").String()
	require.Equal(t, "claude-opus-4-8", actualModel, "Default test model should be claude-opus-4-8 when modelID is empty")
}
