package xai

import "testing"

// TestDefaultModelsIncludeGrok45AsDefaultAliasTarget 校验 Grok 默认模型顺序和别名映射都指向 Grok 4.5。
func TestDefaultModelsIncludeGrok45AsDefaultAliasTarget(t *testing.T) {
	t.Parallel()

	ids := DefaultModelIDs()
	if len(ids) == 0 || ids[0] != "grok-4.5" {
		t.Fatalf("DefaultModelIDs() first model = %q, want grok-4.5", firstModelID(ids))
	}

	mapping := DefaultModelMapping()
	if got := mapping["grok"]; got != "grok-4.5" {
		t.Fatalf("DefaultModelMapping()[grok] = %q, want grok-4.5", got)
	}
	if got := mapping["grok-latest"]; got != "grok-4.5" {
		t.Fatalf("DefaultModelMapping()[grok-latest] = %q, want grok-4.5", got)
	}
	if got := mapping["grok-4.5"]; got != "grok-4.5" {
		t.Fatalf("DefaultModelMapping()[grok-4.5] = %q, want grok-4.5", got)
	}
}

// firstModelID 返回模型 ID 列表的首项，空列表返回空字符串，便于失败信息保持清晰。
func firstModelID(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	return ids[0]
}
