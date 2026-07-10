package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type capturingOpsRepository struct {
	service.OpsRepository
	filter *service.OpsErrorLogFilter
}

func (r *capturingOpsRepository) ListErrorLogs(_ context.Context, filter *service.OpsErrorLogFilter) (*service.OpsErrorLogList, error) {
	r.filter = filter
	return &service.OpsErrorLogList{
		Errors:   []*service.OpsErrorLog{},
		Page:     filter.Page,
		PageSize: filter.PageSize,
	}, nil
}

func newOpsErrorListTestRouter(repo service.OpsRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	svc := service.NewOpsService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(svc)
	router := gin.New()
	router.GET("/errors", handler.GetErrorLogs)
	router.GET("/upstream-errors", handler.ListUpstreamErrors)
	return router
}

func TestOpsHandlerGetErrorLogsPassesUsageFiltersAndSort(t *testing.T) {
	repo := &capturingOpsRepository{}
	router := newOpsErrorListTestRouter(repo)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/errors?time_range=1h&phase=upstream&user_id=42&api_key_id=7&model=gpt-5.6-sol&category=auth&sort_by=status_code&sort_order=asc", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	if repo.filter == nil {
		t.Fatal("repository filter was not captured")
	}
	if repo.filter.UserID == nil || *repo.filter.UserID != 42 {
		t.Fatalf("user_id = %v, want 42", repo.filter.UserID)
	}
	if repo.filter.APIKeyID == nil || *repo.filter.APIKeyID != 7 {
		t.Fatalf("api_key_id = %v, want 7", repo.filter.APIKeyID)
	}
	if repo.filter.Model != "gpt-5.6-sol" {
		t.Fatalf("model = %q, want gpt-5.6-sol", repo.filter.Model)
	}
	if !reflect.DeepEqual(repo.filter.ErrorPhasesAny, []string{"auth"}) || len(repo.filter.ErrorTypesAny) != 0 {
		t.Fatalf("category filters = phases %v types %v, want auth phase", repo.filter.ErrorPhasesAny, repo.filter.ErrorTypesAny)
	}
	if repo.filter.Phase != "upstream" {
		t.Fatalf("phase = %q, want upstream", repo.filter.Phase)
	}
	if repo.filter.IncludeRecoveredUpstream {
		t.Fatal("general error list must not include recovered upstream rows")
	}
	if repo.filter.SortBy != "status_code" || repo.filter.SortOrder != "asc" {
		t.Fatalf("sort = %q %q, want status_code asc", repo.filter.SortBy, repo.filter.SortOrder)
	}
}

func TestOpsHandlerListUpstreamErrorsOptsIntoRecoveredRowsAndSort(t *testing.T) {
	repo := &capturingOpsRepository{}
	router := newOpsErrorListTestRouter(repo)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/upstream-errors?time_range=1h&sort_by=model&sort_order=asc", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	if repo.filter == nil {
		t.Fatal("repository filter was not captured")
	}
	if repo.filter.Phase != "upstream" || !repo.filter.IncludeRecoveredUpstream {
		t.Fatalf("upstream recovery scope = phase %q include %v", repo.filter.Phase, repo.filter.IncludeRecoveredUpstream)
	}
	if repo.filter.SortBy != "model" || repo.filter.SortOrder != "asc" {
		t.Fatalf("sort = %q %q, want model asc", repo.filter.SortBy, repo.filter.SortOrder)
	}
}

func TestOpsHandlerGetErrorLogsRejectsInvalidIdentityFilters(t *testing.T) {
	for _, query := range []string{"user_id=invalid", "api_key_id=0"} {
		t.Run(query, func(t *testing.T) {
			repo := &capturingOpsRepository{}
			router := newOpsErrorListTestRouter(repo)

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/errors?time_range=1h&"+query, nil)
			router.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", recorder.Code, recorder.Body.String())
			}
			if repo.filter != nil {
				t.Fatal("repository must not be called for an invalid identity filter")
			}
		})
	}
}
