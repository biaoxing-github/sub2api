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
	case http.StatusUnauthorized, http.StatusPaymentRequired:
		return true
	case http.StatusForbidden:
		return isInsufficientBalanceBody(responseBody)
	default:
		return false
	}
}

func disableAPIKeyReason(statusCode int, responseBody []byte) string {
	if isInsufficientBalanceBody(responseBody) {
		return "insufficient_balance"
	}
	if statusCode == http.StatusUnauthorized {
		return "invalid_api_key"
	}
	if statusCode == http.StatusPaymentRequired {
		return "payment_required"
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
