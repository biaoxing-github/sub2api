package service

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestOpenAISSEJSONDocumentScannerRepairsConcatenatedEvents 验证拼接 JSON 被恢复为独立 SSE 事件。
func TestOpenAISSEJSONDocumentScannerRepairsConcatenatedEvents(t *testing.T) {
	first := `{"type":"response.in_progress","response":{"id":"resp_1"}}`
	second := `{"type":"response.completed","response":{"id":"resp_1","usage":{"input_tokens":7,"output_tokens":9}}}`
	scanner := bufio.NewScanner(strings.NewReader("event: response.in_progress\ndata: " + first + second + "\n\n"))
	documentScanner := newOpenAISSEJSONDocumentScanner(scanner)

	var lines []string
	for documentScanner.Scan() {
		lines = append(lines, documentScanner.Text())
	}
	require.NoError(t, documentScanner.Err())
	require.Equal(t, []string{
		"event: response.in_progress",
		"data: " + first,
		"",
		"event: response.completed",
		"data: " + second,
		"",
		"",
	}, lines)
	for _, line := range []string{lines[1], lines[4]} {
		data, ok := extractOpenAISSEDataLine(line)
		require.True(t, ok)
		require.True(t, json.Valid([]byte(data)))
	}
}

// TestSplitOpenAIConcatenatedJSONDocumentsRejectsMalformedPayload 验证普通损坏 JSON 不被启发式改写。
func TestSplitOpenAIConcatenatedJSONDocumentsRejectsMalformedPayload(t *testing.T) {
	documents, repaired := splitOpenAIConcatenatedJSONDocuments([]byte(`{"type":"response.in_progress"}broken`))
	require.False(t, repaired)
	require.Nil(t, documents)
}

// TestSplitOpenAIConcatenatedJSONDocumentsRejectsPayloadOverLimit 验证超限载荷保持原错误语义。
func TestSplitOpenAIConcatenatedJSONDocumentsRejectsPayloadOverLimit(t *testing.T) {
	payload := []byte(`{"type":"response.in_progress","padding":"` + strings.Repeat("x", maxOpenAIConcatenatedJSONBytes) + `"}{"type":"response.completed"}`)
	documents, repaired := splitOpenAIConcatenatedJSONDocuments(payload)
	require.False(t, repaired)
	require.Nil(t, documents)
}

// TestOpenAIStreamingRepairsConcatenatedJSONDocuments 验证标准与透传 HTTP 路径都消费恢复后的独立事件。
func TestOpenAIStreamingRepairsConcatenatedJSONDocuments(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		passthrough := passthrough
		t.Run(map[bool]string{false: "standard", true: "passthrough"}[passthrough], func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			inProgress := `{"type":"response.in_progress","response":{"id":"resp_joined"}}`
			completed := `{"type":"response.completed","response":{"id":"resp_joined","usage":{"input_tokens":7,"output_tokens":9}}}`
			upstreamBody := "event: response.in_progress\ndata: " + inProgress + completed + "\n\n"
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
				Body:       io.NopCloser(strings.NewReader(upstreamBody)),
			}
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			svc := &OpenAIGatewayService{
				cfg:           &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}},
				toolCorrector: NewCodexToolCorrector(),
			}
			account := &Account{ID: 1, Name: "joined-json", Platform: PlatformOpenAI}

			if passthrough {
				result, err := svc.handleStreamingResponsePassthrough(context.Background(), resp, c, account, time.Now(), "gpt-5.6-sol", "gpt-5.6-sol")
				require.NoError(t, err)
				require.NotNil(t, result)
				require.NotNil(t, result.usage)
				require.Equal(t, 7, result.usage.InputTokens)
				require.Equal(t, 9, result.usage.OutputTokens)
			} else {
				result, err := svc.handleStreamingResponse(context.Background(), resp, c, account, time.Now(), "gpt-5.6-sol", "gpt-5.6-sol")
				require.NoError(t, err)
				require.NotNil(t, result)
				require.NotNil(t, result.usage)
				require.Equal(t, 7, result.usage.InputTokens)
				require.Equal(t, 9, result.usage.OutputTokens)
			}

			var eventTypes []string
			for _, line := range strings.Split(recorder.Body.String(), "\n") {
				data, ok := extractOpenAISSEDataLine(line)
				if !ok || strings.TrimSpace(data) == "" || strings.TrimSpace(data) == "[DONE]" {
					continue
				}
				require.True(t, json.Valid([]byte(data)), data)
				var event struct {
					Type string `json:"type"`
				}
				require.NoError(t, json.Unmarshal([]byte(data), &event))
				eventTypes = append(eventTypes, event.Type)
			}
			require.Equal(t, []string{"response.in_progress", "response.completed"}, eventTypes)
		})
	}
}
