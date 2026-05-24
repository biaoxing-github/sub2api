//go:build unit

package service

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
)

func TestMemoryContextJournalAppendListAndBindResponse(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	journal := NewMemoryContextJournal(ContextJournalOptions{
		TTL:             time.Hour,
		MaxSessionBytes: 1024,
		Now:             clock,
	})

	turn, err := journal.AppendTurn(ctx, ContextJournalAppendInput{
		GroupID:     7,
		SessionHash: "sess-a",
		AccountID:   42,
		Protocol:    ContextJournalProtocolOpenAIResponses,
		RequestBody: []byte(`{"model":"gpt-4.1","input":"hello"}`),
		ResponseID:  "resp_123",
	})
	if err != nil {
		t.Fatalf("AppendTurn() error = %v", err)
	}
	if turn.TurnID == "" {
		t.Fatal("TurnID is empty")
	}
	if turn.RequestBodyHash == "" {
		t.Fatal("RequestBodyHash is empty")
	}

	turns, err := journal.ListTurns(ctx, 7, "sess-a")
	if err != nil {
		t.Fatalf("ListTurns() error = %v", err)
	}
	if len(turns) != 1 {
		t.Fatalf("turn count = %d, want 1", len(turns))
	}
	if !bytes.Equal(turns[0].RequestBody, []byte(`{"model":"gpt-4.1","input":"hello"}`)) {
		t.Fatalf("request body = %s", turns[0].RequestBody)
	}
	turns[0].RequestBody[0] = '['
	turnsAgain, err := journal.ListTurns(ctx, 7, "sess-a")
	if err != nil {
		t.Fatalf("ListTurns() second error = %v", err)
	}
	if !bytes.Equal(turnsAgain[0].RequestBody, []byte(`{"model":"gpt-4.1","input":"hello"}`)) {
		t.Fatalf("journal leaked mutable request body: %s", turnsAgain[0].RequestBody)
	}

	if err := journal.BindResponse(ctx, 7, "resp_123", ContextJournalResponseRef{
		GroupID:     7,
		SessionHash: "sess-a",
		AccountID:   42,
		TurnID:      turn.TurnID,
	}, time.Hour); err != nil {
		t.Fatalf("BindResponse() error = %v", err)
	}
	ref, err := journal.GetResponse(ctx, 7, "resp_123")
	if err != nil {
		t.Fatalf("GetResponse() error = %v", err)
	}
	if ref == nil || ref.SessionHash != "sess-a" || ref.AccountID != 42 || ref.TurnID != turn.TurnID {
		t.Fatalf("response ref = %+v", ref)
	}
}

func TestMemoryContextJournalImplementsContextJournalContract(t *testing.T) {
	var _ ContextJournal = NewMemoryContextJournal(ContextJournalOptions{})
}

func TestMemoryContextJournalTurnCarriesReplayMetadata(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC)
	journal := NewMemoryContextJournal(ContextJournalOptions{
		TTL:             time.Hour,
		MaxSessionBytes: 1024,
		Now:             func() time.Time { return now },
	})

	turn, err := journal.AppendTurn(ctx, ContextJournalAppendInput{
		GroupID:               7,
		SessionHash:           "sess-meta",
		AccountID:             42,
		Protocol:              ContextJournalProtocolOpenAIResponses,
		RequestBody:           []byte(`{"input":"hello","previous_response_id":"resp_prev"}`),
		ResponseID:            "resp_next",
		ClientOutputStarted:   true,
		HasPreviousResponseID: true,
		HasFunctionCallOutput: false,
	})
	if err != nil {
		t.Fatalf("AppendTurn() error = %v", err)
	}
	if turn.RequestProtocol != ContextJournalProtocolOpenAIResponses {
		t.Fatalf("RequestProtocol = %q, want %q", turn.RequestProtocol, ContextJournalProtocolOpenAIResponses)
	}
	if !turn.HasPreviousResponseID {
		t.Fatal("HasPreviousResponseID = false, want true")
	}
	if !turn.ClientOutputStarted {
		t.Fatal("ClientOutputStarted = false, want true")
	}
	if !turn.CreatedAt.Equal(now) {
		t.Fatalf("CreatedAt = %v, want %v", turn.CreatedAt, now)
	}
}

func TestMemoryContextJournalTTLExpiresTurnsAndResponseRefs(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	journal := NewMemoryContextJournal(ContextJournalOptions{
		TTL:             time.Minute,
		MaxSessionBytes: 1024,
		Now:             func() time.Time { return now },
	})

	turn, err := journal.AppendTurn(ctx, ContextJournalAppendInput{
		GroupID:     7,
		SessionHash: "sess-expire",
		AccountID:   42,
		Protocol:    ContextJournalProtocolOpenAIResponses,
		RequestBody: []byte(`{"input":"hello"}`),
		ResponseID:  "resp_expire",
	})
	if err != nil {
		t.Fatalf("AppendTurn() error = %v", err)
	}
	if err := journal.BindResponse(ctx, 7, "resp_expire", ContextJournalResponseRef{
		GroupID:     7,
		SessionHash: "sess-expire",
		AccountID:   42,
		TurnID:      turn.TurnID,
	}, time.Minute); err != nil {
		t.Fatalf("BindResponse() error = %v", err)
	}

	now = now.Add(2 * time.Minute)
	turns, err := journal.ListTurns(ctx, 7, "sess-expire")
	if err != nil {
		t.Fatalf("ListTurns() error = %v", err)
	}
	if len(turns) != 0 {
		t.Fatalf("turn count after ttl = %d, want 0", len(turns))
	}
	ref, err := journal.GetResponse(ctx, 7, "resp_expire")
	if err != nil {
		t.Fatalf("GetResponse() error = %v", err)
	}
	if ref != nil {
		t.Fatalf("response ref after ttl = %+v, want nil", ref)
	}
}

func TestMemoryContextJournalGetResponseBindingAlias(t *testing.T) {
	ctx := context.Background()
	journal := NewMemoryContextJournal(ContextJournalOptions{
		TTL:             time.Hour,
		MaxSessionBytes: 1024,
	})
	if err := journal.BindResponse(ctx, 7, "resp_alias", ContextJournalResponseRef{
		SessionHash: "sess-alias",
		AccountID:   42,
		TurnID:      "turn_alias",
	}, time.Hour); err != nil {
		t.Fatalf("BindResponse() error = %v", err)
	}

	ref, err := journal.GetResponseBinding(ctx, 7, "resp_alias")
	if err != nil {
		t.Fatalf("GetResponseBinding() error = %v", err)
	}
	if ref == nil || ref.SessionHash != "sess-alias" || ref.AccountID != 42 || ref.TurnID != "turn_alias" {
		t.Fatalf("response binding = %+v", ref)
	}
}

func TestMemoryContextJournalRejectsSessionOverflow(t *testing.T) {
	ctx := context.Background()
	journal := NewMemoryContextJournal(ContextJournalOptions{
		TTL:             time.Hour,
		MaxSessionBytes: 12,
	})

	if _, err := journal.AppendTurn(ctx, ContextJournalAppendInput{
		GroupID:     7,
		SessionHash: "sess-overflow",
		AccountID:   42,
		RequestBody: []byte(`1234567890`),
	}); err != nil {
		t.Fatalf("first AppendTurn() error = %v", err)
	}
	_, err := journal.AppendTurn(ctx, ContextJournalAppendInput{
		GroupID:     7,
		SessionHash: "sess-overflow",
		AccountID:   42,
		RequestBody: []byte(`abc`),
	})
	if !errors.Is(err, ErrContextJournalSessionOverflow) {
		t.Fatalf("second AppendTurn() error = %v, want ErrContextJournalSessionOverflow", err)
	}
	state, ok := journal.SessionState(ctx, 7, "sess-overflow")
	if !ok {
		t.Fatal("SessionState() missing")
	}
	if !state.Overflow || state.TotalBytes != 10 {
		t.Fatalf("state = %+v", state)
	}
}

func TestMemoryContextJournalBuildReplayFromCompleteTurns(t *testing.T) {
	ctx := context.Background()
	journal := NewMemoryContextJournal(ContextJournalOptions{
		TTL:             time.Hour,
		MaxSessionBytes: 1024,
	})
	if _, err := journal.AppendTurn(ctx, ContextJournalAppendInput{
		GroupID:     7,
		SessionHash: "sess-replay",
		AccountID:   42,
		Protocol:    ContextJournalProtocolOpenAIResponses,
		RequestBody: []byte(`{"input":"hello"}`),
	}); err != nil {
		t.Fatalf("AppendTurn() error = %v", err)
	}

	replay, err := journal.BuildReplay(ctx, 7, "sess-replay")
	if err != nil {
		t.Fatalf("BuildReplay() error = %v", err)
	}
	if !replay.Safe || replay.Reason != ContextReplayReasonSafe {
		t.Fatalf("replay = %+v, want safe", replay)
	}
	if !bytes.Equal(replay.RequestBody, []byte(`{"input":"hello"}`)) {
		t.Fatalf("replay body = %s", replay.RequestBody)
	}
	replay.RequestBody[0] = '['
	again, err := journal.BuildReplay(ctx, 7, "sess-replay")
	if err != nil {
		t.Fatalf("BuildReplay() second error = %v", err)
	}
	if !bytes.Equal(again.RequestBody, []byte(`{"input":"hello"}`)) {
		t.Fatalf("replay leaked mutable body: %s", again.RequestBody)
	}
}

func TestMemoryContextJournalReplaySafetyReasons(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name  string
		input ContextJournalAppendInput
		want  ContextReplayReason
	}{
		{
			name: "function call output unsafe",
			input: ContextJournalAppendInput{
				RequestBody:           []byte(`{"input":[{"type":"function_call_output","output":"ok"}]}`),
				HasFunctionCallOutput: true,
			},
			want: ContextReplayReasonFunctionCallOutput,
		},
		{
			name: "encrypted reasoning unsafe",
			input: ContextJournalAppendInput{
				RequestBody:           []byte(`{"input":[{"type":"reasoning","encrypted_content":"abc"}]}`),
				HasEncryptedReasoning: true,
			},
			want: ContextReplayReasonEncryptedReasoning,
		},
		{
			name: "client output started unsafe",
			input: ContextJournalAppendInput{
				RequestBody:         []byte(`{"input":"hello"}`),
				ClientOutputStarted: true,
			},
			want: ContextReplayReasonClientOutputStarted,
		},
		{
			name: "missing body unsafe",
			input: ContextJournalAppendInput{
				RequestBody: nil,
			},
			want: ContextReplayReasonMissingBody,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			journal := NewMemoryContextJournal(ContextJournalOptions{
				TTL:             time.Hour,
				MaxSessionBytes: 1024,
			})
			tc.input.GroupID = 7
			tc.input.SessionHash = tc.name
			tc.input.AccountID = 42
			if len(tc.input.RequestBody) > 0 {
				if _, err := journal.AppendTurn(ctx, tc.input); err != nil {
					t.Fatalf("AppendTurn() error = %v", err)
				}
			}

			result, err := journal.IsReplaySafe(ctx, 7, tc.name)
			if err != nil {
				t.Fatalf("IsReplaySafe() error = %v", err)
			}
			if result.Safe {
				t.Fatalf("IsReplaySafe() = %+v, want unsafe", result)
			}
			if result.Reason != tc.want {
				t.Fatalf("reason = %q, want %q", result.Reason, tc.want)
			}
		})
	}
}

func TestMemoryContextJournalReplaySafetyReportsJournalOverflow(t *testing.T) {
	ctx := context.Background()
	journal := NewMemoryContextJournal(ContextJournalOptions{
		TTL:             time.Hour,
		MaxSessionBytes: 4,
	})

	_, err := journal.AppendTurn(ctx, ContextJournalAppendInput{
		GroupID:     7,
		SessionHash: "sess-overflow-replay",
		AccountID:   42,
		RequestBody: []byte(`12345`),
	})
	if !errors.Is(err, ErrContextJournalSessionOverflow) {
		t.Fatalf("AppendTurn() error = %v, want overflow", err)
	}
	result, err := journal.IsReplaySafe(ctx, 7, "sess-overflow-replay")
	if err != nil {
		t.Fatalf("IsReplaySafe() error = %v", err)
	}
	if result.Safe || result.Reason != ContextReplayReasonJournalOverflow {
		t.Fatalf("result = %+v, want journal overflow", result)
	}
}

func TestClassifyContextReplaySafety(t *testing.T) {
	cases := []struct {
		name string
		body []byte
		want ContextReplaySafety
	}{
		{
			name: "complete input is safe",
			body: []byte(`{"model":"gpt-4.1","input":[{"role":"user","content":"hello"}]}`),
			want: ContextReplaySafe,
		},
		{
			name: "complete messages are safe",
			body: []byte(`{"model":"gpt-4.1","messages":[{"role":"user","content":"hello"}]}`),
			want: ContextReplaySafe,
		},
		{
			name: "function call output is protected",
			body: []byte(`{"input":[{"type":"function_call_output","call_id":"call_1","output":"ok"}],"previous_response_id":"resp_1"}`),
			want: ContextReplayProtected,
		},
		{
			name: "encrypted reasoning is protected",
			body: []byte(`{"input":[{"type":"reasoning","encrypted_content":"abc"}]}`),
			want: ContextReplayProtected,
		},
		{
			name: "previous response only is protected",
			body: []byte(`{"previous_response_id":"resp_1"}`),
			want: ContextReplayProtected,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyContextReplaySafety(tc.body)
			if got != tc.want {
				t.Fatalf("ClassifyContextReplaySafety() = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestOpenAIGatewayServiceRecordContextJournalTurnBindsResponseID(t *testing.T) {
	ctx := context.Background()
	journal := NewMemoryContextJournal(ContextJournalOptions{
		TTL:             time.Hour,
		MaxSessionBytes: 1024,
	})
	svc := &OpenAIGatewayService{contextJournal: journal}
	groupID := int64(7)
	account := &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}

	turn, err := svc.RecordContextJournalTurn(ctx, &groupID, "sess-record", account, ContextJournalProtocolOpenAIResponses, []byte(`{"input":"hello"}`), &OpenAIForwardResult{
		ResponseID: "resp_record",
	})
	if err != nil {
		t.Fatalf("RecordContextJournalTurn() error = %v", err)
	}
	if turn == nil || turn.AccountID != 42 || turn.ResponseID != "resp_record" {
		t.Fatalf("turn = %+v", turn)
	}

	ref, err := journal.GetResponse(ctx, groupID, "resp_record")
	if err != nil {
		t.Fatalf("GetResponse() error = %v", err)
	}
	if ref == nil || ref.SessionHash != "sess-record" || ref.AccountID != 42 || ref.TurnID != turn.TurnID {
		t.Fatalf("response ref = %+v", ref)
	}
}

func TestOpenAIGatewayServiceRecordContextJournalTurnRequestIDFallbackOnlyForWS(t *testing.T) {
	ctx := context.Background()
	journal := NewMemoryContextJournal(ContextJournalOptions{
		TTL:             time.Hour,
		MaxSessionBytes: 1024,
	})
	svc := &OpenAIGatewayService{contextJournal: journal}
	groupID := int64(7)
	account := &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}

	httpTurn, err := svc.RecordContextJournalTurn(ctx, &groupID, "sess-http", account, ContextJournalProtocolOpenAIResponses, []byte(`{"input":"hello"}`), &OpenAIForwardResult{
		RequestID: "req_http",
	})
	if err != nil {
		t.Fatalf("RecordContextJournalTurn(http) error = %v", err)
	}
	if httpTurn == nil {
		t.Fatal("RecordContextJournalTurn(http) returned nil")
	}
	if httpTurn.ResponseID != "" {
		t.Fatalf("http turn ResponseID = %q, want empty", httpTurn.ResponseID)
	}
	ref, err := journal.GetResponse(ctx, groupID, "req_http")
	if err != nil {
		t.Fatalf("GetResponse(http request id) error = %v", err)
	}
	if ref != nil {
		t.Fatalf("http request id was bound as response id: %+v", ref)
	}

	wsTurn, err := svc.RecordContextJournalTurn(ctx, &groupID, "sess-ws", account, ContextJournalProtocolOpenAIResponses, []byte(`{"input":"hello"}`), &OpenAIForwardResult{
		RequestID:    "resp_ws_from_request_id",
		OpenAIWSMode: true,
	})
	if err != nil {
		t.Fatalf("RecordContextJournalTurn(ws) error = %v", err)
	}
	if wsTurn == nil || wsTurn.ResponseID != "resp_ws_from_request_id" {
		t.Fatalf("ws turn = %+v", wsTurn)
	}
	ref, err = journal.GetResponse(ctx, groupID, "resp_ws_from_request_id")
	if err != nil {
		t.Fatalf("GetResponse(ws response id) error = %v", err)
	}
	if ref == nil || ref.SessionHash != "sess-ws" || ref.AccountID != 42 || ref.TurnID != wsTurn.TurnID {
		t.Fatalf("ws response ref = %+v", ref)
	}
}

func TestOpenAIGatewayServiceRecordContextJournalTurnSkipsMissingSession(t *testing.T) {
	journal := NewMemoryContextJournal(ContextJournalOptions{
		TTL:             time.Hour,
		MaxSessionBytes: 1024,
	})
	svc := &OpenAIGatewayService{contextJournal: journal}
	account := &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}

	turn, err := svc.RecordContextJournalTurn(context.Background(), nil, "", account, ContextJournalProtocolOpenAIResponses, []byte(`{"input":"hello"}`), &OpenAIForwardResult{
		ResponseID: "resp_skip",
	})
	if err != nil {
		t.Fatalf("RecordContextJournalTurn() error = %v", err)
	}
	if turn != nil {
		t.Fatalf("turn = %+v, want nil", turn)
	}
}
