package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
)

type grokUsageLogRepoStub struct {
	UsageLogRepository
	stats *usagestats.AccountStats
}

// GetAccountTodayStats 返回测试指定的 Grok 账号当日用量。
func (r *grokUsageLogRepoStub) GetAccountTodayStats(_ context.Context, _ int64) (*usagestats.AccountStats, error) {
	return r.stats, nil
}

type accountUsageCodexProbeRepo struct {
	stubOpenAIAccountRepo
	updateExtraCh chan map[string]any
	rateLimitCh   chan time.Time
}

func (r *accountUsageCodexProbeRepo) UpdateExtra(_ context.Context, _ int64, updates map[string]any) error {
	if r.updateExtraCh != nil {
		copied := make(map[string]any, len(updates))
		for k, v := range updates {
			copied[k] = v
		}
		r.updateExtraCh <- copied
	}
	return nil
}

func (r *accountUsageCodexProbeRepo) SetRateLimited(_ context.Context, _ int64, resetAt time.Time) error {
	if r.rateLimitCh != nil {
		r.rateLimitCh <- resetAt
	}
	return nil
}

func TestShouldRefreshOpenAICodexSnapshot(t *testing.T) {
	t.Parallel()

	rateLimitedUntil := time.Now().Add(5 * time.Minute)
	now := time.Now()
	usage := &UsageInfo{
		FiveHour: &UsageProgress{Utilization: 0},
		SevenDay: &UsageProgress{Utilization: 0},
	}

	if !shouldRefreshOpenAICodexSnapshot(&Account{RateLimitResetAt: &rateLimitedUntil}, usage, now) {
		t.Fatal("expected rate-limited account to force codex snapshot refresh")
	}

	if shouldRefreshOpenAICodexSnapshot(&Account{}, usage, now) {
		t.Fatal("expected complete non-rate-limited usage to skip codex snapshot refresh")
	}

	if !shouldRefreshOpenAICodexSnapshot(&Account{}, &UsageInfo{FiveHour: nil, SevenDay: &UsageProgress{}}, now) {
		t.Fatal("expected missing 5h snapshot to require refresh")
	}

	staleAt := now.Add(-(openAIProbeCacheTTL + time.Minute)).Format(time.RFC3339)
	if !shouldRefreshOpenAICodexSnapshot(&Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"openai_oauth_responses_websockets_v2_enabled": true,
			"codex_usage_updated_at":                       staleAt,
		},
	}, usage, now) {
		t.Fatal("expected stale ws snapshot to trigger refresh")
	}
}

// TestAccountUsageService_GetGrokUsage_UsesPassiveQuotaAndTodayStats 验证被动配额快照和本地费用同时返回。
func TestAccountUsageService_GetGrokUsage_UsesPassiveQuotaAndTodayStats(t *testing.T) {
	t.Parallel()

	limit := int64(10)
	remaining := int64(-5)
	svc := &AccountUsageService{usageLogRepo: &grokUsageLogRepoStub{stats: &usagestats.AccountStats{
		Requests:     12,
		Tokens:       3456,
		Cost:         0.05,
		StandardCost: 0.04,
		UserCost:     0.09,
	}}}
	account := &Account{
		ID:       5001,
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			grokQuotaSnapshotExtraKey: &xai.QuotaSnapshot{
				Requests: &xai.QuotaWindow{
					Limit:     &limit,
					Remaining: &remaining,
					ResetAt:   "2026-07-10T12:00:00Z",
				},
				HeadersObserved:   true,
				ObservationSource: "images",
				UpdatedAt:         "2026-07-10T11:00:00Z",
			},
		},
	}

	usage, err := svc.getGrokUsage(context.Background(), account)
	if err != nil {
		t.Fatalf("getGrokUsage() error = %v", err)
	}
	if usage.GrokQuotaSnapshotState != "observed" || usage.GrokRequestQuota == nil {
		t.Fatalf("expected observed request quota, got %#v", usage)
	}
	if usage.GrokRequestQuota.Remaining == nil || *usage.GrokRequestQuota.Remaining != -5 {
		t.Fatalf("expected remaining=-5, got %#v", usage.GrokRequestQuota.Remaining)
	}
	if usage.GrokLocalUsage == nil || usage.GrokLocalUsage.UserCost != 0.09 {
		t.Fatalf("expected local user cost 0.09, got %#v", usage.GrokLocalUsage)
	}
}

func TestExtractOpenAICodexProbeUpdatesAccepts429WithCodexHeaders(t *testing.T) {
	t.Parallel()

	headers := make(http.Header)
	headers.Set("x-codex-primary-used-percent", "100")
	headers.Set("x-codex-primary-reset-after-seconds", "604800")
	headers.Set("x-codex-primary-window-minutes", "10080")
	headers.Set("x-codex-secondary-used-percent", "100")
	headers.Set("x-codex-secondary-reset-after-seconds", "18000")
	headers.Set("x-codex-secondary-window-minutes", "300")

	updates, err := extractOpenAICodexProbeUpdates(&http.Response{StatusCode: http.StatusTooManyRequests, Header: headers})
	if err != nil {
		t.Fatalf("extractOpenAICodexProbeUpdates() error = %v", err)
	}
	if len(updates) == 0 {
		t.Fatal("expected codex probe updates from 429 headers")
	}
	if got := updates["codex_5h_used_percent"]; got != 100.0 {
		t.Fatalf("codex_5h_used_percent = %v, want 100", got)
	}
	if got := updates["codex_7d_used_percent"]; got != 100.0 {
		t.Fatalf("codex_7d_used_percent = %v, want 100", got)
	}
}

func TestAccountUsageService_PersistOpenAICodexProbeSnapshotSetsRateLimitWhenExhausted(t *testing.T) {
	t.Parallel()

	repo := &accountUsageCodexProbeRepo{
		stubOpenAIAccountRepo: stubOpenAIAccountRepo{accounts: []Account{{
			ID:       321,
			Platform: PlatformOpenAI,
			Type:     AccountTypeOAuth,
		}}},
		updateExtraCh: make(chan map[string]any, 1),
		rateLimitCh:   make(chan time.Time, 1),
	}
	svc := &AccountUsageService{accountRepo: repo}
	svc.persistOpenAICodexProbeSnapshot(321, map[string]any{
		"codex_7d_used_percent": 100.0,
		"codex_7d_reset_at":     time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second).Format(time.RFC3339),
	})

	select {
	case updates := <-repo.updateExtraCh:
		if got := updates["codex_7d_used_percent"]; got != 100.0 {
			t.Fatalf("codex_7d_used_percent = %v, want 100", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("等待 codex 探测快照写入 extra 超时")
	}

	select {
	case got := <-repo.rateLimitCh:
		if !got.After(time.Now()) {
			t.Fatalf("expected future rate limit reset, got %v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("等待 codex 探测快照提升为运行时限流状态超时")
	}
}

func TestAccountUsageService_PersistOpenAICodexProbeSnapshotDoesNotRateLimitAPIKey(t *testing.T) {
	t.Parallel()

	repo := &accountUsageCodexProbeRepo{
		stubOpenAIAccountRepo: stubOpenAIAccountRepo{accounts: []Account{{
			ID:       322,
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
		}}},
		updateExtraCh: make(chan map[string]any, 1),
		rateLimitCh:   make(chan time.Time, 1),
	}
	svc := &AccountUsageService{accountRepo: repo}
	svc.persistOpenAICodexProbeSnapshot(322, map[string]any{
		"codex_7d_used_percent": 100.0,
		"codex_7d_reset_at":     time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second).Format(time.RFC3339),
	})

	select {
	case <-repo.updateExtraCh:
	case <-time.After(2 * time.Second):
		t.Fatal("等待 codex 探测快照写入 extra 超时")
	}

	select {
	case got := <-repo.rateLimitCh:
		t.Fatalf("api_key 账号不应因 codex 快照百分比进入运行时限流: %v", got)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestAccountUsageService_GetOpenAIUsage_DoesNotPromoteCodexExtraToRateLimit(t *testing.T) {
	t.Parallel()

	resetAt := time.Now().Add(6 * 24 * time.Hour).UTC().Truncate(time.Second)
	repo := &accountUsageCodexProbeRepo{
		rateLimitCh: make(chan time.Time, 1),
	}
	svc := &AccountUsageService{accountRepo: repo}
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"codex_5h_used_percent": 1.0,
			"codex_5h_reset_at":     time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second).Format(time.RFC3339),
			"codex_7d_used_percent": 100.0,
			"codex_7d_reset_at":     resetAt.Format(time.RFC3339),
		},
	}

	usage, err := svc.getOpenAIUsage(context.Background(), account, false)
	if err != nil {
		t.Fatalf("getOpenAIUsage() error = %v", err)
	}
	if usage.SevenDay == nil || usage.SevenDay.Utilization != 100.0 {
		t.Fatalf("预期 7 天用量仍然可见，实际为 %#v", usage.SevenDay)
	}
	if reset := account.EffectiveRateLimitResetAt(); reset == nil || reset.Before(time.Now()) {
		t.Fatalf("预期已耗尽 codex extra 在运行时表现为限流，实际 reset=%v", reset)
	}
	select {
	case got := <-repo.rateLimitCh:
		t.Fatalf("不应将已耗尽的 codex extra 持久化为运行时限流状态: %v", got)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestBuildCodexUsageProgressFromExtra_ZerosExpiredWindow(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)

	t.Run("expired 5h window zeroes utilization", func(t *testing.T) {
		extra := map[string]any{
			"codex_5h_used_percent": 42.0,
			"codex_5h_reset_at":     "2026-03-16T10:00:00Z", // 2h ago
		}
		progress := buildCodexUsageProgressFromExtra(extra, "5h", now)
		if progress == nil {
			t.Fatal("expected non-nil progress")
		}
		if progress.Utilization != 0 {
			t.Fatalf("expected Utilization=0 for expired window, got %v", progress.Utilization)
		}
		if progress.RemainingSeconds != 0 {
			t.Fatalf("expected RemainingSeconds=0, got %v", progress.RemainingSeconds)
		}
		if progress.ResetsAt != nil {
			t.Fatalf("expected expired ResetsAt to be cleared, got %v", progress.ResetsAt)
		}
	})

	t.Run("active 5h window keeps utilization", func(t *testing.T) {
		resetAt := now.Add(2 * time.Hour).Format(time.RFC3339)
		extra := map[string]any{
			"codex_5h_used_percent": 42.0,
			"codex_5h_reset_at":     resetAt,
		}
		progress := buildCodexUsageProgressFromExtra(extra, "5h", now)
		if progress == nil {
			t.Fatal("expected non-nil progress")
		}
		if progress.Utilization != 42.0 {
			t.Fatalf("expected Utilization=42, got %v", progress.Utilization)
		}
	})

	t.Run("expired 7d window zeroes utilization", func(t *testing.T) {
		extra := map[string]any{
			"codex_7d_used_percent": 88.0,
			"codex_7d_reset_at":     "2026-03-15T00:00:00Z", // yesterday
		}
		progress := buildCodexUsageProgressFromExtra(extra, "7d", now)
		if progress == nil {
			t.Fatal("expected non-nil progress")
		}
		if progress.Utilization != 0 {
			t.Fatalf("expected Utilization=0 for expired 7d window, got %v", progress.Utilization)
		}
	})
}

func TestAccountUsageService_EstimateSetupTokenUsageZerosExpiredSessionWindow(t *testing.T) {
	t.Parallel()

	expiredEnd := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
	account := &Account{
		Type:             AccountTypeSetupToken,
		SessionWindowEnd: &expiredEnd,
		Extra: map[string]any{
			"session_window_utilization": 0.72,
		},
	}

	usage := (&AccountUsageService{}).estimateSetupTokenUsage(account)

	if usage == nil || usage.FiveHour == nil {
		t.Fatal("expected setup token 5h usage progress")
	}
	if usage.FiveHour.Utilization != 0 {
		t.Fatalf("expected expired setup token 5h utilization to be zero, got %v", usage.FiveHour.Utilization)
	}
	if usage.FiveHour.RemainingSeconds != 0 {
		t.Fatalf("expected expired setup token remaining seconds to be zero, got %v", usage.FiveHour.RemainingSeconds)
	}
	if usage.FiveHour.ResetsAt != nil {
		t.Fatalf("expected expired setup token ResetsAt to be cleared, got %v", usage.FiveHour.ResetsAt)
	}
}
