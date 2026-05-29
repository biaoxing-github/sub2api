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
