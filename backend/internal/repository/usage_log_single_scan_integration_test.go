//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// TestUsageStatsSingleScan 验证汇总和各端点明细应用同一组筛选与账号费用口径。
func TestUsageStatsSingleScan(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	repo := newUsageLogRepositoryWithSQL(client, tx)
	user := mustCreateUser(t, client, &service.User{Email: "single-scan@test.com"})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-single-scan"})
	account := mustCreateAccount(t, client, &service.Account{Name: "single-scan"})
	now := time.Now().UTC().Truncate(time.Second)
	input, upstream := "/v1/responses", "/v1/chat/completions"
	mode, otherMode := "token", "image"
	matched, unmatched := true, false
	accountCost, multiplier := 3.0, 2.0
	for i, row := range []struct {
		mode     *string
		mismatch *bool
	}{{&mode, &matched}, {&otherMode, &matched}, {&mode, &unmatched}} {
		_, err := repo.Create(ctx, &service.UsageLog{
			UserID: user.ID, APIKeyID: key.ID, AccountID: account.ID,
			RequestID: string(rune('a' + i)), Model: "gpt-5", InputTokens: 10, OutputTokens: 20,
			CacheCreationTokens: 2, CacheReadTokens: 3, TotalCost: 5, ActualCost: 4,
			AccountStatsCost: &accountCost, AccountRateMultiplier: &multiplier,
			InboundEndpoint: &input, UpstreamEndpoint: &upstream,
			BillingMode: row.mode, UpstreamModelMismatch: row.mismatch, CreatedAt: now,
		})
		require.NoError(t, err)
	}
	start, end := now.Add(-time.Second), now.Add(time.Second)
	filters := usagestats.UsageLogFilters{AccountID: account.ID, BillingMode: mode,
		UpstreamModelMismatch: &matched, StartTime: &start, EndTime: &end}
	for _, byUser := range []bool{false, true} {
		if byUser {
			filters.UserID = user.ID
		}
		stats, err := repo.GetStatsWithFilters(ctx, filters)
		require.NoError(t, err)
		require.Equal(t, int64(1), stats.TotalRequests)
		require.Equal(t, int64(35), stats.TotalTokens)
		require.Equal(t, 6.0, *stats.TotalAccountCost)
		wantCost := 6.0
		if byUser {
			wantCost = 4
		}
		for _, details := range [][]usagestats.EndpointStat{stats.Endpoints, stats.UpstreamEndpoints, stats.EndpointPaths} {
			require.Len(t, details, 1)
			require.Equal(t, stats.TotalRequests, details[0].Requests)
			require.Equal(t, stats.TotalTokens, details[0].TotalTokens)
			require.Equal(t, wantCost, details[0].ActualCost)
		}
	}
	filters.EndTime = &now
	empty, err := repo.GetStatsWithFilters(ctx, filters)
	require.NoError(t, err)
	require.Zero(t, empty.TotalRequests)
	require.Empty(t, empty.Endpoints)
	require.Equal(t, 0.0, *empty.TotalAccountCost)
}
