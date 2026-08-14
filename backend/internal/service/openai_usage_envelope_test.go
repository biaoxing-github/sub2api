package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractOpenAIUsageFromJSONBytes_NestedDataEnvelopes(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "chat usage in data",
			body: `{"data":{"usage":{"prompt_tokens":8,"completion_tokens":27,"prompt_tokens_details":{"cached_tokens":4}}}}`,
		},
		{
			name: "responses usage in data response",
			body: `{"data":{"response":{"usage":{"input_tokens":8,"output_tokens":27,"input_tokens_details":{"cached_tokens":4}}}}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usage, ok := extractOpenAIUsageFromJSONBytes([]byte(tt.body))

			require.True(t, ok)
			require.Equal(t, 8, usage.InputTokens)
			require.Equal(t, 27, usage.OutputTokens)
			require.Equal(t, 4, usage.CacheReadInputTokens)
		})
	}
}

func TestExtractOpenAIUsageFromJSONBytes_PreservesNativePathPriority(t *testing.T) {
	body := []byte(`{
		"usage":{"input_tokens":1,"output_tokens":2},
		"response":{"usage":{"input_tokens":3,"output_tokens":4}},
		"data":{"usage":{"prompt_tokens":5,"completion_tokens":6},"response":{"usage":{"input_tokens":7,"output_tokens":8}}}
	}`)

	usage, ok := extractOpenAIUsageFromJSONBytes(body)

	require.True(t, ok)
	require.Equal(t, 1, usage.InputTokens)
	require.Equal(t, 2, usage.OutputTokens)
}

func TestExtractOpenAIUsageFromJSONBytes_PrefersResponseOverData(t *testing.T) {
	body := []byte(`{
		"response":{"usage":{"input_tokens":11,"output_tokens":5}},
		"data":{"usage":{"prompt_tokens":100,"completion_tokens":50}}
	}`)

	usage, ok := extractOpenAIUsageFromJSONBytes(body)

	require.True(t, ok)
	require.Equal(t, 11, usage.InputTokens)
	require.Equal(t, 5, usage.OutputTokens)
}
