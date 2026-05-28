package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type openAIRequestSnapshotRepository struct {
	db *sql.DB
}

func NewOpenAIRequestSnapshotRepository(db *sql.DB) service.OpenAIRequestSnapshotRepository {
	if db == nil {
		return nil
	}
	return &openAIRequestSnapshotRepository{db: db}
}

func (r *openAIRequestSnapshotRepository) SaveOpenAIRequestSnapshot(ctx context.Context, snapshot *service.OpenAIRequestSnapshot) error {
	if r == nil || r.db == nil || snapshot == nil {
		return nil
	}
	inputTypeCounts, err := json.Marshal(snapshot.InputTypeCounts)
	if err != nil {
		return err
	}
	if len(inputTypeCounts) == 0 || string(inputTypeCounts) == "null" {
		inputTypeCounts = []byte(`{}`)
	}
	_, err = r.db.ExecContext(ctx, `
INSERT INTO openai_request_snapshots (
  request_id, client_request_id, session_hash, prompt_cache_key,
  user_id, api_key_id, account_id, group_id, model,
  request_body_sha256, request_body_bytes, request_body,
  context_migration_class, context_migration_reason, snapshot_replayable, replay_block_reason,
  has_previous_response_id, previous_response_id_kind, previous_response_id_len,
  input_item_count, message_item_count, function_call_output_count, custom_tool_output_count,
  reasoning_item_count, input_type_counts, created_at, expires_at
) VALUES (
  $1,$2,$3,$4,
  $5,$6,$7,$8,$9,
  $10,$11,$12::jsonb,
  $13,$14,$15,$16,
  $17,$18,$19,
  $20,$21,$22,$23,
  $24,$25::jsonb,$26,$27
)`,
		snapshot.RequestID, snapshot.ClientRequestID, snapshot.SessionHash, snapshot.PromptCacheKey,
		snapshot.UserID, snapshot.APIKeyID, snapshot.AccountID, snapshot.GroupID, snapshot.Model,
		snapshot.RequestBodySHA256, snapshot.RequestBodyBytes, []byte(snapshot.RequestBody),
		snapshot.ContextMigrationClass, snapshot.ContextMigrationReason, snapshot.SnapshotReplayable, snapshot.ReplayBlockReason,
		snapshot.HasPreviousResponseID, snapshot.PreviousResponseIDKind, snapshot.PreviousResponseIDLen,
		snapshot.InputItemCount, snapshot.MessageItemCount, snapshot.FunctionCallOutputCount, snapshot.CustomToolOutputCount,
		snapshot.ReasoningItemCount, inputTypeCounts, snapshot.CreatedAt, snapshot.ExpiresAt,
	)
	return err
}

func (r *openAIRequestSnapshotRepository) DeleteExpiredOpenAIRequestSnapshots(ctx context.Context, now time.Time, limit int) (int64, error) {
	if r == nil || r.db == nil {
		return 0, nil
	}
	if limit <= 0 {
		limit = 1000
	}
	res, err := r.db.ExecContext(ctx, `
WITH victims AS (
  SELECT id FROM openai_request_snapshots
  WHERE expires_at < $1
  ORDER BY expires_at
  LIMIT $2
)
DELETE FROM openai_request_snapshots s
USING victims
WHERE s.id = victims.id`, now, limit)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
