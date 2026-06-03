//go:build unit

package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestScheduledTestHandlerListRunnerSnapshots(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC()
	h := NewScheduledTestHandler(nil)
	h.runnerSnapshots = func() []service.ScheduledTestRunnerSnapshot {
		return []service.ScheduledTestRunnerSnapshot{{
			Name:               "scheduled_test_runner",
			IntervalMillis:     60000,
			InitialDelayMillis: 10000,
			Running:            true,
			LastStartedAt:      &now,
			RunCount:           3,
			SuccessCount:       2,
			SkippedCount:       1,
		}}
	}
	router := gin.New()
	router.GET("/api/v1/admin/scheduled-test-runner/snapshots", h.ListRunnerSnapshots)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/scheduled-test-runner/snapshots", nil)

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"name":"scheduled_test_runner"`)
	require.Contains(t, rec.Body.String(), `"running":true`)
	require.Contains(t, rec.Body.String(), `"skipped_count":1`)
}
