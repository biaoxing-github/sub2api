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

// OAuth 映射会剥离上游模型后缀；用量元数据必须继续从原始模型候选推导。
func TestExtractOpenAIReasoningEffortFromBodyModelCandidates(t *testing.T) {
	body := []byte(`{"model":"mapped","input":"hello"}`)

	got := extractOpenAIReasoningEffortFromBody(body, "gpt-5.4", "gpt-5.4-xhigh")
	require.NotNil(t, got)
	require.Equal(t, "xhigh", *got)

	got = extractOpenAIReasoningEffortFromBody(body, "gpt-5.6-sol", "gpt-5.6-sol-max")
	require.NotNil(t, got)
	require.Equal(t, "max", *got)
}

func TestExtractOpenAIReasoningEffortModelCandidates(t *testing.T) {
	reqBody := map[string]any{"model": "mapped", "input": "hello"}

	got := extractOpenAIReasoningEffort(reqBody, "gpt-5.3-codex", "gpt-5.3-codex-high")
	require.NotNil(t, got)
	require.Equal(t, "high", *got)
}

// GPT-5.6 的带后缀路由模型也属于同一计费与推理强度模型族。
func TestIsOpenAIGPT56ModelRecognizesSuffixedAliases(t *testing.T) {
	require.True(t, isOpenAIGPT56Model("gpt-5.6-sol-max"))
	require.True(t, isOpenAIGPT56Model("provider/gpt5.6terra-max"))
	require.False(t, isOpenAIGPT56Model("gpt-5.5-pro-max"))
}

// OAuth 上游会规范化模型基名；结果用量的 reasoning effort 仍必须从原始请求
// 模型的后缀恢复，防止后台显示丢失 xhigh/max。
func TestOpenAIGatewayServiceForwardOAuthDerivesEffortFromSuffixModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"usage":{"input_tokens":1,"output_tokens":2}}`)),
		},
	}
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: upstream}
	account := &Account{
		ID:          11,
		Name:        "openai-oauth-suffix",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
		},
		Status:      StatusActive,
		Schedulable: true,
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)

	body := []byte(`{"model":"gpt-5.3-codex-xhigh","instructions":"suffix-test","input":"hello","stream":false}`)
	result, err := svc.Forward(context.Background(), c, account, body)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "gpt-5.3-codex", gjson.GetBytes(upstream.lastBody, "model").String())
	require.NotNil(t, result.ReasoningEffort)
	require.Equal(t, "xhigh", *result.ReasoningEffort)
}
