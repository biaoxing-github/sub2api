//go:build unit

package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAccountBatchTestNonAPIKeySkipsAPIKeyAndClassifies401(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now()
	adminSvc := &stubAdminService{
		accounts: []service.Account{
			{ID: 11, Name: "claude-oauth", Platform: service.PlatformAnthropic, Type: service.AccountTypeOAuth, Status: service.StatusActive, CreatedAt: now, UpdatedAt: now},
			{ID: 12, Name: "openai-apikey", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, CreatedAt: now, UpdatedAt: now},
			{ID: 13, Name: "openai-oauth", Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, Status: service.StatusActive, CreatedAt: now, UpdatedAt: now},
		},
	}
	tester := &stubBatchAccountTester{
		results: map[int64]*service.ScheduledTestResult{
			11: {Status: "success", ResponseText: "ok", LatencyMs: 120},
			13: {Status: "failed", ErrorMessage: "Authentication failed (401): token invalid", LatencyMs: 88},
		},
		release: make(chan struct{}),
		started: make(chan int64, 2),
	}
	repo := newStubAccountBatchTestRepository()
	h := &AccountHandler{adminService: adminSvc, batchAccountTester: tester, accountBatchTestRepo: repo}
	router := gin.New()
	router.POST("/api/v1/admin/accounts/batch-test-non-apikey", h.BatchTestNonAPIKey)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/batch-test-non-apikey", bytes.NewBufferString(`{"model_id":"gpt-5.4","concurrency":2}`))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)
	require.Equal(t, 1, adminSvc.lastListAccounts.calls)
	require.Equal(t, "name", adminSvc.lastListAccounts.sortBy)
	require.Equal(t, "asc", adminSvc.lastListAccounts.sortOrder)

	var body struct {
		Code int `json:"code"`
		Data struct {
			ID                int64  `json:"id"`
			Status            string `json:"status"`
			Total             int    `json:"total"`
			SuccessCount      int    `json:"success_count"`
			FailedCount       int    `json:"failed_count"`
			UnauthorizedCount int    `json:"unauthorized_count"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, 0, body.Code)
	require.Equal(t, int64(1001), body.Data.ID)
	require.Equal(t, service.AccountBatchTestStatusRunning, body.Data.Status)
	require.Equal(t, 2, body.Data.Total)

	startedIDs := []int64{<-tester.started, <-tester.started}
	require.ElementsMatch(t, []int64{11, 13}, startedIDs)
	close(tester.release)
	require.Eventually(t, func() bool {
		run, items, err := repo.GetAccountBatchTestRun(context.Background(), 1001)
		if err != nil {
			return false
		}
		return run.Status == service.AccountBatchTestStatusPartial &&
			run.SuccessCount == 1 &&
			run.FailedCount == 1 &&
			run.UnauthorizedCount == 1 &&
			len(items) == 2 &&
			items[1].Status == service.AccountBatchTestItemStatusFailed &&
			items[1].Category == "unauthorized"
	}, time.Second, 10*time.Millisecond)
	require.ElementsMatch(t, []int64{11, 13}, tester.calledIDsSnapshot())
}

func TestAccountBatchTestNonAPIKeyListAndDetail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newStubAccountBatchTestRepository()
	createdAt := time.Now().Add(-time.Minute)
	finishedAt := time.Now()
	repo.runs[77] = service.AccountBatchTestRun{
		ID: 77, Status: service.AccountBatchTestStatusSuccess, ModelID: "gpt-5.4",
		Total: 1, SuccessCount: 1, CreatedAt: createdAt, FinishedAt: &finishedAt,
	}
	repo.items[77] = []service.AccountBatchTestItem{
		{ID: 1, RunID: 77, AccountID: 11, AccountName: "claude-oauth", Platform: "anthropic", Type: "oauth", Status: service.AccountBatchTestItemStatusSuccess, Category: "ok", LatencyMs: 88},
	}
	h := &AccountHandler{accountBatchTestRepo: repo}
	router := gin.New()
	router.GET("/api/v1/admin/accounts/batch-test-runs", h.ListBatchTestRuns)
	router.GET("/api/v1/admin/accounts/batch-test-runs/:run_id", h.GetBatchTestRun)

	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/batch-test-runs?page=1&page_size=20", nil))
	require.Equal(t, http.StatusOK, listRec.Code)
	require.Contains(t, listRec.Body.String(), `"id":77`)

	detailRec := httptest.NewRecorder()
	router.ServeHTTP(detailRec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/batch-test-runs/77", nil))
	require.Equal(t, http.StatusOK, detailRec.Code)
	require.Contains(t, detailRec.Body.String(), `"account_name":"claude-oauth"`)
}

type stubBatchAccountTester struct {
	mu        sync.Mutex
	results   map[int64]*service.ScheduledTestResult
	calledIDs []int64
	started   chan int64
	release   chan struct{}
}

func (s *stubBatchAccountTester) RunTestBackground(ctx context.Context, accountID int64, modelID string) (*service.ScheduledTestResult, error) {
	if s.started != nil {
		s.started <- accountID
	}
	if s.release != nil {
		select {
		case <-s.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	s.mu.Lock()
	s.calledIDs = append(s.calledIDs, accountID)
	s.mu.Unlock()
	if result, ok := s.results[accountID]; ok {
		return result, nil
	}
	return &service.ScheduledTestResult{Status: "failed", ErrorMessage: "missing stub result"}, nil
}

func (s *stubBatchAccountTester) calledIDsSnapshot() []int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]int64(nil), s.calledIDs...)
}

type stubAccountBatchTestRepository struct {
	mu     sync.Mutex
	nextID int64
	runs   map[int64]service.AccountBatchTestRun
	items  map[int64][]service.AccountBatchTestItem
}

func newStubAccountBatchTestRepository() *stubAccountBatchTestRepository {
	return &stubAccountBatchTestRepository{
		nextID: 1001,
		runs:   make(map[int64]service.AccountBatchTestRun),
		items:  make(map[int64][]service.AccountBatchTestItem),
	}
}

func (r *stubAccountBatchTestRepository) CreateAccountBatchTestRun(ctx context.Context, run *service.AccountBatchTestRun) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	run.ID = r.nextID
	r.nextID++
	if run.CreatedAt.IsZero() {
		run.CreatedAt = time.Now()
	}
	r.runs[run.ID] = *run
	return nil
}

func (r *stubAccountBatchTestRepository) CreateAccountBatchTestItems(ctx context.Context, runID int64, items []service.AccountBatchTestItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := append([]service.AccountBatchTestItem(nil), items...)
	for i := range cp {
		cp[i].ID = int64(i + 1)
		cp[i].RunID = runID
	}
	r.items[runID] = cp
	return nil
}

func (r *stubAccountBatchTestRepository) UpdateAccountBatchTestItem(ctx context.Context, item service.AccountBatchTestItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := r.items[item.RunID]
	for i := range items {
		if items[i].AccountID == item.AccountID {
			items[i] = item
			items[i].ID = int64(i + 1)
			r.items[item.RunID] = items
			return nil
		}
	}
	r.items[item.RunID] = append(items, item)
	return nil
}

func (r *stubAccountBatchTestRepository) UpdateAccountBatchTestRun(ctx context.Context, run *service.AccountBatchTestRun) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runs[run.ID] = *run
	return nil
}

func (r *stubAccountBatchTestRepository) ListAccountBatchTestRuns(ctx context.Context, filter service.AccountBatchTestRunFilter) ([]service.AccountBatchTestRun, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]service.AccountBatchTestRun, 0, len(r.runs))
	for _, run := range r.runs {
		out = append(out, run)
	}
	return out, len(out), nil
}

func (r *stubAccountBatchTestRepository) GetAccountBatchTestRun(ctx context.Context, runID int64) (*service.AccountBatchTestRun, []service.AccountBatchTestItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	run, ok := r.runs[runID]
	if !ok {
		return nil, nil, service.ErrAccountNotFound
	}
	items := append([]service.AccountBatchTestItem(nil), r.items[runID]...)
	return &run, items, nil
}
