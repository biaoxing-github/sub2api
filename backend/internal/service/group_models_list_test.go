package service

import (
	"reflect"
	"testing"
)

// TestNormalizeGroupModelsListConfig_TrimDeduplicateAndDropEmpty 验证模型列表配置会裁剪空白、去重并丢弃空值。
func TestNormalizeGroupModelsListConfig_TrimDeduplicateAndDropEmpty(t *testing.T) {
	cfg := normalizeGroupModelsListConfig(GroupModelsListConfig{
		Enabled: true,
		Models: []string{
			" gpt-5.1 ",
			"",
			"gpt-5.1",
			"  gpt-5.1-codex  ",
			"   ",
		},
	})

	if !cfg.Enabled {
		t.Fatalf("expected enabled flag to be preserved")
	}
	want := []string{"gpt-5.1", "gpt-5.1-codex"}
	if !reflect.DeepEqual(want, cfg.Models) {
		t.Fatalf("expected normalized models %v, got %v", want, cfg.Models)
	}
}

// TestNormalizeGroupModelsListConfig_EmptyModelsPreservesDefault 验证空模型列表保持零值切片，便于 JSON omitempty 与默认禁用行为一致。
func TestNormalizeGroupModelsListConfig_EmptyModelsPreservesDefault(t *testing.T) {
	cfg := normalizeGroupModelsListConfig(GroupModelsListConfig{
		Enabled: true,
		Models:  []string{"", "   "},
	})

	if !cfg.Enabled {
		t.Fatalf("expected enabled flag to be preserved")
	}
	if cfg.Models != nil {
		t.Fatalf("expected empty normalized models to be nil, got %v", cfg.Models)
	}
}

// TestGroup_CustomModelsListEnabled 验证自定义 /v1/models 列表只有在开关开启且模型列表非空时启用。
func TestGroup_CustomModelsListEnabled(t *testing.T) {
	tests := []struct {
		name  string
		group *Group
		want  bool
	}{
		{name: "nil group", group: nil, want: false},
		{name: "zero config", group: &Group{}, want: false},
		{
			name: "enabled without models",
			group: &Group{ModelsListConfig: GroupModelsListConfig{
				Enabled: true,
			}},
			want: false,
		},
		{
			name: "models without enabled",
			group: &Group{ModelsListConfig: GroupModelsListConfig{
				Models: []string{"gpt-5.1"},
			}},
			want: false,
		},
		{
			name: "enabled with models",
			group: &Group{ModelsListConfig: GroupModelsListConfig{
				Enabled: true,
				Models:  []string{"gpt-5.1"},
			}},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.group.CustomModelsListEnabled(); got != tt.want {
				t.Fatalf("expected CustomModelsListEnabled() = %v, got %v", tt.want, got)
			}
		})
	}
}
