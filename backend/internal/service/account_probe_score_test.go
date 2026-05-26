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
