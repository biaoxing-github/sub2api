package service

import (
	"testing"
	"time"
)

func TestDeriveAccountHealthState(t *testing.T) {
	now := time.Unix(1000, 0).UTC()
	resetAt := now.Add(time.Minute)

	cases := []struct {
		name    string
		account *Account
		health  OpenAIPathHealthRecord
		want    string
	}{
		{
			name:    "normal",
			account: &Account{Status: StatusActive, Schedulable: true},
			want:    AccountDerivedHealthNormal,
		},
		{
			name:    "429 cooldown",
			account: &Account{Status: StatusActive, Schedulable: true, RateLimitResetAt: &resetAt},
			want:    AccountDerivedHealthRateLimitedCooldown,
		},
		{
			name:    "401 invalid",
			account: &Account{Status: StatusError, ErrorMessage: "Authentication failed (401): token invalid", Schedulable: false},
			want:    AccountDerivedHealthUnauthorizedInvalid,
		},
		{
			name:    "line degraded",
			account: &Account{Status: StatusActive, Schedulable: true},
			health:  OpenAIPathHealthRecord{State: OpenAIPathHealthStateOpenCircuit, Samples: 3, LastFailureReason: OpenAIPathFailureEOF},
			want:    AccountDerivedHealthLineDegraded,
		},
		{
			name:    "temp unschedulable upstream abnormal",
			account: &Account{Status: StatusActive, Schedulable: true, TempUnschedulableUntil: &resetAt, TempUnschedulableReason: "header_timeout"},
			want:    AccountDerivedHealthUpstreamAbnormal,
		},
		{
			name:    "pending retest",
			account: &Account{Status: StatusError, ErrorMessage: "", Schedulable: false},
			want:    AccountDerivedHealthPendingRetest,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DeriveAccountHealthState(tc.account, tc.health, now)
			if got.State != tc.want {
				t.Fatalf("State = %q, want %q; got=%+v", got.State, tc.want, got)
			}
		})
	}
}
