//go:build unit

package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// billingCacheSafetyStub 保留旧缓存值，并允许测试精确控制异步写入结果。
type billingCacheSafetyStub struct {
	balance      float64
	subscription *SubscriptionCacheData
	rateLimit    *APIKeyRateLimitCacheData

	deductErr              error
	setBalanceErr          error
	subscriptionErr        error
	rateLimitErr           error
	rateLimitSetErr        error
	deductWaitsForDeadline bool

	balanceGets      atomic.Int64
	subscriptionGets atomic.Int64
	rateLimitGets    atomic.Int64
	rateLimitSets    atomic.Int64
	deductCalls      atomic.Int64
}

func (s *billingCacheSafetyStub) GetUserBalance(context.Context, int64) (float64, error) {
	s.balanceGets.Add(1)
	return s.balance, nil
}

func (s *billingCacheSafetyStub) SetUserBalance(context.Context, int64, float64) error {
	return s.setBalanceErr
}

func (s *billingCacheSafetyStub) DeductUserBalance(ctx context.Context, _ int64, _ float64) error {
	s.deductCalls.Add(1)
	if s.deductWaitsForDeadline {
		<-ctx.Done()
		return ctx.Err()
	}
	return s.deductErr
}

func (s *billingCacheSafetyStub) InvalidateUserBalance(context.Context, int64) error { return nil }

func (s *billingCacheSafetyStub) GetSubscriptionCache(context.Context, int64, int64) (*SubscriptionCacheData, error) {
	s.subscriptionGets.Add(1)
	return s.subscription, nil
}

func (s *billingCacheSafetyStub) SetSubscriptionCache(context.Context, int64, int64, *SubscriptionCacheData) error {
	return nil
}

func (s *billingCacheSafetyStub) UpdateSubscriptionUsage(context.Context, int64, int64, float64) error {
	return s.subscriptionErr
}

func (s *billingCacheSafetyStub) InvalidateSubscriptionCache(context.Context, int64, int64) error {
	return nil
}

func (s *billingCacheSafetyStub) GetAPIKeyRateLimit(context.Context, int64) (*APIKeyRateLimitCacheData, error) {
	s.rateLimitGets.Add(1)
	return s.rateLimit, nil
}

func (s *billingCacheSafetyStub) SetAPIKeyRateLimit(context.Context, int64, *APIKeyRateLimitCacheData) error {
	s.rateLimitSets.Add(1)
	return s.rateLimitSetErr
}

func (s *billingCacheSafetyStub) UpdateAPIKeyRateLimitUsage(context.Context, int64, float64) error {
	return s.rateLimitErr
}

func (s *billingCacheSafetyStub) InvalidateAPIKeyRateLimit(context.Context, int64) error { return nil }

type billingCacheSafetySubRepo struct {
	userSubRepoNoop
	sub   *UserSubscription
	delay time.Duration
	calls atomic.Int64
}

func (r *billingCacheSafetySubRepo) GetActiveByUserIDAndGroupID(ctx context.Context, _ int64, _ int64) (*UserSubscription, error) {
	r.calls.Add(1)
	if r.delay > 0 {
		select {
		case <-time.After(r.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return r.sub, nil
}

type billingCacheSafetyRateLimitLoader struct {
	calls atomic.Int64
	data  *APIKeyRateLimitData
	delay time.Duration
}

func (l *billingCacheSafetyRateLimitLoader) GetRateLimitData(ctx context.Context, _ int64) (*APIKeyRateLimitData, error) {
	l.calls.Add(1)
	if l.delay > 0 {
		select {
		case <-time.After(l.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return l.data, nil
}

func billingCacheMetricCount(event OpsCacheEvent) uint64 {
	for _, counter := range SnapshotOpsRuntimeMetrics().Caches {
		if counter.Name == string(OpsCacheBilling) && counter.Event == string(event) {
			return counter.Count
		}
	}
	return 0
}

func stopBillingCacheWorkersForSafetyTest(svc *BillingCacheService) {
	svc.Stop()
	svc.stopped.Store(false)
	svc.cacheWriteMu.Lock()
	svc.cacheWriteChan = make(chan cacheWriteTask, 1)
	svc.cacheWriteChan <- cacheWriteTask{}
	svc.cacheWriteMu.Unlock()
}

func TestBillingCacheServiceQueueFullBypassesPositiveCachesWithoutSyncRedisFallback(t *testing.T) {
	now := time.Now().Add(time.Hour)
	cache := &billingCacheSafetyStub{
		balance: 99,
		subscription: &SubscriptionCacheData{
			Status:    SubscriptionStatusActive,
			ExpiresAt: now,
		},
		rateLimit: &APIKeyRateLimitCacheData{},
	}
	userRepo := &balanceLoadUserRepoStub{balance: 0}
	subRepo := &billingCacheSafetySubRepo{sub: &UserSubscription{
		Status:        SubscriptionStatusActive,
		ExpiresAt:     now,
		DailyUsageUSD: 10,
	}}
	rateLoader := &billingCacheSafetyRateLimitLoader{data: &APIKeyRateLimitData{Usage5h: 10, Window5hStart: &now}}
	svc := NewBillingCacheService(cache, userRepo, subRepo, nil, nil, nil, &config.Config{})
	svc.apiKeyRateLimitLoader = rateLoader
	stopBillingCacheWorkersForSafetyTest(svc)

	svc.QueueDeductBalance(1, 1)
	svc.QueueUpdateSubscriptionUsage(1, 2, 1)
	svc.QueueUpdateAPIKeyRateLimitUsage(3, 1)

	balance, err := svc.GetUserBalance(context.Background(), 1)
	require.NoError(t, err)
	require.Zero(t, balance, "队列满后不能继续信任旧余额缓存")
	require.Equal(t, int64(0), cache.balanceGets.Load(), "不应读取已标记不安全的余额缓存")
	require.Equal(t, int64(0), cache.deductCalls.Load(), "队列满不能为每次丢弃无条件同步写 Redis")

	subscription, err := svc.GetSubscriptionStatus(context.Background(), 1, 2)
	require.NoError(t, err)
	require.Equal(t, 10.0, subscription.DailyUsage, "队列满后订阅检查必须使用主数据")
	require.Equal(t, int64(0), cache.subscriptionGets.Load(), "不应读取已标记不安全的订阅缓存")

	err = svc.checkAPIKeyRateLimits(context.Background(), &APIKey{ID: 3, RateLimit5h: 10})
	require.ErrorIs(t, err, ErrAPIKeyRateLimit5hExceeded, "队列满后 API Key 限流必须使用主数据")
	require.Equal(t, int64(0), cache.rateLimitGets.Load(), "不应读取已标记不安全的限流缓存")
}

func TestBillingCacheServiceWorkerRedisErrorBypassesStaleBalanceWithSingleflight(t *testing.T) {
	cache := &billingCacheSafetyStub{
		balance:       99,
		deductErr:     errors.New("redis unavailable"),
		setBalanceErr: errors.New("redis unavailable"),
	}
	userRepo := &balanceLoadUserRepoStub{balance: 0, delay: 30 * time.Millisecond}
	svc := NewBillingCacheService(cache, userRepo, nil, nil, nil, nil, &config.Config{})
	t.Cleanup(svc.Stop)

	svc.QueueDeductBalance(42, 1)
	require.Eventually(t, func() bool {
		return svc.cacheEntryUnsafe(balanceCacheEntryKey(42))
	}, time.Second, 10*time.Millisecond)

	const callers = 12
	start := make(chan struct{})
	var wg sync.WaitGroup
	errCh := make(chan error, callers)
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			balance, err := svc.GetUserBalance(context.Background(), 42)
			if err == nil && balance != 0 {
				err = errors.New("unsafe balance cache returned a positive value")
			}
			errCh <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}

	require.Equal(t, int64(1), userRepo.calls.Load(), "不安全缓存旁路仍必须合并同 key 回源")
	require.Equal(t, int64(0), cache.balanceGets.Load(), "Redis 写失败后不能继续读取旧余额缓存")
}

func TestBillingCacheServiceQueueClosedMarksAllPositiveCacheKindsUnsafe(t *testing.T) {
	cache := &billingCacheSafetyStub{}
	svc := NewBillingCacheService(cache, nil, nil, nil, nil, nil, &config.Config{})
	svc.Stop()

	svc.QueueDeductBalance(1, 1)
	svc.QueueUpdateSubscriptionUsage(1, 2, 1)
	svc.QueueUpdateAPIKeyRateLimitUsage(3, 1)

	require.True(t, svc.cacheEntryUnsafe(balanceCacheEntryKey(1)))
	require.True(t, svc.cacheEntryUnsafe(subscriptionCacheEntryKey(1, 2)))
	require.True(t, svc.cacheEntryUnsafe(apiKeyRateLimitCacheEntryKey(3)))
	require.Zero(t, cache.deductCalls.Load(), "关闭队列后不能同步写 Redis")
}

func TestBillingCacheServiceWorkerTimeoutBypassesStaleBalance(t *testing.T) {
	cache := &billingCacheSafetyStub{
		balance:                99,
		setBalanceErr:          errors.New("redis unavailable"),
		deductWaitsForDeadline: true,
	}
	userRepo := &balanceLoadUserRepoStub{balance: 0}
	svc := NewBillingCacheService(cache, userRepo, nil, nil, nil, nil, &config.Config{})
	t.Cleanup(svc.Stop)

	svc.QueueDeductBalance(7, 1)
	require.Eventually(t, func() bool {
		return svc.cacheEntryUnsafe(balanceCacheEntryKey(7))
	}, cacheWriteTimeout+time.Second, 10*time.Millisecond)

	balance, err := svc.GetUserBalance(context.Background(), 7)
	require.NoError(t, err)
	require.Zero(t, balance, "worker 超时后不能继续信任旧余额缓存")
	require.Zero(t, cache.balanceGets.Load())
}

func TestBillingCacheServiceUnsafeSubscriptionBypassUsesSingleflight(t *testing.T) {
	now := time.Now().Add(time.Hour)
	cache := &billingCacheSafetyStub{subscription: &SubscriptionCacheData{
		Status:     SubscriptionStatusActive,
		ExpiresAt:  now,
		DailyUsage: 0,
	}}
	subRepo := &billingCacheSafetySubRepo{
		sub: &UserSubscription{
			Status:        SubscriptionStatusActive,
			ExpiresAt:     now,
			DailyUsageUSD: 10,
		},
		delay: 30 * time.Millisecond,
	}
	svc := NewBillingCacheService(cache, nil, subRepo, nil, nil, nil, &config.Config{})
	svc.Stop()
	svc.QueueUpdateSubscriptionUsage(9, 8, 1)

	const callers = 12
	start := make(chan struct{})
	var wg sync.WaitGroup
	errCh := make(chan error, callers)
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			subscription, err := svc.GetSubscriptionStatus(context.Background(), 9, 8)
			if err == nil && subscription.DailyUsage != 10 {
				err = errors.New("unsafe subscription cache returned stale usage")
			}
			errCh <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}

	require.Equal(t, int64(1), subRepo.calls.Load(), "不安全订阅缓存旁路必须合并同一 user/group 回源")
	require.Zero(t, cache.subscriptionGets.Load(), "不安全订阅缓存不能读取旧正向快照")
}

func TestBillingCacheServiceNilCacheBypassUsesSingleflight(t *testing.T) {
	t.Run("balance", func(t *testing.T) {
		userRepo := &balanceLoadUserRepoStub{balance: 12, delay: 30 * time.Millisecond}
		svc := NewBillingCacheService(nil, userRepo, nil, nil, nil, nil, &config.Config{})
		t.Cleanup(svc.Stop)

		const callers = 12
		start := make(chan struct{})
		errCh := make(chan error, callers)
		for range callers {
			go func() {
				<-start
				balance, err := svc.GetUserBalance(context.Background(), 13)
				if err == nil && balance != 12 {
					err = errors.New("nil cache balance returned unexpected value")
				}
				errCh <- err
			}()
		}
		close(start)
		for range callers {
			require.NoError(t, <-errCh)
		}
		require.Equal(t, int64(1), userRepo.calls.Load(), "nil cache 余额旁路必须合并同一用户回源")
	})

	t.Run("subscription", func(t *testing.T) {
		now := time.Now().Add(time.Hour)
		subRepo := &billingCacheSafetySubRepo{
			sub:   &UserSubscription{Status: SubscriptionStatusActive, ExpiresAt: now, DailyUsageUSD: 12},
			delay: 30 * time.Millisecond,
		}
		svc := NewBillingCacheService(nil, nil, subRepo, nil, nil, nil, &config.Config{})
		t.Cleanup(svc.Stop)

		const callers = 12
		start := make(chan struct{})
		errCh := make(chan error, callers)
		for range callers {
			go func() {
				<-start
				subscription, err := svc.GetSubscriptionStatus(context.Background(), 13, 14)
				if err == nil && subscription.DailyUsage != 12 {
					err = errors.New("nil cache subscription returned unexpected usage")
				}
				errCh <- err
			}()
		}
		close(start)
		for range callers {
			require.NoError(t, <-errCh)
		}
		require.Equal(t, int64(1), subRepo.calls.Load(), "nil cache 订阅旁路必须合并同一订阅回源")
	})
}

func TestBillingCacheServiceRateLimitBypassUsesSingleflightAndAsyncRefresh(t *testing.T) {
	t.Run("nil_cache_singleflight", func(t *testing.T) {
		loader := &billingCacheSafetyRateLimitLoader{
			data:  &APIKeyRateLimitData{},
			delay: 30 * time.Millisecond,
		}
		svc := NewBillingCacheService(nil, nil, nil, nil, nil, nil, &config.Config{})
		svc.apiKeyRateLimitLoader = loader
		t.Cleanup(svc.Stop)

		const callers = 12
		start := make(chan struct{})
		errCh := make(chan error, callers)
		for range callers {
			go func() {
				<-start
				errCh <- svc.checkAPIKeyRateLimits(context.Background(), &APIKey{ID: 15, RateLimit5h: 100})
			}()
		}
		close(start)
		for range callers {
			require.NoError(t, <-errCh)
		}
		require.Equal(t, int64(1), loader.calls.Load(), "nil cache API Key 限流旁路必须合并同一键回源")
	})

	t.Run("closed_queue_marks_unsafe_without_sync_redis", func(t *testing.T) {
		cache := &billingCacheSafetyStub{}
		svc := NewBillingCacheService(cache, nil, nil, nil, nil, nil, &config.Config{})
		svc.Stop()

		svc.refreshAPIKeyRateLimitCache(16, &APIKeyRateLimitCacheData{})
		require.True(t, svc.cacheEntryUnsafe(apiKeyRateLimitCacheEntryKey(16)))
		require.Zero(t, cache.rateLimitSets.Load(), "关闭队列后不能同步写 Redis")
	})

	t.Run("worker_error_marks_unsafe", func(t *testing.T) {
		cache := &billingCacheSafetyStub{rateLimitSetErr: errors.New("redis unavailable")}
		svc := NewBillingCacheService(cache, nil, nil, nil, nil, nil, &config.Config{})
		t.Cleanup(svc.Stop)

		svc.refreshAPIKeyRateLimitCache(17, &APIKeyRateLimitCacheData{})
		require.Eventually(t, func() bool {
			return svc.cacheEntryUnsafe(apiKeyRateLimitCacheEntryKey(17))
		}, time.Second, 10*time.Millisecond)
		require.Equal(t, int64(1), cache.rateLimitSets.Load())
	})
}

func TestBillingCacheServiceOlderSnapshotCannotClearNewerUnsafeMarker(t *testing.T) {
	svc := &BillingCacheService{}
	key := balanceCacheEntryKey(42)
	svc.markCacheEntryUnsafe(key)
	oldVersion := svc.cacheSafetyVersion(key)
	svc.markCacheEntryUnsafe(key)
	newVersion := svc.cacheSafetyVersion(key)

	svc.markCacheEntryFresh(key, oldVersion)
	require.True(t, svc.cacheEntryUnsafe(key), "较早快照不能清除后续失败标记")
	require.Equal(t, oldVersion+1, newVersion)

	svc.markCacheEntryFresh(key, newVersion)
	require.False(t, svc.cacheEntryUnsafe(key), "对应版本的完整快照可以恢复缓存读取")
}

func TestBillingCacheServiceRecordsCacheEventDeltas(t *testing.T) {
	hitBefore := billingCacheMetricCount(OpsCacheEventHit)
	missBefore := billingCacheMetricCount(OpsCacheEventMiss)
	writeBefore := billingCacheMetricCount(OpsCacheEventWrite)
	writeDropBefore := billingCacheMetricCount(OpsCacheEventWriteDrop)
	writeErrorBefore := billingCacheMetricCount(OpsCacheEventWriteError)

	cache := &billingCacheSafetyStub{
		balance:       99,
		deductErr:     errors.New("redis unavailable"),
		setBalanceErr: errors.New("redis unavailable"),
	}
	svc := NewBillingCacheService(cache, &balanceLoadUserRepoStub{balance: 0}, nil, nil, nil, nil, &config.Config{})
	t.Cleanup(svc.Stop)

	_, err := svc.GetUserBalance(context.Background(), 77)
	require.NoError(t, err)
	svc.QueueDeductBalance(77, 1)
	require.Eventually(t, func() bool {
		return svc.cacheEntryUnsafe(balanceCacheEntryKey(77))
	}, time.Second, 10*time.Millisecond)
	_, err = svc.GetUserBalance(context.Background(), 77)
	require.NoError(t, err)

	droppedSvc := NewBillingCacheService(&billingCacheSafetyStub{}, nil, nil, nil, nil, nil, &config.Config{})
	droppedSvc.Stop()
	droppedSvc.QueueDeductBalance(88, 1)

	successfulSvc := NewBillingCacheService(&billingCacheSafetyStub{}, nil, nil, nil, nil, nil, &config.Config{})
	t.Cleanup(successfulSvc.Stop)
	successfulSvc.QueueDeductBalance(66, 1)

	require.Eventually(t, func() bool {
		return billingCacheMetricCount(OpsCacheEventWriteError) >= writeErrorBefore+1
	}, time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		return billingCacheMetricCount(OpsCacheEventWrite) >= writeBefore+1
	}, time.Second, 10*time.Millisecond)
	require.GreaterOrEqual(t, billingCacheMetricCount(OpsCacheEventHit), hitBefore+1)
	require.GreaterOrEqual(t, billingCacheMetricCount(OpsCacheEventMiss), missBefore+1)
	require.GreaterOrEqual(t, billingCacheMetricCount(OpsCacheEventWriteDrop), writeDropBefore+1)
}
