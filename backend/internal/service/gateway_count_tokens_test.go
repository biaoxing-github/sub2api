//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGatewayService_ForwardCountTokens_OpenAIOAuthUnsupportedUsesLocalFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{
			name:       "missing scope",
			statusCode: http.StatusForbidden,
			body:       `{"error":{"type":"invalid_request_error","code":"missing_scope","message":"Missing scopes: api.responses.write"}}`,
		},
		{
			name:       "input tokens unsupported",
			statusCode: http.StatusNotFound,
			body:       `{"error":{"type":"invalid_request_error","message":"The input_tokens endpoint is not supported"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := &anthropicHTTPUpstreamRecorder{
				resp: &http.Response{
					StatusCode: tt.statusCode,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(tt.body)),
				},
			}
			svc := &GatewayService{
				cfg:              &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}},
				httpUpstream:     upstream,
				rateLimitService: &RateLimitService{},
			}
			account := &Account{
				ID:          7101,
				Name:        "openai-oauth-count-tokens",
				Platform:    PlatformOpenAI,
				Type:        AccountTypeOAuth,
				Concurrency: 1,
				Credentials: map[string]any{
					"access_token": "oauth-token",
				},
				Status:      StatusActive,
				Schedulable: true,
			}
			body := []byte(`{"model":"gpt-5.5","messages":[{"role":"user","content":[{"type":"text","text":"hello world"}]}]}`)
			parsed := &ParsedRequest{Body: NewRequestBodyRef(body), Model: "gpt-5.5"}
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", strings.NewReader(string(body)))
			c.Request.Header.Set("Content-Type", "application/json")

			err := svc.ForwardCountTokens(context.Background(), c, account, parsed)

			require.NoError(t, err)
			require.Equal(t, http.StatusOK, rec.Code)
			require.Greater(t, int(gjson.GetBytes(rec.Body.Bytes(), "input_tokens").Int()), 0)
			require.NotNil(t, upstream.lastReq)
			require.Equal(t, "Bearer oauth-token", getHeaderRaw(upstream.lastReq.Header, "authorization"))
		})
	}
}
