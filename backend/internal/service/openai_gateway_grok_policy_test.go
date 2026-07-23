package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// grokContentPolicyRepo 记录内容策略拒绝路径是否错误写入账号状态或配额快照。
type grokContentPolicyRepo struct {
	stubOpenAIAccountRepo
	tempUnschedulableCalls int
	updateExtraCalls       int
	lastTempUntil          time.Time
	lastTempReason         string
}

// SetTempUnschedulable 记录账号被临时移出调度池的持久化请求。
func (r *grokContentPolicyRepo) SetTempUnschedulable(_ context.Context, _ int64, until time.Time, reason string) error {
	r.tempUnschedulableCalls++
	r.lastTempUntil = until
	r.lastTempReason = reason
	return nil
}

// UpdateExtra 记录 Grok 配额快照的持久化请求。
func (r *grokContentPolicyRepo) UpdateExtra(_ context.Context, _ int64, _ map[string]any) error {
	r.updateExtraCalls++
	return nil
}

// grokContentPolicyAccount 构造可用于 HTTP 入口回归的 Grok API Key 账号。
func grokContentPolicyAccount(id int64) *Account {
	return &Account{
		ID:          id,
		Name:        "grok-policy-test",
		Platform:    PlatformGrok,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "grok-policy-token",
			"base_url": "https://xai.test/v1",
		},
	}
}

// grokContentPolicyResponse 返回带配额头的请求级内容策略 403，用于确认该头不会被持久化。
func grokContentPolicyResponse(message string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusForbidden,
		Header: http.Header{
			"Content-Type":          []string{"application/json"},
			"Xai-Subscription-Tier": []string{"premium"},
			"Xai-Request-Id":        []string{"grok-policy-request"},
		},
		Body: io.NopCloser(bytes.NewBufferString(`{"error":{"code":"new_sensitive","message":"` + message + `"}}`)),
	}
}

// grokContentPolicyOAuthAccount 构造可进入 Grok Chat Responses bridge 的 OAuth 账号。
func grokContentPolicyOAuthAccount(id int64) *Account {
	account := grokContentPolicyAccount(id)
	account.Type = AccountTypeOAuth
	account.Credentials = map[string]any{
		"access_token": "grok-policy-oauth-token",
		"base_url":     "https://cli-chat-proxy.grok.com",
	}
	return account
}

// TestIsGrokContentPolicyRejection 区分请求内容拒绝、账号访问拒绝和不相关状态码。
func TestIsGrokContentPolicyRejection(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   bool
	}{
		{
			name:   "敏感内容码",
			status: http.StatusForbidden,
			body:   `{"error":{"code":"new_sensitive","message":"image is sensitive"}}`,
			want:   true,
		},
		{
			name:   "嵌套内容策略码",
			status: http.StatusForbidden,
			body:   `{"response":{"error":{"code":"content_policy_violation"}}}`,
			want:   true,
		},
		{
			name:   "账号停用优先于策略文本",
			status: http.StatusForbidden,
			body:   `{"error":{"code":"account_suspended","message":"account suspended due to policy violation"}}`,
			want:   false,
		},
		{
			name:   "订阅不足不是请求内容问题",
			status: http.StatusForbidden,
			body:   `{"error":{"message":"subscription required"}}`,
			want:   false,
		},
		{
			name:   "非403不分类",
			status: http.StatusBadRequest,
			body:   `{"error":{"code":"new_sensitive"}}`,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isGrokContentPolicyRejection(tt.status, []byte(tt.body)))
		})
	}
}

// TestHandleGrokContentPolicy403LeavesAccountState 验证账号状态更新入口自身也拒绝内容策略 403。
func TestHandleGrokContentPolicy403LeavesAccountState(t *testing.T) {
	repo := &grokContentPolicyRepo{}
	svc := &OpenAIGatewayService{accountRepo: repo}
	body := []byte(`{"error":{"code":"content_filter","message":"prohibited content"}}`)

	svc.handleGrokAccountUpstreamError(context.Background(), grokContentPolicyAccount(6100), http.StatusForbidden, nil, body)

	require.Zero(t, repo.tempUnschedulableCalls)
	require.Zero(t, repo.updateExtraCalls)
	require.False(t, svc.shouldFailoverGrokUpstreamError(http.StatusForbidden, body))
}

// TestForwardGrokResponsesContentPolicy403DoesNotConsumeAccount 验证 Responses 内容策略拒绝直接返回客户端，不能消耗账号池。
func TestForwardGrokResponsesContentPolicy403DoesNotConsumeAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"grok-4.5","input":"blocked prompt"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))

	repo := &grokContentPolicyRepo{}
	svc := &OpenAIGatewayService{
		accountRepo:  repo,
		httpUpstream: &httpUpstreamRecorder{resp: grokContentPolicyResponse("text is sensitive")},
	}
	_, err := svc.forwardGrokResponses(context.Background(), c, grokContentPolicyAccount(6101), body, "grok-4.5", false, time.Now())

	var failoverErr *UpstreamFailoverError
	require.Error(t, err)
	require.False(t, errors.As(err, &failoverErr))
	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Equal(t, "invalid_request_error", gjson.Get(recorder.Body.String(), "error.type").String())
	require.Equal(t, "text is sensitive", gjson.Get(recorder.Body.String(), "error.message").String())
	require.Zero(t, repo.tempUnschedulableCalls)
	require.Zero(t, repo.updateExtraCalls)
}

// TestForwardGrokChatBridgeContentPolicy403DoesNotConsumeAccount 验证 Chat bridge 也直接返回请求级 403。
func TestForwardGrokChatBridgeContentPolicy403DoesNotConsumeAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"grok-4.5","messages":[{"role":"user","content":"blocked prompt"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Set("api_key", &APIKey{ID: 6106, Group: &Group{Platform: PlatformGrok}})

	repo := &grokContentPolicyRepo{}
	svc := &OpenAIGatewayService{
		accountRepo:  repo,
		httpUpstream: &httpUpstreamRecorder{resp: grokContentPolicyResponse("text is sensitive")},
	}
	_, err := svc.forwardGrokChatCompletionsViaResponses(context.Background(), c, grokContentPolicyOAuthAccount(6106), body, "grok-policy-cache", "")

	var failoverErr *UpstreamFailoverError
	require.Error(t, err)
	require.False(t, errors.As(err, &failoverErr))
	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Equal(t, "invalid_request_error", gjson.Get(recorder.Body.String(), "error.type").String())
	require.Zero(t, repo.tempUnschedulableCalls)
	require.Zero(t, repo.updateExtraCalls)
}

// TestForwardGrokAnthropicContentPolicy403DoesNotConsumeAccount 验证 /v1/messages 的 Responses 兼容路径不因请求内容拒绝切换 Grok 账号。
func TestForwardGrokAnthropicContentPolicy403DoesNotConsumeAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"grok-4.5","max_tokens":64,"messages":[{"role":"user","content":"blocked prompt"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))

	repo := &grokContentPolicyRepo{}
	svc := &OpenAIGatewayService{
		accountRepo:  repo,
		httpUpstream: &httpUpstreamRecorder{resp: grokContentPolicyResponse("text is sensitive")},
	}
	_, err := svc.ForwardAsAnthropic(context.Background(), c, grokContentPolicyAccount(6107), body, "", "")

	var failoverErr *UpstreamFailoverError
	require.Error(t, err)
	require.False(t, errors.As(err, &failoverErr))
	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Equal(t, "invalid_request_error", gjson.Get(recorder.Body.String(), "error.type").String())
	require.Zero(t, repo.tempUnschedulableCalls)
	require.Zero(t, repo.updateExtraCalls)
}

// TestGrokContentPolicy403SharedCompatFallbackDoesNotMutate 验证共享 Chat 错误回退也绕过自定义错误码和账号写入。
func TestGrokContentPolicy403SharedCompatFallbackDoesNotMutate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	repo := &grokContentPolicyRepo{}
	svc := &OpenAIGatewayService{accountRepo: repo}
	account := grokContentPolicyAccount(6102)
	account.Credentials["custom_error_codes_enabled"] = true
	account.Credentials["custom_error_codes"] = []any{float64(http.StatusTooManyRequests)}
	resp := grokContentPolicyResponse("prohibited content")

	_, err := svc.handleCompatErrorResponse(resp, c, account, writeChatCompletionsError, "grok-4.5")

	require.Error(t, err)
	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Equal(t, "invalid_request_error", gjson.Get(recorder.Body.String(), "error.type").String())
	require.Equal(t, "prohibited content", gjson.Get(recorder.Body.String(), "error.message").String())
	require.Zero(t, repo.tempUnschedulableCalls)
	require.Zero(t, repo.updateExtraCalls)
}

// TestForwardGrokImagesContentPolicy403DoesNotPersistQuota 验证图片入口在内容策略拒绝时不写配额快照或冷却账号。
func TestForwardGrokImagesContentPolicy403DoesNotPersistQuota(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	body := []byte(`{"model":"grok-imagine","prompt":"blocked image"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c, _ := gin.CreateTestContext(recorder)
	c.Request = req

	repo := &grokContentPolicyRepo{}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		accountRepo:  repo,
		httpUpstream: &httpUpstreamRecorder{resp: grokContentPolicyResponse("image is sensitive")},
	}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)

	_, err = svc.ForwardImages(context.Background(), c, grokContentPolicyAccount(6103), body, parsed, "")

	var failoverErr *UpstreamFailoverError
	require.Error(t, err)
	require.False(t, errors.As(err, &failoverErr))
	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Equal(t, "invalid_request_error", gjson.Get(recorder.Body.String(), "error.type").String())
	require.Zero(t, repo.tempUnschedulableCalls)
	require.Zero(t, repo.updateExtraCalls)
}

// TestGrokSchedulerProbeContentPolicy403DoesNotMutate 验证后台探测将内容策略作为探测失败而非账号失效。
func TestGrokSchedulerProbeContentPolicy403DoesNotMutate(t *testing.T) {
	repo := &grokContentPolicyRepo{}
	svc := &OpenAIGatewayService{
		accountRepo:  repo,
		httpUpstream: &httpUpstreamRecorder{resp: grokContentPolicyResponse("text is sensitive")},
	}

	err := svc.sendGrokSchedulerExhaustionProbe(context.Background(), grokContentPolicyAccount(6104), "grok-4.5", false)

	require.Error(t, err)
	require.Zero(t, repo.tempUnschedulableCalls)
	require.Zero(t, repo.updateExtraCalls)
}

// TestGrokEntitlement403HonorsConfiguredCooldown 验证非内容策略 403 仍使用管理员配置的临时不可调度时长。
func TestGrokEntitlement403HonorsConfiguredCooldown(t *testing.T) {
	repo := &grokContentPolicyRepo{}
	svc := &OpenAIGatewayService{accountRepo: repo}
	account := grokContentPolicyAccount(6105)
	account.Credentials["temp_unschedulable_enabled"] = true
	account.Credentials["temp_unschedulable_rules"] = []any{
		map[string]any{
			"error_code":       float64(http.StatusForbidden),
			"keywords":         []any{"subscription required"},
			"duration_minutes": float64(7),
		},
	}
	before := time.Now()

	svc.handleGrokAccountUpstreamError(
		context.Background(), account, http.StatusForbidden, nil,
		[]byte(`{"error":{"message":"subscription required"}}`),
	)

	require.Equal(t, 1, repo.tempUnschedulableCalls)
	require.Greater(t, repo.lastTempUntil, before.Add(6*time.Minute))
	require.Less(t, repo.lastTempUntil, before.Add(8*time.Minute))
}
