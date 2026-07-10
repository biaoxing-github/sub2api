package service

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestNormalizeOpenAICodexCompactReasoningEffortForAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"gpt-5.6-sol","reasoning":{"effort":"max"}}`)

	t.Run("oauth compact gpt56", func(t *testing.T) {
		ctx := newOpenAICompactTestContext("/v1/responses/compact")
		normalized, changed, err := normalizeOpenAICodexCompactReasoningEffortForAccount(ctx, &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeOAuth,
		}, body)
		require.NoError(t, err)
		require.True(t, changed)
		require.JSONEq(t, `{"model":"gpt-5.6-sol","reasoning":{"effort":"xhigh"}}`, string(normalized))
	})

	t.Run("api key remains max", func(t *testing.T) {
		ctx := newOpenAICompactTestContext("/v1/responses/compact")
		normalized, changed, err := normalizeOpenAICodexCompactReasoningEffortForAccount(ctx, &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
		}, body)
		require.NoError(t, err)
		require.False(t, changed)
		require.JSONEq(t, string(body), string(normalized))
	})

	t.Run("regular responses remains max", func(t *testing.T) {
		ctx := newOpenAICompactTestContext("/v1/responses")
		normalized, changed, err := normalizeOpenAICodexCompactReasoningEffortForAccount(ctx, &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeOAuth,
		}, body)
		require.NoError(t, err)
		require.False(t, changed)
		require.JSONEq(t, string(body), string(normalized))
	})
}

func newOpenAICompactTestContext(path string) *gin.Context {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", path, nil)
	return ctx
}
