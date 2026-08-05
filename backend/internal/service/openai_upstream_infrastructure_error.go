package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const (
	openAIUpstreamInfrastructureRuleIndex = -2
	openAIUpstreamResourceMinimumCooldown = 5 * time.Minute
)

// openAIUpstreamInfrastructureFailure 描述共享基础设施故障对应的账号级冷却策略。
type openAIUpstreamInfrastructureFailure struct {
	Reason          string
	MinimumCooldown time.Duration
}

// classifyOpenAIUpstreamInfrastructureFailure 只识别继续请求同一上游也无法恢复的高置信度故障。
func classifyOpenAIUpstreamInfrastructureFailure(statusCode int, message string, responseBody []byte) (openAIUpstreamInfrastructureFailure, bool) {
	if statusCode == 529 || statusCode == http.StatusUnauthorized || statusCode == http.StatusPaymentRequired ||
		statusCode == http.StatusForbidden || statusCode == http.StatusTooManyRequests {
		return openAIUpstreamInfrastructureFailure{}, false
	}
	if isOpenAIContextWindowError(message, responseBody) || isOpenAIModelNotFoundError(statusCode, responseBody) {
		return openAIUpstreamInfrastructureFailure{}, false
	}
	if statusCode == http.StatusRequestEntityTooLarge {
		return openAIUpstreamInfrastructureFailure{
			Reason:          "upstream_request_entity_too_large",
			MinimumCooldown: openAIUpstreamResourceMinimumCooldown,
		}, true
	}
	// 424 表示当前上游依赖不可用，请求本身无法通过同一账号立即恢复。
	if statusCode == http.StatusFailedDependency {
		return openAIUpstreamInfrastructureFailure{Reason: "upstream_failed_dependency"}, true
	}

	classification := ClassifyUpstreamError(UpstreamErrorInput{
		StatusCode: statusCode,
		Message:    message,
		Body:       responseBody,
	})
	if classification.Category == UpstreamErrorCategoryPreviousResponseNotFound ||
		classification.Category == UpstreamErrorCategoryUnauthorized ||
		classification.Category == UpstreamErrorCategoryRateLimited ||
		classification.Category == UpstreamErrorCategoryQuota ||
		classification.Category == UpstreamErrorCategoryBusinessLimited ||
		classification.Category == UpstreamErrorCategoryReauthRequired {
		return openAIUpstreamInfrastructureFailure{}, false
	}

	raw := strings.TrimSpace(message + " " + string(responseBody))
	if reason, _, matched := classifyUpstreamResourceExhaustion(statusCode, raw); matched {
		return openAIUpstreamInfrastructureFailure{
			Reason:          reason,
			MinimumCooldown: openAIUpstreamResourceMinimumCooldown,
		}, true
	}
	if statusCode >= http.StatusInternalServerError && statusCode < 600 {
		return openAIUpstreamInfrastructureFailure{Reason: "upstream_server_error"}, true
	}
	return openAIUpstreamInfrastructureFailure{}, false
}

// IsOpenAIUpstreamInfrastructureFailure 供 handler 和其他边界层复用同一账号级基础设施故障判定。
func IsOpenAIUpstreamInfrastructureFailure(statusCode int, message string, responseBody []byte) bool {
	_, matched := classifyOpenAIUpstreamInfrastructureFailure(statusCode, message, responseBody)
	return matched
}

// classifyUpstreamResourceExhaustion 返回资源耗尽故障的稳定原因和展示标签。
func classifyUpstreamResourceExhaustion(statusCode int, raw string) (reason string, label string, matched bool) {
	if statusCode == 529 || statusCode == http.StatusUnauthorized || statusCode == http.StatusPaymentRequired ||
		statusCode == http.StatusForbidden || statusCode == http.StatusTooManyRequests || statusCode == http.StatusRequestEntityTooLarge {
		return "", "", false
	}
	if statusCode == http.StatusInsufficientStorage {
		return "upstream_resource_exhausted", "上游资源耗尽/507", true
	}

	lower := strings.ToLower(strings.TrimSpace(raw))
	if lower == "" {
		return "", "", false
	}
	switch {
	case containsAnyUpstreamErrorText(lower,
		"disk storage creation failed",
		"disk free-space floor reached",
		"failed to write to temp file",
		"failed to create temporary file",
		"failed to create temp file",
		"could not create temporary file",
		"unable to create temporary file",
		"no space left on device",
		"insufficient storage",
		"disk quota exceeded",
		"filesystem is full",
		"file system is full",
		"not enough space on the disk",
		"read-only file system",
	):
		return "upstream_storage_unavailable", "上游存储不可用", true
	case containsAnyUpstreamErrorText(lower,
		"out of memory",
		"cannot allocate memory",
		"memory allocation failed",
		"failed to allocate memory",
		"memory limit exceeded",
		"memory exhausted",
		"oom killed",
		"oom-killed",
	):
		return "upstream_memory_exhausted", "上游内存耗尽", true
	case containsAnyUpstreamErrorText(lower,
		"too many open files",
		"file descriptor limit",
		"file descriptors exhausted",
		"file table overflow",
		"emfile",
	):
		return "upstream_file_descriptors_exhausted", "上游文件句柄耗尽", true
	case containsAnyUpstreamErrorText(lower,
		"database connection pool exhausted",
		"connection pool exhausted",
		"too many database connections",
		"too many clients already",
		"remaining connection slots are reserved",
		"failed to acquire database connection",
		"database connection pool timeout",
		"database connection pool is full",
		"database connection pool limit reached",
	):
		return "upstream_database_exhausted", "上游数据库连接耗尽", true
	default:
		return "", "", false
	}
}

// handleOpenAIUpstreamInfrastructureFailure 在响应规则前完成基础设施故障的账号状态更新。
func (s *OpenAIGatewayService) handleOpenAIUpstreamInfrastructureFailure(
	ctx context.Context,
	account *Account,
	statusCode int,
	headers http.Header,
	responseBody []byte,
	requestedModel string,
) bool {
	if _, matched := classifyOpenAIUpstreamInfrastructureFailure(statusCode, "", responseBody); !matched {
		return false
	}
	return s.handleOpenAIAccountUpstreamError(ctx, account, statusCode, headers, responseBody, requestedModel)
}

// tryOpenAIUpstreamInfrastructureCooldown 把共享基础设施故障写成账号级临时不可调度状态。
func (s *RateLimitService) tryOpenAIUpstreamInfrastructureCooldown(
	ctx context.Context,
	account *Account,
	statusCode int,
	upstreamMessage string,
	responseBody []byte,
) bool {
	if account == nil || account.Platform != PlatformOpenAI {
		return false
	}
	failure, matched := classifyOpenAIUpstreamInfrastructureFailure(statusCode, upstreamMessage, responseBody)
	if !matched {
		return false
	}

	previousReason := strings.TrimSpace(account.TempUnschedulableReason)
	var existingUntil *time.Time
	if account.TempUnschedulableUntil != nil {
		until := *account.TempUnschedulableUntil
		existingUntil = &until
	}
	if s.accountRepo != nil {
		if dbAccount, err := s.accountRepo.GetByID(ctx, account.ID); err == nil && dbAccount != nil {
			if previousReason == "" {
				previousReason = strings.TrimSpace(dbAccount.TempUnschedulableReason)
			}
			if dbAccount.TempUnschedulableUntil != nil && (existingUntil == nil || dbAccount.TempUnschedulableUntil.After(*existingUntil)) {
				until := *dbAccount.TempUnschedulableUntil
				existingUntil = &until
			}
		}
	}

	errorCount := nextOpenAIUpstreamInfrastructureErrorCount(previousReason, failure.Reason)
	cooldown := probeIntervalFromErrorCount(errorCount)
	if cooldown < failure.MinimumCooldown {
		cooldown = failure.MinimumCooldown
	}
	now := time.Now()
	until := now.Add(cooldown)
	if existingUntil != nil && existingUntil.After(until) {
		until = *existingUntil
	}
	state := &TempUnschedState{
		UntilUnix:       until.Unix(),
		TriggeredAtUnix: now.Unix(),
		StatusCode:      statusCode,
		MatchedKeyword:  failure.Reason,
		RuleIndex:       openAIUpstreamInfrastructureRuleIndex,
		ErrorCount:      errorCount,
		ErrorMessage:    truncateTempUnschedMessage(responseBody, tempUnschedMessageMaxBytes),
	}
	reason := failure.Reason
	if raw, err := json.Marshal(state); err == nil {
		reason = string(raw)
	}

	account.TempUnschedulableUntil = &until
	account.TempUnschedulableReason = reason
	s.notifyAccountSchedulingBlocked(account, until, failure.Reason)
	if s.accountRepo != nil {
		if err := s.accountRepo.SetTempUnschedulable(ctx, account.ID, until, reason); err != nil {
			slog.Warn("openai_upstream_infrastructure_cooldown_failed", "account_id", account.ID, "status_code", statusCode, "reason", failure.Reason, "error", err)
		}
	}
	if s.tempUnschedCache != nil {
		if err := s.tempUnschedCache.SetTempUnsched(ctx, account.ID, state); err != nil {
			slog.Warn("temp_unsched_cache_set_failed", "account_id", account.ID, "error", err)
		}
	}

	slog.Warn("openai_upstream_infrastructure_cooldown", "account_id", account.ID, "status_code", statusCode, "reason", failure.Reason, "error_count", errorCount, "until", until)
	return true
}

func nextOpenAIUpstreamInfrastructureErrorCount(previousReason string, reason string) int {
	var state TempUnschedState
	if err := json.Unmarshal([]byte(strings.TrimSpace(previousReason)), &state); err != nil {
		return 1
	}
	if state.RuleIndex != openAIUpstreamInfrastructureRuleIndex || !strings.EqualFold(state.MatchedKeyword, reason) {
		return 1
	}
	if state.ErrorCount <= 0 {
		return 2
	}
	return state.ErrorCount + 1
}
