//go:build unit

package admin

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type blockingAccountProbeService struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
	req     service.AccountProbeRunRequest
}

func newBlockingAccountProbeService() *blockingAccountProbeService {
	return &blockingAccountProbeService{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (s *blockingAccountProbeService) Start(ctx context.Context, req service.AccountProbeRunRequest) (service.AccountProbeResult, error) {
	s.req = req
	return service.AccountProbeResult{
		ID:           99,
		AccountID:    req.AccountID,
		Profile:      req.Profile,
		Status:       service.AccountProbeStatusRunning,
		Model:        req.Model,
		RequestMode:  req.RequestMode,
		RequestCount: 1,
	}, nil
}

func (s *blockingAccountProbeService) RunExisting(ctx context.Context, run service.AccountProbeResult, req service.AccountProbeRunRequest) (service.AccountProbeResult, error) {
	s.once.Do(func() { close(s.started) })
	select {
	case <-s.release:
	case <-ctx.Done():
		return run, ctx.Err()
	}
	run.Status = service.AccountProbeStatusSuccess
	run.SuccessCount = run.RequestCount
	return run, nil
}

func (s *blockingAccountProbeService) List(ctx context.Context, filter service.AccountProbeHistoryFilter) ([]service.AccountProbeResult, error) {
	return nil, nil
}

func (s *blockingAccountProbeService) Get(ctx context.Context, accountID, runID int64) (*service.AccountProbeResult, error) {
	return nil, nil
}

func (s *blockingAccountProbeService) ListReports(ctx context.Context, filter service.AccountProbeReportFilter) (service.AccountProbeReportPage, error) {
	return service.AccountProbeReportPage{}, nil
}

func (s *blockingAccountProbeService) GetReport(ctx context.Context, runID int64) (*service.AccountProbeReportItem, error) {
	return nil, nil
}

func (s *blockingAccountProbeService) DeleteReports(ctx context.Context, runIDs []int64) (service.AccountProbeReportDeleteResult, error) {
	return service.AccountProbeReportDeleteResult{}, nil
}

func (s *blockingAccountProbeService) ListRanking(ctx context.Context, limit int) ([]service.AccountProbeRankingItem, error) {
	return nil, nil
}

type recordingAccountProbeService struct {
	mu          sync.Mutex
	startReqs   []service.AccountProbeRunRequest
	runReqs     []service.AccountProbeRunRequest
	runDone     chan struct{}
	startErrors map[int64]error
}

func newRecordingAccountProbeService() *recordingAccountProbeService {
	return &recordingAccountProbeService{runDone: make(chan struct{})}
}

func (s *recordingAccountProbeService) Start(ctx context.Context, req service.AccountProbeRunRequest) (service.AccountProbeResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.startErrors[req.AccountID]; err != nil {
		return service.AccountProbeResult{}, err
	}
	s.startReqs = append(s.startReqs, req)
	return service.AccountProbeResult{
		ID:           int64(100 + len(s.startReqs)),
		AccountID:    req.AccountID,
		Profile:      req.Profile,
		Status:       service.AccountProbeStatusRunning,
		Model:        req.Model,
		RequestMode:  req.RequestMode,
		RequestCount: 4,
	}, nil
}

func (s *recordingAccountProbeService) RunExisting(ctx context.Context, run service.AccountProbeResult, req service.AccountProbeRunRequest) (service.AccountProbeResult, error) {
	s.mu.Lock()
	s.runReqs = append(s.runReqs, req)
	if len(s.runReqs) == 2 {
		close(s.runDone)
	}
	s.mu.Unlock()
	run.Status = service.AccountProbeStatusSuccess
	return run, nil
}

func (s *recordingAccountProbeService) List(ctx context.Context, filter service.AccountProbeHistoryFilter) ([]service.AccountProbeResult, error) {
	return nil, nil
}

func (s *recordingAccountProbeService) Get(ctx context.Context, accountID, runID int64) (*service.AccountProbeResult, error) {
	return nil, nil
}

func (s *recordingAccountProbeService) ListReports(ctx context.Context, filter service.AccountProbeReportFilter) (service.AccountProbeReportPage, error) {
	return service.AccountProbeReportPage{}, nil
}

func (s *recordingAccountProbeService) GetReport(ctx context.Context, runID int64) (*service.AccountProbeReportItem, error) {
	return nil, nil
}

func (s *recordingAccountProbeService) DeleteReports(ctx context.Context, runIDs []int64) (service.AccountProbeReportDeleteResult, error) {
	return service.AccountProbeReportDeleteResult{}, nil
}

func (s *recordingAccountProbeService) ListRanking(ctx context.Context, limit int) ([]service.AccountProbeRankingItem, error) {
	return nil, nil
}

func TestAccountProbeCreateReturnsAcceptedAndRunsInBackground(t *testing.T) {
	gin.SetMode(gin.TestMode)
	probeSvc := newBlockingAccountProbeService()
	h := &AccountHandler{accountProbeService: probeSvc}
	router := gin.New()
	router.POST("/api/v1/admin/accounts/:id/probe-runs", h.CreateProbeRun)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/128/probe-runs", bytes.NewBufferString(`{"mode":"quick","model":"gpt-5.4"}`))
	req.Header.Set("Content-Type", "application/json")

	done := make(chan struct{})
	go func() {
		router.ServeHTTP(rec, req)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("CreateProbeRun should return before the probe task finishes")
	}
	require.Equal(t, http.StatusAccepted, rec.Code)
	require.Contains(t, rec.Body.String(), `"status":"running"`)

	select {
	case <-probeSvc.started:
	case <-time.After(time.Second):
		t.Fatal("background probe task did not start")
	}
	close(probeSvc.release)
}

func TestAccountModelProbeBatchCreateRunsManualValidationOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	probeSvc := newRecordingAccountProbeService()
	h := &AccountHandler{accountProbeService: probeSvc}
	router := gin.New()
	router.POST("/api/v1/admin/account-model-probe-runs/batch", h.BatchCreateModelProbeRuns)

	rec := httptest.NewRecorder()
	body := `{"account_ids":[128,129],"model":"gpt-5.4","request_mode":"stream","trusted_comparison_account_id":777}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/account-model-probe-runs/batch", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)
	require.Contains(t, rec.Body.String(), `"accepted_count":2`)
	require.Len(t, probeSvc.startReqs, 2)
	for _, got := range probeSvc.startReqs {
		require.Equal(t, service.AccountProbeProfileModelValidation, got.Profile)
		require.Equal(t, "gpt-5.4", got.Model)
		require.Equal(t, "stream", got.RequestMode)
		require.Equal(t, int64(777), got.TrustedComparisonID)
		require.True(t, got.ModelValidationOnly)
	}

	select {
	case <-probeSvc.runDone:
	case <-time.After(time.Second):
		t.Fatal("background batch model probe tasks did not start")
	}
	probeSvc.mu.Lock()
	defer probeSvc.mu.Unlock()
	for _, got := range probeSvc.runReqs {
		require.Equal(t, int64(777), got.TrustedComparisonID)
	}
}

func TestAccountModelProbeBatchCreateSkipsStartFailuresAndRunsAccepted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	probeSvc := newRecordingAccountProbeService()
	probeSvc.startErrors = map[int64]error{130: errors.New("no api key available")}
	h := &AccountHandler{accountProbeService: probeSvc}
	router := gin.New()
	router.POST("/api/v1/admin/account-model-probe-runs/batch", h.BatchCreateModelProbeRuns)

	rec := httptest.NewRecorder()
	body := `{"account_ids":[128,130,129],"model":"gpt-5.4","request_mode":"stream"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/account-model-probe-runs/batch", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)
	require.Contains(t, rec.Body.String(), `"accepted_count":2`)
	require.Contains(t, rec.Body.String(), `"skipped_count":1`)
	require.Contains(t, rec.Body.String(), `"account_id":130`)
	require.Contains(t, rec.Body.String(), `"message":"no api key available"`)
	require.Len(t, probeSvc.startReqs, 2)

	select {
	case <-probeSvc.runDone:
	case <-time.After(time.Second):
		t.Fatal("background batch model probe tasks did not start for accepted runs")
	}
}

func TestAccountProbeCreateParsesRequestMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	probeSvc := newBlockingAccountProbeService()
	h := &AccountHandler{accountProbeService: probeSvc}
	router := gin.New()
	router.POST("/api/v1/admin/accounts/:id/probe-runs", h.CreateProbeRun)

	rec := httptest.NewRecorder()
	body := `{"mode":"quick","model":"gpt-5.4","request_mode":"stream"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/128/probe-runs", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)
	require.Equal(t, "stream", probeSvc.req.RequestMode)
	close(probeSvc.release)
}

func TestAccountModelProbeCreateRunsManualValidationOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	probeSvc := newBlockingAccountProbeService()
	h := &AccountHandler{accountProbeService: probeSvc}
	router := gin.New()
	router.POST("/api/v1/admin/account-model-probe-runs", h.CreateModelProbeRun)

	rec := httptest.NewRecorder()
	body := `{"account_id":128,"model":"gpt-5.4","request_mode":"stream","trusted_comparison_account_id":777}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/account-model-probe-runs", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	done := make(chan struct{})
	go func() {
		router.ServeHTTP(rec, req)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("CreateModelProbeRun should return before the probe task finishes")
	}
	require.Equal(t, http.StatusAccepted, rec.Code)
	require.Equal(t, int64(128), probeSvc.req.AccountID)
	require.Equal(t, "gpt-5.4", probeSvc.req.Model)
	require.Equal(t, "stream", probeSvc.req.RequestMode)
	require.Equal(t, int64(777), probeSvc.req.TrustedComparisonID)
	require.True(t, probeSvc.req.ModelValidationOnly)

	select {
	case <-probeSvc.started:
	case <-time.After(time.Second):
		t.Fatal("background model probe task did not start")
	}
	close(probeSvc.release)
}
