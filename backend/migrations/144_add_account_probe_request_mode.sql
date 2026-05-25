ALTER TABLE account_probe_runs
    ADD COLUMN IF NOT EXISTS request_mode VARCHAR(20) NOT NULL DEFAULT 'non_stream';
