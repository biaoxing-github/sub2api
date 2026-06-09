package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenAIGatewayService_ListOpenAIAccountSchedulingPoolShowsHealthAndReasons(t *testing.T) {
	now := time.Date(2026, 6, 9, 10, 0, 0, 0, time.UTC)
	loadFactor := 3
	accounts := []Account{
		{
			ID:            101,
			Name:          "ready-api-key",
			Platform:      PlatformOpenAI,
			Type:          AccountTypeAPIKey,
			Status:        StatusActive,
			Schedulable:   true,
			Concurrency:   5,
			Priority:      20,
			LoadFactor:    &loadFactor,
			Credentials:   map[string]any{"api_key": "sk-ready"},
			AccountGroups: []AccountGroup{{GroupID: 7}},
		},
		{
			ID:          102,
			Name:        "line-degraded",
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 2,
		},
		{
			ID:          103,
			Name:        "runtime-blocked",
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
		},
		{
			ID:          104,
			Name:        "model-filtered",
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Credentials: map[string]any{
				"api_key":       "sk-filtered",
				"model_mapping": map[string]any{"gpt-4.1": "gpt-4.1"},
			},
		},
		{
			ID:          105,
			Name:        "manual-disabled",
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: false,
		},
	}
	pathHealth := NewOpenAIPathHealthTracker(OpenAIPathHealthOptions{
		Enabled:                  true,
		CircuitBreakerEnabled:    true,
		DegradedFailureThreshold: 1,
		OpenFailureThreshold:     4,
		Cooldown:                 time.Minute,
	})
	pathHealth.RecordFailure(OpenAIPathHealthKeyForAccount(&accounts[1], string(OpenAIUpstreamTransportHTTPSSE)), OpenAIPathFailureEOF, nil)
	svc := &OpenAIGatewayService{
		accountRepo:      schedulerTestOpenAIAccountRepo{accounts: accounts},
		openaiPathHealth: pathHealth,
	}
	svc.BlockAccountScheduling(&accounts[2], now.Add(10*time.Minute), "429")

	snapshot, err := svc.ListOpenAIAccountSchedulingPool(context.Background(), OpenAIAccountSchedulingPoolFilter{
		GroupID:   schedulingPoolInt64Ptr(7),
		Model:     "gpt-5.5",
		Endpoint:  OpenAIEndpointCapabilityResponses,
		Transport: OpenAIUpstreamTransportHTTPSSE,
	}, now)

	require.NoError(t, err)
	require.Equal(t, 4, snapshot.Total)
	require.Equal(t, 1, snapshot.SchedulableCount)
	require.Equal(t, 1, snapshot.DegradedCount)
	require.Equal(t, 1, snapshot.BlockedCount)
	require.Equal(t, 1, snapshot.FilteredCount)
	require.Equal(t, int64(7), *snapshot.GroupID)
	require.Equal(t, "gpt-5.5", snapshot.Model)
	require.NotContains(t, openAISchedulingPoolItemIDs(snapshot.Items), int64(105))

	items := openAISchedulingPoolItemsByID(snapshot.Items)
	require.Equal(t, OpenAIAccountSchedulingPoolStatusSchedulable, items[101].PoolStatus)
	require.Equal(t, 3, items[101].EffectiveLoadFactor)
	require.Empty(t, items[101].PoolReasons)

	require.Equal(t, OpenAIAccountSchedulingPoolStatusDegraded, items[102].PoolStatus)
	require.Equal(t, AccountDerivedHealthLineDegraded, items[102].DerivedHealth.State)
	require.Contains(t, items[102].PoolReasons, "path_health:degraded:unexpected_eof")

	require.Equal(t, OpenAIAccountSchedulingPoolStatusBlocked, items[103].PoolStatus)
	require.NotNil(t, items[103].RuntimeBlock)
	require.Equal(t, "429", items[103].RuntimeBlock.Reason)
	require.Contains(t, items[103].PoolReasons, "runtime_block:429")

	require.Equal(t, OpenAIAccountSchedulingPoolStatusFiltered, items[104].PoolStatus)
	require.Contains(t, items[104].PoolReasons, "model_unsupported:gpt-5.5")
}

func openAISchedulingPoolItemsByID(items []OpenAIAccountSchedulingPoolItem) map[int64]OpenAIAccountSchedulingPoolItem {
	out := make(map[int64]OpenAIAccountSchedulingPoolItem, len(items))
	for _, item := range items {
		out[item.Account.ID] = item
	}
	return out
}

func openAISchedulingPoolItemIDs(items []OpenAIAccountSchedulingPoolItem) []int64 {
	out := make([]int64, 0, len(items))
	for _, item := range items {
		out = append(out, item.Account.ID)
	}
	return out
}

func schedulingPoolInt64Ptr(v int64) *int64 {
	return &v
}
