//go:build unit

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type apiKeyProbeHTTPServiceStub struct {
	result service.APIKeyProbeResult
	err    error
	req    service.APIKeyProbeRunRequest
}

func (s *apiKeyProbeHTTPServiceStub) Run(ctx context.Context, req service.APIKeyProbeRunRequest) (service.APIKeyProbeResult, error) {
	s.req = req
	if s.err != nil {
		return service.APIKeyProbeResult{}, s.err
	}
	return s.result, nil
}

func (s *apiKeyProbeHTTPServiceStub) List(ctx context.Context, filter service.APIKeyProbeHistoryFilter) ([]service.APIKeyProbeResult, error) {
	return []service.APIKeyProbeResult{s.result}, s.err
}

func (s *apiKeyProbeHTTPServiceStub) Get(ctx context.Context, userID, apiKeyID, runID int64) (*service.APIKeyProbeResult, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &s.result, nil
}

func setupAPIKeyProbeRouter(probeSvc *apiKeyProbeHTTPServiceStub, userID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewAPIKeyProbeHandler(probeSvc)
	router.POST("/api/v1/keys/:id/probe-runs", func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: userID})
		handler.CreateProbeRun(c)
	})
	return router
}

func TestAPIKeyProbeHandler_RunParsesOptionsAndUsesAuthenticatedUser(t *testing.T) {
	probeSvc := &apiKeyProbeHTTPServiceStub{
		result: service.APIKeyProbeResult{
			Estimate: service.APIKeyProbeEstimate{Requests: 9},
			Latency:  service.APIKeyProbeLatencyStats{P50Millis: 120, P95Millis: 300, AvgMillis: 180, MaxMillis: 400},
		},
	}
	router := setupAPIKeyProbeRouter(probeSvc, 42)

	body := `{"mode":"standard","include_codex_stability":true,"include_long_context":true}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/keys/7/probe-runs", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(42), probeSvc.req.UserID)
	require.Equal(t, int64(7), probeSvc.req.APIKeyID)
	require.Equal(t, service.APIKeyProbeProfileStandard, probeSvc.req.Profile)
	require.True(t, probeSvc.req.IncludeCodexStability)
	require.True(t, probeSvc.req.IncludeLongContext)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Estimate struct {
				Requests int `json:"requests"`
			} `json:"estimate"`
			Latency struct {
				P50Millis int `json:"p50_ms"`
				P95Millis int `json:"p95_ms"`
				AvgMillis int `json:"avg_ms"`
				MaxMillis int `json:"max_ms"`
			} `json:"latency"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Equal(t, 9, resp.Data.Estimate.Requests)
	require.Equal(t, 120, resp.Data.Latency.P50Millis)
	require.Equal(t, 300, resp.Data.Latency.P95Millis)
	require.Equal(t, 180, resp.Data.Latency.AvgMillis)
	require.Equal(t, 400, resp.Data.Latency.MaxMillis)
}

func TestAPIKeyProbeHandler_RunDefaultsToStandardProfile(t *testing.T) {
	probeSvc := &apiKeyProbeHTTPServiceStub{}
	router := setupAPIKeyProbeRouter(probeSvc, 42)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/keys/7/probe-runs", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, service.APIKeyProbeProfileStandard, probeSvc.req.Profile)
}

func TestAPIKeyProbeHandler_RunMapsOwnershipFailureToForbidden(t *testing.T) {
	probeSvc := &apiKeyProbeHTTPServiceStub{err: service.ErrInsufficientPerms}
	router := setupAPIKeyProbeRouter(probeSvc, 42)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/keys/7/probe-runs", bytes.NewBufferString(`{"mode":"quick"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Equal(t, service.APIKeyProbeProfileQuick, probeSvc.req.Profile)
}

func TestAPIKeyProbeHandler_RunRejectsBadKeyID(t *testing.T) {
	probeSvc := &apiKeyProbeHTTPServiceStub{}
	router := setupAPIKeyProbeRouter(probeSvc, 42)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/keys/nope/probe-runs", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Zero(t, probeSvc.req.APIKeyID)
}
