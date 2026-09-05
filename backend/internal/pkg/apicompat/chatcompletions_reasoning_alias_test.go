package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChatReasoningAlias_NonStreamingResponse(t *testing.T) {
	var response ChatCompletionsResponse
	require.NoError(t, json.Unmarshal([]byte(`{
		"id":"chatcmpl-reasoning-alias",
		"model":"reasoning-model",
		"choices":[{"message":{"role":"assistant","content":"final answer","reasoning":"fallback reasoning"},"finish_reason":"stop"}]
	}`), &response))

	out := ChatCompletionsResponseToResponses(&response, "reasoning-model", nil, nil, false, nil)

	require.Len(t, out.Output, 2)
	require.Equal(t, "reasoning", out.Output[0].Type)
	require.Equal(t, "fallback reasoning", out.Output[0].Summary[0].Text)
	require.Equal(t, "final answer", out.Output[1].Content[0].Text)
}

func TestChatReasoningAlias_StreamingResponse(t *testing.T) {
	var chunk ChatCompletionsChunk
	require.NoError(t, json.Unmarshal([]byte(`{
		"id":"chatcmpl-reasoning-alias",
		"model":"reasoning-model",
		"choices":[{"index":0,"delta":{"reasoning":"streamed fallback"},"finish_reason":null}]
	}`), &chunk))

	events := ChatCompletionsChunkToResponsesEvents(&chunk, NewChatCompletionsToResponsesStreamState("reasoning-model"))
	var deltas []string
	for _, event := range events {
		if event.Type == "response.reasoning_summary_text.delta" {
			deltas = append(deltas, event.Delta)
		}
	}
	require.Equal(t, []string{"streamed fallback"}, deltas)
}

func TestChatReasoningAlias_ReasoningContentTakesPrecedence(t *testing.T) {
	preferred := "preferred reasoning"
	fallback := "fallback reasoning"

	require.Equal(t, preferred, (ChatMessage{
		ReasoningContent: preferred,
		Reasoning:        fallback,
	}).reasoningText())
	require.Equal(t, &preferred, (ChatDelta{
		ReasoningContent: &preferred,
		Reasoning:        &fallback,
	}).reasoningText())
}
