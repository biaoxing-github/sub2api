package service

import (
	"context"
	"log/slog"
	"strings"
)

const featureKeyCodexImageGenerationBridge = "codex_image_generation_bridge"

func boolOverridePtr(v bool) *bool {
	return &v
}

func boolOverrideFromMap(values map[string]any, keys ...string) *bool {
	if values == nil {
		return nil
	}
	for _, key := range keys {
		if v, ok := values[key].(bool); ok {
			return boolOverridePtr(v)
		}
	}
	return nil
}

func platformBoolOverride(values map[string]any, key string, platform string) *bool {
	if values == nil {
		return nil
	}
	if v, ok := values[key].(bool); ok {
		return boolOverridePtr(v)
	}
	raw, ok := values[key].(map[string]any)
	if !ok {
		return nil
	}
	platform = strings.TrimSpace(platform)
	if platform == "" {
		return nil
	}
	if v, ok := raw[platform].(bool); ok {
		return boolOverridePtr(v)
	}
	return nil
}

// CodexImageGenerationBridgeOverride returns the channel-level override for Codex
// image_generation bridge injection. Nil means follow the global/account policy.
func (c *Channel) CodexImageGenerationBridgeOverride(platform string) *bool {
	if c == nil {
		return nil
	}
	return platformBoolOverride(c.FeaturesConfig, featureKeyCodexImageGenerationBridge, platform)
}

// CodexImageGenerationBridgeOverride returns the account-level override for Codex
// image_generation bridge injection. Nil means follow the channel/global policy.
func (a *Account) CodexImageGenerationBridgeOverride() *bool {
	if a == nil || a.Platform != PlatformOpenAI || a.Extra == nil {
		return nil
	}
	if override := boolOverrideFromMap(a.Extra, featureKeyCodexImageGenerationBridge, "codex_image_generation_bridge_enabled"); override != nil {
		return override
	}
	openaiConfig, _ := a.Extra[PlatformOpenAI].(map[string]any)
	return boolOverrideFromMap(openaiConfig, featureKeyCodexImageGenerationBridge, "codex_image_generation_bridge_enabled")
}

func (s *OpenAIGatewayService) disableCodexImageGenerationBridgeForAccount(ctx context.Context, account *Account, reason string) {
	if s == nil || account == nil || account.Platform != PlatformOpenAI {
		return
	}
	if account.Extra == nil {
		account.Extra = make(map[string]any)
	}
	account.Extra[featureKeyCodexImageGenerationBridge] = false
	if s.accountRepo == nil {
		return
	}
	persistCtx := context.Background()
	if ctx != nil {
		persistCtx = context.WithoutCancel(ctx)
	}
	if err := s.accountRepo.UpdateExtra(persistCtx, account.ID, map[string]any{featureKeyCodexImageGenerationBridge: false}); err != nil {
		slog.Warn("codex_image_generation_bridge_disable_failed", "account_id", account.ID, "reason", reason, "error", err)
		return
	}
	slog.Info("codex_image_generation_bridge_disabled", "account_id", account.ID, "reason", reason)
}
