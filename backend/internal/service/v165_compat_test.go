//go:build unit

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type v165GrokAccountRepo struct {
	mockAccountRepoForGemini
	tempUnschedCalls int
}

func (r *v165GrokAccountRepo) SetTempUnschedulable(context.Context, int64, time.Time, string) error {
	r.tempUnschedCalls++
	return nil
}

func TestV165GrokDropsOrphanedToolChoice(t *testing.T) {
	patched, err := patchGrokResponsesBody([]byte(`{"model":"grok-4.5","input":"hello","tool_choice":"auto"}`), "grok-4.5")
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(patched, "tool_choice").Exists())

	withTools, err := patchGrokResponsesBody([]byte(`{"model":"grok-4.5","input":"hello","tools":[{"type":"function","name":"lookup"}],"tool_choice":"auto"}`), "grok-4.5")
	require.NoError(t, err)
	require.Equal(t, "auto", gjson.GetBytes(withTools, "tool_choice").String())
}

func TestV165GrokPoolModeKeepsSchedulingOn5xx(t *testing.T) {
	repo := &v165GrokAccountRepo{}
	svc := &OpenAIGatewayService{accountRepo: repo}
	pool := &Account{ID: 611, Platform: PlatformGrok, Type: AccountTypeAPIKey, Credentials: map[string]any{"pool_mode": true}}
	nonPool := &Account{ID: 612, Platform: PlatformGrok, Type: AccountTypeAPIKey}

	svc.handleGrokAccountUpstreamError(context.Background(), pool, http.StatusBadGateway, nil, nil)
	require.Zero(t, repo.tempUnschedCalls)

	svc.handleGrokAccountUpstreamError(context.Background(), nonPool, http.StatusBadGateway, nil, nil)
	require.Equal(t, 1, repo.tempUnschedCalls)
}

func TestV165PoolModeRetryDoesNotEnterModelCooldown(t *testing.T) {
	account := &Account{ID: 47, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
		"pool_mode":                    true,
		"pool_mode_retry_status_codes": []any{float64(524)},
	}}
	require.True(t, shouldPreserveOpenAIPoolModeRetry(account, 524))
	require.False(t, shouldPreserveOpenAIPoolModeRetry(account, http.StatusServiceUnavailable))
	require.False(t, shouldPreserveOpenAIPoolModeRetry(&Account{Type: AccountTypeAPIKey}, 524))
}

func TestV165OpenAIResponsesSanitizers(t *testing.T) {
	body := []byte(`{"model":"gpt-5.6-sol","input":[{"type":"message","id":"item_bad","namespace":"client","content":[{"type":"input_text","text":"hello","namespace":"nested"}]},{"type":"function_call","id":"fc_valid","call_id":"call_1","name":"lookup","arguments":"{}"},{"type":"function_call_output","id":"item_output","call_id":"call_1","output":"done"}]}`)

	withoutNamespaces, err := stripOpenAIResponsesInputNamespaces(body, false)
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(withoutNamespaces, "input.0.namespace").Exists())
	require.Equal(t, "nested", gjson.GetBytes(withoutNamespaces, "input.0.content.0.namespace").String())

	sanitized, changed, err := sanitizeOpenAIResponsesInputItemIDs(withoutNamespaces)
	require.NoError(t, err)
	require.True(t, changed)
	require.False(t, gjson.GetBytes(sanitized, "input.0.id").Exists())
	require.Equal(t, "fc_valid", gjson.GetBytes(sanitized, "input.1.id").String())
	require.Equal(t, "item_output", gjson.GetBytes(sanitized, "input.2.id").String())
}

func TestV165GeminiChatCompletionsPreservesInlineImage(t *testing.T) {
	geminiResp := map[string]any{"candidates": []any{map[string]any{
		"content": map[string]any{"parts": []any{
			map[string]any{"text": "rendered:\n"},
			map[string]any{"inlineData": map[string]any{"mimeType": "image/png", "data": "aW1hZ2U="}},
		}},
		"finishReason": "STOP",
	}}}
	rawData, err := json.Marshal(geminiResp)
	require.NoError(t, err)

	got, _, err := geminiResponseToChatCompletions(geminiResp, "gemini-test", rawData, nil)
	require.NoError(t, err)
	var content string
	require.NoError(t, json.Unmarshal(got.Choices[0].Message.Content, &content))
	require.Equal(t, "rendered:\n![image](data:image/png;base64,aW1hZ2U=)", content)

	withoutImage, _ := convertGeminiToClaudeMessage(geminiResp, "gemini-test", rawData, false)
	require.Len(t, withoutImage["content"].([]any), 1)
}

func TestV165PricingSchedulerBlankRemoteURLDoesNotStart(t *testing.T) {
	svc := NewPricingService(&config.Config{Pricing: config.PricingConfig{RemoteURL: "  \t  "}}, nil)
	defer svc.Stop()
	svc.startUpdateScheduler()

	done := make(chan struct{})
	go func() {
		svc.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("blank remote URL must not start scheduler")
	}
}

func TestV165ClaudeOpus5CatalogPricingAndBedrock(t *testing.T) {
	require.Contains(t, claude.DefaultModelIDs(), "claude-opus-5")
	require.Equal(t, "us.anthropic.claude-opus-5-v1", domain.DefaultBedrockModelMapping["claude-opus-5"])
	require.True(t, isBedrockClaude45OrNewer("us.anthropic.claude-opus-5-v1"))
	require.True(t, bedrockModelSupportsToolSearch("us.anthropic.claude-opus-5-v1"))
	require.True(t, isBedrockOpus47OrNewer("us.anthropic.claude-opus-5-v1"))

	billing := NewBillingService(&config.Config{}, nil)
	pricing, err := billing.GetModelPricing("claude-opus-5")
	require.NoError(t, err)
	require.InDelta(t, 5e-6, pricing.InputPricePerToken, 1e-12)
	require.InDelta(t, 25e-6, pricing.OutputPricePerToken, 1e-12)
}
