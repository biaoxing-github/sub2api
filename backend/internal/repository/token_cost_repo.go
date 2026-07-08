package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type tokenCostRepository struct {
	db *sql.DB
}

// NewTokenCostRepository 创建 token 成本计算器 SQL 仓储。
func NewTokenCostRepository(db *sql.DB) service.TokenCostRepository {
	if db == nil {
		return nil
	}
	return &tokenCostRepository{db: db}
}

func (r *tokenCostRepository) StorageLabel() string {
	return "sql:token-cost"
}

func (r *tokenCostRepository) LoadState(ctx context.Context) (service.TokenCostState, bool, error) {
	state := service.TokenCostState{}
	err := r.db.QueryRowContext(ctx, `
SELECT version, updated_at_text, rank_mode, personal_recharge_r
FROM token_cost_state
WHERE id = 1`).Scan(&state.Version, &state.UpdatedAt, &state.RankMode, &state.PersonalRechargeR)
	if errors.Is(err, sql.ErrNoRows) {
		return service.TokenCostState{}, false, nil
	}
	if err != nil {
		return service.TokenCostState{}, false, err
	}

	platforms, err := r.loadPlatforms(ctx)
	if err != nil {
		return service.TokenCostState{}, false, err
	}
	history, err := r.loadHistory(ctx)
	if err != nil {
		return service.TokenCostState{}, false, err
	}
	events, err := r.loadEvents(ctx)
	if err != nil {
		return service.TokenCostState{}, false, err
	}
	state.Platforms = platforms
	state.History = history
	state.Events = events
	return state, true, nil
}

func (r *tokenCostRepository) SaveState(ctx context.Context, state service.TokenCostState) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
INSERT INTO token_cost_state (id, version, updated_at_text, rank_mode, personal_recharge_r, updated_at)
VALUES (1, $1, $2, $3, $4, CURRENT_TIMESTAMP)
ON CONFLICT (id) DO UPDATE SET
  version = EXCLUDED.version,
  updated_at_text = EXCLUDED.updated_at_text,
  rank_mode = EXCLUDED.rank_mode,
  personal_recharge_r = EXCLUDED.personal_recharge_r,
  updated_at = CURRENT_TIMESTAMP`,
		state.Version, state.UpdatedAt, state.RankMode, state.PersonalRechargeR,
	); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM token_cost_platforms`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM token_cost_history`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM token_cost_events`); err != nil {
		return err
	}
	for index, platform := range state.Platforms {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO token_cost_platforms (
  id, name, balance_usd, calc_balance_usd, rate_r, rate_usd, plus, pro_min, pro_max, note, sort_order, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,CURRENT_TIMESTAMP)`,
			platform.ID, platform.Name, platform.BalanceUSD, tokenCostFloatValue(platform.CalcBalanceUSD),
			platform.RateR, platform.RateUSD, tokenCostFloatValue(platform.Plus), tokenCostFloatValue(platform.ProMin),
			tokenCostFloatValue(platform.ProMax), platform.Note, index,
		); err != nil {
			return err
		}
	}
	for index, item := range state.History {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO token_cost_history (sort_order, at_text, summary, total_balance, pro_min, pro_max, plus)
VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			index, item.At, item.Summary, item.TotalBalance, tokenCostFloatValue(item.ProMin),
			tokenCostFloatValue(item.ProMax), tokenCostFloatValue(item.Plus),
		); err != nil {
			return err
		}
	}
	for index, event := range state.Events {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO token_cost_events (sort_order, at_text, title, detail)
VALUES ($1,$2,$3,$4)`,
			index, event.At, event.Title, event.Detail,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *tokenCostRepository) loadPlatforms(ctx context.Context) ([]service.TokenCostPlatform, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, name, balance_usd, calc_balance_usd, rate_r, rate_usd, plus, pro_min, pro_max, note
FROM token_cost_platforms
ORDER BY sort_order ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []service.TokenCostPlatform{}
	for rows.Next() {
		var item service.TokenCostPlatform
		var calcBalance sql.NullFloat64
		var plus sql.NullFloat64
		var proMin sql.NullFloat64
		var proMax sql.NullFloat64
		if err := rows.Scan(
			&item.ID, &item.Name, &item.BalanceUSD, &calcBalance, &item.RateR, &item.RateUSD,
			&plus, &proMin, &proMax, &item.Note,
		); err != nil {
			return nil, err
		}
		item.CalcBalanceUSD = tokenCostFloatPointer(calcBalance)
		item.Plus = tokenCostFloatPointer(plus)
		item.ProMin = tokenCostFloatPointer(proMin)
		item.ProMax = tokenCostFloatPointer(proMax)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *tokenCostRepository) loadHistory(ctx context.Context) ([]service.TokenCostHistoryEntry, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT at_text, summary, total_balance, pro_min, pro_max, plus
FROM token_cost_history
ORDER BY sort_order ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []service.TokenCostHistoryEntry{}
	for rows.Next() {
		var item service.TokenCostHistoryEntry
		var proMin sql.NullFloat64
		var proMax sql.NullFloat64
		var plus sql.NullFloat64
		if err := rows.Scan(&item.At, &item.Summary, &item.TotalBalance, &proMin, &proMax, &plus); err != nil {
			return nil, err
		}
		item.ProMin = tokenCostFloatPointer(proMin)
		item.ProMax = tokenCostFloatPointer(proMax)
		item.Plus = tokenCostFloatPointer(plus)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *tokenCostRepository) loadEvents(ctx context.Context) ([]service.TokenCostEvent, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT at_text, title, detail
FROM token_cost_events
ORDER BY sort_order ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []service.TokenCostEvent{}
	for rows.Next() {
		var item service.TokenCostEvent
		if err := rows.Scan(&item.At, &item.Title, &item.Detail); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func tokenCostFloatValue(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

func tokenCostFloatPointer(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	return &value.Float64
}
