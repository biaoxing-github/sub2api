-- 记录 Responses 转发链路的分阶段耗时，历史用量保持 NULL。
ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS latency_stages JSONB;
