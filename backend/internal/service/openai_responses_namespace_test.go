package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// API Key 请求声明 namespace 工具时，历史工具调用必须保留对应 namespace。
func TestAPIKeyNamespaceDeclarationPreservesHistoricalToolCallNamespace(t *testing.T) {
	body := []byte(`{"tools":[{"type":"namespace","name":"collaboration","tools":[]}],"input":[{"type":"function_call","namespace":"collaboration","name":"spawn_agent","arguments":"{}"},{"type":"message","namespace":"stale","role":"assistant","content":[]}]}`)
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}

	keepToolCallNamespaces := shouldKeepOpenAIResponsesToolCallNamespaces(account, false, body)
	updated, err := stripOpenAIResponsesInputNamespaces(body, keepToolCallNamespaces)

	require.NoError(t, err)
	require.True(t, keepToolCallNamespaces)
	require.Equal(t, "collaboration", gjson.GetBytes(updated, "input.0.namespace").String())
	require.False(t, gjson.GetBytes(updated, "input.1.namespace").Exists())
}
