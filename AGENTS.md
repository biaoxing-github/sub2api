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

- 固定顺序：先提交，再给镜像生成递增版本，再构建/推送不可变版本镜像，再启动候选容器验证，最后切流量；部署必须使用已提交的 HEAD，不用未提交工作树构建线上镜像。
- 提交前执行 `git status --short --branch`、`git diff --cached --check`，只暂存同一功能范围的文件；提交 subject 使用中文，并按功能拆分多个 commit。
- 禁止把生产或当前 Codex 网关连接到可变标签 `sub2api:multi-key-local`。该标签只能作为“最新本地构建源”打不可变候选版本，不能直接作为发布镜像、compose 运行镜像或网关主链路镜像。
- 每次构建或候选验证必须使用递增且不可复用的版本标签，推荐格式：`sub2api:vYYYYMMDD.N-<12位commit>`，例如 `sub2api:v20260607.1-79653902afb2`。同一个版本标签禁止覆盖重建；候选失败、需要重建或需要重新验证时必须递增 `N`。
- 2026-06-07 首次按此规则判定：当前可用/回滚镜像为 `sub2api:v0134-absorption-check`；最新本地构建源为 `sub2api:multi-key-local`；已从该源生成首个不可变候选 `sub2api:v20260607.1-666797082235`。该候选 green 启动失败于 `157_user_platform_quotas.sql` 迁移 checksum mismatch，禁止复用此版本号；下一次候选版本从 `v20260607.2-...` 或新的日期序号继续递增。
- 构建前记录当前线上镜像：`docker inspect sub2api --format 'ConfigImage={{.Config.Image}} ImageID={{.Image}} Health={{if .State.Health}}{{.State.Health.Status}}{{end}}'`，并把它作为本次回滚目标。
- 构建应用镜像使用干净提交归档或干净工作树，PowerShell 示例：`$commit = git rev-parse --short=12 HEAD; $version = "v$(Get-Date -Format yyyyMMdd).1-$commit"; git archive --format=tar HEAD | docker build --pull=false -t "sub2api:$version" --label "org.opencontainers.image.version=$version" --label "org.opencontainers.image.revision=$commit" --build-arg COMMIT=$commit -`。
- 如需推送镜像，只推送不可变版本标签：`docker tag "sub2api:$version" "<registry>/sub2api:$version"; docker push "<registry>/sub2api:$version"`；禁止把 `latest`、`multi-key-local` 或已在线标签作为发布入口。
- 部署前必须先拉起候选容器，候选容器不得占用线上 8080 端口，建议使用同一 Docker network、独立容器名和本机候选端口：`sub2api-candidate` + `127.0.0.1:18080:8080`。候选容器必须使用新版本镜像，不能复用线上容器名 `sub2api`。
- 候选容器验证至少包括：`http://127.0.0.1:18080/health`、管理前端静态资源 200、受保护管理 API 未登录返回 401、容器 `Health=healthy` 持续至少 60 秒、最近日志过滤 `panic`、`fatal`、`migration.*fail`、`checksum`、`pq:`、`bind`、`listen`、`rebuild failed`。
- 零停机目标形态：在 `D:\sub2api-deploy` 增加固定入口代理（nginx / caddy / traefik 均可），只有代理发布 `0.0.0.0:8080`；`sub2api-blue` 与 `sub2api-green` 只在内部网络暴露 8080。候选容器通过健康检查后，更新代理 upstream 并 reload 代理完成切流。
- 固定入口代理已正式接管 `8080`：`D:\sub2api-deploy\docker-compose.proxy.yml` 的 Compose 项目名为 `sub2api-entry-proxy`，容器名为 `sub2api-proxy`，当前绑定 `0.0.0.0:${SUB2API_PROXY_PUBLIC_PORT:-8080}:8080` 和 `127.0.0.1:${SUB2API_PROXY_PORT:-18081}:8080`；upstream 文件为 `D:\sub2api-deploy\proxy\upstreams\active.conf`，当前指向 `sub2api-green:8080`。后续正常请求优先连接 `8080`，`18081` 作为本机旁路验证入口保留。
- 当前 green 候选 Compose 为 `D:\sub2api-deploy\docker-compose.green.yml`，Compose 项目名 `sub2api-green-candidate`，容器名 `sub2api-green`，默认绑定 `127.0.0.1:${SUB2API_GREEN_PORT:-18082}:8080`。候选失败时先停止 `sub2api-green`，保持 `proxy/upstreams/active.conf` 指向上一可用容器。
- 代理切流推荐流程：保留当前 active 容器（blue），用新版本启动 idle 容器（green）并验证；验证通过后将代理 upstream 从 blue 改到 green，执行代理热重载；公网 `http://127.0.0.1:8080/health` 和管理端冒烟通过后，继续保留 blue 至少一个观察窗口用于秒级回滚。
- 没有入口代理时，不允许宣称零停机切流。只能先用候选端口验证新版本，再在明确接受短暂停机的前提下，把 compose 的应用镜像变量改为新版本并仅重建 `sub2api`；如果当前网关承载 Codex 工作流，必须优先补代理，不走直接重建。
- 回滚规则：切流后若新版本出现健康检查失败、panic/fatal、迁移错误、端口绑定失败、网关请求大面积失败，立即把代理 upstream 切回上一版本容器；没有代理时用构建前记录的旧镜像标签重建 `sub2api`。
- 禁止在常规应用部署中执行 `docker compose up -d`、`docker compose restart` 或任何带 `postgres`、`redis` 服务名的重建/重启命令。
- PostgreSQL 和 Redis 不随应用部署重启。只有新增 SQL 迁移或必须修复线上 schema/data 时，才对运行中的 PostgreSQL 执行定向 SQL 写入；写入完成后只切换/重建应用容器，仍不重启 `sub2api-postgres` 和 `sub2api-redis`。
- 新 SQL 写入必须先确认迁移文件或 schema diff，再通过运行中的数据库容器执行幂等 SQL，例如 `docker exec -i sub2api-postgres psql -U <user> -d <db> -v ON_ERROR_STOP=1`；不得通过重建 PostgreSQL 容器来触发 schema 修复。

## 验证约定

- 后端变更至少跑相关 Go 聚焦测试；影响 handler/service 公共路径时补编译切片。
- 前端类型或组件变更至少跑 `npm run typecheck` 或相关 Vitest。
- 无法执行的验证必须记录到 `verification.md`。
