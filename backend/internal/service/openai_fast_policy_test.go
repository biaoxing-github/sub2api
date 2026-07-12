package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type openAIFastPolicyRepoStub struct {
	values map[string]string
}

func TestEvaluateOpenAIFastPolicy_UserRulePrecedesGlobalRule(t *testing.T) {
	settings := &OpenAIFastPolicySettings{Rules: []OpenAIFastPolicyRule{
		{ServiceTier: OpenAIFastTierPriority, Action: BetaPolicyActionBlock, Scope: BetaPolicyScopeAll},
		{ServiceTier: OpenAIFastTierPriority, Action: BetaPolicyActionPass, Scope: BetaPolicyScopeAll, UserIDs: []int64{42}},
	}}

	action, _ := evaluateOpenAIFastPolicyWithSettings(settings, 42, &Account{Type: AccountTypeOAuth}, "gpt-5.6-terra", OpenAIFastTierPriority)
	require.Equal(t, BetaPolicyActionPass, action)
	action, _ = evaluateOpenAIFastPolicyWithSettings(settings, 7, &Account{Type: AccountTypeOAuth}, "gpt-5.6-terra", OpenAIFastTierPriority)
	require.Equal(t, BetaPolicyActionBlock, action)
}

func (s *openAIFastPolicyRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *openAIFastPolicyRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	if v, ok := s.values[key]; ok {
		return v, nil
	}
	return "", ErrSettingNotFound
}

func (s *openAIFastPolicyRepoStub) Set(ctx context.Context, key, value string) error {
	if s.values == nil {
		s.values = map[string]string{}
	}
	s.values[key] = value
	return nil
}

func (s *openAIFastPolicyRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}

func (s *openAIFastPolicyRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *openAIFastPolicyRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *openAIFastPolicyRepoStub) Delete(ctx context.Context, key string) error {
	panic("unexpected Delete call")
}

func newOpenAIGatewayServiceWithSettings(t *testing.T, settings *OpenAIFastPolicySettings) *OpenAIGatewayService {
	t.Helper()
	repo := &openAIFastPolicyRepoStub{values: map[string]string{}}
	if settings != nil {
		raw, err := json.Marshal(settings)
		require.NoError(t, err)
		repo.values[SettingKeyOpenAIFastPolicySettings] = string(raw)
	}
	return &OpenAIGatewayService{
		settingService: NewSettingService(repo, &config.Config{}),
	}
}

func openAIFastFilterPriorityPolicy() *OpenAIFastPolicySettings {
	return &OpenAIFastPolicySettings{
		Rules: []OpenAIFastPolicyRule{{
			ServiceTier:    OpenAIFastTierPriority,
			Action:         BetaPolicyActionFilter,
			Scope:          BetaPolicyScopeAll,
			ModelWhitelist: []string{},
			FallbackAction: BetaPolicyActionPass,
		}},
	}
}

func newOpenAIGatewayServiceWithPromptCacheSettings(t *testing.T, settings *OpenAIPromptCacheSettings) *OpenAIGatewayService {
	t.Helper()
	repo := &openAIFastPolicyRepoStub{values: map[string]string{}}
	if settings != nil {
		raw, err := json.Marshal(settings)
		require.NoError(t, err)
		repo.values[SettingKeyOpenAIPromptCacheSettings] = string(raw)
	}
	return &OpenAIGatewayService{
		settingService: NewSettingService(repo, &config.Config{}),
	}
}

func (s *OpenAIGatewayService) applyOpenAIPromptCacheSettingsToBody(ctx context.Context, model string, body []byte) ([]byte, error) {
	if s == nil || s.settingService == nil || len(body) == 0 || strings.TrimSpace(gjson.GetBytes(body, "prompt_cache_key").String()) != "" {
		return body, nil
	}
	settings, err := s.settingService.GetOpenAIPromptCacheSettings(ctx)
	if err != nil || settings == nil {
		return body, err
	}
	model = strings.TrimSpace(model)
	for _, rule := range settings.Rules {
		key := strings.TrimSpace(rule.PromptCacheKey)
		if key == "" || !openAIPromptCacheRuleMatchesModelForTest(rule, model) {
			continue
		}
		updated, setErr := sjson.SetBytes(body, "prompt_cache_key", key)
		if setErr != nil {
			return body, setErr
		}
		if retention := strings.TrimSpace(rule.PromptCacheRetention); retention != "" &&
			!gjson.GetBytes(updated, "prompt_cache_retention").Exists() {
			updated, setErr = sjson.SetBytes(updated, "prompt_cache_retention", retention)
			if setErr != nil {
				return body, setErr
			}
		}
		return updated, nil
	}
	return body, nil
}

func openAIPromptCacheRuleMatchesModelForTest(rule OpenAIPromptCacheRule, model string) bool {
	if len(rule.ModelWhitelist) == 0 {
		return true
	}
	for _, pattern := range rule.ModelWhitelist {
		if matchWildcard(strings.TrimSpace(pattern), model) {
			return true
		}
	}
	return false
}

func TestApplyOpenAIPromptCacheSettingsToBody_InjectsConfiguredKey(t *testing.T) {
	svc := newOpenAIGatewayServiceWithPromptCacheSettings(t, &OpenAIPromptCacheSettings{
		Rules: []OpenAIPromptCacheRule{{
			PromptCacheKey: "hermes-codex-gpt55-main",
			ModelWhitelist: []string{"gpt-5.5", "gpt-5.5*"},
		}},
	})

	updated, err := svc.applyOpenAIPromptCacheSettingsToBody(context.Background(), "gpt-5.5", []byte(`{"model":"gpt-5.5","input":"hi"}`))
	require.NoError(t, err)
	require.Equal(t, "hermes-codex-gpt55-main", gjson.GetBytes(updated, "prompt_cache_key").String())
}

func TestApplyOpenAIPromptCacheSettingsToBody_DoesNotOverrideClientKey(t *testing.T) {
	svc := newOpenAIGatewayServiceWithPromptCacheSettings(t, &OpenAIPromptCacheSettings{
		Rules: []OpenAIPromptCacheRule{{
			PromptCacheKey: "hermes-codex-gpt55-main",
			ModelWhitelist: []string{"gpt-5.5"},
		}},
	})

	updated, err := svc.applyOpenAIPromptCacheSettingsToBody(context.Background(), "gpt-5.5", []byte(`{"model":"gpt-5.5","prompt_cache_key":"client-key"}`))
	require.NoError(t, err)
	require.Equal(t, "client-key", gjson.GetBytes(updated, "prompt_cache_key").String())
}

func TestApplyOpenAIPromptCacheSettingsToBody_SkipsUnmatchedModel(t *testing.T) {
	svc := newOpenAIGatewayServiceWithPromptCacheSettings(t, &OpenAIPromptCacheSettings{
		Rules: []OpenAIPromptCacheRule{{
			PromptCacheKey: "hermes-codex-gpt55-main",
			ModelWhitelist: []string{"gpt-5.5"},
		}},
	})

	updated, err := svc.applyOpenAIPromptCacheSettingsToBody(context.Background(), "gpt-4o", []byte(`{"model":"gpt-4o"}`))
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(updated, "prompt_cache_key").Exists())
}

func TestSetOpenAIPromptCacheSettings_NormalizesAndStores(t *testing.T) {
	repo := &openAIFastPolicyRepoStub{values: map[string]string{}}
	settingService := NewSettingService(repo, &config.Config{})

	err := settingService.SetOpenAIPromptCacheSettings(context.Background(), &OpenAIPromptCacheSettings{
		Rules: []OpenAIPromptCacheRule{{
			PromptCacheKey:       "  hermes-codex-gpt55-main  ",
			PromptCacheRetention: "24h",
			ModelWhitelist:       []string{" gpt-5.5 ", "gpt-5.5*"},
		}},
	})
	require.NoError(t, err)

	loaded, err := settingService.GetOpenAIPromptCacheSettings(context.Background())
	require.NoError(t, err)
	require.Len(t, loaded.Rules, 1)
	require.Equal(t, "hermes-codex-gpt55-main", loaded.Rules[0].PromptCacheKey)
	require.Equal(t, []string{"gpt-5.5", "gpt-5.5*"}, loaded.Rules[0].ModelWhitelist)
}

func TestEvaluateOpenAIFastPolicy_DefaultPassesKnownTiers(t *testing.T) {
	require.Empty(t, DefaultOpenAIFastPolicySettings().Rules, "default policy must not rewrite service_tier unless admin configured rules")

	svc := newOpenAIGatewayServiceWithSettings(t, DefaultOpenAIFastPolicySettings())
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}

	action, _ := svc.evaluateOpenAIFastPolicy(context.Background(), account, "gpt-5.5", OpenAIFastTierPriority)
	require.Equal(t, BetaPolicyActionPass, action)

	action, _ = svc.evaluateOpenAIFastPolicy(context.Background(), account, "gpt-5.5-turbo", OpenAIFastTierPriority)
	require.Equal(t, BetaPolicyActionPass, action)

	action, _ = svc.evaluateOpenAIFastPolicy(context.Background(), account, "gpt-4", OpenAIFastTierPriority)
	require.Equal(t, BetaPolicyActionPass, action)

	action, _ = svc.evaluateOpenAIFastPolicy(context.Background(), account, "gpt-5.5", OpenAIFastTierFlex)
	require.Equal(t, BetaPolicyActionPass, action)

	// empty tier → pass
	action, _ = svc.evaluateOpenAIFastPolicy(context.Background(), account, "gpt-5.5", "")
	require.Equal(t, BetaPolicyActionPass, action)
}

func TestEvaluateOpenAIFastPolicy_BlockRuleCarriesMessage(t *testing.T) {
	settings := &OpenAIFastPolicySettings{
		Rules: []OpenAIFastPolicyRule{{
			ServiceTier:    OpenAIFastTierPriority,
			Action:         BetaPolicyActionBlock,
			Scope:          BetaPolicyScopeAll,
			ErrorMessage:   "fast mode is not allowed",
			ModelWhitelist: []string{"gpt-5.5"},
			FallbackAction: BetaPolicyActionPass,
		}},
	}
	svc := newOpenAIGatewayServiceWithSettings(t, settings)
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}

	action, msg := svc.evaluateOpenAIFastPolicy(context.Background(), account, "gpt-5.5", OpenAIFastTierPriority)
	require.Equal(t, BetaPolicyActionBlock, action)
	require.Equal(t, "fast mode is not allowed", msg)
}

func TestEvaluateOpenAIFastPolicy_ScopeFiltersOAuth(t *testing.T) {
	settings := &OpenAIFastPolicySettings{
		Rules: []OpenAIFastPolicyRule{{
			ServiceTier: OpenAIFastTierAny,
			Action:      BetaPolicyActionFilter,
			Scope:       BetaPolicyScopeOAuth,
		}},
	}
	svc := newOpenAIGatewayServiceWithSettings(t, settings)

	// OAuth account → rule matches
	oauthAccount := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	action, _ := svc.evaluateOpenAIFastPolicy(context.Background(), oauthAccount, "gpt-4", OpenAIFastTierPriority)
	require.Equal(t, BetaPolicyActionFilter, action)

	// API Key account → rule skipped → pass
	apiKeyAccount := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	action, _ = svc.evaluateOpenAIFastPolicy(context.Background(), apiKeyAccount, "gpt-4", OpenAIFastTierPriority)
	require.Equal(t, BetaPolicyActionPass, action)
}

func TestApplyOpenAIFastPolicyToBody_DefaultPassesPriorityAndFast(t *testing.T) {
	svc := newOpenAIGatewayServiceWithSettings(t, DefaultOpenAIFastPolicySettings())
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}

	body := []byte(`{"model":"gpt-5.5","service_tier":"priority","messages":[]}`)
	updated, err := svc.applyOpenAIFastPolicyToBody(context.Background(), account, "gpt-5.5", body)
	require.NoError(t, err)
	require.Equal(t, string(body), string(updated))

	body = []byte(`{"model":"gpt-5.5","service_tier":"fast"}`)
	updated, err = svc.applyOpenAIFastPolicyToBody(context.Background(), account, "gpt-5.5", body)
	require.NoError(t, err)
	require.Equal(t, "priority", gjson.GetBytes(updated, "service_tier").String())

	body = []byte(`{"model":"gpt-4","service_tier":"priority"}`)
	updated, err = svc.applyOpenAIFastPolicyToBody(context.Background(), account, "gpt-4", body)
	require.NoError(t, err)
	require.Equal(t, string(body), string(updated))

	// No service_tier → no-op
	body = []byte(`{"model":"gpt-5.5"}`)
	updated, err = svc.applyOpenAIFastPolicyToBody(context.Background(), account, "gpt-5.5", body)
	require.NoError(t, err)
	require.Equal(t, string(body), string(updated))
}

func TestApplyOpenAIFastPolicyToBody_ExplicitFilterRemovesField(t *testing.T) {
	svc := newOpenAIGatewayServiceWithSettings(t, openAIFastFilterPriorityPolicy())
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}

	body := []byte(`{"model":"gpt-5.5","service_tier":"priority","messages":[]}`)
	updated, err := svc.applyOpenAIFastPolicyToBody(context.Background(), account, "gpt-5.5", body)
	require.NoError(t, err)
	require.NotContains(t, string(updated), `"service_tier"`)

	body = []byte(`{"model":"gpt-5.5","service_tier":"fast"}`)
	updated, err = svc.applyOpenAIFastPolicyToBody(context.Background(), account, "gpt-5.5", body)
	require.NoError(t, err)
	require.NotContains(t, string(updated), `"service_tier"`)
}

// TestApplyOpenAIFastPolicyToBody_OfficialTiersBypassDefaultRule 验证默认配置
// 下客户端显式发送的 OpenAI 官方合法 tier 能透传到上游而不被静默剥离。
func TestApplyOpenAIFastPolicyToBody_OfficialTiersBypassDefaultRule(t *testing.T) {
	svc := newOpenAIGatewayServiceWithSettings(t, DefaultOpenAIFastPolicySettings())
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}

	for _, tier := range []string{"auto", "default", "scale"} {
		body := []byte(`{"model":"gpt-5.5","service_tier":"` + tier + `"}`)
		updated, err := svc.applyOpenAIFastPolicyToBody(context.Background(), account, "gpt-5.5", body)
		require.NoError(t, err, "tier %q should pass without error", tier)
		require.Contains(t, string(updated), `"service_tier":"`+tier+`"`,
			"tier %q should be preserved in body under default policy", tier)
	}

	// evaluate 层也应判定为 pass（默认配置没有内置规则）。
	for _, tier := range []string{"auto", "default", "scale"} {
		action, _ := svc.evaluateOpenAIFastPolicy(context.Background(), account, "gpt-5.5", tier)
		require.Equal(t, BetaPolicyActionPass, action, "tier %q should evaluate to pass", tier)
	}
}

// TestApplyOpenAIFastPolicyToBody_AllRuleStripsOfficialTiers 验证管理员显式配置
// ServiceTier=all + Action=filter 规则后，auto/default/scale 等官方 tier 也会
// 被剥离。这是符合预期的——首条匹配 short-circuit，"all" 覆盖任意已识别 tier。
func TestApplyOpenAIFastPolicyToBody_AllRuleStripsOfficialTiers(t *testing.T) {
	settings := &OpenAIFastPolicySettings{
		Rules: []OpenAIFastPolicyRule{{
			ServiceTier: OpenAIFastTierAny,
			Action:      BetaPolicyActionFilter,
			Scope:       BetaPolicyScopeAll,
		}},
	}
	svc := newOpenAIGatewayServiceWithSettings(t, settings)
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}

	for _, tier := range []string{"auto", "default", "scale", "priority", "flex"} {
		body := []byte(`{"model":"gpt-5.5","service_tier":"` + tier + `"}`)
		updated, err := svc.applyOpenAIFastPolicyToBody(context.Background(), account, "gpt-5.5", body)
		require.NoError(t, err)
		require.NotContains(t, string(updated), `"service_tier"`,
			"tier %q should be stripped under ServiceTier=all + filter rule", tier)
	}
}

// TestApplyOpenAIFastPolicyToBody_UnknownTierStripped 验证真未知 tier 仍被剥离
// （normalize 返回 nil → normalizeResponsesBodyServiceTier 删除字段；
// applyOpenAIFastPolicyToBody 在 normTier 为空时直接 no-op，因为字段已不可能存在
// 于经过前置归一化的请求里。这里直接调 apply 验证它对未识别值不会异常）。
func TestApplyOpenAIFastPolicyToBody_UnknownTierStripped(t *testing.T) {
	svc := newOpenAIGatewayServiceWithSettings(t, DefaultOpenAIFastPolicySettings())
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}

	// normalize 阶段会将未知值剥离
	require.Nil(t, normalizeOpenAIServiceTier("xxx"))

	// applyOpenAIFastPolicyToBody 收到未识别 tier 时不报错，body 透传不变
	// （不属于本函数职责——上层 normalizeResponsesBodyServiceTier 已剥离）
	body := []byte(`{"model":"gpt-5.5","service_tier":"xxx"}`)
	updated, err := svc.applyOpenAIFastPolicyToBody(context.Background(), account, "gpt-5.5", body)
	require.NoError(t, err)
	require.Equal(t, string(body), string(updated))
}

func TestApplyOpenAIFastPolicyToBody_BlockReturnsTypedError(t *testing.T) {
	settings := &OpenAIFastPolicySettings{
		Rules: []OpenAIFastPolicyRule{{
			ServiceTier:    OpenAIFastTierPriority,
			Action:         BetaPolicyActionBlock,
			Scope:          BetaPolicyScopeAll,
			ErrorMessage:   "fast mode is blocked for gpt-5.5",
			ModelWhitelist: []string{"gpt-5.5"},
			FallbackAction: BetaPolicyActionPass,
		}},
	}
	svc := newOpenAIGatewayServiceWithSettings(t, settings)
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}

	body := []byte(`{"model":"gpt-5.5","service_tier":"priority"}`)
	updated, err := svc.applyOpenAIFastPolicyToBody(context.Background(), account, "gpt-5.5", body)
	require.Error(t, err)
	var blocked *OpenAIFastBlockedError
	require.True(t, errors.As(err, &blocked))
	require.Contains(t, blocked.Message, "fast mode is blocked")
	require.Equal(t, string(body), string(updated)) // body not mutated on block
}

func TestSetOpenAIFastPolicySettings_Validation(t *testing.T) {
	repo := &openAIFastPolicyRepoStub{values: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})

	// Invalid action rejected
	err := svc.SetOpenAIFastPolicySettings(context.Background(), &OpenAIFastPolicySettings{
		Rules: []OpenAIFastPolicyRule{{
			ServiceTier: OpenAIFastTierPriority,
			Action:      "bogus",
			Scope:       BetaPolicyScopeAll,
		}},
	})
	require.Error(t, err)

	// Invalid service_tier rejected
	err = svc.SetOpenAIFastPolicySettings(context.Background(), &OpenAIFastPolicySettings{
		Rules: []OpenAIFastPolicyRule{{
			ServiceTier: "turbo",
			Action:      BetaPolicyActionPass,
			Scope:       BetaPolicyScopeAll,
		}},
	})
	require.Error(t, err)

	// Valid settings persisted
	err = svc.SetOpenAIFastPolicySettings(context.Background(), &OpenAIFastPolicySettings{
		Rules: []OpenAIFastPolicyRule{{
			ServiceTier: OpenAIFastTierPriority,
			Action:      BetaPolicyActionFilter,
			Scope:       BetaPolicyScopeAll,
		}},
	})
	require.NoError(t, err)

	got, err := svc.GetOpenAIFastPolicySettings(context.Background())
	require.NoError(t, err)
	require.Len(t, got.Rules, 1)
	require.Equal(t, OpenAIFastTierPriority, got.Rules[0].ServiceTier)
}
