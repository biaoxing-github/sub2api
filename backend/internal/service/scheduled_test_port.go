package service

import (
	"context"
	"time"
)

const (
	ScheduledTestTaskTypeAccountTest  = "account_test"
	ScheduledTestTaskTypeAccountProbe = "account_probe"
)

// ScheduledTestPlan represents a scheduled test plan domain model.
type ScheduledTestPlan struct {
	ID                  int64      `json:"id"`
	AccountID           int64      `json:"account_id"`
	TaskType            string     `json:"task_type"`
	ModelID             string     `json:"model_id"`
	CronExpression      string     `json:"cron_expression"`
	Enabled             bool       `json:"enabled"`
	MaxResults          int        `json:"max_results"`
	AutoRecover         bool       `json:"auto_recover"`
	ProbeMode           string     `json:"probe_mode"`
	ProbeRequestMode    string     `json:"probe_request_mode"`
	ProbeCodexStability bool       `json:"probe_codex_stability"`
	ProbeLongContext    bool       `json:"probe_long_context"`
	LastRunAt           *time.Time `json:"last_run_at"`
	NextRunAt           *time.Time `json:"next_run_at"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// ScheduledTestResult represents a single test execution result.
type ScheduledTestResult struct {
	ID                int64     `json:"id"`
	PlanID            int64     `json:"plan_id"`
	Status            string    `json:"status"`
	ResponseText      string    `json:"response_text"`
	ErrorMessage      string    `json:"error_message"`
	LatencyMs         int64     `json:"latency_ms"`
	FirstTokenMs      *int      `json:"first_token_ms,omitempty"`
	AccountProbeRunID int64     `json:"account_probe_run_id,omitempty"`
	StartedAt         time.Time `json:"started_at"`
	FinishedAt        time.Time `json:"finished_at"`
	CreatedAt         time.Time `json:"created_at"`
}

// ScheduledTestRunnerSnapshot 描述定时体检 runner 的内存运行态。
type ScheduledTestRunnerSnapshot struct {
	Name               string     `json:"name"`
	IntervalMillis     int64      `json:"interval_ms"`
	InitialDelayMillis int64      `json:"initial_delay_ms"`
	Running            bool       `json:"running"`
	LastStartedAt      *time.Time `json:"last_started_at,omitempty"`
	LastFinishedAt     *time.Time `json:"last_finished_at,omitempty"`
	LastSuccessAt      *time.Time `json:"last_success_at,omitempty"`
	LastErrorAt        *time.Time `json:"last_error_at,omitempty"`
	LastError          string     `json:"last_error,omitempty"`
	LastDurationMillis int64      `json:"last_duration_ms"`
	MaxDurationMillis  int64      `json:"max_duration_ms"`
	RunCount           int64      `json:"run_count"`
	SuccessCount       int64      `json:"success_count"`
	FailureCount       int64      `json:"failure_count"`
	SkippedCount       int64      `json:"skipped_count"`
}

// ScheduledTestPlanRepository defines the data access interface for test plans.
type ScheduledTestPlanRepository interface {
	Create(ctx context.Context, plan *ScheduledTestPlan) (*ScheduledTestPlan, error)
	GetByID(ctx context.Context, id int64) (*ScheduledTestPlan, error)
	ListByAccountID(ctx context.Context, accountID int64) ([]*ScheduledTestPlan, error)
	ListDue(ctx context.Context, now time.Time) ([]*ScheduledTestPlan, error)
	Update(ctx context.Context, plan *ScheduledTestPlan) (*ScheduledTestPlan, error)
	Delete(ctx context.Context, id int64) error
	UpdateAfterRun(ctx context.Context, id int64, lastRunAt time.Time, nextRunAt time.Time) error
}

// ScheduledTestResultRepository defines the data access interface for test results.
type ScheduledTestResultRepository interface {
	Create(ctx context.Context, result *ScheduledTestResult) (*ScheduledTestResult, error)
	ListByPlanID(ctx context.Context, planID int64, limit int) ([]*ScheduledTestResult, error)
	PruneOldResults(ctx context.Context, planID int64, keepCount int) error
}
