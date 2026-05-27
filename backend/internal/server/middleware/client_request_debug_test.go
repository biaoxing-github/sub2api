package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type clientRequestDebugSettingGetter struct {
	enabled bool
}

func (g clientRequestDebugSettingGetter) ClientRequestDebugLogEnabled() bool {
	return g.enabled
}

func TestClientRequestDebugLogger_DisabledDoesNotLogAndPreservesBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sink, restore := captureMiddlewareStructuredLog(t)
	defer restore()

	router := gin.New()
	router.Use(ClientRequestDebugLogger(clientRequestDebugSettingGetter{enabled: false}))
	router.POST("/v1/responses", func(c *gin.Context) {
		body, err := c.GetRawData()
		require.NoError(t, err)
		c.String(http.StatusOK, string(body))
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-5.5"}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"model":"gpt-5.5"}`, rec.Body.String())
	require.False(t, sink.ContainsMessage("client_request_debug.request_body"))
}

func TestClientRequestDebugLogger_EnabledLogsFullBodyAndPreservesBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sink, restore := captureMiddlewareStructuredLog(t)
	defer restore()

	router := gin.New()
	router.Use(ClientRequestDebugLogger(clientRequestDebugSettingGetter{enabled: true}))
	router.POST("/v1/responses", func(c *gin.Context) {
		body, err := c.GetRawData()
		require.NoError(t, err)
		c.String(http.StatusOK, string(body))
	})

	raw := `{"model":"gpt-5.5","input":[{"type":"input_text","text":"hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses?trace=1", strings.NewReader(raw))
	req.Header.Set("Authorization", "Bearer sk-secret")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Codex Desktop")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, raw, rec.Body.String())
	require.True(t, sink.ContainsMessage("client_request_debug.request_body"))
	require.True(t, sink.ContainsFieldValue("request_body", raw))
	require.True(t, sink.ContainsFieldValue("authorization", "[redacted]"))
	require.False(t, sink.ContainsFieldValue("authorization", "sk-secret"))
	require.True(t, sink.ContainsFieldValue("path", "/v1/responses"))
	require.True(t, sink.ContainsFieldValue("query", "trace=1"))
}

type middlewareInMemoryLogSink struct {
	events []*logger.LogEvent
}

func (s *middlewareInMemoryLogSink) WriteLogEvent(event *logger.LogEvent) {
	if event == nil {
		return
	}
	cloned := *event
	if event.Fields != nil {
		cloned.Fields = make(map[string]any, len(event.Fields))
		for k, v := range event.Fields {
			cloned.Fields[k] = v
		}
	}
	s.events = append(s.events, &cloned)
}

func (s *middlewareInMemoryLogSink) ContainsMessage(substr string) bool {
	for _, ev := range s.events {
		if ev != nil && strings.Contains(ev.Message, substr) {
			return true
		}
	}
	return false
}

func (s *middlewareInMemoryLogSink) ContainsFieldValue(field, substr string) bool {
	for _, ev := range s.events {
		if ev == nil || ev.Fields == nil {
			continue
		}
		if strings.Contains(strings.TrimSpace(toStringForClientRequestDebugTest(ev.Fields[field])), substr) {
			return true
		}
	}
	return false
}

func captureMiddlewareStructuredLog(t *testing.T) (*middlewareInMemoryLogSink, func()) {
	t.Helper()
	err := logger.Init(logger.InitOptions{
		Level:       "debug",
		Format:      "json",
		ServiceName: "sub2api",
		Environment: "test",
		Output: logger.OutputOptions{
			ToStdout: false,
			ToFile:   false,
		},
		Sampling: logger.SamplingOptions{Enabled: false},
	})
	require.NoError(t, err)
	sink := &middlewareInMemoryLogSink{}
	logger.SetSink(sink)
	return sink, func() {
		logger.SetSink(nil)
	}
}

func toStringForClientRequestDebugTest(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
