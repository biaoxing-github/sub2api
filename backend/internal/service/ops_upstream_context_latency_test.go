package service

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSnapshotOpsLatencyStagesCopiesCurrentRequestValues(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	SetOpsLatencyMs(c, OpsAuthLatencyMsKey, 12)
	SetOpsLatencyMs(c, OpsRoutingLatencyMsKey, 18)
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, 5502)
	SetOpsLatencyMs(c, OpsResponseLatencyMsKey, 204)

	stages := SnapshotOpsLatencyStages(c)
	require.NotNil(t, stages)
	require.Equal(t, int64(12), *stages.AuthMs)
	require.Equal(t, int64(18), *stages.RoutingMs)
	require.Equal(t, int64(5502), *stages.UpstreamMs)
	require.Equal(t, int64(204), *stages.ResponseMs)

	SetOpsLatencyMs(c, OpsAuthLatencyMsKey, 99)
	require.Equal(t, int64(12), *stages.AuthMs)
}

func TestSnapshotOpsLatencyStagesReturnsNilWithoutMeasurements(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	require.Nil(t, SnapshotOpsLatencyStages(c))
}
