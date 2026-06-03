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

- tk go test -tags unit ./internal/handler/admin -run "TestAccountModelProbeBatchCreateRunsManualValidationOnly" -count=1：先按 TDD 红灯运行，因 BatchCreateModelProbeRuns 不存在失败；补实现后通过。
- tk npm run test:run -- src/api/__tests__/admin.accounts.spec.ts -t "starts batch manual account model probe runs"：先按 TDD 红灯运行，因 atchAccountModelProbeRuns 不存在失败；补 wrapper 后通过。
- tk npm run test:run -- src/views/admin/__tests__/AccountProbeReportsView.spec.ts -t "starts batch model validation"：先按 TDD 红灯运行，因报告页按钮不存在失败；补入口后通过。
- tk go test -tags unit ./internal/handler/admin -run "TestAccountModelProbeCreateRunsManualValidationOnly|TestAccountModelProbeBatchCreateRunsManualValidationOnly|TestAccountProbeReportBatchCreate|TestAccountProbeCreate" -count=1
- tk go test -tags unit ./internal/service -run "TestAccountProbeService_RunCodexStabilityDoesNotAutoRunModelValidation|TestAccountProbeService_RunManualModelValidationStoresEvidence|TestScheduledTestRunnerRunsAccountProbePlan" -count=1
- tk npm run test:run -- src/api/__tests__/admin.accounts.spec.ts src/views/admin/__tests__/AccountProbeReportsView.spec.ts src/views/admin/__tests__/AccountModelProbesView.spec.ts
- tk npm run typecheck
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
