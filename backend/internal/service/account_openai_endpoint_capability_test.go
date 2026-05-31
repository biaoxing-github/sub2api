package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccount_SupportsOpenAIEndpointCapability_DefaultAllowsKnownEndpoints(t *testing.T) {
	account := &Account{
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{},
	}

	require.True(t, account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityChatCompletions))
	require.True(t, account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityEmbeddings))
}

func TestAccount_SupportsOpenAIEndpointCapability_ConfiguredListRestrictsEndpoint(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			openAIEndpointCapabilitiesCredentialKey: []any{"embeddings"},
		},
	}

	require.False(t, account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityChatCompletions))
	require.True(t, account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityEmbeddings))
}

func TestAccount_SupportsOpenAIEndpointCapability_EmbeddingsRequiresAPIKey(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			openAIEndpointCapabilitiesCredentialKey: map[string]any{
				string(OpenAIEndpointCapabilityEmbeddings): true,
			},
		},
	}

	require.False(t, account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityEmbeddings))
	require.True(t, account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityChatCompletions))
}
