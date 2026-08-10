package repository

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type newAPICheckinProviderDataJSONArg struct{}

type newAPICheckinCacheSummaryJSONArg struct{}

func (newAPICheckinCacheSummaryJSONArg) Match(value driver.Value) bool {
	raw, ok := value.(string)
	if !ok || raw == "" || strings.Contains(raw, "sk-secret") {
		return false
	}
	var summary service.NewAPICheckinAccountAPIKeySummary
	return json.Unmarshal([]byte(raw), &summary) == nil && len(summary.APIKeys) == 1 && summary.APIKeys[0].MaskedKey == "sk-se***cret"
}

type newAPICheckinCacheMatchKeysJSONArg struct{}

func (newAPICheckinCacheMatchKeysJSONArg) Match(value driver.Value) bool {
	raw, ok := value.(string)
	if !ok {
		return false
	}
	var keys map[string]string
	return json.Unmarshal([]byte(raw), &keys) == nil && keys["7"] == "sk-secret"
}

func (newAPICheckinProviderDataJSONArg) Match(value driver.Value) bool {
	var raw []byte
	switch typed := value.(type) {
	case []byte:
		raw = typed
	case string:
		raw = []byte(typed)
	default:
		return false
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return false
	}
	models, _ := payload["models"].([]any)
	return payload["plan_name"] == "尝鲜套餐" && len(models) == 1 && models[0] == "gpt-5.6-terra"
}

// TestNewAPICheckinRepositorySaveBalanceCachePersistsProviderData 验证只读数据可跨进程恢复。
func TestNewAPICheckinRepositorySaveBalanceCachePersistsProviderData(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer db.Close()

	quota := int64(12500000)
	usedQuota := int64(9954)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT a.id").
		WithArgs("sub2-demo", "primary").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(42))
	mock.ExpectExec("INSERT INTO newapi_checkin_account_balances").
		WithArgs(
			int64(42), "只读数据正常", "读取成功", nil, nil, nil,
			"", "", "", "", quota, "$25", usedQuota, "$0.0199075",
			nil, "", "2026-07-15 12:00:00", "account-refresh:sub2-demo/primary", newAPICheckinProviderDataJSONArg{},
		).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	repo := NewAPICheckinRepository(db)
	err = repo.SaveBalanceCache(context.Background(), service.NewAPICheckinBalancePayload{
		Source: "account-refresh:sub2-demo/primary",
		Accounts: []service.NewAPICheckinBalanceAccount{{
			Site:             "sub2-demo",
			Provider:         "sub2api",
			Account:          "primary",
			UserID:           "primary",
			Status:           "只读数据正常",
			Message:          "读取成功",
			Quota:            &quota,
			QuotaDisplay:     "$25",
			UsedQuota:        &usedQuota,
			UsedQuotaDisplay: "$0.0199075",
			LastRefreshedAt:  "2026-07-15 12:00:00",
			ProviderData: map[string]any{
				"plan_name": "尝鲜套餐",
				"models":    []any{"gpt-5.6-terra"},
			},
		}},
	})

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestNewAPICheckinRepositoryAPIKeyCacheRoundTrip 验证摘要与匹配 Key 分栏持久化并能恢复引用匹配信息。
func TestNewAPICheckinRepositoryAPIKeyCacheRoundTrip(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO newapi_checkin_api_key_cache").
		WithArgs("demo", "1001", newAPICheckinCacheSummaryJSONArg{}, newAPICheckinCacheMatchKeysJSONArg{}, "2026-07-15 19:00:00").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	summaryJSON := `{"site":"demo","provider":"newapi","user_id":"1001","status":"ready","group_status":"ready","available_groups":["default"],"api_keys":[{"id":7,"name":"codex","masked_key":"sk-se***cret","group":"default"}]}`
	mock.ExpectQuery("SELECT s.name, a.user_id, c.summary, c.match_keys, c.refreshed_at").
		WillReturnRows(sqlmock.NewRows([]string{"name", "user_id", "summary", "match_keys", "refreshed_at"}).
			AddRow("demo", "1001", []byte(summaryJSON), []byte(`{"7":"sk-secret"}`), "2026-07-15 19:00:00"))

	repo := NewAPICheckinRepository(db)
	err = repo.SaveAPIKeyCache(context.Background(), []service.NewAPICheckinAPIKeyCacheEntry{{
		Site: "demo", UserID: "1001", RefreshedAt: "2026-07-15 19:00:00",
		Summary: service.NewAPICheckinAccountAPIKeySummary{
			Site: "demo", Provider: "newapi", UserID: "1001", Status: "ready", GroupStatus: "ready",
			AvailableGroups: []string{"default"},
			APIKeys:         []service.NewAPICheckinAPIKeySummary{{ID: 7, Name: "codex", MaskedKey: "sk-se***cret", Group: "default", MatchKey: "sk-secret"}},
		},
	}})
	require.NoError(t, err)
	entries, err := repo.LoadAPIKeyCache(context.Background())
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "sk-secret", entries[0].Summary.APIKeys[0].MatchKey)
	encoded, err := json.Marshal(entries[0].Summary)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "sk-secret")
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestNewAPICheckinRepositoryDeleteSiteDataRemovesAllRows 验证平台删除使用同一事务清理关联表。
func TestNewAPICheckinRepositoryDeleteSiteDataRemovesAllRows(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM newapi_checkin_run_results").WithArgs("demo").WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("DELETE FROM newapi_checkin_history").WithArgs("demo").WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectExec("DELETE FROM newapi_checkin_monthly_records").WithArgs("demo").WillReturnResult(sqlmock.NewResult(0, 4))
	mock.ExpectExec("DELETE FROM newapi_checkin_api_key_cache").WithArgs("demo").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM newapi_checkin_account_balances").WithArgs("demo").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM newapi_checkin_accounts").WithArgs("demo").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM newapi_checkin_sites").WithArgs("demo").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	repo := NewAPICheckinRepository(db)
	require.NoError(t, repo.DeleteSiteData(context.Background(), "demo"))
	require.NoError(t, mock.ExpectationsWereMet())
}
