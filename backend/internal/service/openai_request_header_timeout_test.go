package service

import (
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestOpenAIRequestHeaderTimeoutForBodyUsesContextSizeBuckets(t *testing.T) {
	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Gateway: config.GatewayConfig{
				OpenAIRequestHeaderTimeoutSeconds: 60,
			},
		},
	}

	require.Equal(t, 10*time.Second, svc.openAIRequestHeaderTimeoutForBody([]byte(`{"input":"hello"}`)))
	require.Equal(t, 15*time.Second, svc.openAIRequestHeaderTimeoutForBody([]byte(`{"input":"`+strings.Repeat("中", 40000)+`"}`)))
	require.Equal(t, 20*time.Second, svc.openAIRequestHeaderTimeoutForBody([]byte(`{"input":"`+strings.Repeat("中", 160000)+`"}`)))
}

func TestOpenAIRequestHeaderTimeoutForBodyRespectsConfiguredCap(t *testing.T) {
	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Gateway: config.GatewayConfig{
				OpenAIRequestHeaderTimeoutSeconds: 12,
			},
		},
	}

	require.Equal(t, 10*time.Second, svc.openAIRequestHeaderTimeoutForBody([]byte(`{"input":"hello"}`)))
	require.Equal(t, 12*time.Second, svc.openAIRequestHeaderTimeoutForBody([]byte(`{"input":"`+strings.Repeat("x", 160000)+`"}`)))
}

func TestOpenAIRequestHeaderTimeoutForBodyUsesWaitGuardCap(t *testing.T) {
	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Gateway: config.GatewayConfig{
				OpenAIRequestHeaderTimeoutSeconds: 60,
			},
		},
	}
	policy := openAICodexStabilityPolicy{
		Enabled:                     true,
		DynamicHeaderTimeoutEnabled: true,
		WaitGuardEnabled:            true,
		MaxHeaderWaitSeconds:        8,
	}

	require.Equal(t, 8*time.Second, svc.openAIRequestHeaderTimeoutForBodyWithPolicy([]byte(`{"input":"hello"}`), policy))
}

func TestOpenAIRequestHeaderTimeoutForBodyCanBeDisabled(t *testing.T) {
	svc := &OpenAIGatewayService{}

	require.Equal(t, time.Duration(0), svc.openAIRequestHeaderTimeoutForBody([]byte(`{"input":"hello"}`)))
}

func TestOpenAICodexStabilityPolicyResolvesByMode(t *testing.T) {
	tests := []struct {
		name     string
		cfg      config.GatewayCodexStabilityConfig
		isCodex  bool
		wantOn   bool
		wantDyn  bool
		wantFail bool
	}{
		{
			name: "off disables policy",
			cfg: config.GatewayCodexStabilityConfig{
				Mode:                        config.GatewayCodexStabilityModeOff,
				DynamicHeaderTimeoutEnabled: true,
				RequestPhaseFailoverEnabled: true,
			},
			isCodex: false,
			wantOn:  false,
		},
		{
			name: "codex mode ignores non codex",
			cfg: config.GatewayCodexStabilityConfig{
				Mode:                        config.GatewayCodexStabilityModeCodex,
				DynamicHeaderTimeoutEnabled: true,
				RequestPhaseFailoverEnabled: true,
			},
			isCodex: false,
			wantOn:  false,
		},
		{
			name: "codex mode matches codex",
			cfg: config.GatewayCodexStabilityConfig{
				Mode:                        config.GatewayCodexStabilityModeCodex,
				DynamicHeaderTimeoutEnabled: true,
				RequestPhaseFailoverEnabled: true,
			},
			isCodex:  true,
			wantOn:   true,
			wantDyn:  true,
			wantFail: true,
		},
		{
			name: "all responses matches non codex",
			cfg: config.GatewayCodexStabilityConfig{
				Mode:                        config.GatewayCodexStabilityModeAllOpenAIResponses,
				DynamicHeaderTimeoutEnabled: true,
				RequestPhaseFailoverEnabled: true,
			},
			isCodex:  false,
			wantOn:   true,
			wantDyn:  true,
			wantFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{CodexStability: tt.cfg}}}

			policy := svc.openAICodexStabilityPolicy(tt.isCodex)

			require.Equal(t, tt.wantOn, policy.Enabled)
			require.Equal(t, tt.wantDyn, policy.DynamicHeaderTimeoutEnabled)
			require.Equal(t, tt.wantFail, policy.RequestPhaseFailoverEnabled)
		})
	}
}

func TestOpenAICodexStabilityPolicyIncludesWaitGuard(t *testing.T) {
	svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{
		CodexStability: config.GatewayCodexStabilityConfig{
			Mode:                        config.GatewayCodexStabilityModeCodex,
			DynamicHeaderTimeoutEnabled: true,
			RequestPhaseFailoverEnabled: true,
			StreamKeepaliveEnabled:      true,
		},
		CodexWaitGuard: config.GatewayCodexWaitGuardConfig{
			Enabled:                  true,
			MaxHeaderWaitSeconds:     9,
			MaxStreamSilentSeconds:   21,
			KeepaliveIntervalSeconds: 6,
			ProtectAfterOutput:       true,
		},
	}}}

	policy := svc.openAICodexStabilityPolicy(true)

	require.True(t, policy.WaitGuardEnabled)
	require.Equal(t, 9, policy.MaxHeaderWaitSeconds)
	require.Equal(t, 21, policy.MaxStreamSilentSeconds)
	require.Equal(t, 6, policy.KeepaliveIntervalSeconds)
	require.True(t, policy.ProtectAfterOutput)
}

func TestOpenAIRequestHeaderTimeoutRequiresStablePolicy(t *testing.T) {
	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Gateway: config.GatewayConfig{
				OpenAIRequestHeaderTimeoutSeconds: 60,
				CodexStability: config.GatewayCodexStabilityConfig{
					Mode:                        config.GatewayCodexStabilityModeOff,
					DynamicHeaderTimeoutEnabled: true,
				},
			},
		},
	}

	require.Equal(t, time.Duration(0), svc.openAIRequestHeaderTimeoutForBodyWithPolicy([]byte(`{"input":"hello"}`), svc.openAICodexStabilityPolicy(true)))

	svc.cfg.Gateway.CodexStability.Mode = config.GatewayCodexStabilityModeCodex
	require.Equal(t, 10*time.Second, svc.openAIRequestHeaderTimeoutForBodyWithPolicy([]byte(`{"input":"hello"}`), svc.openAICodexStabilityPolicy(true)))
}

func TestOpenAIPassthroughTimeoutHeadersRespectsStableSuppression(t *testing.T) {
	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Gateway: config.GatewayConfig{
				OpenAIPassthroughAllowTimeoutHeaders: true,
				CodexStability: config.GatewayCodexStabilityConfig{
					Mode:                         config.GatewayCodexStabilityModeCodex,
					SuppressClientTimeoutHeaders: true,
				},
			},
		},
	}

	policy := svc.openAICodexStabilityPolicy(true)
	require.False(t, svc.isOpenAIPassthroughTimeoutHeadersAllowedForPolicy(policy))

	svc.cfg.Gateway.CodexStability.Mode = config.GatewayCodexStabilityModeOff
	policy = svc.openAICodexStabilityPolicy(true)
	require.True(t, svc.isOpenAIPassthroughTimeoutHeadersAllowedForPolicy(policy))
}
