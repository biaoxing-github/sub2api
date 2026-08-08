package service

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
)

// WithResolvedTargetPlatform stores the concrete provider selected for a composite request.
func WithResolvedTargetPlatform(ctx context.Context, platform string) context.Context {
	platform = strings.TrimSpace(platform)
	if ctx == nil || platform == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxkey.ResolvedTargetPlatform, platform)
}

// WithResolvedUpstreamModel stores the concrete upstream model selected for a composite request.
func WithResolvedUpstreamModel(ctx context.Context, model string) context.Context {
	model = strings.TrimSpace(model)
	if ctx == nil || model == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxkey.ResolvedUpstreamModel, model)
}

// ResolvedTargetPlatformFromContext returns the provider selected by composite routing.
func ResolvedTargetPlatformFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	platform, ok := ctx.Value(ctxkey.ResolvedTargetPlatform).(string)
	platform = strings.TrimSpace(platform)
	return platform, ok && platform != ""
}

// DetectModelPlatform maps unambiguous public model IDs to a concrete provider.
func DetectModelPlatform(model string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(model))
	if normalized == "" {
		return "", false
	}
	normalized = strings.TrimPrefix(normalized, "models/")
	if slash := strings.IndexByte(normalized, '/'); slash > 0 {
		provider := strings.TrimSpace(normalized[:slash])
		rest := strings.TrimSpace(normalized[slash+1:])
		switch provider {
		case "anthropic", "claude":
			return PlatformAnthropic, true
		case "openai", "chatgpt":
			return PlatformOpenAI, true
		case "google", "google-ai-studio", "gemini":
			return PlatformGemini, true
		case "xai", "x-ai", "grok":
			return PlatformGrok, true
		}
		if rest != "" {
			normalized = strings.TrimPrefix(rest, "models/")
		}
	}
	switch {
	case strings.HasPrefix(normalized, "anthropic.claude-"), strings.HasPrefix(normalized, "claude-"):
		return PlatformAnthropic, true
	case strings.HasPrefix(normalized, "gpt-"), strings.HasPrefix(normalized, "chatgpt-"), strings.HasPrefix(normalized, "codex-"), strings.HasPrefix(normalized, "text-embedding-"), strings.HasPrefix(normalized, "text-moderation-"), strings.HasPrefix(normalized, "omni-moderation-"), strings.HasPrefix(normalized, "dall-e-"), strings.HasPrefix(normalized, "gpt-image-"), strings.HasPrefix(normalized, "tts-"), strings.HasPrefix(normalized, "whisper-"), hasOpenAISeriesPrefix(normalized):
		return PlatformOpenAI, true
	case strings.HasPrefix(normalized, "gemini-"), strings.HasPrefix(normalized, "learnlm-"):
		return PlatformGemini, true
	case normalized == "grok" || strings.HasPrefix(normalized, "grok-"):
		return PlatformGrok, true
	default:
		return "", false
	}
}

func hasOpenAISeriesPrefix(model string) bool {
	for _, prefix := range []string{"o1", "o3", "o4", "o5"} {
		if model == prefix || strings.HasPrefix(model, prefix+"-") {
			return true
		}
	}
	return false
}

func resolveCompositeTargetPlatform(ctx context.Context, group *Group, requestedModel string) (string, bool) {
	if platform, ok := ResolvedTargetPlatformFromContext(ctx); ok {
		return platform, true
	}
	if group == nil || group.Platform != PlatformComposite {
		return "", false
	}
	return DetectModelPlatform(requestedModel)
}

func isConcreteRequestPlatform(platform string) bool {
	switch platform {
	case PlatformAnthropic, PlatformOpenAI, PlatformGemini, PlatformAntigravity, PlatformGrok:
		return true
	default:
		return false
	}
}
