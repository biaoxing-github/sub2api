# 操作日志

## 2026-07-12 Devil

- 对比 `v0.1.150..v0.1.151`，按本地架构吸收 9 个非合并提交的生产行为。
- 使用 TDD 验证 cache-write 显式零值、request_type alias 和 bare GPT-5.6 展示。
- 执行后端与前端全量验证，修正 151 行为变化对应的旧断言。
- 核对发布前 active green v0.1.150.2 与 idle blue v0.1.150.1 均 healthy。

## 2026-07-13 Devil

- 通过 Obsidian Local REST、项目流水、Memory 和 CodeGraph 注入项目约束，确认采用方案 1 的提交簇选择性融合边界。
- 在隔离工作树 `D:\sub2api-src-152`、分支 `codex/merge-v0.1.152` 中完成 13 个上游修复簇的 cherry-pick 与冲突融合，原工作树保持不变。
- 使用 `apply_patch` 融合 Grok CLI 头部、OAuth 默认地址、Responses 请求清理、Grok fallback 计费和本地前端组件模式；使用 `gofmt` 格式化 Go 文件。
- 使用 Go 聚焦/包级测试、Vue typecheck、Vitest 聚焦/全量测试验证；记录 handler 6 个既有失败，不扩大本轮修改范围。
- 更新主版本唯一来源 `backend/cmd/server/VERSION` 为 `v0.1.152`；核对 alpha/search migration、按次计费测试和 VersionBadge 单 `v` 行为仍保留。
- 本轮未执行镜像构建、部署、Git push、registry push 或真实认证上游请求。
