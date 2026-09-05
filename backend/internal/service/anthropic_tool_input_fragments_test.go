//go:build unit

package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAnthropicToolInputFragments 验证工具起始空对象不会污染后续参数 JSON。
func TestAnthropicToolInputFragments(t *testing.T) {
	input := appendRawJSON(json.RawMessage("{ \n\t }"), `{"query":`)
	input = appendRawJSON(input, `"status"}`)
	require.JSONEq(t, `{"query":"status"}`, string(input))
	require.Equal(t, `{"x":1}{"query":"status"}`, string(appendRawJSON(json.RawMessage(`{"x":1}`), `{"query":"status"}`)))
}
