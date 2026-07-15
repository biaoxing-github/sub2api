package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type newAPICheckinAccountRepoStub struct {
	accounts []Account
	updated  *Account
}

func (r *newAPICheckinAccountRepoStub) ListByPlatform(_ context.Context, platform string) ([]Account, error) {
	return r.accounts, nil
}
func (r *newAPICheckinAccountRepoStub) GetByID(_ context.Context, id int64) (*Account, error) {
	for index := range r.accounts {
		if r.accounts[index].ID == id {
			return &r.accounts[index], nil
		}
	}
	return nil, fmt.Errorf("account not found")
}
func (r *newAPICheckinAccountRepoStub) Update(_ context.Context, account *Account) error {
	r.updated = account
	return nil
}

// TestNewAPICheckinAttachMainAccountReferences 验证 URL 规范化、引用高亮与可关联账号标记。
func TestNewAPICheckinAttachMainAccountReferences(t *testing.T) {
	svc := NewNewAPICheckinService(NewAPICheckinOptions{})
	summary := NewAPICheckinAccountAPIKeySummary{APIKeys: []NewAPICheckinAPIKeySummary{{ID: 7, fullKey: "sk-linked"}}}
	svc.attachMainAccountReferences(&summary, NewAPICheckinSite{BaseURL: "HTTPS://Demo.Example/"}, []Account{
		{ID: 11, Name: "主账号", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://demo.example/v1", "api_keys": []any{"sk-linked"}}},
		{ID: 12, Name: "其他站点", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://other.example", "api_keys": []any{"sk-linked"}}},
	})
	require.Len(t, summary.APIKeys[0].TargetAccounts, 1)
	require.Equal(t, int64(11), summary.APIKeys[0].TargetAccounts[0].ID)
	require.True(t, summary.APIKeys[0].TargetAccounts[0].Referenced)
	require.Len(t, summary.APIKeys[0].ReferencedAccounts, 1)
}

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

// TestNewAPICheckinSetAccountDisplayName 验证管理员可手工保存用户名或邮箱标识。
func TestNewAPICheckinSetAccountDisplayName(t *testing.T) {
	repo := newMemoryNewAPICheckinRepository(t, map[string]any{
		"sites": []map[string]any{{
			"name":     "sub2-demo",
			"provider": "sub2api",
			"enabled":  true,
			"base_url": "https://sub2.example",
			"accounts": []map[string]any{{"name": "primary", "user_id": "primary", "access_key": "sk-secret"}},
		}},
	})
	svc := newTestNewAPICheckinService(t, repo, nil)

	summary, err := svc.SetAccountDisplayName(context.Background(), "sub2-demo", "primary", "owner@example.com")
	require.NoError(t, err)
	require.Equal(t, "owner@example.com", summary.Sites[0].Accounts[0].DisplayName)
	require.Equal(t, "sk-secret", summary.Sites[0].Accounts[0].AccessKey)
	require.Equal(t, "owner@example.com", repo.config.Sites[0].Accounts[0].DisplayName)
}

// TestNewAPICheckinAPIKeysMasksGeneratedKeys 验证页面只接收带 sk- 前缀的脱敏 API Key 和已知分组。
func TestNewAPICheckinAPIKeysMasksGeneratedKeys(t *testing.T) {
	seen := make(chan string, 6)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- r.Method + " " + r.URL.String() + " " + r.Header.Get("Authorization") + " " + r.Header.Get("New-Api-User")
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/token/":
			if r.Header.Get("New-Api-User") == "1001" {
				_, _ = w.Write([]byte(`{"success":true,"data":{"total":1,"items":[{"id":7,"name":"codex","key":"abcdef12345678","status":1,"group":"codex-team"}]}}`))
				return
			}
			_, _ = w.Write([]byte(`{"success":true,"data":{"total":0,"items":[]}}`))
		case "/api/user/self":
			_, _ = w.Write([]byte(`{"success":true,"data":{"group":"default"}}`))
		case "/api/user/available_groups":
			_, _ = w.Write([]byte(`{"success":true,"data":["default","codex-team"]}`))
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
	close(seen)
	requests := make([]string, 0, 2)
	for request := range seen {
		requests = append(requests, request)
	}
	require.Equal(t, 2, payload.AccountCount)
	require.Equal(t, 1, payload.AvailableCount)
	require.Equal(t, 1, payload.MissingCount)
	require.Zero(t, payload.ErrorCount)
	require.Len(t, payload.Accounts, 2)
	require.Equal(t, "ready", payload.Accounts[0].Status)
	require.Equal(t, "codex", payload.Accounts[0].APIKeys[0].Name)
	require.Equal(t, "sk-ab***5678", payload.Accounts[0].APIKeys[0].MaskedKey)
	require.Equal(t, int64(7), payload.Accounts[0].APIKeys[0].ID)
	require.Equal(t, "codex-team", payload.Accounts[0].APIKeys[0].Group)
	require.Equal(t, "ready", payload.Accounts[0].GroupStatus)
	require.Equal(t, []string{"codex-team", "default"}, payload.Accounts[0].AvailableGroups)
	require.Equal(t, "missing", payload.Accounts[1].Status)
	require.Empty(t, payload.Accounts[1].APIKeys)
	require.Contains(t, requests, "GET /api/token/?p=1&size=100 Bearer access-a 1001")
	require.Contains(t, requests, "GET /api/token/?p=1&size=100 Bearer access-b 1002")
}

// TestNewAPICheckinRevealAndUpdateAPIKeyGroup 验证完整 Key 按需返回，分组更新保留原 token 字段。
func TestNewAPICheckinRevealAndUpdateAPIKeyGroup(t *testing.T) {
	var updated map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPut {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&updated))
			_, _ = w.Write([]byte(`{"success":true,"message":"updated"}`))
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"data":{"items":[{"id":7,"name":"codex","key":"abcdef12345678","status":1,"group":"default","remain_quota":12345,"unlimited_quota":false,"model_limits_enabled":true,"model_limits":"gpt-5"}]}}`))
	}))
	defer upstream.Close()

	repo := newMemoryNewAPICheckinRepository(t, map[string]any{"sites": []map[string]any{{
		"name": "demo", "enabled": true, "base_url": upstream.URL,
		"accounts": []map[string]any{{"name": "alpha", "user_id": "1001", "access_key": "access-a"}},
	}}})
	svc := newTestNewAPICheckinService(t, repo, upstream.Client())

	revealed, err := svc.RevealAPIKey(context.Background(), "demo", "1001", 7)
	require.NoError(t, err)
	require.Equal(t, "sk-abcdef12345678", revealed.Key)
	require.Equal(t, "sk-ab***5678", revealed.MaskedKey)

	result, err := svc.UpdateAPIKeyGroup(context.Background(), "demo", "1001", 7, "codex-team", 0)
	require.NoError(t, err)
	require.Equal(t, "codex-team", result.Group)
	require.Equal(t, "codex-team", updated["group"])
	require.Equal(t, float64(12345), updated["remain_quota"])
	require.Equal(t, true, updated["model_limits_enabled"])
	require.Equal(t, "gpt-5", updated["model_limits"])
}

// TestNewAPICheckinSub2LoginCredentialsAndGroupManagement 验证 sub2api 登录凭据保存后可读取和修改 Key 分组。
func TestNewAPICheckinSub2LoginCredentialsAndGroupManagement(t *testing.T) {
	var updated map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/auth/login":
			var login map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&login))
			require.Equal(t, "owner@example.com", login["email"])
			require.Equal(t, "fixture-password", login["password"])
			_, _ = w.Write([]byte(`{"code":0,"data":{"access_token":"jwt-token","token_type":"Bearer"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/keys":
			require.Equal(t, "Bearer jwt-token", r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(`{"code":0,"data":{"items":[{"id":1067,"name":"codex","key":"90f918dd12345684cd","group_id":22,"group":{"id":22,"name":"尝鲜套餐"}}],"total":1}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/groups/available":
			require.Equal(t, "Bearer jwt-token", r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(`{"code":0,"data":[{"id":22,"name":"尝鲜套餐"},{"id":26,"name":"codex--pro"}]}`))
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/keys/1067":
			require.Equal(t, "Bearer jwt-token", r.Header.Get("Authorization"))
			require.NoError(t, json.NewDecoder(r.Body).Decode(&updated))
			_, _ = w.Write([]byte(`{"code":0,"data":{"id":1067},"message":"updated"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	repo := newMemoryNewAPICheckinRepository(t, map[string]any{"sites": []map[string]any{{
		"name": "sub2-demo", "provider": "sub2api", "enabled": true, "base_url": upstream.URL,
		"accounts": []map[string]any{{"name": "primary", "user_id": "primary", "access_key": "sk-gateway"}},
	}}})
	svc := newTestNewAPICheckinService(t, repo, upstream.Client())

	saved, err := svc.SetAccountLoginCredentials(context.Background(), "sub2-demo", "primary", "owner@example.com", "fixture-password")
	require.NoError(t, err)
	require.True(t, saved.LoginOK)
	require.Equal(t, 1, saved.APIKeyCount)
	require.Equal(t, 2, saved.GroupCount)
	require.NotNil(t, saved.Config)
	require.Equal(t, "owner@example.com", saved.Config.Sites[0].Accounts[0].LoginUsername)
	require.True(t, saved.Config.Sites[0].Accounts[0].HasLoginPassword)
	encoded, err := json.Marshal(saved.Config)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "fixture-password")
	require.NotContains(t, string(encoded), `"login_password":`)

	payload, err := svc.APIKeys(context.Background())
	require.NoError(t, err)
	require.Equal(t, "ready", payload.Accounts[0].Status)
	require.Equal(t, int64(1067), payload.Accounts[0].APIKeys[0].ID)
	require.Equal(t, int64(22), payload.Accounts[0].APIKeys[0].GroupID)
	require.Equal(t, "尝鲜套餐", payload.Accounts[0].APIKeys[0].Group)
	require.Equal(t, "sk-90***84cd", payload.Accounts[0].APIKeys[0].MaskedKey)
	require.Equal(t, []NewAPICheckinGroupOption{{ID: 22, Name: "尝鲜套餐"}, {ID: 26, Name: "codex--pro"}}, payload.Accounts[0].AvailableGroupOptions)

	revealed, err := svc.RevealAPIKey(context.Background(), "sub2-demo", "primary", 1067)
	require.NoError(t, err)
	require.Equal(t, "sk-90f918dd12345684cd", revealed.Key)

	groupResult, err := svc.UpdateAPIKeyGroup(context.Background(), "sub2-demo", "primary", 1067, "codex--pro", 26)
	require.NoError(t, err)
	require.Equal(t, int64(26), groupResult.GroupID)
	require.Equal(t, "codex--pro", groupResult.Group)
	require.Equal(t, map[string]any{"group_id": float64(26)}, updated)
}

// TestNewAPICheckinRefreshSub2AccountReadsUsageAndModelsOnly 验证 sub2 数据源只调用只读接口。
func TestNewAPICheckinRefreshSub2AccountReadsUsageAndModelsOnly(t *testing.T) {
	var seen []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String()+" "+r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/usage":
			_, _ = w.Write([]byte(`{"mode":"unrestricted","isValid":true,"planName":"尝鲜套餐","remaining":25,"unit":"USD","subscription":{"daily_limit_usd":25,"daily_usage_usd":0,"expires_at":"2026-08-12T13:55:02+08:00"},"usage":{"today":{"requests":0,"cost":0},"total":{"requests":6,"total_tokens":7648,"cost":0.0199075,"actual_cost":0.00117365}},"daily_usage":[{"date":"2026-07-12","requests":3,"cost":0.0193875}],"model_stats":[{"model":"gpt-5.6-terra","requests":6,"cost":0.0199075}]}`))
		case "/v1/models":
			_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"gpt-5.6-terra"},{"id":"gpt-5.6-sol"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	repo := newMemoryNewAPICheckinRepository(t, map[string]any{
		"sites": []map[string]any{
			{
				"name":     "sub2-demo",
				"provider": "sub2api",
				"enabled":  true,
				"base_url": upstream.URL,
				"accounts": []map[string]any{
					{"name": "primary", "user_id": "primary", "access_key": "sk-test-secret"},
				},
			},
		},
	})

	svc := newTestNewAPICheckinService(t, repo, upstream.Client())
	result, err := svc.RefreshAccountBalance(context.Background(), "sub2-demo", "primary")
	require.NoError(t, err)
	require.True(t, result.OK)
	require.Equal(t, "sub2api", result.Account.Provider)
	require.Equal(t, "$25", result.Account.QuotaDisplay)
	require.Equal(t, "$0.0199075", result.Account.UsedQuotaDisplay)
	require.Equal(t, "尝鲜套餐", result.Account.ProviderData["plan_name"])
	require.Equal(t, float64(2), result.Account.ProviderData["model_count"])
	require.Equal(t, []any{"gpt-5.6-terra", "gpt-5.6-sol"}, result.Account.ProviderData["models"])
	require.Equal(t, []string{
		"GET /v1/usage?days=30 Bearer sk-test-secret",
		"GET /v1/models Bearer sk-test-secret",
	}, seen)
	require.Empty(t, result.History.Entries)
	require.Empty(t, result.LastRun.AccountResults)
	require.Empty(t, result.Monthly.Records)
}

// TestNewAPICheckinSub2ProviderNeverRunsCheckin 验证 sub2 数据源不会进入签到或 Key 枚举流程。
func TestNewAPICheckinSub2ProviderNeverRunsCheckin(t *testing.T) {
	repo := newMemoryNewAPICheckinRepository(t, map[string]any{
		"sites": []map[string]any{
			{
				"name":     "sub2-demo",
				"provider": "sub2api",
				"enabled":  true,
				"base_url": "https://sub2.example",
				"accounts": []map[string]any{
					{"name": "primary", "user_id": "primary", "access_key": "sk-test-secret"},
				},
			},
		},
	})
	svc := newTestNewAPICheckinService(t, repo, nil)

	report, err := svc.RunFullCheckin(context.Background())
	require.NoError(t, err)
	require.Zero(t, report.TaskCount)
	_, err = svc.RunSingleCheckin(context.Background(), "sub2-demo", "primary")
	require.ErrorContains(t, err, "只读数据源")

	apiKeys, err := svc.APIKeys(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, apiKeys.AccountCount)
	require.Equal(t, 1, apiKeys.UnsupportedCount)
	require.Zero(t, apiKeys.ErrorCount)
	require.Equal(t, "unsupported", apiKeys.Accounts[0].Status)
	require.Equal(t, "unsupported", apiKeys.Accounts[0].GroupStatus)

	monthlyTasks := buildNewAPIMonthlyTasks(repo.config, "sub2-demo", "")
	require.Empty(t, monthlyTasks)
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
