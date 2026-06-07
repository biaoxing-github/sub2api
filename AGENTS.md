# AGENTS.md — sub2api 项目操作手册

本文件从 `C:\Users\27404\.codex\AGENTS.md` 派生，用于 `D:\sub2api-src` 项目级约束。

## 项目约定

- Windows 环境优先使用 PowerShell 命令；检索源码时使用 `Select-String` / `Get-ChildItem`，避免使用 `rg`。
- CodeGraph MCP 出现一次 `Transport closed`、超时或连接断开时，只能视为瞬时传输故障；必须先重新调用 `codegraph_status` 或原 CodeGraph 工具重试，必要时检查 `.codegraph/daemon.log`、`.codegraph/daemon.pid` 并等待/重启 daemon 后再重试。只有连续 3 次 CodeGraph 调用失败且日志证明不可恢复时，才允许降级到 PowerShell 源码检索，并在交付中说明降级证据。
- 写代码优先 `apply_patch`，再用 `gofmt`、项目既有测试和类型检查验证。
- 后端 Go 代码保持现有 package 分层：handler 只做 HTTP 编排，service 承载业务规则，repository 承载 SQL。
- 前端保持现有 Vue 3 + TypeScript + Tailwind 风格，新增字段同步更新 `frontend/src/types/index.ts`。
- 涉及 OpenAI 上游稳定性时，统一优先复用 `ClassifyUpstreamError`、`OpenAIPathHealthTracker`、Ops 诊断字段，避免各处手写不同错误分类。
- 非 API_KEY 批量体检必须被视为低优先级后台流量；新增并发、限流、降级逻辑不得阻塞真实 `/responses` 请求。
- 每次会话结束时追加 `docs/feature_list.jsonl` 和 `docs/process_list.jsonl`，记录用户可见功能与过程状态。

## 提交、构建、部署、验证约定

- 固定顺序：先提交，再构建，再部署，最后验证；部署必须使用已提交的 HEAD，不用未提交工作树构建线上镜像。
- 提交前执行 `git status --short --branch`、`git diff --cached --check`，只暂存同一功能范围的文件；提交 subject 使用中文，并按功能拆分多个 commit。
- 构建应用镜像使用 PowerShell：`$commit = git rev-parse --short=12 HEAD; docker build --pull=false -t sub2api:multi-key-local --build-arg COMMIT=$commit .`。
- 部署应用只允许重建 `sub2api` 服务：`docker compose -f D:\sub2api-deploy\docker-compose.yml up -d --no-build --no-deps --force-recreate sub2api`。
- 禁止在常规应用部署中执行 `docker compose up -d`、`docker compose restart` 或任何带 `postgres`、`redis` 服务名的重建/重启命令。
- PostgreSQL 和 Redis 不随应用部署重启。只有新增 SQL 迁移或必须修复线上 schema/data 时，才对运行中的 PostgreSQL 执行定向 SQL 写入；写入完成后只重启/重建 `sub2api`，仍不重启 `sub2api-postgres` 和 `sub2api-redis`。
- 新 SQL 写入必须先确认迁移文件或 schema diff，再通过运行中的数据库容器执行幂等 SQL，例如 `docker exec -i sub2api-postgres psql -U <user> -d <db> -v ON_ERROR_STOP=1`；不得通过重建 PostgreSQL 容器来触发 schema 修复。
- 部署验证至少包括：`docker compose -f D:\sub2api-deploy\docker-compose.yml ps`、运行容器镜像 ID 与 `sub2api:multi-key-local` 对比、`http://127.0.0.1:8080/health`、管理前端或管理 API 冒烟、最近日志过滤 `panic`、`fatal`、`migration.*fail`、`checksum`、`pq:`、`rebuild failed`。

## 验证约定

- 后端变更至少跑相关 Go 聚焦测试；影响 handler/service 公共路径时补编译切片。
- 前端类型或组件变更至少跑 `npm run typecheck` 或相关 Vitest。
- 无法执行的验证必须记录到 `verification.md`。
