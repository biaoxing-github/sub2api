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

func TestOpenAIGatewayServiceListSchedulableAccountsIncludesGrokForOpenAIGroup(t *testing.T) {
	t.Parallel()

	groupID := int64(7)
	repo := grokOpenAICompatibleAccountRepo{
		accounts: []Account{
			{
				ID:            1,
				Platform:      PlatformOpenAI,
				Type:          AccountTypeAPIKey,
				Status:        StatusActive,
				Schedulable:   true,
				Credentials:   map[string]any{"api_key": "sk-openai"},
				AccountGroups: []AccountGroup{{GroupID: groupID}},
			},
			{
				ID:            2,
				Platform:      PlatformGrok,
				Type:          AccountTypeAPIKey,
				Status:        StatusActive,
				Schedulable:   true,
				Credentials:   map[string]any{"api_key": "xai-grok"},
				AccountGroups: []AccountGroup{{GroupID: groupID}},
			},
			{
				ID:            3,
				Platform:      PlatformGrok,
				Type:          AccountTypeAPIKey,
				Status:        StatusActive,
				Schedulable:   true,
				Credentials:   map[string]any{"api_key": "xai-other"},
				AccountGroups: []AccountGroup{{GroupID: 99}},
			},
		},
	}
	svc := &OpenAIGatewayService{
		accountRepo: repo,
	}

	accounts, err := svc.listSchedulableAccounts(context.Background(), &groupID, PlatformOpenAI)

	require.NoError(t, err)
	require.Equal(t, []int64{1, 2}, grokTestAccountIDs(accounts))
}

func TestOpenAIAccountSchedulerSelectsGrokForOpenAIChatCompletionsOnly(t *testing.T) {
	t.Parallel()

	groupID := int64(8)
	grokAccount := Account{
		ID:            21,
		Platform:      PlatformGrok,
		Type:          AccountTypeAPIKey,
		Status:        StatusActive,
		Schedulable:   true,
		Concurrency:   1,
		Credentials:   map[string]any{"api_key": "xai-chat"},
		AccountGroups: []AccountGroup{{GroupID: groupID}},
	}
	svc := &OpenAIGatewayService{
		accountRepo: grokOpenAICompatibleAccountRepo{
			accounts: []Account{grokAccount},
		},
	}
	scheduler := newDefaultOpenAIAccountScheduler(svc, nil)

	selection, _, err := scheduler.Select(context.Background(), OpenAIAccountScheduleRequest{
		GroupID:          &groupID,
		Platform:         PlatformOpenAI,
		RequiredEndpoint: OpenAIEndpointCapabilityChatCompletions,
	})
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, grokAccount.ID, selection.Account.ID)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}

	selection, _, err = scheduler.Select(context.Background(), OpenAIAccountScheduleRequest{
		GroupID:          &groupID,
		Platform:         PlatformOpenAI,
		RequiredEndpoint: OpenAIEndpointCapabilityResponses,
	})
	require.Error(t, err)
	require.Nil(t, selection)
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

type grokOpenAICompatibleAccountRepo struct {
	AccountRepository
	accounts []Account
}

func (r grokOpenAICompatibleAccountRepo) ListSchedulableByGroupIDAndPlatform(ctx context.Context, groupID int64, platform string) ([]Account, error) {
	return r.listByGroupAndPlatforms(groupID, []string{platform}), nil
}

func (r grokOpenAICompatibleAccountRepo) ListSchedulableByGroupIDAndPlatforms(ctx context.Context, groupID int64, platforms []string) ([]Account, error) {
	return r.listByGroupAndPlatforms(groupID, platforms), nil
}

func (r grokOpenAICompatibleAccountRepo) ListSchedulableByPlatform(ctx context.Context, platform string) ([]Account, error) {
	return r.listByPlatforms([]string{platform}), nil
}

func (r grokOpenAICompatibleAccountRepo) ListSchedulableByPlatforms(ctx context.Context, platforms []string) ([]Account, error) {
	return r.listByPlatforms(platforms), nil
}

func (r grokOpenAICompatibleAccountRepo) ListSchedulableUngroupedByPlatform(ctx context.Context, platform string) ([]Account, error) {
	return r.listUngroupedByPlatforms([]string{platform}), nil
}

func (r grokOpenAICompatibleAccountRepo) ListSchedulableUngroupedByPlatforms(ctx context.Context, platforms []string) ([]Account, error) {
	return r.listUngroupedByPlatforms(platforms), nil
}

func (r grokOpenAICompatibleAccountRepo) listByGroupAndPlatforms(groupID int64, platforms []string) []Account {
	result := make([]Account, 0, len(r.accounts))
	for _, acc := range r.accounts {
		account := acc
		if grokTestPlatformIn(acc.Platform, platforms) && account.IsSchedulable() && isAccountInRequestedGroup(&account, &groupID) {
			result = append(result, acc)
		}
	}
	return result
}

func (r grokOpenAICompatibleAccountRepo) listUngroupedByPlatforms(platforms []string) []Account {
	result := make([]Account, 0, len(r.accounts))
	for _, acc := range r.accounts {
		account := acc
		if grokTestPlatformIn(acc.Platform, platforms) && account.IsSchedulable() && isAccountInRequestedGroup(&account, nil) {
			result = append(result, acc)
		}
	}
	return result
}

func (r grokOpenAICompatibleAccountRepo) listByPlatforms(platforms []string) []Account {
	result := make([]Account, 0, len(r.accounts))
	for _, acc := range r.accounts {
		if grokTestPlatformIn(acc.Platform, platforms) && acc.IsSchedulable() {
			result = append(result, acc)
		}
	}
	return result
}

func grokTestPlatformIn(platform string, platforms []string) bool {
	for _, item := range platforms {
		if platform == item {
			return true
		}
	}
	return false
}

func grokTestAccountIDs(accounts []Account) []int64 {
	ids := make([]int64, 0, len(accounts))
	for _, account := range accounts {
		ids = append(ids, account.ID)
	}
	return ids
}
