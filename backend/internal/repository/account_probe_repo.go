package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type accountProbeRepository struct {
	db *sql.DB
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
  account_id, mode, status, model, codex_stability, long_context,
  request_count, success_count, failure_count,
  input_tokens, output_tokens, total_tokens,
  p50_ms, p95_ms, avg_ms, max_ms, first_token_ms,
  estimated_input_tokens_min, estimated_input_tokens_max,
  estimated_output_tokens_min, estimated_output_tokens_max,
  error_message, summary, created_at, started_at, finished_at
) VALUES (
  $1,$2,$3,$4,$5,$6,
  $7,$8,$9,
  $10,$11,$12,
  $13,$14,$15,$16,$17,
  $18,$19,$20,$21,
  NULLIF($22,''), NULLIF($23,''), $24,$25,$26
) RETURNING id, created_at`,
		run.AccountID, run.Profile, run.Status, run.Model, run.IncludeCodexStability, run.IncludeLongContext,
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

func (r *accountProbeRepository) SaveAccountProbeSample(ctx context.Context, sample service.AccountProbeSample) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO account_probe_samples (
  run_id, request_index, sample_type, label, status, model,
  api_key_fingerprint, api_key_masked, upstream_endpoint, http_status,
  duration_ms, first_token_ms,
  input_tokens, output_tokens, total_tokens,
  error_code, error_message, created_at
) VALUES (
  $1,$2,$3,$4,$5,$6,
  $7,$8,NULLIF($9,''),$10,
  $11,$12,
  $13,$14,$15,
  NULLIF($16,''),NULLIF($17,''),$18
)`,
		sample.RunID, sample.RequestIndex, sample.Type, sample.Label, sample.Status, sample.Model,
		sample.APIKeyFingerprint, sample.APIKeyMasked, sample.UpstreamEndpoint, sample.HTTPStatus,
		sample.DurationMillis, sample.FirstTokenMillis,
		sample.InputTokens, sample.OutputTokens, sample.TotalTokens,
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

	var out []service.AccountProbeResult
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

func (r *accountProbeRepository) ListAccountProbeSamples(ctx context.Context, runID int64) ([]service.AccountProbeSample, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, run_id, request_index, sample_type, label, status, model,
       api_key_fingerprint, api_key_masked, COALESCE(upstream_endpoint,''), http_status,
       duration_ms, first_token_ms,
       input_tokens, output_tokens, total_tokens,
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
SELECT id, account_id, mode, status, model, codex_stability, long_context,
       request_count, success_count, failure_count,
       input_tokens, output_tokens, total_tokens,
       p50_ms, p95_ms, avg_ms, max_ms, first_token_ms,
       estimated_input_tokens_min, estimated_input_tokens_max,
       estimated_output_tokens_min, estimated_output_tokens_max,
       error_message, summary, created_at, started_at, finished_at
`

func scanAccountProbeRun(scanner interface{ Scan(...any) error }) (*service.AccountProbeResult, error) {
	var run service.AccountProbeResult
	var firstToken sql.NullInt64
	var errorMessage, summary sql.NullString
	var startedAt, finishedAt sql.NullTime
	if err := scanner.Scan(
		&run.ID, &run.AccountID, &run.Profile, &run.Status, &run.Model, &run.IncludeCodexStability, &run.IncludeLongContext,
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

func scanAccountProbeSample(scanner interface{ Scan(...any) error }) (*service.AccountProbeSample, error) {
	var sample service.AccountProbeSample
	var firstToken sql.NullInt64
	if err := scanner.Scan(
		&sample.ID, &sample.RunID, &sample.RequestIndex, &sample.Type, &sample.Label, &sample.Status, &sample.Model,
		&sample.APIKeyFingerprint, &sample.APIKeyMasked, &sample.UpstreamEndpoint, &sample.HTTPStatus,
		&sample.DurationMillis, &firstToken,
		&sample.InputTokens, &sample.OutputTokens, &sample.TotalTokens,
		&sample.ErrorCode, &sample.ErrorMessage, &sample.CreatedAt,
	); err != nil {
		return nil, err
	}
	if firstToken.Valid {
		v := int(firstToken.Int64)
		sample.FirstTokenMillis = &v
	}
	return &sample, nil
}

var _ service.AccountProbeRepository = (*accountProbeRepository)(nil)
