//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
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

func (r *accountProbeRepoStub) ListAccountProbeSamples(ctx context.Context, runID int64) ([]AccountProbeSample, error) {
	return r.samples, nil
}

type accountProbeHTTPClientStub struct {
	requests []*http.Request
}

func (c *accountProbeHTTPClientStub) Do(req *http.Request) (*http.Response, error) {
	c.requests = append(c.requests, req)
	body := `{"usage":{"input_tokens":12,"output_tokens":3,"total_tokens":15}}`
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
	require.NotEmpty(t, repo.samples[0].APIKeyFingerprint)
	require.NotContains(t, repo.samples[0].APIKeyMasked, "sk-one")
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
