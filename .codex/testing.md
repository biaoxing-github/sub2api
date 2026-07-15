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

## 2026-07-13 v0.1.152.3 蓝绿发布验证 - Devil

- PASS：从 committed HEAD `edacdd5e729b` 构建并核验不可变镜像 `sub2api:v0.1.152.3`，OCI version/revision 与容器二进制版本一致。
- PASS：仅重建 idle blue；候选 `18083` 完整未登录冒烟通过，启动期一次 `pq` 查询取消后独立约 66 秒窗口内 7 次 health=200、关键日志新增 0。
- PASS：nginx 配置检查、reload 和 green -> blue 切流通过；`8080/18081/18083` 完整冒烟与主资源 SHA-256 一致。
- PASS：切流后约 66 秒三入口持续健康，blue/proxy restart 0，关键日志新增 0；green 回滚容器 healthy/restart 0，PostgreSQL、Redis 未重启。
- PASS：Chrome 管理员登录态确认版本按钮 `v0.1.152`、镜像版本 `v0.1.152.3`，新增/编辑弹窗默认基本设置及更多设置切换符合预期，未保存账号数据。
- 未执行：真实认证上游请求、Git push、registry push。

## 2026-07-14 签到工具 API Key 脱敏展示 - Devil

- 用户行为：管理员打开签到工具账号目录后，页面后台读取各账号已生成的 API Key；有 Key 时展示 `sk-前2位***后4位`，无 Key 时展示“未生成”，读取异常时展示“读取失败”。
- RED：`go test ./internal/service -run TestNewAPICheckinAPIKeysMasksGeneratedKeys -count=1` 失败于 `svc.APIKeys undefined`。
- RED：`.\node_modules\.bin\vitest.cmd run src/views/admin/tools/newapiCheckinLegacy.generated.test.ts` 失败于页面未显示 `sk-ab***5678`。
- GREEN：`go test ./internal/service -run NewAPICheckin -count=1` 通过。
- GREEN：`go test ./internal/handler/admin -run NewAPICheckin -count=1` 通过，且响应体断言不包含完整上游 Key。
- GREEN：`go test ./cmd/server -run '^$' -count=1` 通过。
- GREEN：`.\node_modules\.bin\vitest.cmd run src/views/admin/tools/newapiCheckinLegacy.generated.test.ts` 通过，1/1。
- GREEN：`.\node_modules\.bin\vue-tsc.cmd --noEmit` 与 `git diff --check` 通过。
- 覆盖边界：本轮未执行真实管理员登录态接口或浏览器截图；未提交、构建镜像或部署。

## 2026-07-13 模型设置独立 Tab 验证 - Devil

- RED：三个目标测试文件中 3 项按预期失败，分别证明第三个 Tab 不存在、创建弹窗未隔离模型区、编辑弹窗默认仍显示模型配置；同轮其余 26 项通过。
- GREEN：`AccountFormTabs` 增加“模型设置”Tab，API Key、OAuth、Vertex、Bedrock 和 Antigravity 的模型限制/映射只在该 Tab 显示。
- PASS：`npm run test -- --run src/components/account/__tests__/AccountFormTabs.spec.ts src/components/account/__tests__/AccountBasicInfoFields.spec.ts src/components/account/__tests__/AccountAPIKeyCredentialsFields.spec.ts src/components/account/__tests__/CreateAccountModal.grok.spec.ts src/components/account/__tests__/EditAccountModal.spec.ts`，5 个文件、37 项测试全部通过。
- PASS：本轮目标 Vue/TypeScript/i18n 文件 ESLint 通过；`npm run typecheck` 退出码 0；`git diff --check` 无空白错误。
- PASS：CodeGraph 重索引完成，2302 个文件、71580 个节点。
- 待执行：不可变镜像构建、候选部署、管理员登录态页面验证和蓝绿切流。

## 2026-07-13 v0.1.152.5 蓝绿发布验证 - Devil

- PASS：`v0.1.152.4` 因二进制缺失 `image_version` 在候选部署前被拒绝，未部署或复用该标签。
- PASS：从 committed HEAD `022bc50d240a` 构建并核验不可变镜像 `sub2api:v0.1.152.5`，OCI version/revision 与容器二进制版本一致。
- PASS：仅重建 idle green；候选 `18082` 完整未登录冒烟通过，启动期一次 `pq` 查询取消后独立观察窗 7 次 health=200、关键日志新增 0。
- PASS：nginx 配置检查、reload 和 blue -> green 切流通过；`8080/18081/18082` 完整冒烟与主资源 SHA-256 一致。
- PASS：切流后 70 秒三入口持续健康，green/proxy restart 0，近 3 分钟精确严重日志过滤均为 0；blue 回滚容器 healthy/restart 0。
- PASS：Chrome 管理员登录态确认版本按钮 `v0.1.152`、镜像版本 `v0.1.152.5`，新增/编辑弹窗三个 Tab 的字段隔离符合范围，未保存账号数据。
- PASS：PostgreSQL、Redis 未重启。
- 未执行：真实认证上游请求、Git push、registry push。

## 2026-07-14 v0.1.152.6 蓝绿发布验证 - Devil

- PASS：从业务提交 `ef9d5c851e30` 构建不可变镜像 `sub2api:v0.1.152.6`，OCI version/revision、二进制版本和 ImageID 一致。
- PASS：仅重建 idle blue；候选 `18083` 完整未登录冒烟通过，启动期一次 `pq` 查询取消后独立观察窗健康且关键日志新增 0。
- PASS：nginx 配置检查、reload 和 green -> blue 切流通过；`8080/18081/18083` 完整冒烟与主资源 SHA-256 一致。
- PASS：切流后 62 秒三入口持续健康，blue healthy/restart 0，proxy running/restart 0，关键日志匹配均为 0。
- PASS：Chrome 管理员登录态确认主版本 `v0.1.152`、镜像版本 `v0.1.152.6`，平台目录显示 `API Key` 列、`sk-前2位***后4位` 和“未生成”。
- PASS：验收过程未执行签到、创建 Key 或配置写入；PostgreSQL、Redis 未重启。
- 未执行：Git push、registry push。

## 2026-07-14 签到总览卡片响应式样式修复 - Devil

- RED：目标 Vitest 新增布局规则测试按预期失败，确认旧样式仍使用固定双列/三列和 `word-break: break-all`。
- PASS：目标 Vitest 2/2 通过。
- PASS：`.\node_modules\.bin\vue-tsc.cmd --noEmit` 退出码 0。
- PASS：`npm run build` 完成，Vite 转换 945 个模块并成功生成生产资源；仅保留既有动态导入、chunk 大小和 Browserslist 提示。
- PASS：Playwright 在 1600、1024、768、390px 宽度检查，无页面、卡片或金额水平溢出，金额多行计数为 0。
- PASS：`git diff --check` 通过。

## 2026-07-15 sub2api 只读数据源与手工账号标识 - Devil

- PASS（用户停止检测前）：sub2 service 聚焦测试、全部 `TestNewAPICheckin*` service 测试、repository/migration 测试和 server 编译切片通过。
- PASS（用户停止检测前）：签到工具目标 Vitest 3/3、Vue typecheck、生产前端构建通过。
- PASS（用户停止检测前）：浏览器确认 sub2 站点不会出现可用签到/月度同步动作，并修正余额明细宽表布局。
- 未执行：最终紧凑表格布局和 `POST /account-display-name` 手工标识接口未再运行测试或浏览器检查；原因是用户明确要求后续自行检测。
- 未执行：提交、镜像构建、部署、线上数据库写入。

## 2026-07-15 v0.1.155.1 蓝绿发布验证 - Devil

- PASS：NewAPICheckin service、repository/migration、admin handler 聚焦测试和 server 二进制构建通过。
- PASS：目标 Vitest 3/3、Vue typecheck 和前端生产构建通过。
- PASS：从 committed HEAD `ab937dd48b7f` 构建不可变镜像 `sub2api:v0.1.155.1`，OCI version/revision、ImageID 和运行容器一致。
- PASS：候选 `18083` 冒烟、未授权 401、60 秒稳定观察、三入口资源哈希一致和 green -> blue 切流通过。
- PASS：切流后 12 次 health=200，blue healthy/restart 0，blue 严重日志 0，proxy error 日志 0。
- PASS：生产库备份后幂等写入 `sub-vcnovb`，确认 `provider=sub2api`、后台签到关闭、Key 长度 67，脱敏为 `sk-90***84cd`。
- PASS：公开嵌入页 1440px 静态布局无重叠；未点击签到、刷新或名称同步动作。
- LIMIT：`internal/server` 全包测试因既有 `stubUserRepo` 接口缺口无法编译；管理登录态因部署密码为空未验证；公开直链内联脚本被现有 CSP 拦截。
- PASS：PostgreSQL、Redis 未重启，green 回滚容器保持 healthy/restart 0。

## 2026-07-15 sub-vcnovb 账号扩容与完整 Key 按需展示 - Devil

- PASS：写入前备份 `newapi_checkin_sites`、`newapi_checkin_accounts` 至 `D:\sub2api-deploy\db-backups\newapi-checkin-config-before-sub-vcnovb-8-accounts-20260715-124340.sql`。
- PASS：生产 `sub-vcnovb` 幂等增加 8 个账号，总数由 1 增至 9；用户名、展示名使用提交邮箱，出口档位为 `ip-slot-sub2-01` 至 `ip-slot-sub2-08`。
- PASS：数据库复核 8 个 Key 长度均为 67，仅输出 `sk-12***cc0c`、`sk-e5***f971`、`sk-6c***7428`、`sk-b5***035c`、`sk-06***08e4`、`sk-95***7254`、`sk-eb***3f00`、`sk-d6***8a4b`。
- PASS：NewAPICheckin service 聚焦测试连续 3 轮通过；handler、repository 聚焦测试与 `go build ./cmd/server` 通过。
- PASS：签到工具目标 Vitest 3/3、Vue typecheck、前端生产构建通过。
- PASS：Chrome 桌面与 390x844 验证默认仅含脱敏 Key，点击“显示”后才把完整 Key 写入 DOM，点击“隐藏”后恢复脱敏；窄屏页面无整体横向溢出，Key 可换行，控制台页面错误为 0。
- LIMIT：Windows CGO 未启用，未执行 `go test -race`；已将并发测试的无锁 slice 改为有缓冲 channel，并连续运行聚焦测试验证。
- 边界：未执行签到、月度同步、套餐/模型刷新、上游认证请求、Git commit、镜像构建或部署。

## 2026-07-15 v0.1.155.2 蓝绿发布验证 - Devil

- PASS：业务提交 `9e055eeba143`；后端聚焦测试、server build、Vitest 3/3、Vue typecheck 和生产构建通过。
- PASS：首次镜像因主版本参数遗漏在候选前拒绝并删除；最终不可变镜像主版本=`v0.1.155`、image_version=`v0.1.155.2`、revision=`9e055eeba143`。
- PASS：仅重建 idle green；候选完整冒烟通过，独立 65 秒观察窗 13 次 health=200、restart 0、关键日志新增 0。
- PASS：nginx 配置检查、reload 和 blue -> green 切流通过；三入口状态码与主资源 SHA-256 一致。
- PASS：切流后 13 组、三入口 39 次 health=200，green/proxy restart 0，严重日志新增 0；blue 回滚容器保持 healthy/restart 0。
- PASS：线上主资源包含 Key 显示/隐藏代码且不含真实账号 Key；生产库保持 9 个启用账号、9 个 Key 长度 67。
- PASS：Chrome 验证公开签到页桌面与 390x844 非空且无页面整体横向溢出。
- LIMIT：管理 Chrome 登录态已过期且部署环境没有可用管理员密码，未验证登录态平台目录与版本徽标。
- PASS：未调用上游、未签到、未刷新、未重启 PostgreSQL 或 Redis。

## 2026-07-15 平台目录弹窗、NewAPI Key 与分组管理 - Devil

- PASS：NewAPICheckin service、handler 聚焦测试通过，server 全包测试通过。
- PASS：签到工具 Vitest 3/3、Vue typecheck、前端生产构建通过；`git diff --check` 无错误。
- PASS：NewAPI 生成 Key 列表默认仅返回脱敏值；点击显示后才调用独立接口读取完整 `sk-` Key。
- PASS：分组更新测试确认先读取完整 token，再只替换 `group`，原额度、状态和模型限制字段保持不变。
- PASS：Playwright 桌面验证主 Tab 不再包含平台目录，按钮打开大弹窗；完整 Key 显示/隐藏与分组提交载荷正确，控制台错误为 0。
- PASS：390x844 验证弹窗标题和关闭按钮可见，页面无整体横向溢出，宽目录表格仅在自身容器内滚动。
- LIMIT：NewAPI `/api/user/available_groups` 对现有 access key 返回权限不足，因此页面明确标记为 partial，仅展示账号与 token 已知分组。
- LIMIT：sub2api `sk-` 调用 `/api/v1/keys` 和 `/api/v1/groups/available` 均返回 `401 INVALID_TOKEN`，页面明确提示需要账号登录态。
- 边界：未签到、未执行真实上游分组写入、未提交、未构建镜像、未部署。

## 2026-07-15 sub2api 登录凭据与分组读取 - Devil

- PASS：使用用户修正后的 VC 登录凭据只读调用 `/api/v1/auth/login` 返回 200，随后读取 1 个 API Key 和 28 个可用分组。
- PASS：API Key `1067` 的当前分组 ID 为 `22`；仅核对脱敏 Key `sk-90***84cd`，未在测试日志记录登录密码或完整 Key。
- PASS：`go test ./internal/service -run NewAPICheckin -count=1`、repository 聚焦测试、admin handler 与 routes 聚焦测试通过。
- PASS：签到工具 Vitest 3/3 与 `vue-tsc --noEmit` 通过；此前同一代码切片的 Vite 生产构建通过。
- PASS：本地页面 1280x720 与 390x844 平台目录弹窗无重叠，浏览器 console warning/error 为 0。
- LIMIT：本地管理 API 未登录，浏览器账号目录为空；登录凭据弹窗、密码不回显、Key 显示和 group_id 提交由 Vitest 覆盖。
- 边界：未签到、未执行真实分组更新、未提交、未构建镜像、未部署。

## 2026-07-15 签到 Key 关联主平台账号 - Devil

- PASS：`go test -tags=unit ./internal/service -run TestNewAPICheckin`，覆盖 `/v1` 与末尾斜杠 URL 归一化、引用账号识别。
- PASS：admin handler/routes 单测与 `go test ./cmd/server` 通过，Wire 注入可编译。
- PASS：签到工具 Vitest 3/3，覆盖已引用高亮、目标账号标识、追加确认及请求体；`vue-tsc --noEmit` 通过。
- PASS：Vite 生产构建成功；HTML 与 generated TypeScript 关联标记同步检查通过。
- PASS：Browser 在 1440x900 与 390x844 检查平台目录弹窗；无横向溢出、按钮越界或 console warning/error。
- LIMIT：全量 service 测试存在 2 条与本功能无关的既有 Codex User-Agent 断言失败；本功能聚焦测试通过。

## 2026-07-15 v0.1.155.3 蓝绿发布验证 - Devil

- PASS：业务提交 `c47c17ba202d`，从 committed HEAD 构建 `sub2api:v0.1.155.3`，OCI 标签、ImageID 和二进制版本一致。
- PASS：迁移 176 执行前完成 custom dump 备份，迁移后 51 个账号保留，两个登录字段在线存在且为空。
- PASS：仅部署 idle blue；候选 health/首页/签到工具为 200，管理 API 和 Responses 未登录为 401。
- PASS：候选与切流后观察窗均超过 65 秒，blue healthy/restart 0，新增应用严重日志和 proxy error 均为 0。
- PASS：三入口主资源路径与 SHA-256 一致，线上静态代码包含登录凭据、登录测试和 group_id 能力。
- PASS：active 已从 green 切到 blue；green `v0.1.155.2` 保持 healthy/restart 0 作为回滚目标。
- PASS：PostgreSQL、Redis 未重启且 restart 0。
- LIMIT：管理员登录态不可用，未执行生产页面凭据保存或真实分组更新；未签到、未月度同步。

## 2026-07-15 v0.1.155.4 蓝绿发布验证 - Devil

- PASS：业务提交 `071b55713a6c`，从 committed HEAD 构建 `sub2api:v0.1.155.4`，OCI version/revision、ImageID 和二进制提交号一致。
- PASS：仅部署 idle green；候选 health/首页/主资源/签到工具为 200，管理 API 和 Responses 未登录为 401。
- PASS：候选运行 94 秒后 healthy/restart 0；启动期一次 `pq: canceling statement due to user request` 后新增严重日志为 0。
- PASS：镜像静态代码包含 Key 关联 API、交互标记和引用高亮类名。
- PASS：代理从 blue 零停机切到 green；三入口资源路径与 SHA-256 一致。
- PASS：切流后 61 秒连续采样均为 200，green/proxy 新增严重日志为 0；green/blue 均 healthy/restart 0。
- PASS：PostgreSQL、Redis 未重启且 healthy/restart 0。
- LIMIT：部署环境无可用管理员登录配置，未在线读取受保护版本接口；未执行生产账号 Key 追加/替换。
- 边界：未签到、未月度同步、未 Git push 或 registry push。
