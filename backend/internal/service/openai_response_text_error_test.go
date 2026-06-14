//go:build unit

package service

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
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

func TestOpenAIResponseTextErrorRulesFromSettingService(t *testing.T) {
	settingService := NewSettingService(&settingRepoStub{
		values: map[string]string{
			SettingKeyOpenAIResponseTextErrorRules: `[
				{
					"id":"global-community-ad",
					"match":{"textIncludes":["join the new community"]},
					"action":"observe"
				}
			]`,
		},
	}, &config.Config{})

	rules := settingService.GetOpenAIResponseTextErrorRules(context.Background())

	require.Len(t, rules, 1)
	require.Equal(t, "global-community-ad", rules[0].ID)
	require.Equal(t, []string{"join the new community"}, rules[0].Match.TextIncludes)
	require.Equal(t, openAIResponseTextRuleActionObserve, rules[0].Action)
}

func TestOpenAIResponseTextErrorDetectorThreeLevelRules(t *testing.T) {
	t.Run("账号规则优先于管理端全局规则", func(t *testing.T) {
		globalRules := []openAIResponseTextRule{
			{
				ID: "management-community-ad",
				Match: openAIResponseTextRuleMatch{
					TextIncludes: []string{"join the new community"},
				},
				Action: openAIResponseTextRuleActionObserve,
			},
		}
		detector := newOpenAIResponseTextErrorDetector(&Account{
			Platform: PlatformOpenAI,
			Credentials: map[string]any{
				"openai_response_text_error_enabled": true,
				"openai_response_text_error_rules": []any{
					map[string]any{
						"id": "account-community-ad",
						"match": map[string]any{
							"textIncludes": []any{"join the new community"},
						},
						"action": "avoid_ttl",
					},
				},
			},
		}, globalRules)

		match, matched := detector.ObserveTextMatch("please join the new community")

		require.True(t, matched)
		require.Equal(t, "account-community-ad", match.RuleID)
		require.Equal(t, openAIResponseTextRuleActionAvoidTTL, match.Action)
	})

	t.Run("管理端全局规则对未配置账号规则的 OpenAI 账号生效", func(t *testing.T) {
		detector := newOpenAIResponseTextErrorDetector(&Account{
			Platform:    PlatformOpenAI,
			Credentials: map[string]any{},
		}, []openAIResponseTextRule{
			{
				ID: "management-community-ad",
				Match: openAIResponseTextRuleMatch{
					TextIncludes: []string{"join the new community"},
				},
				Action: openAIResponseTextRuleActionObserve,
			},
		})

		match, matched := detector.ObserveTextMatch("please join the new community")

		require.True(t, matched)
		require.Equal(t, "management-community-ad", match.RuleID)
		require.Equal(t, openAIResponseTextRuleActionObserve, match.Action)
	})

	t.Run("系统默认规则只处理安全的流内错误码", func(t *testing.T) {
		detector := newOpenAIResponseTextErrorDetector(&Account{
			Platform:    PlatformOpenAI,
			Credentials: map[string]any{},
		})

		match, matched := detector.ObserveSSEPayloadMatch([]byte(`{"type":"response.failed","error":{"code":"cyber_policy","message":"blocked"}}`))

		require.True(t, matched)
		require.Equal(t, openAIResponseTextDefaultRuleID, match.RuleID)
		require.Equal(t, "cyber_policy", match.Keyword)
		require.Equal(t, openAIResponseTextRuleActionRetry, match.Action)
	})
}

func TestOpenAIResponseTextErrorRulesSafetyLimits(t *testing.T) {
	t.Run("规则数量超过上限时不解析后续规则", func(t *testing.T) {
		rules := make([]any, 0, openAIResponseTextRuleMaxCount+1)
		for index := 0; index < openAIResponseTextRuleMaxCount; index++ {
			rules = append(rules, map[string]any{
				"id": fmt.Sprintf("rule-%d", index),
				"match": map[string]any{
					"textIncludes": []any{fmt.Sprintf("never-hit-%d", index)},
				},
				"action": "avoid_ttl",
			})
		}
		rules = append(rules, map[string]any{
			"id": "beyond-limit",
			"match": map[string]any{
				"textIncludes": []any{"beyond-limit-hit"},
			},
			"action": "avoid_ttl",
		})
		detector := newOpenAIResponseTextErrorDetector(&Account{
			Platform: PlatformOpenAI,
			Credentials: map[string]any{
				"openai_response_text_error_enabled": true,
				"openai_response_text_error_rules":   rules,
			},
		})

		_, matched := detector.ObserveTextMatch("beyond-limit-hit")

		require.Len(t, detector.rules, openAIResponseTextRuleMaxCount)
		require.False(t, matched)
	})

	t.Run("文本匹配项超过长度上限时直接丢弃", func(t *testing.T) {
		tooLongKeyword := strings.Repeat("长", openAIResponseTextRuleKeywordMaxRunes+1)
		detector := newOpenAIResponseTextErrorDetector(&Account{
			Platform: PlatformOpenAI,
			Credentials: map[string]any{
				"openai_response_text_error_enabled": true,
				"openai_response_text_error_rules": []any{
					map[string]any{
						"id": "too-long-keyword",
						"match": map[string]any{
							"textIncludes": []any{tooLongKeyword},
						},
						"action": "avoid_ttl",
					},
				},
			},
		})

		_, matched := detector.ObserveTextMatch(tooLongKeyword)

		require.False(t, matched)
	})
}

func TestOpenAIResponseTextErrorDetectorSkipsLargePayload(t *testing.T) {
	detector := newOpenAIResponseTextErrorDetector(&Account{
		Platform: PlatformOpenAI,
		Credentials: map[string]any{
			"openai_response_text_error_enabled": true,
			"openai_response_text_error_rules": []any{
				map[string]any{
					"id": "community-ad",
					"match": map[string]any{
						"textIncludes": []any{"join the new community"},
					},
					"action": "avoid_ttl",
				},
			},
		},
	})
	payload := []byte(fmt.Sprintf(`{"type":"response.output_text.delta","delta":"%s join the new community"}`, strings.Repeat("x", openAIResponseTextPayloadMaxBytes)))

	_, matched := detector.ObserveSSEPayloadMatch(payload)

	require.False(t, matched)
}

func TestOpenAIResponseTextErrorDetectorSkipsImagePayloads(t *testing.T) {
	detector := newOpenAIResponseTextErrorDetector(&Account{
		Platform: PlatformOpenAI,
		Credentials: map[string]any{
			"openai_response_text_error_enabled": true,
			"openai_response_text_error_rules": []any{
				map[string]any{
					"id": "community-ad",
					"match": map[string]any{
						"textIncludes": []any{"join the new community"},
					},
					"action": "avoid_ttl",
				},
			},
		},
	})

	for _, payload := range [][]byte{
		[]byte(`{"type":"response.output_item.done","item":{"type":"image_generation_call","result":"join the new community"}}`),
		[]byte(`{"type":"response.output_image.delta","b64_json":"join the new community"}`),
		[]byte(`{"type":"response.output_item.done","item":{"content":[{"type":"input_image","image_url":{"url":"data:image/png;base64,join the new community"}}]}}`),
	} {
		_, matched := detector.ObserveSSEPayloadMatch(payload)
		require.False(t, matched)
	}

	match, matched := detector.ObserveSSEPayloadMatch([]byte(`{"type":"response.output_text.delta","delta":"please join the new community"}`))

	require.True(t, matched)
	require.Equal(t, "community-ad", match.RuleID)
}
