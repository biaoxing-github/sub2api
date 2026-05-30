//go:build unit

package admin

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAccountBatchTestLimiterLimitsGroupConcurrency(t *testing.T) {
	limiter := newAccountBatchTestLimiter(10, 1, time.Second, time.Second)

	release, err := limiter.Acquire(context.Background(), "openai:oauth:group:1")
	require.NoError(t, err)
	defer release()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	secondRelease, secondErr := limiter.Acquire(ctx, "openai:oauth:group:1")
	require.ErrorIs(t, secondErr, context.DeadlineExceeded)
	require.Nil(t, secondRelease)
}

func TestAccountBatchTestLimiterAllowsDifferentGroups(t *testing.T) {
	limiter := newAccountBatchTestLimiter(10, 1, time.Second, time.Second)

	firstRelease, err := limiter.Acquire(context.Background(), "openai:oauth:group:1")
	require.NoError(t, err)
	defer firstRelease()

	secondRelease, err := limiter.Acquire(context.Background(), "openai:oauth:group:2")
	require.NoError(t, err)
	secondRelease()
}

func TestAccountBatchTestLimiterWaitsAfterRiskyBurst(t *testing.T) {
	limiter := newAccountBatchTestLimiter(10, 3, 10*time.Second, 20*time.Millisecond)
	key := "openai:oauth:group:1"
	start := time.Now()

	limiter.RecordResult(key, "rate_limited")
	limiter.RecordResult(key, "unexpected_eof")
	limiter.RecordResult(key, "header_timeout")

	release, err := limiter.Acquire(context.Background(), key)
	require.NoError(t, err)
	require.NotNil(t, release)
	require.GreaterOrEqual(t, time.Since(start), 15*time.Millisecond)
	release()
}

func TestAccountBatchTestLimiterRecoversAfterPauseWindow(t *testing.T) {
	base := time.Unix(1000, 0)
	now := base
	limiter := newAccountBatchTestLimiter(10, 3, 10*time.Second, time.Minute)
	limiter.now = func() time.Time { return now }
	key := "openai:oauth:group:1"

	limiter.RecordResult(key, "rate_limited")
	limiter.RecordResult(key, "unexpected_eof")
	limiter.RecordResult(key, "header_timeout")
	now = base.Add(time.Minute + time.Second)

	release, err := limiter.Acquire(context.Background(), key)
	require.NoError(t, err)
	require.NotNil(t, release)
	release()
}

func TestAccountBatchTestLimiterReturnsPauseErrorWhenContextCanceled(t *testing.T) {
	base := time.Unix(1000, 0)
	limiter := newAccountBatchTestLimiter(10, 3, 10*time.Second, time.Minute)
	limiter.now = func() time.Time { return base }
	key := "openai:oauth:group:1"

	limiter.RecordResult(key, "rate_limited")
	limiter.RecordResult(key, "unexpected_eof")
	limiter.RecordResult(key, "header_timeout")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	release, err := limiter.Acquire(ctx, key)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Contains(t, err.Error(), "paused")
	require.Nil(t, release)
}
