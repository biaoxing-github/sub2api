package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResponsesUsageNestedCacheWritePresenceOverridesFallback(t *testing.T) {
	var usage ResponsesUsage
	err := json.Unmarshal([]byte(`{
		"cache_write_tokens":99,
		"input_tokens_details":{"cache_write_tokens":0,"cache_creation_tokens":88},
		"prompt_tokens_details":{"cache_write_tokens":77,"cache_creation_tokens":66}
	}`), &usage)
	require.NoError(t, err)
	require.Zero(t, usage.CacheCreationInputTokens)
}

func TestResponsesUsageNestedCacheWritePriority(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{name: "input cache write", body: `{"input_tokens_details":{"cache_write_tokens":11},"prompt_tokens_details":{"cache_write_tokens":22}}`, want: 11},
		{name: "prompt cache write", body: `{"prompt_tokens_details":{"cache_write_tokens":22},"input_tokens_details":{"cache_creation_tokens":33}}`, want: 22},
		{name: "input cache creation", body: `{"input_tokens_details":{"cache_creation_tokens":33},"prompt_tokens_details":{"cache_creation_tokens":44}}`, want: 33},
		{name: "prompt cache creation", body: `{"prompt_tokens_details":{"cache_creation_tokens":44},"cache_write_tokens":55}`, want: 44},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var usage ResponsesUsage
			require.NoError(t, json.Unmarshal([]byte(tc.body), &usage))
			require.Equal(t, tc.want, usage.CacheCreationInputTokens)
		})
	}
}
