//go:build unit

package repository

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func newSessionIDUsageLog(sessionID *string) *service.UsageLog {
	return &service.UsageLog{
		UserID:       1,
		APIKeyID:     2,
		AccountID:    3,
		RequestID:    "req-session-id",
		Model:        "gpt-5.4",
		InputTokens:  10,
		OutputTokens: 5,
		TotalCost:    1,
		ActualCost:   1,
		SessionID:    sessionID,
		CreatedAt:    time.Now().UTC(),
	}
}

func TestPrepareUsageLogInsertSessionIDArgWiring(t *testing.T) {
	require.Len(t, usageLogInsertArgTypes, 54)
	sessionID := "session-persisted-123"
	prepared := prepareUsageLogInsert(newSessionIDUsageLog(&sessionID))
	require.Len(t, prepared.args, len(usageLogInsertArgTypes))

	sessionArg, ok := prepared.args[len(prepared.args)-2].(sql.NullString)
	require.True(t, ok)
	require.True(t, sessionArg.Valid)
	require.Equal(t, sessionID, sessionArg.String)
	require.Equal(t, "text", usageLogInsertArgTypes[len(usageLogInsertArgTypes)-2])
}

func TestPrepareUsageLogInsertSessionIDNullWhenAbsent(t *testing.T) {
	prepared := prepareUsageLogInsert(newSessionIDUsageLog(nil))
	sessionArg := prepared.args[len(prepared.args)-2].(sql.NullString)
	require.False(t, sessionArg.Valid)

	empty := ""
	prepared = prepareUsageLogInsert(newSessionIDUsageLog(&empty))
	sessionArg = prepared.args[len(prepared.args)-2].(sql.NullString)
	require.False(t, sessionArg.Valid)
}

func TestUsageLogInsertQueriesIncludeSessionID(t *testing.T) {
	require.Contains(t, usageLogSelectColumns, "session_id")
	sessionID := "session-in-query"
	log := newSessionIDUsageLog(&sessionID)
	prepared := prepareUsageLogInsert(log)
	key := usageLogBatchKey(log.RequestID, log.APIKeyID)

	batchQuery, batchArgs := buildUsageLogBatchInsertQuery(
		[]string{key},
		map[string]usageLogInsertPrepared{key: prepared},
	)
	require.GreaterOrEqual(t, strings.Count(batchQuery, "session_id"), 3)
	require.Len(t, batchArgs, len(prepared.args)+1)

	bestEffortQuery, bestEffortArgs := buildUsageLogBestEffortInsertQuery([]usageLogInsertPrepared{prepared})
	require.GreaterOrEqual(t, strings.Count(bestEffortQuery, "session_id"), 3)
	require.Len(t, bestEffortArgs, len(prepared.args))
}
