package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIGatewayServiceListSchedulableAccountsUsesRequestedPlatform(t *testing.T) {
	t.Parallel()

	repo := stubOpenAIAccountRepo{
		accounts: []Account{
			{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true},
			{ID: 2, Platform: PlatformGrok, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true},
		},
	}

	svc := &OpenAIGatewayService{
		accountRepo: &repo,
	}

	accounts, err := svc.listSchedulableAccounts(context.Background(), nil, PlatformGrok)
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	require.Equal(t, PlatformGrok, accounts[0].Platform)
}

func TestOpenAIGatewayServiceGetAccessTokenUsesGrokOAuthCredential(t *testing.T) {
	t.Parallel()

	svc := &OpenAIGatewayService{}
	account := &Account{
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "grok-access-token",
		},
	}

	token, tokenType, err := svc.GetAccessToken(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, "grok-access-token", token)
	require.Equal(t, "oauth", tokenType)
}

func TestOpenAIGatewayServiceGetAccessTokenUsesGrokAPIKeyCredential(t *testing.T) {
	t.Parallel()

	svc := &OpenAIGatewayService{}
	account := &Account{
		Platform: PlatformGrok,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "xai-api-key",
		},
	}

	token, tokenType, err := svc.GetAccessToken(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, "xai-api-key", token)
	require.Equal(t, "apikey", tokenType)
}
