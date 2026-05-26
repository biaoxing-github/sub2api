//go:build unit

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAccountLoadFactorAdvisor_InsufficientSamplesReturnsNeedsProbe(t *testing.T) {
	advisor := NewAccountLoadFactorAdvisor(AccountLoadFactorAdvisorOptions{MinSamples: 3})
	account := &Account{ID: 1, Concurrency: 4}

	result := advisor.Advise(account, OpenAIPathHealthRecord{
		State:        OpenAIPathHealthStateHealthy,
		Samples:      2,
		SuccessCount: 2,
		TTFTEWMAMs:   800,
	})

	require.Nil(t, result.SuggestedLoadFactor)
	require.Equal(t, AccountAvailabilityRadarNeedsProbe, result.AvailabilityRadar.Status)
	require.Contains(t, result.Reasons, "样本不足")
}

func TestAccountLoadFactorAdvisor_HealthyFastAccountSuggestsConservativeIncrease(t *testing.T) {
	advisor := NewAccountLoadFactorAdvisor(AccountLoadFactorAdvisorOptions{MinSamples: 3})
	current := 8
	account := &Account{ID: 1, Concurrency: 4, LoadFactor: &current}

	result := advisor.Advise(account, OpenAIPathHealthRecord{
		State:                OpenAIPathHealthStateHealthy,
		Samples:              10,
		SuccessCount:         9,
		FailureCount:         1,
		TTFTEWMAMs:           500,
		HeaderWaitEWMAMs:     200,
		ConsecutiveFailures:  0,
		WindowFailures:       0,
		ConsecutiveSuccesses: 4,
	})

	require.NotNil(t, result.SuggestedLoadFactor)
	require.Equal(t, 12, *result.SuggestedLoadFactor)
	require.Equal(t, AccountAvailabilityRadarFastStable, result.AvailabilityRadar.Status)
	require.Contains(t, result.Reasons, "成功率 90%")
}

func TestAccountLoadFactorAdvisor_DegradedSlowAccountReducesSuggestion(t *testing.T) {
	advisor := NewAccountLoadFactorAdvisor(AccountLoadFactorAdvisorOptions{MinSamples: 3})
	account := &Account{ID: 2, Concurrency: 10}

	result := advisor.Advise(account, OpenAIPathHealthRecord{
		State:            OpenAIPathHealthStateDegraded,
		Samples:          8,
		SuccessCount:     5,
		FailureCount:     3,
		TTFTEWMAMs:       5000,
		HeaderWaitEWMAMs: 1800,
		EOFCount:         1,
		WindowFailures:   3,
	})

	require.NotNil(t, result.SuggestedLoadFactor)
	require.Equal(t, 3, *result.SuggestedLoadFactor)
	require.Equal(t, AccountAvailabilityRadarUnstable, result.AvailabilityRadar.Status)
}

func TestAccountLoadFactorAdvisor_OpenCircuitReportsCooldown(t *testing.T) {
	advisor := NewAccountLoadFactorAdvisor(AccountLoadFactorAdvisorOptions{MinSamples: 1})
	account := &Account{ID: 3, Concurrency: 20}
	until := time.Now().Add(time.Minute)

	result := advisor.Advise(account, OpenAIPathHealthRecord{
		State:         OpenAIPathHealthStateOpenCircuit,
		Samples:       5,
		SuccessCount:  1,
		FailureCount:  4,
		CooldownUntil: &until,
	})

	require.NotNil(t, result.SuggestedLoadFactor)
	require.Equal(t, 1, *result.SuggestedLoadFactor)
	require.Equal(t, AccountAvailabilityRadarCooldown, result.AvailabilityRadar.Status)
}

func TestAccountLoadFactorAdvisor_ClampsSuggestion(t *testing.T) {
	advisor := NewAccountLoadFactorAdvisor(AccountLoadFactorAdvisorOptions{MinSamples: 1})
	huge := 20000
	account := &Account{ID: 4, Concurrency: 20000, LoadFactor: &huge}

	result := advisor.Advise(account, OpenAIPathHealthRecord{
		State:            OpenAIPathHealthStateHealthy,
		Samples:          10,
		SuccessCount:     10,
		TTFTEWMAMs:       100,
		HeaderWaitEWMAMs: 50,
	})

	require.NotNil(t, result.SuggestedLoadFactor)
	require.Equal(t, 10000, *result.SuggestedLoadFactor)
}
