//go:build unit

package service

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestOpenAISessionHeaderHyphen 验证 HTTP 粘性、缓存身份及 WS 转发采用相同优先级。
func TestOpenAISessionHeaderHyphen(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	c.Request.Header.Set("session-id", "codex-session")
	c.Request.Header.Set("session_id", "legacy-session")
	svc := &OpenAIGatewayService{}
	require.Equal(t, "codex-session", svc.ExtractSessionID(c, nil))
	require.Equal(t, "codex-session", explicitOpenAISessionID(c, nil))
	want, _ := deriveOpenAISessionHashes("codex-session")
	require.Equal(t, want, svc.GenerateSessionHash(c, nil))
	require.Equal(t, "codex-session", resolveOpenAIWSSessionHeaders(c, "cache-key").SessionID)
	c.Request.Header.Del("session-id")
	require.Equal(t, "legacy-session", explicitOpenAISessionID(c, nil))
}
