//go:build unit

package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// --- shared test helpers ---

type queuedHTTPUpstream struct {
	responses []*http.Response
	errs      []error
	requests  []*http.Request
	tlsFlags  []bool
}

func (u *queuedHTTPUpstream) Do(_ *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	return nil, fmt.Errorf("unexpected Do call")
}

func (u *queuedHTTPUpstream) DoWithTLS(req *http.Request, _ string, _ int64, _ int, profile *tlsfingerprint.Profile) (*http.Response, error) {
	u.requests = append(u.requests, req)
	u.tlsFlags = append(u.tlsFlags, profile != nil)
	if len(u.errs) > 0 {
		err := u.errs[0]
		u.errs = u.errs[1:]
		if err != nil {
			return nil, err
		}
	}
	if len(u.responses) == 0 {
		return nil, fmt.Errorf("no mocked response")
	}
	resp := u.responses[0]
	u.responses = u.responses[1:]
	return resp, nil
}

func newJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// --- test functions ---

func newTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/1/test", nil)
	return c, rec
}

type openAIAccountTestRepo struct {
	mockAccountRepoForGemini
	updatedExtra       map[string]any
	updatedCredentials map[string]any
	bulkUpdatedIDs     []int64
	bulkUpdatedPayload AccountBulkUpdate
	rateLimitedID      int64
	rateLimitedAt      *time.Time
	tempUnschedID      int64
	tempUnschedAt      *time.Time
	tempUnschedReason  string
	clearedErrorID     int64
	setErrorID         int64
	setErrorMsg        string
}

func (r *openAIAccountTestRepo) UpdateExtra(_ context.Context, _ int64, updates map[string]any) error {
	r.updatedExtra = updates
	return nil
}

func (r *openAIAccountTestRepo) UpdateCredentials(_ context.Context, _ int64, credentials map[string]any) error {
	r.updatedCredentials = cloneCredentials(credentials)
	return nil
}

func (r *openAIAccountTestRepo) BulkUpdate(_ context.Context, ids []int64, updates AccountBulkUpdate) (int64, error) {
	r.bulkUpdatedIDs = append([]int64(nil), ids...)
	r.bulkUpdatedPayload = updates
	return int64(len(ids)), nil
}

func (r *openAIAccountTestRepo) SetRateLimited(_ context.Context, id int64, resetAt time.Time) error {
	r.rateLimitedID = id
	r.rateLimitedAt = &resetAt
	return nil
}

func (r *openAIAccountTestRepo) SetTempUnschedulable(_ context.Context, id int64, until time.Time, reason string) error {
	r.tempUnschedID = id
	r.tempUnschedAt = &until
	r.tempUnschedReason = reason
	return nil
}

func (r *openAIAccountTestRepo) ClearError(_ context.Context, id int64) error {
	r.clearedErrorID = id
	return nil
}

func (r *openAIAccountTestRepo) SetError(_ context.Context, id int64, errorMsg string) error {
	r.setErrorID = id
	r.setErrorMsg = errorMsg
	return nil
}

func TestAccountTestService_OpenAISuccessPersistsSnapshotFromHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()

	resp := newJSONResponse(http.StatusOK, "")
	resp.Body = io.NopCloser(strings.NewReader(`data: {"type":"response.completed"}

`))
	resp.Header.Set("x-codex-primary-used-percent", "88")
	resp.Header.Set("x-codex-primary-reset-after-seconds", "604800")
	resp.Header.Set("x-codex-primary-window-minutes", "10080")
	resp.Header.Set("x-codex-secondary-used-percent", "42")
	resp.Header.Set("x-codex-secondary-reset-after-seconds", "18000")
	resp.Header.Set("x-codex-secondary-window-minutes", "300")

	repo := &openAIAccountTestRepo{}
	upstream := &queuedHTTPUpstream{responses: []*http.Response{resp}}
	svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream}
	account := &Account{
		ID:          89,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{"access_token": "test-token"},
	}

	err := svc.testOpenAIAccountConnection(ctx, account, "gpt-5.4", "", "")
	require.NoError(t, err)
	require.NotEmpty(t, repo.updatedExtra)
	require.Equal(t, 42.0, repo.updatedExtra["codex_5h_used_percent"])
	require.Equal(t, 88.0, repo.updatedExtra["codex_7d_used_percent"])
	require.Contains(t, recorder.Body.String(), "test_complete")
}

func TestAccountTestService_OpenAIResponsesStreamEOFAfterOutputCompletes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()

	resp := newJSONResponse(http.StatusOK, "")
	resp.Body = io.NopCloser(strings.NewReader(`data: {"type":"response.output_text.delta","delta":"hi"}

`))

	upstream := &queuedHTTPUpstream{responses: []*http.Response{resp}}
	svc := &AccountTestService{httpUpstream: upstream}
	account := &Account{
		ID:          90,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{"access_token": "test-token"},
	}

	err := svc.testOpenAIAccountConnection(ctx, account, "gpt-5.4", "", "")
	require.NoError(t, err)
	require.Contains(t, recorder.Body.String(), `"text":"hi"`)
	require.Contains(t, recorder.Body.String(), `"success":true`)
	require.NotContains(t, recorder.Body.String(), "Stream ended before response.completed")
}

func TestAccountTestService_OpenAIResponsesStreamDoneAfterOutputCompletes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()
	svc := &AccountTestService{}

	stream := strings.NewReader(`data: {"type":"response.output_text.delta","delta":"hi"}

data: [DONE]

`)
	err := svc.processOpenAIStream(ctx, stream)
	require.NoError(t, err)
	require.Contains(t, recorder.Body.String(), `"text":"hi"`)
	require.Contains(t, recorder.Body.String(), `"success":true`)
	require.NotContains(t, recorder.Body.String(), "Stream ended before response.completed")
}

func TestAccountTestService_OpenAIResponsesStreamTextDoneAfterOutputCompletes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()
	svc := &AccountTestService{}

	stream := strings.NewReader(`data: {"type":"response.output_text.done","text":"hi-done"}

`)
	err := svc.processOpenAIStream(ctx, stream)
	require.NoError(t, err)
	require.Contains(t, recorder.Body.String(), `"text":"hi-done"`)
	require.Contains(t, recorder.Body.String(), `"success":true`)
	require.NotContains(t, recorder.Body.String(), "Stream ended before response.completed")
}

func TestAccountTestService_OpenAIResponsesStreamItemDoneAfterOutputCompletes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()
	svc := &AccountTestService{}

	stream := strings.NewReader(`data: {"type":"response.output_item.done","item":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hi-item"}]}}

data: [DONE]

`)
	err := svc.processOpenAIStream(ctx, stream)
	require.NoError(t, err)
	require.Contains(t, recorder.Body.String(), `"text":"hi-item"`)
	require.Contains(t, recorder.Body.String(), `"success":true`)
	require.NotContains(t, recorder.Body.String(), "Stream ended before response.completed")
}

func TestAccountTestService_OpenAIResponsesStreamItemAddedAfterOutputCompletes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()
	svc := &AccountTestService{}

	stream := strings.NewReader(`data: {"type":"response.output_item.added","item":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hi-added"}]}}

data: [DONE]

`)
	err := svc.processOpenAIStream(ctx, stream)
	require.NoError(t, err)
	require.Contains(t, recorder.Body.String(), `"text":"hi-added"`)
	require.Contains(t, recorder.Body.String(), `"success":true`)
	require.NotContains(t, recorder.Body.String(), "Stream ended before response.completed")
}

func TestAccountTestService_OpenAIResponsesEmptyStreamStillFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()
	svc := &AccountTestService{}

	err := svc.processOpenAIStream(ctx, strings.NewReader(""))
	require.Error(t, err)
	require.Contains(t, recorder.Body.String(), "Stream ended before response.completed")
	require.NotContains(t, recorder.Body.String(), `"success":true`)
}

func TestAccountTestService_OpenAIResponsesStreamEmitsFirstTokenMs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()
	svc := &AccountTestService{}

	stream := strings.NewReader(`data: {"type":"response.output_text.delta","delta":"hi"}

data: {"type":"response.completed"}

`)
	err := svc.processOpenAIStream(ctx, stream)
	require.NoError(t, err)

	_, _, firstTokenMs := parseTestSSEOutput(recorder.Body.String())
	require.NotNil(t, firstTokenMs)
	require.GreaterOrEqual(t, *firstTokenMs, 0)
	require.Contains(t, recorder.Body.String(), `"first_token_ms"`)
}

func TestAccountTestService_OpenAIResponsesStreamBareJSONErrorReturnsUpstreamMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()
	svc := &AccountTestService{}

	stream := strings.NewReader(`{"error":{"message":"Only Codex clients can use this group (detected: bad ua)","type":"new_api_error","code":"access_denied"}}
`)
	err := svc.processOpenAIStream(ctx, stream)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Only Codex clients can use this group")
	require.Contains(t, recorder.Body.String(), "Only Codex clients can use this group")
	require.NotContains(t, recorder.Body.String(), "Stream ended before response.completed")
	require.NotContains(t, recorder.Body.String(), `"success":true`)
}

func TestAccountTestService_OpenAIResponsesNonSSEHTMLReturnsProtocolError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()
	svc := &AccountTestService{}

	stream := strings.NewReader(`<!DOCTYPE html>
<html><title>relay landing</title></html>
`)
	err := svc.processOpenAIStream(ctx, stream)
	require.Error(t, err)
	require.Contains(t, err.Error(), "non-SSE HTML response")
	require.Contains(t, recorder.Body.String(), "non-SSE HTML response")
	require.NotContains(t, recorder.Body.String(), "Stream ended before response.completed")
	require.NotContains(t, recorder.Body.String(), `"success":true`)
}

func TestAccountTestService_TestAccountConnectionWithResultReturnsLatencyAndFirstToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()
	account := &Account{
		ID:          1,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{"access_token": "test-token"},
	}
	repo := &openAIAccountTestRepo{
		mockAccountRepoForGemini: mockAccountRepoForGemini{
			accountsByID: map[int64]*Account{1: account},
		},
	}
	resp := newJSONResponse(http.StatusOK, strings.Join([]string{
		`data: {"type":"response.output_text.delta","delta":"hi"}`,
		``,
		`data: {"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`,
		``,
	}, "\n"))
	upstream := &queuedHTTPUpstream{responses: []*http.Response{resp}}
	svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream}

	result, err := svc.TestAccountConnectionWithResult(ctx, 1, "gpt-5.4", "", "")

	require.NoError(t, err)
	require.True(t, result.Success)
	require.NotNil(t, result.LatencyMs)
	require.GreaterOrEqual(t, *result.LatencyMs, 0)
	require.NotNil(t, result.FirstTokenMs)
	require.GreaterOrEqual(t, *result.FirstTokenMs, 0)
	require.Contains(t, recorder.Body.String(), `"latency_ms"`)
	require.Contains(t, recorder.Body.String(), `"first_token_ms"`)
}

func TestAccountTestService_TestAccountConnectionWithResultClassifiesFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()
	account := &Account{
		ID:          1,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{"access_token": "test-token"},
	}
	repo := &openAIAccountTestRepo{
		mockAccountRepoForGemini: mockAccountRepoForGemini{
			accountsByID: map[int64]*Account{1: account},
		},
	}
	resp := newJSONResponse(http.StatusPaymentRequired, `{"error":{"message":"insufficient balance"}}`)
	upstream := &queuedHTTPUpstream{responses: []*http.Response{resp}}
	svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream}

	result, err := svc.TestAccountConnectionWithResult(ctx, 1, "gpt-5.4", "", "")

	require.Error(t, err)
	require.False(t, result.Success)
	require.Equal(t, http.StatusPaymentRequired, result.HTTPStatus)
	require.Equal(t, "payment_required", result.Reason)
	require.NotNil(t, result.LatencyMs)
	require.Contains(t, recorder.Body.String(), `"latency_ms"`)
}

func TestAccountTestService_OpenAIChatCompletionsStreamEmitsFirstTokenMs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()
	svc := &AccountTestService{}

	stream := strings.NewReader(`data: {"choices":[{"delta":{"content":"hi"}}]}

data: [DONE]

`)
	err := svc.processOpenAIChatCompletionsStream(ctx, stream)
	require.NoError(t, err)

	_, _, firstTokenMs := parseTestSSEOutput(recorder.Body.String())
	require.NotNil(t, firstTokenMs)
	require.GreaterOrEqual(t, *firstTokenMs, 0)
	require.Contains(t, recorder.Body.String(), `"first_token_ms"`)
}

func TestAccountTestService_OpenAIResponseTextErrorInterceptsProbe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"openai_response_text_error_enabled":  true,
			"openai_response_text_error_keywords": []any{"join the new community"},
		},
	}

	t.Run("ChatCompletions命中关键词转失败", func(t *testing.T) {
		ctx, recorder := newTestContext()
		svc := &AccountTestService{}
		stream := strings.NewReader("data: {\"choices\":[{\"delta\":{\"content\":\"Welcome to join the new community!\"}}]}\n\ndata: [DONE]\n\n")
		err := svc.processOpenAIChatCompletionsStreamWithStart(ctx, stream, time.Now(), newOpenAIResponseTextErrorDetector(account))
		require.Error(t, err)
		require.NotContains(t, recorder.Body.String(), `"success":true`)
	})

	t.Run("Responses命中关键词转失败", func(t *testing.T) {
		ctx, recorder := newTestContext()
		svc := &AccountTestService{}
		stream := strings.NewReader("data: {\"type\":\"response.output_text.delta\",\"delta\":\"join the new community now\"}\n\ndata: {\"type\":\"response.completed\"}\n\n")
		err := svc.processOpenAIStreamWithStart(ctx, stream, time.Now(), newOpenAIResponseTextErrorDetector(account))
		require.Error(t, err)
		require.NotContains(t, recorder.Body.String(), `"success":true`)
	})

	t.Run("未配置关键词不误杀", func(t *testing.T) {
		ctx, recorder := newTestContext()
		svc := &AccountTestService{}
		plain := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
		stream := strings.NewReader("data: {\"choices\":[{\"delta\":{\"content\":\"join the new community\"}}]}\n\ndata: [DONE]\n\n")
		err := svc.processOpenAIChatCompletionsStreamWithStart(ctx, stream, time.Now(), newOpenAIResponseTextErrorDetector(plain))
		require.NoError(t, err)
		require.Contains(t, recorder.Body.String(), `"success":true`)
	})
}

func TestAccountTestService_TestAccountConnectionWithResultAppliesOpenAIResponseTextErrorKeywords(t *testing.T) {
	gin.SetMode(gin.TestMode)

	matchedBody := strings.Join([]string{
		`data: {"type":"response.output_text.delta","delta":"join the new community"}`,
		"",
		`data: {"type":"response.completed"}`,
		"",
	}, "\n")

	t.Run("开启关键词时命中正文转为探测失败", func(t *testing.T) {
		ctx, _ := newTestContext()
		account := &Account{
			ID:          1,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Concurrency: 1,
			Credentials: map[string]any{
				"api_key":                             "key-with-rule",
				"base_url":                            "https://example.com",
				"openai_response_text_error_enabled":  true,
				"openai_response_text_error_keywords": []any{"join the new community"},
			},
			Extra: map[string]any{"openai_responses_supported": true},
		}
		repo := &openAIAccountTestRepo{
			mockAccountRepoForGemini: mockAccountRepoForGemini{
				accountsByID: map[int64]*Account{account.ID: account},
			},
		}
		upstream := &queuedHTTPUpstream{responses: []*http.Response{newJSONResponse(http.StatusOK, matchedBody)}}
		svc := &AccountTestService{
			accountRepo:  repo,
			httpUpstream: upstream,
			cfg: &config.Config{
				Security: config.SecurityConfig{
					URLAllowlist: config.URLAllowlistConfig{Enabled: false},
				},
			},
		}

		result, err := svc.TestAccountConnectionWithResult(ctx, account.ID, "gpt-5.4", "", "")

		require.Error(t, err)
		require.False(t, result.Success)
		require.Equal(t, "upstream_abnormal", result.Reason)
		require.NotEqual(t, "probe_success", result.Reason)
		require.Contains(t, result.ErrorMessage, "join the new community")
	})

	t.Run("未开启关键词时同样正文仍然探测成功", func(t *testing.T) {
		ctx, _ := newTestContext()
		account := &Account{
			ID:          2,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Concurrency: 1,
			Credentials: map[string]any{
				"api_key":  "key-without-rule",
				"base_url": "https://example.com",
			},
			Extra: map[string]any{"openai_responses_supported": true},
		}
		repo := &openAIAccountTestRepo{
			mockAccountRepoForGemini: mockAccountRepoForGemini{
				accountsByID: map[int64]*Account{account.ID: account},
			},
		}
		upstream := &queuedHTTPUpstream{responses: []*http.Response{newJSONResponse(http.StatusOK, matchedBody)}}
		svc := &AccountTestService{
			accountRepo:  repo,
			httpUpstream: upstream,
			cfg: &config.Config{
				Security: config.SecurityConfig{
					URLAllowlist: config.URLAllowlistConfig{Enabled: false},
				},
			},
		}

		result, err := svc.TestAccountConnectionWithResult(ctx, account.ID, "gpt-5.4", "", "")

		require.NoError(t, err)
		require.True(t, result.Success)
		require.Equal(t, "probe_success", result.Reason)
	})
}

func TestAccountTestService_OpenAI429PersistsSnapshotAndRateLimitState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := newTestContext()

	resp := newJSONResponse(http.StatusTooManyRequests, `{"error":{"type":"usage_limit_reached","message":"limit reached","resets_at":1777283883}}`)
	resp.Header.Set("x-codex-primary-used-percent", "100")
	resp.Header.Set("x-codex-primary-reset-after-seconds", "604800")
	resp.Header.Set("x-codex-primary-window-minutes", "10080")
	resp.Header.Set("x-codex-secondary-used-percent", "100")
	resp.Header.Set("x-codex-secondary-reset-after-seconds", "18000")
	resp.Header.Set("x-codex-secondary-window-minutes", "300")

	repo := &openAIAccountTestRepo{}
	upstream := &queuedHTTPUpstream{responses: []*http.Response{resp}}
	svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream}
	account := &Account{
		ID:          88,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusError,
		Concurrency: 1,
		Credentials: map[string]any{"access_token": "test-token"},
	}

	err := svc.testOpenAIAccountConnection(ctx, account, "gpt-5.4", "", "")
	require.Error(t, err)
	require.NotEmpty(t, repo.updatedExtra)
	require.Equal(t, 100.0, repo.updatedExtra["codex_5h_used_percent"])
	require.Equal(t, account.ID, repo.rateLimitedID)
	require.NotNil(t, repo.rateLimitedAt)
	require.Equal(t, account.ID, repo.clearedErrorID)
	require.Equal(t, StatusActive, account.Status)
	require.Empty(t, account.ErrorMessage)
	require.NotNil(t, account.RateLimitResetAt)
}

func TestAccountTestService_OpenAI429BodyOnlyPersistsRateLimitAndClearsStaleError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := newTestContext()

	resp := newJSONResponse(http.StatusTooManyRequests, `{"error":{"type":"usage_limit_reached","message":"limit reached","resets_at":"1777283883"}}`)

	repo := &openAIAccountTestRepo{}
	upstream := &queuedHTTPUpstream{responses: []*http.Response{resp}}
	svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream}
	account := &Account{
		ID:           77,
		Platform:     PlatformOpenAI,
		Type:         AccountTypeOAuth,
		Status:       StatusError,
		ErrorMessage: "Access forbidden (403): account may be suspended or lack permissions",
		Concurrency:  1,
		Credentials:  map[string]any{"access_token": "test-token"},
	}

	err := svc.testOpenAIAccountConnection(ctx, account, "gpt-5.4", "", "")
	require.Error(t, err)
	require.Equal(t, account.ID, repo.rateLimitedID)
	require.NotNil(t, repo.rateLimitedAt)
	require.Equal(t, account.ID, repo.clearedErrorID)
	require.Equal(t, StatusActive, account.Status)
	require.Empty(t, account.ErrorMessage)
	require.NotNil(t, account.RateLimitResetAt)
	require.Empty(t, repo.updatedExtra)
}

func TestAccountTestService_OpenAI429SyncsObservedPlanType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := newTestContext()

	resp := newJSONResponse(http.StatusTooManyRequests, `{"error":{"type":"usage_limit_reached","message":"limit reached","plan_type":"free","resets_at":1777283883}}`)

	repo := &openAIAccountTestRepo{}
	upstream := &queuedHTTPUpstream{responses: []*http.Response{resp}}
	svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream}
	account := &Account{
		ID:          81,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Concurrency: 1,
		Credentials: map[string]any{"access_token": "test-token", "plan_type": "plus"},
	}

	err := svc.testOpenAIAccountConnection(ctx, account, "gpt-5.4", "", "")
	require.Error(t, err)
	require.Equal(t, []int64{account.ID}, repo.bulkUpdatedIDs)
	require.Equal(t, "free", repo.bulkUpdatedPayload.Credentials["plan_type"])
	require.Equal(t, "free", account.Credentials["plan_type"])
	require.Equal(t, account.ID, repo.rateLimitedID)
	require.NotNil(t, account.RateLimitResetAt)
}

func TestAccountTestService_OpenAI429ActiveAccountDoesNotClearError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := newTestContext()

	resp := newJSONResponse(http.StatusTooManyRequests, `{"error":{"type":"usage_limit_reached","message":"limit reached","resets_in_seconds":3600}}`)

	repo := &openAIAccountTestRepo{}
	upstream := &queuedHTTPUpstream{responses: []*http.Response{resp}}
	svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream}
	account := &Account{
		ID:          78,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Concurrency: 1,
		Credentials: map[string]any{"access_token": "test-token"},
	}

	err := svc.testOpenAIAccountConnection(ctx, account, "gpt-5.4", "", "")
	require.Error(t, err)
	require.Equal(t, account.ID, repo.rateLimitedID)
	require.NotNil(t, repo.rateLimitedAt)
	require.Zero(t, repo.clearedErrorID)
	require.Equal(t, StatusActive, account.Status)
	require.NotNil(t, account.RateLimitResetAt)
}

func TestAccountTestService_OpenAI429WithoutResetSignalDoesNotMutateRuntimeState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := newTestContext()

	resp := newJSONResponse(http.StatusTooManyRequests, `{"error":{"type":"usage_limit_reached","message":"limit reached"}}`)

	repo := &openAIAccountTestRepo{}
	upstream := &queuedHTTPUpstream{responses: []*http.Response{resp}}
	svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream}
	account := &Account{
		ID:           79,
		Platform:     PlatformOpenAI,
		Type:         AccountTypeOAuth,
		Status:       StatusError,
		ErrorMessage: "stale 403",
		Concurrency:  1,
		Credentials:  map[string]any{"access_token": "test-token"},
	}

	err := svc.testOpenAIAccountConnection(ctx, account, "gpt-5.4", "", "")
	require.Error(t, err)
	require.Zero(t, repo.rateLimitedID)
	require.Nil(t, repo.rateLimitedAt)
	require.Zero(t, repo.clearedErrorID)
	require.Equal(t, StatusError, account.Status)
	require.Equal(t, "stale 403", account.ErrorMessage)
	require.Nil(t, account.RateLimitResetAt)
}

func TestAccountTestService_OpenAI401SetsPermanentErrorOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := newTestContext()

	resp := newJSONResponse(http.StatusUnauthorized, `{"error":"bad token"}`)

	repo := &openAIAccountTestRepo{}
	upstream := &queuedHTTPUpstream{responses: []*http.Response{resp}}
	svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream}
	account := &Account{
		ID:          80,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Concurrency: 1,
		Credentials: map[string]any{"access_token": "test-token"},
	}

	err := svc.testOpenAIAccountConnection(ctx, account, "gpt-5.4", "", "")
	require.Error(t, err)
	require.Equal(t, account.ID, repo.setErrorID)
	require.Contains(t, repo.setErrorMsg, "Authentication failed (401)")
	require.Zero(t, repo.rateLimitedID)
	require.Zero(t, repo.clearedErrorID)
	require.Nil(t, account.RateLimitResetAt)
}

func TestAccountTestService_OpenAIAPIKeyInsufficientBalanceDisablesSelectedKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()

	resp1 := newJSONResponse(http.StatusForbidden, `{"code":"INSUFFICIENT_BALANCE","message":"Insufficient account balance"}`)
	repo := &openAIAccountTestRepo{}
	upstream := &queuedHTTPUpstream{responses: []*http.Response{resp1}}
	svc := &AccountTestService{
		accountRepo:  repo,
		httpUpstream: upstream,
		cfg: &config.Config{
			Security: config.SecurityConfig{
				URLAllowlist: config.URLAllowlistConfig{Enabled: false},
			},
		},
	}
	account := &Account{
		ID:          90,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_keys": []any{"key-empty", "key-ok"},
			"base_url": "https://example.com",
		},
		Extra: map[string]any{"openai_responses_supported": true},
	}

	err := svc.testOpenAIAccountConnection(ctx, account, "gpt-5.4", "", "")
	require.Error(t, err)
	require.Contains(t, recorder.Body.String(), "API returned 403")
	require.NotNil(t, repo.updatedCredentials)
	disabled, _ := repo.updatedCredentials[CredentialAPIKeysDisabled].(map[string]any)
	require.Contains(t, disabled, FingerprintAPIKey("key-empty"))
	require.Zero(t, repo.tempUnschedID)
	require.Equal(t, []string{"key-ok"}, account.GetAPIKeys())
	require.Zero(t, repo.setErrorID)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "Bearer key-empty", upstream.requests[0].Header.Get("Authorization"))
}

func TestAccountTestService_OpenAIAPIKeyTriesNextRequestBaseURLOnTransientError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()

	resp := newJSONResponse(http.StatusOK, "")
	resp.Body = io.NopCloser(strings.NewReader(`data: {"type":"response.completed"}

`))
	upstream := &queuedHTTPUpstream{
		errs:      []error{fmt.Errorf("Post \"https://bad.example.com/v1/responses\": unexpected EOF"), nil},
		responses: []*http.Response{resp},
	}
	svc := &AccountTestService{
		httpUpstream: upstream,
		cfg: &config.Config{
			Security: config.SecurityConfig{
				URLAllowlist: config.URLAllowlistConfig{Enabled: false},
			},
		},
	}
	account := &Account{
		ID:          91,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":           "key-ok",
			"base_url":          "https://bad.example.com/v1",
			"request_base_urls": []string{"https://bad.example.com/v1", "https://good.example.com/v1"},
		},
		Extra: map[string]any{"openai_responses_supported": true},
	}

	err := svc.testOpenAIAccountConnection(ctx, account, "gpt-5.4", "", "")
	require.NoError(t, err)
	require.Contains(t, recorder.Body.String(), "test_complete")
	require.Len(t, upstream.requests, 2)
	require.Equal(t, "https://bad.example.com/v1/responses", upstream.requests[0].URL.String())
	require.Equal(t, "https://good.example.com/v1/responses", upstream.requests[1].URL.String())
}

func TestAccountTestService_OpenAIAPIKeyResponsesTestUsesGatewayCodexSimulationHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()
	ctx.Request.Header.Set("User-Agent", "curl/8.0")
	ctx.Request.Header.Set("originator", "opencode")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			`data: {"type":"response.output_text.delta","delta":"pong"}`,
			"",
			`data: {"type":"response.completed"}`,
			"",
		}, "\n"))),
	}}
	svc := &AccountTestService{
		httpUpstream: upstream,
		cfg:          &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
	}
	account := &Account{
		ID:          416,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-free5",
			"base_url": "https://new.sharedchat.cc/codex",
		},
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesSupported: true,
			OpenAICodexCLISimulationEnabledExtraKey:  true,
		},
	}

	err := svc.testOpenAIAccountConnection(ctx, account, "gpt-5.5", "ping", "")
	require.NoError(t, err)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "https://new.sharedchat.cc/codex/responses", upstream.lastReq.URL.String())
	require.NotNil(t, upstream.lastTLSProfile)
	require.Equal(t, builtInDefaultTLSFingerprintProfileName, upstream.lastTLSProfile.Name)
	require.Equal(t, "Bearer sk-free5", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, "application/json", upstream.lastReq.Header.Get("Content-Type"))
	require.Equal(t, "text/event-stream", upstream.lastReq.Header.Get("Accept"))
	require.Equal(t, codexCLIUserAgent(), upstream.lastReq.Header.Get("User-Agent"))
	require.Equal(t, "codex_cli_rs", upstream.lastReq.Header.Get("originator"))
	require.Equal(t, "responses=experimental", upstream.lastReq.Header.Get("OpenAI-Beta"))
	require.Equal(t, codexCLIVersion(), upstream.lastReq.Header.Get("version"))
	require.Equal(t, "compact-history", upstream.lastReq.Header.Get("X-Codex-Beta-Features"))
	require.NotEmpty(t, upstream.lastReq.Header.Get("X-Client-Request-Id"))
	require.NotEmpty(t, upstream.lastReq.Header.Get("Session-Id"))
	require.NotEmpty(t, upstream.lastReq.Header.Get("Thread-Id"))
	require.NotEmpty(t, upstream.lastReq.Header.Get("X-Codex-Window-Id"))
	require.NotEmpty(t, upstream.lastReq.Header.Get("X-Codex-Turn-Metadata"))
	require.NotEmpty(t, upstream.lastReq.Header.Get("X-Codex-Installation-Id"))
	sessionID := gjson.GetBytes(upstream.lastBody, "client_metadata.session_id").String()
	threadID := gjson.GetBytes(upstream.lastBody, "client_metadata.thread_id").String()
	turnID := gjson.GetBytes(upstream.lastBody, "client_metadata.turn_id").String()
	windowID := gjson.GetBytes(upstream.lastBody, "client_metadata.x-codex-window-id").String()
	installationID := gjson.GetBytes(upstream.lastBody, "client_metadata.x-codex-installation-id").String()
	turnMetadata := gjson.GetBytes(upstream.lastBody, "client_metadata.x-codex-turn-metadata").String()
	require.Equal(t, codexCLIOriginator, gjson.GetBytes(upstream.lastBody, "client_metadata.originator").String())
	require.Equal(t, "codex", gjson.GetBytes(upstream.lastBody, "client_metadata.x-openai-client-source").String())
	require.NotEmpty(t, sessionID)
	require.Equal(t, sessionID, threadID)
	require.NotEmpty(t, turnID)
	require.Equal(t, sessionID+":0", windowID)
	require.Equal(t, resolveCodexSimulationInstallationID(account), installationID)
	require.Equal(t, sessionID, upstream.lastReq.Header.Get("Session-Id"))
	require.Equal(t, threadID, upstream.lastReq.Header.Get("Thread-Id"))
	require.Equal(t, windowID, upstream.lastReq.Header.Get("X-Codex-Window-Id"))
	require.Equal(t, installationID, upstream.lastReq.Header.Get("X-Codex-Installation-Id"))
	require.Equal(t, turnMetadata, upstream.lastReq.Header.Get("X-Codex-Turn-Metadata"))
	require.Equal(t, sessionID, gjson.Get(turnMetadata, "session_id").String())
	require.Equal(t, threadID, gjson.Get(turnMetadata, "thread_id").String())
	require.Equal(t, turnID, gjson.Get(turnMetadata, "turn_id").String())
	require.Equal(t, windowID, gjson.Get(turnMetadata, "window_id").String())
	require.Equal(t, installationID, gjson.Get(turnMetadata, "installation_id").String())
	require.False(t, gjson.GetBytes(upstream.lastBody, "store").Bool())
	require.NotEmpty(t, gjson.GetBytes(upstream.lastBody, "prompt_cache_key").String())
	require.Equal(t, "low", gjson.GetBytes(upstream.lastBody, "reasoning.effort").String())
	require.Equal(t, "low", gjson.GetBytes(upstream.lastBody, "text.verbosity").String())
	require.Equal(t, "reasoning.encrypted_content", gjson.GetBytes(upstream.lastBody, "include.0").String())
	require.Contains(t, recorder.Body.String(), `"type":"test_complete"`)
}

func TestAccountTestService_OpenAIAPIKeyResponsesUnsupportedUsesChatCompletionsPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()

	upstreamBody := strings.Join([]string{
		`data: {"id":"chatcmpl_test","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"pong"},"finish_reason":null}]}`,
		"",
		`data: {"id":"chatcmpl_test","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}
	svc := &AccountTestService{
		httpUpstream: upstream,
		cfg:          &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
	}
	account := &Account{
		ID:          91,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://compat-upstream.example/v1",
		},
		Extra: map[string]any{openai_compat.ExtraKeyResponsesSupported: false},
	}

	err := svc.testOpenAIAccountConnection(ctx, account, "gpt-5.4", "hello", "")
	require.NoError(t, err)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "https://compat-upstream.example/v1/chat/completions", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer sk-test", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, "text/event-stream", upstream.lastReq.Header.Get("Accept"))
	require.Equal(t, "gpt-5.4", gjson.GetBytes(upstream.lastBody, "model").String())
	require.True(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.Equal(t, "hello", gjson.GetBytes(upstream.lastBody, "messages.0.content").String())
	require.False(t, gjson.GetBytes(upstream.lastBody, "input").Exists())
	body := recorder.Body.String()
	require.Contains(t, body, "pong")
	require.Contains(t, body, "已通过 /v1/chat/completions 验证")
	require.Contains(t, body, `"success":true`)
	require.NotContains(t, body, "当前测试接口仅支持 Responses API 路径")
}

func TestAccountTestService_OpenAIAPIKeyChatCompletionsTestUsesGatewayCodexSimulationHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()
	ctx.Request.Header.Set("User-Agent", "curl/8.0")
	ctx.Request.Header.Set("originator", "opencode")

	upstreamBody := strings.Join([]string{
		`data: {"id":"chatcmpl_codex","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"pong"},"finish_reason":null}]}`,
		"",
		`data: {"id":"chatcmpl_codex","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}
	svc := &AccountTestService{
		httpUpstream: upstream,
		cfg:          &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
	}
	account := &Account{
		ID:          470,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-sharedchat",
			"base_url": "https://new.sharedchat.cc/codex/v1",
		},
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesSupported: false,
			OpenAICodexCLISimulationEnabledExtraKey:  true,
		},
	}

	err := svc.testOpenAIAccountConnection(ctx, account, "gpt-5.5", "ping", "")
	require.NoError(t, err)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "https://new.sharedchat.cc/codex/v1/chat/completions", upstream.lastReq.URL.String())
	require.Equal(t, codexCLIUserAgent(), upstream.lastReq.Header.Get("User-Agent"))
	require.Equal(t, "codex_cli_rs", upstream.lastReq.Header.Get("originator"))
	require.Equal(t, "responses=experimental", upstream.lastReq.Header.Get("OpenAI-Beta"))
	require.Equal(t, codexCLIVersion(), upstream.lastReq.Header.Get("version"))
	require.Equal(t, "compact-history", upstream.lastReq.Header.Get("X-Codex-Beta-Features"))
	require.NotEmpty(t, upstream.lastReq.Header.Get("X-Client-Request-Id"))
	require.NotEmpty(t, upstream.lastReq.Header.Get("Session-Id"))
	require.NotEmpty(t, upstream.lastReq.Header.Get("Thread-Id"))
	require.NotEmpty(t, upstream.lastReq.Header.Get("X-Codex-Window-Id"))
	require.NotEmpty(t, upstream.lastReq.Header.Get("X-Codex-Turn-Metadata"))
	require.NotEmpty(t, upstream.lastReq.Header.Get("X-Codex-Installation-Id"))
	require.Contains(t, recorder.Body.String(), `"success":true`)
}

func TestAccountTestService_OpenAIChatCompletionsPathReturns4xx(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()

	upstream := &httpUpstreamRecorder{resp: newJSONResponse(http.StatusBadRequest, `{"error":{"message":"bad request"}}`)}
	svc := &AccountTestService{
		httpUpstream: upstream,
		cfg:          &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
	}
	account := &Account{
		ID:          92,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://compat-upstream.example",
		},
		Extra: map[string]any{openai_compat.ExtraKeyResponsesSupported: false},
	}

	err := svc.testOpenAIAccountConnection(ctx, account, "gpt-5.4", "", "")
	require.Error(t, err)
	require.Equal(t, "https://compat-upstream.example/v1/chat/completions", upstream.lastReq.URL.String())
	require.Contains(t, err.Error(), "Chat Completions API (/v1/chat/completions) returned 400")
	require.Contains(t, recorder.Body.String(), "/v1/chat/completions")
	require.NotContains(t, recorder.Body.String(), `"success":true`)
}

func TestAccountTestService_OpenAIChatCompletionsPathTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()

	upstream := &httpUpstreamRecorder{err: context.DeadlineExceeded}
	svc := &AccountTestService{
		httpUpstream: upstream,
		cfg:          &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
	}
	account := &Account{
		ID:          93,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://compat-upstream.example",
		},
		Extra: map[string]any{openai_compat.ExtraKeyResponsesSupported: false},
	}

	err := svc.testOpenAIAccountConnection(ctx, account, "gpt-5.4", "", "")
	require.Error(t, err)
	require.Equal(t, "https://compat-upstream.example/v1/chat/completions", upstream.lastReq.URL.String())
	require.Contains(t, err.Error(), "Chat Completions API (/v1/chat/completions) request failed")
	require.Contains(t, err.Error(), context.DeadlineExceeded.Error())
	require.Contains(t, recorder.Body.String(), "/v1/chat/completions")
	require.NotContains(t, recorder.Body.String(), `"success":true`)
}

func TestAccountTestService_OpenAIChatCompletionsPathRejectsNonJSONStream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader("data: not-json\n\n")),
	}}
	svc := &AccountTestService{
		httpUpstream: upstream,
		cfg:          &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
	}
	account := &Account{
		ID:          94,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://compat-upstream.example",
		},
		Extra: map[string]any{openai_compat.ExtraKeyResponsesSupported: false},
	}

	err := svc.testOpenAIAccountConnection(ctx, account, "gpt-5.4", "", "")
	require.Error(t, err)
	require.Equal(t, "https://compat-upstream.example/v1/chat/completions", upstream.lastReq.URL.String())
	require.Contains(t, err.Error(), "Invalid Chat Completions response from /v1/chat/completions")
	require.Contains(t, recorder.Body.String(), "/v1/chat/completions")
	require.NotContains(t, recorder.Body.String(), `"success":true`)
}

func TestAccountTestService_OpenAIChatCompletionsPathDisablesSelectedKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := newTestContext()

	repo := &openAIAccountTestRepo{}
	upstream := &queuedHTTPUpstream{responses: []*http.Response{
		newJSONResponse(http.StatusUnauthorized, `{"error":{"message":"invalid api key"}}`),
	}}
	svc := &AccountTestService{
		accountRepo:  repo,
		httpUpstream: upstream,
		cfg:          &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
	}
	account := &Account{
		ID:          95,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_keys": []any{"key-chat-bad", "key-chat-ok"},
			"base_url": "https://compat-upstream.example",
		},
		Extra: map[string]any{openai_compat.ExtraKeyResponsesSupported: false},
	}

	err := svc.testOpenAIAccountConnection(ctx, account, "gpt-5.4", "", "")
	require.Error(t, err)
	require.Zero(t, repo.setErrorID)
	require.NotNil(t, repo.updatedCredentials)
	disabled, _ := repo.updatedCredentials[CredentialAPIKeysDisabled].(map[string]any)
	require.Contains(t, disabled, FingerprintAPIKey("key-chat-bad"))
	require.Zero(t, repo.tempUnschedID)
	require.Equal(t, []string{"key-chat-ok"}, account.GetAPIKeys())
}

func TestAccountTestService_OpenAICompactPathDisablesSelectedKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := newTestContext()

	repo := &openAIAccountTestRepo{}
	upstream := &queuedHTTPUpstream{responses: []*http.Response{
		newJSONResponse(http.StatusTooManyRequests, `{"error":{"message":"slow down"}}`),
	}}
	svc := &AccountTestService{
		accountRepo:  repo,
		httpUpstream: upstream,
		cfg:          &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
	}
	account := &Account{
		ID:          96,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_keys": []any{"key-compact-bad", "key-compact-ok"},
			"base_url": "https://compat-upstream.example",
		},
	}

	err := svc.testOpenAICompactConnection(ctx, account, "gpt-5.4")
	require.Error(t, err)
	require.Zero(t, repo.rateLimitedID)
	require.Zero(t, repo.setErrorID)
	require.NotNil(t, repo.updatedCredentials)
	disabled, _ := repo.updatedCredentials[CredentialAPIKeysDisabled].(map[string]any)
	require.Contains(t, disabled, FingerprintAPIKey("key-compact-bad"))
	require.Zero(t, repo.tempUnschedID)
	require.Equal(t, []string{"key-compact-ok"}, account.GetAPIKeys())
}

func TestAccountTestService_OpenAIImagePathDisablesSelectedKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := newTestContext()

	repo := &openAIAccountTestRepo{}
	upstream := &queuedHTTPUpstream{responses: []*http.Response{
		newJSONResponse(http.StatusBadRequest, `{"error":{"code":"insufficient_quota","message":"insufficient balance"}}`),
	}}
	svc := &AccountTestService{
		accountRepo:  repo,
		httpUpstream: upstream,
		cfg:          &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
	}
	account := &Account{
		ID:          97,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_keys": []any{"key-image-bad", "key-image-ok"},
			"base_url": "https://compat-upstream.example",
		},
	}

	err := svc.testOpenAIImageAPIKey(ctx, context.Background(), account, "gpt-image-1", "test image")
	require.Error(t, err)
	require.Zero(t, repo.setErrorID)
	require.NotNil(t, repo.updatedCredentials)
	disabled, _ := repo.updatedCredentials[CredentialAPIKeysDisabled].(map[string]any)
	require.Contains(t, disabled, FingerprintAPIKey("key-image-bad"))
	require.Zero(t, repo.tempUnschedID)
	require.Equal(t, []string{"key-image-ok"}, account.GetAPIKeys())
}
