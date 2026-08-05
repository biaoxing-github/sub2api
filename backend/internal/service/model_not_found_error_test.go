package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// modelNotFoundRateLimitCall 记录一次模型级冷却写入，便于断言账号 ID、模型 key 与过期时间。
type modelNotFoundRateLimitCall struct {
	accountID int64
	scope     string
	resetAt   time.Time
}

// modelNotFoundAccountRepoStub 是只服务本文件模型不存在冷却测试的 AccountRepository 桩。
type modelNotFoundAccountRepoStub struct {
	modelRateLimitCalls []modelNotFoundRateLimitCall
}

func (s *modelNotFoundAccountRepoStub) Create(ctx context.Context, account *Account) error {
	panic("unexpected Create call")
}

func (s *modelNotFoundAccountRepoStub) GetByID(ctx context.Context, id int64) (*Account, error) {
	panic("unexpected GetByID call")
}

func (s *modelNotFoundAccountRepoStub) GetByIDs(ctx context.Context, ids []int64) ([]*Account, error) {
	panic("unexpected GetByIDs call")
}

func (s *modelNotFoundAccountRepoStub) ExistsByID(ctx context.Context, id int64) (bool, error) {
	panic("unexpected ExistsByID call")
}

func (s *modelNotFoundAccountRepoStub) GetByCRSAccountID(ctx context.Context, crsAccountID string) (*Account, error) {
	panic("unexpected GetByCRSAccountID call")
}

func (s *modelNotFoundAccountRepoStub) FindByExtraField(ctx context.Context, key string, value any) ([]Account, error) {
	panic("unexpected FindByExtraField call")
}

func (s *modelNotFoundAccountRepoStub) ListCRSAccountIDs(ctx context.Context) (map[string]int64, error) {
	panic("unexpected ListCRSAccountIDs call")
}

func (s *modelNotFoundAccountRepoStub) Update(ctx context.Context, account *Account) error {
	panic("unexpected Update call")
}

func (s *modelNotFoundAccountRepoStub) Delete(ctx context.Context, id int64) error {
	panic("unexpected Delete call")
}

func (s *modelNotFoundAccountRepoStub) List(ctx context.Context, params pagination.PaginationParams) ([]Account, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}

func (s *modelNotFoundAccountRepoStub) ListWithFilters(ctx context.Context, params pagination.PaginationParams, platform, accountType, status, search string, groupID int64, privacyMode, planType string) ([]Account, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}

func (s *modelNotFoundAccountRepoStub) ListByGroup(ctx context.Context, groupID int64) ([]Account, error) {
	panic("unexpected ListByGroup call")
}

func (s *modelNotFoundAccountRepoStub) ListActive(ctx context.Context) ([]Account, error) {
	panic("unexpected ListActive call")
}

func (s *modelNotFoundAccountRepoStub) ListByPlatform(ctx context.Context, platform string) ([]Account, error) {
	panic("unexpected ListByPlatform call")
}

func (s *modelNotFoundAccountRepoStub) UpdateLastUsed(ctx context.Context, id int64) error {
	panic("unexpected UpdateLastUsed call")
}

func (s *modelNotFoundAccountRepoStub) BatchUpdateLastUsed(ctx context.Context, updates map[int64]time.Time) error {
	panic("unexpected BatchUpdateLastUsed call")
}

func (s *modelNotFoundAccountRepoStub) SetError(ctx context.Context, id int64, errorMsg string) error {
	panic("unexpected SetError call")
}

func (s *modelNotFoundAccountRepoStub) ClearError(ctx context.Context, id int64) error {
	panic("unexpected ClearError call")
}

func (s *modelNotFoundAccountRepoStub) SetSchedulable(ctx context.Context, id int64, schedulable bool) error {
	panic("unexpected SetSchedulable call")
}

func (s *modelNotFoundAccountRepoStub) AutoPauseExpiredAccounts(ctx context.Context, now time.Time) (int64, error) {
	panic("unexpected AutoPauseExpiredAccounts call")
}

func (s *modelNotFoundAccountRepoStub) BindGroups(ctx context.Context, accountID int64, groupIDs []int64) error {
	panic("unexpected BindGroups call")
}

func (s *modelNotFoundAccountRepoStub) ListSchedulable(ctx context.Context) ([]Account, error) {
	panic("unexpected ListSchedulable call")
}

func (s *modelNotFoundAccountRepoStub) ListSchedulableByGroupID(ctx context.Context, groupID int64) ([]Account, error) {
	panic("unexpected ListSchedulableByGroupID call")
}

func (s *modelNotFoundAccountRepoStub) ListSchedulableByPlatform(ctx context.Context, platform string) ([]Account, error) {
	panic("unexpected ListSchedulableByPlatform call")
}

func (s *modelNotFoundAccountRepoStub) ListSchedulableByGroupIDAndPlatform(ctx context.Context, groupID int64, platform string) ([]Account, error) {
	panic("unexpected ListSchedulableByGroupIDAndPlatform call")
}

func (s *modelNotFoundAccountRepoStub) ListSchedulableByPlatforms(ctx context.Context, platforms []string) ([]Account, error) {
	panic("unexpected ListSchedulableByPlatforms call")
}

func (s *modelNotFoundAccountRepoStub) ListSchedulableByGroupIDAndPlatforms(ctx context.Context, groupID int64, platforms []string) ([]Account, error) {
	panic("unexpected ListSchedulableByGroupIDAndPlatforms call")
}

func (s *modelNotFoundAccountRepoStub) ListSchedulableUngroupedByPlatform(ctx context.Context, platform string) ([]Account, error) {
	panic("unexpected ListSchedulableUngroupedByPlatform call")
}

func (s *modelNotFoundAccountRepoStub) ListSchedulableUngroupedByPlatforms(ctx context.Context, platforms []string) ([]Account, error) {
	panic("unexpected ListSchedulableUngroupedByPlatforms call")
}

func (s *modelNotFoundAccountRepoStub) SetRateLimited(ctx context.Context, id int64, resetAt time.Time) error {
	panic("unexpected SetRateLimited call")
}

// SetModelRateLimit 记录模型级冷却写入，测试不会访问真实数据库。
func (s *modelNotFoundAccountRepoStub) SetModelRateLimit(ctx context.Context, id int64, scope string, resetAt time.Time) error {
	s.modelRateLimitCalls = append(s.modelRateLimitCalls, modelNotFoundRateLimitCall{
		accountID: id,
		scope:     scope,
		resetAt:   resetAt,
	})
	return nil
}

func (s *modelNotFoundAccountRepoStub) SetOverloaded(ctx context.Context, id int64, until time.Time) error {
	panic("unexpected SetOverloaded call")
}

func (s *modelNotFoundAccountRepoStub) SetTempUnschedulable(ctx context.Context, id int64, until time.Time, reason string) error {
	panic("unexpected SetTempUnschedulable call")
}

func (s *modelNotFoundAccountRepoStub) ClearTempUnschedulable(ctx context.Context, id int64) error {
	panic("unexpected ClearTempUnschedulable call")
}

func (s *modelNotFoundAccountRepoStub) ClearRateLimit(ctx context.Context, id int64) error {
	panic("unexpected ClearRateLimit call")
}

func (s *modelNotFoundAccountRepoStub) ClearAntigravityQuotaScopes(ctx context.Context, id int64) error {
	panic("unexpected ClearAntigravityQuotaScopes call")
}

func (s *modelNotFoundAccountRepoStub) ClearModelRateLimits(ctx context.Context, id int64) error {
	panic("unexpected ClearModelRateLimits call")
}

func (s *modelNotFoundAccountRepoStub) UpdateSessionWindow(ctx context.Context, id int64, start, end *time.Time, status string) error {
	panic("unexpected UpdateSessionWindow call")
}

func (s *modelNotFoundAccountRepoStub) UpdateExtra(ctx context.Context, id int64, updates map[string]any) error {
	panic("unexpected UpdateExtra call")
}

func (s *modelNotFoundAccountRepoStub) BulkUpdate(ctx context.Context, ids []int64, updates AccountBulkUpdate) (int64, error) {
	panic("unexpected BulkUpdate call")
}

func (s *modelNotFoundAccountRepoStub) IncrementQuotaUsed(ctx context.Context, id int64, amount float64) error {
	panic("unexpected IncrementQuotaUsed call")
}

func (s *modelNotFoundAccountRepoStub) ResetQuotaUsed(ctx context.Context, id int64) error {
	panic("unexpected ResetQuotaUsed call")
}

var _ AccountRepository = (*modelNotFoundAccountRepoStub)(nil)

func TestIsOpenAIModelNotFoundError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       []byte
		want       bool
	}{
		{
			name:       "400 model_not_found code",
			statusCode: http.StatusBadRequest,
			body:       []byte(`{"error":{"message":"The model gpt-missing does not exist or you do not have access to it.","type":"invalid_request_error","code":"model_not_found"}}`),
			want:       true,
		},
		{
			name:       "400 unknown model message",
			statusCode: http.StatusBadRequest,
			body:       []byte(`{"error":{"message":"Unknown model: gpt-ghost","type":"invalid_request_error"}}`),
			want:       true,
		},
		{
			name:       "400 ChatGPT account does not support Codex model",
			statusCode: http.StatusBadRequest,
			body:       []byte(`{"error":{"code":"bad_response_status_code","message":"The 'gpt-5.6-sol' model is not supported when using Codex with a ChatGPT account."}}`),
			want:       true,
		},
		{
			name:       "400 unsupported parameter is not a model error",
			statusCode: http.StatusBadRequest,
			body:       []byte(`{"error":{"message":"Unsupported parameter: temperature","type":"invalid_request_error"}}`),
			want:       false,
		},
		{
			name:       "404 model not found message",
			statusCode: http.StatusNotFound,
			body:       []byte(`{"error":{"message":"model not found: gpt-ghost","type":"not_found_error"}}`),
			want:       true,
		},
		{
			name:       "404 unknown model message",
			statusCode: http.StatusNotFound,
			body:       []byte(`{"error":{"message":"Unknown model: gpt-ghost","type":"invalid_request_error","code":"unknown_model"}}`),
			want:       true,
		},
		{
			name:       "404 plain route not found",
			statusCode: http.StatusNotFound,
			body:       []byte(`{"error":{"message":"Not Found","type":"invalid_request_error","code":"not_found"}}`),
			want:       false,
		},
		{
			name:       "500 model text does not count",
			statusCode: http.StatusInternalServerError,
			body:       []byte(`{"error":{"message":"model not found","code":"model_not_found"}}`),
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isOpenAIModelNotFoundError(tt.statusCode, tt.body))
		})
	}
}

func TestOpenAIHandleErrorResponse_ChatGPTAccountUnsupportedModelTriggersFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const (
		requestedModel = "gpt-5.5"
		upstreamModel  = "gpt-5.6-sol"
	)
	repo := &modelNotFoundAccountRepoStub{}
	service := &OpenAIGatewayService{accountRepo: repo}
	account := &Account{
		ID:       480,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"model_mapping":                map[string]any{requestedModel: upstreamModel},
			"pool_mode":                    true,
			"pool_mode_retry_status_codes": []any{float64(http.StatusBadRequest)},
		},
		Extra: map[string]any{},
	}
	responseBody := []byte(`{"error":{"code":"bad_response_status_code","message":"The 'gpt-5.6-sol' model is not supported when using Codex with a ChatGPT account."}}`)
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(responseBody)),
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	requestBody := []byte(`{"model":"gpt-5.5","input":"hello"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", bytes.NewReader(requestBody))

	result, err := service.handleErrorResponse(context.Background(), resp, c, account, requestBody, requestedModel)

	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadRequest, failoverErr.StatusCode)
	require.JSONEq(t, string(responseBody), string(failoverErr.ResponseBody))
	require.False(t, failoverErr.RetryableOnSameAccount)
	require.Equal(t, OpenAIStreamActionRetryNextAccount, failoverErr.ActionLabel)
	require.False(t, c.Writer.Written(), "failover must not commit a direct 502 response")
	require.False(t, service.isOpenAIAccountRuntimeBlocked(account))
	require.Len(t, repo.modelRateLimitCalls, 1)
	require.Equal(t, int64(480), repo.modelRateLimitCalls[0].accountID)
	require.Equal(t, upstreamModel, repo.modelRateLimitCalls[0].scope)
}

func TestOpenAIHandleErrorResponse_UnsupportedParameterFailsOverForNon2xx(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &modelNotFoundAccountRepoStub{}
	service := &OpenAIGatewayService{accountRepo: repo}
	account := &Account{
		ID:       481,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Extra:    map[string]any{},
	}
	responseBody := []byte(`{"error":{"message":"Unsupported parameter: temperature","type":"invalid_request_error"}}`)
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(responseBody)),
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	requestBody := []byte(`{"model":"gpt-5.6-sol","input":"hello","temperature":0.5}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", bytes.NewReader(requestBody))

	result, err := service.handleErrorResponse(context.Background(), resp, c, account, requestBody, "gpt-5.6-sol")

	require.Nil(t, result)
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadRequest, failoverErr.StatusCode)
	require.False(t, c.Writer.Written(), "切号前不得向客户端提交 400 响应")
	require.Empty(t, repo.modelRateLimitCalls)
}

func TestHandleOpenAIAccountUpstreamErrorForModel_ModelNotFoundSetsModelCooldownWithoutFailover(t *testing.T) {
	repo := &modelNotFoundAccountRepoStub{}
	service := &OpenAIGatewayService{accountRepo: repo}
	account := &Account{
		ID:       42,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra:    map[string]any{},
	}
	body := []byte(`{"error":{"message":"The model gpt-missing does not exist or you do not have access to it.","code":"model_not_found"}}`)

	before := time.Now().Add(openAIModelNotFoundCooldown - time.Minute)
	shouldFailover := service.handleOpenAIAccountUpstreamErrorForModel(context.Background(), account, http.StatusNotFound, http.Header{}, body, "gpt-missing")
	after := time.Now().Add(openAIModelNotFoundCooldown + time.Minute)

	require.False(t, shouldFailover)
	require.Len(t, repo.modelRateLimitCalls, 1)
	require.Equal(t, int64(42), repo.modelRateLimitCalls[0].accountID)
	require.Equal(t, "gpt-missing", repo.modelRateLimitCalls[0].scope)
	require.True(t, repo.modelRateLimitCalls[0].resetAt.After(before))
	require.True(t, repo.modelRateLimitCalls[0].resetAt.Before(after))

	limits, ok := account.Extra[modelRateLimitsKey].(map[string]any)
	require.True(t, ok)
	limit, ok := limits["gpt-missing"].(map[string]any)
	require.True(t, ok)
	resetAt, ok := limit["rate_limit_reset_at"].(string)
	require.True(t, ok)
	parsedResetAt, err := time.Parse(time.RFC3339, resetAt)
	require.NoError(t, err)
	require.True(t, parsedResetAt.After(before))
	require.True(t, parsedResetAt.Before(after))
}

func TestIsOpenAIPoolModeRetryableOnSameAccount_ModelNotFoundIsNeverSameAccountRetryable(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"pool_mode":                    true,
			"pool_mode_retry_status_codes": []any{float64(http.StatusNotFound)},
		},
	}
	body := []byte(`{"error":{"message":"Unknown model: gpt-ghost","code":"model_not_found"}}`)

	require.True(t, account.IsPoolMode())
	require.True(t, account.IsPoolModeRetryableStatus(http.StatusNotFound))
	require.False(t, isOpenAIPoolModeRetryableOnSameAccount(account, http.StatusNotFound, "model_not_found", body))
}
