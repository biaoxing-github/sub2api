//go:build unit

package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	}
	h := &AccountHandler{adminService: adminSvc, batchAccountTester: tester}
	router := gin.New()
	router.POST("/api/v1/admin/accounts/batch-test-non-apikey", h.BatchTestNonAPIKey)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/batch-test-non-apikey", bytes.NewBufferString(`{"model_id":"gpt-5.4","concurrency":2}`))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 1, adminSvc.lastListAccounts.calls)
	require.Equal(t, "name", adminSvc.lastListAccounts.sortBy)
	require.Equal(t, "asc", adminSvc.lastListAccounts.sortOrder)
	require.ElementsMatch(t, []int64{11, 13}, tester.calledIDs)

	var body struct {
		Code int `json:"code"`
		Data struct {
			Total             int `json:"total"`
			SuccessCount      int `json:"success_count"`
			FailedCount       int `json:"failed_count"`
			UnauthorizedCount int `json:"unauthorized_count"`
			Items             []struct {
				AccountID int64  `json:"account_id"`
				Status    string `json:"status"`
				Category  string `json:"category"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, 0, body.Code)
	require.Equal(t, 2, body.Data.Total)
	require.Equal(t, 1, body.Data.SuccessCount)
	require.Equal(t, 1, body.Data.FailedCount)
	require.Equal(t, 1, body.Data.UnauthorizedCount)
	require.Equal(t, int64(11), body.Data.Items[0].AccountID)
	require.Equal(t, "success", body.Data.Items[0].Status)
	require.Equal(t, "ok", body.Data.Items[0].Category)
	require.Equal(t, int64(13), body.Data.Items[1].AccountID)
	require.Equal(t, "failed", body.Data.Items[1].Status)
	require.Equal(t, "unauthorized", body.Data.Items[1].Category)
}

type stubBatchAccountTester struct {
	results   map[int64]*service.ScheduledTestResult
	calledIDs []int64
}

func (s *stubBatchAccountTester) RunTestBackground(ctx context.Context, accountID int64, modelID string) (*service.ScheduledTestResult, error) {
	s.calledIDs = append(s.calledIDs, accountID)
	if result, ok := s.results[accountID]; ok {
		return result, nil
	}
	return &service.ScheduledTestResult{Status: "failed", ErrorMessage: "missing stub result"}, nil
}
