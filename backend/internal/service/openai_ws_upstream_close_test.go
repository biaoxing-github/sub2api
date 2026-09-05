package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestWSIngressUpstreamNormalCloseBeforeTerminal 保证真实上游正常关闭仍保留未完成请求的故障来源。
func TestWSIngressUpstreamNormalCloseBeforeTerminal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		if _, _, err = conn.Read(r.Context()); err != nil {
			return
		}
		_ = conn.Write(r.Context(), coderws.MessageText, []byte(`{"type":"response.output_text.delta","delta":"hello"}`))
		_ = conn.Close(coderws.StatusNormalClosure, "done without terminal")
	}))
	defer upstream.Close()
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3
	svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: &httpUpstreamRecorder{}, cache: &stubGatewayCache{}, openaiWSResolver: NewOpenAIWSProtocolResolver(cfg), toolCorrector: NewCodexToolCorrector()}
	account := &Account{ID: 119, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1,
		Credentials: map[string]any{"api_key": "sk-test", "base_url": upstream.URL},
		Extra:       map[string]any{"responses_websockets_v2_enabled": true}}
	errorsCh := make(chan error, 1)
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			errorsCh <- err
			return
		}
		defer conn.CloseNow()
		_, first, err := conn.Read(r.Context())
		if err != nil {
			errorsCh <- err
			return
		}
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = r
		errorsCh <- svc.ProxyResponsesWebSocketFromClient(r.Context(), c, conn, account, "sk-test", first, nil)
	}))
	defer downstream.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(downstream.URL, "http"), nil)
	require.NoError(t, err)
	defer client.CloseNow()
	require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-5.1","input":"hi"}`)))
	_, message, err := client.Read(ctx)
	require.NoError(t, err)
	require.Contains(t, string(message), "hello")
	select {
	case err := <-errorsCh:
		require.Error(t, err)
		require.Equal(t, coderws.StatusNormalClosure, coderws.CloseStatus(err))
		require.True(t, IsOpenAIWSIngressUpstreamFailure(err))
	case <-ctx.Done():
		t.Fatal("waiting for upstream close attribution")
	}
}
