CREATE TABLE IF NOT EXISTS account_batch_test_runs (
    id BIGSERIAL PRIMARY KEY,
    status VARCHAR(20) NOT NULL DEFAULT 'running',
    model_id VARCHAR(100) NOT NULL DEFAULT '',
    platform VARCHAR(32) NOT NULL DEFAULT '',
    status_filter VARCHAR(32) NOT NULL DEFAULT '',
    search TEXT NOT NULL DEFAULT '',
    concurrency INTEGER NOT NULL DEFAULT 2,
    limit_count INTEGER NOT NULL DEFAULT 500,
    total_count INTEGER NOT NULL DEFAULT 0,
    success_count INTEGER NOT NULL DEFAULT 0,
    failed_count INTEGER NOT NULL DEFAULT 0,
    unauthorized_count INTEGER NOT NULL DEFAULT 0,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS account_batch_test_items (
    id BIGSERIAL PRIMARY KEY,
    run_id BIGINT NOT NULL REFERENCES account_batch_test_runs(id) ON DELETE CASCADE,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    account_name VARCHAR(255) NOT NULL DEFAULT '',
    platform VARCHAR(32) NOT NULL DEFAULT '',
    account_type VARCHAR(32) NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    category VARCHAR(32) NOT NULL DEFAULT '',
    message TEXT,
    error_message TEXT,
    latency_ms INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_account_batch_test_runs_created
    ON account_batch_test_runs(created_at DESC);

CREATE INDEX IF NOT EXISTS idx_account_batch_test_runs_status
    ON account_batch_test_runs(status);

CREATE INDEX IF NOT EXISTS idx_account_batch_test_items_run
    ON account_batch_test_items(run_id, id);

CREATE INDEX IF NOT EXISTS idx_account_batch_test_items_account_created
    ON account_batch_test_items(account_id, created_at DESC);
