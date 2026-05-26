package service

import (
	"fmt"
	"math"
	"time"
)

const (
	AccountAvailabilityRadarFastStable  = "fast_stable"
	AccountAvailabilityRadarSlowUsable  = "slow_usable"
	AccountAvailabilityRadarUnstable    = "unstable"
	AccountAvailabilityRadarBalanceRisk = "balance_risk"
	AccountAvailabilityRadarCooldown    = "cooldown"
	AccountAvailabilityRadarNeedsProbe  = "needs_probe"
)

type AccountLoadFactorAdvisorOptions struct {
	MinSamples int64
	Now        func() time.Time
}

type AccountAvailabilityRadar struct {
	Status  string   `json:"status"`
	Label   string   `json:"label"`
	Reasons []string `json:"reasons,omitempty"`
}

type AccountLoadFactorAdvice struct {
	SuggestedLoadFactor *int                     `json:"suggested_load_factor"`
	Reasons             []string                 `json:"reasons,omitempty"`
	AvailabilityRadar   AccountAvailabilityRadar `json:"availability_radar"`
	PathHealthSamples   int64                    `json:"path_health_samples"`
}

type AccountLoadFactorAdvisor struct {
	minSamples int64
	now        func() time.Time
}

func NewAccountLoadFactorAdvisor(options AccountLoadFactorAdvisorOptions) *AccountLoadFactorAdvisor {
	if options.MinSamples <= 0 {
		options.MinSamples = 3
	}
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	return &AccountLoadFactorAdvisor{
		minSamples: options.MinSamples,
		now:        options.Now,
	}
}

func (a *AccountLoadFactorAdvisor) Advise(account *Account, health OpenAIPathHealthRecord) AccountLoadFactorAdvice {
	if a == nil {
		a = NewAccountLoadFactorAdvisor(AccountLoadFactorAdvisorOptions{})
	}
	reasons := make([]string, 0, 4)
	if account == nil {
		reasons = append(reasons, "账号不存在")
		return AccountLoadFactorAdvice{
			Reasons:           reasons,
			AvailabilityRadar: buildAccountAvailabilityRadar(AccountAvailabilityRadarNeedsProbe, reasons),
		}
	}

	if health.Samples < a.minSamples {
		reasons = append(reasons, "样本不足", fmt.Sprintf("样本数 %d/%d", health.Samples, a.minSamples))
		return AccountLoadFactorAdvice{
			Reasons:           reasons,
			AvailabilityRadar: buildAccountAvailabilityRadar(AccountAvailabilityRadarNeedsProbe, reasons),
			PathHealthSamples: health.Samples,
		}
	}

	base := maxInt(account.EffectiveLoadFactor(), account.Concurrency, 1)
	successRate := accountLoadFactorSuccessRate(health)
	successFactor := clampFloat(successRate, 0.3, 1.2)
	speedFactor := accountLoadFactorSpeedFactor(health)
	healthFactor := accountLoadFactorHealthFactor(health.State)

	suggested := clampInt(int(math.Round(float64(base)*successFactor*speedFactor*healthFactor)), 1, 10000)
	reasons = append(reasons,
		fmt.Sprintf("基础容量 %d", base),
		fmt.Sprintf("成功率 %.0f%%", successRate*100),
		fmt.Sprintf("速度系数 %.2f", speedFactor),
		fmt.Sprintf("线路状态 %s", health.State),
	)

	status := a.accountAvailabilityRadarStatus(account, health, successRate)
	return AccountLoadFactorAdvice{
		SuggestedLoadFactor: &suggested,
		Reasons:             reasons,
		AvailabilityRadar:   buildAccountAvailabilityRadar(status, reasons),
		PathHealthSamples:   health.Samples,
	}
}

func (a *AccountLoadFactorAdvisor) accountAvailabilityRadarStatus(account *Account, health OpenAIPathHealthRecord, successRate float64) string {
	now := a.now()
	if health.State == OpenAIPathHealthStateOpenCircuit ||
		(account.OverloadUntil != nil && now.Before(*account.OverloadUntil)) ||
		(account.TempUnschedulableUntil != nil && now.Before(*account.TempUnschedulableUntil)) ||
		account.effectiveRateLimitResetAt(now) != nil {
		return AccountAvailabilityRadarCooldown
	}
	if accountBalanceRisk(account) {
		return AccountAvailabilityRadarBalanceRisk
	}
	if health.State == OpenAIPathHealthStateDegraded || health.ConsecutiveFailures > 0 || successRate < 0.75 ||
		health.EOFCount > 0 || health.HeaderTimeoutCount > 0 || health.Status401Count > 0 {
		return AccountAvailabilityRadarUnstable
	}
	if accountLoadFactorLatencyMs(health) > 1500 {
		return AccountAvailabilityRadarSlowUsable
	}
	return AccountAvailabilityRadarFastStable
}

func accountLoadFactorSuccessRate(health OpenAIPathHealthRecord) float64 {
	total := health.SuccessCount + health.FailureCount
	if total <= 0 {
		return 1
	}
	return float64(health.SuccessCount) / float64(total)
}

func accountLoadFactorSpeedFactor(health OpenAIPathHealthRecord) float64 {
	latency := accountLoadFactorLatencyMs(health)
	if latency <= 0 {
		return 1
	}
	return clampFloat(1000/latency, 0.5, 1.5)
}

func accountLoadFactorLatencyMs(health OpenAIPathHealthRecord) float64 {
	switch {
	case health.TTFTEWMAMs > 0 && health.HeaderWaitEWMAMs > 0:
		return (health.TTFTEWMAMs + health.HeaderWaitEWMAMs) / 2
	case health.TTFTEWMAMs > 0:
		return health.TTFTEWMAMs
	case health.HeaderWaitEWMAMs > 0:
		return health.HeaderWaitEWMAMs
	default:
		return 0
	}
}

func accountLoadFactorHealthFactor(state string) float64 {
	switch state {
	case OpenAIPathHealthStateHealthy, "":
		return 1.1
	case OpenAIPathHealthStateHalfOpen:
		return 0.5
	case OpenAIPathHealthStateDegraded:
		return 0.8
	case OpenAIPathHealthStateOpenCircuit:
		return 0.05
	default:
		return 0.8
	}
}

func accountBalanceRisk(account *Account) bool {
	if account == nil {
		return false
	}
	if account.IsAPIKeyOrBedrock() && account.IsQuotaExceeded() {
		return true
	}
	balance := UpstreamBalanceSnapshotFromExtra(account.Extra)
	if balance == nil {
		return false
	}
	if balance.FailedCount > 0 && balance.OKCount == 0 {
		return true
	}
	return balance.Available > 0 && balance.Available < 1
}

func buildAccountAvailabilityRadar(status string, reasons []string) AccountAvailabilityRadar {
	return AccountAvailabilityRadar{
		Status:  status,
		Label:   accountAvailabilityRadarLabel(status),
		Reasons: reasons,
	}
}

func accountAvailabilityRadarLabel(status string) string {
	switch status {
	case AccountAvailabilityRadarFastStable:
		return "快且稳"
	case AccountAvailabilityRadarSlowUsable:
		return "可用偏慢"
	case AccountAvailabilityRadarUnstable:
		return "不稳定"
	case AccountAvailabilityRadarBalanceRisk:
		return "余额风险"
	case AccountAvailabilityRadarCooldown:
		return "冷却中"
	case AccountAvailabilityRadarNeedsProbe:
		return "待探测"
	default:
		return "待探测"
	}
}

func clampFloat(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func maxInt(values ...int) int {
	out := values[0]
	for _, v := range values[1:] {
		if v > out {
			out = v
		}
	}
	return out
}
