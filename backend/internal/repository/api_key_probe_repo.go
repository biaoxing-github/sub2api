package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type apiKeyProbeRepository struct {
	db *sql.DB
}

func NewAPIKeyProbeRepository(db *sql.DB) service.APIKeyProbeRepository {
	return &apiKeyProbeRepository{db: db}
}

func (r *apiKeyProbeRepository) CreateProbeRun(ctx context.Context, run *service.APIKeyProbeResult) error {
	if run == nil {
		return errors.New("api key probe run is nil")
	}
	query := `
INSERT INTO api_key_probe_runs (
  user_id, api_key_id, mode, status, model, codex_stability, long_context,
  request_count, success_count, failure_count,
  input_tokens, output_tokens, total_tokens, total_cost, actual_cost,
  p50_ms, p95_ms, avg_ms, max_ms, first_token_ms,
  estimated_input_tokens_min, estimated_input_tokens_max,
  estimated_output_tokens_min, estimated_output_tokens_max,
  error_message, summary, created_at, started_at, finished_at
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,
  $8,$9,$10,
  $11,$12,$13,$14,$15,
  $16,$17,$18,$19,$20,
  $21,$22,$23,$24,
  NULLIF($25,''), NULLIF($26,''), $27,$28,$29
) RETURNING id, created_at`
	return r.db.QueryRowContext(ctx, query,
		run.UserID, run.APIKeyID, run.Profile, run.Status, run.Model, run.IncludeCodexStability, run.IncludeLongContext,
		run.RequestCount, run.SuccessCount, run.FailureCount,
		run.Usage.InputTokens, run.Usage.OutputTokens, run.Usage.TotalTokens, run.Usage.TotalCost, run.Usage.ActualCost,
		run.Latency.P50Millis, run.Latency.P95Millis, run.Latency.AvgMillis, run.Latency.MaxMillis, run.FirstTokenMillis,
		run.Estimate.InputTokensMin, run.Estimate.InputTokensMax, run.Estimate.OutputTokensMin, run.Estimate.OutputTokensMax,
		run.ErrorMessage, run.Summary, run.CreatedAt, run.StartedAt, run.FinishedAt,
	).Scan(&run.ID, &run.CreatedAt)
}

func (r *apiKeyProbeRepository) UpdateProbeRun(ctx context.Context, run *service.APIKeyProbeResult) error {
	if run == nil {
		return errors.New("api key probe run is nil")
	}
	_, err := r.db.ExecContext(ctx, `
UPDATE api_key_probe_runs SET
  status = $2,
  success_count = $3,
  failure_count = $4,
  input_tokens = $5,
  output_tokens = $6,
  total_tokens = $7,
  total_cost = $8,
  actual_cost = $9,
  p50_ms = $10,
  p95_ms = $11,
  avg_ms = $12,
  max_ms = $13,
  first_token_ms = $14,
  error_message = NULLIF($15,''),
  summary = NULLIF($16,''),
  finished_at = $17
WHERE id = $1`,
		run.ID,
		run.Status,
		run.SuccessCount,
		run.FailureCount,
		run.Usage.InputTokens,
		run.Usage.OutputTokens,
		run.Usage.TotalTokens,
		run.Usage.TotalCost,
		run.Usage.ActualCost,
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

func (r *apiKeyProbeRepository) SaveSample(ctx context.Context, sample service.APIKeyProbeSample) error {
	query := `
INSERT INTO api_key_probe_samples (
  run_id, request_index, sample_type, label, request_id, status,
  usage_log_id, usage_log_status, usage_log_message,
  account_id, account_name, upstream_endpoint,
  duration_ms, first_token_ms,
  input_tokens, output_tokens, total_tokens, total_cost, actual_cost,
  error_code, error_message, created_at
) VALUES (
  $1,$2,$3,$4,$5,$6,
  NULLIF($7,0),$8,NULLIF($9,''),
  NULLIF($10,0),NULLIF($11,''),NULLIF($12,''),
  $13,$14,
  $15,$16,$17,$18,$19,
  NULLIF($20,''),NULLIF($21,''),$22
)`
	_, err := r.db.ExecContext(ctx, query,
		sample.RunID, sample.RequestIndex, sample.Type, sample.Label, sample.RequestID, sample.Status,
		sample.UsageLogID, sample.UsageLogStatus, sample.UsageLogMessage,
		sample.AccountID, sample.AccountName, sample.UpstreamEndpoint,
		sample.DurationMillis, sample.FirstTokenMillis,
		sample.InputTokens, sample.OutputTokens, sample.TotalTokens, sample.TotalCost, sample.ActualCost,
		sample.ErrorCode, sample.ErrorMessage, sample.CreatedAt,
	)
	return err
}

func (r *apiKeyProbeRepository) ListProbeRuns(ctx context.Context, filter service.APIKeyProbeHistoryFilter) ([]service.APIKeyProbeResult, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT id, user_id, api_key_id, mode, status, model, codex_stability, long_context,
       request_count, success_count, failure_count,
       input_tokens, output_tokens, total_tokens, total_cost, actual_cost,
       p50_ms, p95_ms, avg_ms, max_ms, first_token_ms,
       estimated_input_tokens_min, estimated_input_tokens_max,
       estimated_output_tokens_min, estimated_output_tokens_max,
       error_message, summary, created_at, started_at, finished_at
FROM api_key_probe_runs
WHERE user_id = $1 AND api_key_id = $2
ORDER BY created_at DESC
LIMIT $3`, filter.UserID, filter.APIKeyID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []service.APIKeyProbeResult
	for rows.Next() {
		run, err := scanProbeRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *run)
	}
	return out, rows.Err()
}

func (r *apiKeyProbeRepository) GetProbeRun(ctx context.Context, userID, apiKeyID, runID int64) (*service.APIKeyProbeResult, error) {
	run, err := scanProbeRun(r.db.QueryRowContext(ctx, `
SELECT id, user_id, api_key_id, mode, status, model, codex_stability, long_context,
       request_count, success_count, failure_count,
       input_tokens, output_tokens, total_tokens, total_cost, actual_cost,
       p50_ms, p95_ms, avg_ms, max_ms, first_token_ms,
       estimated_input_tokens_min, estimated_input_tokens_max,
       estimated_output_tokens_min, estimated_output_tokens_max,
       error_message, summary, created_at, started_at, finished_at
FROM api_key_probe_runs
WHERE id = $1 AND user_id = $2 AND api_key_id = $3`, runID, userID, apiKeyID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrAPIKeyNotFound
		}
		return nil, err
	}
	return run, nil
}

func (r *apiKeyProbeRepository) ListProbeSamples(ctx context.Context, runID int64) ([]service.APIKeyProbeSample, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, run_id, request_index, sample_type, label, request_id, status,
       COALESCE(usage_log_id,0), usage_log_status, COALESCE(usage_log_message,''),
       COALESCE(account_id,0), COALESCE(account_name,''), COALESCE(upstream_endpoint,''),
       duration_ms, first_token_ms,
       input_tokens, output_tokens, total_tokens, total_cost, actual_cost,
       COALESCE(error_code,''), COALESCE(error_message,''), created_at
FROM api_key_probe_samples
WHERE run_id = $1
ORDER BY request_index ASC`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []service.APIKeyProbeSample
	for rows.Next() {
		sample, err := scanProbeSample(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *sample)
	}
	return out, rows.Err()
}

func (r *apiKeyProbeRepository) FindUsageByRequestID(ctx context.Context, apiKeyID int64, requestID string) (*service.UsageLog, error) {
	normalized := strings.TrimSpace(requestID)
	if normalized == "" {
		return nil, sql.ErrNoRows
	}
	row := r.db.QueryRowContext(ctx, `
SELECT
  ul.id, ul.user_id, ul.api_key_id, ul.account_id, ul.request_id,
  COALESCE(ul.model,''), COALESCE(ul.requested_model,''), COALESCE(ul.input_tokens,0), COALESCE(ul.output_tokens,0),
  COALESCE(ul.cache_creation_tokens,0), COALESCE(ul.cache_read_tokens,0),
  COALESCE(ul.total_cost,0), COALESCE(ul.actual_cost,0),
  ul.duration_ms, ul.first_token_ms, ul.upstream_endpoint,
  COALESCE(a.name,''), ul.created_at
FROM usage_logs ul
LEFT JOIN accounts a ON a.id = ul.account_id
WHERE ul.api_key_id = $1 AND ul.request_id = $2
ORDER BY ul.created_at DESC
LIMIT 1`, apiKeyID, normalized)
	var (
		log              service.UsageLog
		durationMs       sql.NullInt64
		firstTokenMs     sql.NullInt64
		upstreamEndpoint sql.NullString
		accountName      string
	)
	if err := row.Scan(
		&log.ID, &log.UserID, &log.APIKeyID, &log.AccountID, &log.RequestID,
		&log.Model, &log.RequestedModel, &log.InputTokens, &log.OutputTokens,
		&log.CacheCreationTokens, &log.CacheReadTokens,
		&log.TotalCost, &log.ActualCost,
		&durationMs, &firstTokenMs, &upstreamEndpoint,
		&accountName, &log.CreatedAt,
	); err != nil {
		return nil, err
	}
	if durationMs.Valid {
		v := int(durationMs.Int64)
		log.DurationMs = &v
	}
	if firstTokenMs.Valid {
		v := int(firstTokenMs.Int64)
		log.FirstTokenMs = &v
	}
	if upstreamEndpoint.Valid {
		log.UpstreamEndpoint = &upstreamEndpoint.String
	}
	if accountName != "" {
		log.Account = &service.Account{ID: log.AccountID, Name: accountName}
	}
	return &log, nil
}

func scanProbeRun(scanner interface{ Scan(...any) error }) (*service.APIKeyProbeResult, error) {
	var run service.APIKeyProbeResult
	var firstToken sql.NullInt64
	var errorMessage, summary sql.NullString
	var startedAt, finishedAt sql.NullTime
	err := scanner.Scan(
		&run.ID, &run.UserID, &run.APIKeyID, &run.Profile, &run.Status, &run.Model, &run.IncludeCodexStability, &run.IncludeLongContext,
		&run.RequestCount, &run.SuccessCount, &run.FailureCount,
		&run.Usage.InputTokens, &run.Usage.OutputTokens, &run.Usage.TotalTokens, &run.Usage.TotalCost, &run.Usage.ActualCost,
		&run.Latency.P50Millis, &run.Latency.P95Millis, &run.Latency.AvgMillis, &run.Latency.MaxMillis, &firstToken,
		&run.Estimate.InputTokensMin, &run.Estimate.InputTokensMax, &run.Estimate.OutputTokensMin, &run.Estimate.OutputTokensMax,
		&errorMessage, &summary, &run.CreatedAt, &startedAt, &finishedAt,
	)
	if err != nil {
		return nil, err
	}
	run.Estimate.Requests = run.RequestCount
	run.Estimate.TotalTokensMin = run.Estimate.InputTokensMin + run.Estimate.OutputTokensMin
	run.Estimate.TotalTokensMax = run.Estimate.InputTokensMax + run.Estimate.OutputTokensMax
	run.TotalTokens = run.Usage.TotalTokens
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

func scanProbeSample(scanner interface{ Scan(...any) error }) (*service.APIKeyProbeSample, error) {
	var sample service.APIKeyProbeSample
	var firstToken sql.NullInt64
	if err := scanner.Scan(
		&sample.ID, &sample.RunID, &sample.RequestIndex, &sample.Type, &sample.Label, &sample.RequestID, &sample.Status,
		&sample.UsageLogID, &sample.UsageLogStatus, &sample.UsageLogMessage,
		&sample.AccountID, &sample.AccountName, &sample.UpstreamEndpoint,
		&sample.DurationMillis, &firstToken,
		&sample.InputTokens, &sample.OutputTokens, &sample.TotalTokens, &sample.TotalCost, &sample.ActualCost,
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

var _ service.APIKeyProbeRepository = (*apiKeyProbeRepository)(nil)
