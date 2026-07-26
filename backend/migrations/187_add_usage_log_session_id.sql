-- 持久化客户端显式提供的会话标识，用于按会话关联用量记录。
-- 字段保持可空且无默认值，缺失或非法的会话标识继续保存为 NULL。
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS session_id VARCHAR(255);
