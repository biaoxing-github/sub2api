package service

// ServiceTierBillingResolution 分离出站请求、上游观测和最终结算档位。
type ServiceTierBillingResolution struct {
	Requested  string // 策略处理后实际发往上游的档位。
	Observed   string // 上游响应的有效声明。
	Billing    string // 最终用于账单和用量记录的档位。
	Downgraded bool   // 上游实际服务档位更便宜。
}

// ResolveBillingServiceTier 仅允许上游明确的较低档位降低账单，不允许提高费用。
func ResolveBillingServiceTier(requested, observed string) ServiceTierBillingResolution {
	requested = normalizeBillingServiceTier(requested)
	observed = normalizeBillingServiceTier(observed)
	result := ServiceTierBillingResolution{Requested: requested, Observed: observed, Billing: requested}
	if observed == "" || observed == requested {
		return result
	}
	observedRank, known := serviceTierCostRank(observed)
	requestedRank, _ := serviceTierCostRank(requested)
	if known && observedRank < requestedRank {
		result.Billing = observed
		result.Downgraded = true
	}
	return result
}

// serviceTierCostRank 使用现有倍率顺序比较档位，未知值不触发调整。
func serviceTierCostRank(tier string) (int, bool) {
	switch normalizeBillingServiceTier(tier) {
	case "flex":
		return 0, true
	case "", "default", "standard", "auto", "scale":
		return 1, true
	case "priority", "fast":
		return 2, true
	default:
		return 1, false
	}
}

// ResolveOpenAIServiceTierBilling 保留 Codex OAuth 的 default 回显例外。
func ResolveOpenAIServiceTierBilling(account *Account, requested, observed string) ServiceTierBillingResolution {
	if account != nil && account.IsOpenAIOAuth() && normalizeBillingServiceTier(observed) == "default" {
		return ServiceTierBillingResolution{Requested: normalizeBillingServiceTier(requested), Observed: "default", Billing: normalizeBillingServiceTier(requested)}
	}
	return ResolveBillingServiceTier(requested, observed)
}
