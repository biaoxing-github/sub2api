package repository

import (
	"context"
	"database/sql"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

func TestTokenCostRepositoryLoadStateReturnsMissingWhenSingletonAbsent(t *testing.T) {
	db := newTokenCostRepoSQLite(t)
	repo := NewTokenCostRepository(db)

	state, found, err := repo.LoadState(context.Background())

	require.NoError(t, err)
	require.False(t, found)
	require.Empty(t, state.Platforms)
	require.Equal(t, "sql:token-cost", repo.StorageLabel())
}

func TestTokenCostRepositorySaveAndLoadStatePreservesOrderAndNulls(t *testing.T) {
	db := newTokenCostRepoSQLite(t)
	repo := NewTokenCostRepository(db)
	calcBalance := 10.7142857143
	plus := 0.03
	proMin := 0.1
	proMax := 0.5
	state := service.TokenCostState{
		Version:           1,
		UpdatedAt:         "2026-07-06T03:38:28.869Z",
		RankMode:          "pro",
		PersonalRechargeR: 400,
		Platforms: []service.TokenCostPlatform{
			{
				ID:             "tokeness",
				Name:           "tokeness",
				BalanceUSD:     75,
				CalcBalanceUSD: &calcBalance,
				RateR:          1,
				RateUSD:        1,
				Plus:           &plus,
				ProMin:         &proMin,
				ProMax:         &proMax,
				Note:           "折算后计算购买力",
			},
			{
				ID:         "encore",
				Name:       "encore",
				BalanceUSD: 1055,
				RateR:      1,
				RateUSD:    10,
				Plus:       nil,
				ProMin:     nil,
				ProMax:     nil,
				Note:       "空倍率要保持 null",
			},
		},
		History: []service.TokenCostHistoryEntry{
			{At: "2026-07-05", Summary: "记录历史点", TotalBalance: 1130, Plus: &plus, ProMin: &proMin, ProMax: &proMax},
		},
		Events: []service.TokenCostEvent{
			{At: "2026/7/6 11:38:28", Title: "编辑平台", Detail: "小白code 的 plus：0.13 -> 0.1"},
		},
	}

	require.NoError(t, repo.SaveState(context.Background(), state))
	loaded, found, err := repo.LoadState(context.Background())

	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, state, loaded)
	require.NotNil(t, loaded.Platforms[0].CalcBalanceUSD)
	require.Nil(t, loaded.Platforms[1].Plus)
	require.Nil(t, loaded.Platforms[1].ProMin)
	require.Equal(t, "编辑平台", loaded.Events[0].Title)
}

func TestTokenCostRepositorySaveStateReplacesRemovedRows(t *testing.T) {
	db := newTokenCostRepoSQLite(t)
	repo := NewTokenCostRepository(db)
	first := service.TokenCostState{
		Version:           1,
		UpdatedAt:         "2026-07-05T00:00:00Z",
		RankMode:          "plus",
		PersonalRechargeR: 333,
		Platforms: []service.TokenCostPlatform{
			{ID: "old-a", Name: "old-a", BalanceUSD: 1, RateR: 1, RateUSD: 1},
			{ID: "old-b", Name: "old-b", BalanceUSD: 2, RateR: 1, RateUSD: 1},
		},
		History: []service.TokenCostHistoryEntry{
			{At: "2026-07-05", Summary: "old", TotalBalance: 3},
		},
		Events: []service.TokenCostEvent{
			{At: "2026/7/5", Title: "old", Detail: "old"},
		},
	}
	second := service.TokenCostState{
		Version:           1,
		UpdatedAt:         "2026-07-06T00:00:00Z",
		RankMode:          "plus",
		PersonalRechargeR: 400,
		Platforms: []service.TokenCostPlatform{
			{ID: "new-only", Name: "new-only", BalanceUSD: 9, RateR: 1, RateUSD: 1},
		},
		History: []service.TokenCostHistoryEntry{
			{At: "2026-07-06", Summary: "new", TotalBalance: 9},
		},
		Events: []service.TokenCostEvent{
			{At: "2026/7/6", Title: "new", Detail: "new"},
		},
	}

	require.NoError(t, repo.SaveState(context.Background(), first))
	require.NoError(t, repo.SaveState(context.Background(), second))
	loaded, found, err := repo.LoadState(context.Background())

	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, second, loaded)
}

func newTokenCostRepoSQLite(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:token_cost_repo?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`
CREATE TABLE token_cost_state (
    id INTEGER PRIMARY KEY,
    version INTEGER NOT NULL,
    updated_at_text TEXT NOT NULL DEFAULT '',
    rank_mode TEXT NOT NULL DEFAULT 'plus',
    personal_recharge_r REAL NOT NULL DEFAULT 400,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE token_cost_platforms (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    balance_usd REAL NOT NULL DEFAULT 0,
    calc_balance_usd REAL,
    rate_r REAL NOT NULL DEFAULT 1,
    rate_usd REAL NOT NULL DEFAULT 1,
    plus REAL,
    pro_min REAL,
    pro_max REAL,
    note TEXT NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE token_cost_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    sort_order INTEGER NOT NULL DEFAULT 0,
    at_text TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    total_balance REAL NOT NULL DEFAULT 0,
    pro_min REAL,
    pro_max REAL,
    plus REAL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE token_cost_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    sort_order INTEGER NOT NULL DEFAULT 0,
    at_text TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    detail TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`)
	require.NoError(t, err)
	return db
}
