package service

import (
	"github.com/stretchr/testify/require"
	"testing"
)

// 空能力容器不限制路由，非空列表和显式关闭仍保持本地配置语义。
func TestAccountEmptyEndpointCapabilities(t *testing.T) {
	for _, raw := range []any{[]any{}, []string{}, map[string]any{}, map[string]bool{}} {
		a := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{openAIEndpointCapabilitiesCredentialKey: raw}}
		require.True(t, a.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityChatCompletions))
	}
	for _, raw := range []any{[]string{"embeddings"}, []any{"embeddings"}, map[string]any{"chat_completions": false}, map[string]bool{"chat_completions": false}} {
		a := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{openAIEndpointCapabilitiesCredentialKey: raw}}
		require.False(t, a.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityChatCompletions))
	}
}
