package repository

import (
	"context"
	"database/sql/driver"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestEnqueueSchedulerOutbox_UsesPersistentDedupKeyForIdempotentEvents(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	accountID := int64(42)
	payload := map[string]any{"group_ids": []int64{7, 8}}
	mock.ExpectExec("INSERT INTO scheduler_outbox .*dedup_key.*ON CONFLICT").
		WithArgs(
			service.SchedulerOutboxEventAccountChanged,
			&accountID,
			nil,
			sqlmock.AnyArg(),
			schedulerOutboxDedupKeyArg{},
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = enqueueSchedulerOutbox(context.Background(), db, service.SchedulerOutboxEventAccountChanged, &accountID, nil, payload)

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnqueueSchedulerOutbox_DoesNotDedupLastUsedEvents(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	accountID := int64(42)
	payload := map[string]any{"last_used": map[string]int64{"42": 1710000000}}
	mock.ExpectExec("INSERT INTO scheduler_outbox \\(event_type, account_id, group_id, payload\\)").
		WithArgs(
			service.SchedulerOutboxEventAccountLastUsed,
			&accountID,
			nil,
			sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = enqueueSchedulerOutbox(context.Background(), db, service.SchedulerOutboxEventAccountLastUsed, &accountID, nil, payload)

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSchedulerOutboxDedupKeyIncludesPayload(t *testing.T) {
	accountID := int64(42)
	first := schedulerOutboxDedupKey(service.SchedulerOutboxEventAccountChanged, &accountID, nil, []byte(`{"group_ids":[7]}`))
	second := schedulerOutboxDedupKey(service.SchedulerOutboxEventAccountChanged, &accountID, nil, []byte(`{"group_ids":[8]}`))

	require.NotEqual(t, first, second)
	require.True(t, strings.HasPrefix(first, "scheduler_outbox:"))
	require.Len(t, strings.TrimPrefix(first, "scheduler_outbox:"), 64)
}

func TestSchedulerOutboxRepositoryListAfterAndReleaseDedup(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	createdAt := time.Now().UTC()
	rows := sqlmock.NewRows([]string{"id", "event_type", "account_id", "group_id", "payload", "created_at"}).
		AddRow(int64(11), service.SchedulerOutboxEventAccountChanged, int64(42), nil, []byte(`{"group_ids":[7]}`), createdAt)
	mock.ExpectQuery("WITH selected AS MATERIALIZED .*SET dedup_key = NULL").
		WithArgs(int64(10), 200).
		WillReturnRows(rows)

	repo := NewSchedulerOutboxRepository(db)
	events, err := repo.ListAfterAndReleaseDedup(context.Background(), 10, 200)

	require.NoError(t, err)
	require.Len(t, events, 1)
	require.EqualValues(t, 11, events[0].ID)
	require.NotNil(t, events[0].AccountID)
	require.EqualValues(t, 42, *events[0].AccountID)
	require.Equal(t, []any{float64(7)}, events[0].Payload["group_ids"])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSchedulerOutboxRepositoryDeleteConsumedUpTo(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectExec("DELETE FROM scheduler_outbox").
		WithArgs(int64(99), 5000).
		WillReturnResult(sqlmock.NewResult(0, 3))

	repo := NewSchedulerOutboxRepository(db)
	deleted, err := repo.DeleteConsumedUpTo(context.Background(), 99, 0)

	require.NoError(t, err)
	require.EqualValues(t, 3, deleted)
	require.NoError(t, mock.ExpectationsWereMet())
}

type schedulerOutboxDedupKeyArg struct{}

func (schedulerOutboxDedupKeyArg) Match(v driver.Value) bool {
	raw, ok := v.(string)
	return ok && strings.HasPrefix(raw, "scheduler_outbox:") && len(strings.TrimPrefix(raw, "scheduler_outbox:")) == 64
}
