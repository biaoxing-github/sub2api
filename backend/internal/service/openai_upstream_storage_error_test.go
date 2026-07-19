//go:build unit

package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const upstreamDiskStorageFailureBody = `{"error":{"message":"Invalid request: Invalid request: disk storage creation failed: failed to write to temp file: disk free-space floor reached: free=5671645184 reserved=0 requested=1048576 minimum=10737418240 (request id: test-request)","type":"new_api_error","param":"","code":""}}`

// TestClassifyOpenAIUpstreamInfrastructureFailure 验证只把高置信度基础设施故障纳入账号级熔断。
func TestClassifyOpenAIUpstreamInfrastructureFailure(t *testing.T) {
	tests := []struct {
		name            string
		statusCode      int
		body            []byte
		wantMatched     bool
		wantReason      string
		wantMinCooldown time.Duration
	}{
		{name: "disk temp file failure", statusCode: http.StatusBadRequest, body: []byte(upstreamDiskStorageFailureBody), wantMatched: true, wantReason: "upstream_storage_unavailable", wantMinCooldown: 5 * time.Minute},
		{name: "no space left", statusCode: http.StatusBadRequest, body: []byte(`{"error":{"message":"write /tmp/output: no space left on device"}}`), wantMatched: true, wantReason: "upstream_storage_unavailable", wantMinCooldown: 5 * time.Minute},
		{name: "out of memory", statusCode: http.StatusBadRequest, body: []byte(`{"error":{"message":"worker failed: out of memory"}}`), wantMatched: true, wantReason: "upstream_memory_exhausted", wantMinCooldown: 5 * time.Minute},
		{name: "file descriptors exhausted", statusCode: http.StatusBadRequest, body: []byte(`{"error":{"message":"accept4: too many open files"}}`), wantMatched: true, wantReason: "upstream_file_descriptors_exhausted", wantMinCooldown: 5 * time.Minute},
		{name: "database pool exhausted", statusCode: http.StatusBadRequest, body: []byte(`{"error":{"message":"database connection pool exhausted"}}`), wantMatched: true, wantReason: "upstream_database_exhausted", wantMinCooldown: 5 * time.Minute},
		{name: "insufficient storage status", statusCode: http.StatusInsufficientStorage, body: []byte(`{"error":{"message":"insufficient storage"}}`), wantMatched: true, wantReason: "upstream_resource_exhausted", wantMinCooldown: 5 * time.Minute},
		{name: "generic 500", statusCode: http.StatusInternalServerError, body: []byte(`{"error":{"message":"internal server error"}}`), wantMatched: true, wantReason: "upstream_server_error"},
		{name: "generic 502", statusCode: http.StatusBadGateway, body: []byte(`{"error":{"message":"bad gateway"}}`), wantMatched: true, wantReason: "upstream_server_error"},
		{name: "generic 503", statusCode: http.StatusServiceUnavailable, body: []byte(`{"error":{"message":"service unavailable"}}`), wantMatched: true, wantReason: "upstream_server_error"},
		{name: "generic 504", statusCode: http.StatusGatewayTimeout, body: []byte(`{"error":{"message":"gateway timeout"}}`), wantMatched: true, wantReason: "upstream_server_error"},
		{name: "529 keeps overload path", statusCode: 529, body: []byte(`{"error":{"message":"overloaded"}}`)},
		{name: "ordinary bad request", statusCode: http.StatusBadRequest, body: []byte(`{"error":{"message":"invalid parameter: temperature"}}`)},
		{name: "context window", statusCode: http.StatusBadRequest, body: []byte(`{"error":{"message":"maximum context length exceeded"}}`)},
		{name: "model unsupported", statusCode: http.StatusBadRequest, body: []byte(`{"error":{"message":"The requested model is not supported with Codex when using a ChatGPT account"}}`)},
		{name: "previous response", statusCode: http.StatusBadRequest, body: []byte(`{"error":{"code":"previous_response_not_found","message":"previous response not found"}}`)},
		{name: "request too large", statusCode: http.StatusRequestEntityTooLarge, body: []byte(`{"error":{"message":"request too large"}}`)},
		{name: "unauthorized", statusCode: http.StatusUnauthorized, body: []byte(`{"error":{"message":"invalid api key"}}`)},
		{name: "forbidden", statusCode: http.StatusForbidden, body: []byte(`{"error":{"message":"forbidden"}}`)},
		{name: "rate limited", statusCode: http.StatusTooManyRequests, body: []byte(`{"error":{"message":"rate limit exceeded"}}`)},
		{name: "quota", statusCode: http.StatusBadRequest, body: []byte(`{"error":{"message":"insufficient quota"}}`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			failure, matched := classifyOpenAIUpstreamInfrastructureFailure(tt.statusCode, "", tt.body)

			require.Equal(t, tt.wantMatched, matched)
			if !tt.wantMatched {
				return
			}
			require.Equal(t, tt.wantReason, failure.Reason)
			require.Equal(t, tt.wantMinCooldown, failure.MinimumCooldown)
		})
	}
}

// TestRateLimitServiceHandleUpstreamStorageFailureUsesAccountCooldown 验证上游磁盘故障冷却整个账号，而不是依次禁用账号内的 Key。
func TestRateLimitServiceHandleUpstreamStorageFailureUsesAccountCooldown(t *testing.T) {
	account := &Account{
		ID:          62020,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{
			"api_keys":                     []any{"sk-storage-a", "sk-storage-b"},
			"pool_mode":                    true,
			"pool_mode_retry_status_codes": []any{float64(http.StatusBadRequest)},
		},
	}
	require.Equal(t, "sk-storage-a", account.GetAPIKey())

	repo := &rateLimitAccountRepoStub{account: account}
	blocker := &runtimeBlockRecorder{}
	service := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	service.SetAccountRuntimeBlocker(blocker)
	startedAt := time.Now()

	shouldDisable := service.HandleUpstreamError(
		context.Background(),
		account,
		http.StatusBadRequest,
		http.Header{},
		[]byte(upstreamDiskStorageFailureBody),
	)

	require.True(t, shouldDisable)
	require.Equal(t, 0, repo.updateCredentialsCalls, "共享存储故障不得污染单个 Key 状态")
	require.Equal(t, []string{"sk-storage-a", "sk-storage-b"}, account.GetAPIKeys())
	require.Equal(t, 1, repo.tempCalls)
	require.Equal(t, 0, repo.setErrorCalls)
	require.NotNil(t, account.TempUnschedulableUntil)
	require.WithinDuration(t, startedAt.Add(5*time.Minute), *account.TempUnschedulableUntil, 2*time.Second)
	require.Len(t, blocker.accounts, 1)
	require.Same(t, account, blocker.accounts[0])
	require.WithinDuration(t, *account.TempUnschedulableUntil, blocker.until[0], time.Second)
	require.Equal(t, "upstream_storage_unavailable", blocker.reasons[0])

	var state TempUnschedState
	require.NoError(t, json.Unmarshal([]byte(account.TempUnschedulableReason), &state))
	require.Equal(t, http.StatusBadRequest, state.StatusCode)
	require.Equal(t, "upstream_storage_unavailable", state.MatchedKeyword)
	require.Equal(t, 1, state.ErrorCount)
}

// TestRateLimitServiceHandleGenericUpstream5xxUsesAccountCooldown 验证通用 5xx 使用账号级渐进冷却，不轮换 Key。
func TestRateLimitServiceHandleGenericUpstream5xxUsesAccountCooldown(t *testing.T) {
	account := &Account{
		ID:          62022,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{
			"api_keys":                     []any{"sk-server-a", "sk-server-b"},
			"pool_mode":                    true,
			"pool_mode_retry_status_codes": []any{float64(http.StatusServiceUnavailable)},
		},
	}
	require.Equal(t, "sk-server-a", account.GetAPIKey())

	repo := &rateLimitAccountRepoStub{account: account}
	blocker := &runtimeBlockRecorder{}
	service := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	service.SetAccountRuntimeBlocker(blocker)
	startedAt := time.Now()

	shouldDisable := service.HandleUpstreamError(
		context.Background(),
		account,
		http.StatusServiceUnavailable,
		http.Header{},
		[]byte(`{"error":{"message":"upstream temporarily unavailable"}}`),
	)

	require.True(t, shouldDisable)
	require.Equal(t, 0, repo.updateCredentialsCalls)
	require.Equal(t, []string{"sk-server-a", "sk-server-b"}, account.GetAPIKeys())
	require.Equal(t, 1, repo.tempCalls)
	require.NotNil(t, account.TempUnschedulableUntil)
	require.WithinDuration(t, startedAt.Add(30*time.Second), *account.TempUnschedulableUntil, 2*time.Second)
	require.Len(t, blocker.accounts, 1)
	require.Equal(t, "upstream_server_error", blocker.reasons[0])

	var state TempUnschedState
	require.NoError(t, json.Unmarshal([]byte(account.TempUnschedulableReason), &state))
	require.Equal(t, http.StatusServiceUnavailable, state.StatusCode)
	require.Equal(t, "upstream_server_error", state.MatchedKeyword)
	require.Equal(t, 1, state.ErrorCount)
}

// TestRateLimitServiceHandleOAuthUpstream5xxUsesAccountCooldown 验证 OAuth 账号也复用同一账号级熔断。
func TestRateLimitServiceHandleOAuthUpstream5xxUsesAccountCooldown(t *testing.T) {
	account := &Account{
		ID:          62025,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
	}
	repo := &rateLimitAccountRepoStub{account: account}
	blocker := &runtimeBlockRecorder{}
	service := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	service.SetAccountRuntimeBlocker(blocker)

	shouldDisable := service.HandleUpstreamError(
		context.Background(),
		account,
		http.StatusBadGateway,
		http.Header{},
		[]byte(`{"error":{"message":"bad gateway"}}`),
	)

	require.True(t, shouldDisable)
	require.Equal(t, 1, repo.tempCalls)
	require.Equal(t, 0, repo.setErrorCalls)
	require.Len(t, blocker.accounts, 1)
	require.Equal(t, "upstream_server_error", blocker.reasons[0])
}

// TestRateLimitServiceOrdinaryBadRequestDoesNotTripInfrastructureCircuit 验证普通请求错误保持原行为。
func TestRateLimitServiceOrdinaryBadRequestDoesNotTripInfrastructureCircuit(t *testing.T) {
	account := &Account{
		ID:          62023,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{
			"api_keys": []any{"sk-normal-a", "sk-normal-b"},
		},
	}
	repo := &rateLimitAccountRepoStub{account: account}
	blocker := &runtimeBlockRecorder{}
	service := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	service.SetAccountRuntimeBlocker(blocker)

	shouldDisable := service.HandleUpstreamError(
		context.Background(),
		account,
		http.StatusBadRequest,
		http.Header{},
		[]byte(`{"error":{"message":"invalid parameter: temperature"}}`),
	)

	require.False(t, shouldDisable)
	require.Equal(t, 0, repo.tempCalls)
	require.Equal(t, 0, repo.updateCredentialsCalls)
	require.Empty(t, blocker.accounts)
	require.Nil(t, account.TempUnschedulableUntil)
}

// TestOpenAIHandleErrorResponseUpstreamStorageFailureTriggersFailover 验证首次磁盘故障立即切号，并阻止后续调度再次选择当前账号。
func TestOpenAIHandleErrorResponseUpstreamStorageFailureTriggersFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)

	account := &Account{
		ID:          62021,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{
			"api_keys":                     []any{"sk-storage-a", "sk-storage-b"},
			"pool_mode":                    true,
			"pool_mode_retry_status_codes": []any{float64(http.StatusBadRequest)},
		},
	}
	repo := &rateLimitAccountRepoStub{account: account}
	rateLimitService := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	service := &OpenAIGatewayService{
		accountRepo:      repo,
		rateLimitService: rateLimitService,
	}
	rateLimitService.SetAccountRuntimeBlocker(service)

	responseBody := []byte(upstreamDiskStorageFailureBody)
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(responseBody)),
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	requestBody := []byte(`{"model":"gpt-5.6-sol","input":"hello"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", bytes.NewReader(requestBody))

	result, err := service.handleErrorResponse(context.Background(), resp, c, account, requestBody, "gpt-5.6-sol")

	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadRequest, failoverErr.StatusCode)
	require.JSONEq(t, string(responseBody), string(failoverErr.ResponseBody))
	require.False(t, failoverErr.RetryableOnSameAccount)
	require.False(t, c.Writer.Written(), "切号前不得向客户端提交错误响应")
	require.True(t, service.isOpenAIAccountRuntimeBlocked(account))
	require.Equal(t, 1, repo.tempCalls)
	require.Equal(t, 0, repo.updateCredentialsCalls)
}

// TestOpenAIHandleErrorResponseGeneric5xxTriggersAccountFailover 验证通用 5xx 首次命中后立即切号并阻断账号。
func TestOpenAIHandleErrorResponseGeneric5xxTriggersAccountFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)

	account := &Account{
		ID:          62024,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{
			"api_keys":                     []any{"sk-server-a", "sk-server-b"},
			"pool_mode":                    true,
			"pool_mode_retry_status_codes": []any{float64(http.StatusServiceUnavailable)},
		},
	}
	repo := &rateLimitAccountRepoStub{account: account}
	rateLimitService := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	service := &OpenAIGatewayService{
		accountRepo:      repo,
		rateLimitService: rateLimitService,
	}
	rateLimitService.SetAccountRuntimeBlocker(service)

	responseBody := []byte(`{"error":{"message":"upstream temporarily unavailable"}}`)
	resp := &http.Response{
		StatusCode: http.StatusServiceUnavailable,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(responseBody)),
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	requestBody := []byte(`{"model":"gpt-5.6-sol","input":"hello"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", bytes.NewReader(requestBody))

	result, err := service.handleErrorResponse(context.Background(), resp, c, account, requestBody, "gpt-5.6-sol")

	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusServiceUnavailable, failoverErr.StatusCode)
	require.False(t, failoverErr.RetryableOnSameAccount)
	require.False(t, c.Writer.Written())
	require.True(t, service.isOpenAIAccountRuntimeBlocked(account))
	require.Equal(t, 1, repo.tempCalls)
	require.Equal(t, 0, repo.updateCredentialsCalls)
}

// TestOpenAIPoolModeInfrastructureFailureNeverRetriesSameAccount 验证池模式配置不会覆盖基础设施熔断。
func TestOpenAIPoolModeInfrastructureFailureNeverRetriesSameAccount(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"pool_mode":                    true,
			"pool_mode_retry_status_codes": []any{float64(http.StatusBadRequest), float64(http.StatusServiceUnavailable)},
		},
	}

	require.False(t, isOpenAIPoolModeRetryableOnSameAccount(account, http.StatusBadRequest, "", []byte(upstreamDiskStorageFailureBody)))
	require.False(t, isOpenAIPoolModeRetryableOnSameAccount(account, http.StatusServiceUnavailable, "service unavailable", []byte(`{"error":{"message":"service unavailable"}}`)))
	require.True(t, isOpenAIPoolModeRetryableOnSameAccount(account, http.StatusBadRequest, "invalid parameter", []byte(`{"error":{"message":"invalid parameter"}}`)))
}

// TestOpenAIPassthroughInfrastructureFailureTriggersFailover 验证透传模式也会拦截基础设施故障。
func TestOpenAIPassthroughInfrastructureFailureTriggersFailover(t *testing.T) {
	require.True(t, shouldFailoverOpenAIPassthroughResponse(http.StatusBadRequest, "", []byte(upstreamDiskStorageFailureBody)))
	require.True(t, shouldFailoverOpenAIPassthroughResponse(http.StatusServiceUnavailable, "service unavailable", nil))
	require.False(t, shouldFailoverOpenAIPassthroughResponse(http.StatusBadRequest, "invalid parameter", nil))
	require.False(t, shouldFailoverOpenAIPassthroughResponse(http.StatusBadGateway, "", []byte(`{"error":{"message":"maximum context length exceeded"}}`)))
}

// TestOpenAITempUnschedulerDoesNotShortenInfrastructureCooldown 验证 handler 不会用 30 秒覆盖已写入的资源故障冷却。
func TestOpenAITempUnschedulerDoesNotShortenInfrastructureCooldown(t *testing.T) {
	account := &Account{
		ID:          62026,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{"api_keys": []any{"sk-storage"}},
	}
	repo := &rateLimitAccountRepoStub{account: account}
	rateLimitService := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	service := &OpenAIGatewayService{accountRepo: repo, rateLimitService: rateLimitService}
	rateLimitService.SetAccountRuntimeBlocker(service)

	require.True(t, rateLimitService.HandleUpstreamError(
		context.Background(), account, http.StatusInsufficientStorage, http.Header{}, []byte(`{"error":{"message":"insufficient storage"}}`),
	))
	require.Equal(t, 1, repo.tempCalls)
	before := *account.TempUnschedulableUntil

	service.TempUnscheduleRetryableError(context.Background(), account.ID, &UpstreamFailoverError{
		StatusCode:   http.StatusInsufficientStorage,
		ResponseBody: []byte(`{"error":{"message":"insufficient storage"}}`),
	})

	require.Equal(t, 1, repo.tempCalls)
	require.Equal(t, before, *account.TempUnschedulableUntil)
}
