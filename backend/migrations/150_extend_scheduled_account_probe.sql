ALTER TABLE scheduled_test_plans
    ADD COLUMN IF NOT EXISTS task_type VARCHAR(32) NOT NULL DEFAULT 'account_test';

ALTER TABLE scheduled_test_plans
    ADD COLUMN IF NOT EXISTS probe_mode VARCHAR(20) NOT NULL DEFAULT 'standard';

ALTER TABLE scheduled_test_plans
    ADD COLUMN IF NOT EXISTS probe_request_mode VARCHAR(20) NOT NULL DEFAULT 'stream';

ALTER TABLE scheduled_test_plans
    ADD COLUMN IF NOT EXISTS probe_codex_stability BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE scheduled_test_plans
    ADD COLUMN IF NOT EXISTS probe_long_context BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE scheduled_test_results
    ADD COLUMN IF NOT EXISTS account_probe_run_id BIGINT REFERENCES account_probe_runs(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_str_account_probe_run_id
    ON scheduled_test_results(account_probe_run_id)
    WHERE account_probe_run_id IS NOT NULL;
