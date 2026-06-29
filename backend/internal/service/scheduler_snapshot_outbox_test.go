//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSchedulerSnapshotPollOutboxReadsAndReleasesDedupBeforeCleanup(t *testing.T) {
	cache := &schedulerOutboxPollCache{}
	repo := &schedulerOutboxPollRepo{
		events: []SchedulerOutboxEvent{{
			ID:        12,
			EventType: SchedulerOutboxEventAccountLastUsed,
			Payload: map[string]any{
				"last_used": map[string]any{"42": float64(1710000000)},
			},
			CreatedAt: time.Now().Add(-time.Second),
		}},
	}

	svc := NewSchedulerSnapshotService(cache, repo, nil, nil, nil)
	svc.pollOutbox()

	if repo.listAfter != 0 {
		t.Fatalf("expected list after watermark 0, got %d", repo.listAfter)
	}
	if !repo.usedReleaseList {
		t.Fatalf("expected pollOutbox to use ListAfterAndReleaseDedup")
	}
	if cache.watermark != 12 {
		t.Fatalf("expected watermark 12, got %d", cache.watermark)
	}
	if repo.cleanupWatermark != 12 {
		t.Fatalf("expected cleanup watermark 12, got %d", repo.cleanupWatermark)
	}
	if !repo.lease.released {
		t.Fatalf("expected cleanup lease to be released")
	}
	if got := cache.lastUsed[42]; got.Unix() != 1710000000 {
		t.Fatalf("expected last_used cache update, got %v", got)
	}
}

func TestSchedulerSnapshotPollOutboxSkipsCleanupWhenWatermarkWriteFails(t *testing.T) {
	cache := &schedulerOutboxPollCache{setWatermarkErr: errors.New("redis unavailable")}
	repo := &schedulerOutboxPollRepo{
		events: []SchedulerOutboxEvent{{
			ID:        12,
			EventType: SchedulerOutboxEventAccountLastUsed,
			Payload: map[string]any{
				"last_used": map[string]any{"42": float64(1710000000)},
			},
			CreatedAt: time.Now().Add(-time.Second),
		}},
	}

	svc := NewSchedulerSnapshotService(cache, repo, nil, nil, nil)
	svc.pollOutbox()

	if repo.cleanupCalls != 0 {
		t.Fatalf("expected cleanup to wait for a durable watermark write, got %d calls", repo.cleanupCalls)
	}
}

type schedulerOutboxPollCache struct {
	watermark       int64
	setWatermarkErr error
	lastUsed        map[int64]time.Time
}

func (c *schedulerOutboxPollCache) GetSnapshot(ctx context.Context, bucket SchedulerBucket) ([]*Account, bool, error) {
	return nil, false, nil
}

func (c *schedulerOutboxPollCache) SetSnapshot(ctx context.Context, bucket SchedulerBucket, accounts []Account) error {
	return nil
}

func (c *schedulerOutboxPollCache) GetAccount(ctx context.Context, accountID int64) (*Account, error) {
	return nil, nil
}

func (c *schedulerOutboxPollCache) SetAccount(ctx context.Context, account *Account) error {
	return nil
}

func (c *schedulerOutboxPollCache) DeleteAccount(ctx context.Context, accountID int64) error {
	return nil
}

func (c *schedulerOutboxPollCache) UpdateLastUsed(ctx context.Context, updates map[int64]time.Time) error {
	c.lastUsed = updates
	return nil
}

func (c *schedulerOutboxPollCache) TryLockBucket(ctx context.Context, bucket SchedulerBucket, ttl time.Duration) (bool, error) {
	return true, nil
}

func (c *schedulerOutboxPollCache) UnlockBucket(ctx context.Context, bucket SchedulerBucket) error {
	return nil
}

func (c *schedulerOutboxPollCache) ListBuckets(ctx context.Context) ([]SchedulerBucket, error) {
	return nil, nil
}

func (c *schedulerOutboxPollCache) GetOutboxWatermark(ctx context.Context) (int64, error) {
	return c.watermark, nil
}

func (c *schedulerOutboxPollCache) SetOutboxWatermark(ctx context.Context, id int64) error {
	if c.setWatermarkErr != nil {
		return c.setWatermarkErr
	}
	c.watermark = id
	return nil
}

type schedulerOutboxPollRepo struct {
	events           []SchedulerOutboxEvent
	listAfter        int64
	usedReleaseList  bool
	cleanupCalls     int
	cleanupWatermark int64
	lease            schedulerOutboxPollLease
}

func (r *schedulerOutboxPollRepo) ListAfterAndReleaseDedup(ctx context.Context, afterID int64, limit int) ([]SchedulerOutboxEvent, error) {
	r.usedReleaseList = true
	r.listAfter = afterID
	return r.events, nil
}

func (r *schedulerOutboxPollRepo) MaxID(ctx context.Context) (int64, error) {
	return 12, nil
}

func (r *schedulerOutboxPollRepo) DeleteConsumedUpTo(ctx context.Context, watermark int64, limit int) (int64, error) {
	r.cleanupCalls++
	r.cleanupWatermark = watermark
	return 0, nil
}

func (r *schedulerOutboxPollRepo) TryAcquireCleanupLock(ctx context.Context) (SchedulerOutboxCleanupLease, bool, error) {
	return &r.lease, true, nil
}

type schedulerOutboxPollLease struct {
	released bool
}

func (l *schedulerOutboxPollLease) Release() {
	l.released = true
}
