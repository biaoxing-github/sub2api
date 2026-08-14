package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestSanitizeOpenAIResponsesInputItemIDs_ReasoningPrefix(t *testing.T) {
	body := []byte(`{"input":[{"type":"reasoning","id":"reasoning_invalid","summary":[]},{"type":"reasoning","id":"rs_valid","summary":[]}]}`)

	sanitized, changed, err := sanitizeOpenAIResponsesInputItemIDs(body)

	require.NoError(t, err)
	require.True(t, changed)
	require.False(t, gjson.GetBytes(sanitized, "input.0.id").Exists())
	require.Equal(t, "rs_valid", gjson.GetBytes(sanitized, "input.1.id").String())
}
