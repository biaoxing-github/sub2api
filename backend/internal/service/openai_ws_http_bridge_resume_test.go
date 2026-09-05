package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestBuildOpenAIWSCurrentTurnRetryPayloadRejectsOrphanToolOutput(t *testing.T) {
	payload := []byte(`{"type":"response.create","model":"mapped-model","previous_response_id":"resp_old"}`)
	fullInput := []json.RawMessage{
		json.RawMessage(`{"type":"function_call_output","call_id":"missing_call","output":"done"}`),
	}

	retryPayload, retrySafe, err := buildOpenAIWSCurrentTurnRetryPayload(payload, fullInput, true, "gpt-5.6-sol")

	require.NoError(t, err)
	require.False(t, retrySafe)
	require.Nil(t, retryPayload)
}

// TestBuildOpenAIWSCurrentTurnRetryPayloadContext 验证缺失历史拒绝、模型恢复及请求隔离。
func TestBuildOpenAIWSCurrentTurnRetryPayloadContext(t *testing.T) {
	payload := []byte(`{"type":"response.create","model":"mapped","previous_response_id":"resp_old"}`)
	input := []json.RawMessage{json.RawMessage(`{"role":"user","content":"current"}`)}
	retry, ok, err := buildOpenAIWSCurrentTurnRetryPayload(payload, input, false, "original")
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, retry)
	retry, ok, err = buildOpenAIWSCurrentTurnRetryPayload(payload, input, true, "original")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "original", gjson.GetBytes(retry, "model").String())
	require.False(t, gjson.GetBytes(retry, "previous_response_id").Exists())
	require.Equal(t, "resp_old", gjson.GetBytes(payload, "previous_response_id").String())
	cause := &UpstreamFailoverError{StatusCode: 429}
	wrapped := newOpenAIWSCurrentTurnFailoverError(cause, retry)
	retry[0] = 'x'
	first, current := OpenAIWSCurrentTurnRetryPayload(wrapped)
	require.True(t, current)
	require.Equal(t, byte('{'), first[0])
	first[0] = 'x'
	second, _ := OpenAIWSCurrentTurnRetryPayload(wrapped)
	require.Equal(t, byte('{'), second[0])
	require.ErrorIs(t, wrapped, cause)
}

// TestOpenAIWSHTTPBridgeSemantic429 验证两类语义限流均在写出前返回原始 429。
func TestOpenAIWSHTTPBridgeSemantic429(t *testing.T) {
	for _, event := range []string{
		`{"type":"error","error":{"code":"rate_limit_exceeded"}}`,
		`{"type":"response.failed","response":{"error":{"type":"usage_limit_reached"}}}`,
	} {
		t.Run(gjson.Get(event, "type").String(), func(t *testing.T) {
			upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("data: " + event + "\n\n"))}}
			svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
			writes := 0
			_, err := svc.proxyOpenAIWSHTTPBridgeTurn(context.Background(), c, &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth}, "token", []byte(`{"model":"gpt-5","input":[]}`), 0, "gpt-5", "", "", "", 2, func([]byte) error { writes++; return nil })
			var failover *UpstreamFailoverError
			require.ErrorAs(t, err, &failover)
			require.Equal(t, 429, failover.StatusCode)
			require.Zero(t, writes)
		})
	}
}

// TestOpenAIWSReplayCollectorFullOutput 验证完整回放去重且工具专用回放保持原范围。
func TestOpenAIWSReplayCollectorFullOutput(t *testing.T) {
	collector := &openAIWSToolCallReplayCollector{}
	collector.AddEvent("response.output_item.done", []byte(`{"item":{"id":"m1","type":"message","role":"assistant","content":[]}}`))
	collector.AddEvent("response.completed", []byte(`{"response":{"output":[{"id":"m1","type":"message","role":"assistant","content":[]},{"id":"f1","type":"function_call","call_id":"c1"}]}}`))
	require.Len(t, collector.AllItems(), 2)
	require.Len(t, collector.Items(), 1)
	items := collector.AllItems()
	items[0][0] = 'x'
	require.Equal(t, byte('{'), collector.AllItems()[0][0])
}

func TestProxyOpenAIWSHTTPBridgeTurnLaterTurn429FailsOverBeforeClientWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     http.Header{"Retry-After": []string{"60"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"type":"usage_limit_reached","message":"The usage limit has been reached"}}`)),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{ID: 129, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 1}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
	payload := []byte(`{"type":"response.create","model":"gpt-5.6-sol","previous_response_id":"resp_old","input":[{"role":"user","content":"continue"}]}`)
	writes := 0

	result, err := svc.proxyOpenAIWSHTTPBridgeTurn(
		context.Background(), c, account, "access-token", payload, len(payload),
		"gpt-5.6-sol", "", "", "", 281,
		func([]byte) error {
			writes++
			return nil
		},
	)

	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusTooManyRequests, failoverErr.StatusCode)
	require.Zero(t, writes)
}

func TestProxyOpenAIWSHTTPBridgeTurnLaterTurnDoesNotFailOverAfterDownstreamOutput(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(
			"data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial\"}\n\n" +
				"data: {\"type\":\"error\",\"error\":{\"type\":\"rate_limit_error\",\"message\":\"limited\"}}\n\n",
		)),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{ID: 10, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
	payload := []byte(`{"type":"response.create","model":"gpt-5","input":"hi"}`)
	var writes [][]byte

	result, err := svc.proxyOpenAIWSHTTPBridgeTurn(
		context.Background(), c, account, "sk-test", payload, len(payload),
		"gpt-5", "", "", "", 281,
		func(message []byte) error {
			writes = append(writes, append([]byte(nil), message...))
			return nil
		},
	)

	require.NotNil(t, result)
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(err, &failoverErr))
	require.Len(t, writes, 2)
	require.Equal(t, "response.output_text.delta", gjson.GetBytes(writes[0], "type").String())
	require.Equal(t, "error", gjson.GetBytes(writes[1], "type").String())
}

func TestOpenAIWSHTTPBridgeLaterTurn429RetriesCurrentTurnOnReplacementAccount(t *testing.T) {
	for _, scenario := range []string{"http", "semantic_error", "semantic_failed", "unknown_previous", "incomplete_output"} {
		t.Run(scenario, func(t *testing.T) {
			testOpenAIWSHTTPBridgeLaterTurn429(t, scenario)
		})
	}
}

// testOpenAIWSHTTPBridgeLaterTurn429 使用真实客户端连接验证切号和不完整历史拒绝。
func testOpenAIWSHTTPBridgeLaterTurn429(t *testing.T, scenario string) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Gateway.OpenAIWS.HTTPBridgeEnabled = true
	cfg.Gateway.OpenAIWS.HTTPBridgeThresholdBytes = 1
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.ModeRouterV2Enabled = true
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3

	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type":          []string{"text/event-stream"},
				openAIWSTurnStateHeader: []string{"old-account-state"},
			},
			Body: io.NopCloser(strings.NewReader(
				"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_first\",\"output\":[{\"id\":\"msg_1\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"first-ok\"}]},{\"id\":\"fc_1\",\"type\":\"function_call\",\"call_id\":\"call_1\",\"name\":\"inspect\",\"arguments\":\"{}\"}],\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}}\n\n",
			)),
		},
		{
			StatusCode: http.StatusTooManyRequests,
			Header:     http.Header{"Retry-After": []string{"60"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":{"type":"usage_limit_reached","message":"The usage limit has been reached"}}`)),
		},
		{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body: io.NopCloser(strings.NewReader(
				"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_second\",\"output\":[{\"id\":\"msg_2\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"second-ok\"}]}],\"usage\":{\"input_tokens\":4,\"output_tokens\":1}}}\n\n",
			)),
		},
	}}
	unsafeContext := scenario == "unknown_previous" || scenario == "incomplete_output"
	if scenario == "incomplete_output" {
		upstream.responses[0].Body = io.NopCloser(strings.NewReader("data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_first\"}}\n\n"))
	}
	if scenario == "semantic_error" || scenario == "semantic_failed" {
		upstream.responses[1].StatusCode = http.StatusOK
		event := `{"type":"error","error":{"code":"rate_limit_exceeded"}}`
		if scenario == "semantic_failed" {
			event = `{"type":"response.failed","response":{"error":{"code":"rate_limit_exceeded"}}}`
		}
		upstream.responses[1].Body = io.NopCloser(strings.NewReader("data: " + event + "\n\n"))
	}
	svc := &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     upstream,
		cache:            &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
	}
	account := &Account{
		ID: 129, Name: "limited", Platform: PlatformOpenAI, Type: AccountTypeOAuth,
		Status: StatusActive, Schedulable: true, Concurrency: 1,
		Credentials: map[string]any{"model_mapping": map[string]any{"gpt-5.6-sol": "mapped-a"}},
		Extra:       map[string]any{"openai_oauth_responses_websockets_v2_mode": OpenAIWSIngressModeCtxPool},
	}
	nextAccount := *account
	nextAccount.ID = 130
	nextAccount.Name = "replacement"
	nextAccount.Credentials = map[string]any{"model_mapping": map[string]any{"gpt-5.6-sol": "mapped-b"}}

	serverErrCh := make(chan error, 1)
	failoverCh := make(chan []byte, 1)
	wsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			serverErrCh <- err
			return
		}
		defer func() { _ = conn.CloseNow() }()

		rec := httptest.NewRecorder()
		ginCtx, _ := gin.CreateTestContext(rec)
		ginCtx.Request = r.Clone(r.Context())
		readCtx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		_, firstMessage, readErr := conn.Read(readCtx)
		cancel()
		if readErr != nil {
			serverErrCh <- readErr
			return
		}
		proxyErr := svc.ProxyResponsesWebSocketFromClient(r.Context(), ginCtx, conn, account, "access-token-a", firstMessage, nil)
		var failoverErr *UpstreamFailoverError
		if !errors.As(proxyErr, &failoverErr) {
			serverErrCh <- proxyErr
			return
		}
		retryPayload, retryCurrentTurn := OpenAIWSCurrentTurnRetryPayload(proxyErr)
		if unsafeContext {
			if !retryCurrentTurn || len(retryPayload) != 0 {
				serverErrCh <- errors.New("incomplete history unexpectedly replayable")
				return
			}
			serverErrCh <- nil
			return
		}
		if !retryCurrentTurn || len(retryPayload) == 0 {
			serverErrCh <- errors.New("missing current-turn retry payload")
			return
		}
		failoverCh <- retryPayload
		serverErrCh <- svc.ProxyResponsesWebSocketFromClient(
			r.Context(), ginCtx, conn, &nextAccount, "access-token-b", retryPayload, nil,
		)
	}))
	defer wsServer.Close()

	dialCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	clientConn, _, err := websocket.Dial(dialCtx, "ws"+strings.TrimPrefix(wsServer.URL, "http"), nil)
	cancel()
	require.NoError(t, err)
	defer func() { _ = clientConn.CloseNow() }()

	writeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	err = clientConn.Write(writeCtx, websocket.MessageText, []byte(`{"type":"response.create","model":"gpt-5.6-sol","input":[{"role":"user","content":"first"}]}`))
	cancel()
	require.NoError(t, err)

	readCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	_, completed, err := clientConn.Read(readCtx)
	cancel()
	require.NoError(t, err)
	require.Equal(t, "response.completed", gjson.GetBytes(completed, "type").String())

	writeCtx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
	secondMessage := `{"type":"response.create","model":"gpt-5.6-sol","previous_response_id":"resp_first","input":[{"type":"function_call_output","call_id":"call_1","output":"second"}]}`
	if scenario == "unknown_previous" {
		secondMessage = strings.Replace(secondMessage, "resp_first", "resp_unknown", 1)
	}
	err = clientConn.Write(writeCtx, websocket.MessageText, []byte(secondMessage))
	cancel()
	require.NoError(t, err)

	readCtx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
	_, retriedCompleted, err := clientConn.Read(readCtx)
	cancel()
	if unsafeContext {
		require.Error(t, err)
		select {
		case serverErr := <-serverErrCh:
			require.NoError(t, serverErr)
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for context rejection")
		}
		require.Len(t, upstream.bodies, 2)
		return
	}
	require.NoError(t, err)
	require.Equal(t, "response.completed", gjson.GetBytes(retriedCompleted, "type").String())
	require.Equal(t, "resp_second", gjson.GetBytes(retriedCompleted, "response.id").String())
	_ = clientConn.Close(websocket.StatusNormalClosure, "done")

	select {
	case retryPayload := <-failoverCh:
		require.NotEmpty(t, retryPayload)
		require.False(t, gjson.GetBytes(retryPayload, "previous_response_id").Exists())
		require.Equal(t, "gpt-5.6-sol", gjson.GetBytes(retryPayload, "model").String())
		input := gjson.GetBytes(retryPayload, "input")
		require.True(t, input.IsArray())
		require.Len(t, input.Array(), 4)
		require.Contains(t, input.Raw, "first")
		require.Contains(t, input.Raw, "first-ok")
		require.Contains(t, input.Raw, "second")
		require.Equal(t, 1, strings.Count(input.Raw, `"id":"fc_1"`))
		require.Equal(t, 2, strings.Count(input.Raw, `"call_id":"call_1"`))
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for current-turn failover")
	}

	select {
	case proxyErr := <-serverErrCh:
		require.NoError(t, proxyErr)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for replacement-account completion")
	}
	require.Len(t, upstream.bodies, 3)
	require.Contains(t, string(upstream.bodies[0]), "first")
	require.NotContains(t, string(upstream.bodies[2]), "previous_response_id")
	require.Contains(t, string(upstream.bodies[2]), "second")
	require.Equal(t, "mapped-a", gjson.GetBytes(upstream.bodies[1], "model").String())
	require.Equal(t, "mapped-b", gjson.GetBytes(upstream.bodies[2], "model").String())
	require.Empty(t, upstream.requests[2].Header.Get(openAIWSTurnStateHeader))
}
