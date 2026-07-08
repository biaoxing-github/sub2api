CREATE TABLE IF NOT EXISTS newapi_checkin_settings (
    id SMALLINT PRIMARY KEY DEFAULT 1,
    default_checkin_path TEXT NOT NULL DEFAULT '/api/user/checkin',
    delay_between_checkins_sec INTEGER NOT NULL DEFAULT 0,
    route_switch_wait_sec INTEGER NOT NULL DEFAULT 0,
    notify_feishu BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT newapi_checkin_settings_singleton CHECK (id = 1)
);

CREATE TABLE IF NOT EXISTS newapi_checkin_sites (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    disabled_reason TEXT NOT NULL DEFAULT '',
    background_checkin_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    base_url TEXT NOT NULL DEFAULT '',
    checkin_path TEXT NOT NULL DEFAULT '',
    site_status_ok BOOLEAN NOT NULL DEFAULT TRUE,
    site_status_message TEXT NOT NULL DEFAULT '',
    quota_display_type VARCHAR(32) NOT NULL DEFAULT 'USD',
    quota_per_unit BIGINT NOT NULL DEFAULT 500000,
    custom_currency_symbol VARCHAR(32) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS newapi_checkin_accounts (
    id BIGSERIAL PRIMARY KEY,
    site_id BIGINT NOT NULL REFERENCES newapi_checkin_sites(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL DEFAULT '',
    username VARCHAR(255) NOT NULL DEFAULT '',
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    user_id VARCHAR(128) NOT NULL,
    access_key TEXT NOT NULL DEFAULT '',
    ip_profile VARCHAR(128) NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    disabled_reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (site_id, user_id)
);

CREATE TABLE IF NOT EXISTS newapi_checkin_account_balances (
    account_id BIGINT PRIMARY KEY REFERENCES newapi_checkin_accounts(id) ON DELETE CASCADE,
    status VARCHAR(64) NOT NULL DEFAULT '',
    message TEXT NOT NULL DEFAULT '',
    checkin_ok BOOLEAN,
    checkin_success BOOLEAN,
    checked_in_today BOOLEAN,
    checkin_status VARCHAR(64) NOT NULL DEFAULT '',
    checkin_status_tone VARCHAR(32) NOT NULL DEFAULT '',
    checkin_message TEXT NOT NULL DEFAULT '',
    checkin_date VARCHAR(10) NOT NULL DEFAULT '',
    quota BIGINT,
    quota_display TEXT NOT NULL DEFAULT '',
    used_quota BIGINT,
    used_quota_display TEXT NOT NULL DEFAULT '',
    quota_awarded BIGINT,
    quota_awarded_display TEXT NOT NULL DEFAULT '',
    last_refreshed_at TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS newapi_checkin_history (
    id BIGSERIAL PRIMARY KEY,
    date VARCHAR(10) NOT NULL,
    recorded_at TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT '',
    site VARCHAR(255) NOT NULL,
    account VARCHAR(255) NOT NULL DEFAULT '',
    user_id VARCHAR(128) NOT NULL,
    ip_profile VARCHAR(128) NOT NULL DEFAULT '',
    checkin_status VARCHAR(64) NOT NULL DEFAULT '',
    checkin_status_tone VARCHAR(32) NOT NULL DEFAULT '',
    checkin_message TEXT NOT NULL DEFAULT '',
    checkin_success BOOLEAN NOT NULL DEFAULT FALSE,
    checked_in_today BOOLEAN NOT NULL DEFAULT FALSE,
    quota_awarded BIGINT,
    quota_awarded_display TEXT NOT NULL DEFAULT '',
    quota_awarded_display_value DOUBLE PRECISION NOT NULL DEFAULT 0,
    balance BIGINT,
    balance_display TEXT NOT NULL DEFAULT '',
    balance_display_value DOUBLE PRECISION NOT NULL DEFAULT 0,
    used_quota BIGINT,
    used_display TEXT NOT NULL DEFAULT '',
    used_display_value DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (site, user_id, date)
);

CREATE TABLE IF NOT EXISTS newapi_checkin_monthly_records (
    id BIGSERIAL PRIMARY KEY,
    site VARCHAR(255) NOT NULL,
    user_id VARCHAR(128) NOT NULL,
    account_name VARCHAR(255) NOT NULL DEFAULT '',
    username VARCHAR(255) NOT NULL DEFAULT '',
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    ip_profile VARCHAR(128) NOT NULL DEFAULT '',
    month VARCHAR(7) NOT NULL,
    checkin_date VARCHAR(10) NOT NULL,
    quota_awarded BIGINT,
    quota_awarded_display TEXT NOT NULL DEFAULT '',
    quota_awarded_display_value DOUBLE PRECISION NOT NULL DEFAULT 0,
    fetched_at TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (site, user_id, checkin_date)
);

CREATE TABLE IF NOT EXISTS newapi_checkin_runs (
    id BIGSERIAL PRIMARY KEY,
    started_at TEXT NOT NULL DEFAULT '',
    ended_at TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT '',
    site_count INTEGER NOT NULL DEFAULT 0,
    task_count INTEGER NOT NULL DEFAULT 0,
    success_count INTEGER NOT NULL DEFAULT 0,
    already_done_count INTEGER NOT NULL DEFAULT 0,
    failed_count INTEGER NOT NULL DEFAULT 0,
    quota_awarded_total BIGINT NOT NULL DEFAULT 0,
    quota_awarded_display TEXT NOT NULL DEFAULT '',
    quota_display_symbol VARCHAR(32) NOT NULL DEFAULT '',
    summary_text TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS newapi_checkin_run_results (
    id BIGSERIAL PRIMARY KEY,
    run_id BIGINT NOT NULL REFERENCES newapi_checkin_runs(id) ON DELETE CASCADE,
    order_index INTEGER NOT NULL DEFAULT 0,
    site VARCHAR(255) NOT NULL,
    account VARCHAR(255) NOT NULL DEFAULT '',
    user_id VARCHAR(128) NOT NULL,
    ip_profile VARCHAR(128) NOT NULL DEFAULT '',
    ok BOOLEAN NOT NULL DEFAULT FALSE,
    success BOOLEAN NOT NULL DEFAULT FALSE,
    message TEXT NOT NULL DEFAULT '',
    checkin_date VARCHAR(10) NOT NULL DEFAULT '',
    quota_awarded BIGINT,
    quota_awarded_display TEXT NOT NULL DEFAULT '',
    quota_awarded_display_value DOUBLE PRECISION NOT NULL DEFAULT 0,
    remaining_quota BIGINT,
    remaining_quota_display TEXT NOT NULL DEFAULT '',
    used_quota BIGINT,
    used_quota_display TEXT NOT NULL DEFAULT '',
    username VARCHAR(255) NOT NULL DEFAULT '',
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    status VARCHAR(64) NOT NULL DEFAULT '',
    checkin_status VARCHAR(64) NOT NULL DEFAULT '',
    checkin_status_tone VARCHAR(32) NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_newapi_checkin_accounts_site
    ON newapi_checkin_accounts(site_id, id);

CREATE INDEX IF NOT EXISTS idx_newapi_checkin_history_date
    ON newapi_checkin_history(date, site, user_id);

CREATE INDEX IF NOT EXISTS idx_newapi_checkin_monthly_month
    ON newapi_checkin_monthly_records(month, site, user_id);

CREATE INDEX IF NOT EXISTS idx_newapi_checkin_runs_created
    ON newapi_checkin_runs(created_at DESC);

CREATE INDEX IF NOT EXISTS idx_newapi_checkin_run_results_run
    ON newapi_checkin_run_results(run_id, order_index);
