package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/antigravity"
	"github.com/stretchr/testify/require"
)

func TestNormalizeAntigravitySubscription_PaidTierWithIneligible(t *testing.T) {
	resp := &antigravity.LoadCodeAssistResponse{
		PaidTier: &antigravity.PaidTierInfo{ID: "g1-pro-tier"},
		IneligibleTiers: []*antigravity.IneligibleTier{
			{ReasonMessage: "location validation required"},
		},
	}

	result := NormalizeAntigravitySubscription(resp)
	require.Equal(t, "Pro", result.PlanType)
	require.Equal(t, "abnormal", result.SubscriptionStatus)
	require.Equal(t, "location validation required", result.SubscriptionError)
}

func TestNormalizeAntigravitySubscription_FreeTierWithIneligible(t *testing.T) {
	resp := &antigravity.LoadCodeAssistResponse{
		PaidTier: &antigravity.PaidTierInfo{ID: "free-tier"},
		IneligibleTiers: []*antigravity.IneligibleTier{
			{ReasonMessage: "some warning"},
		},
	}

	result := NormalizeAntigravitySubscription(resp)
	require.Equal(t, "Abnormal", result.PlanType)
	require.Equal(t, "abnormal", result.SubscriptionStatus)
}
