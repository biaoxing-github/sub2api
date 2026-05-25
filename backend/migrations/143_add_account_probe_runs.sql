CREATE TABLE IF NOT EXISTS account_probe_runs (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    mode VARCHAR(20) NOT NULL DEFAULT 'standard',
    status VARCHAR(20) NOT NULL DEFAULT 'running',
    model VARCHAR(100) NOT NULL DEFAULT '',
    codex_stability BOOLEAN NOT NULL DEFAULT FALSE,
    long_context BOOLEAN NOT NULL DEFAULT FALSE,
    request_count INTEGER NOT NULL DEFAULT 0,
    success_count INTEGER NOT NULL DEFAULT 0,
    failure_count INTEGER NOT NULL DEFAULT 0,
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    total_tokens INTEGER NOT NULL DEFAULT 0,
    p50_ms INTEGER NOT NULL DEFAULT 0,
    p95_ms INTEGER NOT NULL DEFAULT 0,
    avg_ms INTEGER NOT NULL DEFAULT 0,
    max_ms INTEGER NOT NULL DEFAULT 0,
    first_token_ms INTEGER,
    estimated_input_tokens_min INTEGER NOT NULL DEFAULT 0,
    estimated_input_tokens_max INTEGER NOT NULL DEFAULT 0,
    estimated_output_tokens_min INTEGER NOT NULL DEFAULT 0,
    estimated_output_tokens_max INTEGER NOT NULL DEFAULT 0,
    error_message TEXT,
    summary TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS account_probe_samples (
    id BIGSERIAL PRIMARY KEY,
    run_id BIGINT NOT NULL REFERENCES account_probe_runs(id) ON DELETE CASCADE,
    request_index INTEGER NOT NULL DEFAULT 0,
    sample_type VARCHAR(32) NOT NULL DEFAULT '',
    label VARCHAR(100) NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT '',
    model VARCHAR(100) NOT NULL DEFAULT '',
    api_key_fingerprint VARCHAR(96) NOT NULL DEFAULT '',
    api_key_masked VARCHAR(32) NOT NULL DEFAULT '',
    upstream_endpoint TEXT,
    http_status INTEGER NOT NULL DEFAULT 0,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    first_token_ms INTEGER,
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    total_tokens INTEGER NOT NULL DEFAULT 0,
    error_code VARCHAR(80),
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_account_probe_runs_account_created
    ON account_probe_runs(account_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_account_probe_samples_run_index
    ON account_probe_samples(run_id, request_index);
