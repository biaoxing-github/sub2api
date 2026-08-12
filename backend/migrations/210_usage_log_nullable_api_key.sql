-- 管理员手动上游探测使用 JWT 身份，不对应任意普通 API Key。
ALTER TABLE usage_logs ALTER COLUMN api_key_id DROP NOT NULL;
