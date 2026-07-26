package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

type schedulerExhaustionHTTPUpstream struct {
	request  *http.Request
	response *http.Response
}

func (u *schedulerExhaustionHTTPUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.request = req
	return u.response, nil
}

func (u *schedulerExhaustionHTTPUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, concurrency)
}

type schedulerExhaustionProbeRepo struct {
	stubOpenAIAccountRepo
	listByGroupCalls    int
	listByPlatformCalls int
}

func (r *schedulerExhaustionProbeRepo) ListByGroup(ctx context.Context, groupID int64) ([]Account, error) {
	r.listByGroupCalls++
	result := make([]Account, 0, len(r.accounts))
	result = append(result, r.accounts...)
	return result, nil
}

func (r *schedulerExhaustionProbeRepo) ListByPlatform(ctx context.Context, platform string) ([]Account, error) {
	r.listByPlatformCalls++
	var result []Account
	for _, acc := range r.accounts {
		if acc.Platform == platform {
			result = append(result, acc)
		}
	}
	return result, nil
}

func TestOpenAISchedulerExhaustionProbeFiniteTriesEachCandidateTwice(t *testing.T) {
	groupID := int64(9)
	repo := &schedulerExhaustionProbeRepo{
		stubOpenAIAccountRepo: stubOpenAIAccountRepo{accounts: []Account{
			{ID: 11, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1},
			{ID: 12, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1},
		}},
	}
	attempts := map[int64]int{}
	svc := &OpenAIGatewayService{
		accountRepo: repo,
		openAISchedulerExhaustionProbeFunc: func(ctx context.Context, account *Account, requestedModel string, requireCompact bool, stream bool) error {
			attempts[account.ID]++
			return fmt.Errorf("probe %d failed", account.ID)
		},
	}

	recovered, err := svc.RecoverOpenAISchedulerExhaustion(context.Background(), OpenAISchedulerExhaustionProbeOptions{
		GroupID:        &groupID,
		RequestedModel: "gpt-5.2",
	})

	require.False(t, recovered)
	require.Error(t, err)
	require.Equal(t, 1, repo.listByGroupCalls)
	require.Equal(t, 2, attempts[11])
	require.Equal(t, 2, attempts[12])
}

func TestRecoverOpenAISchedulerExhaustionPropagatesStreamMode(t *testing.T) {
	groupID := int64(9)
	repo := &schedulerExhaustionProbeRepo{
		stubOpenAIAccountRepo: stubOpenAIAccountRepo{accounts: []Account{
			{ID: 13, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1},
		}},
	}
	observedStream := false
	svc := &OpenAIGatewayService{
		accountRepo: repo,
		openAISchedulerExhaustionProbeFunc: func(_ context.Context, _ *Account, _ string, _ bool, stream bool) error {
			observedStream = stream
			return nil
		},
	}

	recovered, err := svc.RecoverOpenAISchedulerExhaustion(context.Background(), OpenAISchedulerExhaustionProbeOptions{
		GroupID:        &groupID,
		RequestedModel: "gpt-5.6-sol",
		Stream:         true,
	})

	require.NoError(t, err)
	require.True(t, recovered)
	require.True(t, observedStream)
}

func TestSendOpenAISchedulerExhaustionProbeMatchesRequestedStreamMode(t *testing.T) {
	tests := []struct {
		name           string
		stream         bool
		responseBody   string
		responseType   string
		wantAccept     string
		wantBodyStream bool
	}{
		{
			name:           "non-stream request keeps JSON probe",
			responseBody:   `{"id":"resp_probe","status":"completed"}`,
			responseType:   "application/json",
			wantAccept:     "application/json",
			wantBodyStream: false,
		},
		{
			name:           "stream request uses SSE probe with completed terminal event",
			stream:         true,
			responseBody:   "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_probe\"}}\n\n",
			responseType:   "text/event-stream",
			wantAccept:     "text/event-stream",
			wantBodyStream: true,
		},
		{
			name:           "stream request accepts repaired concatenated terminal event",
			stream:         true,
			responseBody:   "event: response.in_progress\ndata: {\"type\":\"response.in_progress\"}{\"type\":\"response.completed\",\"response\":{\"id\":\"resp_probe\"}}\n\n",
			responseType:   "text/event-stream",
			wantAccept:     "text/event-stream",
			wantBodyStream: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := &schedulerExhaustionHTTPUpstream{response: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{tt.responseType}},
				Body:       io.NopCloser(strings.NewReader(tt.responseBody)),
			}}
			svc := &OpenAIGatewayService{
				httpUpstream: upstream,
				cfg: &config.Config{Security: config.SecurityConfig{
					URLAllowlist: config.URLAllowlistConfig{Enabled: false},
				}},
			}
			account := &Account{
				ID:          480,
				Platform:    PlatformOpenAI,
				Type:        AccountTypeAPIKey,
				Concurrency: 1,
				Credentials: map[string]any{
					"api_key":  "sk-test",
					"base_url": "https://compat-upstream.example/v1",
				},
				Extra: map[string]any{
					"openai_codex_cli_simulation_enabled": true,
				},
			}

			err := svc.sendOpenAISchedulerExhaustionProbe(context.Background(), account, "gpt-5.6-sol", false, tt.stream)

			require.NoError(t, err)
			require.NotNil(t, upstream.request)
			require.Equal(t, tt.wantAccept, upstream.request.Header.Get("accept"))
			require.Equal(t, codexCLIUserAgent(), upstream.request.Header.Get("user-agent"))
			require.Equal(t, codexCLIOriginator, upstream.request.Header.Get("originator"))
			require.Equal(t, "responses=experimental", upstream.request.Header.Get("openai-beta"))
			requestBody, readErr := io.ReadAll(upstream.request.Body)
			require.NoError(t, readErr)
			var payload map[string]any
			require.NoError(t, json.Unmarshal(requestBody, &payload))
			require.Equal(t, tt.wantBodyStream, payload["stream"])
		})
	}
}

// TestSendOpenAISchedulerExhaustionProbeFallsBackUserAgentWithoutSimulation 覆盖
// 模拟开关关闭且无自定义 UA 的 API Key 账号：探测请求不允许落到 Go 默认
// User-Agent（Go-http-client/1.1），否则按客户端 UA 风控的渠道会持续 403。
func TestSendOpenAISchedulerExhaustionProbeFallsBackUserAgentWithoutSimulation(t *testing.T) {
	upstream := &schedulerExhaustionHTTPUpstream{response: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"resp_probe","status":"completed"}`)),
	}}
	svc := &OpenAIGatewayService{
		httpUpstream: upstream,
		cfg: &config.Config{Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{Enabled: false},
		}},
	}
	account := &Account{
		ID:          481,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://compat-upstream.example/v1",
		},
	}

	err := svc.sendOpenAISchedulerExhaustionProbe(context.Background(), account, "gpt-5.6-sol", false, false)

	require.NoError(t, err)
	require.NotNil(t, upstream.request)
	require.Equal(t, codexCLIUserAgent(), upstream.request.Header.Get("user-agent"))
}

func TestSendOpenAISchedulerExhaustionProbeRejectsStreamWithoutTerminalEvent(t *testing.T) {
	upstream := &schedulerExhaustionHTTPUpstream{response: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader("event: response.in_progress\ndata: {\"type\":\"response.in_progress\"}\n\n")),
	}}
	svc := &OpenAIGatewayService{
		httpUpstream: upstream,
		cfg: &config.Config{Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{Enabled: false},
		}},
	}
	account := &Account{
		ID:          480,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://compat-upstream.example/v1",
		},
	}

	err := svc.sendOpenAISchedulerExhaustionProbe(context.Background(), account, "gpt-5.6-sol", false, true)

	require.ErrorContains(t, err, "ended before response.completed")
}

// TestValidateOpenAISchedulerExhaustionProbeStreamRejectsInvalidTerminalStreams 验证异常流不会解除账号调度屏蔽。
func TestValidateOpenAISchedulerExhaustionProbeStreamRejectsInvalidTerminalStreams(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{
			name:    "failed terminal",
			body:    "data: {\"type\":\"response.failed\",\"response\":{\"error\":{\"message\":\"upstream failed\"}}}\n\n",
			wantErr: "upstream failed",
		},
		{
			name:    "incomplete terminal",
			body:    "data: {\"type\":\"response.incomplete\"}\n\n",
			wantErr: "response.incomplete",
		},
		{
			name:    "cancelled terminal",
			body:    "data: {\"type\":\"response.cancelled\"}\n\n",
			wantErr: "response.cancelled",
		},
		{
			name:    "error terminal",
			body:    "data: {\"type\":\"error\",\"error\":{\"message\":\"stream error\"}}\n\n",
			wantErr: "stream error",
		},
		{
			name:    "malformed JSON",
			body:    "data: not-json\n\n",
			wantErr: "invalid SSE JSON data",
		},
		{
			name:    "empty stream",
			body:    "event: ping\n\n",
			wantErr: "no SSE data",
		},
		{
			name:    "done without terminal",
			body:    "data: [DONE]\n\n",
			wantErr: "ended before response.completed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOpenAISchedulerExhaustionProbeStream([]byte(tt.body))

			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestSendGrokSchedulerExhaustionProbeMatchesRequestedStreamMode(t *testing.T) {
	upstream := &schedulerExhaustionHTTPUpstream{response: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader("event: response.completed\ndata: {\"type\":\"response.completed\"}\n\n")),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{
		ID:          481,
		Platform:    PlatformGrok,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "xai-test",
			"base_url": "https://api.x.ai/v1",
		},
	}

	err := svc.sendGrokSchedulerExhaustionProbe(context.Background(), account, "grok-4.3", true)

	require.NoError(t, err)
	require.NotNil(t, upstream.request)
	require.Equal(t, "text/event-stream", upstream.request.Header.Get("accept"))
	requestBody, readErr := io.ReadAll(upstream.request.Body)
	require.NoError(t, readErr)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(requestBody, &payload))
	require.Equal(t, true, payload["stream"])
}

func TestOpenAISchedulerExhaustionProbeStopsOnSuccessAndClearsRuntimeBlock(t *testing.T) {
	groupID := int64(9)
	repo := &schedulerExhaustionProbeRepo{
		stubOpenAIAccountRepo: stubOpenAIAccountRepo{accounts: []Account{
			{ID: 21, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1},
			{ID: 22, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1},
		}},
	}
	attempts := map[int64]int{}
	svc := &OpenAIGatewayService{
		accountRepo: repo,
		openAISchedulerExhaustionProbeFunc: func(ctx context.Context, account *Account, requestedModel string, requireCompact bool, stream bool) error {
			attempts[account.ID]++
			if account.ID == 22 && attempts[account.ID] == 2 {
				return nil
			}
			return errors.New("still unavailable")
		},
	}
	svc.BlockAccountScheduling(&repo.accounts[1], time.Now().Add(time.Minute), "test_block")

	recovered, err := svc.RecoverOpenAISchedulerExhaustion(context.Background(), OpenAISchedulerExhaustionProbeOptions{
		GroupID:        &groupID,
		RequestedModel: "gpt-5.2",
	})

	require.NoError(t, err)
	require.True(t, recovered)
	require.Equal(t, 2, attempts[21])
	require.Equal(t, 2, attempts[22])
	_, blocked := svc.SnapshotOpenAIAccountRuntimeBlock(&repo.accounts[1], time.Now())
	require.False(t, blocked)
}

func TestOpenAISchedulerExhaustionProbeUsesRequestedGrokPlatform(t *testing.T) {
	groupID := int64(9)
	repo := &schedulerExhaustionProbeRepo{
		stubOpenAIAccountRepo: stubOpenAIAccountRepo{accounts: []Account{
			{ID: 61, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1},
			{ID: 62, Platform: PlatformGrok, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1},
		}},
	}
	attempted := make([]int64, 0, 1)
	svc := &OpenAIGatewayService{
		accountRepo: repo,
		openAISchedulerExhaustionProbeFunc: func(ctx context.Context, account *Account, requestedModel string, requireCompact bool, stream bool) error {
			attempted = append(attempted, account.ID)
			return nil
		},
	}

	recovered, err := svc.RecoverOpenAISchedulerExhaustion(context.Background(), OpenAISchedulerExhaustionProbeOptions{
		GroupID:        &groupID,
		Platform:       PlatformGrok,
		RequestedModel: "grok-4.3",
	})

	require.NoError(t, err)
	require.True(t, recovered)
	require.Equal(t, []int64{62}, attempted)
}

func TestOpenAISchedulerExhaustionProbeSkipsTempCoolingAccount(t *testing.T) {
	groupID := int64(9)
	coolingUntil := time.Now().Add(2 * time.Minute)
	repo := &schedulerExhaustionProbeRepo{
		stubOpenAIAccountRepo: stubOpenAIAccountRepo{accounts: []Account{
			{
				ID:                     26,
				Platform:               PlatformOpenAI,
				Type:                   AccountTypeAPIKey,
				Status:                 StatusActive,
				Schedulable:            true,
				Concurrency:            1,
				TempUnschedulableUntil: &coolingUntil,
			},
		}},
	}
	attempts := 0
	svc := &OpenAIGatewayService{
		accountRepo: repo,
		openAISchedulerExhaustionProbeFunc: func(ctx context.Context, account *Account, requestedModel string, requireCompact bool, stream bool) error {
			attempts++
			return nil
		},
	}

	recovered, err := svc.RecoverOpenAISchedulerExhaustion(context.Background(), OpenAISchedulerExhaustionProbeOptions{
		GroupID:        &groupID,
		RequestedModel: "gpt-5.2",
		Infinite:       true,
	})

	require.ErrorIs(t, err, ErrNoAvailableAccounts)
	require.False(t, recovered)
	require.Equal(t, 0, attempts)
}

func TestOpenAISchedulerExhaustionProbeInfiniteIgnoresFiniteAttemptCapUntilSuccess(t *testing.T) {
	groupID := int64(9)
	repo := &schedulerExhaustionProbeRepo{
		stubOpenAIAccountRepo: stubOpenAIAccountRepo{accounts: []Account{
			{ID: 31, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1},
		}},
	}
	attempts := 0
	svc := &OpenAIGatewayService{
		accountRepo: repo,
		openAISchedulerExhaustionProbeFunc: func(ctx context.Context, account *Account, requestedModel string, requireCompact bool, stream bool) error {
			attempts++
			if attempts == 8 {
				return nil
			}
			return errors.New("not yet")
		},
		openAISchedulerExhaustionProbeSleep: func(ctx context.Context, d time.Duration) error {
			return ctx.Err()
		},
	}

	recovered, err := svc.RecoverOpenAISchedulerExhaustion(context.Background(), OpenAISchedulerExhaustionProbeOptions{
		GroupID:        &groupID,
		RequestedModel: "gpt-5.2",
		Infinite:       true,
	})

	require.NoError(t, err)
	require.True(t, recovered)
	require.Equal(t, 8, attempts)
}

func TestOpenAISchedulerExhaustionProbeInfiniteNotifiesAfterThresholdAndRepeatInterval(t *testing.T) {
	groupID := int64(9)
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	repo := &schedulerExhaustionProbeRepo{
		stubOpenAIAccountRepo: stubOpenAIAccountRepo{accounts: []Account{
			{ID: 41, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1},
		}},
	}
	attempts := 0
	var events []openAISchedulerExhaustionProbeNotifyEvent
	svc := &OpenAIGatewayService{
		accountRepo: repo,
		cfg: &config.Config{Gateway: config.GatewayConfig{
			OpenAISchedulerProbeNotifyEnabled:          true,
			OpenAISchedulerProbeNotifyAfterSeconds:     3,
			OpenAISchedulerProbeNotifyRepeatSeconds:    4,
			OpenAISchedulerProbeNotifyRecoveredEnabled: true,
		}},
		openAISchedulerExhaustionProbeNow: func() time.Time {
			return now
		},
		openAISchedulerExhaustionProbeFunc: func(ctx context.Context, account *Account, requestedModel string, requireCompact bool, stream bool) error {
			attempts++
			return errors.New("still no schedulable account")
		},
		openAISchedulerExhaustionProbeSleep: func(ctx context.Context, d time.Duration) error {
			now = now.Add(d)
			if attempts >= 5 {
				return context.Canceled
			}
			return nil
		},
		openAISchedulerExhaustionNotifyFunc: func(ctx context.Context, event openAISchedulerExhaustionProbeNotifyEvent) error {
			events = append(events, event)
			return nil
		},
	}

	recovered, err := svc.RecoverOpenAISchedulerExhaustion(context.Background(), OpenAISchedulerExhaustionProbeOptions{
		GroupID:        &groupID,
		RequestedModel: "gpt-5.2",
		Infinite:       true,
	})

	require.False(t, recovered)
	require.ErrorIs(t, err, context.Canceled)
	require.Len(t, events, 4)
	require.Equal(t, openAISchedulerExhaustionNotifyPhaseWaiting, events[0].Phase)
	require.Equal(t, 30*time.Second, events[0].Elapsed)
	require.Equal(t, 2, events[0].Attempts)
	require.Equal(t, openAISchedulerExhaustionNotifyPhaseWaiting, events[1].Phase)
	require.Equal(t, 1*time.Minute, events[1].Elapsed)
	require.Equal(t, 3, events[1].Attempts)
	require.Equal(t, openAISchedulerExhaustionNotifyPhaseWaiting, events[3].Phase)
	require.Equal(t, 2*time.Minute, events[3].Elapsed)
	require.Equal(t, 5, events[3].Attempts)
	require.Equal(t, "still no schedulable account", events[3].LastError)
}

func TestOpenAISchedulerExhaustionProbeInfiniteSendsRecoveredNotificationAfterWaitingNotification(t *testing.T) {
	groupID := int64(9)
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	repo := &schedulerExhaustionProbeRepo{
		stubOpenAIAccountRepo: stubOpenAIAccountRepo{accounts: []Account{
			{ID: 51, Name: "primary-oauth", Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1},
		}},
	}
	attempts := 0
	var events []openAISchedulerExhaustionProbeNotifyEvent
	svc := &OpenAIGatewayService{
		accountRepo: repo,
		cfg: &config.Config{Gateway: config.GatewayConfig{
			OpenAISchedulerProbeNotifyEnabled:          true,
			OpenAISchedulerProbeNotifyAfterSeconds:     3,
			OpenAISchedulerProbeNotifyRepeatSeconds:    60,
			OpenAISchedulerProbeNotifyRecoveredEnabled: true,
		}},
		openAISchedulerExhaustionProbeNow: func() time.Time {
			return now
		},
		openAISchedulerExhaustionProbeFunc: func(ctx context.Context, account *Account, requestedModel string, requireCompact bool, stream bool) error {
			attempts++
			if attempts == 4 {
				return nil
			}
			return errors.New("still no schedulable account")
		},
		openAISchedulerExhaustionProbeSleep: func(ctx context.Context, d time.Duration) error {
			now = now.Add(d)
			return nil
		},
		openAISchedulerExhaustionNotifyFunc: func(ctx context.Context, event openAISchedulerExhaustionProbeNotifyEvent) error {
			events = append(events, event)
			return nil
		},
	}

	recovered, err := svc.RecoverOpenAISchedulerExhaustion(context.Background(), OpenAISchedulerExhaustionProbeOptions{
		GroupID:        &groupID,
		RequestedModel: "gpt-5.2",
		Infinite:       true,
	})

	require.NoError(t, err)
	require.True(t, recovered)
	require.Len(t, events, 2)
	require.Equal(t, openAISchedulerExhaustionNotifyPhaseWaiting, events[0].Phase)
	require.Equal(t, openAISchedulerExhaustionNotifyPhaseRecovered, events[1].Phase)
	require.Equal(t, int64(51), events[1].AccountID)
	require.Equal(t, "primary-oauth", events[1].AccountName)
	require.Equal(t, 90*time.Second, events[1].Elapsed)
}

func TestOpenAISchedulerExhaustionProbeFeishuNotificationSendsTextPayload(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Contains(t, r.Header.Get("content-type"), "application/json")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"StatusCode":0}`))
	}))
	defer server.Close()

	svc := &OpenAIGatewayService{}
	err := svc.sendOpenAISchedulerExhaustionFeishuNotification(context.Background(), server.URL, openAISchedulerExhaustionProbeNotifyEvent{
		Phase:          openAISchedulerExhaustionNotifyPhaseWaiting,
		StartedAt:      time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC),
		Elapsed:        90 * time.Second,
		Rounds:         45,
		Attempts:       45,
		CandidateCount: 3,
		RequestedModel: "gpt-5.2",
		LastError:      "429 Too Many Requests",
	})

	require.NoError(t, err)
	require.Equal(t, "text", payload["msg_type"])
	content, ok := payload["content"].(map[string]any)
	require.True(t, ok)
	text, ok := content["text"].(string)
	require.True(t, ok)
	require.Contains(t, text, "OpenAI /responses")
	require.Contains(t, text, "无限调度等待")
	require.Contains(t, text, "90s")
	require.Contains(t, text, "45")
	require.Contains(t, text, "429 Too Many Requests")
}

func TestOpenAISchedulerExhaustionProbeFeishuAppNotificationUsesTenantTokenAndChatMessage(t *testing.T) {
	var tokenPayload map[string]string
	var messagePayload map[string]any
	var receiveIDType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			require.Equal(t, http.MethodPost, r.Method)
			require.NoError(t, json.NewDecoder(r.Body).Decode(&tokenPayload))
			w.Header().Set("content-type", "application/json")
			_, _ = w.Write([]byte(`{"code":0,"tenant_access_token":"tenant-token"}`))
		case "/open-apis/im/v1/messages":
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "Bearer tenant-token", r.Header.Get("authorization"))
			receiveIDType = r.URL.Query().Get("receive_id_type")
			require.NoError(t, json.NewDecoder(r.Body).Decode(&messagePayload))
			w.Header().Set("content-type", "application/json")
			_, _ = w.Write([]byte(`{"code":0,"data":{"message_id":"om_xxx"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc := &OpenAIGatewayService{}
	err := svc.sendOpenAISchedulerExhaustionFeishuAppNotification(context.Background(), config.GatewayConfig{
		OpenAISchedulerProbeNotifyFeishuAppID:         "cli_test",
		OpenAISchedulerProbeNotifyFeishuAppSecret:     "secret_test",
		OpenAISchedulerProbeNotifyFeishuDomain:        server.URL,
		OpenAISchedulerProbeNotifyFeishuReceiveIDType: "chat_id",
		OpenAISchedulerProbeNotifyFeishuReceiveID:     "oc_test",
	}, openAISchedulerExhaustionProbeNotifyEvent{
		Phase:          openAISchedulerExhaustionNotifyPhaseWaiting,
		StartedAt:      time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC),
		Elapsed:        75 * time.Second,
		Rounds:         37,
		Attempts:       37,
		CandidateCount: 2,
		RequestedModel: "gpt-5.2",
	})

	require.NoError(t, err)
	require.Equal(t, "cli_test", tokenPayload["app_id"])
	require.Equal(t, "secret_test", tokenPayload["app_secret"])
	require.Equal(t, "chat_id", receiveIDType)
	require.Equal(t, "oc_test", messagePayload["receive_id"])
	require.Equal(t, "text", messagePayload["msg_type"])
	contentRaw, ok := messagePayload["content"].(string)
	require.True(t, ok)
	var content map[string]string
	require.NoError(t, json.Unmarshal([]byte(contentRaw), &content))
	require.Contains(t, content["text"], "OpenAI /responses")
	require.Contains(t, content["text"], "75s")
}

func TestProbeIntervalFromErrorCount(t *testing.T) {
	tests := []struct {
		errorCount int
		want       time.Duration
	}{
		{0, 30 * time.Second},
		{1, 30 * time.Second},
		{2, 30 * time.Second},
		{3, 30 * time.Second},
		{4, 30 * time.Second},
		{5, 1 * time.Minute},
		{6, 5 * time.Minute},
		{7, 30 * time.Minute},
		{8, 60 * time.Minute},
		{10, 60 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("error_count_%d", tt.errorCount), func(t *testing.T) {
			got := probeIntervalFromErrorCount(tt.errorCount)
			if got != tt.want {
				t.Errorf("probeIntervalFromErrorCount(%d) = %v, want %v", tt.errorCount, got, tt.want)
			}
		})
	}
}

func TestRoundDelayFromFailureCounts(t *testing.T) {
	tests := []struct {
		name   string
		counts []int64
		want   time.Duration
	}{
		{"empty falls back to base", nil, openAISchedulerExhaustionProbeLoopDelay},
		{"single low count", []int64{0}, 30 * time.Second},
		{"single mid count", []int64{3}, 30 * time.Second},
		{"max wins", []int64{1, 5, 2}, 1 * time.Minute},
		{"high count caps", []int64{8, 1}, 60 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := roundDelayFromFailureCounts(tt.counts)
			if got != tt.want {
				t.Errorf("roundDelayFromFailureCounts(%v) = %v, want %v", tt.counts, got, tt.want)
			}
		})
	}
}
