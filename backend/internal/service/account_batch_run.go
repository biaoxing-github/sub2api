package service

import (
	"context"
	"time"
)

const (
	AccountBatchTestStatusRunning = "running"
	AccountBatchTestStatusSuccess = "success"
	AccountBatchTestStatusPartial = "partial"
	AccountBatchTestStatusFailed  = "failed"

	AccountBatchTestItemStatusPending = "pending"
	AccountBatchTestItemStatusRunning = "running"
	AccountBatchTestItemStatusSuccess = "success"
	AccountBatchTestItemStatusFailed  = "failed"
)

type AccountBatchTestRun struct {
	ID                int64      `json:"id"`
	Status            string     `json:"status"`
	ModelID           string     `json:"model_id"`
	Platform          string     `json:"platform,omitempty"`
	StatusFilter      string     `json:"status_filter,omitempty"`
	Search            string     `json:"search,omitempty"`
	Concurrency       int        `json:"concurrency"`
	Limit             int        `json:"limit"`
	Total             int        `json:"total"`
	SuccessCount      int        `json:"success_count"`
	FailedCount       int        `json:"failed_count"`
	UnauthorizedCount int        `json:"unauthorized_count"`
	RateLimitedCount  int        `json:"rate_limited_count"`
	ErrorMessage      string     `json:"error_message,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	StartedAt         *time.Time `json:"started_at,omitempty"`
	FinishedAt        *time.Time `json:"finished_at,omitempty"`
}

type AccountBatchTestItem struct {
	ID           int64      `json:"id"`
	RunID        int64      `json:"run_id"`
	AccountID    int64      `json:"account_id"`
	AccountName  string     `json:"account_name"`
	Platform     string     `json:"platform"`
	Type         string     `json:"type"`
	GroupID      int64      `json:"group_id,omitempty"`
	Status       string     `json:"status"`
	Category     string     `json:"category"`
	Message      string     `json:"message,omitempty"`
	ErrorMessage string     `json:"error_message,omitempty"`
	LatencyMs    int        `json:"latency_ms,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
}

type AccountBatchTestRunFilter struct {
	Page     int
	PageSize int
	Status   string
	Keyword  string
}

type AccountBatchTestRunPage struct {
	Items    []AccountBatchTestRun `json:"items"`
	Total    int                   `json:"total"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"page_size"`
}

type AccountBatchTestRunDetail struct {
	AccountBatchTestRun
	Items []AccountBatchTestItem `json:"items"`
}

type AccountBatchTestRepository interface {
	CreateAccountBatchTestRun(ctx context.Context, run *AccountBatchTestRun) error
	CreateAccountBatchTestItems(ctx context.Context, runID int64, items []AccountBatchTestItem) error
	UpdateAccountBatchTestItem(ctx context.Context, item AccountBatchTestItem) error
	UpdateAccountBatchTestRun(ctx context.Context, run *AccountBatchTestRun) error
	ListAccountBatchTestRuns(ctx context.Context, filter AccountBatchTestRunFilter) ([]AccountBatchTestRun, int, error)
	GetAccountBatchTestRun(ctx context.Context, runID int64) (*AccountBatchTestRun, []AccountBatchTestItem, error)
}
