package middleware

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const redactedDebugHeaderValue = "[redacted]"

type ClientRequestDebugSettingGetter interface {
	ClientRequestDebugLogEnabled() bool
}

func ClientRequestDebugLogger(getter ClientRequestDebugSettingGetter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if getter == nil || !getter.ClientRequestDebugLogEnabled() || c == nil || c.Request == nil {
			c.Next()
			return
		}

		body, readErr := readAndRestoreRequestBody(c.Request)
		fields := []zap.Field{
			zap.String("component", "http.client_request_debug"),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("query", c.Request.URL.RawQuery),
			zap.String("content_type", c.GetHeader("Content-Type")),
			zap.String("user_agent", c.GetHeader("User-Agent")),
			zap.String("client_ip", ip.GetClientIP(c)),
			zap.String("remote_addr", c.Request.RemoteAddr),
			zap.Int("request_body_bytes", len(body)),
			zap.String("request_body", string(body)),
			zap.Any("headers", sanitizedDebugHeaders(c.Request.Header)),
		}
		if authorization := c.GetHeader("Authorization"); authorization != "" {
			fields = append(fields, zap.String("authorization", redactedDebugHeaderValue))
		}
		if readErr != nil {
			fields = append(fields, zap.String("request_body_read_error", readErr.Error()))
		}

		logger.FromContext(c.Request.Context()).Info("client_request_debug.request_body", fields...)
		c.Next()
	}
}

func readAndRestoreRequestBody(req *http.Request) ([]byte, error) {
	if req == nil || req.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(req.Body)
	if closeErr := req.Body.Close(); err == nil && closeErr != nil {
		err = closeErr
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	return body, err
}

func sanitizedDebugHeaders(headers http.Header) map[string][]string {
	result := make(map[string][]string, len(headers))
	for key, values := range headers {
		copied := make([]string, len(values))
		if isSensitiveDebugHeader(key) {
			for i := range copied {
				copied[i] = redactedDebugHeaderValue
			}
		} else {
			copy(copied, values)
		}
		result[key] = copied
	}
	return result
}

func isSensitiveDebugHeader(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	if normalized == "" {
		return false
	}
	switch normalized {
	case "authorization", "proxy-authorization", "cookie", "set-cookie", "x-api-key", "x-goog-api-key":
		return true
	}
	return strings.Contains(normalized, "token") ||
		strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "password") ||
		strings.Contains(normalized, "api-key") ||
		strings.Contains(normalized, "apikey")
}
