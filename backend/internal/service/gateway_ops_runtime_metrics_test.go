package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

// opsRuntimeCanceledGroupRepo 让账号选择在分组解析阶段稳定返回取消错误。
type opsRuntimeCanceledGroupRepo struct {
	GroupRepository
}

func (r *opsRuntimeCanceledGroupRepo) GetByIDLite(ctx context.Context, _ int64) (*Group, error) {
	return nil, ctx.Err()
}

func TestGatewayOpsRuntimeMetricsRecordsSnapshotReuseAndSelectionOutcomes(t *testing.T) {
	before := SnapshotOpsRuntimeMetrics()
	fixture := newRequestSchedulingFailureStormFixture(2, 8100)
	ctx := context.WithValue(context.Background(), ctxkey.ForcePlatform, PlatformAnthropic)
	ctx = fixture.svc.WithRequestSchedulingSnapshot(ctx, nil, PlatformAnthropic, true)

	first, err := fixture.svc.SelectAccountWithLoadAwareness(ctx, nil, "", "", nil, "", 0)
	require.NoError(t, err)
	require.NotNil(t, first)
	first.ReleaseFunc()

	second, err := fixture.svc.SelectAccountWithLoadAwareness(ctx, nil, "", "", map[int64]struct{}{first.Account.ID: {}}, "", 0)
	require.NoError(t, err)
	require.NotNil(t, second)
	second.ReleaseFunc()

	_, err = fixture.svc.SelectAccountWithLoadAwareness(ctx, nil, "", "", map[int64]struct{}{
		first.Account.ID:  {},
		second.Account.ID: {},
	}, "", 0)
	require.ErrorIs(t, err, ErrNoAvailableAccounts)

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	groupID := int64(91)
	_, err = (&GatewayService{groupRepo: &opsRuntimeCanceledGroupRepo{}}).SelectAccountWithLoadAwareness(
		canceledCtx,
		&groupID,
		"",
		"",
		nil,
		"",
		0,
	)
	require.ErrorIs(t, err, context.Canceled)

	after := SnapshotOpsRuntimeMetrics()
	require.Equal(t, uint64(1), opsRuntimeCounterDelta(before.Scheduling, after.Scheduling, "scheduler", string(OpsSchedulingEventSnapshotMiss)))
	require.Equal(t, uint64(2), opsRuntimeCounterDelta(before.Scheduling, after.Scheduling, "scheduler", string(OpsSchedulingEventSnapshotHit)))
	require.Equal(t, uint64(2), opsRuntimeRequestStageDelta(before, after, OpsRequestResultSuccess, OpsRequestErrorClassNone))
	require.Equal(t, uint64(1), opsRuntimeRequestStageDelta(before, after, OpsRequestResultFailure, OpsRequestErrorClassInternal))
	require.Equal(t, uint64(1), opsRuntimeRequestStageDelta(before, after, OpsRequestResultCanceled, OpsRequestErrorClassCanceled))
}

func TestGatewayOpsRuntimeMetricsRecordsMissWithoutRequestSnapshot(t *testing.T) {
	before := SnapshotOpsRuntimeMetrics()
	ctx := (&GatewayService{}).withRequestSchedulingPrefetch(context.Background(), nil, PlatformAnthropic, true, nil)
	require.NotNil(t, ctx)
	after := SnapshotOpsRuntimeMetrics()

	require.Equal(t, uint64(1), opsRuntimeCounterDelta(before.Scheduling, after.Scheduling, "scheduler", string(OpsSchedulingEventSnapshotMiss)))
	require.Equal(t, uint64(0), opsRuntimeCounterDelta(before.Scheduling, after.Scheduling, "scheduler", string(OpsSchedulingEventSnapshotHit)))
}

func opsRuntimeCounterDelta(before, after []OpsRuntimeCounterSnapshot, name, event string) uint64 {
	return opsRuntimeCounterCount(after, name, event) - opsRuntimeCounterCount(before, name, event)
}

func opsRuntimeCounterCount(counters []OpsRuntimeCounterSnapshot, name, event string) uint64 {
	for _, counter := range counters {
		if counter.Name == name && counter.Event == event {
			return counter.Count
		}
	}
	return 0
}

func opsRuntimeRequestStageDelta(before, after OpsRuntimeMetricsSnapshot, result OpsRequestResult, errorClass OpsRequestErrorClass) uint64 {
	return opsRuntimeRequestStageCount(after, result, errorClass) - opsRuntimeRequestStageCount(before, result, errorClass)
}

func opsRuntimeRequestStageCount(snapshot OpsRuntimeMetricsSnapshot, result OpsRequestResult, errorClass OpsRequestErrorClass) uint64 {
	for _, metric := range snapshot.RequestStages {
		if metric.Stage == OpsRequestStageSelection && metric.Result == result && metric.ErrorClass == errorClass {
			return metric.Count
		}
	}
	return 0
}
