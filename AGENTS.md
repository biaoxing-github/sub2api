# AGENTS.md — sub2api 项目操作手册

本文件从 `C:\Users\27404\.codex\AGENTS.md` 派生，仅记录 `D:\sub2api-src` 的项目级约束。

## 开发约定

- Windows 下使用 PowerShell；源码检索使用 `Select-String` / `Get-ChildItem`，不使用 `rg`。
- 分析调用关系优先使用 CodeGraph。遇到传输中断先重试并检查 `.codegraph/daemon.log`、`.codegraph/daemon.pid`；连续 3 次失败且日志证明不可恢复后，才降级到 PowerShell 检索并记录原因。
- 修改优先使用 `apply_patch`，保持小步、局部和可回滚；只处理当前任务范围，不清理无关脏文件。
- Go 后端保持既有分层：handler 负责 HTTP 编排，service 负责业务规则，repository 负责 SQL。前端保持 Vue 3 + TypeScript + Tailwind 风格，公共字段同步更新 `frontend/src/types/index.ts`。
- OpenAI 上游错误统一复用 `ClassifyUpstreamError`、`OpenAIPathHealthTracker` 和 Ops 诊断字段。非 API_KEY 批量体检属于低优先级后台流量，不得阻塞真实 `/responses` 请求。

## 提交与版本

- 发布顺序固定为：验证改动 → 提交 → 确定不可变版本 → 从已提交 HEAD 构建 → 部署 idle 候选 → 候选验证 → 代理切流 → 线上观察 → 回填发布记录。禁止用未提交工作树构建线上镜像。
- 提交前执行 `git status --short --branch` 和 `git diff --cached --check`，只暂存同一功能范围；提交 subject 使用中文并按功能拆分。
- 每次更新、构建或发布前重新读取最新语义化 tag：`$baseVersion = git tag --sort=-v:refname | Select-Object -First 1`。主版本、管理端 `current_version` 和镜像版本线必须以该 tag 为唯一基线，禁止复用历史固定版本号。
- 不可变镜像使用 `sub2api:$baseVersion.N`。同时检查本机镜像、`docs/releases/` 和 active/idle 镜像，取本版本线已使用最大 `N + 1`；构建过、验证过或记录为失败候选的标签均不得覆盖。禁止日期标签、`latest` 和 `multi-key-local` 作为发布入口。
- 从 `git archive HEAD` 构建，并传入 `VERSION=$baseVersion`、`IMAGE_VERSION=$version`、`COMMIT=$commit`；同时写入 OCI version/revision 标签。部署前必须核验二进制主版本、`image_version`、commit 与预期完全一致，不一致时顺延新小版本重新构建。
- Dockerfile 使用国内 Alpine 源 `https://mirrors.tuna.tsinghua.edu.cn/alpine`。镜像如需推送，只推不可变版本标签。
- `/admin/system/version` 和前端 store 必须提供 `image_version`。`VersionBadge` 按接口 `current_version` 展示恰好一个 `v` 前缀的主版本，镜像小版本只在下拉框展示；每次发布均验证两者。

## 蓝绿部署

- 部署配置位于 `D:\sub2api-deploy`：代理 `sub2api-proxy` 对外使用 `8080`、本机旁路使用 `18081`；upstream 文件为 `proxy\upstreams\active.conf`。
- green 使用 `docker-compose.green.yml`、容器 `sub2api-green`、候选端口 `18082`；blue 使用 `docker-compose.blue.yml`、容器 `sub2api-blue`、候选端口 `18083`。两者使用同一 Docker network。
- 构建前读取 `active.conf` 并 inspect active 容器，记录镜像、ImageID、健康状态和回滚目标。新版本只能部署到相反颜色的 idle 容器，严禁重建 active。
- 候选必须使用新不可变镜像，并验证：`/health` 200、前端静态资源 200、受保护管理 API 未登录 401、`POST /v1/responses` 未登录 401、`Health=healthy` 持续至少 60 秒、restart 0。
- 候选日志精确过滤：`panic`、`fatal`、`migration.*fail`、`checksum`、`pq:`、`bind:`、`address already in use`、`listen tcp`、`rebuild failed`。命中后不得切流；排除启动期事件后必须重新建立独立 60 秒干净窗口。
- 候选通过后备份并修改 `active.conf`，依次执行 `docker exec sub2api-proxy nginx -t` 和 `docker exec sub2api-proxy nginx -s reload`。切流后验证 `8080`、`18081` 和当前候选端口，重复至少 60 秒健康/日志观察，并比对关键静态资源哈希。
- 保留旧 active 容器作为回滚目标。新版本出现健康、迁移、端口或网关故障时，立即把 upstream 切回旧颜色并重新检查、reload nginx。

## 数据保护

- 常规应用部署只重建 idle 应用容器，不执行无服务名的 `docker compose up -d` 或 `restart`，不得重启 PostgreSQL、Redis 或 active 应用。
- 只有新增迁移或明确的数据修复才写数据库。先核对 migration/schema diff，再通过运行中的 PostgreSQL 执行幂等 SQL，并使用 `ON_ERROR_STOP=1`；不得通过重建数据库容器触发迁移。

## 验证与记录

- 后端变更至少运行相关 Go 聚焦测试；影响 handler/service 公共路径时补编译切片。前端变更至少运行相关 Vitest、`pnpm run typecheck`，发布前运行生产构建和 `git diff --check`。
- 无法执行的验证及风险写入 `verification.md`。每次会话结束追加 `docs/feature_list.jsonl` 和 `docs/process_list.jsonl`，不得覆盖历史记录。
- 每次发布新增 `docs/releases/$baseVersion.N.md`，记录日期、执行者 `Devil`、功能与用户影响、涉及模块、Git 提交、镜像与颜色、验证证据、回滚目标和遗留风险；切流后回填最终状态。过程 JSONL 不能替代发布记录。
