package repository

import (
	"strings"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

// TestAccountListOrderTodayStats 验证今日统计排序使用账号成本口径和当天时间边界。
func TestAccountListOrderTodayStats(t *testing.T) {
	selector := entsql.Dialect(dialect.Postgres).
		Select("*").
		From(entsql.Table("accounts"))

	for _, order := range accountListOrder(pagination.PaginationParams{
		SortBy:    "today_stats",
		SortOrder: pagination.SortOrderDesc,
	}) {
		order(selector)
	}

	query, args := selector.Query()
	require.Contains(t, query, "COALESCE(ul.account_stats_cost, ul.total_cost)")
	require.Contains(t, query, "ul.created_at >= $1")
	require.Contains(t, query, "DESC")
	require.Len(t, args, 1)
	startOfDay, ok := args[0].(time.Time)
	require.True(t, ok)
	require.Equal(t, 0, startOfDay.Hour())
	require.Equal(t, 0, startOfDay.Minute())
	require.Equal(t, 0, startOfDay.Second())
	require.False(t, strings.Contains(query, "NOW()"))
}
