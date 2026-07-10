//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestFilterCodexInput_PreservesOnlyValidFunctionCallIDsWhenPreservingReferences
// 验证续链模式仅保留合法 fc 前缀的 function_call.id，同时不影响 call_id 和其他类型的 id。
func TestFilterCodexInput_PreservesOnlyValidFunctionCallIDsWhenPreservingReferences(t *testing.T) {
	input := []any{
		map[string]any{
			"type":    "function_call",
			"id":      "fc_valid",
			"call_id": "fc_call_valid",
			"name":    "valid",
		},
		map[string]any{
			"type":    "function_call",
			"id":      "item_invalid",
			"call_id": "fc_call_item",
			"name":    "item",
		},
		map[string]any{
			"type":    "function_call",
			"id":      "call_invalid",
			"call_id": "fc_call_other",
			"name":    "other",
		},
		map[string]any{
			"type":    "function_call_output",
			"id":      "output_1",
			"call_id": "fc_call_item",
			"output":  "done",
		},
		map[string]any{
			"type": "message",
			"id":   "item_message",
			"role": "user",
		},
	}

	filtered := filterCodexInputWithOptions(input, codexInputFilterOptions{
		PreserveReferences: true,
	})

	require.Len(t, filtered, len(input))

	validCall, ok := filtered[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "fc_valid", validCall["id"])
	require.Equal(t, "fc_call_valid", validCall["call_id"])

	itemCall, ok := filtered[1].(map[string]any)
	require.True(t, ok)
	require.NotContains(t, itemCall, "id")
	require.Equal(t, "fc_call_item", itemCall["call_id"])

	otherCall, ok := filtered[2].(map[string]any)
	require.True(t, ok)
	require.NotContains(t, otherCall, "id")
	require.Equal(t, "fc_call_other", otherCall["call_id"])

	output, ok := filtered[3].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "output_1", output["id"])

	message, ok := filtered[4].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "item_message", message["id"])
}
