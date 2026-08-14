package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCleanToolSchema_ConvertsNestedIntegerExclusiveMinimum(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"counts": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":             "integer",
					"exclusiveMinimum": float64(0),
				},
			},
			"strict": map[string]any{
				"type":             "integer",
				"exclusiveMinimum": 0,
				"minimum":          5,
			},
			"weak": map[string]any{
				"type":             "integer",
				"exclusiveMinimum": 2,
				"minimum":          1,
			},
		},
	}

	cleaned := cleanToolSchema(schema).(map[string]any)
	properties := cleaned["properties"].(map[string]any)
	items := properties["counts"].(map[string]any)["items"].(map[string]any)
	require.NotContains(t, items, "exclusiveMinimum")
	require.Equal(t, float64(1), items["minimum"])
	require.Equal(t, 5, properties["strict"].(map[string]any)["minimum"])
	require.Equal(t, 3, properties["weak"].(map[string]any)["minimum"])
}

func TestCleanToolSchema_DropsAmbiguousExclusiveMinimumWithoutConversion(t *testing.T) {
	for name, schema := range map[string]map[string]any{
		"number":     {"type": "number", "exclusiveMinimum": 0},
		"fractional": {"type": "integer", "exclusiveMinimum": 0.5},
	} {
		t.Run(name, func(t *testing.T) {
			cleaned := cleanToolSchema(schema).(map[string]any)
			require.NotContains(t, cleaned, "exclusiveMinimum")
			require.NotContains(t, cleaned, "minimum")
		})
	}
}
