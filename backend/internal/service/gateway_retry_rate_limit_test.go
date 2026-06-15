//go:build unit

package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRetryBackoffDelay_AbnormalAccountRequestsAreSpacedAtThirtySeconds(t *testing.T) {
	for _, attempt := range []int{0, 1, 2, 3, 4} {
		t.Run(fmt.Sprintf("attempt_%d", attempt), func(t *testing.T) {
			require.Equal(t, 30*time.Second, retryBackoffDelay(attempt))
		})
	}
}

func TestRetryBudget_AllowsOnlyOneRetryAfterInitialRequest(t *testing.T) {
	require.Equal(t, 2, maxRetryAttempts)
	require.GreaterOrEqual(t, maxRetryElapsed, 60*time.Second)
}

func TestGatewayService_AnthropicRetryExhaustionDoesNotDelegateSameAccountRetry(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	body := []byte(`{"model":"claude-sonnet-4-20250514","stream":false,"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`)
	parsed := &ParsedRequest{
		Body:   NewRequestBodyRef(body),
		Model:  "claude-sonnet-4-20250514",
		Stream: false,
	}

	upstream := &anthropicHTTPUpstreamRecorder{
		responses: []*http.Response{
			{
				StatusCode: http.StatusBadGateway,
				Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid-first"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"first failure"},"type":"error"}`)),
			},
			{
				StatusCode: http.StatusBadGateway,
				Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid-second"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"second failure"},"type":"error"}`)),
			},
		},
	}
	svc := &GatewayService{
		cfg:                 &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}},
		httpUpstream:        upstream,
		rateLimitService:    NewRateLimitService(&errorPolicyRepoStub{}, nil, &config.Config{}, nil, nil),
		tlsFPProfileService: &TLSFingerprintProfileService{},
	}
	account := &Account{
		ID:          701,
		Name:        "anthropic-pool-retry-budget",
		Platform:    PlatformAnthropic,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":                      "sk-anthropic-test",
			"base_url":                     "https://api.anthropic.com",
			"pool_mode":                    true,
			"pool_mode_retry_status_codes": []any{float64(http.StatusBadGateway)},
			"custom_error_codes_enabled":   true,
			"custom_error_codes":           []any{float64(http.StatusUnauthorized)},
		},
		Status:      StatusActive,
		Schedulable: true,
	}

	result, err := svc.Forward(context.Background(), c, account, parsed)

	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.True(t, errors.As(err, &failoverErr))
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.False(t, failoverErr.RetryableOnSameAccount, "服务层已经按 30 秒间隔请求两次，不能再让 handler 对同一账号发起第三次探测")
	require.Len(t, upstream.requests, 2)
}

func TestOpenAIWSRetryBackoff_AbnormalAccountRequestsAreSpacedAtThirtySeconds(t *testing.T) {
	t.Run("default backoff", func(t *testing.T) {
		svc := &OpenAIGatewayService{}

		require.Equal(t, 30*time.Second, svc.openAIWSRetryBackoff(1))
		require.Equal(t, 1, openAIWSReconnectRetryLimit)
		require.Zero(t, svc.openAIWSRetryTotalBudget())
	})

	t.Run("configured smaller values cannot bypass lower bound", func(t *testing.T) {
		svc := &OpenAIGatewayService{cfg: &config.Config{
			Gateway: config.GatewayConfig{
				OpenAIWS: config.GatewayOpenAIWSConfig{
					RetryBackoffInitialMS: 120,
					RetryBackoffMaxMS:     2000,
					RetryJitterRatio:      0.2,
					RetryTotalBudgetMS:    5000,
				},
			},
		}}

		require.GreaterOrEqual(t, svc.openAIWSRetryBackoff(1), 30*time.Second)
		require.Zero(t, svc.openAIWSRetryTotalBudget())
	})
}

func TestGatewayService_TempUnscheduleRetryableErrorServerErrorsUseThirtySecondCooldown(t *testing.T) {
	repo := &serverErrorTempUnscheduleRepo{}
	svc := &GatewayService{accountRepo: repo}

	start := time.Now()
	svc.TempUnscheduleRetryableError(nil, 123, &UpstreamFailoverError{StatusCode: http.StatusInternalServerError})

	require.Len(t, repo.tempCalls, 1)
	require.Equal(t, int64(123), repo.tempCalls[0].accountID)
	require.WithinDuration(t, start.Add(30*time.Second), repo.tempCalls[0].until, 2*time.Second)
	require.Contains(t, repo.tempCalls[0].reason, "500: server error")
}

func TestGatewayService_TempUnscheduleRetryableErrorOverloadUsesThirtySecondCooldown(t *testing.T) {
	repo := &serverErrorTempUnscheduleRepo{}
	svc := &GatewayService{accountRepo: repo}

	start := time.Now()
	svc.TempUnscheduleRetryableError(nil, 456, &UpstreamFailoverError{StatusCode: 529})

	require.Len(t, repo.overloadCalls, 1)
	require.Equal(t, int64(456), repo.overloadCalls[0].accountID)
	require.WithinDuration(t, start.Add(30*time.Second), repo.overloadCalls[0].until, 2*time.Second)
	require.Empty(t, repo.tempCalls)
}

type serverErrorTempUnscheduleRepo struct {
	AccountRepository
	tempCalls     []serverErrorTempCall
	overloadCalls []serverErrorOverloadCall
}

type serverErrorTempCall struct {
	accountID int64
	until     time.Time
	reason    string
}

type serverErrorOverloadCall struct {
	accountID int64
	until     time.Time
}

func (r *serverErrorTempUnscheduleRepo) SetTempUnschedulable(_ context.Context, id int64, until time.Time, reason string) error {
	r.tempCalls = append(r.tempCalls, serverErrorTempCall{
		accountID: id,
		until:     until,
		reason:    reason,
	})
	return nil
}

func (r *serverErrorTempUnscheduleRepo) SetOverloaded(_ context.Context, id int64, until time.Time) error {
	r.overloadCalls = append(r.overloadCalls, serverErrorOverloadCall{
		accountID: id,
		until:     until,
	})
	return nil
}
