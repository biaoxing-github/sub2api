# Verification

日期：2026-05-28
执行者：Devil

## 结果

OpenAI 大请求体稳定性收紧已完成：请求快照不再把超过 2MB 的完整 body 写入 JSONB，显式开启的客户端 Debug 日志和网关调试日志按用户要求仍完整输出 body，快照服务启动后会立即清理过期记录，Docker compose 配置增加 `json-file` 日志轮转。

## 校验方式

- Go 聚焦测试覆盖快照省略、启动清理、上下文字节诊断、Debug 日志全量输出、网关调试日志全量输出和 413 映射。
- Docker compose 配置使用占位环境变量执行 `config --quiet` 校验。

## 校验结果

全部本轮聚焦测试通过。首次不带 `.env` 的 compose 校验因必填数据库/Redis 环境变量缺失失败，补占位变量后通过，属于模板预期行为。

## 风险

已修改当前本地部署的 `D:\sub2api-deploy\docker-compose.yml`，但 Docker 日志轮转对正在运行的旧容器不会仅靠普通 restart 生效，需要 recreate 对应容器后新的 log options 才会应用。

---

日期：2026-05-29
执行者：Devil

## 结果

OpenAI 稳定性五项计划继续收口：非 API_KEY 批量体检增加真实流量让路检查，统一错误分类和派生健康状态补充回归断言，前端账号状态徽章可展示派生健康诊断。

## 校验方式

- `go test -tags unit ./internal/handler/admin -run "TestAccountBatchTestNonAPIKey|TestAccountBatchTestLimiter" -count=1`
- `go test ./internal/service -run "TestClassifyUpstreamError|TestDeriveAccountHealthState|TestDiagnoseOpenAIRequestBody|TestOpenAIPathHealth|TestOpenAIGatewayServiceHandleErrorResponseMaps413|TestBuildOpenAIRequestSnapshot|TestOpenAIRequestSnapshotServiceRunsCleanupOnStart" -count=1`
- `go test ./internal/handler -run "TestOpenAIResponses_RejectsOversizedUpstreamBody|TestOpenAIMapUpstreamError_Maps413ToRequestEntityTooLarge" -count=1`
- `npm run test:run -- src/components/account/__tests__/AccountStatusIndicator.spec.ts`
- `npm run typecheck`
- `go test ./internal/service ./internal/handler ./internal/repository -run "^$"`
- `git diff --check`

## 校验结果

以上命令均通过。`git diff --check` 仅提示 `docs/feature_list.jsonl` 和 `docs/process_list.jsonl` 下次 Git 触碰时会 LF 转 CRLF，没有 whitespace error。

## 风险

批量体检当前按同账号真实并发/等待队列让路；如果多个账号共用同一个远端出口但不是同一 account_id，仍依赖现有全局/分组 limiter 和错误激增暂停，不会主动识别共享出口。

---

日期：2026-05-29
执行者：Devil

## 结果

foyeapi 余额刷新问题已定位并修复。本地运行日志显示账号 181 曾反复出现 `upstream_balance.login_failed`；数据库确认账号 181 为 foyeapi，余额应来自上游登录态 `/api/user/self`。真实调用验证显示 `/api/user/login` 间歇性返回 429，`/api/v1/auth/login` 返回 404，API-key 余额端点不能提供可用有限余额。代码已改为单 key 登录态余额优先，并缓存短时登录态，避免连续刷新反复打登录接口。

## 校验方式

- `go test ./internal/service -run "TestUpstreamBalanceService|TestLoginUpstream" -count=1`
- `go test ./internal/service ./internal/handler -run "^$"`
- 本地 Go 交叉编译 Linux release 二进制，并基于现有 `sub2api:multi-key-local` 镜像替换 `/app/sub2api`
- `docker compose --env-file D:\sub2api-deploy\.env -f D:\sub2api-deploy\docker-compose.yml up -d --no-deps sub2api`
- `Invoke-RestMethod http://127.0.0.1:8080/health`
- 等待余额预热后查询账号 181 的 `upstream_balance_*` 快照

## 校验结果

聚焦测试和编译切片均通过。新容器健康，`/app/sub2api --version` 显示 `commit: foye-balance-local`，`/health` 返回 `{"status":"ok"}`。预热后账号 181 快照为 `available=4.905164`、`used=0.094732`、`total=4.999896`、`ok_count=1`、`failed_count=0`，更新时间 `2026-05-29T09:09:57Z`，来源 `upstream-login:/api/user/self`。

## 风险

完整 Docker 多阶段构建首次失败，原因是本机 Docker Desktop 配置的镜像源对 `node:24-alpine`、`golang:1.26.3-alpine`、`alpine:3.21` 返回 403；本次部署改用本地 Go 编译加现有镜像替换二进制。若后续需要完整镜像重建，需要先修复 Docker registry mirror 或准备基础镜像缓存。

---

日期：2026-05-29
执行者：Devil

## 结果

mikuapi 的 `Input must be a list` 已定位到上游体检请求体形状问题。数据库 `account_probe_samples` 显示账号 13 最近失败样本来自 `https://mikuapi.org/v1/responses`，HTTP 400，错误消息为 `Input must be a list`；正常转发日志和 API key probe 样本未发现同名错误。代码中账号体检和 API Key 体检原先把 Responses `input` 发成字符串，已改为列表形态并附带 `instructions`。

## 校验方式

- `docker exec sub2api-postgres psql -U sub2api -d sub2api -c "... mikuapi ... Input must be a list ..."`
- `go test -tags unit ./internal/service -run "TestAccountProbeService_RunOpenAIAPIKeyPersistsSamples|TestHTTPAPIKeyProbeRunner_RunSampleUsesResponsesListInput" -count=1`
- `go test -tags unit ./internal/service -run "TestAccountProbeService|TestHTTPAPIKeyProbeRunner|TestAPIKeyProbeService" -count=1`
- `go test ./internal/service -run "TestClassifyUpstreamError|TestDeriveAccountHealthState|TestDiagnoseOpenAIRequestBody" -count=1`

## 校验结果

以上 Go 测试均通过。扩大账号体检测试时发现现有 path health 测试桩首 token 可能计为 0ms，已给该内存 SSE 响应测试桩增加 2ms 延迟，使断言稳定。

## 风险

本次修复的是体检请求体格式；mikuapi 仍可能存在自身慢响应、并发限制或 502，这类错误不会被这个修复消除，需要按上游可用性继续观察。

## 部署验证

已用本地 Go 交叉编译 Linux release 二进制，并基于现有 `sub2api:multi-key-local` 镜像替换 `/app/sub2api` 后重建容器。`docker exec sub2api /app/sub2api --version` 显示 `commit: miku-probe-local`，`Invoke-RestMethod http://127.0.0.1:8080/health` 返回 `ok`，`docker ps` 显示容器 `healthy`，最近 2 分钟启动日志未匹配 `panic/fatal/migration failed/listen tcp/error`。

---

日期：2026-05-29
执行者：Devil

## 结果

已按用户要求清空数据库中的所有上游测试报告历史数据。清理范围包括账号体检报告、API Key 体检报告、非 API_KEY 批量测试报告和定时测试结果；未清理账号、API key、用量日志、正常请求快照或定时测试计划配置。

## 校验方式

- 查询报告相关表和外键关系。
- 使用 `pg_dump -Fc --data-only` 备份报告表数据到 `.codex/db-backups/upstream-test-reports-20260529-174705.dump`。
- 在事务中执行 `TRUNCATE ... RESTART IDENTITY` 清理报告表。
- 查询清理后表数量。
- `Invoke-RestMethod http://127.0.0.1:8080/health`。

## 校验结果

清理前：`account_probe_runs=407`、`account_probe_samples=3150`、`api_key_probe_runs=0`、`api_key_probe_samples=0`、`account_batch_test_runs=53`、`account_batch_test_items=2268`、`scheduled_test_results=0`。清理后以上表均为 `0`。服务健康检查返回 `ok`。

## 风险

这是历史报告数据清理，不影响账号本身和真实用量日志。若需要恢复，可使用 `.codex/db-backups/upstream-test-reports-20260529-174705.dump` 进行恢复。

---

日期：2026-05-29
执行者：Devil

## 结果

funnyapi、encore、okcodex 的余额刷新 total-only 问题已定位并修复。真实上游调用确认三者均使用 `/api/v1/auth/login` 登录，余额位于 `/api/v1/auth/me` 的 `data.balance` 或 `data.user.balance`；旧逻辑只识别 `quota/used_quota`，随后被 `/api/v1/usage` 列表 `total` 覆盖，导致快照只有 total、没有 available。代码已补齐登录态 balance 字段解析，并在 `auth/me` 已拿到余额时不再用后续登录态余额探测覆盖。

## 校验方式

- `go test ./internal/service -run "TestUpstreamBalanceService|TestParseUpstreamBalanceResponse|TestParseNewAPIUserSelfResponse|TestParseNewAPIUsageResponse|TestParseSubscriptionSelfResponse|TestLoginUpstream" -count=1`
- `go test ./internal/service ./internal/handler -run "^$"`
- 本地 Go 交叉编译 Linux 二进制，并替换运行容器 `/app/sub2api`
- `Invoke-WebRequest http://127.0.0.1:8080/health`
- 使用临时 admin API key 触发账号 15、25、128 的真实刷新，完成后删除临时 key
- 查询 DB 中三个账号的 `upstream_balance_*` 快照

## 校验结果

聚焦测试和编译切片均通过。容器版本为 `Sub2API 0.1.130 (commit: balance-authme-local, built: 2026-05-29T09:38:08Z)`，`/health` 返回 `{"status":"ok"}`，Docker health 为 `healthy`。真实刷新后：okcodex `available=782.52006635`、funnyapi `available=33.0074149`、encore `available=1001.7470938`，三者均 `ok_count=1`、`failed_count=0`，来源均为 `upstream-login:/api/user/self`。

## 风险

当前保证的是 Sub2API、NewAPI token usage、billing/subscription、以及登录后 `auth/me`/`user.self` 的余额解析互不覆盖。若某个新上游把余额放在新的字段名或新的登录态路径，仍需按真实响应继续扩展解析规则。

---

日期：2026-06-12
执行者：Devil

## 结果

已完成 `sub2api:v0.1.134.37` 构建、idle blue 部署、候选验证、代理切流和公网入口验证。当前 active 为 `sub2api-blue:8080` / `sub2api:v0.1.134.37`，rollback 为 `sub2api-green` / `sub2api:v0.1.134.35`。

## 校验方式

- `go test -tags unit ./internal/handler -run "TestHandleFailoverError" -count=1`
- `go test -tags unit ./internal/service -run "TestAnthropicRequestBaseURLs|TestGatewayService_AnthropicAPIKeyPassthrough_ForwardSwitchesRequestBaseURLOn5xx|TestGatewayService_AnthropicAPIKeyPassthrough_CountTokensSwitchesRequestBaseURLOn5xx|TestHandleUpstreamError429_OpenAIAPIKey|TestRateLimitService_HandleUpstreamError_OpenAIAPIKey" -count=1`
- `npm run typecheck`
- `git diff --cached --check`
- `git archive --format=tar HEAD | docker build --pull=false -t sub2api:v0.1.134.37 ...`
- `docker run --rm --entrypoint /app/sub2api sub2api:v0.1.134.37 --version`
- `SUB2API_BLUE_IMAGE=sub2api:v0.1.134.37 docker compose -f D:\sub2api-deploy\docker-compose.blue.yml up -d`
- 候选 `18083` 与公网 `8080` 的 `/health`、`/`、6 个静态资源、未登录 `system/version`、`/responses`、`/v1/messages` 冒烟。
- `docker exec sub2api-proxy nginx -t` 与 `docker exec sub2api-proxy nginx -s reload`
- `docker logs --since 3m sub2api-blue` 与 `sub2api-proxy` 关键错误过滤。

## 校验结果

以上本地聚焦测试、TypeScript 类型检查、Docker 构建、候选端口冒烟、代理切流和 8080 冒烟均通过。`sub2api:v0.1.134.36` 因构建时未传 `IMAGE_VERSION` 被拦下，未进入候选验证；最终上线的 `v0.1.134.37` 二进制版本显示 `image: v0.1.134.37, commit: 4cc6624a4b63`。候选启动阶段出现 1 条 `openai_request_snapshot` 清理 `pq: canceling statement due to user request`，稳定窗口后与切流后 3 分钟内关键错误过滤命中 0。

## 风险

当前无管理端登录态，未登录访问 `/api/v1/admin/system/version` 返回 401；本轮通过二进制版本、镜像标签和路由代码确认 `image_version` 已写入运行镜像。`sub2api:v0.1.134.36` 为未部署本地构建产物，不应作为发布入口。旧单容器 `sub2api:v0134-absorption-check` 仍在重启中，但固定入口代理已经指向 blue/green 链路，不承载当前 8080 流量。

---

日期：2026-05-29
执行者：Devil

## 结果

上游体检报告已增加批量删除功能。后台新增 `DELETE /api/v1/admin/account-probe-runs`，支持一次删除最多 200 个报告 run，服务层会去重并拒绝空列表；仓储层在事务中删除对应 `account_probe_samples` 和非运行中的 `account_probe_runs`，running 状态报告会被跳过。前端上游体检报告页新增当前页多选、全选、批量删除按钮、确认弹窗和删除结果提示。

## 校验方式

- `go test -tags unit ./internal/handler/admin -run "TestAccountProbe" -count=1`
- `go test ./internal/repository -run "TestAccountProbeRepository" -count=1`
- `go test ./internal/service -run "TestAccountProbeService|TestHTTPAPIKeyProbeRunner|TestAPIKeyProbeService" -count=1`
- `go test ./internal/service ./internal/repository ./internal/handler/admin -run "^$"`
- `npm run test:run -- src/api/__tests__/admin.accounts.spec.ts src/views/admin/__tests__/AccountProbeReportsView.spec.ts`
- `npm run typecheck`
- `npm run build`
- 本地 Go 交叉编译 Linux release 二进制，并基于现有镜像刷新 `sub2api` 容器。
- `Invoke-RestMethod http://127.0.0.1:8080/health`
- `docker exec sub2api /app/sub2api --version`
- 浏览器打开 `http://127.0.0.1:8080/admin/account-probe-reports`

## 校验结果

以上 Go 测试、前端 Vitest、TypeScript 类型检查和前端构建均通过。容器版本显示 `Sub2API 0.1.130 (commit: probe-report-delete-local, built: 2026-05-29T10:44:00Z)`，`/health` 返回 `ok`，`docker ps` 显示 `sub2api` healthy，最近启动日志未匹配 `panic/fatal/migration failed/listen tcp/error`。浏览器访问上游体检报告页可正常加载前端并按未登录状态跳转到登录页。

## 风险

本次删除范围仅为账号上游体检报告，即 `account_probe_runs` 与 `account_probe_samples`；不会删除账号、API Key、用量日志、真实请求快照或非 API_KEY 批量测试报告。当前浏览器冒烟未做登录后的真实点击删除，因为本地会话未持有后台登录态；该交互已由 Vitest 覆盖。

---

日期：2026-05-29
执行者：Devil

## 结果

OpenAI OAuth 上游兼容开关已从单一 Cockpit Tools Toggle 扩展为三态选择：关闭、Cockpit Tools、Codex 直连。Cockpit Tools 沿用原请求头重建逻辑；Codex 直连模式会让 OAuth 账号强制走 HTTP/SSE，重建更接近 Codex Desktop 直连的请求头，并在上游 HTTP 调用时使用内置 Node.js 24.x TLS 指纹。配置层新增 `gateway.openai_oauth_compat_mode` / `GATEWAY_OPENAI_OAUTH_COMPAT_MODE`，旧 `openai_cockpit_tools_compat` / `GATEWAY_OPENAI_COCKPIT_TOOLS_COMPAT` 仍作为兼容映射。

出口 IP 检查结果：当前 Codex 执行环境直连、`127.0.0.1:7897` mihomo 代理、以及 `sub2api` 容器内访问都显示 Tokyo, JP / AS199524 G-Core Labs S.A.；本次检查没有看到美国住宅宽带出口。

## 校验方式

- `go test ./internal/config -run "TestLoadOpenAICockpitToolsCompatConfig" -count=1`
- `go test ./internal/service -run "TestOpenAIWSProtocolResolver|TestOpenAIBuildUpstreamRequestCockpitToolsCompatibilityHeaders|TestOpenAIPassthroughCockpitToolsCompatibilityHeaders|TestOpenAIBuildUpstreamRequestCodexDirectCompatibilityHeaders|TestOpenAIUpstreamCodexDirectUsesTLSProfile" -count=1`
- `go test -tags unit ./internal/service -run "TestSettingService_UpdateSettings_OpenAICockpitToolsCompatRefreshesGatewayConfig|TestSettingService_UpdateSettings_OpenAIOAuthCompatModeRefreshesGatewayConfig|TestSettingService_ParseSettings_OpenAICockpitToolsCompatFallsBackToConfigWhenMissing|TestSettingService_ParseSettings_OpenAIOAuthCompatModeTakesPrecedence" -count=1`
- `go test ./internal/handler/admin -run "TestSettingHandler_UpdateSettings_PersistsOpenAIOAuthCompatMode" -count=1`
- `npm run typecheck`
- `npm run test:run -- src/views/admin/__tests__/SettingsView.spec.ts`
- `npm run build`
- `go test ./internal/config ./internal/service ./internal/handler/admin -run "^$"`
- `curl.exe -s https://ipinfo.io/json`
- `curl.exe --proxy http://127.0.0.1:7897 -s --max-time 10 https://ipinfo.io/json`
- `docker exec sub2api sh -c "(wget -qO- https://ipinfo.io/json || curl -s https://ipinfo.io/json) 2>/dev/null"`
- `git diff --check`

## 校验结果

以上 Go 聚焦测试、后端编译切片、前端类型检查、SettingsView Vitest 和前端生产构建均通过。`npm run build` 只输出既有动态/静态混用和大 chunk 警告；`git diff --check` 无空白错误，仅提示若 Git 触碰 `deploy/.env.example`、`docs/*.jsonl`、`verification.md` 会发生 LF 到 CRLF 的换行转换。

## 风险

Codex 直连模式只能控制 sub2api 发往上游的 HTTP 请求头和 TLS 指纹，不能把出口网络变成 Codex Desktop 当前真实出口；实际 IP 仍取决于运行容器和本机代理。当前检查证据显示出口不是美国住宅宽带，需要切换 mihomo 节点或容器网络后再复测。

---

日期：2026-05-29
执行者：Devil

## 结果

已完成提交、构建、部署和验证。代码提交为 `f62e0fc8 增强上游体检与OpenAI兼容模式`，本地 Docker 镜像 `sub2api:multi-key-local` 使用该提交号构建，当前运行中的 `sub2api` 容器二进制版本显示 `Sub2API 0.1.130 (commit: f62e0fc8, built: 2026-05-29T12:42:36Z)`。

## 校验方式

- `go test ./internal/config -run "TestLoadOpenAICockpitToolsCompatConfig" -count=1`
- `go test ./internal/service -run "TestOpenAIWSProtocolResolver|TestOpenAIBuildUpstreamRequestCockpitToolsCompatibilityHeaders|TestOpenAIPassthroughCockpitToolsCompatibilityHeaders|TestOpenAIBuildUpstreamRequestCodexDirectCompatibilityHeaders|TestOpenAIUpstreamCodexDirectUsesTLSProfile|TestUpstreamBalanceService|TestParseUpstreamBalanceResponse|TestParseNewAPIUserSelfResponse|TestParseNewAPIUsageResponse|TestParseSubscriptionSelfResponse|TestLoginUpstream|TestAccountProbeService|TestHTTPAPIKeyProbeRunner|TestAPIKeyProbeService" -count=1`
- `go test ./internal/handler/admin -run "TestSettingHandler_UpdateSettings_PersistsOpenAIOAuthCompatMode|TestAccountProbe" -count=1`
- `go test ./internal/repository -run "TestAccountProbeRepository" -count=1`
- `npm run typecheck`
- `npm run test:run -- src/api/__tests__/admin.accounts.spec.ts src/views/admin/__tests__/AccountProbeReportsView.spec.ts src/views/admin/__tests__/SettingsView.spec.ts`
- `go test ./internal/config ./internal/service ./internal/handler/admin ./internal/repository -run "^$"`
- `npm run build`
- JSONL 解析 `docs/feature_list.jsonl` 与 `docs/process_list.jsonl`
- `git diff --cached --check`
- `docker build --pull=false --build-arg COMMIT=f62e0fc8 -t sub2api:multi-key-local <git archive build context>`
- `docker compose up -d --no-deps --force-recreate sub2api`
- `docker exec sub2api /app/sub2api --version`
- `Invoke-RestMethod http://127.0.0.1:8080/health`
- `Invoke-WebRequest http://127.0.0.1:8080/admin/settings`
- 浏览器打开 `http://127.0.0.1:8080/admin/settings`
- `docker logs --since 2m sub2api`

## 校验结果

发布前测试均通过；前端构建只保留既有 dynamic import / chunk size 警告。Docker build 第一次在 `go mod download` 遇到 `goproxy.cn` 的 `unexpected EOF`，使用相同 git archive 构建上下文重试后成功。容器重建后 `docker ps` 显示 `sub2api` healthy，`/health` 返回 `{"status":"ok"}`，`/admin/settings` 与 `/admin/account-probe-reports` 均返回 200；浏览器访问 `/admin/settings` 正常跳转到登录页 `redirect=/admin/settings`。最近 2 分钟容器日志未匹配 `panic/fatal/migration failed/listen tcp/error`。

## 开关位置

登录后台后进入 `管理后台 -> 系统设置 -> 网关转发 -> OpenAI OAuth 兼容模式`，可选择 `关闭`、`Cockpit Tools`、`Codex 直连`。保存后会写入后台设置并立即刷新运行时配置；也可以在部署配置中使用 `GATEWAY_OPENAI_OAUTH_COMPAT_MODE=codex_direct` 或 `gateway.openai_oauth_compat_mode: codex_direct` 作为启动默认值。

---

日期：2026-05-29
执行者：Devil

## 结果

Codex 直连模式新增独立开关 `openai_codex_direct_force_ws`。开启后，Codex Desktop 到 sub2api 仍是 HTTP/SSE 入站，sub2api 转发 OpenAI OAuth 非 API_KEY 账号时才允许从 HTTP 入站强制改走上游 WSv2；API_KEY 账号不会被该开关允许 HTTP 入站转 WS。全局 `openai_ws.force_http`、全局 WS disabled、账号级 force_http 仍优先回退 HTTP。

WSv2 上游头部在该模式下按 Codex Desktop 画像重建：默认 `User-Agent` 使用 `codexDesktopUserAgent`，`originator` 继承客户端或回退 `Codex Desktop`，`Session-Id` / `Thread-Id` 兼容映射到 WS 的 `session_id` / `conversation_id`，并保留 `chatgpt-account-id`。

## 校验方式

- `go test ./internal/config -run TestLoadOpenAICockpitToolsCompatConfig -count=1`
- `go test ./internal/service -run "TestOpenAIWSProtocolResolver|TestResolveOpenAIWSDecisionByClientTransport|TestOpenAIGatewayService_BuildOpenAIWSHeadersCodexDirectForceWS|TestOpenAIBuildUpstreamRequestCodexDirectCompatibilityHeaders" -count=1`
- `go test ./internal/handler/admin -run TestSettingHandler_UpdateSettings_PersistsOpenAIOAuthCompatMode -count=1`
- `go test -tags unit ./internal/service -run "TestSettingService_UpdateSettings_OpenAIOAuthCompatModeRefreshesGatewayConfig|TestSettingService_ParseSettings_OpenAIOAuthCompatModeTakesPrecedence" -count=1`
- `npm run typecheck`
- `npm run test:run -- SettingsView`
- `npm run build`
- JSONL 解析 `docs/feature_list.jsonl` 与 `docs/process_list.jsonl`
- `git diff --check`

## 校验结果

以上后端聚焦测试、前端类型检查、SettingsView Vitest、前端生产构建和 JSONL 解析均通过。`npm run build` 保留项目既有 dynamic import / chunk size 警告；`git diff --check` 无空白错误，仅提示 `deploy/.env.example`、`docs/*.jsonl` 和 `verification.md` 在当前 Windows 工作树里会发生 LF 到 CRLF 转换。

## 开关位置

登录后台后进入 `管理后台 -> 系统设置 -> 网关转发 -> OpenAI OAuth 兼容模式`，先选择 `Codex 直连`，下方会出现 `Codex 直连强制上游 WebSocket` 开关。保存后运行时立即生效。部署默认值也可用 `GATEWAY_OPENAI_CODEX_DIRECT_FORCE_WS=true` 或 `gateway.openai_codex_direct_force_ws: true`。
---

日期：2026-05-30
执行者：Devil

## 结果

已修复 Codex 直连强制上游 WSv2 后早期失败不切号的问题。问题根因是 `forwardOpenAIWSV2` 在握手 401 或首帧前 EOF/read_event 这类“尚未写下游响应”的失败中返回普通 `openAIWSFallbackError`，`Forward` 随后调用 `writeOpenAIWSFallbackErrorResponse` 写给客户端，handler 无法继续调度下一个账号。

现在 `Forward` 会先调用 service 侧 `newOpenAIWSFallbackFailoverError`：握手 401/403/429 和连接类 502 会在未写下游前转换为 `UpstreamFailoverError`。401/403/429 会同步走账号状态处理；首帧前 EOF 仅作为线路/连接失败切号，不直接把账号标记失效。`invalid_encrypted_content`、`previous_response_not_found`、`upgrade_required` 等保留原有写回语义，不扩大切号范围。

已构建并部署到本地 Docker 容器。当前运行版本：

- `Sub2API 0.1.130 (commit: ws-early-failover-local, built: 2026-05-30T01:13:56Z)`

## 校验方式

- `go test ./internal/service -run "TestOpenAIGatewayService_Forward_WSv2(Handshake401ReturnsFailoverBeforeWrite|EarlyReadEOFReturnsFailoverBeforeWrite)" -count=1`
- `go test ./internal/service -run "TestOpenAIGatewayService_Forward_WSv2.*(Handshake401|Handshake429|UsageLimit|CodexRateLimits|EarlyReadEOF)|TestOpenAIWSProtocolResolver|TestResolveOpenAIWSDecisionByClientTransport|TestOpenAIGatewayService_BuildOpenAIWSHeadersCodexDirectForceWS" -count=1`
- `go test -tags unit ./internal/service -run "TestSettingService_LoadRuntimeSettingsRefreshesGatewayConfig|TestSettingService_ParseSettings_OpenAIOAuthCompatModeTakesPrecedence|TestSettingService_UpdateSettings_OpenAIOAuthCompatModeRefreshesGatewayConfig" -count=1`
- `go test ./internal/service ./internal/handler -run "^$" -count=1`
- `git diff --check`
- `docker build --pull=false --build-arg COMMIT=ws-early-failover-local --build-arg DATE=<UTC> -t sub2api:multi-key-local .`
- `docker compose -f D:\sub2api-deploy\docker-compose.yml --env-file D:\sub2api-deploy\.env up -d --no-deps --force-recreate sub2api`
- `docker exec sub2api /app/sub2api --version`
- `Invoke-RestMethod http://127.0.0.1:8080/health`
- `docker ps --format "{{.Names}} {{.Image}} {{.Status}}"`
- `docker logs --since 10m sub2api`

## 校验结果

上述 Go 聚焦测试、编译切片和 `git diff --check` 均通过。Docker 镜像构建成功并已重建本地 `sub2api` 服务；`/health` 返回 `{"status":"ok"}`，`docker ps` 显示 `sub2api` healthy。部署后最近日志没有匹配 `panic`、`fatal`、`migration failed`、`listen tcp`、`fallback_error_response_written`、`handshake response status code 101 but got 401` 或 `read frame header: EOF`。

## 风险

本次修的是“上游 WS 早期失败后是否允许 handler 切换账号”。如果所有可用账号本身都没有额度或都返回 401，最终仍会由 handler 汇总后失败；但不会再被某一个 WS 早期失败账号卡住不切号。

---

日期：2026-05-30
执行者：Devil

## 结果

复核用户反馈的 WebSocket 握手 401。当前运行容器版本为 `Sub2API 0.1.130 (commit: ws-early-failover-local, built: 2026-05-30T01:13:56Z)`，设置表中 `openai_oauth_compat_mode=codex_direct` 且 `openai_codex_direct_force_ws=true`。

历史 `failed to WebSocket dial: expected handshake response status code 101 but got 401` 记录来自 2026-05-30 08:49-08:51，集中在 OpenAI OAuth 账号 295。该账号当前状态为 `error`、`schedulable=false`，错误信息是 `Token revoked (401): Encountered invalidated oauth token for user, failing request`，不再参与调度。

09:15 新容器启动后，`ops_error_logs` 中 OpenAI 401/WS 错误计数为 0。本地真实 `/responses` 烟测返回 200，`usage_logs` 显示当前请求调度到账号 293，`openai_ws_mode=true`、`request_type=3`，证明当前强制 WS 路径已生效并能正常返回。

## 校验方式

- 查询 `settings` 表确认 `codex_direct` 与强制 WS 开关。
- 查询 `ops_error_logs` 最近 OpenAI 401/WS 错误。
- 查询账号 293/295 的状态与调度可用性。
- 使用数据库中的本地 `codex` API key 调用 `http://localhost:8080/responses`，只输出状态与响应摘要。
- 查询 `usage_logs` 最近请求的 `openai_ws_mode`、`request_type`、`account_id`。

## 校验结果

本地 `/responses` 烟测返回 200；最近 WS 请求使用账号 293，`openai_ws_mode=true`。账号 295 已因上游撤销 token 置为不可调度。当前没有复现 raw WebSocket handshake 401 透给客户端。

---

日期：2026-05-30
执行者：Devil

## 结果

已修复非 API_KEY 批量体检里 `batch_paused` 直接把未测试账号写成失败的问题。

本次用户反馈账号 `MatthewThornton7001@outlook.com` / `#391`。数据库证据显示它在 run 9、run 10 被标为 `batch_paused`，原因不是账号 391 自己请求上游失败，而是同一批 `openai:oauth:group:2` 里先出现 3 个 429 usage_limit，触发了批量体检分组保护暂停。旧逻辑在暂停窗口内直接返回错误，导致后续账号未实际测试就被写成失败。

现在 limiter 在遇到上游错误爆发导致的分组暂停时，会等待暂停窗口结束后继续获取分组槽位并执行测试；只有上下文取消时才返回 `batch_paused` 错误。真实流量正在占用账号的保护分支仍保持原行为，避免后台体检抢真实请求。

已构建并部署到本地 Docker 容器。当前运行版本：

- `Sub2API 0.1.130 (commit: batch-limiter-wait-local, built: 2026-05-30T03:22:16Z)`

## 校验方式

- 查询 `account_batch_test_runs` 最近批量体检 run。
- 查询账号 391 在 `account_batch_test_items` 的历史 item。
- 查询 run 9、run 10 的全部 item 和 category 分布。
- `go test -tags unit ./internal/handler/admin -run "TestAccountBatchTestLimiter|TestAccountBatchTestNonAPIKey" -count=1`
- `go test ./internal/handler/admin -run "^$" -count=1`
- `go test -tags unit ./internal/handler/admin -count=1`
- `git diff --check`
- `docker build --pull=false --build-arg COMMIT=batch-limiter-wait-local --build-arg DATE=<UTC> -t sub2api:multi-key-local .`
- `docker compose -f D:\sub2api-deploy\docker-compose.yml --env-file D:\sub2api-deploy\.env up -d --no-deps --force-recreate sub2api`
- `docker exec sub2api /app/sub2api --version`
- `Invoke-RestMethod http://127.0.0.1:8080/health`
- `docker ps --format "{{.Names}} {{.Image}} {{.Status}}"`
- `docker logs --since 2m sub2api`

## 校验结果

数据库证据：run 10 分布为 `ok=1`、`rate_limited=3`、`unauthorized=1`、`batch_paused=5`；run 9 分布为 `ok=1`、`rate_limited=3`、`batch_paused=6`。账号 391 的 run 10 item 为 `batch_paused`，错误信息是 `batch test group openai:oauth:group:2 paused until 2026-05-30T10:53:30+08:00 after upstream error burst`。

上述 Go 聚焦测试、admin unit 测试和 `git diff --check` 均通过。Docker 镜像构建成功并已重建本地 `sub2api` 服务；`/health` 返回 `{"status":"ok"}`，`docker ps` 显示 `sub2api` healthy。最近 2 分钟启动日志没有匹配 `panic`、`fatal`、`migration failed`、`listen tcp`、`error`、`batch_paused` 或 `upstream error burst`。

---

日期：2026-05-30
执行者：Devil

## 结果

已修复账号上游体检报告的 token 扣分阈值过低问题。当前标准 9 次体检的总 token 已稳定在 23k-26k，例如 okcodex 的 run 113 为 9/9 成功、总 token 24,036。旧逻辑按整次 run 的固定 3k/6k 总量阈值扣分，会把正常体检误判为 token 过高。

现在 token 评分按 `request_count` 缩放：24,036 tokens 的标准 9 次体检不再出现 `Token 消耗` 扣分项，同时 100,000 tokens 这类异常膨胀仍保留扣分。

## 校验方式

- `go test ./internal/service -run "TestScoreAccountProbeRun|TestAccountProbe" -count=1`
- `go test ./internal/service ./internal/handler -run "^$" -count=1`
- `git diff --check -- backend/internal/service/account_probe_score.go backend/internal/service/account_probe_score_test.go`

## 校验结果

上述 Go 聚焦测试、service/handler 空跑编译和本次变更文件的 diff check 均通过。

---

日期：2026-05-30
执行者：Devil

## 结果

已按“HTTP 的请求也一样”补齐 OpenAI 首 Token 观测：

- 单次 OpenAI OAuth/APIKey HTTP SSE 测试会在首个内容事件返回 `first_token_ms`，账号测试弹窗直接展示首 Token。
- 非 API_KEY 批量测试从同一份 SSE 输出解析 `first_token_ms`，保存到批量测试 item，并在批量记录详情表展示。
- OpenAI HTTP 非流式请求成功时也会把首次可用响应耗时写入 `FirstTokenMs`，便于 ops 侧用同一个字段观察 HTTP/WS 两种路径。
- OpenAI 路由追踪新增 `schedule_layer`、`candidate_count`、`top_k`、`load_skew`、`sticky_previous_hit`、`sticky_session_hit`、`selected_account_type`、`continuity_action/reason` 和余额确认字段。
- `HandleSelectionExhausted` 在切换额度耗尽时直接返回，不再清空失败账号列表后额外退避重试。

## 校验方式

- `go test -tags unit ./internal/service -run "TestAccountTestService_OpenAIResponsesStreamEmitsFirstTokenMs|TestAccountTestService_OpenAIChatCompletionsStreamEmitsFirstTokenMs" -count=1`
- `go test -tags unit ./internal/handler/admin -run "TestAccountBatchTestNonAPIKeyPersistsFirstTokenMs" -count=1`
- `go test ./internal/handler -run "TestApplyOpenAIScheduleDecisionToOpsEntryAddsDetailedRouteTrace|TestHandleSelectionExhausted" -count=1`
- `go test ./internal/repository -run "^$" -count=1`
- `go test -tags unit ./internal/service -run "TestOpenAINonStreamingContentTypePassThrough|TestOpenAINonStreamingContentTypeDefault|TestOpenAIGatewayServiceHandleResponsesImageOutputs_NonStreaming|TestOpenAIGatewayService_OAuthPassthrough_StreamingSetsFirstTokenMs" -count=1`
- `npm run typecheck`
- `go test ./internal/service ./internal/handler ./internal/handler/admin ./internal/repository -run "^$" -count=1`

## 校验结果

上述验证均通过。

---

日期：2026-05-31
执行者：Devil

## 结果

已修复并部署 memo/Codex 文本调用被自动图片桥接误伤的问题。根因是 Codex 客户端普通 `/responses` 请求会被本地 Codex image bridge 自动注入 `image_generation` tool；账号 300 的上游分组不支持图片生成，返回 `Image generation is not enabled for this group`，旧逻辑把这个 403 当作账号鉴权/权限异常进入 `openai_403_temp_unschedulable`。

现在该类 OpenAI 图片权限 403 不会触发账号冷却或禁用；如果错误来自自动桥接注入，网关会先禁用该账号的 `codex_image_generation_bridge`，再用原始文本请求重试一次。已清理账号 300 的旧误伤临时冷却状态，并持久设置 `extra.codex_image_generation_bridge=false`。

当前本地 Docker 运行版本：

- `Sub2API 0.1.130 (commit: codex-image-bridge-fallback-local, built: 2026-05-30T15:57:58Z)`

## 校验方式

- `go test -tags unit ./internal/service -run "TestRateLimitService_HandleUpstreamError_OpenAIImageGenerationPermissionSkipsCooldown|TestOpenAIGatewayServiceForward_CodexBridgePermission403RetriesWithoutBridge" -count=1`
- `go test ./internal/service -run "TestOpenAIGatewayServiceForward_(RejectsDisabledImageGenerationIntents|DisabledGroupAllowsTextOnlyResponses|CodexImageInjectionRespectsGroupCapability|ExplicitImageToolWorksWithBridgeDisabled|ChannelBridgeOverrideEnablesCodexInjection)|TestOpenAIGatewayService_CodexImageGenerationBridgeOverridePrecedence|TestIsImageGenerationIntent" -count=1`
- `go test ./internal/service ./internal/handler -run "^$" -count=1`
- `git diff --check -- backend/internal/service/image_generation_intent.go backend/internal/service/ratelimit_service.go backend/internal/service/codex_image_generation_bridge.go backend/internal/service/openai_gateway_service.go backend/internal/service/openai_image_generation_bridge_fallback_test.go backend/internal/service/ratelimit_service_403_test.go backend/internal/service/ratelimit_service_401_test.go`
- `docker build --pull=false --build-arg COMMIT=codex-image-bridge-fallback-local --build-arg DATE=<UTC> -t sub2api:multi-key-local .`
- `docker compose -f D:\sub2api-deploy\docker-compose.yml --env-file D:\sub2api-deploy\.env up -d --no-deps --force-recreate sub2api`
- `docker exec sub2api /app/sub2api --version`
- `Invoke-RestMethod http://127.0.0.1:8080/health`
- 本地 `/v1/responses` 非流式真实请求。
- `docker logs --since 3m sub2api` 检查 `Image generation is not enabled`、`openai_403_temp_unschedulable`、`account_disabled_auth_error`、`panic`、`fatal`。
- 查询账号 300 的 `status`、`schedulable`、`temp_unschedulable_*` 和 `extra.codex_image_generation_bridge`。

## 校验结果

上述 Go 聚焦测试、service/handler 编译切片和 diff check 均通过。Docker 镜像构建成功并已重建本地 `sub2api` 服务；`/health` 返回 `{"status":"ok"}`。

真实请求验证：本地 `/v1/responses` 非流式请求返回 HTTP 200，耗时约 4 秒；重启后的真实 Codex 风格大上下文 `/responses` 请求多次返回 HTTP 200，日志中账号 300 的请求延迟样本包括约 7.5 秒、16.8 秒、18.5 秒、23.2 秒和 46.8 秒。最近日志未再出现 `Image generation is not enabled`、`openai_403_temp_unschedulable` 或 `account_disabled_auth_error`。日志中仍有少量 `context journal session overflow` 告警，这是上下文记录容量告警，不是客户端调用阻断错误。

数据库验证：账号 300 `status=active`、`schedulable=true`、`error_message` 为空、`temp_unschedulable_until` 为空、`temp_unschedulable_reason` 为空、`extra.codex_image_generation_bridge=false`。

---

日期：2026-05-31
执行者：Devil

## 结果

已将 OpenAI 上游 HTTP/2 代理回退、上游错误分类、工具输出续链识别、API-key SSE 误标兼容等当前工作树修复构建并部署到本地 `sub2api` 容器。当前运行版本：

- `Sub2API 0.1.130 (commit: openai-http2-fallback-local, built: 2026-05-31T03:17:37Z)`

针对用户反馈的 `https://api.aisz.mom/api/v1/usage returned 404`，实测该接口仍返回 HTTP 404，属于 aisz 上游余额端点不兼容；部署后本地 `/v1/responses` 与 `/responses` 真实转发均返回 HTTP 200，说明该余额 404 未再阻断真实转发链路。

## 校验方式

- `go test ./internal/service -run "TestNeedsToolContinuationSignals|TestHasFunctionCallOutput|TestOpenAIWSRawPayloadHasToolCallOutput|TestAccount_SupportsOpenAIEndpointCapability|TestClassifyUpstreamError|TestOpenAIPathHealth|TestHandleSSEToJSON|TestHandleNonStreamingResponse_APIKeyFallsBackToSSEBodyWhenContentTypeIsWrong" -count=1`
- `go test ./internal/repository -run "TestHTTPUpstream|TestOpenAI" -count=1`
- `go test ./internal/config ./internal/handler -run "TestLoadOpenAIHTTP2|TestApplyOpenAIScheduleDecision|TestOpsErrorLogger|TestUsageRecordSubmitTask" -count=1`
- `go test ./internal/service ./internal/repository ./internal/handler -run "^$" -count=1`
- `docker build --pull=false --build-arg COMMIT=openai-http2-fallback-local --build-arg DATE=<UTC> -t sub2api:multi-key-local .`
- `docker compose -f D:\sub2api-deploy\docker-compose.yml --env-file D:\sub2api-deploy\.env up -d --no-deps --force-recreate sub2api`
- `docker exec sub2api /app/sub2api --version`
- `Invoke-RestMethod http://127.0.0.1:8080/health`
- 使用数据库中 `codex` 本地 API key 调用 `POST http://127.0.0.1:8080/v1/responses`，`model=gpt-5.5`，非流式。
- 使用同一 API key 调用 `POST http://127.0.0.1:8080/responses`，`model=gpt-5.5`，流式。
- 使用账号 408 的上游 API key 直接调用 `GET https://api.aisz.mom/api/v1/usage`。
- 查询 `usage_logs` 和 `ops_error_logs` 验证真实请求落库与最近 OpenAI 错误。
- `git diff --check`

## 校验结果

上述 Go 聚焦测试、编译切片、Docker 构建、compose 重建和 diff check 均通过；`/health` 返回 `{"status":"ok"}`。

真实请求验证：

- `/v1/responses` 非流式返回 HTTP 200，耗时 3438ms，`usage_logs` 写入账号 408，`duration_ms=2987`，`first_token_ms=2987`。
- `/responses` 流式第一次返回 HTTP 200，耗时 33336ms，`usage_logs` 写入账号 408，`duration_ms=31162`，`first_token_ms=31120`。
- `/responses` 流式短请求复测返回 HTTP 200，耗时 3103ms，`usage_logs` 写入账号 408，`duration_ms=2757`，`first_token_ms=2280`。
- 最近 10 分钟 `ops_error_logs` 中 OpenAI 错误为 0，容器日志未出现 `panic`、`fatal`、`upstream_error`、`401 Unauthorized` 或 `Incorrect API key`。
- `https://api.aisz.mom/api/v1/usage` 直接调用仍返回 HTTP 404，确认它是余额端点不兼容，不是当前 `/responses` 连接失败根因。

---

日期：2026-05-31
执行者：Devil

## Codex 本地 sub2api 旁路配置验证

保留当前 `C:\Users\27404\.codex\config.toml` 不变，新增：

- `C:\Users\27404\.codex\sub2api-local.config.toml`
- `C:\Users\27404\.codex\codex-sub2api-local.ps1`

脚本运行时从本地 `sub2api-postgres` 读取 `api_keys.id=1`，只注入当前进程的 `SUB2API_API_KEY`，再使用 `sub2api-local` profile 指向 `http://127.0.0.1:8080/v1`。

验证命令：

- `C:\Users\27404\.codex\codex-sub2api-local.ps1 exec --skip-git-repo-check --sandbox read-only --output-last-message .codex\sub2api-local-script-smoke.txt '只回复 OK'`

验证结果：

- Codex CLI 使用 `provider: sub2api_local`，返回 `OK`。
- sub2api 运行日志显示 `/v1/responses` 使用 `api_key_id=1`、`account_id=408` 并返回 HTTP 200。
- `usage_logs` 写入 `first_token_ms=5158`。
- 测试过程中使用 dummy key 触发过 401；该 401 是参数位置/旧版 CLI 验证时的人工测试，不是最终旁路配置失败。

---

日期：2026-05-31
执行者：Devil

## OpenAI 0.1.131-0.1.133 稳定性合并阶段验证

本次完成 OpenAI 端点能力调度、账号级 pool mode 重试状态码、unknown-model 模型级冷却、连续性重放避开旧账号等本地实现收口。当前阶段只做代码实现和本地验证，尚未构建镜像、部署容器或提交 Git commit。

## 校验方式

- `go test ./internal/service -run "TestForwardAsChatCompletions_UnknownModelDoesNotUseDefaultMappedModel|TestForwardAsChatCompletions_APIKeyUnknownModelDoesNotFallbackRawChat|TestOpenAIGatewayService_ResponsesUnknownModelDoesNotFallbackToGPT54|TestIsOpenAIModelNotFoundError|TestHandleOpenAIAccountUpstreamErrorForModel_ModelNotFoundSetsModelCooldown|TestIsOpenAIPoolModeRetryableOnSameAccount_ModelNotFoundIsNeverSameAccountRetryable|TestOpenAIGatewayService_SelectAccountWithScheduler_StickyExhausted|TestOpenAIGatewayService_SelectAccountWithScheduler_PreviousResponseExhausted" -count=1`
- `go test ./internal/service -run "TestOpenAI.*EndpointCapability|Test.*PoolModeRetryable|Test.*ModelNotFound|TestOpenAIContextContinuity.*|TestOpenAIGatewayService_SelectAccountWithScheduler.*|TestForwardAsChatCompletions_.*UnknownModel.*|TestOpenAIGatewayService_ResponsesUnknownModelDoesNotFallbackToGPT54" -count=1`
- `go test ./internal/service ./internal/handler -run "^$" -count=1`
- `git diff --check`

## 校验结果

上述聚焦回归、相关模式测试、service/handler 编译切片和 diff check 均通过。`git diff --check` 仅提示 `docs/feature_list.jsonl`、`docs/process_list.jsonl`、`verification.md` 后续由 Git 触碰时 LF 会变为 CRLF，不存在空白错误。

---

日期：2026-05-31
执行者：Devil

## OpenAI 余额探测不参与调度验证

本次修复针对 `/api/v1/usage returned 404` 等上游余额探测失败：余额快照只保留为诊断，不再参与连续会话换号、protected 中断或候选账号过滤。真实上游请求错误仍按现有 failover 与账号健康逻辑处理。

## 校验方式

- `go test -tags unit ./internal/service -run "OpenAIContextContinuity|SelectAccountWithScheduler_(StickyBalanceExhausted|PreviousResponseBalanceExhausted|LoadBalanceDoesNotPrecheckRealtimeBalance|AllowsOnlyAPIKeyWhenRealtimeBalanceUnknown)" -count=1`

## 校验结果

聚焦测试通过。首次运行因默认 `C:\Users\27404\AppData\Local\go-build` 权限被拒失败，改用项目内 `backend/gocache` 作为 `GOCACHE` 后通过；临时缓存目录已清理。

## 部署验证

- `docker build --pull=false --build-arg COMMIT=openai-balance-routing-ignore-local -t sub2api:multi-key-local .`
- `docker compose -f D:\sub2api-deploy\docker-compose.yml --env-file D:\sub2api-deploy\.env up -d --no-deps --force-recreate sub2api`
- `docker exec sub2api /app/sub2api --version`
- `Invoke-RestMethod http://127.0.0.1:8080/health`
- 使用本地 `api_keys.id=1` 调用 `POST http://127.0.0.1:8080/v1/responses`，`model=gpt-5.5`，输入“只回复 OK”。
- 扫描最近 5 分钟容器日志中的 `api/v1/usage returned 404`、`no available OpenAI accounts`、`context_replay_not_safe`、`upstream_error`、`502 Bad Gateway`。

部署结果：

- 当前运行版本：`Sub2API 0.1.133 (commit: openai-balance-routing-ignore-local, built: 2026-05-31T14:46:28Z)`。
- `/health` 返回 `{"status":"ok"}`。
- 真实 `/v1/responses` 返回 HTTP 200，耗时约 2264ms，输出 `OK`。
- 最近 5 分钟日志关键错误扫描为空。

---

日期：2026-06-02
执行者：Devil

## 分组倍率修改与读取修复验证

本次修复 admin 修改用户专属分组倍率、修改分组默认倍率、批量设置/清空分组倍率后，运行时 gateway 仍可能在缓存 TTL 内读取旧倍率的问题；同时修复清空分组倍率时误删同表 RPM override 的语义错误。

## 校验方式

- `go test -tags unit ./internal/service -run "TestAdminService_ClearGroupRateMultipliers|TestAdminService_BatchSetGroupRateMultipliers|TestUserGroupRateResolverResolve_VersionBumpBypassesStaleCache|TestAdminService_UpdateUserGroupRates_InvalidatesRuntimeCaches|TestAdminService_UpdateGroup_BumpsUserGroupRateCacheVersion" -count=1`
- `go test -tags unit ./internal/service -run "TestAdminService_(ClearGroupRateMultipliers|BatchSetGroupRateMultipliers|UpdateUserGroupRates_InvalidatesRuntimeCaches|UpdateGroup_BumpsUserGroupRateCacheVersion)|TestUserGroupRateResolver|TestGatewayServiceGetUserGroupRateMultiplier|TestGetUserGroupRateMultiplier" -count=1`
- `go test -tags unit ./internal/service -count=1`
- `git diff --check`

## 校验结果

- 两组分组倍率聚焦回归均通过，覆盖清空分组倍率保留 RPM override、批量设置后缓存失效、用户专属倍率更新后运行时缓存版本推进、分组默认倍率更新后缓存版本推进，以及 resolver 版本推进绕开旧缓存。
- `git diff --check` 仅提示 `backend/cmd/server/VERSION`、`docs/feature_list.jsonl`、`docs/process_list.jsonl`、`verification.md` 后续由 Git 触碰时 LF 会变为 CRLF，不存在空白错误。
- `go test -tags unit ./internal/service -count=1` 运行约 600 秒后在既有 OpenAI/渠道相关用例中失败/卡住，失败点与本次分组倍率修改无关，未作为本次修复阻断项。

---

日期：2026-06-02
执行者：Devil

## OpenAI 单可用账号 failover 等待重试验证

本次修复针对 OpenAI `/responses` 只有一个可用账号时，上游返回 `UpstreamFailoverError` 后被立即加入 `failedAccountIDs`，下一轮选号失败并进入 `handleFailoverExhausted`，最终把 5xx 映射成 `Upstream service temporarily unavailable` 的 502。

修复后，handler 层在确认只有一个候选或排除失败账号后没有其他候选时，会释放账号槽位、清空本次排除列表并按 2 秒间隔重新进入调度；等待窗口最多 5 分钟，超过窗口后按原上游错误中断。多候选场景仍优先切换其他账号。

## 校验方式

- `go test ./internal/handler -run "TestOpenAISingleAccountFailoverWindow|TestOpenAIMapUpstreamError_Maps413ToRequestEntityTooLarge" -count=1`
- `go test ./internal/handler -run "^$" -count=1`
- `go test ./internal/service ./internal/handler -run "^$" -count=1`
- `git diff --check -- .\backend\internal\handler\openai_gateway_handler.go .\backend\internal\handler\openai_gateway_handler_test.go`

## 校验结果

- 单账号等待窗口聚焦测试通过，覆盖窗口内继续重试、到达 5 分钟停止、多候选不进入单账号等待。
- handler 编译切片通过。
- service+handler 编译切片中 handler 通过；service 被当前工作树既有 `resetUserGroupRateCacheVersionForTest` 缺失阻塞，错误来自 `internal\service\user_group_rate_resolver_test.go`，不属于本次 OpenAI handler 改动。
- diff check 通过。

---

日期：2026-06-02
执行者：Devil

## OpenAI /responses 候选池耗尽整体轮询验证

本次在前一版“单可用账号等待重试”基础上继续扩展：OpenAI `/responses` 遇到上游 `UpstreamFailoverError` 时，多候选仍先切换其他账号；如果本轮可调度池都进入 `failedAccountIDs` 导致选号失败，或切换保护到达当前候选数量上限，则清空本轮排除列表并按 2 秒间隔重新进入调度，最多等待 5 分钟。超过窗口后才按最后一次上游错误中断客户端。

## 校验方式

- `go test ./internal/handler -run "TestOpenAIFailoverRetryWindow" -count=1`
- `go test ./internal/handler -run "TestOpenAIFailoverRetryWindow|TestOpenAIMapUpstreamError_Maps413ToRequestEntityTooLarge" -count=1`
- `go test ./internal/handler -run "^$" -count=1`

## 校验结果

- 红测先失败于 `openAIFailoverRetryWindow`、`openAIFailoverRetryDelay`、`openAIFailoverRetryMaxWait` 未实现，证明测试覆盖的是新行为。
- 聚焦测试通过，覆盖单候选进入等待窗口、多候选不提前等待、池耗尽后清空排除列表并继续等待、到达 5 分钟窗口后停止。
- handler 编译切片通过。
- `go test ./internal/service ./internal/handler -run "^$" -count=1` 中 handler 通过；service 包仍被当前工作树既有 `resetUserGroupRateCacheVersionForTest` 缺失阻塞，错误来自 `internal\service\user_group_rate_resolver_test.go`，不属于本次 OpenAI handler 改动。

---

日期：2026-06-02
执行者：Devil

## Claude Code 国产模型 thinking SSE 兼容验证

本次修复针对 Claude Code 通过本地 sub2api 调用 `deepseek-v4-pro` 等国产模型时，普通 Anthropic `/v1/messages` 请求未显式开启 thinking，但上游仍先返回 `content_block_start(type=thinking)`、`thinking_delta` 和 `signature_delta` 的情况。Claude Code 的 text 输出会隐藏这些块，复杂提示会表现为长时间 0 字节或 `max turns` 异常。

修复后，Gateway 通用流式响应在 request context 中读取 `ThinkingEnabled=false` 时，会丢弃 thinking block 及对应 delta/signature/stop，继续保留 `message_start`、text block、usage 和 `message_stop`。

## 校验方式

- `go test -tags unit ./internal/service -run "TestHandleStreamingResponse_(SuppressesThinkingWhenRequestDisabled|CacheTokens|ZeroUsageTerminalBeforeOutput|MissingTerminal|DataErrorBeforeOutput)" -count=1`
- `go test -tags unit ./internal/service -run "TestHandleStreamingResponse|TestGatewayService_AnthropicAPIKeyPassthrough|TestForwardAsRawChatCompletions_PreservesDeepSeekReasoningContent" -count=1`
- `go test -tags unit ./internal/service -count=1`

## 校验结果

- 新增 `thinking=false` 回归测试通过，确认 thinking start/delta/signature/stop 被过滤，text、usage、terminal 正常通过。
- 相关 streaming/failover、Anthropic API key passthrough、DeepSeek OpenAI 原生 `reasoning_content` focused tests 通过。
- 全量 `internal/service` 单测超过 90 秒无输出后中断；本次改动已由 focused tests 覆盖，未把既有慢测作为本次阻断项。

---

日期：2026-06-02
执行者：Devil

## Anthropic Messages reasoning-only 可见兜底验证

本次排查针对 Claude Code 通过本地 sub2api 调用国产 reasoning 模型时反复出现空输出、长时间无正文或 `max_tokens` 的问题。

当前源码链路为：Anthropic `/v1/messages` 请求在 `OpenAIGatewayService.ForwardAsAnthropic` 中转为 OpenAI Responses 请求，上游强制流式返回，再由 `apicompat.ResponsesToAnthropic` / `ResponsesEventToAnthropicEvents` 转回 Anthropic Messages。原转换会把 Responses `reasoning` 保真映射为 Anthropic `thinking` block；当上游没有生成最终 `output_text` 时，Claude Code 这类只消费 text 的客户端就表现为空输出。

修复后，Responses 转 Anthropic 时仅在“没有可见 text、没有 tool_use/server tool，但存在 thinking/reasoning 摘要”时追加一个 text 兜底 block。已有 text 不重复，tool_use 不伪装成正文。

## 校验方式

- direct `/v1/models` 查看当前本地模型列表。
- direct `/v1/messages` trivial smoke：`GLM-5.1`、`deepseek-v4-pro`、`deepseek-v4-flash`、`mimo-v2.5-pro`、`mimo-v2.5`。
- direct `/v1/messages` medium smoke：`deepseek-v4-pro`、`GLM-5.1`、`mimo-v2.5-pro`。
- 临时模块内 `go run .\.codex\apicompat-check` 编译生产 apicompat 包，并验证 reasoning-only JSON 与 SSE 兜底。
- `go test ./internal/pkg/apicompat`

## 校验结果

- 当前本地 8080 模型列表只暴露 `GLM-5.1`、`deepseek-v4-flash`、`deepseek-v4-pro`、`mimo-v2.5`、`mimo-v2.5-pro`；旧 `glm-4.5`、`deepseek-v3-1-terminus`、`kimi-k2-0711-preview` 请求返回 503。
- 当前三模型 trivial smoke 均返回 `OK`；DeepSeek/MiMo 同时返回 thinking 与 text，GLM 只返回 text。
- 中等 smoke 结果：`deepseek-v4-pro` 返回 `stop=max_tokens`、`text_len=347`、`thinking_len=920`；`GLM-5.1` 返回 `stop=end_turn`、`text_len=699`；`mimo-v2.5-pro` 返回 `stop=max_tokens`、`text_len=598`、`thinking_len=591`。说明 DeepSeek/MiMo 在中等任务上仍需更高 token 预算或更小任务切分。
- 临时 `go run` 检查通过，确认生产 apicompat 包会在 reasoning-only JSON/SSE 场景追加 Claude Code 可见 text。
- `go test ./internal/pkg/apicompat` 被当前工作树既有测试字段不匹配阻塞，错误包括 `ResponsesStreamEvent.Usage`、`ResponsesInputTokensDetails.AudioTokens`、`ResponsesOutputTokensDetails.AudioTokens`、`ChatUsage.CompletionTokensDetails` 等字段不存在；这些错误与本次新增兜底逻辑无关。
- 本次只修改源码和 skill，未重建/重启当前 `127.0.0.1:8080` 运行网关，因此线上本地进程尚未具备该兜底。

---

日期：2026-06-02
执行者：Devil

## Anthropic Messages server tool 兜底一致性验证

本次修复代码审查发现的流式/非流式行为不一致：非流式 Responses 转 Anthropic 时，只要存在 `server_tool_use` 或 `web_search_tool_result` 就不会把 reasoning 复制成可见 text；流式路径此前只检查 function `tool_use`，因此 reasoning + web_search server tool + 无正文时仍会额外发 text fallback。

## 校验方式

- 新增 `TestStreamingReasoningWithServerToolDoesNotAddVisibleTextFallback`，先验证修复前 completion 事件多出 text fallback。
- 修复后运行 reasoning fallback 聚焦测试。
- 运行 `apicompat` 包测试。
- 运行 `git diff --check`。

## 校验结果

- 红测先失败：`response.completed` 返回 5 个事件，而期望只有 `message_delta` 和 `message_stop`。
- 修复后 `go test ./internal/pkg/apicompat -run "TestStreamingReasoningWithServerToolDoesNotAddVisibleTextFallback|TestStreamingReasoningOnlyAddsVisibleTextFallback|TestResponsesToAnthropic_ReasoningOnlyAddsVisibleTextFallback" -count=1` 通过。
- `go test ./internal/pkg/apicompat -count=1` 通过。
- `git diff --check` 通过，仅保留 Windows 工作区对文档文件的 LF/CRLF 提示。

---

日期：2026-06-02
执行者：Devil

## 本地 sub2api reasoning-only 可见兜底部署验证

本轮验证承接前一节的源码修复，确认本地 Docker 运行镜像已经包含 Responses 转 Anthropic Messages 的 reasoning-only 可见 text 兜底，且 Claude Code 客户端可以看到国产 reasoning 模型的正文输出。

## 校验方式

- `go test ./internal/pkg/apicompat -count=1`
- `docker ps --filter "name=sub2api" --format ...`
- `docker exec sub2api /app/sub2api --version`
- direct `GET /v1/models`
- direct `POST /v1/messages` trivial smoke：`GLM-5.1`、`deepseek-v4-pro`、`mimo-v2.5-pro`
- direct `POST /v1/messages` medium smoke：`deepseek-v4-pro`、`GLM-5.1`、`mimo-v2.5-pro`
- Claude Code `--bare` trivial smoke：`GLM-5.1`、`deepseek-v4-pro`、`mimo-v2.5-pro`
- Claude Code `--bare` medium smoke：`deepseek-v4-pro`

## 校验结果

- `go test ./internal/pkg/apicompat -count=1` 通过。
- 当前 `sub2api` 容器运行 `sub2api:multi-key-local`，状态为 `healthy`。
- 容器版本通过 `docker exec sub2api /app/sub2api --version` 校验，确认镜像内二进制使用本次提交号构建。
- direct `/v1/models` 暴露 `deepseek-v4-flash`、`deepseek-v4-pro`、`GLM-5.1`、`mimo-v2.5`、`mimo-v2.5-pro`。
- direct trivial smoke 均返回可见 text：`GLM-5.1 stop=end_turn text_len=2`，`deepseek-v4-pro stop=end_turn text_len=2 thinking_len=29`，`mimo-v2.5-pro stop=end_turn text_len=2 thinking_len=66`。
- direct medium smoke 均返回可见 text：`deepseek-v4-pro stop=max_tokens text_len=766 thinking_len=1039`，`mimo-v2.5-pro stop=end_turn text_len=198 thinking_len=62`。DeepSeek 此处仍显示请求预算耗尽，但不再是空输出。
- Claude Code `--bare` trivial smoke 均返回 `OK`，DeepSeek medium smoke 返回非空正文，`output_len=786`。

## DeepSeek 输出预算诊断

本次继续检查 `AnthropicToResponses` 请求转换路径：`max_tokens` 仅直接映射为 Responses `max_output_tokens`，低于 `minMaxOutputTokens=128` 时才抬高到 128；未发现 sub2api 在该路径设置 32k 或 384k 的本地硬上限。

对同一个 DeepSeek 中等证明题进行预算对照：

- `max_tokens=900`：`stop=max_tokens`，可见 text 非空。
- `max_tokens=4096`：`stop=end_turn`，`text_len=654`，`thinking_len=777`。
- `max_tokens=4096` 且 `output_config.effort=low`：`stop=end_turn`，`text_len=743`，`thinking_len=696`。

结论：当前主要问题不再是 Claude Code 看不到 reasoning-only 输出，而是调度层需要先区分“小预算导致的 max_tokens”和“真实模型/网关失败”。复杂题评分时应先使用 direct API 明确提升 `max_tokens`，必要时把 DeepSeek 子任务拆到更小粒度，再由 GLM 聚合、Codex 校验。

## 镜像构建备注

标准 `docker build --pull=false --build-arg COMMIT=<commit> -t sub2api:multi-key-local .` 首次重试时被 Docker Desktop 当前 registry mirror 阻断，基础镜像 metadata 请求返回 403 或 TLS timeout。为避免改动宿主 Docker 配置，本轮使用本机 Go 对当前源码执行 Linux/amd64 静态编译，再基于上一版本地 `sub2api:multi-key-local` 运行时镜像替换 `/app/sub2api` 并重新打同名镜像。该镜像随后通过 compose recreate 部署，并用容器内 `--version`、direct API、Claude Code smoke 验证。

---

日期：2026-06-02
执行者：Devil

## DeepSeek 经 sub2api 长输出上限验证

本轮根据用户要求检查当前 sub2api 调用 DeepSeek 的最大输出，并解析本地已生成的长输出响应文件。验证对象是 Anthropic Messages 兼容响应格式下的 `deepseek-v4-pro`。

## 校验方式

- 解析 `.codex/deepseek-long-output-4096.json`
- 解析 `.codex/deepseek-long-output-8192.json`
- 解析 `.codex/deepseek-long-output-16384.json`
- 解析 `.codex/deepseek-long-output-32768.json`
- 解析 `.codex/deepseek-long-output-65536.json`
- 读取 `backend/resources/model-pricing/model_prices_and_context_window.json` 中 DeepSeek 条目
- 未授权探测 `http://127.0.0.1:8080/v1/models`，确认本轮不猜测或输出任何密钥

## 校验结果

- `max_tokens=4096`：`usage.output_tokens=4096`，`stop_reason=max_tokens`
- `max_tokens=8192`：`usage.output_tokens=8192`，`stop_reason=max_tokens`
- `max_tokens=16384`：`usage.output_tokens=16384`，`stop_reason=max_tokens`
- `max_tokens=32768`：`usage.output_tokens=31146`，`stop_reason=end_turn`
- `max_tokens=65536`：`usage.output_tokens=57212`，`stop_reason=end_turn`，正文约 `167999` 字符，编号行数 `3000`
- 模型价格表显示 `deepseek-chat` 的 `max_output_tokens=8192`，`deepseek-reasoner` 的 `max_output_tokens=65536`

结论：当前 sub2api 调 DeepSeek 的长输出能力按模型区分；普通 `deepseek-chat` 是 8192，reasoner/当前 `deepseek-v4-pro` 路由可请求到 65536。本轮实测 65536 请求成功返回 57212 output tokens，但测试 prompt 自己在 3000 行结束，因此没有再次撞满 65536。

---

日期：2026-06-02
执行者：Devil

## 管理员 Dashboard Token 结构化统计验证

本轮为管理员 Dashboard 统计接口和页面增加今日/累计输入 Token、输出 Token、缓存读取 Token 和缓存读取比例。缓存读取比例口径为 `cache_read_tokens / (input_tokens + cache_read_tokens)`，即输入侧缓存命中占比。

## 校验方式

- `go test ./internal/pkg/usagestats -count=1`
- `npm run test:run -- src/views/admin/__tests__/DashboardView.spec.ts`
- `npm run typecheck`
- `npm run build`
- `git diff --check`
- `go test ./internal/handler/admin -run Dashboard -count=1`

## 校验结果

- `go test ./internal/pkg/usagestats -count=1` 通过。
- `npm run test:run -- src/views/admin/__tests__/DashboardView.spec.ts` 通过，2 个 Dashboard 组件测试通过，覆盖默认时间范围和 Token 明细渲染。
- `npm run typecheck` 通过。
- `npm run build` 通过，保留 Vite 既有 chunk size / dynamic import 警告。
- `git diff --check` 通过，仅提示 Windows 工作区将文档 LF 转 CRLF。
- `go test ./internal/handler/admin -run Dashboard -count=1` 被当前包既有测试编译问题阻塞：`internal\handler\admin\account_handler_mixed_channel_test.go:165:57: adminSvc.updatedAccounts[0].Platform undefined (type *service.UpdateAccountInput has no field or method Platform)`。该失败点不在本轮修改文件内。

---

日期：2026-06-03
执行者：Devil

## 上游体检报告排行、12 分钟超时收尾、定时体检与 Gitee 参考梳理

本轮为管理员上游体检报告增加综合得分排行榜和单上游得分历史；将运行中报告超时收尾阈值统一为 12 分钟；扩展定时测试计划支持 `task_type=account_probe`，并在报告页提供创建定时上游体检计划的弹窗入口。

## 校验方式

- `rtk go test ./internal/service -run "TestScheduledTestRunnerRunsAccountProbePlan" -count=1`：先按 TDD 红灯运行，因 `runOnePlanWithTimeout` 不存在失败；补实现后通过。
- `rtk go test ./internal/service -run "TestAccountProbeServiceListRankingDecoratesAggregateScores|TestScheduledTestRunnerRunsAccountProbePlan" -count=1`
- `rtk go test ./internal/repository -run "TestAccountProbeRepositoryDeleteReportRunsSkipsRunning|TestAccountProbeRepositoryExpireStaleRuns" -count=1`
- `rtk go test -tags unit ./internal/handler/admin -run "TestAccountProbeReportRankingReturnsAggregateItems|TestAccountProbeReportListParsesFiltersAndReturnsPage|TestAccountProbeReportBatchDeleteDeduplicatesRunIDs" -count=1`
- `rtk npm run test:run -- src/api/__tests__/admin.accounts.spec.ts`
- `rtk npm run test:run -- src/views/admin/__tests__/AccountProbeReportsView.spec.ts`
- `rtk npm run typecheck`
- `rtk npm run build`
- `rtk go test ./cmd/server -run "^$" -count=1`
- `rtk git diff --check`
- `git -C .codex/external/juhe-ai fetch origin feature/20250602`

## 校验结果

- service 聚焦测试通过，覆盖排行榜聚合装饰和定时 account_probe 计划执行；定时 account_probe 执行上下文带 12 分钟级 deadline。
- repository 聚焦测试通过，覆盖 stale running 报告收尾和删除前先把 12 分钟以上 running 标失败。
- admin handler 单元测试通过，覆盖排行榜接口、报告列表过滤、删除 ID 去重。
- 前端 API 单测 13 个通过；报告页组件单测 7 个通过，覆盖排行榜展示/点击过滤/历史得分和定时体检计划创建。
- `npm run typecheck` 通过。
- `npm run build` 通过；保留项目既有 Browserslist caniuse-lite 过期提示、Vite dynamic import/chunk size 警告。
- `go test ./cmd/server -run "^$" -count=1` 退出码 0，rtk 总结为 `Go test: No tests found`，因此仅作为 server package 编译切片记录。
- `git diff --check` 通过。
- Gitee 参考仓库 `feature/20250602` 已 fetch，当前 HEAD 与 FETCH_HEAD 均为 `6055e7b`。

## Gitee 参考清单

- 调度相关：`backend/src/modules/background/worker-scheduler.ts`，可借鉴任务运行快照、跳过重入、运行次数/失败次数/耗时统计。
- 分组调度相关：`backend/src/domain/group-scheduling.ts`，可参考高并发分组的软并发、排队、首输出慢阈值和图片 lane 并发字段。
- API Key 分组选择相关：`backend/src/modules/gateway/api-key-group-route-selector.service.ts`，可参考 round robin / weighted round robin 的选择顺序，但该实现是内存状态，不适合直接搬到多实例 Go 服务。
- 账号验证相关：`backend/src/modules/accounts/account-test.service.ts`，它通过真实 gateway request 测试 `/v1/responses`、记录首 Token、响应体截断和诊断字段；概念上与本项目现有 AccountProbeService 接近。
- 模型验证相关：`backend/src/modules/model-checks/model-checks.service.ts`、`backend/src/modules/model-checks/model-checks.routes.ts`、`backend/src/storage/model-checks.repository.ts`、`frontend/src/views/model-checks/ModelChecksView.vue`，包含模型检查 run/item 持久化、多探针行为校验、长上下文、可信对比和进度事件。

结论：Gitee 仓库值得后续借鉴“真实网关链路模型校验”“探针 item/run 证据结构”“后台任务运行快照”和“分组调度策略字段”，但不建议直接搬代码；它是 TypeScript/Express 栈，模型名与业务假设固定在 `gpt-5.5/gpt-5.4`，且部分调度状态在内存中维护。
---

日期：2026-06-03
执行者：Devil

## Go-native 实现 Gitee 调度与模型验证可借鉴点

本轮在已有 Go 分层内实现 Gitee `huanminabc/juhe-ai` `feature/20250602` 中值得借鉴的能力：定时 runner 增加运行态快照、重入跳过计数和 admin 查询接口；账号体检增加低成本模型行为验证样本、输出文本提取、验证证据评分与 sample 持久化；前端报告详情展示验证证据，并补 scheduled runner snapshot API wrapper。

## 校验方式

- `rtk npm run test:run -- src/api/__tests__/admin.scheduledTests.spec.ts`：先按 TDD 红灯运行，因 `listRunnerSnapshots is not a function` 失败；补 wrapper 后通过。
- `rtk go test -tags unit ./internal/service -run "TestScheduledTestRunner|TestEvaluateAccountProbeModelValidationEvidence|TestAccountProbeService_RunStoresModelValidationEvidence|TestAccountProbeService_RunOpenAIAPIKeyPersistsSamples|TestAccountProbeService_RunOpenAIAPIKeyStreamModeRecordsFirstToken|TestAccountProbeServiceGetReportLoadsSamplesAndScoreBreakdown|TestAccountProbeServiceListRankingDecoratesAggregateScores" -count=1`
- `rtk go test -tags unit ./internal/handler/admin -run "TestScheduledTestHandlerListRunnerSnapshots|TestAccountProbeReport" -count=1`
- `rtk go test ./internal/repository -run "TestAccountProbeRepository" -count=1`
- `rtk go test ./cmd/server -run TestDoesNotExist -count=1`
- `rtk npm run test:run -- src/views/admin/__tests__/AccountProbeReportsView.spec.ts src/api/__tests__/admin.scheduledTests.spec.ts src/api/__tests__/admin.accounts.spec.ts`
- `rtk npm run typecheck`
- `git diff --check`
- PowerShell `ConvertFrom-Json` 逐行解析 `docs/feature_list.jsonl` 与 `docs/process_list.jsonl`

## 校验结果

- service 聚焦测试通过，覆盖 scheduled runner 快照/重入跳过、模型验证证据评分、AccountProbeService 保存验证证据、已有账号体检报告路径。
- admin handler 聚焦测试通过，覆盖 runner snapshot 查询端点和账号体检报告接口。
- repository 聚焦测试通过，覆盖 account probe sample 的 `output_text` 与 `validation_evidence` 保存和读取。
- server package 编译切片退出码 0，rtk 输出 `Go test: No tests found`，说明该包无匹配测试但编译通过。
- 前端 3 个测试文件共 21 个测试通过，覆盖报告详情验证证据展示、账号报告 API 和 scheduled runner snapshot API；保留既有 Browserslist caniuse-lite 过期提示。
- `vue-tsc --noEmit` 通过。
- `git diff --check` 通过，仅有 Windows LF/CRLF 提示。
- docs JSONL 逐行解析通过。

---

日期：2026-06-03
执行者：Devil

## 手动模型探针页面与自动体检隔离

本轮按用户追加要求调整低成本模型探针：常规上游体检、批量体检和定时体检不再自动执行模型验证样本；模型验证只通过新的管理员手动页面触发。前端新增 `/admin/model-probes` 页面和侧边栏入口，管理员填写账号 ID、模型与请求方式后调用 `/api/v1/admin/account-model-probe-runs`，页面展示最近 `model_validation` 运行并可展开查看输出文本、验证证据、期望值/观察值和单项得分。

## 校验方式

- `rtk npm run test:run -- src/views/admin/__tests__/AccountModelProbesView.spec.ts`：先按 TDD 红灯运行，因页面文件不存在失败；补实现后通过。
- `rtk go test -tags unit ./internal/service -run "TestAccountProbeService_RunCodexStabilityDoesNotAutoRunModelValidation|TestAccountProbeService_RunManualModelValidationStoresEvidence|TestScheduledTestRunnerRunsAccountProbePlan" -count=1`
- `rtk go test -tags unit ./internal/handler/admin -run "TestAccountModelProbeCreateRunsManualValidationOnly|TestAccountProbeReportBatchCreate|TestAccountProbeCreate" -count=1`
- `rtk go test -tags unit ./internal/service -run "TestScheduledTestRunner|TestEvaluateAccountProbeModelValidationEvidence|TestAccountProbeService_RunCodexStabilityDoesNotAutoRunModelValidation|TestAccountProbeService_RunManualModelValidationStoresEvidence|TestAccountProbeService_RunOpenAIAPIKeyPersistsSamples|TestAccountProbeService_RunOpenAIAPIKeyStreamModeRecordsFirstToken|TestAccountProbeServiceGetReportLoadsSamplesAndScoreBreakdown|TestAccountProbeServiceListRankingDecoratesAggregateScores" -count=1`
- `rtk go test -tags unit ./internal/handler/admin -run "TestAccountModelProbeCreateRunsManualValidationOnly|TestAccountProbeReportBatchCreate|TestAccountProbeCreate|TestScheduledTest" -count=1`
- `rtk npm run test:run -- src/views/admin/__tests__/AccountModelProbesView.spec.ts src/views/admin/__tests__/AccountProbeReportsView.spec.ts src/api/__tests__/admin.accounts.spec.ts`
- `rtk npm run typecheck`
- `git diff --check`
- Vite dev server: `http://127.0.0.1:5173/admin/model-probes`
- Playwright 浏览器打开新路由

## 校验结果

- service 聚焦测试通过，覆盖普通 Codex 稳定性体检不会再自动追加模型验证样本、手动 `model_validation` 会保存验证证据、定时 account_probe 计划不再传播模型验证。
- admin handler 聚焦测试通过，覆盖手动模型探针端点只创建 `model_validation` 运行，以及批量/普通体检接口不再携带自动模型验证。
- 前端 3 个测试文件共 23 个测试通过，覆盖新模型探针页面加载、手动提交、详情证据展示，以及报告页批量/定时 payload 不再包含模型验证字段；保留既有 Browserslist caniuse-lite 过期提示。
- `vue-tsc --noEmit` 通过。
- `git diff --check` 通过，仅提示 docs/feature_list.jsonl、docs/process_list.jsonl、verification.md 在 Windows 工作区会由 LF 转 CRLF。
- Vite 已在 5173 端口启动，`/admin/model-probes` 返回 HTTP 200。
- 未登录访问新路由按应用路由守卫跳转到 `/login?redirect=/admin/model-probes`，浏览器控制台 0 errors。

---

日期：2026-06-03
执行者：Devil

## 上游体检报告页批量模型验证

本轮在上游体检报告页增加手动“批量模型验证”入口，同时保留定时常规体检。报告页现在有三个独立动作：常规批量验证、批量模型验证、定时体检。批量模型验证复用 API Key 账号选择弹窗，但提交到 /api/v1/admin/account-model-probe-runs/batch，只创建 model_validation 运行；定时体检继续创建 	ask_type=account_probe 的常规计划，默认 probe_mode=standard，不携带模型验证字段。

## 校验方式

- rtk go test -tags unit ./internal/handler/admin -run "TestAccountModelProbeBatchCreateRunsManualValidationOnly" -count=1：先按 TDD 红灯运行，因 BatchCreateModelProbeRuns 不存在失败；补实现后通过。
- rtk npm run test:run -- src/api/__tests__/admin.accounts.spec.ts -t "starts batch manual account model probe runs"：先按 TDD 红灯运行，因 BatchAccountModelProbeRuns 不存在失败；补 wrapper 后通过。
- rtk npm run test:run -- src/views/admin/__tests__/AccountProbeReportsView.spec.ts -t "starts batch model validation"：先按 TDD 红灯运行，因报告页按钮不存在失败；补入口后通过。
- rtk go test -tags unit ./internal/handler/admin -run "TestAccountModelProbeCreateRunsManualValidationOnly|TestAccountModelProbeBatchCreateRunsManualValidationOnly|TestAccountProbeReportBatchCreate|TestAccountProbeCreate" -count=1
- rtk go test -tags unit ./internal/service -run "TestAccountProbeService_RunCodexStabilityDoesNotAutoRunModelValidation|TestAccountProbeService_RunManualModelValidationStoresEvidence|TestScheduledTestRunnerRunsAccountProbePlan" -count=1
- rtk npm run test:run -- src/api/__tests__/admin.accounts.spec.ts src/views/admin/__tests__/AccountProbeReportsView.spec.ts src/views/admin/__tests__/AccountModelProbesView.spec.ts
- rtk npm run typecheck
- git diff --check

## 校验结果

- admin handler 聚焦测试通过，覆盖单个/批量手动模型探针只创建 ModelValidationOnly 的 model_validation 运行，以及常规体检接口保持可用。
- service 聚焦测试通过，覆盖普通 Codex 稳定性不自动追加模型验证、手动模型验证保存证据、定时 account_probe 仍按常规计划执行。
- 前端 3 个测试文件共 25 个测试通过，覆盖 accounts API、报告页批量模型验证入口、常规批量体检、定时常规体检、模型探针页面。
- ue-tsc --noEmit 通过。
- git diff --check 通过，仅有 Windows LF/CRLF 提示。

---

日期：2026-06-03
执行者：Devil

## 默认测试模型统一为 gpt-5.5

本轮将 OpenAI 上游体检、模型探针、账号测试、非 API_KEY 批量账号测试和账号定时测试的默认模型统一为 `gpt-5.5`。后端 `openai.DefaultTestModel` 改为 `gpt-5.5`；前端账号测试弹窗不再按 free/paid 计划分流到 `gpt-5.4`；报告页批量体检、批量模型验证、定时体检、手动模型探针页面、账号定时测试面板均默认填入或优先选择 `gpt-5.5`。

## 校验方式

- `rtk go test ./internal/pkg/openai -run TestDefaultTestModelUsesGPT55 -count=1`：先红后绿。
- `rtk npm run test:run -- src/components/admin/account/__tests__/AccountTestModal.spec.ts src/components/account/__tests__/AccountTestModal.spec.ts src/views/admin/__tests__/AccountsView.bulkEdit.spec.ts src/views/admin/__tests__/AccountModelProbesView.spec.ts src/views/admin/__tests__/AccountProbeReportsView.spec.ts -t "gpt-5.5|paid OpenAI|prepopulates"`：先红后绿。
- `rtk npm run test:run -- src/views/admin/__tests__/AccountModelProbesView.spec.ts -t "loads manual model probe runs"`：先红后绿。
- `rtk npm run test:run -- src/components/admin/account/__tests__/ScheduledTestsPanel.spec.ts -t "preselects gpt-5.5"`：先红后绿。
- `rtk go test ./internal/pkg/openai -count=1`
- `rtk go test -tags unit ./internal/service -run "TestAccountProbeService_RunOpenAIAPIKeyPersistsSamples|TestAccountProbeService_RunManualModelValidationStoresEvidence|TestAccountTestService_OpenAIResponsesStreamEmitsFirstTokenMs|TestAccountTestService_OpenAIChatCompletionsStreamEmitsFirstTokenMs" -count=1`
- `rtk npm run test:run -- src/components/admin/account/__tests__/AccountTestModal.spec.ts src/components/account/__tests__/AccountTestModal.spec.ts src/components/admin/account/__tests__/ScheduledTestsPanel.spec.ts src/views/admin/__tests__/AccountsView.bulkEdit.spec.ts src/views/admin/__tests__/AccountModelProbesView.spec.ts src/views/admin/__tests__/AccountProbeReportsView.spec.ts`
- `rtk npm run typecheck`

## 校验结果

- `openai` 包测试通过，覆盖默认测试模型常量。
- service 聚焦测试通过，覆盖上游体检、模型验证和账号 OpenAI 请求测试关键路径。
- 前端 6 个测试文件共 29 个测试通过，覆盖账号测试弹窗、账号定时测试面板、非 API_KEY 批量账号测试、报告页批量/模型/定时表单和手动模型探针页面。
- `vue-tsc --noEmit` 通过。

---

日期：2026-06-03
执行者：Devil

## 本地提交、构建、部署与验证

本轮将上游体检报告排行榜、12 分钟运行超时失败、定时常规体检、手动模型验证、报告页批量模型验证以及默认测试模型 `gpt-5.5` 的变更提交后，使用本地 Docker 部署配置重建 `sub2api:multi-key-local` 镜像，并通过 `D:\sub2api-deploy\docker-compose.yml` 重建 `sub2api` 服务。

## 校验方式

- `git diff --cached --check`
- `git commit -m "feat(account-probe): add scheduled model probes"`
- `docker build --pull=false --build-arg COMMIT=<git rev-parse --short HEAD> --build-arg DATE=<utc-now> -t sub2api:multi-key-local .`
- `docker compose -f D:\sub2api-deploy\docker-compose.yml --env-file D:\sub2api-deploy\.env up -d --no-deps --force-recreate sub2api`
- `docker compose -f D:\sub2api-deploy\docker-compose.yml --env-file D:\sub2api-deploy\.env ps sub2api`
- `Invoke-RestMethod http://127.0.0.1:8080/health`
- `Invoke-WebRequest http://127.0.0.1:8080/admin/model-probes`
- `Invoke-WebRequest -Method Post http://127.0.0.1:8080/api/v1/admin/account-model-probe-runs`
- `Invoke-WebRequest -Method Post http://127.0.0.1:8080/api/v1/admin/account-model-probe-runs/batch`
- `Invoke-WebRequest http://127.0.0.1:8080/api/v1/admin/account-probe-runs/ranking?page=1&page_size=1`
- `docker exec sub2api /app/sub2api --version`

## 校验结果

- 暂存区空白检查通过。
- Docker 镜像构建通过；前端容器构建保留既有 Browserslist 过期提示、Vite 动态/静态导入提示和 chunk size 提示。
- `sub2api` 容器重建并进入 healthy 状态，端口映射为 `0.0.0.0:8080->8080/tcp`。
- `/health` 返回 `{"status":"ok"}`。
- `/admin/model-probes` 返回 HTTP 200 的前端 HTML。
- 模型探针单个/批量 POST 路由与排行榜 GET 路由未登录访问均返回 HTTP 401，确认路由存在并进入认证拦截。
- 容器二进制 `--version` 输出包含构建提交、版本号和构建时间。
- 启动日志显示 `ScheduledTestRunner started` 与 `Server started on 0.0.0.0:8080`；日志中的 `OpenAI upstream error ... insufficient_quota` 来自运行中 `/responses` 上游账号额度请求，不属于启动、迁移或监听失败。

---

日期：2026-06-03
执行者：Devil

## 上游排行榜改为弹窗

本轮将上游体检报告页的“上游排行榜”从报告表格上方的常驻区块改为操作区按钮触发的弹窗。排行榜列表、刷新按钮和选中上游后的每次得分历史仍保留在弹窗内；点击排行榜项继续筛选主报告列表，但排行榜不再占用或遮挡具体报告列表区域。

## 校验方式

- `rtk npm run test:run -- src/views/admin/__tests__/AccountProbeReportsView.spec.ts -t "ranking"`：先红后绿，红灯覆盖当前实现默认渲染排行榜项且没有弹窗入口。
- `rtk npm run test:run -- src/views/admin/__tests__/AccountProbeReportsView.spec.ts`
- `rtk npm run typecheck`
- `git diff --check`
- `rtk npm run dev -- --host 127.0.0.1 --port 5173`
- Playwright 打开 `http://127.0.0.1:5173/admin/probe-reports`

## 校验结果

- 排行榜切片测试通过，覆盖默认不渲染排行榜项、点击按钮打开弹窗、在弹窗中点击上游后筛选报告列表并展示历史得分。
- `AccountProbeReportsView.spec.ts` 10 个测试通过。
- `vue-tsc --noEmit` 通过。
- `git diff --check` 通过。
- 浏览器可打开本地 dev server；未登录访问管理页按路由守卫跳转到登录页，因此实际管理页视觉烟测需要登录态。

---

日期：2026-06-03
执行者：Devil

## 上游体检请求样本不展示/批量失败排查

本轮排查 8080 本地容器与 PostgreSQL `sub2api` 数据库，确认不是单个 foyeapi 问题：`account_probe_runs` 中 14:27 批次多个上游账号均失败，错误一致为 `pq: null value in column "output_text" of relation "account_probe_samples" violates not-null constraint`。根因是 `SaveAccountProbeSample` 对空 `output_text` 使用 `NULLIF($16,'')`，而线上表结构 `account_probe_samples.output_text` 是 `NOT NULL DEFAULT ''`；当上游请求失败或无输出文本时，sample 保存失败，run 被标记失败，后续详情页自然看不到请求样本。

同时确认成功 run 的 sample 其实已落库，但报告详情页只展示状态、延迟、token 和错误码，没有直接展示 `upstream_endpoint`、`output_text`、失败错误正文，造成“请求不展示”的观感。修复后：空输出文本按空字符串落库，空 `validation_evidence` 保存为 `[]`；报告详情样本表展示请求地址、样本标签/类型、输出文本和错误正文。

## 校验方式

- `docker exec sub2api /app/sub2api --version`
- `docker exec sub2api-postgres psql -U sub2api -d sub2api -Atc "<account_probe_runs/account_probe_samples 查询>"`
- `rtk go test ./internal/repository -run "TestAccountProbeRepositorySaveFailedSampleAllowsEmptyOutputText|TestAccountProbeRepositorySaveSampleStoresEmptyValidationEvidenceArray|TestAccountProbeRepositorySaveSampleStoresValidationEvidence|TestAccountProbeRepositoryListSamplesLoadsValidationEvidence" -count=1`
- `rtk go test ./internal/repository -run "TestAccountProbeRepository" -count=1`
- `rtk npm run test:run -- src/views/admin/__tests__/AccountProbeReportsView.spec.ts`
- `rtk npm run typecheck`
- `git diff --check`

## 校验结果

- 运行容器版本为 `a24b2d62`，问题发生在已部署旧版本；数据库证据显示 foyeapi、aisz、mikuapi、zz1cc、吱吱鼠等多个上游同类失败。
- repository 聚焦测试 4 个通过，`TestAccountProbeRepository` 组 6 个通过。
- 报告页 Vitest 10 个通过，覆盖详情抽屉展示请求地址、输出文本和失败错误正文。
- `vue-tsc --noEmit` 通过。
- `git diff --check` 通过。
- 当前未重建本地容器，因为工作区同时存在本轮之外的未提交改动，直接发布会把无关改动一起带到 8080。

---

日期：2026-06-03
执行者：Devil

## 模型探针批量入口与正版评分拆分

本轮将批量模型探测从上游体检报告页移到模型探针页：报告页不再展示批量模型验证按钮，也不再在普通报告列表/排行榜默认混入 `model_validation` 记录；模型探针页新增批量验证弹窗，默认按 `platform=openai`、`type=apikey` 加载 API_KEY 账号并全选。模型验证评分改为只按验证证据通过率计算，输出“正版 gpt-5.5 / 疑似掺水 / 掺水明显”结论，不再复用上游体检的延迟、首 token、稳定性、token 消耗评分；模型验证失败也不再写入 OpenAI path health。

## 校验方式

- `go test ./internal/service -run "TestScoreAccount|TestAccountProbeService" -count=1`
- `go test ./internal/repository -run "TestAccountProbeRepository|TestAccountProbeReportWhere" -count=1`
- `npm exec vitest -- src/views/admin/__tests__/AccountModelProbesView.spec.ts src/views/admin/__tests__/AccountProbeReportsView.spec.ts --run`
- `npm run typecheck`

## 校验结果

- service 聚焦测试通过，覆盖模型验证通过率评分、非 gpt-5.5 不标正版、模型验证证据落库和不污染 path health。
- repository 聚焦测试通过，覆盖样本证据保存、空输出落库和报告默认排除模型验证记录。
- 前端 Vitest 13 个测试通过，覆盖模型页批量默认全选 API_KEY 账号、批量提交、报告页移除批量模型入口和详情样本展示。
- `vue-tsc --noEmit` 通过。

---

日期：2026-06-03
执行者：Devil

## 模型探针拆分功能提交、构建、部署验证

本轮将模型探针拆分功能提交为 `1c05fb68 feat(account-probe): split model probes from health reports`，随后构建 `sub2api:multi-key-local` 镜像并使用 `D:\sub2api-deploy\docker-compose.yml` 重建本地 `sub2api` 容器。

## 校验方式

- `git diff --cached --check`
- `docker build -t sub2api:multi-key-local .`
- `docker compose -f D:\sub2api-deploy\docker-compose.yml up -d --no-deps --force-recreate sub2api`
- `docker compose -f D:\sub2api-deploy\docker-compose.yml ps`
- `Invoke-WebRequest http://127.0.0.1:8080/health`
- `docker exec sub2api /app/sub2api --version`
- `curl.exe -i -s http://127.0.0.1:8080/admin/model-probes`
- `curl.exe -i -s http://127.0.0.1:8080/admin/probe-reports`
- `curl.exe -i -s -X POST http://127.0.0.1:8080/api/v1/admin/account-model-probe-runs/batch ...`
- `curl.exe -i -s http://127.0.0.1:8080/api/v1/admin/account-probe-runs?page=1&page_size=1`
- `curl.exe -i -s http://127.0.0.1:8080/api/v1/admin/account-probe-runs/ranking?limit=1`

## 校验结果

- Docker build 通过，前端 production build 只出现既有 Browserslist/chunk 警告；镜像 ID 为 `sha256:da602c21e66b961bca34d3796858db83b0a6b87dbcaccf7f49db1f348a73ef1b`。
- `docker compose ps` 显示 `sub2api`、PostgreSQL、Redis 均 `healthy`，`sub2api` 映射 `0.0.0.0:8080->8080/tcp`。
- `/health` 返回 HTTP 200 与 `{"status":"ok"}`。
- 容器版本输出 `Sub2API 0.1.133 (commit: docker, built: 2026-06-03T06:45:01Z)`。
- `/admin/model-probes` 与 `/admin/probe-reports` 返回 HTTP 200 前端 HTML。
- `/api/v1/admin/account-model-probe-runs/batch`、`/api/v1/admin/account-probe-runs`、`/api/v1/admin/account-probe-runs/ranking` 未登录访问均返回 HTTP 401 `UNAUTHORIZED`，确认路由存在并进入管理端认证拦截。

---

日期：2026-06-03
执行者：Devil

## 模型探针详情改为弹窗

本轮将模型探针列表的“查看”详情从页面下方内联区域改为 `BaseDialog` 弹窗。点击查看时立即打开弹窗并加载详情；弹窗内展示状态、得分结论、时间、样本输出和验证证据；关闭弹窗时清空当前详情状态。

## 校验方式

- `npm exec vitest -- src/views/admin/__tests__/AccountModelProbesView.spec.ts --run`
- `npm run typecheck`

## 校验结果

- `AccountModelProbesView.spec.ts` 3 个测试通过，覆盖详情弹窗展示验证证据并可关闭。
- `vue-tsc --noEmit` 通过。

---

日期：2026-06-03
执行者：Devil

## 模型探针详情弹窗提交、构建、部署验证

本轮将模型探针详情弹窗改动提交为 `e25826ba feat(account-probe): show model probe detail in dialog`，随后构建 `sub2api:multi-key-local` 镜像并使用 `D:\sub2api-deploy\docker-compose.yml` 重建本地 `sub2api` 容器。

## 校验方式

- `git diff --cached --check`
- `docker build -t sub2api:multi-key-local .`
- `docker compose -f D:\sub2api-deploy\docker-compose.yml up -d --no-deps --force-recreate sub2api`
- `docker compose -f D:\sub2api-deploy\docker-compose.yml ps`
- `curl.exe -s http://127.0.0.1:8080/health`
- `docker exec sub2api /app/sub2api --version`
- `Invoke-WebRequest http://127.0.0.1:8080/admin/model-probes`
- `Invoke-WebRequest http://127.0.0.1:8080/admin/probe-reports`
- `curl.exe -i -s -X POST http://127.0.0.1:8080/api/v1/admin/account-model-probe-runs/batch ...`
- `curl.exe -i -s http://127.0.0.1:8080/api/v1/admin/account-probe-runs?page=1&page_size=1`

## 校验结果

- Docker build 通过，前端 production build 只出现既有 Browserslist/chunk 警告；镜像 ID 为 `sha256:131a9604ef2ff96364d8b6e96ddd5e97cb65a8acd452e0763907aa78fd958982`。
- `docker compose ps` 显示 `sub2api`、PostgreSQL、Redis 均 `healthy`，`sub2api` 映射 `0.0.0.0:8080->8080/tcp`。
- `/health` 返回 `{"status":"ok"}`。
- 容器版本输出 `Sub2API 0.1.133 (commit: docker, built: 2026-06-03T07:17:34Z)`。
- `/admin/model-probes` 与 `/admin/probe-reports` 返回 HTTP 200 前端 HTML。
- `/api/v1/admin/account-model-probe-runs/batch` 与 `/api/v1/admin/account-probe-runs` 未登录访问均返回 HTTP 401 `UNAUTHORIZED`，确认路由存在并进入管理端认证拦截。

---

日期：2026-06-03
执行者：Devil

## 模型探针分页与查询

本轮为模型探针页面最近探针列表增加关键字查询和分页控制。查询条件用于账号、模型或错误文本检索；提交查询后回到第一页；翻页和切换每页数量时继续携带当前查询关键字，并固定请求 `mode=model_validation`，避免混入普通上游体检记录。

## 校验方式

- `npm exec vitest -- src/views/admin/__tests__/AccountModelProbesView.spec.ts --run`
- `npm run typecheck`
- `git diff --check`

## 校验结果

- 新增测试先观察到缺少 `model-probe-keyword` 时失败，再实现查询和分页功能。
- `AccountModelProbesView.spec.ts` 4 个测试通过，覆盖关键字查询、翻页和每页数量切换时的请求参数。
- `vue-tsc --noEmit` 通过。
- `git diff --check` 通过。

---

日期：2026-06-03
执行者：Devil

## 模型探针分页查询提交、构建、部署验证

本轮将模型探针分页查询功能提交为 `1705bbd4 feat(account-probe): add model probe search pagination`，随后构建 `sub2api:multi-key-local` 镜像并使用 `D:\sub2api-deploy\docker-compose.yml` 重建本地 `sub2api` 容器。

## 校验方式

- `git diff --cached --check`
- `docker build -t sub2api:multi-key-local .`
- `docker compose -f D:\sub2api-deploy\docker-compose.yml up -d --no-deps --force-recreate sub2api`
- `docker compose -f D:\sub2api-deploy\docker-compose.yml ps`
- `docker exec sub2api /app/sub2api --version`
- `Invoke-WebRequest http://127.0.0.1:8080/health`
- `Invoke-WebRequest http://127.0.0.1:8080/admin/model-probes`
- `Invoke-WebRequest http://127.0.0.1:8080/admin/probe-reports`
- `Invoke-WebRequest -SkipHttpErrorCheck http://127.0.0.1:8080/api/v1/admin/account-probe-runs?page=1&page_size=20&mode=model_validation&keyword=rayapi`
- `Invoke-WebRequest -SkipHttpErrorCheck -Method Post http://127.0.0.1:8080/api/v1/admin/account-model-probe-runs/batch ...`
- `Invoke-WebRequest -SkipHttpErrorCheck http://127.0.0.1:8080/api/v1/admin/account-probe-runs/ranking?limit=1`

## 校验结果

- Docker build 通过，前端 production build 只出现既有 Node deprecation、Browserslist 和 chunk size 警告；镜像 ID 为 `sha256:9baf8d0360252538e8a58c85fa3df00be83cf728629b939f424be2124d9cda2e`，创建时间 `2026-06-03T07:51:46Z`。
- `docker compose ps` 显示 `sub2api`、PostgreSQL、Redis 均 `healthy`，`sub2api` 映射 `0.0.0.0:8080->8080/tcp`。
- 容器版本输出 `Sub2API 0.1.133 (commit: docker, built: 2026-06-03T07:50:49Z)`。
- `/health` 返回 HTTP 200 与 `{"status":"ok"}`。
- `/admin/model-probes` 与 `/admin/probe-reports` 返回 HTTP 200 前端 HTML。
- `/api/v1/admin/account-probe-runs?mode=model_validation&keyword=rayapi`、`/api/v1/admin/account-model-probe-runs/batch` 与 `/api/v1/admin/account-probe-runs/ranking` 未登录访问均返回 HTTP 401 `UNAUTHORIZED`，确认路由存在并进入管理端认证拦截。

---

日期：2026-06-03
执行者：Devil

## 清空模型探针数据

本轮按用户要求清空本地部署数据库中的模型探针数据。模型探针数据范围依据后端代码和页面查询条件界定为 `account_probe_runs.mode = 'model_validation'` 的运行记录，以及这些运行记录对应的 `account_probe_samples` 样本明细；普通上游体检报告 `mode = 'standard'` 不在本次删除范围内。

## 校验方式

- `docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}"`
- `docker compose -f D:\sub2api-deploy\docker-compose.yml ps`
- `docker exec sub2api-postgres psql -U sub2api -d sub2api ...`

## 校验结果

- 当前本地部署容器为 `sub2api`、`sub2api-postgres`、`sub2api-redis`，`sub2api` 显示 `Up ... (healthy)`，端口映射 `0.0.0.0:8080->8080/tcp`。
- 删除前：`account_probe_runs` 中 `mode='model_validation'` 共 39 条；对应 `account_probe_samples` 共 40 条；状态分布为 failed 1、partial 3、running 29、success 6。
- 已在 PostgreSQL 事务中删除 39 条模型探针 run 和 40 条模型探针 sample。
- 删除后复查：`account_probe_runs` 中 `mode='model_validation'` 为 0；对应 sample 为 0。
- 删除后剩余普通上游体检报告仍为 `standard`：failed 2、partial 3、success 20，确认未清理普通体检数据。

---

日期：2026-06-03
执行者：Devil

## 模型探针批量接口 500 修复

本轮按用户要求直接查看本地日志复现 `POST http://localhost:8080/api/v1/admin/account-model-probe-runs/batch` 报错。日志显示 2026-06-03 15:53:55 至 15:54:30 多次请求返回 500，失败位置为 `account_handler.go:1542`，错误为 `no api key available`；数据库中同时间段存在多条 `account_probe_runs.mode='model_validation'` 且 `status='running'` 的遗留记录。根因是批量 handler 顺序调用 `Start` 创建运行记录后，遇到后续账号启动失败就直接返回 500，导致前面已经创建的 running 记录没有进入后台 `RunExisting`。

修复后，批量模型探针会收集启动失败账号并继续处理可接受账号；只要至少有一个账号成功创建运行记录，就启动后台任务并返回 202，响应包含 `accepted_count`、`skipped_count`、`skipped` 和 `runs`。前端同步响应类型、默认只拉取 active OpenAI API Key，并在部分跳过时展示跳过数量。

## 校验方式

- `go test -tags unit ./internal/handler/admin -run "TestAccountModelProbeBatch|TestAccountProbeCreate|TestAccountModelProbeCreate" -count=1`
- `go test -tags unit ./internal/handler/admin -run "Test.*Probe" -count=1`
- `npm exec vitest -- src/views/admin/__tests__/AccountModelProbesView.spec.ts --run`
- `npm run typecheck`
- `git diff --check`
- `docker build -t sub2api:multi-key-local .`
- `docker compose -f D:\sub2api-deploy\docker-compose.yml up -d --no-deps --force-recreate sub2api`
- `curl.exe --noproxy "*" http://localhost:8080/health`
- `curl.exe --noproxy "*" http://localhost:8080/admin/model-probes`
- `curl.exe --noproxy "*" -X POST http://localhost:8080/api/v1/admin/account-model-probe-runs/batch ...`

## 校验结果

- 后端 handler 聚焦测试通过；新增回归覆盖单个账号 `Start` 返回 `no api key available` 时仍返回 202，并继续启动已接受账号的后台任务。
- 更宽的 `Test.*Probe` handler 切片通过。
- `AccountModelProbesView.spec.ts` 4 个测试通过，覆盖 active 账号筛选和部分跳过提示。
- `vue-tsc --noEmit` 通过。
- `git diff --check` 通过，仅提示 Windows 工作区中 JSONL 文档后续可能 LF 转 CRLF。
- Docker build 通过，镜像 ID 为 `sha256:f57350ea3378b16727928f1b0318eaf57abf4f449e4ae80cbb3bae430f8afa8c`，容器版本输出 `Sub2API 0.1.133 (commit: docker, built: 2026-06-03T08:17:24Z)`。
- 本地 `sub2api` 容器重建后 healthy，`/health` 返回 HTTP 200 与 `{"status":"ok"}`，`/admin/model-probes` 返回 HTTP 200 HTML，未登录批量 API 返回 HTTP 401 `UNAUTHORIZED`，确认新容器路由正常进入管理端认证拦截。

---

日期：2026-06-03
执行者：Devil

## 可信对比完整移植

本轮将 juhe-ai `feature/20250602` 的模型检测可信对比完整移植进 sub2api 的模型探针：后端在 `AccountProbeService` 中补上可信账号核心 suite、可信对比核心摘要和 6 类分布相似度摘要，单个模型探针请求与批量请求继续透传 `trusted_comparison_account_id`；前端模型探针页面继续展示可信账号、相似度、覆盖率和目标/可信通过率，类型与文案同步保持一致。

## 校验方式

- `go test -tags unit ./internal/service -run "Test.*ModelValidation|TestScoreAccountModelValidation" -count=1`
- `go test -tags unit ./internal/handler/admin -run "Test.*Probe" -count=1`
- `npm exec vitest -- src/views/admin/__tests__/AccountModelProbesView.spec.ts --run`
- `npm run typecheck`
- `git diff --check`
- `go test -tags unit ./internal/service ./internal/handler/admin -count=1 -timeout=15m`

## 校验结果

- 后端 `internal/service` 聚焦测试通过，覆盖模型验证评分、可信对比核心摘要和分布相似度摘要。
- 后端 `internal/handler/admin` probe 切片通过，覆盖单跑、批跑和后台线程参数透传。
- 前端 `AccountModelProbesView.spec.ts` 4 个测试通过，`vue-tsc --noEmit` 通过。
- `git diff --check` 通过，仅提示 `docs/feature_list.jsonl`、`docs/process_list.jsonl` 会在 Windows 工作区下由 LF 转 CRLF。
- 组合包 `go test -tags unit ./internal/service ./internal/handler/admin -count=1 -timeout=15m` 超时，按用户要求跳过更大包继续等待。

---

日期：2026-06-03
执行者：Devil

## 模型探针请求响应证据展示

本轮根据用户反馈补齐报告详情弹窗中的每次模型验证输入输出证据。后端新增 `account_probe_samples.request_prompt`、`request_body`、`response_body`，执行 OpenAI API Key 模型探针时保存原始 prompt、去除 Authorization 的请求 JSON 或 GET 摘要、上游非流式响应体或流式 `response.completed.response`。前端报告详情样本列表新增 Prompt、Request Body、Response Body 展示区，便于对照每次请求为什么得分一致或失败。

## 校验方式

- `go test -tags unit ./internal/service ./internal/repository -run TestAccountProbe -count=1`
- `npm run test:run -- AccountProbeReportsView.spec.ts`
- `npm run typecheck`
- `go build ./cmd/server`
- `npm run build`
- `docker compose -f D:\sub2api-deploy\docker-compose.yml ps`
- `Invoke-WebRequest http://127.0.0.1:8080/health`
- 本地生成短期管理员 JWT 后调用 `POST /api/v1/admin/account-model-probe-runs`
- `GET /api/v1/admin/account-probe-runs/120`

## 校验结果

- 后端 `internal/service` 与 `internal/repository` 的 `TestAccountProbe` 聚焦测试通过。
- 前端 `AccountProbeReportsView.spec.ts` 10 个测试通过，覆盖详情样本展示新增字段；`vue-tsc --noEmit` 通过。
- 后端 `go build ./cmd/server` 退出码 0；前端生产构建通过，仍有项目既有 Browserslist 过期、Vite dynamic import 和 chunk size 警告。
- 本地 compose 中 `sub2api`、`sub2api-postgres`、`sub2api-redis` 均 healthy，`/health` 返回 HTTP 200 与 `{"status":"ok"}`。
- 真实模型探针：`POST /api/v1/admin/account-model-probe-runs` 返回 202，创建 `run_id=120`，最终状态 `partial`；数据库中该 run 生成 19 条样本，18 条有 `request_prompt`，19 条有 `request_body`，19 条有 `response_body`。
- 详情 API `GET /api/v1/admin/account-probe-runs/120` 返回 HTTP 200，样本 JSON 中可见新增请求体和返回体字段；行为验证样本中可见 prompt 字段。

## 提交后部署验证

- `docker build -t sub2api:multi-key-local .` 通过，镜像 manifest list 为 `sha256:c8a65b24a30d80f6d83432f7001d8659901098f664971ddfb38b2b1bc5240a60`。
- `docker compose -f D:\sub2api-deploy\docker-compose.yml up -d --no-deps --force-recreate sub2api` 通过。
- 部署后 `sub2api` 容器状态为 `running`，健康状态为 `healthy`。
- `Invoke-WebRequest http://127.0.0.1:8080/health` 返回 HTTP 200。
- `GET /api/v1/admin/account-probe-runs/120` 返回 HTTP 200，19 条样本中 18 条带 `request_prompt`，19 条带 `request_body`，19 条带 `response_body`。

---

日期：2026-06-03
执行者：Devil

## 模型探针删除/批量删除

本轮为管理端模型探针页面增加删除能力：列表新增当前页选择列，运行中的探针不可选择；每行新增删除按钮，批量删除按钮会删除当前已选的非 running 探针；删除走现有 `DELETE /api/v1/admin/account-probe-runs`，后端仍按既有规则跳过运行中的记录并删除对应样本。

## 校验方式

- `npm exec vitest -- src/views/admin/__tests__/AccountModelProbesView.spec.ts --run`
- `npm run typecheck`
- `git diff --check`
- `npm run build`
- `Invoke-WebRequest http://127.0.0.1:5174/admin/model-probes`

## 校验结果

- TDD 红灯已观察：新增删除测试在实现前失败于找不到单删按钮和批量选择框。
- `AccountModelProbesView.spec.ts` 6 个测试通过，覆盖单删、当前页批删、running 记录跳过选择、删除后刷新列表。
- `vue-tsc --noEmit` 通过；前端生产构建通过，仍有项目既有 Browserslist、Vite dynamic import 和 chunk size 警告。
- `git diff --check` 通过，仅提示 `docs/process_list.jsonl` 后续可能 LF 转 CRLF。
- 临时 Vite dev server 的 `/admin/model-probes` HTTP smoke 返回 200 且 HTML 包含 Vue 挂载点；5174 端口进程已停止。
- Playwright MCP 浏览器验证被现有 browser user-data-dir 占用阻塞，未执行截图。

---

日期：2026-06-03
执行者：Devil

## 模型探针删除/批量删除提交后部署验证

本轮按用户要求将模型探针删除功能提交、构建、部署到本地 `sub2api` 容器，并完成部署后 smoke。Docker 构建上下文按 `.dockerignore` 排除 `docs/` 和 `*.md`，部署镜像使用已提交的前端/后端源码输入。

## 校验方式

- `git diff --cached --check`
- `git commit -m "feat(admin): add model probe deletion"`
- `docker build -t sub2api:multi-key-local .`
- `docker compose -f D:\sub2api-deploy\docker-compose.yml up -d --no-deps --force-recreate sub2api`
- `docker inspect sub2api --format '{{.State.Status}} {{.State.Health.Status}}'`
- `Invoke-WebRequest http://127.0.0.1:8080/health`
- `Invoke-WebRequest http://127.0.0.1:8080/admin/model-probes`
- `Invoke-WebRequest http://127.0.0.1:8080/admin/probe-reports`
- 未登录请求模型探针列表、删除和批量创建管理端 API

## 校验结果

- 功能提交成功，初始提交为 `dee9c07d feat(admin): add model probe deletion`。
- Docker build 通过，镜像 digest 为 `sha256:2c4be589d090471d1cde748f45a4315418bd736053fe6774d8bec0922d440b84`；构建仍有项目既有 Browserslist、Vite dynamic import 和 chunk size 警告。
- 本地 compose 重建 `sub2api` 成功，容器状态 `running healthy`。
- `/health` 返回 HTTP 200 与 `{"status":"ok"}`。
- `/admin/model-probes` 和 `/admin/probe-reports` 均返回 HTTP 200 HTML，包含 Vue 挂载点。
- `GET /api/v1/admin/account-probe-runs?mode=model_validation&page=1&page_size=20` 未登录返回 HTTP 401。
- `DELETE /api/v1/admin/account-probe-runs` 未登录返回 HTTP 401。
- `POST /api/v1/admin/account-model-probe-runs/batch` 未登录返回 HTTP 401。
- 容器尾部日志未见 panic/fatal；仅有一条既有 `/responses` `context canceled` WARN，与本次模型探针页面 smoke 无关。

---

日期：2026-06-03
执行者：Devil

## 模型探针详情展示 Prompt、请求体和返回体

本轮将模型探针页面自身的详情弹窗补齐到与报告详情一致：每个样本在输出文本和验证证据之外，额外展示 `request_prompt`、`request_body`、`response_body`，用于直接复盘模型验证时发送给上游的 Prompt、脱敏请求体和上游返回体。后端字段、仓储和类型已存在，本次只补模型探针页前端展示和测试覆盖。

## 校验方式

- `npm exec vitest -- src/views/admin/__tests__/AccountModelProbesView.spec.ts --run`
- `npm run typecheck`
- `git diff --check`
- `npm run build`

## 校验结果

- TDD 红灯已观察：新增 transcript 断言后，`AccountModelProbesView.spec.ts` 失败于找不到 `admin.accountModelProbes.requestPrompt`。
- 补齐模板与中英文文案后，`AccountModelProbesView.spec.ts` 6 个测试全部通过。
- `npm run typecheck` 通过，`vue-tsc --noEmit` 退出码 0。
- `git diff --check` 通过。
- `npm run build` 通过；保留项目既有 Browserslist caniuse-lite 过期、Vite dynamic import 和 chunk size 警告。

---

日期：2026-06-04
执行者：Devil

## 模型探针解析结果展示提交后部署验证

本轮已将 `fix(admin): show parsed model probe failures` 提交后构建到 `sub2api:multi-key-local`，并重建 `D:\sub2api-deploy\docker-compose.yml` 管理的本地 `sub2api` 容器。构建继续使用官方 registry 全路径基础镜像参数，以避开本机 Docker 阿里镜像源对基础镜像的 403。

## 校验方式

- `git commit -m "fix(admin): show parsed model probe failures"`
- `docker build --build-arg NODE_IMAGE=registry-1.docker.io/library/node:24-alpine --build-arg GOLANG_IMAGE=registry-1.docker.io/library/golang:1.26.3-alpine --build-arg ALPINE_IMAGE=registry-1.docker.io/library/alpine:3.21 -t sub2api:multi-key-local .`
- `docker compose -f D:\sub2api-deploy\docker-compose.yml up -d --no-deps --force-recreate sub2api`
- `docker compose -f D:\sub2api-deploy\docker-compose.yml ps`
- `Invoke-WebRequest http://127.0.0.1:8080/health`
- `Invoke-WebRequest http://127.0.0.1:8080/admin/model-probes`
- `Invoke-WebRequest http://127.0.0.1:8080/admin/probe-reports`
- 未登录请求模型探针列表、删除和批量创建管理端 API
- Playwright 打开 `http://127.0.0.1:8080/admin/model-probes`
- `docker logs --tail 200 sub2api | Select-String -Pattern 'panic|fatal'`

## 校验结果

- 提交成功，初始提交为 `cd382364 fix(admin): show parsed model probe failures`。
- Docker build 通过，镜像 manifest list 为 `sha256:5b94228ad790e269556cbf55b2d2fae1642b8d325e7bfc7b06efea1b67a15578`；构建仍保留既有 Browserslist、Vite dynamic import 和 chunk size 警告。
- 本地 compose 重建 `sub2api` 成功，容器状态 `running healthy`。
- `/health` 返回 HTTP 200 与 `{"status":"ok"}`。
- `/admin/model-probes` 和 `/admin/probe-reports` 均返回 HTTP 200 HTML，包含 Vue 挂载点。
- `GET /api/v1/admin/account-probe-runs?mode=model_validation&page=1&page_size=20` 未登录返回 HTTP 401。
- `DELETE /api/v1/admin/account-probe-runs` 未登录返回 HTTP 401。
- `POST /api/v1/admin/account-model-probe-runs/batch` 未登录返回 HTTP 401。
- Playwright 实际打开 `/admin/model-probes` 后按预期跳转到 `/login?redirect=/admin/model-probes`，登录页标题为 `Login - Sub2API`。
- 容器日志近 200 行未匹配 `panic` 或 `fatal`。

---

日期：2026-06-04
执行者：Devil

## 模型探针失败原因展示提交后部署验证

本轮已将 `fix(admin): show model probe failure reasons` 提交后构建到 `sub2api:multi-key-local`，并重建 `D:\sub2api-deploy\docker-compose.yml` 管理的本地 `sub2api` 容器。首次 Docker build 因本机 Docker registry mirror `https://1d75j62o.mirror.aliyuncs.com` 对 `alpine/node/golang` 基础镜像返回 403 失败；随后通过官方 registry 全路径拉取基础镜像，并用等价 build args 构建成功。

## 校验方式

- `git commit -m "fix(admin): show model probe failure reasons"`
- `docker pull registry-1.docker.io/library/alpine:3.21`
- `docker pull registry-1.docker.io/library/node:24-alpine`
- `docker pull registry-1.docker.io/library/golang:1.26.3-alpine`
- `docker build --build-arg NODE_IMAGE=registry-1.docker.io/library/node:24-alpine --build-arg GOLANG_IMAGE=registry-1.docker.io/library/golang:1.26.3-alpine --build-arg ALPINE_IMAGE=registry-1.docker.io/library/alpine:3.21 -t sub2api:multi-key-local .`
- `docker compose -f D:\sub2api-deploy\docker-compose.yml up -d --no-deps --force-recreate sub2api`
- `docker compose -f D:\sub2api-deploy\docker-compose.yml ps`
- `Invoke-WebRequest http://127.0.0.1:8080/health`
- `Invoke-WebRequest http://127.0.0.1:8080/admin/model-probes`
- `Invoke-WebRequest http://127.0.0.1:8080/admin/probe-reports`
- 未登录请求模型探针列表、删除和批量创建管理端 API
- Playwright 打开 `http://127.0.0.1:8080/admin/model-probes`
- `docker logs --tail 200 sub2api | Select-String -Pattern 'panic|fatal'`

## 校验结果

- 提交成功，初始提交为 `37464e9e fix(admin): show model probe failure reasons`。
- Docker build 通过，镜像 manifest list 为 `sha256:6b0b23bb51b004e9c24b738653466c8e998ded49f7e24d1c0886cbdf28303821`；构建仍保留既有 Browserslist、Vite dynamic import 和 chunk size 警告。
- 本地 compose 重建 `sub2api` 成功，容器状态 `running healthy`。
- `/health` 返回 HTTP 200 与 `{"status":"ok"}`。
- `/admin/model-probes` 和 `/admin/probe-reports` 均返回 HTTP 200 HTML，包含 Vue 挂载点。
- `GET /api/v1/admin/account-probe-runs?mode=model_validation&page=1&page_size=20` 未登录返回 HTTP 401。
- `DELETE /api/v1/admin/account-probe-runs` 未登录返回 HTTP 401。
- `POST /api/v1/admin/account-model-probe-runs/batch` 未登录返回 HTTP 401。
- Playwright 实际打开 `/admin/model-probes` 后按预期跳转到 `/login?redirect=/admin/model-probes`，登录页标题为 `Login - Sub2API`，页面渲染 `Sub2API`。
- 容器日志近 200 行未匹配 `panic` 或 `fatal`。

---

日期：2026-06-04
执行者：Devil

## 模型探针未通过时展示模型返回和解析结果

本轮分析后端模型验证链路确认：`sample.output_text` 保存从上游响应解析出的模型文本输出，`validation_evidence.observed` 保存各验证项实际解析或观察到的结果，`validation_evidence.expected` 保存导致判定不通过的期望值。为便于定位 `model_validation_failed`，模型探针详情在失败样本下新增“模型返回 / 解析结果”区块，展示模型输出、解析结果和期望值；仍不展示原始 `response_body` 返回体。

## 校验方式

- `npm exec vitest -- src/views/admin/__tests__/AccountModelProbesView.spec.ts --run`
- `npm run typecheck`
- `git diff --check`
- `npm run build`

## 校验结果

- TDD 红灯已观察：新增断言后，`AccountModelProbesView.spec.ts` 失败于缺少 `admin.accountModelProbes.modelResult`。
- 补齐失败样本“模型返回 / 解析结果”区块后，`AccountModelProbesView.spec.ts` 6 个测试全部通过。
- `npm run typecheck` 通过，`vue-tsc --noEmit` 退出码 0。
- `git diff --check` 通过。
- `npm run build` 通过；保留项目既有 Browserslist caniuse-lite 过期、Vite dynamic import 和 chunk size 警告。

---

日期：2026-06-04
执行者：Devil

## 模型探针失败原因展示并隐藏返回体

本轮按用户要求调整模型探针页面详情弹窗：样本报错或模型验证未通过时，单独展示错误码、错误信息和未通过证据中的实际/期望原因；模型探针详情不再展示 `response_body` 返回体。报告查询页的原始响应证据不在本轮范围内，继续保留。

## 校验方式

- `npm exec vitest -- src/views/admin/__tests__/AccountModelProbesView.spec.ts --run`
- `npm run typecheck`
- `git diff --check`
- `npm run build`

## 校验结果

- TDD 红灯已观察：新增断言后，`AccountModelProbesView.spec.ts` 失败于详情仍包含 `admin.accountModelProbes.responseBody`。
- 补齐失败原因聚合并移除模型探针详情返回体展示后，`AccountModelProbesView.spec.ts` 6 个测试全部通过。
- `npm run typecheck` 通过，`vue-tsc --noEmit` 退出码 0。
- `git diff --check` 通过。
- `npm run build` 通过；保留项目既有 Browserslist caniuse-lite 过期、Vite dynamic import 和 chunk size 警告。

---

日期：2026-06-04
执行者：Devil

## BazaarLink 模型探针对接

本轮按用户要求接入 BazaarLink probe API 到模型探针页面：新增独立 BazaarLink 验证按钮和弹窗，用两个按钮区分快速验证与完整验证；提交后后端立即返回后台任务，不等待外部检测完成；列表和详情新增探针来源，区分本地校验与 BazaarLink API。前端只提交账号 ID、模型和模式，不接收 API key；后端从账号已保存的 OpenAI API key 组装 BazaarLink 请求，持久化的请求体、响应体和错误摘要均脱敏。

## 校验方式

- `go test -tags unit ./internal/service -run TestAccountProbeService_RunBazaarLinkRedactsSecretFieldsFromErrorMessage -count=1`
- `go test -tags unit ./internal/service ./internal/handler/admin -run "Test.*Bazaar|Test.*ModelProbe" -count=1`
- `npm exec vitest -- src/views/admin/__tests__/AccountModelProbesView.spec.ts --run`
- `npm run typecheck`
- `git diff --check`
- `npm run build`

## 校验结果

- TDD 红灯已观察：前端新增 BazaarLink 断言后先失败于缺少独立按钮、来源列和结构化结果展示。
- TDD 红灯已观察：后端 BazaarLink 错误摘要用例先失败于保存的错误信息仍包含 `sk-live-secret`。
- 补齐后端错误摘要脱敏后，BazaarLink 单用例通过。
- 后端 service/admin handler 聚焦测试通过。
- 前端 `AccountModelProbesView.spec.ts` 9 个测试全部通过。
- `npm run typecheck` 通过，`vue-tsc --noEmit` 退出码 0。
- `git diff --check` 通过。
- `npm run build` 通过；保留项目既有 Browserslist caniuse-lite 过期、Vite dynamic import 和 chunk size 警告。

---

日期：2026-06-04
执行者：Devil

## BazaarLink 模型探针推送构建部署验证

本轮按用户要求执行推送、构建、部署和验证。功能提交为 `a701d841 feat(admin): add BazaarLink model probes`，本地构建产物已部署到 `D:\sub2api-deploy\docker-compose.yml` 管理的 `sub2api` 容器，监听 `http://127.0.0.1:8080`。远端推送被 GitHub 权限阻塞：当前凭据用户 `biaoxing-github` 没有 `Wei-Shaw/sub2api` 写权限，因此远端分支未更新。

## 校验方式

- `git push -u origin feature/account-api-key-rotation`
- `docker build -t sub2api:multi-key-local --build-arg NODE_IMAGE=registry-1.docker.io/library/node:24-alpine --build-arg GOLANG_IMAGE=registry-1.docker.io/library/golang:1.26.3-alpine --build-arg ALPINE_IMAGE=registry-1.docker.io/library/alpine:3.21 --build-arg POSTGRES_IMAGE=registry-1.docker.io/library/postgres:18-alpine .`
- `docker compose -f D:\sub2api-deploy\docker-compose.yml up -d --force-recreate sub2api`
- `docker inspect sub2api --format ...`
- `Invoke-WebRequest http://127.0.0.1:8080/health`
- `Invoke-WebRequest http://127.0.0.1:8080/admin/model-probes`
- `Invoke-WebRequest http://127.0.0.1:8080/api/v1/admin/account-probe-runs?page=1&page_size=1 -SkipHttpErrorCheck`
- `Invoke-WebRequest http://127.0.0.1:8080/api/v1/admin/account-model-probe-runs/bazaarlink -Method Post -SkipHttpErrorCheck`
- `psql` 查询 `account_probe_runs.probe_source`
- Playwright MCP 打开 `/admin/model-probes`
- `docker logs sub2api --tail 220 | Select-String 'panic|fatal'`

## 校验结果

- 推送失败：`Permission to Wei-Shaw/sub2api.git denied to biaoxing-github`，HTTP 403。
- Docker build 通过，镜像 manifest list 为 `sha256:df66153f11ed5aae07f197571c740fd4e6aff46f1dbe8c7c9431b86fc18e3ace`；保留既有 Browserslist、Vite dynamic import、chunk size 和 Node DEP0190 警告。
- compose force-recreate `sub2api` 通过，Postgres 和 Redis 均 healthy。
- `sub2api` 容器状态为 `running healthy`，使用新镜像 `sha256:df66153f11ed5aae07f197571c740fd4e6aff46f1dbe8c7c9431b86fc18e3ace`。
- `/health` 返回 HTTP 200 与 `{"status":"ok"}`。
- `/admin/model-probes` 返回 HTTP 200，生产前端入口可访问。
- 未登录访问模型探针列表接口和 BazaarLink 后台任务创建接口均返回 HTTP 401，认证边界正常。
- 数据库迁移已生效：`account_probe_runs.probe_source` 存在，默认值为 `self_validation`。
- Playwright 实际打开 `/admin/model-probes` 后按预期跳转到 `/login?redirect=/admin/model-probes`，页面标题为 `Login - Sub2API`。
- 容器日志近 220 行未匹配 `panic/fatal`。

---

日期：2026-06-04
执行者：Devil

## BazaarLink 模型探针异步轮询与账号 423 验证

本轮继续按用户要求对接 BazaarLink API key 探测，不让管理端请求等待外部探测完成。前一版同步请求会让 BazaarLink/Cloudflare 边缘超时；本轮改为不发送 `sync=true`，创建 BazaarLink run 后轮询 `/api/probe/run/{runId}`，并兼容 BazaarLink 实际返回的 `identityAssessment.status=match` 身份确认状态。

## 校验方式

- `go test -tags unit ./internal/service -run TestAccountProbeService_RunBazaarLink -count=1`
- `go test -tags unit ./internal/service ./internal/handler/admin -run "Test.*Bazaar|Test.*ModelProbe" -count=1`
- `git diff --check`
- `docker build -t sub2api:multi-key-local --build-arg NODE_IMAGE=registry-1.docker.io/library/node:24-alpine --build-arg GOLANG_IMAGE=registry-1.docker.io/library/golang:1.26.3-alpine --build-arg ALPINE_IMAGE=registry-1.docker.io/library/alpine:3.21 --build-arg POSTGRES_IMAGE=registry-1.docker.io/library/postgres:18-alpine .`
- `docker compose -f D:\sub2api-deploy\docker-compose.yml up -d --force-recreate sub2api`
- `docker inspect sub2api` 健康检查
- 本地生成短期管理员 JWT 后调用 `POST /api/v1/admin/account-model-probe-runs/bazaarlink`，请求体为 `account_id=423, model=gpt-5.5, mode=quick`
- `psql` 查询 `account_probe_runs` 与 `account_probe_samples`

## 校验结果

- 后端 BazaarLink 聚焦测试通过。
- 后端 service/admin handler BazaarLink 与模型探针聚焦测试通过。
- `git diff --check` 通过。
- Docker build 通过，镜像 manifest list 为 `sha256:9ba59053e0878bf3ad2c3586c5c3545a7853b179daac883e2f3646305795f7d6`。
- compose force-recreate `sub2api` 通过，容器状态为 `running healthy`。
- 首次真实后台验证 `run_id=227` 返回 HTTP 200，但 `identityAssessment.status=match` 被旧逻辑误判为失败；该结果用于确认状态兼容缺口。
- 修复并重新部署后，再次触发账号 423 BazaarLink quick 验证返回 HTTP 202，创建 `run_id=229`。
- 数据库最终结果：`account_probe_runs.id=229` 为 `success`，`success_count=1`，`failure_count=0`，summary 为 `完成 1/1 次请求，平均延迟 94251 ms，消耗 12264 tokens`。
- 样本结果：`account_probe_samples.id=1552` 为 `success`，`sample_type=bazaarlink_api`，`http_status=200`，验证证据 passed=true，observed 为 `status=match; confidence=0.98; family=openai`。

---

日期：2026-06-04
执行者：Devil

## BazaarLink 模型探针分数展示修复

本轮修复 BazaarLink 探针已成功但模型探针详情页不显示结构化分数的问题。根因是 BazaarLink 原始响应较大，后端用于展示的 `response_body` 会截断，前端直接 `JSON.parse(response_body)` 失败后把 BazaarLink 结果卡整体隐藏；同时真实成功样本已落库的 `validation_evidence.score` 可能为 `0/100`，但 `observed` 中包含 `confidence=0.98`。

## 校验方式

- TDD 红灯：`go test -tags unit ./internal/service -run TestBazaarLinkProbeEvidenceUsesConfidenceWhenScoreMissing -count=1`
- TDD 红灯：`npm exec vitest -- src/views/admin/__tests__/AccountModelProbesView.spec.ts --run -t "renders BazaarLink score from validation evidence"`
- `go test -tags unit ./internal/service -run "Test.*Bazaar|TestBazaar" -count=1`
- `npm exec vitest -- src/views/admin/__tests__/AccountModelProbesView.spec.ts --run`
- `npm run typecheck`
- `git diff --check`
- `npm run build`

## 校验结果

- 后端红灯先失败于 BazaarLink 顶层 `score` 缺失时 evidence 分数为 `0`；修复后通过，成功身份验证会从 `confidence` 推导可展示分数。
- 前端红灯先失败于截断 `response_body` 时没有 BazaarLink 结构化结果卡；收紧为真实旧数据形态后，又先失败于 `score=0` 时显示 `0 / 100`。
- 前端修复后，即使 `response_body` 不是合法 JSON，也会使用 `bazaarlink_identity` evidence 展示结果卡，并从 `observed` 提取 `status=match`、`confidence=0.98`、`family=openai`、`v3f=gpt-5.5`，分数展示为 `98 / 100`。
- 后端评分聚合兼容旧成功样本：当 `bazaarlink_identity` evidence 已通过但 `score=0` 时，列表/详情 API 的 run score 会从 `observed` 的 confidence 推导，不再把成功探针算成 0 分。
- 后端 BazaarLink 聚焦测试通过。
- `AccountModelProbesView.spec.ts` 10 个测试全部通过。
- `npm run typecheck` 通过，`vue-tsc --noEmit` 退出码 0。
- `git diff --check` 通过。
- `npm run build` 通过；保留项目既有 Browserslist caniuse-lite 过期、Vite dynamic import 和 chunk size 警告。

---

日期：2026-06-04
执行者：Devil

## BazaarLink match/risk 与 V3 候选展示修复

本轮修复 BazaarLink 探针返回 `identityAssessment.status=match` 但页面仍按失败展示的问题，并补齐详情页的 BazaarLink 得分、V3 候选模型和风险提示展示。后端不再把 `riskFlags` 当作身份失败条件；只要身份状态为 `confirmed`、`match` 或 `matched`，样本就按通过落库，风险提示保留为 warning message。前端兼容旧的 `failed + bazaarlink_identity_mismatch` 记录：当截断响应或 evidence 显示身份已 match 时，详情页按成功展示，仍在 BazaarLink 结果卡中显示 `riskFlags`。

## 校验方式

- `go test -tags unit ./internal/service -run "TestAccountProbeService_RunBazaarLinkUsesAccountAPIKeyAndPersistsRedactedResult|TestBazaarLinkProbeEvidenceTreatsMatchWithRiskFlagsAsPassed|TestBazaarLinkProbeEvidenceUsesConfidenceWhenScoreMissing|TestScoreAccountProbeRunBazaarLinkUsesObservedConfidenceForLegacyEvidence" -count=1`
- `go test -tags unit ./internal/service -run "Test.*Bazaar|TestBazaar" -count=1`
- `npm test -- --run src/views/admin/__tests__/AccountModelProbesView.spec.ts`
- `npm run typecheck`
- `git diff --check`

## 校验结果

- 后端聚焦测试通过，`match + riskFlags` 样本落库为 success，evidence severity 为 warning，message 和 observed 保留风险提示。
- 后端 BazaarLink 相关测试通过。
- 前端 `AccountModelProbesView.spec.ts` 11 个测试全部通过；新增截断 BazaarLink 响应测试覆盖 V3 候选模型、风险标记、98 / 100 分数和旧失败状态修正。
- 前端类型检查通过。
- `git diff --check` 通过。
- 本轮按用户最新要求未执行构建、部署和线上探测。

---

日期：2026-06-04
执行者：Devil

## BazaarLink match/risk 修复推送构建部署与历史数据修正

本轮按用户要求提交、推送、构建、部署、验证，并修正历史误判数据。源码提交为 `ed3d7465 fix(admin): repair BazaarLink match probe results`；远端推送到 `origin feature/account-api-key-rotation` 时仍被 GitHub 返回 403，当前凭据用户 `biaoxing-github` 没有 `Wei-Shaw/sub2api` 写权限。随后继续基于本地已提交状态构建并部署到本机 `D:\sub2api-deploy\docker-compose.yml` 管理的 `sub2api` 容器。

## 校验方式

- `git push -u origin feature/account-api-key-rotation`
- `docker build --pull=false -t sub2api:multi-key-local --build-arg NODE_IMAGE=registry-1.docker.io/library/node:24-alpine --build-arg GOLANG_IMAGE=registry-1.docker.io/library/golang:1.26.3-alpine --build-arg ALPINE_IMAGE=registry-1.docker.io/library/alpine:3.21 --build-arg POSTGRES_IMAGE=postgres:18-alpine .`
- `docker compose -f D:\sub2api-deploy\docker-compose.yml up -d --force-recreate sub2api`
- `docker inspect sub2api`
- `SELECT filename FROM schema_migrations WHERE filename='154_repair_bazaarlink_match_probe_history.sql'`
- 历史误判样本和 run 查询：`account_probe_runs.id IN (227,242)`、`account_probe_samples.id IN (1551,1589)`
- `Invoke-WebRequest http://127.0.0.1:8080/health`
- `Invoke-WebRequest http://127.0.0.1:8080/admin/model-probes`
- `curl.exe -i "http://127.0.0.1:8080/api/v1/admin/account-probe-runs?page=1&page_size=1"`
- Playwright 打开 `http://127.0.0.1:8080/admin/model-probes`
- `docker logs sub2api --tail 220 | Select-String 'panic|fatal|migration.*error|checksum mismatch|apply migration'`

## 校验结果

- `git push` 失败：`Permission to Wei-Shaw/sub2api.git denied to biaoxing-github`，HTTP 403；远端未更新。
- 首次 Docker build 因 Docker Desktop 直连 `registry-1.docker.io` 拉取 `postgres:18-alpine` 元数据超时失败；使用本地已有 `postgres:18-alpine` 并加 `--pull=false` 后构建通过。
- Docker build 成功，新镜像 manifest list 为 `sha256:843ea986727259ba57b0bd257871083a8dfc984c154d391b49bcbf5c5225540c`。
- Docker compose force-recreate 成功，`sub2api` 容器状态为 `running healthy`，镜像为 `sha256:843ea986727259ba57b0bd257871083a8dfc984c154d391b49bcbf5c5225540c`。
- `schema_migrations` 已记录 `154_repair_bazaarlink_match_probe_history.sql`，应用时间为 `2026-06-04 14:13:31 +08`。
- 历史误判剩余数为 0。
- run 227 与 run 242 均已修正为 `status=success, success_count=1, failure_count=0`；summary 分别为 `完成 1/1 次请求，平均延迟 166036 ms，消耗 16094 tokens` 与 `完成 1/1 次请求，平均延迟 113163 ms，消耗 20188 tokens`。
- sample 1551 与 sample 1589 均已修正为 `status=success, http_status=200`，error_code/error_message 已清空；sample 1551 evidence 为 `passed=true, severity=info, score=98`，sample 1589 evidence 为 `passed=true, severity=warning, score=91`，风险提示 message 仍保留。
- `/health` 返回 HTTP 200 与 `{"status":"ok"}`。
- `/admin/model-probes` 返回 HTTP 200，HTML 长度 2627。
- 未登录访问 `GET /api/v1/admin/account-probe-runs?page=1&page_size=1` 返回 HTTP 401 与 `{"code":"UNAUTHORIZED","message":"Authorization required"}`。
- Playwright 打开 `/admin/model-probes` 后按预期跳转 `Login - Sub2API`，console warning/error 为 0。
- 容器日志近 220 行未匹配 panic、fatal、migration error、checksum mismatch 或 apply migration 错误。

---

日期：2026-06-04
执行者：Devil

## BazaarLink 探测得分改为只使用上游返回 score

本轮根据最新要求修正 BazaarLink 探测得分口径：探测得分只采用 BazaarLink 返回结果中的 `score`，`identityAssessment.confidence` 只作为身份置信度展示，不再换算成样本 evidence 分数、run 聚合分数或前端结果卡分数。新增迁移 `155_align_bazaarlink_evidence_score.sql`，把旧版从 confidence 派生出的历史 evidence 分数归零；事务回滚验证显示仅命中样本 1551，样本 1589 的 BazaarLink 返回分数 91 保持不变，riskFlags 仍保留展示。

## 校验方式

- TDD 红灯：`go test -tags unit ./internal/service -run "TestBazaarLinkProbeEvidenceKeepsReturnedZeroScoreWhenConfidenceExists|TestBazaarLinkProbeEvidenceTreatsMatchWithRiskFlagsAsPassed|TestScoreAccountProbeRunBazaarLinkKeepsReturnedZeroScoreForLegacyEvidence" -count=1`
- TDD 红灯：`npm test -- --run src/views/admin/__tests__/AccountModelProbesView.spec.ts`
- `gofmt -w internal\service\bazaarlink_probe.go internal\service\account_probe_score.go internal\service\account_probe_test.go`
- `go test -tags unit ./internal/service -run "TestBazaarLinkProbeEvidenceKeepsReturnedZeroScoreWhenConfidenceExists|TestBazaarLinkProbeEvidenceTreatsMatchWithRiskFlagsAsPassed|TestScoreAccountProbeRunBazaarLinkKeepsReturnedZeroScoreForLegacyEvidence" -count=1`
- `go test -tags unit ./internal/service -run "Test.*Bazaar|TestBazaar" -count=1`
- `go test -tags unit ./internal/service ./internal/handler/admin -run "Test.*Bazaar|Test.*ModelProbe" -count=1`
- `npm test -- --run src/views/admin/__tests__/AccountModelProbesView.spec.ts`
- `npm run typecheck`
- `git diff --check`
- `npm run build`
- 迁移事务回滚验证：`BEGIN; 155_align_bazaarlink_evidence_score.sql; SELECT sample 1551/1589 evidence score; ROLLBACK;`

## 校验结果

- 后端红灯先失败于旧逻辑把 confidence=0.98 换算为 98；修复后聚焦用例通过。
- 前端红灯先失败于详情页继续显示 `98 / 100`；修复后 `score=0` 的 BazaarLink 响应显示 `0 / 100`，confidence 仍显示 `98%`，V3 候选和 riskFlags 仍展示。
- 迁移 SQL 事务回滚验证通过：`UPDATE 1`，事务内 sample 1551 score 从 98 改为 0，sample 1589 score 保持 91，随后 ROLLBACK。
- 后端 BazaarLink/模型探针相关测试通过。
- 前端 `AccountModelProbesView.spec.ts` 11 个测试通过。
- `npm run typecheck` 通过。
- `git diff --check` 通过。
- `npm run build` 通过；保留既有 Browserslist、Vite dynamic import 和 chunk size 警告。

---

日期：2026-06-04
执行者：Devil

## BazaarLink 快速验证历史列表 0 分修复

本轮针对账号 423 的快速验证列表显示 `0 / 不可检测` 做根因排查。数据库确认 run `229`、`227` 均为 `request_mode=quick`、`probe_source=bazaarlink_api`，样本 evidence score 为 `0` 且没有 `display_score`，但 `response_body` 仍保留 `"v3"` 候选片段。因此列表接口虽然已经加载 samples，但旧 quick 样本没有新 evidence 字段时仍退回 0 分。

修复后，后端在 quick 模式下会从 sample 的 BazaarLink `response_body` 解析 V3 candidates，并按声明模型匹配候选分：例如 `gpt-5.5` 匹配 `GPT-5.5/openai/gpt-5.5` 的 `0.9852066599830172`，列表返回 `display_score=98.52066599830172`，聚合整数 score 为 `99`，不再显示 `不可检测`。

## 校验方式

- 数据库只读排查：查询账号 423 的 BazaarLink quick run，确认 run `229/227` evidence score 为 0、display_score 为空、response_body 内存在 `"v3"`。
- TDD 红灯：`go test -tags unit ./internal/service -run "TestAccountProbeServiceListReportsUsesLegacyBazaarLinkQuickCandidateScore" -count=1`
- `gofmt -w internal\service\account_probe_score.go internal\service\bazaarlink_probe.go internal\service\account_probe_report_test.go`
- `go test -tags unit ./internal/service -run "TestAccountProbeServiceListReportsUsesLegacyBazaarLinkQuickCandidateScore" -count=1`
- `go test -tags unit ./internal/service ./internal/handler/admin -run "Test.*Bazaar|Test.*ModelProbe|TestAccountProbeServiceListReports" -count=1`
- `npm test -- --run src/views/admin/__tests__/AccountModelProbesView.spec.ts`
- `npm run typecheck`
- `git diff --check`

## 校验结果

- 红灯先失败于 `expected: 99 actual: 0`。
- 修复后新增历史 quick 兼容测试通过。
- 后端 BazaarLink/模型探针/列表报表聚焦测试通过。
- 前端 `AccountModelProbesView.spec.ts` 12 个测试通过。
- `npm run typecheck` 通过。
- `git diff --check` 通过。
- 本轮未构建部署，截图中的容器需要用本轮源码重新构建/重启后才会显示新分数。

---

日期：2026-06-04
执行者：Devil

## BazaarLink score 口径修复部署与历史数据落库验证

本轮源码提交为 `bb3e68d9 fix(admin): align BazaarLink probe score source`。远端推送到 `origin feature/account-api-key-rotation` 仍被 GitHub 返回 403，当前凭据用户 `biaoxing-github` 没有 `Wei-Shaw/sub2api` 写权限；随后继续基于本地已提交源码构建 `sub2api:multi-key-local` 并部署到本机 `D:\sub2api-deploy\docker-compose.yml` 管理的 `sub2api` 容器。

## 校验方式

- `git push -u origin feature/account-api-key-rotation`
- `docker build --pull=false -t sub2api:multi-key-local --build-arg NODE_IMAGE=registry-1.docker.io/library/node:24-alpine --build-arg GOLANG_IMAGE=registry-1.docker.io/library/golang:1.26.3-alpine --build-arg ALPINE_IMAGE=registry-1.docker.io/library/alpine:3.21 --build-arg POSTGRES_IMAGE=postgres:18-alpine .`
- `docker compose -f D:\sub2api-deploy\docker-compose.yml up -d --force-recreate sub2api`
- `docker inspect sub2api`
- `SELECT filename, applied_at FROM schema_migrations WHERE filename IN ('154_repair_bazaarlink_match_probe_history.sql','155_align_bazaarlink_evidence_score.sql')`
- 历史样本查询：`account_probe_samples.id IN (1551,1589)` 的 `bazaarlink_identity` evidence
- confidence 派生残留查询：`remaining_confidence_derived_scores`
- `Invoke-WebRequest http://127.0.0.1:8080/health`
- `Invoke-WebRequest http://127.0.0.1:8080/admin/model-probes`
- `curl.exe -i "http://127.0.0.1:8080/api/v1/admin/account-probe-runs?page=1&page_size=1"`
- Playwright 打开 `http://127.0.0.1:8080/admin/model-probes`
- `docker logs sub2api --tail 260 2>&1 | Select-String "panic|fatal|migration.*error|checksum mismatch|apply migration|155_align"`

## 校验结果

- `git push` 失败：`Permission to Wei-Shaw/sub2api.git denied to biaoxing-github`，HTTP 403；远端未更新。
- Docker build 成功，新镜像 manifest list 为 `sha256:a1bd8b83b0c6c27b85b0414975358d1da122037f5e0a4c7d4daf2c37ec9f2b4e`。
- Docker compose force-recreate 成功，`sub2api` 容器状态为 `running healthy`，镜像为 `sha256:a1bd8b83b0c6c27b85b0414975358d1da122037f5e0a4c7d4daf2c37ec9f2b4e`。
- `schema_migrations` 已记录 `155_align_bazaarlink_evidence_score.sql`，应用时间为 `2026-06-04 14:35:26.126566 +08`。
- sample 1551 已修正为 `status=success, run_status=success, evidence_score=0, passed=true, severity=info`。
- sample 1589 保持 `status=success, run_status=success, evidence_score=91, passed=true, severity=warning`，riskFlags message 仍保留。
- confidence 派生分数残留数为 0。
- `/health` 返回 HTTP 200 与 `{"status":"ok"}`。
- `/admin/model-probes` 返回 HTTP 200，HTML 长度 2627。
- 未登录访问 `GET /api/v1/admin/account-probe-runs?page=1&page_size=1` 返回 HTTP 401 与 `{"code":"UNAUTHORIZED","message":"Authorization required"}`。
- Playwright 打开 `/admin/model-probes` 后按预期跳转 `Login - Sub2API`，console warning/error 为 0。
- 容器日志近 260 行未匹配 panic、fatal、migration error、checksum mismatch、apply migration 或 155_align 错误。

---

日期：2026-06-04
执行者：Devil

## BazaarLink 完整/快速得分展示口径修复

本轮根据最新口径修正模型探针页面分数展示：BazaarLink 完整验证的详情卡和列表页使用 BazaarLink 返回/落库 evidence 总分，例如 `91`；快速验证使用声明模型在 V3 候选列表中的候选分，例如检测 `gpt-5.5` 时展示 `GPT-5.5` 候选 `98.5`。后端列表页现在会为 BazaarLink 报表行加载 samples 再聚合分数，避免单样本成功退回成功率 `100`；前端不再从截断 `response_body` 任意位置读取第一个 `score`，避免把候选分或其他嵌套分误当总分。

## 校验方式

- TDD 红灯：`go test -tags unit ./internal/service -run "TestAccountProbeServiceListReportsUsesBazaarLinkEvidenceScore|TestAccountProbeService_RunBazaarLinkUsesAccountAPIKeyAndPersistsRedactedResult|TestAccountProbeService_RunBazaarLinkPollsAsyncRunUntilCompleted" -count=1`
- TDD 红灯：`npm test -- --run src/views/admin/__tests__/AccountModelProbesView.spec.ts`
- `gofmt -w internal\service\account_probe.go internal\service\account_probe_report.go internal\service\account_probe_score.go internal\service\bazaarlink_probe.go internal\service\account_probe_report_test.go internal\service\account_probe_test.go`
- `go test -tags unit ./internal/service -run "TestAccountProbeServiceListReportsUsesBazaarLinkEvidenceScore|TestAccountProbeService_RunBazaarLinkUsesAccountAPIKeyAndPersistsRedactedResult|TestAccountProbeService_RunBazaarLinkPollsAsyncRunUntilCompleted" -count=1`
- `npm test -- --run src/views/admin/__tests__/AccountModelProbesView.spec.ts`
- `go test -tags unit ./internal/service ./internal/handler/admin -run "Test.*Bazaar|Test.*ModelProbe|TestAccountProbeServiceListReports" -count=1`
- `npm run typecheck`
- `git diff --check`
- `npm run build`

## 校验结果

- 后端红灯先失败于列表未加载 samples、quick 仍使用顶层 `score=87/93`。
- 前端红灯先失败于完整验证详情显示候选嵌套分 `1.0 / 100`，quick 列表显示整数 `99` 而不是候选展示分 `98.5`。
- 修复后后端 BazaarLink/模型探针/报表聚焦测试通过。
- 前端 `AccountModelProbesView.spec.ts` 11 个测试通过。
- `npm run typecheck` 通过。
- `git diff --check` 通过。
- `npm run build` 通过；保留既有 Browserslist、Vite dynamic import 和 chunk size 警告。

---

日期：2026-06-04
执行者：Devil

## 本地校验列表分数与 BazaarLink 展示样式隔离

本轮修复模型探针列表中 `self_validation` 本地校验错误很多但仍显示 `100` 的问题。根因是列表页只为 BazaarLink 模型验证加载 samples，本地校验列表项缺少 `validation_evidence` 后退回 `success_count/request_count` 兜底；账号 `408` 的 run `257` 实际为 `request_count=19`、`success_count=31`、`failure_count=7`，兜底会被 clamp 成 `100`。同时前端 `formatRunScore` 对所有来源都优先使用 `display_score`，导致本地校验也可能带上 BazaarLink 候选展示分样式。

## 校验方式

- `go test -tags unit ./internal/service -run TestAccountProbeServiceListReportsUsesSelfValidationEvidenceScore -count=1` 红灯确认列表未加载本地校验 samples。
- `npm exec vitest -- src/views/admin/__tests__/AccountModelProbesView.spec.ts --run -t "does not use BazaarLink display score"` 红灯确认本地校验行误用 `display_score=100`。
- `go test -tags unit ./internal/service ./internal/handler/admin -run "Test.*Bazaar|Test.*ModelProbe|TestAccountProbeServiceListReports" -count=1`
- `npm exec vitest -- src/views/admin/__tests__/AccountModelProbesView.spec.ts --run`
- `npm run typecheck`
- `npm run build`
- `git diff --check`
- `docker build --pull=false -t sub2api:multi-key-local .`
- `docker compose -f D:\sub2api-deploy\docker-compose.yml up -d --force-recreate sub2api`
- `Invoke-WebRequest http://127.0.0.1:8080/health`
- `Invoke-WebRequest http://127.0.0.1:8080/admin/model-probes`
- SQL 聚合 run `257` 的 `account_probe_samples.validation_evidence` 分数。

## 校验结果

- 两个红灯用例修复后均转绿。
- 后端相关测试通过：`internal/service` 与 `internal/handler/admin` 聚焦测试均通过。
- 前端 `AccountModelProbesView.spec.ts` 12 个测试通过。
- `npm run typecheck` 通过。
- `npm run build` 通过；保留既有 Browserslist、Vite dynamic import、chunk size 和 Node DEP0190 警告。
- `git diff --check` 无空白错误；仅提示 docs JSONL 工作区 CRLF 转换警告。
- 首次 Docker build 被本机 Docker mirror `1d75j62o.mirror.aliyuncs.com` 对 `node/golang/alpine` 的 HEAD 请求 403 拦截；补齐本机缓存短名 tag 后用 `--pull=false` 构建成功。
- 新镜像 manifest list 为 `sha256:4218fa3019f1912cb2134efe179fa792e91d4d6893521ba33d3c91f4a78bdb0e`，容器 `sub2api` 已 force-recreate，状态 `running healthy`。
- `/health` 返回 HTTP 200 与 `{"status":"ok"}`，`/admin/model-probes` 返回 HTTP 200。
- 未登录访问模型探针管理 API 返回 HTTP 401，认证拦截正常；由于 `D:\sub2api-deploy\.env` 未配置 `ADMIN_PASSWORD`，未做登录态 API 读取。
- run `257` 的真实 evidence 聚合为 `38` 条证据、`30` 条通过、原始分 `345/400`、折算 `86.3`，后端列表会显示约 `86`，不再是 `100`。
- 容器日志近 5 分钟未匹配 `panic|fatal|migration|checksum`；仅见启动时远端价格 hash 拉取超时，和本次模型探针逻辑无关。

---

日期：2026-06-04
执行者：Devil

## 本地校验列表分数修复提交后构建部署验证

本轮已将本地校验列表分数与 BazaarLink 展示样式隔离修复提交到当前分支，并基于提交后的工作区重新构建 `sub2api:multi-key-local`，部署到本机 `D:\sub2api-deploy\docker-compose.yml` 管理的 `sub2api` 容器。`.dockerignore` 忽略 `*.md` 与 `docs/`，补充验证记录不会进入镜像构建上下文。

## 校验方式

- `git diff --cached --check`
- `git commit -m "fix(admin): align model probe scoring"`
- `docker build --pull=false -t sub2api:multi-key-local .`
- `docker compose -f D:\sub2api-deploy\docker-compose.yml up -d --force-recreate sub2api`
- `docker inspect sub2api --format '{{.State.Status}} {{.State.Health.Status}} {{.Image}}'`
- `Invoke-WebRequest http://127.0.0.1:8080/health`
- `Invoke-WebRequest http://127.0.0.1:8080/admin/model-probes`
- 未登录访问 `GET /api/v1/admin/account-probe-runs?mode=model_validation&keyword=aisz&sort_by=created_at&sort_order=desc&page=1&page_size=20`
- SQL 聚合 run `257` 的 `account_probe_samples.validation_evidence` 分数。
- `docker logs --since 5m sub2api` 过滤 `panic|fatal|migration|checksum`
- `go test -tags unit ./internal/service ./internal/handler/admin -run "Test.*Bazaar|Test.*ModelProbe|TestAccountProbeServiceListReports" -count=1`
- `npm exec vitest -- src/views/admin/__tests__/AccountModelProbesView.spec.ts --run`
- `npm run typecheck`

## 校验结果

- 提交前 `git diff --cached --check` 通过。
- Docker build 成功，新镜像 manifest list 为 `sha256:36c952dcf5bda2a154c98333b39eb3174193436ddad27a0ad20684b92cf7ebb6`。
- Docker compose force-recreate 成功，`sub2api` 容器状态为 `running healthy`，镜像为 `sha256:36c952dcf5bda2a154c98333b39eb3174193436ddad27a0ad20684b92cf7ebb6`。
- `/health` 返回 HTTP 200 与 `{"status":"ok"}`。
- `/admin/model-probes` 返回 HTTP 200，HTML 长度 2627。
- 未登录访问模型探针管理 API 返回 HTTP 401，认证拦截正常。
- run `257` 的真实 evidence 聚合为 `38` 条证据、`30` 条通过、原始分 `345/400`、折算 `86.3`，列表分数不再按兜底显示 `100`。
- 容器日志近 5 分钟未匹配 `panic|fatal|migration|checksum`。
- 后端聚焦测试通过：`internal/service` 与 `internal/handler/admin` 均通过。
- 前端 `AccountModelProbesView.spec.ts` 12 个测试通过。
- `npm run typecheck` 通过。
- 浏览器 MCP 当前被另一个 `mcp-chrome` 实例占用，无法执行页面截图/console smoke；本轮以前端构建、组件测试、类型检查和已部署页面 HTTP 200 作为替代验证证据。

---

日期：2026-06-04
执行者：Devil

## BazaarLink 快速候选分历史数据修正发布验证

本轮修复 BazaarLink quick 历史记录在列表页显示 `0` 的问题：旧样本 `validation_evidence.score=0` 且没有 `display_score` 时，后端会从 BazaarLink `response_body` 的 V3 candidates 中按声明模型匹配候选分；同时新增迁移 `156_align_bazaarlink_quick_candidate_scores.sql` 回填既有历史样本。代码修复已提交为 `c2b0f81c fix(admin): backfill BazaarLink quick candidate scores`，并基于该提交完成本地 Docker 构建与 compose 部署。

## 校验方式

- `git push -u origin feature/account-api-key-rotation`
- `docker build --pull=false -t sub2api:multi-key-local --build-arg NODE_IMAGE=registry-1.docker.io/library/node:24-alpine --build-arg GOLANG_IMAGE=registry-1.docker.io/library/golang:1.26.3-alpine --build-arg ALPINE_IMAGE=registry-1.docker.io/library/alpine:3.21 --build-arg POSTGRES_IMAGE=postgres:18-alpine .`
- `docker compose -f D:\sub2api-deploy\docker-compose.yml up -d --force-recreate sub2api`
- `docker inspect sub2api --format '{{.State.Status}} {{if .State.Health}}{{.State.Health.Status}}{{end}} {{.Image}}'`
- `curl.exe -s -i http://127.0.0.1:8080/health`
- `curl.exe -s -o NUL -w "%{http_code} %{size_download}\n" http://127.0.0.1:8080/admin/model-probes`
- 未登录访问 `GET /api/v1/admin/account-probe-runs?mode=model_validation&keyword=qingflow&sort_by=created_at&sort_order=desc&page=1&page_size=20`
- SQL 查询 `schema_migrations` 中 `156_align_bazaarlink_quick_candidate_scores.sql`
- SQL 查询 `account_probe_runs/account_probe_samples` 中 run `227`、`229` 的 `validation_evidence.score/display_score`
- `docker logs sub2api --tail 260` 过滤 `panic|fatal|migration.*error|checksum mismatch|apply migration|156_align`

## 校验结果

- 代码提交 `c2b0f81c` 已生成；远端推送被网络连接阻断，错误为 `fatal: unable to access 'https://github.com/Wei-Shaw/sub2api.git/': Recv failure: Connection was reset`，此前重试还出现过 `Could not connect to server`，当前远端未确认更新。
- Docker build 成功，新镜像 manifest list 为 `sha256:86a5049ebdc5923d72655f8287969b2dc5aa2c10fe69506a584bbf527e5c19b4`。
- Docker compose force-recreate 成功，`sub2api` 容器状态为 `running healthy`，镜像为 `sha256:86a5049ebdc5923d72655f8287969b2dc5aa2c10fe69506a584bbf527e5c19b4`。
- `/health` 返回 HTTP 200 与 `{"status":"ok"}`。
- `/admin/model-probes` 返回 HTTP 200，HTML 长度 2671。
- 未登录访问模型探针管理 API 返回 HTTP 401，认证拦截正常。
- 迁移 `156_align_bazaarlink_quick_candidate_scores.sql` 已落库，`applied_at=2026-06-04 16:10:36.378201+08`。
- 历史数据已修正：run `227` / sample `1551` 为 `score=98`、`display_score=98.0389688269813500`；run `229` / sample `1552` 为 `score=99`、`display_score=99.3506366160804800`。
- 容器日志关键错误过滤为空，未见 panic、fatal、迁移错误或 checksum mismatch。

---

日期：2026-06-04
执行者：Devil

## API Key 手动分组倍率优先级修复验证

本轮修复 OpenAI API Key 账号余额刷新时接口分组倍率覆盖手动倍率的问题。新增 `upstream_manual_rate_multiplier` / `upstream_manual_rate_group_name` 作为手动分组倍率字段；账号存在手动倍率时，余额分组和 `converted_available` 换算优先使用手动倍率；未设置手动倍率时才按上游接口返回分组倍率计算。上游接口分组仍保存到 `upstream_fetched_groups` 与 `upstream_common_rate_*` 缓存，但不再覆盖手动设置。

## 校验方式

- `go test -tags unit ./internal/service -run "Test.*UpstreamBalance|TestGroupsForKey|TestManualRateGroups" -count=1`
- `npm exec vitest -- src/components/account/__tests__/EditAccountModal.spec.ts --run`
- `npm run typecheck`
- `git diff --check`

## 校验结果

- 后端余额聚焦测试通过，覆盖手动倍率优先、接口分组缓存隔离、旧 `credentials.upstream_common_rate_multiplier` 兼容。
- 前端 `EditAccountModal.spec.ts` 14 个测试通过，覆盖编辑弹窗不再把 `extra.upstream_common_rate_multiplier` 接口缓存当成手动值提交。
- `npm run typecheck` 通过，`vue-tsc --noEmit` 退出码 0。
- `git diff --check` 通过。
- 本轮未执行 Docker 构建部署。

---

日期：2026-06-04
执行者：Devil

## API Key 手动分组倍率提交构建部署验证

本轮在 `fix(accounts): preserve manual upstream group rate` 提交 `0d4aa5c6` 基础上完成本地构建、compose 重建和部署后冒烟。构建使用 `--pull=false` 与官方 registry 显式镜像，镜像 digest 为 `sha256:6b6ae31c930f06fdc9f4281a16813d646cbf1dcc8c9c1c70ce1518d3b0b39f03`。

## 校验方式

- `git commit -m "fix(accounts): preserve manual upstream group rate"`
- `docker build --pull=false -t sub2api:multi-key-local --build-arg NODE_IMAGE=registry-1.docker.io/library/node:24-alpine --build-arg GOLANG_IMAGE=registry-1.docker.io/library/golang:1.26.3-alpine --build-arg ALPINE_IMAGE=registry-1.docker.io/library/alpine:3.21 --build-arg POSTGRES_IMAGE=registry-1.docker.io/library/postgres:18-alpine --build-arg COMMIT=0d4aa5c6a5d5 .`
- `docker compose -f D:\sub2api-deploy\docker-compose.yml up -d --force-recreate sub2api`
- `docker inspect sub2api --format '{{.State.Status}} {{if .State.Health}}{{.State.Health.Status}}{{end}} {{.Image}}'`
- `curl.exe -s -i http://127.0.0.1:8080/health`
- `curl.exe -s -o NUL -w "%{http_code} %{size_download}\n" http://127.0.0.1:8080/admin/accounts`
- `curl.exe -s -i "http://127.0.0.1:8080/api/v1/admin/accounts?page=1&page_size=1"`
- `docker logs sub2api --tail 260 | Select-String -Pattern "panic|fatal|migration.*error|checksum mismatch"`

## 校验结果

- 代码已提交为 `0d4aa5c6 fix(accounts): preserve manual upstream group rate`。
- Docker build 成功，镜像 manifest list 为 `sha256:6b6ae31c930f06fdc9f4281a16813d646cbf1dcc8c9c1c70ce1518d3b0b39f03`。
- `docker compose` force-recreate 成功，`sub2api` 容器状态为 `running healthy`，镜像与刚构建的 digest 一致。
- `/health` 返回 HTTP 200 与 `{"status":"ok"}`。
- `/admin/accounts` 返回 HTTP 200，HTML 长度 2671。
- 未登录访问 `GET /api/v1/admin/accounts?page=1&page_size=1` 返回 HTTP 401，认证拦截正常。
- 容器日志近 260 行未匹配 `panic`、`fatal`、迁移错误或 checksum mismatch。

---

日期：2026-06-06
执行者：Devil

## OpenAI 响应文本异常关键词

本轮为指定 OpenAI 账号增加响应正文关键词异常规则。账号开启 `openai_response_text_error_enabled` 并配置 `openai_response_text_error_keywords` 后，流式/非流式响应中只要命中关键词（例如 `加入新家园`），就按上游异常进入 failover；没有可切换账号时返回明确的 upstream error。

## 校验方式

- `go test -tags unit ./internal/service -run "TestOpenAI(StreamingConfiguredResponseTextReturnsFailoverBeforeOutput|NonStreamingConfiguredResponseTextReturnsFailover)$" -count=1`
- `go test -tags unit ./internal/service -run "TestOpenAI.*ResponseText|TestHandleSSEToJSON|TestHandleNonStreamingResponse_APIKeyFallsBackToSSEBodyWhenContentTypeIsWrong" -count=1`
- `go test -tags unit ./internal/handler -run "FailoverExhausted|StreamWrittenGuard" -count=1`
- `npm run typecheck`
- `git diff --check`

## 校验结果

- 两个新增 TDD 用例先红后绿，覆盖流式首个输出前命中关键词、非流式 JSON 命中关键词。
- service 聚焦测试通过，覆盖 SSE-to-JSON 既有行为未回退。
- handler 聚焦测试通过，确认 failover exhausted 相关路径可编译并保持现有流保护测试通过。
- 前端 `vue-tsc --noEmit` 通过。
- `git diff --check` 通过。

---

日期：2026-06-06
执行者：Devil

## OpenAI 响应文本异常关键词提交构建部署验证

本轮将 OpenAI 指定账号响应文本异常关键词功能提交为 `d035c39e feat(openai): flag configured response text`，随后从已提交状态生成干净构建上下文完成 Docker 构建、compose 重建和本地冒烟验证。首次 Docker build 因 Docker Hub direct registry 拉取显式 `postgres:18-alpine` 超时失败；复用本地缓存默认镜像 tag 并指定 `--pull=false` 后构建成功。

## 校验方式

- `git commit -m "feat(openai): flag configured response text"`
- `git archive --format=tar -o <temp>\sub2api-d035c39ecfd0.tar HEAD`
- `docker build --pull=false -t sub2api:multi-key-local --build-arg COMMIT=d035c39ecfd0 <temp>\sub2api-d035c39ecfd0`
- `docker compose -f D:\sub2api-deploy\docker-compose.yml up -d --force-recreate sub2api`
- `docker inspect sub2api --format '{{.State.Status}} {{if .State.Health}}{{.State.Health.Status}}{{end}} {{.Image}}'`
- `curl.exe -s -o NUL -w "%{http_code} %{size_download}\n" http://127.0.0.1:8080/health`
- `curl.exe -s -o NUL -w "%{http_code} %{size_download}\n" http://127.0.0.1:8080/admin/accounts`
- `curl.exe -s -o NUL -w "%{http_code} %{size_download}\n" "http://127.0.0.1:8080/api/v1/admin/accounts?page=1&page_size=1"`
- `docker logs sub2api --tail 260 | Select-String -Pattern "panic|fatal|migration.*error|checksum mismatch"`

## 校验结果

- Docker build 成功，镜像 `sub2api:multi-key-local` digest 为 `sha256:9b24d19111347710a1a03310adb719aaf4aba291a991ef953d23c957b7645c53`。
- `docker compose` force-recreate 成功，Postgres 和 Redis healthy，`sub2api` 启动成功。
- `docker inspect` 显示 `sub2api` 为 `running healthy`，运行镜像为 `sha256:9b24d19111347710a1a03310adb719aaf4aba291a991ef953d23c957b7645c53`。
- `/health` 返回 HTTP 200，响应大小 15。
- `/admin/accounts` 返回 HTTP 200，响应大小 2671。
- 未登录访问 `GET /api/v1/admin/accounts?page=1&page_size=1` 返回 HTTP 401，认证拦截正常。
- 容器日志近 260 行未匹配 `panic`、`fatal`、迁移错误或 checksum mismatch。

---

日期：2026-06-06
执行者：Devil

## OpenAI HTTP stream 断流调度避让

本轮修复 HTTP `/responses` 流式响应已经向下游写出内容后，上游 read error 没有进入 OpenAI path-health 的问题。由于同一条 HTTP SSE 响应已经开始输出后不能透明拼接到另一个上游，修复目标是把断流记入账号级和实际 baseURL 级健康状态，让后续客户端重试由现有调度器和 circuit breaker 自动避开坏账号。

## 校验方式

- `go test -tags unit ./internal/service -run "TestOpenAIStreamingReadErrorAfterOutputRecordsPathHealthFailure" -count=1`
- `go test -tags unit ./internal/service -run "TestOpenAIStreamingReadErrorAfterOutputRecordsPathHealthFailure|TestOpenAIStreamingHTTP2ReadErrorAfterOutputRecordsProtocolFailure|TestOpenAIStreamingReadErrorBeforeOutputReturnsFailover|TestBuildOpenAIAccountLoadPlanHalfOpenOnlyForProbe" -count=1`
- `go test -tags unit ./internal/service -run "TestOpenAI.*Stream|TestBuildOpenAIAccountLoadPlan|TestOpenAIPathHealth|TestClassifyUpstreamError" -count=1`
- `go test -tags unit ./internal/service -count=1`

## 校验结果

- 新增 TDD 用例先红后绿：红灯失败于账号级 path-health `FailureCount=0`；修复后断流会记录到 `OpenAIPathHealthKeyForAccount(..., https_sse)`。
- HTTP/2 `stream ID ... INTERNAL_ERROR` 用例通过，断流原因归一为 `http2_protocol_error`，并同时写入实际 `requestBaseURL` 的 path-health key。
- 已输出前断流仍走原有 failover 行为，半开账号仍只进入 probe profile，调度器过滤逻辑未被改变。
- 聚焦和更宽切片测试均通过。
- 整包 `internal/service` 单包测试运行 244 秒超时，未得到失败断言；本轮不把整包测试计为通过。

---

日期：2026-06-06
执行者：Devil

## OpenAI HTTP stream 断流调度避让提交构建部署验证

本轮将 OpenAI HTTP stream 断流调度避让修复提交为 `be69c3be fix(openai): record stream read failures for scheduling`，随后从已提交状态生成干净构建上下文完成 Docker 构建、compose 重建和本地 smoke。构建部署全程排除 `.codegraph/` 与 `.playwright-mcp/` 工具目录。

## 校验方式

- `git commit -m "fix(openai): record stream read failures for scheduling"`
- `git archive --format=tar -o <temp>\sub2api-be69c3be07d8.tar HEAD`
- `docker build --pull=false -t sub2api:multi-key-local --build-arg COMMIT=be69c3be07d8 <temp>\sub2api-be69c3be07d8`
- `docker compose -f D:\sub2api-deploy\docker-compose.yml up -d --force-recreate sub2api`
- `docker inspect sub2api --format '{{.State.Status}} {{if .State.Health}}{{.State.Health.Status}}{{end}} {{.Image}}'`
- `curl.exe -s -o NUL -w "%{http_code} %{size_download}\n" http://127.0.0.1:8080/health`
- `curl.exe -s -o NUL -w "%{http_code} %{size_download}\n" http://127.0.0.1:8080/admin/accounts`
- `curl.exe -s -o NUL -w "%{http_code} %{size_download}\n" "http://127.0.0.1:8080/api/v1/admin/accounts?page=1&page_size=1"`
- `docker logs sub2api --since 2026-06-06T11:27:50 --tail 400 2>&1 | Select-String -Pattern "panic|fatal|migration.*error|checksum mismatch"`

## 校验结果

- 提交前聚焦 Go 测试通过，`git diff --check` 通过，`docs/feature_list.jsonl` 与 `docs/process_list.jsonl` 可解析为 JSONL。
- Docker build 成功，镜像 `sub2api:multi-key-local` manifest list digest 为 `sha256:512bfb2869d12c7111ab540ad85ea11c5f51edd661e89003c0e9389ca260ec84`。
- `docker compose` force-recreate 成功，Postgres 和 Redis healthy，`sub2api` 启动成功。
- `docker inspect` 显示 `sub2api` 为 `running healthy`，运行镜像为 `sha256:512bfb2869d12c7111ab540ad85ea11c5f51edd661e89003c0e9389ca260ec84`。
- `/health` 返回 HTTP 200，响应大小 15。
- `/admin/accounts` 返回 HTTP 200，响应大小 2671。
- 未登录访问 `GET /api/v1/admin/accounts?page=1&page_size=1` 返回 HTTP 401，认证拦截正常。
- 容器日志关键启动错误过滤未匹配 `panic`、`fatal`、迁移错误或 checksum mismatch。
- 部署后真实 `/responses` 流量日志捕获到 HTTP/2 `stream ID ... INTERNAL_ERROR` 断流，新版本已在线承接该类 read error；同一 HTTP 响应仍不会中途拼接切换，后续客户端重试由 path-health 与 scheduler 自动避让。
- 本轮额外验证了 free5 的 Codex 访问形态：真实 Codex CLI 直连 free5 成功，且把捕获到的真实 Codex body 用 Node fetch / Node HTTP2 重放后同样成功；非流式极简请求仍会得到 `403 codex_access_restricted`。这说明 upstream 要的是 Codex CLI 风格流式 `/v1/responses` 请求，不是简单桌面 UA 伪装。

---

日期：2026-06-06
执行者：Devil

## OpenAI HTTP request-phase context canceled 自动切换

本轮修复 HTTP `/responses` 请求阶段上游返回 `context canceled` 时没有触发账号切换的问题。此前这类错误会被写成普通 502 `Upstream request failed`，handler 拿不到 `UpstreamFailoverError`，所以即使调度池还有可用账号也不会继续切换。本轮将入站客户端请求 context 纳入请求阶段错误判断：客户端仍连接时，`context.Canceled` 视为请求阶段上游瞬时错误并返回 failover；客户端已经取消/断开时，保持原非 failover 行为，避免把真实客户端断开误判为上游故障。

## 校验方式

- `go test -tags unit ./internal/service -run "TestOpenAIGatewayService_ForwardRequestPhaseContextCanceled" -count=1`
- `go test -tags unit ./internal/service -run "TestOpenAIGatewayService_Forward(RequestHeaderTimeoutReturnsFailover|RequestPhaseContextCanceled.*|APIKeyRequestBaseURLFailoverBeforeAccountFailover)" -count=1`
- `go test -tags unit ./internal/service -run "TestOpenAIStreaming(ReadErrorAfterOutputRecordsPathHealthFailure|MissingTerminalEventRecordsPathHealthFailure)|TestOpenAIGatewayService_Forward(RequestHeaderTimeoutReturnsFailover|RequestPhaseContextCanceled.*|APIKeyRequestBaseURLFailoverBeforeAccountFailover)" -count=1`
- `go build ./cmd/server`

## 校验结果

- TDD 红测先失败于错误链只有 `upstream request failed: context canceled`，没有 `*UpstreamFailoverError`。
- 修复后客户端仍连接的 request-phase `context.Canceled` 返回 `UpstreamFailoverError`，调度层可以继续切账号；客户端请求 context 已取消时仍写 502 且不返回 failover。
- 原有 request header timeout failover、API Key request baseURL failover、stream read error path-health 和 missing terminal event path-health 回归测试通过。
- 后端 `go build ./cmd/server` 通过。
- 本轮未提交、未部署；当前工作树还叠有 TLS fingerprint/admin settings 相关未提交改动，避免把不属于本问题的改动混入本次提交。

---

日期：2026-06-06
执行者：Devil

## Codex 直连 TLS 指纹默认启用

本轮将 Codex 模拟选项补齐 TLS 指纹选择能力。新增后台设置 `openai_codex_direct_tls_fingerprint_profile_id`，默认值为 `0`，表示在 Codex 直连模式下启用内置 `Built-in Default (Node.js 24.x)` TLS 指纹；`-1` 表示随机已有模板，正数表示使用指定 TLS 指纹模板。运行时仅在 `openai_oauth_compat_mode=codex_direct` 时生效，并覆盖 OpenAI 上游 HTTP 请求的 OAuth 与 API key 账号路径，free5 这类 API key 账号也会走 Codex direct TLS 指纹；`off` 和 `cockpit_tools` 模式不启用该全局指纹。

## 校验方式

- `go test -tags unit ./internal/service -run "TestSettingService_(UpdateSettings_OpenAIOAuthCompatModeRefreshesGatewayConfig|ParseSettings_OpenAIOAuthCompatModeTakesPrecedence|ParseSettings_OpenAICodexDirectTLSFingerprintProfileIDFallsBackToConfig|LoadRuntimeSettingsRefreshesGatewayConfig)" -count=1`
- `go test -tags unit ./internal/service -run "TestOpenAI(UpstreamTLSProfileCodexDirectAppliesToOAuthAndAPIKey|BuildUpstreamRequestCodexDirectCompatibilityHeaders)" -count=1`
- `go test -tags unit ./internal/handler/admin -run "Test.*Setting" -count=1`
- `go test -tags unit ./cmd/server -run TestDoesNotExist -count=1`
- `go test -tags unit ./internal/config -run '^$' -count=1`
- `npm run typecheck`
- `git diff --check`
- `go test -tags unit ./internal/handler -run "TestOpenAI" -count=1`

## 校验结果

- 后端 setting service 聚焦测试通过，确认新设置会写入 repo、热刷新到 `cfg.Gateway.OpenAICodexDirectTLSFingerprintProfileID`，缺失/非法值会按配置默认值回退。
- OpenAI gateway 聚焦测试通过，确认 Codex direct 模式下 OAuth 与 API key 账号都会解析到内置 TLS 指纹，指定模板 ID 时会使用所选模板；关闭 Codex direct 时返回 `nil`，不启用 TLS 指纹。
- admin setting handler 聚焦测试通过，server wire 包和 config 包编译切片通过。
- 前端 `vue-tsc --noEmit` 通过，设置页新增 Codex direct TLS 指纹下拉框和类型字段。
- `git diff --check` 通过，仅提示 docs JSONL 文件未来可能被 Git 转为 CRLF。
- 更宽的 `go test -tags unit ./internal/handler -run "TestOpenAI" -count=1` 失败，失败点在既有 OpenAI handler 用例：`TestOpenAIEnsureForwardErrorResponse_DoesNotOverrideWrittenResponse`、panic fallback response overwrite、`TestOpenAIResponsesWebSocket_ContinuityReplayForwardsSanitizedBodyToNextAccount`。这些失败不在本轮 Codex direct TLS 指纹设置路径上，本轮未修改对应 handler 行为。

---

日期：2026-06-06
执行者：Devil

## OpenAI request-phase failover 与 Codex direct TLS 指纹部署验证

本轮将已提交状态 `bf918f20b975` 通过 `git archive HEAD` 导出干净构建上下文，构建并部署到本地 `sub2api` 容器。该提交包含 HTTP `/responses` 请求阶段 `context canceled` 自动切换修复，以及 Codex direct TLS 指纹选择设置。

## 校验方式

- `go test -tags unit ./internal/service -run "TestOpenAI(UpstreamTLSProfileCodexDirectAppliesToOAuthAndAPIKey|BuildUpstreamRequestCodexDirectCompatibilityHeaders)|TestOpenAIGatewayService_Forward(RequestHeaderTimeoutReturnsFailover|RequestPhaseContextCanceled.*|APIKeyRequestBaseURLFailoverBeforeAccountFailover)|TestOpenAIStreaming(ReadErrorAfterOutputRecordsPathHealthFailure|MissingTerminalEventRecordsPathHealthFailure)" -count=1`
- `go test -tags unit ./internal/service -run "TestSettingService_(UpdateSettings_OpenAIOAuthCompatModeRefreshesGatewayConfig|ParseSettings_OpenAIOAuthCompatModeTakesPrecedence|ParseSettings_OpenAICodexDirectTLSFingerprintProfileIDFallsBackToConfig|LoadRuntimeSettingsRefreshesGatewayConfig)" -count=1`
- `go test -tags unit ./internal/handler/admin -run "Test.*Setting" -count=1`
- `go build ./cmd/server`
- `npm run typecheck`
- `git diff --check`
- JSONL parse check for `docs/feature_list.jsonl` and `docs/process_list.jsonl`
- `git archive --format=tar -o <temp>\sub2api-bf918f20b975.tar HEAD`
- `docker build --pull=false -t sub2api:multi-key-local --build-arg COMMIT=bf918f20b975 <temp>\sub2api-bf918f20b975`
- `docker compose -f D:\sub2api-deploy\docker-compose.yml up -d --force-recreate sub2api`
- `docker inspect sub2api --format '{{.State.Status}} {{if .State.Health}}{{.State.Health.Status}}{{end}} {{.Image}}'`
- `curl.exe -s -o NUL -w "%{http_code} %{size_download}\n" http://127.0.0.1:8080/health`
- `curl.exe -s -o NUL -w "%{http_code} %{size_download}\n" http://127.0.0.1:8080/admin/accounts`
- `curl.exe -s -o NUL -w "%{http_code} %{size_download}\n" "http://127.0.0.1:8080/api/v1/admin/accounts?page=1&page_size=1"`
- `docker logs sub2api --since 2026-06-06T16:17:31+08:00 --tail 500 2>&1 | Select-String -Pattern "panic|fatal|migration.*error|checksum mismatch"`

## 校验结果

- request-phase `context canceled` 自动切换、stream path-health 回归、Codex direct TLS 指纹解析、setting service 热刷新、admin settings handler 聚焦测试均通过。
- 后端 `go build ./cmd/server` 通过，前端 `npm run typecheck` 通过，`git diff --check` 通过，docs JSONL 可解析。
- Docker build 成功，镜像 `sub2api:multi-key-local` manifest list digest 为 `sha256:732a2816dde1623af15102c5f0574d7cfdf4770fde2b394d0ec09d7a1a9ac552`。
- `docker compose` force-recreate 成功，Postgres 和 Redis healthy，`sub2api` 启动成功。
- `docker inspect` 显示 `sub2api` 为 `running healthy`，运行镜像为 `sha256:732a2816dde1623af15102c5f0574d7cfdf4770fde2b394d0ec09d7a1a9ac552`。
- `/health` 返回 HTTP 200，响应大小 15。
- `/admin/accounts` 返回 HTTP 200，响应大小 2671。
- 未登录访问 `GET /api/v1/admin/accounts?page=1&page_size=1` 返回 HTTP 401，认证拦截正常。
- 容器日志关键启动错误过滤未匹配 `panic`、`fatal`、迁移错误或 checksum mismatch。
- 部署后真实 `/responses` 流量返回 HTTP 200；日志中的 `Upstream scan ended after terminal event: context canceled` 是 terminal event 后的流式收尾，不是 request-phase 502。
- 启动日志中有一次远程价格表 hash 拉取 `context deadline exceeded`，属于外部 GitHub 访问超时；服务继续启动并保持 healthy。

---

日期：2026-06-06
执行者：Devil

## OpenAI Responses SSE 初始心跳

本轮在保留真实输出前账号 failover 的前提下，优化本地 `/v1/responses` 流式响应的首字节活性。后端在上游返回 200 且 SSE headers 已设置后立即写入无语义 SSE comment `:\n\n` 并 flush；真实 OpenAI preamble 事件仍按原逻辑缓冲，避免把两个账号的真实事件拼到同一客户端流里。

## 校验方式

- `go test -tags unit ./internal/service -run "TestOpenAIStreaming(ResponseFailedBeforeOutput|ConfiguredResponseTextReturnsFailoverBeforeOutput|PreambleOnlyMissingTerminalReturnsFailover|WaitGuardTimeoutBeforeOutputReturnsFailover|PolicyResponseFailedBeforeOutputPassesThrough|ReadErrorBeforeOutputReturnsFailover|PassthroughResponseFailedBeforeOutputReturnsFailover|PreambleKeepaliveUsesDownstreamIdle)|TestOpenAIGatewayService_Forward(RequestHeaderTimeoutReturnsFailover|RequestPhaseContextCanceled.*|APIKeyRequestBaseURLFailoverBeforeAccountFailover)" -count=1`
- `go test -tags unit ./internal/handler -run "TestOpenAI(HandleStreamingAwareError|EnsureForwardErrorResponse|HandleFailoverExhausted_AppendsResponsesFailedAfterHeartbeat)" -count=1`
- `go build ./cmd/server`
- `git diff --check`

## 校验结果

- 服务层聚焦测试通过，确认上游 200 后会先输出 `:\n\n`，但真实输出标记仍为 false，因此 `response.failed`、响应文本异常、preamble-only、wait guard timeout、read error 等真实输出前失败仍返回 `UpstreamFailoverError`，可继续切账号。
- handler 聚焦测试通过，确认只有真实 OpenAI SSE 事件 flush 后才阻断 failover；仅心跳提交 HTTP 200 后，如果最终账号池耗尽，会追加 Responses 协议兼容的 `response.failed` SSE，而不是写 JSON。
- 后端 `go build ./cmd/server` 通过。
- `git diff --check` 通过，仅提示 docs JSONL 文件未来可能被 Git 转为 CRLF。
- 本轮未部署，未做 okcodex 实测。

## OpenAI zz1cc 真实可用数据时间对照

- 目标：比较 `https://zz1cc.cc.cd/v1/responses` 与本地 `http://127.0.0.1:8080/v1/responses` 在“真实可用文本”到达时的差异。
- 方法：用账号 127 `zz1cc` 的上游 API key 与临时本地 API key，创建临时 group 将 `zz1cc` 单独挂到本地网关，发同一 payload（`model=gpt-5.5`、`stream=true`、`input=只输出两个汉字：收到`）各 3 次，测量 `headersMs`、`firstDataMs`、`firstNonPreambleMs`、`firstTextDeltaMs`。
- 结果：直连 `firstTextDeltaMs` 中位 4184.3ms，本地中位 7017.5ms；直连 `firstNonPreambleMs` 中位 4184.1ms，本地中位 6629.2ms。两边事件序列一致，都是先 `response.created` / `response.in_progress` / `response.metadata` / `response.output_item.added` 再到 `response.output_text.delta`。
- 结论：这次看不出“零拷贝”能把真实可用数据时间显著拉近；秒级差异主要还是 upstream 生成/排队加上本地路由/缓冲/调度开销。账号 127 已恢复 `schedulable=false`、`priority=20`，临时 group/API key 已删除。

## OpenAI funnyapi 真实可用数据时间对照

- 目标：比较账号 25 `funnyapi` 的直连 `https://codex.subgo.qzz.io/v1/responses` 与本地 `http://127.0.0.1:8080/v1/responses` 在真实可用文本到达时的差异。
- 方法：用 `credentials.api_key` 创建临时本地 API key，将账号单独挂到本地网关，发同一 payload（`model=gpt-5.5`、`stream=true`、`input=Reply with OK only.`）各 3 次，测量 `headersMs`、`firstDataMs`、`firstNonPreambleMs`、`firstTextDeltaMs`。
- 结果：直连 `firstTextDeltaMs` 中位 2063.6ms，本地中位 2309.9ms；直连 `firstNonPreambleMs` 中位 1896.0ms，本地中位 1832.6ms。两边事件序列一致，都是先 `response.created` / `response.in_progress` / `response.output_item.added` 再到 `response.output_text.delta`。
- 结论：`funnyapi` 明显比 `zz1cc` 快，但本地网关仍会在真实文本到达前叠加少量路由与缓冲开销。账号 25 已恢复 `schedulable=false`、`priority=11`，临时 group/API key 已删除。

## 上游体检报告最快优先排序

- 目标：选择上游体检报告的平均耗时、P95、首 Token 排序时，优先看到最快记录，而不是最慢记录。
- 方法：在 `AccountProbeReportsView.spec.ts` 里新增排序行为测试，切换 `success_rate`、`avg_latency_ms`、`p95_ms`、`first_token_ms` 后断言传给 `listAccountProbeRuns` 的 `sort_order`。
- 结果：红测确认旧行为在 `avg_latency_ms` 上发出 `sort_order: "desc"`；修复后成功率保持 `desc`，平均耗时、P95、首 Token 都发出 `asc`。
- 验证：`npm run test:run -- src/views/admin/__tests__/AccountProbeReportsView.spec.ts` 通过 11/11；`npm run typecheck` 通过；`git diff --check` 退出 0，仅提示 docs JSONL 行尾将被 Git 转成 CRLF。

## 上游体检报告最快优先排序发布验证

- 日期：2026-06-06T20:43:09+08:00
- 执行者：Devil
- 目标：按“提交、构建、部署、验证”闭环发布上游体检报告耗时类排序修复。
- 提交：`ae58428ce850 fix(frontend): 修复体检报告耗时排序方向`，包含 `frontend/src/views/admin/AccountProbeReportsView.vue` 和 `frontend/src/views/admin/__tests__/AccountProbeReportsView.spec.ts`。
- 构建：前端 `npm run build` 通过；从 `git archive HEAD` 清洁归档构建 Docker 镜像 `sub2api:multi-key-local` 通过，镜像 ID 为 `sha256:e6c782a3da021899154feff60fd058ab1d73528e062a55cca65eb2d02b293ee3`。
- 部署：在 `D:\sub2api-deploy` 执行 `docker compose -f D:\sub2api-deploy\docker-compose.yml up -d --no-build --no-deps --force-recreate sub2api`，只重建应用容器，Postgres/Redis 保持运行。
- 验证：`docker inspect sub2api` 显示 `Status=running`、`Health=healthy`、镜像 ID 匹配；`Invoke-WebRequest http://127.0.0.1:8080/health` 返回 200，内容为 `{"status":"ok"}`。

## sub2api v0.1.134 吸收状态清单

- 日期：2026-06-07T13:06:36+08:00
- 执行者：Devil
- 目标：按用户要求新增一份可长期维护的 v0.1.134 吸收状态清单，后续继续开发时直接更新状态，不再重复拉取 release note。
- 变更：新增 `docs/SUB2API_V0_1_134_ABSORPTION_LIST_CN.md`，按 `[x]`、`[~]`、`[ ]` 标记已完成、并行开发和暂不吸收条目；同步追加 docs JSONL、`.codex/operations-log.md`、`.codex/testing.md`。
- 关键边界：继续保留 OpenAI failover-before-real-output 的真实输出 marker gate，不把初始 SSE heartbeat 算成真实输出。
- 验证：已读取 `docs/SUB2API_V0_1_134_ABSORPTION_LIST_CN.md` 前 120 行确认状态表落地；Python 解析 `docs/feature_list.jsonl` 与 `docs/process_list.jsonl` 通过，分别为 123 与 121 条 JSON 记录；`git diff --check` 通过，仅提示既有 LF-to-CRLF 行尾提醒。

## juhe-ai 借鉴功能 Phase 1/2 开发验证

- 日期：2026-06-07T14:19:14+08:00
- 执行者：Devil
- 目标：按 `docs/JUHE_AI_FEATURE_20250605_BORROWABLE_FEATURES_CN.md` 开始开发可借鉴功能，并给已完成/进行中功能打状态标记。
- 变更：`backend/internal/service/ops_request_details.go` 将 `action_label`、`action_metadata` 从 ops upstream error event 透出到 request timeline 与 Codex diagnosis；`backend/internal/service/ops_request_timeline_test.go` 增加动作字段覆盖；`frontend/src/api/admin/ops.ts` 补充 upstream error event 动作字段类型；借鉴清单把 `账号有效可用性汇总`、`断流动作分级` 标为 `[x]`，把上游桶级避让、半开可见化、断流审计元数据标为 `[~]`。
- 验证：`gofmt -w internal/service/ops_request_details.go internal/service/ops_request_timeline_test.go` 通过；`go test ./internal/service -run "TestOpsServiceGetRequestTimelineIncludesLatencyAndUpstreamErrors|TestOpsServiceGetCodexDiagnosisIncludesBaseURLFailover|TestOpsServiceGetCodexDiagnosisIncludesActionMetadata|TestOpenAIPathHealthRecordFailureWithActionStoresLastActionLabel|TestOpenAIGatewayServiceRequestPhaseFailoverCarriesActionMetadata|TestOpenAIGatewayServiceRecordOpenAIPathHealthFailureLabelsAccountAndBucket" -count=1` 通过；`corepack pnpm typecheck` 通过。

## v0.1.134 本地编译碎片与分组描述清空验证

- 日期：2026-06-07T14:29:15+08:00
- 执行者：Devil
- 目标：按用户要求只修本地编译碎片，把本轮已经写完且验证过的条目落到 `docs/SUB2API_V0_1_134_ABSORPTION_LIST_CN.md`，其他待定项不继续扩大修改。
- 变更：补齐 `emailSyncRepoStub`、`balanceLoadUserRepoStub`、`mockUserRepo`、`emailBindUserRepoStub` 的 `GetByIDIncludeDeleted`；将 `generate_session_hash_test.go` 中旧的 `ParseGatewayRequest([]byte, "gemini")` 调用改为 `ParseGatewayRequest(NewRequestBodyRef(...), "gemini")`；将 v0.1.134 清单里的“修复管理员清空分组描述未持久化”标为 `[x]`。
- OpenAI 保留项：复扫并运行 handler 聚焦测试，`openai_gateway_handler.go` 的 failover gate 仍使用 `service.OpenAIRealClientOutputStarted(c)`，没有回退到 `Writer.Size()` / `Writer.Written()` 判断；初始 SSE heartbeat 仍不会阻断真实输出前 failover。
- 验证：`go test -tags unit ./internal/service -run "TestAdminService_UpdateGroup|TestGenerateSessionHash_Gemini" -count=1` 通过；`go test -tags unit ./internal/service -run TestDoesNotExist -count=1` 通过；`go test ./internal/handler -run "TestOpenAIForwardErrorAlreadyCommunicated_HeartbeatIsNotRealOutput|TestOpenAIHandleFailoverExhausted_AppendsResponsesFailedAfterHeartbeat" -count=1` 通过。

## 账号 API Key 列表单 Key 删除与 429 单 Key 停用验证

- 日期：2026-06-07T15:07:16+08:00
- 执行者：Devil
- 目标：支持管理端删除账号 Key 列表里的单个 Key，并让 API Key 列表账号遇到某个 Key 返回 429 时只停用该 Key，不把整个账号设为不可用。
- 变更：`backend/internal/service/account.go` 新增按指纹删除 Key 并清理停用元数据；`backend/internal/service/admin_service.go`、`backend/internal/handler/admin/account_handler.go`、`backend/internal/server/routes/admin.go` 增加删除接口；`frontend/src/api/admin/accounts.ts` 和 `frontend/src/components/account/EditAccountModal.vue` 增加前端删除动作；`backend/internal/service/ratelimit_service.go` 对 429 优先停用本次选中的 Key；`backend/internal/service/gateway_service.go` 改用 `GetAPIKey()` 取 API Key，确保通用路径也记录 `LastSelectedAPIKey()`。
- 验证：`go test -tags unit ./internal/service -run "TestAccount(GetAPIKey|RemoveAPIKey)|TestGatewayServiceGetAccessTokenUsesCredentialAPIKeys|TestHandleUpstreamError429_OpenAIAPIKeyDisablesSelectedKeyOnly|TestAdminService_DeleteAccountAPIKey" -count=1` 通过；`go test -tags unit ./internal/service -run "TestRateLimitService|TestHandleUpstreamError|TestAccount(GetAPIKey|RemoveAPIKey)|TestGatewayServiceGetAccessTokenUsesCredentialAPIKeys|TestAdminService_DeleteAccountAPIKey" -count=1` 通过；`go test ./internal/handler/admin -run TestDoesNotExist -count=1`、`go test ./internal/server/routes -run TestDoesNotExist -count=1`、`go test -tags unit ./cmd/server -run TestDoesNotExist -count=1` 通过；`npm run test:run -- src/components/account/__tests__/EditAccountModal.spec.ts` 通过 15/15；`npm run typecheck` 通过。

## juhe-ai 上游桶级避让聚合调度验证

- 日期：2026-06-07T16:59:00+08:00
- 执行者：Devil
- 目标：完成 `docs/JUHE_AI_FEATURE_20250605_BORROWABLE_FEATURES_CN.md` Phase 3 的上游桶级避让，让同 proxy/baseURL/transport 的路径故障能影响同 bucket 账号，而不是只惩罚单个账号。
- 变更：`OpenAIPathHealthKey` 新增不带 `AccountID` 的聚合 bucket key helper；OpenAI 断流失败会同时记录账号 key、baseURL key 和聚合 bucket key；账号调度候选、sticky 命中校验、`request_base_urls` 排序都参考账号 key 与 bucket key 中更严重的 path-health 状态。
- 验证：红测 `go test ./internal/service -run TestBuildOpenAIAccountLoadPlanSkipsOpenBucketAcrossAccounts -count=1` 先失败于缺少 `OpenAIPathHealthBucketKeyForAccount`；修复后聚焦测试、相关 path-health/scheduler/streaming 切片、`go test ./internal/service -run TestDoesNotExist -count=1` 和 `go test ./cmd/server -run TestDoesNotExist -count=1` 均通过。

## 单服务部署操作规范验证

- 日期：2026-06-07T17:13:51+08:00
- 执行者：Devil
- 目标：把提交、构建、部署、验证的固定操作步骤写入 `AGENTS.md`，并固化常规部署不重启 Redis/PostgreSQL 的边界。
- 变更：`AGENTS.md` 新增“提交、构建、部署、验证约定”，包含中文拆分提交、`docker build` 镜像构建、`docker compose` 只重建 `sub2api`、新 SQL 仅对运行中 PostgreSQL 定向写入、部署后接口和日志验证清单。
- 验证：复查命令均为 PowerShell 可执行形式；常规部署命令明确使用 `--no-deps --force-recreate sub2api`，没有包含 `postgres` 或 `redis` 服务名；本轮 Go 聚焦测试 `go test ./internal/service -run "Test(OpenAIPathHealthBucketKeyForAccountClearsAccountID|OpenAIGatewayServiceRecordOpenAIPathHealthFailureLabelsAccountAndBucket|BuildOpenAIAccountLoadPlanSkipsOpenBucketAcrossAccounts)" -count=1`、`go test ./internal/service -run TestDoesNotExist -count=1`、`go test ./cmd/server -run TestDoesNotExist -count=1` 均通过。

## juhe-ai P1 可观测性并行开发验证

- 日期：2026-06-07T17:45:23+08:00
- 执行者：Devil
- 目标：并行完成 `docs/JUHE_AI_FEATURE_20250605_BORROWABLE_FEATURES_CN.md` 中 P1 的半开/运行态阻塞可见化、异步副作用队列复用确认、断流审计元数据补齐。
- 变更：`OpenAIGatewayService` 增加运行态阻塞 reason 快照；账号有效可用性新增 `precheck_pending`、`local_suppressed`、`precheck_failed` 并进入管理端 `effective_availability` 徽章；断流审计 `action_metadata` 新增 `stream_action`、`avoidance_scope`、`path_health_state`、`retry_after`；复核 `/responses` usage 记录继续复用 `UsageRecordWorkerPool`，不新增独立 side-effect queue。
- 验证：`go test -tags unit ./internal/service -run "TestOpenAIRuntimeBlock_SnapshotIncludesReasonAndUntil|TestDeriveAccountEffectiveAvailability|TestOpenAIStreamActionMetadataIncludesPathHealthAuditFields|TestOpenAIGatewayServiceRequestPhaseFailoverCarriesActionMetadata" -count=1` 通过；相关 OpenAI path-health/scheduler/gateway 切片通过；`go test ./internal/service -run TestDoesNotExist -count=1`、`go test ./cmd/server -run TestDoesNotExist -count=1` 通过；前端 `npm run typecheck` 通过。

## 账号 API Key 列表 429/403 单 Key 优先停用验证

- 日期：2026-06-07T18:15:59+08:00
- 执行者：Devil
- 目标：修复 API Key 列表账号中单个 Key 返回 429 或 403 余额不足时，Key 状态不变且账号被限流/临时不可调度的问题；确认每个账号的 Key 列表删除能力仍可用。
- 变更：`RateLimitService.HandleUpstreamError` 在通用 `tryTempUnschedulable` 前先处理 `shouldDisableCurrentAPIKey` 覆盖的 API Key 错误，429 与 403 余额不足会优先把 `LastSelectedAPIKey()` 写入 `api_keys_disabled`；新增 429/403 temp rule 场景回归测试。
- 验证：红测命令先失败于 `account_temp_unschedulable` 抢先处理；修复后同命令通过。`go test -tags unit ./internal/service -run "TestHandleUpstreamError429|TestHandle429|TestRateLimitService_HandleUpstreamError_OpenAI403|TestAdminService_DeleteAccountAPIKey|TestAccountRemoveAPIKey|TestGatewayServiceGetAccessTokenUsesCredentialAPIKeys" -count=1` 通过；`go test ./internal/service -run TestDoesNotExist -count=1` 通过；`npm run test:run -- src/components/account/__tests__/EditAccountModal.spec.ts` 通过 15/15；`npm run typecheck` 通过。

## 固定入口代理旁路试运行验证

- 日期：2026-06-07T18:22:43+08:00
- 执行者：Devil
- 目标：先增加固定入口代理，但不切换当前 `8080` 主链路，避免当前 Codex 网关连接因应用容器异常而中断。
- 变更：在 `D:\sub2api-deploy` 新增独立代理 Compose：`docker-compose.proxy.yml`、`proxy/nginx.conf`、`proxy/upstreams/active.conf`。代理容器 `sub2api-proxy` 加入现有 `sub2api-deploy_sub2api-network`，默认绑定 `127.0.0.1:18081->8080`，upstream 指向当前运行容器 `sub2api:8080`。
- 验证：`docker compose -f D:\sub2api-deploy\docker-compose.proxy.yml config` 通过；`docker exec sub2api-proxy nginx -t` 通过；`http://127.0.0.1:18081/health` 返回 200 `{"status":"ok"}`；`http://127.0.0.1:18081/` 返回 200；未登录访问 `http://127.0.0.1:18081/api/v1/admin/accounts?page=1&page_size=1` 返回 401；原主入口 `http://127.0.0.1:8080/health` 仍返回 200；当前应用容器仍为 `sub2api:v0134-absorption-check` 且 `Health=healthy`。

## OpenAI 本地账号并发饱和切号验证

- 日期：2026-06-07T18:28:03+08:00
- 执行者：Devil
- 目标：修复选中账号本地等待队列满或账号 slot 等待超时时，当前请求直接返回 429 而没有继续调度其他可用账号的问题。
- 变更：`acquireResponsesAccountSlot` 改为结构化结果；本地账号队列满和账号 slot 等待超时会生成 `source=local_concurrency`、`stream_action=retry_next_account` 的 failover 结果。Responses、Anthropic Messages、Chat Completions、Images 四个入口收到该结果后会记录本次账号失败、记录切号、把账号加入排除集合并继续调度。所有候选都被本地并发打满时，最终 429 文案明确为 `Local account concurrency saturated`。
- 验证：红测先失败于旧函数签名；修复后 `go test ./internal/handler -run "TestOpenAIAcquireResponsesAccountSlot_(WaitQueueFullSwitchesAccount|WaitTimeoutSwitchesAccount)" -count=1` 通过。相关测试 `go test ./internal/handler -run "TestOpenAI(AcquireResponsesAccountSlot|HandleFailoverExhausted_LocalAccountConcurrency|HandleFailoverExhausted_AppendsResponsesFailedAfterHeartbeat|ForwardErrorAlreadyCommunicated)" -count=1` 通过；`go test ./internal/handler -run TestDoesNotExist -count=1`、`go test ./cmd/server -run TestDoesNotExist -count=1`、`go test ./internal/handler -run "TestConcurrencyHelper" -count=1` 通过；`git diff --check` 通过，仅有既有 LF-to-CRLF 警告。
- 已知无关失败：`go test ./internal/handler -run "TestOpenAI" -count=1` 仍失败于既有 WebSocket continuity 测试 `TestOpenAIResponsesWebSocket_ContinuityReplayForwardsSanitizedBodyToNextAccount`，本轮没有修改 WebSocket 路径。

## OpenAI API Key 更多异常单 Key 暂停验证

- 日期：2026-06-07T18:49:24+08:00
- 执行者：Devil
- 目标：继续修复 aisz 类 OpenAI API Key 列表账号中，单个 Key 返回 400 配额/余额不足、403 Key disabled/revoked/invalid，或账号测试旁路返回 401/429/400 quota 时，Key 状态不变但账号被 SetError、限流或冷却的问题。
- 变更：`shouldDisableCurrentAPIKey` 新增 400 quota 和 403 invalid/disabled/revoked key 判断，普通 workspace forbidden policy 不进入单 Key 停用；`AccountTestService` 新增 `disableOpenAIAPIKeyFromTestError`，并接入 `/v1/chat/completions`、`/responses/compact`、images 以及主 responses 探活错误分支。
- 验证：`go test -tags unit ./internal/service -run "TestAccountTestService_OpenAI(ChatCompletionsPathDisablesCurrentAPIKey|CompactPathDisablesCurrentAPIKey|ImagePathDisablesCurrentAPIKey)|TestRateLimitService_HandleUpstreamError_OpenAIAPIKey(BadRequestQuotaDisablesSelectedKey|ForbiddenInvalidKeyDisablesSelectedKey)|TestShouldDisableCurrentAPIKeySkipsForbiddenPolicy" -count=1` 通过；`go test -tags unit ./internal/service -run "TestAccountTestService_OpenAI|TestRateLimitService_HandleUpstreamError_OpenAIAPIKey|TestHandleUpstreamError429|TestRateLimitService_HandleUpstreamError_OpenAI403|TestShouldDisableCurrentAPIKey" -count=1` 通过；`go test ./internal/service -run TestDoesNotExist -count=1` 通过；相关 `git diff --check` 通过。

## Docker green 候选镜像首次递增版本验证

- 日期：2026-06-07T18:48:38+08:00
- 执行者：Devil
- 目标：按用户确认的规则，将当前可用镜像判定为 `sub2api:v0134-absorption-check`，将最新本地构建源判定为 `sub2api:multi-key-local`，并基于该源生成首次不可变 green 候选版本。
- 版本：`sub2api:multi-key-local` 与 `sub2api:v20260607.1-666797082235` 指向同一镜像 ID `8c12ff741670`；当前稳定容器 `sub2api` 仍运行 `sub2api:v0134-absorption-check` 且 `Health=healthy`。
- 结果：`sub2api-green` 使用 `sub2api:v20260607.1-666797082235` 启动失败，已停止在 `Exited (1)` 状态，未执行代理 reload，未把 `18081` 切到 green。
- 根因证据：green 日志报 `migration 157_user_platform_quotas.sql checksum mismatch`，数据库 `schema_migrations` 记录 checksum 为 `21cccff17048932c21048b4868f5b2685f9f7208607dcd3c62c89a9ea2d9a400`，失败镜像启动时报 file checksum 为 `fe485a0663f8948174819a2d36b06f260c2dd8a8e85b6e39cd85798b75090873`；当前源码按迁移 runner 的 trim 后 SHA256 计算结果与数据库一致，为 `21cccff17048932c21048b4868f5b2685f9f7208607dcd3c62c89a9ea2d9a400`。
- 结论：首个候选版本 `sub2api:v20260607.1-666797082235` 判定失败并禁止复用；不能修改数据库 checksum 绕过，下一次应基于修正后的最新构建重新生成递增版本，例如 `v20260607.2-...`。
- 安全验证：`http://127.0.0.1:8080/health` 返回 200 `{"status":"ok"}`；固定入口 `http://127.0.0.1:18081/health` 返回 200 `{"status":"ok"}`；`D:\sub2api-deploy\proxy\upstreams\active.conf` 仍指向 `sub2api:8080`。

## OpenAI 本地并发切号与迁移 checksum 修复发布验证

- 日期：2026-06-07T19:24:17+08:00
- 执行者：Devil
- 提交：`7c8bfdca3 fix(openai): 本地并发饱和时切换账号`；`708e0c307 fix(migrations): 兼容历史换行校验`。
- 构建：先构建 `sub2api:v20260607.2-7c8bfdca398b`，green 仍失败于 157 checksum mismatch；根因确认为数据库记录的是 CRLF checksum，干净 Git 归档镜像是 LF checksum。随后补迁移 checksum 兼容白名单并构建 `sub2api:v20260607.3-708e0c307a5e`，同时更新 `sub2api:multi-key-local` 到同一镜像。
- 部署：`D:\sub2api-deploy\docker-compose.green.yml` 已指向 `sub2api:v20260607.3-708e0c307a5e`；只重建 `sub2api-green` 候选，PostgreSQL、Redis 和原 `sub2api:v0134-absorption-check` 未重启。green 健康后，将 `D:\sub2api-deploy\proxy\upstreams\active.conf` 从 `sub2api:8080` 切到 `sub2api-green:8080` 并执行 `nginx -s reload`。
- 验证：提交前后 Go 聚焦测试通过，包括 handler 本地并发切号、repository checksum 兼容、service API Key 异常单 Key 暂停、cmd/server 编译切片。green `http://127.0.0.1:18082/health` 返回 200，根路径 200，未登录 admin API 返回 401；`docker exec sub2api-green /app/sub2api --version` 显示 `v20260607.3-708e0c307a5e`；green 最近日志过滤无 `panic`、`fatal`、`migration.*fail`、`checksum`、`pq:`、`bind`、`listen`。
- 切流结果：固定入口 `http://127.0.0.1:18081/health` 返回 200，根路径 200，未登录 admin API 返回 401；nginx 日志显示 reload 后 upstream 为 `172.23.0.7:8080`（green），旧 upstream `172.23.0.4:8080` 不再承接 18081 验证请求。原 `http://127.0.0.1:8080/health` 仍返回 200，旧容器保留为回滚目标。

## 固定入口代理正式接管 8080 验证

- 日期：2026-06-07T19:49:52+08:00
- 执行者：Devil
- 目标：按用户要求将正式入口 `8080` 切到固定入口代理，允许短暂中断服务，后续普通客户端继续连接 `8080` 即可走新 green。
- 变更：`D:\sub2api-deploy\docker-compose.proxy.yml` 新增 `0.0.0.0:${SUB2API_PROXY_PUBLIC_PORT:-8080}:8080` 端口绑定；停止旧容器 `sub2api:v0134-absorption-check` 释放 8080 后，强制重建 `sub2api-proxy` 让 proxy 同时绑定 `0.0.0.0:8080` 和 `127.0.0.1:18081`；`active.conf` 保持指向 `sub2api-green:8080`。
- 验证：切换后 `http://127.0.0.1:8080/health` 返回 200，根路径 200，未登录 admin API 返回 401；`http://127.0.0.1:18081/health` 和 `http://127.0.0.1:18082/health` 均返回 200；`docker ps` 显示 `sub2api-proxy` 绑定 `0.0.0.0:8080->8080/tcp` 与 `127.0.0.1:18081->8080/tcp`，`sub2api-green` healthy，旧 `sub2api` 已停止；nginx 日志显示 8080 上的后台管理、静态资源与 Codex `/responses` 请求均转发到 `172.23.0.7:8080`。
- 备注：日志中 `/v1/messages` 出现的 502/499 已进入 `sub2api-green`，对应 green 内部上游账号限流、上游 EOF 或客户端取消，不属于 8080 端口或 nginx 切换失败。

## 2026-06-07 20:33 +08:00 - OpenAI `/responses` quota/billing 流式失败保活

- 执行者：Devil
- 目标：修复 OpenAI 上游 `response.failed` 中的 quota/billing/insufficient_quota 在已有 partial delta 后被透传给 Codex 客户端，导致 goal 中途断掉的问题。
- 变更：`backend/internal/service/openai_gateway_service.go` 的普通 `/responses` 流和 passthrough 流都延迟提交真实 SSE 事件：成功终态后才 flush 给客户端；遇到可 failover 的 `response.failed` 时只保留心跳并返回 `UpstreamFailoverError`，让上层继续切账号；策略/安全失败仍透传。`backend/internal/service/openai_gateway_service_test.go` 新增普通流和 passthrough 流 quota-after-output 回归测试。
- 验证：`go test ./internal/service -run "TestOpenAIStreaming(QuotaFailedAfterOutputReturnsFailover|PassthroughQuotaFailedAfterOutputReturnsFailover)" -count=1` 通过；`go test ./internal/service -run "TestOpenAIStreaming" -count=1` 通过；`go test ./internal/service -run TestDoesNotExist -count=1` 通过；`git diff --check` 通过。
- 备注：`go test ./internal/service -count=1` 全包仍失败，失败点集中在既有 WS/rate-limit 测试：`TestOpenAIGatewayService_Forward_WSv2*` 与 `TestOpenAIGatewayService_UpdateCodexUsageSnapshot_ExhaustedSnapshotSetsRateLimit`，本轮目标流式 `/responses` 验证已通过。

## 2026-06-07 21:00 +08:00 - OpenAI Responses Codex goal failover 计划工程复核

- 执行者：Devil
- 目标：按 `plan-eng-review` 视角复核当前 quota/billing 流式保活实现是否符合 `docs/OPENAI_RESPONSES_CODEX_GOAL_STREAM_FAILOVER_PLAN_CN.md` 与 juhe-ai feature 借鉴方向，并优化开发计划。
- 结论：当前实现可作为 Phase 0 止血，能避免 Codex 客户端直接收到 quota/billing `response.failed`，但通过成功终态前全量缓冲真实 SSE 牺牲流式体验，不符合最终计划。计划文档已补充 Phase 1-5：event 级缓冲、首字前所有流内失败 failover、首字后 Codex retryable 改写、turn/request 级避让、ops 可见性与测试矩阵。
- 验证：CodeGraph 成功返回 `openAIStreamFailedEventShouldFailover`、`handleStreamingResponsePassthrough`、`handleStreamingResponse` 等关键符号；PowerShell 行级复核当前 service、handler、测试与 juhe 借鉴文档；`git diff --check` 通过，仅有既有 `docs/feature_list.jsonl` 与 `docs/process_list.jsonl` LF-to-CRLF warning。
- 未执行：本轮只更新本地计划文档和记录，未改 Go 行为代码，因此未跑 Go 测试。

## 2026-06-07 21:25 +08:00 - Codex agentic 请求判定与 v0.1.134 版本规则校正

- 执行者：Devil
- 目标：回应用户两个补充要求：先判断当前接收到的 `/responses` 请求是否满足计划里的 `isCodexAgenticRequest`，并把部署版本规则从日期/自主序号改回 Git 版本线 `v0.1.134`。
- 结论：当前请求应判定为 Codex agentic。入口代理最近 30 分钟持续记录 `POST /responses`，User-Agent 为 `Codex Desktop/0.137.0-alpha.4 (Windows 10.0.26200; x86_64) unknown (Codex Desktop; 26.602.40724)`；服务端持续记录 `[OpenAI] Injected /responses image_generation tool for Codex client`。该日志只会在 `isCodexCLI` 命中且 Codex image_generation bridge 启用后出现，因此已经证明当前请求命中了 Codex 官方客户端路径。当前日志窗口没有直接打印 `stream=true` 和 `tools_count`，这两项由既有 Codex `/responses` 解析样本和 Codex Desktop 行为支持，后续 helper 需要用 body 测试或诊断字段显式补强。
- 变更：`AGENTS.md` 的候选镜像规则已从 `sub2api:vYYYYMMDD.N-<12位commit>` 改为跟随 Git 版本线，本轮目标版本为 `sub2api:v0.1.134`；`docs/OPENAI_RESPONSES_CODEX_GOAL_STREAM_FAILOVER_PLAN_CN.md` 的 NOT in scope 同步声明不使用日期/自主序号候选。
- 验证：`git show-ref --tags v0.1.134` 确认 tag 存在；`docker logs sub2api-proxy --since 30m` 看到 Codex Desktop `/responses` 请求；`docker logs sub2api-green --since 30m` 看到 Codex image_generation bridge 注入日志；CodeGraph `status` 仍为 `Transport closed`，按项目降级规则使用 PowerShell 与容器日志复核。
- 未执行：未改 Go 行为代码，因此未跑 Go 测试。

## 2026-06-07 21:42 +08:00 - OpenAI `/responses` 中转保护范围放宽

- 执行者：Devil
- 目标：按用户反馈修正计划：流式失败保护不应依赖 Codex 客户端信号，只要请求走 sub2api 中转就应该有这个功能。
- 变更：`docs/OPENAI_RESPONSES_CODEX_GOAL_STREAM_FAILOVER_PLAN_CN.md` 已把 `isCodexAgenticRequest` 口径改为 `isGatewayProtectedResponsesRequest`。硬条件只保留“进入 sub2api OpenAI `/responses` 中转链路”；Codex UA、`x-codex-*`、`stream=true`、`tools_count`、`turn_id` 仅作为诊断和 retry key 精度增强字段，不再作为功能开关。Phase 3、状态机、测试计划、验收标准和生产失败模式同步改为 gateway retryable 语义。
- 验证：PowerShell 扫描计划文档，确认不再保留 `isCodexAgenticRequest`、`Codex agentic`、`Codex retryable` 作为启用条件的旧口径。
- 未执行：未改 Go 行为代码，因此未跑 Go 测试。

## 2026-06-07 22:07 +08:00 - OpenAI `/responses` Phase 1-3 并行开发集成验证

- 执行者：Devil
- 目标：参照并行开发结果，把 OpenAI `/responses` SSE failover 从 Phase 0 止血推进到 Phase 1-3 最小实现，并验证普通流、passthrough 流和 handler 首字前 failover 相关路径。
- 变更：普通 `/responses` 流和 passthrough 流在首个真实 SSE event 到达完整边界后立即 flush preamble + 当前 event；首字前任意 `response.failed` 返回 `UpstreamFailoverError`；首字后上游 `response.failed` 改写为网关生成的脱敏 `response.failed`，错误码为 `upstream_retryable_error`，不再把 quota/billing 原文下发给客户端。`docs/OPENAI_RESPONSES_CODEX_GOAL_STREAM_FAILOVER_PLAN_CN.md` 已同步标记 Phase 1-3 最小 HTTP/SSE 实现完成，Phase 4/5 仍待实现。
- 并行审计证据：Hubble 确认测试应从 Phase 0 的 after-output failover 断言替换为实时首字和 gateway retryable；Euclid 确认 handler 当前请求内 `failedAccountIDs` 足以承接首字前切号，首字后必须由 service 写 retryable 失败事件，不能返回 `UpstreamFailoverError` 让 handler 在真实输出后切号。
- 验证：`go test ./internal/service -run "TestOpenAIStreaming" -count=1` 通过；`go test ./internal/service -run "TestOpenAIStreaming|TestOpenAIGatewayServiceRequestPhaseFailoverCarriesActionMetadata|TestOpenAIGatewayServiceRecordOpenAIPathHealthFailureLabelsAccountAndBucket" -count=1` 通过；`go test ./internal/handler -run "TestOpenAIForwardErrorAlreadyCommunicated_HeartbeatIsNotRealOutput|TestOpenAIHandleFailoverExhausted_AppendsResponsesFailedAfterHeartbeat" -count=1` 通过；`go test ./cmd/server -run TestDoesNotExist -count=1` 通过；`go test ./internal/service -run TestDoesNotExist -count=1` 通过；`git diff --check -- backend/internal/service/openai_gateway_service.go backend/internal/service/openai_gateway_service_test.go` 通过。
- 已知无关失败：更宽的 service WS/rate-limit/usage snapshot 检查仍失败于既有 `TestOpenAIGatewayService_Forward_WSv2*` 与 `TestOpenAIGatewayService_UpdateCodexUsageSnapshot_ExhaustedSnapshotSetsRateLimit`；handler WebSocket continuity 仍失败于 `TestOpenAIResponsesWebSocket_ContinuityReplayForwardsSanitizedBodyToNextAccount`。本轮没有修改 WebSocket 路径。
- CodeGraph 降级：`codegraph_context`、`codegraph_status`、`codegraph_search` 连续返回 `Transport closed`；`.codegraph/daemon.log` 与 `Get-Process -Id 8556` 显示 daemon 进程仍在。按项目规则记录降级，使用 PowerShell 行级复核源码和测试。

## 2026-06-07 22:26 +08:00 - v0.1.134 补丁版本线校正

- 执行者：Devil
- 目标：按用户纠正，版本不要跳到 `v0.1.135`，而应沿 `v0.1.134` 向后延伸为 `v0.1.134.1`。
- 变更：`AGENTS.md` 的发布规则改为当前版本线从 `sub2api:v0.1.134` 向 `sub2api:v0.1.134.N` 补丁延伸，本轮目标版本固定为 `sub2api:v0.1.134.1`；构建示例同步改为 `$version = "v0.1.134.1"`。
- 清理：误建的本地 Git tag `v0.1.135` 已删除；误构建的本地镜像 `sub2api:v0.1.135` 已删除，后续不使用该版本。
- 验证：`git tag --list 'v0.1.134*' 'v0.1.135'` 仅保留 `v0.1.134`；`docker images` 已查不到 `sub2api:v0.1.134` 或 `sub2api:v0.1.135` 发布镜像。

## 2026-06-07 22:45 +08:00 - 8080 502 恢复与 blue/green 发布流程修正

- 执行者：Devil
- 问题：部署时直接重建了当前 active 的 `sub2api-green`，没有先起新的 idle 容器验证，导致 nginx upstream 指向的旧容器连接失效，公网 `http://localhost:8080/responses` 一度返回 `502 Bad Gateway`。
- 恢复：执行 `docker exec sub2api-proxy nginx -t` 与 `docker exec sub2api-proxy nginx -s reload` 后，`http://127.0.0.1:8080/health`、`http://127.0.0.1:18081/health`、`http://127.0.0.1:18082/health` 均恢复 200。
- 流程修正：新增 `D:\sub2api-deploy\docker-compose.blue.yml`，启动独立 `sub2api-blue`，默认镜像为上一回滚版本 `sub2api:v0134-absorption-check`，端口为 `127.0.0.1:18083:8080`。当前 active upstream 保持 `sub2api-green:8080`，后续当 active 为 green 时，下一版必须先部署到 blue、验证 blue、再修改 `active.conf` 并 reload nginx，禁止重建 active green。
- 当前状态：`sub2api-green` 运行 `sub2api:v0.1.134.1` 且 healthy；`sub2api-blue` 运行 `sub2api:v0134-absorption-check` 且 healthy；`sub2api-proxy` 继续绑定 `0.0.0.0:8080` 和 `127.0.0.1:18081`；未重启 PostgreSQL 与 Redis。
- 验证：稳定等待 65 秒后，`8080/health`、`18081/health`、`18082/health`、`18083/health`、根路径均返回 200；`GET /api/v1/admin/dashboard/stats` 未登录返回 401；`POST /responses` 未登录返回 401，确认不再是 nginx 502。
- 日志：宽泛 `bind` 关键字命中 4 条 `openai.ws_bind_response_account_failed` WARN，内容为客户端取消导致的 `context canceled`，不是端口绑定失败；更精确过滤 `panic|fatal|migration.*fail|checksum|pq:|bind:|address already in use|listen tcp|rebuild failed` 对 `sub2api-green`、`sub2api-blue`、`sub2api-proxy` 均为 0。

## 2026-06-07 22:53 +08:00 - 蓝绿交替规则固化与本地版本号校正

- 执行者：Devil
- 目标：按用户要求，把 blue/green 交替发布规则写入 `AGENTS.md`，并把本地 sub2api 代码内置版本从 `0.1.133` 改为 `0.1.134`。
- 变更：`AGENTS.md` 明确发布前必须先读取 `D:\sub2api-deploy\proxy\upstreams\active.conf` 判断 active 颜色；新版本只能部署到相反颜色的 idle 容器；idle 容器完全启动、候选端口可访问、健康稳定并完成冒烟后，才允许修改 upstream 并 reload nginx；切流后旧 active 必须继续运行作为回滚目标。文档中补充了 green 新/blue 旧时下一次发 blue，再下一次发 green 的交替示例。
- 版本：`backend/cmd/server/VERSION` 从 `0.1.133` 改为 `0.1.134`，运行时默认版本来源仍为 `cmd/server/main.go` embed 的 `VERSION` 文件，构建期 `-ldflags main.Version=...` 仍可覆盖。
- 验证：`go test ./cmd/server -run TestDoesNotExist -count=1` 通过；`go run ./cmd/server --version` 输出 `Sub2API 0.1.134 (commit: unknown, built: unknown)`；`git diff --check -- AGENTS.md backend/cmd/server/VERSION` 通过，仅输出既有 LF-to-CRLF 工作区提示。

## 2026-06-07 23:09 +08:00 - v0.1.134.2 blue 部署与切流验证

- 执行者：Devil
- 提交与标签：`ac81d0b95 docs(deploy): 固化蓝绿交替发布规则`；Git tag `v0.1.134.2` 指向该提交。
- 构建：首次按默认基础镜像构建时，Docker Hub 阿里镜像源对 `node:24-alpine`、`golang:1.26.3-alpine`、`alpine:3.21` 元数据请求返回 403；随后使用 `m.daocloud.io/docker.io/library/...` 基础镜像参数重试成功，生成 `sub2api:v0.1.134.2`，label revision 为 `ac81d0b9535e`，运行时版本为 `0.1.134`。
- 部署：部署前 `active.conf` 指向 `sub2api-green:8080`，因此按新规则只重建 idle 的 `sub2api-blue`；`D:\sub2api-deploy\docker-compose.blue.yml` 默认镜像更新为 `sub2api:v0.1.134.2`，执行 `docker compose -f docker-compose.blue.yml up -d --no-deps --force-recreate sub2api-blue`。未重启 PostgreSQL、Redis、proxy 或 active green。
- blue 候选验证：等待 65 秒后，`http://127.0.0.1:18083/health` 返回 200，根路径 200，`GET /api/v1/admin/dashboard/stats` 未登录返回 401，`POST /responses` 未登录返回 401；`docker exec sub2api-blue /app/sub2api --version` 输出 `Sub2API 0.1.134 (commit: ac81d0b9535e, built: 2026-06-07T15:05:08Z)`；精确错误日志过滤为 0。
- 切流验证：将 `D:\sub2api-deploy\proxy\upstreams\active.conf` 从 `sub2api-green:8080` 改为 `sub2api-blue:8080`，`docker exec sub2api-proxy nginx -t` 与 reload 通过；公网 `http://127.0.0.1:8080/health` 200，`18081/health` 200，`18083/health` 200，保留的 green `18082/health` 200，根路径 200，admin 未登录 401，`POST /responses` 未登录 401；`sub2api-blue` 为 active healthy，`sub2api-green` 仍运行 `sub2api:v0.1.134.1` 作为回滚。

## 2026-06-07 23:57 +08:00 - JUHE_AI P2 功能开发验证

- 执行者：Devil
- 目标：并行完成 `docs/JUHE_AI_FEATURE_20250605_BORROWABLE_FEATURES_CN.md` 中剩余三个 P2：可配置流式拦截策略、错误处理策略规则、管理端动作模板说明。
- 变更：新增 `backend/internal/service/openai_stream_policy.go`，用固定内置规则覆盖 OpenAI `response.failed` 的 quota/billing、capacity/overload、policy/invalid_request；`openai_gateway_service.go` 将 request/http_response/stream 三个阶段都写入动作标签和 action_metadata；前端新增 `actionTemplates.ts`、Vitest 单测和中英文 i18n，Ops Codex 诊断时间线展示动作说明和关键元数据；JUHE 清单三个 P2 已标记 `[x]`。
- 验证：`go test ./internal/service -run "TestOpenAIStreamInterceptDecisionUsesBuiltInRules|TestOpenAIUpstreamErrorPolicySeparatesPhasesAndActions" -count=1` 通过；`go test ./internal/service -run "TestOpenAIStream|TestOpenAIGatewayServiceRequestPhaseFailoverCarriesActionMetadata|TestOpenAIHTTPResponsePolicyCarriesActionMetadata|TestOpenAIUpstreamErrorPolicy|TestClassifyUpstreamError" -count=1` 通过；`npm run test:run -- src/views/admin/ops/utils/__tests__/actionTemplates.spec.ts` 通过；`npm run typecheck` 通过。
- 风险：当前“可配置流式拦截策略”按清单适配方式先实现为固定内置规则，尚未开放管理端动态配置；后续可复用现有规则结构和 action_metadata 扩展配置仓储。

## 2026-06-08 00:24 +08:00 - OpenAI `/responses` 可调度账号耗尽探测恢复

- 执行者：Devil
- 目标：修正 OpenAI `/responses` 调度器在“没有可调度账号”时直接把 429/调度耗尽错误返回客户端的问题；默认先对同一调度范围内账号做小请求探测，每个候选账号最多 6 次；新增全局开关允许持续探测等待直到账号恢复。
- 变更：新增 `OpenAIGatewayService.RecoverOpenAISchedulerExhaustion`，当 handler 调度失败且本次请求还没有失败账号时触发；候选账号限定在同一调度范围，保留 active、schedulable、模型和 `/responses` 能力过滤，但不使用运行时 `IsSchedulable()`，因此可探测 rate-limit/temp-unsched/runtime block 状态中的账号。探测成功后清理运行时调度屏蔽、恢复 rate-limit 状态、写 path health 成功并重新进入真实调度。新增 `openai_scheduler_exhaustion_probe_infinite_wait_enabled` 全局设置，管理端可开关，默认关闭。
- 验证：`gofmt` 已执行；旧长命名扫描无命中；后端设置热刷新、探测次数/成功/无限等待、相邻 service failover、handler `/responses` 聚焦用例均通过；前端 `npm run typecheck` 通过；`git diff --check` 退出 0，仅提示既有 `docs/feature_list.jsonl` 与 `docs/process_list.jsonl` LF-to-CRLF。
- 已知无关失败：更宽的 `go test ./internal/handler -run "TestOpenAI" -count=1` 仍失败于既有 WebSocket continuity 用例 `TestOpenAIResponsesWebSocket_ContinuityReplayForwardsSanitizedBodyToNextAccount`，期望 `resp_handler_continuity_replayed`，实际 `resp_should_not_use_exhausted`；本轮未改 WebSocket continuity 路径。

## 2026-06-08 07:58 +08:00 - v0.1.134.3 green 国内源构建部署与切流验证

- 执行者：Devil
- 提交与标签：`901e5e0f0 docs(git): 记录按功能提交整理`；Git tag `v0.1.134.3` 指向该提交。
- 构建：使用国内源基础镜像参数 `m.daocloud.io/docker.io/library/node:24-alpine`、`golang:1.26.3-alpine`、`alpine:3.21`、`postgres:18-alpine` 从 `git archive HEAD` 构建不可变镜像 `sub2api:v0.1.134.3`。镜像 label revision 为 `901e5e0f0fe9`，`docker run --rm --entrypoint /app/sub2api sub2api:v0.1.134.3 --version` 输出 `Sub2API 0.1.134 (commit: 901e5e0f0fe9, built: 2026-06-07T23:52:49Z)`。
- 部署：部署前 `active.conf` 指向 `sub2api-blue:8080`，active blue 运行 `sub2api:v0.1.134.2` 且 healthy；本轮只更新 idle green，`D:\sub2api-deploy\docker-compose.green.yml` 默认镜像改为 `sub2api:v0.1.134.3`，执行 `docker compose -f docker-compose.green.yml up -d --no-deps --force-recreate sub2api-green`。未重启 PostgreSQL、Redis、proxy 或 active blue。
- green 候选验证：等待 65 秒后，`sub2api-green` 为 `ConfigImage=sub2api:v0.1.134.3` 且 healthy；`http://127.0.0.1:18082/health` 200，根路径 200，`GET /api/v1/admin/dashboard/stats` 未登录 401，`POST /responses` 未登录 401；候选日志精确过滤 `panic|fatal|migration.*fail|checksum|pq:|bind:|address already in use|listen tcp|rebuild failed` 为 0。
- 切流验证：将 `D:\sub2api-deploy\proxy\upstreams\active.conf` 从 `sub2api-blue:8080` 改为 `sub2api-green:8080`，`docker exec sub2api-proxy nginx -t` 与 reload 通过。切流后 `8080/health`、`18081/health`、`18082/health`、`18083/health` 均 200；公网根路径 200，admin 未登录 401，`POST /responses` 未登录 401；`sub2api-green` active healthy，`sub2api-blue` 继续运行 `sub2api:v0.1.134.2` 作为回滚目标，proxy 错误日志过滤为 0。

## 2026-06-08 08:20 +08:00 - 主版本外新增镜像子版本展示

- 执行者：Devil
- 目标：按用户要求保留主版本 `0.1.134` 展示，同时新增镜像子版本展示位置，用于显示 `v0.1.134.N` 这类镜像发布标签。
- 变更：后端 BuildInfo、UpdateService、`/admin/system/version` 与 `/admin/system/check-updates` 增加 `image_version`；Docker 构建参数增加 `IMAGE_VERSION` 并通过 ldflags 注入 `main.ImageVersion`；前端 VersionInfo、App Store 和 VersionBadge 已缓存并展示镜像版本，管理员版本 badge 顶部和下拉详情均可看到镜像版本，主版本仍保持 `v0.1.134` 主视觉。
- 验证：`go test ./internal/service -run TestUpdateServiceCheckUpdateExposesImageVersionSeparately -count=1` 通过；`go test ./cmd/server -run TestProvideServiceBuildInfo -count=1` 通过；`go run -ldflags "-X main.Version=0.1.134 -X main.ImageVersion=v0.1.134.4 -X main.Commit=testcommit -X main.Date=2026-06-08T00:00:00Z" ./cmd/server --version` 输出主版本和 image 版本；`vitest run src/stores/__tests__/app.spec.ts src/components/common/__tests__/VersionBadge.spec.ts` 通过；`npm run typecheck` 通过；`npm run build` 通过。
- 说明：本轮未构建 Docker 镜像、未部署、未切流；当前线上仍是上一轮 active green `sub2api:v0.1.134.3`。

## 2026-06-08 08:23 +08:00 - NewAPI 上游额度刷新失败排查

- 执行者：Devil
- 目标：按用户要求查明 NewAPI 额度刷新失败原因，并使用 `zz1cc` 验证。
- 证据：当前 active 为 `sub2api-green`，镜像 `sub2api:v0.1.134.3`；`zz1cc` 对应 account_id `127`，base_url 为 `https://zz1cc.cc.cd`，有上游登录用户名/密码和 1 个 API key。`settings` 中 `realtime_balance_prewarm_enabled=true`、`realtime_balance_prewarm_interval_seconds=60`、`realtime_balance_prewarm_active_account_limit=20`；当前 20 个 OpenAI apikey 账号中 13 个带 NewAPI 登录凭证。
- 现象：最近 30 分钟 `sub2api-green` 日志中 account 127 出现 10 次 `upstream_balance.login_failed`，错误均为 `https://zz1cc.cc.cd/api/user/login returned 429`。同类账号 181/184 也反复 429，说明不是单账号密码错误。
- `zz1cc` 直连验证：宿主机读取同一账号配置后，`POST /api/user/login` 返回 200 并设置 Cookie；带 Cookie + `New-Api-User=378` 请求 `/api/user/self` 返回 200 且包含 `quota`，`/api/user/groups` 与 `/api/pricing` 也可用。直接 API key 兜底端点中 `/api/v1/usage` 为 404，`/api/usage/token/` 返回 `unlimited_quota=true` 且没有有限余额，不能作为可用额度来源。
- 结论：失败根因是后台实时余额预热每 60 秒批量登录 NewAPI，触发上游登录限流；一旦登录态拿不到，API key 兜底路径在 `zz1cc` 上又没有可解析的有限余额，所以界面看到额度刷新失败。账号本身和上游 quota 数据是可用的。
- 未执行：本轮未改业务代码、未部署、未切流。

## 2026-06-08 08:40 +08:00 - NewAPI 上游额度刷新持久化登录态

- 执行者：Devil
- 目标：降低 NewAPI 额度刷新登录频率，把上游登录所需 token/cookie/`New-Api-User` 持久化到账号 extra，失效后再重新登录。
- 变更：`backend/internal/service/upstream_balance.go` 新增 `upstream_auth_session` extra 读写；登录成功后保存 token、cookie、`new_api_user`、`expires_at` 和登录配置 cache key；刷新前优先使用 DB/进程缓存会话请求 `/api/user/self` 等登录态余额接口；缓存会话拿不到余额时清理旧会话并重新登录写回。缓存 key 移除 `account.updated_at`，改为账号 ID、base URL、用户名和密码指纹，避免余额字段更新打穿缓存。`backend/internal/config/config.go` 将 `gateway.realtime_balance_prewarm.interval_seconds` 默认值从 60 调整为 900，服务初始化时也会把低于 15 分钟的间隔夹到 15 分钟。
- 验证：新增测试先红后绿；`go test ./internal/service -run "TestNewUpstreamBalanceServiceClampsShortRefreshInterval|TestUpstreamBalanceService(UsesPersistedAuthenticatedSessionBeforeLogin|RefreshesPersistedSessionAfterUnauthorized)" -count=1 -v` 通过；`go test ./internal/service -run "TestUpstreamBalanceService|TestNewUpstreamBalanceServiceClampsShortRefreshInterval|TestLoginUpstream|TestParseNewAPI" -count=1` 通过；`go test ./internal/service -run TestDoesNotExist -count=1` 通过；`go test ./internal/config -count=1` 通过。
- 说明：本轮未提交、未构建 Docker 镜像、未部署、未切流；当前线上仍是上一轮 active green，发布后即使 DB 中 `realtime_balance_prewarm_interval_seconds` 仍为 60，服务启动也会按 15 分钟下限执行。

## 2026-06-08 09:00 +08:00 - OpenAI 无限调度等待长时间通知

- 执行者：Devil
- 目标：在 OpenAI `/responses` 可调度账号耗尽且开启无限小请求探测时，等待过久可通知管理员，避免客户端长时间无响应但无人感知。
- 变更：新增 `openai_scheduler_exhaustion_probe_notify_*` 配置和管理端设置，支持通知开关、首次通知秒数、重复通知秒数、飞书机器人 webhook 和恢复通知开关；无限探测循环超过阈值后发送 waiting 通知并按重复间隔限流，已发送等待通知后账号恢复会按配置发送 recovered 通知。飞书通知使用 5 秒超时的 text webhook，通知失败只写日志，不中断调度探测。
- 验证：`go test ./internal/service -run "TestOpenAISchedulerExhaustionProbe" -count=1` 通过；`go test -tags unit ./internal/service -run "TestSettingService_UpdateSettings_OpenAISchedulerExhaustionProbe" -count=1` 通过；`go test ./internal/handler/admin -run TestNonExistent -count=0` 通过；`npm run typecheck` 通过。
- 说明：本轮未提交、未构建 Docker 镜像、未部署、未切流；飞书 webhook 默认留空，通知开关默认关闭。

## 2026-06-08 12:14 +08:00 - OpenAI 账号级 Codex CLI 模拟开关

- 执行者：Devil
- 目标：让 `free5` 这类 OpenAI API Key 账号可通过账号级开关在请求上游时模拟 Codex CLI；未开启时不改变默认上游请求规则，避免全局影响首字/首 token 表现。
- 变更：前端新增、编辑、批量编辑账号弹窗已提供 `请求上游时模拟 Codex CLI` 配置入口；开关写入账号 extra 的 `openai_codex_cli_simulation_enabled`。后端按账号 extra 显式 true 才在 HTTP/SSE、自动透传和 WS 上游请求中应用 Codex CLI User-Agent、originator 与版本头，并保留 Codex 请求体字段；关闭或未设置时保持原规则。
- 验证：`npm run typecheck` 通过；`go test ./internal/service -run "TestAccount_IsOpenAICodexCLISimulationEnabled|TestOpenAIGatewayService_APIKey(CodexCLISimulation|PassthroughCodexCLISimulation)|TestOpenAIGatewayService_OAuthLegacy_CompositeCodexUAUsesCodexOriginator" -count=1 -v` 通过。
- 说明：本轮未构建 Docker 镜像、未部署、未切流；需要上线后在对应 OpenAI 账号上手动开启该账号级开关。

## 2026-06-08 12:48 +08:00 - v0.1.134.7 部署与 free5 模拟验证

- 执行者：Devil
- 修正：复核部署前发现 `shouldSimulateOpenAICodexCLI` 仍受旧 `Gateway.ForceCodexCLI` 影响，已按账号级开关语义收口为只读取 `accounts.extra.openai_codex_cli_simulation_enabled`；`openai_ws_v2_passthrough_adapter.go` 同步改为调用同一 helper。新增 `TestOpenAIGatewayService_ShouldSimulateCodexCLIUsesAccountSwitchOnly` 锁定“全局 force 不绕过账号开关”。
- 提交与镜像：提交 `ed076e054 fix(openai): 收口 Codex CLI 模拟开关`；Git tag `v0.1.134.7` 指向该提交；从 `git archive HEAD` 构建 `sub2api:v0.1.134.7`，镜像 label `org.opencontainers.image.version=v0.1.134.7`、`revision=ed076e054050`；`/app/sub2api --version` 输出 `Sub2API 0.1.134 (image: v0.1.134.7, commit: ed076e054050, ...)`。
- 部署：部署前 active upstream 为 `sub2api-green:8080`，green/blue 均 healthy 且运行 `sub2api:v0.1.134.4`；本轮只更新 `D:\sub2api-deploy\docker-compose.blue.yml` 默认镜像并执行 `docker compose -f docker-compose.blue.yml up -d --no-deps --force-recreate sub2api-blue`，未重启 PostgreSQL、Redis 或 active green。
- 候选验证：等待 65 秒后，`sub2api-blue` 为 `ConfigImage=sub2api:v0.1.134.7` 且 healthy；`http://127.0.0.1:18083/health` 200，根路径 200，`GET /api/v1/admin/dashboard/stats` 未登录 401，`POST /responses` 未登录 401；blue 精确错误日志过滤 `panic|fatal|migration.*fail|checksum|pq:|bind:|address already in use|listen tcp|rebuild failed` 为空。
- 切流验证：将 `D:\sub2api-deploy\proxy\upstreams\active.conf` 从 `sub2api-green:8080` 改为 `sub2api-blue:8080`；`docker exec sub2api-proxy nginx -t` 与 reload 通过；切流后 `8080/health`、`18081/health`、`18083/health`、`18082/health` 均 200，公网根路径 200，admin 未登录 401，`POST /responses` 未登录 401；proxy/blue 错误日志过滤为空，green 继续作为回滚容器。
- free5 实测：DB 确认 `free5` 为 account_id `416`，OpenAI APIKey，base_url `https://new.sharedchat.cc/codex`；已设置 `openai_codex_cli_simulation_enabled=true` 并恢复 `schedulable=true`。第一次用现有 `自用` 分组请求命中其他同组账号 `426`，因此创建仅绑定 `free5` 的临时验证分组 `temp_group_id=8` 和临时 key `temp_api_key_id=7`；真实 `POST http://127.0.0.1:8080/responses` 返回 HTTP 200，响应 `output_text=OK`，blue 日志确认 `account_id=416`，未出现 `codex_access_restricted`。临时 key 和临时分组已标记 inactive/deleted。

## 2026-06-08 12:59 +08:00 - 删除全局 Force Codex CLI 开关

- 执行者：Devil
- 目标：账号级 `openai_codex_cli_simulation_enabled` 已替代全局模拟语义，删除旧的全局 `gateway.force_codex_cli` / `GATEWAY_FORCE_CODEX_CLI` 配置和放行逻辑，避免全局影响所有客户端或首 token 表现。
- 变更：删除 `backend/internal/config/config.go` 中的 `GatewayConfig.ForceCodexCLI` 字段和默认值；删除 `openai_gateway_handler.go` 紧凑日志里的 `force_codex_cli` 字段；删除 `openai_client_restriction_detector.go` 中 `CodexClientRestrictionReasonForceCodexCLI` 和全局配置兜底放行分支；更新相关测试，图片桥接测试改为通过账号级模拟开关触发 Codex 判定；删除 `deploy/.env.example` 与 `deploy/config.example.yaml` 中的全局配置示例；同时清理 `D:\sub2api-deploy\.env` 中的旧变量。
- 保留：账号级 `openai_codex_cli_simulation_enabled` 继续控制上游请求模拟；OAuth 账号 `codex_cli_only` 仍存在，但只按官方 UA/originator、账号级/全局 allowed clients 判定，不再被全局 force 配置绕过。
- 验证：残留扫描 `ForceCodexCLI|force_codex_cli|FORCE_CODEX_CLI|CodexClientRestrictionReasonForceCodexCLI` 为空；`go test ./internal/config -count=1` 通过；`go test ./internal/service -run "TestOpenAICodexClientRestrictionDetector|TestOpenAIGatewayService_GetCodexClientRestrictionDetector|TestOpenAIGatewayService_CodexCLIOnly|TestOpenAIGatewayService_APIKey(CodexCLISimulation|PassthroughCodexCLISimulation)|TestOpenAIGatewayService_Forward_CodexBridgeInjectionSetsImageBilling" -count=1 -v` 通过；`go test ./internal/handler -run TestDoesNotExist -count=1` 通过；`git diff --check` 仅提示既有 `deploy/.env.example` LF-to-CRLF 工作区提示。
- 说明：本轮未提交、未构建 Docker 镜像、未部署；当前线上 active 仍是上一轮 `sub2api-blue:v0.1.134.7`，需要后续按蓝绿流程发布后线上二进制才会彻底移除该配置读取。

## 2026-06-08 14:52 +08:00 - JUHE 统一错误处理规则和 Anthropic 1M 开关接线

- 执行者：Devil
- 目标：把 juhe feature 最新可借鉴的统一错误处理规则 schema、恢复策略和规则引导接到 sub2api 账号创建/编辑弹窗，同时补齐 Anthropic API Key 1M 上下文开关读写。
- 变更：新增 `AccountErrorHandlingCard`、`accountErrorHandling*` helper、统一 payload / validation / legacy compat 桥；Create/Edit 账号弹窗统一写 `error_handling_rules` 并同步兼容旧 `custom_error_codes` / `temp_unschedulable_rules`；Anthropic 1M 开关也跟随 `extra.anthropic_context_1m_enabled` 读写。
- 验证：`npm run typecheck` 通过；`npm run test:run -- src/components/account/__tests__/errorHandlingRules.spec.ts src/components/account/__tests__/EditAccountModal.spec.ts` 通过；`go test ./internal/service -run "TestAccountGetErrorHandlingRules_NormalizesUnifiedSchema|TestCheckErrorPolicy_UnifiedErrorHandlingRules|TestHandleUpstreamError_UnifiedErrorHandlingRules|TestResolveUnifiedRateLimitResetAt" -count=1` 通过。
- 说明：CodeGraph 连续 3 次 `Transport closed`，按项目规则已降级为 PowerShell 源码检索继续推进；本轮未部署。

## 2026-06-08 15:04 +08:00 - Anthropic API Key 1M 上下文按钮与 anyrouter 开启

- 执行者：Devil
- 目标：给 Anthropic API Key 账号增加“1M 上下文”按钮，并把现有 `anyrouter` 账号直接开启，止住 `context-1m` 未启用导致的上游 400。
- 变更：Create/Edit 账号弹窗新增 `anthropic_context_1m_enabled` 按钮态，按钮状态写入/读取 `extra.anthropic_context_1m_enabled`；后端在 Anthropic API Key 场景自动补 `context-1m-2025-08-07` beta，并在账号级开启时覆盖全局 beta 过滤；`anyrouter` 账号（id=444）已在数据库里改为开启。
- 验证：`go test ./internal/service -run 'TestComputeFinalAnthropicBeta_APIKey|TestComputeFinalCountTokensAnthropicBeta_APIKey|TestEffectiveAnthropicBetaDropSet_Context1MAccountOverride|TestMergeAnthropicBetaDropping_Context1M' -count=1` 通过；`npm run typecheck` 通过；`npm run test:run -- src/components/account/__tests__/EditAccountModal.spec.ts` 通过；浏览器本地首页加载正常，受保护 `/admin/accounts` 跳转到登录页且控制台无错误。
- 说明：本轮未提交、未构建、未部署；如果要让线上二进制生效，还需要按当前蓝绿流程再发候选镜像。

## 2026-06-08 15:26 +08:00 - Anthropic API key 透传剥离 cch_session_id

- 执行者：Devil
- 目标：在 Anthropic API key 透传链路里剥离 `cch_session_id`，避免 Claude Code 本地调度把内部会话字段原样送到 Anthropic 上游触发 400。
- 变更：在 `gateway_request.go` 新增 `sanitizeAnthropicAPIKeyPassthroughBody`，并把它接到 Anthropic `messages`、`count_tokens` 与通用构建链路；回归测试补入 `cch_session_id` 断言。
- 验证：`gofmt -w backend/internal/service/gateway_request.go backend/internal/service/gateway_service.go backend/internal/service/gateway_anthropic_apikey_passthrough_test.go`；`git diff --check`；`go test ./internal/service -run "TestGatewayService_AnthropicAPIKeyPassthrough_(ForwardStreamPreservesBodyAndAuthReplacement|ForwardCountTokensPreservesBody|ModelMappingEdgeCases)" -count=1`；`go test ./internal/service -run "TestGatewayService_AnthropicAPIKeyPassthrough_ForwardCountTokensPreservesBody" -count=1 -v`，均通过。
- 说明：仅补回这次相关的窄测，工作树里原本就存在的其他 Anthropic 透传失败用例未处理。

## 2026-06-08 16:03 +08:00 - 账号单个 API Key 状态恢复

- 执行者：Devil
- 目标：给管理员账号编辑里的单个已停用 API Key 增加按 fingerprint 恢复状态功能，只清除该 Key 的停用元数据，不改原始 Key 列表。
- 变更：`AdminService` 新增 `RestoreAccountAPIKeyState`，复用 `Account.RestoreAPIKeyByFingerprint` 和 `persistAccountCredentials`；新增 `POST /api/v1/admin/accounts/:id/api-keys/:fingerprint/restore-state`；前端账号编辑弹窗在 disabled key chip 上显示恢复按钮，调用 `restoreAccountAPIKeyState` 并返回更新后的账号快照。
- 验证：`go test -tags unit ./internal/service -run "TestAdminService_RestoreAccountAPIKeyState|TestAdminService_DeleteAccountAPIKey" -count=1` 通过；`go test ./internal/handler/admin -run "TestAccountHandlerRestoreAPIKeyStateByFingerprint" -count=1` 通过；`go test ./internal/server/routes -run TestDoesNotExist -count=1` 通过；`go test -tags unit ./internal/service -run "TestAccount_.*APIKey|TestAdminService_RestoreAccountAPIKeyState|TestAdminService_DeleteAccountAPIKey" -count=1` 通过；`npm run test:run -- src/api/__tests__/admin.accounts.spec.ts src/components/account/__tests__/EditAccountModal.spec.ts` 通过，36 tests；`npm run typecheck` 通过；`go test ./internal/server -run TestDoesNotExist -count=1` 通过；`git diff --check -- <本轮相关文件>` 通过。
- 说明：本轮未提交、未构建 Docker 镜像、未部署、未切流。

## 2026-06-08 16:18 +08:00 - 账号单个 API Key 状态恢复接管复核

- 执行者：Devil
- 目标：接续上一轮“单独 key 状态恢复”实现，复核实际代码路径和 fresh 验证结果，确认交付结论不只依赖上一轮摘要。
- 复核：CodeGraph 正常定位 `RestoreAccountAPIKeyState`、`RestoreAPIKeyState`、`RestoreAPIKeyByFingerprint`、`hasAPIKeyFingerprint` 和 `removeDisabledAPIKeyFingerprint`；确认恢复动作只删除 `api_keys_disabled[fingerprint]`，保留 `api_keys` / `api_key` 原始 Key，未知 fingerprint 返回 `ErrAccountAPIKeyNotFound`，未停用但仍存在的 fingerprint 不重复持久化。
- 范围校正：本功能相关文件包括 `backend/internal/service/account.go`、`backend/internal/service/account_api_keys_test.go`、`backend/internal/service/admin_service.go`、`backend/internal/service/admin_service_credentials_merge_test.go`、`backend/internal/handler/admin/account_handler.go`、`backend/internal/handler/admin/admin_service_stub_test.go`、`backend/internal/handler/admin/account_refresh_handler_test.go`、`backend/internal/server/routes/admin.go`、`frontend/src/api/admin/accounts.ts`、`frontend/src/api/__tests__/admin.accounts.spec.ts`、`frontend/src/components/account/EditAccountModal.vue`、`frontend/src/components/account/__tests__/EditAccountModal.spec.ts`、`frontend/src/i18n/locales/zh.ts`、`frontend/src/i18n/locales/en.ts`。
- Fresh 验证：`go test -tags unit ./internal/service -run "TestAccount.*APIKey|TestAdminService_RestoreAccountAPIKeyState|TestAdminService_DeleteAccountAPIKey" -count=1` 通过；`go test ./internal/handler/admin -run "TestAccountHandlerRestoreAPIKeyStateByFingerprint" -count=1` 通过；`npm run test:run -- src/api/__tests__/admin.accounts.spec.ts src/components/account/__tests__/EditAccountModal.spec.ts` 通过，2 files / 36 tests；`npm run typecheck` 通过；`go test ./internal/server/routes -run TestDoesNotExist -count=1` 通过；`go test ./internal/server -run TestDoesNotExist -count=1` 通过；`git diff --check -- <本轮相关文件>` 通过，仅提示 docs JSONL LF/CRLF 工作区警告。
- 说明：本轮未提交、未构建 Docker 镜像、未部署、未切流；工作树仍包含此前多项未提交改动，未做回退或整理。

## 2026-06-08 17:58 +08:00 - v0.1.134.8 构建与 green 切流

- 执行者：Devil
- 目标：把已提交的 `5d0d6b4736b9` 构建为不可变镜像并按蓝绿流程发布，保留 `sub2api-blue:v0.1.134.7` 作为回滚目标。
- 发布前状态：`active.conf` 原指向 `sub2api-blue:8080`；`sub2api-blue` 为 `sub2api:v0.1.134.7`、healthy、running；`sub2api-green` 为 `sub2api:v0.1.134.4`、healthy、running。
- 构建：目标镜像 `sub2api:v0.1.134.8` 不存在后开始构建；第一次构建失败在 Alpine `apk add` 下载 `zstd-libs` 时出现 `unexpected end of file`，未生成半成品镜像；第二次重试成功，镜像 label 为 `org.opencontainers.image.version=v0.1.134.8`、`org.opencontainers.image.revision=5d0d6b4736b9`，ImageID `sha256:6a9ebb7be521d41c22fd536b8262312e76ad215255b7694acc675dcd2f59a2c5`。前端生产构建通过，仅保留既有 Browserslist/chunk size/dynamic import 警告。
- 候选部署：仅重建 idle `sub2api-green`，未重建 active blue；等待 65 秒后 `sub2api-green` 为 `sub2api:v0.1.134.8`、healthy、running。候选端口 `18082/health` 200，根路径 200，静态资源 `/assets/index-D_mBlHt-.js` 200，`GET /api/v1/admin/dashboard/stats` 未登录 401，`POST /responses` 未登录 401，最近 5 分钟 green 日志关键错误过滤 `panic|fatal|migration.*fail|checksum|pq:|bind:|address already in use|listen tcp|rebuild failed` 为 0。
- 切流：将 `D:\sub2api-deploy\proxy\upstreams\active.conf` 从 `sub2api-blue:8080` 改为 `sub2api-green:8080`；`docker exec sub2api-proxy nginx -t` 通过；`docker exec sub2api-proxy nginx -s reload` 成功；同时把 `D:\sub2api-deploy\docker-compose.green.yml` 默认镜像更新为 `sub2api:v0.1.134.8`。
- 切流后验证：`8080/health`、`18081/health`、`18082/health`、`18083/health` 均 200；`8080` 根路径 200，静态资源 `/assets/index-D_mBlHt-.js` 200，`GET /api/v1/admin/dashboard/stats` 未登录 401，`POST /responses` 未登录 401；`sub2api-green` 为 `sub2api:v0.1.134.8` healthy，`sub2api-blue` 为 `sub2api:v0.1.134.7` healthy；proxy 和 green 最近 5 分钟关键错误过滤均为 0。
- 当前状态：active 已切到 green `sub2api:v0.1.134.8`；blue `sub2api:v0.1.134.7` 保留运行作为回滚目标。本轮未推送远端。

## 2026-06-08 18:26 +08:00 - 发版版本日志规则补入验证

- 执行者：Devil
- 观察：`docs/SUB2API_V0_1_134_ABSORPTION_LIST_CN.md` 是近期 v0.1.134 release note 吸收清单；`docs/process_list.jsonl` / `docs/feature_list.jsonl` 尾部记录了 `v0.1.134.8` green 发布流水。
- 变更：`AGENTS.md` 已新增发版版本日志要求，规定每次发版在 `docs/releases/<不可变版本号>.md` 写日期、执行者、Git 提交、镜像标签、active/idle、更新内容、验证结果、回滚目标和遗留风险，并明确 JSONL 过程流水不能替代版本日志。
- 验证：`git diff --check -- AGENTS.md` 通过，仅有 LF-to-CRLF 工作区换行提示。
- 说明：本轮只改文档规则，未构建、未部署、未运行 Go/前端测试。

## 2026-06-08 19:25 +08:00 - OpenAI Responses 调度耗尽无限探测

- 执行者：Devil
- 目标：修复 `openai_scheduler_exhaustion_probe_infinite_wait_enabled=true` 时，`/v1/responses` 在 failover 已排除账号后仍直接返回最后一次 429/502 的问题；只要调度池仍有探测候选账号，就进入原有无限探测，不再受 `failedAccountIDs` 非空阻断。
- 变更：`OpenAIGatewayHandler.Responses` 的选号失败分支改为通过 `openAISchedulerExhaustionProbeMode` 判断探测模式；初始选号失败保持原 finite/infinite 语义，failover 后仅在 infinite 开启时继续调用 `RecoverOpenAISchedulerExhaustion`。探测恢复成功后清空失败账号集合、同账号重试计数、切换计数和最后 failover 错误，再重新选号。
- 验证：新增决策红测先失败于 helper 缺失；实现后 handler 聚焦测试、OpenAI scheduler exhaustion service 测试和本轮 Go 文件 `git diff --check` 均通过。
- 已知无关失败：`go test ./internal/handler -count=1` 仍失败在 `TestGatewayEnsureForwardErrorResponse_DoesNotOverrideWrittenResponse` 与 `TestOpenAIResponsesWebSocket_ContinuityReplayForwardsSanitizedBodyToNextAccount`，单独复跑同样失败，本轮未处理。
- 说明：本轮未提交、未构建 Docker 镜像、未部署、未切流；线上 `sub2api:v0.1.134.8` 需要后续发版才会包含该修复。

## 2026-06-08 19:37 +08:00 - JUHE 可用时段与删除清理队列取舍收口

- 执行者：Devil
- 目标：继续完成 JUHE 可借鉴功能，把账号可用时段计划接入账号创建/编辑与服务端调度判断；同时复核 JUHE 删除账号关联数据清理队列是否适合吸收，并把镜像小版本页面展示规则补入项目手册。
- 变更：新增 `extra.availability_schedule` schema、后端 `Account.IsSchedulableAt` 和可用时段解析/判断；新增前端 `AccountAvailabilityScheduleEditor`、helper、类型与中英文文案，Create/Edit 账号弹窗会读写同一字段。`VersionBadge` 测试补充关闭态按钮必须显示 `image_version`，`AGENTS.md` 已要求发布或版本接口变更时页面展示不可变镜像小版本。
- JUHE 清理队列取舍：复核 `adminServiceImpl.DeleteAccount`、`accountRepository.Delete`、`backend/migrations/001_init.sql`、`066_add_scheduled_test_tables.sql`、`143_add_account_probe_runs.sql`、`145_add_account_batch_test_runs.sql` 后，决定不新增账号删除清理 target/worker。当前账号硬关联数据通过 SQL `ON DELETE CASCADE`、`scheduled_test_plans` 显式删除、Redis 调度快照清理和 scheduler outbox 覆盖；`ops_error_logs` 没有账号外键，按历史审计快照保留。
- 验证：`go test -tags unit ./internal/service -run "TestAccountIsSchedulableAt_AvailabilitySchedule|TestAccountIsSchedulable_QuotaExceeded|TestOpenAISchedulerExhaustionProbe" -count=1` 通过；`go test ./internal/handler -run "TestOpenAISchedulerExhaustionProbeMode|TestOpenAIFailoverRetryWindow" -count=1` 通过；`npm run test:run -- src/components/account/__tests__/AccountAvailabilitySchedule.spec.ts src/components/account/__tests__/EditAccountModal.spec.ts src/components/common/__tests__/VersionBadge.spec.ts` 通过，28 tests；`npm run typecheck` 通过；`git diff --check` 通过，仅有 `AGENTS.md`、`docs/feature_list.jsonl`、`docs/process_list.jsonl` 的 LF/CRLF 工作区提示。
- 说明：`docs/JUHE_AI_FEATURE_20250605_BORROWABLE_FEATURES_CN.md` 属于 `.gitignore` 的 `docs/*` 本地忽略范围，本轮已写入本地文档；可追溯结论同步写入本文件和 JSONL 过程记录。

## 2026-06-08 20:34 +08:00 - v0.1.134.9 国内源构建与 blue 切流

- 执行者：Devil
- 目标：回应 registry/apk 下载失败后的重试要求，把 Docker 多阶段构建改为国内 Alpine `apk` 源，并从已提交 HEAD 构建、部署和验证 `sub2api:v0.1.134.9`。
- 国内源变更：`Dockerfile` 新增 `ALPINE_APK_REPOSITORY=https://mirrors.tuna.tsinghua.edu.cn/alpine`，backend-builder 与 runtime stage 在 `apk add` 前替换 `/etc/apk/repositories`；`AGENTS.md` 已写入 registry/base image pull 与 `apk` I/O/403/超时失败后的处理规则。该规则已提交为 `81ca6995b`。
- 发布前状态：`active.conf` 指向 `sub2api-green:8080`；`sub2api-green` 为 `sub2api:v0.1.134.8`、healthy、running；`sub2api-blue` 为 `sub2api:v0.1.134.7`、healthy、running；目标镜像 `sub2api:v0.1.134.9` 不存在。
- 构建：从提交 `81ca6995b74b` 执行 `git archive --format=tar HEAD | docker build --pull=false --build-arg VERSION=0.1.134 --build-arg IMAGE_VERSION=v0.1.134.9 --build-arg COMMIT=81ca6995b74b -t sub2api:v0.1.134.9 ... -` 成功。构建日志显示 backend-builder 与 runtime 的 `apk add` 均使用 `https://mirrors.tuna.tsinghua.edu.cn/alpine`；镜像 label 为 `org.opencontainers.image.version=v0.1.134.9`、`org.opencontainers.image.revision=81ca6995b74b`，ImageID `sha256:627cd6d42904ff3c150d0e382148ad8035180accc99435f837e40e89cef9652a`。首次构建保护脚本因 PowerShell native command 退出码处理误判而提前停止，未生成镜像；修正检查方式后构建通过。
- 候选部署：仅设置 `SUB2API_BLUE_IMAGE=sub2api:v0.1.134.9` 并执行 `docker compose -f docker-compose.blue.yml up -d --no-deps --force-recreate sub2api-blue`，未触碰 active green、PostgreSQL 或 Redis。等待 65 秒后 `sub2api-blue` 为 `sub2api:v0.1.134.9`、healthy、running。
- 候选验证：`18083/health` 200，根路径 200，静态资源 `/assets/index-DKn8VwAg.js` 200，`GET /api/v1/admin/accounts?page=1&page_size=1` 未登录 401，`POST /responses` 未登录 401；最近 10 分钟 blue 日志关键错误过滤 `panic|fatal|migration.*fail|checksum|pq:|bind:|address already in use|listen tcp|rebuild failed` 为 0。第一次候选验证脚本已打印全部通过状态，但因 PowerShell 空数组判断写法误抛；修正后复跑通过。
- 切流：将 `D:\sub2api-deploy\proxy\upstreams\active.conf` 从 `sub2api-green:8080` 改为 `sub2api-blue:8080`；`docker exec sub2api-proxy nginx -t` 通过；`docker exec sub2api-proxy nginx -s reload` 成功；`D:\sub2api-deploy\docker-compose.blue.yml` 默认镜像已同步为 `sub2api:v0.1.134.9`，`docker compose -f docker-compose.blue.yml config --services` 通过。
- 切流后验证：`8080/health`、`18081/health`、`18083/health`、`18082/health` 均 200；`8080` 根路径 200，静态资源 200，未登录 admin API 401，`POST /responses` 未登录 401；`sub2api-blue` 为 `sub2api:v0.1.134.9` healthy，`sub2api-green` 为 `sub2api:v0.1.134.8` healthy，`sub2api-proxy` running；blue 日志关键错误过滤为 0，proxy 最近日志关键错误过滤为 0。
- 当前状态：active 已切到 blue `sub2api:v0.1.134.9`；green `sub2api:v0.1.134.8` 保留运行作为回滚目标。本轮未推送远端 registry。
## 2026-06-09 00:12 +08:00 - JUHE 最高优先级借鉴收口

- 执行者：Devil
- 目标：把 JUHE / v0.1.135 的 API Key 独占分组强制校验、OpenAI sticky session 分组校验和 previous_response_id 跨组剥离从待做同步为已完成，并补聚焦验证。
- 验证：`go test ./internal/server/middleware -run ''^TestApiKeyAuthWithSubscriptionGoogleRejectsExclusiveGroupWithoutUserGrant$'' -count=1 -v` 通过。
- 验证：`go test ./internal/service -run ''^TestOpenAIGatewayService_SelectAccountWithScheduler_(EnabledUsesAdvancedPreviousResponseRouting|PreviousResponseSkipsAccountOutsideGroup|SessionStickySkipsAccountOutsideGroup)$|^TestOpenAIWSStateStore_ResponseAccountLocalCacheIsGroupScoped$'' -count=1 -v` 通过。
- 说明：这轮主要是文档状态同步和聚焦验证，未提交、未构建镜像、未部署。

## 2026-06-09 08:52 +08:00 - free5 Codex 最新客户端模拟与人工测试请求流修复

- 执行者：Devil
- 根因：账号级 `openai_codex_cli_simulation_enabled=true` 已生效，但上游 403 文案要求最新版 Codex 客户端；当前模拟常量仍是 `codex_cli_rs/0.125.0` / `version=0.125.0`。同时 `AccountTestService.testOpenAIAccountConnection` 普通 Responses 人工测试绕过正式 `OpenAIGatewayService.buildUpstreamRequestWithBaseURL`，只手写 `Content-Type` / `Authorization`，导致人工测试与真实网关热路径不一致。
- RED：`go test -tags unit ./internal/service -run "TestOpenAICodexCLISimulationUsesLatestClientVersion|TestAccountTestService_OpenAIAPIKeyResponsesTestUsesGatewayCodexSimulationHeaders" -count=1 -v` 先失败，分别证明版本仍为 `0.125.0`、人工测试上游 `User-Agent` 为空。
- 变更：把 Codex CLI 模拟和默认 OpenAI Codex UA 统一到 npm 当前 `@openai/codex` 最新 `0.138.0`；新增 `buildOpenAITestResponsesRequest`，让 OpenAI Responses/compact 人工测试复用正式网关 builder，并在人工测试入口补齐入站 Codex 客户端身份。
- GREEN：上述 RED 命令通过；`go test -tags unit ./internal/service -run "TestAccountTestService_OpenAI|TestAccountTestService_TestAccountConnection_OpenAICompact|TestOpenAIGatewayService_APIKeyCodexCLISimulation|TestAccount_IsOpenAICodexCLISimulationEnabled" -count=1` 通过；`go test ./cmd/server ./internal/handler -run TestNoSuchTest -count=1` 通过；`git diff --check` 退出码 0，仅有 docs JSONL LF/CRLF 工作区警告。
- 已知无关失败：`go test -tags unit ./internal/service -count=1` 仍失败在既有 `TestOpenAINonStreamingConfiguredResponseTextReturnsFailover`、OpenAI image bridge 403 fallback、OAuth client-cancel、OpenAI passthrough failover stub panic 等路径，本轮未修改这些失败链路。

## 2026-06-09 09:57 +08:00 - 账号页当前降级策略展示

- 执行者：Devil
- 目标：让管理员在账号页面一眼看到当前账号处于哪一种降级策略，而不是只看到普通状态或派生健康标签。
- RED：`npm run test:run -- src/components/account/__tests__/AccountStatusIndicator.spec.ts` 先失败，失败点为账号状态组件未直接展示 `admin.accounts.status.degradationStrategy`。
- 变更：`AccountStatusIndicator` 新增降级策略 badge，覆盖轻微异常、中度异常、线路降级、临时不可调度、429 冷却、余额不足、余额耗尽、停用、待复测；保留原 tooltip 展示原因和恢复时间。
- GREEN：`npm run test:run -- src/components/account/__tests__/AccountStatusIndicator.spec.ts` 通过，7/7 tests passed。
- GREEN：`npm run typecheck` 通过。
- 说明：本轮不部署线上、不构建镜像、不切流，只提交本地代码。

## 2026-06-09 17:42 +08:00 - 调度池分组下拉与 Anthropic 查询

- 执行者：Devil
- 根因：上一版调度池页面默认不传分组，后端复杂模式下按未分组 OpenAI 池查询；当前真实可调度账号绑定在“自用”分组，所以页面为空。调度池接口也没有暴露 `platform`，只能看 OpenAI。
- RED：`go test -tags unit ./internal/service -run "TestOpenAIGatewayService_ListOpenAIAccountSchedulingPool" -count=1` 先因 `OpenAIAccountSchedulingPoolFilter.Platform` / snapshot `Platform` 缺失编译失败；`go test -tags unit ./internal/handler/admin -run "TestAccountHandlerListSchedulingPool" -count=1` 同样因平台字段缺失失败；`npm run test:run -- src/views/admin/__tests__/AccountSchedulingPoolView.spec.ts` 先失败于未加载分组和缺少协议下拉。
- 变更：调度池过滤和响应新增 `platform`，默认 OpenAI，支持 `openai` / `anthropic`；Anthropic 查询复用 scheduler snapshot 的分组调度口径，包含已启用 mixed scheduling 的 antigravity 账号；前端把手填分组 ID 改为分组下拉，默认选中名为“自用”的 active 分组，并新增协议下拉，Anthropic 模式隐藏 OpenAI endpoint/transport/image 专属筛选。
- GREEN：`go test -tags unit ./internal/handler/admin ./internal/service -run "(TestAccountHandlerListSchedulingPool|TestOpenAIGatewayService_ListOpenAIAccountSchedulingPool)" -count=1` 通过。
- GREEN：`go test ./cmd/server -run TestNoSuchTest -count=1` 通过。
- GREEN：`npm run test:run -- src/views/admin/__tests__/AccountSchedulingPoolView.spec.ts` 通过，2/2 tests passed。
- GREEN：`npm run typecheck` 通过。
- GREEN：`git diff --check` 通过。
- 浏览器冒烟：本地 `http://127.0.0.1:5173/admin/account-scheduling-pool` 返回 200；Playwright 打开后按未登录规则跳转登录页，console 0 errors。
- 说明：本轮只改调度池管理端和接口，不构建镜像、不部署、不切流。

## 2026-06-09 18:13 +08:00 - v0.1.134.17 蓝绿构建、部署、验证

- 执行者：Devil
- 构建：从已提交 `ed18ce9ab114` 执行 `git archive --format=tar HEAD | docker build --pull=false -t sub2api:v0.1.134.17 --label org.opencontainers.image.version=v0.1.134.17 --label org.opencontainers.image.revision=ed18ce9ab114 --build-arg COMMIT=ed18ce9ab114 --build-arg VERSION=v0.1.134 --build-arg IMAGE_VERSION=v0.1.134.17 -` 成功；镜像 ID `sha256:57b37199d0e341194b7f17f59e425094c362e693d6ce800b47f68b01b20ac73c`。
- 候选部署：发布前 active 为 `sub2api-green/sub2api:v0.1.134.16`，只重建 idle `sub2api-blue` 到 `sub2api:v0.1.134.17`，未重启 PostgreSQL/Redis。
- 候选验证：`18083` health/root/admin/settings 均 200；未登录 admin accounts 与 `/responses` 均 401；管理端 check-updates 返回 `image_version=v0.1.134.17`；`groups/all` 找到“自用” `id=2`；调度池 `platform=openai&group=2` 返回 `total=7`，`platform=anthropic&group=2` 返回 `total=0`；blue 60 秒 healthy 稳定；关键错误日志命中 0。
- 切流：`active.conf` 从 `sub2api-green:8080` 切到 `sub2api-blue:8080`，`docker exec sub2api-proxy nginx -t` 与 `docker exec sub2api-proxy nginx -s reload` 成功。
- 切流后验证：`8080` health/root/admin/settings 均 200；未登录 admin accounts 与 `/responses` 均 401；check-updates 返回 `image_version=v0.1.134.17`；`groups/all` 与 OpenAI/Anthropic 调度池分组查询同候选验证通过；blue 应用日志关键错误命中 0，proxy 最近 10 分钟错误日志命中 0。
- 当前状态：active 已切到 `sub2api-blue/sub2api:v0.1.134.17`；回滚容器 `sub2api-green/sub2api:v0.1.134.16` 保持 running/healthy。

## 2026-06-09 19:32 +08:00 - 调度池可用性雷达异常展示

- 执行者：Devil
- 根因：账号列表的“不稳定/待探测”等异常来自 `load_factor_advice.availability_radar` 与账号探测写入的 `derived_health`；调度池页面只展示 `pool_status`、`path_health` 与 `pool_reasons`，没有渲染同源雷达 badge，也没有给 `light_abnormal`、`moderate_abnormal`、`temp_unschedulable`、`quota_low`、`quota_exhausted` 等派生健康状态上色。
- RED：`npm run test:run -- src/views/admin/__tests__/AccountSchedulingPoolView.spec.ts` 先失败，新增用例里的池内账号 `unstable-pool` 能显示“轻微异常”，但调度池看不到“不稳定”和雷达原因。
- 变更：`AccountSchedulingPoolView` 健康列复用 `AccountAvailabilityRadarBadge`；原因列合并 `pool_reasons`、`derived_health.reason`、`derived_health.last_failure_reason`、`availability_radar.reasons` 与 `load_factor_advice.reasons`；行底色和健康标签补齐探测异常、临时不可调度、额度异常、冷却与雷达异常状态。
- GREEN：`npm run test:run -- src/views/admin/__tests__/AccountSchedulingPoolView.spec.ts` 通过，3/3 tests passed。
- GREEN：`npm run typecheck` 通过。
- GREEN：`git diff --check` 通过。

## 2026-06-09 19:44 +08:00 - v0.1.134.18 蓝绿构建、部署、验证

- 执行者：Devil
- 构建：从已提交 `72372cab5040` 执行 `git archive --format=tar HEAD | docker build --pull=false -t sub2api:v0.1.134.18 --label org.opencontainers.image.version=v0.1.134.18 --label org.opencontainers.image.revision=72372cab5040 --build-arg COMMIT=72372cab5040 --build-arg VERSION=v0.1.134 --build-arg IMAGE_VERSION=v0.1.134.18 -` 通过，镜像 ID `sha256:d9056cc3f0cb3acf7190b8a1220980b7df81ca29d2288ea1ae77019fb47c1c22`。
- 候选部署：发布前 active 为 `sub2api-blue/sub2api:v0.1.134.17`，只重建 idle `sub2api-green` 到 `sub2api:v0.1.134.18`，未重启 PostgreSQL/Redis。
- 候选验证：`18082` health/root/admin/static 均 200；未登录 admin accounts 与 `/responses` 均 401；管理鉴权的 `system/version` 与 `check-updates` 返回 `image_version=v0.1.134.18`；`groups/all` 找到“自用”；调度池 `platform=openai&group=2` 返回 `total=6` 且 6 条带雷达异常状态 `needs_probe/normal`；`platform=anthropic&group=2` 返回 `total=0`；green 60 秒 healthy 稳定；关键错误日志命中 0。
- 切流：`active.conf` 从 `sub2api-blue:8080` 切到 `sub2api-green:8080`，`docker exec sub2api-proxy nginx -t` 与 `docker exec sub2api-proxy nginx -s reload` 成功；`docker-compose.green.yml` 默认镜像同步为 `sub2api:v0.1.134.18`。
- 切流后验证：`8080` health/root/admin/static 均 200；未登录 admin accounts 与 `/responses` 均 401；管理鉴权的 `system/version` 与 `check-updates` 返回 `image_version=v0.1.134.18`；OpenAI/Anthropic 调度池分组查询同候选验证通过；green 应用日志关键错误命中 0，proxy 最近 15 分钟关键错误日志命中 0。
- 当前状态：active 已切到 `sub2api-green/sub2api:v0.1.134.18`；回滚容器 `sub2api-blue/sub2api:v0.1.134.17` 保持 running/healthy。

## 2026-06-09 20:21 +08:00 - 调度池待探测样本口径修复

- 执行者：Devil
- 根因：`待探测` 由 `AccountLoadFactorAdvisor` 根据 `OpenAIPathHealthRecord.Samples < MinSamples` 计算；此前真实 OpenAI 成功调用只写 usage log，没有稳定调用账号级 `RecordSuccess`，所以真实请求成功后调度池仍可能一直显示 `needs_probe/待探测`。
- 修复：`OpenAIGatewayService.RecordUsage` 在普通模式和 simple 模式的成功用量记录出口调用 `recordOpenAIAccountSuccessfulCall`，使用当前账号对象写入账号级 `OpenAIPathHealthTracker.RecordSuccess`，并同步调度器运行时成功统计。
- 验证：`go test -tags unit ./internal/service -run "TestOpenAIGatewayServiceRecordUsage_FeedsPathHealthSample|TestOpenAIGatewayServiceRecordUsage_ZeroUsageStillWritesUsageLog" -count=1` 通过。
- 验证：`go test -tags unit ./internal/service -run "(TestOpenAIGatewayServiceRecordUsage_FeedsPathHealthSample|TestOpenAIGatewayService_OpenAIAccountSchedulerMetrics|TestOpenAIPathHealth|TestAccountLoadFactorAdvisor|TestOpenAIGatewayService_ListOpenAIAccountSchedulingPool)" -count=1` 通过。
- 验证：`go test -tags unit ./internal/handler/admin ./internal/service -run "(TestAccountHandlerListSchedulingPool|TestOpenAIGatewayServiceRecordUsage_FeedsPathHealthSample|TestAccountLoadFactorAdvisor)" -count=1` 通过。
- 验证：`git diff --check` 通过。

## 2026-06-09 21:43 +08:00 - 通用 Responses 调用写入调度池健康样本

- 执行者：Devil
- 根因：线上 `usage_logs` 已证明真实 `/responses -> /v1/responses` 调度在增长，但这条热路径由 `GatewayHandler.Responses -> GatewayService.RecordUsage` 记录用量，不走 `OpenAIGatewayService.RecordUsage`。上一轮只在 OpenAI 专用 RecordUsage 成功出口写入 `OpenAIPathHealthTracker`，所以通用 `/responses` 的真实成功调用不会增加调度池内存健康样本，调度池仍显示 `samples=0`。
- 修复：`GatewayService` 新增共享 `OpenAIPathHealthTracker` 注入点；`GatewayHandler` 构造时把 `OpenAIGatewayService.OpenAIPathHealthTracker()` 注入给通用网关；`GatewayService.recordUsageCore` 在普通计费和 simple mode 成功记录 usage log 后，对 OpenAI 账号写入账号级 `RecordSuccess`。
- 验证：`go test -tags unit ./internal/service -run "TestGatewayServiceRecordUsage_OpenAIResponsesFeedsPathHealthSample|TestOpenAIGatewayServiceRecordUsage_FeedsPathHealthSample|TestAccountLoadFactorAdvisor|TestOpenAIGatewayService_ListOpenAIAccountSchedulingPool" -count=1` 通过。
- 验证：`go test -tags unit ./internal/handler/admin ./internal/service -run "(TestAccountHandlerListSchedulingPool|TestGatewayServiceRecordUsage_OpenAIResponsesFeedsPathHealthSample|TestOpenAIGatewayServiceRecordUsage_FeedsPathHealthSample|TestAccountLoadFactorAdvisor)" -count=1` 通过。
- 验证：`go test ./cmd/server -run TestNoSuchTest -count=1` 通过。
- 验证：`git diff --check` 通过。
- 已知无关阻塞：`go test -tags unit ./internal/handler ./internal/service -run "(TestGatewayServiceRecordUsage_OpenAIResponsesFeedsPathHealthSample|TestAccountHandlerListSchedulingPool)" -count=1` 中 `internal/service` 通过，但 `internal/handler` 整包编译失败在既有 `userHandlerRepoStub` 缺少 `GetByIDIncludeDeleted`，本轮未修改该测试桩链路。

## 2026-06-09 22:16 +08:00 - 调度池 path-health 读取完整 OpenAI BaseURL

- 执行者：Devil
- 根因：v0.1.134.20 切流后 green 日志和 `usage_logs` 证明 `/responses` 成功调用已经进入 OpenAI 账号 `free6` / `dengxian-4`，但调度池 samples 仍为 0。进一步比对发现真实调用记录样本时使用完整账号 credentials 中的 `base_url` / `request_base_urls`，而调度池从 scheduler snapshot 读取的账号对象不一定带完整 OpenAI BaseURL，导致调度池按默认 `https://api.openai.com` key 读取，和真实样本写入的 `https://ai2.hhhl.cc/v1` / `https://api.denxio.top` key 不一致。
- 修复：调度池列表仍返回调度快照账号，但构建 OpenAI APIKey 账号的 path-health 时按账号 ID 读取完整账号，仅用于计算内部 health key 和能力判断，避免把样本读到错误 upstream。
- RED/GREEN：新增 `TestOpenAIGatewayService_ListOpenAIAccountSchedulingPoolReadsPathHealthWithFullAPIKeyBaseURL`，覆盖列表账号缺少 base_url、完整账号带自定义 upstream、样本写在完整 key 上时调度池必须读到 samples 的场景。
- 验证：`go test -tags unit ./internal/service -run "TestOpenAIGatewayService_ListOpenAIAccountSchedulingPoolReadsPathHealthWithFullAPIKeyBaseURL|TestOpenAIGatewayService_ListOpenAIAccountSchedulingPoolShowsHealthAndReasons|TestGatewayServiceRecordUsage_OpenAIResponsesFeedsPathHealthSample|TestOpenAIGatewayServiceRecordUsage_FeedsPathHealthSample|TestAccountLoadFactorAdvisor" -count=1` 通过。
- 验证：`go test -tags unit ./internal/handler/admin ./internal/service -run "(TestAccountHandlerListSchedulingPool|TestOpenAIGatewayService_ListOpenAIAccountSchedulingPoolReadsPathHealthWithFullAPIKeyBaseURL|TestOpenAIGatewayService_ListOpenAIAccountSchedulingPool|TestGatewayServiceRecordUsage_OpenAIResponsesFeedsPathHealthSample|TestOpenAIGatewayServiceRecordUsage_FeedsPathHealthSample|TestAccountLoadFactorAdvisor)" -count=1` 通过。
- 验证：`go test ./cmd/server -run TestNoSuchTest -count=1` 通过。
- 验证：`git diff --check` 通过。

## 2026-06-09 22:30 +08:00 - v0.1.134.20 / v0.1.134.21 蓝绿发布验证

- 执行者：Devil
- v0.1.134.20：从提交 `79b4b9480fcc` 构建 `sub2api:v0.1.134.20` 成功，部署到 idle green 并切流；基础冒烟通过，但切流后真实 OpenAI `/responses` 200 已进入 DB 和 green 日志时，调度池 samples 仍为 0。该版本判定为特性验证失败，已被 v0.1.134.21 替代。
- v0.1.134.21：从提交 `d017b05e786f` 构建 `sub2api:v0.1.134.21` 成功，部署到 idle blue；候选 `18083` health/root/admin/settings 200，静态资源 6/6 200，未登录 admin accounts 与 `/responses` 均 401，管理端版本返回 `image_version=v0.1.134.21`。
- 候选验证：OpenAI 调度池 group=2 返回 `total=3`、`schedulable_count=3`，health key upstream 为 `https://ai2.hhhl.cc/v1`、`https://api.denxio.top`、`https://api.denxio.top`，证明 key 错位已修复。
- 稳定窗口：blue 60 秒后仍 `Health=healthy`，blue 关键错误日志过滤命中 0。
- 切流：`active.conf` 从 `sub2api-green:8080` 切到 `sub2api-blue:8080`，`nginx -t` 与 reload 成功，`docker-compose.blue.yml` 默认镜像同步为 `sub2api:v0.1.134.21`。
- 切流后冒烟：`8080` health/root/admin/settings 200，静态资源 6/6 200，未登录 admin accounts 与 `/responses` 均 401，管理端版本返回 `image_version=v0.1.134.21`。
- 真实流量验证：`2026-06-09 22:26:48+08` 后 DB 中 OpenAI 账号 `free6` 有 5 次 usage log，5 次都有 `first_token_ms`；调度池最终显示 `free6 samples=14 success=14 ttft_ewma_ms=14900.55 upstream=https://ai2.hhhl.cc/v1`。
- 日志验证：切流后 `sub2api-blue` 与 `sub2api-proxy` 关键错误日志过滤命中 0。
- 当前状态：active=`sub2api-blue/sub2api:v0.1.134.21`；rollback=`sub2api-green/sub2api:v0.1.134.20`，green 保持 running/healthy；未重启 PostgreSQL 和 Redis。

## 2026-06-09 21:01 +08:00 - v0.1.134.19 蓝绿构建、部署、验证

- 执行者：Devil
- 构建：从已提交 `92d113d86e96` 构建不可变镜像 `sub2api:v0.1.134.19` 成功；镜像 ID `sha256:870e9eca1d4cc770155413da93b94a877757fae601fc9c0edfd6d8ae051b8c9f`；镜像标签确认 `org.opencontainers.image.version=v0.1.134.19`、`org.opencontainers.image.revision=92d113d86e96`。
- 候选部署：发布前 active 为 `sub2api-green/sub2api:v0.1.134.18`，只重建 idle `sub2api-blue` 到 `sub2api:v0.1.134.19`，未重启 PostgreSQL/Redis。
- 候选验证：`18083` health/root/admin/static/settings 均 200；未登录 admin accounts 与 `/responses` 均 401；管理鉴权的 `system/version` 返回 `image_version=v0.1.134.19`；`groups/all` 找到“自用”；OpenAI/Anthropic 调度池分组查询均 200；blue 运行健康且关键错误日志命中 0。
- 切流：`active.conf` 从 `sub2api-green:8080` 切到 `sub2api-blue:8080`，`docker exec sub2api-proxy nginx -t` 与 `docker exec sub2api-proxy nginx -s reload` 成功；`docker-compose.blue.yml` 默认镜像同步为 `sub2api:v0.1.134.19`。
- 切流后验证：`8080` health/root/admin/settings 均 200，管理前端静态资源 6/6 返回 200；未登录 admin accounts 与 `/responses` 均 401；管理鉴权的 `system/version` 返回 `image_version=v0.1.134.19`；`groups/all` 选中 `2:自用`；OpenAI 调度池 `total=6`、`schedulable=6`、`degraded=0`、`blocked=0`，Anthropic 调度池 `total=0`；blue 和 proxy 关键错误日志命中 0。
- 当前状态：active 已切到 `sub2api-blue/sub2api:v0.1.134.19`；回滚容器 `sub2api-green/sub2api:v0.1.134.18` 保持 running/healthy。

## 2026-06-10 12:28 +08:00 - Anthropic 单账号调度退避配置接线

- 执行者：Devil
- 变更：新增的 `anthropic_single_account_backoff_seconds` 已从 `GatewayConfig` 默认值/环境变量加载接入到 `GatewayHandler.Messages` 与 `GatewayHandler.ChatCompletions` 的 Anthropic failover 状态；默认仍为 2 秒，正数配置可覆盖，0 走默认值。
- 变更：`openai_scheduler_cooldown_multiplier` 与 `anthropic_scheduler_cooldown_multiplier` 补齐默认值和正数校验，避免配置为 0 或负数后进入运行时；本轮未把倍率强接到多条冷却路径，避免未经验证地改变既有冷却语义。
- 验证：`go test -tags unit ./internal/config -run "TestLoadDefaultSchedulerRetryConfig|TestLoadSchedulerRetryConfigFromEnv|TestValidateConfig_OpenAIWSRules" -count=1` 通过。
- 验证：`go test -tags unit ./internal/handler/failover_loop.go ./internal/handler/failover_loop_test.go -run "TestNewFailoverState|TestNewFailoverStateWithBackoff|TestHandleSelectionExhausted" -count=1` 通过。
- 验证：`go test ./cmd/server -run TestNoSuchTest -count=1` 通过。
- 验证：`git diff --check` 通过。
- 已知无关阻塞：`go test -tags unit ./internal/handler -run "TestNewFailoverState|TestNewFailoverStateWithBackoff|TestHandleSelectionExhausted" -count=1` 整包编译失败在既有 `userHandlerRepoStub` 缺少 `GetByIDIncludeDeleted`，本轮未修改该 auth/user handler 测试桩链路。

## 2026-06-10 12:43 +08:00 - v0.1.134.24 蓝绿构建、部署、验证

- 执行者：Devil
- 提交：`5b3f22d50b06`（`feat(gateway): 接通 Anthropic 单账号退避配置`）。
- 构建：从已提交 `HEAD` 通过 `git archive HEAD | docker build ...` 构建不可变镜像 `sub2api:v0.1.134.24`，镜像 ID `sha256:9a1177b4b4588cb3bd2ae746250d35338d908b2f6a90284595fb0ecf2e66b5f9`。
- 镜像标签验证：`docker image inspect sub2api:v0.1.134.24` 返回 `Version=v0.1.134.24`、`Revision=5b3f22d50b06`。
- 候选部署：发布前 active 为 `sub2api-green/sub2api:v0.1.134.20`；只重建 idle `sub2api-blue` 到 `sub2api:v0.1.134.24`，未重启 PostgreSQL/Redis。
- 候选验证：`18083` health/root/admin/settings 均 200，静态资源 6/6 返回 200，未登录 admin accounts 与 `/responses` 均 401，`sub2api-blue` 60 秒后仍 `Health=healthy`。
- 候选日志：启动窗口有 1 条 `[OpenAI] cleanup expired request snapshots failed err=pq: canceling statement due to user request`；切流前近 5 分钟复查关键错误过滤命中 0，判定为非重复启动清理观察项。
- 切流：`D:\sub2api-deploy\proxy\upstreams\active.conf` 从 `sub2api-green:8080` 切到 `sub2api-blue:8080`，`docker exec sub2api-proxy nginx -t` 与 `docker exec sub2api-proxy nginx -s reload` 成功。
- 切流后验证：`8080` health/root/admin/settings 均 200，静态资源 6/6 返回 200，未登录 admin accounts 与 `/responses` 均 401，`sub2api-blue` Health `healthy`，`sub2api-green` 继续 running/healthy 作为回滚。
- 日志验证：切流后 `sub2api-blue` 与 `sub2api-proxy` 关键错误过滤 `panic|fatal|migration.*fail|checksum|pq:|bind:|address already in use|listen tcp|rebuild failed` 命中 0。
- 当前状态：active=`sub2api-blue/sub2api:v0.1.134.24`；rollback=`sub2api-green/sub2api:v0.1.134.20`。
- 已知限制：`D:\sub2api-deploy\.env` 的 `ADMIN_PASSWORD` 为空，无法登录管理端验证 `/api/v1/admin/system/version`；版本由 Docker label 和容器内 `/app/sub2api --version` 验证。

## 2026-06-10 16:01 +08:00 - Claude Code 上游格式错误 failover 与 v0.1.134.25 发布

- 执行者：Devil
- 根因：Claude Code 经 `/v1/messages` 进入 OpenAI 兼容转发时，部分远端返回 HTTP 400，正文为 `There was an issue with the format or content of your request` 并携带多个 `request id`。旧逻辑把它当普通 upstream 400 写给客户端，客户端直接失败，无法继续尝试其他远端。
- 修复：`isOpenAITransientProcessingError` 增加窄口径匹配，只有 HTTP 400、固定格式或内容错误文案、且包含 `request id` 时才归入临时处理错误；既有 `shouldFailoverOpenAIUpstreamResponse` 会在写客户端前返回 `UpstreamFailoverError`，让外层调度继续 failover。
- 验证：`go test -tags unit ./internal/service -run "TestIsOpenAITransientProcessingError|TestOpenAIGatewayService_ForwardAsAnthropic_FormatContentIssueTriggersFailover" -count=1` 通过。
- 验证：`go test -tags unit ./internal/service -run "TestOpenAIGatewayService_Forward_(TransientProcessingErrorTriggersFailover|ModelCapacityErrorTriggersFailoverAndSameAccountRetry|LogsInstructionsRequiredDetails)|TestForwardAsAnthropic_(NormalizesRoutingAndEffortForGpt54XHigh|ReplaysWithoutContinuationWhenPreviousResponseMissing|DisablesAPIKeyContinuationWhenUpstreamRequiresWebSocketV2)|TestOpenAIUpstreamErrorPolicySeparatesPhasesAndActions" -count=1` 通过。
- 验证：`go test ./cmd/server -run TestNoSuchTest -count=1` 通过。
- 验证：`git diff --check -- backend/internal/service/openai_gateway_service.go backend/internal/service/openai_gateway_service_codex_cli_only_test.go` 通过。
- 构建：从提交 `20bfc6a74c88` 构建 `sub2api:v0.1.134.25`，镜像 ID `sha256:628d79048a9c2fc4ccf075a134e0b01ed2caf8aa3cdb2c8910de914e2ee0ff54`，label `version=v0.1.134.25`、`revision=20bfc6a74c88`。
- 候选验证：idle `sub2api-green` 候选端口 `18082` health/root/admin/settings 200，静态资源 6/6 200，未登录 admin accounts 与 `/responses` 401，60 秒 Health `healthy`。
- 切流验证：`active.conf` 从 `sub2api-blue:8080` 切到 `sub2api-green:8080`，`nginx -t` 与 reload 成功；入口 `8080` health/root/admin/settings 200，未登录 admin accounts 与 `/responses` 401。
- 现时复核：`active.conf` 指向 `sub2api-green:8080`；`sub2api-green` 为 `sub2api:v0.1.134.25` 且 Health `healthy`；`sub2api-blue/sub2api:v0.1.134.24` 保持 running/healthy 作为回滚。
- 日志观察：近 2 小时 `sub2api-green` 有 1 条 request snapshot 清理 `pq: canceling statement due to user request`；`sub2api-proxy` 关键错误过滤命中 0。该观察项与本轮 Claude Code failover 改动无关，继续留意即可。
- 已知无关阻塞：`go test -tags unit ./internal/service -run "^TestForwardAsAnthropic_DoneSentinelWithoutTerminalReturnsError$" -count=1` 失败在既有 done-sentinel 期望与 failover 错误文本差异，本轮未修改该链路。

## 2026-06-10 17:20 +08:00 - Anthropic 原生 API Key 格式错误 failover 与 v0.1.134.26 发布

- 执行者：Devil
- 根因：现场日志确认请求 `202606100732187175600488268d9d6syj42KHv` 来自 `api_key_id=2`、`group_id=1`，选中 Anthropic 账号 `445(君公益)`，`platform=anthropic type=apikey`。旧逻辑在普通 Anthropic API Key `GatewayService.Forward` 中把该 400 当作 non-retryable 写给客户端，`fallback_error_response_written=true`。
- 账号确认：DB 查询账号 `445` 为 active/schedulable，所属组 `1`，`extra.anthropic_passthrough` 为空，说明真实路径不是 OpenAI 兼容，也不是 API Key 透传分支。
- 修复：新增 Anthropic 固定格式/内容 400 窄匹配，要求 HTTP 400、消息包含 `There was an issue with the format or content of your request` 且带 `request id`；命中后在写客户端前返回 `UpstreamFailoverError`，handler 继续切换账号。
- 验证：`go test -tags unit ./internal/service -run "TestGatewayService_AnthropicAPIKey(Passthrough)?_FormatContentIssueTriggersFailover|TestGatewayService_AnthropicAPIKeyPassthrough_(Ordinary400KeepsDefaultError|ForwardStreamPreservesBodyAndAuthReplacement|ForwardCountTokensPreservesBody)|TestGatewayHandleErrorResponse_(NoRuleKeepsDefault|AppliesRuleFor422)|TestOpenAIHandleErrorResponse_NoRuleKeepsDefault" -count=1` 通过。
- 验证：`go test ./cmd/server -run TestNoSuchTest -count=1` 通过。
- 验证：`git diff --check -- backend/internal/service/gateway_service.go backend/internal/service/gateway_anthropic_apikey_passthrough_test.go` 通过。
- 构建：从提交 `d3932faa5653` 通过 `git archive HEAD | docker build ...` 构建 `sub2api:v0.1.134.26`；镜像 ID `sha256:fc330df3bacc0f54fe930e47859ab14f83001c0a3d25b9f96dc1db9fcd2bed28`，label `version=v0.1.134.26`、`revision=d3932faa5653`。
- 候选部署：发布前 active 为 `sub2api-green/sub2api:v0.1.134.25`；只重建 idle `sub2api-blue` 到 `sub2api:v0.1.134.26`，未重启 PostgreSQL/Redis。
- 候选验证：`18083` health/root/admin/settings 均 200，静态资源 6/6 返回 200，未登录 admin accounts 与 `/responses` 均 401，`sub2api-blue` 60 秒后仍 `Health=healthy`。
- 切流：`active.conf` 从 `sub2api-green:8080` 切到 `sub2api-blue:8080`，`nginx -t` 与 reload 成功。
- 切流后验证：`8080` health/root/admin/settings 均 200，静态资源 6/6 返回 200，未登录 admin accounts、`/responses`、`/v1/messages` 均 401；`sub2api-blue` Health `healthy`，`sub2api-green` 继续 running/healthy 作为回滚。
- 日志验证：切流后 5 分钟内 `sub2api-blue` 与 `sub2api-proxy` 关键错误过滤命中 0。
- 当前状态：active=`sub2api-blue/sub2api:v0.1.134.26`；rollback=`sub2api-green/sub2api:v0.1.134.25`。
- 已知观察：候选启动窗口有 1 条 request snapshot 清理 `pq: canceling statement due to user request`，最近 2 分钟与切流后窗口无重复；继续作为既有观察项留意。

## 2026-06-10 20:40 +08:00 - v0.1.134.27 蓝绿构建、部署、验证

- 执行者：Devil
- 提交：`3efcbe745a5b`（`refactor(gateway): IsAccountBlocked 走调度快照并复用统一可调度判断`），包含上一线上版本后的 `a788ab5eb023` 与 `3efcbe745a5b`。
- 变更：503/429 failover 错误统一写入临时熔断；单账号 selection exhausted 会检查账号阻断状态避免空转；`IsAccountBlocked` 改走调度快照并复用 `IsSchedulableAt`。
- 代码验证：`go test -tags unit ./internal/service -run "TestAccountIsSchedulableAt|TestGatewayService.*Blocked|Test.*Selection|Test.*TempUnschedule" -count=1` 通过。
- 代码验证：`go test ./cmd/server -run TestNoSuchTest -count=1` 通过。
- 代码验证：`git diff --check` 无 whitespace 错误，仅 JSONL LF/CRLF 提示。
- 构建：从已提交 `HEAD` 通过 `git archive HEAD | docker build ...` 构建不可变镜像 `sub2api:v0.1.134.27`，镜像 ID `sha256:d6bf487cb2c46e6d4f773b46d9cdce22b13dbed166564585a0846797be307f91`。
- 镜像标签验证：`docker image inspect sub2api:v0.1.134.27` 返回 `Version=v0.1.134.27`、`Revision=3efcbe745a5b`。
- 候选部署：发布前 active 为 `sub2api-blue/sub2api:v0.1.134.26`；只重建 idle `sub2api-green` 到 `sub2api:v0.1.134.27`，未重启 PostgreSQL/Redis。
- 候选验证：`18082` health/root/admin/settings 均 200，静态资源 6/6 返回 200，未登录 admin accounts 与 `/responses` 均 401，`sub2api-green` Health `healthy`。
- 候选日志：启动窗口有 1 条 `[OpenAI] cleanup expired request snapshots failed err=pq: canceling statement due to user request`；切流前最近 1 分钟复查关键错误过滤命中 0，判定为非重复启动清理观察项。
- 切流：`D:\sub2api-deploy\proxy\upstreams\active.conf` 从 `sub2api-blue:8080` 切到 `sub2api-green:8080`，`docker exec sub2api-proxy nginx -t` 与 `docker exec sub2api-proxy nginx -s reload` 成功。
- 切流后验证：`8080` health/root/admin/settings 均 200，静态资源 6/6 返回 200，未登录 admin accounts 与 `/responses` 均 401，`sub2api-green` Health `healthy`，`sub2api-blue` 继续 running/healthy 作为回滚。
- 日志验证：切流后 `sub2api-green` 与 `sub2api-proxy` 关键错误过滤命中 0。
- 当前状态：active=`sub2api-green/sub2api:v0.1.134.27`；rollback=`sub2api-blue/sub2api:v0.1.134.26`。
- 已知限制：`D:\sub2api-deploy\.env` 的 `ADMIN_PASSWORD` 为空，无法登录管理端验证 `/api/v1/admin/system/version`；版本由 Docker label 和容器内 `/app/sub2api --version` 验证。

## 2026-06-11 16:26 +08:00 - v0.1.134.28 蓝绿构建、部署、验证

- 执行者：Devil
- 提交：`fc0c8cdd0da8`（`test(account): 固定 API Key 禁用恢复测试时间`），包含 `e8b2b676e..fc0c8cdd0da8` 范围内 v0.1.136 网关吸收、账号池用量、Anthropic 余额配置、调度探测、API Key 分级恢复和测试时间源修复。
- 前馈：Obsidian Local REST 本轮 2 秒超时；继续以项目源码、git diff、CodeGraph 和 live runtime 为准。
- 代码验证：`go test -tags unit ./internal/service -run "TestAccountGetAPIKey|TestAccountDisableAPIKey|TestAccountGetAPIKeys|TestAccountRemoveAPIKey|TestAccountRestoreAPIKey|TestBuildAccountUsageSummary|TestUpstreamBalanceService|TestOpenAISchedulerExhaustionProbe|TestRecoverAccountAfterManualProbe|TestIdempotency|TestOpenAIPathHealth" -count=1` 通过。
- 代码验证：`go test -tags unit ./internal/handler/admin -run "TestAccountHandler(GetUsageSummary|DashboardSummary|ActionItems|RefreshUpstreamBalance|ManualProbe)" -count=1` 通过。
- 代码验证：`go test ./cmd/server -run TestNoSuchTest -count=1` 通过。
- 前端验证：账号 API、编辑弹窗、账号池用量卡片、调度池页面和 model whitelist 共 5 个文件 54 项 Vitest 通过。
- 前端验证：`AccountsView.bulkEdit.spec.ts` 的 `passes total account cost sorting to the server and displays usage totals` 单用例通过。
- 前端验证：`npm run typecheck` 通过。
- 构建：从已提交 `HEAD` 通过 `git archive HEAD | docker build ...` 构建不可变镜像 `sub2api:v0.1.134.28`，镜像 ID `sha256:11bd45565c2688a2d4d6d04883b7e6565a315564484563b8b42b97f298e94204`。
- 镜像标签验证：`docker image inspect sub2api:v0.1.134.28` 返回 `Version=v0.1.134.28`、`Revision=fc0c8cdd0da8`。
- 候选部署：发布前 active 为 `sub2api-green/sub2api:v0.1.134.27`；只重建 idle `sub2api-blue` 到 `sub2api:v0.1.134.28`，未重启 PostgreSQL/Redis。
- 候选验证：`18083` health/root/admin/settings 均 200，静态资源 6/6 返回 200，未登录 admin accounts、`/responses`、`/v1/messages` 均 401，`sub2api-blue` Health `healthy`。
- 候选日志：启动窗口有 1 条 `[OpenAI] cleanup expired request snapshots failed err=pq: canceling statement due to user request`；切流前最近 1 分钟复查关键错误过滤命中 0，判定为非重复启动清理观察项。
- 切流：`D:\sub2api-deploy\proxy\upstreams\active.conf` 从 `sub2api-green:8080` 切到 `sub2api-blue:8080`，`docker exec sub2api-proxy nginx -t` 与 `docker exec sub2api-proxy nginx -s reload` 成功。
- 切流后验证：`8080` health/root/admin/settings 均 200，静态资源 6/6 返回 200，未登录 admin accounts、`/responses`、`/v1/messages` 均 401，`sub2api-blue` Health `healthy`，`sub2api-green` 继续 running/healthy 作为回滚。
- 日志验证：切流后 `sub2api-blue` 与 `sub2api-proxy` 关键错误过滤命中 0。
- 当前状态：active=`sub2api-blue/sub2api:v0.1.134.28`；rollback=`sub2api-green/sub2api:v0.1.134.27`。
- 已知限制：`D:\sub2api-deploy\.env` 的 `ADMIN_PASSWORD` 为空，无法登录管理端验证 `/api/v1/admin/system/version`；版本由 Docker label 和容器内 `/app/sub2api --version` 验证。
- 已知非阻断：`go test -tags unit ./internal/service ./internal/handler/admin -count=1` 宽包运行中 `internal/handler/admin` 通过，但 `internal/service` 命中 OpenAI 旧链路测试期望差异和 test stub nil panic；本轮按账号、调度、管理端和构建/上线冒烟作为发布挡板，后续单独整理宽包测试。

## 2026-06-11 17:58 +08:00 - v0.1.134.29 蓝绿构建、部署、验证

- 执行者：Devil
- 提交：`2f35bad37ae5`（`test(handler): 同步用户仓储测试桩接口`），包含 `fc0c8cdd0da8..2f35bad37ae5` 范围内调度耗尽探测 SSE pending/keepalive、分级退避心跳频率和 handler 测试桩接口同步。
- 前馈：Obsidian Local REST 本轮 2 秒超时；继续以项目源码、git diff、CodeGraph 和 live runtime 为准。
- 代码验证：`go test -tags unit ./internal/service -run "TestOpenAISchedulerExhaustionProbe|TestProbeIntervalFromErrorCount" -count=1` 通过。
- 代码验证：`go test -tags unit ./internal/handler -run TestNoSuchTest -count=1` 通过。
- 代码验证：`go test ./cmd/server -run TestNoSuchTest -count=1` 通过。
- 代码验证：`git diff --check` 通过。
- 构建：从已提交 `HEAD` 通过 `git archive HEAD | docker build ...` 构建不可变镜像 `sub2api:v0.1.134.29`，镜像 ID `sha256:d0a7868406b341256ca70168dce5bb93722264ca2e6f173b98db97c708cb7dc3`。
- 镜像标签验证：`docker image inspect sub2api:v0.1.134.29` 返回 `Version=v0.1.134.29`、`Revision=2f35bad37ae5`。
- 候选部署：发布前 active 为 `sub2api-blue/sub2api:v0.1.134.28`；只重建 idle `sub2api-green` 到 `sub2api:v0.1.134.29`，未重启 PostgreSQL/Redis。
- 候选验证：`sub2api-green` Health `healthy` 持续超过 60 秒；`18082` health/root/admin/settings 均 200，静态资源 6/6 返回 200，未登录 admin accounts、`/responses`、`/v1/messages` 均 401。
- 候选日志：启动窗口有 1 条 `[OpenAI] cleanup expired request snapshots failed err=pq: canceling statement due to user request`；切流前最近 1 分钟复查关键错误过滤命中 0，判定为非重复启动清理观察项。
- 切流：`D:\sub2api-deploy\proxy\upstreams\active.conf` 从 `sub2api-blue:8080` 切到 `sub2api-green:8080`，`docker exec sub2api-proxy nginx -t` 与 `docker exec sub2api-proxy nginx -s reload` 成功。
- 切流后验证：`8080` health/root/admin/settings 均 200，静态资源 6/6 返回 200，未登录 admin accounts、`/responses`、`/v1/messages` 均 401，`sub2api-green` Health `healthy`，`sub2api-blue` 继续 running/healthy 作为回滚。
- 日志验证：切流后 `sub2api-green` 与 `sub2api-proxy` 关键错误过滤命中 0。
- 当前状态：active=`sub2api-green/sub2api:v0.1.134.29`；rollback=`sub2api-blue/sub2api:v0.1.134.28`。
- 已知限制：`D:\sub2api-deploy\.env` 的 `ADMIN_PASSWORD` 为空，无法登录管理端验证 `/api/v1/admin/system/version`；版本由 Docker label 和容器内 `/app/sub2api --version` 验证。
- 已知观察：request snapshot 清理任务在候选启动窗口出现 1 条 `pq: canceling statement due to user request`，切流前最近 1 分钟与切流后窗口无重复；继续作为既有观察项留意。

## 2026-06-11 22:45 +08:00 - v0.1.134.30 蓝绿构建、部署、验证

- 执行者：Devil
- 提交：`e94e0b8778ab`（`feat(handler): failover 耗尽回退探测保活循环，502/503/429 单账号不终止`），包含 `2f35bad37ae5..e94e0b8778ab` 范围内 failover 耗尽回退探测保活循环和 `sleepWithProbeKeepalive` helper。
- 前馈：Obsidian Local REST 本轮 2 秒超时；继续以项目源码、git diff、CodeGraph 和 live runtime 为准。
- 代码验证：`go test -tags unit ./internal/service -run "TestOpenAISchedulerExhaustionProbe|TestProbeIntervalFromErrorCount|TestOpenAIGatewayServiceStreamFailover|TestOpenAIGatewayService_Forward_ModelCapacityErrorTriggersFailover" -count=1` 通过。
- 代码验证：`go test -tags unit ./internal/handler -run TestNoSuchTest -count=1` 通过。
- 代码验证：`go test ./cmd/server -run TestNoSuchTest -count=1` 通过。
- 代码验证：`git diff --check` 通过。
- 构建：从已提交 `HEAD` 通过 `git archive HEAD | docker build ...` 构建不可变镜像 `sub2api:v0.1.134.30`，镜像 ID `sha256:169a9357b377bfc83bfcaafd52a56f6c3c6ef14be035a6267a82a2d9a624d40a`。
- 镜像标签验证：`docker image inspect sub2api:v0.1.134.30` 返回 `Version=v0.1.134.30`、`Revision=e94e0b8778ab`。
- 候选部署：发布前 active 为 `sub2api-green/sub2api:v0.1.134.29`；只重建 idle `sub2api-blue` 到 `sub2api:v0.1.134.30`，未重启 PostgreSQL/Redis。
- 候选验证：`sub2api-blue` Health `healthy` 持续超过 60 秒；`18083` health/root/admin/settings 均 200，静态资源 6/6 返回 200，未登录 admin accounts、`/responses`、`/v1/messages` 均 401。
- 候选日志：启动窗口有 1 条 `[OpenAI] cleanup expired request snapshots failed err=pq: canceling statement due to user request`；切流前最近 1 分钟复查关键错误过滤命中 0，判定为非重复启动清理观察项。
- 切流：`D:\sub2api-deploy\proxy\upstreams\active.conf` 从 `sub2api-green:8080` 切到 `sub2api-blue:8080`，`docker exec sub2api-proxy nginx -t` 与 `docker exec sub2api-proxy nginx -s reload` 成功。
- 切流后验证：`8080` health/root/admin/settings 均 200，静态资源 6/6 返回 200，未登录 admin accounts、`/responses`、`/v1/messages` 均 401，`sub2api-blue` Health `healthy`，`sub2api-green` 继续 running/healthy 作为回滚。
- 日志验证：切流后 `sub2api-blue` 与 `sub2api-proxy` 关键错误过滤命中 0。
- 当前状态：active=`sub2api-blue/sub2api:v0.1.134.30`；rollback=`sub2api-green/sub2api:v0.1.134.29`。
- 已知限制：`D:\sub2api-deploy\.env` 的 `ADMIN_PASSWORD` 为空，无法登录管理端验证 `/api/v1/admin/system/version`；版本由 Docker label 和容器内 `/app/sub2api --version` 验证。
- 已知观察：request snapshot 清理任务在候选启动窗口出现 1 条 `pq: canceling statement due to user request`，切流前最近 1 分钟与切流后窗口无重复；继续作为既有观察项留意。

## 2026-06-11 14:50 +08:00 - 账号池用量与 Anthropic 上游余额配置

- 执行者：Devil
- 变更：账号页“ChatGPT 账号池用量”改为“账号池用量”，后端账号池汇总不再读取或计算 5 小时/7 天额度窗口，不再返回按 ChatGPT plan 展示的数据；账号数改为当前筛选下的全系统账号数。
- 变更：账号页移除账号状态汇总卡片与对应首屏 dashboard/status summary 请求；账号池用量卡片分离展示 OpenAI 额度/真实额度与 Anthropic 额度/真实额度。
- 变更：Anthropic APIKey 账号纳入上游余额刷新，默认余额 base URL 使用 `https://api.anthropic.com`；编辑弹窗为 Anthropic APIKey 开放上游账号、密码、倍率、倍率分组、余额端点和手动余额字段，保存使用与 OpenAI 一致的 `upstream_manual_*` 字段。
- 验证：`go test -tags unit ./internal/service -run "TestBuildAccountUsageSummary|TestUpstreamBalanceService" -count=1` 通过。
- 验证：`go test -tags unit ./internal/handler/admin -run "TestAccountHandler(GetUsageSummary|DashboardSummary|ActionItems|RefreshUpstreamBalance)" -count=1` 通过。
- 验证：`npm run test:run -- src/components/admin/account/__tests__/AccountUsageSummaryPanel.spec.ts src/components/account/__tests__/EditAccountModal.spec.ts src/api/__tests__/admin.accounts.spec.ts src/views/admin/__tests__/AccountsView.bulkEdit.spec.ts` 通过，48 tests passed。
- 验证：`npm run typecheck` 通过；`go test ./cmd/server -run TestNoSuchTest -count=1` 通过；`git diff --check` 通过。
- 已知观察：Vitest 输出仍有既有 `common.time.never` i18n 缺失警告和 Browserslist 数据提示，本轮未修改该无关链路。

## 2026-06-11 22:57 +08:00 - OpenAI 调度池筛选与人工探测

- 执行者：Devil
- 变更范围：`frontend/src/views/admin/AccountSchedulingPoolView.vue`、`frontend/src/views/admin/__tests__/AccountSchedulingPoolView.spec.ts`
- 验证命令：
  - `npm run test:run -- src/views/admin/__tests__/AccountSchedulingPoolView.spec.ts`
  - `npm run typecheck`
  - `git diff --check -- frontend/src/views/admin/AccountSchedulingPoolView.vue frontend/src/views/admin/__tests__/AccountSchedulingPoolView.spec.ts`
- 结果：全部通过。
- 风险：本轮未构建镜像、未部署；变更停留在前端源码和测试层。

## 2026-06-11 23:36 +08:00 - v0.1.134.31 蓝绿构建、部署、验证

- 执行者：Devil
- 提交：`9205bbe40afb`（`feat(admin): 调整调度池筛选与人工探测模型`），包含 `e94e0b8778ab..9205bbe40afb` 范围内调度池筛选简化、OpenAI 人工探测入口和人工探测默认模型选择规则。
- 前馈：Obsidian Local REST 本轮 2 秒超时；继续以项目源码、git diff、CodeGraph 和 live runtime 为准。
- 代码验证：`npm run test:run -- src/views/admin/__tests__/AccountSchedulingPoolView.spec.ts` 通过，1 file / 6 tests passed。
- 代码验证：`npm run typecheck` 通过。
- 代码验证：`docs/feature_list.jsonl` 与 `docs/process_list.jsonl` 逐行 `ConvertFrom-Json` 解析通过。
- 代码验证：`git diff --check` 通过。
- 构建：从已提交 `HEAD` 通过 `git archive HEAD | docker build ...` 构建不可变镜像 `sub2api:v0.1.134.31`，镜像 ID `sha256:54d910b5676df0557e20cadf2898bc6cc2c862478196f31fe6abaf4e9dcdd2ce`。
- 镜像标签验证：`docker image inspect sub2api:v0.1.134.31` 返回 `Version=v0.1.134.31`、`Revision=9205bbe40afb`。
- 候选部署：发布前 active 为 `sub2api-blue/sub2api:v0.1.134.30`；只重建 idle `sub2api-green` 到 `sub2api:v0.1.134.31`，未重启 PostgreSQL/Redis。
- 候选验证：`sub2api-green` Health `healthy` 持续超过 60 秒；`18082` health/root/admin/settings 均 200，静态资源 6/6 返回 200，未登录 admin accounts、`/responses`、`/v1/messages` 均 401。
- 候选日志：启动窗口有 1 条 `[OpenAI] cleanup expired request snapshots failed err=pq: canceling statement due to user request`；切流前最近 1 分钟复查关键错误过滤命中 0，判定为非重复启动清理观察项。
- 切流：`D:\sub2api-deploy\proxy\upstreams\active.conf` 从 `sub2api-blue:8080` 切到 `sub2api-green:8080`，`docker exec sub2api-proxy nginx -t` 与 `docker exec sub2api-proxy nginx -s reload` 成功。
- 切流后验证：`8080` health/root/admin/settings 均 200，静态资源 6/6 返回 200，未登录 admin accounts、`/responses`、`/v1/messages` 均 401，`sub2api-green` Health `healthy`，`sub2api-blue` 继续 running/healthy 作为回滚。
- 日志验证：切流后 `sub2api-green` 与 `sub2api-proxy` 关键错误过滤命中 0。
- 当前状态：active=`sub2api-green/sub2api:v0.1.134.31`；rollback=`sub2api-blue/sub2api:v0.1.134.30`。
- 已知限制：`D:\sub2api-deploy\.env` 的 `ADMIN_PASSWORD` 为空，无法登录管理端验证 `/api/v1/admin/system/version`；版本由 Docker label 和容器内 `/app/sub2api --version` 验证。
- 已知观察：request snapshot 清理任务在候选启动窗口出现 1 条 `pq: canceling statement due to user request`，切流前最近 1 分钟与切流后窗口无重复；继续作为既有观察项留意。

## 2026-06-11 23:12 +08:00 - 调度池人工探测模型选择纠偏

- 执行者：Devil
- 变更范围：`frontend/src/views/admin/AccountSchedulingPoolView.vue`、`frontend/src/views/admin/__tests__/AccountSchedulingPoolView.spec.ts`
- 根因：账号测试弹窗先通过 `getAvailableModels` 选择默认模型，Anthropic 优先选择 `sonnet`，调度池先前固定传入 `claude-opus-4-8` 与账号页不一致。
- 验证命令：
  - `npm run test:run -- src/views/admin/__tests__/AccountSchedulingPoolView.spec.ts`
  - `npm run typecheck`
- 结果：全部通过。
- 风险：本轮未构建镜像、未部署；变更停留在前端源码和测试层。

## 2026-06-11 23:56 +08:00 - v0.1.134.32 调度池人工探测失败提示修复发布

- 执行者：Devil
- 变更范围：`frontend/src/api/admin/accounts.ts`、`frontend/src/views/admin/AccountSchedulingPoolView.vue`、`frontend/src/views/admin/__tests__/AccountSchedulingPoolView.spec.ts`
- 根因：后端 `ManualProbeResponse.result` 是 `omitempty`，线上失败响应可能缺少 `result`；前端失败分支直接读取 `result.message`，触发 `Cannot read properties of undefined (reading 'message')`。
- 验证命令：
  - RED：`npm run test:run -- src/views/admin/__tests__/AccountSchedulingPoolView.spec.ts -t "omits result"` 修复前复现 TypeError。
  - GREEN：`npm run test:run -- src/views/admin/__tests__/AccountSchedulingPoolView.spec.ts -t "omits result"` 修复后通过。
  - GREEN：`npm run test:run -- src/views/admin/__tests__/AccountSchedulingPoolView.spec.ts` 7 项通过。
  - GREEN：`npm run typecheck` 通过。
  - GREEN：`git diff --check` 通过。
- 构建与发布：
  - 镜像：`sub2api:v0.1.134.32`
  - 提交：`29ae82de1755`
  - active：`sub2api-blue` / `sub2api:v0.1.134.32`
  - rollback：`sub2api-green` / `sub2api:v0.1.134.31`
- 候选和入口验证：`18083` 与 `8080` 的 health/root/admin/settings、6 个静态资源、未登录 admin accounts 401、`/responses` 401、`/v1/messages` 401 均通过；切流后 `sub2api-blue` 与 `sub2api-proxy` 关键错误日志命中 0。
- 风险：`D:\sub2api-deploy\.env` 中 `ADMIN_PASSWORD` 为空，无法执行登录态点击复测；已通过回归测试覆盖缺失 `result` 的返回体，并用入口静态资源验证新前端已上线。

## 2026-06-12 08:21 +08:00 - 账号页 Anthropic 默认测试模型调整

- 执行者：Devil
- 变更范围：`frontend/src/components/admin/account/AccountTestModal.vue`、`frontend/src/components/account/AccountTestModal.vue`、`frontend/src/views/admin/AccountSchedulingPoolView.vue` 及对应 Vitest。
- 根因：账号测试弹窗和调度池人工探测仍沿用 Anthropic 优先 `sonnet` 的默认模型规则；用户要求账号页面 Anthropic 默认测试模型改为 `claude-opus-4-8`。
- RED：`npm run test:run -- src/components/admin/account/__tests__/AccountTestModal.spec.ts src/components/account/__tests__/AccountTestModal.spec.ts src/views/admin/__tests__/AccountSchedulingPoolView.spec.ts` 修复前失败 3 项，均显示实际选择/提交 `claude-sonnet-4-5`。
- GREEN：同一 Vitest 命令修复后通过，3 个测试文件 19 项测试通过。
- GREEN：`npm run typecheck` 通过，`vue-tsc --noEmit` 无类型错误。
- GREEN：`git diff --check` 通过；仅提示既有 `.codegraph/daemon.pid` LF/CRLF 工作树告警，本轮前端 diff 无空白错误。
- 风险：本轮未构建镜像、未部署；变更停留在前端源码和测试层。

## 2026-06-12 08:35 +08:00 - v0.1.134.33 人工探测结果契约发布

- 执行者：Devil
- 根因：后端 `ManualProbeResponse.result` 直接返回 `AccountTestConnectionResult`，Go JSON 默认输出 `Success`、`LatencyMs`、`ErrorMessage`；调度池前端读取 `result.success`、`result.latency_ms`、`result.message`，成功结果会被当作失败。
- 变更：新增 `ManualProbeResultResponse` DTO，把结果转换为 `success`、`message`、`error`、`latency_ms`、`first_token_ms`、`http_status`、`reason`。
- RED：`go test -tags unit ./internal/handler/admin -run "TestManualProbeResultResponseFromService" -count=1` 修复前失败，缺少 DTO 转换函数。
- GREEN：`go test -tags unit ./internal/handler/admin -run "TestAccountHandler_ManualProbe|TestManualProbeResultResponseFromService" -count=1` 通过。
- GREEN：`npm run test:run -- src/views/admin/__tests__/AccountSchedulingPoolView.spec.ts` 7 项通过。
- GREEN：`npm run typecheck` 通过。
- GREEN：`git diff --check` 通过，仅提示既有 `.codegraph/daemon.pid` 换行告警。
- 构建：从提交 `529d33b3b371` 构建 `sub2api:v0.1.134.33`，镜像 ID `sha256:9710887ced132acd59636b6abc1f0c0247206fe1dc9ccfca92479302df1e3366`，`/app/sub2api --version` 显示 `image: v0.1.134.33`。
- 候选验证：idle `sub2api-green` / `18082` health/root/6 个静态资源 200；未登录 system version/admin accounts/responses/v1 messages 均 401；Health healthy 持续超过 60 秒。
- 切流验证：代理从 blue 切到 green，`nginx -t` 与 reload 成功；入口 `8080` health/root/6 个静态资源 200，未登录边界均 401；切流后 green/proxy 关键错误日志命中 0。
- 状态：该版本随后被 `sub2api:v0.1.134.34` supersede，保留为 green 回滚目标。

## 2026-06-12 08:35 +08:00 - v0.1.134.34 最终发布验证

- 执行者：Devil
- 变更范围：`v0.1.134.33` 人工探测结果契约修复 + Anthropic 默认测试模型调整。
- GREEN：`npm run test:run -- src/components/admin/account/__tests__/AccountTestModal.spec.ts src/components/account/__tests__/AccountTestModal.spec.ts src/views/admin/__tests__/AccountSchedulingPoolView.spec.ts` 通过，3 个测试文件 19 项测试通过。
- GREEN：`go test -tags unit ./internal/handler/admin -run "TestAccountHandler_ManualProbe|TestManualProbeResultResponseFromService" -count=1` 通过。
- GREEN：`npm run typecheck` 通过。
- GREEN：`git diff --check` 通过，仅提示既有 `.codegraph/daemon.pid` 换行告警。
- 构建：从提交 `c7b059f169c5` 构建 `sub2api:v0.1.134.34`，镜像 ID `sha256:3b3bd45128d53726781d729ddf9e45e763b96d39ad6e9d78d58a8a341a0504e7`，`/app/sub2api --version` 显示 `image: v0.1.134.34`。
- 候选验证：idle `sub2api-blue` / `18083` health/root/6 个静态资源 200；未登录 system version/admin accounts/responses/v1 messages 均 401；Health healthy 持续超过 60 秒。
- 候选日志：启动窗口有 1 条 request snapshot 清理 `pq: canceling statement due to user request`，后续 45 秒关键错误过滤无重复。
- 切流验证：代理从 green 切到 blue，`nginx -t` 与 reload 成功；入口 `8080` health/root/6 个静态资源 200，未登录边界均 401；切流后 blue/proxy 关键错误日志命中 0。
- 当前状态：active=`sub2api-blue/sub2api:v0.1.134.34`；rollback=`sub2api-green/sub2api:v0.1.134.33`。
- 限制：`D:\sub2api-deploy\.env` 中 `ADMIN_PASSWORD` 为空，无法执行登录态点击复测。

## 2026-06-12 12:21 +08:00 - v0.1.134.35 用户反馈复核

- 执行者：Devil
- 反馈范围：调度池人工探测 HTTP 200 仍显示探测失败；真实余额换算异常。
- 当前线上状态：`D:\sub2api-deploy\proxy\upstreams\active.conf` 指向 `sub2api-green:8080`；`sub2api-green` 使用 `sub2api:v0.1.134.35`，Health `healthy`；`sub2api-blue` 使用 `sub2api:v0.1.134.34`，Health `healthy`，作为回滚目标。
- GREEN：`docker exec sub2api-green /app/sub2api --version` 输出 `Sub2API 0.1.134 (image: v0.1.134.35, commit: 59b93fa4585f, built: 2026-06-12T03:51:52Z)`。
- GREEN：`go test ./internal/handler/admin -run 'TestManualProbe|TestAccountSchedulingPool' -count=1` 通过，验证人工探测 JSON 响应契约和调度池 handler 切片。
- GREEN：干净 HEAD 归档中运行 `go test -tags unit ./internal/service -run '^(TestAccountTestService_OpenAIResponseTextErrorInterceptsProbe|TestParseUpstreamBalanceResponse_NewAPIQuotaUnits|TestParseUpstreamBalanceResponse_NewAPIQuotaUnitsNestedUser)$' -count=1` 通过，验证 OpenAI 200 错误文本识别和 NewAPI quota 单位换算。
- GREEN：`npm run typecheck` 通过。
- GREEN：`npm run test:run -- AccountSchedulingPoolView` 通过，1 个测试文件 7 项测试通过；`npm run test -- AccountSchedulingPoolView` 为 Vitest watch 模式，124 秒超时，不代表用例失败。
- GREEN：`GET http://127.0.0.1:8080/health` 返回 200；`GET http://127.0.0.1:18082/health` 返回 200；候选根页面返回 200 且加载静态资源 `index-BhTF4Mvu.js`。
- GREEN：最近 10 分钟 `sub2api-green` 关键错误过滤 `panic|fatal|migration.*fail|checksum|pq:|bind:|address already in use|listen tcp|rebuild failed` 命中 0。
- 已知验证阻塞：当前工作树里已有未提交测试改动导致直接运行 `go test ./internal/service ...` 失败，失败点是 `account_test_service_anthropic_test.go` 引用只在 `//go:build unit` 文件中定义的 `anthropicHTTPUpstreamRecorder`，以及 `account_base_url_test.go` 引用不存在的 `GetAnthropicRequestBaseURLs`；因此本轮用 `git archive HEAD` 的干净归档验证已提交修复。
- 限制：当前 Playwright 会话未登录管理端，访问 `/admin/accounts/scheduling-pool` 被重定向到 `/login`，且部署 `.env` 的 `ADMIN_PASSWORD` 为空，未执行真实点击复测。

## 2026-06-13 10:53 +08:00 - P0-2 OpenAI 账号探测响应正文关键词拦截回归覆盖

- 执行者：Devil
- 变更范围：`backend/internal/service/account_test_service_openai_test.go`
- 现状确认：`backend/internal/service/account_test_service.go` 已在 OpenAI responses 与 chat-completions 探测流中传入 `newOpenAIResponseTextErrorDetector(account)`，本轮未改生产代码。
- GREEN：`go test -tags unit ./internal/service -run "TestAccountTestService_TestAccountConnectionWithResultAppliesOpenAIResponseTextErrorKeywords" -count=1` 通过。
- GREEN：`go test -tags unit ./internal/service -run "TestAccountTest.*ResponseText|TestAccountTestOutcome|TestAccountTestService_TestAccountConnectionWithResultAppliesOpenAIResponseTextErrorKeywords" -count=1` 通过。
- GREEN：`git diff --check -- backend/internal/service/account_test_service_openai_test.go docs/feature_list.jsonl docs/process_list.jsonl` 通过；仅有 docs JSONL 工作树 CRLF 提示。
- 风险：新增测试首次运行即通过，说明生产代码已满足计划 P0-2；本轮提交只补完整路径回归覆盖和记录，未构建镜像、未部署、未推送。

## 2026-06-13 11:05 +08:00 - OpenAI Responses failover 重试指数退避

- 执行者：Devil
- 变更范围：`backend/internal/handler/openai_gateway_handler.go`、`backend/internal/handler/openai_gateway_handler_test.go`
- RED：`go test -tags unit ./internal/handler -run "TestOpenAIFailoverRetryWindow" -count=1` 修复前失败，缺少 `openAIFailoverRetryMaxDelay` 与增长逻辑。
- GREEN：`go test -tags unit ./internal/handler -run "TestOpenAIFailoverRetryWindow|TestHandleSelectionExhausted" -count=1` 通过。
- GREEN：`go test ./cmd/server -run TestNoSuchTest -count=1` 通过。
- GREEN：`git diff --check -- backend/internal/handler/openai_gateway_handler.go backend/internal/handler/openai_gateway_handler_test.go` 通过。
- 风险：本轮仅改变 `/responses` failover retry window 的等待时序；30 秒总窗口、候选排除与探测状态机不变。未构建镜像、未部署、未推送。

## 2026-06-13 11:13 +08:00 - 单账号选号耗尽指数退避

- 执行者：Devil
- 变更范围：`backend/internal/config/config.go`、`backend/internal/handler/failover_loop.go`、`backend/internal/handler/failover_loop_test.go`
- RED：`go test -tags unit ./internal/handler -run "TestSingleAccountExhaustionBackoffDelay" -count=1` 修复前失败，缺少退避计算 helper 和上限常量。
- GREEN：`go test -tags unit ./internal/config -run "TestLoadDefaultSchedulerRetryConfig|TestLoadSchedulerRetryConfigFromEnv|TestValidateConfig_OpenAIWSRules" -count=1` 通过。
- GREEN：`go test -tags unit ./internal/handler -run "TestNewFailoverState|TestSingleAccountExhaustionBackoffDelay|TestHandleSelectionExhausted|TestOpenAIFailoverRetryWindow" -count=1` 通过。
- GREEN：`go test ./cmd/server -run TestNoSuchTest -count=1` 通过。
- GREEN：`git diff --check -- backend/internal/config/config.go backend/internal/handler/failover_loop.go backend/internal/handler/failover_loop_test.go` 通过。
- 风险：本轮只改变单请求内选号耗尽等待节奏，不新增跨请求冷却；未构建镜像、未部署、未推送。

## 2026-06-13 11:30 +08:00 - OpenAI passthrough 流上游静默超时

- 执行者：Devil
- 变更范围：`backend/internal/service/openai_gateway_service.go`、`backend/internal/service/openai_gateway_service_test.go`
- RED：`go test -tags unit ./internal/service -run "TestOpenAIStreamingPassthroughTimeout" -count=1` 修复前失败，两个 passthrough 静默流用例均在 1.5 秒内未返回。
- GREEN：`go test -tags unit ./internal/service -run "TestOpenAIStreamingPassthroughTimeout" -count=1` 通过。
- GREEN：`go test -tags unit ./internal/service -run "TestOpenAIStreamingPassthrough(ResponseFailed|QuotaFailed|ResponseDone|ResponseIncomplete|MissingTerminal|Timeout)|TestOpenAIStreamingTimeout|TestOpenAIStreamingWaitGuardTimeoutBeforeOutputReturnsFailover" -count=1` 通过。
- GREEN：`go test -tags unit ./internal/service -run "TestOpenAIStreaming(Passthrough|Timeout|WaitGuard|TooLong|ContextCanceled|RecordsPathHealth)|TestOpenAIPathHealth|TestOpenAIResponseText" -count=1` 通过。
- GREEN：`go test ./cmd/server -run TestNoSuchTest -count=1` 通过。
- GREEN：`git diff --check -- backend/internal/service/openai_gateway_service.go backend/internal/service/openai_gateway_service_test.go` 通过。
- 风险：本轮只补 passthrough 流式路径的上游静默超时；普通 `/responses` 流式路径已有独立 `intervalCh` 逻辑。未构建镜像、未部署、未推送。

## 2026-06-13 11:42 +08:00 - FastLane TTFT path-health 调度验收覆盖

- 执行者：Devil
- 变更范围：`backend/internal/service/openai_account_scheduler_test.go`
- CodeGraph：`codegraph_context`、`codegraph_status`、`codegraph_search` 连续 `Transport closed`；检查 `.codegraph/daemon.log` 发现 `write EPIPE` 与 daemon 重建记录后，按项目规则降级到 PowerShell 源码检索。
- 说明：生产代码已具备 `OpenAIPathHealthTracker.RecordSuccess` 的 TTFT EWMA、`ScoreBoost` 和 FastLane `pathBoost` 接入；本轮只补可验收回归测试。
- GREEN：`go test -tags unit ./internal/service -run "TestBuildOpenAIAccountLoadPlanFastLanePathHealthTTFTBoostPrefersLowerTTFT" -count=1` 通过。
- GREEN：`go test -tags unit ./internal/service -run "TestBuildOpenAIAccountLoadPlan(ProfilesScoreSpeedVsStability|FastLanePathHealthTTFTBoostPrefersLowerTTFT|HalfOpenOnlyForProbe|SkipsOpenBucketAcrossAccounts)|TestOpenAIPathHealthScoreBoostUsesTTFT" -count=1` 通过。
- GREEN：`go test ./cmd/server -run TestNoSuchTest -count=1` 通过。
- GREEN：`git diff --check -- backend/internal/service/openai_account_scheduler_test.go` 通过。
- 风险：`selectionOrder` 保留既有加权随机机制，测试只锁定确定性的 `pathBoost` 和 `score` 契约。未构建镜像、未部署、未推送。

## 2026-06-13 11:52 +08:00 - prompt-cache 内容推导亲和 opt-in

- 执行者：Devil
- 变更范围：`backend/internal/config/config.go`、`backend/internal/service/openai_gateway_service.go`、`backend/internal/service/openai_gateway_service_test.go`
- RED：`go test -tags unit ./internal/service -run "TestOpenAIGatewayService_GenerateSessionHash_(ContentFallbackRequiresPromptCacheAffinityOptIn|ExplicitSignalWinsOverContentFallbackOptIn)" -count=1` 修复前编译失败，缺少 `PromptCacheAffinityContentFallbackEnabled` 配置字段。
- GREEN：`go test -tags unit ./internal/service -run "TestOpenAIGatewayService_GenerateSessionHash" -count=1` 通过。
- GREEN：`go test -tags unit ./internal/config -run "TestLoadDefaultSchedulerRetryConfig|TestLoadSchedulerRetryConfigFromEnv|TestValidateConfig_OpenAIWSRules" -count=1` 通过。
- GREEN：`go test ./cmd/server -run TestNoSuchTest -count=1` 通过。
- GREEN：`git diff --check -- backend/internal/config/config.go backend/internal/service/openai_gateway_service.go backend/internal/service/openai_gateway_service_test.go` 通过。
- 风险：默认关闭弱 content fallback 会减少无显式会话信号请求的账号粘连；显式 `session_id`、`conversation_id`、`prompt_cache_key` 和 WS ingress fallbackSeed 语义不变。未构建镜像、未部署、未推送。

## 2026-06-13 15:54 +08:00 - OpenAI response text 过滤 P0 阶段分流与冷却

- 执行者：Devil
- 变更范围：`backend/internal/service/openai_gateway_service.go`、`backend/internal/service/openai_response_text_error.go`、`backend/internal/service/openai_gateway_service_test.go`
- RED：`go test -tags unit ./internal/service -run "TestOpenAIStreamingConfiguredResponseText" -count=1` 修复前失败，写前命中未触发账号冷却，写后命中仍返回 `UpstreamFailoverError`。
- RED：`go test -tags unit ./internal/service -run "TestOpenAIStreaming.*ConfiguredResponseText" -count=1` 修复前失败，passthrough 写后命中同样返回 `UpstreamFailoverError`。
- OBSERVED：`go test -tags unit ./internal/service -run "TestOpenAINonStreamingConfiguredResponseTextReturnsFailover" -count=1` 修复前失败，JSON 非流式响应正文命中关键词未被拦截。
- GREEN：`go test -tags unit ./internal/service -run "TestOpenAIStreaming.*ConfiguredResponseText|TestOpenAINonStreamingConfiguredResponseTextReturnsFailover" -count=1` 通过。
- GREEN：`go test -tags unit ./internal/service -run "TestOpenAIStreaming.*ResponseText|TestOpenAINonStreamingConfiguredResponseTextReturnsFailover|TestOpenAIGatewayServiceStreamFailoverAvoid|TestOpenAIStreaming.*ResponseFailed" -count=1` 通过。
- GREEN：`go test ./cmd/server -run TestNoSuchTest -count=1` 通过。
- GREEN：`git diff --check` 退出码 0；仅提示既有 `.codegraph/daemon.pid` 换行警告。
- 风险：本轮只落地 P0，结构化规则、observe/dry-run、三级策略源和 `drop_event` 未实现；未构建镜像、未部署、未推送。

## 2026-06-13 17:23 +08:00 - OpenAI response text 过滤 P1 结构化规则与 observe

- 执行者：Devil
- 变更范围：`backend/internal/service/openai_response_text_error.go`、`backend/internal/service/openai_gateway_service.go`、`backend/internal/service/openai_response_text_error_test.go`、`backend/internal/service/openai_gateway_service_test.go`
- RED：`go test -tags unit ./internal/service -run "TestOpenAIResponseTextErrorDetectorStructuredRules|TestOpenAIResponseTextErrorDetectorLegacyKeywordsMapToAvoidTTL|TestOpenAIStreamingStructuredResponseTextObserveDoesNotModifyFlow" -count=1` 修复前编译失败，缺少结构化 match API、动作枚举和 observe 网关分支。
- GREEN：同一 P1 聚焦命令通过，覆盖 `textExcludes` 白名单、`textIncludes` 命中动作、`errorCodes` 命中动作、旧关键词映射为 `avoid_ttl`、observe 命中只记录 `stream_observe` 且不改流/不冷却账号。
- GREEN：`go test -tags unit ./internal/service -run "TestOpenAIStreaming.*ResponseText|TestOpenAINonStreamingConfiguredResponseTextReturnsFailover|TestOpenAIGatewayServiceStreamFailoverAvoid|TestOpenAIStreaming.*ResponseFailed|TestOpenAIResponseTextErrorDetector|TestAccountTestService.*ResponseText" -count=1` 通过，旧关键词和 P0 response text/response.failed 相邻语义保持。
- GREEN：`go test ./cmd/server -run TestNoSuchTest -count=1` 通过，server 编译切片无测试运行。
- OBSERVED：不带 `-tags unit` 的 `./internal/service` 聚焦命令仍被既有 `anthropicHTTPUpstreamRecorder` 测试桩缺失阻塞；本轮沿用项目既有 service 单测标签执行。
- 风险：本轮只做账号 credentials 结构化规则兼容读取和 observe/dry-run；未做管理端 UI、全局规则源、三级策略合并和 SSE `drop_event`。未构建镜像、未部署、未推送。

## 2026-06-13 17:45 +08:00 - OpenAI response text 过滤 P2-B 安全边界

- 执行者：Devil
- 变更范围：`backend/internal/service/openai_response_text_error.go`、`backend/internal/service/openai_response_text_error_test.go`
- RED：`go test -tags unit ./internal/service -run "TestOpenAIResponseTextErrorRulesSafetyLimits|TestOpenAIResponseTextErrorDetectorSkips" -count=1` 修复前编译失败，缺少规则数量上限、匹配项长度上限和 SSE payload 大小上限常量。
- GREEN：同一 P2-B 聚焦命令通过，覆盖超过上限的后续规则不生效、超长文本匹配项丢弃、超大 SSE payload 跳过、图片/base64 SSE 事件跳过且普通文本事件仍可命中。
- GREEN：`go test -tags unit ./internal/service -run "TestOpenAIStreaming.*ResponseText|TestOpenAINonStreamingConfiguredResponseTextReturnsFailover|TestOpenAIResponseTextErrorDetector|TestAccountTestService.*ResponseText" -count=1` 通过，P0/P1 response text 相邻路径保持。
- GREEN：`go test ./cmd/server -run TestNoSuchTest -count=1` 通过，server 编译切片无测试运行。
- 风险：本轮只做 P2-B detector 安全边界；TTL 上下限因当前结构化规则尚无 TTL 字段而暂不扩展配置面；未做 P2-A 三级策略源和 P2-C `drop_event`。未构建镜像、未部署、未推送。

## 2026-06-13 18:08 +08:00 - OpenAI response text 过滤 P2-A 三级策略源

- 执行者：Devil
- 变更范围：`backend/internal/service/domain_constants.go`、`backend/internal/service/setting_service.go`、`backend/internal/service/openai_response_text_error.go`、`backend/internal/service/openai_gateway_service.go`、`backend/internal/service/openai_response_text_error_test.go`
- RED：`go test -tags unit ./internal/service -run "TestOpenAIResponseTextErrorRulesFromSettingService|TestOpenAIResponseTextErrorDetectorThreeLevelRules" -count=1` 修复前编译失败，缺少全局 setting key、SettingService 读取方法、detector 多源合并入口和默认规则 ID。
- GREEN：同一 P2-A 聚焦命令通过，覆盖 settings 表全局规则解析、账号规则优先于管理端全局规则、管理端规则对未配置账号规则的 OpenAI 账号生效、系统默认 `cyber_policy` error code 规则生效。
- GREEN：`go test -tags unit ./internal/service -run "TestOpenAIResponseTextErrorRulesFromSettingService|TestOpenAIResponseTextErrorDetectorThreeLevelRules|TestOpenAIResponseTextErrorDetectorStructuredRules|TestOpenAIResponseTextErrorDetectorLegacyKeywordsMapToAvoidTTL|TestOpenAIResponseTextErrorRulesSafetyLimits|TestOpenAIResponseTextErrorDetectorSkips" -count=1` 通过。
- GREEN：`go test -tags unit ./internal/service -run "TestOpenAIStreaming.*ResponseText|TestOpenAINonStreamingConfiguredResponseTextReturnsFailover|TestOpenAIResponseTextErrorDetector|TestAccountTestService.*ResponseText|TestOpenAIFastPolicy|TestOpenAIPromptCache" -count=1` 通过，response text、账号测试、fast policy、prompt cache 相邻路径保持。
- GREEN：`go test ./cmd/server -run TestNoSuchTest -count=1` 通过，server 编译切片无测试运行。
- 风险：本轮只做后端 settings 全局规则源和内置安全默认 error-code 规则，不做管理端 UI/CRUD；P2-C `drop_event` 仍按计划保持未实现。未构建镜像、未部署、未推送。

## 2026-06-13 19:13 +08:00 - OpenAI response text 过滤 P2-C drop_event

- 执行者：Devil
- 变更范围：`backend/internal/service/openai_response_text_error.go`、`backend/internal/service/openai_gateway_service.go`、`backend/internal/service/openai_gateway_service_test.go`
- RED：`go test -tags unit ./internal/service -run "TestOpenAIStreaming.*DropEvent" -count=1` 修复前失败，`drop` 仍被当作 `retry_no_avoidance` 触发错误终止。
- GREEN：`go test -tags unit ./internal/service -run "TestOpenAIStreaming.*DropEvent|TestOpenAIStreaming.*ResponseText|TestOpenAINonStreamingConfiguredResponseTextReturnsFailover|TestOpenAIResponseTextErrorDetector|TestAccountTestService.*ResponseText" -count=1` 通过，覆盖普通 streaming、passthrough streaming 单帧丢弃和既有 response text 相邻路径。
- GREEN：`go test -tags unit ./internal/service -run "TestOpenAIStreaming.*DropEvent|TestOpenAIResponseTextErrorDetector" -count=1` 通过。
- GREEN：`go test ./cmd/server -run TestNoSuchTest -count=1` 通过，server 编译切片无测试运行。
- 风险：本轮只做 SSE 单事件丢弃，不引入跨 chunk SSE buffer；非流式 JSON 的 `drop` 仍沿用错误/failover 语义。未构建镜像、未部署、未推送。

## 2026-06-14 14:13 +08:00 - 停用 cockpit-tools 模式并发布 v0.1.134.38

- 执行者：Devil
- 变更范围：`backend/internal/config/config.go`、`backend/internal/service/setting_service.go`、`backend/internal/service/openai_gateway_service.go`、`backend/internal/service/openai_ws_protocol_resolver.go`、`backend/internal/handler/admin/setting_handler.go`、`frontend/src/views/admin/SettingsView.vue`、`deploy/.env.example`、`deploy/config.example.yaml`
- GREEN：`go test -tags unit ./internal/config -run "TestLoadOpenAICockpitToolsCompatConfig" -count=1` 通过，覆盖旧 cockpit 布尔和枚举配置归一为 `off`。
- GREEN：`go test -tags unit ./internal/service -run "TestOpenAIWSProtocolResolver|TestSettingService_(UpdateSettings_LegacyOpenAICockpitToolsCompatNormalizesToOff|UpdateSettings_LegacyOpenAIOAuthCompatModeRefreshesGatewayConfigAsOff|ParseSettings_LegacyOpenAICockpitToolsCompatFallsBackToOffWhenMissing|ParseSettings_DeprecatedCodexDirectNormalizesToOff|LoadRuntimeSettingsRefreshesGatewayConfig)|TestOpenAIGatewayService_APIKeyRequestBaseURLFailoverBeforeAccountFailover|TestOpenAIBuildUpstreamRequestLegacyCockpitToolsCompatIsIgnored|TestOpenAIPassthroughLegacyCockpitToolsCompatIsIgnored|TestOpenAIBuildUpstreamRequestAccountCodexSimulationHeaders|TestOpenAIUpstreamTLSProfileUsesAccountLevelCodexSimulationOnly" -count=1` 通过，覆盖 legacy cockpit ignored、WS 决策、SettingService 热刷新和 API Key base URL transport failover。
- GREEN：`go test -tags unit ./internal/handler/admin -run "TestSettingHandler_UpdateSettings_NormalizesDeprecatedOpenAIOAuthModeToOff|Test.*Setting" -count=1` 通过，覆盖旧请求体归一和设置保存链路。
- GREEN：`go test -tags unit ./cmd/server -run "^$" -count=1` 通过，server 编译切片无测试运行。
- GREEN：`npm run typecheck` 通过。
- BUILD：`sub2api:v0.1.134.38` 从提交 `60f272128043` 的 `git archive HEAD` 构建成功，镜像 label `org.opencontainers.image.revision=60f272128043`，镜像 ID `sha256:ae4764cc6c2d93bacb8e7f3c7dbfc78e585fb8585855046225489d27eabfad06`。
- DEPLOY：发布前 active 为 blue `sub2api:v0.1.134.37`；新版本部署到 idle green，候选端口 `18082` 验证 `/health` 200、首页 200、静态资源 `/assets/index-CN6LQCCL.js` 200、未登录 `/api/v1/admin/system/version`/`/responses`/`/v1/messages` 均 401。
- CUTOVER：`D:\sub2api-deploy\proxy\upstreams\active.conf` 切到 `sub2api-green:8080`，`docker exec sub2api-proxy nginx -t` 通过，`docker exec sub2api-proxy nginx -s reload` 成功；切流后 `8080` 同一组冒烟通过。
- 风险：未做登录态管理端浏览器保存操作；已用 handler/service/typecheck 覆盖保存与类型链路。blue `sub2api:v0.1.134.37` 保持 healthy，可按 release note 秒级切回。

## 2026-06-15 11:07 +08:00 - 账号管理页高密度总览布局

- 执行者：Devil
- 变更范围：`frontend/src/views/admin/AccountsView.vue`、`frontend/src/components/common/DataTable.vue`、`docs/feature_list.jsonl`、`docs/process_list.jsonl`
- GREEN：`npm run typecheck` 通过。
- GREEN：`npm run build` 通过；仅保留既有 Vite dynamic-import/chunk 体积警告。
- GREEN：Playwright mock 数据验证账号管理页桌面布局：1366、1440、1920 宽度下 `wrapperOverflowX=0`、`wrapperScrollLeft=0`、body 无正向横向溢出；行高约 311/291/199 px；截图保存为 `.codex/accounts-layout-after-1366.png`、`.codex/accounts-layout-after-1440.png`、`.codex/accounts-layout-after-1920.png`。
- 风险：移动端仍走 DataTable 卡片路径，本轮主要解决桌面“大页面一次看齐、无需左右滑动”的账号管理体验；未提交、未部署。

## 2026-06-15 16:54 +08:00 - 账号管理页高密度总览布局发布

- 执行者：Devil
- 变更范围：`frontend/src/views/admin/AccountsView.vue`、`frontend/src/components/common/DataTable.vue`
- GREEN：`npm run typecheck` 通过。
- GREEN：`npm run build` 通过，只有既有 chunk size 警告。
- BUILD：提交 `488015772fb1` 从 `git archive HEAD` 构建 `sub2api:v0.1.134.39`，镜像 revision `488015772fb1`，镜像 ID `sha256:f680eb1894586023a62a63c7a7833410601cfda06cc7bb81e1b6d96e954e80f8`。
- DEPLOY：blue 候选 `18083` 冒烟通过，`/health`、首页、静态资源 200，未登录 `/api/v1/admin/system/version`、`/responses`、`/v1/messages` 均 401。
- VERIFY：blue 容器健康超过 60 秒；切流后 `8080` 同组冒烟通过；blue/green 均 healthy。
- OBSERVE：日志仅命中一次 `pq: canceling statement due to user request` 的后台清理噪声，未见 panic/fatal/migration/bind/listen tcp/rebuild failed。

## 2026-06-15 21:15 +08:00 - OpenAI 与 Anthropic 异常账号探测频率限制发布

- 执行者：Devil
- 变更范围：`backend/internal/handler/failover_loop.go`、`backend/internal/handler/openai_gateway_handler.go`、`backend/internal/handler/openai_chat_completions.go`、`backend/internal/handler/openai_images.go`、`backend/internal/service/gateway_service.go`、`backend/internal/service/openai_gateway_service.go`、`backend/internal/service/account_test_service.go`、`backend/internal/service/gateway_retry_rate_limit_test.go`、相关测试与 JSONL 记录。
- GREEN：`go test -tags unit ./internal/service -run "TestGatewayService_AnthropicAPIKeyPassthrough_.*DoesNotSwitchRequestBaseURLOn5xx|TestGatewayService_AnthropicAPIKeyPassthrough_CountTokensDoesNotSwitchRequestBaseURLOn5xx|TestOpenAIGatewayService_APIKeyRequestBaseURLDoesNotFailoverBeforeAccountFailover|TestGatewayService_AnthropicRetryExhaustionDoesNotDelegateSameAccountRetry|TestGatewayService_TempUnscheduleRetryableError|TestRetryBackoffDelay|TestRetryBudget|TestOpenAIWSRetryBackoff|TestProbeIntervalFromErrorCount|TestOpenAIWSReconnect" -count=1` 通过。
- GREEN：`go test ./internal/handler -run "TestSingleAccountExhaustionBackoffDelay|TestHandleFailoverError" -count=1 -v` 通过。
- COMMIT：提交 `3490f86a4cbc fix(gateway): 限制异常账号探测频率`。
- BUILD：从 `git archive HEAD` 构建 `sub2api:v0.1.134.40`，镜像 ID `sha256:e192033cbb10a9a230143895558f2a0a3e4d4571901e14c7bfcc15b5a917db4a`，label revision `3490f86a4cbc`，二进制版本显示 `image: v0.1.134.40`。
- DEPLOY：发布前 active 为 blue `sub2api:v0.1.134.39`；新版本部署到 idle green，候选端口 `18082` 的 health/home/static 200，未登录 admin version、`/responses`、`/v1/messages` 均 401，green healthy 持续 66 秒。
- CUTOVER：`D:\sub2api-deploy\proxy\upstreams\active.conf` 切到 `sub2api-green:8080`，`nginx -t` 和 reload 通过；切流后 `8080` 与 `18081` 同组冒烟通过，green/blue 均 healthy，proxy/green 关键错误日志命中 0。
- 风险：未做登录态管理页操作；blue `sub2api:v0.1.134.39` 保留 healthy 作为回滚目标。

## 2026-06-16 12:14 +08:00 - 调度恢复与 hotaruapi 修复发布 v0.1.134.41

- 执行者：Devil
- 变更范围：已提交 HEAD `0610d7bdcf69` 中的调度池人工测试恢复和 hotaruapi Codex CLI 模拟修复；本次追加 `docs/releases/v0.1.134.41.md`、`docs/feature_list.jsonl`、`docs/process_list.jsonl`。
- GREEN：`go test -tags unit ./internal/service -run 'TestRateLimitService_(RecordAccountProbeOutcome|RecoverAccount|ClearRateLimit)|TestAccountProbeService_RunRecordsAccountProbeOutcomeFailure|TestAccountDerivedHealth|TestAccountTestService_OpenAIResponsesStreamBareJSONErrorReturnsUpstreamMessage|TestAccountTestService_OpenAIAPIKeyTriesNextRequestBaseURLOnTransientError|TestAccountTestService_OpenAIAPIKeyResponsesTestUsesGatewayCodexSimulationHeaders|TestOpenAIStreamingTerminalEventFromSSEEventLineCompletes|TestOpenAIStreamingMissingTerminalEventReturnsIncompleteError|TestOpenAICodexCLISimulationUsesLatestClientVersion|TestOpenAIBuildUpstreamRequestAccountCodexSimulationHeaders|TestOpenAIGatewayService_BuildOpenAIWSHeadersAccountCodexSimulationForceWS|TestApplyOpenAICodexLatestClientHeadersMatchesCapturedClientShape' -count=1` 通过。
- GREEN：`go test -tags unit ./internal/pkg/openai -run 'TestIsCodexOfficialClient' -count=1` 通过。
- GREEN：`go test ./cmd/server -run TestNoSuchTest -count=1` 通过，server 编译切片无测试运行。
- BUILD：从 `git archive HEAD` 构建 `sub2api:v0.1.134.41`，镜像 ID `sha256:0314391dcdbad4277477fc817431f32cacf2488e38e49bc0627b2d84820d4801`，label revision `0610d7bdcf69`，二进制版本显示 `image: v0.1.134.41`。
- DEPLOY：发布前 active 为 green `sub2api:v0.1.134.40`；新版本部署到 idle blue，候选端口 `18083` 的 health/home/static 200，未登录 admin version、`/responses`、`/v1/messages` 均 401，blue healthy 持续 65 秒。
- CUTOVER：`D:\sub2api-deploy\proxy\upstreams\active.conf` 切到 `sub2api-blue:8080`，`nginx -t` 和 reload 通过；切流后 `8080` 与 `18081` 同组冒烟通过，blue/green 均 healthy，proxy/blue 关键错误日志命中 0。
- 风险：未做登录态管理端页面操作；green `sub2api:v0.1.134.40` 保留 healthy 作为回滚目标。

## 2026-06-16 13:21 +08:00 - free-rawchat 账号隔离调用验证

- 执行者：Devil
- 范围：线上 active `sub2api-blue` / `sub2api:v0.1.134.41`，账号 `free-rawchat` `account_id=470`，模型 `gpt-5.3-codex`。
- VERIFY：创建临时隔离 group/key，只绑定 `account_id=470`，经入口 `http://127.0.0.1:8080/responses` 发起 OpenAI Responses 流式请求；日志确认临时 key 请求进入 `group_id=28`，随后上游调用命中 `account_id=470`。
- FAIL：上游返回 HTTP 403，错误码 `codex_access_restricted`，错误消息为“请使用最新版的codex客户端或codex cli调用”。因为临时组只有这一个账号，failover 后调度器持续 `no available OpenAI accounts supporting model: gpt-5.3-codex`，客户端侧最终超时。
- CLEANUP：临时 group/key 已全部软删除或停用，`free-rawchat` 已恢复为原始状态：`schedulable=false`，只绑定回 `group_id=2`，临时不可调度与 error_message 字段为空。
- 结论：passes:false；sub2api 能调度并调用到 `free-rawchat`，但该账号上游拒绝 Codex 形状请求，当前不能作为可通过账号使用。

## 2026-06-16 16:08 +08:00 - gptai-plus 缺终态流和手动探测恢复修复

- 执行者：Devil
- 变更范围：`backend/internal/service/openai_gateway_service.go`、`backend/internal/service/account_probe.go`、`backend/internal/handler/admin/account_handler.go` 及对应 service/handler 回归测试。
- 证据：线上 `sub2api-blue` / `sub2api:v0.1.134.41` 日志显示 `gptai-plus` `account_id=434` 的 `/responses` 流在已有输出后缺少 terminal event，旧逻辑返回 `stream usage incomplete: missing terminal event` 并让客户端快速终止；数据库显示该账号最近三次 probe 成功且 `account_probe_health=normal`，但 `schedulable=false` 未恢复，因为异步人工 probe 被记为 `account_probe` 而不是 `manual_test`。
- RED：`go test -tags unit ./internal/service -run 'TestOpenAIStreamingMissingTerminalEventAfterOutputSynthesizesTerminal|TestOpenAIStreamingMissingTerminalEventRecordsPathHealthFailure|TestAccountProbeOutcomeFromRunManualTriggerUsesManualTestSource' -count=1 -v` 修复前编译失败，缺少 `missingTerminalEvent` 标记和手动触发映射。
- GREEN：`go test -tags unit ./internal/service -run 'TestOpenAIStreaming(MissingTerminalEventAfterOutputSynthesizesTerminal|TerminalEventFromSSEEventLineCompletes|MissingTerminalEventRecordsPathHealthFailure|ClientDisconnectDrainsUpstreamUsage|HTTP2ReadErrorAfterOutputRecordsProtocolFailure)|TestAccountProbe(Service_RunRecordsAccountProbeOutcomeFailure|OutcomeFromRunManualTriggerUsesManualTestSource)|TestRateLimitService_(RecordAccountProbeOutcome|RecoverAccountState)' -count=1 -v` 通过。
- GREEN：`go test -tags unit ./internal/handler/admin -run 'TestAccountProbe(Create|ReportBatchCreate)|TestAccountModelProbe' -count=1 -v` 通过。
- GREEN：`go test ./cmd/server -run TestNoSuchTest -count=1` 通过，server 编译切片无测试运行。
- GREEN：`git diff --check -- backend/internal/service/openai_gateway_service.go backend/internal/service/openai_gateway_service_test.go backend/internal/service/account_probe.go backend/internal/service/account_probe_test.go backend/internal/handler/admin/account_handler.go backend/internal/handler/admin/account_probe_async_test.go backend/internal/handler/admin/account_probe_report_test.go` 通过。
- 风险：本轮未提交、未构建镜像、未部署；线上 `gptai-plus` 当前仍保持原始 `schedulable=false`，需要发布后再次手动探测或按管理动作恢复。

## 2026-06-16 17:23:09 +08:00 Devil - gptai-plus missing usage stream accounting
- go test -tags unit ./internal/service -run "TestOpenAIStreaming(MissingTerminalEventAfterOutputSynthesizesTerminal|TerminalEventFromSSEEventLineCompletes|ReuseScannerBufferAndStillWorks)|TestOpenAIGatewayServiceRecordUsage_(ZeroUsageStillWritesUsageLog|MissingObservedUsageRejectsUsageLog|FeedsPathHealthSample|MissingPricingRecordsZeroCostUsageLog)" -count=1 -v: PASS
- go test ./cmd/server -run TestNoSuchTest -count=1: PASS
- go test -tags unit ./internal/handler -run TestNoSuchTest -count=1: PASS
- git diff --check: PASS with existing CRLF warnings for .codegraph/daemon.pid and docs files

## 2026-06-16 18:00:20 +08:00 Devil - gptai-plus terminal without usage verification
- Direct upstream account_id=434 /v1/responses: HTTP 200, response.completed=True, output_delta=True, usage=False; response saved under .codex/gptai-plus-direct-upstream.sse without secrets.
- go test ./internal/service -run "TestSelectAccountWithLoadAwareness_ReusesRequestSchedulingSnapshot|TestSelectAccountWithLoadAwareness_PrivacyUnsetSkipsWithoutSetError|TestOpenAIStreamingTerminalEventWithoutUsageMarksUsageMissing" -count=1 -v: PASS
- go test ./cmd/server -run TestNoSuchTest -count=1: PASS
- go test -tags unit ./internal/handler -run TestNoSuchTest -count=1: PASS
- git diff --check: PASS with existing .codegraph/daemon.pid CRLF warning only

## 2026-06-16 18:06:45 +08:00 Devil - release v0.1.134.45
- Build: sub2api:v0.1.134.45 from HEAD dc03035462ab, image version in binary reports v0.1.134.45.
- Deploy: active switched from green v0.1.134.44 to blue v0.1.134.45 via nginx upstream reload.
- Candidate blue 18083: health 200, home 200, unauth admin version/responses/messages 401, healthy after 60s.
- Candidate logs: one startup cleanup pq: canceling statement due to user request; a clean 90s observation window then had critical log hits=0.
- Post-cutover 8080/18081: health 200 and unauth /responses 401.
- Real gptai-plus upstream account_id=434: HTTP 200, response.completed=True, output_delta=True, usage=False.

## 2026-06-17 12:00:00 +08:00 Devil - OPTIMIZATION_TODO priority implementation
- Scope: completed the remaining optimization backlog from `docs/OPTIMIZATION_TODO.md` across backend hot path scheduling, admin UI simplification/performance, shared account modal fields, and payment polling backoff.
- RED/FIX: `AccountsView.bulkEdit.spec.ts` was repaired after a bad mechanical edit introduced extra `)` in several mocks; the single-file Vitest run exposed exact parser lines and the final repair kept wrapper unmount/timer cleanup.
- GREEN: `rtk npm run test:run -- src/views/admin/__tests__/AccountsView.bulkEdit.spec.ts`: 11 tests passed.
- GREEN: `rtk npm run test:run -- src/views/admin/__tests__/DashboardView.spec.ts src/views/admin/__tests__/AccountSchedulingPoolView.spec.ts src/views/admin/__tests__/AccountsView.bulkEdit.spec.ts src/components/payment/__tests__/PaymentStatusPolling.spec.ts src/components/payment/__tests__/PaymentStatusPanel.spec.ts src/components/account/__tests__/AccountPoolModeSection.spec.ts src/components/account/__tests__/AccountBasicInfoFields.spec.ts src/components/account/__tests__/AccountUpstreamBalanceFields.spec.ts src/components/account/__tests__/AccountModelRestrictionSection.spec.ts src/components/account/__tests__/EditAccountModal.spec.ts`: 10 files and 57 tests passed.
- GREEN: `rtk npm run typecheck`: passed.
- GREEN: `rtk go test -tags unit ./internal/service -run "TestOpenAIAccountSchedulerPrivacySetSkipsWithoutSetError|TestSelectAccountWithLoadAwareness_RequestSchedulingSnapshotReusesPrefetchAcrossExclusions|TestWithWindowCostPrefetch" -count=1 -v`: 5 tests passed.
- GREEN: `rtk go test -tags unit ./internal/handler -run "TestResolveGatewayRequestSchedulingSnapshotKey|TestGatewayHandlerSubmitUsageRecordTask_NilTask" -count=1 -v`: 5 tests passed.
- GREEN: `rtk go test ./cmd/server -run TestNoSuchTest -count=1`: passed with no tests.
- GREEN: `git diff --check`: passed; only existing CRLF warnings for `.codegraph/daemon.pid`, `docs/feature_list.jsonl`, and `docs/process_list.jsonl`.
- Warnings observed: Browserslist `caniuse-lite` data is stale; Vitest still emits existing i18n missing key warnings for `common.time.never`.
- Remaining risk: P2 modal extraction intentionally leaves Antigravity model mapping, OpenAI compact mapping, and quota-control sections in the original modals because they carry account-specific state sync and submit semantics.

## 2026-06-17 14:08:31 +08:00 Devil - OPTIMIZATION_TODO parallel tail completion
- Scope: continued parallel completion of the remaining safe optimization tail. Account modals now share API Key credential fields and upstream credential fields; SettingsView now shares the repeated section save button; `docs/OPTIMIZATION_TODO.md` now reflects the actual implemented state.
- GREEN: `rtk npm run test:run -- src/components/account/__tests__/AccountAPIKeyCredentialsFields.spec.ts src/components/account/__tests__/AccountUpstreamCredentialsFields.spec.ts src/components/account/__tests__/AccountOptionSelector.spec.ts src/components/account/__tests__/AccountQuotaControlSection.spec.ts src/components/account/__tests__/AccountAnthropicQuotaControlSection.spec.ts src/components/account/__tests__/AccountAntigravityModelMappingSection.spec.ts src/components/account/__tests__/AccountOpenAICompactModeSection.spec.ts src/components/account/__tests__/AccountModelMappingList.spec.ts src/components/account/__tests__/AccountPoolModeSection.spec.ts src/components/account/__tests__/AccountBasicInfoFields.spec.ts src/components/account/__tests__/AccountUpstreamBalanceFields.spec.ts src/components/account/__tests__/AccountModelRestrictionSection.spec.ts src/components/account/__tests__/EditAccountModal.spec.ts`: 13 files and 45 tests passed.
- GREEN: `rtk npm run test:run -- src/views/admin/__tests__/SettingsSaveBar.spec.ts src/views/admin/__tests__/SettingsTabNavigation.spec.ts src/views/admin/__tests__/SettingsSectionSaveButton.spec.ts src/views/admin/__tests__/SettingsView.spec.ts`: 4 files and 21 tests passed.
- GREEN: `rtk npm run typecheck`: passed.
- GREEN: `rtk go test -tags unit ./internal/service -run "TestOpenAIAccountSchedulerPrivacySetSkipsWithoutSetError|TestSelectAccountWithLoadAwareness_RequestSchedulingSnapshotReusesPrefetchAcrossExclusions|TestWithWindowCostPrefetch|TestOpenAIAccountSchedulerSkipsAPIKeyAccountWithNoActiveKeys" -count=1 -v`: 6 tests passed.
- GREEN: `rtk go test -tags unit ./internal/handler -run "TestResolveGatewayRequestSchedulingSnapshotKey|TestGatewayHandlerSubmitUsageRecordTask_NilTask" -count=1 -v`: 5 tests passed.
- GREEN: `rtk go test ./cmd/server -run TestNoSuchTest -count=1`: passed with no tests.
- GREEN: `git diff --check`: passed; only existing CRLF warnings for `.codegraph/daemon.pid`, `docs/feature_list.jsonl`, `docs/process_list.jsonl`, and `verification.md`.
- Warnings observed: Browserslist `caniuse-lite` data is stale; `rtk` reports no global hook installed.
- Remaining risk: no known unfinished P0/P1/P2/P3 item from `docs/OPTIMIZATION_TODO.md`; further account/Settings decomposition would be optional high-coupling refactor work, not part of the current optimization backlog.

## 2026-06-17 15:58:36 +08:00 Devil - version 0.1.136 and image version display
- Scope: paused visual/style changes and only updated version semantics. Main binary version source is now `0.1.136`; image version remains a separate build/runtime value surfaced as `image_version`.
- GREEN: `go test ./internal/service -run TestUpdateServiceCheckUpdateExposesImageVersionSeparately -count=1`: passed.
- GREEN: `go test ./cmd/server -run TestNoSuchTest -count=1`: passed with no tests.
- GREEN: `corepack pnpm vitest run src/components/common/__tests__/VersionBadge.spec.ts src/stores/__tests__/app.spec.ts`: 2 files and 24 tests passed.
- GREEN: `corepack pnpm typecheck`: passed.
- GREEN: `git diff --check`: passed; only existing CRLF warnings for `.codegraph/daemon.pid`, `backend/cmd/server/VERSION`, `docs/feature_list.jsonl`, `docs/process_list.jsonl`, and `verification.md`.
- Warnings observed: initial `pnpm ...` command failed because `pnpm` is not directly on PATH; `corepack pnpm` is available and was used. Browserslist data is stale.

## 2026-06-17 16:45:12 +08:00 Devil - release v0.1.136.1
- Build: `sub2api:v0.1.136.1` from HEAD `d19b309c89dc`; image labels report `version=v0.1.136.1` and `revision=d19b309c89dc`.
- Binary version: `docker run --rm sub2api:v0.1.136.1 /app/sub2api -version` and `docker exec sub2api-blue /app/sub2api -version` both report `Sub2API 0.1.136 (image: v0.1.136.1, commit: d19b309c89dc)`.
- Deploy: active switched from green `sub2api:v0.1.134.48` to blue `sub2api:v0.1.136.1` via nginx upstream reload.
- Candidate blue 18083: `/health` 200, home 200, unauth `/api/v1/admin/accounts` 401, unauth `/responses` 401, unauth `/v1/responses` 401, container `Health=healthy` after 70s.
- Candidate logs: one startup request snapshot cleanup `pq: canceling statement due to user request` was observed before the observation window; the following 70s window had critical log hits=0.
- Post-cutover 8080/18081: `/health` 200, home 200, unauth `/api/v1/admin/system/version` 401, unauth `/responses` 401.
- Browser smoke: Playwright opened `http://127.0.0.1:8080/`, redirected to `/home`, title `Home - Sub2API`, page snapshot nonempty.
- Current state: active blue `sub2api:v0.1.136.1`; rollback green `sub2api:v0.1.134.48`.

## 2026-06-18 08:27:39 +08:00 Devil - Jungongyi GPT Codex CLI simulation terminal compatibility
- Scope: fixed account test and account probe OpenAI Responses stream parsing for upstreams that emit text deltas and then close with EOF or `data: [DONE]` without `response.completed`.
- Evidence: production `sub2api-blue` log contains `Account test error: Stream ended before response.completed` from `service/account_test_service.go:1653` at `2026-06-18T08:21:27+08:00`.
- RED: account test and account probe parser tests failed on output-present EOF/[DONE] cases before implementation; empty stream stayed failing.
- GREEN: service focused command passed: `go test -tags unit ./internal/service -run "TestAccountTestService_OpenAIResponses(StreamEOFAfterOutputCompletes|StreamDoneAfterOutputCompletes|EmptyStreamStillFails|StreamEmitsFirstTokenMs|StreamBareJSONErrorReturnsUpstreamMessage)$|TestReadAccountProbeOpenAIResponses(StreamEOFAfterOutputCompletes|StreamDoneAfterOutputCompletes|EmptyStreamStillFails)$|TestAccountProbeService_RunOpenAIAPIKeyStreamModeRecordsFirstToken$|TestOpenAIStreaming(TerminalEventWithoutUsageAddsClientTerminalFields|TerminalEventWithDoneDoesNotDuplicateDoneMarker|MissingTerminalEventAfterOutputSynthesizesTerminal|MissingTerminalEventRecordsPathHealthFailure)" -count=1 -v`.
- GREEN: `go test ./cmd/server -run TestNoSuchTest -count=1` passed.
- GREEN: `git diff --check -- backend/internal/service/account_test_service.go backend/internal/service/account_test_service_openai_test.go backend/internal/service/account_probe.go backend/internal/service/account_probe_test.go` passed.
- Remaining state: not committed, not built, not deployed; online `sub2api-blue` still runs `sub2api:v0.1.136.1` until a release is performed.

## 2026-06-18 19:19:25 +08:00 Devil - multi-key API key status display and selected-key cooldown

- Scope: OpenAI API Key accounts with multiple saved keys now expose each saved key's current status in account DTO/UI, and schedulable upstream errors first cool down the selected failing key before falling back to account-level scheduling cooldown.
- Backend behavior: `RateLimitService.tryAPIKeyAccountSchedulingCooldown` calls selected-key cooldown for multi-key accounts using `LastSelectedAPIKey`; single-key or un-attributable errors keep the existing account-level cooldown path. Admin account tests now use the same cooldown helper.
- DTO/UI behavior: `api_key_items` now includes `status`, `disabled_until`, and `disabled_count`; expired key cooldowns render as active again; the account key field displays active/cooling status, reason, recovery time, cooldown count, restore and delete actions.
- GREEN: `go test -tags unit ./internal/service -run "TestRateLimitService_HandleUpstreamError_OpenAIAPIKey(402UsesSchedulingCooldown|InsufficientBalance403UsesSchedulingCooldown|BadRequestQuotaUsesSchedulingCooldown|ForbiddenInvalidKeyUsesSchedulingCooldown)|TestRateLimitService_HandleUpstreamError_OpenAI403InsufficientBalanceDisablesSelectedKey|TestHandleUpstreamError429_OpenAIAPIKey(DisablesSelectedKey|SchedulingCooldownUsesSteppedErrorCount|WithTempRulesDisablesSelectedKey)|TestRateLimitService_HandleUpstreamError_(OpenAIAPIKey429UsesAccountScheduling|NonOAuth401)|TestAccountTestService_OpenAI(APIKeyInsufficientBalanceDisablesSelectedKey|ChatCompletionsPathDisablesSelectedKey|CompactPathDisablesSelectedKey|ImagePathDisablesSelectedKey)" -count=1`: PASS.
- GREEN: `go test -tags unit ./internal/handler/dto -run TestAccountFromServiceShallow -count=1`: PASS.
- GREEN: `go test ./cmd/server -run TestNoSuchTest -count=1`: PASS.
- GREEN: `corepack pnpm vitest run src/components/account/__tests__/AccountAPIKeyCredentialsFields.spec.ts src/components/account/__tests__/EditAccountModal.spec.ts`: PASS, 2 files / 25 tests.
- GREEN: `corepack pnpm typecheck`: PASS.
- Warning observed: Vitest reports stale Browserslist `caniuse-lite` data; no test failure.
- Remaining state: not committed, not built, not deployed; worktree still contains unrelated pre-existing UI/static/generated changes outside this feature.

## 2026-06-18 17:00:44 +08:00 Devil - release v0.1.136.5
- Commit: `ec1ad1cfa83a fix(openai): 修正 Codex 模拟裸域名响应路径`。
- Build: `sub2api:v0.1.136.5` from committed HEAD `ec1ad1cfa83a`; binary reports `Sub2API 0.1.136 (image: v0.1.136.5, commit: ec1ad1cfa83a, built: 2026-06-18T08:43:06Z)`。
- Candidate blue 18083: `/health` 200, home 200, unauth `/api/v1/admin/accounts` 401, unauth `/responses` 401, unauth `/v1/responses` 401, `Health=healthy`, `RestartCount=0` after 65 seconds。
- Cutover: nginx upstream switched from `sub2api-green:8080` to `sub2api-blue:8080`; `nginx -t` and reload both succeeded。
- Post-cutover 8080/18081: `/health` 200, home 200, unauth `/api/v1/admin/system/version` 401, unauth `/responses` 401, unauth `/v1/responses` 401。
- Real admin route verification: authenticated `POST /api/v1/admin/accounts/453/test` returned HTTP 200 SSE with `test_start -> error -> status`; business error is `model_not_found` for `gpt-5.5`, and the old protocol error `Stream ended before response.completed` no longer appears。
- Log window after real verification: `sub2api-blue` 65-second critical scan had hit count 0; logs do not contain `Stream ended before response.completed` or `non-SSE HTML response`。

## 2026-06-18 21:49:36 +08:00 Devil - SettingsView Gateway tab extraction
- Scope: continued `.cursor/plans/性能调度重构与页面简约化_53324140.plan.md` Phase 2a and extracted the remaining Gateway settings blocks into `frontend/src/views/admin/settings/GatewaySettingsTab.vue`.
- Behavior: `SettingsView.vue` now renders one Gateway tab component and keeps loading, saving, OpenAI route-policy preset operations, Web Search emulation actions, and payload orchestration in the parent.
- GREEN: `corepack pnpm typecheck`: passed.
- GREEN: `corepack pnpm vitest run src/views/admin/__tests__/SettingsView.spec.ts src/views/admin/__tests__/SettingsTabNavigation.spec.ts src/views/admin/__tests__/SettingsSaveBar.spec.ts src/views/admin/__tests__/SettingsSectionSaveButton.spec.ts src/views/admin/settings/__tests__/GeneralSettingsTab.spec.ts src/views/admin/settings/__tests__/AgreementSettingsTab.spec.ts src/views/admin/settings/__tests__/FeaturesSettingsTab.spec.ts src/views/admin/settings/__tests__/SecuritySettingsTab.spec.ts src/views/admin/settings/__tests__/UsersSettingsTab.spec.ts src/views/admin/settings/__tests__/PaymentSettingsTab.spec.ts src/views/admin/settings/__tests__/EmailSettingsTab.spec.ts src/views/admin/settings/__tests__/GatewaySettingsTab.spec.ts`: 12 files and 30 tests passed.
- GREEN: `corepack pnpm vitest run src/views/admin/__tests__/SettingsView.spec.ts src/views/admin/settings/__tests__/GatewaySettingsTab.spec.ts`: 2 files and 16 tests passed.
- GREEN: `corepack pnpm build`: passed; existing warnings were stale Browserslist data, mixed dynamic/static imports, and chunks larger than 500 KB.
- GREEN: `git diff --check -- frontend/src/views/admin/SettingsView.vue frontend/src/views/admin/settings/GatewaySettingsTab.vue frontend/src/views/admin/settings/__tests__/GatewaySettingsTab.spec.ts`: exit 0; existing CRLF warning remained for `frontend/src/views/admin/SettingsView.vue`.
- Remaining state: not committed, not deployed; worktree still contains unrelated pre-existing backend/frontend/static changes outside this Settings tab extraction slice.

## 2026-06-18 22:34:21 +08:00 Devil - release v0.1.136.6
- Commit: `7a97fb7baa88 docs: 记录调度重构与页面简化进度`.
- Build: `sub2api:v0.1.136.6` from committed HEAD `7a97fb7baa88`; image label version/revision are `v0.1.136.6` / `7a97fb7baa88`.
- Binary: `docker run --rm sub2api:v0.1.136.6 /app/sub2api -version` reported `Sub2API 0.1.136 (image: v0.1.136.6, commit: 7a97fb7baa88, built: 2026-06-18T14:24:37Z)`.
- Active before release: blue `sub2api:v0.1.136.5`; idle target: green `sub2api:v0.1.136.4`.
- Candidate green 18082: `/health` 200, home 200, unauth `/api/v1/admin/accounts` 401, unauth `/responses` 401, unauth `/v1/responses` 401, `Health=healthy`, `RestartCount=0`.
- Candidate logs: startup had one existing cleanup noise `pq: canceling statement due to user request`; following 70-second critical log window had hit count 0.
- Cutover: `D:\sub2api-deploy\proxy\upstreams\active.conf` switched from `sub2api-blue:8080` to `sub2api-green:8080`; `nginx -t` and reload both succeeded.
- Post-cutover 8080/18081: `/health` 200, home 200, unauth `/api/v1/admin/system/version` 401, unauth `/responses` 401, unauth `/v1/responses` 401.
- Browser/page smoke by HTTP: `http://127.0.0.1:8080/` returned 200, contained `<title>`, and body length was 2627.
- Post-cutover logs: 65-second green critical scan had hit count 0.
- Current state: active green `sub2api:v0.1.136.6`; rollback blue `sub2api:v0.1.136.5`; no PostgreSQL/Redis restart.

## 2026-06-18 23:18:55 +08:00 Devil - release v0.1.136.7
- Commit: `806cea4a3e28 fix(openai): 修复 rawchat Codex 模拟头`.
- Root cause evidence: old active green logged trace `6fd1389d-2dc7-49f0-a647-c6fee5d51db7` at `2026-06-18T22:45:27+08:00` from `service/account_test_service.go:793` with upstream `codex_access_restricted`.
- Fix scope: rawchat `/v1/chat/completions`, `/responses` forced raw chat fallback, and admin OpenAI API Key chat-completions test now reuse `applyOpenAICodexCLISimulationHeaders`.
- GREEN: `go test -tags unit ./internal/service -run TestAccountTestService_OpenAIAPIKeyChatCompletionsTestUsesGatewayCodexSimulationHeaders -count=1`.
- GREEN: `go test -tags unit ./internal/service -run TestForwardAsRawChatCompletions_UsesCodexSimulationHeaders -count=1`.
- GREEN: `go test -tags unit ./internal/service -run TestForwardResponses_ForceChatCompletionsUsesCodexSimulationHeaders -count=1`.
- GREEN: focused service regression for rawchat, forced chat fallback, and account-test chat-completions paths passed.
- GREEN: `go test -tags unit ./cmd/server -run TestNoSuchTest -count=1`.
- Build: `sub2api:v0.1.136.7` from committed HEAD `806cea4a3e28`; image label version/revision are `v0.1.136.7` / `806cea4a3e28`.
- Binary: `docker run --rm sub2api:v0.1.136.7 /app/sub2api -version` reported `Sub2API 0.1.136 (image: v0.1.136.7, commit: 806cea4a3e28, built: 2026-06-18T15:01:18Z)`.
- Candidate blue 18083: `/health` 200, home 200, static JS 200, unauth `/api/v1/admin/accounts` 401, unauth `/responses` 401, unauth `/v1/responses` 401, `Health=healthy`, `RestartCount=0`.
- Candidate logs: 65-second critical log window hit count 0.
- Cutover: `D:\sub2api-deploy\proxy\upstreams\active.conf` switched from `sub2api-green:8080` to `sub2api-blue:8080`; `nginx -t` and reload both succeeded.
- Post-cutover 8080/18081: `/health` 200, home 200, unauth `/api/v1/admin/system/version` 401, unauth `/responses` 401, unauth `/v1/responses` 401.
- Post-cutover logs: 65-second blue critical scan hit count 0 and recent blue logs did not contain `codex_access_restricted`.
- Not run: authenticated `POST /api/v1/admin/accounts/470/test`, because `D:\sub2api-deploy\.env` does not contain `ADMIN_EMAIL` or `ADMIN_PASSWORD`; no admin JWT was available and no database/login bypass was used.
- Current state: active blue `sub2api:v0.1.136.7`; rollback green `sub2api:v0.1.136.6`; no PostgreSQL/Redis restart.

## 2026-06-19 23:22:45 +08:00 Devil - GwentDraw 本地任务脚本更新
- Scope: ignored local script `D:\sub2api-src\scripts\gwent_draw.ps1`.
- Behavior: `draw` now loops until response content contains a cooldown signal, auth fails, or `draw_until_cooldown_max_attempts` is reached.
- Behavior: every script run builds a per-account summary and sends it to Hermes Feishu home channel when `notify_feishu` is enabled.
- GREEN: PowerShell parser returned `PARSE_OK`.
- GREEN: Hermes Feishu config load confirmed platform enabled and home channel present.
- GREEN: direct Hermes `send_message_tool` Feishu test returned `success=true` and a Feishu `message_id`.
- Not run: full script execution, because it would perform real `share_unlock` and `draw` operations.

## 2026-06-19 23:30:13 +08:00 Devil - OpenAI 502 cooldown rule and scheduler probe fix
- Scope: `backend/internal/service/ratelimit_service.go`, `backend/internal/service/openai_scheduler_exhaustion_probe.go`, and focused service tests.
- RED: `go test -tags unit ./internal/service -run 'TestHandleUpstreamError_UnifiedErrorHandlingRules/openai_5xx_temp_unschedulable_rules_override_state_skip' -count=1` failed because OpenAI 502 was still skipped by `openai_account_state_mutation_skipped`.
- RED: `go test ./internal/service -run 'TestOpenAISchedulerExhaustionProbeSkipsTempCoolingAccount' -count=1` failed because a temp-cooling account was still selected and recovered.
- Fix: `shouldSkipOpenAIAccountStateMutation` now lets explicit legacy `temp_unschedulable_rules` handle matching OpenAI 5xx responses; `isOpenAISchedulerExhaustionProbeCandidate` now excludes accounts whose `temp_unschedulable_until` is still in the future.
- GREEN: `go test -tags unit ./internal/service -run 'TestHandleUpstreamError_UnifiedErrorHandlingRules/openai_5xx_temp_unschedulable_rules_override_state_skip' -count=1`.
- GREEN: `go test ./internal/service -run 'TestOpenAISchedulerExhaustionProbeSkipsTempCoolingAccount' -count=1`.
- GREEN: `go test -tags unit ./internal/service -run 'Test(CheckErrorPolicy|HandleUpstreamError_UnifiedErrorHandlingRules|AccountGetErrorHandlingRules_NormalizesUnifiedSchema)' -count=1`.
- GREEN: `go test ./internal/service -run 'TestOpenAISchedulerExhaustionProbe' -count=1`.
- GREEN: `git diff --check -- backend/internal/service/ratelimit_service.go backend/internal/service/openai_scheduler_exhaustion_probe.go backend/internal/service/error_policy_test.go backend/internal/service/openai_scheduler_exhaustion_probe_test.go`.
- RED unrelated: `go test ./internal/service -count=1` still fails outside this slice in `TestAccountDisableAPIKeyWritesDisabledUntilAndCount` timing expectations and `TestOpenAIGatewayService_Forward_WSv2ErrorEventUsageLimitPersistsRateLimit` nil-pointer behavior in a test stub.
- Current state: not committed, not built into an image, not deployed.

## 2026-06-19 23:52:13 +08:00 Devil - v0.1.136.8 OpenAI 502 cooldown release
- Scope: committed OpenAI 5xx temp-unschedulable rule fix and scheduler probe guard from `4d1a741d14e1`.
- Build: `git archive --format=tar HEAD | docker build --pull=false -t sub2api:v0.1.136.8 ... -` succeeded from committed HEAD; image label version is `v0.1.136.8`, revision is `4d1a741d14e1`.
- Image: `sub2api:v0.1.136.8`, image ID `sha256:671656df0fe2bc2addc307ed8ef13ea63c66866a7e223610b3ef8c45fcc5aae0`.
- Pre-release state: active blue `sub2api:v0.1.136.7`; idle green `sub2api:v0.1.136.6`.
- Candidate deploy: recreated only `sub2api-green` with `sub2api:v0.1.136.8`; PostgreSQL, Redis, proxy, and active blue were not restarted.
- GREEN candidate: `sub2api-green` `Health=healthy`, `RestartCount=0`, image `sub2api:v0.1.136.8`.
- GREEN candidate smoke: `http://127.0.0.1:18082/health` 200, homepage 200, first JS static asset 200, unauth `/api/v1/admin/accounts` 401, unauth `/api/v1/admin/system/version` 401, unauth `/responses` 401, unauth `/v1/responses` 401.
- Note: initial 2-minute candidate log scan saw one startup cleanup cancellation line `pq: canceling statement due to user request`; follow-up 70-second candidate critical log window had hit count 0.
- Cutover: changed `D:\sub2api-deploy\proxy\upstreams\active.conf` from `sub2api-blue:8080` to `sub2api-green:8080`; `docker exec sub2api-proxy nginx -t` passed and `docker exec sub2api-proxy nginx -s reload` succeeded.
- GREEN post-cutover smoke: `8080` and `18081` `/health` 200, homepage 200, unauth `/api/v1/admin/system/version` 401, unauth `/responses` 401, unauth `/v1/responses` 401.
- GREEN post-cutover logs: 70-second green critical log scan hit count 0.
- Current state: active green `sub2api:v0.1.136.8`; rollback blue `sub2api:v0.1.136.7` remains healthy.
- Not run: authenticated admin version check, because no admin JWT was available and no login bypass/database mutation was used.

## 2026-06-20 13:36:27 +08:00 Devil - free-rawchat Codex Accept override fix
- Scope: local code fix only; not committed, not built into an image, not deployed.
- Root cause: API Key Codex simulation inherited client `Accept: */*` through `openaiAllowedHeaders`; `ensureOpenAICodexClientMetadataHeaders` only set `text/event-stream` when the header was empty, so Codex simulation could send the wrong Accept value upstream.
- Fix: `ensureOpenAICodexClientMetadataHeaders` now always sets `Accept: text/event-stream` for Codex simulation metadata/header preparation.
- RED: `go test -tags unit ./internal/service -run TestOpenAIBuildUpstreamRequestAPIKeyCodexSimulationOverridesWildcardAccept -count=1` failed with actual `Accept: */*`.
- GREEN: `go test -tags unit ./internal/service -run 'TestOpenAIBuildUpstreamRequestAPIKeyCodexSimulationUsesCodexProviderResponsesPath|TestOpenAIBuildUpstreamRequestAPIKeyCodexSimulationUsesV1ResponsesForBareHost|TestOpenAIBuildUpstreamRequestAPIKeyCodexSimulationOverridesWildcardAccept|TestOpenAIBuildUpstreamRequestAPIKeyCodexSimulationPreservesRealCodexClientHeaders|TestOpenAIBuildUpstreamRequestAccountCodexSimulationHeaders|TestAccountProbeService_RunOpenAIAPIKeyCodexSimulationUsesCodexHeaders|TestAccountTestService_OpenAIAPIKeyResponsesTestUsesGatewayCodexSimulationHeaders|TestAccountTestService_OpenAIAPIKeyChatCompletionsTestUsesGatewayCodexSimulationHeaders|TestForwardAsChatCompletions_APIKeyCodexSimulationUsesTLSProfile|TestForwardAsRawChatCompletions_UsesCodexSimulationHeaders|TestForwardResponses_ForceChatCompletionsUsesCodexSimulationHeaders' -count=1`.
- GREEN: `git diff --check` exited 0 with only CRLF normalization warnings for already dirty files.
- LIMIT: `go test -tags unit ./internal/service -count=1` timed out after 124 seconds via the tool wrapper and did not emit failure details.

## 2026-06-20 13:51:20 +08:00 Devil - release v0.1.136.9
- Commit: `a65044b5a370b4f0b0db9e1caefbdd203005630a` (`fix(openai): 修复 Codex 模拟 Accept 覆盖`).
- Build: `sub2api:v0.1.136.9` from committed HEAD via `git archive --format=tar HEAD | docker build --pull=false ... -`.
- Image: `sha256:53c16df84bcb450e8aee9e193b813ccf25746db6d965cf0441274e489289e4e0`; labels version/revision are `v0.1.136.9` / `a65044b5a370`.
- Binary: `docker run --rm sub2api:v0.1.136.9 /app/sub2api -version` reported `Sub2API 0.1.136 (image: v0.1.136.9, commit: a65044b5a370, built: 2026-06-20T05:43:51Z)`.
- Pre-release state: active green `sub2api:v0.1.136.8`; rollback blue `sub2api:v0.1.136.7`.
- Candidate deploy: recreated only `sub2api-blue` with `sub2api:v0.1.136.9`; active green, PostgreSQL, Redis, and proxy were not restarted during candidate deployment.
- GREEN candidate: `sub2api-blue` `Health=healthy`, `RestartCount=0`, image `sub2api:v0.1.136.9`.
- GREEN candidate smoke: `http://127.0.0.1:18083/health` 200, homepage 200, static JS 200, unauth `/api/v1/admin/accounts` 401, unauth `/responses` 401, unauth `/v1/responses` 401.
- GREEN candidate logs: 60-second health window stayed healthy and recent 2-minute critical log scan hit count 0.
- Cutover: changed `D:\sub2api-deploy\proxy\upstreams\active.conf` from `sub2api-green:8080` to `sub2api-blue:8080`; `docker exec sub2api-proxy nginx -t` passed and reload succeeded.
- Compose sync: `D:\sub2api-deploy\docker-compose.blue.yml` default image updated to `sub2api:v0.1.136.9`.
- GREEN post-cutover smoke: `8080` and `18081` `/health` 200, homepage 200, static JS 200, unauth `/api/v1/admin/system/version` 401, unauth `/responses` 401, unauth `/v1/responses` 401.
- GREEN post-cutover logs: 65-second health window stayed healthy; 90-second blue critical log scan hit count 0; proxy error log scan hit count 0.
- Current state: active blue `sub2api:v0.1.136.9`; rollback green `sub2api:v0.1.136.8` remains healthy.
- Not run: authenticated admin version/account 470 test, because `D:\sub2api-deploy\.env` has no `ADMIN_PASSWORD`; no JWT/database bypass was used.

## 2026-06-20 19:10:00 +08:00 Devil - release v0.1.136.10
- Commit: `6162f1969290fb0f854db501a54f43a641cf346a` (`fix(openai): 修复 Codex 模拟安装标识与版本头`).
- GREEN: `go test -tags unit ./internal/service -run 'TestApplyCodexCLISimulationClientMetadata_APIKeyAccountAddsStableInstallationID|TestApplyOpenAICodexLatestClientHeadersMatchesCapturedClientShape|TestAccountTestService_OpenAIAPIKeyResponsesTestUsesGatewayCodexSimulationHeaders|TestAccountTestService_OpenAIAPIKeyChatCompletionsTestUsesGatewayCodexSimulationHeaders|TestOpenAIGatewayService_APIKeyCodexCLISimulation_ForcesHeadersAndPreservesCodexFields|TestOpenAIGatewayService_APIKeyPassthroughCodexCLISimulation_ForcesHeaders|TestOpenAIBuildUpstreamRequestAccountCodexSimulationHeaders|TestOpenAIBuildUpstreamRequestAPIKeyCodexSimulationOverridesWildcardAccept|TestOpenAIBuildUpstreamRequestAPIKeyCodexSimulationUsesCodexProviderResponsesPath|TestOpenAIBuildUpstreamRequestAPIKeyCodexSimulationUsesV1ResponsesForBareHost|TestOpenAIBuildUpstreamRequestAPIKeyCodexSimulationPreservesRealCodexClientHeaders' -count=1` 通过。
- GREEN: 依据当前代码派生 installation_id `81b4ba42-4573-4eff-943c-4c57c5d55b9a` 运行 `go run .\cmd\codex-live-probe`，对 `https://new.sharedchat.cc/codex/responses` 返回 `status=200`、`protocol_mode=openai_h1`。
- GREEN: `docker image inspect sub2api:v0.1.136.10` 显示 version/revision 为 `v0.1.136.10` / `6162f1969290`。
- GREEN: `docker run --rm sub2api:v0.1.136.10 /app/sub2api -version` 输出 `Sub2API 0.1.136 (image: v0.1.136.10, commit: 6162f1969290, built: 2026-06-20T11:04:07Z)`。
- OBSERVE: 第一次重建 green 候选后发现 `D:\sub2api-deploy\docker-compose.green.yml` 默认镜像仍是 `sub2api:v0.1.136.8`；修正为 `sub2api:v0.1.136.10` 后重新重建 green。
- GREEN: green 候选 `18082` `/health` 200、首页 200、静态资源 200、未登录 `/api/v1/admin/accounts` 401、未登录 `/responses` 401、未登录 `/v1/responses` 401。
- OBSERVE: 候选初始关键日志窗口命中 1 条 `pq: canceling statement due to user request`，定位为 `openai_request_snapshot` 启动后清理任务；后续 70 秒 candidate 关键日志窗口命中 0。
- GREEN: `docker exec sub2api-proxy nginx -t` 通过，`docker exec sub2api-proxy nginx -s reload` 成功。
- GREEN: 切流后 `8080` 与 `18081` `/health` 200、首页 200、`8080` 静态资源 200、未登录 `/api/v1/admin/system/version` 401、未登录 `/responses` 401、未登录 `/v1/responses` 401。
- GREEN: 切流后 75 秒健康窗口保持 healthy；90 秒 green 关键日志命中 0，proxy 错误日志命中 0。
- Current state: active green `sub2api:v0.1.136.10`; rollback blue `sub2api:v0.1.136.9` remains healthy.
- Not run: authenticated admin version/account 470 test, because `D:\sub2api-deploy\.env` has no `ADMIN_PASSWORD`; no JWT/database bypass was used.

## 2026-06-21 11:02:00 +08:00 Devil - release v0.1.136.12 free-rawchat Codex client header fix
- Commit: `92841900f3570a49082862b2662c33a7f392fd70` (`fix(openai): 强制 Codex 模拟使用最新版客户端头`).
- GREEN: `go test -tags unit ./internal/service -run TestOpenAIBuildUpstreamRequestAPIKeyCodexSimulationOverridesOutdatedRealCodexClientHeaders -count=1` passed.
- GREEN: focused Codex simulation test set passed, covering outdated real Codex client headers, Accept override, Codex provider responses path, bare host `/v1/responses`, API Key passthrough, and latest captured client shape.
- GREEN: `git diff --check -- backend/internal/service/openai_gateway_service.go backend/internal/service/openai_gateway_service_test.go` passed.
- GREEN: `docker image inspect sub2api:v0.1.136.12` reported version/revision `v0.1.136.12` / `92841900f357`.
- GREEN: `docker run --rm sub2api:v0.1.136.12 /app/sub2api -version` reported `Sub2API 0.1.136 (image: v0.1.136.12, commit: 92841900f357, built: 2026-06-21T02:53:28Z)`.
- GREEN: only idle `sub2api-blue` was recreated with `sub2api:v0.1.136.12`; active green, PostgreSQL, Redis, and proxy were not restarted during candidate deployment.
- GREEN: blue candidate `18083` health/home/static/unauth admin/unauth `/responses`/unauth `/v1/responses` smoke passed; 65-second health window stayed healthy and critical log hits were 0.
- GREEN: switched `D:\sub2api-deploy\proxy\upstreams\active.conf` from `sub2api-green:8080` to `sub2api-blue:8080`; `docker exec sub2api-proxy nginx -t` passed and reload succeeded.
- GREEN: post-cutover `8080` and `18081` health/home/static/unauth admin/unauth responses smoke passed.
- GREEN: account `470/free-rawchat` was restored to `status=active`, `schedulable=true`, and empty error/temp-unschedulable fields.
- GREEN: real public gateway request `request_id=free-rawchat-verify-20260621110046` used old `Codex Desktop/0.140.0` / `Version: 0.140.0` headers against `POST http://127.0.0.1:8080/v1/responses` and returned HTTP 200; access log confirmed `account_id=470`, `status_code=200`, `model=gpt-5.5`; full SSE contained `response.completed` with text `Hi!`.
- GREEN: after the real request, account 470 remained `active/schedulable=true`; blue recent problem log hits were 0 and proxy recent problem log hits were 0.
- Current state: active blue `sub2api:v0.1.136.12`; rollback green `sub2api:v0.1.136.10` remains healthy.
- Not run: authenticated admin system version API check, because `ADMIN_PASSWORD` is empty; version evidence came from image labels, binary `-version`, and running container image.

## 2026-06-21 18:xx:00 +08:00 Devil - Codex Desktop 模拟头对齐
- GREEN: ackend/internal/pkg/openai/codex_version_fetcher.go 将 Codex Desktop 模拟 UA 尾段从重复 CLI 版本号改为真实桌面 build 26.616.32156。
- GREEN: go test ./internal/pkg/openai ./internal/service -run "TestCodexCLIVersionFetcherAcceptsNpmLatestEndpoint|TestCodexCLIUserAgentForVersionMatchesCapturedDesktopShape|TestOpenAICodexCLISimulationUsesLatestClientVersion" -count=1 passed.
- GREEN: go test ./internal/service -run "TestApplyOpenAICodexLatestClientHeadersMatchesCapturedClientShape|TestOpenAIBuildUpstreamRequestAPIKeyCodexSimulationOverridesOutdatedRealCodexClientHeaders|TestAccountTestService_OpenAIAPIKeyResponsesTestUsesGatewayCodexSimulationHeaders|TestAccountTestService_OpenAIAPIKeyChatCompletionsTestUsesGatewayCodexSimulationHeaders|TestOpenAIGatewayService_BuildOpenAIWSHeadersAccountCodexSimulationUsesLatestCodexDesktopHeaders|TestOpenAIGatewayService_BuildOpenAIWSHeadersAccountCodexSimulationPreservesRealCodexClientHeaders" -count=1 passed.
- GREEN: API Key Codex 模拟链路、账号测试链路和 WS 头构造都已切到新 Desktop UA 形状；真实客户端透传分支未改。
- LIMIT: not committed, not built, not deployed.

## 2026-06-21 18:xx:30 +08:00 Devil - Codex Desktop build 位动态学习
- GREEN: ackend/internal/pkg/openai/codex_version_fetcher.go 新增真实 Desktop UA build 提取、观察和进程内缓存；模拟 UA 改为读取当前已学习 build。
- GREEN: ackend/internal/service/openai_gateway_service.go 在识别到真实 Codex 客户端请求时先观察其 Desktop UA，再继续 API Key 模拟头构造。
- GREEN: go test ./internal/pkg/openai ./internal/service -run "TestCodexCLIVersionFetcherAcceptsNpmLatestEndpoint|TestCodexCLIUserAgentForVersionMatchesCapturedDesktopShape|TestObserveCodexDesktopUserAgentUpdatesSyntheticBuildSuffix|TestObserveCodexDesktopUserAgentIgnoresUnsupportedShape|TestOpenAICodexCLISimulationUsesLatestClientVersion" -count=1 passed.
- GREEN: go test ./internal/service -run "TestOpenAIBuildUpstreamRequestAPIKeyCodexSimulationLearnsDesktopBuildFromRealClientUA|TestOpenAIBuildUpstreamRequestAPIKeyCodexSimulationOverridesOutdatedRealCodexClientHeaders|TestApplyOpenAICodexLatestClientHeadersMatchesCapturedClientShape|TestOpenAIGatewayService_BuildOpenAIWSHeadersAccountCodexSimulationUsesLatestCodexDesktopHeaders|TestOpenAIGatewayService_BuildOpenAIWSHeadersAccountCodexSimulationPreservesRealCodexClientHeaders" -count=1 passed.
- LIMIT: only process-local cache; not persisted across restart, not committed, not deployed.

## 2026-06-21 20:00:00 +08:00 Devil - release v0.1.136.13 dynamic Codex Desktop build learning
- Commit:  4e9a2775410bde8cb29e6e2b2404841cb2e8f0d (ix(openai): 动态学习 Codex Desktop build 位).
- GREEN: go test ./internal/pkg/openai ./internal/service -run "TestCodexCLIVersionFetcherAcceptsNpmLatestEndpoint|TestCodexCLIUserAgentForVersionMatchesCapturedDesktopShape|TestObserveCodexDesktopUserAgentUpdatesSyntheticBuildSuffix|TestObserveCodexDesktopUserAgentIgnoresUnsupportedShape|TestOpenAICodexCLISimulationUsesLatestClientVersion|TestOpenAIBuildUpstreamRequestAPIKeyCodexSimulationLearnsDesktopBuildFromRealClientUA|TestOpenAIBuildUpstreamRequestAPIKeyCodexSimulationOverridesOutdatedRealCodexClientHeaders|TestApplyOpenAICodexLatestClientHeadersMatchesCapturedClientShape|TestOpenAIGatewayService_BuildOpenAIWSHeadersAccountCodexSimulationUsesLatestCodexDesktopHeaders|TestOpenAIGatewayService_BuildOpenAIWSHeadersAccountCodexSimulationPreservesRealCodexClientHeaders|TestAccountTestService_OpenAIAPIKeyResponsesTestUsesGatewayCodexSimulationHeaders|TestAccountTestService_OpenAIAPIKeyChatCompletionsTestUsesGatewayCodexSimulationHeaders" -count=1 passed.
- GREEN: go test ./cmd/server -run TestNoSuchTest -count=1 passed.
- GREEN: docker image inspect sub2api:v0.1.136.13 reported version/revision 0.1.136.13 /  4e9a2775410.
- GREEN: docker run --rm sub2api:v0.1.136.13 /app/sub2api -version reported Sub2API 0.1.136 (image: v0.1.136.13, commit: 04e9a2775410, built: 2026-06-21T03:54:42Z).
- GREEN: only idle green was recreated with sub2api:v0.1.136.13; active blue, PostgreSQL, Redis, and proxy were not restarted during candidate deploy.
- GREEN: candidate green 18082 health/home/static asset/unauth admin/unauth /responses/unauth /v1/responses smoke passed; later 75-second green critical log scan hit 0.
- GREEN: switched D:\sub2api-deploy\proxy\upstreams\active.conf from sub2api-blue:8080 to sub2api-green:8080; docker exec sub2api-proxy nginx -t passed and reload succeeded.
- GREEN: post-cutover 8080 and 18081 health/home/unauth admin/unauth responses smoke passed; 75-second green and proxy log scans hit 0.
- GREEN: D:\sub2api-deploy\.env now points SUB2API_GREEN_IMAGE=sub2api:v0.1.136.13; SUB2API_BLUE_IMAGE=sub2api:v0.1.136.12 remains as rollback.
- LIMIT: authenticated admin version API not run because ADMIN_PASSWORD is empty; dynamic Desktop build learning is process-local and not persisted across restart.

## 2026-06-21 20:32 +08:00 - account-level Claude CLI version override

- GREEN: Added account credential `claude_cli_version` so Anthropic and Antigravity accounts can override Claude CLI version per account; empty or invalid values fall back to the current global CLI version.
- GREEN: The account override now drives Anthropic account-test headers, session and payload generation, upstream model-sync requests, Claude OAuth default headers, Claude Code mimic headers, billing block `cc_version`, and OAuth metadata `user_id` version formatting.
- GREEN: Admin create/edit account flows now expose a `Claude CLI version` field and persist it as `credentials.claude_cli_version`.
- GREEN: `go test ./internal/service -run "TestAccountGetClaudeCLIVersion|TestGenerateSessionStringUsesAccountClaudeCLIVersion|TestAccountTestService_AnthropicAPIKeyUsesAccountClaudeCLIVersionOverride|TestRewriteSystemForNonClaudeCode|TestBuildAnthropicUpstreamModelsRequestUsesAccountClaudeCLIVersionOverride" -count=1` passed.
- GREEN: `npm --prefix frontend test -- --run AccountAPIKeyCredentialsFields.spec.ts EditAccountModal.spec.ts` passed; Vitest summary was 3 files / 40 tests passed.
- GREEN: `npm --prefix frontend run typecheck` passed.
- LIMIT: local code and focused verification only; not committed, not built, not deployed.

## 2026-06-21 20:51 +08:00 - release v0.1.136.14 account-level Claude CLI version override

- GREEN: committed feature slice `ae3e03f6317cdcb1eae6ae16cbfd62fb55692c06` (`feat(account): 支持按账号覆盖 Claude CLI 版本`).
- GREEN: `docker image inspect sub2api:v0.1.136.14` reported image ID `sha256:4ed310839f242c3a99436ef52d705ef50a50ba40ee73a3f0184fa629d8a7f9d0`, version `v0.1.136.14`, revision `ae3e03f6317c`.
- GREEN: `docker run --rm sub2api:v0.1.136.14 /app/sub2api -version` reported `Sub2API 0.1.136 (image: v0.1.136.14, commit: ae3e03f6317c, built: 2026-06-21T04:47:20Z)`.
- GREEN: only idle `sub2api-blue` was recreated with `sub2api:v0.1.136.14`; active green, PostgreSQL, Redis, and proxy were not restarted during candidate deployment.
- GREEN: blue candidate `18083` health/home/static/unauth admin/unauth `/responses`/unauth `/v1/responses` smoke passed; 65-second health window stayed healthy.
- OBSERVE: blue candidate log window hit 1 known `openai_request_snapshot` cleanup noise row: `pq: canceling statement due to user request`; no panic/fatal, migration failure, bind conflict, or 502 followed.
- GREEN: switched `D:\sub2api-deploy\proxy\upstreams\active.conf` from `sub2api-green:8080` to `sub2api-blue:8080`; `docker exec sub2api-proxy nginx -t` passed and reload succeeded.
- GREEN: post-cutover `8080` and `18081` health/home/unauth admin/unauth responses smoke passed.
- GREEN: after cutover, blue 75-second health window stayed healthy; proxy recent problem log hits were 0.
- Current state: active blue `sub2api:v0.1.136.14`; rollback green `sub2api:v0.1.136.13` remains healthy.
- Not run: authenticated admin system version API check and real logged-in Anthropic business request, because `ADMIN_PASSWORD` is empty and this slice relied on focused tests plus public boundary smoke.

## 2026-06-21 21:34 +08:00 - release v0.1.136.15 Claude CLI i18n key-level fix

- GREEN: committed fix slice `968843d3409b8b50b808e5cf3874c6dd5a2287e3` (`fix(i18n): 修正 Claude CLI 版本词条层级`).
- GREEN: `npm --prefix frontend run typecheck` passed before build.
- GREEN: `docker image inspect sub2api:v0.1.136.15` reported image ID `sha256:a47c9b99947272207e96819bda8f1bf0721742d5c1d628b9b7f12a30fdc8c3a4`, version `v0.1.136.15`, revision `968843d3409b`.
- GREEN: `docker run --rm sub2api:v0.1.136.15 /app/sub2api -version` reported `Sub2API 0.1.136 (image: v0.1.136.15, commit: 968843d3409b, built: 2026-06-21T05:30:25Z)`.
- GREEN: only idle `sub2api-green` was recreated with `sub2api:v0.1.136.15`; active blue, PostgreSQL, Redis, and proxy were not restarted during candidate deployment.
- GREEN: green candidate `18082` health/home/static/unauth admin/unauth `/responses`/unauth `/v1/responses` smoke passed; static asset `/assets/index-CvksB-nV.js` returned 200.
- GREEN: green candidate 65-second health window stayed healthy.
- OBSERVE: green candidate log window hit 1 known `openai_request_snapshot` cleanup noise row: `pq: canceling statement due to user request`; no panic/fatal, migration failure, bind conflict, or 502 followed.
- GREEN: switched `D:\sub2api-deploy\proxy\upstreams\active.conf` from `sub2api-blue:8080` to `sub2api-green:8080`; `docker exec sub2api-proxy nginx -t` passed and reload succeeded.
- GREEN: post-cutover `8080` and `18081` health/home/unauth admin/unauth responses smoke passed.
- GREEN: after cutover, green 75-second health window stayed healthy; proxy recent problem log hits were 0.
- Current state: active green `sub2api:v0.1.136.15`; rollback blue `sub2api:v0.1.136.14` remains healthy.
- Not run: authenticated admin UI screenshot/version check because `ADMIN_PASSWORD` is empty; verification used static asset, code path, and typecheck evidence.

## 2026-06-22 16:12 +08:00 - OpenAI 客户端中断与 free-观澜 调查

- PASS：当前入口代理走 `sub2api-green:8080`，active 容器健康；旧 `sub2api` 重启不在入口链路。
- PASS：账号 461/free-观澜 在调查窗口没有 `openai.forward_failed`，14 条 `/responses` usage 均完成并计费。
- PASS：账号 461 在 15:41:56 由管理接口改为不可调度，不是自动失败降级。
- PASS：实际可见失败集中在账号 483/479 的 `context canceled` 或 `stream usage incomplete: context canceled`，并且 handler 已尝试写 Responses 协议 `response.failed` fallback。
- PASS：当前主要风险是自用组可调度 OpenAI 账号不足，导致 `scheduler_exhaustion_probe_failed: no available accounts` 后被客户端取消。
- RISK：缺少用户本机客户端的精确报错时间/request_id；若用户看到的是某条具体 461 请求中断，需要拿该时间或 request_id 再做一对一关联。

## 2026-06-23 10:12 +08:00 - 多 Key 429/余额不足不冻结账号

- PASS：多 Key OpenAI API Key 账号只剩最后一个可用 Key 时，429 现在写入该 Key 的 `api_keys_disabled` 冷却记录，不写账号级 `SetTempUnschedulable`、`SetRateLimited` 或 `SetError`。
- PASS：多 Key OpenAI API Key 账号只剩最后一个可用 Key 时，403 `insufficient balance` 现在写入该 Key 的 `api_keys_disabled` 冷却记录，不触发账号级临时不可调度规则，也不冻结账号。
- PASS：相关 selected-key cooldown 聚焦测试通过：`go test -tags unit ./internal/service -run "TestRateLimitService_HandleUpstreamError_OpenAIAPIKey429DisablesLastActiveKeyWithoutFreezingAccount|TestRateLimitService_HandleUpstreamError_OpenAI403InsufficientBalanceDisablesLastActiveKeyWithoutFreezingAccount|TestRateLimitService_HandleUpstreamError_OpenAI403InsufficientBalanceDisablesSelectedKey|TestHandleUpstreamError429_OpenAIAPIKey(DisablesSelectedKey|WithTempRulesDisablesSelectedKey)|TestAccountTestService_OpenAI(APIKeyInsufficientBalanceDisablesSelectedKey|ChatCompletionsPathDisablesSelectedKey|CompactPathDisablesSelectedKey|ImagePathDisablesSelectedKey)" -count=1`。
- PASS：`go test ./cmd/server -run TestNoSuchTest -count=1` 编译切片通过。
- FAIL-UNRELATED：`go test -tags unit ./internal/service -count=1 -timeout 8m` 仍失败，失败点不在本次三文件 diff 范围：`TestAccountDisableAPIKeyWritesDisabledUntilAndCount` 的旧间隔断言不匹配；`TestOpenAIGatewayService_Forward_WSv2ErrorEventUsageLimitPersistsRateLimit` 在账号级调度路径 panic。
- RISK：本轮未构建镜像、未部署；线上仍需后续按蓝绿发布流程验证真实多 Key 请求行为。

## 2026-06-23 10:29 +08:00 - 人工测试 Key 报错与冷却展示

- PASS：账号 manual-probe JSON 响应现在返回 DTO 账号，包含每个 key 的 `api_key_items.last_error`、`reason`、`disabled_until`，并继续移除原始 `credentials.api_keys`；验证命令 `go test ./internal/handler/admin -run TestAccountHandler_ManualProbeReturnsAPIKeyItemsWithCoolingError -count=1` 通过。
- PASS：429 与 403 insufficient balance 的 selected-key cooldown 记录会持久化 `last_error`；验证命令分别为 `go test -tags unit ./internal/service -run TestRateLimitService_HandleUpstreamError_OpenAIAPIKey429DisablesLastActiveKeyWithoutFreezingAccount -count=1` 和 `go test -tags unit ./internal/service -run TestRateLimitService_HandleUpstreamError_OpenAI403InsufficientBalanceDisablesLastActiveKeyWithoutFreezingAccount -count=1`，均通过。
- PASS：manual-probe handler 聚焦用例 `go test ./internal/handler/admin -run "TestAccountHandler_ManualProbe" -count=1` 通过。
- PASS：前端 `AccountTestModal` 会在测试结束后通知列表页刷新账号；调度池 manual probe 失败响应后也会刷新池子；Key 状态组件显示 `last_error`。验证命令 `corepack pnpm vitest run src/views/admin/__tests__/AccountSchedulingPoolView.spec.ts src/components/admin/account/__tests__/AccountTestModal.spec.ts src/components/account/__tests__/EditAccountModal.spec.ts src/views/admin/__tests__/AccountsView.bulkEdit.spec.ts` 通过，4 个文件 49 个测试通过。
- PASS：`corepack pnpm typecheck` 通过；`go test ./internal/service -run '^$' -count=1` service 包编译校验通过。
- WARN：前端 Vitest 输出仍有既有 `common.time.never` i18n 缺 key 警告和 Browserslist 数据过期提示。
- LIMIT：`go test ./internal/service -count=1` 完整 service 包测试 124 秒超时，未作为通过证据；本轮未构建镜像、未部署、未切流。

## 2026-06-23 11:35 +08:00 - release v0.1.136.16 per-key manual probe visibility

- GREEN: build source is committed HEAD 6118358ff43a; image sub2api:v0.1.136.16 has ID sha256:624f6ca00c017e58457b417fc23c3e68bb9b7b24268d6f6424f4dce0fa066bcd, label version v0.1.136.16, revision 6118358ff43a.
- GREEN: binary version check printed Sub2API 0.1.136 (image: v0.1.136.16, commit: 6118358ff43a, built: 2026-06-23T03:07:54Z).
- GREEN: only idle sub2api-blue was recreated with sub2api:v0.1.136.16; active sub2api-green, PostgreSQL, Redis, and sub2api-proxy were not restarted during candidate deploy.
- GREEN: candidate blue 18083 health/home/static asset/unauth admin/unauth /responses/unauth /v1/responses smoke passed; static asset /assets/index-DI9h4SpI.js returned 200.
- GREEN: candidate blue stayed healthy after a 60+ second health window; docker logs sub2api-blue --since 5m had no critical matches for panic/fatal/migration/checksum/pq/bind/listen/rebuild patterns.
- GREEN: switched D:\sub2api-deploy\proxy\upstreams\active.conf from sub2api-green:8080 to sub2api-blue:8080; docker exec sub2api-proxy nginx -t passed and reload succeeded.
- GREEN: post-cutover public 8080 and local proxy 18081 health/home/static/unauth admin/unauth /responses/unauth /v1/responses smoke passed.
- GREEN: after a 70-second post-cutover observation window, sub2api-blue and sub2api-green were both healthy; 8080, 18081, and 18083 /health all returned 200; blue/proxy critical log matches were 0.
- Current state: active blue sub2api:v0.1.136.16; rollback green sub2api:v0.1.136.15 remains healthy.
- LIMIT: ADMIN_PASSWORD is empty in D:\sub2api-deploy\.env, so authenticated /api/v1/admin/system/version verification was not run. Unauthenticated /api/v1/admin/system/version returned 401 as expected; version evidence came from image labels, binary version output, frontend route availability, and the running container image.

## 2026-06-23 12:14 +08:00 - release v0.1.136.17 scheduler exhaustion stream cap

- PASS: incident evidence shows the 11:24 blank client symptom was scheduler exhaustion for `gpt-5.5`, not a process crash. Active `sub2api-blue:v0.1.136.16` had `RestartCount=0`, and the 11:23:30-11:25:30 log window had no panic/fatal/migration/bind/listen failures.
- PASS: logs showed `openai.account_select_failed: no available OpenAI accounts supporting model: gpt-5.5`, `openai.scheduler_exhaustion_probe_failed: no available accounts`, and selected-key cooldowns for account `486` after upstream 429. Account `461/free-观澜` was `schedulable=false` with `invalid_api_key` and had no 11:20-11:30 usage rows.
- PASS: `go test ./internal/handler -run "TestOpenAIScheduler(ExhaustionProbeMode|ProbePendingUsesSSEComment|ProbeClientWaitDeadline)$" -count=1` passed.
- PASS: `go test ./internal/handler -run "TestOpenAIHandleStreamingAwareError|TestGatewayHandleStreamingAwareError|TestOpenAIRecoverResponsesPanic|TestOpenAIScheduler|TestOpenAIEnsureForwardErrorResponse" -count=1` passed.
- PASS: `go test ./cmd/server -run TestNoSuchTest -count=1` passed; `git diff --check -- backend/internal/handler/openai_gateway_handler.go backend/internal/handler/openai_gateway_handler_test.go` passed.
- PASS: image `sub2api:v0.1.136.17` was built from committed HEAD `da3bc79f09a0`; label and binary version checks matched `v0.1.136.17` and commit `da3bc79f09a0`.
- PASS: only idle `sub2api-green` was recreated with `sub2api:v0.1.136.17`; active blue, PostgreSQL, Redis, and proxy were not restarted during candidate deploy.
- PASS: candidate green `18082` health/home/static asset/unauth admin/unauth `/responses`/unauth `/v1/responses` checks passed; 60+ second candidate health stayed healthy with `RestartCount=0`.
- OBSERVE: candidate green log window had one known `openai_request_snapshot` cleanup noise row: `pq: canceling statement due to user request`; no critical panic/fatal/migration/bind/listen pattern followed.
- PASS: proxy switched from `sub2api-blue:8080` to `sub2api-green:8080`; nginx config test and reload passed.
- PASS: post-cutover public `8080` and local proxy `18081` smoke checks passed; Codex-style live probe returned HTTP 200 and showed `: scheduler_probe_pending`, keepalives, then `event: response.created`.
- PASS: 70-second post-cutover observation kept green and blue healthy; `8080`, `18081`, and `18082` `/health` returned 200; green/proxy critical log scans were clean.
- Current state: active green `sub2api:v0.1.136.17`; rollback blue `sub2api:v0.1.136.16` remains healthy.
- LIMIT: full repository test suite and authenticated admin version API were not run. This was a focused handler fix verified by targeted tests, image checks, candidate/public smokes, live probe, and runtime log observation.

## 2026-06-23 12:39 +08:00 - 上游错误写入对应 API Key

- PASS：账号人工测试连接收到 OpenAI API Key 上游 503 时，manual-probe JSON 响应返回 DTO 账号，`api_key_items[0].last_error` 包含 `API returned 503` 与上游响应体，且不泄露 `credentials.api_keys`。
- PASS：账号人工测试连接收到非典型上游 HTTP 错误 529 时，也会把错误写入本次选中 Key 的 `api_keys_disabled` 冷却记录，前端刷新后可在对应 key 行看到 `last_error`、`reason`、`disabled_until`。
- PASS：调度池/后台账号探测收到上游 503 或非典型 HTTP 错误 418 时，会通过样本里的 key fingerprint 反查对应 Key 并写入 `api_keys_disabled`，用于管理端定位不可用 key。
- PASS：聚焦验证 `rtk go test -tags unit ./internal/service -run "TestAccountTestService_OpenAIManualTestRecordsAnyUpstreamHTTPErrorOnSelectedKey|TestAccountProbeService_RunRecordsAnyUpstreamHTTPErrorOnSelectedKey|TestAccountProbeService_RunRecordsSelectedAPIKeyError|TestRateLimitService_HandleUpstreamError_OpenAIAPIKey503RecordsSelectedKeyError" -count=1` 通过，4 个测试通过。
- PASS：manual-probe handler 聚焦验证 `rtk go test ./internal/handler/admin -run "TestAccountHandler_ManualProbeReturnsAPIKeyItemsWithUpstream503Error|TestAccountHandler_ManualProbeReturnsAPIKeyItemsWithCoolingError" -count=1` 通过，2 个测试通过。
- PASS：更宽 selected-key 回归 `rtk go test -tags unit ./internal/service -run "TestRateLimitService_HandleUpstreamError_OpenAIAPIKey429DisablesLastActiveKeyWithoutFreezingAccount|TestRateLimitService_HandleUpstreamError_OpenAIAPIKey503RecordsSelectedKeyError|TestRateLimitService_HandleUpstreamError_OpenAI403InsufficientBalanceDisablesLastActiveKeyWithoutFreezingAccount|TestRateLimitService_HandleUpstreamError_OpenAIAPIKeyForbiddenInvalidKeyUsesSchedulingCooldown|TestHandleUpstreamError429_OpenAIAPIKeySchedulingCooldownUsesSteppedErrorCount|TestAccountProbeService_RunRecordsAccountProbeOutcomeFailure|TestAccountProbeService_RunRecordsSelectedAPIKeyError|TestAccountProbeService_RunRecordsAnyUpstreamHTTPErrorOnSelectedKey|TestAccountTestService_OpenAIManualTestRecordsAnyUpstreamHTTPErrorOnSelectedKey" -count=1` 通过，9 个测试通过。
- PASS：admin handler 回归 `rtk go test ./internal/handler/admin -run "TestAccountHandler_ManualProbeReturnsAPIKeyItemsWithCoolingError|TestAccountHandler_ManualProbeReturnsAPIKeyItemsWithUpstream503Error|TestAccountHandler_ManualProbeReturnsJSONWithoutSSE|TestAccountHandler_ManualProbeRepairsSchedulingPoolState" -count=1` 通过，4 个测试通过。
- PASS：`rtk go test ./internal/handler/admin -count=1` 通过，201 个测试通过；`rtk go test ./cmd/server -run TestNoSuchTest -count=1` 完成 server 编译切片；`git diff --check` 通过。
- WARN：CodeGraph MCP 本轮继续返回 `Transport closed`，`.codegraph/daemon.log` 显示 daemon 监听与 `socket error: write EPIPE`，已按项目规则降级 PowerShell 精确检索。
- LIMIT：未重跑完整 `go test -tags unit ./internal/service -count=1`，前序该包全量仍有无关历史失败；本轮以聚焦 service/admin、handler 包级和 server 编译切片作为发布前验证。

## 2026-06-23 14:04 +08:00 - release v0.1.136.18 上游错误写入 Key 状态

- PASS：代码提交 `80fdfcbd7a0c` 已构建为不可变镜像 `sub2api:v0.1.136.18`，镜像 ID `sha256:51b104d6589dd6160c47b8fbb1dfb562e884efff01d1e45dbaaef59c8fa46f1a`；镜像标签和二进制版本输出均匹配 commit `80fdfcbd7a0c`。
- PASS：只重建 idle `sub2api-blue`；发布候选期间 active `sub2api-green`、PostgreSQL、Redis 和 `sub2api-proxy` 未重启。
- PASS：blue 候选端口 `18083` 的 `/health`、首页、静态资源、未登录 admin 401、未登录 `/responses` 401、未登录 `/v1/responses` 401 均通过，候选 60+ 秒观察保持 `healthy` 且 `RestartCount=0`。
- OBSERVE：候选日志窗口出现 1 条已知 `openai_request_snapshot` 清理噪声 `pq: canceling statement due to user request`，未伴随 panic、fatal、migration、bind、listen 或 rebuild 失败。
- PASS：代理 upstream 已从 `sub2api-green:8080` 切到 `sub2api-blue:8080`；`docker exec sub2api-proxy nginx -t` 通过，reload 成功。
- PASS：切流后公网入口 `8080` 和本机代理入口 `18081` 的健康、首页、静态资源、未登录 admin 401、未登录 `/responses` 401、未登录 `/v1/responses` 401 均通过。
- PASS：2026-06-23 14:04 +08:00 复核 `8080`、`18081`、`18083` 的 `/health` 均返回 200；`sub2api-blue` 运行 `sub2api:v0.1.136.18` 且 `healthy RestartCount=0`，`sub2api-green` 运行 `sub2api:v0.1.136.17` 且 `healthy RestartCount=0`；nginx 配置测试通过。
- Current state：active blue `sub2api:v0.1.136.18`；rollback green `sub2api:v0.1.136.17`。
- LIMIT：本轮未执行 authenticated admin version endpoint，因为本地部署自动化路径没有可用 admin password；未重跑完整 `go test -tags unit ./internal/service -count=1`，原因同上方已知无关历史失败。

## 2026-06-24T15:24:54+08:00 - account test upstream busy display normalization

- PASS: go test -tags unit ./internal/service -run 'TestAccountTestService_OpenAIOverloadedMessageUsesFriendlyDisplayError|TestAccountTestService_OpenAI|TestAccountTestService_TestAccountConnectionWithResult|TestAccountTestService_OpenAIAPIKey' -count=1 passed.
- PASS: go test ./cmd/server -run TestNoSuchTest -count=1 passed, completing the server compile slice.
- PASS: git diff --check -- backend/internal/service/account_test_service.go backend/internal/service/account_test_service_openai_test.go passed.
- NOTE: The raw upstream OpenAI overloaded message is still logged as Account test error; account-test SSE display and structured result now use 上游服务繁忙，请稍后重试.
- LIMIT: No image build or deployment was performed in this turn.

## 2026-06-24 15:30 +08:00 - release v0.1.136.19 account test overloaded display

- PASS: feature commit `ea62cb19599b` was built into immutable image `sub2api:v0.1.136.19`; image ID `sha256:bc5bca75fe7bc8b70b7050341a02b629c692b1146ee146c5ffa673e2aadb942a`, image label version `v0.1.136.19`, and revision `ea62cb19599b` matched.
- PASS: binary version check printed `Sub2API 0.1.136 (image: v0.1.136.19, commit: ea62cb19599b, built: 2026-06-24T15:26:17Z)`.
- PASS: only idle `sub2api-green` was recreated with `sub2api:v0.1.136.19`; active `sub2api-blue`, PostgreSQL, Redis, and `sub2api-proxy` were not restarted during candidate deployment.
- PASS: candidate green `18082` health, home page, static asset, unauthenticated admin 401, unauthenticated `/responses` 401, and unauthenticated `/v1/responses` 401 checks passed.
- PASS: candidate green stayed `healthy` with `RestartCount=0` after a 60+ second health window and had no critical release-window log matches.
- PASS: proxy upstream changed from `sub2api-blue:8080` to `sub2api-green:8080`; `docker exec sub2api-proxy nginx -t` passed and reload succeeded at 2026-06-24 15:30 +08:00.
- PASS: post-cutover public `8080` and local proxy `18081` health, home page, static asset, unauthenticated admin 401, unauthenticated `/responses` 401, and unauthenticated `/v1/responses` 401 checks passed.
- PASS: after a 70+ second post-cutover observation window, `sub2api-green` and `sub2api-blue` were both healthy; `8080`, `18081`, and `18082` `/health` all returned 200; green/proxy critical release-window log scans were clean.
- Current state: active green `sub2api:v0.1.136.19`; rollback blue `sub2api:v0.1.136.18`.
- OBSERVE: 2026-06-25 08:48 +08:00 continuation recheck still showed active green and rollback blue healthy, with public `8080`, local proxy `18081`, and candidate `18082` `/health` returning 200. Both app containers showed `RestartCount=7`, indicating an overnight external/Docker restart after the release window.
- LIMIT: authenticated admin version endpoint was not run because `ADMIN_PASSWORD` is empty in the local deployment automation path.

## 2026-06-26T08:22:00+08:00 - OpenAI Responses 缺失终止事件不再合成完成

- PASS：新增 RED/GREEN 回归 `TestOpenAIStreamingMissingTerminalAfterOutputWritesFailedEvent`，确认上游已有 `response.output_text.delta` 但没有 `response.completed/failed/incomplete/cancelled` 时，服务端写出 `event: response.failed`，不再合成 `response.completed`。
- PASS：更新旧语义测试 `TestOpenAIStreamingUnexpectedEOFAfterOutputFailsAndRecordsPathHealthFailure`、`TestOpenAIStreamingMissingTerminalEventAfterOutputWritesFailedTerminal`、`TestOpenAIStreamingMissingTerminalEventRecordsPathHealthFailure`，路径健康 EOF 记录仍保留，客户端终止事件改为失败。
- PASS：`go test -tags unit ./internal/service -run TestOpenAIStreaming -count=1` 通过。
- PASS：`go test ./internal/handler -run TestOpenAIForwardErrorAlreadyCommunicated -count=1` 通过，服务层已写 `response.failed` 后 handler 不会再补第二个泛化失败事件。
- PASS：`go test ./cmd/server -run TestNoSuchTest -count=1` 通过，完成 server 编译切片。
- PASS：`git diff --check` 仅提示既有 `.codegraph/daemon.pid` 行尾警告；本轮触达 Go 文件无空白错误。
- LIMIT：本轮未构建镜像、未部署、未执行真实上游 OpenAI 请求；验证边界为本地聚焦单元测试和编译切片。

## 2026-06-26T09:03:13+08:00 - OpenAI Codex CLI User-Agent 账号级配置

- PASS：新增账号凭据键 `openai_codex_cli_user_agent`；OpenAI Codex CLI 模拟请求优先使用账号配置的 User-Agent，空值继续回退动态 Codex Desktop 形态。
- PASS：HTTP `/responses` 模拟头与 OpenAI websocket 模拟头都改为读取账号级 User-Agent；账号测评 OpenAI API Key Responses 路径也覆盖了配置化 UA。
- PASS：管理端创建/编辑 OpenAI 账号时可填写 Codex CLI User-Agent；API Key 账号在凭据区配置，OAuth 账号在 Codex 模拟设置区配置；清空后删除 credentials 字段并恢复动态默认。
- PASS：`go test -tags unit ./internal/service -run "TestAccount_GetOpenAICodexCLIUserAgent|TestAccountTestService_OpenAIAPIKeyResponsesTestUsesConfiguredCodexCLIUserAgent|TestAccountTestService_OpenAIAPIKeyResponsesTestUsesGatewayCodexSimulationHeaders|TestAccountTestService_OpenAIAPIKeyChatCompletionsTestUsesCodexSimulationHeaders" -count=1` 通过。
- PASS：`corepack pnpm vitest run src/components/account/__tests__/AccountAPIKeyCredentialsFields.spec.ts src/components/account/__tests__/EditAccountModal.spec.ts` 通过，2 个文件 30 个测试。
- PASS：`corepack pnpm typecheck`、`go test ./cmd/server -run TestNoSuchTest -count=1`、`git diff --check -- <本轮触达文件>` 均通过。
- LIMIT：本轮未构建镜像、未部署、未执行真实上游 OpenAI 请求；线上生效需后续走提交、不可变镜像和蓝绿验证流程。

## 2026-06-26 09:20 +08:00 - release v0.1.136.20 Codex 中断与请求头配置

- PASS：功能提交 `ca57fa228efa` 已构建为不可变镜像 `sub2api:v0.1.136.20`，镜像 ID `sha256:d5873d8dfc4797b1b7432af2432896146552620b0e6bb3fb83735d26ee1e5a84`；镜像标签和二进制版本输出均匹配 commit `ca57fa228efa`。
- PASS：只重建 idle `sub2api-blue`；候选发布期间 active `sub2api-green`、PostgreSQL、Redis 和 `sub2api-proxy` 未重启。
- PASS：blue 候选端口 `18083` 的 `/health`、首页、静态资源、未登录 admin 401、未登录 `/responses` 401、未登录 `/v1/responses` 401 均通过。
- PASS：blue 候选 65+ 秒观察保持 `healthy` 且 `RestartCount=0`。
- OBSERVE：候选日志窗口出现 1 条已知 `openai_request_snapshot` 清理噪声 `pq: canceling statement due to user request`，未伴随 panic、fatal、migration、bind、listen 或 rebuild 失败。
- PASS：代理 upstream 已从 `sub2api-green:8080` 切到 `sub2api-blue:8080`；`docker exec sub2api-proxy nginx -t` 通过，reload 成功。
- PASS：切流后公网入口 `8080` 和本机代理入口 `18081` 的健康、首页、静态资源、未登录 admin 401、未登录 `/responses` 401、未登录 `/v1/responses` 401 均通过。
- PASS：75+ 秒切流后观察中 `sub2api-blue` 与 `sub2api-green` 均保持 `healthy`；`8080`、`18081`、`18083` `/health` 全部返回 200；blue/proxy 关键日志扫描干净。
- Current state：active blue `sub2api:v0.1.136.20`；rollback green `sub2api:v0.1.136.19`。
- LIMIT：本轮未执行真实上游 OpenAI 请求；authenticated admin version endpoint 未运行，因为本地部署自动化路径 `ADMIN_PASSWORD` 为空。

## 2026-06-29T09:31:16+08:00 - v0.1.137-139 与 juhe-ai 可吸收优化计划

- PASS：`git ls-remote --tags origin refs/tags/v0.1.137 refs/tags/v0.1.138 refs/tags/v0.1.139` 确认远端标签对象分别为 `96e25629cd4a`、`e7a6bd46e3d9`、`a28c29d6902`。
- PASS：本地已存在并刷新 juhe-ai `feature/20250617` 分支，HEAD 为 `246ebb8d29dafe513e96cbf4f42918285ed9dca2`。
- PASS：CodeGraph 健康，索引 2190 files / 68929 nodes；已定位 OpenAI gateway、Codex client restriction、scheduler、provider 兼容入口。
- PASS：已生成计划文档 `docs/SUB2API_V0_1_137_139_JUHE_ABSORPTION_CEO_PLAN_CN.md`，按 D 范围拆分 Phase 1-4。
- LIMIT：本轮只生成计划文档，未修改运行时代码，未运行 Go/Vitest，未构建镜像，未部署。

## 2026-06-29T12:46:34+08:00 - Phase 1 zstd 上游响应解压

- PASS：保留并完成 Phase 1 第一个功能：`decompressResponseBody` 支持 `Content-Encoding: zstd`，成功解压后移除 `Content-Encoding` / `Content-Length` 并把 `ContentLength` 置为 `-1`。
- PASS：新增回归测试覆盖 zstd usage JSON 解压、gzip/br/deflate 既有编码保持可用、非法 zstd 与空 zstd 响应记录 `zstd_decompress_failed` 并保留原始响应体。
- PASS：`go test ./internal/repository -run TestDecompressResponseBody -count=1` 通过。
- PASS：`go test ./internal/repository -count=1` 通过。
- PASS：`git diff --check -- backend/internal/repository/http_upstream.go backend/internal/repository/decompress_response_test.go` 通过。
- LIMIT：本轮只完成 zstd 解压一个功能；未构建镜像、未部署、未执行真实上游请求。

## 2026-06-29 Devil - Phase 1 非 JSON 2xx failover

- 变更范围：ackend/internal/service/openai_gateway_service.go、ackend/internal/service/openai_gateway_service_test.go。
- RED：go test -tags unit ./internal/service -run "TestOpenAIGatewayService_Forward(NonStream2xxHTMLTriggersFailover|NonStream2xxEmptyBodyTriggersFailover|NonStream2xxJSONStillPasses|Stream2xxHTMLTriggersFailoverBeforeWrite|Stream2xxSSEStillPasses)$" -count=1 首次失败，HTML/空 body 非流式仍是普通 invalid json response，流式 HTML 返回旧的 missing-terminal failover 语义。
- GREEN：同一命令通过，覆盖 200 text/html、200 empty body、200 normal JSON、200 text/html stream、200 normal SSE。
- 相关回归：go test -tags unit ./internal/service -run "TestOpenAIStreaming(ResponseFailedBeforeOutputReturnsFailover|ClientDisconnectDrainsUpstreamUsage|TerminalEventWithoutUsageAddsClientTerminalFields)$" -count=1 通过。
- 相关回归：go test -tags unit ./internal/service -run "TestOpenAIGatewayService_ForwardRequestHeaderTimeoutReturnsFailover$|TestOpenAIGatewayService_APIKeyRequestBaseURLDoesNotFailoverBeforeAccountFailover$" -count=1 通过。
- 编译切片：go test -tags unit ./internal/service -run '^$' -count=1 通过。
- Diff 检查：git diff --check -- backend/internal/service/openai_gateway_service.go backend/internal/service/openai_gateway_service_test.go 通过。
- 已知非本轮失败：扩大命令 go test -tags unit ./internal/service -run "TestOpenAIGatewayService_Forward|TestOpenAIStreaming(ResponseFailedBeforeOutputReturnsFailover|ResponseFailedAfterOutputSendsRetryableFailedEvent|TerminalEventWithoutUsageAddsClientTerminalFields|ClientDisconnectDrainsUpstreamUsage)$" -count=1 124 秒超时；分段后 TestOpenAIGatewayService_ForwardRequestPhaseContextCanceledReturnsFailoverWhenClientStillConnected 和 TestOpenAIGatewayService_ForwardRequestPhaseContextCanceledDoesNotFailoverWhenClientCanceled 当前失败，返回/断言为既有 context canceled 请求阶段语义，未在本轮修复。
## 2026-06-29 Devil - Phase 1 SSE event:error 保真与 ops 诊断

- 变更范围：backend/internal/service/openai_gateway_service.go、backend/internal/service/openai_gateway_service_test.go。
- RED：go test -tags unit ./internal/service -run "TestOpenAIStreamingEventError(BeforeOutputReturnsFailoverWithRawBody|AfterOutputWritesRealFailedEvent)$" -count=1 首次失败；当前代码把 event:error 归入 missing terminal，未返回真实上游 message，也未记录 upstream_response_body。
- GREEN：同一命令通过；覆盖真实输出前 event:error 返回 UpstreamFailoverError 并保留原始 body，真实输出后写出包含上游 message 的 response.failed 且不再补 missing-terminal 泛化失败。
- 聚焦回归：go test -tags unit ./internal/service -run "TestOpenAIStreaming(EventErrorBeforeOutputReturnsFailoverWithRawBody|EventErrorAfterOutputWritesRealFailedEvent|ResponseFailedBeforeOutputReturnsFailover|QuotaFailedAfterOutputWritesGatewayRetryableFailure|MissingTerminalAfterOutputWritesFailedEvent|MissingTerminalEventAfterOutputWritesFailedTerminal)$|TestOpenAIGatewayService_Forward(NonStream2xxHTMLTriggersFailover|NonStream2xxEmptyBodyTriggersFailover|NonStream2xxJSONStillPasses|Stream2xxHTMLTriggersFailoverBeforeWrite|Stream2xxSSEStillPasses)$" -count=1 通过。
- Handler 回归：go test ./internal/handler -run "TestOpenAIForwardErrorAlreadyCommunicated" -count=1 通过，确认 service 已写失败终止后 handler 不追加泛化错误。
- 编译切片：go test -tags unit ./internal/service -run '^$' -count=1 通过。
- Diff 检查：git diff --check -- backend/internal/service/openai_gateway_service.go backend/internal/service/openai_gateway_service_test.go 通过。
- 边界：本轮覆盖 OpenAI Responses 主流式处理及默认 Chat Completions 转 Responses 路径；APIKey raw chat completions 直转流解析属于独立路径，未在本轮改动。

## 2026-06-29 Devil - Phase 1 并行吸收网关/provider/token/images 剩余内容

- 变更范围：backend/internal/repository/http_upstream.go、backend/internal/repository/decompress_response_test.go、backend/internal/service/openai_gateway_service.go、backend/internal/service/openai_gateway_service_test.go、backend/internal/service/openai_gateway_service_tool_correction_test.go、backend/internal/service/openai_gateway_chat_completions*.go、backend/internal/service/openai_images*.go、backend/internal/service/token_refresh_service*.go、backend/internal/service/vertex_service_account*.go、backend/internal/service/gemini_messages_compat_service*.go、backend/internal/pkg/apicompat/anthropic_to_responses.go、backend/internal/pkg/apicompat/anthropic_responses_test.go。
- RED：go test -tags unit ./internal/service -run "TestOpenAIStreaming(Passthrough)?DedupesFunctionCallArgumentsBeforeWrite$" -count=1 首次失败，普通流式和 passthrough 流式都会把重复的 Responses function-call arguments 原样写给客户端。
- GREEN：同一命令通过；普通流式与 passthrough 流式现在写客户端前会把完整重复 JSON arguments 去重。
- 聚焦回归：go test -tags unit ./internal/service -run "Test(SanitizeOpenAIResponseFailedEventForClient|CorrectToolCallsInResponseBody_DedupesResponsesFunctionCallArguments|OpenAIStreaming.*DedupesFunctionCallArgumentsBeforeWrite)$" -count=1 通过。
- 聚焦回归：go test -tags unit ./internal/service -run "Test(OpenAIStreamingResponseFailedBeforeOutputReturnsFailover|OpenAIStreamingPassthroughResponseFailedBeforeOutputReturnsFailover|OpenAIStreamingPassthroughQuotaFailedAfterOutputWritesGatewayRetryableFailure|OpenAIStreamingMissingTerminalEventAfterOutputWritesFailedTerminal)$" -count=1 通过。
- Phase 1 聚合：go test -tags unit ./internal/service -run "Test(OpenAIStreaming|OpenAI2xx|EventError|IsOpenAITransientProcessingError|ForwardAsChatCompletions_TransportErrorReturnsFailover|ForwardAsRawChatCompletions_TransportErrorReturnsFailover|ForwardAsRawChatCompletions_OverloadedAndTransientErrorsFailoverButOrdinary400DoesNot|IsNonRetryableRefreshError|TokenRefreshService_RefreshWithRetry_RedactsStoredErrorText|TokenRefreshService_RefreshWithRetry_RedactsTempUnschedulableReason|SanitizeOpenAIResponseFailedEventForClient|CorrectToolCallsInResponseBody_DedupesResponsesFunctionCallArguments)$" -count=1 通过。
- Images 聚焦：go test ./internal/service -run "TestOpenAIGatewayServiceForwardImages_OAuth(NonStreamResponseIncompleteTriggersFailover|NonStreamContentFilterIncompleteReturnsClient400|NonStreamNoImageOutputTriggersFailoverWithSummary|ServerErrorReturnsFailoverBody|NonStreamModerationBlockedReturnsClientError)$" -count=1 通过。
- Images 聚合：go test ./internal/service -run "(TestOpenAIGatewayServiceForwardImages_.*Images|TestOpenAIGatewayServiceForwardImages_.*OAuth|TestCollectOpenAIImagesFromResponsesBody_|TestBuildOpenAIImagesResponsesRequest_)" -count=1 通过。
- Provider 聚焦：go test ./internal/pkg/apicompat -run TestAnthropicToResponses_ProviderReasoningEffortCompatibility -count=1 通过；go test ./internal/service -run "Test(BuildVertexAnthropicRequestBodyFiltersUnsupportedBeta|NormalizeGeminiRequestForAIStudioCleansUnsupportedSchemaShapes)" -count=1 通过。
- Repository/handler 回归：go test ./internal/repository -run TestDecompressResponseBody -count=1 通过；go test ./internal/handler -run TestOpenAIForwardErrorAlreadyCommunicated -count=1 通过。
- Diff 检查：Phase 1 touched backend files scoped git diff --check 通过。
- 边界：本轮未构建镜像、未部署、未执行真实上游请求；scheduler outbox dedup/cleanup 仍是独立数据库迁移切片，未混入本轮网关/provider/token/images 改动。

## 2026-06-29 Devil - Phase 1 scheduler outbox dedup 与清理

- 变更范围：backend/internal/repository/scheduler_outbox_repo.go、backend/internal/repository/scheduler_outbox_repo_test.go、backend/internal/repository/ops_write_pressure_integration_test.go、backend/internal/service/scheduler_outbox.go、backend/internal/service/scheduler_snapshot_service.go、backend/internal/service/scheduler_snapshot_outbox_test.go、backend/migrations/161_scheduler_outbox_dedup_key.sql、backend/migrations/162_scheduler_outbox_pending_dedup_key_index_notx.sql。
- RED：go test ./internal/repository -run "Test(EnqueueSchedulerOutbox_UsesPersistentDedupKeyForIdempotentEvents|SchedulerOutboxRepositoryListAfterAndReleaseDedup|SchedulerOutboxRepositoryDeleteConsumedUpTo|SchedulerOutboxDedupKeyIncludesPayload)" -count=1 首次失败；缺少 schedulerOutboxDedupKey、ListAfterAndReleaseDedup、DeleteConsumedUpTo。
- RED：go test -tags unit ./internal/service -run "TestSchedulerSnapshotPollOutbox" -count=1 首次失败；SchedulerOutboxRepository 仍是旧 ListAfter 接口，未定义 cleanup lease。
- GREEN：go test ./internal/repository -run "Test(EnqueueSchedulerOutbox_UsesPersistentDedupKeyForIdempotentEvents|EnqueueSchedulerOutbox_DoesNotDedupLastUsedEvents|SchedulerOutboxRepositoryListAfterAndReleaseDedup|SchedulerOutboxRepositoryDeleteConsumedUpTo|SchedulerOutboxDedupKeyIncludesPayload)" -count=1 通过。
- GREEN：go test -tags unit ./internal/service -run "TestSchedulerSnapshotPollOutbox" -count=1 通过。
- 聚焦回归：go test ./internal/repository -run "SchedulerOutbox|EnqueueSchedulerOutbox" -count=1 通过。
- 聚焦回归：go test -tags unit ./internal/service -run "SchedulerSnapshot|Scheduler" -count=1 通过。
- 包级回归：go test ./internal/repository -count=1 通过。
- Scheduler 聚焦：go test -tags unit ./internal/service -run "TestSchedulerSnapshotPollOutbox|Test(OpenAISelectAccountWithLoadAwareness_HydratesSelectedAccountFromSchedulerSnapshot|GatewaySelectAccountWithLoadAwareness_HydratesSelectedAccountFromSchedulerSnapshot)" -count=1 通过。
- Repository 聚焦：go test ./internal/repository -run "Test(EnqueueSchedulerOutbox|SchedulerOutbox|ApplyMigrations)" -count=1 通过。
- Diff 检查：git diff --check -- backend/internal/repository/scheduler_outbox_repo.go backend/internal/repository/scheduler_outbox_repo_test.go backend/internal/repository/ops_write_pressure_integration_test.go backend/internal/service/scheduler_outbox.go backend/internal/service/scheduler_snapshot_service.go backend/internal/service/scheduler_snapshot_outbox_test.go backend/migrations/161_scheduler_outbox_dedup_key.sql backend/migrations/162_scheduler_outbox_pending_dedup_key_index_notx.sql 通过。
- 集成测试阻塞：go test -tags integration ./internal/repository -run "Test(EnqueueSchedulerOutbox_DeduplicatesIdempotentEvents|EnqueueSchedulerOutbox_DoesNotDeduplicateLastUsed|SchedulerSnapshotOutboxReplay)$" -count=1 未进入业务测试，原因是 Docker/Testcontainers 拉取 testcontainers/ryuk:0.13.0 时镜像源返回 403 Forbidden。
- 已知非本轮失败：go test -tags unit ./internal/service -count=1 当前在 scheduler 范围外失败，包括 account API key cooldown 时间断言、image bridge 403 fallback、OAuth passthrough client-cancel、OpenAI passthrough 429/529 failover panic。
- 边界：本轮未提交、未构建镜像、未部署、未执行真实上游请求。

## 2026-06-30 Devil - Phase 2 codex_cli_only chat/completions 覆盖与指纹诊断

- 变更范围：backend/internal/service/openai_gateway_chat_completions.go、backend/internal/service/openai_gateway_service.go、backend/internal/service/openai_gateway_service_codex_cli_only_test.go。
- RED：go test ./internal/service -run '^TestCodexCLIOnlyFingerprintProfile$|^TestLogCodexCLIOnlyDetection_RejectedIncludesRequestDetails$' -count=1 首次失败，缺少 codexCLIOnlyFingerprintProfile。
- GREEN：同一命令通过，覆盖 metadata/header 指纹来源摘要和拒绝日志字段。
- 聚焦回归：go test ./internal/service -run 'TestOpenAIGatewayService_ForwardAsChatCompletions_RejectsCodexCLIOnlyNonOfficialClient|TestOpenAIGatewayService_ForwardAsChatCompletions_AllowsAPIKeyRawChatWhenCodexCLIOnlyExtraIsPresent|TestLogCodexCLIOnlyDetection_RejectedIncludesRequestDetails|TestLogCodexCLIOnlyDetection_OnlyLogsRejected|TestOpenAICodexClientRestrictionDetector_Detect' -count=1 通过。
- gofmt 后聚焦回归：go test ./internal/service -run 'TestCodexCLIOnlyFingerprintProfile|TestOpenAIGatewayService_ForwardAsChatCompletions_RejectsCodexCLIOnlyNonOfficialClient|TestOpenAIGatewayService_ForwardAsChatCompletions_AllowsAPIKeyRawChatWhenCodexCLIOnlyExtraIsPresent|TestLogCodexCLIOnlyDetection_RejectedIncludesRequestDetails|TestLogCodexCLIOnlyDetection_OnlyLogsRejected|TestOpenAICodexClientRestrictionDetector_Detect' -count=1 通过。
- 编译切片：go test ./cmd/server -run TestNoSuchTest -count=1 通过。
- Diff 检查：git diff --check -- backend/internal/service/openai_gateway_chat_completions.go backend/internal/service/openai_gateway_service.go backend/internal/service/openai_gateway_service_codex_cli_only_test.go 通过。
- JSONL 审计：docs/feature_list.jsonl 与 docs/process_list.jsonl 尾部 8 行 ConvertFrom-Json 解析通过；同时修复上一条记录缺少换行导致的尾部粘连。
- 已知非本轮失败：go test ./internal/service -count=1 仍在本切片外失败，包括 account API key cooldown 时间断言、OAuth passthrough client-cancel、passthrough legacy originator 预期和 OpenAI passthrough failover nil repo 路径。
- 边界：本轮未提交、未构建镜像、未部署、未执行真实上游请求。

## 2026-06-30 Devil - Phase 2 GPT-5.5 Codex instructions 与 Claude Code terminal 模板

- 取舍：本地 sub2api 不新增全局白名单/黑名单准入系统；PAT auth 是 Personal Access Token 上游认证适配，本轮按用户要求跳过。
- 变更范围：backend/internal/pkg/openai/constants.go、backend/internal/pkg/openai/constants_test.go、backend/internal/pkg/openai/instructions_gpt5_5.txt、frontend/src/components/keys/UseKeyModal.vue、frontend/src/components/keys/__tests__/UseKeyModal.spec.ts、frontend/src/utils/ccswitchImport.ts、frontend/src/utils/__tests__/ccswitchImport.spec.ts、docs/SUB2API_V0_1_137_139_JUHE_ABSORPTION_CEO_PLAN_CN.md。
- RED：go test ./internal/pkg/openai -run TestCodexBaseInstructionsForModel -count=1 首次失败，gpt-5.5/gpt-5.4/gpt-5 仍回退到 GPT-5.1 或旧 Codex prompt。
- RED：corepack pnpm vitest run src/components/keys/__tests__/UseKeyModal.spec.ts 首次失败，OpenAI Codex config 仍为 gpt-5.4，Claude Code terminal snippet 缺少 CLAUDE_CODE_ATTRIBUTION_HEADER=0。
- GREEN：corepack pnpm vitest run src/components/keys/__tests__/UseKeyModal.spec.ts src/utils/__tests__/ccswitchImport.spec.ts 通过，覆盖 Codex 模板默认 gpt-5.5、goals feature、CC Switch 默认 gpt-5.5 和 Claude Code attribution env。
- GREEN：go test ./internal/pkg/openai -run 'TestDefaultTestModelUsesGPT55|TestCodexBaseInstructionsForModel' -count=1 通过。
- Phase 2 回归：go test ./internal/service -run 'TestCodexCLIOnlyFingerprintProfile|TestOpenAIGatewayService_ForwardAsChatCompletions_RejectsCodexCLIOnlyNonOfficialClient|TestOpenAIGatewayService_ForwardAsChatCompletions_AllowsAPIKeyRawChatWhenCodexCLIOnlyExtraIsPresent|TestLogCodexCLIOnlyDetection_RejectedIncludesRequestDetails|TestLogCodexCLIOnlyDetection_OnlyLogsRejected|TestOpenAICodexClientRestrictionDetector_Detect' -count=1 通过。
- Diff 检查：git diff --check -- Phase 2 touched backend/frontend files 通过。
- 边界：本轮未提交、未构建镜像、未部署、未执行真实上游请求。

## 2026-06-30 14:39 +08:00 - release v0.1.139.1 Codex 二期增强

- PASS：功能提交 `16caf15094b5` 已构建为不可变镜像 `sub2api:v0.1.139.1`，镜像 ID `sha256:673cc9c9c41ea9d79b55d2ee04b4b0546604f3d6caa96a4e9000c93894694068`；镜像 label `version=v0.1.139`、`revision=16caf15094b5` 与提交匹配。
- PASS：二进制版本输出为 `Sub2API v0.1.139 (image: v0.1.139.1, commit: 16caf15094b5, built: 2026-06-30T06:26:33Z)`。
- PASS：只重建 idle `sub2api-blue`；候选发布期间 active `sub2api-green`、PostgreSQL、Redis 和 `sub2api-proxy` 未重启。
- PASS：blue 候选端口 `18083` 的 `/health`、首页、静态资源、未登录 `GET /api/v1/admin/users` 401、未登录 `/responses` 401、未登录 `/v1/responses` 401 均通过。
- PASS：blue 候选观察超过 300 秒后仍为 `healthy` 且 `RestartCount=0`；最终候选日志窗口没有 panic、fatal、migration、checksum、bind、listen 或 rebuild 失败。
- OBSERVE：候选启动早期出现 1 条已知 `openai_request_snapshot` 清理噪声 `pq: canceling statement due to user request`，最终候选与切流后窗口未重复出现。
- PASS：代理 upstream 已从 `sub2api-green:8080` 切到 `sub2api-blue:8080`；`docker exec sub2api-proxy nginx -t` 通过，reload 于 2026-06-30 14:35:04 +08:00 成功。
- PASS：切流后公网入口 `8080` 和本机代理入口 `18081` 的健康、首页、静态资源、未登录 `GET /api/v1/admin/users` 401、未登录 `/responses` 401、未登录 `/v1/responses` 401 均通过。
- PASS：65+ 秒切流后观察中 `8080`、`18081`、`18083` `/health` 全部返回 200；`sub2api-blue` 运行 `sub2api:v0.1.139.1` 且 `healthy RestartCount=0`；blue 关键日志扫描干净。
- Current state：active blue `sub2api:v0.1.139.1`；rollback green `sub2api:v0.1.136.21`。
- LIMIT：未执行真实上游 OpenAI 请求；authenticated `/api/v1/admin/system/version` 未运行，因为本地部署自动化路径 `ADMIN_PASSWORD` 为空，版本小号通过代码/前端测试和二进制输出验证。

## 2026-06-30 Devil - 稳定性/性能/缓存命中吸收与发布自动化收敛

- 变更范围：`backend/internal/handler/concurrency_error_response.go`、`backend/internal/handler/gateway_helper.go`、Gateway/Chat/Responses/Gemini/OpenAI handler 用户槽位路径、`backend/internal/service/openai_gateway_chat_completions.go`、`deploy/release-bluegreen.ps1`、`docs/SUB2API_V0_1_137_139_JUHE_ABSORPTION_CEO_PLAN_CN.md`。
- RED/GREEN：`go test ./internal/handler -run 'TestAcquireUserSlotWithWait_' -count=1` 通过，覆盖立即获取跳过 wait counter、等待成功释放、超时释放、请求取消释放。
- RED/GREEN：`go test ./internal/service -run 'TestForwardAsChatCompletions_OAuthDoesNotInjectDefaultInstructions' -count=1` 通过，覆盖 OAuth Chat Completions 桥接不再注入默认 Codex instructions，仅保留空 `instructions` 字段。
- 聚焦回归：`go test ./internal/handler -run 'Test(WaitForSlotWithPingTimeout|AcquireUserSlotWithWait|ConcurrencyErrorResponse)' -count=1` 通过。
- 聚焦回归：`go test ./internal/service -run 'TestForwardAsChatCompletions_(OAuthDoesNotInjectDefaultInstructions|.*TransportError|.*ClientDisconnect|.*Terminal|.*Event)' -count=1` 通过。
- 编译切片：`go test ./cmd/server -run TestNoSuchTest -count=1` 通过。
- 编译切片：`go test ./internal/handler -run TestNoSuchTest -count=1` 通过。
- 编译切片：`go test ./internal/service -run TestNoSuchTest -count=1` 通过。
- 发布脚本 dry-run：`powershell -NoProfile -ExecutionPolicy Bypass -File .\deploy\release-bluegreen.ps1 -ImageVersion v0.1.139.2` 通过，只打印计划，未构建、未部署、未切流；识别当前 active blue、idle green。
- 发布脚本 cutover dry-run：`powershell -NoProfile -ExecutionPolicy Bypass -File .\deploy\release-bluegreen.ps1 -ImageVersion v0.1.139.2 -Cutover` 通过，只打印切流计划，确认 upstream 将指向 `sub2api-green:8080`，未写入 `active.conf`。
- Diff 检查：scoped `git diff --check` 通过；`deploy/release-bluegreen.ps1` 尾随空白检查通过。
- JSONL 审计：`docs/feature_list.jsonl` 尾部 4 行、`docs/process_list.jsonl` 尾部 3 行 `ConvertFrom-Json` 解析通过。
- 边界：本轮未执行 `-Execute`，因此没有构建新镜像、没有重建容器、没有变更 `D:\sub2api-deploy\.env` 或 `active.conf`、没有线上切流。

## 2026-07-01 Devil - 缓存命中第一/第二优先级

- 变更范围：`backend/internal/service/openai_compat_prompt_cache_key.go`、`backend/internal/service/openai_gateway_chat_completions.go`、`backend/internal/service/openai_gateway_messages.go` 及对应测试。
- RED：`go test ./internal/service -run 'TestDeriveCompatPromptCacheKey_UsesDeveloperRole|TestForwardAsChatCompletions_OAuthDoesNotInjectDefaultInstructions|TestForwardAsChatCompletions_ResponsesShapeDoesNotHTMLEscapeOAuthBody|TestForwardAsAnthropic_APIKeyPromptCacheInjectionDoesNotHTMLEscapeBody' -count=1` 首次失败，确认 Chat developer role 未进入自动 `prompt_cache_key` 种子、Chat `session_id` 使用未隔离 key、Chat/Messages upstream JSON 仍 HTML escape。
- GREEN：同一命令通过，确认 developer instructions 参与 key、后续 turns 不扰动 key、Chat session_id 使用 `generateSessionUUID(isolateOpenAISessionID(apiKeyID, promptCacheKey))`、Chat/Messages upstream body 保留 `<tag>&value` 而不是 `\u003c/\u003e/\u0026`。
- 聚焦回归：`go test ./internal/service -run 'Test(DeriveCompatPromptCacheKey|DeriveAnthropicCompatPromptCacheKey|DeriveOpenAIContentSessionSeed|ShouldAutoInjectPromptCacheKeyForCompat)' -count=1` 通过。
- 聚焦回归：`go test ./internal/service -run 'TestForwardAsChatCompletions_(OAuthDoesNotInjectDefaultInstructions|ResponsesShapeDoesNotHTMLEscapeOAuthBody|.*TransportError|.*ClientDisconnect|.*Terminal|.*Event)' -count=1` 通过。
- 聚焦回归：`go test ./internal/service -run 'TestForwardAsAnthropic_(InjectsPromptCacheKeyForAPIKeyMessagesDispatch|APIKeyPromptCacheInjectionDoesNotHTMLEscapeBody|AutoDerivesPromptCacheKeyWhenMessagesDispatchHasNoSessionID|DoesNotAutoDerivePromptCacheKeyForNonCodexModel|OAuthCompatKeepsFullReplayForCacheGrowth|OAuthKeepsSystemAsDeveloperInput|OAuthAddsTodoGuardAfterSystemDeveloperInstruction|OAuthToolCallsKeepOriginalCallIDs)' -count=1` 通过。
- 编译切片：`go test ./internal/service -run TestNoSuchTest -count=1` 通过；`go test ./cmd/server -run TestNoSuchTest -count=1` 通过。
- Diff 检查：scoped `git diff --check -- backend/internal/service/openai_compat_prompt_cache_key.go backend/internal/service/openai_compat_prompt_cache_key_test.go backend/internal/service/openai_gateway_chat_completions.go backend/internal/service/openai_gateway_chat_completions_test.go backend/internal/service/openai_gateway_messages.go backend/internal/service/openai_compat_model_test.go` 通过。
- 边界：本轮未提交、未构建镜像、未部署、未执行真实上游请求；工作树仍包含前序未提交改动和既有无关 `.codegraph/daemon.pid`、`backend/cmd/codex-live-probe/`、`tmp_body.json`。

## 2026-07-01 Devil - v0.1.140 稳定性/计费/上下文窗口补充吸收

- 来源：`git fetch --tags origin` 后确认 `v0.1.140` 存在；`git log --oneline --reverse v0.1.139..v0.1.140` 显示本版包含图片计费、fallback pricing 日志、Codex image bridge tool choice、上下文窗口不切号等提交。本轮只选择低冲突后端优化，不整 tag 合并。
- 变更范围：`backend/internal/service/image_output_accounting.go`、`backend/internal/service/image_generation_intent_test.go`、`backend/internal/service/openai_codex_transform.go`、`backend/internal/service/openai_codex_transform_test.go`、`backend/internal/service/openai_gateway_service.go`、`backend/internal/service/openai_gateway_service_test.go`、`backend/internal/service/openai_ws_forwarder.go`、`backend/internal/service/openai_ws_forwarder_success_test.go`、`backend/internal/service/billing_service.go`、`backend/internal/service/billing_service_test.go`、`backend/internal/service/openai_account_runtime_block_fastpath.go`、`backend/internal/service/openai_stream_policy.go`、`backend/internal/service/openai_gateway_chat_completions.go`、`backend/internal/service/openai_gateway_chat_completions_test.go`。
- RED/GREEN：`go test ./internal/service -run 'Test(EnsureOpenAIResponsesImageGenerationToolChoiceAuto|OpenAIGatewayServiceForward_CodexImageInjectionRespectsGroupCapability|OpenAIGatewayServiceForward_ChannelBridgeOverrideEnablesCodexInjection|OpenAIGatewayServiceForward_CodexBridgePreservesExistingToolChoice|OpenAIGatewayService_Forward_WSv2_ImageGenerationCountsOutputs)' -count=1` 首次失败在 WS 图片桥接测试，原因是测试未打开 `Gateway.CodexImageGenerationBridgeEnabled`，上游收到的 payload 没有 `tool_choice`；补齐测试配置后通过。
- 聚焦回归：`go test ./internal/service -run 'TestOpenAIImageOutputCounter(SkipsTextOnlyDataArrays|RequiresCompletedImageResult)' -count=1` 通过。
- 聚焦回归：`go test ./internal/service -run 'Test(EnsureOpenAIResponsesImageGenerationToolChoiceAuto|OpenAIGatewayServiceForward_CodexImageInjectionRespectsGroupCapability|OpenAIGatewayServiceForward_ChannelBridgeOverrideEnablesCodexInjection|OpenAIGatewayServiceForward_CodexBridgePreservesExistingToolChoice|OpenAIGatewayService_Forward_WSv2_ImageGenerationCountsOutputs)' -count=1` 通过。
- 聚焦回归：`go test ./internal/service -run 'Test(IsOpenAIContextWindowError|ShouldFailoverOpenAIUpstreamResponseContextWindow502|OpenAIHandleErrorResponse_ContextWindow502KeepsMessageWithoutFailover|ForwardAsChatCompletions_.*ContextWindow|OpenAIStreamingContextWindowResponseFailedBeforeOutputPassesThrough)' -count=1` 通过。
- 聚焦回归：`go test -tags unit ./internal/service -run 'TestGetModelPricing_(FallbackWarnLoggedOncePerModel|FallbackWarnPerModelNotGlobal|GLM52FallsBackToGLM5Price)' -count=1` 通过。
- 编译切片：`go test ./internal/service -run 'TestNoSuchTest' -count=1` 通过。
- 编译切片：`go test ./cmd/server -run 'TestNoSuchTest' -count=1` 通过。
- Diff 检查：scoped `git diff --check` 对 v0.1.140 吸收触达文件通过。
- 边界：本轮未提交、未构建镜像、未部署、未执行真实上游请求；工作树仍含前序未提交缓存命中/发布脚本改动和既有无关 `.codegraph/daemon.pid`、`backend/cmd/codex-live-probe/`、`tmp_body.json`。

## 2026-07-01 13:51 +08:00 - release v0.1.140.1 缓存与稳定性吸收

- PASS：主版本已更新为 `v0.1.140`，不可变镜像为 `sub2api:v0.1.140.1`；镜像 ID `sha256:aa039bf83691c588318129d80355e831ceb783d700a9c24f4d63d0dae1760d0d`。
- PASS：镜像 label 为 `org.opencontainers.image.version=v0.1.140`、`org.opencontainers.image.revision=2b37b941016c`；二进制版本输出为 `Sub2API v0.1.140 (image: v0.1.140.1, commit: 2b37b941016c, built: 2026-07-01T05:25:46Z)`。
- PASS：部署只重建 idle `sub2api-green`；发布期间保留 active `sub2api-blue`、PostgreSQL、Redis 和 `sub2api-proxy`，未对数据库或 Redis 执行重启。
- PASS：green 候选端口 `18082` 的 `/health`、首页、静态资源、未登录管理 API 401、未登录 `/responses` 401、未登录 `/v1/responses` 401 均通过。
- OBSERVE：候选启动早期出现 1 条 `openai_request_snapshot cleanup expired request snapshots failed err=pq: canceling statement due to user request`；重新观察 120 秒后关键日志窗口干净。
- PASS：代理 upstream 已从 `sub2api-blue:8080` 切到 `sub2api-green:8080`；`docker exec sub2api-proxy nginx -t` 通过，`docker exec sub2api-proxy nginx -s reload` 于 2026-07-01 13:32:18 +08:00 成功。
- PASS：切流后公网入口 `8080` 与本机代理入口 `18081` 的 `/health`、首页、静态资源、未登录管理 API 401、未登录 `/responses` 401、未登录 `/v1/responses` 401 均通过。
- PASS：75 秒切流后观察中 `sub2api-green` 运行 `sub2api:v0.1.140.1`，状态为 `healthy Status=running RestartCount=0`；最近 120 秒 app/proxy 日志未命中 panic、fatal、migration、checksum、bind、listen、rebuild 等发布关键错误。
- Current state：active green `sub2api:v0.1.140.1`；rollback blue `sub2api:v0.1.139.1`。
- LIMIT：未执行真实上游 OpenAI 请求；authenticated `/api/v1/admin/system/version` 未运行，因为本地部署自动化路径没有可用管理端密码。

## 2026-07-01 18:02 +08:00 - 上游体检 max_output_tokens 兼容修复

- 变更范围：`backend/internal/service/account_probe.go`、`backend/internal/service/account_probe_test.go`、`backend/internal/service/api_key_probe.go`、`backend/internal/service/api_key_probe_test.go`。
- RED：`go test -tags unit ./internal/service -run TestBuildOpenAIResponsesProbePayloadForSampleOmitsMaxOutputTokens -count=1` 首次失败，确认账号体检 Responses payload 会发送 `"max_output_tokens":16`。
- 修复：体检专用 `buildOpenAIResponsesProbePayload` 不再发送 `max_output_tokens`；保留 `model`、列表形态 `input`、`stream`、`store=false`、`instructions`，避免 zz1cc 这类兼容上游因未知参数误报失败。
- GREEN：账号体检与 API Key 体检聚焦测试均通过；`go test -tags unit ./internal/service -run 'Test(AccountProbe|APIKeyProbe|HTTPAPIKeyProbeRunner)' -count=1` 通过。
- LIMIT：本轮未提交、未构建镜像、未部署、未执行真实 zz1cc 上游请求。

## 2026-07-01 19:03 +08:00 - release v0.1.140.2 上游体检兼容修复

- PASS：功能提交 `06dfb66318d1` 已作为构建源；镜像 `sub2api:v0.1.140.2` 构建完成，镜像 ID `sha256:f279c27978740d6c1f52dfaa340a236281bb282264c8f0a9b1b5f9e185376dd0`。
- PASS：镜像 label 为 `org.opencontainers.image.version=v0.1.140`、`org.opencontainers.image.revision=06dfb66318d1`；二进制版本输出为 `Sub2API v0.1.140 (image: v0.1.140.2, commit: 06dfb66318d1, built: 2026-07-01T10:56:06Z)`。
- OBSERVE：首次脚本构建因 TUNA Alpine v3.23 包索引 403 失败；阿里源构建时出现一次 runtime `zstd-libs` layer I/O error；最终用中科大 Alpine 源成功构建同一 committed HEAD 和同一未存在的不可变 tag。
- PASS：部署只重建 idle `sub2api-blue`；发布期间保留 active `sub2api-green`、PostgreSQL、Redis 和 `sub2api-proxy`，未对数据库或 Redis 执行重启。
- PASS：blue 候选端口 `18083` 的 `/health`、首页、静态资源、未登录管理 API 401、未登录 `/responses` 401、未登录 `/v1/responses` 401 均通过。
- OBSERVE：候选启动早期出现 1 条已知 `openai_request_snapshot cleanup expired request snapshots failed err=pq: canceling statement due to user request`；重新观察 75 秒后关键日志窗口干净。
- PASS：代理 upstream 已从 `sub2api-green:8080` 切到 `sub2api-blue:8080`；`docker exec sub2api-proxy nginx -t` 通过，`docker exec sub2api-proxy nginx -s reload` 于 2026-07-01 19:00:31 +08:00 成功。
- PASS：切流后公网入口 `8080` 与本机代理入口 `18081` 的 `/health`、首页、静态资源、未登录管理 API 401、未登录 `/responses` 401、未登录 `/v1/responses` 401 均通过。
- PASS：75 秒切流后观察中 `sub2api-blue` 运行 `sub2api:v0.1.140.2`，状态为 `healthy Status=running Restart=0`；最近 75 秒 app/proxy 日志未命中 panic、fatal、migration、checksum、bind、listen、rebuild 等发布关键错误。
- Current state：active blue `sub2api:v0.1.140.2`；rollback green `sub2api:v0.1.140.1`。
- LIMIT：未执行真实 zz1cc 上游请求；authenticated `/api/v1/admin/system/version` 未运行，因为本地部署自动化路径没有可用管理端密码。

## 2026-07-03 14:20 +08:00 - P0 v0.1.142/v0.1.143 稳定性吸收

- 变更范围：`backend/internal/service/gateway_service.go`、`backend/internal/service/openai_gateway_service.go`、`backend/internal/service/openai_ws_forwarder.go`、`backend/internal/service/account.go`、`backend/internal/service/billing_service.go`、`backend/internal/service/openai_codex_transform.go`、`backend/internal/pkg/antigravity/*`、`backend/internal/repository/claude_oauth_service.go` 及对应测试。
- 修复/增强：Anthropic 流式在 thinking disabled 时抑制 thinking block；Claude Code CLI `>=2.1.193` 使用 noop `content_block_delta` keepalive；OpenAI HTTP 413 直接回 413 且不触发切号；WS 写客户端前的 429/401/read/dial/acquire/event/missing-final 失败转为 failover；WS retry budget `<=0` 立即耗尽；WS 首帧前延迟写 header；`codex.rate_limits` WS 事件解析并持久化，耗尽时设置账号运行时限流；APIKey WS ingress 记录 selected key，使 429/401 进入单 key 冷却；OpenAI OAuth `count_tokens` 不支持时返回本地估算；Antigravity reasoning model 请求减少不兼容参数。
- PASS：`gofmt` 覆盖本轮 Go 改动和新增 `gateway_count_tokens_test.go`。
- PASS：P0 focused service tests 通过：`go test -tags unit ./internal/service -run '<P0 focused Anthropic/OpenAI/WS/count_tokens tests>' -count=1 -timeout 3m`，输出 `ok github.com/Wei-Shaw/sub2api/internal/service 7.301s`。
- PASS：`go test ./internal/pkg/antigravity -count=1` 通过，输出 `ok github.com/Wei-Shaw/sub2api/internal/pkg/antigravity 0.054s`。
- PASS：`go test ./internal/repository -run 'TestClaudeOAuth' -count=1` 通过，输出 `ok github.com/Wei-Shaw/sub2api/internal/repository 0.079s`。
- PASS：`go test ./cmd/server -run TestNoSuchTest -count=1` 通过，输出 `ok github.com/Wei-Shaw/sub2api/cmd/server 0.046s [no tests to run]`。
- PASS：JSONL tail parse check 通过。
- PASS：scoped `git diff --check` 覆盖本轮 P0 touched files 通过。
- FAIL（宽单测既有风险）：`go test -tags unit ./internal/service -count=1 -timeout 5m` 返回 `ExitCode=1`，失败项为 `TestGatewayService_GroupResolution_ReusesContextGroup`、`TestGatewayService_GroupResolution_IgnoresInvalidContextGroup`、`TestGatewayService_GroupResolution_FallbackUsesLiteOnce`、`TestOpenAISelectAccountWithLoadAwareness_FiltersUnschedulable`、`TestOpenAISelectAccountWithLoadAwareness_FiltersUnschedulableWhenNoConcurrencyService`、`TestOpenAISelectAccountWithLoadAwareness_DoesNotPrecheckRealtimeBalance`、`TestOpenAISelectAccountWithLoadAwareness_AllowsOnlyAPIKeyWhenRealtimeBalanceUnknown`、`TestOpenAIGatewayService_ForwardRequestPhaseContextCanceledReturnsFailoverWhenClientStillConnected`、`TestOpenAIGatewayService_ForwardRequestPhaseContextCanceledDoesNotFailoverWhenClientCanceled`。
- OBSERVE：CodeGraph 本轮 `codegraph_status` 仍返回 `Transport closed`，`.codegraph/daemon.log` 最近记录包含多次 `socket error: write EPIPE`，因此按项目规则降级为 PowerShell 本地检索和 Go 测试验证。
- LIMIT：本轮提交前未构建镜像、未部署、未执行真实上游请求；`.codegraph/daemon.pid`、`backend/cmd/codex-live-probe/`、`tmp_body.json` 为无关脏文件，未纳入 P0。

## 2026-07-03 16:51 +08:00 - release v0.1.143.1 P0 稳定性吸收

- PASS：功能提交 `522de1755b1f` 已作为构建源；镜像 `sub2api:v0.1.143.1` 构建完成，镜像 ID `sha256:aaa17f5ff13c55a0b4b1e054c241c95d9a62de0d66aa877ed3a76c500b0e11ba`。
- PASS：镜像 label 为 `org.opencontainers.image.version=v0.1.143`、`org.opencontainers.image.revision=522de1755b1f`；二进制版本输出为 `Sub2API v0.1.143 (image: v0.1.143.1, commit: 522de1755b1f, built: 2026-07-03T08:43:57Z)`。
- PASS：使用 committed HEAD 归档构建临时上下文，显式传入 `ALPINE_APK_REPOSITORY=https://mirrors.ustc.edu.cn/alpine`；构建未使用未提交工作树。
- PASS：部署只重建 idle `sub2api-green`；发布期间保留 active `sub2api-blue`、PostgreSQL、Redis 和 `sub2api-proxy`，未对数据库或 Redis 执行重启。
- PASS：green 候选端口 `18082` 的 `/health`、首页、静态资源、未登录管理 API 401、未登录 `/responses` 401、未登录 `/v1/responses` 401 均通过。
- PASS：75 秒候选观察中 `sub2api-green` 运行 `sub2api:v0.1.143.1`，状态为 `healthy Status=running Restart=0`；最近 120 秒 app 日志关键错误命中 0。
- PASS：代理 upstream 已从 `sub2api-blue:8080` 切到 `sub2api-green:8080`；`docker exec sub2api-proxy nginx -t` 通过，`docker exec sub2api-proxy nginx -s reload` 于 2026-07-03 16:48:45 +08:00 成功。
- PASS：切流后公网入口 `8080` 与本机代理入口 `18081` 的 `/health`、首页、静态资源、未登录管理 API 401、未登录 `/responses` 401、未登录 `/v1/responses` 401 均通过。
- PASS：65 秒切流后观察中 `8080`、`18081`、`18082` `/health` 全部返回 200；green 应用和 proxy 最近 120 秒关键错误日志命中 0。
- Current state：active green `sub2api:v0.1.143.1`；rollback blue `sub2api:v0.1.140.2`。
- LIMIT：`ADMIN_PASSWORD` 为空，未执行 authenticated `/api/v1/admin/system/version`；未执行真实上游 OpenAI 请求。
- BLOCKED：`git push -u origin codex/merge-v0.1.134-updates` 被 GitHub 403 拒绝，错误为 `Permission to Wei-Shaw/sub2api.git denied to biaoxing-github`。

## 2026-07-03 17:20 +08:00 - P1 v0.1.142/v0.1.143 bugfix and metadata absorption

- 变更范围：`backend/internal/repository/account_repo.go`、`backend/internal/repository/account_repo_integration_test.go`、`backend/internal/service/openai_oauth_service.go`、`backend/internal/service/openai_subscription_test.go`、`frontend/src/composables/useOpenAIOAuth.ts`、`frontend/src/composables/__tests__/useOpenAIOAuth.spec.ts`。
- RED：`go test -tags integration ./internal/repository -run "TestAccountRepoSuite/TestListWithFilters_CountDoesNotPolluteListQuerySoftDeletePredicate" -count=1 -timeout 3m` 先失败，捕获列表 SQL 中 `"deleted_at" is null` 出现 2 次。
- GREEN：账号分页 Count 改为 `q.Clone().Count(ctx)` 后，`go test -tags integration ./internal/repository -run "TestAccountRepoSuite/(TestList|TestListWithFilters|TestPreload_And_VirtualFields)" -count=1 -timeout 3m` 通过。
- RED：`go test -tags unit ./internal/service -run "TestShouldApplyChatGPTAccountInfoPlanType" -count=1 -timeout 3m` 先因 `shouldApplyChatGPTAccountInfoPlanType` 未定义失败。
- GREEN：OpenAI OAuth `accounts/check` plan type 只在本地为空时补全；`go test -tags unit ./internal/service -run "TestShouldApplyChatGPTAccountInfoPlanType"` 和 OAuth 相邻切片通过。
- RED：`npm run test:run -- src/composables/__tests__/useOpenAIOAuth.spec.ts` 先失败，确认前端 `buildCredentials` 未透传 `subscription_expires_at`。
- GREEN：前端 OAuth credentials 透传 `subscription_expires_at` 后，同一 Vitest spec 4 tests 通过。
- PASS：确认 compact bridge 本地已有覆盖，`TestOpenAIGatewayServiceForward_CodexImageInjectionSkipsCompactRequest` 和 group capability 注入测试通过。
- PASS：`npm run typecheck` 通过；`go test ./cmd/server -run TestNoSuchTest -count=1` 通过；scoped `git diff --check` 通过。
- LIMIT：本轮未提交、未构建镜像、未部署、未执行真实上游请求；无关 `.codegraph/daemon.pid`、`backend/cmd/codex-live-probe/`、`tmp_body.json` 未触碰。

## 2026-07-03 17:31 +08:00 - P1 continue verification refresh

- PASS：重新执行 `go test -tags integration ./internal/repository -run "TestAccountRepoSuite/(TestList|TestListWithFilters|TestPreload_And_VirtualFields)" -count=1 -timeout 3m`，输出 `ok github.com/Wei-Shaw/sub2api/internal/repository 8.545s`。
- PASS：重新执行 `go test -tags unit ./internal/service -run "TestShouldApplyChatGPTAccountInfoPlanType|TestOpenAITokenProvider|TestOpenAI.*OAuth|TestAccountTestService_OpenAI429SyncsObservedPlanType|TestHandle429_OpenAISyncsObservedPlanType|TestOpenAIGatewayServiceForward_CodexImageInjectionSkipsCompactRequest|TestOpenAIGatewayServiceForward_CodexImageInjectionRespectsGroupCapability" -count=1 -timeout 3m`，输出 `ok github.com/Wei-Shaw/sub2api/internal/service 1.441s`。
- PASS：重新执行 `npm run test:run -- src/composables/__tests__/useOpenAIOAuth.spec.ts`，Vitest 输出 1 个文件、4 个测试全部通过。
- PASS：重新执行 `npm run typecheck`，`vue-tsc --noEmit` 通过。
- PASS：重新执行 `go test ./cmd/server -run TestNoSuchTest -count=1`，输出 `ok github.com/Wei-Shaw/sub2api/cmd/server 0.046s [no tests to run]`。
- PASS：scoped `git diff --check` 覆盖 P1 触达文件和记录文件，通过；仅提示 `docs/feature_list.jsonl`、`docs/process_list.jsonl` 下次 Git 触碰时 LF 会替换为 CRLF。
- LIMIT：本轮继续验证仍未提交、未构建镜像、未部署、未执行真实上游请求；无关 `.codegraph/daemon.pid`、`backend/cmd/codex-live-probe/`、`tmp_body.json` 未触碰。

## 2026-07-03 17:59 +08:00 - release v0.1.143.2 P1 bugfix absorption

- PASS：功能提交 `b89f51f07159` 已创建；提交范围只包含 P1 代码、测试和记录，未包含 `.codegraph/daemon.pid`、`backend/cmd/codex-live-probe/`、`tmp_body.json`。
- BLOCKED：`git push -u origin codex/merge-v0.1.134-updates` 被 GitHub 403 拒绝，错误为 `Permission to Wei-Shaw/sub2api.git denied to biaoxing-github`。
- PASS：发布脚本 dry-run 确认 active 为 green、idle 为 blue、候选端口为 `18083`、构建源为 committed HEAD `b89f51f07159`。
- PASS：`powershell -NoProfile -ExecutionPolicy Bypass -File .\deploy\release-bluegreen.ps1 -ImageVersion v0.1.143.2 -Execute -Cutover -AllowDirty` 构建不可变镜像 `sub2api:v0.1.143.2`，镜像 ID 为 `sha256:d3fc3024126f29e938a212eb5e2c4482cd27abbe5098d78f2508f55fa1f3fae4`。
- PASS：镜像 label 为 `org.opencontainers.image.version=v0.1.143`、`org.opencontainers.image.revision=b89f51f07159`；二进制版本输出为 `Sub2API v0.1.143 (image: v0.1.143.2, commit: b89f51f07159, built: 2026-07-03T09:51:37Z)`。
- OBSERVE：发布脚本在候选日志扫描阶段因 PowerShell 将 `docker logs` stderr 启动 WARN 视作 native command error 而提前停止；此时构建、blue 部署、候选 smoke、容器 healthy 已完成，尚未切流。
- PASS：手动复核 `sub2api-blue` 最近 180 秒日志，发布关键错误命中 0；手动复核候选 `18083` 的 `/health`、首页静态资源、未登录 admin、未登录 `/responses`、未登录 `/v1/responses` 全部符合预期。
- PASS：手动将 `D:\sub2api-deploy\proxy\upstreams\active.conf` 从 `sub2api-green:8080` 切到 `sub2api-blue:8080`，`docker exec sub2api-proxy nginx -t` 通过，`docker exec sub2api-proxy nginx -s reload` 于 2026-07-03 17:57:11 +08:00 成功。
- PASS：切流后 `8080`、`18081`、`18083` 两轮 smoke 均通过：`/health` 200、首页静态资源 200、未登录 admin 401、未登录 `/responses` 401、未登录 `/v1/responses` 401。
- PASS：65 秒观察后，blue 运行 `sub2api:v0.1.143.2` 且 `healthy Status=running Restart=0`；green 回滚容器运行 `sub2api:v0.1.143.1` 且 `healthy Status=running Restart=0`。
- PASS：最近 120 秒 `sub2api-blue` 与 `sub2api-proxy` 日志未命中 panic、fatal、migration、checksum、pq、bind、listen、rebuild 等发布关键错误。
- Current state：active blue `sub2api:v0.1.143.2`；rollback green `sub2api:v0.1.143.1`。
- LIMIT：`ADMIN_PASSWORD` 为空，未执行 authenticated `/api/v1/admin/system/version`；未执行真实上游 OpenAI 请求。

## 2026-07-03 19:11 +08:00 - P2 v0.1.142/v0.1.143 polish absorption

- 变更范围：`backend/internal/service/openai_gateway_service.go`、`gateway_forward_as_chat_completions.go`、`gateway_forward_as_responses.go`、`response_header_filter.go`、`backend/internal/handler/dto/*`、`frontend/src/types/index.ts` 及对应测试。
- RED：非流式 JSON 响应头测试先失败，确认上游 `text/event-stream` 或自定义 Content-Type 会残留到聚合 JSON 响应上。
- GREEN：复制上游响应头后统一调用 `forceJSONContentType` 覆盖聚合 JSON 响应；保留没有最终 JSON 的 SSE 原样返回分支。
- RED：账号 DTO JSON 先缺少 API Key `suffix`；GREEN 后 `api_key_items` 同时暴露 `fingerprint`、`masked` 和末 4 位 `suffix`，且原始 key 仍不出现在响应 JSON 中。
- RED：过期 5h 窗口仍保留旧 `resets_at`，Setup Token 过期 session window 仍显示旧使用率；GREEN 后过期窗口清零并清除旧 reset 时间。
- PASS：`go test ./internal/handler/dto -count=1`。
- PASS：`go test ./internal/service -run 'TestOpenAINonStreamingContentTypeForcesJSON|TestOpenAINonStreamingContentTypeDefault|TestHandleSSEToJSON_CompletedEventReturnsJSON|TestHandlePassthroughSSEToJSON_CompletedEventReturnsJSONContentType|TestHandleNonStreamingResponse_APIKeyFallsBackToSSEBodyWhenContentTypeIsWrong|TestHandleSSEToJSON_NoFinalResponseKeepsSSEBody|TestForwardAsChatCompletions_BufferedTerminalWithoutUpstreamCloseReturns|TestBuildCodexUsageProgressFromExtra_ZerosExpiredWindow|TestAccountUsageService_EstimateSetupTokenUsageZerosExpiredSessionWindow' -count=1`。
- PASS：`go test -tags unit ./internal/service -run 'TestHandleCCBufferedFromAnthropic_PreservesMessageStartCacheUsageAndReasoning|TestHandleResponsesBufferedStreamingResponse_PreservesMessageStartCacheUsage' -count=1`。
- PASS：`go test ./internal/service -run '^$'` 与 `go test ./cmd/server -run '^$'` 编译切片通过。
- PASS：`npm run typecheck` 通过。
- PASS：scoped `git diff --check` 覆盖 P2 触达文件，通过。
- LIMIT：本轮未提交、未构建镜像、未部署、未执行真实上游请求；无关 `.codegraph/daemon.pid`、`backend/cmd/codex-live-probe/`、`tmp_body.json` 未触碰。

## 2026-07-03 19:21 +08:00 - P2 completion verification refresh

- PASS：CodeGraph `codegraph_status` 返回健康，当前索引包含 2195 个文件、69170 个节点、181922 条边。
- PASS：`go test ./internal/handler/dto -count=1` 通过，输出 `ok github.com/Wei-Shaw/sub2api/internal/handler/dto 0.050s`。
- PASS：P2 精确 service 测试通过：`go test ./internal/service -run '^(TestBuildCodexUsageProgressFromExtra_ZerosExpiredWindow|TestAccountUsageService_EstimateSetupTokenUsageZerosExpiredSessionWindow|TestOpenAINonStreamingContentTypeForcesJSON|TestOpenAINonStreamingContentTypeDefault|TestHandleSSEToJSON_CompletedEventReturnsJSON|TestHandlePassthroughSSEToJSON_CompletedEventReturnsJSONContentType|TestHandleNonStreamingResponse_APIKeyFallsBackToSSEBodyWhenContentTypeIsWrong)$' -count=1`，输出 `ok github.com/Wei-Shaw/sub2api/internal/service 0.048s`。
- PASS：P2 Anthropic bridge/Chat Completions 精确测试通过：`go test ./internal/service -run '^(TestHandleCCBufferedFromAnthropic_PreservesMessageStartCacheUsageAndReasoning|TestHandleResponsesBufferedStreamingResponse_PreservesMessageStartCacheUsage|TestForwardAsChatCompletions_BufferedTerminalWithoutUpstreamCloseReturns)$' -count=1`，输出 `ok github.com/Wei-Shaw/sub2api/internal/service 0.045s`。
- PASS：`go test -tags unit ./internal/service -run 'TestGatewayForwardAs(ChatCompletions|Responses)|TestAnthropic|TestBuffered' -count=1` 通过，输出 `ok github.com/Wei-Shaw/sub2api/internal/service 0.106s`。
- PASS：`go test ./internal/service -run '^$'` 和 `go test ./cmd/server -run '^$'` 编译切片通过。
- PASS：`npm run typecheck` 通过，输出 `vue-tsc --noEmit` 无错误。
- OBSERVE：一次宽正则 service 测试命令误包含 WebSocket/限流历史用例并返回失败；随后改为锚定 P2 测试名后全部通过，因此该失败不作为 P2 缺陷。
- LIMIT：本轮仍未提交、未构建镜像、未部署、未执行真实上游请求；无关 `.codegraph/daemon.pid`、`backend/cmd/codex-live-probe/`、`tmp_body.json` 保持未处理。

## 2026-07-03 19:42 +08:00 - release v0.1.143.3 P2 polish absorption

- PASS：功能提交 `aaf5363c6` 已创建；提交范围只包含 P2 代码、测试和记录，未包含 `.codegraph/daemon.pid`、`backend/cmd/codex-live-probe/`、`tmp_body.json`。
- BLOCKED：`git push -u origin codex/merge-v0.1.134-updates` 被 GitHub 403 拒绝，错误为 `Permission to Wei-Shaw/sub2api.git denied to biaoxing-github`。
- PASS：发布脚本 dry-run 确认 active 为 blue、idle 为 green、候选端口为 `18082`、构建源为 committed HEAD `aaf5363c6494`。
- PASS：`powershell -NoProfile -ExecutionPolicy Bypass -File .\deploy\release-bluegreen.ps1 -ImageVersion v0.1.143.3 -Execute -Cutover -AllowDirty` 构建不可变镜像 `sub2api:v0.1.143.3`，镜像 ID 为 `sha256:23ba0766aff8973db12ace28b2194e707fa126fac4ffa50362f619a0ca242b45`。
- PASS：镜像 label 为 `org.opencontainers.image.version=v0.1.143`、`org.opencontainers.image.revision=aaf5363c6494`；二进制版本输出为 `Sub2API v0.1.143 (image: v0.1.143.3, commit: aaf5363c6494, built: 2026-07-03T11:34:00Z)`。
- OBSERVE：发布脚本在候选日志扫描阶段因 PowerShell 将 `docker logs` stderr 启动 WARN 视作 native command error 而提前停止；此时构建、green 部署、候选 smoke、容器 healthy 已完成，尚未切流。
- OBSERVE：初始候选日志窗口出现 1 条 `pq: canceling statement due to user request` 的过期快照清理日志；随后新候选窗口与切流后 120 秒窗口均未复现发布关键错误。
- PASS：手动复核候选 `18082` 的 `/health`、首页静态资源、未登录 admin、未登录 `/responses`、未登录 `/v1/responses` 全部符合预期；最近 120 秒 green 发布关键错误命中 0。
- PASS：手动将 `D:\sub2api-deploy\proxy\upstreams\active.conf` 从 `sub2api-blue:8080` 切到 `sub2api-green:8080`，`docker exec sub2api-proxy nginx -t` 通过，`docker exec sub2api-proxy nginx -s reload` 于 2026-07-03 19:40:14 +08:00 成功。
- PASS：切流后 `8080`、`18081`、`18082` smoke 均通过：`/health` 200、首页静态资源 200、未登录 admin 401、未登录 `/responses` 401、未登录 `/v1/responses` 401。
- PASS：65 秒观察后，green 运行 `sub2api:v0.1.143.3` 且 `healthy Status=running Restart=0`；blue 回滚容器运行 `sub2api:v0.1.143.2` 且 `healthy Status=running Restart=0`。
- PASS：最近 120 秒 `sub2api-green` 与 `sub2api-proxy` 日志未命中 panic、fatal、migration、checksum、pq、bind、listen、rebuild 等发布关键错误。
- Current state：active green `sub2api:v0.1.143.3`；rollback blue `sub2api:v0.1.143.2`。
- LIMIT：`ADMIN_PASSWORD` 为空，未执行 authenticated `/api/v1/admin/system/version`；未执行真实上游 OpenAI 请求。

## 2026-07-06 08:29 +08:00 - v0.1.144 P0/P1 absorption

- PASS：CodeGraph `codegraph_status` 健康，当前索引包含 2195 个文件、69172 个节点、182059 条边。
- PASS：实现 P0：`gateway.usage_record.overflow_policy` 默认改为 `sync`，worker pool 默认/非法配置也回到 `sync`，best-effort usage_log 被标记 dropped 时同步兜底写入。
- PASS：实现 P1：OpenAI Responses HTTP/WS 成功结果回填映射后的 `BillingModel`，RecordUsage 继续按 `BillingModelSource` 区分上游模型与请求模型计费。
- PASS：`go test -tags unit ./internal/config -run TestLoad_DefaultGatewayUsageRecordConfig -count=1` -> `ok github.com/Wei-Shaw/sub2api/internal/config 0.057s`。
- PASS：`go test -tags unit ./internal/service -run 'Test(WriteUsageLogBestEffort_FallsBackWhenBestEffortDropped|GatewayServiceRecordUsage_DroppedUsageLogFallsBackToSyncCreate|UsageRecordWorkerPool_OptionsFromConfig_NilConfig|UsageRecordWorkerPool_NormalizeOptions_BoundsAndDefaults)$' -count=1` -> `ok github.com/Wei-Shaw/sub2api/internal/service 0.054s`。
- PASS：`go test -tags unit ./internal/service -run 'TestOpenAIGatewayService_Forward_TextResponses(SetsBillingModelToMappedModel|WithoutMappingKeepsRequestedBillingModel)$' -count=1` -> `ok github.com/Wei-Shaw/sub2api/internal/service 0.057s`。
- PASS：`go test -tags unit ./internal/service -run 'TestOpenAIGatewayServiceRecordUsage_ResponsesMappedBillingModelHonorsBillingModelSource$' -count=1` -> `ok github.com/Wei-Shaw/sub2api/internal/service 0.054s`。
- PASS：宽一点的 P0/P1 service/config 切片通过：`go test -tags unit ./internal/service -run 'Test(OpenAIGatewayService_Forward_TextResponses|OpenAIGatewayServiceRecordUsage_.*Billing|GatewayServiceRecordUsage_.*UsageLog|UsageRecordWorkerPool_)' -count=1` 与 `go test -tags unit ./internal/config ./internal/service ... -count=1` 均通过。
- PASS：server 编译面 `go test -tags unit ./cmd/server -run TestNonExistent -count=0` 通过。
- PASS：`git diff --check` 通过；仅提示既有 `.codegraph/daemon.pid`、`docs/feature_list.jsonl`、`docs/process_list.jsonl` 下次 Git 触碰时 LF 会替换为 CRLF。
- LIMIT：本轮只完成 P0/P1 开发验证，未提交、未推送、未构建镜像、未部署、未执行真实上游 OpenAI 请求；无关 `.codegraph/daemon.pid`、`backend/cmd/codex-live-probe/`、`tmp_body.json` 未处理。

## 2026-07-08 08:39 +08:00 - NewApi 签到 dashboard SQL 迁移

- 变更范围：新增 `backend/migrations/163_add_newapi_checkin_tables.sql`、`backend/internal/repository/newapi_checkin_repo.go`、`backend/internal/service/newapi_checkin_service.go`、`backend/internal/handler/admin/newapi_checkin_handler.go`，并接入 `handler/wire`、`repository/wire`、`service/wire`、`server/routes/admin.go`、`cmd/server/wire_gen.go`；前端新增 `frontend/public/newapi-checkin/index.html`、`frontend/src/views/admin/NewAPICheckinView.vue`，并接入路由、侧边栏和中英文文案。
- PASS：页面和旧线程功能保持一致：总览、平台目录、签到记录、实时余额、月度历史、实时余额站点筛选联动趋势下拉、Turnstile 禁用站点保留展示/余额/月度查询，自动和手动签到跳过禁用站点。
- PASS：存储从 JSON 文件迁移到 sub2api SQL：配置、最近运行、余额缓存、签到历史、月度记录均通过 SQL repository 读写；页面存储文案改为 `sub2api SQL` / `读取 SQL 数据` / `写回 SQL`。
- PASS：CodeGraph `codegraph_status` 健康，当前索引 2203 files / 69519 nodes / 183206 edges。
- PASS：`git diff --check` 通过；仅提示既有 `.codegraph/daemon.pid`、`docs/feature_list.jsonl`、`docs/process_list.jsonl` LF/CRLF warning。
- PASS：`go test ./internal/handler/admin -run NewAPICheckin -count=1` 通过，输出 `ok github.com/Wei-Shaw/sub2api/internal/handler/admin 0.100s`。
- PASS：`go test ./internal/repository -run NewAPICheckin -count=1` 通过，输出 `ok github.com/Wei-Shaw/sub2api/internal/repository 0.040s [no tests to run]`。
- PASS：`go test ./internal/service -run NewAPICheckin -count=1` 通过，输出 `ok github.com/Wei-Shaw/sub2api/internal/service 0.064s`。
- PASS：`go test ./internal/handler/... ./internal/server/... ./cmd/server -run NewAPICheckin -count=1` 通过，覆盖 handler、admin handler、server routes、middleware、cmd/server 编译面。
- PASS：`npm run typecheck` 通过，`vue-tsc --noEmit` 无错误。
- PASS：内联 dashboard JS 语法检查通过：`inline_script_check=ok scripts=1`。
- PASS：静态锚点检查确认 `SUB2API_NEWAPI_PREFIX`、API URL 归一化、Authorization 注入、余额趋势联动函数和站点筛选属性均存在。
- PASS：Vite dev server `http://127.0.0.1:5174/` 下 `/newapi-checkin/index.html` 和 `/admin/newapi-checkin` 均返回 200。
- PASS：Browser 渲染冒烟确认页面标题为 `NewApi Checkin Dashboard`，页面非空，点击 `平台目录` 后配置视图选中，无相关 console error/warn；截图已在本轮 Browser 输出中保留。
- PASS：`npm run build` 通过；产物 `backend/internal/web/dist/newapi-checkin/index.html` 存在，包含 `sub2api SQL` 文案，不包含旧的 `本地 JSON 缓存` / `写回 JSON` / `读取本地 JSON`。
- OBSERVE：Browser 插件 `domSnapshot()` 报 `TypeError: o.incrementalAriaSnapshot is not a function`，因此同一 Browser 会话改用只读 evaluate、locator、console logs 和 screenshot 作为渲染证据。
- OBSERVE：当前本机 8080 后端仍是旧运行进程，`/api/v1/admin/newapi-checkin/config` 返回 404；当前源码里的新路由已通过 Go 测试和构建验证，真实 API smoke 需要重启/部署这份工作树后再跑。
- LIMIT：本轮未提交、未推送、未构建 Docker 镜像、未部署、未执行 authenticated admin API，也未对真实 NewApi 上游发起签到/余额请求。

## 2026-07-08 09:48 +08:00 - 工具箱子 tab 与 Token 成本 SQL 集成

- 变更范围：新增 token-cost SQL repository/service 测试、admin handler/routes/wire、`backend/migrations/164_add_token_cost_tables.sql`、`frontend/src/views/admin/AdminToolsView.vue`、`frontend/public/token-cost/index.html`、工具箱路由/侧边栏/i18n 和前端组件测试。
- PASS：左侧菜单现在只有 `/admin/tools` 一个工具箱入口；页面内部子 tab 区分 `NewApi 签到` 与 `Token 成本`；旧 `/admin/newapi-checkin` 重定向到 `tab=newapi`，新增 `/admin/token-cost` 重定向到 `tab=token-cost`。
- PASS：Token 成本运行时存储改为 SQL：`token_cost_state` 单例表、`token_cost_platforms`、`token_cost_history`、`token_cost_events`；迁移通过临时 JSONB 导入初始状态后落到规范表，不再依赖生产 JSON 文件。
- PASS：`go test ./internal/service ./internal/repository ./internal/handler/admin ./internal/handler ./internal/server/routes ./cmd/server -run 'Test(TokenCost|NewAPICheckin)|TestNonExistent' -count=1` 全部通过。
- PASS：`backend/migrations/164_add_token_cost_tables.sql` 在本机 `sub2api-postgres` 里用 `BEGIN ... ROLLBACK` 试跑通过，插入统计为 21 个平台、16 条历史、37 条事件。
- PASS：`npm run test:run -- src/views/admin/__tests__/AdminToolsView.spec.ts` 通过 2 个测试；`npm run typecheck` 通过；`git diff --check` 通过，仅有既有 LF/CRLF warning。
- PASS：Vite dev server `http://127.0.0.1:5173/` 下 Browser 渲染 `/token-cost/index.html` 和 `/newapi-checkin/index.html` 均非空且无 console error/warn；token-cost 页面脚本包含 `/api/v1/admin/token-cost` 与 Authorization 注入。
- OBSERVE：Browser evaluate 作用域为只读，无法注入 fake admin localStorage，所以受保护的 `/admin/tools` 容器渲染由 Vitest 覆盖，Browser 只验证两个 iframe 文档本体。
- LIMIT：未提交、未推送、未构建 Docker 镜像、未部署、未执行 authenticated admin API smoke。

## 2026-07-08 09:06 +08:00 - Token 成本 SQL 数据正式导入

- PASS：本机 `sub2api-postgres` 的 `schema_migrations` 已记录 `163_add_newapi_checkin_tables.sql` 与 `164_add_token_cost_tables.sql`，应用时间为 `2026-07-08 09:02:02~09:02:03 +08:00`，checksum 与本地迁移文件一致。
- PASS：`token_cost_state`、`token_cost_platforms`、`token_cost_history`、`token_cost_events` 均已存在。
- PASS：导入后记录数为 `token_cost_state=1`、`token_cost_platforms=21`、`token_cost_history=16`、`token_cost_events=37`。
- PASS：21 个平台余额数据已可从 SQL 读取：`torchai`、`okcodex`、`encore`、`qingflow`、`devpool`、`zz1cc`、`aisz`、`dawclaude`、`eirouter`、`tuling`、`乾行`、`5yuan`、`小白code`、`mikuapi`、`superapi`、`tokeness`、`词元`、`qiutian`、`openhh`、`登仙赞助`、`jucodex`。
- PASS：`token_cost_state` 当前为 `version=1`、`updated_at_text=2026-07-06T03:38:28.869Z`、`rank_mode=plus`、`personal_recharge_r=400`。
- PASS：`http://127.0.0.1:8080/health` 返回 `{"status":"ok"}`；未登录访问 `/api/v1/admin/token-cost/health` 与 `/api/v1/admin/token-cost/state` 返回 401，鉴权保护正常。
- LIMIT：本地 `ADMIN_PASSWORD` 为空，登录接口拒绝空密码，因此未执行 authenticated token-cost state API 读数；本轮未重启应用容器、未重启 PostgreSQL/Redis、未构建镜像、未部署、未提交。

## 2026-07-09 07:58 +08:00 - NewApi 后台任务 request context 修复与 v0.1.146.2 发布

- 根因：`StartFullCheckinJob` 与 `StartMonthlySyncJob` 把 `c.Request.Context()` 传入 goroutine；启动请求返回后 request context 被取消，后台全量签到和月度同步会被 `context canceled` 打断。
- 修复：两个后台任务都改为使用 `context.WithoutCancel(ctx)` 后再进入 goroutine，保留 context value 但不继承 HTTP 请求取消信号。
- RED：`go test ./internal/service -run 'TestNewAPICheckinStart(FullCheckin|MonthlySync)JobDetachesFromRequestContext' -count=1` 修复前失败，全量签到 `exitCode=-1`，月度同步 `error=context canceled`。
- PASS：修复后同一红测转绿，输出 `ok github.com/Wei-Shaw/sub2api/internal/service 1.091s`。
- PASS：`go test ./internal/service -run NewAPICheckin -count=1` 通过。
- PASS：`go test ./internal/handler/admin -run NewAPICheckin -count=1` 通过。
- PASS：`go test ./internal/server/routes ./cmd/server -run TestNonExistent -count=0` 通过。
- PASS：`git diff --check -- backend/internal/service/newapi_checkin_service.go backend/internal/service/newapi_checkin_service_test.go` 通过。
- PASS：提交 `b5ae4937387f fix(admin): 修复 NewApi 后台任务请求取消` 已创建。
- PASS：不可变镜像 `sub2api:v0.1.146.2` 已构建，镜像 ID 为 `sha256:31cb8b2c420c754c4b67cccc75e8d3f1563244d841b18b33c672088217bd2de6`，二进制版本输出为 `Sub2API v0.1.146 (image: v0.1.146.2, commit: b5ae4937387f, built: 2026-07-08T23:53:01Z)`。
- PASS：候选 green `18082` 通过 `/health`、静态资源、未登录 admin 401、未登录 `/responses` 401、未登录 `/v1/responses` 401、容器 healthy 和关键日志扫描。
- PASS：代理已从 `sub2api-blue:8080` 切到 `sub2api-green:8080`，`nginx -t` 与 `nginx -s reload` 成功。
- PASS：切流后 `http://127.0.0.1:8080/health`、`http://127.0.0.1:18081/health`、`http://127.0.0.1:18082/health` 均返回 200。
- PASS：切流后未登录 `GET /api/v1/admin/users`、`POST /api/v1/admin/newapi-checkin/run-full-checkin`、`POST /api/v1/admin/newapi-checkin/sync-monthly`、`POST /responses`、`POST /v1/responses` 均返回 401。
- PASS：65 秒观察后，active green 运行 `sub2api:v0.1.146.2` 且 `healthy Status=running Restart=0`；green/proxy 最近 120 秒关键日志命中 0。
- Current state：active green `sub2api:v0.1.146.2`；rollback blue `sub2api:v0.1.146.1`。
- OBSERVE：发布脚本在候选日志扫描阶段被 Docker 启动 WARN stderr 触发 `NativeCommandError` 停止；候选验证已通过，随后手动完成日志扫描、切流和观察。
- LIMIT：部署 `.env` 没有可用于非交互登录的 `ADMIN_PASSWORD` 明文值，未执行已认证后台任务 API body；没有执行 Git push。

## 2026-07-09 09:09 +08:00 - NewApi 余额符号与月度回填修复发布到 v0.1.146.3

- 根因：部分站点状态上游返回 `custom_currency_symbol=¤`，总余额/已用展示直接信任该占位符，导致页面前缀异常；同时“刷新当前站点余额/签到”在签到接口未返回奖励时，没有用当天月度记录回填 `quota_awarded` 与历史展示。
- 修复：`getNewAPIDisplaySymbol` 过滤通用货币占位符并回退默认美元符号；新增 `applyMonthlyAwardToCheckinLocked` 和历史/余额/月度统一展示文本规范化，使当天月度记录可以补齐签到奖励并修正旧 `¤...` 展示值。
- PASS：`go test ./internal/service -run 'TestNewAPICheckin(DisplaySymbolFallsBack|RefreshSiteBalancesUsesMonthlyRecordForTodayAward|HistoryUsesMonthlyRecordForExistingTodayAward)' -count=1` 通过，输出 `ok github.com/Wei-Shaw/sub2api/internal/service 0.050s`。
- PASS：候选 blue `18083` 通过 `/health`、`/admin/tools`、未登录 `GET /api/v1/admin/users`、未登录 `POST /responses`、未登录 `POST /v1/responses`，状态符合预期。
- PASS：`docker exec sub2api-blue /app/sub2api --version` 输出 `Sub2API v0.1.146 (image: v0.1.146.3, commit: d4c107435861, built: 2026-07-09T00:34:16Z)`；容器 `Image=sub2api:v0.1.146.3 Health=healthy Restart=0`。
- PASS：候选 blue 关键日志扫描 `panic|fatal|migration.*fail|checksum|pq:|bind:|address already in use|listen tcp|rebuild failed` 命中 0。
- PASS：代理已从 `sub2api-green:8080` 切到 `sub2api-blue:8080`，`nginx -t` 与 `nginx -s reload` 成功。
- PASS：切流后 `http://127.0.0.1:8080/health`、`http://127.0.0.1:18081/health`、`http://127.0.0.1:18083/health` 均返回 200。
- PASS：切流后未登录 `GET /api/v1/admin/users`、`POST /api/v1/admin/newapi-checkin/run-full-checkin`、`POST /responses`、`POST /v1/responses` 均返回 401。
- PASS：65 秒观察后，active blue 运行 `sub2api:v0.1.146.3` 且 `healthy Status=running Restart=0`；blue/proxy 最近 75 秒关键日志命中 0。
- Current state：active blue `sub2api:v0.1.146.3`；rollback green `sub2api:v0.1.146.2`。
- LIMIT：部署 `.env` 没有可用于非交互登录的 `ADMIN_PASSWORD` 明文值，未执行已认证 NewApi 页面点击或后台 API body 读取；没有执行 Git push。

## 2026-07-08 09:10 +08:00 - v0.1.143.4 SQL 工具箱发布验证

- PASS：功能提交 `c7718ed4d2db` 已存在，提交范围为 NewApi 签到 SQL 迁移、Token 成本 SQL 集成、管理端工具箱 tab、测试和过程记录。
- PASS：不可变镜像 `sub2api:v0.1.143.4` 已构建，镜像 ID 为 `sha256:99fcc89d142cad5134f730f491a856d33d45d4a2511723bb13ff599e83cfb67a`，label 为 `org.opencontainers.image.version=v0.1.143` 与 `org.opencontainers.image.revision=c7718ed4d2db`。
- PASS：`docker exec sub2api-blue /app/sub2api --version` 输出 `Sub2API v0.1.143 (image: v0.1.143.4, commit: c7718ed4d2db, built: 2026-07-08T01:00:40Z)`。
- PASS：候选 blue `18083` 验证通过：`/health` 200、首页 200、`/newapi-checkin/index.html` 200、`/token-cost/index.html` 200、未登录 admin/users 401、NewApi config 401、Token 成本 health 401、`/responses` 401、`/v1/responses` 401。
- PASS：`sub2api-blue` 运行 `sub2api:v0.1.143.4`，`Status=running Health=healthy Restart=0`。
- OBSERVE：候选启动早期日志出现 1 条 `pq: canceling statement due to user request` 的过期快照清理日志；后续最近 5 分钟候选和 active 均未复现发布阻断关键字。
- PASS：手动将 `D:\sub2api-deploy\proxy\upstreams\active.conf` 从 `sub2api-green:8080` 切到 `sub2api-blue:8080`；`docker exec sub2api-proxy nginx -t` 通过，`docker exec sub2api-proxy nginx -s reload` 于 2026-07-08 09:07:50 +08:00 成功。
- PASS：切流后 `8080`、`18081`、`18083` 冒烟均通过：`/health` 200、首页 200、`/newapi-checkin/index.html` 200、`/token-cost/index.html` 200、未登录 admin/users 401、NewApi config 401、Token 成本 health 401、`/responses` 401、`/v1/responses` 401。
- PASS：65 秒观察后，active blue 仍为 `sub2api:v0.1.143.4` 且 `healthy Status=running Restart=0`；rollback green 仍为 `sub2api:v0.1.143.3` 且 `healthy`；公网 `8080/health` 仍为 200。
- PASS：最近 120 秒 `sub2api-blue` 与 `sub2api-green` 日志未命中 panic、fatal、migration、checksum、pq、bind、listen、rebuild 等发布关键错误。
- PASS：`schema_migrations` 已记录 163/164 两个迁移；NewApi 8 张 SQL 表存在且当前为空；Token 成本 SQL 行数为 `token_cost_state=1`、`token_cost_platforms=21`、`token_cost_history=16`、`token_cost_events=37`。
- Current state：active blue `sub2api:v0.1.143.4`；rollback green `sub2api:v0.1.143.3`。
- LIMIT：本地 `ADMIN_PASSWORD` 为空，未执行 authenticated admin API；未执行真实 NewApi 上游签到/余额请求或真实 OpenAI 上游请求；未推送远程。

## 2026-07-08 11:20 +08:00 - v0.1.144/145/146 上游改动吸收评估

- PASS：`git fetch origin --prune --tags` 成功，新增远端 tag `v0.1.145` 与 `v0.1.146`；本地已有 `v0.1.144`。
- PASS：tag commit 确认：`v0.1.144` -> `41def4ba0386`，`v0.1.145` -> `3fa08aa9303b`，`v0.1.146` -> `d7a6a4513a58`。
- PASS：对比范围确认：`v0.1.143..v0.1.144` 为 97 files changed / 4532 insertions / 782 deletions；`v0.1.144..v0.1.145` 为 94 files changed / 5538 insertions / 690 deletions；`v0.1.145..v0.1.146` 为 75 files changed / 4653 insertions / 390 deletions。
- PASS：CodeGraph 状态健康，当前索引为 2208 files / 69607 nodes / 183419 edges；已用 CodeGraph 核对本地 `writeUsageLogBestEffort`、`RecordUsage`、endpoint、scheduler、concurrency 相关实现。
- OBSERVE：当前工作树仍有 144 P0/P1 未提交改动：usage record 默认 sync / usage log sync fallback / Responses mapped billing model；本次只做读取分析，没有继续吸收或覆盖业务源码。
- RECOMMEND：优先候选为 146 endpoint compact 归一化、145 OpenAI OAuth account test headers/custom UA、145 Anthropic custom models list、145 Antigravity server-invalidated token refresh、146 non-v1 OpenAI models URL、144 Codex session import identity collision、144 7d_oi/Fable model-level rate limit、144 concurrency slot cleanup。
- LIMIT：未修改业务代码、未跑 Go/npm 测试、未提交、未构建镜像、未部署、未执行真实上游请求。

## 2026-07-08 09:39 +08:00 - v0.1.144/145/146 P0 吸收本地验证

- PASS：按 TDD 先补 RED 用例，初次运行确认失败：endpoint 缺少 `EndpointResponsesCompact`，Codex 导入索引仍是单账号返回签名，Antigravity 强制刷新 extra key 缺失。
- PASS：已吸收 P0 小补丁：Responses compact/alias/raw path 归一化、OpenAI `/models` 非 v1 base URL、Anthropic custom models list 默认模型合并、Codex session import 身份冲突保护、Antigravity 401 强制刷新标记、OpenAI OAuth account-test Codex UA/originator。
- PASS：`go test ./internal/handler -run 'TestNormalizeInboundEndpoint|TestDeriveUpstreamEndpoint|TestResponsesSubpathSuffix|TestInboundEndpointMiddleware|TestGetInboundEndpoint|TestGetUpstreamEndpoint|TestGatewayModels' -count=1` 通过。
- PASS：`go test ./internal/handler/admin -run 'TestCodex' -count=1` 通过。
- PASS：`go test -tags unit ./internal/service -run 'TestBuildOpenAIModelsURL|TestAccountTestService_OpenAI.*OAuth|TestAntigravity|TestTokenRefreshService_RefreshWithRetry_Antigravity|TestRateLimitService_HandleUpstreamError_OAuth401' -count=1` 通过。
- PASS：编译/聚焦验证 `go test ./internal/handler ./internal/handler/admin -run 'TestNormalizeInboundEndpoint|TestDeriveUpstreamEndpoint|TestResponsesSubpathSuffix|TestInboundEndpointMiddleware|TestGetInboundEndpoint|TestGetUpstreamEndpoint|TestGatewayModels|TestCodex' -count=1`、`go test -tags unit ./cmd/server -run TestNonExistent -count=0` 均通过。
- PASS：`git diff --check` exit 0；仅提示既有 `.codegraph/daemon.pid`、`docs/feature_list.jsonl`、`docs/process_list.jsonl` LF/CRLF warning。
- LIMIT：本轮未提交、未构建镜像、未部署、未执行真实 OpenAI/Anthropic/Antigravity 上游请求；工作树仍包含此前未提交的 144 P0/P1 与 SQL 工具相关脏改。

## 2026-07-08 09:43 +08:00 - v0.1.144/145/146 P0 吸收复核

- PASS：复跑 `go test ./internal/handler -run 'TestNormalizeInboundEndpoint|TestDeriveUpstreamEndpoint|TestResponsesSubpathSuffix|TestInboundEndpointMiddleware|TestGetInboundEndpoint|TestGetUpstreamEndpoint|TestGatewayModels' -count=1` 通过，输出 `ok github.com/Wei-Shaw/sub2api/internal/handler 0.073s`。
- PASS：复跑 `go test ./internal/handler/admin -run 'TestCodex' -count=1` 通过，输出 `ok github.com/Wei-Shaw/sub2api/internal/handler/admin 0.057s`。
- PASS：复跑 `go test -tags unit ./internal/service -run 'TestBuildOpenAIModelsURL|TestAccountTestService_OpenAI.*OAuth|TestAntigravity|TestTokenRefreshService_RefreshWithRetry_Antigravity|TestRateLimitService_HandleUpstreamError_OAuth401' -count=1` 通过，输出 `ok github.com/Wei-Shaw/sub2api/internal/service 5.612s`。
- PASS：复跑合并 handler/admin 验证与 server 编译切片通过：`go test ./internal/handler ./internal/handler/admin -run 'TestNormalizeInboundEndpoint|TestDeriveUpstreamEndpoint|TestResponsesSubpathSuffix|TestInboundEndpointMiddleware|TestGetInboundEndpoint|TestGetUpstreamEndpoint|TestGatewayModels|TestCodex' -count=1`，`go test -tags unit ./cmd/server -run TestNonExistent -count=0`。
- PASS：`git diff --check` exit 0；仅提示既有 `.codegraph/daemon.pid`、`docs/feature_list.jsonl`、`docs/process_list.jsonl` LF/CRLF warning。
- LIMIT：本次复核未提交、未构建镜像、未部署、未执行真实 OpenAI/Anthropic/Antigravity 上游请求。

## 2026-07-08 09:56 +08:00 - v0.1.143.7 工具页 iframe/静态分发发布验证

- PASS：提交 `27812928bbb8 fix(admin): 修复工具页静态分发`，仅包含 `Dockerfile`、`backend/internal/web/embed_on.go`、`backend/internal/web/embed_test.go`。
- PASS：`go test -tags embed ./internal/web -run 'TestFrontendServer_Middleware/serves_standalone_tool_pages_before_spa_fallback' -count=1` 通过。
- PASS：`go test -tags embed ./internal/web -run 'TestFrontendServer_Middleware|TestServeEmbeddedFrontend' -count=1` 通过。
- PASS：`go test ./internal/server/middleware ./internal/server/routes ./cmd/server -run 'Test(SecurityHeaders|EnhanceCSPPolicy|AddToDirective)|TestNonExistent' -count=1` 通过。
- PASS：构建不可变镜像 `sub2api:v0.1.143.7`，镜像 label 为 `version=v0.1.143`、`revision=27812928bbb8`，构建日志确认前端层写入 `frontend build commit=27812928bbb8`。
- PASS：候选 green `http://127.0.0.1:18082` 验证通过：`/health` 200、首页静态资源 200、`/admin/tools?tab=newapi` 200、两个工具页 200 且标题分别为 `NewApi Checkin Dashboard` 与 `API Token 余额动态计算器`、受保护 API 返回 401。
- PASS：候选和切流后均验证 CSP/XFO：父页 `frame-src 'self'`、子页 `frame-ancestors 'self'`、`X-Frame-Options=SAMEORIGIN`。
- PASS：切流到 green 后，`http://127.0.0.1:8080`、`http://127.0.0.1:18081`、`http://127.0.0.1:18082` 三路健康检查与工具页冒烟均通过。
- PASS：切流后 active 为 `sub2api-green` / `sub2api:v0.1.143.7`，rollback 为 `sub2api-blue` / `sub2api:v0.1.143.6`。
- PASS：切流后 fresh 60 秒日志窗口无 `panic|fatal|migration.*fail|checksum|pq:|bind:|address already in use|listen tcp|rebuild failed`。
- LIMIT：候选启动早期出现过一次 `pq: canceling statement due to user request`，后续 fresh 60 秒窗口未复现；管理版本 API 因部署 `.env` 缺少 `ADMIN_PASSWORD` 未做登录态响应体验证。

## 2026-07-08 10:03 +08:00 - v0.1.144/145/146 P1 吸收本地验证

- PASS：已完成 P1 小补丁吸收：Anthropic `7d_oi` 仅写入 Fable 模型级冷却、并发槽位后台清理改为扫描 cache keys、OpenAI 默认模型加入 `gpt-5.6-sol/terra/luna`、GPT-5.6 计费回退 GPT-5.4、Codex 客户端版本限制返回具体升级/降级提示。
- PASS：版本文件 `backend/cmd/server/VERSION` 读回为 `v0.1.146`。
- PASS：`go test -tags unit ./internal/service -run 'Test(IsModelRateLimited_AnthropicFableFamilyKey|IsAnthropicFableModel|HandleUpstreamError_Anthropic7dOi|HandleUpstreamError_Anthropic5hWindow|StartSlotCleanupWorker_UsesCacheWideCleanupWithoutAccountRepo|GetModelPricing_GPT56|CodexClientRestrictionMessage|OpenAIGatewayService_Forward_VersionGateMessage)' -count=1` 通过，输出 `ok github.com/Wei-Shaw/sub2api/internal/service 0.081s`。
- PASS：`go test -tags unit ./internal/service -run 'Test(CalculateAnthropic429ResetTime|IsAnthropicWindowExceeded|UpdateSessionWindow|Handle429_AnthropicPlatformUnaffected|OpenAICodexClientRestrictionDetector|OpenAIGatewayService_GetCodexClientRestrictionDetector|OpenAIGatewayService_ForwardAsChatCompletions_RejectsCodexCLIOnlyNonOfficialClient|GetModelPricing)' -count=1` 通过，输出 `ok github.com/Wei-Shaw/sub2api/internal/service 0.062s`。
- PASS：`go test ./internal/pkg/openai -run 'TestDefaultModelsIncludeGPT56Family|TestCodexBaseInstructionsForModel' -count=1` 通过，输出 `ok github.com/Wei-Shaw/sub2api/internal/pkg/openai 0.022s`。
- PASS：编译切片 `go test -tags unit ./cmd/server -run TestNonExistent -count=0` 与 `go test -tags unit ./internal/repository -run TestNonExistent -count=0` 均通过。
- PASS：`git diff --check` exit 0；仅提示 `.codegraph/daemon.pid`、`backend/cmd/server/VERSION`、`docs/feature_list.jsonl`、`docs/process_list.jsonl` 的 LF/CRLF warning。
- LIMIT：本轮未提交、未构建镜像、未部署、未推送、未执行真实 Anthropic/OpenAI 上游请求。

## Toolbox SQL data import - 2026-07-08T11:33:25+08:00

Actor: Devil

Sources:
- NewApi config: C:\Users\27404\Documents\Playground\scripts\newapi_checkin_config.json
- NewApi balances: C:\Users\27404\Documents\Playground\output\newapi_checkin_balances.json
- NewApi latest run: C:\Users\27404\Documents\Playground\output\newapi_checkin_latest.json
- NewApi monthly/balance history: C:\Users\27404\Documents\Playground\output\newapi_checkin.sqlite
- Token cost state: C:\Users\27404\Documents\Playground\docs\token-api-cost-state.json

Actions:
- Ran .codex/import_toolbox_data.ps1 -DryRun through PostgreSQL transaction and ROLLBACK; SQL mapping produced expected row counts.
- Ran .codex/import_toolbox_data.ps1 and COMMIT; replaced toolbox SQL data from latest local JSON/SQLite sources.

Verification:
- NewApi SQL counts: sites=4, accounts=31, balances=31, latest_run_results=24, history=246, monthly=332.
- Token cost SQL counts: platforms=21, history=16, events=37, total_balance=2699.13, plus_count=17, pro_count=18.
- HTTP smoke: /health 200, /admin/tools 200, protected admin APIs returned 401 without login as expected.
- Active runtime: sub2api-blue image sub2api:v0.1.143.8 healthy; sub2api-green rollback image sub2api:v0.1.143.7 healthy.
- Fresh active logs: no panic/fatal/migration failure/checksum/pq/bind/listen/rebuild-failed matches in the last 10 minutes.

Risk / notes:
- No application rebuild or container restart was required because NewApi and TokenCost services load state from SQL repositories on each admin request.
- Access keys were imported into SQL from the provided config source but were not written into tracked repository files or logs.
## TokenCost platform count SQL refresh fix - 2026-07-08T11:46:36+08:00

Actor: Devil

Root cause:
- The TokenCost native legacy page rendered stale localStorage data first. Its raw fetch path did not reuse the admin axios token-refresh interceptor, so an expired auth_token caused /api/v1/admin/token-cost/health to return 401 and prevented loading the 21-platform SQL state.

Change:
- Added one-shot auth refresh and retry to frontend/src/views/admin/tools/tokenCostLegacy.generated.ts requestJson.
- Added frontend/src/views/admin/tools/tokenCostLegacy.generated.test.ts covering stale 16-platform local cache plus expired token, then refreshed token and 21-platform SQL state load.

Verification:
- npm run test:run -- tokenCostLegacy.generated.test.ts: 1 test passed.
- npm run typecheck: passed.

Runtime data evidence before release:
- PostgreSQL token_cost_platforms count is 21.
## TokenCost platform count release - 2026-07-08T11:55:15+08:00

Actor: Devil

Release:
- Commit: 9e0a85aa5b07 fix(admin): 修复 Token 成本页旧缓存显示
- Image: sub2api:v0.1.143.9
- ImageID: sha256:670c7cd169c37d5b77b04a5ef5e567bb621e153d9f927ffb696e8440f9585240
- Active after cutover: sub2api-green:8080
- Rollback: sub2api-blue sub2api:v0.1.143.8

Post-cutover verification:
- /health on 8080, 18081 and 18082 returned 200.
- /admin/tools on 8080 returned 200.
- /api/v1/admin/token-cost/state without login returned 401 as expected.
- /responses without login returned 401, not 502.
- Public AdminTools chunk is assets/AdminToolsView-DrvtKGhj.js and contains /api/v1/auth/refresh plus 401 retry and token-cost state page loading path.
- SQL token_cost_platforms count remains 21.
- Fresh 30-second post-cutover green log window had no panic/fatal/migration failure/checksum/pq/bind/listen/rebuild-failed matches.

## TokenCost dynamic API release - 2026-07-08T13:25:00+08:00

Actor: Devil

Release:
- Commit: 201ae65fd661 fix(admin): 动态加载 Token 成本数据
- Image: sub2api:v0.1.146.1
- ImageID: sha256:5e147cf4542b4df70303d05d4a81700e6495aef742df1b236d1cfa2c07f6d617
- Active after cutover: sub2api-blue:8080
- Rollback: sub2api-green sub2api:v0.1.143.9

Verification:
- Frontend focused tests, typecheck, backend TokenCost tests, and npm build passed before release.
- /health on 8080, 18081 and 18083 returned 200 after cutover.
- /, /assets/index-DkZDqoJb.js, protected admin APIs, /responses and /v1/responses smoke checks passed.
- /api/v1/admin/token-cost/state returned 401 without login, confirming the protected dynamic route is online.
- PostgreSQL counts are token_cost_platforms=21, token_cost_history=16, token_cost_events=37, token_cost_state=1.
- Source state file C:\Users\27404\Documents\Playground\docs\token-api-cost-state.json also has platforms=21, history=16, events=37.
- Public standalone token-cost page contains /api/v1/admin/token-cost/state and no longer contains STORAGE_KEY or token-api-cost-state.
- Fresh 75-second post-cutover blue log window had no panic/fatal/migration failure/checksum/bind/listen/rebuild-failed matches.

Limit:
- Noninteractive authenticated admin state read could not run because deploy .env has no readable ADMIN_PASSWORD value; no password or access token was printed.

## Grok OpenAI-compatible backend absorption - 2026-07-09T10:40:00+08:00

Actor: Devil

Scope:
- Absorb the minimum Grok backend slice from historical sub2api branch code so Grok groups can schedule Grok accounts through the existing OpenAI-compatible `/v1/responses`, `/v1/messages`, and `/v1/chat/completions` flow.

Verification:
- `go test ./internal/service -run 'TestPatchGrokResponsesBodySetsMappedModelAndDropsUnsupportedFields|TestBuildGrokResponsesRequestUsesAccountBaseURLAndBearerToken|TestOpenAIGatewayServiceListSchedulableAccountsUsesRequestedPlatform|TestOpenAIGatewayServiceGetAccessTokenUsesGrokOAuthCredential|TestOpenAISchedulerExhaustionProbeUsesRequestedGrokPlatform'`
- `go test ./internal/service -run 'TestOpenAISchedulerExhaustionProbeFiniteTriesEachCandidateTwice|TestOpenAISchedulerExhaustionProbeStopsOnSuccessAndClearsRuntimeBlock'`
- `go test ./internal/service -run TestDoesNotExist`
- `go test ./internal/server/routes -run 'TestGatewayRoutesGrokMessagesCountTokensReturnsOpenAICompatible404|TestGatewayRoutesOpenAIResponsesCompactPathIsRegistered|TestGatewayRoutesOpenAIImagesPathsAreRegistered'`
- `go test ./internal/handler/... -run TestDoesNotExist`

Observed result:
- Grok platform is now threaded through OpenAI-compatible account selection, sticky-session recheck, and scheduler exhaustion probing.
- Grok OAuth accounts now read `access_token` from Grok credentials and `/responses` requests branch to xAI-specific forwarding.
- Admin backend now accepts `platform=grok` for group create/update and returns Grok default model candidates.

Limit:
- This round did not add a full admin frontend Grok creation flow; creating or editing Grok accounts/groups still depends on backend API or existing data until the UI branch is absorbed separately.

## Grok admin frontend integration - 2026-07-09T16:00:00+08:00

Actor: Devil

Scope:
- Integrate the Grok admin frontend slice so Grok can be selected in account creation, OAuth authorization, refresh-token import, and re-authorization flows.
- Wire the existing Grok OAuth admin API into the shared admin API export and add missing xAI OAuth package helpers required by the backend Grok OAuth service.

Verification:
- `npm run typecheck`
- `go test ./internal/service -run 'TestPatchGrokResponsesBodySetsMappedModelAndDropsUnsupportedFields|TestBuildGrokResponsesRequestUsesAccountBaseURLAndBearerToken|TestOpenAIGatewayServiceListSchedulableAccountsUsesRequestedPlatform|TestOpenAIGatewayServiceGetAccessTokenUsesGrokOAuthCredential|TestOpenAISchedulerExhaustionProbeUsesRequestedGrokPlatform'`
- `go test ./internal/server/routes -run 'TestGatewayRoutesGrokMessagesCountTokensReturnsOpenAICompatible404|TestGatewayRoutesOpenAIResponsesCompactPathIsRegistered|TestGatewayRoutesOpenAIImagesPathsAreRegistered'`
- `go test ./internal/handler/... -run TestDoesNotExist`
- `go test ./cmd/server -run TestDoesNotExist`
- `git diff --check`

Observed result:
- Frontend typecheck passed after adding Grok platform/type/i18n/color/API/composable integration.
- Grok account creation now exposes OAuth-only selection, Grok auth URL generation, code exchange, and manual refresh-token validation paths.
- Account re-authorization modals now route Grok accounts through Grok OAuth state, auth URL, and code exchange instead of falling back to Anthropic.
- Backend focused service/routes/handler/server compile gates passed after adding xAI OAuth helpers used by Grok OAuth service and token client.

Limit:
- This round did not commit, build, deploy, or perform authenticated browser click-through with a real Grok account.

## Grok API Key account support - 2026-07-09T16:28:19+08:00

Actor: Devil

Scope:
- Add Grok API Key account support on top of the Grok admin frontend integration.
- Let Grok API Key credentials use `credentials.api_key` for gateway access tokens instead of the OpenAI-specific `openai_api_key` field.
- Expose Grok API Key selection and xAI API Key/base URL hints in the account creation and edit forms.

Verification:
- RED before implementation: `go test ./internal/service -run TestOpenAIGatewayServiceGetAccessTokenUsesGrokAPIKeyCredential` failed with `api_key not found in credentials`.
- `npm run typecheck`
- `go test ./internal/service -run 'TestOpenAIGatewayServiceGetAccessTokenUsesGrokOAuthCredential|TestOpenAIGatewayServiceGetAccessTokenUsesGrokAPIKeyCredential|TestBuildGrokResponsesRequestUsesAccountBaseURLAndBearerToken|TestOpenAIGatewayServiceListSchedulableAccountsUsesRequestedPlatform'`
- `go test ./internal/server/routes -run 'TestGatewayRoutesGrokMessagesCountTokensReturnsOpenAICompatible404|TestGatewayRoutesOpenAIResponsesCompactPathIsRegistered|TestGatewayRoutesOpenAIImagesPathsAreRegistered'`
- `go test ./internal/handler/... -run TestDoesNotExist`
- `go test ./cmd/server -run TestDoesNotExist`
- `git diff --check -- frontend/src/components/account/EditAccountModal.vue frontend/src/components/account/CreateAccountModal.vue frontend/src/components/account/AccountAPIKeyCredentialsFields.vue frontend/src/i18n/locales/en.ts frontend/src/i18n/locales/zh.ts backend/internal/service/openai_gateway_service.go backend/internal/service/openai_gateway_service_grok_test.go`

Observed result:
- Grok API Key accounts now return token type `apikey` from `GetAccessToken` and forward the stored xAI key through the existing OpenAI-compatible gateway path.
- Account creation now allows selecting Grok API Key with xAI API Key wording and `https://api.x.ai/v1` as the Grok default base URL.
- Account editing now shows the Grok base URL hint and preserves the Grok default base URL instead of falling back to Anthropic defaults.

Limit:
- This round did not commit, build, deploy, or perform authenticated browser click-through with a real Grok API Key account.

## Grok OAuth/API Key release v0.1.146.4 - 2026-07-09T16:54:00+08:00

Actor: Devil

Scope:
- Commit the Grok OAuth/API Key integration and deploy it through the blue-green release path.
- Build immutable image `sub2api:v0.1.146.4` from committed code commit `dc4bc0971d95`.
- Deploy the new image to idle green, validate candidate smoke, cut the proxy from blue to green, and observe post-cutover health/logs.

Verification:
- Code commit: `dc4bc0971d95 feat(grok): 接入 Grok OAuth 和 API Key`.
- First docker build attempt with the default Tsinghua Alpine apk mirror failed before image export with `Connection refused` and missing `git/tzdata`; no image tag was produced.
- Retry docker build with `ALPINE_APK_REPOSITORY=https://mirrors.aliyun.com/alpine`: passed, including frontend `pnpm run build` and backend release build.
- Image inspect: `sub2api:v0.1.146.4` -> `sha256:36b619c43989ddc0db962e32ee6a9f0ac2a77d0483f5ac1aee36903e4629c639`, label revision `dc4bc0971d95`.
- Image version: `Sub2API v0.1.146 (image: v0.1.146.4, commit: dc4bc0971d95, built: 2026-07-09T08:37:52Z)`.
- Candidate green `18082`: `/health` 200, `/admin/tools` 200, unauthenticated `GET /api/v1/admin/users` 401, `POST /responses` 401, `POST /v1/responses` 401.
- Candidate green state: `Image=sub2api:v0.1.146.4 Health=healthy Status=running Restart=0`; active blue stayed `sub2api:v0.1.146.3 Health=healthy Status=running Restart=0`.
- Candidate first log scan found one `service/openai_request_snapshot` cleanup `pq: canceling statement due to user request`; a fresh 75-second candidate window then had zero critical matches and health remained 200.
- Proxy cutover: `active.conf` switched from `sub2api-blue:8080` to `sub2api-green:8080`; `docker exec sub2api-proxy nginx -t` and `nginx -s reload` passed.
- Post-cutover `8080`, `18081`, and `18082` health checks returned 200.
- Post-cutover unauthenticated `GET /api/v1/admin/users`, `POST /api/v1/admin/grok/oauth/auth-url`, `POST /responses`, and `POST /v1/responses` returned 401.
- After a 65-second observation window, active green remained `sub2api:v0.1.146.4` with `healthy Status=running Restart=0`; rollback blue remained `sub2api:v0.1.146.3` with `healthy Status=running Restart=0`; green/proxy critical log matches were 0.

Observed result:
- Grok OAuth/API Key integration is now active behind the public proxy on green.
- The public gateway remains protected for unauthenticated OpenAI-compatible and Grok OAuth admin routes.
- Rollback target remains blue `sub2api:v0.1.146.3`.

Limit:
- No Git push was run.
- No authenticated admin browser click-through or real Grok/xAI upstream request was run because the deployment environment does not expose a non-interactive admin password in `.env`.

## GPT-5.6 model loading absorption - 2026-07-09T16:56:00+08:00

Actor: Devil

Scope:
- Load the v0.1.146 GPT-5.6 OpenAI model family into frontend model whitelist/preset mapping.
- Complete backend pricing fallback so `gpt-5.6-sol`, `gpt-5.6-terra`, and `gpt-5.6-luna` can be priced when the remote catalog has not caught up.
- Align Codex OAuth model normalization tables with the v0.1.146 GPT-5.6 model entries.

Verification:
- RED before implementation: `npm run test:run -- src/composables/__tests__/useModelWhitelist.spec.ts` failed because the OpenAI whitelist and presets did not include `gpt-5.6-sol`.
- RED before implementation: `go test ./internal/service -run 'TestGetModelPricing_Gpt56UsesStaticFallbackWhenRemoteMissing|TestNormalizeCodexModel_Gpt53' -count=1` failed because `PricingService.GetModelPricing("gpt-5.6-*")` returned nil.
- PASS: `npm run test:run -- src/composables/__tests__/useModelWhitelist.spec.ts`.
- PASS: `npm run typecheck`.
- PASS: `go test ./internal/service -run 'TestGetModelPricing_Gpt56UsesStaticFallbackWhenRemoteMissing|TestGetModelPricing_Gpt54UsesStaticFallbackWhenRemoteMissing|TestGetModelPricing_OpenAICompactAliasUsesStaticFallback|TestNormalizeCodexModel_Gpt53' -count=1`.
- PASS: `go test ./internal/pkg/openai -run 'TestDefaultModelsIncludeGPT56Family|TestCodexBaseInstructionsForModel' -count=1`.
- PASS: `go test ./internal/handler/admin -run 'TestAccountHandlerGetAvailableModels_OpenAIOAuthPassthroughFallsBackToDefaults|TestAccountHandlerGetAvailableModels_OpenAIOAuthUsesExplicitModelMapping' -count=1`.
- PASS: `git diff --check` returned exit 0 with existing LF/CRLF warnings for `.codegraph/daemon.pid` and `docs/releases/v0.1.146.4.md`.

Limit:
- This round did not commit, build, deploy, or push.

## Grok default model and Grok 4.5 loading - 2026-07-09T17:12:49+08:00

Actor: Devil

Scope:
- Fix Grok account model loading so frontend Grok whitelist/preset selection uses Grok models instead of falling back to Claude defaults.
- Add `grok-4.5` as the first Grok/xAI default model in backend defaults and frontend Grok whitelist.
- Map `grok` and `grok-latest` aliases to `grok-4.5` consistently in backend xAI defaults and frontend presets.

Verification:
- RED before implementation: `npm run test:run -- src/composables/__tests__/useModelWhitelist.spec.ts` failed because Grok frontend platform selection did not load Grok-specific model/preset branches.
- RED before implementation: `go test ./internal/pkg/xai -run TestDefaultModelsIncludeGrok45AsDefaultAliasTarget -count=1` failed because the backend xAI default first model and aliases still targeted `grok-4.3`.
- PASS: `gofmt -w internal\pkg\xai\models.go internal\pkg\xai\models_test.go`.
- PASS: `go test ./internal/pkg/xai -run TestDefaultModelsIncludeGrok45AsDefaultAliasTarget -count=1` -> `ok github.com/Wei-Shaw/sub2api/internal/pkg/xai 0.035s`.
- PASS: `npm run test:run -- src/composables/__tests__/useModelWhitelist.spec.ts` -> 1 file passed, 14 tests passed.
- PASS: `npm run typecheck`.
- PASS: `git diff --check -- backend/internal/pkg/xai/models.go backend/internal/pkg/xai/models_test.go frontend/src/composables/useModelWhitelist.ts frontend/src/composables/__tests__/useModelWhitelist.spec.ts`.

Observed result:
- Grok model selection now starts with `grok-4.5`, keeps `grok-4.3`, and does not include Claude fallback entries.
- Grok preset mappings now include `grok-4.5`, and `grok` / `grok-latest` resolve to `grok-4.5`.
- Backend xAI `/models` defaults include `grok-4.5` first, and the default alias mapping resolves `grok` / `grok-latest` to `grok-4.5`.

Limit:
- This round did not commit, build, deploy, push, or run a real authenticated Grok upstream request.

## GPT-5.6 tier pricing correction - 2026-07-09T17:13:34+08:00

Actor: Devil

Scope:
- Replace the temporary GPT-5.6 -> GPT-5.4 fallback pricing with explicit GPT-5.6 Sol/Terra/Luna tier prices.
- Keep `PricingService` dynamic-price fallback and `BillingService` actual charging fallback aligned.

Verification:
- RED: `go test ./internal/service -run 'TestGetModelPricing_Gpt56UsesTierStaticFallbackWhenRemoteMissing' -count=1` failed before implementation because Sol and Luna still returned GPT-5.4 input price `2.5e-6`.
- RED: file-level `go test -tags unit ... billing_service_test.go ... -run 'TestGetModelPricing_GPT56UsesTierFallbackPricing' -count=1` failed before implementation because BillingService still returned GPT-5.4 input price for Sol/Luna.
- RED: after adding the long-context assertion, the same BillingService target test failed because GPT-5.6 still inherited GPT-5.4 `LongContextInputThreshold=272000` through `applyModelSpecificPricingPolicy`.
- PASS: `gofmt -w internal/service/pricing_service.go internal/service/pricing_service_test.go internal/service/billing_service.go internal/service/billing_service_test.go`.
- PASS: `go test ./internal/service -run 'TestGetModelPricing_Gpt56UsesTierStaticFallbackWhenRemoteMissing' -count=1`.
- PASS: file-level BillingService target test `go test -tags unit ... -run 'TestGetModelPricing_GPT56UsesTierFallbackPricing|TestGetModelPricing_OpenAIGPT54Fallback|TestGetModelPricing_OpenAIGPT55ProFallback' -count=1`.
- PASS: `go test ./internal/service -run 'TestGetModelPricing_Gpt56UsesTierStaticFallbackWhenRemoteMissing|TestGetModelPricing_Gpt54UsesStaticFallbackWhenRemoteMissing|TestGetModelPricing_OpenAICompactAliasUsesStaticFallback|TestNormalizeCodexModel_Gpt53' -count=1`.
- PASS: `go test ./internal/pkg/openai -run 'TestDefaultModelsIncludeGPT56Family|TestCodexBaseInstructionsForModel' -count=1`.
- PASS: `go test ./internal/handler/admin -run 'TestAccountHandlerGetAvailableModels_OpenAIOAuthPassthroughFallsBackToDefaults|TestAccountHandlerGetAvailableModels_OpenAIOAuthUsesExplicitModelMapping' -count=1`.
- PASS: `npm run test:run -- src/composables/__tests__/useModelWhitelist.spec.ts` -> 1 file passed, 14 tests passed.
- PASS: `npm run typecheck`.
- PASS: `git diff --check` returned exit 0 with existing LF/CRLF warnings only.

Observed result:
- `gpt-5.6-sol` static fallback now uses input `5e-6`, output `30e-6`.
- `gpt-5.6-terra` static fallback now uses input `2.5e-6`, output `15e-6`.
- `gpt-5.6-luna` static fallback now uses input `1e-6`, output `6e-6`.
- GPT-5.6 tiers no longer inherit GPT-5.4 long-context fallback rules; GPT-5.4 and GPT-5.5 fallback tests still pass.

Limit:
- Full `go test -tags unit ./internal/service` was not used as a verification gate because existing unrelated token refresh unit tests currently do not compile against the `NewTokenRefreshService` signature.
- This round did not commit, build, deploy, or push.

## GPT-5.6 visibility release v0.1.146.5 - 2026-07-09T19:09:08+08:00

Actor: Devil

Scope:
- Diagnose why GPT-5.6 models were not visible in the admin model selector.
- Deploy already-built image `sub2api:v0.1.146.5` from commit `eff26f02c454` to the idle blue container.
- Switch active traffic from green `sub2api:v0.1.146.4` to blue `sub2api:v0.1.146.5` after candidate verification.

Root cause:
- The GPT-5.6 code was present in `eff26f02c454` and image `sub2api:v0.1.146.5`, but the live proxy still pointed to `sub2api-green:8080`, which was running `sub2api:v0.1.146.4`.

Verification:
- PASS: `docker image inspect sub2api:v0.1.146.5` showed `Version=v0.1.146.5` and `Revision=eff26f02c454dfee2ab588e752542aefe6a489fc`.
- PASS: recreated only idle `sub2api-blue` with `SUB2API_BLUE_IMAGE=sub2api:v0.1.146.5`.
- PASS: `http://127.0.0.1:18083/health` returned `200 {"status":"ok"}` and blue stayed `healthy`, restart count 0.
- PASS: candidate protected checks returned 401 for `GET /api/v1/admin/users`, `POST /responses`, and `POST /v1/responses`.
- PASS: public active health passed on `8080`, proxy bypass health passed on `18081`, and candidate health passed on `18083`.
- PASS: `GET /assets/ModelWhitelistSelector.vue_vue_type_script_setup_true_lang-BCLhTQ0Z.js` returned `200 text/javascript` and contained `gpt-5.6-sol`, `gpt-5.6-terra`, `gpt-5.6-luna`, `GPT-5.6 Sol`, `GPT-5.6 Terra`, and `GPT-5.6 Luna`.
- PASS: `docker exec sub2api-proxy nginx -t` passed and `docker exec sub2api-proxy nginx -s reload` succeeded.
- PASS: post-cutover 65-second blue/proxy critical log window had 0 matches, and blue stayed healthy with restart count 0.

Observed result:
- Active upstream is now `sub2api-blue:8080`.
- Online image is now `sub2api:v0.1.146.5`.
- Rollback target is `sub2api-green:8080` on `sub2api:v0.1.146.4`.

Limit:
- Authenticated browser click-through was not run because no reusable noninteractive admin login state was available.
- If a browser tab still shows the old list, it is using a cached route chunk and should reload the page.

## 2026-07-09T20:32:46+08:00 Devil - juhe-ai speed_first 首字节慢降级吸收验证

本轮将 juhe-ai speed_first 中可借鉴的“首字节慢观测、短期降级、成功恢复”落到 sub2api 现有 OpenAI path health / fast lane 调度中：慢首字节不计入硬故障熔断，只通过 FirstByteDegradedUntil 让账号候选和同 baseURL 路径短期降权；连续快首字节恢复后清除降级。未新增独立 soft deadline；现有 CodexWaitGuard/pre-output failover 已覆盖协议边界，后续如需更激进切换应基于该机制增强。

验证结果：
- 新增三条聚焦测试先红后绿，覆盖 tracker 降级/恢复、scheduler 降权、baseURL 排序后置。
- 相关 path-health/scheduler 回归通过。
- 因 `NewTokenRefreshService` 既有测试签名漂移阻挡 service 包编译，已机械补齐 Antigravity 参数位的 `nil`，并通过 token refresh/privacy retry 聚焦测试。
- server 编译门通过。

## 2026-07-09T22:18:00+08:00 Devil - OpenAI 首字节主动切换 Phase 2

本轮完成参考会话 `019f46ad-5e52-7b71-ac37-c147758cad8d` 中定义的第二阶段：当 OpenAI `/responses` 请求仍处于输出前 request-phase，且等待响应头/首字节超时时，网关记录慢首字节切换样本并返回可被 handler 消费的 `UpstreamFailoverError`，由现有 failover loop 切到下一个候选；客户端已取消或已收到真实输出时不做透明切换。

实现结果：
- `OpenAIPathHealthTracker.RecordFirstByteSlow` 用于显式慢首字节样本，触发 `FirstByteDegradedUntil`，但不增加硬故障计数和 circuit breaker 计数。
- 普通 OpenAI HTTP/SSE 与 passthrough request-phase 分支均写入账号、baseURL、聚合 bucket 的慢首字节状态。
- `UpstreamFailoverError.ActionMetadata` 增加 `first_byte_cutover_allowed`、`first_byte_cutover_phase`、`first_byte_wait_ms`、`degraded_account_id`、`degraded_base_url` 等审计字段。
- 新增 `docs/OPENAI_FIRST_BYTE_CUTOVER_PHASE2_CN.md` 记录行为边界、入口和验证结果。

验证结果：
- PASS: `go test -tags unit ./internal/service -run "TestOpenAIPathHealthFirstByte|TestOrderedOpenAIRequestBaseURLsDemotesFirstByteDegradedURL" -count=1`.
- PASS: `go test -tags unit ./internal/service -run "TestOpenAIGatewayServiceRequestPhaseFailoverCarriesActionMetadata|TestOpenAIGatewayService_ForwardRequestHeaderTimeoutReturnsFailover|TestOpenAIGatewayService_ForwardRequestPhaseContextCanceled|TestOpenAIGatewayService_APIKeyRequestBaseURLDoesNotFailoverBeforeAccountFailover" -count=1`.
- PASS: `go test -tags unit ./internal/service -run "TestOpenAIPathHealth|TestBuildOpenAIAccountLoadPlan|TestOpenAIAccountScheduleProfile" -count=1`.
- PASS: `go test -tags unit ./internal/handler -run "TestOpenAIHandleFailoverExhausted|TestOpenAIEnsureForwardErrorResponse|TestOpenAIHandleStreamingAwareError" -count=1`.
- PASS: `go test -tags unit ./cmd/server -run TestNonExistent -count=0`.
- PASS: `git diff --check`，仅有既有 CRLF warning。

Limit:
- 本轮只完成本地实现和验证；未提交、未构建镜像、未部署、未做线上流量切换。
## 2026-07-09 Devil - OpenAI first-byte protected cutover phase 2 handoff verification

- Scope: local implementation verification and documentation check only; no commit, image build, deployment, or online cutover was performed.
- CodeGraph status: healthy (`2231` indexed files, `70184` nodes, `184477` edges).
- Obsidian prefeed: Local REST API available; read `Projects/00-项目总览.md` and `Areas/开发知识库/00-总览.md`.
- Focused tests:
  - `go test -tags unit ./internal/service -run "TestOpenAIPathHealthFirstByte|TestOrderedOpenAIRequestBaseURLsDemotesFirstByteDegradedURL" -count=1` -> pass.
  - `go test -tags unit ./internal/service -run "TestOpenAIGatewayServiceRequestPhaseFailoverCarriesActionMetadata|TestOpenAIGatewayService_ForwardRequestHeaderTimeoutReturnsFailover|TestOpenAIGatewayService_ForwardRequestPhaseContextCanceled|TestOpenAIGatewayService_APIKeyRequestBaseURLDoesNotFailoverBeforeAccountFailover" -count=1` -> pass.
  - `go test -tags unit ./internal/service -run "TestOpenAIPathHealth|TestBuildOpenAIAccountLoadPlan|TestOpenAIAccountScheduleProfile" -count=1` -> pass.
  - `go test -tags unit ./internal/handler -run "TestOpenAIHandleFailoverExhausted|TestOpenAIEnsureForwardErrorResponse|TestOpenAIHandleStreamingAwareError" -count=1` -> pass.
  - `go test -tags unit ./cmd/server -run TestNonExistent -count=0` -> pass.
- Static diff check: `git diff --check` -> pass, with CRLF conversion warnings only.
- Documentation: `docs/OPENAI_FIRST_BYTE_CUTOVER_PHASE2_CN.md` exists; it is ignored by `.gitignore:130 docs/*` and must be force-added if committed.
- JSONL bookkeeping: latest lines of `docs/feature_list.jsonl` and `docs/process_list.jsonl` parse successfully with `ConvertFrom-Json`.

## 2026-07-09T23:32:00+08:00 Devil - Grok 账号可编入 OpenAI 分组并按 OpenAI 兼容请求调度

本轮修复 Grok 账号放入 OpenAI 分组后无法进入调度候选池的问题。Grok 请求协议沿用 OpenAI 兼容格式，因此 OpenAI 分组的候选账号平台扩展为 `openai + grok`；Grok 平台分组仍只查询 `grok` 账号。Grok 账号仍仅支持 `chat/completions` 能力，不放开 `/responses`。

实现结果：
- `OpenAIGatewayService.listSchedulableAccounts` 在 OpenAI 请求平台下使用多平台账号查询，覆盖普通 repo 与 scheduler snapshot 路径。
- `isOpenAICompatibleAccountForPlatform` 允许 OpenAI 请求平台匹配 Grok 账号，同时保持 Grok 请求平台不反向匹配 OpenAI 账号。
- `SupportsOpenAIImageCapability("")` 将空图片能力视为“不要求图片能力”，避免 Grok chat/completions 被空能力误拒绝。
- `GroupSelector` 在账号平台为 `grok` 时显示 `openai` 与 `grok` 分组，支持管理端直接把 Grok 账号编入 OpenAI 分组。

验证结果：
- RED: `go test ./internal/service -run 'TestOpenAIGatewayServiceListSchedulableAccountsIncludesGrokForOpenAIGroup|TestOpenAIAccountSchedulerSelectsGrokForOpenAIChatCompletionsOnly' -count=1` 初次失败，OpenAI 分组只返回 OpenAI 账号，Grok 调度返回 `no available OpenAI accounts`。
- RED: `npm run test:run -- src/components/common/__tests__/GroupSelector.spec.ts` 初次失败，Grok 账号只显示 Grok 分组。
- PASS: `go test ./internal/service -run 'TestOpenAIGatewayServiceListSchedulableAccountsIncludesGrokForOpenAIGroup|TestOpenAIAccountSchedulerSelectsGrokForOpenAIChatCompletionsOnly' -count=1`。
- PASS: `go test ./internal/service -run 'TestOpenAIGatewayService.*Grok|TestOpenAIAccountScheduler' -count=1`。
- PASS: `go test ./internal/service -run 'TestOpenAISelectAccountWithLoadAwareness_FiltersUnschedulable|TestOpenAIGatewayService.*Grok|TestOpenAIAccountScheduler' -count=1`。
- PASS: `go test ./internal/service -run 'TestOpenAIGatewayService_ListOpenAIAccountSchedulingPool|TestOpenAIGatewayService_ManualProbeSuccessRestoresSchedulingPoolHealth' -count=1`。
- PASS: `go test ./internal/service -count=1`。
- PASS: `npm run test:run -- src/components/common/__tests__/GroupSelector.spec.ts`。
- PASS: `npm run typecheck`。
- PASS: `git diff --check`，仅有既有 CRLF warning。

Limit:
- 本轮只完成本地实现和验证；未提交、未构建镜像、未部署、未做真实 Grok 上游请求。

## 2026-07-10T00:18:00+08:00 Devil - Grok OpenAI-compatible group scheduling release v0.1.146.6

本轮将 Grok 账号可编入 OpenAI 分组并参与 OpenAI 兼容 `chat/completions` 调度的修复提交、构建并发布到线上。发布镜像从已提交 `ac5d8054855c` 归档构建，未包含工作树中其他未提交的 OpenAI first-byte/compact 等改动。

验证结果：
- PASS: commit `ac5d8054855c` (`fix(grok): 支持 OpenAI 分组兼容调度`)。
- PASS: `go test ./internal/service -run 'TestOpenAIGatewayServiceListSchedulableAccountsUsesRequestedPlatform|TestOpenAIGatewayServiceListSchedulableAccountsIncludesGrokForOpenAIGroup|TestOpenAIAccountSchedulerSelectsGrokForOpenAIChatCompletionsOnly|TestOpenAIAccountSchedulingPool' -count=1`。
- PASS: `go test -tags unit ./internal/service -run 'TestSchedulerSnapshotDefaultBucketsIncludesOpenAIMixedBuckets|TestSchedulerSnapshotRebuildBucketsForPlatformIncludesOpenAIMixedBucket' -count=1`。
- PASS: `npm run test:run -- GroupSelector.spec.ts`。
- PASS: `npm run typecheck`。
- PASS: `git diff --cached --check`。
- NOTE: dirty-worktree `go test -tags unit ./internal/service -count=1` still fails in existing `GatewayService_GroupResolution_*` tests; release image is built from committed HEAD, not dirty worktree.
- PASS: `docker build` produced `sub2api:v0.1.146.6`, revision `ac5d8054855c`, image ID `sha256:88eda0006b3031bea41baaee6fcf57ced88266081a76dfe977a6a1214f1b53cb`。
- PASS: idle green candidate `18082` health/static/protected-route smoke passed.
- PASS: green candidate remained healthy for 65 seconds, restart count 0, critical logs 0.
- PASS: proxy upstream switched from `sub2api-blue:8080` to `sub2api-green:8080`; `nginx -t` and reload passed.
- PASS: post-cutover `8080/18081/18082` health and unauthenticated protection checks passed.
- PASS: post-cutover green/proxy 65-second critical log window had 0 matches.

Observed result:
- Active upstream is now `sub2api-green:8080`.
- Online image is now `sub2api:v0.1.146.6`.
- Rollback target is `sub2api-blue:8080` on `sub2api:v0.1.146.5`.

Limit:
- Authenticated browser click-through and real Grok upstream request were not run because no reusable noninteractive admin login state or real Grok request credentials were available in this turn.

## 2026-07-10T07:30:00+08:00 Devil - v0.1.147 P0 完成与 v0.1.149 拉取评估

本轮完成 v0.1.147 P0 的本地收口，并安全拉取 v0.1.149 tag 做逐点吸收评估。未在当前大量未提交改动的工作树中执行 merge/cherry-pick。

实现结果：
- `GatewayService.Forward` 和 `ForwardCountTokens` 在 `*gin.Context == nil` 时不再读取 header 或写 context。
- Claude OAuth identity fingerprint 和 tool-name rewrite context 写入增加 nil guard。
- `handleErrorResponse` 的 passthrough、400 body 和统一 JSON 错误写回只在 HTTP context 存在时执行；无 context 时返回 Go error。
- 上游错误体读取失败会记录日志，不再静默吞掉 read error。
- 新增 `gateway_nil_context_test.go`，覆盖普通上游错误、400 错误体和 count_tokens 传输错误。

验证结果：
- PASS: v0.1.147 P0 service 聚焦测试。
- PASS: 流内 SSE Ops handler 聚焦测试。
- PASS: Google API Key middleware 聚焦测试。
- PASS: 支付状态/支付流程/路由守卫/auth store 前端 79 tests。
- PASS: 前端 `vue-tsc --noEmit`。
- PASS: `cmd/server` 编译切片。
- FAIL (既有非本轮基线): 完整 `internal/service` unit 包只有 3 个 `GatewayService_GroupResolution_*` 用例失败，均为 expected repository call count 1 / actual 0；147 P0 聚焦用例全部通过。

v0.1.149 拉取结果：
- PASS: `git fetch origin --prune --tags`。
- tag: `v0.1.149` -> `dd1a116f4879992f04bb6ddbc77069bd503c3f4a`。
- 上游没有 `v0.1.148` tag。
- `v0.1.147..v0.1.149`: 72 files changed, 3467 insertions, 195 deletions。
- 已生成 `docs/V0.1.149_ABSORPTION_PLAN_CN.md`，P0 为 compact JSON->SSE bridge 和 response.failed 语义错误透传；版本在线回退不适合当前 immutable image + blue/green 发布链路。

边界：
- 未提交、未构建镜像、未部署、未推送、未执行真实上游请求。
- 工作树原有无关脏改保持未处理。

## 2026-07-10T08:19:07+08:00 Devil - 并行吸收 v0.1.147 剩余优化与 v0.1.149 P0

本轮继续按功能片段吸收上游行为，不执行整 tag merge。v0.1.149 P0 的 compact JSON -> Responses SSE bridge 和 `response.failed` 语义错误透传已经落入当前单体网关结构；同时补齐 v0.1.147 剩余的 Web Search 历史块过滤、Antigravity 生产端点默认值、Codex call-input ID 清理、Responses/Chat `response_format` 互转、`namespace=image_gen` 识别和 CRS 180 秒请求超时。

用户可见行为：
- Codex body-signal compact 请求在原始 `stream:true` 时会把上游 unary JSON 转为合法 Responses SSE，逐项发送 `response.output_item.done`，最后发送 `response.completed`；路径型 compact 和非 2xx 错误仍保持原 JSON/HTTP 语义。
- Responses、passthrough、Chat Completions 和 Messages 路径遇到流内 `response.failed` 时，会按错误语义映射 400/401/403/429/502/503，并使用账号真实平台匹配 OpenAI/Grok 错误透传规则；已写出 heartbeat 的流保持 HTTP 200 并发送流内失败事件。
- 历史消息中的本地模拟 Web Search 块会在所有 Anthropic 上游请求前移除；DeepSeek、Kimi、Moonshot、GLM、MiniMax 和 Qwen thinking 类模型还会移除真实 Web Search 块，避免第三方兼容上游返回 400；保留文本摘要和其他工具块。
- Antigravity 默认转发到生产端点，只有显式设置 `daily` 或 `sandbox` 才使用联调端点。
- Codex 续链输入只保留合法 `fc*` call-input ID，Chat/Responses 的 `json_schema` 等结构化输出格式可以双向转换，`namespace=image_gen` 同时支持顶层 tools 和 `input.additional_tools`。
- 管理端 CRS 同步请求超时调整为 180 秒，避免大量账号同步被默认超时提前中断。

验证结果：
- PASS: v0.1.147/v0.1.149 组合 service 聚焦测试。
- PASS: compact body-signal 和 Ops/流内错误 handler 聚焦测试。
- PASS: 完整 `internal/pkg/apicompat`、`internal/pkg/antigravity` 和 `internal/pkg/httputil` unit 包。
- PASS: CRS API Vitest 17 tests 和前端 `vue-tsc --noEmit`。
- PASS: `cmd/server` 编译门。
- PASS: `gofmt -d` 无输出，`git diff --check` exit 0，仅有既有 CRLF warning。
- FAIL (既有非本轮 service 基线): 3 个 `GatewayService_GroupResolution_*` 测试仍为 expected repository call count 1 / actual 0。
- FAIL (既有非本轮 handler 基线): 2 个 failover retry-window 测试仍期望至少 1 分钟而当前为 30 秒；4 个 WebSocket 测试桩缺少 `ListSchedulableByPlatforms`，导致服务端 panic 后客户端 EOF。

边界：
- 当前完成的是代码吸收和本地验证。
- 未提交、未构建镜像、未部署、未推送，也未执行需要真实凭证的上游请求。

## 2026-07-10T09:29:53+08:00 Devil - Codex 最新模型/思考强度同步与失败用量收口

本轮以本机实时 `C:\Users\27404\.codex\models_cache.json` 和当前 Codex 可调用能力清单交叉核对模型事实。缓存客户端版本为 `0.144.0`，抓取时间为 `2026-07-10 00:25:41`；两处来源对以下组合一致：

- `gpt-5.6-sol`: 默认 `low`，支持 `low, medium, high, xhigh, max, ultra`，上下文 `372000`。
- `gpt-5.6-terra`: 默认 `medium`，支持 `low, medium, high, xhigh, max, ultra`，上下文 `372000`。
- `gpt-5.6-luna`: 默认 `medium`，支持 `low, medium, high, xhigh, max`，上下文 `372000`；不声明 `ultra`。

实现与用户可见行为：

- OpenCode 配置生成器加入三种 GPT-5.6 模型、对应上下文/输出限制和精确 variants。
- 后端 reasoning effort 提取保留 `max`、`ultra`，不再把新强度归一化为空；模型后缀如 `gpt-5.6-terra-ultra` 也可正确提取。
- Responses、Chat Completions、Messages 在 `response.failed` 或其他失败返回中已观察到 usage/图片用量时，失败尝试进入既有计费链路且只提交一次；未观察到真实用量时不计费。
- 上游已经写出 JSON/SSE 语义错误时，handler 不再追加泛化错误体；compact body-signal 在客户端要求流式时可桥接为 Responses SSE。

### 对脑龄和认知训练的影响

- **直接代码/数据库影响：无。** CodeGraph、目标 diff 和迁移目录均未发现脑龄或认知训练模块，本轮没有修改其评分、训练计划、题库、用户档案或结果表。
- **模型选择的间接影响：有。** 如果脑龄或认知训练服务通过 sub2api 调用 OpenAI/Codex，调用方可选择三种 GPT-5.6 模型及其受支持的思考强度；输出质量、延迟和费用可能随模型/强度改变。
- **失败计费的间接影响：有。** 如果上游失败事件已带 usage，调用方余额、配额和 usage 记录会反映实际消耗；同一失败尝试不得重复扣费。
- **长会话的间接影响：有。** 使用 `/responses/compact` 且 `stream:true` 的认知训练长会话会收到标准 Responses SSE 事件，不再收到与客户端预期不一致的 unary JSON。
- **错误处理的间接影响：有。** 调用方可能收到更准确的 400/401/403/429/502/503 或流内 `response.failed`，应按协议错误处理，不应只判断固定泛化错误文案。

### 脑龄和认知训练建议测试

| 层级 | 测试输入 | 核心断言 |
| --- | --- | --- |
| 模型目录/配置 | 生成 OpenCode 配置并读取模型白名单 | 三个模型均可见；Sol/Terra 有 `ultra`，Luna 无 `ultra`；上下文为 `372000` |
| 脑龄回归 | 使用脱敏的同一份历史评估输入，分别走旧默认模型和三种 GPT-5.6 | HTTP/JSON 契约不变；脑龄字段、单位、范围和必填项满足脑龄项目自身约束；原始评估记录不被覆盖 |
| 认知训练回归 | 使用同一脱敏用户画像生成训练计划、单次训练反馈和长会话续接 | 训练项目、难度、时长、顺序等字段符合认知训练项目契约；已有训练历史不被改写；compact SSE 事件顺序合法 |
| 思考强度 | Sol/Terra 测 `low/medium/high/xhigh/max/ultra`，Luna 测到 `max` | 请求被接受且 usage 中记录期望强度；Luna 配置不暴露 `ultra` |
| 失败用量 | 模拟 `response.failed` 携带 usage、无 usage、图片 usage 三类响应 | 携带用量时现有 usage/余额链路只记一次；无用量时不记；客户端只收到一次语义错误 |
| 真实上游冒烟 | 每个模型选择一个低成本提示，覆盖非流式、流式和 compact | 返回模型可用、事件终止完整、usage 可解析、无重复错误体；记录延迟和费用基线 |

脑龄和认知训练的业务契约测试必须在对应项目仓库执行；本仓库只能验证网关协议、模型目录、计费和错误语义，不能证明业务评分或训练算法正确。

### 已有数据与新表用途

- 本轮 **没有新增表**，也没有 migration、SQL、Ent schema 或数据回填，因此不存在“把已有数据迁入新表”的用途。
- 现有用户、API Key、账号、脑龄结果、认知训练历史和 usage 数据继续保留在各自原表；模型清单和 OpenCode variants 是代码配置，不创建模型数据表。
- `response.failed` 中观测到的 token/图片用量复用现有 `RecordUsage`、usage log、余额/配额链路，只会新增正常的既有表记录或更新既有余额，不写入任何新表。
- 若“新表”指脑龄或认知训练另一个仓库中的设计，需要切换到该仓库并根据真实 schema、迁移文件和字段映射另行编写数据用途说明；本轮不虚构映射。

验证结果：

- PASS: `go test -count=1 -tags=unit ./internal/pkg/apicompat`、`./internal/pkg/antigravity`、`./internal/pkg/httputil`。
- PASS: `go test -count=1 ./cmd/server`。
- PASS: compact、`response.failed`、Messages fallback、失败用量 helper 和 `max/ultra` reasoning 的 service/handler 聚焦测试。
- PASS: `npx vitest run src/components/keys/__tests__/UseKeyModal.spec.ts src/composables/__tests__/useModelWhitelist.spec.ts`，2 files / 18 tests。
- PASS: `npm run typecheck`、目标前端文件 ESLint、目标 Go 文件 `gofmt -l` 无输出、`git diff --check` exit 0。
- FAIL（既有 service 基线，已单独复现）: 3 个 `GatewayService_GroupResolution_*` 仍为 expected 1 / actual 0。
- FAIL（既有 handler 基线，已单独复现）: 2 个 retry-window 断言仍要求至少 1 分钟而当前是 30 秒；4 个 WebSocket 旧桩仍在 `ListSchedulableByPlatforms` panic 后返回 EOF。

边界：本轮完成本地代码与验证收口；未提交、未构建镜像、未部署、未推送，也未执行脑龄/认知训练仓库或真实上游业务测试。

## 2026-07-10T10:19:35+08:00 Devil - v0.1.146.7 提交、构建、蓝绿部署与验证

- PASS：选择性提交 `ed7b48d53`（147/149 网关与模型）、`bba25335f`（支付与 Google 鉴权）、`c98052790`（吸收验证文档）；未提交 `.codegraph/daemon.pid`、`backend/cmd/codex-live-probe/`、`tmp_body.json`。
- PASS：发布前 service/handler/shared packages/middleware/server 聚焦验证通过；前端 7 files / 114 tests、typecheck、ESLint 通过。
- PASS：从 committed HEAD `c98052790ca0` 构建 `sub2api:v0.1.146.7`，image ID `sha256:396be554d6823724faa5d840acd079b4526d87a9bcaec3e1770835d97b33ed24`。
- PASS：idle blue `18083` 候选 health/static/401 冒烟通过，镜像正确、healthy、restart count 0。
- INVESTIGATED：首次候选日志窗口出现一次 `openai_request_snapshots` 清理 10 秒超时；只读 DB 证据为 1673/1673 rows expired、relation 3510 MB、`expires_at` 索引存在，根因为大请求体批量删除超过共享 10 秒 context，不是新镜像启动或请求路径故障。
- PASS：新的独立 75 秒候选窗口关键日志 0，未重建或覆盖不可变镜像。
- PASS：代理 upstream 从 green 切换到 blue；nginx 配置测试/reload 成功。
- PASS：切流后 `8080/18081/18083` health 200，admin 和 Responses 未授权均为 401；65 秒 blue/proxy 关键日志 0。
- PASS：公网 KeysView bundle 精确验证 Sol/Terra 支持 `max/ultra`，Luna 支持 `max` 且无 `ultra`。
- PASS：容器二进制版本为 `v0.1.146`、image version `v0.1.146.7`、commit `c98052790ca0`。
- STATE：active=`sub2api-blue:8080 sub2api:v0.1.146.7`；rollback=`sub2api-green:8080 sub2api:v0.1.146.6`。
- LIMIT：未 push、未执行真实认证 GPT-5.6 上游请求；快照清理 backlog 作为后续维护风险保留。
## 2026-07-10T12:11:25+08:00 Devil - v0.1.149 吸收完成与版本切换

本轮完成 v0.1.149 已选 P0/P1 吸收项收口，并将 `backend/cmd/server/VERSION` 从 `v0.1.146` 改为 `v0.1.149`。P2 在线二进制回退不吸收，继续使用不可变镜像和 blue/green 代理回滚。

实现结果：
- Ops 错误列表接通 `user_id`、`api_key_id`、`model`、`category`、`sort_by`、`sort_order`，筛选最终进入 repository SQL。
- 普通错误列表即使筛选 `phase=upstream` 也保留客户端可见状态守卫；只有上游专用列表显式包含 status<400 的恢复态记录。
- Ops 排序使用 `created_at/model/status_code` 白名单，并追加同方向 `e.id` 作为稳定分页键。
- 用量页错误标签每次进入重新查询，排行首次进入懒挂载、后续每次进入主动刷新；筛选变化继续自动刷新排行。
- `UsageRequestType` 与 v0.1.149 的 `cyber` 请求类型保持一致。
- `docs/V0.1.149_ABSORPTION_PLAN_CN.md` 已更新为最终完成清单和数据用途说明。

验证结果：
- PASS: Ops handler 红绿测试，覆盖筛选透传、非法用户/Key、恢复态开关和排序参数。
- PASS: Ops repository 红绿测试，覆盖状态守卫、`cyber_policy` 例外、排序白名单和稳定分页。
- PASS: Grok/角色 service 聚焦测试。
- PASS: compact/`response.failed` service 与 handler 聚焦测试。
- PASS: dashboard 用户排行、Ops handler、usage/ops repository 和 `internal/pkg/xai` 聚焦测试。
- PASS: `go test -tags unit ./cmd/server -run TestNonExistent -count=0`。
- PASS: 前端 7 个 Vitest 文件，43/43 tests。
- PASS: `npm run typecheck`。
- PASS: 本轮变更文件 scoped ESLint。
- PASS: `gofmt -d` 无输出，`git diff --check` 通过。
- PASS: `go run ./cmd/server -version` 输出 `Sub2API v0.1.149 (commit: unknown, built: unknown)`。
- PASS: 工作树没有 migration/schema 变更。
- BASELINE: 全量 `npm run lint:check` 仍有 207 个与本轮无关的 Settings prop mutation/旧测试保留名错误；本轮新增的 3 个 UsageFilters 错误已修复，scoped ESLint 为 0。

数据边界：
- 无新表、无 migration/schema/SQL、无历史数据迁移。
- 既有 `usage_logs`/usage 聚合继续用于 Token 排行、费用和余额；`ops_error_logs` 继续用于错误页；`users`、`api_keys`、`accounts`、`groups` 继续提供角色与筛选维度。
- 对脑龄和认知训练无直接代码或表结构变更；若它们通过 sub2api 调用模型，只会间接受到模型、错误语义、延迟和费用统计变化影响。

边界：
- 未提交、未构建镜像、未部署、未推送；当前 HEAD 仍为 `558cac0f4a4f`。

## 2026-07-10T13:08:35+08:00 Devil - v0.1.149.1 提交、构建、部署与验证

提交结果：
- `70d6606ef feat(grok): 吸收图片配额与模型改进`。
- `37a994938 feat(admin): 增强角色与用量观测`。
- `e56621837 chore(version): 切换服务版本至 v0.1.149`。

构建结果：
- PASS：从 committed HEAD `e56621837035` 构建 `sub2api:v0.1.149.1`。
- PASS：镜像 ID `sha256:a278012b8ecc621fe2ceeb728b6984c00639f8de850d0b4013ee1842b0825c8a`。
- PASS：label version/revision 和容器二进制 `v0.1.149 / v0.1.149.1 / e56621837035` 一致。

部署结果：
- PASS：仅把 idle green 重建为 `sub2api:v0.1.149.1`，active blue、PostgreSQL、Redis、proxy 未重启。
- PASS：候选 `18082` 完整冒烟和独立 65 秒日志窗口通过。
- PASS：代理切到 `sub2api-green:8080`，nginx 配置校验和 reload 通过。
- PASS：切流后 `8080/18081` 完整冒烟、线上 UsageView bundle 和独立 65 秒 green/proxy 日志窗口通过。
- PASS：最终 green/blue 均 healthy、restart 0；active green，rollback blue `sub2api:v0.1.146.7`。

边界：
- 未执行 Git push、镜像 registry push、真实认证上游请求或带管理登录态的浏览器点击。

## 2026-07-10T21:27:02+08:00 — Devil
- 本轮目标：选择性吸收 sub2api v0.1.150 更新；所有 OpenAI 探测默认模型为 gpt-5.6-terra。
- 验证：后端 service 全量、单位/集成聚焦测试、DTO/routes/server、前端 10 个规格和 typecheck 通过；gofmt、diff check 通过。
- 审查修复：SubscriptionService.Stop 现在会取消订阅缓存 Pub/Sub 监听上下文；回归测试已先失败后通过。
- 边界：未提交、未构建镜像、未部署、未推送；未执行真实认证上游请求。
- 遗留：AccountsView Vitest 的 common.time.never 英文 locale 警告为既有问题，未在本轮无关范围内修改。

## 2026-07-10 v0.1.150.1 发布验证 — Devil

- 代码提交：`4db1f5a4467b914fa25fa57c52385cf07bb4e017`；发布镜像：`sub2api:v0.1.150.1`。
- 镜像验证：OCI version/revision 标签、容器内 `-version` 的 image version 和 commit 均与发布目标一致。
- 蓝绿：构建前 active=green（`v0.1.149.1`）；只重建 idle blue（`v0.1.150.1`）；候选连续健康超过 60 秒后切流，green 保持为回滚实例。
- 候选与线上：18083、8080、18081 的 health/root=200；未登录管理版本 API、`/responses`、`/v1/responses`=401；三个入口的真实入口 chunk SHA-256 一致。
- 日志：候选和切流后的独立 75 秒窗口中未匹配 panic、fatal、migration failure、checksum、pq、端口绑定或 rebuild failure；blue/green 均 healthy，RestartCount=0。
- 边界：未执行真实认证上游请求、管理员登录态浏览器操作、Git push 或 registry push。

## 2026-07-11 16:12 +08:00 - superapi-buzz 新增两个签到账号

- 执行者：Devil。
- 存储边界：签到工具账号配置实际存放在 PostgreSQL `newapi_checkin_sites` / `newapi_checkin_accounts`，本轮未修改 Go、Vue、migration 或 schema。
- 备份：`D:\sub2api-deploy\backups\newapi-checkin-before-superapi-20260711-161134.dump`，包含 8 张 NewApi 签到相关表，大小 50919 字节。
- PASS：用户 `10478`、`10479` 的 `/api/user/self` 鉴权成功，识别为 `jjlin`、`peterzhu`。
- PASS：事务幂等写入后，`superapi-buzz` 从 5 个账号增加到 7 个；新增行启用，线路为 `ip-slot-22`、`ip-slot-23`，凭据仅做脱敏核验。
- PASS：两账号真实调用 `/api/user/checkin` 均返回“今日已签到”；现有 `queryCheckinStatusLocked` 会将该结果判定为 `CheckinOK=true`、`CheckedInToday=true`。
- PASS：线上 `http://127.0.0.1:8080/health` 返回 200，active 仍为 `sub2api-blue:8080` / `sub2api:v0.1.150.1`。
- 边界：未重启或重建应用、PostgreSQL、Redis、proxy；未提交、未构建、未部署、未推送。

## 2026-07-12 - 月度历史签到次数与奖励重复累计修复

- 执行者：Devil。
- 根因：数据库已用 `UNIQUE (site, user_id, checkin_date)` 保证物理记录唯一，但 `aggregateMonthly` 对传入记录直接累加；重复快照或旧数据进入汇总时会把同一账号同一天的次数和奖励重复计算。
- 修复：汇总入口先按 `site + user_id + checkin_date` 收敛，重复键采用最新记录，再生成站点月、账号月、站点日和账号日四类汇总。
- RED：新增重复账号日记录测试，修复前明确失败为 `expected 1, actual 2`。
- PASS：修复后重复输入在四类汇总中均为 `checkin_count=1`，奖励只计算一次。
- PASS：NewApi service、repository、handler 聚焦验证和 server 编译切片均退出 0。
- 线上数据核对：`newapi_checkin_monthly_records` 当前 461 行、461 个唯一账号日键、0 个重复组，不需要执行历史数据删除或 schema 变更。
- 边界：该段记录的是实现阶段；随后已按下述 v0.1.150.2 发布记录完成提交、构建与部署。

## 2026-07-12 - v0.1.150.2 提交、构建、部署与验证

- PASS：业务提交 `1fdd17980f32`，不可变镜像 `sub2api:v0.1.150.2` 构建成功，OCI 标签和二进制 image/commit 一致。
- PASS：仅重建 idle green；候选 `18082` health/home/静态资源为 200，管理版本 API、`/responses`、`/v1/responses` 未登录均为 401。
- PASS：候选独立 65 秒窗口内 14 次 healthy/restart 0，关键日志 0。
- PASS：nginx 配置检查与 reload 成功，active 从 blue 切到 green。
- PASS：切流后 `8080/18081/18082` 冒烟通过，三个入口的 `index-B5YO7pRB.js` SHA-256 均为 `e5626be17e00e8251d714e41c9210633b6066f9a67b2ffbf923cf7e1b3b94278`。
- PASS：切流后独立 65 秒窗口内 green 持续 healthy/restart 0，green/proxy 关键日志 0；blue `v0.1.150.1` 保持 healthy 作为回滚。
- PASS：PostgreSQL、Redis 未重启；无 schema、migration 或数据清理。
- 边界：未执行 Git push、镜像 registry push、管理员登录态页面操作或真实认证上游请求。

## 2026-07-13 - v0.1.152 选择性融合验证

- 执行者：Devil。
- 范围：按方案 1 选择性吸收上游 `v0.1.152` 的 compact 稳定性、Anthropic cache usage、Codex tool bridge / item ID / remote compaction / Messages identity、Fast/Flex、alpha/search、Grok cache identity、按次计费与最终 Grok 修复；保留本地探针、Ops/NewApi、动态 Codex 身份、alpha/search 计费和 VersionBadge 行为。
- PASS：Grok 计费回归完成 RED -> GREEN；`grok-4.20-*` 使用 `grok-4.3` 价格，Composer 使用 `grok-build-0.1` 价格，缓存输入均为 `0.2e-6`。
- PASS：`go test ./internal/repository -run 'GrokCLI|HTTPUpstream' -count=1`、`go test ./internal/service -run 'Grok|AccountBaseURL|Billing' -count=1`、`go test -tags unit ./internal/service -run 'Grok' -count=1`。
- PASS：`go test ./internal/pkg/apicompat -count=1`、`go test ./internal/service -count=1`、`go test ./internal/repository -count=1`、`go test ./cmd/server -count=1`。
- PASS：`npm run typecheck` 与 `npm run test -- --run`；Grok API Key 创建、OAuth、剩余容量进度条、UseKeyModal 和 VersionBadge 用例通过。
- 已知失败：`go test ./internal/handler -count=1` 仍有 6 个融合前已存在的失败，分别为 2 个 retry-window 断言和 4 个 WebSocket repository-stub / continuity 用例；本轮未修改对应实现。
- PASS：`backend/cmd/server/VERSION` 为 `v0.1.152`；VersionBadge 继续从 API/store 取主版本并规范为恰好一个 `v` 前缀。
- 边界：完成本地提交与验证；未构建镜像、未部署、未执行 Git push、registry push、真实认证上游请求或管理员登录态浏览器验证。

## 2026-07-13 - v0.1.152.1 提交、构建、部署与验证

- 执行者：Devil。
- PASS：从 committed HEAD `6bf0d9f51e0e` 构建不可变镜像 `sub2api:v0.1.152.1`；OCI version/revision 与容器二进制主版本、镜像版本、commit 一致。
- PASS：备份 `.env` 与 `active.conf`，只将 idle blue 从 `v0.1.151.1` 重建为 `v0.1.152.1`；active green、PostgreSQL、Redis、proxy 未重建。
- PASS：候选 `18083` 的 health/root/静态资源为 200，管理版本 API、`/responses`、`/v1/responses` 未登录为 401。
- OBSERVED：首次候选窗口持续 healthy/restart 0，但启动期出现一次 `pq: canceling statement due to user request`；保持 green active，未提前切流。
- PASS：后续独立 65 秒候选窗口持续 healthy/restart 0，关键日志 0。
- PASS：nginx 配置检查和 reload 成功，active 从 green 切换到 blue。
- PASS：切流后 `8080/18081/18083` 冒烟通过，三个入口的 `index-j8WaZ3Bq.js` SHA-256 均为 `0145B7D6134A5D9AFC8E518FCAC490E2ADB684A3EB136948A756B028E85A4AC5`。
- PASS：切流后独立 65 秒内 blue healthy/restart 0、proxy running/restart 0，联合关键日志 0；green `v0.1.151.2` 保持 healthy 作为回滚。
- PASS：本轮追加的 `docs/feature_list.jsonl` 与 `docs/process_list.jsonl` 尾记录均可独立解析。
- BASELINE：两份 JSONL 历史区各有 254 行早期编码/JSON 损坏；为避免 500 余行无关重写，本轮未修复历史记录。
- 边界：未执行 Git push、registry push、真实认证上游请求或管理员登录态页面操作；handler 仍保留 6 个已记录的既有失败。
## 2026-07-12 - NewApi 签到数据库直接去重

- 执行者：Devil。
- 备份：`D:\sub2api-deploy\backups\newapi-dedup-20260712-104749.dump`，包含月度明细和签到历史两张表，29081 字节。
- PASS：事务按 `site + user_id + checkin_date/date` 分组，每组仅保留 `created_at/id` 最新记录。
- 结果：`newapi_checkin_monthly_records` 删除 0 行，最终 461 行、重复组 0。
- 结果：`newapi_checkin_history` 删除 0 行，最终 339 行、重复组 0。
- 结论：线上数据已唯一，无历史重复数据需要清理；此前显示叠加由旧版汇总逻辑导致。

## 2026-07-12 - v0.1.151 吸收与本地验证

- 执行者：Devil。
- PASS：完整吸收 GPT-5.6 计费、cache-write 解析、Codex 身份配对、用户级 Fast/Flex、image_gen 清理、Grok reasoning 和 setup-token 刷新行为。
- PASS：主版本更新为 `v0.1.151`，不可变镜像版本目标为 `v0.1.151.1`。
- PASS：后端 service 全包及相关 pkg/repository/WS/middleware/handler/migrations/cmd server 测试。
- PASS：前端 Vitest 全量测试与 `npm run typecheck`。
- 发布前状态：active green=`sub2api:v0.1.150.2` healthy；idle blue=`sub2api:v0.1.150.1` healthy。

## 2026-07-12 - v0.1.151.1 构建阻断

- PASS：提交 `84bb9e56fb80`；migration 173 已定向执行并验证约束允许 request_type 0..4。
- FAIL：清华 Alpine 源 403/拒绝连接；USTC 下载 brotli-libs 时 TLS 中断；阿里云 Alpine 成功后 goproxy.cn 多项依赖 unexpected EOF。
- 按连续三次失败规则暂停；未生成镜像、未部署 idle blue、未切流。
- PASS：active green 仍为 `sub2api:v0.1.150.2` healthy，blue `v0.1.150.1` healthy。

## 2026-07-12 - VersionBadge 主版本同步

- RED：接口返回 `v0.1.151` 时组件显示为 `vv0.1.151`。
- PASS：主版本、最新版本和非管理员版本统一规范为恰好一个 `v` 前缀。
- PASS：VersionBadge 2 个 Vitest 用例通过，`npm run typecheck` 通过。
- 约束：以后发布只更新统一版本源和 `IMAGE_VERSION`，并验证侧边栏、下拉主版本及镜像版本。

## 2026-07-12 - v0.1.151.1 构建、蓝绿部署与线上验证

- 执行者：Devil。
- PASS：从 committed HEAD `92ba858d98e8` 构建 `sub2api:v0.1.151.1`；OCI version/revision 和容器内 `-version` 一致。
- PASS：仅重建 idle blue；`18083` health、首页和静态资源为 200，管理 API、`/responses`、`/v1/responses` 未登录为 401。
- PASS：候选持续 healthy、restart 0；一次启动期 `pq: canceling statement due to user request` 未重复，后续独立 120 秒关键日志窗口为 0。
- PASS：nginx 配置检查和 reload 成功，active 从 green 切到 blue。
- PASS：`8080`、`18081`、`18083` 完整冒烟通过；切流后 68 秒三个健康入口持续 200，blue healthy、restart 0，最近 120 秒关键日志为 0。
- PASS：green `sub2api:v0.1.150.2` 保持 healthy、restart 0 作为回滚；PostgreSQL、Redis 未重启。
- 边界：未执行 Git push、镜像 registry push、管理员登录态页面操作或真实认证上游请求。

## 2026-07-12 - 探测提示词随机快速验证题

- 执行者：Devil。
- PASS：新增 8 道低 token、答案明确的共享快速验证题，通用探测每次随机选择，不再固定发送 `hi` 或 `ok`。
- PASS：账号人工测试、Claude/Bedrock/OpenAI/Gemini、API Key Responses、调度耗尽、compact 和 API Key 快速体检均接入共享题库。
- PASS：监控高级请求模板每次打开随机选择快速验证题；中英文界面文案同步更新。
- PASS：`go test ./internal/service ./internal/handler/admin -count=1`。
- PASS：`npm run typecheck`。
- PASS：生产源码扫描未发现通用检测入口遗留固定 `hi`、`Respond with OK` 或 `只回复 ok`；需要精确判分的专项能力验证提示词保持不变。
- 边界：仅完成源码修改与本地验证，未提交、构建或部署。

## 2026-07-12 v0.1.151.2 蓝绿发布验证

- 执行者：Devil。
- PASS：业务提交 `dd489bb537d1`，不可变镜像 `sub2api:v0.1.151.2` 的 revision、版本标签和 ImageID 校验一致。
- PASS：仅重建 idle green，候选 `18082` 的健康页、首页、静态资源为 200，受保护管理 API 与 Responses 路径未登录为 401。
- PASS：候选独立观察 90 秒持续 healthy、restart 0，关键日志匹配 0。
- PASS：nginx 配置校验通过并从 blue 切流到 green。
- PASS：切流后 `8080`、`18081`、`18082` 冒烟通过，观察 75 秒后 green/blue 均 healthy、restart 0，新增关键日志匹配 0。
- PASS：PostgreSQL 和 Redis 未重启；blue `sub2api:v0.1.151.1` 保留为回滚目标。
- 风险：green 启动初期出现一次 request snapshot 清理 SQL 取消，后续两个干净观察窗均未复现；未执行真实认证上游请求。

## 2026-07-13 - OpenAI 模拟测验改用 Responses streaming

- 执行者：Devil。
- PASS：`responses_supported=false` 的 OpenAI API Key 账号人工测试不再分流到 `/v1/chat/completions`，统一请求 `/v1/responses`。
- PASS：请求体为 Responses `input` 结构并携带 `stream:true`，请求头显式携带 `Accept: text/event-stream`。
- PASS：Responses SSE 的 `response.output_text.delta` 与 `response.completed` 被现有解析器正确处理。
- PASS：协议相关 6 个 unit 用例和 service/server 非 unit 编译测试通过。
- 边界：图片测试和 compact 测试路径未修改；真实网关兼容与回退行为未修改。

## 2026-07-13 v0.1.152.2 蓝绿发布验证

- 执行者：Devil。
- PASS：业务提交 `9b42afcdf0e7`，不可变镜像版本、revision、二进制版本和 ImageID 一致。
- PASS：仅重建 idle green；候选 `18082` 冒烟及 75 秒 healthy/restart 0、关键日志 0 观察通过。
- PASS：nginx 配置检查通过并从 blue 切流到 green。
- PASS：切流后三入口冒烟和主 chunk SHA-256 一致；77 秒公网/green/proxy 观察稳定，关键日志 0。
- PASS：blue `v0.1.152.1` 保留为健康回滚目标；PostgreSQL、Redis 未重启。
- 边界：未执行管理员登录态真实上游模拟测验、Git push 或 registry push。

## 2026-07-13 - 君公益上游 403 诊断

- 执行者：Devil。
- PASS：确认账号 `494/君公益` 的人工测试已请求 `https://muyuan.do/v1/responses`，协议切换已生效。
- PASS：上游返回 Cloudflare HTML 403，页面明确包含 `Sorry, you have been blocked`，不是 sub2api 生成的错误或 OpenAI JSON 权限错误。
- PASS：不带凭证的首页 GET 和 `/v1/responses` POST 在普通 UA、Codex UA 下均稳定返回 403，排除 API Key、请求体和 UA 为主因。
- PASS：账号未绑定独立代理；本机 `muyuan.do` 解析为 Mihomo Fake-IP `198.18.0.170`，默认路由由 Mihomo TUN 接管，Cloudflare 识别出口 IP 为 `40.83.88.242`。
- PASS：同域名账号 `445` 的余额登录也返回 Cloudflare 403；账号 `494` 在 2026-07-06 曾有成功 usage 记录，说明当前故障是出口/WAF 状态变化。
- 结论：当前 Mihomo/Clash 出口被 `muyuan.do` Cloudflare 拦截；`upstream_abnormal` 是失败后的调度结果。

## 2026-07-13 - 账号新增/编辑表单简化

- 执行者：Devil。
- PASS：新增共享“基本设置 / 更多设置”Tab；新增和编辑弹窗每次打开均回到基本设置。
- PASS：基本设置保留名称、平台/账号类型、Base URL、API Key、模型限制和分组；备注、额外连接参数、代理/并发、配额、调度、错误处理和协议开关进入更多设置。
- PASS：API Key 字段组件按核心/高级区渲染，现有 API Key 管理、模型映射和提交载荷保持原逻辑。
- PASS：5 个聚焦测试文件共 37 项通过；ESLint、`vue-tsc --noEmit`、`git diff --check` 全部通过。
- PASS：1280x720 临时预览默认编辑弹窗无重叠或裁切，预览文件已删除，最终工作树未保留测试入口。
- 边界：未提交、未构建镜像、未部署；真实管理路由开发态需要登录，Browser 插件跨标签点击异常导致高级 Tab 浏览器截图未作为证据。

## 2026-07-13 - v0.1.152.3 蓝绿发布验证

- 执行者：Devil。
- PASS：业务提交 `edacdd5e729b`；不可变镜像 `sub2api:v0.1.152.3` 的 version、revision、二进制版本和 ImageID 一致。
- PASS：仅重建 idle blue；候选 `18083` 的 health、首页、静态资源为 200，管理与 Responses 受保护接口未登录为 401。
- OBSERVED：候选启动期出现一次 `pq: canceling statement due to user request`；后续独立约 66 秒窗口中 7 次 health=200、blue healthy/restart 0、关键日志新增 0。
- PASS：nginx 配置检查通过并从 green 切流到 blue；切流后三入口冒烟、主资源路径和 SHA-256 一致。
- PASS：切流后约 66 秒三入口持续健康，blue/proxy restart 0，关键日志新增 0；green 回滚容器 healthy/restart 0。
- PASS：管理员登录态页面唯一显示主版本 `v0.1.152` 和镜像版本 `v0.1.152.3`；新增/编辑弹窗默认基本设置，更多设置可切换，未提交账号数据。
- PASS：PostgreSQL、Redis 未重启。
- 边界：未执行真实认证上游请求、Git push 或 registry push。

## 2026-07-15 - 平台目录弹窗与 Key/分组管理

- 执行者：Devil。
- PASS：平台目录从主 Tab 移除，改为独立按钮打开原生大弹窗；主视图保留总览、签到记录、实时余额、月度历史。
- PASS：NewAPI 生成 Key 支持默认脱敏、按需显示完整值、读取 token 当前分组及更新分组。
- PASS：更新分组采用“重新读取完整 token -> 仅替换 group -> PUT /api/token/”，测试验证额度和模型限制字段未丢失。
- PASS：Go service/handler/server、Vitest 3/3、Vue typecheck、生产构建、桌面与 390px Playwright 验证通过，浏览器控制台错误为 0。
- OBSERVED：NewAPI 完整分组接口对当前 access key 返回权限不足，页面以 partial 状态展示已知分组；未伪装为完整列表。
- OBSERVED：sub2api 的 `sk-` 不具备用户 JWT 登录态，两个用户管理接口均返回 `401 INVALID_TOKEN`；页面标记为不支持并说明需要账号登录凭据。
- 边界：本轮未执行真实分组写入、Git commit、镜像构建或部署。

## 2026-07-15 sub-vcnovb 8 账号与完整 Key 展示验证 - Devil

- PASS：生产库写入前已有 SQL 备份；8 个账号幂等写入后，`sub-vcnovb` 共 9 个账号，邮箱标识和独立出口档位均已落库。
- PASS：完整访问 Key 只由受保护管理配置接口返回；页面默认展示 `sk-前2位***后4位`，按账号点击“显示/隐藏”切换。
- PASS：后端聚焦测试、server 构建、前端 Vitest/typecheck/build 全部通过。
- PASS：桌面 1564px 页面和 390x844 窄屏浏览器检查通过；窄屏 `document.scrollWidth === clientWidth`，目录表格使用内部横向滚动，完整 Key 正常换行。
- PASS：点击前完整 Key 不在 DOM；显示后 `aria-pressed=true` 且完整 Key 可见；隐藏后 `aria-pressed=false` 且完整 Key 从 DOM 移除。
- 边界：仅查询生产数据库进行脱敏复核，未调用目标站点上游、未签到、未提交、未构建镜像、未部署。

## 2026-07-15 v0.1.155.2 发布验证 - Devil

- PASS：committed HEAD `9e055eeba143` 构建镜像 `sub2api:v0.1.155.2`，ImageID=`sha256:ed19ef17dfdcce6235d03a354d3fc8b76062a9a97483093a1a5953477873d8c9`。
- PASS：最终二进制和 OCI 标签同时确认主版本 `v0.1.155`、镜像版本 `v0.1.155.2`、revision `9e055eeba143`。
- PASS：候选 green `18082` 完整冒烟与 65 秒稳定观察通过后切流；active=`sub2api-green:8080`，rollback=`sub2api-blue:8080`。
- PASS：切流后三入口完整冒烟、资源路径和 SHA-256 一致；65 秒健康采样及关键日志观察通过。
- PASS：green、blue 均 healthy/restart 0，proxy running/restart 0；PostgreSQL、Redis 未重启。
- PASS：线上静态主资源包含完整 Key 按需展示实现，不包含 8 个真实账号 Key；数据库保持 9/9 启用账号。
- LIMIT：管理员登录态过期，未验证受保护页面的最终交互和版本徽标；公开签到页桌面/移动布局已由 Chrome 验证。
- 边界：未执行上游请求、签到、套餐/模型刷新、Git push 或 registry push。

## 2026-07-15 - v0.1.155.1 sub2api 签到工具发布验证

- 执行者：Devil。
- PASS：业务提交 `ab937dd48b7f`；不可变镜像 `sub2api:v0.1.155.1` 的 version、revision、ImageID 与 active blue 一致。
- PASS：候选 `18083` 健康、静态资源和未授权路由冒烟通过；稳定观察后切流到 blue。
- PASS：切流后三入口主资源及 SHA-256 一致，连续 60 秒 health=200，blue/proxy restart 0 且新增严重日志 0。
- PASS：数据库备份后新增 `sub-vcnovb` 只读站点，后台签到关闭，账号 Key 仅脱敏复核为 `sk-90***84cd`。
- PASS：nginx 当前 upstream 为 `sub2api-blue:8080`，配置检查通过；green `sub2api:v0.1.155` 保持 healthy/restart 0。
- LIMIT：部署环境未配置管理员密码，未取得登录态页面截图；公开直链静态布局正常，但内联脚本受现有 CSP 限制。
- LIMIT：`internal/server` 全包测试存在既有测试桩接口缺口；未执行真实远端刷新、签到、模型生成、Git push 或 registry push。

## 2026-07-15 sub2api 只读数据源与账号标识边界 - Devil

- 已实现：`provider=sub2api` 只调用 `GET /v1/usage?days=30` 与 `GET /v1/models`，不进入签到、月度同步、NewAPI Key 枚举或名称同步流程。
- 已实现：套餐、余额、累计请求/Token/成本、到期时间和模型列表持久化到余额缓存并在页面展示。
- 已确认：当前目标站点的 `/v1/usage` 响应不包含 username/email，API Key 也不能访问 `/api/v1/auth/me`，因此不能仅凭现有 Key 自动取得用户名或邮箱。
- 已实现：管理端新增 `POST /admin/newapi-checkin/account-display-name`，平台目录可手工填写用户名或邮箱，复用现有 `display_name` 字段，不保存密码。
- 验证边界：此前 sub2 读取链路的 Go/Vitest/typecheck/build/browser 检查已通过；手工标识接口和最终页面变更按用户要求未继续检测，结果待用户自行验证。

## 2026-07-14 - 签到总览卡片响应式样式修复

- 执行者：Devil。
- PASS：站点卡片网格改为按容器宽度自动换列，不再依赖浏览器视口断点。
- PASS：站点标题和状态徽标允许合理换行，长站点名不会挤压或覆盖徽标。
- PASS：统计项按最小 92px 自动重排；金额使用 16px 等宽数字并保持单行，不再逐字符断行。
- PASS：目标 Vitest 2/2、Vue TypeScript 检查、生产构建和 `git diff --check` 全部通过。
- PASS：1600、1024、768、390px 浏览器几何检查均无页面、卡片、数值水平溢出，数值无多行。
- 边界：本轮未提交、未构建 Docker 镜像、未部署线上环境。

## 2026-07-14 - v0.1.152.6 蓝绿发布验证

- 执行者：Devil。
- PASS：业务提交 `ef9d5c851e30`；不可变镜像 `sub2api:v0.1.152.6` 的 version、revision、二进制版本和 ImageID 一致。
- PASS：仅重建 idle blue；候选 `18083` 的 health、首页、工具页和静态资源为 200，管理与 Responses 受保护接口未登录为 401。
- OBSERVED：候选启动期出现一次 `pq: canceling statement due to user request`；后续独立候选观察窗内未复现关键错误。
- PASS：nginx 配置检查通过并从 green 切流到 blue；切流后三入口完整冒烟、主资源路径和 SHA-256 一致。
- PASS：切流后 62 秒内 4 次三入口 health 均为 200，blue healthy/restart 0，proxy running/restart 0，blue/proxy 关键日志匹配 0。
- PASS：管理员登录态页面显示主版本 `v0.1.152` 和镜像版本 `v0.1.152.6`；签到平台目录展示 `API Key` 表头、`sk-前2位***后4位` 脱敏值和“未生成”。
- PASS：active blue=`sub2api:v0.1.152.6`；rollback green=`sub2api:v0.1.152.5`；PostgreSQL、Redis 未重启。
- 边界：未执行签到写入、上游 Key 创建、Git push 或 registry push。

## 2026-07-14 - 签到工具 API Key 脱敏展示

- 执行者：Devil。
- RED：service 测试先失败于 `APIKeys` 方法不存在；嵌入页测试先失败于账号目录没有 `sk-ab***5678`。
- PASS：新增 `GET /api/v1/admin/newapi-checkin/api-keys`，按 6 并发读取上游 token 列表，只返回名称和 `sk-前2位***后4位` 脱敏值。
- PASS：账号目录异步加载 API Key，不阻塞 SQL 配置、余额、历史和月度数据；无 Key 显示“未生成”，读取失败显示“读取失败”。
- PASS：service NewAPICheckin 测试、admin handler NewAPICheckin 测试、server 编译切片、嵌入页 Vitest 1/1、Vue typecheck 和 diff check 全部通过。
- PASS：handler 测试直接断言 HTTP 响应含 `sk-ab***5678` 且不含完整上游 Key。
- 边界：仅完成源码与本地自动化验证，未提交、构建镜像、部署或执行管理员登录态线上页面验证。

## 2026-07-14 - 全部签到站点与 Key 脱敏复核

- 执行者：Devil。
- PASS：从运行中 PostgreSQL 读取 5 个站点、42 个账号，随后实时请求各站点 `/api/status` 和各账号 `/api/token/?p=1&size=100`。
- PASS：5 个站点状态接口均成功；42 个账号 token 列表均成功读取，首次超时的 3 个 dawclaudecode 账号定向重试后成功。
- 结果：25 个账号已有至少 1 个生成的 API Key，17 个账号暂无 API Key。
- 脱敏规则：签到 access key 与生成的 API Key 均只展示前 2 位和后 4 位。
- 边界：全程只读，未创建、修改或删除任何上游 API Key，未修改数据库。

## 2026-07-14 - aiaiai 签到站点与账号写入验证

- 执行者：Devil。
- 备份：`D:\sub2api-deploy\backups\newapi-checkin-before-aiaiai-20260714-075511.dump`，包含 8 张 NewAPI 签到相关表，大小 54950 字节。
- PASS：`https://api.aiaiai001.com/api/status` 返回 CNY、`quota_per_unit=500000`。
- PASS：9 个 access key 均通过 `/api/user/self` 验证，返回用户 ID 与提交的 9 个 ID 一致。
- PASS：事务幂等写入 `aiaiai` 站点和 9 个启用账号，线路为 `ip-slot-36` 至 `ip-slot-44`，数据库仅做脱敏凭据核对。
- PASS：9 个账号真实调用 `/api/user/checkin` 均返回“今日已签到”。
- PASS：`/api/token/?p=1&size=100` 可读取账号已生成的完整 API Key；账号 525、619 各有 1 个 `codex` Key，其余 7 个账号当前没有已生成 Key。
- 边界：未在上游创建新 API Key，未重启应用、PostgreSQL 或 Redis，未修改 schema。

## 2026-07-13 - 模型设置独立 Tab

- 执行者：Devil。
- RED：第三个 Tab、创建弹窗模型隔离和编辑弹窗模型隔离测试均因目标行为缺失而失败；同轮其余 26 项通过。
- PASS：基本设置只保留核心账号字段，更多设置保留低频连接/调度字段，模型限制与模型映射统一进入独立“模型设置”Tab。
- PASS：API Key、OAuth、Vertex、Bedrock 和 Antigravity 的模型配置入口均绑定 `activeFormTab === 'models'`，API Key 凭证区在模型 Tab 隐藏。
- PASS：共享 Tab 支持三个选项的鼠标与方向键/Home/End 操作，默认值仍为 `basic`。
- PASS：账号表单 5 个聚焦测试文件、37 项测试，目标 ESLint、`vue-tsc --noEmit`、`git diff --check` 全部通过。
- 边界：本节仅记录源码与本地验证；镜像构建、候选部署和线上页面验证待后续发布步骤完成。

## 2026-07-13 - v0.1.152.5 蓝绿发布验证

- 执行者：Devil。
- PASS：`v0.1.152.4` 因二进制缺失 `image_version` 在候选部署前被拒绝，未部署、未切流、未覆盖重建。
- PASS：业务提交 `022bc50d240a`；不可变镜像 `sub2api:v0.1.152.5` 的 version、revision、二进制版本和 ImageID 一致。
- PASS：仅重建 idle green；候选 `18082` 的 health、首页、静态资源为 200，管理与 Responses 受保护接口未登录为 401。
- OBSERVED：候选启动期出现一次 `pq: canceling statement due to user request`；后续独立观察窗中 7 次 health=200、green healthy/restart 0、关键日志新增 0。
- PASS：nginx 配置检查通过并从 blue 切流到 green；切流后三入口完整冒烟、主资源路径和 SHA-256 一致。
- PASS：切流后 70 秒三入口持续健康，green/proxy restart 0，近 3 分钟精确严重日志过滤均为 0；blue 回滚容器 healthy/restart 0。
- PASS：管理员登录态页面显示主版本 `v0.1.152` 和镜像版本 `v0.1.152.5`；新增/编辑弹窗的基本、更多、模型设置三个 Tab 符合发布范围，未提交账号数据。
- PASS：PostgreSQL、Redis 未重启。
- 边界：未执行真实认证上游请求、Git push 或 registry push。

## 2026-07-15 - sub2api 登录凭据验证与分组管理补全

- 执行者：Devil。
- PASS：VC 修正密码登录成功，JWT 仅在单次请求链内使用且不持久化；读取到 1 个 Key、28 个可用分组。
- PASS：保存接口的配置摘要仅返回登录账号和 `has_login_password`，不返回密码；仓库当前改动中未检出新旧密码明文。
- PASS：后端 NewAPICheckin service/repository/handler/routes 聚焦测试、前端 Vitest 3/3、Vue typecheck 和 diff check 通过。
- PASS：平台目录弹窗在桌面与 390px 窄屏下可用，无前端 console warning/error。
- 边界：未调用签到接口，未执行真实 Key 分组更新，未提交、构建镜像或部署。
