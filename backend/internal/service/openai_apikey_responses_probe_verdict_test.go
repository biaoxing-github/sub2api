package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

type responsesProbeVerdictRepo struct {
	AccountRepository
	// account 是本次探测加载的账号。
	account *Account
	// updates 记录探测最终写入的 extra 字段；nil 表示保持 unknown。
	updates map[string]any
}

// GetByID 返回测试固定的 OpenAI APIKey 账号。
func (r *responsesProbeVerdictRepo) GetByID(_ context.Context, _ int64) (*Account, error) {
	return r.account, nil
}

// UpdateExtra 捕获探测对账号 extra 的持久化结果。
func (r *responsesProbeVerdictRepo) UpdateExtra(_ context.Context, _ int64, updates map[string]any) error {
	r.updates = updates
	return nil
}

type responsesProbeVerdictUpstream struct {
	// status 是模拟的上游 HTTP 状态码。
	status int
	// body 是模拟的非流式 Responses 响应体。
	body string
}

// Do 返回测试指定的上游响应。
func (u *responsesProbeVerdictUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	return &http.Response{
		StatusCode: u.status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(u.body)),
	}, nil
}

// DoWithTLS 复用 Do 的固定响应，满足探测服务的 TLS 上游接口。
func (u *responsesProbeVerdictUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, concurrency)
}

// runResponsesProbeVerdict 执行一次探测并返回实际持久化的 extra 更新。
func runResponsesProbeVerdict(t *testing.T, status int, body string) map[string]any {
	t.Helper()
	account := &Account{
		ID:       4200,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://responses-probe.example.test/v1",
		},
	}
	repo := &responsesProbeVerdictRepo{account: account}
	svc := &AccountTestService{
		accountRepo:  repo,
		httpUpstream: &responsesProbeVerdictUpstream{status: status, body: body},
		cfg: &config.Config{Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{Enabled: false},
		}},
	}

	svc.ProbeOpenAIAPIKeyResponsesSupport(context.Background(), account.ID)
	return repo.updates
}

func TestProbeOpenAIAPIKeyResponsesSupport_InconclusiveResponseKeepsUnknown(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{
			name: "incomplete_max_output_tokens",
			body: `{"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"}}`,
		},
		{
			name: "failed_status_on_http_200",
			body: `{"status":"failed","error":{"code":"server_error"}}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Nil(t, runResponsesProbeVerdict(t, http.StatusOK, tc.body),
				"inconclusive response must keep openai_responses_supported unknown")
		})
	}
}

func TestProbeOpenAIAPIKeyResponsesSupport_ConclusiveResponseStillPersists(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   bool
	}{
		{name: "completed_with_function_call", status: http.StatusOK, body: `{"status":"completed","output":[{"type":"function_call","name":"probe_ping"}]}`, want: true},
		{name: "completed_reasoning_only", status: http.StatusOK, body: `{"status":"completed","output":[{"type":"reasoning"}]}`, want: false},
		{name: "incomplete_other_reason", status: http.StatusOK, body: `{"status":"incomplete","incomplete_details":{"reason":"content_filter"}}`, want: false},
		{name: "no_status_with_function_call", status: http.StatusOK, body: `{"output":[{"type":"function_call","name":"probe_ping"}]}`, want: true},
		{name: "endpoint_absent", status: http.StatusNotFound, body: `{"error":{"message":"Not Found"}}`, want: false},
		{name: "server_error", status: http.StatusInternalServerError, body: `{"status":"failed"}`, want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			updates := runResponsesProbeVerdict(t, tc.status, tc.body)
			require.NotNil(t, updates, "conclusive response must persist a verdict")
			require.Equal(t, tc.want, updates[openai_compat.ExtraKeyResponsesSupported])
		})
	}
}

func TestResponsesProbeVerdictIsConclusive(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   bool
	}{
		{name: "completed", status: http.StatusOK, body: `{"status":"completed"}`, want: true},
		{name: "incomplete_max_output_tokens", status: http.StatusOK, body: `{"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"}}`, want: false},
		{name: "incomplete_other_reason", status: http.StatusOK, body: `{"status":"incomplete","incomplete_details":{"reason":"content_filter"}}`, want: true},
		{name: "failed", status: http.StatusOK, body: `{"status":"failed"}`, want: false},
		{name: "non_2xx_ignores_body", status: http.StatusInternalServerError, body: `{"status":"failed"}`, want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, responsesProbeVerdictIsConclusive(tc.status, []byte(tc.body)))
		})
	}
}
