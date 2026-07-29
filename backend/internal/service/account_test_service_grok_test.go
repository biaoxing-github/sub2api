//go:build unit

package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type accountTestGrokRepo struct {
	*mockAccountRepoForPlatform
	tempUnschedulableCalls int
	lastTempUntil          time.Time
	lastTempReason         string
}

func (r *accountTestGrokRepo) SetTempUnschedulable(_ context.Context, _ int64, until time.Time, reason string) error {
	r.tempUnschedulableCalls++
	r.lastTempUntil = until
	r.lastTempReason = reason
	return nil
}

func newAccountTestGrokRepo(account *Account) *accountTestGrokRepo {
	return &accountTestGrokRepo{mockAccountRepoForPlatform: &mockAccountRepoForPlatform{
		accountsByID: map[int64]*Account{account.ID: account},
	}}
}

func TestAccountTestServiceRoutesGrokAPIKeyToOfficialResponsesProbe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	account := &Account{
		ID:          516,
		Platform:    PlatformGrok,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_keys": []any{"xai-test-key"},
			"base_url": "https://api.x.ai/v1",
		},
	}
	repo := newAccountTestGrokRepo(account)
	upstream := &schedulerExhaustionHTTPUpstream{response: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader("event: response.completed\ndata: {\"type\":\"response.completed\"}\n\n")),
	}}
	svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/516/test", nil)

	err := svc.TestAccountConnection(c, account.ID, "", "", "")

	require.NoError(t, err)
	require.NotNil(t, upstream.request)
	require.Equal(t, "https://api.x.ai/v1/responses", upstream.request.URL.String())
	require.Equal(t, "Bearer xai-test-key", upstream.request.Header.Get("Authorization"))
	requestBody, readErr := io.ReadAll(upstream.request.Body)
	require.NoError(t, readErr)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(requestBody, &payload))
	require.Equal(t, grokDefaultResponsesModel, payload["model"])
	require.Contains(t, recorder.Body.String(), `"model":"grok-4.5"`)
	require.Contains(t, recorder.Body.String(), `"type":"test_complete"`)
}

func TestAccountTestServiceGrokPaymentRequiredUsesOfficialCooldown(t *testing.T) {
	gin.SetMode(gin.TestMode)
	account := &Account{
		ID:          517,
		Platform:    PlatformGrok,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "xai-test-key",
			"base_url": "https://api.x.ai/v1",
		},
	}
	repo := newAccountTestGrokRepo(account)
	upstream := &schedulerExhaustionHTTPUpstream{response: &http.Response{
		StatusCode: http.StatusPaymentRequired,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"code":"personal-team-blocked:spending-limit"}`)),
	}}
	svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/517/test", nil)
	before := time.Now()

	err := svc.TestAccountConnection(c, account.ID, "grok-4.5", "", "")

	require.Error(t, err)
	require.Equal(t, 1, repo.tempUnschedulableCalls)
	require.Equal(t, "grok payment required", repo.lastTempReason)
	require.WithinDuration(t, before.Add(30*time.Minute), repo.lastTempUntil, time.Second)
	require.Contains(t, recorder.Body.String(), `"type":"error"`)
}
