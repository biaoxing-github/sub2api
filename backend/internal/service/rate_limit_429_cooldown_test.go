//go:build unit

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type rateLimit429AccountRepoStub struct {
	mockAccountRepoForGemini
	rateLimitCalls     int
	tempCalls          int
	lastRateLimitID    int64
	lastRateLimitReset time.Time
	lastTempUntil      time.Time
	lastTempReason     string
	updatedCredentials map[string]any
}

func (r *rateLimit429AccountRepoStub) SetRateLimited(_ context.Context, id int64, resetAt time.Time) error {
	r.rateLimitCalls++
	r.lastRateLimitID = id
	r.lastRateLimitReset = resetAt
	return nil
}

func (r *rateLimit429AccountRepoStub) SetTempUnschedulable(_ context.Context, _ int64, until time.Time, reason string) error {
	r.tempCalls++
	r.lastTempUntil = until
	r.lastTempReason = reason
	return nil
}

func (r *rateLimit429AccountRepoStub) UpdateCredentials(_ context.Context, _ int64, credentials map[string]any) error {
	r.updatedCredentials = cloneCredentials(credentials)
	return nil
}

func TestGetRateLimit429CooldownSettings_DefaultsWhenNotSet(t *testing.T) {
	repo := newMockSettingRepo()
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetRateLimit429CooldownSettings(context.Background())
	require.NoError(t, err)
	require.True(t, settings.Enabled)
	require.Equal(t, 5, settings.CooldownSeconds)
}

func TestGetRateLimit429CooldownSettings_ReadsFromDB(t *testing.T) {
	repo := newMockSettingRepo()
	data, _ := json.Marshal(RateLimit429CooldownSettings{Enabled: false, CooldownSeconds: 12})
	repo.data[SettingKeyRateLimit429CooldownSettings] = string(data)
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetRateLimit429CooldownSettings(context.Background())
	require.NoError(t, err)
	require.False(t, settings.Enabled)
	require.Equal(t, 12, settings.CooldownSeconds)
}

func TestSetRateLimit429CooldownSettings_EnabledRejectsOutOfRange(t *testing.T) {
	svc := NewSettingService(newMockSettingRepo(), &config.Config{})

	for _, seconds := range []int{0, -1, 7201, 99999} {
		err := svc.SetRateLimit429CooldownSettings(context.Background(), &RateLimit429CooldownSettings{
			Enabled: true, CooldownSeconds: seconds,
		})
		require.Error(t, err, "should reject enabled=true + cooldown_seconds=%d", seconds)
		require.Contains(t, err.Error(), "cooldown_seconds must be between 1-7200")
	}
}

func TestHandle429_FallbackUsesDBSeconds(t *testing.T) {
	accountRepo := &rateLimit429AccountRepoStub{}
	settingRepo := newMockSettingRepo()
	data, _ := json.Marshal(RateLimit429CooldownSettings{Enabled: true, CooldownSeconds: 12})
	settingRepo.data[SettingKeyRateLimit429CooldownSettings] = string(data)

	settingSvc := NewSettingService(settingRepo, &config.Config{})
	svc := NewRateLimitService(accountRepo, nil, &config.Config{}, nil, nil)
	svc.SetSettingService(settingSvc)

	account := &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	before := time.Now()
	svc.handle429(context.Background(), account, http.Header{}, []byte(`{"error":{"type":"rate_limit_error","message":"slow down"}}`))
	after := time.Now()

	require.Equal(t, 1, accountRepo.rateLimitCalls)
	require.Equal(t, int64(42), accountRepo.lastRateLimitID)
	require.True(t, !accountRepo.lastRateLimitReset.Before(before.Add(12*time.Second)) && !accountRepo.lastRateLimitReset.After(after.Add(12*time.Second)))
}

func TestHandle429_FallbackDisabledSkipsLocalMark(t *testing.T) {
	accountRepo := &rateLimit429AccountRepoStub{}
	settingRepo := newMockSettingRepo()
	data, _ := json.Marshal(RateLimit429CooldownSettings{Enabled: false, CooldownSeconds: 12})
	settingRepo.data[SettingKeyRateLimit429CooldownSettings] = string(data)

	settingSvc := NewSettingService(settingRepo, &config.Config{})
	svc := NewRateLimitService(accountRepo, nil, &config.Config{}, nil, nil)
	svc.SetSettingService(settingSvc)

	account := &Account{ID: 43, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	svc.handle429(context.Background(), account, http.Header{}, []byte(`{"error":{"type":"rate_limit_error","message":"slow down"}}`))

	require.Zero(t, accountRepo.rateLimitCalls)
}

func TestHandle429_FallbackUsesDefaultSecondsWhenSettingServiceMissing(t *testing.T) {
	accountRepo := &rateLimit429AccountRepoStub{}
	cfg := &config.Config{}
	svc := NewRateLimitService(accountRepo, nil, cfg, nil, nil)

	account := &Account{ID: 44, Platform: PlatformGemini, Type: AccountTypeAPIKey}
	before := time.Now()
	svc.handle429(context.Background(), account, http.Header{}, []byte(`{"error":{"message":"slow down"}}`))
	after := time.Now()

	require.Equal(t, 1, accountRepo.rateLimitCalls)
	require.Equal(t, int64(44), accountRepo.lastRateLimitID)
	require.True(t, !accountRepo.lastRateLimitReset.Before(before.Add(5*time.Second)) && !accountRepo.lastRateLimitReset.After(after.Add(5*time.Second)))
}

// 确认多 Key 账号遇到 429 时只冷却本次报错 Key，保留同账号其他 Key 继续参与调度。
func TestHandleUpstreamError429_OpenAIAPIKeyDisablesSelectedKey(t *testing.T) {
	accountRepo := &rateLimit429AccountRepoStub{}
	svc := NewRateLimitService(accountRepo, nil, &config.Config{}, nil, nil)
	account := &Account{
		ID:       45,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_keys": []any{"key-a", "key-b"},
		},
	}
	require.Equal(t, "key-a", account.GetAPIKey())

	shouldDisable := svc.HandleUpstreamError(context.Background(), account, http.StatusTooManyRequests, http.Header{}, []byte(`{"error":{"message":"slow down"}}`))

	require.True(t, shouldDisable)
	require.Zero(t, accountRepo.rateLimitCalls)
	require.Equal(t, 0, accountRepo.tempCalls)
	require.NotNil(t, accountRepo.updatedCredentials)
	disabled, _ := accountRepo.updatedCredentials[CredentialAPIKeysDisabled].(map[string]any)
	record, _ := disabled[FingerprintAPIKey("key-a")].(map[string]any)
	require.NotNil(t, record)
	require.Equal(t, "rate_limited", record["reason"])
	require.Equal(t, 1, record["disabled_count"])
	require.Equal(t, []string{"key-b"}, account.GetAPIKeys())
}

// 确认单 Key API Key 账号默认冷却按同类错误次数使用账号级阶梯退避时间。
func TestHandleUpstreamError429_OpenAIAPIKeySchedulingCooldownUsesSteppedErrorCount(t *testing.T) {
	accountRepo := &rateLimit429AccountRepoStub{}
	svc := NewRateLimitService(accountRepo, nil, &config.Config{}, nil, nil)
	now := time.Now()
	previousState := &TempUnschedState{
		UntilUnix:       now.Add(-time.Second).Unix(),
		TriggeredAtUnix: now.Add(-2 * time.Second).Unix(),
		StatusCode:      http.StatusTooManyRequests,
		MatchedKeyword:  "rate_limited",
		RuleIndex:       apiKeyAccountSchedulingRuleIndex,
		ErrorCount:      1,
	}
	previousReason, err := json.Marshal(previousState)
	require.NoError(t, err)
	account := &Account{
		ID:                      47,
		Platform:                PlatformOpenAI,
		Type:                    AccountTypeAPIKey,
		TempUnschedulableReason: string(previousReason),
		TempUnschedulableUntil:  &now,
		Credentials:             map[string]any{"api_keys": []any{"key-a"}},
	}

	shouldDisable := svc.HandleUpstreamError(context.Background(), account, http.StatusTooManyRequests, http.Header{}, []byte(`{"error":{"message":"slow down"}}`))

	require.True(t, shouldDisable)
	require.Equal(t, 1, accountRepo.tempCalls)
	require.WithinDuration(t, time.Now().Add(probeIntervalFromErrorCount(2)), accountRepo.lastTempUntil, 2*time.Second)
	var state TempUnschedState
	require.NoError(t, json.Unmarshal([]byte(accountRepo.lastTempReason), &state))
	require.Equal(t, 2, state.ErrorCount)
	require.Equal(t, "rate_limited", state.MatchedKeyword)
	require.Equal(t, []string{"key-a"}, account.GetAPIKeys())
}

// 确认多 Key 账号配置了临时不可调度规则时，429 优先冷却本次报错 Key。
func TestHandleUpstreamError429_OpenAIAPIKeyWithTempRulesDisablesSelectedKey(t *testing.T) {
	accountRepo := &rateLimit429AccountRepoStub{}
	svc := NewRateLimitService(accountRepo, nil, &config.Config{}, nil, nil)
	account := &Account{
		ID:       46,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_keys":                   []any{"key-a", "key-b"},
			"temp_unschedulable_enabled": true,
			"temp_unschedulable_rules": []any{
				map[string]any{
					"error_code":       429,
					"keywords":         []any{"slow down"},
					"duration_minutes": 10,
				},
			},
		},
	}
	require.Equal(t, "key-a", account.GetAPIKey())

	shouldDisable := svc.HandleUpstreamError(context.Background(), account, http.StatusTooManyRequests, http.Header{}, []byte(`{"error":{"message":"slow down"}}`))

	require.True(t, shouldDisable)
	require.Zero(t, accountRepo.rateLimitCalls)
	require.Equal(t, 0, accountRepo.tempCalls)
	require.NotNil(t, accountRepo.updatedCredentials)
	disabled, _ := accountRepo.updatedCredentials[CredentialAPIKeysDisabled].(map[string]any)
	require.Contains(t, disabled, FingerprintAPIKey("key-a"))
	require.Equal(t, []string{"key-b"}, account.GetAPIKeys())
}
