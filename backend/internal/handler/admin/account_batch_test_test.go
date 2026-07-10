//go:build unit

package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestAccountBatchTestNonAPIKeyPersistsFirstTokenMs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now()
	firstTokenMs := 345
	adminSvc := &stubAdminService{
		accounts: []service.Account{
			{ID: 14, Name: "openai-oauth-ttft", Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, Status: service.StatusActive, CreatedAt: now, UpdatedAt: now},
		},
	}
	tester := &stubBatchAccountTester{
		results: map[int64]*service.ScheduledTestResult{
			14: {Status: "success", ResponseText: "ok", LatencyMs: 900, FirstTokenMs: &firstTokenMs},
		},
	}
	repo := newStubAccountBatchTestRepository()
	h := &AccountHandler{adminService: adminSvc, batchAccountTester: tester, accountBatchTestRepo: repo}
	router := gin.New()
	router.POST("/api/v1/admin/accounts/batch-test-non-apikey", h.BatchTestNonAPIKey)
	router.GET("/api/v1/admin/accounts/batch-test-runs/:run_id", h.GetBatchTestRun)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/batch-test-non-apikey", bytes.NewBufferString(`{"model_id":"gpt-5.5","concurrency":1}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusAccepted, rec.Code)

	require.Eventually(t, func() bool {
		_, items, err := repo.GetAccountBatchTestRun(context.Background(), 1001)
		return err == nil && len(items) == 1 && items[0].FirstTokenMs != nil && *items[0].FirstTokenMs == firstTokenMs
	}, time.Second, 10*time.Millisecond)

	detailRec := httptest.NewRecorder()
	router.ServeHTTP(detailRec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/batch-test-runs/1001", nil))
	require.Equal(t, http.StatusOK, detailRec.Code)
	require.Contains(t, detailRec.Body.String(), `"first_token_ms":345`)
}

func TestAccountBatchTestNonAPIKeyClassifiesAndCounts429(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now()
	adminSvc := &stubAdminService{
		accounts: []service.Account{
			{ID: 21, Name: "openai-oauth-ok", Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, Status: service.StatusActive, CreatedAt: now, UpdatedAt: now},
			{ID: 22, Name: "openai-oauth-429", Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, Status: service.StatusActive, CreatedAt: now, UpdatedAt: now},
		},
	}
	tester := &stubBatchAccountTester{
		results: map[int64]*service.ScheduledTestResult{
			21: {Status: "success", ResponseText: "ok", LatencyMs: 80},
			22: {Status: "failed", ErrorMessage: "API returned 429: rate limit exceeded", LatencyMs: 92},
		},
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

	require.Eventually(t, func() bool {
		run, items, err := repo.GetAccountBatchTestRun(context.Background(), 1001)
		if err != nil {
			return false
		}
		return run.Status == service.AccountBatchTestStatusPartial &&
			run.SuccessCount == 1 &&
			run.FailedCount == 1 &&
			run.UnauthorizedCount == 0 &&
			run.RateLimitedCount == 1 &&
			len(items) == 2 &&
			items[1].Status == service.AccountBatchTestItemStatusFailed &&
			items[1].Category == "rate_limited"
	}, time.Second, 10*time.Millisecond)
}

func TestAccountBatchTestNonAPIKeyUsesUnifiedClassifierForCloudflareAndCircuit(t *testing.T) {
	require.Equal(t, "cloudflare_waf", classifyAccountTestError("<html><title>Just a moment...</title>cloudflare</html>"))
	require.Equal(t, "client_ip_circuit_open", classifyAccountTestError(`API returned 429: {"error":{"code":"client_ip_error_circuit_open"}}`))
	require.Equal(t, "header_timeout", classifyAccountTestError("timed out waiting for OpenAI upstream response headers"))
}

func TestAccountBatchTestNonAPIKeyUsesSelectedAccountsAndSkipsAPIKeys(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now()
	adminSvc := &stubAdminService{
		accounts: []service.Account{
			{ID: 31, Name: "selected-oauth", Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, Status: service.StatusActive, CreatedAt: now, UpdatedAt: now},
			{ID: 32, Name: "selected-api-key", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, CreatedAt: now, UpdatedAt: now},
			{ID: 33, Name: "selected-oauth-two", Platform: service.PlatformAnthropic, Type: service.AccountTypeOAuth, Status: service.StatusActive, CreatedAt: now, UpdatedAt: now},
		},
	}
	tester := &stubBatchAccountTester{
		results: map[int64]*service.ScheduledTestResult{
			31: {Status: "success", ResponseText: "ok", LatencyMs: 80},
			33: {Status: "success", ResponseText: "ok", LatencyMs: 90},
		},
	}
	repo := newStubAccountBatchTestRepository()
	h := &AccountHandler{adminService: adminSvc, batchAccountTester: tester, accountBatchTestRepo: repo}
	router := gin.New()
	router.POST("/api/v1/admin/accounts/batch-test-non-apikey", h.BatchTestNonAPIKey)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/batch-test-non-apikey", bytes.NewBufferString(`{"model_id":"gpt-5.4","account_ids":[31,32,33,31],"concurrency":99,"platform":"ignored"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusAccepted, rec.Code)
	require.Equal(t, 0, adminSvc.lastListAccounts.calls)
	require.Equal(t, []int64{31, 32, 33}, adminSvc.lastGetAccountsByIDs.ids)

	var body struct {
		Data struct {
			Total       int `json:"total"`
			Concurrency int `json:"concurrency"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, 2, body.Data.Total)
	require.Equal(t, 20, body.Data.Concurrency)
	require.Eventually(t, func() bool {
		return len(tester.calledIDsSnapshot()) == 2
	}, time.Second, 10*time.Millisecond)
	require.ElementsMatch(t, []int64{31, 33}, tester.calledIDsSnapshot())
}

func TestAccountBatchTestNonAPIKeyUsesGroupFilterAndDefaultConcurrency(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now()
	adminSvc := &stubAdminService{
		accounts: []service.Account{
			{ID: 41, Name: "group-oauth", Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, Status: service.StatusActive, CreatedAt: now, UpdatedAt: now},
		},
	}
	tester := &stubBatchAccountTester{results: map[int64]*service.ScheduledTestResult{
		41: {Status: "success", ResponseText: "ok", LatencyMs: 80},
	}}
	repo := newStubAccountBatchTestRepository()
	h := &AccountHandler{adminService: adminSvc, batchAccountTester: tester, accountBatchTestRepo: repo}
	router := gin.New()
	router.POST("/api/v1/admin/accounts/batch-test-non-apikey", h.BatchTestNonAPIKey)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/batch-test-non-apikey", bytes.NewBufferString(`{"group":"12"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)
	require.Equal(t, 1, adminSvc.lastListAccounts.calls)
	require.Equal(t, int64(12), adminSvc.lastListAccounts.groupID)
	var body struct {
		Data struct {
			Concurrency int `json:"concurrency"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, 5, body.Data.Concurrency)
	require.Eventually(t, func() bool {
		models := tester.calledModelsSnapshot()
		return len(models) == 1 && models[0] == "gpt-5.6-terra"
	}, time.Second, 10*time.Millisecond)
}

func TestAccountBatchTestNonAPIKeyPausesWhenLiveTrafficUsesAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now()
	adminSvc := &stubAdminService{
		accounts: []service.Account{
			{ID: 46, Name: "busy-openai-oauth", Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, Status: service.StatusActive, CreatedAt: now, UpdatedAt: now},
		},
	}
	tester := &stubBatchAccountTester{results: map[int64]*service.ScheduledTestResult{
		46: {Status: "success", ResponseText: "should not run", LatencyMs: 80},
	}}
	repo := newStubAccountBatchTestRepository()
	h := &AccountHandler{
		adminService:         adminSvc,
		batchAccountTester:   tester,
		accountBatchTestRepo: repo,
		concurrencyService:   service.NewConcurrencyService(&stubBatchConcurrencyCache{loadMap: map[int64]*service.AccountLoadInfo{46: {AccountID: 46, CurrentConcurrency: 1, WaitingCount: 2}}}),
	}
	router := gin.New()
	router.POST("/api/v1/admin/accounts/batch-test-non-apikey", h.BatchTestNonAPIKey)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/batch-test-non-apikey", bytes.NewBufferString(`{"concurrency":1}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)
	require.Eventually(t, func() bool {
		run, items, err := repo.GetAccountBatchTestRun(context.Background(), 1001)
		if err != nil || len(items) != 1 {
			return false
		}
		return run.Status == service.AccountBatchTestStatusFailed &&
			run.SuccessCount == 0 &&
			run.FailedCount == 1 &&
			items[0].Status == service.AccountBatchTestItemStatusFailed &&
			items[0].Category == "batch_paused" &&
			strings.Contains(items[0].ErrorMessage, "live traffic")
	}, time.Second, 10*time.Millisecond)
	require.Empty(t, tester.calledIDsSnapshot())
}

func TestAccountBatchTestNonAPIKeyPassesPlanTypeFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now()
	adminSvc := &stubAdminService{
		accounts: []service.Account{
			{ID: 51, Name: "free-oauth", Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, Status: service.StatusActive, CreatedAt: now, UpdatedAt: now, Credentials: map[string]any{"plan_type": "free"}},
		},
	}
	tester := &stubBatchAccountTester{results: map[int64]*service.ScheduledTestResult{
		51: {Status: "success", ResponseText: "ok", LatencyMs: 80},
	}}
	repo := newStubAccountBatchTestRepository()
	h := &AccountHandler{adminService: adminSvc, batchAccountTester: tester, accountBatchTestRepo: repo}
	router := gin.New()
	router.POST("/api/v1/admin/accounts/batch-test-non-apikey", h.BatchTestNonAPIKey)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/batch-test-non-apikey", bytes.NewBufferString(`{"plan_type":"free"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)
	require.Equal(t, 1, adminSvc.lastListAccounts.calls)
	require.Equal(t, "free", adminSvc.lastListAccounts.planType)
	require.Eventually(t, func() bool {
		models := tester.calledModelsSnapshot()
		return len(models) == 1 && models[0] == "gpt-5.6-terra"
	}, time.Second, 10*time.Millisecond)
}

func TestAccountBatchTestNonAPIKeyListAndDetail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newStubAccountBatchTestRepository()
	createdAt := time.Now().Add(-time.Minute)
	finishedAt := time.Now()
	repo.runs[77] = service.AccountBatchTestRun{
		ID: 77, Status: service.AccountBatchTestStatusSuccess, ModelID: "gpt-5.4",
		Total: 1, SuccessCount: 1, UnauthorizedCount: 99, CreatedAt: createdAt, FinishedAt: &finishedAt,
	}
	repo.items[77] = []service.AccountBatchTestItem{
		{ID: 1, RunID: 77, AccountID: 11, AccountName: "claude-oauth", Platform: "anthropic", Type: "oauth", Status: service.AccountBatchTestItemStatusSuccess, Category: "ok", LatencyMs: 88},
		{ID: 2, RunID: 77, AccountID: 12, AccountName: "openai-429", Platform: "openai", Type: "oauth", Status: service.AccountBatchTestItemStatusFailed, Category: "rate_limited", LatencyMs: 66},
		{ID: 3, RunID: 77, AccountID: 13, AccountName: "openai-401", Platform: "openai", Type: "oauth", Status: service.AccountBatchTestItemStatusFailed, Category: "unauthorized", LatencyMs: 55},
		{ID: 4, RunID: 77, AccountID: 14, AccountName: "legacy-429", Platform: "openai", Type: "oauth", Status: service.AccountBatchTestItemStatusFailed, Category: "error", ErrorMessage: "API returned 429: usage_limit_reached", LatencyMs: 44},
		{ID: 5, RunID: 77, AccountID: 15, AccountName: "legacy-401", Platform: "openai", Type: "oauth", Status: service.AccountBatchTestItemStatusFailed, Category: "error", ErrorMessage: "Authentication failed (401): token invalid", LatencyMs: 33},
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
	require.Contains(t, detailRec.Body.String(), `"rate_limited_count":2`)
	require.Contains(t, detailRec.Body.String(), `"unauthorized_count":1`)

	filteredRec := httptest.NewRecorder()
	router.ServeHTTP(filteredRec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/batch-test-runs/77?category=rate_limited", nil))
	require.Equal(t, http.StatusOK, filteredRec.Code)
	require.Contains(t, filteredRec.Body.String(), `"account_name":"openai-429"`)
	require.Contains(t, filteredRec.Body.String(), `"account_name":"legacy-429"`)
	require.NotContains(t, filteredRec.Body.String(), `"account_name":"claude-oauth"`)
	require.NotContains(t, filteredRec.Body.String(), `"account_name":"openai-401"`)

	unauthorizedRec := httptest.NewRecorder()
	router.ServeHTTP(unauthorizedRec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/batch-test-runs/77?category=unauthorized", nil))
	require.Equal(t, http.StatusOK, unauthorizedRec.Code)
	require.Contains(t, unauthorizedRec.Body.String(), `"account_name":"openai-401"`)
	require.Contains(t, unauthorizedRec.Body.String(), `"account_name":"legacy-401"`)
	require.NotContains(t, unauthorizedRec.Body.String(), `"account_name":"openai-429"`)
}

type stubBatchAccountTester struct {
	mu           sync.Mutex
	results      map[int64]*service.ScheduledTestResult
	calledIDs    []int64
	calledModels []string
	started      chan int64
	release      chan struct{}
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
	s.calledModels = append(s.calledModels, modelID)
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

func (s *stubBatchAccountTester) calledModelsSnapshot() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.calledModels...)
}

type stubAccountBatchTestRepository struct {
	mu     sync.Mutex
	nextID int64
	runs   map[int64]service.AccountBatchTestRun
	items  map[int64][]service.AccountBatchTestItem
}

type stubBatchConcurrencyCache struct {
	loadMap map[int64]*service.AccountLoadInfo
}

func (c *stubBatchConcurrencyCache) AcquireAccountSlot(context.Context, int64, int, string) (bool, error) {
	return true, nil
}

func (c *stubBatchConcurrencyCache) ReleaseAccountSlot(context.Context, int64, string) error {
	return nil
}

func (c *stubBatchConcurrencyCache) GetAccountConcurrency(context.Context, int64) (int, error) {
	return 0, nil
}

func (c *stubBatchConcurrencyCache) GetAccountConcurrencyBatch(_ context.Context, accountIDs []int64) (map[int64]int, error) {
	out := make(map[int64]int, len(accountIDs))
	for _, id := range accountIDs {
		if load := c.loadMap[id]; load != nil {
			out[id] = load.CurrentConcurrency
		}
	}
	return out, nil
}

func (c *stubBatchConcurrencyCache) IncrementAccountWaitCount(context.Context, int64, int) (bool, error) {
	return true, nil
}

func (c *stubBatchConcurrencyCache) DecrementAccountWaitCount(context.Context, int64) error {
	return nil
}

func (c *stubBatchConcurrencyCache) GetAccountWaitingCount(context.Context, int64) (int, error) {
	return 0, nil
}

func (c *stubBatchConcurrencyCache) AcquireUserSlot(context.Context, int64, int, string) (bool, error) {
	return true, nil
}

func (c *stubBatchConcurrencyCache) ReleaseUserSlot(context.Context, int64, string) error {
	return nil
}

func (c *stubBatchConcurrencyCache) GetUserConcurrency(context.Context, int64) (int, error) {
	return 0, nil
}

func (c *stubBatchConcurrencyCache) IncrementWaitCount(context.Context, int64, int) (bool, error) {
	return true, nil
}

func (c *stubBatchConcurrencyCache) DecrementWaitCount(context.Context, int64) error {
	return nil
}

func (c *stubBatchConcurrencyCache) GetAccountsLoadBatch(_ context.Context, accounts []service.AccountWithConcurrency) (map[int64]*service.AccountLoadInfo, error) {
	out := make(map[int64]*service.AccountLoadInfo, len(accounts))
	for _, account := range accounts {
		if load := c.loadMap[account.ID]; load != nil {
			cp := *load
			out[account.ID] = &cp
			continue
		}
		out[account.ID] = &service.AccountLoadInfo{AccountID: account.ID}
	}
	return out, nil
}

func (c *stubBatchConcurrencyCache) GetUsersLoadBatch(context.Context, []service.UserWithConcurrency) (map[int64]*service.UserLoadInfo, error) {
	return map[int64]*service.UserLoadInfo{}, nil
}

func (c *stubBatchConcurrencyCache) CleanupExpiredAccountSlots(context.Context, int64) error {
	return nil
}

func (c *stubBatchConcurrencyCache) CleanupExpiredAccountSlotKeys(context.Context) error {
	return nil
}

func (c *stubBatchConcurrencyCache) CleanupStaleProcessSlots(context.Context, string) error {
	return nil
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
	run.UnauthorizedCount = 0
	run.RateLimitedCount = 0
	for _, item := range items {
		if item.Category == "unauthorized" {
			run.UnauthorizedCount++
		}
		if accountBatchTestItemMatchesCategory(item, "rate_limited") {
			run.RateLimitedCount++
		}
	}
	return &run, items, nil
}
