package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestScoreAccountProbeRunRewardsFastSuccessfulStreamRun(t *testing.T) {
	firstToken := 640
	run := AccountProbeResult{
		Status:           AccountProbeStatusSuccess,
		RequestCount:     3,
		SuccessCount:     3,
		FailureCount:     0,
		TotalTokens:      420,
		FirstTokenMillis: &firstToken,
		Latency: AccountProbeLatencyStats{
			AvgMillis: 1200,
			P95Millis: 1800,
			MaxMillis: 1900,
		},
	}

	score := ScoreAccountProbeRun(run)

	require.GreaterOrEqual(t, score.Score, 90)
	require.Equal(t, AccountProbeGradeExcellent, score.Grade)
	require.GreaterOrEqual(t, score.Confidence, 80)
	require.NotEmpty(t, score.ScoreItems)
	require.Empty(t, score.PenaltyItems)
	require.Contains(t, strings.Join(score.ScoreItems, " "), "成功率 100%")
}

func TestScoreAccountProbeRunExplainsLatencyAndFirstTokenPenalties(t *testing.T) {
	firstToken := 6500
	run := AccountProbeResult{
		Status:           AccountProbeStatusSuccess,
		RequestCount:     3,
		SuccessCount:     3,
		TotalTokens:      600,
		FirstTokenMillis: &firstToken,
		Latency: AccountProbeLatencyStats{
			AvgMillis: 8500,
			P95Millis: 18000,
			MaxMillis: 19000,
		},
	}

	score := ScoreAccountProbeRun(run)

	require.Less(t, score.Score, 80)
	require.Contains(t, strings.Join(score.PenaltyItems, " "), "P95")
	require.Contains(t, strings.Join(score.PenaltyItems, " "), "首 Token")
}

func TestScoreAccountProbeRunDoesNotPenalizeCurrentStandardTokenVolume(t *testing.T) {
	firstToken := 900
	run := AccountProbeResult{
		Status:           AccountProbeStatusSuccess,
		RequestCount:     9,
		SuccessCount:     9,
		FailureCount:     0,
		TotalTokens:      24036,
		FirstTokenMillis: &firstToken,
		Latency: AccountProbeLatencyStats{
			AvgMillis: 1900,
			P95Millis: 2400,
			MaxMillis: 2600,
		},
	}

	score := ScoreAccountProbeRun(run)

	require.NotContains(t, strings.Join(score.PenaltyItems, " "), "Token 消耗")
	require.Contains(t, strings.Join(score.ScoreItems, " "), "Token 消耗 24036，+5")
	require.GreaterOrEqual(t, score.Score, 90)
}

func TestScoreAccountProbeRunStillPenalizesRunawayTokenVolume(t *testing.T) {
	firstToken := 900
	run := AccountProbeResult{
		Status:           AccountProbeStatusSuccess,
		RequestCount:     9,
		SuccessCount:     9,
		FailureCount:     0,
		TotalTokens:      100000,
		FirstTokenMillis: &firstToken,
		Latency: AccountProbeLatencyStats{
			AvgMillis: 1900,
			P95Millis: 2400,
			MaxMillis: 2600,
		},
	}

	score := ScoreAccountProbeRun(run)

	require.Contains(t, strings.Join(score.PenaltyItems, " "), "Token 消耗 100000，-3")
	require.NotContains(t, strings.Join(score.ScoreItems, " "), "Token 消耗 100000，+5")
}

func TestScoreAccountProbeRunPenalizesFailuresAndTimeouts(t *testing.T) {
	run := AccountProbeResult{
		Status:       AccountProbeStatusPartial,
		RequestCount: 3,
		SuccessCount: 1,
		FailureCount: 2,
		ErrorMessage: "context deadline exceeded",
		TotalTokens:  90,
		Latency:      AccountProbeLatencyStats{AvgMillis: 5000, P95Millis: 15000, MaxMillis: 15000},
		Samples:      []AccountProbeSample{{Status: AccountProbeSampleFailed, ErrorMessage: "unexpected EOF"}},
	}

	score := ScoreAccountProbeRun(run)

	require.Less(t, score.Score, 60)
	require.Contains(t, strings.Join(score.PenaltyItems, " "), "成功率 33%")
	require.Contains(t, strings.Join(score.PenaltyItems, " "), "context deadline exceeded")
	require.Contains(t, strings.Join(score.PenaltyItems, " "), "unexpected EOF")
}

func TestScoreAccountProbeRunMarksLowConfidenceForSingleSample(t *testing.T) {
	run := AccountProbeResult{
		Status:       AccountProbeStatusSuccess,
		RequestCount: 1,
		SuccessCount: 1,
		Latency:      AccountProbeLatencyStats{AvgMillis: 900, P95Millis: 900},
	}

	score := ScoreAccountProbeRun(run)

	require.Less(t, score.Confidence, 70)
	require.Contains(t, strings.Join(score.PenaltyItems, " "), "样本数不足")
}

func TestScoreAccountModelValidationUsesAuthenticityPassRate(t *testing.T) {
	run := AccountProbeResult{
		Profile:      AccountProbeProfileModelValidation,
		Status:       AccountProbeStatusPartial,
		Model:        "gpt-5.5",
		RequestCount: 4,
		SuccessCount: 3,
		FailureCount: 1,
		Samples: []AccountProbeSample{
			{ValidationEvidence: []AccountProbeValidationEvidence{{Key: "exact_uppercase", Passed: true, Score: 10, MaxScore: 10}}},
			{ValidationEvidence: []AccountProbeValidationEvidence{{Key: "json_arithmetic", Passed: true, Score: 10, MaxScore: 10}}},
			{ValidationEvidence: []AccountProbeValidationEvidence{{Key: "code_transform", Passed: true, Score: 10, MaxScore: 10}}},
			{ValidationEvidence: []AccountProbeValidationEvidence{{Key: "three_line_format", Passed: false, Score: 0, MaxScore: 10}}},
		},
	}

	score := ScoreAccountProbeRun(run)

	require.Equal(t, 75, score.Score)
	require.Equal(t, AccountProbeGradeUncertain, score.Grade)
	require.Equal(t, "不确定", score.Label)
	require.Equal(t, 82, score.Confidence)
	require.Contains(t, strings.Join(score.ScoreItems, " "), "强验证通过 3/4")
	require.Contains(t, strings.Join(score.PenaltyItems, " "), "模型验证未通过 1/4")
	require.NotContains(t, strings.Join(score.ScoreItems, " "), "平均耗时")
	require.NotContains(t, strings.Join(score.ScoreItems, " "), "Token 消耗")
}

func TestScoreAccountModelValidationDoesNotMarkOtherModelsGenuineGPT55(t *testing.T) {
	run := AccountProbeResult{
		Profile:      AccountProbeProfileModelValidation,
		Status:       AccountProbeStatusSuccess,
		Model:        "gpt-5.4-mini",
		RequestCount: 4,
		SuccessCount: 4,
		Samples: []AccountProbeSample{
			{ValidationEvidence: []AccountProbeValidationEvidence{{Key: "exact_uppercase", Passed: true, Score: 10, MaxScore: 10}}},
			{ValidationEvidence: []AccountProbeValidationEvidence{{Key: "json_arithmetic", Passed: true, Score: 10, MaxScore: 10}}},
			{ValidationEvidence: []AccountProbeValidationEvidence{{Key: "code_transform", Passed: true, Score: 10, MaxScore: 10}}},
			{ValidationEvidence: []AccountProbeValidationEvidence{{Key: "three_line_format", Passed: true, Score: 10, MaxScore: 10}}},
		},
	}

	score := ScoreAccountProbeRun(run)

	require.Equal(t, 100, score.Score)
	require.Equal(t, AccountProbeGradeUnavailable, score.Grade)
	require.Equal(t, "不可检测", score.Label)
	require.Contains(t, strings.Join(score.PenaltyItems, " "), "仅支持 gpt-5.5/gpt-5.4")
}

func TestScoreAccountModelValidationSupportsGPT54SnapshotsAndRejectsMiniVariants(t *testing.T) {
	baseSamples := []AccountProbeSample{
		{ValidationEvidence: []AccountProbeValidationEvidence{{Key: "responses_basic", Passed: true, Score: 20, MaxScore: 20}}},
		{ValidationEvidence: []AccountProbeValidationEvidence{{Key: "behavior_probe", Passed: true, Score: 35, MaxScore: 35}}},
		{ValidationEvidence: []AccountProbeValidationEvidence{{Key: "long_context", Passed: true, Score: 15, MaxScore: 15}}},
		{ValidationEvidence: []AccountProbeValidationEvidence{{Key: "stability", Passed: true, Score: 15, MaxScore: 15}}},
		{ValidationEvidence: []AccountProbeValidationEvidence{{Key: "cross_model", Passed: true, Score: 10, MaxScore: 10}}},
	}
	snapshot := AccountProbeResult{
		Profile:      AccountProbeProfileModelValidation,
		Status:       AccountProbeStatusSuccess,
		Model:        "gpt-5.4-2026-06-02",
		RequestCount: len(baseSamples),
		SuccessCount: len(baseSamples),
		Samples:      baseSamples,
	}

	snapshotScore := ScoreAccountProbeRun(snapshot)

	require.Equal(t, 100, snapshotScore.Score)
	require.Equal(t, AccountProbeGradeHighConfidence, snapshotScore.Grade)
	require.NotContains(t, strings.Join(snapshotScore.PenaltyItems, " "), "验证目标")

	mini := snapshot
	mini.Model = "gpt-5.4-mini"
	miniScore := ScoreAccountProbeRun(mini)

	require.Equal(t, AccountProbeGradeUnavailable, miniScore.Grade)
	require.Contains(t, strings.Join(miniScore.PenaltyItems, " "), "仅支持 gpt-5.5/gpt-5.4")
}

func TestScoreAccountModelValidationMarksResponseModelMismatchSuspicious(t *testing.T) {
	run := AccountProbeResult{
		Profile:      AccountProbeProfileModelValidation,
		Status:       AccountProbeStatusPartial,
		Model:        "gpt-5.5",
		RequestCount: 2,
		SuccessCount: 1,
		FailureCount: 1,
		Samples: []AccountProbeSample{
			{ValidationEvidence: []AccountProbeValidationEvidence{{
				Key:           "responses_basic",
				Category:      "responses_basic",
				ExpectedModel: "gpt-5.5",
				ResponseModel: "gpt-5.5-mini",
				Passed:        false,
				Score:         6,
				MaxScore:      20,
				Message:       "上游返回模型 gpt-5.5-mini，与请求模型 gpt-5.5 不一致",
			}}},
			{ValidationEvidence: []AccountProbeValidationEvidence{{Key: "behavior_probe", Passed: true, Score: 35, MaxScore: 35}}},
		},
	}

	score := ScoreAccountProbeRun(run)

	require.Equal(t, AccountProbeGradeSuspicious, score.Grade)
	require.Equal(t, "疑似替换或降级", score.Label)
	require.Contains(t, strings.Join(score.PenaltyItems, " "), "响应模型字段与请求模型不一致")
}
