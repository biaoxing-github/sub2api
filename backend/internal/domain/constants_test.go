package domain

import "testing"

func TestDefaultModelMappingsIncludeClaudeFable5(t *testing.T) {
	t.Parallel()

	if got := DefaultAntigravityModelMapping["claude-fable-5"]; got != "claude-fable-5" {
		t.Fatalf("unexpected Antigravity mapping for claude-fable-5: got %q", got)
	}

	if got := DefaultBedrockModelMapping["claude-fable-5"]; got != "us.anthropic.claude-fable-5-v1" {
		t.Fatalf("unexpected Bedrock mapping for claude-fable-5: got %q", got)
	}
}

func TestDefaultAntigravityModelMapping_ImageCompatibilityAliases(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"gemini-2.5-flash-image":         "gemini-2.5-flash-image",
		"gemini-2.5-flash-image-preview": "gemini-2.5-flash-image",
		"gemini-3.1-flash-image":         "gemini-3.1-flash-image",
		"gemini-3.1-flash-image-preview": "gemini-3.1-flash-image",
		"gemini-3-pro-image":             "gemini-3.1-flash-image",
		"gemini-3-pro-image-preview":     "gemini-3.1-flash-image",
	}

	for from, want := range cases {
		got, ok := DefaultAntigravityModelMapping[from]
		if !ok {
			t.Fatalf("expected mapping for %q to exist", from)
		}
		if got != want {
			t.Fatalf("unexpected mapping for %q: got %q want %q", from, got, want)
		}
	}
}
