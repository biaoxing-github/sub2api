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

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestShouldNormalizeDeepSeekResponsesReasoning(t *testing.T) {
	tests := []struct {
		name          string
		account       *Account
		originalModel string
		mappedModel   string
		want          bool
	}{
		{
			name:          "requested DeepSeek model",
			originalModel: "deepseek-reasoner",
			want:          true,
		},
		{
			name:          "mapped DeepSeek model",
			originalModel: "reasoning-alias",
			mappedModel:   "deepseek-v4-pro",
			want:          true,
		},
		{
			name: "official DeepSeek endpoint",
			account: &Account{
				Platform: PlatformOpenAI,
				Type:     AccountTypeAPIKey,
				Credentials: map[string]any{
					"base_url": "https://api.deepseek.com/v1",
				},
			},
			originalModel: "reasoning-alias",
			want:          true,
		},
		{
			name: "OpenAI native response",
			account: &Account{
				Platform: PlatformOpenAI,
				Type:     AccountTypeAPIKey,
				Credentials: map[string]any{
					"base_url": "https://api.openai.com/v1",
				},
			},
			originalModel: "gpt-5.6",
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, shouldNormalizeDeepSeekResponsesReasoning(tt.account, tt.originalModel, tt.mappedModel))
		})
	}
}

func TestNormalizeDeepSeekResponsesReasoningSSEBody_LegacyReasoningBecomesSummary(t *testing.T) {
	body := []byte(strings.Join([]string{
		`event: response.output_item.added`,
		`data: {"type":"response.output_item.added","output_index":0,"item":{"id":"rs_1","type":"reasoning","status":"in_progress","summary":[],"content":[]},"sequence_number":1}`,
		``,
		`event: response.reasoning_text.delta`,
		`data: {"type":"response.reasoning_text.delta","item_id":"rs_1","output_index":0,"content_index":0,"delta":"plan ","sequence_number":2}`,
		``,
		`event: response.reasoning_text.delta`,
		`data: {"type":"response.reasoning_text.delta","item_id":"rs_1","output_index":0,"content_index":0,"delta":"carefully","sequence_number":3}`,
		``,
		`event: response.reasoning_text.done`,
		`data: {"type":"response.reasoning_text.done","item_id":"rs_1","output_index":0,"content_index":0,"text":"plan carefully","sequence_number":4}`,
		``,
		`event: response.output_item.done`,
		`data: {"type":"response.output_item.done","output_index":0,"item":{"id":"rs_1","type":"reasoning","status":"completed","summary":[],"content":[{"type":"reasoning_text","text":"plan carefully"}]},"sequence_number":5}`,
		``,
		`event: response.completed`,
		`data: {"type":"response.completed","response":{"id":"resp_1","object":"response","status":"completed","output":[{"id":"rs_1","type":"reasoning","status":"completed","summary":[],"content":[{"type":"reasoning_text","text":"plan carefully"}]},{"id":"call_1","type":"function_call","name":"lookup","arguments":"{}","call_id":"call_1","status":"completed"}],"usage":{"input_tokens":3,"output_tokens":4,"total_tokens":7}},"sequence_number":6}`,
		``,
	}, "\n"))

	normalized, changed := normalizeDeepSeekResponsesReasoningSSEBody(body, true)
	require.True(t, changed)

	var payloads []string
	forEachOpenAISSEDataPayload(string(normalized), func(data []byte) {
		payloads = append(payloads, string(data))
	})
	require.Len(t, payloads, 8)
	require.Equal(t, []string{
		"response.output_item.added",
		"response.reasoning_summary_part.added",
		"response.reasoning_summary_text.delta",
		"response.reasoning_summary_text.delta",
		"response.reasoning_summary_text.done",
		"response.reasoning_summary_part.done",
		"response.output_item.done",
		"response.completed",
	}, responseEventTypes(payloads))

	for i, payload := range payloads {
		require.Equal(t, int64(i+1), gjson.Get(payload, "sequence_number").Int())
	}
	require.Equal(t, "summary_text", gjson.Get(payloads[1], "part.type").String())
	require.Equal(t, "", gjson.Get(payloads[1], "part.text").String())
	require.False(t, gjson.Get(payloads[2], "content_index").Exists())
	require.Equal(t, int64(0), gjson.Get(payloads[2], "summary_index").Int())
	require.Equal(t, "plan carefully", gjson.Get(payloads[5], "part.text").String())
	require.Equal(t, "plan carefully", gjson.Get(payloads[6], "item.summary.0.text").String())
	require.False(t, gjson.Get(payloads[6], "item.content").Exists())
	require.Equal(t, "plan carefully", gjson.Get(payloads[7], "response.output.0.summary.0.text").String())
	require.False(t, gjson.Get(payloads[7], "response.output.0.content").Exists())
	require.Equal(t, "function_call", gjson.Get(payloads[7], "response.output.1.type").String())
	require.Equal(t, "lookup", gjson.Get(payloads[7], "response.output.1.name").String())
	require.NotContains(t, string(normalized), "response.reasoning_text")
}

func TestNormalizeDeepSeekResponsesReasoningSSEBody_NativeSummaryStaysUntouched(t *testing.T) {
	body := []byte(strings.Join([]string{
		`event: response.reasoning_summary_part.added`,
		`data: {"type":"response.reasoning_summary_part.added","item_id":"rs_1","output_index":0,"summary_index":0,"part":{"type":"summary_text","text":""},"sequence_number":1}`,
		``,
		`event: response.reasoning_summary_text.delta`,
		`data: {"type":"response.reasoning_summary_text.delta","item_id":"rs_1","output_index":0,"summary_index":0,"delta":"native summary","sequence_number":2}`,
		``,
		`event: response.reasoning_summary_part.done`,
		`data: {"type":"response.reasoning_summary_part.done","item_id":"rs_1","output_index":0,"summary_index":0,"part":{"type":"summary_text","text":"native summary"},"sequence_number":3}`,
		``,
	}, "\n"))

	normalized, changed := normalizeDeepSeekResponsesReasoningSSEBody(body, true)
	require.False(t, changed)
	require.Equal(t, body, normalized)
}

func TestNormalizeDeepSeekResponsesReasoningSSEBody_DisabledStaysUntouched(t *testing.T) {
	body := []byte("data: {\"type\":\"response.reasoning_text.delta\",\"delta\":\"raw\"}\n\n")

	normalized, changed := normalizeDeepSeekResponsesReasoningSSEBody(body, false)
	require.False(t, changed)
	require.Equal(t, body, normalized)
}

func TestNormalizeDeepSeekResponsesReasoningResponse_MovesContentAndPreservesTools(t *testing.T) {
	body := []byte(`{"id":"resp_1","object":"response","output":[{"id":"rs_1","type":"reasoning","status":"completed","summary":[],"content":[{"type":"reasoning_text","text":"reasoned answer"}]},{"id":"call_1","type":"function_call","name":"lookup","arguments":"{}","call_id":"call_1","status":"completed"}],"usage":{"input_tokens":3,"output_tokens":4,"total_tokens":7}}`)

	normalized, changed := normalizeDeepSeekResponsesReasoningResponse(body, true)
	require.True(t, changed)
	require.Equal(t, "reasoned answer", gjson.GetBytes(normalized, "output.0.summary.0.text").String())
	require.Equal(t, "summary_text", gjson.GetBytes(normalized, "output.0.summary.0.type").String())
	require.False(t, gjson.GetBytes(normalized, "output.0.content").Exists())
	require.Equal(t, "function_call", gjson.GetBytes(normalized, "output.1.type").String())
	require.Equal(t, int64(7), gjson.GetBytes(normalized, "usage.total_tokens").Int())
}

func TestNormalizeDeepSeekResponsesReasoningResponse_NativeSummaryStaysUntouched(t *testing.T) {
	body := []byte(`{"id":"resp_1","object":"response","output":[{"id":"rs_1","type":"reasoning","summary":[{"type":"summary_text","text":"native"}],"content":[{"type":"reasoning_text","text":"private"}]}]}`)

	normalized, changed := normalizeDeepSeekResponsesReasoningResponse(body, true)
	require.False(t, changed)
	require.Equal(t, body, normalized)
}

func TestHandleStreamingResponsePassthrough_DeepSeekReasoningNormalized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(deepSeekLegacyReasoningSSE())),
	}
	account := &Account{
		ID:       1,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url": "https://api.deepseek.com/v1",
		},
	}

	result, err := svc.handleStreamingResponsePassthrough(context.Background(), resp, c, account, time.Now(), "deepseek-reasoner", "deepseek-reasoner")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.usageObserved)
	require.Contains(t, rec.Body.String(), "response.reasoning_summary_part.added")
	require.Contains(t, rec.Body.String(), "response.reasoning_summary_part.done")
	require.NotContains(t, rec.Body.String(), "response.reasoning_text")
	finalResponse, ok := extractCodexFinalResponse(rec.Body.String())
	require.True(t, ok)
	require.Equal(t, "plan carefully", gjson.GetBytes(finalResponse, "output.0.summary.0.text").String())
	require.Equal(t, "function_call", gjson.GetBytes(finalResponse, "output.1.type").String())
}

func TestHandleNonStreamingResponsePassthrough_DeepSeekReasoningNormalized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"id":"resp_1","object":"response","output":[{"id":"rs_1","type":"reasoning","status":"completed","summary":[],"content":[{"type":"reasoning_text","text":"reasoned answer"}]},{"id":"call_1","type":"function_call","name":"lookup","arguments":"{}","call_id":"call_1","status":"completed"}],"usage":{"input_tokens":3,"output_tokens":4,"total_tokens":7}}`,
		)),
	}

	result, err := svc.handleNonStreamingResponsePassthrough(context.Background(), resp, c, PlatformOpenAI, "deepseek-reasoner", "deepseek-reasoner", true)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 3, result.usage.InputTokens)
	require.Equal(t, 4, result.usage.OutputTokens)
	require.Equal(t, "reasoned answer", gjson.Get(rec.Body.String(), "output.0.summary.0.text").String())
	require.False(t, gjson.Get(rec.Body.String(), "output.0.content").Exists())
	require.Equal(t, "function_call", gjson.Get(rec.Body.String(), "output.1.type").String())
}

func TestHandleNonStreamingResponsePassthrough_DeepSeekSSEReasoningNormalized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(deepSeekLegacyReasoningSSE())),
	}

	result, err := svc.handleNonStreamingResponsePassthrough(context.Background(), resp, c, PlatformOpenAI, "deepseek-reasoner", "deepseek-reasoner", true)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.usageObserved)
	require.Contains(t, rec.Header().Get("Content-Type"), "application/json")
	require.Equal(t, "plan carefully", gjson.Get(rec.Body.String(), "output.0.summary.0.text").String())
	require.Equal(t, "function_call", gjson.Get(rec.Body.String(), "output.1.type").String())
}

func deepSeekLegacyReasoningSSE() string {
	return strings.Join([]string{
		`event: response.output_item.added`,
		`data: {"type":"response.output_item.added","output_index":0,"item":{"id":"rs_1","type":"reasoning","status":"in_progress","summary":[],"content":[]},"sequence_number":1}`,
		``,
		`event: response.reasoning_text.delta`,
		`data: {"type":"response.reasoning_text.delta","item_id":"rs_1","output_index":0,"content_index":0,"delta":"plan ","sequence_number":2}`,
		``,
		`event: response.reasoning_text.delta`,
		`data: {"type":"response.reasoning_text.delta","item_id":"rs_1","output_index":0,"content_index":0,"delta":"carefully","sequence_number":3}`,
		``,
		`event: response.reasoning_text.done`,
		`data: {"type":"response.reasoning_text.done","item_id":"rs_1","output_index":0,"content_index":0,"text":"plan carefully","sequence_number":4}`,
		``,
		`event: response.output_item.done`,
		`data: {"type":"response.output_item.done","output_index":0,"item":{"id":"rs_1","type":"reasoning","status":"completed","summary":[],"content":[{"type":"reasoning_text","text":"plan carefully"}]},"sequence_number":5}`,
		``,
		`event: response.completed`,
		`data: {"type":"response.completed","response":{"id":"resp_1","object":"response","status":"completed","output":[{"id":"rs_1","type":"reasoning","status":"completed","summary":[],"content":[{"type":"reasoning_text","text":"plan carefully"}]},{"id":"call_1","type":"function_call","name":"lookup","arguments":"{}","call_id":"call_1","status":"completed"}],"usage":{"input_tokens":3,"output_tokens":4,"total_tokens":7}},"sequence_number":6}`,
		``,
	}, "\n")
}

func responseEventTypes(payloads []string) []string {
	types := make([]string, 0, len(payloads))
	for _, payload := range payloads {
		types = append(types, gjson.Get(payload, "type").String())
	}
	return types
}
