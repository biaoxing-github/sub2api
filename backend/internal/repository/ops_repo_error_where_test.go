package repository

import (
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestBuildOpsErrorLogsWhere_QueryUsesQualifiedColumns(t *testing.T) {
	filter := &service.OpsErrorLogFilter{
		Query: "ACCESS_DENIED",
	}

	where, args := buildOpsErrorLogsWhere(filter)
	if where == "" {
		t.Fatalf("where should not be empty")
	}
	if len(args) != 1 {
		t.Fatalf("args len = %d, want 1", len(args))
	}
	if !strings.Contains(where, "e.request_id ILIKE $") {
		t.Fatalf("where should include qualified request_id condition: %s", where)
	}
	if !strings.Contains(where, "e.client_request_id ILIKE $") {
		t.Fatalf("where should include qualified client_request_id condition: %s", where)
	}
	if !strings.Contains(where, "e.error_message ILIKE $") {
		t.Fatalf("where should include qualified error_message condition: %s", where)
	}
}

func TestBuildOpsErrorLogsWhere_UserQueryUsesExistsSubquery(t *testing.T) {
	filter := &service.OpsErrorLogFilter{
		UserQuery: "admin@",
	}

	where, args := buildOpsErrorLogsWhere(filter)
	if where == "" {
		t.Fatalf("where should not be empty")
	}
	if len(args) != 1 {
		t.Fatalf("args len = %d, want 1", len(args))
	}
	if !strings.Contains(where, "EXISTS (SELECT 1 FROM users u WHERE u.id = e.user_id AND u.email ILIKE $") {
		t.Fatalf("where should include EXISTS user email condition: %s", where)
	}
}

func TestBuildOpsErrorLogsWhere_RecoveredUpstreamRequiresExplicitOptIn(t *testing.T) {
	requestWhere, _ := buildOpsErrorLogsWhere(&service.OpsErrorLogFilter{Phase: "upstream"})
	if !strings.Contains(requestWhere, "COALESCE(e.status_code, 0) >= 400") {
		t.Fatalf("request error filter must keep the client-visible status guard: %s", requestWhere)
	}
	if !strings.Contains(requestWhere, "e.error_phase = $") {
		t.Fatalf("request error filter must retain the upstream phase condition: %s", requestWhere)
	}

	upstreamWhere, _ := buildOpsErrorLogsWhere(&service.OpsErrorLogFilter{
		Phase:                    "upstream",
		IncludeRecoveredUpstream: true,
	})
	if strings.Contains(upstreamWhere, "COALESCE(e.status_code, 0) >= 400") {
		t.Fatalf("upstream list must expose recovered rows: %s", upstreamWhere)
	}
}

func TestBuildOpsErrorLogsWhere_CyberPolicyRemainsVisibleAfterStreamStarts(t *testing.T) {
	where, _ := buildOpsErrorLogsWhere(&service.OpsErrorLogFilter{})
	if !strings.Contains(where, "e.error_type = 'cyber_policy'") {
		t.Fatalf("status guard must preserve streaming cyber policy failures: %s", where)
	}
}

func TestOpsErrorLogsOrderByUsesWhitelistAndStableTiebreaker(t *testing.T) {
	tests := []struct {
		name   string
		filter *service.OpsErrorLogFilter
		want   string
	}{
		{name: "default", filter: nil, want: "e.created_at DESC, e.id DESC"},
		{name: "created at asc", filter: &service.OpsErrorLogFilter{SortBy: "created_at", SortOrder: "asc"}, want: "e.created_at ASC, e.id ASC"},
		{name: "model asc", filter: &service.OpsErrorLogFilter{SortBy: "model", SortOrder: "asc"}, want: "COALESCE(NULLIF(TRIM(e.requested_model), ''), e.model) ASC, e.id ASC"},
		{name: "status desc", filter: &service.OpsErrorLogFilter{SortBy: "status_code", SortOrder: "desc"}, want: "COALESCE(e.upstream_status_code, e.status_code, 0) DESC, e.id DESC"},
		{name: "unknown column falls back", filter: &service.OpsErrorLogFilter{SortBy: "error_message", SortOrder: "asc"}, want: "e.created_at ASC, e.id ASC"},
		{name: "unknown direction falls back", filter: &service.OpsErrorLogFilter{SortBy: "model", SortOrder: "sideways"}, want: "COALESCE(NULLIF(TRIM(e.requested_model), ''), e.model) DESC, e.id DESC"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := opsErrorLogsOrderBy(test.filter); got != test.want {
				t.Fatalf("order by = %q, want %q", got, test.want)
			}
		})
	}
}
