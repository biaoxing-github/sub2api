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

type blockingAccountProbeService struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func newBlockingAccountProbeService() *blockingAccountProbeService {
	return &blockingAccountProbeService{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (s *blockingAccountProbeService) Start(ctx context.Context, req service.AccountProbeRunRequest) (service.AccountProbeResult, error) {
	return service.AccountProbeResult{
		ID:           99,
		AccountID:    req.AccountID,
		Profile:      req.Profile,
		Status:       service.AccountProbeStatusRunning,
		Model:        req.Model,
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
