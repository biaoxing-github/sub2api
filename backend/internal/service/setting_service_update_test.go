//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/antigravity"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type settingUpdateRepoStub struct {
	updates map[string]string
	values  map[string]string
}

func (s *settingUpdateRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *settingUpdateRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	panic("unexpected GetValue call")
}

func (s *settingUpdateRepoStub) Set(ctx context.Context, key, value string) error {
	panic("unexpected Set call")
}

func (s *settingUpdateRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}

func (s *settingUpdateRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	s.updates = make(map[string]string, len(settings))
	for k, v := range settings {
		s.updates[k] = v
	}
	return nil
}

func (s *settingUpdateRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	result := make(map[string]string, len(s.values))
	for k, v := range s.values {
		result[k] = v
	}
	return result, nil
}

func (s *settingUpdateRepoStub) Delete(ctx context.Context, key string) error {
	panic("unexpected Delete call")
}

type settingAntigravityUARepoStub struct {
	values map[string]string
}

func (s *settingAntigravityUARepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *settingAntigravityUARepoStub) GetValue(ctx context.Context, key string) (string, error) {
	if value, ok := s.values[key]; ok {
		return value, nil
	}
	return "", ErrSettingNotFound
}

func (s *settingAntigravityUARepoStub) Set(ctx context.Context, key, value string) error {
	panic("unexpected Set call")
}

func (s *settingAntigravityUARepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}

func (s *settingAntigravityUARepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *settingAntigravityUARepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *settingAntigravityUARepoStub) Delete(ctx context.Context, key string) error {
	panic("unexpected Delete call")
}

type defaultSubGroupReaderStub struct {
	byID  map[int64]*Group
	errBy map[int64]error
	calls []int64
}

func (s *defaultSubGroupReaderStub) GetByID(ctx context.Context, id int64) (*Group, error) {
	s.calls = append(s.calls, id)
	if err, ok := s.errBy[id]; ok {
		return nil, err
	}
	if g, ok := s.byID[id]; ok {
		return g, nil
	}
	return nil, ErrGroupNotFound
}

func TestSettingService_UpdateSettings_DefaultSubscriptions_ValidGroup(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	groupReader := &defaultSubGroupReaderStub{
		byID: map[int64]*Group{
			11: {ID: 11, SubscriptionType: SubscriptionTypeSubscription},
		},
	}
	svc := NewSettingService(repo, &config.Config{})
	svc.SetDefaultSubscriptionGroupReader(groupReader)

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		DefaultSubscriptions: []DefaultSubscriptionSetting{
			{GroupID: 11, ValidityDays: 30},
		},
	})
	require.NoError(t, err)
	require.Equal(t, []int64{11}, groupReader.calls)

	raw, ok := repo.updates[SettingKeyDefaultSubscriptions]
	require.True(t, ok)

	var got []DefaultSubscriptionSetting
	require.NoError(t, json.Unmarshal([]byte(raw), &got))
	require.Equal(t, []DefaultSubscriptionSetting{
		{GroupID: 11, ValidityDays: 30},
	}, got)
}

func TestSettingService_UpdateSettings_DefaultSubscriptions_RejectsNonSubscriptionGroup(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	groupReader := &defaultSubGroupReaderStub{
		byID: map[int64]*Group{
			12: {ID: 12, SubscriptionType: SubscriptionTypeStandard},
		},
	}
	svc := NewSettingService(repo, &config.Config{})
	svc.SetDefaultSubscriptionGroupReader(groupReader)

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		DefaultSubscriptions: []DefaultSubscriptionSetting{
			{GroupID: 12, ValidityDays: 7},
		},
	})
	require.Error(t, err)
	require.Equal(t, "DEFAULT_SUBSCRIPTION_GROUP_INVALID", infraerrors.Reason(err))
	require.Nil(t, repo.updates)
}

func TestSettingService_UpdateSettings_DefaultSubscriptions_RejectsNotFoundGroup(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	groupReader := &defaultSubGroupReaderStub{
		errBy: map[int64]error{
			13: ErrGroupNotFound,
		},
	}
	svc := NewSettingService(repo, &config.Config{})
	svc.SetDefaultSubscriptionGroupReader(groupReader)

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		DefaultSubscriptions: []DefaultSubscriptionSetting{
			{GroupID: 13, ValidityDays: 7},
		},
	})
	require.Error(t, err)
	require.Equal(t, "DEFAULT_SUBSCRIPTION_GROUP_INVALID", infraerrors.Reason(err))
	require.Equal(t, "13", infraerrors.FromError(err).Metadata["group_id"])
	require.Nil(t, repo.updates)
}

func TestSettingService_UpdateSettings_DefaultSubscriptions_RejectsDuplicateGroup(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	groupReader := &defaultSubGroupReaderStub{
		byID: map[int64]*Group{
			11: {ID: 11, SubscriptionType: SubscriptionTypeSubscription},
		},
	}
	svc := NewSettingService(repo, &config.Config{})
	svc.SetDefaultSubscriptionGroupReader(groupReader)

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		DefaultSubscriptions: []DefaultSubscriptionSetting{
			{GroupID: 11, ValidityDays: 30},
			{GroupID: 11, ValidityDays: 60},
		},
	})
	require.Error(t, err)
	require.Equal(t, "DEFAULT_SUBSCRIPTION_GROUP_DUPLICATE", infraerrors.Reason(err))
	require.Equal(t, "11", infraerrors.FromError(err).Metadata["group_id"])
	require.Nil(t, repo.updates)
}

func TestSettingService_UpdateSettings_DefaultSubscriptions_RejectsDuplicateGroupWithoutGroupReader(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		DefaultSubscriptions: []DefaultSubscriptionSetting{
			{GroupID: 11, ValidityDays: 30},
			{GroupID: 11, ValidityDays: 60},
		},
	})
	require.Error(t, err)
	require.Equal(t, "DEFAULT_SUBSCRIPTION_GROUP_DUPLICATE", infraerrors.Reason(err))
	require.Equal(t, "11", infraerrors.FromError(err).Metadata["group_id"])
	require.Nil(t, repo.updates)
}

func TestSettingService_UpdateSettings_RegistrationEmailSuffixWhitelist_Normalized(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		RegistrationEmailSuffixWhitelist: []string{"example.com", "@EXAMPLE.com", " @foo.bar ", "*.EDU.CN"},
	})
	require.NoError(t, err)
	require.Equal(t, `["@example.com","@foo.bar","*.edu.cn"]`, repo.updates[SettingKeyRegistrationEmailSuffixWhitelist])
}

func TestSettingService_UpdateSettings_RegistrationEmailSuffixWhitelist_Invalid(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		RegistrationEmailSuffixWhitelist: []string{"@invalid_domain"},
	})
	require.Error(t, err)
	require.Equal(t, "INVALID_REGISTRATION_EMAIL_SUFFIX_WHITELIST", infraerrors.Reason(err))
}

func TestParseDefaultSubscriptions_NormalizesValues(t *testing.T) {
	got := parseDefaultSubscriptions(`[{"group_id":11,"validity_days":30},{"group_id":11,"validity_days":60},{"group_id":0,"validity_days":10},{"group_id":12,"validity_days":99999}]`)
	require.Equal(t, []DefaultSubscriptionSetting{
		{GroupID: 11, ValidityDays: 30},
		{GroupID: 11, ValidityDays: 60},
		{GroupID: 12, ValidityDays: MaxValidityDays},
	}, got)
}

func TestSettingService_UpdateSettings_TablePreferences(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		TableDefaultPageSize: 50,
		TablePageSizeOptions: []int{20, 50, 100},
	})
	require.NoError(t, err)
	require.Equal(t, "50", repo.updates[SettingKeyTableDefaultPageSize])
	require.Equal(t, "[20,50,100]", repo.updates[SettingKeyTablePageSizeOptions])

	err = svc.UpdateSettings(context.Background(), &SystemSettings{
		TableDefaultPageSize: 1000,
		TablePageSizeOptions: []int{20, 100},
	})
	require.NoError(t, err)
	require.Equal(t, "1000", repo.updates[SettingKeyTableDefaultPageSize])
	require.Equal(t, "[20,100]", repo.updates[SettingKeyTablePageSizeOptions])
}

func TestSettingService_UpdateSettings_PaymentVisibleMethodsAndAdvancedScheduler(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		PaymentVisibleMethodAlipaySource:  "alipay",
		PaymentVisibleMethodWxpaySource:   "easypay",
		PaymentVisibleMethodAlipayEnabled: true,
		PaymentVisibleMethodWxpayEnabled:  false,
		OpenAIAdvancedSchedulerEnabled:    true,
	})
	require.NoError(t, err)
	require.Equal(t, VisibleMethodSourceOfficialAlipay, repo.updates[SettingPaymentVisibleMethodAlipaySource])
	require.Equal(t, VisibleMethodSourceEasyPayWechat, repo.updates[SettingPaymentVisibleMethodWxpaySource])
	require.Equal(t, "true", repo.updates[SettingPaymentVisibleMethodAlipayEnabled])
	require.Equal(t, "false", repo.updates[SettingPaymentVisibleMethodWxpayEnabled])
	require.Equal(t, "true", repo.updates[openAIAdvancedSchedulerSettingKey])
}

func TestSettingService_UpdateSettings_CodexStabilityRefreshesGatewayConfig(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	cfg := &config.Config{}
	svc := NewSettingService(repo, cfg)

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		CodexStabilityMode:                         config.GatewayCodexStabilityModeOff,
		CodexStabilityDynamicHeaderTimeoutEnabled:  false,
		CodexStabilityRequestPhaseFailoverEnabled:  false,
		CodexStabilitySuppressClientTimeoutHeaders: true,
		CodexStabilityStreamKeepaliveEnabled:       false,
	})
	require.NoError(t, err)
	require.Equal(t, config.GatewayCodexStabilityModeOff, repo.updates[SettingKeyCodexStabilityMode])
	require.Equal(t, "false", repo.updates[SettingKeyCodexStabilityDynamicHeaderTimeoutEnabled])
	require.Equal(t, "false", repo.updates[SettingKeyCodexStabilityRequestPhaseFailoverEnabled])
	require.Equal(t, "true", repo.updates[SettingKeyCodexStabilitySuppressClientTimeoutHeaders])
	require.Equal(t, "false", repo.updates[SettingKeyCodexStabilityStreamKeepaliveEnabled])
	require.Equal(t, config.GatewayCodexStabilityModeOff, cfg.Gateway.CodexStability.Mode)
	require.False(t, cfg.Gateway.CodexStability.DynamicHeaderTimeoutEnabled)
	require.False(t, cfg.Gateway.CodexStability.RequestPhaseFailoverEnabled)
	require.True(t, cfg.Gateway.CodexStability.SuppressClientTimeoutHeaders)
	require.False(t, cfg.Gateway.CodexStability.StreamKeepaliveEnabled)
}

func TestSettingService_UpdateSettings_OpenAICockpitToolsCompatRefreshesGatewayConfig(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	cfg := &config.Config{}
	svc := NewSettingService(repo, cfg)

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		OpenAICockpitToolsCompat: true,
	})
	require.NoError(t, err)
	require.Equal(t, "true", repo.updates[SettingKeyOpenAICockpitToolsCompat])
	require.True(t, cfg.Gateway.OpenAICockpitToolsCompat)
	require.Equal(t, config.GatewayOpenAIOAuthCompatModeCockpitTools, cfg.Gateway.OpenAIOAuthCompatMode)
}

func TestSettingService_UpdateSettings_OpenAIOAuthCompatModeRefreshesGatewayConfig(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	cfg := &config.Config{}
	svc := NewSettingService(repo, cfg)

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		OpenAIOAuthCompatMode:                    config.GatewayOpenAIOAuthCompatModeCockpitTools,
		OpenAICodexDirectForceWS:                 true,
		OpenAICodexDirectTLSFingerprintProfileID: 42,
	})
	require.NoError(t, err)
	require.Equal(t, config.GatewayOpenAIOAuthCompatModeCockpitTools, repo.updates[SettingKeyOpenAIOAuthCompatMode])
	require.Equal(t, "true", repo.updates[SettingKeyOpenAICockpitToolsCompat])
	require.Equal(t, "true", repo.updates[SettingKeyOpenAICodexDirectForceWS])
	require.Equal(t, "42", repo.updates[SettingKeyOpenAICodexDirectTLSFingerprintProfileID])
	require.Equal(t, config.GatewayOpenAIOAuthCompatModeCockpitTools, cfg.Gateway.OpenAIOAuthCompatMode)
	require.True(t, cfg.Gateway.OpenAICockpitToolsCompat)
	require.True(t, cfg.Gateway.OpenAICodexDirectForceWS)
	require.Equal(t, int64(42), cfg.Gateway.OpenAICodexDirectTLSFingerprintProfileID)
}

func TestSettingService_UpdateSettings_OpenAISchedulerExhaustionProbeRefreshesGatewayConfig(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	cfg := &config.Config{}
	svc := NewSettingService(repo, cfg)

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		OpenAISchedulerProbeInfiniteWaitEnabled:       true,
		OpenAISchedulerProbeNotifyEnabled:             true,
		OpenAISchedulerProbeNotifyChannel:             openAISchedulerExhaustionNotifyChannelFeishuApp,
		OpenAISchedulerProbeNotifyAfterSeconds:        45,
		OpenAISchedulerProbeNotifyRepeatSeconds:       120,
		OpenAISchedulerProbeNotifyFeishuWebhookURL:    "https://example.test/feishu",
		OpenAISchedulerProbeNotifyFeishuAppID:         "cli_test",
		OpenAISchedulerProbeNotifyFeishuAppSecret:     "secret_test",
		OpenAISchedulerProbeNotifyFeishuDomain:        "feishu",
		OpenAISchedulerProbeNotifyFeishuReceiveIDType: "chat_id",
		OpenAISchedulerProbeNotifyFeishuReceiveID:     "oc_test",
		OpenAISchedulerProbeNotifyRecoveredEnabled:    false,
	})
	require.NoError(t, err)
	require.Equal(t, "true", repo.updates[SettingKeyOpenAISchedulerProbeInfiniteWaitEnabled])
	require.Equal(t, "true", repo.updates[SettingKeyOpenAISchedulerProbeNotifyEnabled])
	require.Equal(t, openAISchedulerExhaustionNotifyChannelFeishuApp, repo.updates[SettingKeyOpenAISchedulerProbeNotifyChannel])
	require.Equal(t, "45", repo.updates[SettingKeyOpenAISchedulerProbeNotifyAfterSeconds])
	require.Equal(t, "120", repo.updates[SettingKeyOpenAISchedulerProbeNotifyRepeatSeconds])
	require.Equal(t, "https://example.test/feishu", repo.updates[SettingKeyOpenAISchedulerProbeNotifyFeishuWebhookURL])
	require.Equal(t, "cli_test", repo.updates[SettingKeyOpenAISchedulerProbeNotifyFeishuAppID])
	require.Equal(t, "secret_test", repo.updates[SettingKeyOpenAISchedulerProbeNotifyFeishuAppSecret])
	require.Equal(t, "feishu", repo.updates[SettingKeyOpenAISchedulerProbeNotifyFeishuDomain])
	require.Equal(t, "chat_id", repo.updates[SettingKeyOpenAISchedulerProbeNotifyFeishuReceiveIDType])
	require.Equal(t, "oc_test", repo.updates[SettingKeyOpenAISchedulerProbeNotifyFeishuReceiveID])
	require.Equal(t, "false", repo.updates[SettingKeyOpenAISchedulerProbeNotifyRecoveredEnabled])
	require.True(t, cfg.Gateway.OpenAISchedulerProbeInfiniteWaitEnabled)
	require.True(t, cfg.Gateway.OpenAISchedulerProbeNotifyEnabled)
	require.Equal(t, openAISchedulerExhaustionNotifyChannelFeishuApp, cfg.Gateway.OpenAISchedulerProbeNotifyChannel)
	require.Equal(t, 45, cfg.Gateway.OpenAISchedulerProbeNotifyAfterSeconds)
	require.Equal(t, 120, cfg.Gateway.OpenAISchedulerProbeNotifyRepeatSeconds)
	require.Equal(t, "https://example.test/feishu", cfg.Gateway.OpenAISchedulerProbeNotifyFeishuWebhookURL)
	require.Equal(t, "cli_test", cfg.Gateway.OpenAISchedulerProbeNotifyFeishuAppID)
	require.Equal(t, "secret_test", cfg.Gateway.OpenAISchedulerProbeNotifyFeishuAppSecret)
	require.Equal(t, "feishu", cfg.Gateway.OpenAISchedulerProbeNotifyFeishuDomain)
	require.Equal(t, "chat_id", cfg.Gateway.OpenAISchedulerProbeNotifyFeishuReceiveIDType)
	require.Equal(t, "oc_test", cfg.Gateway.OpenAISchedulerProbeNotifyFeishuReceiveID)
	require.False(t, cfg.Gateway.OpenAISchedulerProbeNotifyRecoveredEnabled)
}

func TestSettingService_UpdateSettings_GatewayRuntimeStabilityRefreshesConfig(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	cfg := &config.Config{}
	svc := NewSettingService(repo, cfg)

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		CodexAutopilotEnabled:                    true,
		CodexAutopilotObserveOnly:                false,
		CodexAutopilotWindowSeconds:              180,
		CodexAutopilotMinSamples:                 9,
		CodexAutopilotHeaderTimeoutThreshold:     3,
		CodexAutopilotEOFThreshold:               4,
		CodexAutopilotSilentStreamTimeoutSeconds: 45,
		OpenAIPathHealthEnabled:                  true,
		OpenAIPathHealthCircuitBreakerEnabled:    true,
		OpenAIPathHealthFailureWindowSeconds:     120,
		OpenAIPathHealthCooldownSeconds:          90,
		OpenAIPathHealthDegradedFailures:         2,
		OpenAIPathHealthOpenFailures:             5,
		OpenAIPathHealthHalfOpenMaxProbes:        1,
		OpenAIFastLaneEnabled:                    true,
		OpenAIFastLaneNewSessionOnly:             true,
		OpenAIFastLaneTTFTWeight:                 0.7,
		OpenAIFastLaneHeaderWaitWeight:           0.3,
		OpenAIFastLaneMinSamples:                 6,
		OpenAIFastLaneExploreRatio:               0.2,
		RealtimeBalancePrewarmEnabled:            true,
		RealtimeBalancePrewarmIntervalSeconds:    300,
		RealtimeBalancePrewarmActiveAccountLimit: 12,
		RealtimeBalanceConfirmTopN:               4,
		RealtimeBalanceConfirmTimeoutMs:          2500,
		CodexWaitGuardEnabled:                    true,
		CodexWaitGuardMaxHeaderWaitSeconds:       35,
		CodexWaitGuardMaxStreamSilentSeconds:     60,
		CodexWaitGuardKeepaliveIntervalSeconds:   8,
		CodexWaitGuardProtectAfterOutput:         true,
		ContextJournalBackend:                    "redis",
		ContextJournalTTLHours:                   48,
		ContextJournalMaxSessionBytes:            1 << 20,
	})
	require.NoError(t, err)

	require.Equal(t, "true", repo.updates[SettingKeyCodexAutopilotEnabled])
	require.Equal(t, "180", repo.updates[SettingKeyCodexAutopilotWindowSeconds])
	require.Equal(t, "true", repo.updates[SettingKeyOpenAIPathHealthEnabled])
	require.Equal(t, "120", repo.updates[SettingKeyOpenAIPathHealthFailureWindowSeconds])
	require.Equal(t, "true", repo.updates[SettingKeyOpenAIFastLaneEnabled])
	require.Equal(t, "0.7", repo.updates[SettingKeyOpenAIFastLaneTTFTWeight])
	require.Equal(t, "true", repo.updates[SettingKeyRealtimeBalancePrewarmEnabled])
	require.Equal(t, "4", repo.updates[SettingKeyRealtimeBalanceConfirmTopN])
	require.Equal(t, "true", repo.updates[SettingKeyCodexWaitGuardEnabled])
	require.Equal(t, "redis", repo.updates[SettingKeyContextJournalBackend])
	require.Equal(t, "1048576", repo.updates[SettingKeyContextJournalMaxSessionBytes])

	require.True(t, cfg.Gateway.CodexAutopilot.Enabled)
	require.Equal(t, 180, cfg.Gateway.CodexAutopilot.WindowSeconds)
	require.True(t, cfg.Gateway.OpenAIPathHealth.Enabled)
	require.True(t, cfg.Gateway.OpenAIPathHealth.CircuitBreakerEnabled)
	require.Equal(t, 120, cfg.Gateway.OpenAIPathHealth.FailureWindowSeconds)
	require.True(t, cfg.Gateway.OpenAIFastLane.Enabled)
	require.Equal(t, 0.7, cfg.Gateway.OpenAIFastLane.TTFTWeight)
	require.Equal(t, 0.2, cfg.Gateway.OpenAIFastLane.ExploreRatio)
	require.True(t, cfg.Gateway.RealtimeBalancePrewarm.Enabled)
	require.Equal(t, 4, cfg.Gateway.RealtimeBalanceConfirmTopN)
	require.Equal(t, 2500, cfg.Gateway.RealtimeBalanceConfirmTimeoutMs)
	require.True(t, cfg.Gateway.CodexWaitGuard.Enabled)
	require.True(t, cfg.Gateway.CodexWaitGuard.ProtectAfterOutput)
	require.Equal(t, "redis", cfg.Gateway.ContextJournal.Backend)
	require.Equal(t, int64(1<<20), cfg.Gateway.ContextJournal.MaxSessionBytes)
}

func TestSettingService_UpdateSettings_AntigravityUserAgentVersion(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		AntigravityUserAgentVersion: "1.23.2",
	})
	require.NoError(t, err)
	require.Equal(t, "1.23.2", repo.updates[SettingKeyAntigravityUserAgentVersion])
}

func TestSettingService_UpdateSettings_APIKeyACLTrustForwardedIPRefreshesConfig(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	cfg := &config.Config{}
	svc := NewSettingService(repo, cfg)

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		APIKeyACLTrustForwardedIP: true,
	})
	require.NoError(t, err)
	require.Equal(t, "true", repo.updates[SettingKeyAPIKeyACLTrustForwardedIP])
	require.True(t, cfg.Security.TrustForwardedIPForAPIKeyACL)
	require.True(t, cfg.TrustForwardedIPForAPIKeyACL())
}

func TestSettingService_ParseSettings_APIKeyACLTrustForwardedIPFallsBackToConfigWhenMissing(t *testing.T) {
	cfg := &config.Config{}
	cfg.Security.TrustForwardedIPForAPIKeyACL = true
	svc := NewSettingService(&settingUpdateRepoStub{}, cfg)

	got := svc.parseSettings(map[string]string{})

	require.True(t, got.APIKeyACLTrustForwardedIP)
}

func TestSettingService_ParseSettings_OpenAICockpitToolsCompatFallsBackToConfigWhenMissing(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAICockpitToolsCompat = true
	svc := NewSettingService(&settingUpdateRepoStub{}, cfg)

	got := svc.parseSettings(map[string]string{})

	require.True(t, got.OpenAICockpitToolsCompat)
	require.Equal(t, config.GatewayOpenAIOAuthCompatModeCockpitTools, got.OpenAIOAuthCompatMode)
}

func TestSettingService_ParseSettings_DeprecatedCodexDirectNormalizesToOff(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAICockpitToolsCompat = true
	cfg.Gateway.OpenAICodexDirectForceWS = true
	cfg.Gateway.OpenAICodexDirectTLSFingerprintProfileID = -1
	svc := NewSettingService(&settingUpdateRepoStub{}, cfg)

	got := svc.parseSettings(map[string]string{
		SettingKeyOpenAIOAuthCompatMode:                    config.GatewayOpenAIOAuthCompatModeCodexDirect,
		SettingKeyOpenAICodexDirectForceWS:                 "false",
		SettingKeyOpenAICodexDirectTLSFingerprintProfileID: "42",
	})

	require.False(t, got.OpenAICockpitToolsCompat)
	require.Equal(t, config.GatewayOpenAIOAuthCompatModeOff, got.OpenAIOAuthCompatMode)
	require.False(t, got.OpenAICodexDirectForceWS)
	require.Equal(t, int64(42), got.OpenAICodexDirectTLSFingerprintProfileID)
}

func TestSettingService_ParseSettings_OpenAICodexDirectTLSFingerprintProfileIDFallsBackToConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAICodexDirectTLSFingerprintProfileID = -1
	svc := NewSettingService(&settingUpdateRepoStub{}, cfg)

	got := svc.parseSettings(map[string]string{})
	require.Equal(t, int64(-1), got.OpenAICodexDirectTLSFingerprintProfileID)

	got = svc.parseSettings(map[string]string{
		SettingKeyOpenAICodexDirectTLSFingerprintProfileID: "-2",
	})
	require.Equal(t, int64(-1), got.OpenAICodexDirectTLSFingerprintProfileID)
}

func TestSettingService_LoadRuntimeSettingsRefreshesGatewayConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.Security.TrustForwardedIPForAPIKeyACL = false
	cfg.Gateway.OpenAICockpitToolsCompat = true
	cfg.Gateway.OpenAIOAuthCompatMode = config.GatewayOpenAIOAuthCompatModeCockpitTools
	cfg.Gateway.OpenAICodexDirectForceWS = false
	cfg.Gateway.OpenAICodexDirectTLSFingerprintProfileID = 0
	cfg.Gateway.CodexStability.Mode = config.GatewayCodexStabilityModeOff
	cfg.Gateway.CodexStability.DynamicHeaderTimeoutEnabled = false
	cfg.Gateway.OpenAIPathHealth.Enabled = false
	cfg.Gateway.OpenAIFastLane.Enabled = false
	cfg.Gateway.RealtimeBalanceConfirmTopN = 1
	cfg.Gateway.RealtimeBalanceConfirmTimeoutMs = 100
	repo := &settingUpdateRepoStub{values: map[string]string{
		SettingKeyAPIKeyACLTrustForwardedIP:                 "true",
		SettingKeyOpenAIOAuthCompatMode:                     config.GatewayOpenAIOAuthCompatModeCodexDirect,
		SettingKeyOpenAICockpitToolsCompat:                  "false",
		SettingKeyOpenAICodexDirectForceWS:                  "true",
		SettingKeyOpenAICodexDirectTLSFingerprintProfileID:  "-1",
		SettingKeyCodexStabilityMode:                        config.GatewayCodexStabilityModeCodex,
		SettingKeyCodexStabilityDynamicHeaderTimeoutEnabled: "true",
		SettingKeyOpenAIPathHealthEnabled:                   "true",
		SettingKeyOpenAIFastLaneEnabled:                     "true",
		SettingKeyRealtimeBalanceConfirmTopN:                "4",
		SettingKeyRealtimeBalanceConfirmTimeoutMs:           "2500",
	}}
	svc := NewSettingService(repo, cfg)

	require.NoError(t, svc.LoadRuntimeSettings(context.Background()))

	require.True(t, cfg.TrustForwardedIPForAPIKeyACL())
	require.Equal(t, config.GatewayOpenAIOAuthCompatModeOff, cfg.Gateway.OpenAIOAuthCompatMode)
	require.False(t, cfg.Gateway.OpenAICockpitToolsCompat)
	require.True(t, cfg.Gateway.OpenAICodexDirectForceWS)
	require.Equal(t, int64(-1), cfg.Gateway.OpenAICodexDirectTLSFingerprintProfileID)
	require.Equal(t, config.GatewayCodexStabilityModeCodex, cfg.Gateway.CodexStability.Mode)
	require.True(t, cfg.Gateway.CodexStability.DynamicHeaderTimeoutEnabled)
	require.True(t, cfg.Gateway.OpenAIPathHealth.Enabled)
	require.True(t, cfg.Gateway.OpenAIFastLane.Enabled)
	require.Equal(t, 4, cfg.Gateway.RealtimeBalanceConfirmTopN)
	require.Equal(t, 2500, cfg.Gateway.RealtimeBalanceConfirmTimeoutMs)
}

func TestSettingService_GetAntigravityUserAgentVersion_Precedence(t *testing.T) {
	t.Run("后台设置优先", func(t *testing.T) {
		svc := NewSettingService(&settingAntigravityUARepoStub{values: map[string]string{
			SettingKeyAntigravityUserAgentVersion: "1.24.0",
		}}, &config.Config{})

		require.Equal(t, "1.24.0", svc.GetAntigravityUserAgentVersion(context.Background()))
	})

	t.Run("空值回退配置默认值", func(t *testing.T) {
		svc := NewSettingService(&settingAntigravityUARepoStub{values: map[string]string{
			SettingKeyAntigravityUserAgentVersion: "",
		}}, &config.Config{})

		require.Equal(t, antigravity.GetDefaultUserAgentVersion(), svc.GetAntigravityUserAgentVersion(context.Background()))
	})

	t.Run("缺失回退配置默认值", func(t *testing.T) {
		svc := NewSettingService(&settingAntigravityUARepoStub{values: map[string]string{}}, &config.Config{})

		require.Equal(t, antigravity.GetDefaultUserAgentVersion(), svc.GetAntigravityUserAgentVersion(context.Background()))
	})
}

func TestSettingService_UpdateSettings_RejectsInvalidPaymentVisibleMethodSource(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		PaymentVisibleMethodAlipaySource: "not-a-provider",
	})
	require.Error(t, err)
	require.Equal(t, "INVALID_PAYMENT_VISIBLE_METHOD_SOURCE", infraerrors.Reason(err))
	require.Nil(t, repo.updates)
}
