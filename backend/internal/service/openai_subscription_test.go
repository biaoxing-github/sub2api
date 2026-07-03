package service

import "testing"

func TestShouldApplyChatGPTAccountInfoPlanType(t *testing.T) {
	tests := []struct {
		name      string
		current   string
		candidate string
		want      bool
	}{
		{name: "preserve existing pro plan", current: "pro", candidate: "self_serve_business_usage_based", want: false},
		{name: "preserve existing free plan", current: "free", candidate: "team", want: false},
		{name: "ignore empty candidate", current: "", candidate: "", want: false},
		{name: "apply candidate when current is empty", current: "", candidate: "pro", want: true},
		{name: "trim spaces before deciding", current: "  ", candidate: " plus ", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldApplyChatGPTAccountInfoPlanType(tt.current, tt.candidate); got != tt.want {
				t.Fatalf("shouldApplyChatGPTAccountInfoPlanType(%q, %q) = %v, want %v", tt.current, tt.candidate, got, tt.want)
			}
		})
	}
}
