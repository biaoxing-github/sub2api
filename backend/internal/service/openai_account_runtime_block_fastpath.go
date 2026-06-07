package service

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const (
	openAIAccountStateUpdateTimeout       = 5 * time.Second
	openAIOAuth429FallbackCooldown        = 5 * time.Second
	openAIStopSchedulingBridgeCooldown    = 2 * time.Minute
	openAIOAuth429StormWindow             = 10 * time.Second
	openAIOAuth429StormThreshold          = 20
	openAIOAuth429StormMaxAccountSwitches = 1
)

type OpenAIAccountRuntimeBlockSnapshot struct {
	Reason string     `json:"reason,omitempty"`
	Until  *time.Time `json:"until,omitempty"`
}

func openAIAccountStateContext(ctx context.Context) (context.Context, context.CancelFunc) {
	base := context.Background()
	if ctx != nil {
		base = context.WithoutCancel(ctx)
	}
	return context.WithTimeout(base, openAIAccountStateUpdateTimeout)
}

func isOpenAIOAuthAccount(account *Account) bool {
	return account != nil && account.Platform == PlatformOpenAI && account.Type == AccountTypeOAuth
}

func isOpenAIAccount(account *Account) bool {
	return account != nil && account.Platform == PlatformOpenAI
}

func (s *OpenAIGatewayService) handleOpenAIAccountUpstreamError(ctx context.Context, account *Account, statusCode int, headers http.Header, responseBody []byte, requestedModel ...string) bool {
	model := ""
	if len(requestedModel) > 0 {
		model = requestedModel[0]
	}
	return s.handleOpenAIAccountUpstreamErrorForModel(ctx, account, statusCode, headers, responseBody, model)
}

func (s *OpenAIGatewayService) handleOpenAIAccountUpstreamErrorForModel(ctx context.Context, account *Account, statusCode int, headers http.Header, responseBody []byte, requestedModel string) bool {
	stateCtx, cancel := openAIAccountStateContext(ctx)
	defer cancel()

	if s.handleOpenAIModelNotFoundCooldown(stateCtx, account, statusCode, responseBody, requestedModel) {
		return false
	}
	if statusCode == http.StatusTooManyRequests {
		s.markOpenAIOAuth429RateLimited(stateCtx, account, headers, responseBody)
	}
	if s == nil || account == nil || s.rateLimitService == nil {
		return false
	}
	shouldDisable := s.rateLimitService.HandleUpstreamError(stateCtx, account, statusCode, headers, responseBody)
	if shouldDisable {
		s.BlockAccountScheduling(account, time.Time{}, "upstream_disable")
	}
	return shouldDisable
}

func (s *OpenAIGatewayService) handleOpenAIModelNotFoundCooldown(ctx context.Context, account *Account, statusCode int, responseBody []byte, requestedModel string) bool {
	if s == nil || account == nil || !account.IsOpenAI() || !isOpenAIModelNotFoundError(statusCode, responseBody) {
		return false
	}
	modelKey := resolveOpenAIModelRateLimitKey(account, requestedModel)
	if modelKey == "" {
		return false
	}
	resetAt := time.Now().Add(openAIModelNotFoundCooldown)
	if s.accountRepo != nil {
		if err := s.accountRepo.SetModelRateLimit(ctx, account.ID, modelKey, resetAt); err != nil {
			slog.Warn("openai_model_not_found_cooldown_failed", "account_id", account.ID, "model", modelKey, "error", err)
		}
	}
	updateAccountModelRateLimitExtra(account, modelKey, resetAt)
	slog.Info("openai_model_not_found_cooldown", "account_id", account.ID, "model", modelKey, "reset_at", resetAt)
	return true
}

func updateAccountModelRateLimitExtra(account *Account, modelKey string, resetAt time.Time) {
	if account == nil || modelKey == "" {
		return
	}
	if account.Extra == nil {
		account.Extra = make(map[string]any)
	}
	limits, _ := account.Extra[modelRateLimitsKey].(map[string]any)
	if limits == nil {
		limits = make(map[string]any)
	}
	limits[modelKey] = map[string]any{
		"rate_limited_at":     time.Now().UTC().Format(time.RFC3339),
		"rate_limit_reset_at": resetAt.UTC().Format(time.RFC3339),
	}
	account.Extra[modelRateLimitsKey] = limits
}

func (s *OpenAIGatewayService) markOpenAIOAuth429RateLimited(ctx context.Context, account *Account, headers http.Header, responseBody []byte) {
	if s == nil || !isOpenAIOAuthAccount(account) {
		return
	}
	s.recordOpenAIOAuth429()

	cooldownUntil := time.Now().Add(openAIOAuth429FallbackCooldown)
	if s.rateLimitService != nil {
		if resetAt := s.rateLimitService.calculateOpenAI429ResetTime(headers); resetAt != nil && resetAt.After(time.Now()) {
			cooldownUntil = *resetAt
		} else if resetUnix := parseOpenAIRateLimitResetTime(responseBody); resetUnix != nil {
			if resetAt := time.Unix(*resetUnix, 0); resetAt.After(time.Now()) {
				cooldownUntil = resetAt
			}
		} else if cooldown, ok := s.rateLimitService.get429FallbackCooldown(ctx, account); ok && cooldown > 0 {
			cooldownUntil = time.Now().Add(cooldown)
		}
	}
	s.BlockAccountScheduling(account, cooldownUntil, "429")
}

func (s *OpenAIGatewayService) BlockAccountScheduling(account *Account, until time.Time, reason string) {
	if s == nil || !isOpenAIAccount(account) {
		return
	}
	now := time.Now()
	blockUntil := until
	if blockUntil.IsZero() || !blockUntil.After(now) {
		blockUntil = now.Add(openAIStopSchedulingBridgeCooldown)
	}
	reason = strings.TrimSpace(reason)

	for {
		current, loaded := s.openaiAccountRuntimeBlockUntil.Load(account.ID)
		if !loaded {
			actual, stored := s.openaiAccountRuntimeBlockUntil.LoadOrStore(account.ID, blockUntil)
			if !stored {
				s.storeOpenAIAccountRuntimeBlockReason(account.ID, reason)
				return
			}
			current = actual
		}

		currentUntil, ok := current.(time.Time)
		if !ok || currentUntil.IsZero() {
			if s.openaiAccountRuntimeBlockUntil.CompareAndSwap(account.ID, current, blockUntil) {
				return
			}
			continue
		}
		if currentUntil.After(blockUntil) {
			return
		}
		if s.openaiAccountRuntimeBlockUntil.CompareAndSwap(account.ID, current, blockUntil) {
			s.storeOpenAIAccountRuntimeBlockReason(account.ID, reason)
			return
		}
	}
}

func (s *OpenAIGatewayService) storeOpenAIAccountRuntimeBlockReason(accountID int64, reason string) {
	if s == nil || accountID <= 0 {
		return
	}
	if reason == "" {
		s.openaiAccountRuntimeBlockReason.Delete(accountID)
		return
	}
	s.openaiAccountRuntimeBlockReason.Store(accountID, reason)
}

func (s *OpenAIGatewayService) ClearAccountSchedulingBlock(accountID int64) {
	if s == nil || accountID <= 0 {
		return
	}
	s.openaiAccountRuntimeBlockUntil.Delete(accountID)
	s.openaiAccountRuntimeBlockReason.Delete(accountID)
}

func (s *OpenAIGatewayService) SnapshotOpenAIAccountRuntimeBlock(account *Account, now time.Time) (OpenAIAccountRuntimeBlockSnapshot, bool) {
	if s == nil || !isOpenAIAccount(account) {
		return OpenAIAccountRuntimeBlockSnapshot{}, false
	}
	if now.IsZero() {
		now = time.Now()
	}
	value, ok := s.openaiAccountRuntimeBlockUntil.Load(account.ID)
	if !ok {
		return OpenAIAccountRuntimeBlockSnapshot{}, false
	}
	cooldownUntil, ok := value.(time.Time)
	if !ok || cooldownUntil.IsZero() {
		s.ClearAccountSchedulingBlock(account.ID)
		return OpenAIAccountRuntimeBlockSnapshot{}, false
	}
	if !now.Before(cooldownUntil) {
		s.ClearAccountSchedulingBlock(account.ID)
		return OpenAIAccountRuntimeBlockSnapshot{}, false
	}
	reason := ""
	if raw, ok := s.openaiAccountRuntimeBlockReason.Load(account.ID); ok {
		reason, _ = raw.(string)
		reason = strings.TrimSpace(reason)
	}
	until := cooldownUntil
	return OpenAIAccountRuntimeBlockSnapshot{
		Reason: reason,
		Until:  &until,
	}, true
}

func (s *OpenAIGatewayService) isOpenAIAccountRuntimeBlocked(account *Account) bool {
	_, ok := s.SnapshotOpenAIAccountRuntimeBlock(account, time.Now())
	return ok
}

func (s *OpenAIGatewayService) recordOpenAIOAuth429() {
	if s == nil {
		return
	}
	now := time.Now()
	windowStart := s.openaiOAuth429WindowStartUnixNano.Load()
	if windowStart == 0 || now.Sub(time.Unix(0, windowStart)) >= openAIOAuth429StormWindow {
		if s.openaiOAuth429WindowStartUnixNano.CompareAndSwap(windowStart, now.UnixNano()) {
			s.openaiOAuth429WindowCount.Store(1)
			return
		}
	}
	s.openaiOAuth429WindowCount.Add(1)
}

func (s *OpenAIGatewayService) isOpenAIOAuth429Storm() bool {
	if s == nil {
		return false
	}
	windowStart := s.openaiOAuth429WindowStartUnixNano.Load()
	if windowStart == 0 || time.Since(time.Unix(0, windowStart)) >= openAIOAuth429StormWindow {
		return false
	}
	return s.openaiOAuth429WindowCount.Load() >= openAIOAuth429StormThreshold
}

func (s *OpenAIGatewayService) ShouldStopOpenAIOAuth429Failover(account *Account, statusCode int, failedSwitches int) bool {
	if statusCode != http.StatusTooManyRequests || failedSwitches < openAIOAuth429StormMaxAccountSwitches {
		return false
	}
	if !isOpenAIOAuthAccount(account) {
		return false
	}
	return s.isOpenAIOAuth429Storm()
}
