//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

func TestAdminServiceCreateUserRole(t *testing.T) {
	t.Run("admin", func(t *testing.T) {
		repo := &userRepoStub{nextID: 30}
		svc := &adminServiceImpl{userRepo: repo}

		user, err := svc.CreateUser(context.Background(), &CreateUserInput{
			Email: "admin@test.com", Password: "strong-pass", Role: RoleAdmin,
		})
		require.NoError(t, err)
		require.Equal(t, RoleAdmin, user.Role)
	})

	t.Run("default user", func(t *testing.T) {
		repo := &userRepoStub{nextID: 31}
		svc := &adminServiceImpl{userRepo: repo}

		user, err := svc.CreateUser(context.Background(), &CreateUserInput{
			Email: "plain@test.com", Password: "strong-pass",
		})
		require.NoError(t, err)
		require.Equal(t, RoleUser, user.Role)
	})

	t.Run("invalid", func(t *testing.T) {
		repo := &userRepoStub{nextID: 32}
		svc := &adminServiceImpl{userRepo: repo}

		_, err := svc.CreateUser(context.Background(), &CreateUserInput{
			Email: "bad@test.com", Password: "strong-pass", Role: "superuser",
		})
		require.Error(t, err)
		require.Empty(t, repo.created)
	})
}

func TestAdminServiceUpdateUserRole(t *testing.T) {
	t.Run("promote", func(t *testing.T) {
		base := &userRepoStub{user: &User{ID: 42, Email: "u@example.com", Role: RoleUser}}
		repo := &rpmUserRepoStub{userRepoStub: base}
		invalidator := &authCacheInvalidatorStub{}
		svc := &adminServiceImpl{userRepo: repo, redeemCodeRepo: &redeemRepoStub{}, authCacheInvalidator: invalidator}

		updated, err := svc.UpdateUser(context.Background(), 42, &UpdateUserInput{Role: RoleAdmin})
		require.NoError(t, err)
		require.Equal(t, RoleAdmin, updated.Role)
		require.Equal(t, []int64{42}, invalidator.userIDs)
	})

	t.Run("invalid", func(t *testing.T) {
		base := &userRepoStub{user: &User{ID: 42, Email: "u@example.com", Role: RoleUser}}
		repo := &rpmUserRepoStub{userRepoStub: base}
		svc := &adminServiceImpl{userRepo: repo, redeemCodeRepo: &redeemRepoStub{}}

		_, err := svc.UpdateUser(context.Background(), 42, &UpdateUserInput{Role: "root"})
		require.Error(t, err)
		require.Nil(t, repo.lastUpdated)
	})
}

// roleGuardUserRepoStub 提供可控管理员总数，用于验证最后管理员保护。
type roleGuardUserRepoStub struct {
	*rpmUserRepoStub
	adminTotal int64
	listCalls  int
}

func (s *roleGuardUserRepoStub) ListWithFilters(_ context.Context, _ pagination.PaginationParams, _ UserListFilters) ([]User, *pagination.PaginationResult, error) {
	s.listCalls++
	return nil, &pagination.PaginationResult{Total: s.adminTotal}, nil
}

func TestAdminServiceUpdateUserRejectsLastAdminDemotion(t *testing.T) {
	base := &userRepoStub{user: &User{ID: 42, Email: "a@example.com", Role: RoleAdmin}}
	repo := &roleGuardUserRepoStub{rpmUserRepoStub: &rpmUserRepoStub{userRepoStub: base}, adminTotal: 1}
	svc := &adminServiceImpl{userRepo: repo, redeemCodeRepo: &redeemRepoStub{}}

	_, err := svc.UpdateUser(context.Background(), 42, &UpdateUserInput{Role: RoleUser})
	require.ErrorContains(t, err, "last admin")
	require.Nil(t, repo.lastUpdated)
	require.Equal(t, 1, repo.listCalls)
}

func TestAdminServiceUpdateUserRejectsSelfDemotion(t *testing.T) {
	base := &userRepoStub{user: &User{ID: 42, Email: "a@example.com", Role: RoleAdmin}}
	repo := &roleGuardUserRepoStub{rpmUserRepoStub: &rpmUserRepoStub{userRepoStub: base}, adminTotal: 2}
	svc := &adminServiceImpl{userRepo: repo, redeemCodeRepo: &redeemRepoStub{}}

	_, err := svc.UpdateUser(context.Background(), 42, &UpdateUserInput{
		Role:         RoleUser,
		ActorAdminID: 42,
	})
	require.ErrorContains(t, err, "cannot demote yourself")
	require.Nil(t, repo.lastUpdated)
	require.Equal(t, 0, repo.listCalls)
}
