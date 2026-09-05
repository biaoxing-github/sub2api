package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponsesInputToChatMessages_DeveloperRoleMapsToSystem(t *testing.T) {
	messages, err := responsesInputToChatMessages("", json.RawMessage(`[{"role":"developer","content":"follow project instructions"}]`))
	require.NoError(t, err)
	require.Len(t, messages, 1)

	assert.Equal(t, "system", messages[0].Role)
	assert.JSONEq(t, `"follow project instructions"`, string(messages[0].Content))
}

func TestResponsesInputToChatMessages_SkipsInvalidHistoricalFunctionCall(t *testing.T) {
	input := json.RawMessage(`[
		{"type":"function_call","call_id":"call_bad","name":"exec_command","arguments":"{\"cmd\": \"ssh root@HOST"},
		{"type":"function_call_output","call_id":"call_bad","output":"failed to parse function arguments"},
		{"type":"function_call","call_id":"call_ok","name":"exec_command","arguments":"{}"},
		{"type":"function_call_output","call_id":"call_ok","output":"ok"},
		{"role":"user","content":"continue"}
	]`)

	messages, err := responsesInputToChatMessages("", input)
	require.NoError(t, err)
	require.Len(t, messages, 3)
	require.Equal(t, "assistant", messages[0].Role)
	require.Len(t, messages[0].ToolCalls, 1)
	require.Equal(t, "call_ok", messages[0].ToolCalls[0].ID)
	require.Equal(t, "tool", messages[1].Role)
	require.Equal(t, "call_ok", messages[1].ToolCallID)
	require.Equal(t, "user", messages[2].Role)
}

func TestResponsesInputToChatMessages_SkipsInvalidEmptyCallIDOutput(t *testing.T) {
	input := json.RawMessage(`[
		{"type":"function_call","call_id":"","name":"exec_command","arguments":"{\"cmd\": \"ssh root@HOST"},
		{"type":"function_call_output","call_id":"","output":"failed to parse function arguments"},
		{"role":"user","content":"continue"}
	]`)

	messages, err := responsesInputToChatMessages("", input)
	require.NoError(t, err)
	require.Len(t, messages, 1)
	require.Equal(t, "user", messages[0].Role)
}

func TestChatCompletionsResponseToResponses_SkipsInvalidFunctionArguments(t *testing.T) {
	resp := &ChatCompletionsResponse{
		Model: "deepseek-v4-flash",
		Choices: []ChatChoice{{
			Message: ChatMessage{
				Role: "assistant",
				ToolCalls: []ChatToolCall{
					{ID: "call_bad", Type: "function", Function: ChatFunctionCall{Name: "exec_command", Arguments: `{"cmd": "ssh root@HOST`}},
					{ID: "call_ok", Type: "function", Function: ChatFunctionCall{Name: "exec_command", Arguments: `{}`}},
				},
			},
			FinishReason: "length",
		}},
	}

	out := ChatCompletionsResponseToResponses(resp, "deepseek-v4-flash", nil, nil, false, nil)
	require.Equal(t, "incomplete", out.Status)
	require.Len(t, out.Output, 1)
	require.Equal(t, "function_call", out.Output[0].Type)
	require.Equal(t, "call_ok", out.Output[0].CallID)
	require.Equal(t, `{}`, out.Output[0].Arguments)
}

func TestResponsesInputToChatMessages_KeepsChatCompletionRoles(t *testing.T) {
	input := json.RawMessage(`[
		{"role":"system","content":"system message"},
		{"role":"user","content":"user message"},
		{"role":"assistant","content":"assistant message"},
		{"role":"tool","content":"tool message"}
	]`)

	messages, err := responsesInputToChatMessages("", input)
	require.NoError(t, err)
	require.Len(t, messages, 4)

	assert.Equal(t, []string{"system", "user", "assistant", "tool"}, chatMessageRoles(messages))
}

func TestResponsesInputToChatMessages_EmptyRoleFallsBackToUser(t *testing.T) {
	messages, err := responsesInputToChatMessages("", json.RawMessage(`[{"role":"","content":"hello"}]`))
	require.NoError(t, err)
	require.Len(t, messages, 1)

	assert.Equal(t, "user", messages[0].Role)
}

func TestResponsesInputToChatMessages_DeveloperRoleTrimAndCaseInsensitive(t *testing.T) {
	input := json.RawMessage(`[
		{"role":" Developer ","content":"one"},
		{"role":"\tDEVELOPER\n","content":"two"}
	]`)

	messages, err := responsesInputToChatMessages("", input)
	require.NoError(t, err)
	require.Len(t, messages, 2)

	assert.Equal(t, []string{"system", "system"}, chatMessageRoles(messages))
}

func TestResponsesToChatCompletionsRequest_InstructionsAndInputDeveloperRole(t *testing.T) {
	req := &ResponsesRequest{
		Model:        "gpt-4o",
		Instructions: "Use concise answers.",
		Input: json.RawMessage(`[
			{"role":"developer","content":[{"type":"input_text","text":"Prefer JSON."}]},
			{"role":"user","content":"Hello"}
		]`),
	}

	out, err := ResponsesToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, out.Messages, 3)

	assert.Equal(t, []string{"system", "system", "user"}, chatMessageRoles(out.Messages))
	assert.JSONEq(t, `"Use concise answers."`, string(out.Messages[0].Content))
	assert.JSONEq(t, `"Prefer JSON."`, string(out.Messages[1].Content))
	assert.JSONEq(t, `"Hello"`, string(out.Messages[2].Content))
}

func TestResponsesToChatCompletionsRequest_TextFormat(t *testing.T) {
	tests := []struct {
		name               string
		textFormat         string
		wantResponseFormat string
	}{
		{
			name:       "json_object",
			textFormat: `{"type":"json_object"}`,
			wantResponseFormat: `{
				"type":"json_object"
			}`,
		},
		{
			name: "json_schema",
			textFormat: `{
				"type":"json_schema",
				"name":"answer",
				"schema":{
					"type":"object",
					"properties":{"ok":{"type":"boolean"}},
					"required":["ok"],
					"additionalProperties":false
				},
				"strict":true
			}`,
			wantResponseFormat: `{
				"type":"json_schema",
				"json_schema":{
					"name":"answer",
					"schema":{
						"type":"object",
						"properties":{"ok":{"type":"boolean"}},
						"required":["ok"],
						"additionalProperties":false
					},
					"strict":true
				}
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req ResponsesRequest
			require.NoError(t, json.Unmarshal([]byte(`{
				"model":"gpt-4o",
				"input":[{"role":"user","content":"Return JSON"}],
				"text":{"format":`+tt.textFormat+`}
			}`), &req))

			out, err := ResponsesToChatCompletionsRequest(&req)
			require.NoError(t, err)

			payload, err := json.Marshal(out)
			require.NoError(t, err)
			var serialized struct {
				ResponseFormat json.RawMessage `json:"response_format"`
			}
			require.NoError(t, json.Unmarshal(payload, &serialized))
			require.NotEmpty(t, serialized.ResponseFormat)
			assert.JSONEq(t, tt.wantResponseFormat, string(serialized.ResponseFormat))
		})
	}
}

func TestResponsesToChatCompletionsRequest_ParallelToolCalls(t *testing.T) {
	var req ResponsesRequest
	require.NoError(t, json.Unmarshal([]byte(`{
		"model":"gpt-4o",
		"input":[{"role":"user","content":"Use tools"}],
		"parallel_tool_calls":false
	}`), &req))

	out, err := ResponsesToChatCompletionsRequest(&req)
	require.NoError(t, err)

	payload, err := json.Marshal(out)
	require.NoError(t, err)
	assert.Contains(t, string(payload), `"parallel_tool_calls":false`)
}

func TestResponsesInputToChatMessages_ReasoningAndToolPairing(t *testing.T) {
	input := json.RawMessage(`[
		{"type":"reasoning","summary":[{"type":"summary_text","text":"plan"}]},
		{"type":"function_call","call_id":"call_1","name":"ping","arguments":"{}"},
		{"type":"function_call_output","call_id":"call_1","output":"pong"},
		{"type":"function_call","call_id":"call_2","name":"skip_me","arguments":"{}"}
	]`)

	messages, err := responsesInputToChatMessages("", input)
	require.NoError(t, err)
	require.Len(t, messages, 2)

	assert.Equal(t, "assistant", messages[0].Role)
	assert.Equal(t, "plan", messages[0].ReasoningContent)
	require.Len(t, messages[0].ToolCalls, 1)
	assert.Equal(t, "call_1", messages[0].ToolCalls[0].ID)
	assert.Equal(t, "ping", messages[0].ToolCalls[0].Function.Name)

	assert.Equal(t, "tool", messages[1].Role)
	assert.Equal(t, "call_1", messages[1].ToolCallID)
	assert.JSONEq(t, `"pong"`, string(messages[1].Content))
}

func chatMessageRoles(messages []ChatMessage) []string {
	roles := make([]string, 0, len(messages))
	for _, message := range messages {
		roles = append(roles, message.Role)
	}
	return roles
}
