package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

type AccountUsageSummaryWindow struct {
	UsedCost                  float64    `json:"used_cost"`
	EstimatedLimitCost        float64    `json:"estimated_limit_cost"`
	Utilization               float64    `json:"utilization"`
	UsedPercentSum            float64    `json:"used_percent_sum"`
	RemainingPercentSum       float64    `json:"remaining_percent_sum"`
	AccountsInWindow          int        `json:"accounts_in_window"`
	Requests                  int64      `json:"requests"`
	AccountsWithSnapshot      int        `json:"accounts_with_snapshot"`
	AccountsWithLimitEstimate int        `json:"accounts_with_limit_estimate"`
	EarliestResetAt           *time.Time `json:"earliest_reset_at,omitempty"`
}

type AccountUsageSummaryGroup struct {
	PlanType             string                     `json:"plan_type"`
	PlanLabel            string                     `json:"plan_label"`
	AccountType          string                     `json:"account_type,omitempty"`
	AccountTypeLabel     string                     `json:"account_type_label,omitempty"`
	AccountCount         int                        `json:"account_count"`
	SchedulableCount     int                        `json:"schedulable_count"`
	RateLimitedCount     int                        `json:"rate_limited_count"`
	MissingSnapshotCount int                        `json:"missing_snapshot_count"`
	FiveHour             AccountUsageSummaryWindow  `json:"five_hour"`
	SevenDay             AccountUsageSummaryWindow  `json:"seven_day"`
	LatestUpdatedAt      *time.Time                 `json:"latest_updated_at,omitempty"`
	OldestUpdatedAt      *time.Time                 `json:"oldest_updated_at,omitempty"`
	UpstreamBalance      UpstreamBalanceSummary     `json:"upstream_balance"`
	Types                []AccountUsageSummaryGroup `json:"types,omitempty"`
}

type AccountUsageSummary struct {
	GeneratedAt             time.Time                  `json:"generated_at"`
	TotalAccounts           int                        `json:"total_accounts"`
	SchedulableAccounts     int                        `json:"schedulable_accounts"`
	RateLimitedAccounts     int                        `json:"rate_limited_accounts"`
	MissingSnapshotAccounts int                        `json:"missing_snapshot_accounts"`
	FiveHour                AccountUsageSummaryWindow  `json:"five_hour"`
	SevenDay                AccountUsageSummaryWindow  `json:"seven_day"`
	UpstreamBalance         UpstreamBalanceSummary     `json:"upstream_balance"`
	Plans                   []AccountUsageSummaryGroup `json:"plans"`
}

type accountUsageSummaryAccumulator struct {
	group       AccountUsageSummaryGroup
	accountIDs  []int64
	childByType map[string]*accountUsageSummaryAccumulator
}

func (s *AccountUsageService) GetAccountUsageSummary(ctx context.Context, accounts []Account) (*AccountUsageSummary, error) {
	if s == nil {
		return BuildAccountUsageSummary(ctx, accounts, nil, time.Now())
	}
	var batchReader accountWindowStatsBatchReader
	if s.usageLogRepo != nil {
		batchReader, _ = s.usageLogRepo.(accountWindowStatsBatchReader)
	}
	return BuildAccountUsageSummary(ctx, accounts, batchReader, time.Now())
}

func BuildAccountUsageSummary(ctx context.Context, accounts []Account, statsReader accountWindowStatsBatchReader, now time.Time) (*AccountUsageSummary, error) {
	if now.IsZero() {
		now = time.Now()
	}

	openAIAccounts := make([]Account, 0, len(accounts))
	accountIDs := make([]int64, 0, len(accounts))
	for _, account := range accounts {
		if !account.IsOpenAI() {
			continue
		}
		openAIAccounts = append(openAIAccounts, account)
		if account.ID > 0 {
			accountIDs = append(accountIDs, account.ID)
		}
	}

	stats5h, err := readAccountSummaryWindowStats(ctx, statsReader, accountIDs, now.Add(-5*time.Hour))
	if err != nil {
		return nil, err
	}
	stats7d, err := readAccountSummaryWindowStats(ctx, statsReader, accountIDs, now.Add(-7*24*time.Hour))
	if err != nil {
		return nil, err
	}

	summary := &AccountUsageSummary{
		GeneratedAt: now.UTC(),
		Plans:       make([]AccountUsageSummaryGroup, 0),
	}
	byPlan := make(map[string]*accountUsageSummaryAccumulator)

	for i := range openAIAccounts {
		account := &openAIAccounts[i]
		planType := normalizeAccountSummaryPlanTypeForAccount(account)
		accountType := normalizeAccountSummaryAccountType(account.Type)
		planAcc := getAccountUsageSummaryAccumulator(byPlan, planType, "", true)
		typeAcc := getAccountUsageSummaryAccumulator(planAcc.childByType, planType, accountType, false)

		applyAccountToSummaryAccumulator(planAcc, account, planType, stats5h[account.ID], stats7d[account.ID], now)
		applyAccountToSummaryAccumulator(typeAcc, account, planType, stats5h[account.ID], stats7d[account.ID], now)

		summary.TotalAccounts++
		if account.IsSchedulable() {
			summary.SchedulableAccounts++
		}
		if account.IsRateLimited() {
			summary.RateLimitedAccounts++
		}
		if !accountHasCodexUsageSnapshot(account.Extra, now) {
			summary.MissingSnapshotAccounts++
		}
		if balance := UpstreamBalanceSnapshotFromExtra(account.Extra); balance != nil {
			applySnapshotToSummary(&summary.UpstreamBalance, balance)
		} else if account.Type == AccountTypeAPIKey {
			summary.UpstreamBalance.AccountCount++
			summary.UpstreamBalance.MissingAccounts++
		}
		applyAccountWindowToSummary(&summary.FiveHour, account, planType, "5h", stats5h[account.ID], now)
		applyAccountWindowToSummary(&summary.SevenDay, account, planType, "7d", stats7d[account.ID], now)
	}

	planKeys := make([]string, 0, len(byPlan))
	for key := range byPlan {
		planKeys = append(planKeys, key)
	}
	sort.Slice(planKeys, func(i, j int) bool {
		return comparePlanTypes(planKeys[i], planKeys[j]) < 0
	})

	for _, planKey := range planKeys {
		planGroup := byPlan[planKey].group
		typeKeys := make([]string, 0, len(byPlan[planKey].childByType))
		for key := range byPlan[planKey].childByType {
			typeKeys = append(typeKeys, key)
		}
		sort.Slice(typeKeys, func(i, j int) bool {
			return compareAccountTypes(typeKeys[i], typeKeys[j]) < 0
		})
		for _, typeKey := range typeKeys {
			planGroup.Types = append(planGroup.Types, byPlan[planKey].childByType[typeKey].group)
		}
		summary.Plans = append(summary.Plans, planGroup)
	}

	finalizeAccountUsageSummaryWindow(&summary.FiveHour)
	finalizeAccountUsageSummaryWindow(&summary.SevenDay)
	for i := range summary.Plans {
		finalizeAccountUsageSummaryGroup(&summary.Plans[i])
	}

	return summary, nil
}

func normalizeAccountSummaryPlanTypeForAccount(account *Account) string {
	if account == nil {
		return "unknown"
	}
	if normalizeAccountSummaryAccountType(account.Type) == AccountTypeAPIKey {
		return "api_key"
	}
	return normalizeAccountSummaryPlanType(account.GetCredential("plan_type"))
}

func readAccountSummaryWindowStats(ctx context.Context, reader accountWindowStatsBatchReader, accountIDs []int64, startTime time.Time) (map[int64]*usagestats.AccountStats, error) {
	result := make(map[int64]*usagestats.AccountStats)
	if reader == nil || len(accountIDs) == 0 {
		return result, nil
	}
	return reader.GetAccountWindowStatsBatch(ctx, uniquePositiveInt64s(accountIDs), startTime)
}

func uniquePositiveInt64s(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func getAccountUsageSummaryAccumulator(m map[string]*accountUsageSummaryAccumulator, planType, accountType string, withChildren bool) *accountUsageSummaryAccumulator {
	key := accountType
	if key == "" {
		key = planType
	}
	if acc, ok := m[key]; ok {
		return acc
	}
	acc := &accountUsageSummaryAccumulator{
		group: AccountUsageSummaryGroup{
			PlanType:         planType,
			PlanLabel:        accountSummaryPlanLabel(planType),
			AccountType:      accountType,
			AccountTypeLabel: accountSummaryAccountTypeLabel(accountType),
		},
	}
	if withChildren {
		acc.childByType = make(map[string]*accountUsageSummaryAccumulator)
	}
	m[key] = acc
	return acc
}

func applyAccountToSummaryAccumulator(acc *accountUsageSummaryAccumulator, account *Account, planType string, stats5h, stats7d *usagestats.AccountStats, now time.Time) {
	if acc == nil || account == nil {
		return
	}
	acc.group.AccountCount++
	if account.IsSchedulable() {
		acc.group.SchedulableCount++
	}
	if account.IsRateLimited() {
		acc.group.RateLimitedCount++
	}
	if !accountHasCodexUsageSnapshot(account.Extra, now) {
		acc.group.MissingSnapshotCount++
	}
	if balance := UpstreamBalanceSnapshotFromExtra(account.Extra); balance != nil {
		applySnapshotToSummary(&acc.group.UpstreamBalance, balance)
	} else if account.Type == AccountTypeAPIKey {
		acc.group.UpstreamBalance.AccountCount++
		acc.group.UpstreamBalance.MissingAccounts++
	}
	if updatedAt := accountCodexUsageUpdatedAt(account.Extra); updatedAt != nil {
		setAccountSummaryFreshness(&acc.group, *updatedAt)
	}
	applyAccountWindowToSummary(&acc.group.FiveHour, account, planType, "5h", stats5h, now)
	applyAccountWindowToSummary(&acc.group.SevenDay, account, planType, "7d", stats7d, now)
}

func applyAccountWindowToSummary(target *AccountUsageSummaryWindow, account *Account, planType, window string, stats *usagestats.AccountStats, now time.Time) {
	if !accountSummaryWindowApplies(planType, window) {
		return
	}
	if target != nil {
		target.AccountsInWindow++
	}
	applyWindowsToSummary(target, buildCodexUsageProgressFromExtra(account.Extra, window, now), windowStatsFromAccountStats(stats))
}

func accountSummaryWindowApplies(planType, window string) bool {
	if planType == "free" && window == "5h" {
		return false
	}
	return true
}

func applyWindowsToSummary(target *AccountUsageSummaryWindow, progress *UsageProgress, stats *WindowStats) {
	if target == nil {
		return
	}
	if stats != nil {
		target.UsedCost += stats.Cost
		target.Requests += stats.Requests
	}
	if progress == nil {
		return
	}
	target.AccountsWithSnapshot++
	target.UsedPercentSum += progress.Utilization
	target.RemainingPercentSum += 100 - progress.Utilization
	if progress.ResetsAt != nil {
		if target.EarliestResetAt == nil || progress.ResetsAt.Before(*target.EarliestResetAt) {
			reset := progress.ResetsAt.UTC()
			target.EarliestResetAt = &reset
		}
	}
	usedCost := 0.0
	if stats != nil {
		usedCost = stats.Cost
	}
	if progress.Utilization <= 0 || usedCost <= 0 {
		return
	}
	target.EstimatedLimitCost += usedCost / (progress.Utilization / 100)
	target.AccountsWithLimitEstimate++
}

func finalizeAccountUsageSummaryGroup(group *AccountUsageSummaryGroup) {
	finalizeAccountUsageSummaryWindow(&group.FiveHour)
	finalizeAccountUsageSummaryWindow(&group.SevenDay)
	for i := range group.Types {
		finalizeAccountUsageSummaryGroup(&group.Types[i])
	}
}

func finalizeAccountUsageSummaryWindow(window *AccountUsageSummaryWindow) {
	if window == nil {
		return
	}
	if window.AccountsInWindow > 0 {
		window.RemainingPercentSum = float64(window.AccountsInWindow)*100 - window.UsedPercentSum
	}
	if window.EstimatedLimitCost <= 0 {
		return
	}
	window.Utilization = window.UsedCost / window.EstimatedLimitCost * 100
}

func accountHasCodexUsageSnapshot(extra map[string]any, now time.Time) bool {
	return buildCodexUsageProgressFromExtra(extra, "5h", now) != nil || buildCodexUsageProgressFromExtra(extra, "7d", now) != nil
}

func accountCodexUsageUpdatedAt(extra map[string]any) *time.Time {
	if len(extra) == 0 {
		return nil
	}
	raw, ok := extra["codex_usage_updated_at"]
	if !ok || raw == nil {
		return nil
	}
	ts, err := parseTime(strings.TrimSpace(fmt.Sprint(raw)))
	if err != nil {
		return nil
	}
	utc := ts.UTC()
	return &utc
}

func setAccountSummaryFreshness(group *AccountUsageSummaryGroup, updatedAt time.Time) {
	if group == nil || updatedAt.IsZero() {
		return
	}
	updatedAt = updatedAt.UTC()
	if group.LatestUpdatedAt == nil || updatedAt.After(*group.LatestUpdatedAt) {
		group.LatestUpdatedAt = &updatedAt
	}
	if group.OldestUpdatedAt == nil || updatedAt.Before(*group.OldestUpdatedAt) {
		group.OldestUpdatedAt = &updatedAt
	}
}

func normalizeAccountSummaryPlanType(planType string) string {
	switch strings.ToLower(strings.TrimSpace(planType)) {
	case "":
		return "unknown"
	case "chatgptpro":
		return "pro"
	default:
		return strings.ToLower(strings.TrimSpace(planType))
	}
}

func accountSummaryPlanLabel(planType string) string {
	switch planType {
	case "free":
		return "Free"
	case "plus":
		return "Plus"
	case "pro":
		return "Pro"
	case "team":
		return "Team"
	case "api_key":
		return "API Key"
	case "abnormal":
		return "Abnormal"
	case "unknown":
		return "Unknown"
	default:
		if planType == "" {
			return "Unknown"
		}
		return planType
	}
}

func normalizeAccountSummaryAccountType(accountType string) string {
	accountType = strings.TrimSpace(accountType)
	if accountType == "" {
		return "unknown"
	}
	return accountType
}

func accountSummaryAccountTypeLabel(accountType string) string {
	switch accountType {
	case AccountTypeOAuth:
		return "OAuth"
	case AccountTypeSetupToken:
		return "Setup Token"
	case AccountTypeAPIKey:
		return "API Key"
	case AccountTypeUpstream:
		return "Upstream"
	case AccountTypeBedrock:
		return "Bedrock"
	case AccountTypeServiceAccount:
		return "Service Account"
	case "":
		return ""
	case "unknown":
		return "Unknown"
	default:
		return accountType
	}
}

func comparePlanTypes(a, b string) int {
	ar := accountSummaryPlanRank(a)
	br := accountSummaryPlanRank(b)
	if ar != br {
		if ar < br {
			return -1
		}
		return 1
	}
	return strings.Compare(a, b)
}

func accountSummaryPlanRank(planType string) int {
	switch planType {
	case "free":
		return 0
	case "plus":
		return 1
	case "pro":
		return 2
	case "team":
		return 3
	case "api_key":
		return 4
	case "abnormal":
		return 98
	case "unknown":
		return 99
	default:
		return 50
	}
}

func compareAccountTypes(a, b string) int {
	ar := accountSummaryAccountTypeRank(a)
	br := accountSummaryAccountTypeRank(b)
	if ar != br {
		if ar < br {
			return -1
		}
		return 1
	}
	return strings.Compare(a, b)
}

func accountSummaryAccountTypeRank(accountType string) int {
	switch accountType {
	case AccountTypeOAuth:
		return 0
	case AccountTypeSetupToken:
		return 1
	case AccountTypeAPIKey:
		return 2
	case AccountTypeUpstream:
		return 3
	case AccountTypeBedrock:
		return 4
	case AccountTypeServiceAccount:
		return 5
	case "unknown":
		return 99
	default:
		return 50
	}
}
