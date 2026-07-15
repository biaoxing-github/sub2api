CREATE TABLE IF NOT EXISTS newapi_checkin_api_key_cache (
    account_id BIGINT PRIMARY KEY REFERENCES newapi_checkin_accounts(id) ON DELETE CASCADE,
    summary JSONB NOT NULL DEFAULT '{}'::jsonb,
    match_keys JSONB NOT NULL DEFAULT '{}'::jsonb,
    refreshed_at TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
