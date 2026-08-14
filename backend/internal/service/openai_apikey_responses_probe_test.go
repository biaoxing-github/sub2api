package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type openAIResponsesProbeRepo struct {
	AccountRepository
	account *Account
	updates map[string]any
}

func (r *openAIResponsesProbeRepo) GetByID(_ context.Context, _ int64) (*Account, error) {
	return r.account, nil
}

func (r *openAIResponsesProbeRepo) UpdateExtra(_ context.Context, _ int64, updates map[string]any) error {
	r.updates = updates
	return nil
}

type openAIResponsesProbeUpstream struct {
	request *http.Request
}

func (u *openAIResponsesProbeUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.request = req
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"id":"resp_probe","status":"completed","output":[{"type":"function_call","name":"probe_ping"}]}`)),
	}, nil
}

func (u *openAIResponsesProbeUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, concurrency)
}

func TestProbeOpenAIAPIKeyResponsesSupportUsesAccountPassthroughUserAgent(t *testing.T) {
	account := &Account{
		ID:       490,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":    "sk-test",
			"base_url":   "https://responses-probe.example.test/v1",
			"user_agent": "account-probe/1.0",
			"model_mapping": map[string]any{
				"client-model": "upstream-probe-model",
			},
		},
	}
	repo := &openAIResponsesProbeRepo{account: account}
	upstream := &openAIResponsesProbeUpstream{}
	svc := &AccountTestService{
		accountRepo:  repo,
		httpUpstream: upstream,
		cfg: &config.Config{Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{Enabled: false},
		}},
	}

	svc.ProbeOpenAIAPIKeyResponsesSupport(context.Background(), account.ID)

	require.NotNil(t, upstream.request)
	require.Equal(t, "https://responses-probe.example.test/v1/responses", upstream.request.URL.String())
	require.Equal(t, "account-probe/1.0", upstream.request.Header.Get("user-agent"))
	require.Equal(t, "application/json", upstream.request.Header.Get("accept"))
	requestBody, err := io.ReadAll(upstream.request.Body)
	require.NoError(t, err)
	require.Equal(t, "upstream-probe-model", gjson.GetBytes(requestBody, "model").String())
	require.Equal(t, "required", gjson.GetBytes(requestBody, "tool_choice").String())
	require.Equal(t, "probe_ping", gjson.GetBytes(requestBody, "tools.0.name").String())
	require.Equal(t, int64(openaiResponsesProbeMaxOutputTokens), gjson.GetBytes(requestBody, "max_output_tokens").Int())
	require.Equal(t, true, repo.updates["openai_responses_supported"])
}

// TestDecideResponsesProbeSupport 固定端点存在性与工具能力的判定矩阵。
func TestDecideResponsesProbeSupport(t *testing.T) {
	functionCall := []byte(`{"output":[{"type":"reasoning"},{"type":"function_call","name":"probe_ping"}]}`)
	reasoningOnly := []byte(`{"output":[{"type":"reasoning"}]}`)
	cases := []struct {
		name   string
		status int
		body   []byte
		want   bool
	}{
		{name: "404 endpoint absent", status: http.StatusNotFound, body: functionCall, want: false},
		{name: "405 endpoint absent", status: http.StatusMethodNotAllowed, body: functionCall, want: false},
		{name: "200 with function call", status: http.StatusOK, body: functionCall, want: true},
		{name: "200 reasoning only", status: http.StatusOK, body: reasoningOnly, want: false},
		{name: "200 invalid json", status: http.StatusOK, body: []byte("not-json"), want: false},
		{name: "400 conservative true", status: http.StatusBadRequest, body: reasoningOnly, want: true},
		{name: "401 conservative true", status: http.StatusUnauthorized, want: true},
		{name: "500 conservative true", status: http.StatusInternalServerError, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, decideResponsesProbeSupport(tc.status, tc.body))
		})
	}
}

// TestResponsesProbeBodyHasFunctionCall 覆盖 output 数组的合法、缺失与异常输入。
func TestResponsesProbeBodyHasFunctionCall(t *testing.T) {
	require.True(t, responsesProbeBodyHasFunctionCall([]byte(`{"output":[{"type":"function_call"}]}`)))
	require.True(t, responsesProbeBodyHasFunctionCall([]byte(`{"output":[{"type":"reasoning"},{"type":"function_call"}]}`)))
	require.False(t, responsesProbeBodyHasFunctionCall([]byte(`{"output":[{"type":"reasoning"}]}`)))
	require.False(t, responsesProbeBodyHasFunctionCall([]byte(`{"output":[]}`)))
	require.False(t, responsesProbeBodyHasFunctionCall([]byte(`{}`)))
	require.False(t, responsesProbeBodyHasFunctionCall([]byte(`garbage`)))
}

// TestSelectResponsesProbeModel 验证具体映射优先、通配符跳过和默认模型回退。
func TestSelectResponsesProbeModel(t *testing.T) {
	require.Equal(t, openai.DefaultTestModel, selectResponsesProbeModel(&Account{}))

	account := &Account{Credentials: map[string]any{
		"model_mapping": map[string]any{
			"client-b": "zeta-model",
			"client-a": "alpha-model",
		},
	}}
	require.Equal(t, "alpha-model", selectResponsesProbeModel(account))

	wildcardAccount := &Account{Credentials: map[string]any{
		"model_mapping": map[string]any{
			"a": "*",
			"b": "  ",
			"c": "real-model",
		},
	}}
	require.Equal(t, "real-model", selectResponsesProbeModel(wildcardAccount))

	allWildcardAccount := &Account{Credentials: map[string]any{
		"model_mapping": map[string]any{"a": "gpt-*"},
	}}
	require.Equal(t, openai.DefaultTestModel, selectResponsesProbeModel(allWildcardAccount))
}
