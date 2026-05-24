package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

type accountUsageSummaryStatsRepo struct {
	stats map[time.Duration]map[int64]*usagestats.AccountStats
}

func (r *accountUsageSummaryStatsRepo) GetAccountWindowStatsBatch(_ context.Context, accountIDs []int64, startTime time.Time) (map[int64]*usagestats.AccountStats, error) {
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

func TestBuildAccountUsageSummaryGroupsByPlanAndAccountType(t *testing.T) {
	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)
	reset5h := now.Add(3 * time.Hour).Format(time.RFC3339)
	reset7d := now.Add(5 * 24 * time.Hour).Format(time.RFC3339)
	rateLimitedUntil := time.Date(2099, 3, 16, 13, 0, 0, 0, time.UTC)

	accounts := []Account{
		{
			ID:               1,
			Platform:         PlatformOpenAI,
			Type:             AccountTypeOAuth,
			Status:           StatusActive,
			Schedulable:      true,
			Credentials:      map[string]any{"plan_type": "free"},
			RateLimitResetAt: nil,
			Extra: map[string]any{
				"codex_7d_used_percent":  25.0,
				"codex_7d_reset_at":      reset7d,
				"codex_usage_updated_at": now.Add(-10 * time.Minute).Format(time.RFC3339),
			},
		},
		{
			ID:          2,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: true,
			Credentials: map[string]any{"plan_type": "free"},
			Extra: map[string]any{
				"codex_7d_used_percent":  50.0,
				"codex_7d_reset_at":      reset7d,
				"codex_usage_updated_at": now.Add(-20 * time.Minute).Format(time.RFC3339),
			},
		},
		{
			ID:               3,
			Platform:         PlatformOpenAI,
			Type:             AccountTypeSetupToken,
			Status:           StatusActive,
			Schedulable:      false,
			RateLimitResetAt: &rateLimitedUntil,
			Credentials:      map[string]any{"plan_type": "plus"},
			Extra: map[string]any{
				"codex_5h_used_percent":  80.0,
				"codex_5h_reset_at":      reset5h,
				"codex_usage_updated_at": now.Add(-30 * time.Minute).Format(time.RFC3339),
			},
		},
		{
			ID:          4,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: true,
			Credentials: map[string]any{"plan_type": "free"},
			Extra:       map[string]any{},
		},
		{
			ID:          5,
			Platform:    PlatformAnthropic,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: true,
			Credentials: map[string]any{"plan_type": "free"},
		},
	}

	repo := &accountUsageSummaryStatsRepo{stats: map[time.Duration]map[int64]*usagestats.AccountStats{
		5 * time.Hour: {
			1: {Requests: 10, Cost: 10, StandardCost: 10, UserCost: 12},
			2: {Requests: 20, Cost: 20, StandardCost: 20, UserCost: 25},
			3: {Requests: 30, Cost: 8, StandardCost: 8, UserCost: 9},
		},
		7 * 24 * time.Hour: {
			1: {Requests: 100, Cost: 40, StandardCost: 40, UserCost: 44},
			2: {Requests: 200, Cost: 60, StandardCost: 60, UserCost: 66},
		},
	}}

	summary, err := BuildAccountUsageSummary(context.Background(), accounts, repo, now)
	if err != nil {
		t.Fatalf("BuildAccountUsageSummary() error = %v", err)
	}

	if summary.TotalAccounts != 4 {
		t.Fatalf("TotalAccounts = %d, want 4", summary.TotalAccounts)
	}
	if summary.MissingSnapshotAccounts != 1 {
		t.Fatalf("MissingSnapshotAccounts = %d, want 1", summary.MissingSnapshotAccounts)
	}
	if len(summary.Plans) != 2 {
		t.Fatalf("plans len = %d, want 2", len(summary.Plans))
	}

	free := summary.Plans[0]
	if free.PlanType != "free" {
		t.Fatalf("first plan = %q, want free", free.PlanType)
	}
	if free.AccountCount != 3 || free.SchedulableCount != 3 || free.MissingSnapshotCount != 1 {
		t.Fatalf("free counts = %#v", free)
	}
	if got, want := free.FiveHour.UsedCost, 0.0; got != want {
		t.Fatalf("free 5h used cost = %v, want %v", got, want)
	}
	if got, want := free.FiveHour.EstimatedLimitCost, 0.0; got != want {
		t.Fatalf("free 5h estimated limit = %v, want %v", got, want)
	}
	if got, want := free.FiveHour.Utilization, 0.0; got != want {
		t.Fatalf("free 5h utilization = %v, want %v", got, want)
	}
	if got, want := free.FiveHour.UsedPercentSum, 0.0; got != want {
		t.Fatalf("free 5h used percent sum = %v, want %v", got, want)
	}
	if got, want := free.FiveHour.RemainingPercentSum, 0.0; got != want {
		t.Fatalf("free 5h remaining percent sum = %v, want %v", got, want)
	}
	if got, want := free.SevenDay.RemainingPercentSum, 225.0; got != want {
		t.Fatalf("free 7d remaining percent sum = %v, want %v", got, want)
	}
	if got, want := free.SevenDay.EstimatedLimitCost, 280.0; got != want {
		t.Fatalf("free 7d estimated limit = %v, want %v", got, want)
	}
	if free.Types[0].AccountType != AccountTypeOAuth || free.Types[0].AccountCount != 3 {
		t.Fatalf("free type aggregate = %#v", free.Types)
	}

	plus := summary.Plans[1]
	if plus.PlanType != "plus" {
		t.Fatalf("second plan = %q, want plus", plus.PlanType)
	}
	if plus.Types[0].AccountType != AccountTypeSetupToken {
		t.Fatalf("plus type = %q, want setup-token", plus.Types[0].AccountType)
	}
	if plus.RateLimitedCount != 1 {
		t.Fatalf("plus rate limited count = %d, want 1", plus.RateLimitedCount)
	}
	if got, want := plus.FiveHour.EstimatedLimitCost, 10.0; got != want {
		t.Fatalf("plus 5h estimated limit = %v, want %v", got, want)
	}
	if got, want := plus.FiveHour.UsedPercentSum, 80.0; got != want {
		t.Fatalf("plus 5h used percent sum = %v, want %v", got, want)
	}
	if got, want := plus.FiveHour.RemainingPercentSum, 20.0; got != want {
		t.Fatalf("plus 5h remaining percent sum = %v, want %v", got, want)
	}
	if plus.SevenDay.AccountsWithLimitEstimate != 0 {
		t.Fatalf("plus 7d accounts with limit = %d, want 0", plus.SevenDay.AccountsWithLimitEstimate)
	}
}

func TestBuildAccountUsageSummaryClassifiesAPIKeyPlan(t *testing.T) {
	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)
	account := Account{
		ID:          10,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Extra: map[string]any{
			"codex_5h_used_percent": 50.0,
			"codex_5h_reset_at":     now.Add(time.Hour).Format(time.RFC3339),
		},
	}
	repo := &accountUsageSummaryStatsRepo{stats: map[time.Duration]map[int64]*usagestats.AccountStats{
		5 * time.Hour: {
			10: {Requests: 5, Cost: 2},
		},
	}}

	summary, err := BuildAccountUsageSummary(context.Background(), []Account{account}, repo, now)
	if err != nil {
		t.Fatalf("BuildAccountUsageSummary() error = %v", err)
	}
	if len(summary.Plans) != 1 {
		t.Fatalf("plans len = %d, want 1", len(summary.Plans))
	}
	if summary.Plans[0].PlanType != "api_key" {
		t.Fatalf("plan type = %q, want api_key", summary.Plans[0].PlanType)
	}
	if summary.Plans[0].PlanLabel != "API Key" {
		t.Fatalf("plan label = %q, want API Key", summary.Plans[0].PlanLabel)
	}
	if len(summary.Plans[0].Types) != 1 || summary.Plans[0].Types[0].AccountType != AccountTypeAPIKey {
		t.Fatalf("type aggregate = %#v, want API Key", summary.Plans[0].Types)
	}
}

func TestBuildAccountUsageSummaryExcludesAPIKeyFromCodexWindows(t *testing.T) {
	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)
	reset5h := now.Add(time.Hour).Format(time.RFC3339)
	reset7d := now.Add(24 * time.Hour).Format(time.RFC3339)
	accounts := []Account{
		{
			ID:          10,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Extra: map[string]any{
				"codex_5h_used_percent": 90.0,
				"codex_5h_reset_at":     reset5h,
				"codex_7d_used_percent": 80.0,
				"codex_7d_reset_at":     reset7d,
			},
		},
		{
			ID:          11,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: true,
			Credentials: map[string]any{"plan_type": "plus"},
			Extra: map[string]any{
				"codex_5h_used_percent": 25.0,
				"codex_5h_reset_at":     reset5h,
				"codex_7d_used_percent": 50.0,
				"codex_7d_reset_at":     reset7d,
			},
		},
	}
	repo := &accountUsageSummaryStatsRepo{stats: map[time.Duration]map[int64]*usagestats.AccountStats{
		5 * time.Hour: {
			10: {Requests: 100, Cost: 100},
			11: {Requests: 10, Cost: 10},
		},
		7 * 24 * time.Hour: {
			10: {Requests: 700, Cost: 700},
			11: {Requests: 70, Cost: 70},
		},
	}}

	summary, err := BuildAccountUsageSummary(context.Background(), accounts, repo, now)
	if err != nil {
		t.Fatalf("BuildAccountUsageSummary() error = %v", err)
	}

	if got, want := summary.FiveHour.UsedCost, 10.0; got != want {
		t.Fatalf("summary 5h used cost = %v, want %v", got, want)
	}
	if got, want := summary.FiveHour.UsedPercentSum, 25.0; got != want {
		t.Fatalf("summary 5h used percent = %v, want %v", got, want)
	}
	if got, want := summary.FiveHour.RemainingPercentSum, 75.0; got != want {
		t.Fatalf("summary 5h remaining percent = %v, want %v", got, want)
	}
	if got, want := summary.SevenDay.UsedCost, 70.0; got != want {
		t.Fatalf("summary 7d used cost = %v, want %v", got, want)
	}
	if got, want := summary.SevenDay.UsedPercentSum, 50.0; got != want {
		t.Fatalf("summary 7d used percent = %v, want %v", got, want)
	}
	if got, want := summary.SevenDay.RemainingPercentSum, 50.0; got != want {
		t.Fatalf("summary 7d remaining percent = %v, want %v", got, want)
	}
	if len(summary.Plans) != 2 {
		t.Fatalf("plans len = %d, want 2", len(summary.Plans))
	}
	apiKeyPlan := summary.Plans[1]
	if apiKeyPlan.PlanType != "api_key" {
		t.Fatalf("second plan = %q, want api_key", apiKeyPlan.PlanType)
	}
	if got := apiKeyPlan.FiveHour.UsedCost; got != 0 {
		t.Fatalf("api key 5h used cost = %v, want 0", got)
	}
	if got := apiKeyPlan.SevenDay.UsedPercentSum; got != 0 {
		t.Fatalf("api key 7d used percent = %v, want 0", got)
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
	if len(summary.Plans) != 2 {
		t.Fatalf("plans len = %d, want 2", len(summary.Plans))
	}
	plus := summary.Plans[0]
	if plus.PlanType != "plus" {
		t.Fatalf("first plan = %q, want plus", plus.PlanType)
	}
	if got, want := plus.MissingCodexSnapshotCount, 1; got != want {
		t.Fatalf("plus MissingCodexSnapshotCount = %d, want %d", got, want)
	}
	apiKey := summary.Plans[1]
	if apiKey.PlanType != "api_key" {
		t.Fatalf("second plan = %q, want api_key", apiKey.PlanType)
	}
	if got := apiKey.MissingCodexSnapshotCount; got != 0 {
		t.Fatalf("api key MissingCodexSnapshotCount = %d, want 0", got)
	}
	if got, want := apiKey.UpstreamBalance.MissingAccounts, 1; got != want {
		t.Fatalf("api key upstream missing accounts = %d, want %d", got, want)
	}
}

func TestBuildAccountUsageSummaryAllowsPercentSumsBeyondOneHundred(t *testing.T) {
	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)
	accounts := []Account{
		{
			ID:          21,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: true,
			Credentials: map[string]any{"plan_type": "plus"},
			Extra: map[string]any{
				"codex_5h_used_percent":  120.0,
				"codex_5h_reset_at":      now.Add(time.Hour).Format(time.RFC3339),
				"codex_7d_used_percent":  75.0,
				"codex_7d_reset_at":      now.Add(24 * time.Hour).Format(time.RFC3339),
				"codex_usage_updated_at": now.Format(time.RFC3339),
			},
		},
		{
			ID:          22,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: true,
			Credentials: map[string]any{"plan_type": "plus"},
			Extra: map[string]any{
				"codex_5h_used_percent":  60.0,
				"codex_5h_reset_at":      now.Add(time.Hour).Format(time.RFC3339),
				"codex_7d_used_percent":  50.0,
				"codex_7d_reset_at":      now.Add(24 * time.Hour).Format(time.RFC3339),
				"codex_usage_updated_at": now.Format(time.RFC3339),
			},
		},
	}

	summary, err := BuildAccountUsageSummary(context.Background(), accounts, nil, now)
	if err != nil {
		t.Fatalf("BuildAccountUsageSummary() error = %v", err)
	}
	if got, want := summary.FiveHour.UsedPercentSum, 180.0; got != want {
		t.Fatalf("5h used percent sum = %v, want %v", got, want)
	}
	if got, want := summary.FiveHour.RemainingPercentSum, 20.0; got != want {
		t.Fatalf("5h remaining percent sum = %v, want %v", got, want)
	}
	if got, want := summary.SevenDay.UsedPercentSum, 125.0; got != want {
		t.Fatalf("7d used percent sum = %v, want %v", got, want)
	}
	if got, want := summary.SevenDay.RemainingPercentSum, 75.0; got != want {
		t.Fatalf("7d remaining percent sum = %v, want %v", got, want)
	}
}

func TestBuildAccountUsageSummaryIgnoresFreeFiveHourWindow(t *testing.T) {
	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)
	account := Account{
		ID:          31,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{"plan_type": "free"},
		Extra: map[string]any{
			"codex_5h_used_percent": 90.0,
			"codex_5h_reset_at":     now.Add(time.Hour).Format(time.RFC3339),
			"codex_7d_used_percent": 40.0,
			"codex_7d_reset_at":     now.Add(24 * time.Hour).Format(time.RFC3339),
		},
	}
	repo := &accountUsageSummaryStatsRepo{stats: map[time.Duration]map[int64]*usagestats.AccountStats{
		5 * time.Hour: {
			31: {Requests: 5, Cost: 3},
		},
		7 * 24 * time.Hour: {
			31: {Requests: 7, Cost: 4},
		},
	}}

	summary, err := BuildAccountUsageSummary(context.Background(), []Account{account}, repo, now)
	if err != nil {
		t.Fatalf("BuildAccountUsageSummary() error = %v", err)
	}
	if got := summary.FiveHour.AccountsWithSnapshot; got != 0 {
		t.Fatalf("summary 5h snapshots = %d, want 0", got)
	}
	if got := summary.FiveHour.UsedPercentSum; got != 0 {
		t.Fatalf("summary 5h used percent = %v, want 0", got)
	}
	if got := summary.FiveHour.UsedCost; got != 0 {
		t.Fatalf("summary 5h used cost = %v, want 0", got)
	}
	if got, want := summary.SevenDay.UsedPercentSum, 40.0; got != want {
		t.Fatalf("summary 7d used percent = %v, want %v", got, want)
	}
	if got, want := summary.SevenDay.RemainingPercentSum, 60.0; got != want {
		t.Fatalf("summary 7d remaining percent = %v, want %v", got, want)
	}
	if len(summary.Plans) != 1 {
		t.Fatalf("plans len = %d, want 1", len(summary.Plans))
	}
	if got := summary.Plans[0].FiveHour.AccountsWithSnapshot; got != 0 {
		t.Fatalf("free plan 5h snapshots = %d, want 0", got)
	}
	if got, want := summary.Plans[0].SevenDay.UsedPercentSum, 40.0; got != want {
		t.Fatalf("free plan 7d used percent = %v, want %v", got, want)
	}
}

func TestBuildAccountUsageSummaryCountsUnsampledAccountsAsRemainingCapacity(t *testing.T) {
	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)
	accounts := []Account{
		{
			ID:          41,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: true,
			Credentials: map[string]any{"plan_type": "plus"},
			Extra: map[string]any{
				"codex_5h_used_percent": 30.0,
				"codex_5h_reset_at":     now.Add(time.Hour).Format(time.RFC3339),
			},
		},
		{
			ID:          42,
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
	if got, want := summary.FiveHour.AccountsInWindow, 2; got != want {
		t.Fatalf("5h accounts in window = %d, want %d", got, want)
	}
	if got, want := summary.FiveHour.UsedPercentSum, 30.0; got != want {
		t.Fatalf("5h used percent = %v, want %v", got, want)
	}
	if got, want := summary.FiveHour.RemainingPercentSum, 170.0; got != want {
		t.Fatalf("5h remaining percent = %v, want %v", got, want)
	}
}
