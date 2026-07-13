# 本地测试记录

## 2026-07-13 v0.1.152 选择性融合 - Devil

- PASS：Grok repository/service/unit 聚焦测试。
- PASS：`go test ./internal/pkg/apicompat -count=1`。
- PASS：`go test ./internal/service -count=1`。
- PASS：`go test ./internal/repository -count=1`。
- PASS：`go test ./cmd/server -count=1`。
- PASS：前端 `npm run typecheck`。
- PASS：前端 `npm run test -- --run`。
- KNOWN FAIL：`go test ./internal/handler -count=1` 的 2 个 retry-window 与 4 个 WebSocket stub/continuity 用例；均为融合前既有失败。
- 未执行：Docker 镜像构建、部署、线上冒烟、真实认证上游请求和管理员登录态浏览器验证。

## 2026-07-13 v0.1.152.1 本地发布验证 - Devil

- PASS：committed archive 镜像构建，OCI labels 与二进制版本一致。
- PASS：idle blue `18083` 完整未登录冒烟。
- OBSERVED：启动期一次 `pq: canceling statement due to user request`；未切流，后续独立 65 秒窗口关键日志 0。
- PASS：nginx 配置检查、reload 和 green -> blue 切流。
- PASS：`8080/18081/18083` 完整冒烟与入口 chunk SHA-256 一致。
- PASS：切流后 65 秒 blue/proxy 状态稳定且关键日志 0。
- PASS：两份 JSONL 本轮新增尾记录可解析；历史区各 254 行无效为既有基线，未在本轮重写。
- 未执行：管理员登录态浏览器验证、真实认证上游请求、Git push、registry push。

## 2026-07-13 OpenAI 模拟测验 Responses streaming - Devil

- RED：`go test -tags unit ./internal/service -run 'TestAccountTestService_OpenAIAPIKeyResponsesUnsupportedStillUsesResponsesStreaming' -count=1 -v`，旧实现仍走 `/v1/chat/completions`，按预期失败。
- PASS：同一目标测试在实现后通过，请求为 `/v1/responses`、`stream:true`、`Accept: text/event-stream`，并按 Responses SSE 解析输出。
- PASS：6 个协议相关用例覆盖成功、Codex 模拟头、4xx、超时、非 JSON SSE 和选中 API Key 禁用。
- PASS：`go test -tags unit ./internal/service -run 'TestAccountTestService_(OpenAIAPIKeyResponses|OpenAIResponsesPath|TestAccountConnectionWithResult)' -count=1`。
- PASS：`go test ./internal/service ./cmd/server -count=1`。
- 既有基线：扩展运行 `TestAccountTestService_OpenAI` 时 2 个 OAuth originator 断言仍期望 `codex_cli_rs`，实际为合并版本的 `Codex Desktop`；与本次 API Key 协议切换无关。

## 2026-07-13 v0.1.152.2 本地发布验证 - Devil

- PASS：从 committed HEAD `9b42afcdf0e7` 构建并核验不可变镜像 `sub2api:v0.1.152.2`。
- PASS：仅重建 idle green，候选 `18082` 完整未登录冒烟和 75 秒干净观察窗通过。
- PASS：nginx 配置检查、reload 和 blue -> green 切流通过。
- PASS：`8080/18081/18082` 完整冒烟、主 chunk SHA-256 一致和切流后 77 秒干净观察窗通过。
- PASS：green active 与 blue rollback 均 healthy/restart 0；PostgreSQL、Redis 未重启。
- 未执行：管理员登录态真实上游模拟测验、Git push、registry push。

## 2026-07-13 账号表单 Tab 简化验证 - Devil

- RED：新增 `AccountFormTabs`、名称/备注独立渲染、API Key 核心/高级字段分区、创建/编辑弹窗默认 Tab 测试；实现前 5 个测试文件按预期失败。
- PASS：`npm run test -- --run src/components/account/__tests__/AccountFormTabs.spec.ts src/components/account/__tests__/AccountBasicInfoFields.spec.ts src/components/account/__tests__/AccountAPIKeyCredentialsFields.spec.ts src/components/account/__tests__/CreateAccountModal.grok.spec.ts src/components/account/__tests__/EditAccountModal.spec.ts`，5 个文件、37 个测试全部通过。
- PASS：`npx eslint` 检查本轮 12 个 Vue/TS/i18n 文件，0 错误。
- PASS：`npm run typecheck`，`vue-tsc --noEmit` 退出码 0。
- PASS：`git diff --check`，无空白错误；CodeGraph 重索引后 2302 个文件、71562 个节点。
- PASS：临时预览入口在 1280x720 渲染真实编辑弹窗，默认仅显示核心字段，截图未见重叠、裁切或空白页；预览文件验证后已删除。
- 限制：开发端口真实管理路由需要登录；Browser 插件的点击动作错误落到先前登录标签，因此未取得可靠的高级 Tab 浏览器截图。Tab 点击、键盘切换和字段显隐由 Vitest 覆盖。
