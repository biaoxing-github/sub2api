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
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "legacy-key",
		},
	}

	require.Equal(t, []string{"legacy-key"}, account.GetAPIKeys())
	require.True(t, account.DisableAPIKey("legacy-key", "invalid_api_key", testNow()))
	require.Empty(t, account.GetAPIKeys())
	require.Empty(t, account.GetAPIKey())
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
