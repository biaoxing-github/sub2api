//go:build unit

package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type accountProbeAccountRepoStub struct {
	account *Account
}

func (r *accountProbeAccountRepoStub) GetByID(ctx context.Context, id int64) (*Account, error) {
	if r.account == nil || r.account.ID != id {
		return nil, ErrAccountNotFound
	}
	out := *r.account
	return &out, nil
}

type accountProbeRepoStub struct {
	runID   int64
	created *AccountProbeResult
	updated *AccountProbeResult
	samples []AccountProbeSample
}

func (r *accountProbeRepoStub) CreateAccountProbeRun(ctx context.Context, run *AccountProbeResult) error {
	r.runID = 99
	run.ID = r.runID
	copy := *run
	r.created = &copy
	return nil
}

func (r *accountProbeRepoStub) UpdateAccountProbeRun(ctx context.Context, run *AccountProbeResult) error {
	copy := *run
	r.updated = &copy
	return nil
}

func (r *accountProbeRepoStub) ExpireStaleAccountProbeRuns(ctx context.Context, olderThan time.Duration) error {
	return nil
}

func (r *accountProbeRepoStub) SaveAccountProbeSample(ctx context.Context, sample AccountProbeSample) error {
	r.samples = append(r.samples, sample)
	return nil
}

func (r *accountProbeRepoStub) ListAccountProbeRuns(ctx context.Context, filter AccountProbeHistoryFilter) ([]AccountProbeResult, error) {
	return nil, nil
}

func (r *accountProbeRepoStub) GetAccountProbeRun(ctx context.Context, accountID, runID int64) (*AccountProbeResult, error) {
	return nil, ErrAccountNotFound
}

func (r *accountProbeRepoStub) ListAccountProbeReportRuns(ctx context.Context, filter AccountProbeReportFilter) ([]AccountProbeReportItem, int, error) {
	return nil, 0, nil
}

func (r *accountProbeRepoStub) GetAccountProbeReportRun(ctx context.Context, runID int64) (*AccountProbeReportItem, error) {
	return nil, ErrAccountNotFound
}

func (r *accountProbeRepoStub) ListAccountProbeSamples(ctx context.Context, runID int64) ([]AccountProbeSample, error) {
	return r.samples, nil
}

type contextCanceledProbeRepoStub struct {
	accountProbeRepoStub
	updatedWithCanceledCtx bool
}

func (r *contextCanceledProbeRepoStub) UpdateAccountProbeRun(ctx context.Context, run *AccountProbeResult) error {
	if errors.Is(ctx.Err(), context.Canceled) {
		r.updatedWithCanceledCtx = true
		return ctx.Err()
	}
	return r.accountProbeRepoStub.UpdateAccountProbeRun(ctx, run)
}

type accountProbeHTTPClientStub struct {
	requests []*http.Request
	bodies   []string
	mu       sync.Mutex
}

func (c *accountProbeHTTPClientStub) Do(req *http.Request) (*http.Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.requests = append(c.requests, req)
	if req.Body != nil {
		data, _ := io.ReadAll(req.Body)
		c.bodies = append(c.bodies, string(data))
	}
	body := `{"usage":{"input_tokens":12,"output_tokens":3,"total_tokens":15}}`
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}, nil
}

type contextDeadlineProbeHTTPClientStub struct{}

func (c *contextDeadlineProbeHTTPClientStub) Do(req *http.Request) (*http.Response, error) {
	<-req.Context().Done()
	return nil, req.Context().Err()
}

type flakyAccountProbeHTTPClientStub struct {
	mu       sync.Mutex
	attempts int
}

func (c *flakyAccountProbeHTTPClientStub) Do(req *http.Request) (*http.Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.attempts++
	if c.attempts == 1 {
		return nil, io.ErrUnexpectedEOF
	}
	body := `{"usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6}}`
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}, nil
}

type blockingProbeHTTPClientStub struct {
	started   chan struct{}
	release   chan struct{}
	active    int
	maxActive int
	mu        sync.Mutex
}

func (c *blockingProbeHTTPClientStub) Do(req *http.Request) (*http.Response, error) {
	c.mu.Lock()
	c.active++
	if c.active > c.maxActive {
		c.maxActive = c.active
	}
	started := c.started
	release := c.release
	c.mu.Unlock()
	if started != nil {
		started <- struct{}{}
	}
	if release != nil {
		select {
		case <-release:
		case <-req.Context().Done():
		}
	}
	c.mu.Lock()
	c.active--
	c.mu.Unlock()
	body := `{"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}, nil
}

func TestAccountProbeService_RunOpenAIAPIKeyPersistsSamples(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       128,
		Name:     "encore",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Status:   StatusActive,
		Credentials: map[string]any{
			"base_url": "https://example.test/v1",
			"api_keys": []any{"sk-one", "sk-two"},
		},
		Extra: map[string]any{"openai_api_mode": "responses"},
	}
	repo := &accountProbeRepoStub{}
	client := &accountProbeHTTPClientStub{}
	svc := NewAccountProbeService(&accountProbeAccountRepoStub{account: account}, repo, client, nil)

	result, err := svc.Run(context.Background(), AccountProbeRunRequest{
		AccountID: 128,
		Profile:   AccountProbeProfileStandard,
		Model:     "gpt-test",
	})

	require.NoError(t, err)
	require.Equal(t, int64(128), result.AccountID)
	require.Equal(t, AccountProbeStatusSuccess, result.Status)
	require.Equal(t, 3, result.RequestCount)
	require.Equal(t, 3, result.SuccessCount)
	require.Equal(t, 45, result.TotalTokens)
	require.Len(t, repo.samples, 3)
	require.NotNil(t, repo.updated)
	require.Equal(t, int64(99), repo.samples[0].RunID)
	require.Equal(t, "sk-one", strings.TrimPrefix(client.requests[0].Header.Get("Authorization"), "Bearer "))
	require.Equal(t, "sk-two", strings.TrimPrefix(client.requests[1].Header.Get("Authorization"), "Bearer "))
	require.Contains(t, client.requests[0].URL.String(), "/v1/responses")
	require.Contains(t, client.bodies[0], `"stream":false`)
	require.NotEmpty(t, repo.samples[0].APIKeyFingerprint)
	require.NotContains(t, repo.samples[0].APIKeyMasked, "sk-one")
}

type accountProbeStreamHTTPClientStub struct {
	requests []*http.Request
	bodies   []string
}

func (c *accountProbeStreamHTTPClientStub) Do(req *http.Request) (*http.Response, error) {
	c.requests = append(c.requests, req)
	if req.Body != nil {
		data, _ := io.ReadAll(req.Body)
		c.bodies = append(c.bodies, string(data))
	}
	body := strings.Join([]string{
		`data: {"type":"response.created"}`,
		``,
		`data: {"type":"response.output_text.delta","delta":"ok"}`,
		``,
		`data: {"type":"response.completed","response":{"usage":{"input_tokens":8,"output_tokens":2,"total_tokens":10}}}`,
		``,
	}, "\n")
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}, nil
}

func TestAccountProbeService_RunOpenAIAPIKeyStreamModeRecordsFirstToken(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       128,
		Name:     "encore",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Status:   StatusActive,
		Credentials: map[string]any{
			"base_url": "https://stream-mode.example.test/v1",
			"api_key":  "sk-one",
		},
		Extra: map[string]any{"openai_api_mode": "responses"},
	}
	repo := &accountProbeRepoStub{}
	client := &accountProbeStreamHTTPClientStub{}
	svc := NewAccountProbeService(&accountProbeAccountRepoStub{account: account}, repo, client, nil)

	result, err := svc.Run(context.Background(), AccountProbeRunRequest{
		AccountID:   128,
		Profile:     AccountProbeProfileQuick,
		Model:       "gpt-test",
		RequestMode: AccountProbeRequestModeStream,
	})

	require.NoError(t, err)
	require.Equal(t, AccountProbeRequestModeStream, result.RequestMode)
	require.Equal(t, AccountProbeStatusSuccess, result.Status)
	require.Equal(t, 10, result.TotalTokens)
	require.NotNil(t, result.FirstTokenMillis)
	require.Len(t, repo.samples, 1)
	require.NotNil(t, repo.samples[0].FirstTokenMillis)
	require.Equal(t, "text/event-stream", client.requests[0].Header.Get("Accept"))
	require.Contains(t, client.bodies[0], `"stream":true`)
}

func TestAccountProbeService_RetriesTransientProbeFailure(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       128,
		Name:     "encore",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Status:   StatusActive,
		Credentials: map[string]any{
			"base_url": "https://retry-transient.example.test/v1",
			"api_key":  "sk-one",
		},
		Extra: map[string]any{"openai_api_mode": "responses"},
	}
	repo := &accountProbeRepoStub{}
	client := &flakyAccountProbeHTTPClientStub{}
	svc := NewAccountProbeService(&accountProbeAccountRepoStub{account: account}, repo, client, nil)

	result, err := svc.Run(context.Background(), AccountProbeRunRequest{
		AccountID: 128,
		Profile:   AccountProbeProfileQuick,
		Model:     "gpt-test",
	})

	require.NoError(t, err)
	require.Equal(t, AccountProbeStatusSuccess, result.Status)
	require.Equal(t, 1, result.SuccessCount)
	require.Equal(t, 0, result.FailureCount)
	require.Equal(t, 2, client.attempts)
	require.Len(t, repo.samples, 1)
	require.Equal(t, AccountProbeSampleSuccess, repo.samples[0].Status)
}

func TestAccountProbeService_LimitsConcurrentRunsForSameBaseURL(t *testing.T) {
	account := &Account{
		ID:       128,
		Name:     "encore",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Status:   StatusActive,
		Credentials: map[string]any{
			"base_url": "https://same.example.test/v1",
			"api_key":  "sk-one",
		},
		Extra: map[string]any{"openai_api_mode": "responses"},
	}
	client := &blockingProbeHTTPClientStub{
		started: make(chan struct{}, 2),
		release: make(chan struct{}),
	}
	svc := NewAccountProbeService(&accountProbeAccountRepoStub{account: account}, &accountProbeRepoStub{}, client, nil)

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = svc.Run(context.Background(), AccountProbeRunRequest{
				AccountID: 128,
				Profile:   AccountProbeProfileQuick,
				Model:     "gpt-test",
			})
		}()
	}

	require.Eventually(t, func() bool {
		client.mu.Lock()
		defer client.mu.Unlock()
		return client.active == 1 && client.maxActive == 1
	}, time.Second, 10*time.Millisecond)
	close(client.release)
	wg.Wait()
	client.mu.Lock()
	require.Equal(t, 1, client.maxActive)
	client.mu.Unlock()
}

func TestAccountProbeService_RunExistingFinalizesAfterCallerContextDeadline(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       128,
		Name:     "encore",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Status:   StatusActive,
		Credentials: map[string]any{
			"base_url": "https://canceled-context.example.test/v1",
			"api_key":  "sk-one",
		},
		Extra: map[string]any{"openai_api_mode": "responses"},
	}
	repo := &contextCanceledProbeRepoStub{}
	svc := NewAccountProbeService(&accountProbeAccountRepoStub{account: account}, repo, &contextDeadlineProbeHTTPClientStub{}, nil)

	run, err := svc.Start(context.Background(), AccountProbeRunRequest{
		AccountID: 128,
		Profile:   AccountProbeProfileQuick,
		Model:     "gpt-test",
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := svc.RunExisting(ctx, run, AccountProbeRunRequest{
		AccountID: 128,
		Profile:   AccountProbeProfileQuick,
		Model:     "gpt-test",
	})

	require.NoError(t, err)
	require.Equal(t, AccountProbeStatusFailed, result.Status)
	require.NotNil(t, repo.updated)
	require.False(t, repo.updatedWithCanceledCtx)
	require.Equal(t, AccountProbeStatusFailed, repo.updated.Status)
	require.Equal(t, 1, repo.updated.FailureCount)
	require.NotNil(t, repo.updated.FinishedAt)
}

func TestAccountProbeService_RunRejectsUnsupportedAccount(t *testing.T) {
	t.Parallel()

	svc := NewAccountProbeService(&accountProbeAccountRepoStub{account: &Account{
		ID:       7,
		Platform: PlatformGemini,
		Type:     AccountTypeAPIKey,
	}}, &accountProbeRepoStub{}, &accountProbeHTTPClientStub{}, nil)

	_, err := svc.Run(context.Background(), AccountProbeRunRequest{AccountID: 7})

	require.ErrorContains(t, err, "only openai api key accounts support probe runs")
}

func TestFinalizeAccountProbeResultMarksPartial(t *testing.T) {
	t.Parallel()

	run := &AccountProbeResult{
		Samples: []AccountProbeSample{
			{Status: AccountProbeSampleSuccess, DurationMillis: 100, TotalTokens: 10},
			{Status: AccountProbeSampleFailed, DurationMillis: 300, ErrorMessage: "upstream failed"},
		},
	}
	finalizeAccountProbeResult(run)

	require.Equal(t, AccountProbeStatusPartial, run.Status)
	require.Equal(t, 1, run.SuccessCount)
	require.Equal(t, 1, run.FailureCount)
	require.Equal(t, 200, run.Latency.AvgMillis)
	require.Equal(t, "upstream failed", run.ErrorMessage)
	require.WithinDuration(t, time.Now(), *run.FinishedAt, time.Second)
}

func TestClassifyAccountProbeErrorDetectsCloudflareWAF(t *testing.T) {
	t.Parallel()

	key, label, penalty := classifyAccountProbeError(`<!DOCTYPE html><title>Just a moment...</title><center>cloudflare</center>`)

	require.Equal(t, "cloudflare_waf", key)
	require.Equal(t, "Cloudflare/WAF 拦截", label)
	require.Equal(t, 15, penalty)
}
