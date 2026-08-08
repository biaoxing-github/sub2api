package handler

import (
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/sjson"
)

// resolveCompositeRequest 解析 Composite 分组的具体平台和上游模型，并把结果写入请求上下文。
// 非 Composite 分组保持原请求不变；Composite 未命中路由时返回错误，禁止继续进入账号调度。
func resolveCompositeRequest(c *gin.Context, resolver *service.CompositeRouteResolver, apiKey *service.APIKey, model, endpoint string, body []byte) ([]byte, string, error) {
	model = strings.TrimSpace(model)
	if c == nil || c.Request == nil || apiKey == nil || apiKey.Group == nil || apiKey.Group.Platform != service.PlatformComposite {
		return body, model, nil
	}
	if resolver == nil {
		return nil, "", fmt.Errorf("composite route resolver is unavailable")
	}
	if platform, ok := service.ResolvedTargetPlatformFromContext(c.Request.Context()); ok {
		upstreamModel, modelOK := service.ResolvedUpstreamModelFromContext(c.Request.Context())
		if modelOK && isConcreteCompositePlatform(platform) {
			if len(body) > 0 && upstreamModel != model {
				var err error
				body, err = sjson.SetBytes(body, "model", upstreamModel)
				if err != nil {
					return nil, "", fmt.Errorf("rewrite composite upstream model: %w", err)
				}
			}
			return body, upstreamModel, nil
		}
	}
	decision, err := resolver.Resolve(c.Request.Context(), apiKey.Group.ID, model, endpoint)
	if err != nil {
		return nil, "", err
	}
	if !decision.Matched || !isConcreteCompositePlatform(decision.TargetPlatform) {
		reason := strings.TrimSpace(decision.Reason)
		if reason == "" {
			reason = "no supported target platform"
		}
		return nil, "", fmt.Errorf("composite route not found for model %q endpoint %q: %s", model, endpoint, reason)
	}
	upstreamModel := strings.TrimSpace(decision.UpstreamModel)
	if upstreamModel == "" {
		upstreamModel = model
	}
	ctx := service.WithResolvedTargetPlatform(c.Request.Context(), decision.TargetPlatform)
	ctx = service.WithResolvedUpstreamModel(ctx, upstreamModel)
	c.Request = c.Request.WithContext(ctx)
	if len(body) > 0 && upstreamModel != model {
		body, err = sjson.SetBytes(body, "model", upstreamModel)
		if err != nil {
			return nil, "", fmt.Errorf("rewrite composite upstream model: %w", err)
		}
	}
	return body, upstreamModel, nil
}

// isConcreteCompositePlatform 限制 Composite 路由只能落到现有可转发平台。
func isConcreteCompositePlatform(platform string) bool {
	switch strings.TrimSpace(platform) {
	case service.PlatformAnthropic, service.PlatformOpenAI, service.PlatformGemini, service.PlatformAntigravity, service.PlatformGrok:
		return true
	default:
		return false
	}
}

// compositeOpenAIReasoningAllowed 仅当最终目标平台为 OpenAI 时启用 OpenAI reasoning policy。
func compositeOpenAIReasoningAllowed(c *gin.Context, apiKey *service.APIKey) bool {
	if apiKey == nil || apiKey.Group == nil {
		return false
	}
	if apiKey.Group.Platform == service.PlatformOpenAI {
		return true
	}
	if apiKey.Group.Platform != service.PlatformComposite || c == nil || c.Request == nil {
		return false
	}
	platform, ok := service.ResolvedTargetPlatformFromContext(c.Request.Context())
	return ok && platform == service.PlatformOpenAI
}
