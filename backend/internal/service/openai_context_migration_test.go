package service

import "testing"

func TestClassifyOpenAIContextMigration(t *testing.T) {
	cases := []struct {
		name       string
		body       []byte
		streamed   bool
		wantClass  string
		wantReason string
	}{
		{
			name:      "codex full portable context with tool outputs",
			body:      []byte(`{"model":"gpt-5.5","stream":true,"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]},{"type":"function_call_output","call_id":"call_1","output":"ok"},{"type":"reasoning","summary":[{"type":"summary_text","text":"kept"}]}]}`),
			wantClass: OpenAIContextMigrationPortableFull,
		},
		{
			name:      "codex compacted summary context",
			body:      []byte(`{"model":"gpt-5.5","stream":true,"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"summary"}]},{"type":"message","role":"developer","content":[{"type":"input_text","text":"rules"}]}]}`),
			wantClass: OpenAIContextMigrationPortableSummary,
		},
		{
			name:      "previous response only binds upstream",
			body:      []byte(`{"model":"gpt-5.5","previous_response_id":"resp_prev"}`),
			wantClass: OpenAIContextMigrationUpstreamBound,
		},
		{
			name:      "already streamed is unsafe",
			body:      []byte(`{"model":"gpt-5.5","input":[{"type":"message"}]}`),
			streamed:  true,
			wantClass: OpenAIContextMigrationUnsafeAfterStream,
		},
		{
			name:      "previous response with plain input is still upstream bound before snapshot check",
			body:      []byte(`{"model":"gpt-5.5","previous_response_id":"resp_prev","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"continue"}]}]}`),
			wantClass: OpenAIContextMigrationUpstreamBound,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyOpenAIContextMigration(tc.body, tc.streamed)
			if got.Class != tc.wantClass {
				t.Fatalf("Class = %q, want %q; observation=%+v", got.Class, tc.wantClass, got)
			}
			if got.RequestBodyBytes != len(tc.body) {
				t.Fatalf("RequestBodyBytes = %d, want %d", got.RequestBodyBytes, len(tc.body))
			}
		})
	}
}

func TestMarkOpenAIContextMigrationSnapshotReplayable(t *testing.T) {
	obs := ClassifyOpenAIContextMigration([]byte(`{"model":"gpt-5.5","previous_response_id":"resp_prev","input":[{"type":"message"}]}`), false)
	obs = markOpenAIContextMigrationSnapshotReplayable(obs, "journal_hit")

	if obs.Class != OpenAIContextMigrationSnapshotReplayable {
		t.Fatalf("Class = %q", obs.Class)
	}
	if !obs.SnapshotAvailable || !obs.SnapshotReplayable {
		t.Fatalf("snapshot flags = available:%v replayable:%v", obs.SnapshotAvailable, obs.SnapshotReplayable)
	}
	if obs.Reason != "journal_hit" {
		t.Fatalf("Reason = %q", obs.Reason)
	}
}

func TestOpenAIContextMigrationDetailMap(t *testing.T) {
	obs := ClassifyOpenAIContextMigration([]byte(`{"model":"gpt-5.5","input":[{"type":"message"},{"type":"function_call_output"}]}`), false)
	detail := obs.DetailMap()
	if detail["context_migration_class"] != OpenAIContextMigrationPortableFull {
		t.Fatalf("detail = %+v", detail)
	}
	if detail["input_item_count"] != 2 {
		t.Fatalf("input_item_count = %v", detail["input_item_count"])
	}
	if detail["function_call_output_count"] != 1 {
		t.Fatalf("function_call_output_count = %v", detail["function_call_output_count"])
	}
}
