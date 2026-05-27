package service

import (
	"context"
	"strings"
	"testing"
)

func TestOpenAIContextContinuityBalanceUnknownIncludesDiagnosticDetail(t *testing.T) {
	checker := NewRealtimeBalanceChecker(realtimeBalanceRefresherFunc(func(ctx context.Context, account *Account) (*UpstreamBalanceSnapshot, error) {
		t.Fatal("continuity balance check must not refresh remote balance on request path")
		return nil, nil
	}), RealtimeBalanceCheckerOptions{})
	svc := &OpenAIGatewayService{realtimeBalanceChecker: checker}

	available, checked, detail, err := svc.checkRealtimeBalanceAvailableForContinuity(
		context.Background(),
		&Account{ID: 91, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
	)

	if err == nil || !strings.Contains(err.Error(), "realtime balance not verified") {
		t.Fatalf("error = %v, want local snapshot not verified", err)
	}
	if available || !checked {
		t.Fatalf("available=%v checked=%v, want checked unavailable", available, checked)
	}
	if detail == nil {
		t.Fatal("detail is nil")
	}
	if detail["balance_confirm_source"] != RealtimeBalanceSourceError {
		t.Fatalf("detail = %+v", detail)
	}
	if detail["replay_safe"] != false {
		t.Fatalf("detail replay_safe = %v, want false", detail["replay_safe"])
	}
	if reason, _ := detail["reason"].(string); !strings.Contains(reason, "realtime balance not verified") {
		t.Fatalf("reason = %q, want local snapshot not verified", reason)
	}
}

func TestOpenAIContextContinuityReplayBodyReportsProtectedReasonDetail(t *testing.T) {
	svc := &OpenAIGatewayService{}

	body, reason, detail, ok := svc.buildOpenAIContinuityReplayBody(
		context.Background(),
		nil,
		"resp_prev",
		"session_hash",
		[]byte(`{"model":"gpt-5.5","previous_response_id":"resp_prev"}`),
	)

	if ok || body != nil {
		t.Fatalf("ok=%v body=%s, want protected", ok, string(body))
	}
	if reason != OpenAIContinuityReasonReplayNotSafe {
		t.Fatalf("reason = %q", reason)
	}
	if detail == nil {
		t.Fatal("detail is nil")
	}
	if detail["replay_safe"] != false || detail["protected"] != true || detail["reason"] != OpenAIContinuityReasonReplayNotSafe {
		t.Fatalf("detail = %+v", detail)
	}
}

func TestOpenAIContextContinuityReplayBodyAllowsPortableFullWithoutPreviousResponseID(t *testing.T) {
	svc := &OpenAIGatewayService{}
	requestBody := []byte(`{"model":"gpt-5.5","input":[{"type":"message","role":"user"},{"type":"function_call_output","call_id":"call_1","output":"ok"}]}`)

	body, reason, detail, ok := svc.buildOpenAIContinuityReplayBody(
		context.Background(),
		nil,
		"",
		"session_hash",
		requestBody,
	)

	if !ok {
		t.Fatalf("ok=false reason=%q detail=%+v", reason, detail)
	}
	if string(body) != string(requestBody) {
		t.Fatalf("body = %s, want original request body", string(body))
	}
	if detail["context_migration_class"] != OpenAIContextMigrationPortableFull {
		t.Fatalf("detail = %+v", detail)
	}
	if detail["replay_safe"] != true {
		t.Fatalf("replay_safe = %v, want true", detail["replay_safe"])
	}
}
