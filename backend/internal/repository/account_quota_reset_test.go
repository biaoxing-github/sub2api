package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// TestQuotaResetMissingAccount 验证不存在的账号返回错误且不触发调度快照。
func TestQuotaResetMissingAccount(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := newAccountRepositoryWithSQL(nil, db, nil)
	mock.ExpectExec("UPDATE accounts SET extra").WithArgs(int64(42)).WillReturnResult(sqlmock.NewResult(0, 0))
	require.ErrorIs(t, repo.ResetQuotaUsed(context.Background(), 42), service.ErrAccountNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}
