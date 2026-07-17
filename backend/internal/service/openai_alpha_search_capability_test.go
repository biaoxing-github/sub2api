package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// alpha/search 的调度候选同时允许 OpenAI OAuth 与 API Key 账号，Grok 不应被误选。
func TestAccountSupportsOpenAIEndpointCapabilityAlphaSearch(t *testing.T) {
	apiKey := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	oauth := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	grok := &Account{Platform: PlatformGrok, Type: AccountTypeAPIKey}

	require.True(t, apiKey.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityAlphaSearch))
	require.True(t, oauth.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityAlphaSearch))
	require.False(t, grok.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityAlphaSearch))
}
