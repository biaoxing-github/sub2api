//go:build unit

package repository

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestBuildSchedulerMetadataAccount_KeepsOpenAIWSFlags(t *testing.T) {
	account := service.Account{
		ID:       42,
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeOAuth,
		Extra: map[string]any{
			"openai_oauth_responses_websockets_v2_enabled": true,
			"openai_oauth_responses_websockets_v2_mode":    service.OpenAIWSIngressModePassthrough,
			"openai_ws_force_http":                         true,
			"openai_responses_mode":                        "force_chat_completions",
			"openai_responses_supported":                   false,
			"mixed_scheduling":                             true,
			"unused_large_field":                           "drop-me",
		},
	}

	got := buildSchedulerMetadataAccount(account)

	require.Equal(t, true, got.Extra["openai_oauth_responses_websockets_v2_enabled"])
	require.Equal(t, service.OpenAIWSIngressModePassthrough, got.Extra["openai_oauth_responses_websockets_v2_mode"])
	require.Equal(t, true, got.Extra["openai_ws_force_http"])
	require.Equal(t, "force_chat_completions", got.Extra["openai_responses_mode"])
	require.Equal(t, false, got.Extra["openai_responses_supported"])
	require.Equal(t, true, got.Extra["mixed_scheduling"])
	require.Nil(t, got.Extra["unused_large_field"])
}

func TestBuildSchedulerMetadataAccount_KeepsSlimGroupMembership(t *testing.T) {
	account := service.Account{
		ID:       42,
		Platform: service.PlatformAnthropic,
		GroupIDs: []int64{7, 9, 7, 0},
		AccountGroups: []service.AccountGroup{
			{
				AccountID: 42,
				GroupID:   7,
				Priority:  2,
				Account:   &service.Account{ID: 42, Name: "drop-from-metadata"},
				Group:     &service.Group{ID: 7, Name: "drop-from-metadata"},
			},
			{
				AccountID: 42,
				GroupID:   11,
				Priority:  3,
				Group:     &service.Group{ID: 11, Name: "drop-from-metadata"},
			},
			{
				AccountID: 42,
				GroupID:   0,
				Priority:  4,
			},
		},
	}

	got := buildSchedulerMetadataAccount(account)

	require.Equal(t, []int64{7, 9, 11}, got.GroupIDs)
	require.Len(t, got.AccountGroups, 2)
	require.Equal(t, int64(42), got.AccountGroups[0].AccountID)
	require.Equal(t, int64(7), got.AccountGroups[0].GroupID)
	require.Equal(t, 2, got.AccountGroups[0].Priority)
	require.Nil(t, got.AccountGroups[0].Account)
	require.Nil(t, got.AccountGroups[0].Group)
	require.Equal(t, int64(11), got.AccountGroups[1].GroupID)
	require.Nil(t, got.Groups)
}
func TestBuildSchedulerMetadataAccount_KeepsOpenAIPassthroughForModelGate(t *testing.T) {
	for _, key := range []string{"openai_passthrough", "openai_oauth_passthrough"} {
		t.Run(key, func(t *testing.T) {
			account := service.Account{
				ID:       383,
				Platform: service.PlatformOpenAI,
				Type:     service.AccountTypeOAuth,
				Credentials: map[string]any{
					// 账号从白名单模式切到透传后常见的残留映射，未列出请求的模型。
					"model_mapping": map[string]any{"gpt-5.5": "gpt-5.5"},
					"access_token":  "drop-me",
				},
				Extra: map[string]any{key: true},
			}
			require.True(t, account.IsModelSupported("gpt-5.6-sol"),
				"前置条件：透传账号本应放行白名单外的模型")

			meta := buildSchedulerMetadataAccount(account)

			// 走一遍真实的序列化/反序列化路径（写入 sched:meta 再由 decodeCachedAccount 读回）。
			payload, err := json.Marshal(meta)
			require.NoError(t, err)
			var restored service.Account
			require.NoError(t, json.Unmarshal(payload, &restored))

			require.Equal(t, true, restored.Extra[key])
			require.True(t, restored.IsOpenAIPassthroughEnabled())
			require.True(t, restored.IsModelSupported("gpt-5.6-sol"),
				"投影裁掉透传开关会让透传账号在候选过滤阶段被误判为 model_not_supported")
			// 白名单本身仍需保留：非透传账号依赖它做模型门。
			require.Equal(t, map[string]any{"gpt-5.5": "gpt-5.5"}, restored.Credentials["model_mapping"])
		})
	}
}
