//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdminService_UpdateUserGroupRates_InvalidatesRuntimeCaches(t *testing.T) {
	resetUserGroupRateCacheVersionForTest()

	base := &userRepoStub{user: &User{ID: 42, Email: "u@example.com"}}
	repo := &rpmUserRepoStub{userRepoStub: base}
	rateRepo := &userGroupRateRepoStubForGroupRate{}
	invalidator := &authCacheInvalidatorStub{}
	svc := &adminServiceImpl{
		userRepo:             repo,
		redeemCodeRepo:       &redeemRepoStub{},
		userGroupRateRepo:    rateRepo,
		authCacheInvalidator: invalidator,
	}
	customRate := 1.8

	_, err := svc.UpdateUser(context.Background(), 42, &UpdateUserInput{
		GroupRates: map[int64]*float64{7: &customRate},
	})

	require.NoError(t, err)
	require.Equal(t, int64(42), rateRepo.syncedUserID)
	require.Equal(t, customRate, *rateRepo.syncedUserRates[7])
	require.Equal(t, []int64{42}, invalidator.userIDs)
	require.Equal(t, uint64(1), currentUserGroupRateCacheVersion())
}
