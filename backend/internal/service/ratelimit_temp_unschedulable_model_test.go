package service

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type tempUnschedulableModelCall struct {
	accountID int64
	model     string
	resetAt   time.Time
}

// tempUnschedulableModelRepoStub 记录账号级和模型级冷却，验证规则命中范围。
type tempUnschedulableModelRepoStub struct {
	mockAccountRepoForGemini
	account           *Account
	tempCalls         int
	modelCalls        []tempUnschedulableModelCall
	modelRateLimitErr error
}

type tempUnschedulableRuntimeBlockRecorder struct {
	accounts []*Account
}

func (r *tempUnschedulableRuntimeBlockRecorder) BlockAccountScheduling(account *Account, _ time.Time, _ string) {
	r.accounts = append(r.accounts, account)
}

func (r *tempUnschedulableRuntimeBlockRecorder) ClearAccountSchedulingBlock(_ int64) {}

func (r *tempUnschedulableModelRepoStub) GetByID(_ context.Context, id int64) (*Account, error) {
	if r.account != nil && r.account.ID == id {
		return r.account, nil
	}
	return nil, nil
}

func (r *tempUnschedulableModelRepoStub) SetTempUnschedulable(_ context.Context, _ int64, _ time.Time, _ string) error {
	r.tempCalls++
	return nil
}

func (r *tempUnschedulableModelRepoStub) SetModelRateLimit(_ context.Context, id int64, model string, resetAt time.Time) error {
	r.modelCalls = append(r.modelCalls, tempUnschedulableModelCall{accountID: id, model: model, resetAt: resetAt})
	return r.modelRateLimitErr
}

func newTempUnschedulableRuleAccount(poolMode bool, statusCode int, keyword string) *Account {
	return &Account{
		ID:          63001,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{
			"api_keys":                   []any{"sk-temp-a", "sk-temp-b"},
			"pool_mode":                  poolMode,
			"temp_unschedulable_enabled": true,
			"temp_unschedulable_rules": []any{
				map[string]any{
					"error_code":       float64(statusCode),
					"keywords":         []any{keyword},
					"duration_minutes": float64(10),
				},
			},
		},
	}
}

func TestCheckErrorPolicy_PoolModeHonorsNon401TempRulePerModel(t *testing.T) {
	account := newTempUnschedulableRuleAccount(true, http.StatusServiceUnavailable, "capacity unavailable")
	repo := &tempUnschedulableModelRepoStub{account: account}
	service := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)

	result := service.CheckErrorPolicy(
		context.Background(),
		account,
		http.StatusServiceUnavailable,
		[]byte(`{"error":{"message":"capacity unavailable"}}`),
		"gpt-5.4",
	)

	require.Equal(t, ErrorPolicyTempUnscheduled, result)
	require.Zero(t, repo.tempCalls)
	require.Len(t, repo.modelCalls, 1)
	require.Equal(t, "gpt-5.4", repo.modelCalls[0].model)
}

func TestHandleUpstreamError_PoolModeTempRulePrecedesInfrastructureCooldown(t *testing.T) {
	account := newTempUnschedulableRuleAccount(true, http.StatusServiceUnavailable, "capacity unavailable")
	repo := &tempUnschedulableModelRepoStub{account: account}
	blocker := &tempUnschedulableRuntimeBlockRecorder{}
	service := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	service.SetAccountRuntimeBlocker(blocker)

	shouldDisable := service.HandleUpstreamError(
		context.Background(),
		account,
		http.StatusServiceUnavailable,
		http.Header{},
		[]byte(`{"error":{"message":"capacity unavailable"}}`),
		"gpt-5.4",
	)

	require.True(t, shouldDisable)
	require.Zero(t, repo.tempCalls, "显式模型规则命中后不得扩大为账号级基础设施冷却")
	require.Len(t, repo.modelCalls, 1)
	require.Empty(t, blocker.accounts)
}

func TestHandleUpstreamError_UnifiedTempRuleUsesModelScope(t *testing.T) {
	account := &Account{
		ID:          63002,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{
			"error_handling_rules": []any{
				map[string]any{
					"enabled":         true,
					"name":            "model capacity",
					"priority":        float64(1),
					"action":          "temp_unschedulable",
					"status_codes":    []any{float64(http.StatusServiceUnavailable)},
					"keywords":        []any{"capacity unavailable"},
					"durationMinutes": float64(10),
				},
			},
		},
	}
	repo := &tempUnschedulableModelRepoStub{account: account}
	service := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)

	shouldDisable := service.HandleUpstreamError(
		context.Background(),
		account,
		http.StatusServiceUnavailable,
		http.Header{},
		[]byte(`{"error":{"message":"capacity unavailable"}}`),
		"gpt-5.4",
	)

	require.True(t, shouldDisable)
	require.Zero(t, repo.tempCalls)
	require.Len(t, repo.modelCalls, 1)
	require.Equal(t, "gpt-5.4", repo.modelCalls[0].model)
}

func TestCheckErrorPolicy_PoolModeKeeps401Semantics(t *testing.T) {
	account := newTempUnschedulableRuleAccount(true, http.StatusUnauthorized, "unauthorized")
	repo := &tempUnschedulableModelRepoStub{account: account}
	service := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)

	result := service.CheckErrorPolicy(
		context.Background(),
		account,
		http.StatusUnauthorized,
		[]byte(`{"error":{"message":"unauthorized"}}`),
		"gpt-5.4",
	)

	require.Equal(t, ErrorPolicySkipped, result)
	require.Zero(t, repo.tempCalls)
	require.Empty(t, repo.modelCalls)
}

func TestOpenAITempRule_KnownModelDoesNotBlockWholeAccount(t *testing.T) {
	account := newTempUnschedulableRuleAccount(false, http.StatusForbidden, "endpoint blocked")
	repo := &tempUnschedulableModelRepoStub{account: account}
	rateLimitService := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	gateway := &OpenAIGatewayService{accountRepo: repo, rateLimitService: rateLimitService}
	rateLimitService.SetAccountRuntimeBlocker(gateway)

	shouldDisable := gateway.handleOpenAIAccountUpstreamError(
		context.Background(),
		account,
		http.StatusForbidden,
		http.Header{},
		[]byte(`{"error":{"message":"endpoint blocked"}}`),
		"gpt-5.4",
	)

	require.True(t, shouldDisable)
	require.Zero(t, repo.tempCalls)
	require.Len(t, repo.modelCalls, 1)
	require.False(t, gateway.isOpenAIAccountRuntimeBlocked(account))
}

func TestOpenAITempRule_UnknownModelKeepsAccountBlock(t *testing.T) {
	account := newTempUnschedulableRuleAccount(false, http.StatusForbidden, "endpoint blocked")
	repo := &tempUnschedulableModelRepoStub{account: account}
	rateLimitService := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	gateway := &OpenAIGatewayService{accountRepo: repo, rateLimitService: rateLimitService}
	rateLimitService.SetAccountRuntimeBlocker(gateway)

	shouldDisable := gateway.handleOpenAIAccountUpstreamError(
		context.Background(),
		account,
		http.StatusForbidden,
		http.Header{},
		[]byte(`{"error":{"message":"endpoint blocked"}}`),
	)

	require.True(t, shouldDisable)
	require.Equal(t, 1, repo.tempCalls)
	require.Empty(t, repo.modelCalls)
	require.True(t, gateway.isOpenAIAccountRuntimeBlocked(account))
}

func TestOpenAITempRule_ModelWriteFailureDoesNotWidenScope(t *testing.T) {
	account := newTempUnschedulableRuleAccount(false, http.StatusForbidden, "endpoint blocked")
	repo := &tempUnschedulableModelRepoStub{account: account, modelRateLimitErr: errors.New("write failed")}
	rateLimitService := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	gateway := &OpenAIGatewayService{accountRepo: repo, rateLimitService: rateLimitService}
	rateLimitService.SetAccountRuntimeBlocker(gateway)

	shouldDisable := gateway.handleOpenAIAccountUpstreamError(
		context.Background(),
		account,
		http.StatusForbidden,
		http.Header{},
		[]byte(`{"error":{"message":"endpoint blocked"}}`),
		"gpt-5.4",
	)

	require.True(t, shouldDisable)
	require.Zero(t, repo.tempCalls)
	require.Len(t, repo.modelCalls, 1)
	require.False(t, gateway.isOpenAIAccountRuntimeBlocked(account))
}
