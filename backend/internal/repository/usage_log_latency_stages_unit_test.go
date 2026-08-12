//go:build unit

package repository

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestPrepareUsageLogInsertLatencyStagesArgWiring(t *testing.T) {
	authMs := int64(12)
	routingMs := int64(18)
	upstreamMs := int64(5502)
	responseMs := int64(204)
	log := newSessionIDUsageLog(nil)
	log.LatencyStages = &service.UsageLatencyStages{
		AuthMs:     &authMs,
		RoutingMs:  &routingMs,
		UpstreamMs: &upstreamMs,
		ResponseMs: &responseMs,
	}

	prepared := prepareUsageLogInsert(log)
	require.Len(t, prepared.args, len(usageLogInsertArgTypes))
	require.JSONEq(t, `{"auth_ms":12,"routing_ms":18,"upstream_ms":5502,"response_ms":204}`, prepared.args[len(prepared.args)-3].(string))
	require.Equal(t, "jsonb", usageLogInsertArgTypes[len(usageLogInsertArgTypes)-3])
	require.Contains(t, usageLogSelectColumns, "latency_stages")

	query, args := buildUsageLogBestEffortInsertQuery([]usageLogInsertPrepared{prepared})
	require.GreaterOrEqual(t, strings.Count(query, "latency_stages"), 3)
	require.Len(t, args, len(prepared.args))
}

func TestUsageLatencyStagesJSONRoundTrip(t *testing.T) {
	stages := usageLatencyStagesFromNullJSON(sql.NullString{
		Valid:  true,
		String: `{"auth_ms":7,"upstream_ms":1200}`,
	})
	require.NotNil(t, stages)
	require.Equal(t, int64(7), *stages.AuthMs)
	require.Equal(t, int64(1200), *stages.UpstreamMs)
	require.Nil(t, stages.RoutingMs)
}
