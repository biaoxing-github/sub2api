package service

import (
	"strings"
	"testing"
)

func TestQuickValidationPromptsAreShortDistinctQuestions(t *testing.T) {
	seen := make(map[string]struct{}, len(quickValidationPrompts))
	for index := range quickValidationPrompts {
		prompt := quickValidationPromptAt(index)
		if strings.TrimSpace(prompt) == "" {
			t.Fatalf("prompt %d is empty", index)
		}
		if strings.EqualFold(strings.TrimSpace(prompt), "hi") {
			t.Fatalf("prompt %d still uses fixed hi", index)
		}
		if len([]rune(prompt)) > 40 {
			t.Fatalf("prompt %d is too long: %q", index, prompt)
		}
		if _, exists := seen[prompt]; exists {
			t.Fatalf("prompt %d is duplicated: %q", index, prompt)
		}
		seen[prompt] = struct{}{}
	}
}

func TestRandomQuickValidationPromptComesFromQuestionPool(t *testing.T) {
	want := make(map[string]struct{}, len(quickValidationPrompts))
	for _, prompt := range quickValidationPrompts {
		want[prompt] = struct{}{}
	}

	for range 32 {
		if prompt := RandomQuickValidationPrompt(); prompt == "" {
			t.Fatal("random prompt is empty")
		} else if _, exists := want[prompt]; !exists {
			t.Fatalf("random prompt is outside the pool: %q", prompt)
		}
	}
}
