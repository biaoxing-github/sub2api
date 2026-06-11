package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// sleepWithProbeKeepalive 按探测失败次数分级退避，期间发送 SSE 心跳保活。
// 返回 false 表示客户端已断开，调用方应立即 return。
func (h *OpenAIGatewayHandler) sleepWithProbeKeepalive(c *gin.Context, reqLog *zap.Logger, failureCount int, streamStarted *bool) bool {
	probeDelay := h.gatewayService.ProbeIntervalFromErrorCount(failureCount)
	heartbeatInterval := 1 * time.Second
	if probeDelay >= 60*time.Second {
		heartbeatInterval = 10 * time.Second
	} else if probeDelay >= 10*time.Second {
		heartbeatInterval = 5 * time.Second
	}
	reqLog.Warn("openai.scheduler_exhaustion_probe_backoff",
		zap.Duration("probe_delay", probeDelay),
		zap.Duration("heartbeat_interval", heartbeatInterval),
		zap.Int("probe_failure_count", failureCount),
	)
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()
	timer := time.NewTimer(probeDelay)
	defer timer.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			h.handleStreamingAwareError(c, 503, "api_error", "Request cancelled during scheduler exhaustion probe", *streamStarted)
			return false
		case <-ticker.C:
			if *streamStarted {
				c.Writer.WriteString(": keepalive\n\n")
				c.Writer.Flush()
			}
		case <-timer.C:
			return true
		}
	}
}
