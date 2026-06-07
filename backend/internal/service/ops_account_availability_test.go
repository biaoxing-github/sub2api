//go:build unit

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDeriveAccountEffectiveAvailability(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	future := now.Add(15 * time.Minute)
	pathCooldown := now.Add(5 * time.Minute)

	tests := []struct {
		name       string
		account    *Account
		pathHealth OpenAIPathHealthRecord
		hasPath    bool
		wantState  string
		wantReason string
		wantUntil  bool
	}{
		{
			name: "healthy account",
			account: &Account{
				Status:      StatusActive,
				Schedulable: true,
				Platform:    PlatformOpenAI,
			},
			wantState: AccountEffectiveAvailabilityHealthy,
		},
		{
			name: "rate limited account",
			account: &Account{
				Status:           StatusActive,
				Schedulable:      true,
				RateLimitResetAt: &future,
				Platform:         PlatformOpenAI,
			},
			wantState:  AccountEffectiveAvailabilityRateLimited,
			wantReason: "rate_limit_reset_at",
			wantUntil:  true,
		},
		{
			name: "temp unschedulable account",
			account: &Account{
				Status:                  StatusActive,
				Schedulable:             true,
				TempUnschedulableUntil:  &future,
				TempUnschedulableReason: "header_timeout",
				Platform:                PlatformOpenAI,
			},
			wantState:  AccountEffectiveAvailabilityTempUnschedulable,
			wantReason: "header_timeout",
			wantUntil:  true,
		},
		{
			name: "open circuit path-health account",
			account: &Account{
				Status:      StatusActive,
				Schedulable: true,
				Platform:    PlatformOpenAI,
			},
			pathHealth: OpenAIPathHealthRecord{
				State:             OpenAIPathHealthStateOpenCircuit,
				LastFailureReason: "unexpected_eof",
				CooldownUntil:     &pathCooldown,
			},
			hasPath:    true,
			wantState:  AccountEffectiveAvailabilityPathOpenCircuit,
			wantReason: "unexpected_eof",
			wantUntil:  true,
		},
		{
			name: "half open path-health account",
			account: &Account{
				Status:      StatusActive,
				Schedulable: true,
				Platform:    PlatformOpenAI,
			},
			pathHealth: OpenAIPathHealthRecord{
				State:             OpenAIPathHealthStateHalfOpen,
				LastFailureReason: "probe_limit",
				CooldownUntil:     &pathCooldown,
			},
			hasPath:    true,
			wantState:  AccountEffectiveAvailabilityPathHalfOpen,
			wantReason: "probe_limit",
			wantUntil:  true,
		},
		{
			name: "error account",
			account: &Account{
				Status:       StatusError,
				Schedulable:  false,
				ErrorMessage: "401 invalid token",
				Platform:     PlatformOpenAI,
			},
			wantState:  AccountEffectiveAvailabilityError,
			wantReason: "401 invalid token",
		},
		{
			name: "degraded path-health account",
			account: &Account{
				Status:      StatusActive,
				Schedulable: true,
				Platform:    PlatformOpenAI,
			},
			pathHealth: OpenAIPathHealthRecord{
				State:             OpenAIPathHealthStateDegraded,
				LastFailureReason: "header_timeout",
			},
			hasPath:    true,
			wantState:  AccountEffectiveAvailabilityPathDegraded,
			wantReason: "header_timeout",
		},
		{
			name: "disabled account",
			account: &Account{
				Status:      StatusDisabled,
				Schedulable: true,
				Platform:    PlatformOpenAI,
			},
			wantState:  AccountEffectiveAvailabilityDisabled,
			wantReason: "status_not_active",
		},
		{
			name: "unschedulable account",
			account: &Account{
				Status:      StatusActive,
				Schedulable: false,
				Platform:    PlatformOpenAI,
			},
			wantState:  AccountEffectiveAvailabilityUnschedulable,
			wantReason: "schedulable_disabled",
		},
		{
			name: "path health takes precedence over temp unschedulable",
			account: &Account{
				Status:                 StatusActive,
				Schedulable:            true,
				TempUnschedulableUntil: &future,
				Platform:               PlatformOpenAI,
			},
			pathHealth: OpenAIPathHealthRecord{
				State:             OpenAIPathHealthStateOpenCircuit,
				LastFailureReason: "unexpected_eof",
			},
			hasPath:    true,
			wantState:  AccountEffectiveAvailabilityPathOpenCircuit,
			wantReason: "unexpected_eof",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := deriveAccountEffectiveAvailability(tt.account, now, tt.pathHealth, tt.hasPath)
			require.Equal(t, tt.wantState, got.State)
			require.Equal(t, tt.wantReason, got.Reason)
			if tt.wantUntil {
				require.NotNil(t, got.Until)
			} else {
				require.Nil(t, got.Until)
			}
		})
	}
}
