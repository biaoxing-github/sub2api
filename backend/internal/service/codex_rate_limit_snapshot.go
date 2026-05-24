package service

import "time"

func codexSnapshotRateLimitResetAt(updates map[string]any, now time.Time) *time.Time {
	if len(updates) == 0 {
		return nil
	}
	if resetAt := codexWindowExhaustedResetAt(updates, "7d", now); resetAt != nil {
		return resetAt
	}
	return codexWindowExhaustedResetAt(updates, "5h", now)
}

func codexSnapshotShouldLimitAccount(account *Account) bool {
	return account != nil && account.IsOpenAIOAuth()
}
