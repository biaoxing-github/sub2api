//go:build unit

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAccountIsSchedulable_QuotaExceeded(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		account *Account
		want    bool
	}{
		{
			name: "apikey daily quota exceeded",
			account: &Account{
				Status:      StatusActive,
				Schedulable: true,
				Type:        AccountTypeAPIKey,
				Extra: map[string]any{
					"quota_daily_limit": 10.0,
					"quota_daily_used":  10.0,
					"quota_daily_start": now.Add(-1 * time.Hour).Format(time.RFC3339),
				},
			},
			want: false,
		},
		{
			name: "apikey weekly quota exceeded",
			account: &Account{
				Status:      StatusActive,
				Schedulable: true,
				Type:        AccountTypeAPIKey,
				Extra: map[string]any{
					"quota_weekly_limit": 50.0,
					"quota_weekly_used":  50.0,
					"quota_weekly_start": now.Add(-2 * 24 * time.Hour).Format(time.RFC3339),
				},
			},
			want: false,
		},
		{
			name: "apikey total quota exceeded",
			account: &Account{
				Status:      StatusActive,
				Schedulable: true,
				Type:        AccountTypeAPIKey,
				Extra: map[string]any{
					"quota_limit": 100.0,
					"quota_used":  100.0,
				},
			},
			want: false,
		},
		{
			name: "apikey quota not exceeded",
			account: &Account{
				Status:      StatusActive,
				Schedulable: true,
				Type:        AccountTypeAPIKey,
				Extra: map[string]any{
					"quota_daily_limit": 10.0,
					"quota_daily_used":  5.0,
					"quota_daily_start": now.Add(-1 * time.Hour).Format(time.RFC3339),
				},
			},
			want: true,
		},
		{
			name: "apikey expired daily period restores schedulable",
			account: &Account{
				Status:      StatusActive,
				Schedulable: true,
				Type:        AccountTypeAPIKey,
				Extra: map[string]any{
					"quota_daily_limit": 10.0,
					"quota_daily_used":  10.0,
					"quota_daily_start": now.Add(-25 * time.Hour).Format(time.RFC3339),
				},
			},
			want: true,
		},
		{
			name: "oauth ignores quota exceeded",
			account: &Account{
				Status:      StatusActive,
				Schedulable: true,
				Type:        AccountTypeOAuth,
				Extra: map[string]any{
					"quota_daily_limit": 10.0,
					"quota_daily_used":  10.0,
					"quota_daily_start": now.Add(-1 * time.Hour).Format(time.RFC3339),
				},
			},
			want: true,
		},
		{
			name: "bedrock quota exceeded",
			account: &Account{
				Status:      StatusActive,
				Schedulable: true,
				Type:        AccountTypeBedrock,
				Extra: map[string]any{
					"quota_limit": 200.0,
					"quota_used":  200.0,
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.account.IsSchedulable())
		})
	}
}

func TestAccountCodexExtraEffectiveRateLimit(t *testing.T) {
	now := time.Now().UTC()
	resetAt := now.Add(6 * time.Hour).Truncate(time.Second)

	account := &Account{
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Extra: map[string]any{
			"codex_7d_used_percent": 100.0,
			"codex_7d_reset_at":     resetAt.Format(time.RFC3339),
		},
	}

	effective := account.EffectiveRateLimitResetAt()
	require.NotNil(t, effective)
	require.WithinDuration(t, resetAt, *effective, time.Second)
	require.True(t, account.IsRateLimited())
	require.False(t, account.IsSchedulable())
}

func TestAccountCodexExtraEffectiveRateLimitIgnoresAPIKeyAndExpiredWindow(t *testing.T) {
	past := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	future := time.Now().UTC().Add(time.Hour).Truncate(time.Second)

	apiKey := &Account{
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Extra: map[string]any{
			"codex_7d_used_percent": 100.0,
			"codex_7d_reset_at":     future.Format(time.RFC3339),
		},
	}
	require.Nil(t, apiKey.EffectiveRateLimitResetAt())
	require.False(t, apiKey.IsRateLimited())
	require.True(t, apiKey.IsSchedulable())

	expiredOAuth := &Account{
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Extra: map[string]any{
			"codex_7d_used_percent": 100.0,
			"codex_7d_reset_at":     past.Format(time.RFC3339),
		},
	}
	require.Nil(t, expiredOAuth.EffectiveRateLimitResetAt())
	require.False(t, expiredOAuth.IsRateLimited())
	require.True(t, expiredOAuth.IsSchedulable())
}
