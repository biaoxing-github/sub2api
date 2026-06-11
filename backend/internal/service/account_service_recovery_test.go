//go:build unit

// RecoverAccountAfterManualProbe 方法单元测试
// 测试手动探测成功后恢复账号为可调度状态

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// accountRepoRecoveryStub 是测试 RecoverAccountAfterManualProbe 的桩实现
type accountRepoRecoveryStub struct {
	accountRepoStub // 嵌入基础桩
	getByIDAccount  *Account
	getByIDErr      error
	updateCalled    bool
	updatedAccount  *Account
	clearTempCalled bool
	clearRLCalled   bool
}

func (s *accountRepoRecoveryStub) GetByID(ctx context.Context, id int64) (*Account, error) {
	return s.getByIDAccount, s.getByIDErr
}

func (s *accountRepoRecoveryStub) Update(ctx context.Context, account *Account) error {
	s.updateCalled = true
	s.updatedAccount = account
	return nil
}

func (s *accountRepoRecoveryStub) ClearTempUnschedulable(ctx context.Context, id int64) error {
	s.clearTempCalled = true
	return nil
}

func (s *accountRepoRecoveryStub) ClearRateLimit(ctx context.Context, id int64) error {
	s.clearRLCalled = true
	return nil
}

// TestAccountService_RecoverAfterManualProbe_Success 测试成功恢复
func TestAccountService_RecoverAfterManualProbe_Success(t *testing.T) {
	ctx := context.Background()
	accountID := int64(123)
	platform := "anthropic"

	blockedAccount := &Account{
		ID:          accountID,
		Platform:    platform,
		Schedulable: false,
		Status:      "error",
	}

	repo := &accountRepoRecoveryStub{
		getByIDAccount: blockedAccount,
	}

	service := &AccountService{
		accountRepo: repo,
	}

	recovered, err := service.RecoverAccountAfterManualProbe(ctx, accountID, platform)

	require.NoError(t, err)
	require.NotNil(t, recovered)
	require.True(t, recovered.Schedulable)
	require.Equal(t, "active", recovered.Status)
	require.True(t, repo.updateCalled)
	require.True(t, repo.clearTempCalled)
	require.True(t, repo.clearRLCalled)
}

// TestAccountService_RecoverAfterManualProbe_AccountNotFound 测试账号不存在
func TestAccountService_RecoverAfterManualProbe_AccountNotFound(t *testing.T) {
	ctx := context.Background()

	repo := &accountRepoRecoveryStub{
		getByIDErr: ErrAccountNotFound,
	}

	service := &AccountService{accountRepo: repo}

	recovered, err := service.RecoverAccountAfterManualProbe(ctx, 999, "anthropic")

	require.Error(t, err)
	require.Nil(t, recovered)
	require.False(t, repo.updateCalled)
}

// TestAccountService_RecoverAfterManualProbe_PlatformMismatch 测试平台不匹配
func TestAccountService_RecoverAfterManualProbe_PlatformMismatch(t *testing.T) {
	ctx := context.Background()

	account := &Account{
		ID:       123,
		Platform: "openai",
		Status:   "error",
	}

	repo := &accountRepoRecoveryStub{
		getByIDAccount: account,
	}

	service := &AccountService{accountRepo: repo}

	recovered, err := service.RecoverAccountAfterManualProbe(ctx, 123, "anthropic")

	require.Error(t, err)
	require.Nil(t, recovered)
	require.ErrorContains(t, err, "platform mismatch")
	require.False(t, repo.updateCalled)
}

// TestAccountService_RecoverAfterManualProbe_FromInactive 测试从 inactive 恢复
func TestAccountService_RecoverAfterManualProbe_FromInactive(t *testing.T) {
	ctx := context.Background()

	account := &Account{
		ID:          456,
		Platform:    "openai",
		Schedulable: false,
		Status:      "inactive",
	}

	repo := &accountRepoRecoveryStub{
		getByIDAccount: account,
	}

	service := &AccountService{accountRepo: repo}

	recovered, err := service.RecoverAccountAfterManualProbe(ctx, 456, "openai")

	require.NoError(t, err)
	require.NotNil(t, recovered)
	require.True(t, recovered.Schedulable)
	require.Equal(t, "active", recovered.Status)
}
