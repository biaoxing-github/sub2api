package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestOpenAIWSBridgeRateLimitTerminalBoundary 覆盖两类账号在输出前后收到限流终态的行为。
func TestOpenAIWSBridgeRateLimitTerminalBoundary(t *testing.T) {
	for _, accountType := range []string{AccountTypeOAuth, AccountTypeAPIKey} {
		for _, event := range []string{"error", "response.failed"} {
			for _, partial := range []bool{false, true} {
				name := accountType + "/" + event
				if partial {
					name += "/partial"
				} else {
					name += "/before_output"
				}
				t.Run(name, func(t *testing.T) {
					payload := `{"type":"error","error":{"code":"rate_limit_exceeded","message":"limited"}}`
					if event == "response.failed" {
						payload = `{"type":"response.failed","response":{"error":{"code":"rate_limit_exceeded","message":"limited"}}}`
					}
					body := "data: " + payload + "\n\n"
					if partial {
						body = "data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial\"}\n\n" + body
					}
					upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}}
					svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
					c, _ := gin.CreateTestContext(httptest.NewRecorder())
					c.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
					writes := 0
					result, err := svc.proxyOpenAIWSHTTPBridgeTurn(context.Background(), c, &Account{ID: 8, Platform: PlatformOpenAI, Type: accountType}, "token", []byte(`{"model":"gpt-5","input":[]}`), 0, "gpt-5", "", "", "", 2, func([]byte) error { writes++; return nil })
					require.Error(t, err)
					var failover *UpstreamFailoverError
					if partial {
						require.NotNil(t, result)
						require.False(t, errors.As(err, &failover))
						require.Equal(t, 2, writes)
					} else {
						require.ErrorAs(t, err, &failover)
						require.Equal(t, 429, failover.StatusCode)
						require.Zero(t, writes)
					}
				})
			}
		}
	}
}
