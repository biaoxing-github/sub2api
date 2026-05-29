package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestAccountProbeRepositoryExpireStaleRuns(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("UPDATE account_probe_runs").
		WithArgs("running", "failed", "probe run timed out before completion", "体检任务超时未完成，已自动收尾", "900 seconds").
		WillReturnResult(sqlmock.NewResult(0, 3))

	repo := NewAccountProbeRepository(db)
	err = repo.ExpireStaleAccountProbeRuns(context.Background(), 15*time.Minute)

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAccountProbeRepositoryDeleteReportRunsSkipsRunning(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM account_probe_runs").
		WithArgs(sqlmock.AnyArg(), "running").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec("DELETE FROM account_probe_samples").
		WithArgs(sqlmock.AnyArg(), "running").
		WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectExec("DELETE FROM account_probe_runs").
		WithArgs(sqlmock.AnyArg(), "running").
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()

	repo := NewAccountProbeRepository(db)
	result, err := repo.DeleteAccountProbeReportRuns(context.Background(), []int64{91, 92, 93})

	require.NoError(t, err)
	require.Equal(t, 3, result.RequestedCount)
	require.Equal(t, 2, result.DeletedCount)
	require.Equal(t, 1, result.SkippedRunningCount)
	require.NoError(t, mock.ExpectationsWereMet())
}
