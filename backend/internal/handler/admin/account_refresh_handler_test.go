package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestMarkAccountRefreshFailureSetsErrorAndReturnsLatestAccount(t *testing.T) {
	adminSvc := newStubAdminService()
	adminSvc.accounts = []service.Account{
		{
			ID:          11,
			Name:        "bad-refresh",
			Platform:    service.PlatformOpenAI,
			Type:        service.AccountTypeOAuth,
			Status:      service.StatusActive,
			Schedulable: true,
		},
	}
	handler := &AccountHandler{adminService: adminSvc}

	updated := handler.markAccountRefreshFailure(context.Background(), &adminSvc.accounts[0], errors.New("invalid_grant"))

	require.NotNil(t, updated)
	require.Equal(t, int64(11), updated.ID)
	require.Equal(t, service.StatusError, updated.Status)
	require.Equal(t, "invalid_grant", updated.ErrorMessage)
	require.Equal(t, []int64{11}, adminSvc.setErrorAccountIDs)
	require.Equal(t, []string{"invalid_grant"}, adminSvc.setErrorMessages)
}

func TestMarkAccountRefreshSuccessClearsPreviousErrorAndSchedulableState(t *testing.T) {
	rateLimitedUntil := time.Now().Add(time.Hour)
	adminSvc := newStubAdminService()
	adminSvc.accounts = []service.Account{
		{
			ID:                     12,
			Name:                   "recovered-refresh",
			Platform:               service.PlatformOpenAI,
			Type:                   service.AccountTypeOAuth,
			Status:                 service.StatusError,
			ErrorMessage:           "previous refresh failed",
			Schedulable:            false,
			RateLimitResetAt:       &rateLimitedUntil,
			Credentials:            map[string]any{"access_token": "old"},
			TempUnschedulableUntil: &rateLimitedUntil,
		},
	}
	handler := &AccountHandler{adminService: adminSvc}

	updated := handler.markAccountRefreshSuccess(
		context.Background(),
		&adminSvc.accounts[0],
		&service.Account{ID: 12, Status: service.StatusError, Schedulable: false},
	)

	require.NotNil(t, updated)
	require.Equal(t, int64(12), updated.ID)
	require.Equal(t, service.StatusActive, updated.Status)
	require.Empty(t, updated.ErrorMessage)
	require.True(t, updated.Schedulable)
	require.Nil(t, updated.RateLimitResetAt)
	require.Nil(t, updated.TempUnschedulableUntil)
	require.Equal(t, []int64{12}, adminSvc.clearedAccountIDs)
	require.Equal(t, []int64{12}, adminSvc.schedulableAccountIDs)
	require.Equal(t, []bool{true}, adminSvc.schedulableValues)
}

func TestListAccountsReturnsUsageTotalsAndPassesUsageSort(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminSvc := newStubAdminService()
	adminSvc.accounts = []service.Account{
		{
			ID:               21,
			Name:             "heavy-usage",
			Platform:         service.PlatformOpenAI,
			Type:             service.AccountTypeOAuth,
			Status:           service.StatusActive,
			Schedulable:      true,
			TotalAccountCost: 12.3456,
			TotalRequests:    42,
		},
	}
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.GET("/api/v1/admin/accounts", handler.List)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts?sort_by=total_account_cost&sort_order=desc", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "total_account_cost", adminSvc.lastListAccounts.sortBy)
	require.Equal(t, "desc", adminSvc.lastListAccounts.sortOrder)

	var resp struct {
		Data struct {
			Items []struct {
				ID               int64   `json:"id"`
				TotalAccountCost float64 `json:"total_account_cost"`
				TotalRequests    int64   `json:"total_requests"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Data.Items, 1)
	require.Equal(t, int64(21), resp.Data.Items[0].ID)
	require.Equal(t, 12.3456, resp.Data.Items[0].TotalAccountCost)
	require.Equal(t, int64(42), resp.Data.Items[0].TotalRequests)
}

func TestAccountHandlerRestoreAPIKeyStateByFingerprint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminSvc := newStubAdminService()
	adminSvc.accounts = []service.Account{
		{
			ID:          31,
			Name:        "multi-key",
			Platform:    service.PlatformOpenAI,
			Type:        service.AccountTypeAPIKey,
			Status:      service.StatusActive,
			Schedulable: true,
			Credentials: map[string]any{
				"api_keys": []any{"sk-disabled"},
				service.CredentialAPIKeysDisabled: map[string]any{
					"sha256:disabled": map[string]any{"reason": "rate_limited"},
				},
			},
		},
	}
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.POST("/api/v1/admin/accounts/:id/api-keys/:fingerprint/restore-state", handler.RestoreAPIKeyState)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/31/api-keys/sha256:disabled/restore-state", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, []int64{31}, adminSvc.restoredAPIKeyIDs)
	require.Equal(t, []string{"sha256:disabled"}, adminSvc.restoredAPIKeyFPs)
	require.Contains(t, rec.Body.String(), `"id":31`)
}
