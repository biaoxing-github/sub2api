//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestChannelMonitorV2CompositeErrorAggregation 在独立 PostgreSQL 中验证实际 SQL 与平台归因。
func TestChannelMonitorV2CompositeErrorAggregation(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)
	var groupID, accountID int64
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO groups(name, platform) VALUES ('monitor-composite-test', 'composite') RETURNING id`).Scan(&groupID))
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO accounts(name, platform, type, credentials) VALUES ('monitor-account-test', 'openai', 'apikey', '{}') RETURNING id`).Scan(&accountID))
	start := time.Now().UTC().Truncate(time.Minute)
	_, err := tx.ExecContext(ctx, `INSERT INTO ops_error_logs(request_id, group_id, account_id, platform, model, error_phase, error_type, status_code, created_at)
		VALUES ('monitor-concrete', $1, $2, 'composite', 'gpt-5', 'upstream', 'upstream', 502, $3),
		('monitor-fallback', $1, NULL, 'anthropic', 'claude', 'upstream', 'upstream', 502, $3),
		('monitor-unknown', $1, NULL, 'composite', 'unknown-model', 'routing', 'upstream', 502, $3)`, groupID, accountID, start)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, channelMonitorV2ErrorAggregationSQL, start, start.Add(time.Minute))
	require.NoError(t, err)
	rows, err := tx.QueryContext(ctx, `SELECT platform, error_requests FROM channel_monitor_v2_metrics_1m WHERE group_id = $1`, groupID)
	require.NoError(t, err)
	defer rows.Close()
	actual := map[string]int64{}
	for rows.Next() {
		var platform string
		var count int64
		require.NoError(t, rows.Scan(&platform, &count))
		actual[platform] = count
	}
	require.NoError(t, rows.Err())
	require.Equal(t, map[string]int64{"openai": 1, "anthropic": 1, "unknown": 1}, actual)
}
