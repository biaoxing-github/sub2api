package service

import (
	"fmt"
	"math"
	"strings"
)

const (
	AccountProbeGradeExcellent = "excellent"
	AccountProbeGradeStable    = "stable"
	AccountProbeGradeSlow      = "slow"
	AccountProbeGradeUnstable  = "unstable"
	AccountProbeGradePoor      = "poor"
)

type AccountProbeScore struct {
	Score        int      `json:"score"`
	Grade        string   `json:"grade"`
	Label        string   `json:"label"`
	Confidence   int      `json:"confidence"`
	ScoreItems   []string `json:"score_items,omitempty"`
	PenaltyItems []string `json:"penalty_items,omitempty"`
}

func ScoreAccountProbeRun(run AccountProbeResult) AccountProbeScore {
	requestCount := run.RequestCount
	if requestCount <= 0 {
		requestCount = run.SuccessCount + run.FailureCount
	}
	scoreItems := make([]string, 0, 5)
	penaltyItems := make([]string, 0, 6)

	successRate := 1.0
	if requestCount > 0 {
		successRate = float64(run.SuccessCount) / float64(requestCount)
	}
	successPoints := int(math.Round(successRate * 40))
	if successRate >= 1 {
		scoreItems = append(scoreItems, "成功率 100%，+40")
	} else {
		penaltyItems = append(penaltyItems, fmt.Sprintf("成功率 %.0f%%，-%d", successRate*100, 40-successPoints))
	}

	latencyPoints, latencyPenalties := accountProbeLatencyScore(run)
	penaltyItems = append(penaltyItems, latencyPenalties...)
	if latencyPoints >= 23 && run.Latency.AvgMillis > 0 {
		scoreItems = append(scoreItems, fmt.Sprintf("平均耗时 %dms，+%d", run.Latency.AvgMillis, latencyPoints))
	}

	firstTokenPoints, firstTokenPenalties := accountProbeFirstTokenScore(run)
	penaltyItems = append(penaltyItems, firstTokenPenalties...)
	if firstTokenPoints >= 14 && run.FirstTokenMillis != nil {
		scoreItems = append(scoreItems, fmt.Sprintf("首 Token %dms，+%d", *run.FirstTokenMillis, firstTokenPoints))
	}

	stabilityPoints, stabilityPenalties := accountProbeStabilityScore(run)
	penaltyItems = append(penaltyItems, stabilityPenalties...)
	if stabilityPoints == 15 {
		scoreItems = append(scoreItems, "无关键错误，+15")
	}

	tokenPoints, tokenPenalties := accountProbeTokenScore(run)
	penaltyItems = append(penaltyItems, tokenPenalties...)
	if tokenPoints == 5 && run.TotalTokens > 0 {
		scoreItems = append(scoreItems, fmt.Sprintf("Token 消耗 %d，+5", run.TotalTokens))
	}

	total := clampInt(successPoints+latencyPoints+firstTokenPoints+stabilityPoints+tokenPoints, 0, 100)
	confidence := accountProbeScoreConfidence(requestCount)
	if requestCount > 0 && requestCount < 3 {
		penaltyItems = append(penaltyItems, fmt.Sprintf("样本数不足 %d/3，可信度降低", requestCount))
	}

	grade := accountProbeGrade(total)
	return AccountProbeScore{
		Score:        total,
		Grade:        grade,
		Label:        accountProbeGradeLabel(grade),
		Confidence:   confidence,
		ScoreItems:   scoreItems,
		PenaltyItems: penaltyItems,
	}
}

func accountProbeLatencyScore(run AccountProbeResult) (int, []string) {
	avg := run.Latency.AvgMillis
	if avg == 0 {
		avg = run.AvgLatencyMillis
	}
	p95 := run.Latency.P95Millis
	if avg <= 0 && p95 <= 0 {
		return 15, []string{"缺少耗时样本，-10"}
	}
	points := 25
	var penalties []string
	switch {
	case avg > 12000:
		points -= 15
		penalties = append(penalties, fmt.Sprintf("平均耗时 %dms，-15", avg))
	case avg > 8000:
		points -= 10
		penalties = append(penalties, fmt.Sprintf("平均耗时 %dms，-10", avg))
	case avg > 4000:
		points -= 5
		penalties = append(penalties, fmt.Sprintf("平均耗时 %dms，-5", avg))
	}
	switch {
	case p95 > 20000:
		points -= 12
		penalties = append(penalties, fmt.Sprintf("P95 %dms，-12", p95))
	case p95 > 15000:
		points -= 10
		penalties = append(penalties, fmt.Sprintf("P95 %dms，-10", p95))
	case p95 > 8000:
		points -= 5
		penalties = append(penalties, fmt.Sprintf("P95 %dms，-5", p95))
	}
	return clampInt(points, 0, 25), penalties
}

func accountProbeFirstTokenScore(run AccountProbeResult) (int, []string) {
	if run.FirstTokenMillis == nil {
		if strings.EqualFold(run.RequestMode, AccountProbeRequestModeStream) {
			return 8, []string{"流式请求缺少首 Token 记录，-7"}
		}
		return 12, nil
	}
	firstToken := *run.FirstTokenMillis
	points := 15
	var penalties []string
	switch {
	case firstToken > 8000:
		points -= 12
		penalties = append(penalties, fmt.Sprintf("首 Token %dms，-12", firstToken))
	case firstToken > 5000:
		points -= 8
		penalties = append(penalties, fmt.Sprintf("首 Token %dms，-8", firstToken))
	case firstToken > 2500:
		points -= 4
		penalties = append(penalties, fmt.Sprintf("首 Token %dms，-4", firstToken))
	}
	return clampInt(points, 0, 15), penalties
}

func accountProbeStabilityScore(run AccountProbeResult) (int, []string) {
	points := 15
	seen := make(map[string]bool)
	var penalties []string
	for _, message := range accountProbeErrorMessages(run) {
		key, label, penalty := classifyAccountProbeError(message)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		points -= penalty
		penalties = append(penalties, fmt.Sprintf("%s，-%d", label, penalty))
	}
	if run.FailureCount > 0 && !seen["failure"] {
		points -= min(run.FailureCount*3, 9)
		penalties = append(penalties, fmt.Sprintf("失败请求 %d 条，-%d", run.FailureCount, min(run.FailureCount*3, 9)))
	}
	return clampInt(points, 0, 15), penalties
}

func accountProbeTokenScore(run AccountProbeResult) (int, []string) {
	switch {
	case run.TotalTokens <= 0:
		return 4, nil
	case run.TotalTokens > 6000:
		return 2, []string{fmt.Sprintf("Token 消耗 %d，-3", run.TotalTokens)}
	case run.TotalTokens > 3000:
		return 3, []string{fmt.Sprintf("Token 消耗 %d，-2", run.TotalTokens)}
	default:
		return 5, nil
	}
}

func accountProbeErrorMessages(run AccountProbeResult) []string {
	messages := make([]string, 0, len(run.Samples)+1)
	if strings.TrimSpace(run.ErrorMessage) != "" {
		messages = append(messages, run.ErrorMessage)
	}
	for _, sample := range run.Samples {
		if strings.TrimSpace(sample.ErrorCode) != "" {
			messages = append(messages, sample.ErrorCode)
		}
		if strings.TrimSpace(sample.ErrorMessage) != "" {
			messages = append(messages, sample.ErrorMessage)
		}
	}
	return messages
}

func classifyAccountProbeError(message string) (string, string, int) {
	lower := strings.ToLower(strings.TrimSpace(message))
	switch {
	case lower == "":
		return "", "", 0
	case strings.Contains(lower, "context deadline exceeded"):
		return "context_deadline", "context deadline exceeded", 15
	case strings.Contains(lower, "timeout"):
		return "timeout", "请求超时", 12
	case strings.Contains(lower, "unexpected eof"):
		return "unexpected_eof", "unexpected EOF", 10
	case strings.Contains(lower, "401") || strings.Contains(lower, "unauthorized") || strings.Contains(lower, "invalid api key"):
		return "auth", "认证失败/401", 15
	case strings.Contains(lower, "quota") || strings.Contains(lower, "insufficient_quota") || strings.Contains(lower, "usage_limit"):
		return "quota", "额度不足", 15
	default:
		return "upstream_error", "上游错误", 6
	}
}

func accountProbeScoreConfidence(requestCount int) int {
	switch {
	case requestCount >= 8:
		return 95
	case requestCount >= 5:
		return 90
	case requestCount >= 3:
		return 82
	case requestCount == 2:
		return 62
	case requestCount == 1:
		return 45
	default:
		return 20
	}
}

func accountProbeGrade(score int) string {
	switch {
	case score >= 90:
		return AccountProbeGradeExcellent
	case score >= 75:
		return AccountProbeGradeStable
	case score >= 60:
		return AccountProbeGradeSlow
	case score >= 40:
		return AccountProbeGradeUnstable
	default:
		return AccountProbeGradePoor
	}
}

func accountProbeGradeLabel(grade string) string {
	switch grade {
	case AccountProbeGradeExcellent:
		return "优秀"
	case AccountProbeGradeStable:
		return "稳定可用"
	case AccountProbeGradeSlow:
		return "可用偏慢"
	case AccountProbeGradeUnstable:
		return "不稳定"
	case AccountProbeGradePoor:
		return "不建议调度"
	default:
		return "未知"
	}
}
