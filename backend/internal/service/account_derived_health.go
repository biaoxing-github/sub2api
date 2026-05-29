package service

import (
	"strings"
	"time"
)

const (
	AccountDerivedHealthNormal                 = "normal"
	AccountDerivedHealthRateLimitedCooldown    = "rate_limited_cooldown"
	AccountDerivedHealthUnauthorizedInvalid    = "unauthorized_invalid"
	AccountDerivedHealthLineDegraded           = "line_degraded"
	AccountDerivedHealthUpstreamAbnormal       = "upstream_abnormal"
	AccountDerivedHealthPendingRetest          = "pending_retest"
)

type AccountDerivedHealthState struct {
	State             string     `json:"state"`
	Label             string     `json:"label"`
	Reason            string     `json:"reason,omitempty"`
	Until             *time.Time `json:"until,omitempty"`
	PathHealthState   string     `json:"path_health_state,omitempty"`
	LastFailureReason string     `json:"last_failure_reason,omitempty"`
}

func DeriveAccountHealthState(account *Account, health OpenAIPathHealthRecord, now time.Time) AccountDerivedHealthState {
	if now.IsZero() {
		now = time.Now()
	}
	if account == nil {
		return AccountDerivedHealthState{State: AccountDerivedHealthPendingRetest, Label: "待复测", Reason: "account_missing"}
	}
	if resetAt := account.effectiveRateLimitResetAt(now); resetAt != nil {
		return AccountDerivedHealthState{State: AccountDerivedHealthRateLimitedCooldown, Label: "429冷却", Reason: "rate_limit_reset_at", Until: resetAt}
	}
	classification := ClassifyUpstreamError(UpstreamErrorInput{StatusCode: 0, Message: account.ErrorMessage})
	if account.Status == StatusError && classification.Category == UpstreamErrorCategoryUnauthorized {
		return AccountDerivedHealthState{State: AccountDerivedHealthUnauthorizedInvalid, Label: "401失效", Reason: classification.Category}
	}
	if health.State == OpenAIPathHealthStateOpenCircuit || health.State == OpenAIPathHealthStateDegraded || health.State == OpenAIPathHealthStateHalfOpen {
		return AccountDerivedHealthState{
			State:             AccountDerivedHealthLineDegraded,
			Label:             "线路降级",
			Reason:            firstNonEmptyString(health.LastFailureReason, health.State),
			Until:             health.CooldownUntil,
			PathHealthState:   strings.TrimSpace(health.State),
			LastFailureReason: strings.TrimSpace(health.LastFailureReason),
		}
	}
	if account.TempUnschedulableUntil != nil && now.Before(*account.TempUnschedulableUntil) {
		return AccountDerivedHealthState{State: AccountDerivedHealthUpstreamAbnormal, Label: "上游异常", Reason: account.TempUnschedulableReason, Until: account.TempUnschedulableUntil}
	}
	if account.Status == StatusError {
		if strings.TrimSpace(account.ErrorMessage) == "" {
			return AccountDerivedHealthState{State: AccountDerivedHealthPendingRetest, Label: "待复测", Reason: "manual_or_unknown_error"}
		}
		return AccountDerivedHealthState{State: AccountDerivedHealthUpstreamAbnormal, Label: "上游异常", Reason: classification.Category}
	}
	if !account.Schedulable {
		return AccountDerivedHealthState{State: AccountDerivedHealthPendingRetest, Label: "待复测", Reason: "schedulable_disabled"}
	}
	return AccountDerivedHealthState{State: AccountDerivedHealthNormal, Label: "正常"}
}
