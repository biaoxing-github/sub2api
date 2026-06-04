package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type accountProbeReportRepoStub struct {
	items          []AccountProbeReportItem
	total          int
	filter         AccountProbeReportFilter
	detail         *AccountProbeReportItem
	samples        []AccountProbeSample
	samplesByRunID map[int64][]AccountProbeSample
	sampleRunID    int64
	sampleRunIDs   []int64
	rankings       []AccountProbeReportItem
	rankingLimit   int
}

func (r *accountProbeReportRepoStub) CreateAccountProbeRun(ctx context.Context, run *AccountProbeResult) error {
	return nil
}

func (r *accountProbeReportRepoStub) UpdateAccountProbeRun(ctx context.Context, run *AccountProbeResult) error {
	return nil
}

func (r *accountProbeReportRepoStub) ExpireStaleAccountProbeRuns(ctx context.Context, olderThan time.Duration) error {
	return nil
}

func (r *accountProbeReportRepoStub) SaveAccountProbeSample(ctx context.Context, sample AccountProbeSample) error {
	return nil
}

func (r *accountProbeReportRepoStub) ListAccountProbeRuns(ctx context.Context, filter AccountProbeHistoryFilter) ([]AccountProbeResult, error) {
	return nil, nil
}

func (r *accountProbeReportRepoStub) GetAccountProbeRun(ctx context.Context, accountID, runID int64) (*AccountProbeResult, error) {
	return nil, ErrAccountNotFound
}

func (r *accountProbeReportRepoStub) ListAccountProbeReportRuns(ctx context.Context, filter AccountProbeReportFilter) ([]AccountProbeReportItem, int, error) {
	r.filter = filter
	return r.items, r.total, nil
}

func (r *accountProbeReportRepoStub) GetAccountProbeReportRun(ctx context.Context, runID int64) (*AccountProbeReportItem, error) {
	if r.detail == nil {
		return nil, ErrAccountNotFound
	}
	copy := *r.detail
	return &copy, nil
}

func (r *accountProbeReportRepoStub) DeleteAccountProbeReportRuns(ctx context.Context, runIDs []int64) (AccountProbeReportDeleteResult, error) {
	return AccountProbeReportDeleteResult{RequestedCount: len(runIDs), DeletedCount: len(runIDs)}, nil
}

func (r *accountProbeReportRepoStub) ListAccountProbeSamples(ctx context.Context, runID int64) ([]AccountProbeSample, error) {
	r.sampleRunID = runID
	r.sampleRunIDs = append(r.sampleRunIDs, runID)
	if r.samplesByRunID != nil {
		return r.samplesByRunID[runID], nil
	}
	return r.samples, nil
}

func (r *accountProbeReportRepoStub) ListAccountProbeRankingRuns(ctx context.Context, limit int) ([]AccountProbeReportItem, error) {
	r.rankingLimit = limit
	return r.rankings, nil
}

func TestAccountProbeServiceListReportsDecoratesAndSortsByScore(t *testing.T) {
	now := time.Now()
	repo := &accountProbeReportRepoStub{
		total: 2,
		items: []AccountProbeReportItem{
			{
				AccountName: "slow",
				AccountProbeResult: AccountProbeResult{
					ID: 1, AccountID: 10, Status: AccountProbeStatusPartial, RequestCount: 3, SuccessCount: 1, FailureCount: 2,
					Latency: AccountProbeLatencyStats{AvgMillis: 9000, P95Millis: 18000}, CreatedAt: now,
				},
			},
			{
				AccountName: "fast",
				AccountProbeResult: AccountProbeResult{
					ID: 2, AccountID: 11, Status: AccountProbeStatusSuccess, RequestCount: 3, SuccessCount: 3,
					Latency: AccountProbeLatencyStats{AvgMillis: 900, P95Millis: 1200}, CreatedAt: now,
				},
			},
		},
	}
	svc := NewAccountProbeService(nil, repo, nil, nil)

	page, err := svc.ListReports(context.Background(), AccountProbeReportFilter{Sort: "score", Order: "desc", PageSize: 200})

	require.NoError(t, err)
	require.Equal(t, 100, repo.filter.PageSize)
	require.Equal(t, 1, repo.filter.Page)
	require.Equal(t, 2, page.Total)
	require.Len(t, page.Items, 2)
	require.Equal(t, "fast", page.Items[0].AccountName)
	require.Greater(t, page.Items[0].Score, page.Items[1].Score)
	require.Greater(t, page.Items[0].SuccessRate, 0.99)
	require.Greater(t, page.Summary.AverageScore, 0.0)
}

func TestAccountProbeServiceListReportsUsesBazaarLinkEvidenceScore(t *testing.T) {
	now := time.Now()
	repo := &accountProbeReportRepoStub{
		total: 1,
		items: []AccountProbeReportItem{{
			AccountName: "bazaar",
			AccountProbeResult: AccountProbeResult{
				ID: 42, AccountID: 12, Profile: AccountProbeProfileModelValidation, ProbeSource: AccountProbeSourceBazaarLinkAPI,
				Status: AccountProbeStatusSuccess, RequestMode: string(BazaarLinkProbeModeFull), Model: "gpt-5.5",
				RequestCount: 1, SuccessCount: 1, CreatedAt: now,
			},
		}},
		samplesByRunID: map[int64][]AccountProbeSample{
			42: {{
				RunID: 42, Type: AccountProbeSourceBazaarLinkAPI, Status: AccountProbeSampleSuccess, Model: "gpt-5.5",
				ValidationEvidence: []AccountProbeValidationEvidence{{
					Key:      "bazaarlink_identity",
					Label:    "BazaarLink 模型身份",
					Expected: "gpt-5.5",
					Observed: "status=match; confidence=0.98; family=openai",
					Passed:   true,
					Score:    91,
					MaxScore: 100,
				}},
			}},
		},
	}
	svc := NewAccountProbeService(nil, repo, nil, nil)

	page, err := svc.ListReports(context.Background(), AccountProbeReportFilter{Sort: "score", Order: "desc"})

	require.NoError(t, err)
	require.Equal(t, []int64{42}, repo.sampleRunIDs)
	require.Len(t, page.Items, 1)
	require.Equal(t, 91, page.Items[0].Score)
	require.Contains(t, page.Items[0].ScoreItems[0], "91/100")
}

func TestAccountProbeServiceListReportsUsesLegacyBazaarLinkQuickCandidateScore(t *testing.T) {
	now := time.Now()
	repo := &accountProbeReportRepoStub{
		total: 1,
		items: []AccountProbeReportItem{{
			AccountName: "qingflow",
			AccountProbeResult: AccountProbeResult{
				ID: 229, AccountID: 423, Profile: AccountProbeProfileModelValidation, ProbeSource: AccountProbeSourceBazaarLinkAPI,
				Status: AccountProbeStatusSuccess, RequestMode: string(BazaarLinkProbeModeQuick), Model: "gpt-5.5",
				RequestCount: 1, SuccessCount: 1, CreatedAt: now,
			},
		}},
		samplesByRunID: map[int64][]AccountProbeSample{
			229: {{
				RunID: 229, Type: AccountProbeSourceBazaarLinkAPI, Label: "BazaarLink 快速验证", Status: AccountProbeSampleSuccess, Model: "gpt-5.5",
				ResponseBody: `{"runId":"run_229","status":"completed","identityAssessment":{"status":"match","confidence":0.98,"claimedModel":"gpt-5.5","predictedFamily":"openai","v3":{"candidates":[{"displayName":"GPT-5.3 Codex","modelId":"openai/gpt-5.3-codex","family":"openai","score":0.9893329875983731},{"displayName":"GPT-5.5","modelId":"openai/gpt-5.5","family":"openai","score":0.9852066599830172},{"displayName":"GPT-5.4 Mini","modelId":"openai/gpt-5.4-mini","family":"openai","score":0.9052747274960748}]}},"items":`,
				ValidationEvidence: []AccountProbeValidationEvidence{{
					Key:      "bazaarlink_identity",
					Label:    "BazaarLink 模型身份",
					Expected: "gpt-5.5",
					Observed: "status=match; confidence=0.98; family=openai; v3f=gpt-5.5",
					Passed:   true,
					Score:    0,
					MaxScore: 100,
				}},
			}},
		},
	}
	svc := NewAccountProbeService(nil, repo, nil, nil)

	page, err := svc.ListReports(context.Background(), AccountProbeReportFilter{Sort: "score", Order: "desc"})

	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.Equal(t, 99, page.Items[0].Score)
	require.NotNil(t, page.Items[0].DisplayScore)
	require.InDelta(t, 98.52066599830172, *page.Items[0].DisplayScore, 0.000001)
	require.NotEqual(t, AccountProbeGradeUnavailable, page.Items[0].Grade)
	require.NotContains(t, page.Items[0].GradeLabel, "不可检测")
}

func TestAccountProbeServiceListReportsUsesSelfValidationEvidenceScore(t *testing.T) {
	now := time.Now()
	repo := &accountProbeReportRepoStub{
		total: 1,
		items: []AccountProbeReportItem{{
			AccountName: "aisz",
			AccountProbeResult: AccountProbeResult{
				ID: 257, AccountID: 408, Profile: AccountProbeProfileModelValidation, ProbeSource: AccountProbeSourceSelfValidation,
				Status: AccountProbeStatusPartial, RequestMode: AccountProbeRequestModeNonStream, Model: "gpt-5.5",
				RequestCount: 19, SuccessCount: 31, FailureCount: 7, CreatedAt: now,
			},
		}},
		samplesByRunID: map[int64][]AccountProbeSample{
			257: {
				{
					RunID: 257, Type: "model_validation", Status: AccountProbeSampleSuccess, Model: "gpt-5.5",
					ValidationEvidence: []AccountProbeValidationEvidence{{
						Key:      "json_arithmetic",
						Label:    "JSON 算术",
						Expected: `{"sum":83}`,
						Observed: `{"sum":83}`,
						Passed:   true,
						Score:    10,
						MaxScore: 10,
					}},
				},
				{
					RunID: 257, Type: "model_validation", Status: AccountProbeSampleFailed, Model: "gpt-5.5",
					ValidationEvidence: []AccountProbeValidationEvidence{{
						Key:      "responses_non_stream",
						Label:    "Responses 非流式",
						Expected: "返回目标模型",
						Observed: "Service temporarily unavailable",
						Passed:   false,
						Score:    0,
						MaxScore: 10,
					}},
				},
			},
		},
	}
	svc := NewAccountProbeService(nil, repo, nil, nil)

	page, err := svc.ListReports(context.Background(), AccountProbeReportFilter{Sort: "created_at", Order: "desc"})

	require.NoError(t, err)
	require.Equal(t, []int64{257}, repo.sampleRunIDs)
	require.Len(t, page.Items, 1)
	require.Equal(t, 50, page.Items[0].Score)
	require.Contains(t, page.Items[0].ScoreItems[0], "1/2")
	require.Empty(t, page.Items[0].Samples)
}

func TestAccountProbeServiceGetReportLoadsSamplesAndScoreBreakdown(t *testing.T) {
	repo := &accountProbeReportRepoStub{
		detail: &AccountProbeReportItem{
			AccountName: "encore",
			AccountProbeResult: AccountProbeResult{
				ID: 9, AccountID: 12, Status: AccountProbeStatusFailed, RequestCount: 1, FailureCount: 1,
				ErrorMessage: "unexpected EOF",
			},
		},
		samples: []AccountProbeSample{{RunID: 9, Status: AccountProbeSampleFailed, ErrorMessage: "unexpected EOF"}},
	}
	svc := NewAccountProbeService(nil, repo, nil, nil)

	item, err := svc.GetReport(context.Background(), 9)

	require.NoError(t, err)
	require.Equal(t, int64(9), repo.sampleRunID)
	require.Len(t, item.Samples, 1)
	require.NotZero(t, item.Score)
	require.Contains(t, item.PenaltyItems, "unexpected EOF，-10")
}

func TestAccountProbeServiceListRankingDecoratesAggregateScores(t *testing.T) {
	now := time.Now()
	repo := &accountProbeReportRepoStub{
		rankings: []AccountProbeReportItem{
			{
				AccountName: "stable-upstream",
				AccountProbeResult: AccountProbeResult{
					ID: 101, AccountID: 12, Status: AccountProbeStatusSuccess, RequestCount: 3, SuccessCount: 3,
					Latency: AccountProbeLatencyStats{AvgMillis: 1300, P95Millis: 1700}, TotalTokens: 3400, CreatedAt: now.Add(-2 * time.Hour),
				},
			},
			{
				AccountName: "stable-upstream",
				AccountProbeResult: AccountProbeResult{
					ID: 201, AccountID: 12, Status: AccountProbeStatusSuccess, RequestCount: 3, SuccessCount: 3,
					Latency: AccountProbeLatencyStats{AvgMillis: 1100, P95Millis: 1500}, TotalTokens: 3200, CreatedAt: now.Add(-1 * time.Hour),
				},
			},
			{
				AccountName: "stable-upstream",
				AccountProbeResult: AccountProbeResult{
					ID: 301, AccountID: 12, Status: AccountProbeStatusSuccess, RequestCount: 3, SuccessCount: 3,
					Latency: AccountProbeLatencyStats{AvgMillis: 850, P95Millis: 1200}, TotalTokens: 2400, CreatedAt: now,
				},
			},
		},
	}
	svc := NewAccountProbeService(nil, repo, nil, nil)

	items, err := svc.ListRanking(context.Background(), 25)

	require.NoError(t, err)
	require.Equal(t, accountProbeRankingHistorySize, repo.rankingLimit)
	require.Len(t, items, 1)
	require.Equal(t, int64(12), items[0].AccountID)
	require.Equal(t, "stable-upstream", items[0].AccountName)
	require.Equal(t, 3, items[0].RunCount)
	require.GreaterOrEqual(t, items[0].AverageScore, 90.0)
	require.Equal(t, int64(301), items[0].LatestRunID)
	require.GreaterOrEqual(t, items[0].LatestScore, 90)
	require.Equal(t, AccountProbeGradeExcellent, items[0].Grade)
	require.Len(t, items[0].ScoreHistory, 3)
}
