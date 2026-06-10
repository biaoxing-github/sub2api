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
