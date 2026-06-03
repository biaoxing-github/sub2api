package openai

import "testing"

func TestDefaultTestModelUsesGPT55(t *testing.T) {
	if DefaultTestModel != "gpt-5.5" {
		t.Fatalf("DefaultTestModel = %q, want %q", DefaultTestModel, "gpt-5.5")
	}
}
