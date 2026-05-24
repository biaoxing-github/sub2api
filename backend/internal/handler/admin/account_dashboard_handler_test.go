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

func TestAccountHandlerDashboardSummaryUsesCurrentFiltersAndSingleAccountFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)

	now := time.Now().UTC()
	adminSvc := newStubAdminService()
	adminSvc.accounts = []service.Account{
		{
			ID:          1,
			Platform:    service.PlatformOpenAI,
			Type:        service.AccountTypeOAuth,
			Status:      service.StatusActive,
			Schedulable: false,
			Credentials: map[string]any{"plan_type": "free"},
			Extra: map[string]any{
				"codex_7d_used_percent": 50.0,
				"codex_7d_reset_at":     now.Add(time.Hour).Format(time.RFC3339),
			},
		},
	}

	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, &service.AccountUsageService{}, nil, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.GET("/api/v1/admin/accounts/dashboard-summary", handler.GetDashboardSummary)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/dashboard-summary?platform=openai&type=oauth&status=active&group=ungrouped&privacy_mode=training_off&plan_type=free&search=free&sort_by=name&sort_order=asc", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if adminSvc.lastListAccounts.calls != 1 {
		t.Fatalf("ListAccounts calls = %d, want 1", adminSvc.lastListAccounts.calls)
	}
	if adminSvc.lastListAccounts.platform != "openai" ||
		adminSvc.lastListAccounts.accountType != "oauth" ||
		adminSvc.lastListAccounts.status != "active" ||
		adminSvc.lastListAccounts.groupID != service.AccountListGroupUngrouped ||
		adminSvc.lastListAccounts.privacyMode != "training_off" ||
		adminSvc.lastListAccounts.planType != "free" ||
		adminSvc.lastListAccounts.search != "free" ||
		adminSvc.lastListAccounts.sortBy != "name" ||
		adminSvc.lastListAccounts.sortOrder != "asc" {
		t.Fatalf("filters = %#v", adminSvc.lastListAccounts)
	}

	var payload struct {
		Data struct {
			StatusSummary struct {
				Active        int `json:"active"`
				Unschedulable int `json:"unschedulable"`
			} `json:"status_summary"`
			UsageSummary     service.AccountUsageSummary `json:"usage_summary"`
			ActionItemCounts struct {
				Critical int `json:"critical"`
			} `json:"action_item_counts"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Data.StatusSummary.Active != 1 {
		t.Fatalf("active = %d, want 1", payload.Data.StatusSummary.Active)
	}
	if payload.Data.StatusSummary.Unschedulable != 1 {
		t.Fatalf("unschedulable = %d, want 1", payload.Data.StatusSummary.Unschedulable)
	}
	if payload.Data.UsageSummary.TotalAccounts != 1 {
		t.Fatalf("usage total = %d, want 1", payload.Data.UsageSummary.TotalAccounts)
	}
	if payload.Data.ActionItemCounts.Critical != 1 {
		t.Fatalf("critical action count = %d, want 1", payload.Data.ActionItemCounts.Critical)
	}
}

func TestAccountHandlerActionItemsReturnsFilteredItems(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adminSvc := newStubAdminService()
	adminSvc.accounts = []service.Account{
		{ID: 1, Name: "manual-off", Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, Status: service.StatusActive, Schedulable: false},
	}

	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.GET("/api/v1/admin/accounts/action-items", handler.GetActionItems)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/action-items?platform=openai&type=oauth", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if adminSvc.lastListAccounts.calls != 1 {
		t.Fatalf("ListAccounts calls = %d, want 1", adminSvc.lastListAccounts.calls)
	}

	var payload struct {
		Data struct {
			Items []struct {
				AccountID       int64  `json:"account_id"`
				Severity        string `json:"severity"`
				Reason          string `json:"reason"`
				SuggestedAction string `json:"suggested_action"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Data.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(payload.Data.Items))
	}
	item := payload.Data.Items[0]
	if item.AccountID != 1 || item.Severity != "critical" || item.Reason != "schedulable_disabled" || item.SuggestedAction == "" {
		t.Fatalf("item = %#v", item)
	}
}
