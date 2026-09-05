package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestOpenAIImageCapabilityCooldownOnlyAffectsImages 验证工具能力缺失只暂停图片调度。
func TestOpenAIImageCapabilityCooldownOnlyAffectsImages(t *testing.T) {
	repo := &openAIImagesRateLimitRepo{}
	svc := &OpenAIGatewayService{accountRepo: repo}
	account := &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	ctx := withOpenAIImagesSelfBuiltRequest(context.Background())
	body := []byte(`{"error":{"message":"Tool 'image_generation' not found in 'tools' parameter"}}`)
	before := time.Now()
	require.False(t, svc.handleOpenAIAccountUpstreamErrorForModel(ctx, account, http.StatusBadRequest, nil, body, "gpt-5"))
	require.Len(t, repo.modelRateLimitCalls, 1)
	require.Equal(t, openAIImageGenerationRateLimitKey, repo.modelRateLimitCalls[0].scope)
	require.WithinDuration(t, before.Add(30*time.Minute), repo.modelRateLimitCalls[0].resetAt, time.Second)
	require.True(t, account.isModelRateLimitedWithContext(WithOpenAIImageGenerationIntent(context.Background()), "gpt-5"))
	require.False(t, account.isModelRateLimitedWithContext(context.Background(), "gpt-5"))
	require.False(t, svc.isOpenAIAccountRuntimeBlocked(account))
	require.Empty(t, repo.rateLimitCalls)
	require.Empty(t, repo.setErrorCalls)
}

// TestOpenAIImageCapabilityCooldownRequiresSelfBuiltError 避免将客户端错误或文字回复当作能力丢失。
func TestOpenAIImageCapabilityCooldownRequiresSelfBuiltError(t *testing.T) {
	for _, tc := range []struct {
		name      string
		selfBuilt bool
		status    int
		body      string
	}{
		{"passthrough invalid tools", false, 400, `{"error":{"message":"Tool 'image_generation' not found in 'tools' parameter"}}`},
		{"text reply", true, 200, `{"output_text":"I cannot use image_generation"}`},
		{"unrelated bad request", true, 400, `{"error":{"message":"Invalid image_generation size"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &openAIImagesRateLimitRepo{}
			svc := &OpenAIGatewayService{accountRepo: repo}
			ctx := context.Background()
			if tc.selfBuilt {
				ctx = withOpenAIImagesSelfBuiltRequest(ctx)
			}
			account := &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
			require.False(t, svc.handleOpenAIImageCapabilityLoss(ctx, account, tc.status, []byte(tc.body)))
			require.Empty(t, repo.modelRateLimitCalls)
		})
	}
}
