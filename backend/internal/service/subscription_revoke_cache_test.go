//go:build unit

package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type revokeCacheUserSubRepoStub struct {
	userSubRepoNoop

	sub            *UserSubscription
	deleted        bool
	getActiveCalls int
}

type revokeCachePubSubStub struct {
	billingCacheWorkerStub

	mu                     sync.Mutex
	publishedKeys          []string
	subscriptionHandler    func(string)
	subscriptionContext    context.Context
	subscriptionCalls      int
	invalidateSubCallCount int
}

func (s *revokeCachePubSubStub) InvalidateSubscriptionCache(context.Context, int64, int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.invalidateSubCallCount++
	return nil
}

func (s *revokeCachePubSubStub) PublishSubscriptionCacheInvalidation(_ context.Context, cacheKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.publishedKeys = append(s.publishedKeys, cacheKey)
	return nil
}

func (s *revokeCachePubSubStub) SubscribeSubscriptionCacheInvalidation(ctx context.Context, handler func(string)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subscriptionCalls++
	s.subscriptionContext = ctx
	s.subscriptionHandler = handler
	return nil
}

func (s *revokeCachePubSubStub) snapshot() (func(string), []string, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.subscriptionHandler, append([]string(nil), s.publishedKeys...), s.invalidateSubCallCount
}

func (s *revokeCachePubSubStub) subscriberContext() context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.subscriptionContext
}

func (r *revokeCacheUserSubRepoStub) GetByID(_ context.Context, id int64) (*UserSubscription, error) {
	if r.sub == nil || r.sub.ID != id || r.deleted {
		return nil, ErrSubscriptionNotFound
	}
	cp := *r.sub
	return &cp, nil
}

func (r *revokeCacheUserSubRepoStub) Delete(_ context.Context, id int64) error {
	if r.sub == nil || r.sub.ID != id || r.deleted {
		return ErrSubscriptionNotFound
	}
	r.deleted = true
	return nil
}

func (r *revokeCacheUserSubRepoStub) GetActiveByUserIDAndGroupID(_ context.Context, userID, groupID int64) (*UserSubscription, error) {
	r.getActiveCalls++
	if r.deleted || r.sub == nil || r.sub.UserID != userID || r.sub.GroupID != groupID {
		return nil, ErrSubscriptionNotFound
	}
	cp := *r.sub
	return &cp, nil
}

func TestRevokeSubscriptionInvalidatesL1CacheSynchronously(t *testing.T) {
	repo := &revokeCacheUserSubRepoStub{
		sub: &UserSubscription{
			ID:        1,
			UserID:    10,
			GroupID:   20,
			Status:    SubscriptionStatusActive,
			ExpiresAt: time.Now().Add(time.Hour),
		},
	}
	svc := NewSubscriptionService(groupRepoNoop{}, repo, nil, nil, &config.Config{
		SubscriptionCache: config.SubscriptionCacheConfig{
			L1Size:       16,
			L1TTLSeconds: 60,
		},
	})
	t.Cleanup(svc.Stop)

	_, err := svc.GetActiveSubscription(context.Background(), 10, 20)
	require.NoError(t, err)
	svc.subCacheL1.Wait()
	require.Equal(t, 1, repo.getActiveCalls)

	err = svc.RevokeSubscription(context.Background(), 1)
	require.NoError(t, err)

	_, err = svc.GetActiveSubscription(context.Background(), 10, 20)
	require.ErrorIs(t, err, ErrSubscriptionNotFound)
	require.Equal(t, 2, repo.getActiveCalls, "revocation must bypass a stale L1 subscription")
}

func TestSubscriptionServiceRegistersPubSubInvalidationSubscriber(t *testing.T) {
	repo := &revokeCacheUserSubRepoStub{
		sub: &UserSubscription{
			ID:        1,
			UserID:    10,
			GroupID:   20,
			Status:    SubscriptionStatusActive,
			ExpiresAt: time.Now().Add(time.Hour),
		},
	}
	cache := &revokeCachePubSubStub{}
	billingCacheSvc := NewBillingCacheService(cache, nil, nil, nil, nil, nil, &config.Config{})
	t.Cleanup(billingCacheSvc.Stop)
	svc := NewSubscriptionService(groupRepoNoop{}, repo, billingCacheSvc, nil, &config.Config{
		SubscriptionCache: config.SubscriptionCacheConfig{
			L1Size:       16,
			L1TTLSeconds: 60,
		},
	})
	t.Cleanup(svc.Stop)

	_, err := svc.GetActiveSubscription(context.Background(), 10, 20)
	require.NoError(t, err)
	svc.subCacheL1.Wait()

	handler, _, _ := cache.snapshot()
	require.NotNil(t, handler, "subscription service must subscribe to cross-instance L1 invalidations")
	handler(subCacheKey(10, 20))

	_, err = svc.GetActiveSubscription(context.Background(), 10, 20)
	require.NoError(t, err)
	require.Equal(t, 2, repo.getActiveCalls, "remote invalidation must remove the local L1 entry before the next read")
}

func TestSubscriptionServiceStopCancelsPubSubInvalidationSubscriber(t *testing.T) {
	cache := &revokeCachePubSubStub{}
	billingCacheSvc := NewBillingCacheService(cache, nil, nil, nil, nil, nil, &config.Config{})
	t.Cleanup(billingCacheSvc.Stop)
	svc := NewSubscriptionService(nil, nil, billingCacheSvc, nil, &config.Config{
		SubscriptionCache: config.SubscriptionCacheConfig{
			L1Size:       16,
			L1TTLSeconds: 60,
		},
	})

	subscriberCtx := cache.subscriberContext()
	require.NotNil(t, subscriberCtx)
	select {
	case <-subscriberCtx.Done():
		t.Fatal("subscription cache invalidation subscriber stopped before service shutdown")
	default:
	}

	svc.Stop()

	select {
	case <-subscriberCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("subscription cache invalidation subscriber was not canceled by service shutdown")
	}
}

func TestRevokeSubscriptionPublishesCrossInstanceInvalidation(t *testing.T) {
	repo := &revokeCacheUserSubRepoStub{
		sub: &UserSubscription{
			ID:        1,
			UserID:    10,
			GroupID:   20,
			Status:    SubscriptionStatusActive,
			ExpiresAt: time.Now().Add(time.Hour),
		},
	}
	cache := &revokeCachePubSubStub{}
	billingCacheSvc := NewBillingCacheService(cache, nil, nil, nil, nil, nil, &config.Config{})
	t.Cleanup(billingCacheSvc.Stop)
	svc := NewSubscriptionService(groupRepoNoop{}, repo, billingCacheSvc, nil, &config.Config{
		SubscriptionCache: config.SubscriptionCacheConfig{
			L1Size:       16,
			L1TTLSeconds: 60,
		},
	})
	t.Cleanup(svc.Stop)

	require.NoError(t, svc.RevokeSubscription(context.Background(), 1))
	_, published, invalidations := cache.snapshot()
	require.Equal(t, 1, invalidations, "revocation must synchronously evict the shared subscription cache")
	require.Equal(t, []string{subCacheKey(10, 20)}, published)
}

type restoreUserSubscriptionRepoStub struct {
	userSubRepoNoop

	sub            *UserSubscription
	existsActive   bool
	restoreCalls   int
	restoredStatus string
}

func (r *restoreUserSubscriptionRepoStub) GetByIDIncludeDeleted(_ context.Context, id int64) (*UserSubscription, error) {
	if r.sub == nil || r.sub.ID != id {
		return nil, ErrSubscriptionNotFound
	}
	cp := *r.sub
	return &cp, nil
}

func (r *restoreUserSubscriptionRepoStub) ExistsActiveByUserIDAndGroupID(context.Context, int64, int64) (bool, error) {
	return r.existsActive, nil
}

func (r *restoreUserSubscriptionRepoStub) Restore(_ context.Context, id int64, restoredStatus string) (*UserSubscription, error) {
	if r.sub == nil || r.sub.ID != id {
		return nil, ErrSubscriptionNotFound
	}
	r.restoreCalls++
	r.restoredStatus = restoredStatus
	cp := *r.sub
	cp.Status = restoredStatus
	cp.DeletedAt = nil
	r.sub = &cp
	return &cp, nil
}

func TestRestoreSubscriptionExpiredActiveRestoresAsExpired(t *testing.T) {
	deletedAt := time.Now().Add(-time.Hour)
	repo := &restoreUserSubscriptionRepoStub{
		sub: &UserSubscription{
			ID:        1,
			UserID:    10,
			GroupID:   20,
			Status:    SubscriptionStatusActive,
			ExpiresAt: time.Now().Add(-time.Minute),
			DeletedAt: &deletedAt,
		},
	}
	svc := NewSubscriptionService(groupRepoNoop{}, repo, nil, nil, nil)
	t.Cleanup(svc.Stop)

	restored, err := svc.RestoreSubscription(context.Background(), 1)

	require.NoError(t, err)
	require.Equal(t, 1, repo.restoreCalls)
	require.Equal(t, SubscriptionStatusExpired, repo.restoredStatus)
	require.Equal(t, SubscriptionStatusExpired, restored.Status)
	require.Nil(t, restored.DeletedAt)
}

func TestRestoreSubscriptionNotRevokedReturnsConflict(t *testing.T) {
	repo := &restoreUserSubscriptionRepoStub{
		sub: &UserSubscription{
			ID:        1,
			UserID:    10,
			GroupID:   20,
			Status:    SubscriptionStatusActive,
			ExpiresAt: time.Now().Add(time.Hour),
		},
	}
	svc := NewSubscriptionService(groupRepoNoop{}, repo, nil, nil, nil)
	t.Cleanup(svc.Stop)

	_, err := svc.RestoreSubscription(context.Background(), 1)

	require.ErrorIs(t, err, ErrSubscriptionNotRevoked)
	require.Zero(t, repo.restoreCalls)
}

func TestRestoreSubscriptionLiveSubscriptionReturnsConflict(t *testing.T) {
	deletedAt := time.Now().Add(-time.Hour)
	repo := &restoreUserSubscriptionRepoStub{
		existsActive: true,
		sub: &UserSubscription{
			ID:        1,
			UserID:    10,
			GroupID:   20,
			Status:    SubscriptionStatusExpired,
			ExpiresAt: time.Now().Add(-time.Hour),
			DeletedAt: &deletedAt,
		},
	}
	svc := NewSubscriptionService(groupRepoNoop{}, repo, nil, nil, nil)
	t.Cleanup(svc.Stop)

	_, err := svc.RestoreSubscription(context.Background(), 1)

	require.ErrorIs(t, err, ErrSubscriptionRestoreConflict)
	require.Zero(t, repo.restoreCalls)
}
