-- 分组自定义 /v1/models 列表配置。零值为禁用状态，与 Go 结构体默认值保持一致。
ALTER TABLE groups
ADD COLUMN IF NOT EXISTS models_list_config JSONB NOT NULL DEFAULT '{}'::jsonb;

COMMENT ON COLUMN groups.models_list_config IS '自定义 /v1/models 展示列表配置；仅影响模型列表响应，不影响调度';
