package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

type accountUsageSummaryStatsRepo struct {
	stats map[time.Duration]map[int64]*usagestats.AccountStats
	calls int
}

func (r *accountUsageSummaryStatsRepo) GetAccountWindowStatsBatch(_ context.Context, accountIDs []int64, startTime time.Time) (map[int64]*usagestats.AccountStats, error) {
	r.calls++
	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)
	window := now.Sub(startTime)
	var source map[int64]*usagestats.AccountStats
	if r != nil && r.stats != nil {
		source = r.stats[window]
	}
	out := make(map[int64]*usagestats.AccountStats, len(accountIDs))
	for _, id := range accountIDs {
		if source != nil {
			out[id] = source[id]
		}
	}
	return out, nil
}

func TestBuildAccountUsageSummaryDoesNotReadFiveHourOrSevenDayWindows(t *testing.T) {
	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)
	repo := &accountUsageSummaryStatsRepo{stats: map[time.Duration]map[int64]*usagestats.AccountStats{
		5 * time.Hour: {
			1: {Requests: 10, Cost: 10},
		},
		7 * 24 * time.Hour: {
			1: {Requests: 70, Cost: 70},
		},
	}}
	accounts := []Account{
		{
			ID:          1,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: true,
			Credentials: map[string]any{"plan_type": "plus"},
			Extra: map[string]any{
				"codex_5h_used_percent": 90.0,
				"codex_5h_reset_at":     now.Add(time.Hour).Format(time.RFC3339),
				"codex_7d_used_percent": 80.0,
				"codex_7d_reset_at":     now.Add(24 * time.Hour).Format(time.RFC3339),
			},
		},
	}

	summary, err := BuildAccountUsageSummary(context.Background(), accounts, repo, now)
	if err != nil {
		t.Fatalf("BuildAccountUsageSummary() error = %v", err)
	}

	if repo.calls != 0 {
		t.Fatalf("window stats calls = %d, want 0", repo.calls)
	}
	if summary.FiveHour.Requests != 0 || summary.SevenDay.Requests != 0 {
		t.Fatalf("legacy windows should stay empty, got 5h=%#v 7d=%#v", summary.FiveHour, summary.SevenDay)
	}
	if len(summary.Plans) != 0 {
		t.Fatalf("plans len = %d, want 0 because plan quota breakdown is no longer calculated", len(summary.Plans))
	}
}

func TestBuildAccountUsageSummaryCountsAllSystemAccountsAndSplitsProviderBalances(t *testing.T) {
	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)
	openAIUpdatedAt := now.Add(-10 * time.Minute).Format(time.RFC3339)
	anthropicUpdatedAt := now.Add(-5 * time.Minute).Format(time.RFC3339)
	accounts := []Account{
		{
			ID:          1,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Extra: map[string]any{
				UpstreamBalanceAvailableKey: 10.0,
				UpstreamBalanceUsedKey:      2.0,
				UpstreamBalanceTotalKey:     12.0,
				UpstreamBalanceUpdatedAtKey: openAIUpdatedAt,
				UpstreamBalanceKeyCountKey:  1,
				UpstreamBalanceOKCountKey:   1,
			},
		},
		{
			ID:          2,
			Platform:    PlatformAnthropic,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Extra: map[string]any{
				UpstreamBalanceAvailableKey: 20.0,
				UpstreamBalanceUsedKey:      5.0,
				UpstreamBalanceTotalKey:     25.0,
				UpstreamBalanceUpdatedAtKey: anthropicUpdatedAt,
				UpstreamBalanceKeyCountKey:  2,
				UpstreamBalanceOKCountKey:   2,
			},
		},
		{
			ID:          3,
			Platform:    PlatformGemini,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: false,
		},
	}

	summary, err := BuildAccountUsageSummary(context.Background(), accounts, nil, now)
	if err != nil {
		t.Fatalf("BuildAccountUsageSummary() error = %v", err)
	}

	if summary.TotalAccounts != 3 {
		t.Fatalf("total accounts = %d, want all system accounts 3", summary.TotalAccounts)
	}
	if summary.SchedulableAccounts != 2 {
		t.Fatalf("schedulable accounts = %d, want 2", summary.SchedulableAccounts)
	}
	if summary.OpenAIUpstreamBalance.Available != 10 || summary.OpenAIUpstreamBalance.Total != 12 {
		t.Fatalf("openai balance = %#v", summary.OpenAIUpstreamBalance)
	}
	if summary.AnthropicUpstreamBalance.Available != 20 || summary.AnthropicUpstreamBalance.Total != 25 {
		t.Fatalf("anthropic balance = %#v", summary.AnthropicUpstreamBalance)
	}
	if summary.UpstreamBalance.Available != 30 || summary.UpstreamBalance.Total != 37 {
		t.Fatalf("legacy total balance = %#v", summary.UpstreamBalance)
	}
}

func TestBuildAccountUsageSummarySeparatesCodexAndUpstreamMissingSnapshots(t *testing.T) {
	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)
	accounts := []Account{
		{
			ID:          10,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Extra:       map[string]any{},
		},
		{
			ID:          11,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: true,
			Credentials: map[string]any{"plan_type": "plus"},
			Extra:       map[string]any{},
		},
	}

	summary, err := BuildAccountUsageSummary(context.Background(), accounts, nil, now)
	if err != nil {
		t.Fatalf("BuildAccountUsageSummary() error = %v", err)
	}

	if got, want := summary.MissingCodexSnapshotAccounts, 1; got != want {
		t.Fatalf("MissingCodexSnapshotAccounts = %d, want %d", got, want)
	}
	if got, want := summary.MissingSnapshotAccounts, 1; got != want {
		t.Fatalf("legacy MissingSnapshotAccounts = %d, want %d", got, want)
	}
	if got, want := summary.UpstreamBalance.MissingAccounts, 1; got != want {
		t.Fatalf("upstream missing accounts = %d, want %d", got, want)
	}
	if got, want := summary.OpenAIUpstreamBalance.MissingAccounts, 1; got != want {
		t.Fatalf("openai upstream missing accounts = %d, want %d", got, want)
	}
	if len(summary.Plans) != 0 {
		t.Fatalf("plans len = %d, want 0", len(summary.Plans))
	}
}
