package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

type openAIResponsesProbeRepo struct {
	AccountRepository
	account *Account
	updates map[string]any
}

func (r *openAIResponsesProbeRepo) GetByID(_ context.Context, _ int64) (*Account, error) {
	return r.account, nil
}

func (r *openAIResponsesProbeRepo) UpdateExtra(_ context.Context, _ int64, updates map[string]any) error {
	r.updates = updates
	return nil
}

type openAIResponsesProbeUpstream struct {
	request *http.Request
}

func (u *openAIResponsesProbeUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.request = req
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"id":"resp_probe"}`)),
	}, nil
}

func (u *openAIResponsesProbeUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, concurrency)
}

func TestProbeOpenAIAPIKeyResponsesSupportUsesAccountPassthroughUserAgent(t *testing.T) {
	account := &Account{
		ID:       490,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":    "sk-test",
			"base_url":   "https://responses-probe.example.test/v1",
			"user_agent": "account-probe/1.0",
		},
	}
	repo := &openAIResponsesProbeRepo{account: account}
	upstream := &openAIResponsesProbeUpstream{}
	svc := &AccountTestService{
		accountRepo:  repo,
		httpUpstream: upstream,
		cfg: &config.Config{Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{Enabled: false},
		}},
	}

	svc.ProbeOpenAIAPIKeyResponsesSupport(context.Background(), account.ID)

	require.NotNil(t, upstream.request)
	require.Equal(t, "account-probe/1.0", upstream.request.Header.Get("user-agent"))
	require.Equal(t, "application/json", upstream.request.Header.Get("accept"))
	require.Equal(t, true, repo.updates["openai_responses_supported"])
}
