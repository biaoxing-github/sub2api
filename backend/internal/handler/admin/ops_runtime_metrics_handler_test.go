package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func TestOpsRuntimeMetricsHandlerReturnsInjectedSnapshot(t *testing.T) {
	metrics := service.NewOpsRuntimeMetrics()
	metrics.RecordRequestStage(
		service.OpsRequestStageHeaderWait,
		15*time.Millisecond,
		service.OpsRequestMetricLabels{
			Result:     service.OpsRequestResultSuccess,
			Protocol:   service.OpsRequestProtocolHTTP2,
			ErrorClass: service.OpsRequestErrorClassNone,
		},
	)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewOpsHandlerWithRuntimeMetrics(newRuntimeOpsService(t), metrics)
	router.GET("/runtime/metrics", handler.GetRuntimeMetrics)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/runtime/metrics", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		Code int                               `json:"code"`
		Data service.OpsRuntimeMetricsSnapshot `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Code != 0 {
		t.Fatalf("response code = %d, want 0; body=%s", payload.Code, recorder.Body.String())
	}
	if len(payload.Data.RequestStages) != 1 {
		t.Fatalf("request stages = %d, want 1", len(payload.Data.RequestStages))
	}
	if payload.Data.Scheduling == nil || payload.Data.Caches == nil || payload.Data.ConnectionPools == nil {
		t.Fatalf("empty counter collections must be encoded as arrays: %+v", payload.Data)
	}
	stage := payload.Data.RequestStages[0]
	if stage.Stage != service.OpsRequestStageHeaderWait || stage.Count != 1 || stage.TotalDurationMs != 15 {
		t.Fatalf("unexpected request stage: %+v", stage)
	}
}

func TestOpsRuntimeMetricsHandlerRejectsMissingOpsService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/runtime/metrics", NewOpsHandlerWithRuntimeMetrics(nil, service.NewOpsRuntimeMetrics()).GetRuntimeMetrics)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/runtime/metrics", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body=%s", recorder.Code, recorder.Body.String())
	}
}
