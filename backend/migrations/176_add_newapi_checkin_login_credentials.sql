ALTER TABLE newapi_checkin_accounts
    ADD COLUMN IF NOT EXISTS login_username VARCHAR(255) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS login_password TEXT NOT NULL DEFAULT '';
