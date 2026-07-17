package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsExplicitImageGenerationIntent(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "被动 namespace 不计为生图",
			body: `{"model":"gpt-5.5","tools":[{"type":"namespace","name":"image_gen","tools":[{"type":"function","name":"imagegen"}]}],"tool_choice":"auto"}`,
			want: false,
		},
		{
			name: "Responses Lite 被动 namespace 不计为生图",
			body: `{"model":"gpt-5.5","input":[{"type":"additional_tools","tools":[{"type":"namespace","name":"image_gen"}]}],"tool_choice":"auto"}`,
			want: false,
		},
		{
			name: "原生工具计为生图",
			body: `{"model":"gpt-5.5","tools":[{"type":"image_generation"}]}`,
			want: true,
		},
		{
			name: "显式 namespace 选择计为生图",
			body: `{"model":"gpt-5.5","tool_choice":{"type":"namespace","name":"image_gen"}}`,
			want: true,
		},
		{
			name: "函数式图片选择计为生图",
			body: `{"model":"gpt-5.5","tool_choice":{"function":{"namespace":"image_gen","name":"imagegen"}}}`,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, IsExplicitImageGenerationIntent(openAIResponsesEndpoint, "gpt-5.5", []byte(tt.body)))
		})
	}

	require.True(t, IsExplicitImageGenerationIntent("/v1/images/generations", "gpt-5.5", nil))
	require.True(t, IsExplicitImageGenerationIntent(openAIResponsesEndpoint, "gpt-image-2", nil))
}
