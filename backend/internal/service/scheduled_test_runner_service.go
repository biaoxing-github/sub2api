package service

import (
	"context"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/robfig/cron/v3"
)

const (
	scheduledTestDefaultMaxWorkers = 10
	scheduledTestDefaultRunTimeout = 5 * time.Minute
	scheduledTestRunnerJobName     = "scheduled_test_runner"
	scheduledTestRunnerInterval    = time.Minute
	scheduledTestRunnerInitialWait = 10 * time.Second
)

// ScheduledTestRunnerService periodically scans due test plans and executes them.
type ScheduledTestRunnerService struct {
	planRepo           ScheduledTestPlanRepository
	scheduledSvc       *ScheduledTestService
	accountTestSvc     *AccountTestService
	accountProbeRunner scheduledAccountProbeRunner
	rateLimitSvc       *RateLimitService
	cfg                *config.Config

	cron      *cron.Cron
	startOnce sync.Once
	stopOnce  sync.Once

	runtimeMu sync.Mutex
	runtime   ScheduledTestRunnerSnapshot
}

type scheduledAccountProbeRunner interface {
	Start(ctx context.Context, req AccountProbeRunRequest) (AccountProbeResult, error)
	RunExisting(ctx context.Context, run AccountProbeResult, req AccountProbeRunRequest) (AccountProbeResult, error)
}

// NewScheduledTestRunnerService creates a new runner.
func NewScheduledTestRunnerService(
	planRepo ScheduledTestPlanRepository,
	scheduledSvc *ScheduledTestService,
	accountTestSvc *AccountTestService,
	rateLimitSvc *RateLimitService,
	cfg *config.Config,
) *ScheduledTestRunnerService {
	return &ScheduledTestRunnerService{
		planRepo:       planRepo,
		scheduledSvc:   scheduledSvc,
		accountTestSvc: accountTestSvc,
		rateLimitSvc:   rateLimitSvc,
		cfg:            cfg,
		runtime: ScheduledTestRunnerSnapshot{
			Name:               scheduledTestRunnerJobName,
			IntervalMillis:     int64(scheduledTestRunnerInterval / time.Millisecond),
			InitialDelayMillis: int64(scheduledTestRunnerInitialWait / time.Millisecond),
		},
	}
}

func (s *ScheduledTestRunnerService) SetAccountProbeRunner(runner scheduledAccountProbeRunner) {
	if s == nil {
		return
	}
	s.accountProbeRunner = runner
}

// Start begins the cron ticker (every minute).
func (s *ScheduledTestRunnerService) Start() {
	if s == nil {
		return
	}
	s.startOnce.Do(func() {
		loc := time.Local
		if s.cfg != nil {
			if parsed, err := time.LoadLocation(s.cfg.Timezone); err == nil && parsed != nil {
				loc = parsed
			}
		}

		c := cron.New(cron.WithParser(scheduledTestCronParser), cron.WithLocation(loc))
		_, err := c.AddFunc("* * * * *", func() { s.runScheduled() })
		if err != nil {
			logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] not started (invalid schedule): %v", err)
			return
		}
		s.cron = c
		s.cron.Start()
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] started (tick=every minute)")
	})
}

// Stop gracefully shuts down the cron scheduler.
func (s *ScheduledTestRunnerService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		if s.cron != nil {
			ctx := s.cron.Stop()
			select {
			case <-ctx.Done():
			case <-time.After(3 * time.Second):
				logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] cron stop timed out")
			}
		}
	})
}

func (s *ScheduledTestRunnerService) runScheduled() {
	// Delay 10s so execution lands at ~:10 of each minute instead of :00.
	s.runScheduledWithDelay(scheduledTestRunnerInitialWait)
}

func (s *ScheduledTestRunnerService) runScheduledWithDelay(delay time.Duration) {
	if !s.beginScheduledRun() {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] skipped because previous tick is still running")
		return
	}
	started := time.Now()
	var runErr error
	defer func() {
		s.finishScheduledRun(started, runErr)
	}()
	if delay > 0 {
		time.Sleep(delay)
	}

	ctx, cancel := context.WithTimeout(context.Background(), scheduledTestDefaultRunTimeout)
	defer cancel()

	now := time.Now()
	plans, err := s.planRepo.ListDue(ctx, now)
	if err != nil {
		runErr = err
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] ListDue error: %v", err)
		return
	}
	if len(plans) == 0 {
		return
	}

	logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] found %d due plans", len(plans))

	sem := make(chan struct{}, scheduledTestDefaultMaxWorkers)
	var wg sync.WaitGroup

	for _, plan := range plans {
		sem <- struct{}{}
		wg.Add(1)
		go func(p *ScheduledTestPlan) {
			defer wg.Done()
			defer func() { <-sem }()
			s.runOnePlanWithTimeout(p)
		}(plan)
	}

	wg.Wait()
}

func (s *ScheduledTestRunnerService) beginScheduledRun() bool {
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	if s.runtime.Running {
		s.runtime.SkippedCount++
		return false
	}
	now := time.Now()
	s.runtime.Running = true
	s.runtime.RunCount++
	s.runtime.LastStartedAt = &now
	return true
}

func (s *ScheduledTestRunnerService) finishScheduledRun(started time.Time, runErr error) {
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	now := time.Now()
	duration := int64(now.Sub(started) / time.Millisecond)
	if duration < 0 {
		duration = 0
	}
	s.runtime.Running = false
	s.runtime.LastFinishedAt = &now
	s.runtime.LastDurationMillis = duration
	if duration > s.runtime.MaxDurationMillis {
		s.runtime.MaxDurationMillis = duration
	}
	if runErr != nil {
		s.runtime.FailureCount++
		s.runtime.LastErrorAt = &now
		s.runtime.LastError = runErr.Error()
		return
	}
	s.runtime.SuccessCount++
	s.runtime.LastSuccessAt = &now
	s.runtime.LastErrorAt = nil
	s.runtime.LastError = ""
}

// Snapshots returns the runner runtime status for admin observability.
func (s *ScheduledTestRunnerService) Snapshots() []ScheduledTestRunnerSnapshot {
	if s == nil {
		return nil
	}
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	snapshot := s.runtime
	return []ScheduledTestRunnerSnapshot{snapshot}
}

func (s *ScheduledTestRunnerService) runOnePlanWithTimeout(plan *ScheduledTestPlan) {
	ctx, cancel := context.WithTimeout(context.Background(), scheduledTestPlanTimeout(plan))
	defer cancel()
	s.runOnePlan(ctx, plan)
}

func scheduledTestPlanTimeout(plan *ScheduledTestPlan) time.Duration {
	if plan != nil && plan.TaskType == ScheduledTestTaskTypeAccountProbe {
		return accountProbeStaleRunAge
	}
	return scheduledTestDefaultRunTimeout
}

func (s *ScheduledTestRunnerService) runOnePlan(ctx context.Context, plan *ScheduledTestPlan) {
	if plan != nil && plan.TaskType == ScheduledTestTaskTypeAccountProbe {
		s.runOneAccountProbePlan(ctx, plan)
		return
	}
	result, err := s.accountTestSvc.RunTestBackground(ctx, plan.AccountID, plan.ModelID)
	if err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d RunTestBackground error: %v", plan.ID, err)
		return
	}

	if err := s.scheduledSvc.SaveResult(ctx, plan.ID, plan.MaxResults, result); err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d SaveResult error: %v", plan.ID, err)
	}

	// Auto-recover account if test succeeded and auto_recover is enabled.
	if result.Status == "success" && plan.AutoRecover {
		s.tryRecoverAccount(ctx, plan.AccountID, plan.ID)
	}

	nextRun, err := computeNextRun(plan.CronExpression, time.Now())
	if err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d computeNextRun error: %v", plan.ID, err)
		return
	}

	if err := s.planRepo.UpdateAfterRun(ctx, plan.ID, time.Now(), nextRun); err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d UpdateAfterRun error: %v", plan.ID, err)
	}
}

func (s *ScheduledTestRunnerService) runOneAccountProbePlan(ctx context.Context, plan *ScheduledTestPlan) {
	if s.accountProbeRunner == nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d account probe runner is not configured", plan.ID)
		return
	}
	probeReq := AccountProbeRunRequest{
		AccountID:          plan.AccountID,
		Profile:            plan.ProbeMode,
		Model:              plan.ModelID,
		RequestMode:        plan.ProbeRequestMode,
		IncludeLongContext: plan.ProbeLongContext,
	}
	run, err := s.accountProbeRunner.Start(ctx, probeReq)
	if err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d account probe start error: %v", plan.ID, err)
		return
	}
	started := time.Now()
	probeResult, err := s.accountProbeRunner.RunExisting(ctx, run, probeReq)
	finished := time.Now()
	result := &ScheduledTestResult{
		Status:            probeResult.Status,
		ResponseText:      probeResult.Summary,
		ErrorMessage:      probeResult.ErrorMessage,
		LatencyMs:         int64(probeResult.Latency.AvgMillis),
		FirstTokenMs:      probeResult.FirstTokenMillis,
		AccountProbeRunID: probeResult.ID,
		StartedAt:         started,
		FinishedAt:        finished,
	}
	if err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d account probe run error: %v", plan.ID, err)
		if result.ErrorMessage == "" {
			result.ErrorMessage = err.Error()
		}
		result.Status = AccountProbeStatusFailed
	}
	if result.Status == "" {
		result.Status = AccountProbeStatusFailed
	}
	if err := s.scheduledSvc.SaveResult(ctx, plan.ID, plan.MaxResults, result); err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d SaveResult error: %v", plan.ID, err)
	}
	nextRun, err := computeNextRun(plan.CronExpression, time.Now())
	if err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d computeNextRun error: %v", plan.ID, err)
		return
	}
	if err := s.planRepo.UpdateAfterRun(ctx, plan.ID, time.Now(), nextRun); err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d UpdateAfterRun error: %v", plan.ID, err)
	}
}

// tryRecoverAccount attempts to recover an account from recoverable runtime state.
func (s *ScheduledTestRunnerService) tryRecoverAccount(ctx context.Context, accountID int64, planID int64) {
	if s.rateLimitSvc == nil {
		return
	}

	recovery, err := s.rateLimitSvc.RecoverAccountAfterSuccessfulTest(ctx, accountID)
	if err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d auto-recover failed: %v", planID, err)
		return
	}
	if recovery == nil {
		return
	}

	if recovery.ClearedError {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d auto-recover: account=%d recovered from error status", planID, accountID)
	}
	if recovery.ClearedRateLimit {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d auto-recover: account=%d cleared rate-limit/runtime state", planID, accountID)
	}
}
