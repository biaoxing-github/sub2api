package service

import (
	"context"
	"database/sql"
	"time"
)

// LeaderLockCache 为周期后台任务提供跨实例互斥锁。
// 仓储层用 Redis 实现，服务层只依赖接口，释放时按 owner 做 compare-and-delete。
type LeaderLockCache interface {
	TryAcquireLeaderLock(ctx context.Context, key, owner string, ttl time.Duration) (bool, error)
	ReleaseLeaderLock(ctx context.Context, key, owner string) error
}

// tryAcquireSingletonLeaderLock 为周期后台任务选出单实例执行者。
// 优先使用 Redis 锁，Redis 不可用时退回 PostgreSQL advisory lock；没有任何协调后端时按单实例模式运行。
func tryAcquireSingletonLeaderLock(ctx context.Context, cache LeaderLockCache, db *sql.DB, key, owner string, ttl time.Duration) (func(), bool) {
	if ctx == nil {
		ctx = context.Background()
	}

	if cache != nil {
		ok, err := cache.TryAcquireLeaderLock(ctx, key, owner, ttl)
		if err == nil {
			if !ok {
				return nil, false
			}
			release := func() {
				releaseCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				_ = cache.ReleaseLeaderLock(releaseCtx, key, owner)
			}
			return release, true
		}
	}

	if db != nil {
		return tryAcquireDBAdvisoryLock(ctx, db, hashAdvisoryLockID(key))
	}

	return func() {}, true
}
