package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type accountProbeRepository struct {
	db *sql.DB
}

type accountProbeSQLExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func NewAccountProbeRepository(db *sql.DB) service.AccountProbeRepository {
	return &accountProbeRepository{db: db}
}

func (r *accountProbeRepository) CreateAccountProbeRun(ctx context.Context, run *service.AccountProbeResult) error {
	if run == nil {
		return errors.New("account probe run is nil")
	}
	return r.db.QueryRowContext(ctx, `
INSERT INTO account_probe_runs (
  account_id, mode, probe_source, status, model, request_mode, codex_stability, long_context,
  request_count, success_count, failure_count,
  input_tokens, output_tokens, total_tokens,
  p50_ms, p95_ms, avg_ms, max_ms, first_token_ms,
  estimated_input_tokens_min, estimated_input_tokens_max,
  estimated_output_tokens_min, estimated_output_tokens_max,
  error_message, summary, created_at, started_at, finished_at
) VALUES (
  $1,$2,COALESCE(NULLIF($3,''),'self_validation'),$4,$5,$6,$7,$8,
  $9,$10,$11,
  $12,$13,$14,
  $15,$16,$17,$18,$19,
  $20,$21,$22,$23,
  NULLIF($24,''), NULLIF($25,''), $26,$27,$28
) RETURNING id, created_at`,
		run.AccountID, run.Profile, run.ProbeSource, run.Status, run.Model, run.RequestMode, run.IncludeCodexStability, run.IncludeLongContext,
		run.RequestCount, run.SuccessCount, run.FailureCount,
		run.InputTokens, run.OutputTokens, run.TotalTokens,
		run.Latency.P50Millis, run.Latency.P95Millis, run.Latency.AvgMillis, run.Latency.MaxMillis, run.FirstTokenMillis,
		run.Estimate.InputTokensMin, run.Estimate.InputTokensMax, run.Estimate.OutputTokensMin, run.Estimate.OutputTokensMax,
		run.ErrorMessage, run.Summary, run.CreatedAt, run.StartedAt, run.FinishedAt,
	).Scan(&run.ID, &run.CreatedAt)
}

func (r *accountProbeRepository) UpdateAccountProbeRun(ctx context.Context, run *service.AccountProbeResult) error {
	if run == nil {
		return errors.New("account probe run is nil")
	}
	_, err := r.db.ExecContext(ctx, `
UPDATE account_probe_runs SET
  status = $2,
  success_count = $3,
  failure_count = $4,
  input_tokens = $5,
  output_tokens = $6,
  total_tokens = $7,
  p50_ms = $8,
  p95_ms = $9,
  avg_ms = $10,
  max_ms = $11,
  first_token_ms = $12,
  error_message = NULLIF($13,''),
  summary = NULLIF($14,''),
  finished_at = $15
WHERE id = $1`,
		run.ID,
		run.Status,
		run.SuccessCount,
		run.FailureCount,
		run.InputTokens,
		run.OutputTokens,
		run.TotalTokens,
		run.Latency.P50Millis,
		run.Latency.P95Millis,
		run.Latency.AvgMillis,
		run.Latency.MaxMillis,
		run.FirstTokenMillis,
		run.ErrorMessage,
		run.Summary,
		run.FinishedAt,
	)
	return err
}

func (r *accountProbeRepository) ExpireStaleAccountProbeRuns(ctx context.Context, olderThan time.Duration) error {
	return expireStaleAccountProbeRuns(ctx, r.db, olderThan)
}

func expireStaleAccountProbeRuns(ctx context.Context, exec accountProbeSQLExecutor, olderThan time.Duration) error {
	if olderThan <= 0 {
		olderThan = 12 * time.Minute
	}
	_, err := exec.ExecContext(ctx, `
UPDATE account_probe_runs
SET status = $2,
    error_message = COALESCE(NULLIF(error_message,''), $3),
    summary = COALESCE(NULLIF(summary,''), $4),
    finished_at = COALESCE(finished_at, now())
WHERE status = $1
  AND created_at < now() - $5::interval`,
		service.AccountProbeStatusRunning,
		service.AccountProbeStatusFailed,
		"probe run timed out before completion",
		"体检任务超时未完成，已自动收尾",
		formatPostgresDuration(olderThan),
	)
	return err
}

func (r *accountProbeRepository) SaveAccountProbeSample(ctx context.Context, sample service.AccountProbeSample) error {
	if sample.ValidationEvidence == nil {
		sample.ValidationEvidence = []service.AccountProbeValidationEvidence{}
	}
	validationEvidence, err := json.Marshal(sample.ValidationEvidence)
	if err != nil {
		return fmt.Errorf("marshal account probe validation evidence: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
INSERT INTO account_probe_samples (
  run_id, request_index, sample_type, label, status, model,
  api_key_fingerprint, api_key_masked, upstream_endpoint, http_status,
  duration_ms, first_token_ms,
  input_tokens, output_tokens, total_tokens,
  output_text, validation_evidence,
  request_prompt, request_body, response_body,
  error_code, error_message, created_at
) VALUES (
  $1,$2,$3,$4,$5,$6,
  $7,$8,NULLIF($9,''),$10,
  $11,$12,
  $13,$14,$15,
  $16,$17::jsonb,
  $18,$19,$20,
  NULLIF($21,''),NULLIF($22,''),$23
)`,
		sample.RunID, sample.RequestIndex, sample.Type, sample.Label, sample.Status, sample.Model,
		sample.APIKeyFingerprint, sample.APIKeyMasked, sample.UpstreamEndpoint, sample.HTTPStatus,
		sample.DurationMillis, sample.FirstTokenMillis,
		sample.InputTokens, sample.OutputTokens, sample.TotalTokens,
		sample.OutputText, string(validationEvidence),
		sample.RequestPrompt, sample.RequestBody, sample.ResponseBody,
		sample.ErrorCode, sample.ErrorMessage, sample.CreatedAt,
	)
	return err
}

func (r *accountProbeRepository) ListAccountProbeRuns(ctx context.Context, filter service.AccountProbeHistoryFilter) ([]service.AccountProbeResult, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx, accountProbeRunSelectSQL+`
FROM account_probe_runs
WHERE account_id = $1
ORDER BY created_at DESC
LIMIT $2`, filter.AccountID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]service.AccountProbeResult, 0)
	for rows.Next() {
		run, err := scanAccountProbeRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *run)
	}
	return out, rows.Err()
}

func (r *accountProbeRepository) GetAccountProbeRun(ctx context.Context, accountID, runID int64) (*service.AccountProbeResult, error) {
	run, err := scanAccountProbeRun(r.db.QueryRowContext(ctx, accountProbeRunSelectSQL+`
FROM account_probe_runs
WHERE id = $1 AND account_id = $2`, runID, accountID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrAccountNotFound
		}
		return nil, err
	}
	return run, nil
}

func (r *accountProbeRepository) ListAccountProbeReportRuns(ctx context.Context, filter service.AccountProbeReportFilter) ([]service.AccountProbeReportItem, int, error) {
	filter = normalizeAccountProbeReportRepositoryFilter(filter)
	where, args := buildAccountProbeReportWhere(filter)
	totalQuery := "SELECT COUNT(*) FROM account_probe_runs r JOIN accounts a ON a.id = r.account_id" + where
	var total int
	if err := r.db.QueryRowContext(ctx, totalQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	orderBy := accountProbeReportOrderBy(filter)
	limit := filter.PageSize
	offset := (filter.Page - 1) * filter.PageSize
	queryArgs := append(append([]any{}, args...), limit, offset)
	rows, err := r.db.QueryContext(ctx, accountProbeReportSelectSQL+`
FROM account_probe_runs r
JOIN accounts a ON a.id = r.account_id`+where+`
ORDER BY `+orderBy+`
LIMIT $`+fmt.Sprint(len(args)+1)+` OFFSET $`+fmt.Sprint(len(args)+2), queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]service.AccountProbeReportItem, 0)
	for rows.Next() {
		item, err := scanAccountProbeReportItem(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *item)
	}
	return out, total, rows.Err()
}

func (r *accountProbeRepository) GetAccountProbeReportRun(ctx context.Context, runID int64) (*service.AccountProbeReportItem, error) {
	item, err := scanAccountProbeReportItem(r.db.QueryRowContext(ctx, accountProbeReportSelectSQL+`
FROM account_probe_runs r
JOIN accounts a ON a.id = r.account_id
WHERE r.id = $1`, runID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrAccountNotFound
		}
		return nil, err
	}
	return item, nil
}

func (r *accountProbeRepository) DeleteAccountProbeReportRuns(ctx context.Context, runIDs []int64) (service.AccountProbeReportDeleteResult, error) {
	result := service.AccountProbeReportDeleteResult{RequestedCount: len(runIDs)}
	if len(runIDs) == 0 {
		return result, nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()

	if err := expireStaleAccountProbeRuns(ctx, tx, 12*time.Minute); err != nil {
		return result, err
	}

	if err := tx.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM account_probe_runs
WHERE id = ANY($1) AND status = $2`, pq.Array(runIDs), service.AccountProbeStatusRunning).Scan(&result.SkippedRunningCount); err != nil {
		return result, err
	}
	if _, err := tx.ExecContext(ctx, `
DELETE FROM account_probe_samples
WHERE run_id IN (
  SELECT id
  FROM account_probe_runs
  WHERE id = ANY($1) AND COALESCE(status,'') <> $2
)`, pq.Array(runIDs), service.AccountProbeStatusRunning); err != nil {
		return result, err
	}
	deleteResult, err := tx.ExecContext(ctx, `
DELETE FROM account_probe_runs
WHERE id = ANY($1) AND COALESCE(status,'') <> $2`, pq.Array(runIDs), service.AccountProbeStatusRunning)
	if err != nil {
		return result, err
	}
	deleted, err := deleteResult.RowsAffected()
	if err != nil {
		return result, err
	}
	result.DeletedCount = int(deleted)
	if err := tx.Commit(); err != nil {
		return result, err
	}
	return result, nil
}

func (r *accountProbeRepository) ListAccountProbeRankingRuns(ctx context.Context, perAccountLimit int) ([]service.AccountProbeReportItem, error) {
	if perAccountLimit <= 0 || perAccountLimit > 50 {
		perAccountLimit = 20
	}
	rows, err := r.db.QueryContext(ctx, accountProbeReportSelectSQL+`
FROM (
  SELECT r.*, ROW_NUMBER() OVER (PARTITION BY r.account_id ORDER BY r.created_at DESC) AS rn
  FROM account_probe_runs r
  WHERE COALESCE(r.status,'') <> $1
    AND COALESCE(r.mode,'') <> $2
) r
JOIN accounts a ON a.id = r.account_id
WHERE r.rn <= $3
ORDER BY r.account_id ASC, r.created_at DESC`, service.AccountProbeStatusRunning, service.AccountProbeProfileModelValidation, perAccountLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]service.AccountProbeReportItem, 0)
	for rows.Next() {
		item, err := scanAccountProbeReportItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func (r *accountProbeRepository) ListAccountProbeSamples(ctx context.Context, runID int64) ([]service.AccountProbeSample, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, run_id, request_index, sample_type, label, status, model,
       api_key_fingerprint, api_key_masked, COALESCE(upstream_endpoint,''), http_status,
       duration_ms, first_token_ms,
       input_tokens, output_tokens, total_tokens,
       COALESCE(output_text,''), COALESCE(validation_evidence,'[]'::jsonb),
       COALESCE(request_prompt,''), COALESCE(request_body,''), COALESCE(response_body,''),
       COALESCE(error_code,''), COALESCE(error_message,''), created_at
FROM account_probe_samples
WHERE run_id = $1
ORDER BY request_index ASC`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []service.AccountProbeSample
	for rows.Next() {
		sample, err := scanAccountProbeSample(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *sample)
	}
	return out, rows.Err()
}

const accountProbeRunSelectSQL = `
SELECT id, account_id, mode, COALESCE(probe_source,'self_validation'), status, model, COALESCE(request_mode,'non_stream'), codex_stability, long_context,
       request_count, success_count, failure_count,
       input_tokens, output_tokens, total_tokens,
       p50_ms, p95_ms, avg_ms, max_ms, first_token_ms,
       estimated_input_tokens_min, estimated_input_tokens_max,
       estimated_output_tokens_min, estimated_output_tokens_max,
       error_message, summary, created_at, started_at, finished_at
`

const accountProbeReportSelectSQL = `
SELECT r.id, r.account_id, COALESCE(a.name,''), r.mode, COALESCE(r.probe_source,'self_validation'), r.status, r.model, COALESCE(r.request_mode,'non_stream'), r.codex_stability, r.long_context,
       r.request_count, r.success_count, r.failure_count,
       r.input_tokens, r.output_tokens, r.total_tokens,
       r.p50_ms, r.p95_ms, r.avg_ms, r.max_ms, r.first_token_ms,
       r.estimated_input_tokens_min, r.estimated_input_tokens_max,
       r.estimated_output_tokens_min, r.estimated_output_tokens_max,
       r.error_message, r.summary, r.created_at, r.started_at, r.finished_at
`

func scanAccountProbeRun(scanner interface{ Scan(...any) error }) (*service.AccountProbeResult, error) {
	var run service.AccountProbeResult
	var firstToken sql.NullInt64
	var errorMessage, summary sql.NullString
	var startedAt, finishedAt sql.NullTime
	if err := scanner.Scan(
		&run.ID, &run.AccountID, &run.Profile, &run.ProbeSource, &run.Status, &run.Model, &run.RequestMode, &run.IncludeCodexStability, &run.IncludeLongContext,
		&run.RequestCount, &run.SuccessCount, &run.FailureCount,
		&run.InputTokens, &run.OutputTokens, &run.TotalTokens,
		&run.Latency.P50Millis, &run.Latency.P95Millis, &run.Latency.AvgMillis, &run.Latency.MaxMillis, &firstToken,
		&run.Estimate.InputTokensMin, &run.Estimate.InputTokensMax, &run.Estimate.OutputTokensMin, &run.Estimate.OutputTokensMax,
		&errorMessage, &summary, &run.CreatedAt, &startedAt, &finishedAt,
	); err != nil {
		return nil, err
	}
	run.Estimate.Requests = run.RequestCount
	run.Estimate.TotalTokensMin = run.Estimate.InputTokensMin + run.Estimate.OutputTokensMin
	run.Estimate.TotalTokensMax = run.Estimate.InputTokensMax + run.Estimate.OutputTokensMax
	run.AvgLatencyMillis = run.Latency.AvgMillis
	run.MaxLatencyMillis = run.Latency.MaxMillis
	if firstToken.Valid {
		v := int(firstToken.Int64)
		run.FirstTokenMillis = &v
	}
	if errorMessage.Valid {
		run.ErrorMessage = errorMessage.String
	}
	if summary.Valid {
		run.Summary = summary.String
	}
	if startedAt.Valid {
		run.StartedAt = &startedAt.Time
	}
	if finishedAt.Valid {
		run.FinishedAt = &finishedAt.Time
	}
	return &run, nil
}

func scanAccountProbeReportItem(scanner interface{ Scan(...any) error }) (*service.AccountProbeReportItem, error) {
	var item service.AccountProbeReportItem
	var firstToken sql.NullInt64
	var errorMessage, summary sql.NullString
	var startedAt, finishedAt sql.NullTime
	if err := scanner.Scan(
		&item.ID, &item.AccountID, &item.AccountName, &item.Profile, &item.ProbeSource, &item.Status, &item.Model, &item.RequestMode, &item.IncludeCodexStability, &item.IncludeLongContext,
		&item.RequestCount, &item.SuccessCount, &item.FailureCount,
		&item.InputTokens, &item.OutputTokens, &item.TotalTokens,
		&item.Latency.P50Millis, &item.Latency.P95Millis, &item.Latency.AvgMillis, &item.Latency.MaxMillis, &firstToken,
		&item.Estimate.InputTokensMin, &item.Estimate.InputTokensMax, &item.Estimate.OutputTokensMin, &item.Estimate.OutputTokensMax,
		&errorMessage, &summary, &item.CreatedAt, &startedAt, &finishedAt,
	); err != nil {
		return nil, err
	}
	item.Estimate.Requests = item.RequestCount
	item.Estimate.TotalTokensMin = item.Estimate.InputTokensMin + item.Estimate.OutputTokensMin
	item.Estimate.TotalTokensMax = item.Estimate.InputTokensMax + item.Estimate.OutputTokensMax
	item.AvgLatencyMillis = item.Latency.AvgMillis
	item.MaxLatencyMillis = item.Latency.MaxMillis
	if firstToken.Valid {
		v := int(firstToken.Int64)
		item.FirstTokenMillis = &v
	}
	if errorMessage.Valid {
		item.ErrorMessage = errorMessage.String
	}
	if summary.Valid {
		item.Summary = summary.String
	}
	if startedAt.Valid {
		item.StartedAt = &startedAt.Time
	}
	if finishedAt.Valid {
		item.FinishedAt = &finishedAt.Time
	}
	return &item, nil
}

func scanAccountProbeSample(scanner interface{ Scan(...any) error }) (*service.AccountProbeSample, error) {
	var sample service.AccountProbeSample
	var firstToken sql.NullInt64
	var validationEvidence []byte
	if err := scanner.Scan(
		&sample.ID, &sample.RunID, &sample.RequestIndex, &sample.Type, &sample.Label, &sample.Status, &sample.Model,
		&sample.APIKeyFingerprint, &sample.APIKeyMasked, &sample.UpstreamEndpoint, &sample.HTTPStatus,
		&sample.DurationMillis, &firstToken,
		&sample.InputTokens, &sample.OutputTokens, &sample.TotalTokens,
		&sample.OutputText, &validationEvidence,
		&sample.RequestPrompt, &sample.RequestBody, &sample.ResponseBody,
		&sample.ErrorCode, &sample.ErrorMessage, &sample.CreatedAt,
	); err != nil {
		return nil, err
	}
	if firstToken.Valid {
		v := int(firstToken.Int64)
		sample.FirstTokenMillis = &v
	}
	if len(validationEvidence) > 0 {
		if err := json.Unmarshal(validationEvidence, &sample.ValidationEvidence); err != nil {
			return nil, fmt.Errorf("unmarshal account probe validation evidence: %w", err)
		}
	}
	return &sample, nil
}

func normalizeAccountProbeReportRepositoryFilter(filter service.AccountProbeReportFilter) service.AccountProbeReportFilter {
	if strings.TrimSpace(filter.Sort) == "" {
		filter.Sort = service.AccountProbeReportSortCreatedAt
	}
	if strings.TrimSpace(filter.Order) == "" {
		filter.Order = "desc"
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 || filter.PageSize > 100 {
		filter.PageSize = 20
	}
	return filter
}

func buildAccountProbeReportWhere(filter service.AccountProbeReportFilter) (string, []any) {
	clauses := []string{"1=1"}
	args := make([]any, 0, 8)
	add := func(clause string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(clause, len(args)))
	}
	if filter.AccountID > 0 {
		add("r.account_id = $%d", filter.AccountID)
	}
	if strings.TrimSpace(filter.Status) != "" {
		add("r.status = $%d", strings.TrimSpace(filter.Status))
	}
	if strings.TrimSpace(filter.Profile) != "" {
		add("r.mode = $%d", strings.TrimSpace(filter.Profile))
	} else {
		add("COALESCE(r.mode,'') <> $%d", service.AccountProbeProfileModelValidation)
	}
	if strings.TrimSpace(filter.RequestMode) != "" {
		add("COALESCE(r.request_mode,'non_stream') = $%d", strings.TrimSpace(filter.RequestMode))
	}
	if strings.TrimSpace(filter.Model) != "" {
		add("r.model ILIKE $%d", "%"+strings.TrimSpace(filter.Model)+"%")
	}
	if strings.TrimSpace(filter.Keyword) != "" {
		value := "%" + strings.TrimSpace(filter.Keyword) + "%"
		start := len(args) + 1
		args = append(args, value, value, value, value)
		clauses = append(clauses, fmt.Sprintf("(a.name ILIKE $%d OR r.model ILIKE $%d OR COALESCE(r.summary,'') ILIKE $%d OR COALESCE(r.error_message,'') ILIKE $%d)", start, start+1, start+2, start+3))
	}
	if filter.From != nil {
		add("r.created_at >= $%d", *filter.From)
	}
	if filter.To != nil {
		add("r.created_at <= $%d", *filter.To)
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func accountProbeReportOrderBy(filter service.AccountProbeReportFilter) string {
	direction := "DESC"
	if strings.EqualFold(filter.Order, "asc") {
		direction = "ASC"
	}
	switch filter.Sort {
	case service.AccountProbeReportSortSuccessRate:
		return fmt.Sprintf("(CASE WHEN r.request_count > 0 THEN r.success_count::float / r.request_count ELSE 0 END) %s, r.created_at DESC", direction)
	case service.AccountProbeReportSortAvgLatency:
		return "r.avg_ms " + direction + ", r.created_at DESC"
	case service.AccountProbeReportSortP95Latency:
		return "r.p95_ms " + direction + ", r.created_at DESC"
	case service.AccountProbeReportSortFirstToken:
		return "r.first_token_ms " + direction + " NULLS LAST, r.created_at DESC"
	case service.AccountProbeReportSortTotalTokens:
		return "r.total_tokens " + direction + ", r.created_at DESC"
	default:
		return "r.created_at " + direction
	}
}

func formatPostgresDuration(d time.Duration) string {
	if d <= 0 {
		return "0 seconds"
	}
	seconds := int64(d.Round(time.Second) / time.Second)
	return fmt.Sprintf("%d seconds", seconds)
}

var _ service.AccountProbeRepository = (*accountProbeRepository)(nil)
