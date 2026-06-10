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
- 每次发版必须在 `docs/releases/` 下新增或更新对应版本日志（目录不存在则创建），文件名使用不可变版本号，例如 `docs/releases/v0.1.134.8.md`；版本日志必须包含日期、执行者 `Devil`、Git 提交、镜像标签、active/idle 颜色、更新内容、验证结果、回滚目标和遗留风险。`docs/feature_list.jsonl`、`docs/process_list.jsonl` 只作为过程流水，不能替代版本日志；切流完成后必须回填最终 active 颜色、线上镜像和验证结论。
- 每次发布或修改版本接口时，必须把不可变镜像小版本写到管理端可见位置；`/admin/system/version`、前端 store 至少要透出 `image_version`。`VersionBadge` 侧边栏按钮内只展示主版本号（`currentVersion`），镜像小版本（`imageVersion`）只在下拉框内展示，不能只写入日志或文档。
- 禁止把生产或当前 Codex 网关连接到可变标签 `sub2api:multi-key-local`。该标签只能作为“最新本地构建源”打不可变候选版本，不能直接作为发布镜像、compose 运行镜像或网关主链路镜像。
- 每次构建或候选验证必须使用 Git 版本线提供的不可变版本标签，当前版本线从 `sub2api:v0.1.134`（对应 Git tag `v0.1.134`）向后补丁延伸为 `sub2api:v0.1.134.N`，例如本轮使用 `sub2api:v0.1.134.1`。禁止另起 `sub2api:vYYYYMMDD.N-<12位commit>` 这类日期/自主序号；同一个版本标签一旦用于候选验证或推送，禁止覆盖重建。
- 2026-06-07 版本规则校正：当前可用/回滚镜像仍记录为 `sub2api:v0134-absorption-check`；最新本地构建源仍可保留 `sub2api:multi-key-local`，但只能作为构建源，不能作为发布入口。此前生成的 `sub2api:v20260607.*` 仅作为历史候选记录，后续候选/发布改回 Git 版本线，本轮目标版本为 `sub2api:v0.1.134.1`。
- 构建前记录当前 active 颜色和线上镜像：先读取 `D:\sub2api-deploy\proxy\upstreams\active.conf`，确认 active 是 `sub2api-blue` 还是 `sub2api-green`，再执行 `docker inspect <active-container> --format 'ConfigImage={{.Config.Image}} ImageID={{.Image}} Health={{if .State.Health}}{{.State.Health.Status}}{{end}}'`，并把这个 active 容器作为本次回滚目标。
- 构建应用镜像使用干净提交归档或干净工作树，PowerShell 示例：`$commit = git rev-parse --short=12 HEAD; $version = "v0.1.134.1"; git archive --format=tar HEAD | docker build --pull=false -t "sub2api:$version" --label "org.opencontainers.image.version=$version" --label "org.opencontainers.image.revision=$commit" --build-arg COMMIT=$commit -`。若不是本轮 134 补丁发布，先确认对应 Git tag 后再调整 `$version`，不要使用日期/自主序号。
- Docker 多阶段构建必须使用国内 Alpine `apk` 源，默认通过 Dockerfile 的 `ALPINE_APK_REPOSITORY=https://mirrors.tuna.tsinghua.edu.cn/alpine` 注入；遇到 `dl-cdn.alpinelinux.org`、Docker registry 或 `apk add` I/O/403 超时，不要改用未提交源码或覆盖已有不可变 tag，先重试 registry/base image pull，再使用国内 `apk` 源重建。
- 如需推送镜像，只推送不可变版本标签：`docker tag "sub2api:$version" "<registry>/sub2api:$version"; docker push "<registry>/sub2api:$version"`；禁止把 `latest`、`multi-key-local` 或已在线标签作为发布入口。
- 部署前必须先确认 active/idle 颜色，绝不能重建 active 容器：如果 `active.conf` 指向 `sub2api-green:8080`，新版本只能部署到 idle 的 `sub2api-blue`；如果 `active.conf` 指向 `sub2api-blue:8080`，新版本只能部署到 idle 的 `sub2api-green`。禁止对 active 颜色执行 `docker compose up -d --force-recreate`、`docker compose up -d` 或任何会覆盖 active 容器的命令。
- idle 容器必须使用同一 Docker network、独立容器名和本机候选端口：`sub2api-green` 固定使用 `127.0.0.1:${SUB2API_GREEN_PORT:-18082}:8080`，`sub2api-blue` 固定使用 `127.0.0.1:${SUB2API_BLUE_PORT:-18083}:8080`。候选容器必须使用新版本镜像，不能复用或覆盖当前 active 容器。
- idle 候选容器验证至少包括：对应本机候选端口 `/health`、管理前端静态资源 200、受保护管理 API 未登录返回 401、`POST /responses` 未登录返回 401 而不是 nginx 502、容器 `Health=healthy` 持续至少 60 秒、最近日志精确过滤 `panic`、`fatal`、`migration.*fail`、`checksum`、`pq:`、`bind:`、`address already in use`、`listen tcp`、`rebuild failed`。
- 零停机目标形态：在 `D:\sub2api-deploy` 增加固定入口代理（nginx / caddy / traefik 均可），只有代理发布 `0.0.0.0:8080`；`sub2api-blue` 与 `sub2api-green` 只在内部网络暴露 8080。候选容器通过健康检查后，更新代理 upstream 并 reload 代理完成切流。
- 固定入口代理已正式接管 `8080`：`D:\sub2api-deploy\docker-compose.proxy.yml` 的 Compose 项目名为 `sub2api-entry-proxy`，容器名为 `sub2api-proxy`，当前绑定 `0.0.0.0:${SUB2API_PROXY_PUBLIC_PORT:-8080}:8080` 和 `127.0.0.1:${SUB2API_PROXY_PORT:-18081}:8080`；upstream 文件为 `D:\sub2api-deploy\proxy\upstreams\active.conf`，当前指向 `sub2api-green:8080`。后续正常请求优先连接 `8080`，`18081` 作为本机旁路验证入口保留。
- 当前 green Compose 为 `D:\sub2api-deploy\docker-compose.green.yml`，Compose 项目名 `sub2api-green-candidate`，容器名 `sub2api-green`，默认绑定 `127.0.0.1:${SUB2API_GREEN_PORT:-18082}:8080`。当前 blue Compose 为 `D:\sub2api-deploy\docker-compose.blue.yml`，Compose 项目名 `sub2api-blue-candidate`，容器名 `sub2api-blue`，默认绑定 `127.0.0.1:${SUB2API_BLUE_PORT:-18083}:8080`。
- 代理切流强制流程：保留当前 active 容器运行；把新版本部署到相反颜色的 idle 容器；等 idle 容器完全启动、健康检查稳定、候选端口可访问且冒烟通过后，才把 `D:\sub2api-deploy\proxy\upstreams\active.conf` 从旧颜色改到新颜色并执行 `docker exec sub2api-proxy nginx -t` 与 `docker exec sub2api-proxy nginx -s reload`。切流后公网 `http://127.0.0.1:8080/health`、管理端冒烟和 `/responses` 未授权检查通过，再继续保留旧 active 至少一个观察窗口用于秒级回滚。
- 蓝绿交替示例：当前 `green` 是新版本、`blue` 是旧版本时，下一次发布必须把新版本部署到 `blue`，验证 `http://127.0.0.1:18083/health` 与完整冒烟后切流到 `blue`，并保留 `green`；再下一次发布则把新版本部署到 `green`，验证 `http://127.0.0.1:18082/health` 与完整冒烟后切流到 `green`，并保留 `blue`。
- 没有入口代理时，不允许宣称零停机切流。只能先用候选端口验证新版本，再在明确接受短暂停机的前提下，把 compose 的应用镜像变量改为新版本并仅重建 `sub2api`；如果当前网关承载 Codex 工作流，必须优先补代理，不走直接重建。
- 回滚规则：切流后若新版本出现健康检查失败、panic/fatal、迁移错误、端口绑定失败、网关请求大面积失败，立即把代理 upstream 切回上一版本容器；没有代理时用构建前记录的旧镜像标签重建 `sub2api`。
- 禁止在常规应用部署中执行 `docker compose up -d`、`docker compose restart` 或任何带 `postgres`、`redis` 服务名的重建/重启命令。
- PostgreSQL 和 Redis 不随应用部署重启。只有新增 SQL 迁移或必须修复线上 schema/data 时，才对运行中的 PostgreSQL 执行定向 SQL 写入；写入完成后只切换/重建应用容器，仍不重启 `sub2api-postgres` 和 `sub2api-redis`。
- 新 SQL 写入必须先确认迁移文件或 schema diff，再通过运行中的数据库容器执行幂等 SQL，例如 `docker exec -i sub2api-postgres psql -U <user> -d <db> -v ON_ERROR_STOP=1`；不得通过重建 PostgreSQL 容器来触发 schema 修复。

## 验证约定

- 后端变更至少跑相关 Go 聚焦测试；影响 handler/service 公共路径时补编译切片。
- 前端类型或组件变更至少跑 `npm run typecheck` 或相关 Vitest。
- 无法执行的验证必须记录到 `verification.md`。
