-- 148_add_openai_request_snapshots.sql
-- 保存本地自用的 OpenAI 请求快照，用于切号/切 BaseURL 前判断上下文是否可安全重放。
CREATE TABLE IF NOT EXISTS openai_request_snapshots (
    id BIGSERIAL PRIMARY KEY,
    request_id TEXT NOT NULL,
    client_request_id TEXT NOT NULL DEFAULT '',
    session_hash TEXT NOT NULL DEFAULT '',
    prompt_cache_key TEXT NOT NULL DEFAULT '',
    user_id BIGINT NULL,
    api_key_id BIGINT NULL,
    account_id BIGINT NULL,
    group_id BIGINT NULL,
    model TEXT NOT NULL DEFAULT '',
    request_body_sha256 TEXT NOT NULL,
    request_body_bytes INTEGER NOT NULL DEFAULT 0,
    request_body JSONB NOT NULL,
    context_migration_class TEXT NOT NULL DEFAULT '',
    context_migration_reason TEXT NOT NULL DEFAULT '',
    snapshot_replayable BOOLEAN NOT NULL DEFAULT FALSE,
    replay_block_reason TEXT NOT NULL DEFAULT '',
    has_previous_response_id BOOLEAN NOT NULL DEFAULT FALSE,
    previous_response_id_kind TEXT NOT NULL DEFAULT '',
    previous_response_id_len INTEGER NOT NULL DEFAULT 0,
    input_item_count INTEGER NOT NULL DEFAULT 0,
    message_item_count INTEGER NOT NULL DEFAULT 0,
    function_call_output_count INTEGER NOT NULL DEFAULT 0,
    custom_tool_output_count INTEGER NOT NULL DEFAULT 0,
    reasoning_item_count INTEGER NOT NULL DEFAULT 0,
    input_type_counts JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_openai_request_snapshots_created_at ON openai_request_snapshots (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_openai_request_snapshots_expires_at ON openai_request_snapshots (expires_at);
CREATE INDEX IF NOT EXISTS idx_openai_request_snapshots_request_id ON openai_request_snapshots (request_id);
CREATE INDEX IF NOT EXISTS idx_openai_request_snapshots_session_hash ON openai_request_snapshots (session_hash) WHERE session_hash <> '';
