package service

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	gocache "github.com/patrickmn/go-cache"
	"golang.org/x/sync/singleflight"
)

var userGroupRateCacheVersion atomic.Uint64

type userGroupRateResolver struct {
	repo         UserGroupRateRepository
	cache        *gocache.Cache
	cacheTTL     time.Duration
	sf           *singleflight.Group
	logComponent string
}

func newUserGroupRateResolver(repo UserGroupRateRepository, cache *gocache.Cache, cacheTTL time.Duration, sf *singleflight.Group, logComponent string) *userGroupRateResolver {
	if cacheTTL <= 0 {
		cacheTTL = defaultUserGroupRateCacheTTL
	}
	if cache == nil {
		cache = gocache.New(cacheTTL, time.Minute)
	}
	if logComponent == "" {
		logComponent = "service.gateway"
	}
	if sf == nil {
		sf = &singleflight.Group{}
	}

	return &userGroupRateResolver{
		repo:         repo,
		cache:        cache,
		cacheTTL:     cacheTTL,
		sf:           sf,
		logComponent: logComponent,
	}
}

// currentUserGroupRateCacheVersion 返回用户专属分组倍率缓存的全局版本。
func currentUserGroupRateCacheVersion() uint64 {
	return userGroupRateCacheVersion.Load()
}

// bumpUserGroupRateCacheVersion 推进倍率缓存版本，让各 gateway resolver 自动绕开旧缓存键。
func bumpUserGroupRateCacheVersion() uint64 {
	return userGroupRateCacheVersion.Add(1)
}

func userGroupRateCacheKey(userID, groupID int64) string {
	version := currentUserGroupRateCacheVersion()
	if version == 0 {
		return fmt.Sprintf("%d:%d", userID, groupID)
	}
	return fmt.Sprintf("%d:%d:v%d", userID, groupID, version)
}

func (r *userGroupRateResolver) Resolve(ctx context.Context, userID, groupID int64, groupDefaultMultiplier float64) float64 {
	if r == nil || userID <= 0 || groupID <= 0 {
		return groupDefaultMultiplier
	}

	key := userGroupRateCacheKey(userID, groupID)
	if r.cache != nil {
		if cached, ok := r.cache.Get(key); ok {
			if multiplier, castOK := cached.(float64); castOK {
				userGroupRateCacheHitTotal.Add(1)
				return multiplier
			}
		}
	}
	if r.repo == nil {
		return groupDefaultMultiplier
	}
	userGroupRateCacheMissTotal.Add(1)

	value, err, shared := r.sf.Do(key, func() (any, error) {
		if r.cache != nil {
			if cached, ok := r.cache.Get(key); ok {
				if multiplier, castOK := cached.(float64); castOK {
					userGroupRateCacheHitTotal.Add(1)
					return multiplier, nil
				}
			}
		}

		userGroupRateCacheLoadTotal.Add(1)
		userRate, repoErr := r.repo.GetByUserAndGroup(ctx, userID, groupID)
		if repoErr != nil {
			return nil, repoErr
		}

		multiplier := groupDefaultMultiplier
		if userRate != nil {
			multiplier = *userRate
		}
		if r.cache != nil {
			r.cache.Set(key, multiplier, r.cacheTTL)
		}
		return multiplier, nil
	})
	if shared {
		userGroupRateCacheSFSharedTotal.Add(1)
	}
	if err != nil {
		userGroupRateCacheFallbackTotal.Add(1)
		logger.LegacyPrintf(r.logComponent, "get user group rate failed, fallback to group default: user=%d group=%d err=%v", userID, groupID, err)
		return groupDefaultMultiplier
	}

	multiplier, ok := value.(float64)
	if !ok {
		userGroupRateCacheFallbackTotal.Add(1)
		return groupDefaultMultiplier
	}
	return multiplier
}
