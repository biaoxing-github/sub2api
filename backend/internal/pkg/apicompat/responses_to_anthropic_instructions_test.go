package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResponsesToAnthropicRequestInstructionsBecomeSystem(t *testing.T) {
	req := &ResponsesRequest{
		Model:        "claude-sonnet-4-20250514",
		Instructions: "You are a helpful assistant.",
		Input:        json.RawMessage(`[{"role":"user","content":"hello"}]`),
	}

	result, err := ResponsesToAnthropicRequest(req)
	require.NoError(t, err)

	var system string
	require.NoError(t, json.Unmarshal(result.System, &system))
	require.Equal(t, "You are a helpful assistant.", system)
	require.Len(t, result.Messages, 1)
	require.Equal(t, "user", result.Messages[0].Role)
}

func TestConvertResponsesInputToAnthropicDeveloperRoleBecomesSystem(t *testing.T) {
	input := json.RawMessage(`[
		{"role":"developer","content":[{"type":"input_text","text":"You are a code reviewer."}]},
		{"role":"user","content":"review this code"}
	]`)

	system, messages, err := convertResponsesInputToAnthropic("Main instruction.", input)
	require.NoError(t, err)

	var systemText string
	require.NoError(t, json.Unmarshal(system, &systemText))
	require.Equal(t, "Main instruction.\n\nYou are a code reviewer.", systemText)
	require.Len(t, messages, 1)
	require.Equal(t, "user", messages[0].Role)
}
