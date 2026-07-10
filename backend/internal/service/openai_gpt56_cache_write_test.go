package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/stretchr/testify/require"
)

func TestOpenAIUsageParsesNestedCacheWriteTokens(t *testing.T) {
	usage, ok := extractOpenAIUsageFromJSONBytes([]byte(`{
		"usage":{
			"input_tokens":900,
			"output_tokens":50,
			"input_tokens_details":{"cached_tokens":100,"cache_write_tokens":200}
		}
	}`))
	require.True(t, ok)
	require.Equal(t, 900, usage.InputTokens)
	require.Equal(t, 100, usage.CacheReadInputTokens)
	require.Equal(t, 200, usage.CacheCreationInputTokens)
}

func TestGPT56CacheWriteUsesOfficialTierPricing(t *testing.T) {
	pricingService := &PricingService{pricingData: map[string]*LiteLLMModelPricing{
		"gpt-5.6-sol": {
			InputCostPerToken:               5e-6,
			InputCostPerTokenPriority:       10e-6,
			OutputCostPerToken:              30e-6,
			OutputCostPerTokenPriority:      60e-6,
			CacheReadInputTokenCost:         0.5e-6,
			CacheReadInputTokenCostPriority: 1e-6,
		},
	}}
	billingService := NewBillingService(&config.Config{}, pricingService)
	tokens := UsageTokens{InputTokens: 700, OutputTokens: 50, CacheCreationTokens: 200, CacheReadTokens: 100}

	standard, err := billingService.CalculateCostWithServiceTier("gpt-5.6-sol", tokens, 1, "")
	require.NoError(t, err)
	require.InDelta(t, 200*6.25e-6, standard.CacheCreationCost, 1e-12)

	priority, err := billingService.CalculateCostWithServiceTier("gpt-5.6-sol", tokens, 1, "priority")
	require.NoError(t, err)
	require.InDelta(t, 200*12.5e-6, priority.CacheCreationCost, 1e-12)

	flex, err := billingService.CalculateCostWithServiceTier("gpt-5.6-sol", tokens, 1, "flex")
	require.NoError(t, err)
	require.InDelta(t, 200*3.125e-6, flex.CacheCreationCost, 1e-12)
}

func TestGPT56ExplicitZeroCacheWriteOverrideIsPreserved(t *testing.T) {
	zero := 0.0
	billingService := &BillingService{}
	resolver := NewModelPricingResolver(nil, billingService)
	resolved := &ResolvedPricing{
		Mode: BillingModeToken,
		BasePricing: &ModelPricing{
			InputPricePerToken:  5e-6,
			OutputPricePerToken: 30e-6,
		},
	}
	resolver.applyTokenOverrides(&ChannelModelPricing{CacheWritePrice: &zero}, resolved)

	cost, err := billingService.CalculateCostUnified(CostInput{
		Model:          "gpt-5.6-sol",
		Tokens:         UsageTokens{CacheCreationTokens: 100},
		RateMultiplier: 1,
		Resolver:       resolver,
		Resolved:       resolved,
	})
	require.NoError(t, err)
	require.Zero(t, cost.CacheCreationCost)
}

func TestEmbeddedGPT56PricingHasCacheWriteTiers(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "resources", "model-pricing", "model_prices_and_context_window.json"))
	require.NoError(t, err)

	pricingData, err := (&PricingService{}).parsePricingData(body)
	require.NoError(t, err)
	for _, tc := range []struct {
		model                     string
		cacheWrite, cachePriority float64
	}{
		{model: "gpt-5.6-sol", cacheWrite: 6.25e-6, cachePriority: 12.5e-6},
		{model: "gpt-5.6-terra", cacheWrite: 3.125e-6, cachePriority: 6.25e-6},
		{model: "gpt-5.6-luna", cacheWrite: 1.25e-6, cachePriority: 2.5e-6},
	} {
		t.Run(tc.model, func(t *testing.T) {
			pricing := pricingData[tc.model]
			require.NotNil(t, pricing)
			require.InDelta(t, tc.cacheWrite, pricing.CacheCreationInputTokenCost, 1e-12)
			require.InDelta(t, tc.cachePriority, pricing.CacheCreationInputTokenCostPriority, 1e-12)
		})
	}
}

func TestCodexOfflineFallbackMeetsGPT56VersionGate(t *testing.T) {
	require.Equal(t, "0.144.1", openai.CodexCLIDefaultVersion)
	require.Contains(t, DefaultOpenAICodexUserAgent, "codex_cli_rs/"+openai.CodexCLIDefaultVersion)
}
