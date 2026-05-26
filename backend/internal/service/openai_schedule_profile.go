package service

import "strings"

const (
	openAIAccountScheduleProfileFastShort   = "fast_short"
	openAIAccountScheduleProfileCodexStable = "codex_stable"
	openAIAccountScheduleProfileCompact     = "compact"
	openAIAccountScheduleProfileProbe       = "probe"
)

func openAIAccountScheduleProfileFromRequest(req OpenAIAccountScheduleRequest) string {
	switch strings.TrimSpace(req.Profile) {
	case openAIAccountScheduleProfileFastShort,
		openAIAccountScheduleProfileCodexStable,
		openAIAccountScheduleProfileCompact,
		openAIAccountScheduleProfileProbe:
		return strings.TrimSpace(req.Profile)
	}
	if req.RequireCompact {
		return openAIAccountScheduleProfileCompact
	}
	if strings.TrimSpace(req.PreviousResponseID) != "" || strings.TrimSpace(req.SessionHash) != "" || req.StickyAccountID > 0 {
		return openAIAccountScheduleProfileCodexStable
	}
	return openAIAccountScheduleProfileFastShort
}

func (s *OpenAIGatewayService) openAIProfileSchedulerWeights(req OpenAIAccountScheduleRequest) GatewayOpenAIWSSchedulerScoreWeightsView {
	weights := s.openAIWSSchedulerWeights()
	switch openAIAccountScheduleProfileFromRequest(req) {
	case openAIAccountScheduleProfileFastShort:
		weights.Priority *= 0.8
		weights.Load *= 0.8
		weights.Queue *= 0.9
		weights.ErrorRate *= 0.8
		weights.TTFT += s.openAIFastLaneTTFTWeight() + s.openAIFastLaneHeaderWaitWeight()
	case openAIAccountScheduleProfileCodexStable:
		weights.Priority *= 0.8
		weights.Load *= 0.9
		weights.Queue *= 1.2
		weights.ErrorRate *= 1.8
		weights.TTFT *= 0.45
	case openAIAccountScheduleProfileCompact:
		weights.Priority *= 0.8
		weights.Load *= 1.0
		weights.Queue *= 1.0
		weights.ErrorRate *= 1.4
		weights.TTFT *= 0.8
	case openAIAccountScheduleProfileProbe:
		weights.Priority = 0.1
		weights.Load = 0.2
		weights.Queue = 0.2
		weights.ErrorRate = 0.3
		weights.TTFT = 1.0
	}
	return weights
}

func openAIPathHealthScoreMultiplier(state string) float64 {
	switch state {
	case OpenAIPathHealthStateDegraded:
		return 0.65
	case OpenAIPathHealthStateHalfOpen:
		return 0.35
	case OpenAIPathHealthStateOpenCircuit:
		return 0
	default:
		return 1
	}
}
