package service

import (
	"strings"
	"testing"
)

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

func TestClassifyOpenAIContextMigrationRecordsInputTypeBytes(t *testing.T) {
	imagePayload := strings.Repeat("a", 4096)
	body := []byte(`{"model":"gpt-5.5","input":[{"type":"message","content":"hi"},{"type":"image_generation_call","result":"` + imagePayload + `"},{"type":"function_call_output","output":"ok"}]}`)

	got := ClassifyOpenAIContextMigration(body, false)

	if got.InputTypeBytes["image_generation_call"] <= got.InputTypeBytes["message"] {
		t.Fatalf("InputTypeBytes = %+v", got.InputTypeBytes)
	}
	if got.LargestInputItemType != "image_generation_call" {
		t.Fatalf("LargestInputItemType = %q", got.LargestInputItemType)
	}
	if got.LargestInputItemBytes <= 0 {
		t.Fatalf("LargestInputItemBytes = %d", got.LargestInputItemBytes)
	}
	detail := got.DetailMap()
	inputTypeBytes, ok := detail["input_type_bytes"].(map[string]int)
	if !ok || inputTypeBytes["image_generation_call"] <= 0 {
		t.Fatalf("detail input_type_bytes = %+v", detail["input_type_bytes"])
	}
	if detail["largest_input_item_type"] != "image_generation_call" {
		t.Fatalf("detail largest_input_item_type = %+v", detail["largest_input_item_type"])
	}
}

func TestDiagnoseOpenAIRequestBodyReportsThresholdAndTopTypes(t *testing.T) {
	imagePayload := strings.Repeat("a", 8*1024*1024)
	functionPayload := strings.Repeat("b", 1024)
	body := []byte(`{"model":"gpt-5.5","input":[{"type":"image_generation_call","result":"` + imagePayload + `"},{"type":"function_call_output","output":"` + functionPayload + `"}]}`)

	diagnosis := DiagnoseOpenAIRequestBody(body)

	if diagnosis.Level != OpenAIRequestBodyDiagnosisLevel8MB {
		t.Fatalf("Level = %q, want %q; diagnosis=%+v", diagnosis.Level, OpenAIRequestBodyDiagnosisLevel8MB, diagnosis)
	}
	if diagnosis.TotalBytes != len(body) {
		t.Fatalf("TotalBytes = %d, want %d", diagnosis.TotalBytes, len(body))
	}
	if diagnosis.InputTypeBytes["image_generation_call"] <= diagnosis.InputTypeBytes["function_call_output"] {
		t.Fatalf("InputTypeBytes = %+v", diagnosis.InputTypeBytes)
	}
	if len(diagnosis.TopInputTypes) == 0 || diagnosis.TopInputTypes[0].Type != "image_generation_call" {
		t.Fatalf("TopInputTypes = %+v", diagnosis.TopInputTypes)
	}
	detail := diagnosis.DetailMap()
	if detail["request_body_diagnosis_level"] != OpenAIRequestBodyDiagnosisLevel8MB {
		t.Fatalf("detail = %+v", detail)
	}
}

func TestDiagnoseOpenAIRequestBodyRejectsAt32MB(t *testing.T) {
	imagePayload := strings.Repeat("a", 32*1024*1024)
	body := []byte(`{"model":"gpt-5.5","input":[{"type":"image_generation_call","result":"` + imagePayload + `"}]}`)

	diagnosis := DiagnoseOpenAIRequestBody(body)

	if diagnosis.Level != OpenAIRequestBodyDiagnosisLevelReject {
		t.Fatalf("Level = %q, want %q", diagnosis.Level, OpenAIRequestBodyDiagnosisLevelReject)
	}
	if !strings.Contains(diagnosis.UserMessage(), "image_generation_call") {
		t.Fatalf("UserMessage = %q", diagnosis.UserMessage())
	}
}
