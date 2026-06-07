package repository

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const leaderLockKeyPrefix = "leader:lock:"

// leaderLockReleaseScript 仅在 owner token 一致时释放锁，避免过期旧 owner 删除新 owner 的锁。
var leaderLockReleaseScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0
`)

type leaderLockCache struct {
	rdb *redis.Client
}

// NewLeaderLockCache 创建 Redis backed leader lock，供多实例后台任务选主。
func NewLeaderLockCache(rdb *redis.Client) service.LeaderLockCache {
	return &leaderLockCache{rdb: rdb}
}

func (c *leaderLockCache) TryAcquireLeaderLock(ctx context.Context, key, owner string, ttl time.Duration) (bool, error) {
	return c.rdb.SetNX(ctx, leaderLockKeyPrefix+key, owner, ttl).Result()
}

func (c *leaderLockCache) ReleaseLeaderLock(ctx context.Context, key, owner string) error {
	return leaderLockReleaseScript.Run(ctx, c.rdb, []string{leaderLockKeyPrefix + key}, owner).Err()
}
