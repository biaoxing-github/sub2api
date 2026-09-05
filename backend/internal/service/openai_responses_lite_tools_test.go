package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// Lite 转换必须关闭并行工具调用，同时保留超过 float64 精度范围的 JSON 数字。
func TestNormalizeOpenAIResponsesLitePayloadForAPIKeyPreservesLargeNumber(t *testing.T) {
	body := []byte(`{"model":"gpt-5.6-sol","sequence":900719925474099312345,"tools":[{"type":"function","name":"lookup"}],"parallel_tool_calls":true}`)
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}

	updated, changed, err := normalizeOpenAIResponsesLitePayloadForAccount(body, account)

	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "900719925474099312345", gjson.GetBytes(updated, "sequence").Raw)
	require.True(t, gjson.GetBytes(updated, "parallel_tool_calls").Exists())
	require.False(t, gjson.GetBytes(updated, "parallel_tool_calls").Bool())
}
