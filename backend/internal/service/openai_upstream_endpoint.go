package service

import "github.com/gin-gonic/gin"

// ClearActualOpenAIUpstreamEndpoint 清理复用 Gin context 中上一次账号尝试的端点。
func ClearActualOpenAIUpstreamEndpoint(c *gin.Context) {
	if c != nil {
		c.Set(openAIUpstreamEndpointContextKey, "")
	}
}
