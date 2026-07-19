package service

import (
	"context"
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

func TestAccountGetAPIKeysFallsBackToLegacyAPIKeyAndDisabledMetadata(t *testing.T) {
	now := testNow()
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "legacy-key",
		},
		nowForTest: &now,
	}

	require.Equal(t, []string{"legacy-key"}, account.GetAPIKeys())
	require.True(t, account.DisableAPIKey("legacy-key", "invalid_api_key", now))
	require.Empty(t, account.GetAPIKeys())
	require.Empty(t, account.GetAPIKey())
}

func TestAccountGetAPIKeySkipsDisabledAPIKeys(t *testing.T) {
	now := testNow()
	account := &Account{
		ID:   42,
		Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_keys": []any{"key-a", "key-b"},
		},
		nowForTest: &now,
	}
	require.True(t, account.DisableAPIKey("key-a", "insufficient_balance", now))

	require.Equal(t, "key-b", account.GetAPIKey())
	require.Equal(t, "key-b", account.LastSelectedAPIKey())
	require.Equal(t, []string{"key-b"}, account.GetAPIKeys())
}

// 确认通用网关取凭证也复用多 Key 选择逻辑，便于 429 时定位本次使用的 Key。
func TestGatewayServiceGetAccessTokenUsesCredentialAPIKeys(t *testing.T) {
	account := &Account{
		ID:   145,
		Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_keys": []any{"key-a", "key-b"},
		},
	}
	svc := &GatewayService{}

	token, tokenType, err := svc.GetAccessToken(context.Background(), account)

	require.NoError(t, err)
	require.Equal(t, "key-a", token)
	require.Equal(t, "apikey", tokenType)
	require.Equal(t, "key-a", account.LastSelectedAPIKey())
}

// 确认删除 Key 时同步清理停用元数据，避免前端继续展示已删除 Key 的状态。
func TestAccountRemoveAPIKeyByFingerprintRemovesKeyAndDisabledMetadata(t *testing.T) {
	account := &Account{
		ID:   43,
		Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_keys": []any{"key-a", "key-b"},
		},
	}
	require.True(t, account.DisableAPIKey("key-a", "rate_limited", testNow()))

	require.True(t, account.RemoveAPIKeyByFingerprint(FingerprintAPIKey("key-a")))

	require.Equal(t, []string{"key-b"}, normalizeAPIKeys(account.Credentials["api_keys"]))
	require.Equal(t, []string{"key-b"}, account.GetAPIKeys())
	disabled, _ := account.Credentials[CredentialAPIKeysDisabled].(map[string]any)
	require.NotContains(t, disabled, FingerprintAPIKey("key-a"))
}

// 确认旧版单 api_key 字段也可按指纹删除，兼容未迁移到 api_keys 列表的账号。
func TestAccountRemoveAPIKeyByFingerprintSupportsLegacySingleAPIKey(t *testing.T) {
	account := &Account{
		ID:   44,
		Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "legacy-key",
		},
	}

	require.True(t, account.RemoveAPIKeyByFingerprint(FingerprintAPIKey("legacy-key")))

	require.NotContains(t, account.Credentials, "api_key")
	require.Empty(t, account.GetAPIKeys())
	require.Empty(t, account.GetAPIKey())
}

// 确认恢复单个 Key 状态只清理停用元数据，不删除账号中保存的 Key。
func TestAccountRestoreAPIKeyByFingerprintClearsDisabledMetadata(t *testing.T) {
	account := &Account{
		ID:   45,
		Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_keys": []any{"key-a", "key-b"},
		},
	}
	require.True(t, account.DisableAPIKey("key-a", "rate_limited", testNow()))

	exists, restored := account.RestoreAPIKeyByFingerprint(FingerprintAPIKey("key-a"))

	require.True(t, exists)
	require.True(t, restored)
	require.Equal(t, []string{"key-a", "key-b"}, account.GetAPIKeys())
	disabled, _ := account.Credentials[CredentialAPIKeysDisabled].(map[string]any)
	require.NotContains(t, disabled, FingerprintAPIKey("key-a"))
}

// 确认恢复不存在的 Key 不会误改账号凭证。
func TestAccountRestoreAPIKeyByFingerprintRejectsUnknownFingerprint(t *testing.T) {
	account := &Account{
		ID:   46,
		Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_keys": []any{"key-a"},
		},
	}

	exists, restored := account.RestoreAPIKeyByFingerprint(FingerprintAPIKey("missing"))

	require.False(t, exists)
	require.False(t, restored)
	require.Equal(t, []string{"key-a"}, account.GetAPIKeys())
}

func testNow() time.Time {
	return time.Unix(1700000000, 0).UTC()
}

func TestAccountDisableAPIKeyWritesDisabledUntilAndCount(t *testing.T) {
	tests := []struct {
		name          string
		reason        string
		existingCount int
		wantInterval  time.Duration
	}{
		{"rate_limited first", "rate_limited", 0, 1 * time.Second},
		{"rate_limited third", "rate_limited", 2, 10 * time.Second},
		{"rate_limited ninth", "rate_limited", 8, 60 * time.Minute},
		{"service_unavailable first", "service_unavailable", 0, 5 * time.Second},
		{"service_unavailable third", "service_unavailable", 2, 15 * time.Second},
		{"service_unavailable eighth", "service_unavailable", 7, 60 * time.Second},
		{"invalid_api_key first", "invalid_api_key", 0, 30 * time.Minute},
		{"invalid_api_key second", "invalid_api_key", 1, 60 * time.Minute},
		{"payment_required first", "payment_required", 0, 30 * time.Minute},
		{"insufficient_balance third", "insufficient_balance", 2, 60 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := &Account{
				ID:          50,
				Type:        AccountTypeAPIKey,
				Credentials: map[string]any{"api_keys": []any{"test-key"}},
			}
			if tt.existingCount > 0 {
				fp := FingerprintAPIKey("test-key")
				account.Credentials[CredentialAPIKeysDisabled] = map[string]any{
					fp: map[string]any{
						"reason":         tt.reason,
						"disabled_at":    testNow().Add(-time.Hour).UTC().Format(time.RFC3339),
						"disabled_until": testNow().Add(-time.Minute).UTC().Format(time.RFC3339),
						"disabled_count": tt.existingCount,
					},
				}
			}

			now := testNow()
			changed := account.DisableAPIKey("test-key", tt.reason, now)

			require.True(t, changed)
			disabled, _ := account.Credentials[CredentialAPIKeysDisabled].(map[string]any)
			fp := FingerprintAPIKey("test-key")
			record, _ := disabled[fp].(map[string]any)
			require.NotNil(t, record)
			require.Equal(t, tt.reason, record["reason"])
			require.Contains(t, record, "disabled_until")
			require.Contains(t, record, "disabled_count")

			until, _ := time.Parse(time.RFC3339, record["disabled_until"].(string))
			expectedUntil := now.Add(tt.wantInterval)
			require.WithinDuration(t, expectedUntil, until, time.Second)

			count, _ := record["disabled_count"].(int)
			require.Equal(t, tt.existingCount+1, count)
		})
	}
}

func TestAccountGetAPIKeysLazilyRecoverExpiredDisabledKeys(t *testing.T) {
	now := testNow()
	account := &Account{
		ID:   60,
		Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_keys": []any{"key-a", "key-b", "key-c"},
			CredentialAPIKeysDisabled: map[string]any{
				FingerprintAPIKey("key-a"): map[string]any{
					"reason":         "rate_limited",
					"disabled_at":    now.Add(-time.Hour).UTC().Format(time.RFC3339),
					"disabled_until": now.Add(-10 * time.Minute).UTC().Format(time.RFC3339),
					"disabled_count": 2,
				},
				FingerprintAPIKey("key-b"): map[string]any{
					"reason":         "rate_limited",
					"disabled_at":    now.Add(-time.Minute).UTC().Format(time.RFC3339),
					"disabled_until": now.Add(10 * time.Minute).UTC().Format(time.RFC3339),
					"disabled_count": 1,
				},
			},
		},
		nowForTest: &now,
	}

	keys := account.GetAPIKeys()

	require.ElementsMatch(t, []string{"key-a", "key-c"}, keys, "key-a expired and should be recovered, key-b still disabled, key-c never disabled")
}

func TestAccountGetAPIKeysBackfillsLegacyDisabledRecordsWithoutUntil(t *testing.T) {
	now := testNow()
	account := &Account{
		ID:   61,
		Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_keys": []any{"key-old", "key-new"},
			CredentialAPIKeysDisabled: map[string]any{
				FingerprintAPIKey("key-old"): map[string]any{
					"reason":      "rate_limited",
					"disabled_at": now.Add(-time.Hour).UTC().Format(time.RFC3339),
				},
			},
		},
		nowForTest: &now,
	}

	keys := account.GetAPIKeys()

	require.ElementsMatch(t, []string{"key-old", "key-new"}, keys, "legacy record without disabled_until should be backfilled and recovered immediately")
	disabled, _ := account.Credentials[CredentialAPIKeysDisabled].(map[string]any)
	record, _ := disabled[FingerprintAPIKey("key-old")].(map[string]any)
	require.Contains(t, record, "disabled_until", "backfill should add disabled_until")
	require.Contains(t, record, "disabled_count", "backfill should add disabled_count")
}
