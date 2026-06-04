ALTER TABLE account_probe_runs
  ADD COLUMN IF NOT EXISTS probe_source VARCHAR(32) NOT NULL DEFAULT 'self_validation';

CREATE INDEX IF NOT EXISTS idx_account_probe_runs_source_created
  ON account_probe_runs(probe_source, created_at DESC);
