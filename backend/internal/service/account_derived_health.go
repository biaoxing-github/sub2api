package service

import (
	"strings"
	"time"
)

const (
	AccountDerivedHealthNormal              = "normal"
	AccountDerivedHealthRateLimitedCooldown = "rate_limited_cooldown"
	AccountDerivedHealthUnauthorizedInvalid = "unauthorized_invalid"
	AccountDerivedHealthLineDegraded        = "line_degraded"
	AccountDerivedHealthUpstreamAbnormal    = "upstream_abnormal"
	AccountDerivedHealthPendingRetest       = "pending_retest"
	AccountDerivedHealthLightAbnormal       = "light_abnormal"
	AccountDerivedHealthModerateAbnormal    = "moderate_abnormal"
	AccountDerivedHealthTempUnschedulable   = "temp_unschedulable"
	AccountDerivedHealthQuotaLow            = "quota_low"
	AccountDerivedHealthQuotaExhausted      = "quota_exhausted"
	AccountDerivedHealthDisabled            = "disabled"
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
	if state, ok := deriveAccountProbeExtraHealth(account, now); ok {
		return state
	}
	return AccountDerivedHealthState{State: AccountDerivedHealthNormal, Label: "正常"}
}

func deriveAccountProbeExtraHealth(account *Account, now time.Time) (AccountDerivedHealthState, bool) {
	payload := accountProbeHealthPayload(account)
	level := accountProbeHealthLevel(payload)
	if level == "" || level == AccountProbeHealthNormal {
		return AccountDerivedHealthState{}, false
	}
	state := AccountDerivedHealthState{
		State:  accountProbeHealthDerivedState(level),
		Label:  AccountProbeHealthLabel(level),
		Reason: accountProbeHealthReason(payload),
		Until:  accountProbeHealthNextProbeAt(payload, now),
	}
	return state, true
}

func accountProbeHealthDerivedState(level string) string {
	switch strings.TrimSpace(level) {
	case AccountProbeHealthLightAbnormal:
		return AccountDerivedHealthLightAbnormal
	case AccountProbeHealthModerateAbnormal:
		return AccountDerivedHealthModerateAbnormal
	case AccountProbeHealthLineDegraded:
		return AccountDerivedHealthLineDegraded
	case AccountProbeHealthTempUnsched:
		return AccountDerivedHealthTempUnschedulable
	case AccountProbeHealthRateLimited:
		return AccountDerivedHealthRateLimitedCooldown
	case AccountProbeHealthQuotaLow:
		return AccountDerivedHealthQuotaLow
	case AccountProbeHealthQuotaExhausted:
		return AccountDerivedHealthQuotaExhausted
	case AccountProbeHealthDisabled:
		return AccountDerivedHealthDisabled
	case AccountProbeHealthPendingRetest:
		return AccountDerivedHealthPendingRetest
	default:
		return AccountDerivedHealthUpstreamAbnormal
	}
}

func accountProbeHealthReason(payload map[string]any) string {
	for _, key := range []string{"reason", "last_error"} {
		value := strings.TrimSpace(stringFromAny(payload[key]))
		if value != "" {
			return value
		}
	}
	return ""
}

func accountProbeHealthNextProbeAt(payload map[string]any, now time.Time) *time.Time {
	raw := strings.TrimSpace(stringFromAny(payload["next_probe_at"]))
	if raw == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil
	}
	if !now.IsZero() && parsed.Before(now) {
		return nil
	}
	return &parsed
}
