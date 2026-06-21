//go:build unit

package service

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGenerateSessionStringUsesAccountClaudeCLIVersion(t *testing.T) {
	t.Run("legacy metadata format for old override", func(t *testing.T) {
		session, err := generateSessionString("2.1.77")
		require.NoError(t, err)
		parsed := ParseMetadataUserID(session)
		require.NotNil(t, parsed)
		require.False(t, parsed.IsNewFormat)
	})

	t.Run("json metadata format for new override", func(t *testing.T) {
		session, err := generateSessionString("2.1.126")
		require.NoError(t, err)
		parsed := ParseMetadataUserID(session)
		require.NotNil(t, parsed)
		require.True(t, parsed.IsNewFormat)
	})
}

func TestAccountTestService_AnthropicAPIKeyUsesAccountClaudeCLIVersionOverride(t *testing.T) {
	gin.SetMode(gin.TestMode)

	account := Account{
		ID:          446,
		Name:        "legacy-cli-account",
		Platform:    PlatformAnthropic,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":            "upstream-anthropic-key",
			"base_url":           "https://anyrouter.example",
			"claude_cli_version": "2.1.126",
		},
	}
	upstream := &anthropicHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body:       io.NopCloser(bytes.NewBufferString("data: {\"type\":\"message_stop\"}\n\n")),
		},
	}
	svc := &AccountTestService{
		accountRepo:  stubOpenAIAccountRepo{accounts: []Account{account}},
		httpUpstream: upstream,
		cfg:          &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/446/test", bytes.NewReader(nil))

	err := svc.TestAccountConnection(c, account.ID, "claude-opus-4-8", "", AccountTestModeDefault)
	require.NoError(t, err)

	require.Equal(t, "claude-cli/2.1.126 (external, cli)", upstream.lastReq.Header.Get("User-Agent"))
	metadataUserID := gjson.GetBytes(upstream.lastBody, "metadata.user_id").String()
	parsed := ParseMetadataUserID(metadataUserID)
	require.NotNil(t, parsed)
	require.True(t, parsed.IsNewFormat)
}
