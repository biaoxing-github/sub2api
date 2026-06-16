//go:build unit

package admin

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type accountProbeReportHTTPServiceStub struct {
	blockingAccountProbeService
	reportFilter service.AccountProbeReportFilter
	reportPage   service.AccountProbeReportPage
	reportItem   *service.AccountProbeReportItem
	deleteIDs    []int64
	deleteResult service.AccountProbeReportDeleteResult
	rankingLimit int
	rankingItems []service.AccountProbeRankingItem
	startedRuns  []service.AccountProbeRunRequest
	runReqs      []service.AccountProbeRunRequest
	activeRuns   int
	maxActive    int
	runStarted   chan struct{}
	releaseRun   chan struct{}
	mu           sync.Mutex
}

func (s *accountProbeReportHTTPServiceStub) Start(ctx context.Context, req service.AccountProbeRunRequest) (service.AccountProbeResult, error) {
	s.mu.Lock()
	s.startedRuns = append(s.startedRuns, req)
	id := int64(len(s.startedRuns))
	s.mu.Unlock()
	return service.AccountProbeResult{
		ID:          id,
		AccountID:   req.AccountID,
		Profile:     req.Profile,
		Status:      service.AccountProbeStatusRunning,
		Model:       req.Model,
		RequestMode: req.RequestMode,
	}, nil
}

func (s *accountProbeReportHTTPServiceStub) RunExisting(ctx context.Context, run service.AccountProbeResult, req service.AccountProbeRunRequest) (service.AccountProbeResult, error) {
	s.mu.Lock()
	s.runReqs = append(s.runReqs, req)
	s.activeRuns++
	if s.activeRuns > s.maxActive {
		s.maxActive = s.activeRuns
	}
	started := s.runStarted
	release := s.releaseRun
	s.mu.Unlock()
	if started != nil {
		started <- struct{}{}
	}
	if release != nil {
		select {
		case <-release:
		case <-ctx.Done():
		}
	}
	s.mu.Lock()
	s.activeRuns--
	s.mu.Unlock()
	run.Status = service.AccountProbeStatusSuccess
	run.SuccessCount = max(run.RequestCount, 1)
	return run, nil
}

func (s *accountProbeReportHTTPServiceStub) ListReports(ctx context.Context, filter service.AccountProbeReportFilter) (service.AccountProbeReportPage, error) {
	s.reportFilter = filter
	return s.reportPage, nil
}

func (s *accountProbeReportHTTPServiceStub) GetReport(ctx context.Context, runID int64) (*service.AccountProbeReportItem, error) {
	if s.reportItem == nil {
		return nil, service.ErrAccountNotFound
	}
	item := *s.reportItem
	item.ID = runID
	return &item, nil
}

func (s *accountProbeReportHTTPServiceStub) DeleteReports(ctx context.Context, runIDs []int64) (service.AccountProbeReportDeleteResult, error) {
	s.deleteIDs = append([]int64{}, runIDs...)
	if s.deleteResult.RequestedCount == 0 {
		s.deleteResult.RequestedCount = len(runIDs)
	}
	return s.deleteResult, nil
}

func (s *accountProbeReportHTTPServiceStub) ListRanking(ctx context.Context, limit int) ([]service.AccountProbeRankingItem, error) {
	s.rankingLimit = limit
	return s.rankingItems, nil
}

func TestAccountProbeReportListParsesFiltersAndReturnsPage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now()
	probeSvc := &accountProbeReportHTTPServiceStub{
		reportPage: service.AccountProbeReportPage{
			Items: []service.AccountProbeReportItem{{
				AccountName: "encore",
				AccountProbeResult: service.AccountProbeResult{
					ID: 7, AccountID: 181, Status: service.AccountProbeStatusSuccess, RequestCount: 3, SuccessCount: 3, CreatedAt: now,
				},
				Score: 96,
				Grade: service.AccountProbeGradeExcellent,
			}},
			Total: 1, Page: 2, PageSize: 10,
		},
	}
	h := &AccountHandler{accountProbeService: probeSvc}
	router := gin.New()
	router.GET("/api/v1/admin/account-probe-runs", h.ListProbeReportRuns)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/account-probe-runs?account_id=181&status=success&mode=quick&request_mode=stream&model=gpt-5.4&keyword=encore&sort=score&order=asc&page=2&page_size=10", nil)

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(181), probeSvc.reportFilter.AccountID)
	require.Equal(t, "success", probeSvc.reportFilter.Status)
	require.Equal(t, "quick", probeSvc.reportFilter.Profile)
	require.Equal(t, "stream", probeSvc.reportFilter.RequestMode)
	require.Equal(t, "gpt-5.4", probeSvc.reportFilter.Model)
	require.Equal(t, "encore", probeSvc.reportFilter.Keyword)
	require.Equal(t, "score", probeSvc.reportFilter.Sort)
	require.Equal(t, "asc", probeSvc.reportFilter.Order)
	require.Equal(t, 2, probeSvc.reportFilter.Page)
	require.Equal(t, 10, probeSvc.reportFilter.PageSize)
	require.Contains(t, rec.Body.String(), `"account_name":"encore"`)
	require.Contains(t, rec.Body.String(), `"score":96`)
}

func TestAccountProbeReportGetReturnsDetail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	probeSvc := &accountProbeReportHTTPServiceStub{
		reportItem: &service.AccountProbeReportItem{
			AccountName:        "foyeapi",
			AccountProbeResult: service.AccountProbeResult{AccountID: 181, Status: service.AccountProbeStatusFailed},
			Score:              38,
			Grade:              service.AccountProbeGradePoor,
			PenaltyItems:       []string{"unexpected EOF，-10"},
		},
	}
	h := &AccountHandler{accountProbeService: probeSvc}
	router := gin.New()
	router.GET("/api/v1/admin/account-probe-runs/:run_id", h.GetProbeReportRun)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/account-probe-runs/99", nil)

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"id":99`)
	require.Contains(t, rec.Body.String(), `"account_name":"foyeapi"`)
	require.Contains(t, rec.Body.String(), `"penalty_items":["unexpected EOF，-10"]`)
}

func TestAccountProbeReportRankingReturnsAggregateItems(t *testing.T) {
	gin.SetMode(gin.TestMode)
	probeSvc := &accountProbeReportHTTPServiceStub{
		rankingItems: []service.AccountProbeRankingItem{{
			AccountID:    181,
			AccountName:  "foyeapi",
			RunCount:     4,
			AverageScore: 91.5,
			LatestScore:  94,
		}},
	}
	h := &AccountHandler{accountProbeService: probeSvc}
	router := gin.New()
	router.GET("/api/v1/admin/account-probe-runs/ranking", h.ListProbeReportRanking)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/account-probe-runs/ranking?limit=12", nil)

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 12, probeSvc.rankingLimit)
	require.Contains(t, rec.Body.String(), `"account_name":"foyeapi"`)
	require.Contains(t, rec.Body.String(), `"average_score":91.5`)
}

func TestAccountProbeReportBatchDeleteDeduplicatesRunIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	probeSvc := &accountProbeReportHTTPServiceStub{
		deleteResult: service.AccountProbeReportDeleteResult{
			RequestedCount:      2,
			DeletedCount:        1,
			SkippedRunningCount: 1,
		},
	}
	h := &AccountHandler{accountProbeService: probeSvc}
	router := gin.New()
	router.DELETE("/api/v1/admin/account-probe-runs", h.DeleteProbeReportRuns)

	body := `{"run_ids":[91,91,92,0,-1]}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/account-probe-runs", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, []int64{91, 92}, probeSvc.deleteIDs)
	require.Contains(t, rec.Body.String(), `"deleted_count":1`)
	require.Contains(t, rec.Body.String(), `"skipped_running_count":1`)
}

func TestAccountProbeReportBatchCreateDeduplicatesAccounts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	probeSvc := &accountProbeReportHTTPServiceStub{}
	h := &AccountHandler{accountProbeService: probeSvc}
	router := gin.New()
	router.POST("/api/v1/admin/account-probe-runs/batch", h.BatchCreateProbeReportRuns)

	body := `{"account_ids":[181,181,182],"mode":"quick","model":"gpt-5.4","request_mode":"stream","codex_stability":true}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/account-probe-runs/batch", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)
	require.Contains(t, rec.Body.String(), `"accepted_count":2`)
	require.Len(t, probeSvc.startedRuns, 2)
	require.Equal(t, int64(181), probeSvc.startedRuns[0].AccountID)
	require.Equal(t, int64(182), probeSvc.startedRuns[1].AccountID)
	require.Equal(t, "stream", probeSvc.startedRuns[0].RequestMode)
	require.False(t, probeSvc.startedRuns[0].IncludeCodexStability)
	require.True(t, probeSvc.startedRuns[0].ManualTrigger)
	require.True(t, probeSvc.startedRuns[1].ManualTrigger)
}

func TestAccountProbeReportBatchCreateLimitsBackgroundConcurrency(t *testing.T) {
	gin.SetMode(gin.TestMode)
	probeSvc := &accountProbeReportHTTPServiceStub{
		runStarted: make(chan struct{}, 4),
		releaseRun: make(chan struct{}),
	}
	h := &AccountHandler{accountProbeService: probeSvc}
	router := gin.New()
	router.POST("/api/v1/admin/account-probe-runs/batch", h.BatchCreateProbeReportRuns)

	body := `{"account_ids":[181,182,183,184],"mode":"quick","model":"gpt-5.4","request_mode":"stream"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/account-probe-runs/batch", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusAccepted, rec.Code)

	require.Eventually(t, func() bool {
		probeSvc.mu.Lock()
		defer probeSvc.mu.Unlock()
		return probeSvc.activeRuns == 2
	}, time.Second, 10*time.Millisecond)
	probeSvc.mu.Lock()
	require.Equal(t, 2, probeSvc.maxActive)
	probeSvc.mu.Unlock()
	close(probeSvc.releaseRun)
	require.Eventually(t, func() bool {
		probeSvc.mu.Lock()
		defer probeSvc.mu.Unlock()
		return probeSvc.maxActive == 2 && probeSvc.activeRuns == 0
	}, time.Second, 10*time.Millisecond)
	probeSvc.mu.Lock()
	require.Len(t, probeSvc.runReqs, 4)
	for _, got := range probeSvc.runReqs {
		require.True(t, got.ManualTrigger)
	}
	probeSvc.mu.Unlock()
}
