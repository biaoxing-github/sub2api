//go:build unit

package service

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newSessionHeaderContext(t *testing.T, headers map[string]string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	c.Request = req
	return c
}

func TestSanitizeSessionID(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "trim", in: "  sess-123  ", want: "sess-123"},
		{name: "control", in: "sess\r\nInjected", want: ""},
		{name: "invalid utf8", in: string([]byte{'s', 0xff}), want: ""},
		{name: "max", in: strings.Repeat("a", maxPersistedSessionIDLength), want: strings.Repeat("a", maxPersistedSessionIDLength)},
		{name: "overlong", in: strings.Repeat("a", maxPersistedSessionIDLength+1), want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, sanitizeSessionID(tc.in))
		})
	}
}

func TestExtractClientSessionID(t *testing.T) {
	for _, header := range clientSessionIDHeaders {
		t.Run(header, func(t *testing.T) {
			c := newSessionHeaderContext(t, map[string]string{header: "session-value"})
			require.Equal(t, "session-value", ExtractClientSessionID(c))
		})
	}

	precedence := newSessionHeaderContext(t, map[string]string{
		"session_id":      "primary",
		"conversation_id": "secondary",
	})
	require.Equal(t, "primary", ExtractClientSessionID(precedence))

	nonGrok := newSessionHeaderContext(t, map[string]string{grokConversationIDHeader: "ignored"})
	require.Empty(t, ExtractClientSessionID(nonGrok))

	grok := newSessionHeaderContext(t, map[string]string{grokConversationIDHeader: "grok-session"})
	grok.Set("api_key", &APIKey{Group: &Group{Platform: PlatformGrok}})
	require.Equal(t, "grok-session", ExtractClientSessionID(grok))
}

func TestExtractClientSessionIDNilContext(t *testing.T) {
	require.Empty(t, ExtractClientSessionID(nil))
	require.Empty(t, ExtractClientSessionID(&gin.Context{}))
}
