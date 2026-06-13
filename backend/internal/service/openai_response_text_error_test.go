package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIResponseTextErrorDetectorStructuredRules(t *testing.T) {
	t.Run("textExcludes命中时跳过同条规则", func(t *testing.T) {
		detector := newOpenAIResponseTextErrorDetector(&Account{
			Platform: PlatformOpenAI,
			Credentials: map[string]any{
				"openai_response_text_error_enabled": true,
				"openai_response_text_error_rules": []any{
					map[string]any{
						"id": "community-ad",
						"match": map[string]any{
							"textIncludes": []any{"join the new community"},
							"textExcludes": []any{"official community"},
						},
						"action": "avoid_ttl",
					},
				},
			},
		})

		_, matched := detector.ObserveTextMatch("official community says join the new community")

		require.False(t, matched)
	})

	t.Run("textIncludes命中时返回规则动作", func(t *testing.T) {
		detector := newOpenAIResponseTextErrorDetector(&Account{
			Platform: PlatformOpenAI,
			Credentials: map[string]any{
				"openai_response_text_error_enabled": true,
				"openai_response_text_error_rules": []any{
					map[string]any{
						"id": "community-ad",
						"match": map[string]any{
							"textIncludes": []any{"join the new community"},
							"textExcludes": []any{"official community"},
						},
						"action": "avoid_ttl",
					},
				},
			},
		})

		match, matched := detector.ObserveTextMatch("please join the new community now")

		require.True(t, matched)
		require.Equal(t, "community-ad", match.RuleID)
		require.Equal(t, "join the new community", match.Keyword)
		require.Equal(t, "response_text", match.MatchField)
		require.Equal(t, openAIResponseTextRuleActionAvoidTTL, match.Action)
	})

	t.Run("errorCodes命中时返回配置动作", func(t *testing.T) {
		detector := newOpenAIResponseTextErrorDetector(&Account{
			Platform: PlatformOpenAI,
			Credentials: map[string]any{
				"openai_response_text_error_enabled": true,
				"openai_response_text_error_rules": []any{
					map[string]any{
						"id": "cyber-policy",
						"match": map[string]any{
							"errorCodes": []any{"cyber_policy"},
						},
						"action": "retry",
					},
				},
			},
		})

		match, matched := detector.ObserveSSEPayloadMatch([]byte(`{"type":"response.failed","error":{"code":"cyber_policy","message":"blocked"}}`))

		require.True(t, matched)
		require.Equal(t, "cyber-policy", match.RuleID)
		require.Equal(t, "cyber_policy", match.Keyword)
		require.Equal(t, "error.code", match.MatchField)
		require.Equal(t, openAIResponseTextRuleActionRetry, match.Action)
	})
}

func TestOpenAIResponseTextErrorDetectorLegacyKeywordsMapToAvoidTTL(t *testing.T) {
	detector := newOpenAIResponseTextErrorDetector(&Account{
		Platform: PlatformOpenAI,
		Credentials: map[string]any{
			"openai_response_text_error_enabled":  true,
			"openai_response_text_error_keywords": []any{"加入新家园"},
		},
	})

	match, matched := detector.ObserveTextMatch("加入新家园即可继续使用")

	require.True(t, matched)
	require.Equal(t, openAIResponseTextErrorCode, match.RuleID)
	require.Equal(t, "加入新家园", match.Keyword)
	require.Equal(t, "response_text", match.MatchField)
	require.Equal(t, openAIResponseTextRuleActionAvoidTTL, match.Action)
}
