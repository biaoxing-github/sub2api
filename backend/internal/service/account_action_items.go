package service

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	AccountActionSeverityCritical = "critical"
	AccountActionSeverityWarning  = "warning"
	AccountActionSeverityInfo     = "info"

	accountDashboardDrainThresholdUSD = 2.0
	accountDashboardStickyReserveUSD  = 0.5
)

type AccountActionItemCounts struct {
	Critical int `json:"critical"`
	Warning  int `json:"warning"`
	Info     int `json:"info"`
}

type AccountActionItem struct {
	AccountID        int64      `json:"account_id"`
	AccountName      string     `json:"account_name"`
	Severity         string     `json:"severity"`
	Reason           string     `json:"reason"`
	Summary          string     `json:"summary"`
	SuggestedAction  string     `json:"suggested_action"`
	Status           string     `json:"status"`
	Schedulable      bool       `json:"schedulable"`
	RateLimitResetAt *time.Time `json:"rate_limit_reset_at,omitempty"`
	Until            *time.Time `json:"until,omitempty"`
}

type AccountActionItemsResult struct {
	GeneratedAt time.Time               `json:"generated_at"`
	Items       []AccountActionItem     `json:"items"`
	Counts      AccountActionItemCounts `json:"counts"`
}

type AccountStatusSummary struct {
	Total             int `json:"total"`
	Active            int `json:"active"`
	RateLimited       int `json:"rate_limited"`
	Error             int `json:"error"`
	Inactive          int `json:"inactive"`
	TempUnschedulable int `json:"temp_unschedulable"`
	Unschedulable     int `json:"unschedulable"`
}

type AccountBalanceSummary struct {
	Healthy         int `json:"healthy"`
	Draining        int `json:"draining"`
	Exhausted       int `json:"exhausted"`
	BalanceUnknown  int `json:"balance_unknown"`
	MissingSnapshot int `json:"missing_snapshot"`
}

type AccountDashboardSummary struct {
	GeneratedAt       time.Time               `json:"generated_at"`
	StatusSummary     AccountStatusSummary    `json:"status_summary"`
	UsageSummary      *AccountUsageSummary    `json:"usage_summary,omitempty"`
	UsageSummaryError string                  `json:"usage_summary_error,omitempty"`
	BalanceSummary    AccountBalanceSummary   `json:"balance_summary"`
	ActionItemCounts  AccountActionItemCounts `json:"action_item_counts"`
	ActionItemsError  string                  `json:"action_items_error,omitempty"`
}

func BuildAccountActionItems(accounts []Account, now time.Time) AccountActionItemsResult {
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC()
	result := AccountActionItemsResult{
		GeneratedAt: now,
		Items:       make([]AccountActionItem, 0),
	}

	for i := range accounts {
		account := &accounts[i]
		if !account.Schedulable {
			result.add(accountActionItem(account, AccountActionSeverityCritical, "schedulable_disabled", "账号已被手动设为不可调度", "检查账号后恢复调度或移出生产分组", nil, nil))
		}
		if account.Status == StatusError {
			summary := "账号处于错误状态"
			if strings.TrimSpace(account.ErrorMessage) != "" {
				summary = "账号错误: " + strings.TrimSpace(account.ErrorMessage)
			}
			result.add(accountActionItem(account, AccountActionSeverityCritical, "account_error", summary, "查看错误并刷新凭证或清除错误", nil, nil))
		}
		if account.RateLimitResetAt != nil && now.Before(*account.RateLimitResetAt) {
			summary := fmt.Sprintf("账号限流至 %s", account.RateLimitResetAt.UTC().Format(time.RFC3339))
			result.add(accountActionItem(account, AccountActionSeverityWarning, "rate_limited", summary, "等待重置或切换到其他可用账号", account.RateLimitResetAt, nil))
		}
		if account.TempUnschedulableUntil != nil && now.Before(*account.TempUnschedulableUntil) {
			summary := fmt.Sprintf("账号临时不可调度至 %s", account.TempUnschedulableUntil.UTC().Format(time.RFC3339))
			if strings.TrimSpace(account.TempUnschedulableReason) != "" {
				summary += ": " + strings.TrimSpace(account.TempUnschedulableReason)
			}
			result.add(accountActionItem(account, AccountActionSeverityWarning, "temp_unschedulable", summary, "等待冷却结束或手动清除临时不可调度", nil, account.TempUnschedulableUntil))
		}

		state := AccountUpstreamBalanceState(account)
		switch state {
		case "missing_snapshot":
			result.add(accountActionItem(account, AccountActionSeverityWarning, "missing_balance_snapshot", "缺少上游余额快照", "刷新上游余额", nil, nil))
		case RealtimeBalanceStateDraining:
			result.add(accountActionItem(account, AccountActionSeverityWarning, "upstream_balance_draining", "上游余额较低，账号应进入 draining", "充值或减少新会话调度", nil, nil))
		case RealtimeBalanceStateExhausted:
			result.add(accountActionItem(account, AccountActionSeverityCritical, "upstream_balance_exhausted", "上游余额已耗尽或低于粘性预留", "充值或暂停账号", nil, nil))
		case RealtimeBalanceStateUnknown:
			result.add(accountActionItem(account, AccountActionSeverityWarning, "upstream_balance_unknown", "上游余额快照不可用", "刷新上游余额并检查上游响应", nil, nil))
		}
	}

	sort.SliceStable(result.Items, func(i, j int) bool {
		left := result.Items[i]
		right := result.Items[j]
		if rank := severityRank(left.Severity) - severityRank(right.Severity); rank != 0 {
			return rank < 0
		}
		if left.Reason != right.Reason {
			return left.Reason < right.Reason
		}
		return left.AccountID < right.AccountID
	})

	return result
}

func BuildAccountDashboardSummary(accounts []Account, usageSummary *AccountUsageSummary, now time.Time) AccountDashboardSummary {
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC()
	actionItems := BuildAccountActionItems(accounts, now)
	summary := AccountDashboardSummary{
		GeneratedAt:      now,
		UsageSummary:     usageSummary,
		ActionItemCounts: actionItems.Counts,
	}
	summary.StatusSummary.Total = len(accounts)

	for i := range accounts {
		account := &accounts[i]
		switch account.Status {
		case StatusActive:
			summary.StatusSummary.Active++
		case StatusError:
			summary.StatusSummary.Error++
		case StatusDisabled, "inactive":
			summary.StatusSummary.Inactive++
		}
		if !account.Schedulable {
			summary.StatusSummary.Unschedulable++
		}
		if account.RateLimitResetAt != nil && now.Before(*account.RateLimitResetAt) {
			summary.StatusSummary.RateLimited++
		}
		if account.TempUnschedulableUntil != nil && now.Before(*account.TempUnschedulableUntil) {
			summary.StatusSummary.TempUnschedulable++
		}

		switch AccountUpstreamBalanceState(account) {
		case RealtimeBalanceStateHealthy:
			summary.BalanceSummary.Healthy++
		case RealtimeBalanceStateDraining:
			summary.BalanceSummary.Draining++
		case RealtimeBalanceStateExhausted:
			summary.BalanceSummary.Exhausted++
		case RealtimeBalanceStateUnknown:
			summary.BalanceSummary.BalanceUnknown++
		case "missing_snapshot":
			summary.BalanceSummary.MissingSnapshot++
		}
	}

	return summary
}

func AccountUpstreamBalanceState(account *Account) string {
	if account == nil || !account.IsOpenAI() || account.Type != AccountTypeAPIKey {
		return ""
	}
	snapshot := UpstreamBalanceSnapshotFromExtra(account.Extra)
	if snapshot == nil {
		return "missing_snapshot"
	}
	if !IsVerifiedRealtimeBalanceSnapshot(snapshot) {
		return RealtimeBalanceStateUnknown
	}
	switch {
	case snapshot.Available >= accountDashboardDrainThresholdUSD:
		return RealtimeBalanceStateHealthy
	case snapshot.Available >= accountDashboardStickyReserveUSD:
		return RealtimeBalanceStateDraining
	default:
		return RealtimeBalanceStateExhausted
	}
}

func (r *AccountActionItemsResult) add(item AccountActionItem) {
	r.Items = append(r.Items, item)
	switch item.Severity {
	case AccountActionSeverityCritical:
		r.Counts.Critical++
	case AccountActionSeverityWarning:
		r.Counts.Warning++
	case AccountActionSeverityInfo:
		r.Counts.Info++
	}
}

func accountActionItem(account *Account, severity, reason, summary, action string, resetAt, until *time.Time) AccountActionItem {
	return AccountActionItem{
		AccountID:        account.ID,
		AccountName:      account.Name,
		Severity:         severity,
		Reason:           reason,
		Summary:          summary,
		SuggestedAction:  action,
		Status:           account.Status,
		Schedulable:      account.Schedulable,
		RateLimitResetAt: resetAt,
		Until:            until,
	}
}

func severityRank(severity string) int {
	switch severity {
	case AccountActionSeverityCritical:
		return 0
	case AccountActionSeverityWarning:
		return 1
	case AccountActionSeverityInfo:
		return 2
	default:
		return 3
	}
}
