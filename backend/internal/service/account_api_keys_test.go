package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAccountGetAPIKeyRotatesCredentialAPIKeys(t *testing.T) {
	account := &Account{
		Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_keys": []any{" key-a ", "", "key-b"},
			"api_key":  "legacy-key",
		},
	}

	require.Equal(t, "key-a", account.GetAPIKey())
	require.Equal(t, "key-b", account.GetAPIKey())
	require.Equal(t, "key-a", account.GetAPIKey())
}

func TestAccountGetAPIKeyFallsBackToLegacyAPIKey(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_keys": []any{"", " "},
			"api_key":  "legacy-key",
		},
	}

	require.Equal(t, "legacy-key", account.GetAPIKey())
	require.Equal(t, "legacy-key", account.GetOpenAIApiKey())
}

func TestAccountGetAPIKeySkipsDisabledAPIKeys(t *testing.T) {
	account := &Account{
		ID:   42,
		Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_keys": []any{"key-a", "key-b"},
		},
	}
	require.True(t, account.DisableAPIKey("key-a", "insufficient_balance", testNow()))

	require.Equal(t, "key-b", account.GetAPIKey())
	require.Equal(t, "key-b", account.LastSelectedAPIKey())
	require.Equal(t, []string{"key-b"}, account.GetAPIKeys())
}

func testNow() time.Time {
	return time.Unix(1700000000, 0).UTC()
}
