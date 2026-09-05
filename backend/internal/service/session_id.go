package service

import (
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

const maxPersistedSessionIDLength = 255

const (
	openCodeSessionAffinityHeader = "X-Session-Affinity"
	openCodeSessionIDHeader       = "X-Session-Id"
	openCodeNativeSessionHeader   = "X-OpenCode-Session"
	codeBuddyConversationHeader   = "X-Conversation-ID"
	claudeCodeSessionHeader       = "X-Claude-Code-Session-Id"
)

var clientSessionIDHeaders = []string{
	"session_id",
	"conversation_id",
	openCodeSessionAffinityHeader,
	openCodeSessionIDHeader,
	openCodeNativeSessionHeader,
	codeBuddyConversationHeader,
	claudeCodeSessionHeader,
}

// ClaudeCodeSessionIDFromHeader 单独提取 Claude 会话用于 Messages 粘性，不改变其他协议的缓存键。
func ClaudeCodeSessionIDFromHeader(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	return sanitizeSessionID(c.GetHeader(claudeCodeSessionHeader))
}

// ExtractClientSessionID 提取客户端显式提供的会话标识，仅用于 usage_logs.session_id。
// 该值不参与粘性路由、账号选择、request_id 或上游缓存键计算。
func ExtractClientSessionID(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	for _, header := range clientSessionIDHeaders {
		if sessionID := sanitizeSessionID(c.GetHeader(header)); sessionID != "" {
			return sessionID
		}
	}
	if isGrokRequestContext(c) {
		if sessionID := sanitizeSessionID(c.GetHeader(grokConversationIDHeader)); sessionID != "" {
			return sessionID
		}
	}
	return ""
}

// sanitizeSessionID 对齐 VARCHAR(255) 的存储边界，并拒绝控制字符和无效 UTF-8。
// 超长值整体丢弃，避免截断后让两个不同会话错误地合并。
func sanitizeSessionID(raw string) string {
	if !utf8.ValidString(raw) {
		return ""
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	count := 0
	for _, r := range trimmed {
		if r < 0x20 || r == 0x7f {
			return ""
		}
		count++
		if count > maxPersistedSessionIDLength {
			return ""
		}
	}
	return trimmed
}
