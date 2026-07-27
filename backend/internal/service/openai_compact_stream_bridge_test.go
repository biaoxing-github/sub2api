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
	"github.com/tidwall/gjson"
)

func newCompactBridgeTestContext(t *testing.T, markClientStream bool) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", nil)
	if markClientStream {
		MarkOpenAICompactClientStream(c)
	}
	return c, rec
}

func newCompactBridgeTestService() *OpenAIGatewayService {
	cfg := &config.Config{}
	return &OpenAIGatewayService{
		cfg:           cfg,
		toolCorrector: NewCodexToolCorrector(),
	}
}

// parseCompactBridgeSSE 把合成的 SSE 文本拆成 (eventType, dataJSON) 序列。
func parseCompactBridgeSSE(t *testing.T, body string) [][2]string {
	t.Helper()
	var events [][2]string
	for _, block := range strings.Split(strings.TrimSpace(body), "\n\n") {
		lines := strings.Split(block, "\n")
		require.Len(t, lines, 2, "每个 SSE 事件应为 event+data 两行: %q", block)
		require.True(t, strings.HasPrefix(lines[0], "event: "), "缺少 event 行: %q", block)
		require.True(t, strings.HasPrefix(lines[1], "data: "), "缺少 data 行: %q", block)
		events = append(events, [2]string{
			strings.TrimPrefix(lines[0], "event: "),
			strings.TrimPrefix(lines[1], "data: "),
		})
	}
	return events
}

func TestBuildOpenAICompactSSEPayload_EmitsItemsAndCompleted(t *testing.T) {
	finalResponse := []byte(`{
		"id":"resp_compact_1",
		"object":"response",
		"model":"gpt-5.1-codex",
		"status":"completed",
		"output":[
			{"id":"cmp_1","type":"compaction","status":"completed","encrypted_content":"compact-payload","summary":[{"type":"summary_text","text":"compact summary"}],"opaque":{"kept":true}},
			{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"output_text","text":"done"}]}
		],
		"usage":{"input_tokens":9,"output_tokens":4,"total_tokens":13}
	}`)

	payload, ok := buildOpenAICompactSSEPayload(finalResponse)
	require.True(t, ok)

	events := parseCompactBridgeSSE(t, string(payload))
	require.Len(t, events, 3)

	require.Equal(t, "response.output_item.done", events[0][0])
	first := events[0][1]
	require.Equal(t, "response.output_item.done", gjson.Get(first, "type").String())
	require.Equal(t, int64(0), gjson.Get(first, "output_index").Int())
	require.Equal(t, "compaction", gjson.Get(first, "item.type").String())
	require.Equal(t, "cmp_1", gjson.Get(first, "item.id").String())
	require.Equal(t, "compact-payload", gjson.Get(first, "item.encrypted_content").String())
	require.Equal(t, "compact summary", gjson.Get(first, "item.summary.0.text").String())
	require.True(t, gjson.Get(first, "item.opaque.kept").Bool(), "item 原始字段必须逐字节保留")

	require.Equal(t, "response.output_item.done", events[1][0])
	require.Equal(t, int64(1), gjson.Get(events[1][1], "output_index").Int())
	require.Equal(t, "message", gjson.Get(events[1][1], "item.type").String())

	require.Equal(t, "response.completed", events[2][0])
	completed := events[2][1]
	require.Equal(t, "response.completed", gjson.Get(completed, "type").String())
	require.Equal(t, "resp_compact_1", gjson.Get(completed, "response.id").String())
	require.Equal(t, int64(13), gjson.Get(completed, "response.usage.total_tokens").Int())
	require.Len(t, gjson.Get(completed, "response.output").Array(), 2)
}

func TestBuildOpenAICompactSSEPayload_InjectsMissingResponseID(t *testing.T) {
	payload, ok := buildOpenAICompactSSEPayload([]byte(`{"output":[{"type":"compaction","encrypted_content":"x"}]}`))
	require.True(t, ok)

	events := parseCompactBridgeSSE(t, string(payload))
	require.Len(t, events, 2)
	completed := events[1][1]
	// Codex 的 ResponseCompleted 解析要求 response.id 为非空 string，缺失时必须注入。
	id := gjson.Get(completed, "response.id").String()
	require.True(t, strings.HasPrefix(id, "resp_"), "缺失 id 必须注入 resp_* 兜底: %q", id)
	require.NotEqual(t, "resp_", id)
}

func TestBuildOpenAICompactSSEPayload_DropsMalformedUsage(t *testing.T) {
	payload, ok := buildOpenAICompactSSEPayload([]byte(`{
		"id":"resp_1",
		"output":[{"type":"compaction","encrypted_content":"x"}],
		"usage":{"prompt_tokens":9,"completion_tokens":4}
	}`))
	require.True(t, ok)

	events := parseCompactBridgeSSE(t, string(payload))
	completed := events[len(events)-1][1]
	// usage 缺少 Codex 必需的整数字段时必须整体删除，否则 completed 事件解析失败。
	require.False(t, gjson.Get(completed, "response.usage").Exists())
}

func TestBuildOpenAICompactSSEPayload_KeepsWellFormedUsage(t *testing.T) {
	payload, ok := buildOpenAICompactSSEPayload([]byte(`{
		"id":"resp_1",
		"output":[{"type":"compaction","encrypted_content":"x"}],
		"usage":{"input_tokens":9,"output_tokens":4,"total_tokens":13,"input_tokens_details":{"cached_tokens":2}}
	}`))
	require.True(t, ok)

	events := parseCompactBridgeSSE(t, string(payload))
	completed := events[len(events)-1][1]
	require.Equal(t, int64(9), gjson.Get(completed, "response.usage.input_tokens").Int())
	require.Equal(t, int64(2), gjson.Get(completed, "response.usage.input_tokens_details.cached_tokens").Int())
}

func TestBuildOpenAICompactSSEPayload_RejectsNonJSONObject(t *testing.T) {
	for name, body := range map[string][]byte{
		"empty":     nil,
		"sse_text":  []byte("data: {\"type\":\"response.completed\"}\n\n"),
		"array":     []byte(`[{"id":"resp_1"}]`),
		"non_json":  []byte("upstream said no"),
		"bare_true": []byte("true"),
	} {
		_, ok := buildOpenAICompactSSEPayload(body)
		require.False(t, ok, "case %s 不应被合成为 SSE", name)
	}
}

func TestWriteOpenAICompactSSEBridge_RequiresMarkAndSuccessStatus(t *testing.T) {
	finalResponse := []byte(`{"id":"resp_1","output":[{"type":"compaction","encrypted_content":"x"}]}`)

	// 未标记 client stream：不写出，走原 JSON 路径。
	c, rec := newCompactBridgeTestContext(t, false)
	require.False(t, writeOpenAICompactSSEBridge(c, http.StatusOK, finalResponse))
	require.Zero(t, rec.Body.Len())

	// 标记但上游非 2xx：错误响应保持 JSON 原样（Codex 依赖 HTTP 状态码走重试）。
	c, rec = newCompactBridgeTestContext(t, true)
	require.False(t, writeOpenAICompactSSEBridge(c, http.StatusBadGateway, finalResponse))
	require.Zero(t, rec.Body.Len())

	// 标记且 2xx：合成 SSE。
	c, rec = newCompactBridgeTestContext(t, true)
	require.True(t, writeOpenAICompactSSEBridge(c, http.StatusOK, finalResponse))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
	require.Contains(t, rec.Body.String(), "event: response.completed")
}

// 回归 #3875：body-signal 提升后的 compact 请求，上游返回 unary JSON，
// 客户端（Codex remote compact v2）必须收到 SSE 事件流而非 JSON 文档，
// 否则报 "stream closed before response.completed" 并无限重连。
func TestHandleNonStreamingResponse_CompactClientStreamBridgesToSSE(t *testing.T) {
	svc := newCompactBridgeTestService()
	c, rec := newCompactBridgeTestContext(t, true)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(`{
			"id":"resp_compact_json",
			"object":"response",
			"model":"gpt-5.1-codex",
			"status":"completed",
			"output":[{"id":"cmp_1","type":"compaction","status":"completed","encrypted_content":"compact-payload"}],
			"usage":{"input_tokens":9,"output_tokens":4,"total_tokens":13}
		}`)),
	}

	result, err := svc.handleNonStreamingResponse(context.Background(), resp, c, &Account{ID: 1, Type: AccountTypeOAuth}, "gpt-5.5", "gpt-5.5")
	require.NoError(t, err)
	require.NotNil(t, result)

	require.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
	events := parseCompactBridgeSSE(t, rec.Body.String())
	require.Len(t, events, 2)
	require.Equal(t, "response.output_item.done", events[0][0])
	require.Equal(t, "compaction", gjson.Get(events[0][1], "item.type").String())
	require.Equal(t, "response.completed", events[1][0])
	require.Equal(t, "resp_compact_json", gjson.Get(events[1][1], "response.id").String())

	// 计费与响应元数据不受写回形态影响。
	require.NotNil(t, result.usage)
	require.Equal(t, 9, result.usage.InputTokens)
	require.Equal(t, 4, result.usage.OutputTokens)
	require.Equal(t, "resp_compact_json", result.responseID)
}

// 回归防护：path-based compact（Codex v1 unary 协议、链式 sub2api）未标记
// client stream，必须保持 v0.1.146 以来的 JSON 写回行为。
func TestHandleNonStreamingResponse_PathBasedCompactStaysJSON(t *testing.T) {
	svc := newCompactBridgeTestService()
	c, rec := newCompactBridgeTestContext(t, false)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(`{
			"id":"resp_compact_json",
			"output":[{"id":"cmp_1","type":"compaction","encrypted_content":"compact-payload"}],
			"usage":{"input_tokens":9,"output_tokens":4,"total_tokens":13}
		}`)),
	}

	result, err := svc.handleNonStreamingResponse(context.Background(), resp, c, &Account{ID: 1, Type: AccountTypeOAuth}, "gpt-5.5", "gpt-5.5")
	require.NoError(t, err)
	require.NotNil(t, result)

	require.NotContains(t, rec.Header().Get("Content-Type"), "text/event-stream")
	body := rec.Body.String()
	require.Equal(t, "resp_compact_json", gjson.Get(body, "id").String())
	require.Equal(t, "compaction", gjson.Get(body, "output.0.type").String())
}

// 上游对 compact 返回 SSE（如链式网关）时，最终响应经 SSE→JSON 提取后，
// 对 client-stream 请求同样必须再合成回 SSE。
func TestHandleSSEToJSON_CompactClientStreamBridgesToSSE(t *testing.T) {
	svc := newCompactBridgeTestService()
	c, rec := newCompactBridgeTestContext(t, true)
	upstreamSSE := strings.Join([]string{
		`data: {"type":"response.completed","response":{"id":"resp_compact_sse","object":"response","model":"gpt-5.1-codex","status":"completed","output":[{"id":"cmp_sse_1","type":"compaction","status":"completed","encrypted_content":"compact-sse-payload"}],"usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}}`,
		"",
	}, "\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(upstreamSSE)),
	}

	result, err := svc.handleNonStreamingResponse(context.Background(), resp, c, &Account{ID: 1, Type: AccountTypeOAuth}, "gpt-5.5", "gpt-5.5")
	require.NoError(t, err)
	require.NotNil(t, result)

	require.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
	events := parseCompactBridgeSSE(t, rec.Body.String())
	require.Len(t, events, 2)
	require.Equal(t, "response.output_item.done", events[0][0])
	require.Equal(t, "compact-sse-payload", gjson.Get(events[0][1], "item.encrypted_content").String())
	require.Equal(t, "response.completed", events[1][0])
	require.Equal(t, "resp_compact_sse", gjson.Get(events[1][1], "response.id").String())
}

// 混合 SSE 的终态 response.output 可能已有普通消息、但遗漏 raw done 中的
// compaction。桥接后的 item 事件和 response.completed 都必须各保留且只保留一次。
func TestHandleSSEToJSON_CompactClientStreamPreservesOneRawCompactionAlongsideMessage(t *testing.T) {
	svc := newCompactBridgeTestService()
	c, rec := newCompactBridgeTestContext(t, true)
	messageItem := `{"id":"msg_mixed_1","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"hello"}]}`
	compactionItem := `{"id":"cmp_mixed_1","type":"compaction","status":"completed","encrypted_content":"compact-mixed-payload"}`
	upstreamSSE := strings.Join([]string{
		`data: {"type":"response.output_item.done","output_index":0,"item":` + messageItem + `}`,
		"",
		`data: {"type":"response.output_item.done","output_index":1,"item":` + compactionItem + `}`,
		"",
		`data: {"type":"response.completed","response":{"id":"resp_compact_mixed","object":"response","model":"gpt-5.1-codex","status":"completed","output":[` + messageItem + `],"usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}}`,
		"",
	}, "\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(upstreamSSE)),
	}

	result, err := svc.handleNonStreamingResponse(context.Background(), resp, c, &Account{ID: 1, Type: AccountTypeOAuth}, "gpt-5.5", "gpt-5.5")
	require.NoError(t, err)
	require.NotNil(t, result)

	events := parseCompactBridgeSSE(t, rec.Body.String())
	require.Len(t, events, 3)
	compactionEvents := 0
	for _, event := range events {
		if event[0] == "response.output_item.done" && gjson.Get(event[1], "item.type").String() == "compaction" {
			compactionEvents++
			require.Equal(t, "cmp_mixed_1", gjson.Get(event[1], "item.id").String())
		}
	}
	require.Equal(t, 1, compactionEvents)

	completedOutput := gjson.Get(events[2][1], "response.output").Array()
	require.Len(t, completedOutput, 2)
	compactionOutput := 0
	for _, item := range completedOutput {
		if item.Get("type").String() == "compaction" {
			compactionOutput++
			require.Equal(t, "cmp_mixed_1", item.Get("id").String())
		}
	}
	require.Equal(t, 1, compactionOutput)
}

// 上游的 compaction 可能只存在于 raw output_item.done，而 completed.response
// 的 output 为空。桥接前必须恢复该完整 item，否则 Codex 会因缺少 compaction
// 事件而重复请求并重复消耗额度。
func TestReconstructResponseOutputFromSSE_PreservesRawCompactionDoneItem(t *testing.T) {
	bodyText := strings.Join([]string{
		`data: {"type":"response.output_item.done","output_index":0,"item":{"id":"cmp_1","type":"compaction_summary","status":"completed","summary":[{"type":"summary_text","text":"compact summary"}],"encrypted_content":"compact-payload","opaque":{"kept":true}}}`,
		``,
		`data: {"type":"response.completed","response":{"id":"resp_compact","output":[]}}`,
		``,
	}, "\n")

	outputJSON, ok := reconstructResponseOutputFromSSE(bodyText)
	require.True(t, ok)
	items := gjson.ParseBytes(outputJSON).Array()
	require.Len(t, items, 1)
	require.Equal(t, "cmp_1", items[0].Get("id").String())
	require.Equal(t, "compaction_summary", items[0].Get("type").String())
	require.Equal(t, "compact-payload", items[0].Get("encrypted_content").String())
	require.True(t, items[0].Get("opaque.kept").Bool())
}

// 终态 output 已含普通消息时，compact item 仍可能只在 SSE 事件中出现；补全
// 必须附加该原始 item，防止桥接后的 Codex 视为缺少 compaction 并再次请求。
func TestSupplementCompactionItemFromSSE_AppendsMissingCompaction(t *testing.T) {
	c, _ := newCompactBridgeTestContext(t, false)
	finalResponse := []byte(`{"id":"resp_1","output":[{"id":"msg_1","type":"message"}]}`)
	bodyText := `data: {"type":"response.output_item.done","item":{"id":"cmp_1","type":"compaction","encrypted_content":"preserved"}}` + "\n"

	patched := supplementCompactionItemFromSSE(c, finalResponse, bodyText)
	items := gjson.GetBytes(patched, "output").Array()
	require.Len(t, items, 2)
	require.Equal(t, "message", items[0].Get("type").String())
	require.Equal(t, "compaction", items[1].Get("type").String())
	require.Equal(t, "preserved", items[1].Get("encrypted_content").String())
}

// 混合流中普通 item 已完成、compaction 只在 added 时仍需收集；而 done 已给出
// compaction 时，added 只能作为早期事件，绝不能造成两个 compact item。
func TestReconstructResponseOutputFromSSE_MergesAddedCompactionOnlyWhenMissingFromDone(t *testing.T) {
	missingFromDone := strings.Join([]string{
		`data: {"type":"response.output_item.added","item":{"id":"cmp_added","type":"compaction","encrypted_content":"added"}}`,
		`data: {"type":"response.output_item.done","item":{"id":"msg_done","type":"message","content":[{"type":"output_text","text":"hello"}]}}`,
	}, "\n")
	outputJSON, ok := reconstructResponseOutputFromSSE(missingFromDone)
	require.True(t, ok)
	items := gjson.ParseBytes(outputJSON).Array()
	require.Len(t, items, 2)
	require.Equal(t, "msg_done", items[0].Get("id").String())
	require.Equal(t, "cmp_added", items[1].Get("id").String())

	doneHasCompaction := strings.Join([]string{
		`data: {"type":"response.output_item.added","item":{"id":"cmp_duplicate","type":"compaction","status":"in_progress"}}`,
		`data: {"type":"response.output_item.done","item":{"id":"cmp_duplicate","type":"compaction","status":"completed","encrypted_content":"final"}}`,
	}, "\n")
	outputJSON, ok = reconstructResponseOutputFromSSE(doneHasCompaction)
	require.True(t, ok)
	items = gjson.ParseBytes(outputJSON).Array()
	require.Len(t, items, 1)
	require.Equal(t, "final", items[0].Get("encrypted_content").String())
}

// 透传分支（OAuth passthrough）同样命中桥接。
func TestHandleNonStreamingResponsePassthrough_CompactClientStreamBridgesToSSE(t *testing.T) {
	svc := newCompactBridgeTestService()
	c, rec := newCompactBridgeTestContext(t, true)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(`{
			"id":"resp_compact_pt",
			"output":[{"id":"cmp_pt_1","type":"compaction","encrypted_content":"compact-pt-payload"}],
			"usage":{"input_tokens":7,"output_tokens":3,"total_tokens":10}
		}`)),
	}

	result, err := svc.handleNonStreamingResponsePassthrough(context.Background(), resp, c, PlatformOpenAI, "gpt-5.5", "")
	require.NoError(t, err)
	require.NotNil(t, result)

	require.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
	events := parseCompactBridgeSSE(t, rec.Body.String())
	require.Len(t, events, 2)
	require.Equal(t, "compaction", gjson.Get(events[0][1], "item.type").String())
	require.Equal(t, "resp_compact_pt", gjson.Get(events[1][1], "response.id").String())
	require.NotNil(t, result.usage)
	require.Equal(t, 7, result.usage.InputTokens)
}

func TestShouldFailoverOpenAICompactContextWindowResponse(t *testing.T) {
	c, _ := newCompactBridgeTestContext(t, true)
	body := []byte(`{
		"error": {
			"code": "context_length_exceeded",
			"message": "Your input exceeds the context window of this model."
		}
	}`)

	require.True(t, shouldFailoverOpenAICompactContextWindowResponse(c, http.StatusBadRequest, "", body))
	c.Request.URL.Path = "/v1/responses"
	require.False(t, shouldFailoverOpenAICompactContextWindowResponse(c, http.StatusBadRequest, "", body))
	require.False(t, shouldFailoverOpenAICompactContextWindowResponse(c, http.StatusBadGateway, "", body))
}

func TestHandleErrorResponse_CompactContextWindowRequestsFailoverAfterKeepalive(t *testing.T) {
	svc := newCompactBridgeTestService()
	c, rec := newCompactBridgeTestContext(t, true)
	stop := StartOpenAICompactSSEKeepalive(c, keepaliveTestInterval)
	defer stop()
	waitForKeepaliveBeats()

	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(`{
			"error": {
				"code": "context_length_exceeded",
				"message": "Your input exceeds the context window of this model."
			}
		}`)),
	}
	account := &Account{ID: 1, Name: "compact-native", Platform: PlatformOpenAI, Type: AccountTypeOAuth}

	_, err := svc.handleErrorResponse(context.Background(), resp, c, account, []byte(`{"model":"gpt-5.5"}`), "gpt-5.5")

	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.True(t, errors.As(err, &failoverErr))
	require.Equal(t, http.StatusBadRequest, failoverErr.StatusCode)
	require.False(t, failoverErr.RetryableOnSameAccount)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, strings.TrimSpace(stripKeepaliveComments(rec.Body.String())))
}

func TestHandleFailoverErrorResponsePassthrough_CompactContextWindowRequestsFailoverAfterKeepalive(t *testing.T) {
	svc := newCompactBridgeTestService()
	c, rec := newCompactBridgeTestContext(t, true)
	stop := StartOpenAICompactSSEKeepalive(c, keepaliveTestInterval)
	defer stop()
	waitForKeepaliveBeats()

	upstreamBody := []byte(`{
		"error": {
			"code": "context_length_exceeded",
			"message": "Your input exceeds the context window of this model."
		}
	}`)
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(string(upstreamBody))),
	}
	account := &Account{ID: 1, Name: "compact-passthrough", Platform: PlatformOpenAI, Type: AccountTypeOAuth}

	require.True(t, shouldFailoverOpenAICompactContextWindowResponse(c, resp.StatusCode, "", upstreamBody))
	err := svc.handleFailoverErrorResponsePassthrough(context.Background(), resp, c, account, []byte(`{"model":"gpt-5.5"}`))

	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.True(t, errors.As(err, &failoverErr))
	require.Equal(t, http.StatusBadRequest, failoverErr.StatusCode)
	require.False(t, failoverErr.RetryableOnSameAccount)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, strings.TrimSpace(stripKeepaliveComments(rec.Body.String())))
}
