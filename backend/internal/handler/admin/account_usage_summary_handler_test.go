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

func TestAccountHandlerGetUsageSummaryUsesCurrentFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)

	now := time.Now().UTC()
	adminSvc := newStubAdminService()
	adminSvc.accounts = []service.Account{
		{
			ID:          1,
			Platform:    service.PlatformOpenAI,
			Type:        service.AccountTypeOAuth,
			Status:      service.StatusActive,
			Schedulable: true,
			Credentials: map[string]any{"plan_type": "free"},
			Extra: map[string]any{
				"codex_5h_used_percent": 50.0,
				"codex_5h_reset_at":     now.Add(time.Hour).Format(time.RFC3339),
			},
		},
	}

	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, &service.AccountUsageService{}, nil, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.GET("/api/v1/admin/accounts/usage-summary", handler.GetUsageSummary)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/usage-summary?platform=openai&type=oauth&status=active&group=ungrouped&privacy_mode=training_off&search=free", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if adminSvc.lastListAccounts.platform != "openai" {
		t.Fatalf("platform filter = %q, want openai", adminSvc.lastListAccounts.platform)
	}
	if adminSvc.lastListAccounts.accountType != "oauth" {
		t.Fatalf("type filter = %q, want oauth", adminSvc.lastListAccounts.accountType)
	}
	if adminSvc.lastListAccounts.status != "active" {
		t.Fatalf("status filter = %q, want active", adminSvc.lastListAccounts.status)
	}
	if adminSvc.lastListAccounts.groupID != service.AccountListGroupUngrouped {
		t.Fatalf("group filter = %d, want ungrouped", adminSvc.lastListAccounts.groupID)
	}
	if adminSvc.lastListAccounts.privacyMode != "training_off" {
		t.Fatalf("privacy filter = %q, want training_off", adminSvc.lastListAccounts.privacyMode)
	}
	if adminSvc.lastListAccounts.search != "free" {
		t.Fatalf("search filter = %q, want free", adminSvc.lastListAccounts.search)
	}

	var payload struct {
		Data service.AccountUsageSummary `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Data.TotalAccounts != 1 {
		t.Fatalf("total_accounts = %d, want 1", payload.Data.TotalAccounts)
	}
}
