package service

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const openAIImageGenerationRateLimitKey = "openai:image_generation"

// openAIImagesSelfBuiltRequestContextKey 标记网关生成且工具声明完整的图片请求。
type openAIImagesSelfBuiltRequestContextKey struct{}

// withOpenAIImagesSelfBuiltRequest 将上游能力错误与客户端自带工具声明错误分开。
func withOpenAIImagesSelfBuiltRequest(ctx context.Context) context.Context {
	return context.WithValue(ctx, openAIImagesSelfBuiltRequestContextKey{}, true)
}

// handleOpenAIImageCapabilityLoss 仅暂停当前账号的图片工具，不改变文本请求可用性。
func (s *OpenAIGatewayService) handleOpenAIImageCapabilityLoss(ctx context.Context, account *Account, status int, body []byte) bool {
	selfBuilt, _ := ctx.Value(openAIImagesSelfBuiltRequestContextKey{}).(bool)
	if !selfBuilt || account == nil || !account.IsOpenAI() || status != http.StatusBadRequest || !account.ShouldHandleErrorCode(status) {
		return false
	}
	message := strings.ToLower(extractUpstreamErrorMessage(body))
	if !strings.Contains(message, "image_generation") || !strings.Contains(message, "not found in 'tools' parameter") {
		return false
	}
	if s != nil && s.accountRepo != nil {
		resetAt := time.Now().Add(30 * time.Minute)
		if err := s.accountRepo.SetModelRateLimit(ctx, account.ID, openAIImageGenerationRateLimitKey, resetAt); err != nil {
			slog.Warn("openai_image_capability_cooldown_failed", "account_id", account.ID, "error", err)
		}
		updateAccountModelRateLimitExtra(account, openAIImageGenerationRateLimitKey, resetAt)
		if s.schedulerSnapshot != nil {
			if err := s.schedulerSnapshot.UpdateAccountInCache(ctx, account); err != nil {
				slog.Warn("openai_image_capability_snapshot_failed", "account_id", account.ID, "error", err)
			}
		}
	}
	return true
}
