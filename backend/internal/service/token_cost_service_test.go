package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func fixedTokenCostNow() time.Time {
	return time.Date(2026, 7, 8, 10, 11, 12, 0, time.UTC)
}

func TestTokenCostGetStateReturnsDefaultWhenRepositoryEmpty(t *testing.T) {
	repo := &memoryTokenCostRepository{}
	svc := NewTokenCostService(TokenCostOptions{Repository: repo, Now: fixedTokenCostNow})

	state, err := svc.GetState(context.Background())
	require.NoError(t, err)

	require.Equal(t, 1, state.Version)
	require.Equal(t, "plus", state.RankMode)
	require.Equal(t, 400.0, state.PersonalRechargeR)
	require.Empty(t, state.Platforms)
	require.Empty(t, state.History)
	require.Empty(t, state.Events)
}

func TestTokenCostSaveAndLoadStatePreservesEditableDashboardData(t *testing.T) {
	repo := &memoryTokenCostRepository{}
	svc := NewTokenCostService(TokenCostOptions{Repository: repo, Now: fixedTokenCostNow})
	state := TokenCostState{
		Version:           1,
		UpdatedAt:         "2026-07-06T03:38:28.869Z",
		RankMode:          "pro",
		PersonalRechargeR: 400,
		Platforms: []TokenCostPlatform{
			{
				ID:             "tokeness",
				Name:           "tokeness",
				BalanceUSD:     75,
				CalcBalanceUSD: tokenCostFloatPtr(10.7142857143),
				RateR:          1,
				RateUSD:        1,
				Plus:           tokenCostFloatPtr(0.03),
				ProMin:         tokenCostFloatPtr(0.1),
				Note:           "折算后计算购买力",
			},
			{
				ID:         "encore",
				Name:       "encore",
				BalanceUSD: 1055,
				RateR:      1,
				RateUSD:    10,
				Plus:       tokenCostFloatPtr(1),
				ProMin:     tokenCostFloatPtr(1.69),
			},
		},
		History: []TokenCostHistoryEntry{
			{At: "2026-07-05", Summary: "记录历史点", TotalBalance: 1130, Plus: tokenCostFloatPtr(1055), ProMin: tokenCostFloatPtr(1056.07), ProMax: tokenCostFloatPtr(1056.07)},
		},
		Events: []TokenCostEvent{
			{At: "2026/7/6 11:38:28", Title: "编辑平台", Detail: "小白code 的 plus：0.13 -> 0.1"},
		},
	}

	saved, err := svc.SaveState(context.Background(), state)
	require.NoError(t, err)
	require.Equal(t, state.UpdatedAt, saved.UpdatedAt)

	loaded, err := svc.GetState(context.Background())
	require.NoError(t, err)
	require.Equal(t, saved, loaded)
	require.NotNil(t, loaded.Platforms[0].CalcBalanceUSD)
	require.Equal(t, 10.7142857143, *loaded.Platforms[0].CalcBalanceUSD)
	require.Equal(t, "编辑平台", loaded.Events[0].Title)
}

func TestTokenCostCalculateTotalsUsesBalanceCostAndCalcBalanceForBuyingPower(t *testing.T) {
	state := TokenCostState{
		PersonalRechargeR: 400,
		Platforms: []TokenCostPlatform{
			{
				ID:         "book-cost-only",
				Name:       "book-cost-only",
				BalanceUSD: 100,
				RateR:      2,
				RateUSD:    10,
				Plus:       tokenCostFloatPtr(0.5),
				ProMin:     tokenCostFloatPtr(0.25),
				ProMax:     tokenCostFloatPtr(0.5),
			},
			{
				ID:             "calc-balance",
				Name:           "calc-balance",
				BalanceUSD:     75,
				CalcBalanceUSD: tokenCostFloatPtr(10),
				RateR:          1,
				RateUSD:        1,
				Plus:           tokenCostFloatPtr(0.1),
				ProMin:         tokenCostFloatPtr(0.2),
			},
			{
				ID:         "empty",
				Name:       "empty",
				BalanceUSD: 0,
				RateR:      1,
				RateUSD:    1,
				Plus:       tokenCostFloatPtr(0.01),
				ProMin:     tokenCostFloatPtr(0.01),
			},
		},
	}

	totals := CalculateTokenCostTotals(state)

	require.Equal(t, 2, totals.ActiveCount)
	require.InDelta(t, 175, totals.TotalBalance, 0.000001)
	require.InDelta(t, 110, totals.ComparableBalance, 0.000001)
	require.InDelta(t, 95, totals.BookCost, 0.000001)
	require.InDelta(t, 95, totals.ComparableCost, 0.000001)
	require.InDelta(t, 300, totals.PlusPower, 0.000001)
	require.InDelta(t, 250, totals.ProPowerMin, 0.000001)
	require.InDelta(t, 450, totals.ProPowerMax, 0.000001)
	require.InDelta(t, 0.2375, totals.BookMultiple, 0.000001)
	require.InDelta(t, 3.1578947368, totals.PlusPerR, 0.000001)
	require.InDelta(t, 2.6315789474, totals.ProPerRMin, 0.000001)
	require.InDelta(t, 4.7368421053, totals.ProPerRMax, 0.000001)
}

type memoryTokenCostRepository struct {
	state TokenCostState
	found bool
}

func (r *memoryTokenCostRepository) LoadState(context.Context) (TokenCostState, bool, error) {
	return r.state, r.found, nil
}

func (r *memoryTokenCostRepository) SaveState(_ context.Context, state TokenCostState) error {
	r.state = state
	r.found = true
	return nil
}

func (r *memoryTokenCostRepository) StorageLabel() string {
	return "memory:token-cost"
}

func tokenCostFloatPtr(value float64) *float64 {
	return &value
}
