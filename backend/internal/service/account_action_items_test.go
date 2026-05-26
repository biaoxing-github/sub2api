package service

import (
	"testing"
	"time"
)

func TestBuildAccountActionItemsCoversSchedulingAndBalanceReasons(t *testing.T) {
	now := time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC)
	resetAt := now.Add(30 * time.Minute)
	tempUntil := now.Add(15 * time.Minute)

	accounts := []Account{
		{ID: 1, Name: "manual-off", Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: false},
		{ID: 2, Name: "broken", Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusError, Schedulable: true, ErrorMessage: "401"},
		{ID: 3, Name: "rate", Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, RateLimitResetAt: &resetAt},
		{ID: 4, Name: "temp", Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, TempUnschedulableUntil: &tempUntil},
		{ID: 5, Name: "missing-balance", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Extra: map[string]any{}},
		{ID: 6, Name: "draining", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Extra: map[string]any{
			UpstreamBalanceUpdatedAtKey: now.Format(time.RFC3339),
			UpstreamBalanceAvailableKey: 1.2,
			UpstreamBalanceOKCountKey:   1,
		}},
		{ID: 7, Name: "exhausted", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Extra: map[string]any{
			UpstreamBalanceUpdatedAtKey: now.Format(time.RFC3339),
			UpstreamBalanceAvailableKey: 0.2,
			UpstreamBalanceOKCountKey:   1,
		}},
	}

	result := BuildAccountActionItems(accounts, now)
	reasons := map[string]string{}
	for _, item := range result.Items {
		reasons[item.Reason] = item.Severity
	}

	want := map[string]string{
		"schedulable_disabled":       "critical",
		"account_error":              "critical",
		"rate_limited":               "warning",
		"temp_unschedulable":         "warning",
		"missing_balance_snapshot":   "warning",
		"upstream_balance_draining":  "warning",
		"upstream_balance_exhausted": "critical",
	}
	for reason, severity := range want {
		if got := reasons[reason]; got != severity {
			t.Fatalf("reason %s severity = %q, want %q; items=%#v", reason, got, severity, result.Items)
		}
	}
	if result.Counts.Critical != 3 {
		t.Fatalf("critical count = %d, want 3", result.Counts.Critical)
	}
	if result.Counts.Warning != 4 {
		t.Fatalf("warning count = %d, want 4", result.Counts.Warning)
	}
}

func TestBuildAccountDashboardSummaryCountsUnschedulableByRawSchedulableFlag(t *testing.T) {
	now := time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour)

	accounts := []Account{
		{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: false},
		{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusError, Schedulable: false},
		{ID: 3, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, RateLimitResetAt: &future},
		{ID: 4, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusDisabled, Schedulable: true, TempUnschedulableUntil: &future},
		{ID: 5, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Extra: map[string]any{}},
	}

	summary := BuildAccountDashboardSummary(accounts, nil, now)

	if summary.StatusSummary.Total != len(accounts) {
		t.Fatalf("total = %d, want unique account count %d", summary.StatusSummary.Total, len(accounts))
	}
	if summary.StatusSummary.Unschedulable != 2 {
		t.Fatalf("unschedulable = %d, want raw schedulable=false count 2", summary.StatusSummary.Unschedulable)
	}
	if summary.StatusSummary.Active != 3 || summary.StatusSummary.Error != 1 || summary.StatusSummary.Inactive != 1 {
		t.Fatalf("status summary = %#v", summary.StatusSummary)
	}
	if summary.StatusSummary.RateLimited != 1 || summary.StatusSummary.TempUnschedulable != 1 {
		t.Fatalf("runtime status summary = %#v", summary.StatusSummary)
	}
	if summary.BalanceSummary.MissingSnapshot != 1 {
		t.Fatalf("missing snapshot = %d, want 1", summary.BalanceSummary.MissingSnapshot)
	}
}
