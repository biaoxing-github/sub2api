package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type scheduledRunnerPlanRepoStub struct {
	updatedID      int64
	updatedLastRun time.Time
	updatedNextRun time.Time
	listDue        func(ctx context.Context, now time.Time) ([]*ScheduledTestPlan, error)
}

func (r *scheduledRunnerPlanRepoStub) Create(ctx context.Context, plan *ScheduledTestPlan) (*ScheduledTestPlan, error) {
	return plan, nil
}

func (r *scheduledRunnerPlanRepoStub) GetByID(ctx context.Context, id int64) (*ScheduledTestPlan, error) {
	return nil, ErrAccountNotFound
}

func (r *scheduledRunnerPlanRepoStub) ListByAccountID(ctx context.Context, accountID int64) ([]*ScheduledTestPlan, error) {
	return nil, nil
}

func (r *scheduledRunnerPlanRepoStub) ListDue(ctx context.Context, now time.Time) ([]*ScheduledTestPlan, error) {
	if r.listDue != nil {
		return r.listDue(ctx, now)
	}
	return nil, nil
}

func (r *scheduledRunnerPlanRepoStub) Update(ctx context.Context, plan *ScheduledTestPlan) (*ScheduledTestPlan, error) {
	return plan, nil
}

func (r *scheduledRunnerPlanRepoStub) Delete(ctx context.Context, id int64) error {
	return nil
}

func (r *scheduledRunnerPlanRepoStub) UpdateAfterRun(ctx context.Context, id int64, lastRunAt time.Time, nextRunAt time.Time) error {
	r.updatedID = id
	r.updatedLastRun = lastRunAt
	r.updatedNextRun = nextRunAt
	return nil
}

type scheduledRunnerResultRepoStub struct {
	created []*ScheduledTestResult
}

func (r *scheduledRunnerResultRepoStub) Create(ctx context.Context, result *ScheduledTestResult) (*ScheduledTestResult, error) {
	copy := *result
	copy.ID = int64(len(r.created) + 1)
	r.created = append(r.created, &copy)
	return &copy, nil
}

func (r *scheduledRunnerResultRepoStub) ListByPlanID(ctx context.Context, planID int64, limit int) ([]*ScheduledTestResult, error) {
	return nil, nil
}

func (r *scheduledRunnerResultRepoStub) PruneOldResults(ctx context.Context, planID int64, keepCount int) error {
	return nil
}

type scheduledAccountProbeRunnerStub struct {
	startReq                 AccountProbeRunRequest
	runExistingReq           AccountProbeRunRequest
	runExistingHasDeadline   bool
	runExistingDeadlineDelta time.Duration
}

func (s *scheduledAccountProbeRunnerStub) Start(ctx context.Context, req AccountProbeRunRequest) (AccountProbeResult, error) {
	s.startReq = req
	return AccountProbeResult{ID: 8801, AccountID: req.AccountID, Profile: req.Profile, Status: AccountProbeStatusRunning, Model: req.Model, RequestMode: req.RequestMode, RequestCount: 3}, nil
}

func (s *scheduledAccountProbeRunnerStub) RunExisting(ctx context.Context, run AccountProbeResult, req AccountProbeRunRequest) (AccountProbeResult, error) {
	s.runExistingReq = req
	deadline, hasDeadline := ctx.Deadline()
	s.runExistingHasDeadline = hasDeadline
	if hasDeadline {
		s.runExistingDeadlineDelta = time.Until(deadline)
	}
	run.Status = AccountProbeStatusSuccess
	run.SuccessCount = 3
	run.Summary = "完成 3/3 次请求，平均延迟 800 ms，消耗 1200 tokens"
	run.Latency = AccountProbeLatencyStats{AvgMillis: 800}
	finished := time.Now()
	run.FinishedAt = &finished
	return run, nil
}

func TestScheduledTestRunnerRunsAccountProbePlan(t *testing.T) {
	planRepo := &scheduledRunnerPlanRepoStub{}
	resultRepo := &scheduledRunnerResultRepoStub{}
	scheduledSvc := NewScheduledTestService(planRepo, resultRepo)
	probeRunner := &scheduledAccountProbeRunnerStub{}
	runner := NewScheduledTestRunnerService(planRepo, scheduledSvc, nil, nil, nil)
	runner.SetAccountProbeRunner(probeRunner)

	plan := &ScheduledTestPlan{
		ID:                  7,
		AccountID:           181,
		TaskType:            ScheduledTestTaskTypeAccountProbe,
		ModelID:             "gpt-5.5",
		CronExpression:      "*/30 * * * *",
		MaxResults:          5,
		ProbeMode:           AccountProbeProfileStandard,
		ProbeRequestMode:    AccountProbeRequestModeStream,
		ProbeCodexStability: true,
	}

	runner.runOnePlanWithTimeout(plan)

	require.Equal(t, int64(181), probeRunner.startReq.AccountID)
	require.Equal(t, "gpt-5.5", probeRunner.startReq.Model)
	require.Equal(t, AccountProbeProfileStandard, probeRunner.startReq.Profile)
	require.Equal(t, AccountProbeRequestModeStream, probeRunner.startReq.RequestMode)
	require.False(t, probeRunner.startReq.IncludeCodexStability)
	require.Equal(t, probeRunner.startReq, probeRunner.runExistingReq)
	require.True(t, probeRunner.runExistingHasDeadline)
	require.Greater(t, probeRunner.runExistingDeadlineDelta, 11*time.Minute)
	require.LessOrEqual(t, probeRunner.runExistingDeadlineDelta, 12*time.Minute)
	require.Len(t, resultRepo.created, 1)
	require.Equal(t, int64(8801), resultRepo.created[0].AccountProbeRunID)
	require.Equal(t, AccountProbeStatusSuccess, resultRepo.created[0].Status)
	require.Equal(t, int64(7), planRepo.updatedID)
	require.False(t, planRepo.updatedNextRun.IsZero())
}

func TestScheduledTestRunnerSnapshotsSkipOverlappingTicks(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	planRepo := &scheduledRunnerPlanRepoStub{
		listDue: func(ctx context.Context, now time.Time) ([]*ScheduledTestPlan, error) {
			started <- struct{}{}
			<-release
			return nil, nil
		},
	}
	runner := NewScheduledTestRunnerService(planRepo, NewScheduledTestService(planRepo, &scheduledRunnerResultRepoStub{}), nil, nil, nil)

	go runner.runScheduledWithDelay(0)
	<-started
	runner.runScheduledWithDelay(0)

	snapshots := runner.Snapshots()
	require.Len(t, snapshots, 1)
	require.Equal(t, "scheduled_test_runner", snapshots[0].Name)
	require.True(t, snapshots[0].Running)
	require.Equal(t, int64(1), snapshots[0].RunCount)
	require.Equal(t, int64(1), snapshots[0].SkippedCount)
	require.NotNil(t, snapshots[0].LastStartedAt)

	close(release)
	require.Eventually(t, func() bool {
		snapshot := runner.Snapshots()[0]
		return !snapshot.Running && snapshot.SuccessCount == 1 && snapshot.LastFinishedAt != nil && snapshot.LastDurationMillis >= 0
	}, time.Second, 10*time.Millisecond)
}
