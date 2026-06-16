package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const (
	AccountProbeHealthExtraKey = "account_probe_health"

	AccountProbeHealthNormal           = "normal"
	AccountProbeHealthLightAbnormal    = "light_abnormal"
	AccountProbeHealthModerateAbnormal = "moderate_abnormal"
	AccountProbeHealthLineDegraded     = "line_degraded"
	AccountProbeHealthTempUnsched      = "temp_unschedulable"
	AccountProbeHealthRateLimited      = "rate_limited"
	AccountProbeHealthQuotaLow         = "quota_low"
	AccountProbeHealthQuotaExhausted   = "quota_exhausted"
	AccountProbeHealthDisabled         = "disabled"
	AccountProbeHealthPendingRetest    = "pending_retest"

	AccountProbeOutcomeSourceManualTest   = "manual_test"
	AccountProbeOutcomeSourceAccountProbe = "account_probe"
	AccountProbeOutcomeSourceBatchTest    = "batch_test"
	AccountProbeOutcomeSourceGateway      = "gateway"
)

// AccountProbeOutcome 是人工测试、后台探测和网关错误统一写账号健康状态的输入。
type AccountProbeOutcome struct {
	AccountID      int64
	Account        *Account
	Source         string
	Success        bool
	ErrorMessage   string
	HTTPStatus     int
	Reason         string
	LatencyMs      *int
	FirstTokenMs   *int
	KeyFingerprint string
	ObservedAt     time.Time
	BlockedUntil   *time.Time
}

// AccountProbeTransitionResult 描述一次探测结果对账号层状态造成的变化。
type AccountProbeTransitionResult struct {
	PreviousLevel string     `json:"state_before"`
	NextLevel     string     `json:"state_after"`
	StateChanged  bool       `json:"state_changed"`
	BlockedUntil  *time.Time `json:"blocked_until,omitempty"`
	Restored      bool       `json:"restored"`
	Reason        string     `json:"state_reason,omitempty"`
}

// RecordAccountProbeOutcome 将账号探测结果写入 extra.account_probe_health，并按需推进硬阻塞字段。
func (s *RateLimitService) RecordAccountProbeOutcome(ctx context.Context, outcome AccountProbeOutcome) (*AccountProbeTransitionResult, error) {
	if s == nil || s.accountRepo == nil {
		return nil, nil
	}
	account := outcome.Account
	if account == nil && outcome.AccountID > 0 {
		loaded, err := s.accountRepo.GetByID(ctx, outcome.AccountID)
		if err != nil {
			return nil, err
		}
		account = loaded
	}
	if account == nil {
		return nil, ErrAccountNotFound
	}
	if outcome.AccountID <= 0 {
		outcome.AccountID = account.ID
	}
	if outcome.ObservedAt.IsZero() {
		outcome.ObservedAt = time.Now()
	}
	outcome.Source = normalizeAccountProbeOutcomeSource(outcome.Source)
	outcome.Reason = normalizeAccountProbeOutcomeReason(outcome)

	previous := accountProbeHealthPayload(account)
	previousLevel := accountProbeHealthLevel(previous)
	if previousLevel == "" {
		previousLevel = AccountProbeHealthNormal
	}
	nextLevel := accountProbeNextHealthLevel(account, previous, outcome)
	successCount := accountProbePayloadInt(previous["success_count"])
	failureCount := accountProbePayloadInt(previous["failure_count"])
	if outcome.Success {
		successCount++
		failureCount = 0
	} else {
		failureCount++
		successCount = 0
	}

	blockedUntil := accountProbeOutcomeBlockedUntil(outcome, nextLevel)
	transition := &AccountProbeTransitionResult{
		PreviousLevel: previousLevel,
		NextLevel:     nextLevel,
		StateChanged:  previousLevel != nextLevel,
		BlockedUntil:  blockedUntil,
		Restored:      outcome.Success && previousLevel != AccountProbeHealthNormal && nextLevel == AccountProbeHealthNormal,
		Reason:        outcome.Reason,
	}

	if outcome.Success {
		if canRecoverAccountAfterProbeSuccess(account) {
			if _, err := s.RecoverAccountState(ctx, account.ID, AccountRecoveryOptions{
				RestoreSchedulable: outcome.Source == AccountProbeOutcomeSourceManualTest,
			}); err != nil {
				return nil, err
			}
		}
		s.notifyAccountSchedulingBlockCleared(account.ID)
	} else if blockedUntil != nil {
		if err := s.applyAccountProbeBlockedState(ctx, account, nextLevel, *blockedUntil, outcome.Reason); err != nil {
			return nil, err
		}
	}

	payload := map[string]any{
		"level":             nextLevel,
		"failure_count":     failureCount,
		"success_count":     successCount,
		"last_probe_source": outcome.Source,
		"last_probe_at":     outcome.ObservedAt.UTC().Format(time.RFC3339),
		"reason":            outcome.Reason,
	}
	if blockedUntil != nil {
		payload["next_probe_at"] = blockedUntil.UTC().Format(time.RFC3339)
	}
	if msg := strings.TrimSpace(outcome.ErrorMessage); msg != "" {
		payload["last_error"] = msg
	}
	if outcome.HTTPStatus > 0 {
		payload["http_status"] = float64(outcome.HTTPStatus)
	}
	if outcome.LatencyMs != nil && *outcome.LatencyMs >= 0 {
		payload["latency_ms"] = *outcome.LatencyMs
	}
	if outcome.FirstTokenMs != nil && *outcome.FirstTokenMs >= 0 {
		payload["first_token_ms"] = *outcome.FirstTokenMs
	}
	if fp := strings.TrimSpace(outcome.KeyFingerprint); fp != "" {
		payload["key_fingerprint"] = fp
	}
	if err := s.accountRepo.UpdateExtra(ctx, account.ID, map[string]any{AccountProbeHealthExtraKey: payload}); err != nil {
		return nil, err
	}
	if account.Extra == nil {
		account.Extra = map[string]any{}
	}
	account.Extra[AccountProbeHealthExtraKey] = payload
	return transition, nil
}

func (s *RateLimitService) recordLastAPIKeyDisabledOutcome(ctx context.Context, account *Account, statusCode int, headers http.Header, responseBody []byte, upstreamMsg string, reason string) bool {
	if s == nil || account == nil {
		return true
	}
	var blockedUntil *time.Time
	if statusCode == http.StatusTooManyRequests {
		persistOpenAI429PlanType(ctx, s.accountRepo, account, responseBody)
		s.persistOpenAICodexSnapshot(ctx, account, headers)
		if resetAt := s.calculateOpenAI429ResetTime(headers); resetAt != nil {
			blockedUntil = resetAt
		}
	}
	fp := FingerprintAPIKey(account.LastSelectedAPIKey())
	if fp == "" {
		fp = lastDisabledAPIKeyFingerprint(account)
	}
	_, err := s.RecordAccountProbeOutcome(ctx, AccountProbeOutcome{
		AccountID:      account.ID,
		Account:        account,
		Source:         AccountProbeOutcomeSourceGateway,
		Success:        false,
		ErrorMessage:   firstNonEmptyString(upstreamMsg, extractUpstreamErrorMessage(responseBody), fmt.Sprintf("HTTP %d", statusCode)),
		HTTPStatus:     statusCode,
		Reason:         reason,
		KeyFingerprint: fp,
		BlockedUntil:   blockedUntil,
	})
	if err != nil {
		slog.Warn("account_probe_outcome_last_key_failed", "account_id", account.ID, "status_code", statusCode, "reason", reason, "error", err)
	}
	return true
}

func (s *RateLimitService) applyAccountProbeBlockedState(ctx context.Context, account *Account, level string, until time.Time, reason string) error {
	if account == nil {
		return nil
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = level
	}
	switch level {
	case AccountProbeHealthRateLimited:
		now := time.Now()
		account.RateLimitedAt = &now
		account.RateLimitResetAt = &until
		s.notifyAccountSchedulingBlocked(account, until, reason)
		return s.accountRepo.SetRateLimited(ctx, account.ID, until)
	case AccountProbeHealthQuotaExhausted, AccountProbeHealthDisabled, AccountProbeHealthTempUnsched, AccountProbeHealthLineDegraded, AccountProbeHealthModerateAbnormal:
		account.TempUnschedulableUntil = &until
		account.TempUnschedulableReason = reason
		s.notifyAccountSchedulingBlocked(account, until, reason)
		return s.accountRepo.SetTempUnschedulable(ctx, account.ID, until, reason)
	default:
		return nil
	}
}

func accountProbeNextHealthLevel(account *Account, previous map[string]any, outcome AccountProbeOutcome) string {
	if outcome.Success {
		if account != nil && account.IsOpenAIApiKey() && len(account.GetAPIKeys()) == 0 {
			return AccountProbeHealthDisabled
		}
		return AccountProbeHealthNormal
	}
	reason := strings.ToLower(strings.TrimSpace(outcome.Reason))
	msg := strings.ToLower(strings.TrimSpace(outcome.ErrorMessage))
	switch {
	case account != nil && account.IsOpenAIApiKey() && len(account.GetAPIKeys()) == 0 && strings.Contains(reason, "rate"):
		return AccountProbeHealthRateLimited
	case account != nil && account.IsOpenAIApiKey() && len(account.GetAPIKeys()) == 0:
		return AccountProbeHealthDisabled
	case outcome.HTTPStatus == http.StatusTooManyRequests || strings.Contains(reason, "rate_limited"):
		return AccountProbeHealthRateLimited
	case outcome.HTTPStatus == http.StatusPaymentRequired || strings.Contains(reason, "payment") || strings.Contains(reason, "insufficient_balance") || strings.Contains(reason, "quota_exhausted"):
		return AccountProbeHealthQuotaExhausted
	case strings.Contains(reason, "invalid_api_key") || strings.Contains(reason, "key_revoked") || strings.Contains(reason, "api_key_disabled"):
		return AccountProbeHealthDisabled
	case strings.Contains(reason, "line_degraded") || strings.Contains(msg, "eof") || strings.Contains(msg, "timeout") || strings.Contains(msg, "proxy") || strings.Contains(msg, "network"):
		return AccountProbeHealthLineDegraded
	}
	failureCount := accountProbePayloadInt(previous["failure_count"]) + 1
	switch {
	case failureCount >= 3:
		return AccountProbeHealthTempUnsched
	case failureCount == 2:
		return AccountProbeHealthModerateAbnormal
	default:
		return AccountProbeHealthLightAbnormal
	}
}

func accountProbeOutcomeBlockedUntil(outcome AccountProbeOutcome, level string) *time.Time {
	if outcome.BlockedUntil != nil && outcome.BlockedUntil.After(outcome.ObservedAt) {
		return outcome.BlockedUntil
	}
	var d time.Duration
	switch level {
	case AccountProbeHealthRateLimited:
		d = 10 * time.Minute
	case AccountProbeHealthQuotaExhausted:
		d = 60 * time.Minute
	case AccountProbeHealthDisabled:
		d = 60 * time.Minute
	case AccountProbeHealthTempUnsched:
		d = 15 * time.Minute
	case AccountProbeHealthModerateAbnormal, AccountProbeHealthLineDegraded:
		d = 5 * time.Minute
	default:
		return nil
	}
	until := outcome.ObservedAt.Add(d)
	return &until
}

func accountProbeHealthPayload(account *Account) map[string]any {
	if account == nil || account.Extra == nil {
		return map[string]any{}
	}
	switch v := account.Extra[AccountProbeHealthExtraKey].(type) {
	case map[string]any:
		return cloneCredentials(v)
	case map[string]string:
		out := make(map[string]any, len(v))
		for key, value := range v {
			out[key] = value
		}
		return out
	default:
		return map[string]any{}
	}
}

func accountProbeHealthLevel(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	level, ok := payload["level"]
	if !ok || level == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(level))
}

// AccountProbeHealthLabel 返回账号探测健康层级的管理端展示文案。
func AccountProbeHealthLabel(level string) string {
	switch strings.TrimSpace(level) {
	case AccountProbeHealthNormal:
		return "正常"
	case AccountProbeHealthLightAbnormal:
		return "轻微异常"
	case AccountProbeHealthModerateAbnormal:
		return "中度异常"
	case AccountProbeHealthLineDegraded:
		return "线路降级"
	case AccountProbeHealthTempUnsched:
		return "临时不可调度"
	case AccountProbeHealthRateLimited:
		return "限流"
	case AccountProbeHealthQuotaLow:
		return "余额不足"
	case AccountProbeHealthQuotaExhausted:
		return "余额耗尽"
	case AccountProbeHealthDisabled:
		return "停用"
	case AccountProbeHealthPendingRetest:
		return "待复测"
	default:
		if strings.TrimSpace(level) == "" {
			return "未知"
		}
		return strings.TrimSpace(level)
	}
}

func accountProbePayloadInt(value any) int {
	return int(parseExtraFloat64(value))
}

func normalizeAccountProbeOutcomeSource(source string) string {
	source = strings.TrimSpace(source)
	switch source {
	case AccountProbeOutcomeSourceManualTest, AccountProbeOutcomeSourceAccountProbe, AccountProbeOutcomeSourceBatchTest, AccountProbeOutcomeSourceGateway:
		return source
	default:
		if source == "" {
			return AccountProbeOutcomeSourceManualTest
		}
		return source
	}
}

func normalizeAccountProbeOutcomeReason(outcome AccountProbeOutcome) string {
	reason := strings.ToLower(strings.TrimSpace(outcome.Reason))
	if reason != "" {
		return reason
	}
	msg := strings.ToLower(strings.TrimSpace(outcome.ErrorMessage))
	switch {
	case outcome.HTTPStatus == http.StatusTooManyRequests:
		return "rate_limited"
	case outcome.HTTPStatus == http.StatusPaymentRequired:
		return "payment_required"
	case strings.Contains(msg, "insufficient balance") || strings.Contains(msg, "insufficient quota") || strings.Contains(msg, "credit balance"):
		return "insufficient_balance"
	case strings.Contains(msg, "invalid api key") || strings.Contains(msg, "api key has been disabled"):
		return "invalid_api_key"
	case msg != "":
		return "upstream_abnormal"
	default:
		return "probe_success"
	}
}

func canRecoverAccountAfterProbeSuccess(account *Account) bool {
	if account == nil {
		return false
	}
	if account.IsOpenAIApiKey() && len(account.GetAPIKeys()) == 0 {
		return false
	}
	return true
}

func lastDisabledAPIKeyFingerprint(account *Account) string {
	if account == nil || account.Credentials == nil {
		return ""
	}
	disabled := normalizeDisabledAPIKeyFingerprints(account.Credentials[CredentialAPIKeysDisabled])
	for fp := range disabled {
		return fp
	}
	return ""
}
