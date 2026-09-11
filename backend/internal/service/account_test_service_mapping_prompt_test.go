//go:build unit

package service

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// TestAccountTestService_MappedPrompt 验证上游实际收到的模型与问题，以及结构化结果的一致性。
func TestAccountTestService_MappedPrompt(t *testing.T) {
	for _, platform := range []string{"vertex", "bedrock", "gemini"} {
		for _, prompt := range []string{" 请只回答测试成功 ", ""} {
			t.Run(platform+"/"+prompt, func(t *testing.T) {
				account := Account{ID: 901, Platform: PlatformAnthropic, Type: AccountTypeServiceAccount, Credentials: map[string]any{}}
				mapped := "claude-sonnet-4-5@20250929"
				response := "data: {\"type\":\"message_stop\"}\n\n"
				promptPath := "messages.0.content.0.text"
				cache := newClaudeTokenCacheStub()
				switch platform {
				case "vertex":
					account.Credentials["service_account_json"] = map[string]any{"client_email": "test@example.com", "private_key": "test-key", "project_id": "test-project"}
					key, err := parseVertexServiceAccountKey(&account)
					require.NoError(t, err)
					cache.tokens[vertexServiceAccountCacheKey(&account, key)] = "test-token"
				case "bedrock":
					account.Type = AccountTypeBedrock
					account.Credentials["auth_mode"] = "apikey"
					account.Credentials["api_key"] = "test-key"
					account.Credentials["aws_region"] = "us-east-1"
					mapped = "us.anthropic.claude-sonnet-4-5-20250929-v1:0"
					response = `{"content":[{"text":"ok"}]}`
				case "gemini":
					account.Platform = PlatformGemini
					account.Type = AccountTypeAPIKey
					account.Credentials["api_key"] = "test-key"
					mapped = "gemini-2.5-flash"
					promptPath = "contents.0.parts.0.text"
					response = "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"ok\"}]}}]}\n\n"
				}
				account.Credentials["model_mapping"] = map[string]any{"test-alias": mapped}
				upstream := &queuedHTTPUpstream{responses: []*http.Response{{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(response))}}}
				svc := &AccountTestService{
					accountRepo: stubOpenAIAccountRepo{accounts: []Account{account}}, httpUpstream: upstream,
					claudeTokenProvider: &ClaudeTokenProvider{tokenCache: cache},
					cfg:                 &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
				}
				c, _ := newTestContext()
				result, err := svc.TestAccountConnectionWithResult(c, account.ID, "test-alias", prompt, "")
				require.NoError(t, err)
				require.Equal(t, mapped, result.Model)
				require.Len(t, upstream.requests, 1)
				require.Contains(t, upstream.requests[0].URL.Path, mapped)
				body, err := io.ReadAll(upstream.requests[0].Body)
				require.NoError(t, err)
				actualPrompt := gjson.GetBytes(body, promptPath).String()
				if prompt == "" {
					require.NotEmpty(t, actualPrompt)
				} else {
					require.Equal(t, strings.TrimSpace(prompt), actualPrompt)
				}
			})
		}
	}
}
