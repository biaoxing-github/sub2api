package repository

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type newAPICheckinProviderDataJSONArg struct{}

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
