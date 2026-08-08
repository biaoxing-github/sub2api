package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type schedulerCancellationCache struct {
	SchedulerCache
	cancel context.CancelFunc
}

func (c *schedulerCancellationCache) GetSnapshot(context.Context, SchedulerBucket) ([]*Account, bool, error) {
	c.cancel()
	return nil, false, context.Canceled
}

func (c *schedulerCancellationCache) GetAccount(context.Context, int64) (*Account, error) {
	c.cancel()
	return nil, context.Canceled
}

type schedulerCancellationAccountRepo struct {
	AccountRepository
	listCalls    int
	getByIDCalls int
}

func (r *schedulerCancellationAccountRepo) ListSchedulableUngroupedByPlatform(context.Context, string) ([]Account, error) {
	r.listCalls++
	return nil, nil
}

func (r *schedulerCancellationAccountRepo) GetByID(context.Context, int64) (*Account, error) {
	r.getByIDCalls++
	return nil, nil
}

// 取消后的请求不能继续执行数据库回退或缓存回填。
func TestSchedulerSnapshotListStopsAfterRequestCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	cache := &schedulerCancellationCache{cancel: cancel}
	repo := &schedulerCancellationAccountRepo{}
	svc := NewSchedulerSnapshotService(cache, nil, repo, nil, nil)

	accounts, _, err := svc.ListSchedulableAccounts(ctx, nil, PlatformOpenAI, false)

	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, accounts)
	require.Zero(t, repo.listCalls)
}

// 单账号缓存读取取消后不能继续查询数据库。
func TestSchedulerSnapshotGetAccountStopsAfterRequestCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	cache := &schedulerCancellationCache{cancel: cancel}
	repo := &schedulerCancellationAccountRepo{}
	svc := NewSchedulerSnapshotService(cache, nil, repo, nil, nil)

	account, err := svc.GetAccount(ctx, 42)

	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, account)
	require.Zero(t, repo.getByIDCalls)
}
