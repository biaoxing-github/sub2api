package service

import (
	"net/http"
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

func TestAccount_IsPoolModeRetryableStatus_UsesConfiguredCodes(t *testing.T) {
	account := &Account{
		Credentials: map[string]any{
			"pool_mode_retry_status_codes": []any{float64(409), "503", 503, "bad", 99, 600},
		},
	}

	require.False(t, account.IsPoolModeRetryableStatus(http.StatusUnauthorized))
	require.True(t, account.IsPoolModeRetryableStatus(http.StatusConflict))
	require.True(t, account.IsPoolModeRetryableStatus(http.StatusServiceUnavailable))
}

func TestAccount_IsPoolModeRetryableStatus_EmptyConfiguredCodesDisablesStatusRetry(t *testing.T) {
	account := &Account{
		Credentials: map[string]any{
			"pool_mode_retry_status_codes": []any{},
		},
	}

	require.False(t, account.IsPoolModeRetryableStatus(http.StatusUnauthorized))
	require.False(t, account.IsPoolModeRetryableStatus(http.StatusTooManyRequests))
}
