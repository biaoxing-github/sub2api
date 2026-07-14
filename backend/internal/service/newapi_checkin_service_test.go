package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func fixedNewAPICheckinNow() time.Time {
	return time.Date(2026, 7, 8, 9, 10, 11, 0, time.Local)
}

func newTestNewAPICheckinService(t *testing.T, repo NewAPICheckinRepository, client *http.Client) *NewAPICheckinService {
	t.Helper()
	return NewNewAPICheckinService(NewAPICheckinOptions{
		Repository: repo,
		HTTPClient: client,
		Now:        fixedNewAPICheckinNow,
	})
}

func TestNewAPICheckinConfigSummaryCountsAndKeepsDisabledSiteVisible(t *testing.T) {
	repo := newMemoryNewAPICheckinRepository(t, map[string]any{
		"sites": []map[string]any{
			{
				"name":     "enabled-site",
				"enabled":  true,
				"base_url": "https://enabled.example",
				"accounts": []map[string]any{
					{"name": "alpha", "user_id": "1001", "access_key": "key-a", "ip_profile": "slot-a"},
				},
			},
			{
				"name":            "turnstile-site",
				"enabled":         false,
				"disabled_reason": "Turnstile 保护站点，仅保留余额与月度记录查询",
				"base_url":        "https://turnstile.example",
				"accounts": []map[string]any{
					{"name": "beta", "user_id": "2001", "access_key": "key-b", "ip_profile": "slot-b"},
				},
			},
		},
	})

	svc := newTestNewAPICheckinService(t, repo, nil)
	summary, err := svc.ConfigSummary(context.Background())
	require.NoError(t, err)

	require.Equal(t, 2, summary.AllSiteCount)
	require.Equal(t, 1, summary.EnabledSiteCount)
	require.Equal(t, 2, summary.AllAccountCount)
	require.Equal(t, 1, summary.EnabledAccountCount)
	require.Len(t, summary.Sites, 2)
	require.False(t, summary.Sites[1].Enabled)
	require.Equal(t, "turnstile-site", summary.Sites[1].Name)
	require.Equal(t, "Turnstile 保护站点，仅保留余额与月度记录查询", summary.Sites[1].DisabledReason)
}

// TestNewAPICheckinAPIKeysMasksGeneratedKeys 验证页面只接收带 sk- 前缀的脱敏 API Key。
func TestNewAPICheckinAPIKeysMasksGeneratedKeys(t *testing.T) {
	var seen []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String()+" "+r.Header.Get("Authorization")+" "+r.Header.Get("New-Api-User"))
		w.Header().Set("Content-Type", "application/json")
		switch r.Header.Get("New-Api-User") {
		case "1001":
			_, _ = w.Write([]byte(`{"success":true,"data":{"total":1,"items":[{"id":7,"name":"codex","key":"abcdef12345678","status":1}]}}`))
		case "1002":
			_, _ = w.Write([]byte(`{"success":true,"data":{"total":0,"items":[]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	repo := newMemoryNewAPICheckinRepository(t, map[string]any{
		"sites": []map[string]any{
			{
				"name":     "demo",
				"enabled":  true,
				"base_url": upstream.URL,
				"accounts": []map[string]any{
					{"name": "alpha", "user_id": "1001", "access_key": "access-a"},
					{"name": "beta", "user_id": "1002", "access_key": "access-b"},
				},
			},
		},
	})

	svc := newTestNewAPICheckinService(t, repo, upstream.Client())
	payload, err := svc.APIKeys(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, payload.AccountCount)
	require.Equal(t, 1, payload.AvailableCount)
	require.Equal(t, 1, payload.MissingCount)
	require.Zero(t, payload.ErrorCount)
	require.Len(t, payload.Accounts, 2)
	require.Equal(t, "ready", payload.Accounts[0].Status)
	require.Equal(t, "codex", payload.Accounts[0].APIKeys[0].Name)
	require.Equal(t, "sk-ab***5678", payload.Accounts[0].APIKeys[0].MaskedKey)
	require.Equal(t, "missing", payload.Accounts[1].Status)
	require.Empty(t, payload.Accounts[1].APIKeys)
	require.Contains(t, seen, "GET /api/token/?p=1&size=100 Bearer access-a 1001")
	require.Contains(t, seen, "GET /api/token/?p=1&size=100 Bearer access-b 1002")
}

func TestNewAPICheckinSetSiteEnabledPersistsReasonAndSummary(t *testing.T) {
	repo := newMemoryNewAPICheckinRepository(t, map[string]any{
		"sites": []map[string]any{
			{
				"name":     "demo",
				"enabled":  true,
				"base_url": "https://demo.example",
				"accounts": []map[string]any{
					{"name": "alpha", "user_id": "1001", "access_key": "key-a"},
				},
			},
		},
	})

	svc := newTestNewAPICheckinService(t, repo, nil)
	result, err := svc.SetSiteEnabled(context.Background(), "demo", false, "页面禁用签到")
	require.NoError(t, err)

	require.False(t, result.Enabled)
	require.Equal(t, "页面禁用签到", result.DisabledReason)
	require.Equal(t, 0, result.Config.EnabledSiteCount)
	require.Equal(t, 1, result.Config.AllSiteCount)

	summary, err := svc.ConfigSummary(context.Background())
	require.NoError(t, err)
	require.False(t, summary.Sites[0].Enabled)
	require.Equal(t, "页面禁用签到", summary.Sites[0].DisabledReason)
}

func TestNewAPICheckinRunFullCheckinSkipsDisabledSitesAndPreservesLatestWhenNoTasks(t *testing.T) {
	repo := newMemoryNewAPICheckinRepository(t, map[string]any{
		"sites": []map[string]any{
			{
				"name":            "disabled-only",
				"enabled":         false,
				"disabled_reason": "Turnstile 保护站点，仅保留余额与月度记录查询",
				"base_url":        "https://disabled.example",
				"accounts": []map[string]any{
					{"name": "beta", "user_id": "2001", "access_key": "key-b"},
				},
			},
		},
	})
	repo.report = NewAPICheckinReport{
		StartedAt: "2026-07-07 09:00:00",
		TaskCount: 1,
		AccountResults: []NewAPICheckinAccountResult{
			{Site: "old", UserID: "9999", Success: true, Message: "签到成功"},
		},
	}

	svc := newTestNewAPICheckinService(t, repo, nil)
	report, err := svc.RunFullCheckin(context.Background())
	require.NoError(t, err)
	require.Equal(t, 0, report.TaskCount)
	require.Empty(t, report.AccountResults)

	latest, err := svc.LastRun(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, latest.TaskCount)
	require.Len(t, latest.AccountResults, 1)
	require.Equal(t, "old", latest.AccountResults[0].Site)
}

func TestNewAPICheckinRefreshAccountBalanceUsesNewApiHeaders(t *testing.T) {
	var seen []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String()+" "+r.Header.Get("Authorization")+" "+r.Header.Get("New-Api-User"))
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/status":
			_, _ = w.Write([]byte(`{"success":true,"data":{"quota_display_type":"USD","quota_per_unit":500000}}`))
		case r.URL.Path == "/api/user/checkin" && r.Method == http.MethodPost:
			_, _ = w.Write([]byte(`{"success":true,"message":"签到成功","data":{"quota_awarded":50000,"checkin_date":"2026-07-08"}}`))
		case r.URL.Path == "/api/user/self":
			_, _ = w.Write([]byte(`{"success":true,"data":{"username":"demo-user","display_name":"Demo User","quota":250000,"used_quota":10000}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	repo := newMemoryNewAPICheckinRepository(t, map[string]any{
		"sites": []map[string]any{
			{
				"name":     "demo",
				"enabled":  true,
				"base_url": upstream.URL,
				"accounts": []map[string]any{
					{"name": "alpha", "user_id": "1001", "access_key": "key-a", "ip_profile": "slot-a"},
				},
			},
		},
	})

	svc := newTestNewAPICheckinService(t, repo, upstream.Client())
	result, err := svc.RefreshAccountBalance(context.Background(), "demo", "1001")
	require.NoError(t, err)

	require.True(t, result.OK)
	require.Equal(t, "签到成功", result.Checkin.CheckinStatus)
	require.Equal(t, int64(50000), *result.Checkin.QuotaAwarded)
	require.Equal(t, "$0.1", result.Checkin.QuotaAwardedDisplay)
	require.Equal(t, int64(250000), *result.Account.Quota)
	require.Equal(t, "$0.5", result.Account.QuotaDisplay)
	require.Contains(t, seen, "POST /api/user/checkin Bearer key-a 1001")
	require.Contains(t, seen, "GET /api/user/self Bearer key-a 1001")
}

func TestNewAPICheckinStartFullCheckinJobDetachesFromRequestContext(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/status":
			_, _ = w.Write([]byte(`{"success":true,"data":{"quota_display_type":"USD","quota_per_unit":500000}}`))
		case r.URL.Path == "/api/user/checkin" && r.Method == http.MethodPost:
			_, _ = w.Write([]byte(`{"success":true,"message":"签到成功","data":{"quota_awarded":50000,"checkin_date":"2026-07-08"}}`))
		case r.URL.Path == "/api/user/self":
			_, _ = w.Write([]byte(`{"success":true,"data":{"username":"demo-user","display_name":"Demo User","quota":250000,"used_quota":10000}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	repo := newMemoryNewAPICheckinRepository(t, map[string]any{
		"delay_between_checkins_sec": 1,
		"sites": []map[string]any{
			{
				"name":     "demo",
				"enabled":  true,
				"base_url": upstream.URL,
				"accounts": []map[string]any{
					{"name": "alpha", "user_id": "1001", "access_key": "key-a"},
					{"name": "beta", "user_id": "1002", "access_key": "key-b"},
				},
			},
		},
	})

	svc := newTestNewAPICheckinService(t, repo, upstream.Client())
	reqCtx, cancel := context.WithCancel(context.Background())
	start := svc.StartFullCheckinJob(reqCtx)
	require.True(t, start.Started)
	cancel()

	require.Eventually(t, func() bool {
		job := svc.CheckinJobStatus()
		return !job.Running && job.ExitCode != nil
	}, 3*time.Second, 10*time.Millisecond)

	job := svc.CheckinJobStatus()
	require.NotNil(t, job.ExitCode)
	require.Equal(t, 0, *job.ExitCode)
	require.Empty(t, job.Stderr)
	require.Equal(t, "全量签到完成", job.Message)
	require.Equal(t, 2, job.Report.TaskCount)
	require.Equal(t, 2, job.Report.SuccessCount)
}

func TestNewAPICheckinStartMonthlySyncJobDetachesFromRequestContext(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/user/checkin" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"success":true,"data":{"stats":{"records":[{"checkin_date":"2026-07-08","quota_awarded":50000}]}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	baseRepo := newMemoryNewAPICheckinRepository(t, map[string]any{
		"sites": []map[string]any{
			{
				"name":     "demo",
				"enabled":  true,
				"base_url": upstream.URL,
				"accounts": []map[string]any{
					{"name": "alpha", "user_id": "1001", "access_key": "key-a"},
				},
			},
		},
	})
	repo := &contextAwareMonthlyNewAPICheckinRepository{memoryNewAPICheckinRepository: baseRepo}
	svc := newTestNewAPICheckinService(t, repo, upstream.Client())

	reqCtx, cancel := context.WithCancel(context.Background())
	start := svc.StartMonthlySyncJob(reqCtx, "2026-07", "", "")
	require.True(t, start.Started)
	cancel()

	require.Eventually(t, func() bool {
		state := svc.MonthlySyncStatus()
		return !state.Running && state.EndedAt != ""
	}, 3*time.Second, 10*time.Millisecond)

	state := svc.MonthlySyncStatus()
	require.Empty(t, state.Error)
	require.Equal(t, "2026-07 同步完成", state.Message)
	require.Equal(t, 1, state.Total)
	require.Equal(t, 1, state.Processed)
	require.Len(t, repo.monthly, 1)
}

func TestNewAPICheckinDisplaySymbolFallsBackForGenericCurrencyPlaceholder(t *testing.T) {
	quota := int64(95661050)
	got := formatNewAPIDisplayAmount(&quota, NewAPICheckinSiteStatus{
		QuotaDisplayType:     "USD",
		QuotaPerUnit:         500000,
		CustomCurrencySymbol: "¤",
	})

	require.Equal(t, "$191.3221", got)
}

func TestNewAPICheckinRefreshSiteBalancesUsesMonthlyRecordForTodayAward(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/status":
			_, _ = w.Write([]byte(`{"success":true,"data":{"quota_display_type":"USD","quota_per_unit":500000}}`))
		case r.URL.Path == "/api/user/checkin" && r.Method == http.MethodPost:
			_, _ = w.Write([]byte(`{"success":false,"message":"今日已签到","data":{}}`))
		case r.URL.Path == "/api/user/self":
			_, _ = w.Write([]byte(`{"success":true,"data":{"username":"demo-user","display_name":"Demo User","quota":95661050,"used_quota":457142}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	repo := newMemoryNewAPICheckinRepository(t, map[string]any{
		"sites": []map[string]any{
			{
				"name":     "demo",
				"enabled":  true,
				"base_url": upstream.URL,
				"accounts": []map[string]any{
					{"name": "alpha", "user_id": "1001", "access_key": "key-a"},
				},
			},
		},
	})
	awarded := int64(50000)
	repo.monthly = []NewAPICheckinMonthlyRecord{
		{
			Site:                     "demo",
			UserID:                   "1001",
			AccountName:              "alpha",
			Month:                    "2026-07",
			CheckinDate:              "2026-07-08",
			QuotaAwarded:             &awarded,
			QuotaAwardedDisplay:      "¤0.1",
			QuotaAwardedDisplayValue: 0.1,
			FetchedAt:                "2026-07-08 09:00:00",
			Source:                   "manual-sync",
		},
	}

	svc := newTestNewAPICheckinService(t, repo, upstream.Client())
	result, err := svc.RefreshSiteBalances(context.Background(), "demo")
	require.NoError(t, err)

	require.Len(t, result.Accounts, 1)
	require.NotNil(t, result.Accounts[0].QuotaAwarded)
	require.Equal(t, awarded, *result.Accounts[0].QuotaAwarded)
	require.Equal(t, "$0.1", result.Accounts[0].QuotaAwardedDisplay)
	require.NotNil(t, result.Balances.Accounts[0].QuotaAwarded)
	require.Equal(t, awarded, *result.Balances.Accounts[0].QuotaAwarded)
	require.Len(t, result.History.Entries, 1)
	require.NotNil(t, result.History.Entries[0].QuotaAwarded)
	require.Equal(t, awarded, *result.History.Entries[0].QuotaAwarded)
	require.Equal(t, "$0.1", result.History.Entries[0].QuotaAwardedDisplay)
	require.Equal(t, awarded, result.LastRun.QuotaAwardedTotal)
}

func TestNewAPICheckinHistoryUsesMonthlyRecordForExistingTodayAward(t *testing.T) {
	repo := newMemoryNewAPICheckinRepository(t, map[string]any{"sites": []map[string]any{}})
	repo.history = NewAPICheckinHistoryPayload{
		Entries: []NewAPICheckinHistoryEntry{
			{
				Date:                "2026-07-08",
				RecordedAt:          "2026-07-08 09:10:00",
				Site:                "demo",
				Account:             "alpha",
				UserID:              "1001",
				CheckedInToday:      true,
				CheckinStatus:       "今日已签到",
				BalanceDisplay:      "¤191.3221",
				UsedDisplay:         "¤0.91434",
				BalanceDisplayValue: 191.3221,
				UsedDisplayValue:    0.91434,
			},
		},
	}
	awarded := int64(50000)
	repo.monthly = []NewAPICheckinMonthlyRecord{
		{
			Site:                     "demo",
			UserID:                   "1001",
			Month:                    "2026-07",
			CheckinDate:              "2026-07-08",
			QuotaAwarded:             &awarded,
			QuotaAwardedDisplay:      "¤0.1",
			QuotaAwardedDisplayValue: 0.1,
		},
	}

	svc := newTestNewAPICheckinService(t, repo, nil)
	history, err := svc.History(context.Background())
	require.NoError(t, err)

	require.Len(t, history.Entries, 1)
	require.NotNil(t, history.Entries[0].QuotaAwarded)
	require.Equal(t, awarded, *history.Entries[0].QuotaAwarded)
	require.Equal(t, "$0.1", history.Entries[0].QuotaAwardedDisplay)
	require.Equal(t, "$191.3221", history.Entries[0].BalanceDisplay)
	require.Equal(t, "$0.91434", history.Entries[0].UsedDisplay)
	require.Equal(t, 0.1, history.DailySummaries[0]["quota_awarded_display_value"])
}

// TestAggregateMonthlyCountsEachAccountDayOnce 验证重复刷新产生的同账号同日记录不会重复累计次数和奖励。
func TestAggregateMonthlyCountsEachAccountDayOnce(t *testing.T) {
	record := NewAPICheckinMonthlyRecord{
		Site:                     "demo",
		UserID:                   "1001",
		AccountName:              "alpha",
		Month:                    "2026-07",
		CheckinDate:              "2026-07-08",
		QuotaAwardedDisplayValue: 0.5,
	}

	siteMonthly, accountMonthly, siteDaily, accountDaily := aggregateMonthly([]NewAPICheckinMonthlyRecord{record, record})
	for _, rows := range [][]map[string]any{siteMonthly, accountMonthly, siteDaily, accountDaily} {
		require.Len(t, rows, 1)
		require.Equal(t, 1, rows[0]["checkin_count"])
		require.Equal(t, 0.5, rows[0]["quota_awarded_display_value"])
	}
}

type memoryNewAPICheckinRepository struct {
	config  NewAPICheckinConfig
	report  NewAPICheckinReport
	balance NewAPICheckinBalancePayload
	history NewAPICheckinHistoryPayload
	monthly []NewAPICheckinMonthlyRecord
}

func newMemoryNewAPICheckinRepository(t *testing.T, rawConfig map[string]any) *memoryNewAPICheckinRepository {
	t.Helper()
	cfg, err := decodeNewAPIConfigMap(rawConfig)
	require.NoError(t, err)
	return &memoryNewAPICheckinRepository{
		config: cfg,
		balance: NewAPICheckinBalancePayload{
			SiteStatuses: map[string]NewAPICheckinSiteStatus{},
		},
	}
}

func (r *memoryNewAPICheckinRepository) LoadConfig(context.Context) (NewAPICheckinConfig, error) {
	return r.config, nil
}

func (r *memoryNewAPICheckinRepository) SaveConfig(_ context.Context, cfg NewAPICheckinConfig) error {
	r.config = cfg
	return nil
}

func (r *memoryNewAPICheckinRepository) LoadLatestReport(context.Context) (NewAPICheckinReport, error) {
	return r.report, nil
}

func (r *memoryNewAPICheckinRepository) SaveLatestReport(_ context.Context, report NewAPICheckinReport) error {
	r.report = report
	return nil
}

func (r *memoryNewAPICheckinRepository) LoadBalanceCache(context.Context) (NewAPICheckinBalancePayload, error) {
	return r.balance, nil
}

func (r *memoryNewAPICheckinRepository) SaveBalanceCache(_ context.Context, cache NewAPICheckinBalancePayload) error {
	r.balance = cache
	return nil
}

func (r *memoryNewAPICheckinRepository) LoadHistory(context.Context) (NewAPICheckinHistoryPayload, error) {
	return r.history, nil
}

func (r *memoryNewAPICheckinRepository) SaveHistory(_ context.Context, payload NewAPICheckinHistoryPayload) error {
	r.history = payload
	return nil
}

func (r *memoryNewAPICheckinRepository) LoadMonthlyRecords(context.Context) ([]NewAPICheckinMonthlyRecord, error) {
	return r.monthly, nil
}

func (r *memoryNewAPICheckinRepository) SaveMonthlyRecords(_ context.Context, records []NewAPICheckinMonthlyRecord) error {
	r.monthly = records
	return nil
}

func (r *memoryNewAPICheckinRepository) StorageLabel() string {
	return "memory:newapi-checkin"
}

var _ NewAPICheckinRepository = (*memoryNewAPICheckinRepository)(nil)

type contextAwareMonthlyNewAPICheckinRepository struct {
	*memoryNewAPICheckinRepository
}

func (r *contextAwareMonthlyNewAPICheckinRepository) SaveMonthlyRecords(ctx context.Context, records []NewAPICheckinMonthlyRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.memoryNewAPICheckinRepository.SaveMonthlyRecords(ctx, records)
}

var _ NewAPICheckinRepository = (*contextAwareMonthlyNewAPICheckinRepository)(nil)
