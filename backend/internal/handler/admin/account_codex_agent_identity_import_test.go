package admin

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestNormalizeCodexImportEntryAcceptsAgentIdentityAuthJSON(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	privateKeyBase64 := base64.StdEncoding.EncodeToString(der)

	item, err := normalizeCodexImportEntry(codexImportEntry{
		Index: 1,
		Value: map[string]any{
			"auth_mode": "agentIdentity",
			"agent_identity": map[string]any{
				"agent_runtime_id":           "runtime-import",
				"agent_private_key":          privateKeyBase64,
				"account_id":                 "account-import",
				"chatgpt_user_id":            "user-import",
				"email":                      "agent@example.invalid",
				"plan_type":                  "pro",
				"chatgpt_account_is_fedramp": false,
			},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, item)
	require.True(t, item.IsAgentIdentity)
	require.Equal(t, service.OpenAIAuthModeAgentIdentity, item.Credentials["auth_mode"])
	require.Equal(t, "runtime-import", item.Credentials["agent_runtime_id"])
	require.Equal(t, privateKeyBase64, item.Credentials["agent_private_key"])
	require.Equal(t, "account-import", item.Credentials["chatgpt_account_id"])
	require.Equal(t, "user-import", item.Credentials["chatgpt_user_id"])
	require.NotContains(t, item.Credentials, "access_token")
	require.NotContains(t, item.Credentials, "refresh_token")
	require.NotEmpty(t, item.WarningTexts)
}

func TestBuildCodexAgentIdentityKeysUseChatGPTAccountOnly(t *testing.T) {
	keys := buildCodexAgentIdentityKeys("team-a")
	require.Equal(t, []string{"account:team-a"}, keys)
}

func TestCodexAgentIdentityIndexSeparatesTeamsForSameUser(t *testing.T) {
	existing := service.Account{
		ID: 1,
		Credentials: map[string]any{
			"auth_mode":          service.OpenAIAuthModeAgentIdentity,
			"chatgpt_account_id": "team-a",
			"chatgpt_user_id":    "same-user",
			"agent_runtime_id":   "runtime-a",
		},
	}
	index := buildCodexAccountIndex([]service.Account{existing})

	teamBKeys := buildCodexAgentIdentityKeys("team-b")
	matched, _ := index.Find(teamBKeys, "same-user")
	require.Nil(t, matched)

	teamAKeys := buildCodexAgentIdentityKeys("team-a")
	matched, matchedKey := index.Find(teamAKeys, "same-user")
	require.NotNil(t, matched)
	require.Equal(t, int64(1), matched.ID)
	require.Equal(t, "account:team-a", matchedKey)
}

func TestImportCodexSessionsKeepsAgentIdentityTeamsSeparate(t *testing.T) {
	first := buildAgentIdentityImportValue(t, "runtime-a", "team-a", "same-user", "task-a")
	second := buildAgentIdentityImportValue(t, "runtime-b", "team-b", "same-user", "task-b")
	svc := newCodexImportMemoryAdminService(nil)
	handler := NewAccountHandler(svc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	result, err := handler.importCodexSessions(context.Background(), CodexSessionImportRequest{
		SkipDefaultGroupBind: boolPtr(true),
	}, []codexImportEntry{{Index: 1, Value: first}, {Index: 2, Value: second}})
	require.NoError(t, err)
	require.Equal(t, 2, result.Created)
	require.Zero(t, result.Updated)
	require.Zero(t, result.Skipped)
	require.Len(t, svc.createdAccounts, 2)
}

func TestImportCodexSessionsMergesAgentIdentityRuntimesForSameTeam(t *testing.T) {
	first := buildAgentIdentityImportValue(t, "runtime-a", "team-a", "same-user", "task-a")
	second := buildAgentIdentityImportValue(t, "runtime-b", "team-a", "same-user", "task-b")
	firstIdentity, ok := first["agent_identity"].(map[string]any)
	require.True(t, ok)
	existing := service.Account{
		ID:       41,
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeOAuth,
		Credentials: map[string]any{
			"auth_mode":          service.OpenAIAuthModeAgentIdentity,
			"agent_runtime_id":   firstIdentity["agent_runtime_id"],
			"agent_private_key":  firstIdentity["agent_private_key"],
			"task_id":            firstIdentity["task_id"],
			"chatgpt_account_id": firstIdentity["account_id"],
			"chatgpt_user_id":    firstIdentity["chatgpt_user_id"],
		},
	}
	svc := newCodexImportMemoryAdminService([]service.Account{existing})
	handler := NewAccountHandler(svc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	result, err := handler.importCodexSessions(context.Background(), CodexSessionImportRequest{
		SkipDefaultGroupBind: boolPtr(true),
	}, []codexImportEntry{{Index: 1, Value: second}})
	require.NoError(t, err)
	require.Zero(t, result.Created)
	require.Equal(t, 1, result.Updated)
	require.Len(t, svc.updatedAccounts, 1)
	require.Equal(t, "runtime-b", svc.updatedAccounts[0].input.Credentials["agent_runtime_id"])
	require.Equal(t, "task-b", svc.updatedAccounts[0].input.Credentials["task_id"])
}

func buildAgentIdentityImportValue(t *testing.T, runtimeID, accountID, userID, taskID string) map[string]any {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	return map[string]any{
		"auth_mode": "agentIdentity",
		"agent_identity": map[string]any{
			"agent_runtime_id":  runtimeID,
			"agent_private_key": base64.StdEncoding.EncodeToString(der),
			"task_id":           taskID,
			"account_id":        accountID,
			"chatgpt_user_id":   userID,
		},
	}
}

type codexImportMemoryAdminService struct {
	*stubAdminService
	nextID          int64
	updatedAccounts []struct {
		id    int64
		input *service.UpdateAccountInput
	}
}

func newCodexImportMemoryAdminService(accounts []service.Account) *codexImportMemoryAdminService {
	stub := newStubAdminService()
	stub.accounts = append([]service.Account(nil), accounts...)
	return &codexImportMemoryAdminService{
		stubAdminService: stub,
		nextID:           100,
	}
}

func (s *codexImportMemoryAdminService) CreateAccount(ctx context.Context, input *service.CreateAccountInput) (*service.Account, error) {
	s.createdAccounts = append(s.createdAccounts, input)
	if s.createAccountErr != nil {
		return nil, s.createAccountErr
	}
	account := service.Account{
		ID:          s.nextID,
		Name:        input.Name,
		Platform:    input.Platform,
		Type:        input.Type,
		Status:      service.StatusActive,
		Credentials: cloneCodexImportTestMap(input.Credentials),
		Extra:       cloneCodexImportTestMap(input.Extra),
	}
	s.nextID++
	s.accounts = append(s.accounts, account)
	return &account, nil
}

func (s *codexImportMemoryAdminService) UpdateAccount(ctx context.Context, id int64, input *service.UpdateAccountInput) (*service.Account, error) {
	s.updatedAccounts = append(s.updatedAccounts, struct {
		id    int64
		input *service.UpdateAccountInput
	}{id: id, input: input})
	if s.updateAccountErr != nil {
		return nil, s.updateAccountErr
	}
	for idx := range s.accounts {
		if s.accounts[idx].ID == id {
			s.accounts[idx].Credentials = cloneCodexImportTestMap(input.Credentials)
			s.accounts[idx].Extra = cloneCodexImportTestMap(input.Extra)
			return &s.accounts[idx], nil
		}
	}
	account := service.Account{ID: id, Status: service.StatusActive, Credentials: cloneCodexImportTestMap(input.Credentials)}
	return &account, nil
}

func (s *codexImportMemoryAdminService) GetAccount(ctx context.Context, id int64) (*service.Account, error) {
	for idx := range s.accounts {
		if s.accounts[idx].ID == id {
			return &s.accounts[idx], nil
		}
	}
	return s.stubAdminService.GetAccount(ctx, id)
}

func cloneCodexImportTestMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func boolPtr(v bool) *bool {
	return &v
}

func TestImportCodexSessionsCreatesAgentIdentityWithoutOAuthExpiry(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)

	svc := newCodexImportMemoryAdminService(nil)
	handler := NewAccountHandler(svc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	result, err := handler.importCodexSessions(context.Background(), CodexSessionImportRequest{
		SkipDefaultGroupBind: boolPtr(true),
	}, []codexImportEntry{{
		Index: 1,
		Value: map[string]any{
			"auth_mode": "agentIdentity",
			"agent_identity": map[string]any{
				"agent_runtime_id":  "runtime-import",
				"agent_private_key": base64.StdEncoding.EncodeToString(der),
				"task_id":           "task-import",
				"account_id":        "account-import",
				"chatgpt_user_id":   "user-import",
			},
		},
	}})
	require.NoError(t, err)
	require.Equal(t, 1, result.Created)
	require.Zero(t, result.Failed)
	require.Len(t, svc.createdAccounts, 1)
	created := svc.createdAccounts[0]
	require.Nil(t, created.ExpiresAt)
	require.Nil(t, created.AutoPauseOnExpired)
	require.Equal(t, service.OpenAIAuthModeAgentIdentity, created.Credentials["auth_mode"])
	require.NotContains(t, created.Credentials, "access_token")
	require.NotContains(t, created.Credentials, "refresh_token")
}
