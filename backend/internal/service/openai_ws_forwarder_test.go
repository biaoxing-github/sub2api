package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestIsOpenAIWSTokenEvent_TerminalEventsExcluded 验证终止事件不会污染首 token 统计。
func TestIsOpenAIWSTokenEvent_TerminalEventsExcluded(t *testing.T) {
	cases := []struct {
		name      string
		eventType string
		want      bool
	}{
		{name: "empty", eventType: "", want: false},
		{name: "whitespace", eventType: "   ", want: false},
		{name: "created", eventType: "response.created", want: false},
		{name: "in_progress", eventType: "response.in_progress", want: false},
		{name: "output_item_added", eventType: "response.output_item.added", want: false},
		{name: "output_item_done", eventType: "response.output_item.done", want: false},
		{name: "completed", eventType: "response.completed", want: false},
		{name: "done", eventType: "response.done", want: false},
		{name: "completed_padded", eventType: "  response.completed  ", want: false},
		{name: "done_padded", eventType: "  response.done  ", want: false},
		{name: "text_delta", eventType: "response.output_text.delta", want: true},
		{name: "function_args_delta", eventType: "response.function_call_arguments.delta", want: true},
		{name: "output_text_done", eventType: "response.output_text.done", want: true},
		{name: "output_audio_done", eventType: "response.output_audio.done", want: true},
		{name: "reasoning_summary_delta", eventType: "response.reasoning_summary_text.delta", want: true},
		{name: "error", eventType: "error", want: false},
		{name: "unknown_response_event", eventType: "response.reasoning_summary_part.added", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, isOpenAIWSTokenEvent(tc.eventType))
		})
	}
}

// TestIsOpenAIWSTokenEvent_DisjointWithTerminal 守护 token 事件和终止事件集合互斥。
func TestIsOpenAIWSTokenEvent_DisjointWithTerminal(t *testing.T) {
	terminalEvents := []string{
		"response.completed",
		"response.done",
		"response.failed",
		"response.incomplete",
		"response.cancelled",
		"response.canceled",
	}

	for _, eventType := range terminalEvents {
		t.Run(eventType, func(t *testing.T) {
			require.True(t, isOpenAIWSTerminalEvent(eventType))
			require.False(t, isOpenAIWSTokenEvent(eventType))
		})
	}
}
