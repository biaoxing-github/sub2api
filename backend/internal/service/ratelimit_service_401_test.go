//go:build unit

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

type rateLimitAccountRepoStub struct {
	mockAccountRepoForGemini
	account                *Account
	setErrorCalls          int
	clearErrorCalls        int
	clearRateLimitCalls    int
	clearTempUnschedCalls  int
	setSchedulableCalls    int
	tempCalls              int
	rateLimitedCalls       int
	updateCredentialsCalls int
	updateExtraCalls       int
	lastCredentials        map[string]any
	lastExtraUpdates       map[string]any
	lastErrorMsg           string
	lastTempReason         string
	lastRateLimitedUntil   *time.Time
	lastSchedulableValue   bool
}

func (r *rateLimitAccountRepoStub) GetByID(ctx context.Context, id int64) (*Account, error) {
	if r.account != nil && r.account.ID == id {
		return r.account, nil
	}
	return r.mockAccountRepoForGemini.GetByID(ctx, id)
}

func (r *rateLimitAccountRepoStub) SetError(ctx context.Context, id int64, errorMsg string) error {
	r.setErrorCalls++
	r.lastErrorMsg = errorMsg
	return nil
}

func (r *rateLimitAccountRepoStub) ClearError(ctx context.Context, id int64) error {
	r.clearErrorCalls++
	if r.account != nil {
		r.account.Status = StatusActive
		r.account.ErrorMessage = ""
	}
	return nil
}

func (r *rateLimitAccountRepoStub) ClearRateLimit(ctx context.Context, id int64) error {
	r.clearRateLimitCalls++
	return nil
}

func (r *rateLimitAccountRepoStub) ClearTempUnschedulable(ctx context.Context, id int64) error {
	r.clearTempUnschedCalls++
	if r.account != nil {
		r.account.TempUnschedulableUntil = nil
		r.account.TempUnschedulableReason = ""
	}
	return nil
}

func (r *rateLimitAccountRepoStub) SetSchedulable(ctx context.Context, id int64, schedulable bool) error {
	r.setSchedulableCalls++
	r.lastSchedulableValue = schedulable
	if r.account != nil {
		r.account.Schedulable = schedulable
	}
	return nil
}

func (r *rateLimitAccountRepoStub) SetTempUnschedulable(ctx context.Context, id int64, until time.Time, reason string) error {
	r.tempCalls++
	r.lastTempReason = reason
	return nil
}

func (r *rateLimitAccountRepoStub) SetRateLimited(ctx context.Context, id int64, resetAt time.Time) error {
	r.rateLimitedCalls++
	r.lastRateLimitedUntil = &resetAt
	return nil
}

func (r *rateLimitAccountRepoStub) UpdateCredentials(ctx context.Context, id int64, credentials map[string]any) error {
	r.updateCredentialsCalls++
	r.lastCredentials = cloneCredentials(credentials)
	return nil
}

func (r *rateLimitAccountRepoStub) UpdateExtra(ctx context.Context, id int64, updates map[string]any) error {
	r.updateExtraCalls++
	r.lastExtraUpdates = cloneCredentials(updates)
	return nil
}

func TestRateLimitService_HandleUpstreamError_OpenAIAPIKey429DisablesLastActiveKeyWithoutFreezingAccount(t *testing.T) {
	account := &Account{
		ID:          62001,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{
			"api_keys": []any{"cooling-key", "last-key"},
		},
	}
	require.True(t, account.DisableAPIKey("cooling-key", "rate_limited", time.Now()))
	require.Equal(t, "last-key", account.GetAPIKey())
	repo := &rateLimitAccountRepoStub{account: account}
	service := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)

	shouldDisable := service.HandleUpstreamError(context.Background(), account, http.StatusTooManyRequests, http.Header{}, []byte(`{"error":{"code":"rate_limited","message":"too many requests"}}`))

	require.True(t, shouldDisable)
	require.Empty(t, account.GetAPIKeys())
	require.Equal(t, 1, repo.updateCredentialsCalls)
	disabled, _ := repo.lastCredentials[CredentialAPIKeysDisabled].(map[string]any)
	record, _ := disabled[FingerprintAPIKey("last-key")].(map[string]any)
	require.NotNil(t, record)
	require.Contains(t, record["last_error"], "API returned 429")
	require.Contains(t, record["last_error"], "too many requests")
	require.Equal(t, 0, repo.setErrorCalls)
	require.Equal(t, 0, repo.rateLimitedCalls)
	require.Equal(t, 0, repo.tempCalls)
	require.Equal(t, 0, repo.updateExtraCalls)
}

func TestRateLimitService_RecordAccountProbeOutcomeManualFailureWritesHealth(t *testing.T) {
	account := &Account{
		ID:          62002,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{
			"api_keys": []any{"key-a"},
		},
	}
	repo := &rateLimitAccountRepoStub{account: account}
	service := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	latencyMs := 1234
	firstTokenMs := 456

	transition, err := service.RecordAccountProbeOutcome(context.Background(), AccountProbeOutcome{
		AccountID:    account.ID,
		Account:      account,
		Source:       AccountProbeOutcomeSourceManualTest,
		Success:      false,
		HTTPStatus:   http.StatusPaymentRequired,
		Reason:       "payment_required",
		ErrorMessage: "insufficient balance",
		LatencyMs:    &latencyMs,
		FirstTokenMs: &firstTokenMs,
	})

	require.NoError(t, err)
	require.Equal(t, AccountProbeHealthNormal, transition.PreviousLevel)
	require.Equal(t, AccountProbeHealthQuotaExhausted, transition.NextLevel)
	require.True(t, transition.StateChanged)
	require.NotNil(t, transition.BlockedUntil)
	require.Equal(t, 1, repo.tempCalls)
	require.Equal(t, 1, repo.updateExtraCalls)
	health, ok := repo.lastExtraUpdates[AccountProbeHealthExtraKey].(map[string]any)
	require.True(t, ok)
	require.Equal(t, AccountProbeHealthQuotaExhausted, health["level"])
	require.Equal(t, 1, health["failure_count"])
	require.Equal(t, float64(http.StatusPaymentRequired), health["http_status"])
	require.Equal(t, latencyMs, health["latency_ms"])
	require.Equal(t, firstTokenMs, health["first_token_ms"])
}

func TestRateLimitService_RecordAccountProbeOutcomeManualSuccessRestoresSchedulableAndHealthy(t *testing.T) {
	until := time.Now().Add(10 * time.Minute)
	account := &Account{
		ID:                      62003,
		Platform:                PlatformOpenAI,
		Type:                    AccountTypeOAuth,
		Status:                  StatusError,
		ErrorMessage:            "upstream failed",
		Schedulable:             false,
		TempUnschedulableUntil:  &until,
		TempUnschedulableReason: "upstream_5xx",
		Extra: map[string]any{
			AccountProbeHealthExtraKey: map[string]any{
				"level":         AccountProbeHealthTempUnsched,
				"failure_count": 3,
				"last_error":    "upstream failed",
			},
		},
	}
	repo := &rateLimitAccountRepoStub{account: account}
	service := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)

	transition, err := service.RecordAccountProbeOutcome(context.Background(), AccountProbeOutcome{
		AccountID: account.ID,
		Account:   account,
		Source:    AccountProbeOutcomeSourceManualTest,
		Success:   true,
		Reason:    "probe_success",
	})

	require.NoError(t, err)
	require.Equal(t, AccountProbeHealthTempUnsched, transition.PreviousLevel)
	require.Equal(t, AccountProbeHealthNormal, transition.NextLevel)
	require.True(t, transition.Restored)
	require.Equal(t, 1, repo.clearErrorCalls)
	require.Equal(t, 1, repo.clearRateLimitCalls)
	require.Equal(t, 1, repo.clearTempUnschedCalls)
	require.Equal(t, 1, repo.setSchedulableCalls)
	require.True(t, repo.lastSchedulableValue)
	require.Equal(t, 1, repo.updateExtraCalls)
	health, ok := repo.lastExtraUpdates[AccountProbeHealthExtraKey].(map[string]any)
	require.True(t, ok)
	require.Equal(t, AccountProbeHealthNormal, health["level"])
	require.Equal(t, 0, health["failure_count"])
	require.Equal(t, 1, health["success_count"])
	require.NotContains(t, health, "last_error")
	require.NotContains(t, health, "next_probe_at")
}

func TestRateLimitService_RecordAccountProbeOutcomeManualSuccessClearsPathHealth(t *testing.T) {
	account := &Account{
		ID:          62033,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{
			"api_key": "sk-manual-success",
		},
		Extra: map[string]any{
			AccountProbeHealthExtraKey: map[string]any{
				"level":         AccountProbeHealthLineDegraded,
				"failure_count": 2,
				"last_error":    "unexpected EOF",
			},
		},
	}
	repo := &rateLimitAccountRepoStub{account: account}
	gateway := &OpenAIGatewayService{
		openaiPathHealth: NewOpenAIPathHealthTracker(OpenAIPathHealthOptions{
			Enabled:                  true,
			CircuitBreakerEnabled:    true,
			DegradedFailureThreshold: 1,
			OpenFailureThreshold:     2,
		}),
	}
	accountKey := OpenAIPathHealthKeyForAccount(account, string(OpenAIUpstreamTransportHTTPSSE))
	bucketKey := OpenAIPathHealthBucketKeyForAccount(account, string(OpenAIUpstreamTransportHTTPSSE))
	gateway.openaiPathHealth.RecordFailure(accountKey, OpenAIPathFailureEOF, nil)
	gateway.openaiPathHealth.RecordFailure(bucketKey, OpenAIPathFailureEOF, nil)
	service := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	service.SetAccountRuntimeBlocker(gateway)

	transition, err := service.RecordAccountProbeOutcome(context.Background(), AccountProbeOutcome{
		AccountID: account.ID,
		Account:   account,
		Source:    AccountProbeOutcomeSourceManualTest,
		Success:   true,
		Reason:    "probe_success",
	})

	require.NoError(t, err)
	require.True(t, transition.Restored)
	require.Equal(t, OpenAIPathHealthStateHealthy, gateway.openaiPathHealth.Snapshot(accountKey).State)
	require.Equal(t, OpenAIPathHealthStateHealthy, gateway.openaiPathHealth.Snapshot(bucketKey).State)
}

func TestRateLimitService_RecordAccountProbeOutcomeBackgroundSuccessDoesNotRepairSchedulingPoolState(t *testing.T) {
	account := &Account{
		ID:          62004,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: false,
		Extra: map[string]any{
			AccountProbeHealthExtraKey: map[string]any{
				"level":         AccountProbeHealthLightAbnormal,
				"failure_count": 1,
				"last_error":    "upstream failed",
			},
		},
	}
	repo := &rateLimitAccountRepoStub{account: account}
	service := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)

	transition, err := service.RecordAccountProbeOutcome(context.Background(), AccountProbeOutcome{
		AccountID: account.ID,
		Account:   account,
		Source:    AccountProbeOutcomeSourceAccountProbe,
		Success:   true,
		Reason:    "probe_success",
	})

	require.NoError(t, err)
	require.Equal(t, AccountProbeHealthLightAbnormal, transition.PreviousLevel)
	require.Equal(t, AccountProbeHealthLightAbnormal, transition.NextLevel)
	require.False(t, transition.StateChanged)
	require.False(t, transition.Restored)
	require.Equal(t, 0, repo.setSchedulableCalls)
	require.Equal(t, 0, repo.clearErrorCalls)
	require.Equal(t, 0, repo.clearRateLimitCalls)
	require.Equal(t, 0, repo.clearTempUnschedCalls)
	require.Equal(t, 0, repo.updateExtraCalls)
	require.Nil(t, repo.lastExtraUpdates)
	health := account.Extra[AccountProbeHealthExtraKey].(map[string]any)
	require.Equal(t, AccountProbeHealthLightAbnormal, health["level"])
	require.Equal(t, 1, health["failure_count"])
}

func TestRateLimitService_RecordAccountProbeOutcomeBackgroundSuccessDoesNotClearPathHealth(t *testing.T) {
	account := &Account{
		ID:          62035,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{
			"api_key": "sk-background-success",
		},
		Extra: map[string]any{
			AccountProbeHealthExtraKey: map[string]any{
				"level":         AccountProbeHealthLineDegraded,
				"failure_count": 2,
				"last_error":    "unexpected EOF",
			},
		},
	}
	repo := &rateLimitAccountRepoStub{account: account}
	gateway := &OpenAIGatewayService{
		openaiPathHealth: NewOpenAIPathHealthTracker(OpenAIPathHealthOptions{
			Enabled:                  true,
			CircuitBreakerEnabled:    true,
			DegradedFailureThreshold: 1,
			OpenFailureThreshold:     2,
		}),
	}
	accountKey := OpenAIPathHealthKeyForAccount(account, string(OpenAIUpstreamTransportHTTPSSE))
	bucketKey := OpenAIPathHealthBucketKeyForAccount(account, string(OpenAIUpstreamTransportHTTPSSE))
	gateway.openaiPathHealth.RecordFailure(accountKey, OpenAIPathFailureEOF, nil)
	gateway.openaiPathHealth.RecordFailure(bucketKey, OpenAIPathFailureEOF, nil)
	service := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	service.SetAccountRuntimeBlocker(gateway)

	transition, err := service.RecordAccountProbeOutcome(context.Background(), AccountProbeOutcome{
		AccountID: account.ID,
		Account:   account,
		Source:    AccountProbeOutcomeSourceAccountProbe,
		Success:   true,
		Reason:    "probe_success",
	})

	require.NoError(t, err)
	require.Equal(t, AccountProbeHealthLineDegraded, transition.NextLevel)
	require.False(t, transition.Restored)
	require.Equal(t, 0, repo.updateExtraCalls)
	require.Equal(t, OpenAIPathHealthStateDegraded, gateway.openaiPathHealth.Snapshot(accountKey).State)
	require.Equal(t, OpenAIPathHealthStateDegraded, gateway.openaiPathHealth.Snapshot(bucketKey).State)
}

type tokenCacheInvalidatorRecorder struct {
	accounts []*Account
	err      error
}

type openAI403CounterCacheStub struct {
	counts     []int64
	resetCalls []int64
	err        error
}

func (s *openAI403CounterCacheStub) IncrementOpenAI403Count(_ context.Context, _ int64, _ int) (int64, error) {
	if s.err != nil {
		return 0, s.err
	}
	if len(s.counts) == 0 {
		return 1, nil
	}
	count := s.counts[0]
	s.counts = s.counts[1:]
	return count, nil
}

func (s *openAI403CounterCacheStub) ResetOpenAI403Count(_ context.Context, accountID int64) error {
	s.resetCalls = append(s.resetCalls, accountID)
	return nil
}

func (r *tokenCacheInvalidatorRecorder) InvalidateToken(ctx context.Context, account *Account) error {
	r.accounts = append(r.accounts, account)
	return r.err
}

func TestRateLimitService_HandleUpstreamError_OAuth401SetsTempUnschedulable(t *testing.T) {
	t.Run("gemini", func(t *testing.T) {
		repo := &rateLimitAccountRepoStub{}
		invalidator := &tokenCacheInvalidatorRecorder{}
		service := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
		service.SetTokenCacheInvalidator(invalidator)
		account := &Account{
			ID:       100,
			Platform: PlatformGemini,
			Type:     AccountTypeOAuth,
			Credentials: map[string]any{
				"refresh_token":              "rt-100",
				"temp_unschedulable_enabled": true,
				"temp_unschedulable_rules": []any{
					map[string]any{
						"error_code":       401,
						"keywords":         []any{"unauthorized"},
						"duration_minutes": 30,
						"description":      "custom rule",
					},
				},
			},
		}

		shouldDisable := service.HandleUpstreamError(context.Background(), account, 401, http.Header{}, []byte("unauthorized"))

		require.True(t, shouldDisable)
		require.Equal(t, 0, repo.setErrorCalls)
		require.Equal(t, 1, repo.tempCalls)
		require.Len(t, invalidator.accounts, 1)
	})

	t.Run("antigravity_401_uses_SetError", func(t *testing.T) {
		// Antigravity 401 由 applyErrorPolicy 的 temp_unschedulable_rules 控制，
		// HandleUpstreamError 中走 SetError 路径。
		repo := &rateLimitAccountRepoStub{}
		invalidator := &tokenCacheInvalidatorRecorder{}
		service := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
		service.SetTokenCacheInvalidator(invalidator)
		account := &Account{
			ID:       100,
			Platform: PlatformAntigravity,
			Type:     AccountTypeOAuth,
		}

		shouldDisable := service.HandleUpstreamError(context.Background(), account, 401, http.Header{}, []byte("unauthorized"))

		require.True(t, shouldDisable)
		require.Equal(t, 1, repo.setErrorCalls)
		require.Equal(t, 0, repo.tempCalls)
		require.Empty(t, invalidator.accounts)
	})
}

// TestRateLimitService_HandleUpstreamError_OAuth401InvalidatorError
// OpenAI OAuth 401 缓存失效出错时仍走 temp_unschedulable。
// 401 handler 不再回写 credentials，避免请求开始时的快照覆盖 DB 中刚刷新的 refresh_token。
func TestRateLimitService_HandleUpstreamError_OAuth401InvalidatorError(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	invalidator := &tokenCacheInvalidatorRecorder{err: errors.New("boom")}
	service := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	service.SetTokenCacheInvalidator(invalidator)
	account := &Account{
		ID:       101,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "rt-101",
		},
	}

	shouldDisable := service.HandleUpstreamError(context.Background(), account, 401, http.Header{}, []byte("unauthorized"))

	require.True(t, shouldDisable)
	require.Equal(t, 0, repo.setErrorCalls)
	require.Equal(t, 1, repo.tempCalls)
	require.Equal(t, 0, repo.updateCredentialsCalls)
	require.Len(t, invalidator.accounts, 1)
}

func TestRateLimitService_HandleUpstreamError_NonOAuth401(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	invalidator := &tokenCacheInvalidatorRecorder{}
	service := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	service.SetTokenCacheInvalidator(invalidator)
	account := &Account{
		ID:       102,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_keys": []any{"key-a", "key-b"},
		},
	}
	require.Equal(t, "key-a", account.GetAPIKey())

	shouldDisable := service.HandleUpstreamError(context.Background(), account, 401, http.Header{}, []byte("unauthorized"))

	require.True(t, shouldDisable)
	require.Equal(t, 0, repo.setErrorCalls)
	require.Equal(t, 0, repo.tempCalls)
	require.Equal(t, 1, repo.updateCredentialsCalls)
	require.NotNil(t, repo.lastCredentials)
	disabled, _ := repo.lastCredentials[CredentialAPIKeysDisabled].(map[string]any)
	require.Contains(t, disabled, FingerprintAPIKey("key-a"))
	require.Equal(t, []string{"key-b"}, account.GetAPIKeys())
	require.Empty(t, invalidator.accounts)
}

// TestRateLimitService_HandleUpstreamError_OAuth401DoesNotOverwriteCredentials
// 回归测试：确保 401 handler 不再用请求开始时的 account 快照整列覆盖 credentials JSONB。
func TestRateLimitService_HandleUpstreamError_OAuth401DoesNotOverwriteCredentials(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	service := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	account := &Account{
		ID:       103,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":  "token",
			"refresh_token": "rt-103",
		},
	}

	shouldDisable := service.HandleUpstreamError(context.Background(), account, 401, http.Header{}, []byte("unauthorized"))

	require.True(t, shouldDisable)
	require.Equal(t, 0, repo.updateCredentialsCalls, "401 handler must not write credentials back from the request-start snapshot")
	require.Equal(t, 1, repo.tempCalls, "401 handler should still set temp-unschedulable cooldown")
	require.Nil(t, repo.lastCredentials)
}

// 缺少 refresh_token 的 OAuth 账号 401 应直接 SetError 永久禁用，
// 不再走 10 分钟冷却（冷却期内无人能刷新它，结束后还会被选中再 502 一次）。
func TestRateLimitService_HandleUpstreamError_OAuth401NoRefreshTokenSetsError(t *testing.T) {
	t.Run("openai_no_refresh_token", func(t *testing.T) {
		repo := &rateLimitAccountRepoStub{}
		invalidator := &tokenCacheInvalidatorRecorder{}
		service := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
		service.SetTokenCacheInvalidator(invalidator)
		account := &Account{
			ID:       2881,
			Platform: PlatformOpenAI,
			Type:     AccountTypeOAuth,
			Credentials: map[string]any{
				"access_token": "expired-at",
				// no refresh_token
			},
		}

		shouldDisable := service.HandleUpstreamError(context.Background(), account, 401, http.Header{}, []byte("unauthorized"))

		require.True(t, shouldDisable)
		require.Equal(t, 1, repo.setErrorCalls, "AT-only OAuth 401 must SetError")
		require.Equal(t, 0, repo.tempCalls, "AT-only OAuth 401 must NOT temp-unschedule")
		require.Equal(t, 0, repo.updateCredentialsCalls, "no point forcing expires_at when refresh is impossible")
		require.Contains(t, repo.lastErrorMsg, "refresh_token missing")
		require.Len(t, invalidator.accounts, 1, "cache should still be invalidated")
	})

	t.Run("openai_blank_refresh_token_treated_as_missing", func(t *testing.T) {
		repo := &rateLimitAccountRepoStub{}
		service := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
		account := &Account{
			ID:       2882,
			Platform: PlatformOpenAI,
			Type:     AccountTypeOAuth,
			Credentials: map[string]any{
				"access_token":  "expired-at",
				"refresh_token": "   ",
			},
		}

		shouldDisable := service.HandleUpstreamError(context.Background(), account, 401, http.Header{}, []byte("unauthorized"))

		require.True(t, shouldDisable)
		require.Equal(t, 1, repo.setErrorCalls)
		require.Equal(t, 0, repo.tempCalls)
	})
}
