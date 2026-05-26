package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *opsRepository) ListRequestDetails(ctx context.Context, filter *service.OpsRequestDetailFilter) ([]*service.OpsRequestDetail, int64, error) {
	if r == nil || r.db == nil {
		return nil, 0, fmt.Errorf("nil ops repository")
	}

	page, pageSize, startTime, endTime := filter.Normalize()
	offset := (page - 1) * pageSize

	conditions := make([]string, 0, 16)
	args := make([]any, 0, 24)

	// Placeholders $1/$2 reserved for time window inside the CTE.
	args = append(args, startTime.UTC(), endTime.UTC())

	addCondition := func(condition string, values ...any) {
		conditions = append(conditions, condition)
		args = append(args, values...)
	}

	if filter != nil {
		if kind := strings.TrimSpace(strings.ToLower(filter.Kind)); kind != "" && kind != "all" {
			if kind != string(service.OpsRequestKindSuccess) && kind != string(service.OpsRequestKindError) {
				return nil, 0, fmt.Errorf("invalid kind")
			}
			addCondition(fmt.Sprintf("kind = $%d", len(args)+1), kind)
		}

		if platform := strings.TrimSpace(strings.ToLower(filter.Platform)); platform != "" {
			addCondition(fmt.Sprintf("platform = $%d", len(args)+1), platform)
		}
		if filter.GroupID != nil && *filter.GroupID > 0 {
			addCondition(fmt.Sprintf("group_id = $%d", len(args)+1), *filter.GroupID)
		}

		if filter.UserID != nil && *filter.UserID > 0 {
			addCondition(fmt.Sprintf("user_id = $%d", len(args)+1), *filter.UserID)
		}
		if filter.APIKeyID != nil && *filter.APIKeyID > 0 {
			addCondition(fmt.Sprintf("api_key_id = $%d", len(args)+1), *filter.APIKeyID)
		}
		if filter.AccountID != nil && *filter.AccountID > 0 {
			addCondition(fmt.Sprintf("account_id = $%d", len(args)+1), *filter.AccountID)
		}

		if model := strings.TrimSpace(filter.Model); model != "" {
			addCondition(fmt.Sprintf("model = $%d", len(args)+1), model)
		}
		if requestID := strings.TrimSpace(filter.RequestID); requestID != "" {
			addCondition(fmt.Sprintf("request_id = $%d", len(args)+1), requestID)
		}
		if q := strings.TrimSpace(filter.Query); q != "" {
			like := "%" + strings.ToLower(q) + "%"
			startIdx := len(args) + 1
			addCondition(
				fmt.Sprintf("(LOWER(COALESCE(request_id,'')) LIKE $%d OR LOWER(COALESCE(model,'')) LIKE $%d OR LOWER(COALESCE(message,'')) LIKE $%d)",
					startIdx, startIdx+1, startIdx+2,
				),
				like, like, like,
			)
		}

		if filter.MinDurationMs != nil {
			addCondition(fmt.Sprintf("duration_ms >= $%d", len(args)+1), *filter.MinDurationMs)
		}
		if filter.MaxDurationMs != nil {
			addCondition(fmt.Sprintf("duration_ms <= $%d", len(args)+1), *filter.MaxDurationMs)
		}
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	cte := `
WITH combined AS (
  SELECT
    'success'::TEXT AS kind,
    ul.created_at AS created_at,
    ul.request_id AS request_id,
    COALESCE(NULLIF(g.platform, ''), NULLIF(a.platform, ''), '') AS platform,
    ul.model AS model,
    ul.duration_ms AS duration_ms,
    NULL::INT AS status_code,
    NULL::BIGINT AS error_id,
    NULL::TEXT AS phase,
    NULL::TEXT AS severity,
    NULL::TEXT AS message,
    ul.user_id AS user_id,
    ul.api_key_id AS api_key_id,
    ul.account_id AS account_id,
    ul.group_id AS group_id,
    ul.stream AS stream,
    a.name AS account_name,
    ul.requested_model AS requested_model,
    ul.upstream_endpoint AS upstream_endpoint,
    ul.input_tokens AS input_tokens,
    ul.output_tokens AS output_tokens,
    (COALESCE(ul.input_tokens, 0) + COALESCE(ul.output_tokens, 0) + COALESCE(ul.cache_creation_tokens, 0) + COALESCE(ul.cache_read_tokens, 0) + COALESCE(ul.image_output_tokens, 0))::INT AS total_tokens,
    ul.first_token_ms AS first_token_ms,
    ul.total_cost::TEXT AS total_cost,
    ul.actual_cost::TEXT AS actual_cost,
    NULL::INT AS upstream_status_code,
    NULL::TEXT AS upstream_error_message,
    NULL::TEXT AS upstream_error_detail,
    NULL::TEXT AS upstream_errors,
    NULL::BIGINT AS auth_latency_ms,
    NULL::BIGINT AS routing_latency_ms,
    NULL::BIGINT AS upstream_latency_ms,
    NULL::BIGINT AS response_latency_ms,
    NULL::BIGINT AS time_to_first_token_ms
  FROM usage_logs ul
  LEFT JOIN groups g ON g.id = ul.group_id
  LEFT JOIN accounts a ON a.id = ul.account_id
  WHERE ul.created_at >= $1 AND ul.created_at < $2

  UNION ALL

  SELECT
    'error'::TEXT AS kind,
    o.created_at AS created_at,
    COALESCE(NULLIF(o.request_id,''), NULLIF(o.client_request_id,''), '') AS request_id,
    COALESCE(NULLIF(o.platform, ''), NULLIF(g.platform, ''), NULLIF(a.platform, ''), '') AS platform,
    o.model AS model,
    o.duration_ms AS duration_ms,
    o.status_code AS status_code,
    o.id AS error_id,
    o.error_phase AS phase,
    o.severity AS severity,
    o.error_message AS message,
    o.user_id AS user_id,
    o.api_key_id AS api_key_id,
    o.account_id AS account_id,
    o.group_id AS group_id,
    o.stream AS stream,
    a.name AS account_name,
    o.model AS requested_model,
    o.upstream_endpoint AS upstream_endpoint,
    NULL::INT AS input_tokens,
    NULL::INT AS output_tokens,
    NULL::INT AS total_tokens,
    o.time_to_first_token_ms::INT AS first_token_ms,
    NULL::TEXT AS total_cost,
    NULL::TEXT AS actual_cost,
    o.upstream_status_code AS upstream_status_code,
    o.upstream_error_message AS upstream_error_message,
    o.upstream_error_detail AS upstream_error_detail,
    o.upstream_errors::TEXT AS upstream_errors,
    o.auth_latency_ms AS auth_latency_ms,
    o.routing_latency_ms AS routing_latency_ms,
    o.upstream_latency_ms AS upstream_latency_ms,
    o.response_latency_ms AS response_latency_ms,
    o.time_to_first_token_ms AS time_to_first_token_ms
  FROM ops_error_logs o
  LEFT JOIN groups g ON g.id = o.group_id
  LEFT JOIN accounts a ON a.id = o.account_id
  WHERE o.created_at >= $1 AND o.created_at < $2
    AND COALESCE(o.status_code, 0) >= 400
)
`

	countQuery := fmt.Sprintf(`%s SELECT COUNT(1) FROM combined %s`, cte, where)
	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		if err == sql.ErrNoRows {
			total = 0
		} else {
			return nil, 0, err
		}
	}

	sort := "ORDER BY created_at DESC"
	if filter != nil {
		switch strings.TrimSpace(strings.ToLower(filter.Sort)) {
		case "", "created_at_desc":
			// default
		case "duration_desc":
			sort = "ORDER BY duration_ms DESC NULLS LAST, created_at DESC"
		default:
			return nil, 0, fmt.Errorf("invalid sort")
		}
	}

	listQuery := fmt.Sprintf(`
%s
SELECT
  kind,
  created_at,
  request_id,
  platform,
  model,
  duration_ms,
  status_code,
  error_id,
  phase,
  severity,
  message,
  user_id,
  api_key_id,
  account_id,
  group_id,
  stream,
  account_name,
  requested_model,
  upstream_endpoint,
  input_tokens,
  output_tokens,
  total_tokens,
  first_token_ms,
  total_cost,
  actual_cost,
  upstream_status_code,
  upstream_error_message,
  upstream_error_detail,
  upstream_errors,
  auth_latency_ms,
  routing_latency_ms,
  upstream_latency_ms,
  response_latency_ms,
  time_to_first_token_ms
FROM combined
%s
%s
LIMIT $%d OFFSET $%d
`, cte, where, sort, len(args)+1, len(args)+2)

	listArgs := append(append([]any{}, args...), pageSize, offset)
	rows, err := r.db.QueryContext(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	toIntPtr := func(v sql.NullInt64) *int {
		if !v.Valid {
			return nil
		}
		i := int(v.Int64)
		return &i
	}
	toInt64Ptr := func(v sql.NullInt64) *int64 {
		if !v.Valid {
			return nil
		}
		i := v.Int64
		return &i
	}

	out := make([]*service.OpsRequestDetail, 0, pageSize)
	for rows.Next() {
		var (
			kind      string
			createdAt time.Time
			requestID sql.NullString
			platform  sql.NullString
			model     sql.NullString

			durationMs sql.NullInt64
			statusCode sql.NullInt64
			errorID    sql.NullInt64

			phase    sql.NullString
			severity sql.NullString
			message  sql.NullString

			userID    sql.NullInt64
			apiKeyID  sql.NullInt64
			accountID sql.NullInt64
			groupID   sql.NullInt64

			stream bool

			accountName          sql.NullString
			requestedModel       sql.NullString
			upstreamEndpoint     sql.NullString
			inputTokens          sql.NullInt64
			outputTokens         sql.NullInt64
			totalTokens          sql.NullInt64
			firstTokenMs         sql.NullInt64
			totalCost            sql.NullString
			actualCost           sql.NullString
			upstreamStatusCode   sql.NullInt64
			upstreamErrorMessage sql.NullString
			upstreamErrorDetail  sql.NullString
			upstreamErrorsJSON   sql.NullString
			authLatencyMs        sql.NullInt64
			routingLatencyMs     sql.NullInt64
			upstreamLatencyMs    sql.NullInt64
			responseLatencyMs    sql.NullInt64
			timeToFirstTokenMs   sql.NullInt64
		)

		if err := rows.Scan(
			&kind,
			&createdAt,
			&requestID,
			&platform,
			&model,
			&durationMs,
			&statusCode,
			&errorID,
			&phase,
			&severity,
			&message,
			&userID,
			&apiKeyID,
			&accountID,
			&groupID,
			&stream,
			&accountName,
			&requestedModel,
			&upstreamEndpoint,
			&inputTokens,
			&outputTokens,
			&totalTokens,
			&firstTokenMs,
			&totalCost,
			&actualCost,
			&upstreamStatusCode,
			&upstreamErrorMessage,
			&upstreamErrorDetail,
			&upstreamErrorsJSON,
			&authLatencyMs,
			&routingLatencyMs,
			&upstreamLatencyMs,
			&responseLatencyMs,
			&timeToFirstTokenMs,
		); err != nil {
			return nil, 0, err
		}

		item := &service.OpsRequestDetail{
			Kind:      service.OpsRequestKind(kind),
			CreatedAt: createdAt,
			RequestID: strings.TrimSpace(requestID.String),
			Platform:  strings.TrimSpace(platform.String),
			Model:     strings.TrimSpace(model.String),

			DurationMs: toIntPtr(durationMs),
			StatusCode: toIntPtr(statusCode),
			ErrorID:    toInt64Ptr(errorID),
			Phase:      phase.String,
			Severity:   severity.String,
			Message:    message.String,

			UserID:    toInt64Ptr(userID),
			APIKeyID:  toInt64Ptr(apiKeyID),
			AccountID: toInt64Ptr(accountID),
			GroupID:   toInt64Ptr(groupID),

			Stream: stream,

			AccountName:          strings.TrimSpace(accountName.String),
			RequestedModel:       strings.TrimSpace(requestedModel.String),
			UpstreamEndpoint:     strings.TrimSpace(upstreamEndpoint.String),
			InputTokens:          toIntPtr(inputTokens),
			OutputTokens:         toIntPtr(outputTokens),
			TotalTokens:          toIntPtr(totalTokens),
			FirstTokenMs:         toIntPtr(firstTokenMs),
			TotalCost:            strings.TrimSpace(totalCost.String),
			ActualCost:           strings.TrimSpace(actualCost.String),
			UpstreamStatusCode:   toIntPtr(upstreamStatusCode),
			UpstreamErrorMessage: strings.TrimSpace(upstreamErrorMessage.String),
			UpstreamErrorDetail:  strings.TrimSpace(upstreamErrorDetail.String),
			UpstreamErrors:       decodeOpsUpstreamErrorsJSON(upstreamErrorsJSON.String),
			AuthLatencyMs:        toInt64Ptr(authLatencyMs),
			RoutingLatencyMs:     toInt64Ptr(routingLatencyMs),
			UpstreamLatencyMs:    toInt64Ptr(upstreamLatencyMs),
			ResponseLatencyMs:    toInt64Ptr(responseLatencyMs),
			TimeToFirstTokenMs:   toInt64Ptr(timeToFirstTokenMs),
		}

		if item.Platform == "" {
			item.Platform = "unknown"
		}

		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return out, total, nil
}

func decodeOpsUpstreamErrorsJSON(raw string) []*service.OpsUpstreamErrorEvent {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil
	}
	var events []*service.OpsUpstreamErrorEvent
	if err := json.Unmarshal([]byte(raw), &events); err != nil {
		return nil
	}
	return events
}
