package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestOpenAIContextContinuityBalanceUnknownIncludesDiagnosticDetail(t *testing.T) {
	checker := NewRealtimeBalanceChecker(realtimeBalanceRefresherFunc(func(ctx context.Context, account *Account) (*UpstreamBalanceSnapshot, error) {
		return nil, errors.New("upstream timeout")
	}), RealtimeBalanceCheckerOptions{Timeout: time.Second})
	svc := &OpenAIGatewayService{realtimeBalanceChecker: checker}

	available, checked, detail, err := svc.checkRealtimeBalanceAvailableForContinuity(
		context.Background(),
		&Account{ID: 91, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
	)

	if err == nil || !strings.Contains(err.Error(), "upstream timeout") {
		t.Fatalf("error = %v, want upstream timeout", err)
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
	if reason, _ := detail["reason"].(string); !strings.Contains(reason, "upstream timeout") {
		t.Fatalf("reason = %q, want upstream timeout", reason)
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
