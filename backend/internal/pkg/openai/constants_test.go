package openai

import (
	"strings"
	"testing"
)

func firstInstructionLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func TestDefaultTestModelUsesGPT56Terra(t *testing.T) {
	if DefaultTestModel != "gpt-5.6-terra" {
		t.Fatalf("DefaultTestModel = %q, want %q", DefaultTestModel, "gpt-5.6-terra")
	}
}

func TestDefaultModelsIncludeGPT56Family(t *testing.T) {
	ids := map[string]bool{}
	for _, id := range DefaultModelIDs() {
		ids[id] = true
	}
	for _, id := range []string{"gpt-5.6", "gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna"} {
		if !ids[id] {
			t.Fatalf("DefaultModelIDs missing %s", id)
		}
	}
}

func TestCodexBaseInstructionsForModel(t *testing.T) {
	cases := []struct {
		model    string
		wantHead string
	}{
		{"gpt-5-codex", "You are Codex, based on GPT-5"},
		{"gpt-5.3-codex", "You are Codex, based on GPT-5"},
		{"gpt-5.3-codex-spark", "You are Codex, based on GPT-5"},
		{"gpt-5.5", "You are Codex, a coding agent based on GPT-5"},
		{"gpt-5.6-sol", "You are Codex, a coding agent based on GPT-5"},
		{"gpt-5.6-terra", "You are Codex, a coding agent based on GPT-5"},
		{"gpt-5.6-luna", "You are Codex, a coding agent based on GPT-5"},
		{"gpt-5.4", "You are Codex, a coding agent based on GPT-5"},
		{"gpt-5.2", "You are GPT-5.2 running in the Codex CLI"},
		{"gpt-5.1", "You are GPT-5.1 running in the Codex CLI"},
		{"gpt-5", "You are Codex, a coding agent based on GPT-5"},
		{"", "You are Codex, a coding agent based on GPT-5"},
	}

	for _, c := range cases {
		got := strings.TrimSpace(CodexBaseInstructionsForModel(c.model))
		if got == "" {
			t.Errorf("model %q: got empty instructions", c.model)
			continue
		}
		if !strings.HasPrefix(got, c.wantHead) {
			t.Errorf("model %q: got prefix %q, want %q", c.model, firstInstructionLine(got), c.wantHead)
		}
	}
}
