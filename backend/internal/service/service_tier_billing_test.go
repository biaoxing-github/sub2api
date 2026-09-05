package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// 账单金额和持久化用量档位必须采用同一个结算结果。
func TestRecordUsageObservedServiceTier(t *testing.T) {
	for _, kind := range []string{AccountTypeAPIKey, AccountTypeOAuth} {
		repo := &openAIRecordUsageLogRepoStub{inserted: true}
		svc := newOpenAIRecordUsageServiceForTest(repo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
		tier := "priority"
		err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
			Result: &OpenAIForwardResult{RequestID: "tier-" + kind, Model: "gpt-5.4", ServiceTier: &tier, UpstreamResponseServiceTier: "default", Usage: OpenAIUsage{InputTokens: 100, OutputTokens: 50}},
			APIKey: &APIKey{ID: 1}, User: &User{ID: 2}, Account: &Account{ID: 3, Platform: PlatformOpenAI, Type: kind},
		})
		require.NoError(t, err)
		require.NotNil(t, repo.lastLog)
		base, err := svc.billingService.CalculateCost("gpt-5.4", UsageTokens{InputTokens: 100, OutputTokens: 50}, 1)
		require.NoError(t, err)
		want, multiplier := "default", 1.0
		if kind == AccountTypeOAuth {
			want, multiplier = "priority", 2
		}
		require.NotNil(t, repo.lastLog.ServiceTier)
		require.Equal(t, want, *repo.lastLog.ServiceTier)
		require.InDelta(t, base.TotalCost*multiplier, repo.lastLog.TotalCost, 1e-10)
	}
}

// 覆盖请求被降档、缺失/未知声明及 Codex OAuth default 回显例外。
func TestResolveOpenAIServiceTierBilling(t *testing.T) {
	for _, tc := range []struct{ kind, requested, observed, billing string }{
		{AccountTypeAPIKey, "priority", "default", "default"},
		{AccountTypeAPIKey, "priority", "flex", "flex"},
		{AccountTypeAPIKey, "default", "priority", "default"},
		{AccountTypeAPIKey, "priority", "", "priority"},
		{AccountTypeAPIKey, "priority", "unknown", "priority"},
		{AccountTypeOAuth, "priority", "default", "priority"},
		{AccountTypeOAuth, "priority", "flex", "flex"},
	} {
		a := &Account{Platform: PlatformOpenAI, Type: tc.kind}
		require.Equal(t, tc.billing, ResolveOpenAIServiceTierBilling(a, tc.requested, tc.observed).Billing)
	}
}

// Responses 前导事件的 service_tier 是回显；只有终态或一致的 Chat 声明有效。
func TestObservedServiceTierTerminalAndConflict(t *testing.T) {
	o := &upstreamResponseModelObserver{}
	o.ObserveOpenAI([]byte(`{"response":{"model":"gpt-5","service_tier":"flex"}}`), "response.created")
	require.Empty(t, o.ServiceTier())
	o.ObserveOpenAI([]byte(`{"response":{"model":"gpt-5","service_tier":"default"}}`), "response.completed")
	require.Equal(t, "default", o.ServiceTier())
	o = &upstreamResponseModelObserver{}
	o.ObserveOpenAI([]byte(`{"model":"gpt-5","service_tier":"priority"}`), "")
	o.ObserveOpenAI([]byte(`{"model":"gpt-5","service_tier":"default"}`), "")
	require.Empty(t, o.ServiceTier())
	o.ObserveOpenAI([]byte(`{"response":{"model":"gpt-5","service_tier":"flex"}}`), "response.completed")
	require.Equal(t, "flex", o.ServiceTier())
}
