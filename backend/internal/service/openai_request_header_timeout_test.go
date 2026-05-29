package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

type headerRaceHTTPUpstreamStub struct {
	mu     sync.Mutex
	calls  []string
	delays map[string]time.Duration
	tlsHit bool
}

func (u *headerRaceHTTPUpstreamStub) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	baseURL := req.URL.Scheme + "://" + req.URL.Host
	u.mu.Lock()
	u.calls = append(u.calls, baseURL)
	delay := u.delays[baseURL]
	u.mu.Unlock()
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}"))}, nil
}

func (u *headerRaceHTTPUpstreamStub) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	u.mu.Lock()
	u.tlsHit = true
	u.mu.Unlock()
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

func (u *headerRaceHTTPUpstreamStub) callCount() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return len(u.calls)
}

func (u *headerRaceHTTPUpstreamStub) usedTLS() bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.tlsHit
}

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

func TestOpenAIUpstreamCodexDirectUsesTLSProfile(t *testing.T) {
	upstream := &headerRaceHTTPUpstreamStub{}
	svc := &OpenAIGatewayService{
		cfg: &config.Config{Gateway: config.GatewayConfig{
			OpenAIOAuthCompatMode: config.GatewayOpenAIOAuthCompatModeCodexDirect,
		}},
		httpUpstream: upstream,
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, chatgptCodexURL, strings.NewReader(`{}`))
	require.NoError(t, err)
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}

	resp, err := svc.doOpenAIUpstreamWithHeaderTimeout(context.Background(), req, "", account, []byte(`{}`), openAICodexStabilityPolicy{}, "")

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.True(t, upstream.usedTLS())
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

func TestOpenAIHeaderRaceBudgetOnlyConsumedWhenBackupStarts(t *testing.T) {
	upstream := &headerRaceHTTPUpstreamStub{
		delays: map[string]time.Duration{
			"https://primary.example": 30 * time.Millisecond,
			"https://backup.example":  80 * time.Millisecond,
		},
	}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	builder := func(ctx context.Context, baseURL string) (*http.Request, error) {
		return http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/responses", strings.NewReader(`{}`))
	}

	result := svc.doOpenAIUpstreamWithHeaderRace(
		context.Background(),
		&Account{ID: 10},
		[]byte(`{}`),
		"",
		openAICodexStabilityPolicy{},
		"https://primary.example",
		"https://backup.example",
		builder,
		openAIHeaderRaceOptions{Enabled: true, Delay: 200 * time.Millisecond, DailyBudget: 1},
	)

	require.NoError(t, result.err)
	require.False(t, result.backupStarted)
	require.Equal(t, 1, upstream.callCount())
	acquired, remaining := svc.headerRaceBudget.tryAcquire(time.Now(), 1)
	require.True(t, acquired)
	require.Equal(t, 0, remaining)
}

func TestOpenAIHeaderRaceStartsBackupAfterDelayAndUsesBudget(t *testing.T) {
	upstream := &headerRaceHTTPUpstreamStub{
		delays: map[string]time.Duration{
			"https://primary.example": 120 * time.Millisecond,
			"https://backup.example":  10 * time.Millisecond,
		},
	}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	builder := func(ctx context.Context, baseURL string) (*http.Request, error) {
		return http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/responses", strings.NewReader(`{}`))
	}

	result := svc.doOpenAIUpstreamWithHeaderRace(
		context.Background(),
		&Account{ID: 10},
		[]byte(`{}`),
		"",
		openAICodexStabilityPolicy{},
		"https://primary.example",
		"https://backup.example",
		builder,
		openAIHeaderRaceOptions{Enabled: true, Delay: 20 * time.Millisecond, DailyBudget: 1},
	)

	require.NoError(t, result.err)
	require.True(t, result.backupStarted)
	require.True(t, result.fromBackup)
	require.Equal(t, 0, result.budgetRemaining)
	require.Equal(t, 2, upstream.callCount())
}

func TestOpenAIHeaderRaceBudgetZeroDisablesBackupRequest(t *testing.T) {
	upstream := &headerRaceHTTPUpstreamStub{
		delays: map[string]time.Duration{
			"https://primary.example": 30 * time.Millisecond,
		},
	}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	builder := func(ctx context.Context, baseURL string) (*http.Request, error) {
		return http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/responses", strings.NewReader(`{}`))
	}

	result := svc.doOpenAIUpstreamWithHeaderRace(
		context.Background(),
		&Account{ID: 10},
		[]byte(`{}`),
		"",
		openAICodexStabilityPolicy{},
		"https://primary.example",
		"https://backup.example",
		builder,
		openAIHeaderRaceOptions{Enabled: true, Delay: time.Millisecond, DailyBudget: 0},
	)

	require.NoError(t, result.err)
	require.False(t, result.backupStarted)
	require.Equal(t, 1, upstream.callCount())
}
