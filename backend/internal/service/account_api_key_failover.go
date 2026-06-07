package service

import (
	"context"
	"net/http"
	"strings"
	"time"

	"log/slog"

	"github.com/tidwall/gjson"
)

func shouldDisableCurrentAPIKey(statusCode int, responseBody []byte) bool {
	switch statusCode {
	case http.StatusBadRequest:
		return isInsufficientBalanceBody(responseBody)
	case http.StatusUnauthorized, http.StatusPaymentRequired:
		return true
	case http.StatusTooManyRequests:
		return true
	case http.StatusForbidden:
		return isInsufficientBalanceBody(responseBody) || isInvalidAPIKeyBody(responseBody)
	default:
		return false
	}
}

func disableAPIKeyReason(statusCode int, responseBody []byte) string {
	if isInsufficientBalanceBody(responseBody) {
		return "insufficient_balance"
	}
	if isInvalidAPIKeyBody(responseBody) {
		return "invalid_api_key"
	}
	if statusCode == http.StatusUnauthorized {
		return "invalid_api_key"
	}
	if statusCode == http.StatusPaymentRequired {
		return "payment_required"
	}
	if statusCode == http.StatusTooManyRequests {
		return "rate_limited"
	}
	return "upstream_error"
}

func isInsufficientBalanceBody(responseBody []byte) bool {
	lower := strings.ToLower(strings.TrimSpace(string(responseBody)))
	if lower == "" {
		return false
	}
	code := strings.ToLower(strings.TrimSpace(gjson.GetBytes(responseBody, "code").String()))
	if code == "" {
		code = strings.ToLower(strings.TrimSpace(gjson.GetBytes(responseBody, "error.code").String()))
	}
	if code == "insufficient_balance" || code == "insufficient_quota" {
		return true
	}
	return strings.Contains(lower, "insufficient account balance") ||
		strings.Contains(lower, "insufficient balance") ||
		strings.Contains(lower, "insufficient quota") ||
		strings.Contains(lower, "credit balance") ||
		strings.Contains(lower, "billing issue")
}

// isInvalidAPIKeyBody 判断错误体是否明确指向当前 API Key 失效、撤销或被禁用。
func isInvalidAPIKeyBody(responseBody []byte) bool {
	lower := strings.ToLower(strings.TrimSpace(string(responseBody)))
	if lower == "" {
		return false
	}
	code := strings.ToLower(strings.TrimSpace(gjson.GetBytes(responseBody, "code").String()))
	if code == "" {
		code = strings.ToLower(strings.TrimSpace(gjson.GetBytes(responseBody, "error.code").String()))
	}
	switch code {
	case "invalid_api_key", "api_key_disabled", "key_disabled", "api_key_revoked", "key_revoked":
		return true
	}
	return strings.Contains(lower, "invalid api key") ||
		strings.Contains(lower, "api key has been disabled") ||
		strings.Contains(lower, "api key disabled") ||
		strings.Contains(lower, "api key revoked") ||
		strings.Contains(lower, "key revoked")
}

func disableAccountAPIKey(ctx context.Context, repo AccountRepository, account *Account, apiKey, reason string) bool {
	if repo == nil || account == nil || strings.TrimSpace(apiKey) == "" {
		return false
	}
	if len(account.GetAPIKeys()) == 0 {
		return false
	}
	changed := account.DisableAPIKey(apiKey, reason, time.Now())
	if !changed {
		return false
	}
	if err := persistAccountCredentials(ctx, repo, account, account.Credentials); err != nil {
		slog.Warn("account_api_key_disable_failed", "account_id", account.ID, "reason", reason, "error", err)
		return false
	}
	slog.Warn("account_api_key_disabled", "account_id", account.ID, "reason", reason, "remaining_keys", len(account.GetAPIKeys()))
	return true
}
