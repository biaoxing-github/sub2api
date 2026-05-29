package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type accountProbeReportRepoStub struct {
	items       []AccountProbeReportItem
	total       int
	filter      AccountProbeReportFilter
	detail      *AccountProbeReportItem
	samples     []AccountProbeSample
	sampleRunID int64
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
	return r.samples, nil
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
