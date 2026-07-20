package service

import "testing"

func TestExtractAnthropicMonitorText(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "text after thinking", body: `{"content":[{"type":"thinking","thinking":""},{"type":"text","text":"2"}]}`, want: "2"},
		{name: "thinking only", body: `{"content":[{"type":"thinking","thinking":""}]}`, want: ""},
		{name: "multiple text blocks", body: `{"content":[{"type":"text","text":"answer"},{"type":"tool_use","name":"x"},{"type":"text","text":"2"}]}`, want: "answer\n2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractAnthropicMonitorText([]byte(tt.body)); got != tt.want {
				t.Fatalf("extractAnthropicMonitorText() = %q, want %q", got, tt.want)
			}
		})
	}
}
