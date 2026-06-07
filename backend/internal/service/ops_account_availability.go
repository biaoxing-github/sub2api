package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const (
	AccountEffectiveAvailabilityHealthy           = "healthy"
	AccountEffectiveAvailabilityPathOpenCircuit   = "path_open_circuit"
	AccountEffectiveAvailabilityPathHalfOpen      = "path_half_open"
	AccountEffectiveAvailabilityPathDegraded      = "path_degraded"
	AccountEffectiveAvailabilityPrecheckPending   = "precheck_pending"
	AccountEffectiveAvailabilityLocalSuppressed   = "local_suppressed"
	AccountEffectiveAvailabilityPrecheckFailed    = "precheck_failed"
	AccountEffectiveAvailabilityTempUnschedulable = "temp_unschedulable"
	AccountEffectiveAvailabilityRateLimited       = "rate_limited"
	AccountEffectiveAvailabilityOverloaded        = "overloaded"
	AccountEffectiveAvailabilityError             = "error"
	AccountEffectiveAvailabilityDisabled          = "disabled"
	AccountEffectiveAvailabilityUnschedulable     = "unschedulable"
)

// GetAccountAvailabilityStats returns current account availability stats.
//
// Query-level filtering is intentionally limited to platform/group to match the dashboard scope.
func (s *OpsService) GetAccountAvailabilityStats(ctx context.Context, platformFilter string, groupIDFilter *int64) (
	map[string]*PlatformAvailability,
	map[int64]*GroupAvailability,
	map[int64]*AccountAvailability,
	*time.Time,
	error,
) {
	if err := s.RequireMonitoringEnabled(ctx); err != nil {
		return nil, nil, nil, nil, err
	}

	accounts, err := s.listAllAccountsForOps(ctx, platformFilter)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	if groupIDFilter != nil && *groupIDFilter > 0 {
		filtered := make([]Account, 0, len(accounts))
		for _, acc := range accounts {
			for _, grp := range acc.Groups {
				if grp != nil && grp.ID == *groupIDFilter {
					filtered = append(filtered, acc)
					break
				}
			}
		}
		accounts = filtered
	}

	now := time.Now()
	collectedAt := now

	platform := make(map[string]*PlatformAvailability)
	group := make(map[int64]*GroupAvailability)
	account := make(map[int64]*AccountAvailability)

	for _, acc := range accounts {
		if acc.ID <= 0 {
			continue
		}

		isTempUnsched := false
		if acc.TempUnschedulableUntil != nil && now.Before(*acc.TempUnschedulableUntil) {
			isTempUnsched = true
		}

		isRateLimited := acc.RateLimitResetAt != nil && now.Before(*acc.RateLimitResetAt)
		isOverloaded := acc.OverloadUntil != nil && now.Before(*acc.OverloadUntil)
		hasError := acc.Status == StatusError

		pathHealth, hasPathHealth := s.openAIPathHealthForAvailability(&acc)
		runtimeBlock, hasRuntimeBlock := s.openAIRuntimeBlockForAvailability(&acc, now)
		effectiveAvailability := deriveAccountEffectiveAvailability(&acc, now, pathHealth, hasPathHealth, runtimeBlock, hasRuntimeBlock)
		isAvailable := effectiveAvailability.State == AccountEffectiveAvailabilityHealthy

		if acc.Platform != "" {
			if _, ok := platform[acc.Platform]; !ok {
				platform[acc.Platform] = &PlatformAvailability{
					Platform: acc.Platform,
				}
			}
			p := platform[acc.Platform]
			p.TotalAccounts++
			if isAvailable {
				p.AvailableCount++
			}
			if effectiveAvailability.State == AccountEffectiveAvailabilityRateLimited {
				p.RateLimitCount++
			}
			if hasError {
				p.ErrorCount++
			}
		}

		for _, grp := range acc.Groups {
			if grp == nil || grp.ID <= 0 {
				continue
			}
			if _, ok := group[grp.ID]; !ok {
				group[grp.ID] = &GroupAvailability{
					GroupID:   grp.ID,
					GroupName: grp.Name,
					Platform:  grp.Platform,
				}
			}
			g := group[grp.ID]
			g.TotalAccounts++
			if isAvailable {
				g.AvailableCount++
			}
			if effectiveAvailability.State == AccountEffectiveAvailabilityRateLimited {
				g.RateLimitCount++
			}
			if hasError {
				g.ErrorCount++
			}
		}

		displayGroupID := int64(0)
		displayGroupName := ""
		if len(acc.Groups) > 0 && acc.Groups[0] != nil {
			displayGroupID = acc.Groups[0].ID
			displayGroupName = acc.Groups[0].Name
		}

		item := &AccountAvailability{
			AccountID:   acc.ID,
			AccountName: acc.Name,
			Platform:    acc.Platform,
			GroupID:     displayGroupID,
			GroupName:   displayGroupName,
			Status:      acc.Status,

			IsAvailable:   isAvailable,
			IsRateLimited: isRateLimited,
			IsOverloaded:  isOverloaded,
			HasError:      hasError,

			ErrorMessage:          acc.ErrorMessage,
			EffectiveAvailability: &effectiveAvailability,
		}

		if isRateLimited && acc.RateLimitResetAt != nil {
			item.RateLimitResetAt = acc.RateLimitResetAt
			remainingSec := int64(time.Until(*acc.RateLimitResetAt).Seconds())
			if remainingSec > 0 {
				item.RateLimitRemainingSec = &remainingSec
			}
		}
		if isOverloaded && acc.OverloadUntil != nil {
			item.OverloadUntil = acc.OverloadUntil
			remainingSec := int64(time.Until(*acc.OverloadUntil).Seconds())
			if remainingSec > 0 {
				item.OverloadRemainingSec = &remainingSec
			}
		}
		if isTempUnsched && acc.TempUnschedulableUntil != nil {
			item.TempUnschedulableUntil = acc.TempUnschedulableUntil
		}
		if hasPathHealth {
			item.PathHealthState = pathHealth.State
			item.PathHealthCooldownUntil = pathHealth.CooldownUntil
			item.PathHealthLastFailureReason = pathHealth.LastFailureReason
			item.PathHealthConsecutiveFailures = pathHealth.ConsecutiveFailures
			item.PathHealthWindowFailures = pathHealth.WindowFailures
			item.PathHealthEOFCount = pathHealth.EOFCount
			item.PathHealthHeaderTimeoutCount = pathHealth.HeaderTimeoutCount
			item.PathHealthTTFTEWMAMs = pathHealth.TTFTEWMAMs
			item.PathHealthHeaderWaitEWMAMs = pathHealth.HeaderWaitEWMAMs
		}

		account[acc.ID] = item
	}

	return platform, group, account, &collectedAt, nil
}

type OpsAccountAvailability struct {
	Group       *GroupAvailability
	Accounts    map[int64]*AccountAvailability
	CollectedAt *time.Time
}

func (s *OpsService) GetAccountAvailability(ctx context.Context, platformFilter string, groupIDFilter *int64) (*OpsAccountAvailability, error) {
	if s == nil {
		return nil, errors.New("ops service is nil")
	}

	if s.getAccountAvailability != nil {
		return s.getAccountAvailability(ctx, platformFilter, groupIDFilter)
	}

	_, groupStats, accountStats, collectedAt, err := s.GetAccountAvailabilityStats(ctx, platformFilter, groupIDFilter)
	if err != nil {
		return nil, err
	}

	var group *GroupAvailability
	if groupIDFilter != nil && *groupIDFilter > 0 {
		group = groupStats[*groupIDFilter]
	}

	if accountStats == nil {
		accountStats = map[int64]*AccountAvailability{}
	}

	return &OpsAccountAvailability{
		Group:       group,
		Accounts:    accountStats,
		CollectedAt: collectedAt,
	}, nil
}

func (s *OpsService) openAIPathHealthForAvailability(account *Account) (OpenAIPathHealthRecord, bool) {
	if s == nil || s.openAIGatewayService == nil || account == nil || !account.IsOpenAI() {
		return OpenAIPathHealthRecord{}, false
	}
	return s.openAIGatewayService.SnapshotOpenAIPathHealthForAccount(account, OpenAIUpstreamTransportHTTPSSE)
}

func (s *OpsService) openAIRuntimeBlockForAvailability(account *Account, now time.Time) (OpenAIAccountRuntimeBlockSnapshot, bool) {
	if s == nil || s.openAIGatewayService == nil || account == nil || !account.IsOpenAI() {
		return OpenAIAccountRuntimeBlockSnapshot{}, false
	}
	return s.openAIGatewayService.SnapshotOpenAIAccountRuntimeBlock(account, now)
}

func deriveAccountEffectiveAvailability(
	account *Account,
	now time.Time,
	pathHealth OpenAIPathHealthRecord,
	hasPathHealth bool,
	runtimeBlock OpenAIAccountRuntimeBlockSnapshot,
	hasRuntimeBlock bool,
) AccountEffectiveAvailability {
	if account == nil {
		return AccountEffectiveAvailability{State: AccountEffectiveAvailabilityDisabled, Reason: "account_missing"}
	}

	if hasPathHealth {
		switch pathHealth.State {
		case OpenAIPathHealthStateOpenCircuit:
			return AccountEffectiveAvailability{
				State:  AccountEffectiveAvailabilityPathOpenCircuit,
				Reason: pathHealth.LastFailureReason,
				Until:  pathHealth.CooldownUntil,
			}
		case OpenAIPathHealthStateHalfOpen:
			return AccountEffectiveAvailability{
				State:  AccountEffectiveAvailabilityPathHalfOpen,
				Reason: pathHealth.LastFailureReason,
				Until:  pathHealth.CooldownUntil,
			}
		case OpenAIPathHealthStateDegraded:
			return AccountEffectiveAvailability{
				State:  AccountEffectiveAvailabilityPathDegraded,
				Reason: pathHealth.LastFailureReason,
			}
		}
	}

	if hasRuntimeBlock {
		return AccountEffectiveAvailability{
			State:  accountRuntimeBlockAvailabilityState(runtimeBlock.Reason),
			Reason: accountRuntimeBlockReason(runtimeBlock.Reason),
			Until:  runtimeBlock.Until,
		}
	}

	if account.TempUnschedulableUntil != nil && now.Before(*account.TempUnschedulableUntil) {
		state, reason := accountTempUnschedulableAvailability(account.TempUnschedulableReason)
		return AccountEffectiveAvailability{
			State:  state,
			Reason: reason,
			Until:  account.TempUnschedulableUntil,
		}
	}

	if account.RateLimitResetAt != nil && now.Before(*account.RateLimitResetAt) {
		return AccountEffectiveAvailability{
			State:  AccountEffectiveAvailabilityRateLimited,
			Reason: "rate_limit_reset_at",
			Until:  account.RateLimitResetAt,
		}
	}

	if account.OverloadUntil != nil && now.Before(*account.OverloadUntil) {
		return AccountEffectiveAvailability{
			State:  AccountEffectiveAvailabilityOverloaded,
			Reason: "overload_until",
			Until:  account.OverloadUntil,
		}
	}

	if account.Status == StatusError {
		reason := account.ErrorMessage
		if reason == "" {
			reason = "status_error"
		}
		return AccountEffectiveAvailability{
			State:  AccountEffectiveAvailabilityError,
			Reason: reason,
		}
	}

	if account.Status != StatusActive {
		return AccountEffectiveAvailability{
			State:  AccountEffectiveAvailabilityDisabled,
			Reason: "status_not_active",
		}
	}

	if !account.Schedulable {
		return AccountEffectiveAvailability{
			State:  AccountEffectiveAvailabilityUnschedulable,
			Reason: "schedulable_disabled",
		}
	}

	return AccountEffectiveAvailability{State: AccountEffectiveAvailabilityHealthy}
}

func accountTempUnschedulableAvailability(rawReason string) (string, string) {
	reason := strings.TrimSpace(rawReason)
	if reason == "" {
		return AccountEffectiveAvailabilityTempUnschedulable, "temp_unschedulable_until"
	}
	if strings.HasPrefix(reason, "{") {
		var state TempUnschedState
		if err := json.Unmarshal([]byte(reason), &state); err == nil {
			reason = firstNonEmptyString(state.MatchedKeyword, state.ErrorMessage, reason)
		}
	}
	return accountRuntimeBlockAvailabilityState(reason), accountRuntimeBlockReason(reason)
}

func accountRuntimeBlockAvailabilityState(reason string) string {
	lower := strings.ToLower(strings.TrimSpace(reason))
	switch {
	case strings.Contains(lower, "precheck_pending"):
		return AccountEffectiveAvailabilityPrecheckPending
	case strings.Contains(lower, "precheck_failed"),
		strings.Contains(lower, "token_refresh"),
		strings.Contains(lower, "oauth_401"),
		strings.Contains(lower, "upstream_disable"):
		return AccountEffectiveAvailabilityPrecheckFailed
	case strings.Contains(lower, "local_suppressed"),
		strings.Contains(lower, "stream_timeout"),
		strings.Contains(lower, "rate_limit"),
		strings.Contains(lower, "quota"),
		strings.Contains(lower, "429"):
		return AccountEffectiveAvailabilityLocalSuppressed
	default:
		return AccountEffectiveAvailabilityTempUnschedulable
	}
}

func accountRuntimeBlockReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "runtime_blocked"
	}
	return reason
}
