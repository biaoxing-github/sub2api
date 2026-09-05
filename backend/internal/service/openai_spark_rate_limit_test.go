//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// sparkRateLimitRepo 同时观察模型、账号和 handler 补充冷却的持久化结果。
type sparkRateLimitRepo struct {
	openAIWSRateLimitSignalRepo
	modelCalls int       // 模型限流写入次数。
	modelKey   string    // 最后写入的模型。
	modelReset time.Time // 最后模型恢复时间。
	tempCalls  int       // 整账号临时冷却次数。
}

func (r *sparkRateLimitRepo) SetModelRateLimit(_ context.Context, _ int64, model string, reset time.Time) error {
	r.modelCalls++
	r.modelKey, r.modelReset = model, reset
	return nil
}

func (r *sparkRateLimitRepo) SetTempUnschedulable(context.Context, int64, time.Time, string) error {
	r.tempCalls++
	return nil
}

// newSparkRateLimitTestGateway 创建真实限流服务和可检查的仓库边界。
func newSparkRateLimitTestGateway() (*OpenAIGatewayService, *sparkRateLimitRepo, *Account) {
	repo := &sparkRateLimitRepo{}
	account := &Account{ID: 432, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true}
	svc := &OpenAIGatewayService{accountRepo: repo, rateLimitService: &RateLimitService{accountRepo: repo}, cfg: rawChatCompletionsTestConfig(), toolCorrector: NewCodexToolCorrector()}
	svc.rateLimitService.SetAccountRuntimeBlocker(svc)
	return svc, repo, account
}

func sparkQuotaHeaders(used string) http.Header {
	headers := make(http.Header)
	headers.Set("Content-Type", "text/event-stream")
	headers.Set("x-codex-primary-used-percent", used)
	headers.Set("x-codex-primary-reset-after-seconds", "604800")
	headers.Set("x-codex-primary-window-minutes", "10080")
	return headers
}

func sparkTestContext() *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	return c
}

const sparkFailedEvent = `{"type":"response.failed","response":{"id":"resp_spark","status":"failed","error":{"code":"usage_limit_reached","type":"rate_limit_error","message":"Usage limit reached"}}}`

// requireSparkOnlyCooldown 验证当前账号其他模型仍可用且 storm 没有重复计数。
func requireSparkOnlyCooldown(t *testing.T, svc *OpenAIGatewayService, repo *sparkRateLimitRepo, account *Account, longReset bool) {
	t.Helper()
	require.Equal(t, 1, repo.modelCalls)
	require.Equal(t, "gpt-5.3-codex-spark", repo.modelKey)
	require.Empty(t, repo.rateLimitCalls)
	require.Zero(t, repo.tempCalls)
	require.False(t, svc.isOpenAIAccountRuntimeBlocked(account))
	require.True(t, account.isModelRateLimitedWithContext(context.Background(), "gpt-5.3-codex-spark"))
	require.False(t, account.isModelRateLimitedWithContext(context.Background(), "gpt-5.3-codex"))
	require.EqualValues(t, 1, svc.openaiOAuth429WindowCount.Load())
	if longReset {
		require.Greater(t, time.Until(repo.modelReset), 6*24*time.Hour)
	} else {
		require.Positive(t, time.Until(repo.modelReset))
		require.LessOrEqual(t, time.Until(repo.modelReset), time.Duration(defaultRateLimit429CooldownSeconds+1)*time.Second)
	}
}

// TestOpenAISparkRateLimitIngress 覆盖真实 HTTP 错误、SSE 转换和 WS bridge 的公共接线。
func TestOpenAISparkRateLimitIngress(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, ingress := range []string{"http", "chat_buffered", "chat_stream", "messages_buffered", "messages_stream", "responses_stream", "responses_passthrough", "responses_buffered", "passthrough_buffered", "bridge_http", "bridge_error", "bridge_failed", "ws_error", "ws_handshake"} {
		for _, used := range []string{"100", "20"} {
			t.Run(ingress+"/used_"+used, func(t *testing.T) {
				svc, repo, account := newSparkRateLimitTestGateway()
				c := sparkTestContext()
				model := "gpt-5.3-codex-spark"
				headers := sparkQuotaHeaders(used)
				stream := "data: " + sparkFailedEvent + "\n\n"
				resp := &http.Response{StatusCode: http.StatusOK, Header: headers, Body: io.NopCloser(strings.NewReader(stream))}
				var callErr error
				switch ingress {
				case "http":
					resp.StatusCode = http.StatusTooManyRequests
					resp.Body = io.NopCloser(strings.NewReader(`{"error":{"code":"rate_limit_exceeded"}}`))
					callErr = svc.handleFailoverErrorResponsePassthrough(context.Background(), resp, c, account, []byte(`{"model":"gpt-5.3-codex-spark"}`))
				case "chat_buffered":
					_, callErr = svc.handleChatBufferedStreamingResponse(resp, c, account, "alias", model, model, time.Now())
				case "chat_stream":
					_, callErr = svc.handleChatStreamingResponse(resp, c, account, "alias", model, model, false, time.Now(), 0)
				case "messages_buffered":
					_, callErr = svc.handleAnthropicBufferedStreamingResponse(resp, c, account, "alias", model, model, time.Now())
				case "messages_stream":
					_, callErr = svc.handleAnthropicStreamingResponse(resp, c, account, "alias", model, model, time.Now())
				case "responses_stream":
					_, callErr = svc.handleStreamingResponse(context.Background(), resp, c, account, time.Now(), "alias", model)
				case "responses_passthrough":
					_, callErr = svc.handleStreamingResponsePassthrough(context.Background(), resp, c, account, time.Now(), "alias", model)
				case "responses_buffered":
					_, callErr = svc.handleSSEToJSON(resp, c, account, []byte(stream), "alias", model)
				case "passthrough_buffered":
					_, callErr = svc.handlePassthroughSSEToJSON(resp, c, []byte(stream), PlatformOpenAI, "alias", model, false, account)
				case "bridge_http", "bridge_error", "bridge_failed":
					if ingress == "bridge_http" {
						resp.StatusCode = http.StatusTooManyRequests
						resp.Body = io.NopCloser(strings.NewReader(`{"error":{"code":"rate_limit_exceeded"}}`))
					} else if ingress == "bridge_error" {
						resp.Body = io.NopCloser(strings.NewReader("data: {\"type\":\"error\",\"error\":{\"code\":\"rate_limit_exceeded\"}}\n\n"))
					}
					svc.httpUpstream = &httpUpstreamRecorder{resp: resp}
					_, callErr = svc.proxyOpenAIWSHTTPBridgeTurn(context.Background(), c, account, "token", []byte(`{"type":"response.create","model":"gpt-5.3-codex-spark","input":[]}`), 0, model, "", "", "", 1, func([]byte) error { return nil })
				case "ws_error":
					svc.persistOpenAIWSRateLimitSignal(context.Background(), account, headers, []byte(`{"type":"error","error":{"code":"rate_limit_exceeded"}}`), "rate_limit_exceeded", "rate_limit_error", "limit", model)
				case "ws_handshake":
					svc.persistOpenAIWSRateLimitSignal(context.Background(), account, headers, nil, "rate_limit_exceeded", "rate_limit_error", "limit", model)
				}
				var failover *UpstreamFailoverError
				if callErr != nil {
					require.ErrorAs(t, callErr, &failover)
					if ingress != "http" && !strings.HasPrefix(ingress, "bridge_") {
						require.Equal(t, http.StatusBadGateway, failover.StatusCode)
					} else {
						require.Equal(t, http.StatusTooManyRequests, failover.StatusCode)
					}
					svc.TempUnscheduleRetryableError(context.Background(), account.ID, failover)
				}
				requireSparkOnlyCooldown(t, svc, repo, account, used == "100")
			})
		}
	}
}

// TestOpenAISemantic429OrdinaryModelKeepsLocalCooldown 不继承握手长 reset，同时保留普通账号冷却和计数。
func TestOpenAISemantic429OrdinaryModelKeepsLocalCooldown(t *testing.T) {
	for _, mode := range []string{"ws_event", "sse_failed", "http_429"} {
		t.Run(mode, func(t *testing.T) {
			svc, repo, account := newSparkRateLimitTestGateway()
			headers := sparkQuotaHeaders("100")
			if mode == "sse_failed" {
				svc.newOpenAIStreamFailoverErrorWithModel(sparkTestContext(), account, false, "", []byte(sparkFailedEvent), "Usage limit reached", "gpt-5.3-codex", headers)
			} else {
				body := []byte(`{"type":"error","error":{"code":"rate_limit_exceeded"}}`)
				if mode == "http_429" {
					body = nil
				}
				svc.persistOpenAIWSRateLimitSignal(context.Background(), account, headers, body, "rate_limit_exceeded", "rate_limit_error", "", "gpt-5.3-codex")
			}
			require.Zero(t, repo.modelCalls)
			require.Len(t, repo.rateLimitCalls, 1)
			require.True(t, svc.isOpenAIAccountRuntimeBlocked(account))
			require.EqualValues(t, 1, svc.openaiOAuth429WindowCount.Load())
			if mode == "http_429" {
				require.Greater(t, time.Until(repo.rateLimitCalls[0]), 6*24*time.Hour)
			} else {
				require.LessOrEqual(t, time.Until(repo.rateLimitCalls[0]), time.Duration(defaultRateLimit429CooldownSeconds+1)*time.Second)
			}
		})
	}
}

// TestOpenAISparkRateLimitMissingResetDoesNotBorrowOtherWindow 锁定耗尽窗口缺 reset 的分类边界。
func TestOpenAISparkRateLimitMissingResetDoesNotBorrowOtherWindow(t *testing.T) {
	svc, repo, account := newSparkRateLimitTestGateway()
	headers := sparkQuotaHeaders("100")
	headers.Del("x-codex-primary-reset-after-seconds")
	headers.Set("x-codex-secondary-window-minutes", "300")
	headers.Set("x-codex-secondary-used-percent", "10")
	headers.Set("x-codex-secondary-reset-after-seconds", "12000")
	svc.handleOpenAIAccountUpstreamError(context.Background(), account, 429, headers, nil, "gpt-5.3-codex-spark")
	requireSparkOnlyCooldown(t, svc, repo, account, false)
}

// TestOpenAISparkWSForwarder 使用现有 WS 连接池夹具验证真实事件循环中的模型和窗口传递。
func TestOpenAISparkWSForwarder(t *testing.T) {
	for _, payload := range []string{
		`{"type":"error","error":{"code":"rate_limit_exceeded","type":"rate_limit_error","message":"Usage limit reached"}}`,
		sparkFailedEvent,
		`{"type":"codex.rate_limits","rate_limits":{"primary":{"used_percent":100,"reset_after_seconds":604800,"window_minutes":10080}}}`,
	} {
		t.Run(payload, func(t *testing.T) {
			svc, repo, account := newSparkRateLimitTestGateway()
			svc.cfg = newOpenAIWSV2TestConfig()
			account.Concurrency = 1
			pool := newOpenAIWSConnPool(svc.cfg)
			t.Cleanup(pool.Close)
			pool.setClientDialerForTest(&openAIWSCaptureDialer{
				conn:      &openAIWSCaptureConn{events: [][]byte{[]byte(payload)}},
				handshake: sparkQuotaHeaders("100"),
			})
			svc.openaiWSPool = pool
			_, _ = svc.forwardOpenAIWSV2(context.Background(), sparkTestContext(), account,
				map[string]any{"model": "gpt-5.3-codex-spark", "input": []any{}, "stream": true},
				"test-token", OpenAIWSProtocolDecision{Transport: OpenAIUpstreamTransportResponsesWebsocketV2},
				false, true, "alias", "gpt-5.3-codex-spark", time.Now(), 1, "")
			requireSparkOnlyCooldown(t, svc, repo, account, true)
			require.Empty(t, repo.updateExtra, "Spark 窗口不能写入账号全局配额快照")
		})
	}
}
