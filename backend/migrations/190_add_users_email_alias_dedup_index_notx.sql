-- 注册和发送验证码路径通过 repository.existsByEmailAliasWithClient，按去点后的邮箱
-- 表达式查询别名候选。为同一表达式建立索引，避免公开入口退化为 users 全表扫描。
-- text_pattern_ops 同时支持等值探针和 "local+%@domain" 前缀探针，不依赖数据库排序规则。
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_email_dot_stripped
    ON users ((REPLACE(LOWER(TRIM(email)), '.', '')) text_pattern_ops)
    WHERE deleted_at IS NULL;
