ALTER TABLE newapi_checkin_sites
    ADD COLUMN IF NOT EXISTS provider VARCHAR(32) NOT NULL DEFAULT 'newapi';

ALTER TABLE newapi_checkin_account_balances
    ADD COLUMN IF NOT EXISTS provider_data JSONB NOT NULL DEFAULT '{}'::jsonb;
