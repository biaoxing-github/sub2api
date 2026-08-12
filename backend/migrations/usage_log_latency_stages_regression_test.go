package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUsageLogLatencyStagesMigrationIsEmbedded(t *testing.T) {
	sql, err := FS.ReadFile("209_usage_log_latency_stages.sql")
	require.NoError(t, err)
	require.Contains(t, string(sql), "ADD COLUMN IF NOT EXISTS latency_stages JSONB")
}
