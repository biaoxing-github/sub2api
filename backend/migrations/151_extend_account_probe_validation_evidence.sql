ALTER TABLE account_probe_samples
    ADD COLUMN IF NOT EXISTS output_text TEXT NOT NULL DEFAULT '';

ALTER TABLE account_probe_samples
    ADD COLUMN IF NOT EXISTS validation_evidence JSONB NOT NULL DEFAULT '[]'::jsonb;
