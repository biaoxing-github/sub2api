package middleware

import (
	"context"
	"regexp"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const clientRequestIDHeader = "X-Client-Request-ID"

var safeClientRequestIDPattern = regexp.MustCompile(`^api-key-probe-[A-Za-z0-9._:-]{1,120}$`)

// ClientRequestID ensures every request has a unique client_request_id in request.Context().
//
// This is used by the Ops monitoring module for end-to-end request correlation.
func ClientRequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request == nil {
			c.Next()
			return
		}

		if v := c.Request.Context().Value(ctxkey.ClientRequestID); v != nil {
			c.Next()
			return
		}

		id := strings.TrimSpace(c.GetHeader(clientRequestIDHeader))
		if id == "" || !safeClientRequestIDPattern.MatchString(id) {
			id = uuid.New().String()
		}
		ctx := context.WithValue(c.Request.Context(), ctxkey.ClientRequestID, id)
		requestLogger := logger.FromContext(ctx).With(zap.String("client_request_id", strings.TrimSpace(id)))
		ctx = logger.IntoContext(ctx, requestLogger)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
