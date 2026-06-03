ALTER TABLE account_probe_samples
    ADD COLUMN IF NOT EXISTS request_prompt TEXT NOT NULL DEFAULT '';

ALTER TABLE account_probe_samples
    ADD COLUMN IF NOT EXISTS request_body TEXT NOT NULL DEFAULT '';

ALTER TABLE account_probe_samples
    ADD COLUMN IF NOT EXISTS response_body TEXT NOT NULL DEFAULT '';
