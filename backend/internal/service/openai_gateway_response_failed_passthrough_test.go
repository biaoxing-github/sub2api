//go:build unit

package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func buildContextLengthFailedSSE() string {
	failed := `{"type":"response.failed","response":{"id":"resp_err","object":"response","status":"failed","error":{"code":"context_length_exceeded","type":"invalid_request_error","message":"Your input exceeds the context window of this model. Please adjust your input and try again."},"output":[],"usage":{"input_tokens":100000,"output_tokens":0,"total_tokens":100000}}}`
	return fmt.Sprintf("data: %s\n\n", failed)
}

// bindResponseFailedStatusRule 绑定需要同时匹配语义状态码和错误关键词的透传规则。
func bindResponseFailedStatusRule(c *gin.Context, platform string, statusCode int, keyword string, responseCode int) {
	rule := &model.ErrorPassthroughRule{
		ID:              1,
		Name:            "response-failed-status-rule",
		Enabled:         true,
		Priority:        1,
		Platforms:       []string{platform},
		ErrorCodes:      []int{statusCode},
		Keywords:        []string{keyword},
		MatchMode:       model.MatchModeAll,
		ResponseCode:    &responseCode,
		PassthroughBody: true,
	}
	svc := &ErrorPassthroughService{}
	svc.setLocalCache([]*model.ErrorPassthroughRule{rule})
	BindErrorPassthroughService(c, svc)
}

func TestForwardAsChatCompletions_ResponseFailed_ErrorCodeRuleMatchesViaSemanticStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	bindResponseFailedStatusRule(c, PlatformOpenAI, http.StatusBadRequest, "context_length_exceeded", http.StatusBadRequest)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(buildContextLengthFailedSSE())),
	}}
	svc := &OpenAIGatewayService{
		cfg:          rawChatCompletionsTestConfig(),
		httpUpstream: upstream,
	}

	account := rawChatCompletionsTestAccount()
	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")

	require.Error(t, err)
	require.NotNil(t, result)
	require.True(t, result.UsageObserved)
	require.Equal(t, 100000, result.Usage.InputTokens)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "upstream_error", gjson.Get(rec.Body.String(), "error.type").String())
	require.Contains(t, gjson.Get(rec.Body.String(), "error.message").String(), "context window")
}

func TestForwardAsAnthropic_ResponseFailed_ErrorCodeRuleMatchesViaSemanticStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"model":"gpt-5.4","max_tokens":32,"messages":[{"role":"user","content":"hello"}],"stream":false}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	bindResponseFailedStatusRule(c, PlatformOpenAI, http.StatusBadRequest, "context_length_exceeded", http.StatusBadRequest)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(buildContextLengthFailedSSE())),
	}}
	svc := &OpenAIGatewayService{
		cfg:          rawChatCompletionsTestConfig(),
		httpUpstream: upstream,
	}

	account := rawChatCompletionsTestAccount()
	result, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, "", "")

	require.Error(t, err)
	require.NotNil(t, result)
	require.True(t, result.UsageObserved)
	require.Equal(t, 100000, result.Usage.InputTokens)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, gjson.Get(rec.Body.String(), "error.message").String(), "context window")
}

func TestApplyOpenAIStreamFailedErrorPassthroughRuleUsesAccountPlatform(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	bindResponseFailedStatusRule(c, PlatformGrok, http.StatusBadRequest, "context_length_exceeded", http.StatusUnprocessableEntity)
	payload := []byte(strings.TrimSpace(strings.TrimPrefix(buildContextLengthFailedSSE(), "data: ")))

	status, errType, errMsg, matched := applyOpenAIStreamFailedErrorPassthroughRule(
		c,
		PlatformGrok,
		payload,
		"Your input exceeds the context window of this model.",
	)

	require.True(t, matched)
	require.Equal(t, http.StatusUnprocessableEntity, status)
	require.Equal(t, "upstream_error", errType)
	require.Contains(t, errMsg, "context window")
}

func TestOpenAIStreamingResponseFailedBeforeOutputAppliesPassthroughRule(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	bindResponseFailedStatusRule(c, PlatformOpenAI, http.StatusBadRequest, "context_length_exceeded", http.StatusBadRequest)

	svc := &OpenAIGatewayService{
		cfg:           rawChatCompletionsTestConfig(),
		toolCorrector: NewCodexToolCorrector(),
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
			"x-request-id": []string{"req_response_failed"},
		},
		Body: io.NopCloser(strings.NewReader(buildContextLengthFailedSSE())),
	}
	account := rawChatCompletionsTestAccount()

	result, err := svc.handleStreamingResponse(context.Background(), resp, c, account, time.Now(), "gpt-5.4", "gpt-5.4")

	require.Error(t, err)
	require.NotNil(t, result)
	require.True(t, result.usageObserved)
	require.NotNil(t, result.usage)
	require.Equal(t, 100000, result.usage.InputTokens)
	require.Contains(t, err.Error(), "passthrough")
	require.Equal(t, http.StatusBadRequest, rec.Code, "未提交真实输出时应返回语义 HTTP 状态")
	require.Equal(t, "upstream_error", gjson.Get(rec.Body.String(), "error.type").String())
	require.Contains(t, gjson.Get(rec.Body.String(), "error.message").String(), "context window")
	requireOpsUpstreamErrorRecorded(t, c, PlatformOpenAI)
}

func TestOpenAIStreamingPassthroughResponseFailedBeforeOutputAppliesPassthroughRule(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	bindResponseFailedStatusRule(c, PlatformOpenAI, http.StatusBadRequest, "context_length_exceeded", http.StatusBadRequest)

	svc := &OpenAIGatewayService{
		cfg:           rawChatCompletionsTestConfig(),
		toolCorrector: NewCodexToolCorrector(),
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
			"x-request-id": []string{"req_passthrough_response_failed"},
		},
		Body: io.NopCloser(strings.NewReader(buildContextLengthFailedSSE())),
	}
	account := rawChatCompletionsTestAccount()

	result, err := svc.handleStreamingResponsePassthrough(context.Background(), resp, c, account, time.Now(), "gpt-5.4", "gpt-5.4")

	require.Error(t, err)
	require.NotNil(t, result)
	require.True(t, result.usageObserved)
	require.NotNil(t, result.usage)
	require.Equal(t, 100000, result.usage.InputTokens)
	require.Contains(t, err.Error(), "passthrough")
	require.Equal(t, http.StatusBadRequest, rec.Code, "首个真实 SSE 事件前未提交时应返回语义 HTTP 状态")
	require.Equal(t, "upstream_error", gjson.Get(rec.Body.String(), "error.type").String())
	require.Contains(t, gjson.Get(rec.Body.String(), "error.message").String(), "context window")
	requireOpsUpstreamErrorRecorded(t, c, PlatformOpenAI)
}

func TestHandleSSEToJSON_ResponseFailedAppliesSemanticRuleAndReturnsUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	bindResponseFailedStatusRule(c, PlatformOpenAI, http.StatusBadRequest, "context_length_exceeded", http.StatusBadRequest)

	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig()}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}

	result, err := svc.handleSSEToJSON(resp, c, []byte(buildContextLengthFailedSSE()), PlatformOpenAI, "gpt-5.4", "gpt-5.4")

	require.Error(t, err)
	require.NotNil(t, result)
	require.Equal(t, 100000, result.InputTokens)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "upstream_error", gjson.Get(rec.Body.String(), "error.type").String())
	require.NotContains(t, rec.Body.String(), "event: response.failed")
}

func TestHandlePassthroughSSEToJSON_ResponseFailedAppliesSemanticRuleAndReturnsUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	bindResponseFailedStatusRule(c, PlatformOpenAI, http.StatusBadRequest, "context_length_exceeded", http.StatusBadRequest)

	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig()}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}

	result, err := svc.handlePassthroughSSEToJSON(resp, c, []byte(buildContextLengthFailedSSE()), PlatformOpenAI, "gpt-5.4", "gpt-5.4", false)

	require.Error(t, err)
	require.NotNil(t, result)
	require.Equal(t, 100000, result.InputTokens)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "upstream_error", gjson.Get(rec.Body.String(), "error.type").String())
	require.NotContains(t, rec.Body.String(), "event: response.failed")
}

// requireOpsUpstreamErrorRecorded 校验 response.failed 透传命中后仍保留账号平台级 Ops 事件。
func requireOpsUpstreamErrorRecorded(t *testing.T, c *gin.Context, platform string) {
	t.Helper()
	raw, exists := c.Get(OpsUpstreamErrorsKey)
	require.True(t, exists)
	events, ok := raw.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.NotEmpty(t, events)
	require.Equal(t, platform, events[len(events)-1].Platform)
}
