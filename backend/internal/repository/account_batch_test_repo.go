package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type accountBatchTestRepository struct {
	db *sql.DB
}

func NewAccountBatchTestRepository(db *sql.DB) service.AccountBatchTestRepository {
	return &accountBatchTestRepository{db: db}
}

func (r *accountBatchTestRepository) CreateAccountBatchTestRun(ctx context.Context, run *service.AccountBatchTestRun) error {
	if run == nil {
		return errors.New("account batch test run is nil")
	}
	return r.db.QueryRowContext(ctx, `
INSERT INTO account_batch_test_runs (
  status, model_id, platform, status_filter, search, concurrency, limit_count,
  total_count, success_count, failed_count, unauthorized_count,
  error_message, created_at, started_at, finished_at
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,
  $8,$9,$10,$11,
  NULLIF($12,''),$13,$14,$15
) RETURNING id, created_at`,
		run.Status, run.ModelID, run.Platform, run.StatusFilter, run.Search, run.Concurrency, run.Limit,
		run.Total, run.SuccessCount, run.FailedCount, run.UnauthorizedCount,
		run.ErrorMessage, run.CreatedAt, run.StartedAt, run.FinishedAt,
	).Scan(&run.ID, &run.CreatedAt)
}

func (r *accountBatchTestRepository) CreateAccountBatchTestItems(ctx context.Context, runID int64, items []service.AccountBatchTestItem) error {
	if len(items) == 0 {
		return nil
	}
	const query = `
INSERT INTO account_batch_test_items (
  run_id, account_id, account_name, platform, account_type, status, category,
  message, error_message, latency_ms, first_token_ms, created_at, started_at, finished_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF($8,''),NULLIF($9,''),$10,$11,$12,$13,$14)`
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, item := range items {
		_, err := tx.ExecContext(ctx, query,
			runID, item.AccountID, item.AccountName, item.Platform, item.Type, item.Status, item.Category,
			item.Message, item.ErrorMessage, item.LatencyMs, nullableInt(item.FirstTokenMs), item.CreatedAt, item.StartedAt, item.FinishedAt,
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *accountBatchTestRepository) UpdateAccountBatchTestItem(ctx context.Context, item service.AccountBatchTestItem) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE account_batch_test_items SET
  status = $3,
  category = $4,
  message = NULLIF($5,''),
  error_message = NULLIF($6,''),
  latency_ms = $7,
  first_token_ms = $8,
  started_at = $9,
  finished_at = $10
WHERE run_id = $1 AND account_id = $2`,
		item.RunID, item.AccountID, item.Status, item.Category, item.Message, item.ErrorMessage, item.LatencyMs, nullableInt(item.FirstTokenMs), item.StartedAt, item.FinishedAt,
	)
	return err
}

func (r *accountBatchTestRepository) UpdateAccountBatchTestRun(ctx context.Context, run *service.AccountBatchTestRun) error {
	if run == nil {
		return errors.New("account batch test run is nil")
	}
	_, err := r.db.ExecContext(ctx, `
UPDATE account_batch_test_runs SET
  status = $2,
  success_count = $3,
  failed_count = $4,
  unauthorized_count = $5,
  error_message = NULLIF($6,''),
  started_at = $7,
  finished_at = $8
WHERE id = $1`,
		run.ID, run.Status, run.SuccessCount, run.FailedCount, run.UnauthorizedCount, run.ErrorMessage, run.StartedAt, run.FinishedAt,
	)
	return err
}

func (r *accountBatchTestRepository) ListAccountBatchTestRuns(ctx context.Context, filter service.AccountBatchTestRunFilter) ([]service.AccountBatchTestRun, int, error) {
	filter = normalizeAccountBatchTestRunFilter(filter)
	where, args := buildAccountBatchTestRunWhere(filter)
	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM account_batch_test_runs"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := filter.PageSize
	offset := (filter.Page - 1) * filter.PageSize
	queryArgs := append(append([]any{}, args...), limit, offset)
	rows, err := r.db.QueryContext(ctx, accountBatchTestRunSelectSQL+`
FROM account_batch_test_runs`+where+`
ORDER BY created_at DESC
LIMIT $`+fmt.Sprint(len(args)+1)+` OFFSET $`+fmt.Sprint(len(args)+2), queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]service.AccountBatchTestRun, 0)
	for rows.Next() {
		run, err := scanAccountBatchTestRun(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *run)
	}
	return out, total, rows.Err()
}

func (r *accountBatchTestRepository) GetAccountBatchTestRun(ctx context.Context, runID int64) (*service.AccountBatchTestRun, []service.AccountBatchTestItem, error) {
	run, err := scanAccountBatchTestRun(r.db.QueryRowContext(ctx, accountBatchTestRunSelectSQL+`
FROM account_batch_test_runs
WHERE id = $1`, runID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, service.ErrAccountNotFound
		}
		return nil, nil, err
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT id, run_id, account_id, account_name, platform, account_type, status, category,
       COALESCE(message,''), COALESCE(error_message,''), latency_ms, first_token_ms, created_at, started_at, finished_at
FROM account_batch_test_items
WHERE run_id = $1
ORDER BY id ASC`, runID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	items := make([]service.AccountBatchTestItem, 0)
	for rows.Next() {
		item, err := scanAccountBatchTestItem(rows)
		if err != nil {
			return nil, nil, err
		}
		items = append(items, *item)
	}
	return run, items, rows.Err()
}

const accountBatchTestRunSelectSQL = `
SELECT id, status, model_id, platform, status_filter, search, concurrency, limit_count,
       total_count, success_count, failed_count,
       COALESCE((
         SELECT COUNT(*)
         FROM account_batch_test_items i
         WHERE i.run_id = account_batch_test_runs.id
           AND (i.category = 'unauthorized' OR (
             i.category <> 'unauthorized' AND (
               LOWER(COALESCE(i.error_message,'')) LIKE '%401%' OR
               LOWER(COALESCE(i.error_message,'')) LIKE '%unauthorized%' OR
               LOWER(COALESCE(i.error_message,'')) LIKE '%authentication failed%' OR
               LOWER(COALESCE(i.error_message,'')) LIKE '%token invalid%' OR
               LOWER(COALESCE(i.error_message,'')) LIKE '%invalid api key%'
             )
           ))
       ), unauthorized_count),
       COALESCE((
         SELECT COUNT(*)
         FROM account_batch_test_items i
         WHERE i.run_id = account_batch_test_runs.id
           AND (i.category IN ('rate_limited', 'client_ip_circuit_open') OR (
             i.category NOT IN ('rate_limited', 'client_ip_circuit_open') AND (
               LOWER(COALESCE(i.error_message,'')) LIKE '%429%' OR
               LOWER(COALESCE(i.error_message,'')) LIKE '%rate limit%' OR
               LOWER(COALESCE(i.error_message,'')) LIKE '%rate_limited%' OR
               LOWER(COALESCE(i.error_message,'')) LIKE '%too many requests%' OR
               LOWER(COALESCE(i.error_message,'')) LIKE '%client_ip_error_circuit_open%'
             )
           ))
       ), 0),
       COALESCE(error_message,''), created_at, started_at, finished_at
`

func normalizeAccountBatchTestRunFilter(filter service.AccountBatchTestRunFilter) service.AccountBatchTestRunFilter {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 || filter.PageSize > 100 {
		filter.PageSize = 20
	}
	filter.Status = strings.TrimSpace(filter.Status)
	filter.Keyword = strings.TrimSpace(filter.Keyword)
	return filter
}

func buildAccountBatchTestRunWhere(filter service.AccountBatchTestRunFilter) (string, []any) {
	clauses := make([]string, 0)
	args := make([]any, 0)
	if filter.Status != "" {
		args = append(args, filter.Status)
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	if filter.Keyword != "" {
		args = append(args, "%"+strings.ToLower(filter.Keyword)+"%")
		clauses = append(clauses, fmt.Sprintf("(LOWER(model_id) LIKE $%d OR LOWER(platform) LIKE $%d OR LOWER(search) LIKE $%d)", len(args), len(args), len(args)))
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func scanAccountBatchTestRun(scanner interface{ Scan(...any) error }) (*service.AccountBatchTestRun, error) {
	var run service.AccountBatchTestRun
	var startedAt, finishedAt sql.NullTime
	if err := scanner.Scan(
		&run.ID, &run.Status, &run.ModelID, &run.Platform, &run.StatusFilter, &run.Search, &run.Concurrency, &run.Limit,
		&run.Total, &run.SuccessCount, &run.FailedCount, &run.UnauthorizedCount, &run.RateLimitedCount,
		&run.ErrorMessage, &run.CreatedAt, &startedAt, &finishedAt,
	); err != nil {
		return nil, err
	}
	if startedAt.Valid {
		run.StartedAt = &startedAt.Time
	}
	if finishedAt.Valid {
		run.FinishedAt = &finishedAt.Time
	}
	return &run, nil
}

func scanAccountBatchTestItem(scanner interface{ Scan(...any) error }) (*service.AccountBatchTestItem, error) {
	var item service.AccountBatchTestItem
	var startedAt, finishedAt sql.NullTime
	var firstTokenMs sql.NullInt64
	if err := scanner.Scan(
		&item.ID, &item.RunID, &item.AccountID, &item.AccountName, &item.Platform, &item.Type, &item.Status, &item.Category,
		&item.Message, &item.ErrorMessage, &item.LatencyMs, &firstTokenMs, &item.CreatedAt, &startedAt, &finishedAt,
	); err != nil {
		return nil, err
	}
	if firstTokenMs.Valid {
		value := int(firstTokenMs.Int64)
		item.FirstTokenMs = &value
	}
	if startedAt.Valid {
		item.StartedAt = &startedAt.Time
	}
	if finishedAt.Valid {
		item.FinishedAt = &finishedAt.Time
	}
	return &item, nil
}

func nullableInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

var _ service.AccountBatchTestRepository = (*accountBatchTestRepository)(nil)
