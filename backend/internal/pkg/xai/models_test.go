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

// TestDefaultModelsIncludeComposerAndImagineModels 校验 v0.1.149 新增的 Composer 与图片模型可被后端模型接口发现。
func TestDefaultModelsIncludeComposerAndImagineModels(t *testing.T) {
	t.Parallel()

	ids := DefaultModelIDs()
	for _, expected := range []string{
		"grok-composer-2.5-fast",
		"grok-imagine",
		"grok-imagine-image",
		"grok-imagine-image-quality",
		"grok-imagine-edit",
	} {
		if !containsModelID(ids, expected) {
			t.Fatalf("DefaultModelIDs() does not contain %q: %v", expected, ids)
		}
	}

	if got := DefaultModelMapping()["composer-2.5"]; got != "grok-composer-2.5-fast" {
		t.Fatalf("DefaultModelMapping()[composer-2.5] = %q, want grok-composer-2.5-fast", got)
	}
}

// firstModelID 返回模型 ID 列表的首项，空列表返回空字符串，便于失败信息保持清晰。
func firstModelID(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	return ids[0]
}

// containsModelID 判断模型 ID 列表是否包含指定值。
func containsModelID(ids []string, expected string) bool {
	for _, id := range ids {
		if id == expected {
			return true
		}
	}
	return false
}
