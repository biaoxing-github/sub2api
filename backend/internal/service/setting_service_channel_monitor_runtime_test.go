package service

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// channelMonitorRuntimeSettingsRepo 仅记录批量写入结果，用于验证运行时通知时序。
type channelMonitorRuntimeSettingsRepo struct {
	values map[string]string
	setErr error
}

// runtimeReconcileMonitorService 模拟一个启用的 V1 监控项并记录实际探测次数。
type runtimeReconcileMonitorService struct {
	monitors []*ChannelMonitor
	runs     atomic.Int64
	runCh    chan struct{}
}

func (s *runtimeReconcileMonitorService) ListEnabledMonitors(context.Context) ([]*ChannelMonitor, error) {
	return s.monitors, nil
}

func (s *runtimeReconcileMonitorService) RunCheck(context.Context, int64) ([]*CheckResult, error) {
	s.runs.Add(1)
	select {
	case s.runCh <- struct{}{}:
	default:
	}
	return nil, nil
}

func waitForChannelMonitorRuntime(t *testing.T, description string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.True(t, condition(), description)
}

func (r *channelMonitorRuntimeSettingsRepo) Get(context.Context, string) (*Setting, error) {
	panic("unexpected Get call")
}

func (r *channelMonitorRuntimeSettingsRepo) GetValue(context.Context, string) (string, error) {
	return "", ErrSettingNotFound
}

func (r *channelMonitorRuntimeSettingsRepo) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}

func (r *channelMonitorRuntimeSettingsRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			result[key] = value
		}
	}
	return result, nil
}

func (r *channelMonitorRuntimeSettingsRepo) SetMultiple(_ context.Context, settings map[string]string) error {
	if r.setErr != nil {
		return r.setErr
	}
	if r.values == nil {
		r.values = make(map[string]string, len(settings))
	}
	for key, value := range settings {
		r.values[key] = value
	}
	return nil
}

func (r *channelMonitorRuntimeSettingsRepo) GetAll(context.Context) (map[string]string, error) {
	result := make(map[string]string, len(r.values))
	for key, value := range r.values {
		result[key] = value
	}
	return result, nil
}

func (r *channelMonitorRuntimeSettingsRepo) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

func TestSettingServiceUpdateSettingsNotifiesChannelMonitorRuntimeAfterSuccessfulWrite(t *testing.T) {
	repo := &channelMonitorRuntimeSettingsRepo{}
	svc := NewSettingService(repo, &config.Config{})
	sequence := make([]string, 0, 2)
	svc.SetOnUpdateCallback(func() {
		sequence = append(sequence, "cache")
	})
	unsubscribe := svc.SubscribeChannelMonitorRuntime(func() {
		sequence = append(sequence, "channel_monitor")
	})
	t.Cleanup(unsubscribe)

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		ChannelMonitorEnabled:        true,
		ChannelMonitorMode:           ChannelMonitorModeV2,
		ChannelMonitorHideThroughput: true,
	})
	require.NoError(t, err)
	require.Equal(t, []string{"cache", "channel_monitor"}, sequence)
}

func TestSettingServiceUpdateSettingsDoesNotNotifyChannelMonitorRuntimeAfterFailedWrite(t *testing.T) {
	writeErr := errors.New("write failed")
	repo := &channelMonitorRuntimeSettingsRepo{setErr: writeErr}
	svc := NewSettingService(repo, &config.Config{})
	notified := 0
	svc.SubscribeChannelMonitorRuntime(func() {
		notified++
	})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		ChannelMonitorEnabled: true,
		ChannelMonitorMode:    ChannelMonitorModeV2,
	})
	require.ErrorIs(t, err, writeErr)
	require.Zero(t, notified)
}

func TestChannelMonitorRunnerReconcilesImmediatelyAfterRuntimeSettingsUpdate(t *testing.T) {
	repo := &channelMonitorRuntimeSettingsRepo{values: map[string]string{
		SettingKeyChannelMonitorEnabled: "true",
		SettingKeyChannelMonitorMode:    ChannelMonitorModeV1,
	}}
	settingService := NewSettingService(repo, &config.Config{})
	monitorService := &runtimeReconcileMonitorService{
		monitors: []*ChannelMonitor{{
			ID:              7,
			Name:            "runtime-monitor",
			Enabled:         true,
			IntervalSeconds: 60,
		}},
		runCh: make(chan struct{}, 4),
	}
	runner := newChannelMonitorRunner(monitorService, settingService)
	runner.Start()
	t.Cleanup(runner.Stop)

	waitForChannelMonitorRuntime(t, "V1 startup should schedule one task", func() bool {
		return runner.taskCount() == 1 && monitorService.runs.Load() >= 1
	})

	require.NoError(t, settingService.UpdateSettings(context.Background(), &SystemSettings{
		ChannelMonitorEnabled: true,
		ChannelMonitorMode:    ChannelMonitorModeV2,
	}))
	waitForChannelMonitorRuntime(t, "V2 update should cancel all V1 tasks", func() bool {
		return runner.taskCount() == 0
	})
	runner.Schedule(&ChannelMonitor{
		ID:              8,
		Name:            "v2-created-monitor",
		Enabled:         true,
		IntervalSeconds: 60,
	})
	require.Zero(t, runner.taskCount(), "V2 mode must reject V1 CRUD scheduling callbacks")

	runsBeforeResume := monitorService.runs.Load()
	require.NoError(t, settingService.UpdateSettings(context.Background(), &SystemSettings{
		ChannelMonitorEnabled: true,
		ChannelMonitorMode:    ChannelMonitorModeV1,
	}))
	waitForChannelMonitorRuntime(t, "V1 update should rebuild tasks and resume probing", func() bool {
		return runner.taskCount() == 1 && monitorService.runs.Load() > runsBeforeResume
	})
}
