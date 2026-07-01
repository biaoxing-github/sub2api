# sub2api v0.1.137-139 与 juhe-ai feature/20250617 可吸收优化 CEO 计划

- 日期：2026-06-30
- 执行者：Devil
- 工作模式：plan-ceo-review / D，全量 umbrella plan
- 当前分支：`codex/merge-v0.1.134-updates`
- 当前 HEAD：`a137340f9f19`
- 已完成版本：`sub2api:v0.1.136.21`
- 下一次发版线：`v0.1.139.x`
- sub2api 对比来源：`v0.1.137`、`v0.1.138`、`v0.1.139`
- juhe-ai 本地路径：`D:\sub2api-src\tmp\juhe-ai-feature-20250617`

## CEO 结论

不要直接合并 `v0.1.137` 到 `v0.1.139`。

当前分支和 `v0.1.139` 的 merge-base 是 `0cfabaa82ea11fe296ce81c786841dbfaeccac36`，也就是 `v0.1.130`。整合并会把 Grok 订阅、支付返利、广告资源、README、多语言、CI、支付 UI、订阅订单等大量非目标变更一并带入，风险和验证成本都不值得。

正确路线是分阶段吸收：

1. Phase 1 先吸收低风险网关可靠性和 provider 兼容补丁。
2. Phase 2 再吸收 Codex 身份识别与 `codex_cli_only` 硬化。
3. Phase 3 把 juhe-ai 的路由和模型映射思想转成 sub2api 自己的设计，不直接搬 TypeScript。
4. Phase 4 只在业务需要时再评估支付、Grok、admin UI 这些产品面变化。

这不是追版本号。目标是让现有 sub2api 网关更少失败、更容易诊断、更少误配。

## 当前状态

- Phase 1 已完成并随 `sub2api:v0.1.136.21` 上线。
- 下一轮发版线切到 `v0.1.139.x`，不再沿用 `v0.1.136.x`。
- Phase 2 已完成 `/v1/chat/completions` 的 `codex_cli_only` 覆盖、Codex 指纹拒绝诊断、GPT-5.5 Codex instructions、Codex 用户模板默认 GPT-5.5 和 Claude Code terminal `CLAUDE_CODE_ATTRIBUTION_HEADER=0`。
- 本地 sub2api 不需要额外全局白名单/黑名单准入系统；该方向跳过，只保留账号级开关和诊断增强。
- PAT auth 是 Personal Access Token 形态的上游认证适配，涉及认证链路；当前本地 sub2api 不需要，暂不吸收。
- Phase 3 已按 2026-06-30 反馈重定义为“稳定性、性能、缓存命中率、发布自动化收敛”。juhe-ai 的路由、协议矩阵、hybrid smart routing 等产品面设计暂不作为阶段任务；执行清单见下方“Phase 3 重定义”。

## 第一阶段已完成清单

- zstd 上游响应解压，成功后清理压缩头，非法或空 zstd 保留原 body。
- 非 JSON、HTML、空 2xx 上游响应直接 failover。
- SSE `event:error` 保真，客户端和 ops 都能看到真实上游 body。
- Responses function-call arguments 去重，避免重复工具参数。
- 图片路径补齐 `response.incomplete`、无 output、content_filter、OAuth 5xx 的 failover 和诊断。
- Provider 兼容补丁覆盖 Anthropic、Gemini、Vertex、GLM、DeepSeek、thinking effort。
- Token refresh 补齐不可重试错误并做脱敏。
- Scheduler outbox 落地 `dedup_key`、读取释放、消费后清理。

## 已核验证据

### sub2api 标签

- `v0.1.137` tag object `96e25629cd4a6818de8b3072f80e84affc9a3ada`，commit `eba9bea959dad0c6db30994870c60085965e2fd5`，日期 `2026-06-16T20:35:55+08:00`。
- `v0.1.138` tag object `e7a6bd46e3d9daa343e22ab4ccf9173348749879`，commit `6936687870e9f5f21ec814ec8c7fcec9d7b37c10`，日期 `2026-06-21T21:43:58+08:00`。
- `v0.1.139` tag object `a28c29d69029d8fd9c984b09315afeb8e15f7916`，commit `9a0fbcc87dc14c3ad4f87c3fad951a320f109050`，日期 `2026-06-26T17:32:50+08:00`。
- `git ls-remote --tags origin refs/tags/v0.1.137 refs/tags/v0.1.138 refs/tags/v0.1.139` 已确认远端标签对象。

### juhe-ai 分支

- 本地路径：`D:\sub2api-src\tmp\juhe-ai-feature-20250617`
- 远端：`https://gitee.com/huanminabc/juhe-ai`
- 分支：`feature/20250617`
- HEAD：`246ebb8d29dafe513e96cbf4f42918285ed9dca2`
- 提交标题：`fix(accounts): 修复账号模型映射与API密钥路由功能`
- 该仓库是 TypeScript/Nest 风格，不是 sub2api 当前 Go/Vue 分层，结论只用于设计吸收。

### 当前分支已有能力

- 当前分支已有 OpenAI `response.failed` 截获、路径健康、stream failover、缺终止事件失败化等近期本地增强。
- 当前分支已有账号级 `openai_codex_cli_user_agent`，但 `codex_cli_only` 仍主要是账号级允许客户端与宽松官方 UA/originator 判定，没有 `v0.1.139` 的全局黑白名单、引擎指纹和版本门。
- 当前分支的 `http_upstream.go` 只看到 gzip、br、deflate 解压，未看到 zstd。
- 当前分支的 `token_refresh_service.go` 已有 `invalid_grant`、`refresh_token_reused`、`invalid_client` 等不可重试判定，但未覆盖 `invalid_refresh_token`、`app_session_terminated`、`refresh_token_invalidated`。
- 当前分支没有 `backend/internal/handler/no_account_error.go`，`v0.1.139` 的无账号 `404 model_not_found` 分类是候选。

## Phase 1：低风险网关可靠性（已完成）

建议第一阶段优先实施。收益直接，变更面相对可控，验证可以用本地单元测试和少量 handler/service 编译切片闭环。

### 1. zstd 上游响应解压

- 来源：`v0.1.137`
- 上游文件：`backend/internal/repository/http_upstream.go`、`backend/internal/repository/decompress_response_test.go`
- 当前状态：当前分支只处理 gzip、br、deflate。
- 价值：上游返回 `Content-Encoding: zstd` 时，usage、错误体和响应内容可以被正确读取。
- 风险：需要新增 Go 依赖 `github.com/klauspost/compress/zstd`，必须确认 `go.mod/go.sum` 变化可控。
- 验证：新增 zstd 正常解压、无效 zstd 保留原 body、空 zstd 不 panic、既有 gzip/br/deflate 不回归。

### 2. 非 JSON 2xx 响应触发 failover

- 来源：`v0.1.137`
- 价值：上游偶发 200 但 body 不是协议 JSON/SSE 时，不让客户端看到假成功或模糊错误。
- 风险：需要准确区分真实文件流、SSE 心跳和坏响应，不能误伤图片或兼容路径。
- 验证：构造 200 text/html、200 空 body、200 SSE 正常 body、200 JSON 正常 body 四类测试。

### 3. SSE `event:error` 保留真实 body 与 ops 诊断

- 来源：`v0.1.137`
- 价值：保留上游错误原文，减少“客户端看到泛化失败，日志也看不到上游真因”的排障成本。
- 风险：当前分支已有 `response.failed` 终止语义修复，要避免重复写终止事件。
- 验证：handler already-communicated 测试、ops upstream error 字段、chat/responses 两条路径。

### 4. OpenAI 图片路径错误识别与 failover

- 来源：`v0.1.137`、`v0.1.138`、`v0.1.139`
- 候选内容：服务器错误 failover、失败时复用 error body、识别 `response.incomplete`、无图片输出时记录上游摘要并 failover、content_filter 作为 400 透传。
- 当前状态：当前通用 OpenAI stream 已识别 `response.incomplete` 作为 terminal，但图片专用解析需要单独核验和补齐。
- 价值：图片生成最容易出现“无 output 但 HTTP 成功”的坏体验，这块优先级高。
- 风险：图片路径计费和 usage 解析要保持一致，不能重复计费。
- 验证：OAuth 非流式、OAuth 流式、API Key fallback、content_filter 400、无输出 failover、usage 保留。

### 5. OpenAI chat transport overloaded failover

- 来源：`v0.1.139`
- 价值：`server_is_overloaded`、overloaded 类错误在 chat transport 中也能切账号，而不是停在单账号失败。
- 当前状态：当前分支已有 account-test 显示归一化和 stream policy 中 overloaded 分类，但 chat transport 错误 failover 仍需逐路径对比。
- 风险：过宽匹配会把用户请求错误当成可重试上游错误。
- 验证：429/529/503、error code `server_is_overloaded`、普通 400 不 failover。

### 6. `response.failed` 事件净化与 function call 参数去重

- 来源：`v0.1.139`
- 价值：减少客户端看到超长上游失败对象；修复透传中 function call arguments 翻倍，避免工具调用参数变成非法 JSON 或重复执行。
- 当前状态：当前分支已有 function call event 转换，但未见 `v0.1.139` 对 passthrough 重复参数的完整测试。
- 风险：去重算法必须只处理确实重复的完整 JSON 参数，不能截断合法增量。
- 验证：两个 tool call 并发、嵌套 JSON、参数中包含 `}`、fallback chat bridge、passthrough SSE。

### 7. provider 兼容补丁小包

- 来源：`v0.1.137`、`v0.1.138`
- 内容：Vertex Anthropic beta 过滤、Gemini schema 清理 `$defs/definitions` 等不支持字段、GLM reasoning effort 归一化、DeepSeek `max` 转 `xhigh`、国产模型 thinking-enabled 默认 reasoning effort。
- 价值：这些都是请求转换层的确定性兼容修复，产品收益明确。
- 风险：每个 provider 的规则不同，必须用现有 provider 分层分别落点，不做全局字符串替换。
- 验证：每个 provider 一个聚焦表格测试，覆盖保留/剥离/默认值。

### 8. token refresh 不可重试错误补齐

- 来源：`v0.1.137`、`v0.1.139`
- 内容：增加 `invalid_refresh_token`、`app_session_terminated`、`refresh_token_invalidated`。
- 当前状态：当前分支已存在不可重试错误列表，但缺这三类。
- 价值：减少已失效凭证反复刷新造成的后台放大效应。
- 风险：必须确认这些字符串只代表需要用户重授，不代表临时上游错误。
- 验证：`isNonRetryableRefreshError` 表格测试。

### 9. scheduler outbox dedup 与清理

- 来源：`v0.1.137`
- 价值：降低调度通知/快照 outbox 重复行和历史行堆积。
- 风险：涉及迁移和 repository，必须单独作为数据库切片，不和网关可靠性混在一个 commit。
- 验证：迁移幂等、dedup_key 唯一、已消费清理、现有 scheduler snapshot 测试。

## Phase 2：Codex 身份识别与访问限制硬化（开发中）

第二期原候选内容如下，已按“不冲突、只增强、本地不加准入系统”的原则重新取舍：

- `/v1/chat/completions` 强制 `codex_cli_only`：已完成，复用现有账号级开关和官方客户端检测，API Key raw chat 不受影响。
- 引擎指纹信号：已完成第一层拒绝诊断，记录 `codex_fingerprint_profile` 与 header/body 指纹哈希，不参与额外放行/拒绝。
- GPT-5.5 Codex instructions：已完成，新增 GPT-5.5 base prompt，`gpt-5.5/gpt-5.4/gpt-5/未知模型` 回退到最新 GPT-5.5 prompt；Codex 用户模板默认 `gpt-5.5` 并开启 goals。
- `CLAUDE_CODE_ATTRIBUTION_HEADER=0`：已完成，Claude Code 终端模板和 VSCode settings 都显式关闭 attribution header。
- `codex_cli_only` 全局白名单/黑名单、最低/最高版本门：跳过。本地 sub2api 不需要新增准入系统。
- PAT auth：跳过。PAT 是 Personal Access Token 认证适配，涉及上游认证链路，当前本地部署不需要。

第二阶段保留的原则是增强兼容、模板和诊断，不扩大默认拒绝面。

### 1. `codex_cli_only` 全局策略模型

- 来源：`v0.1.139`
- 内容：全局自由白名单、全局黑名单、最低/最高 Codex 引擎版本、app-server allow、engine fingerprint signals。
- 当前状态：当前分支仍是账号开关 + 账号级 allowed clients + 全局 named allowed clients。
- 价值：能更准确地区分官方 Codex、Claude Code、app-server 客户端和伪造 UA。
- 风险：真实客户端 UA/originator 变化频繁，默认策略过严会误拒正在使用的客户端。
- 建议：先做“观测模式”字段和日志，不直接默认强拒；确认真实流量后再打开 strict 门。

### 2. 引擎指纹信号

- 来源：`v0.1.139`
- 内容：`x-codex-` header prefix、`session-id/thread-id`、body path 信号、required AND / variant OR。
- 价值：比只看 UA/originator 更接近真实 Codex 引擎行为。
- 风险：不同 Codex Desktop/CLI/插件版本 headers 不一致。
- 验证：真实 Codex Desktop、Codex CLI、Claude Code 插件、curl 伪造 UA、浏览器 UA。

### 3. `/v1/chat/completions` 强制 `codex_cli_only`

- 来源：`v0.1.139`
- 价值：避免只保护 `/responses`，chat fallback 或 raw chat 绕过账号限制。
- 风险：现有兼容客户端可能仍走 chat endpoint。
- 建议：先统计命中账号和 endpoint，再启用强制拒绝。

### 4. GPT-5.5 Codex instructions 与 PAT auth

- 来源：`v0.1.139`
- 价值：Codex 新模型和认证形态适配。
- 风险：PAT auth 涉及上游认证路径，不应和身份检测同 commit。
- 建议：instructions 可先吸收，PAT auth 独立设计和测试。

### 5. Claude Code terminal 模板 `CLAUDE_CODE_ATTRIBUTION_HEADER=0`

- 来源：`v0.1.139`
- 价值：减少 Claude Code 终端模板带来的不必要上游 attribution 差异。
- 风险：低。
- 建议：可作为 Phase 2 的低风险尾项。

## Phase 3 重定义：稳定性、性能与缓存命中（当前执行）

第三阶段不继续做 juhe-ai 的产品面路由/协议矩阵/hybrid smart routing。当前只做能直接服务本地 sub2api 的稳定性、性能、缓存命中率和发布收敛。

### 本轮已吸收

1. 用户等待队列计数移出热路径
   - 来源：`b0579c489 fix: move user wait queue accounting off hot path`
   - 行为：用户槽位立即可获取时不再写 Redis wait counter；只有真实进入等待队列时才 increment，并在成功、超时、取消时统一 decrement。
   - 价值：高并发真实请求少一次 Redis increment/decrement 往返，降低热路径写压力。
   - 验证：`go test ./internal/handler -run 'Test(WaitForSlotWithPingTimeout|AcquireUserSlotWithWait|ConcurrencyErrorResponse)' -count=1` 通过。

2. Chat Completions 桥接请求体保持稳定
   - 来源：`dbdbfb112 fix: avoid default codex instructions for chat bridge` / `50c3e527`
   - 行为：OAuth Chat Completions 转 Codex Responses 时，普通 chat 请求不再注入整段默认 Codex instructions，只保留空 `instructions` 字段。
   - 价值：请求体更小、更稳定；相同用户消息不会因为默认系统 prompt 被反复拼入而污染上游输入，有利于 OpenAI 侧 prompt/request cache 命中。
   - 验证：`go test ./internal/service -run 'TestForwardAsChatCompletions_(OAuthDoesNotInjectDefaultInstructions|.*TransportError|.*ClientDisconnect|.*Terminal|.*Event)' -count=1` 通过。

3. 发布自动化收敛为手动触发脚本
   - 落点：`deploy/release-bluegreen.ps1`
   - 行为：默认 dry-run，只读取 active/idle 并打印构建、部署、候选验证和可选切流计划；必须显式 `-Execute` 才会构建和部署 idle，必须同时显式 `-Cutover` 才会切代理 upstream。
   - 价值：把重复发布命令收敛到一个脚本，但不做自动触发、定时触发、git hook 或监听式发版。
   - 验证：`powershell -NoProfile -ExecutionPolicy Bypass -File .\deploy\release-bluegreen.ps1 -ImageVersion v0.1.139.2` 与带 `-Cutover` 的 dry-run 均通过，未构建、未部署、未切流。

### 继续可吸收候选

- `8b698ff4c fix account list parameter limit`：账号列表大批量 ID 查询分批与去重，适合作为后台管理性能切片单独做。
- `510adf703 feat(scheduling): add opt-in "prefer soonest reset" account selection`：账号都受限时优先最快重置账号，必须 opt-in，先用真实账号池快照 A/B。
- `cc7612bdb Detect OpenAI overloaded error codes`：本地已有多处 overloaded 文案与 529/503 处理，下一步先补 OpenAI stream/non-stream/chat transport 分类测试矩阵，缺口再合并到统一上游错误分类。
- juhe-ai `b49f5f1` Redis Streams 日志队列：只借鉴“日志写入移出请求路径”的设计，等 usage/ops 写入成为瓶颈再按 Go 分层实现。
- juhe-ai `00d8380` 搜索性能优化：可转为本项目 repository 聚焦测试和索引审计，不搬迁移脚本。
- juhe-ai `c2d05cd` runtime cache/high performance：提交面过大，只保留“读多写少配置缓存 + 明确失效”的方向。

## Phase 3：juhe-ai 设计吸收（历史候选，暂不执行）

2026-06-30 复盘后，本节不再作为第三阶段执行清单。当前第三阶段改为稳定性、性能、缓存命中率和手动发布自动化收敛；juhe-ai 内容只保留为历史候选，后续必须有明确业务场景才重新评估。

juhe-ai 是 TypeScript/Nest 风格，不能直接复制到 Go/Vue。这里吸收的是产品与算法设计。

### 1. API key 多组绑定的 `round_robin` / `weighted`

- 来源：`backend/src/modules/gateway/routing/api-key-group-route-selector.service.ts`
- 设计：API key 绑定多个 group 时，支持普通排序、轮询、平滑权重；状态 map capped at 10000，避免内存无限增长。
- sub2api 可吸收点：当前 API key 多组/多 key 路由若存在固定优先级偏置，可以引入策略字段，让同一个 key 在多个 group 间更公平地分配请求。
- 风险：状态在多进程/多实例下不共享，必须明确这是单实例本地公平，不是全局精确配额。
- 建议：先做 group binding ordering，不碰账号级 scheduler 内部算法。

### 2. 模型映射显式区分入口协议族与上游协议族

- 来源：`backend/src/modules/gateway/protocols/openai-v1/model-mapping.ts`、`frontend/src/views/accounts/accountModelMappingProtocolMatrix.ts`
- 设计：`sourceEndpointFamily` 与 `upstreamEndpointFamily` 显式建模，普通 provider 只允许同协议或原生支持，hybrid provider 才允许跨协议桥接。
- sub2api 可吸收点：减少“模型别名看起来配置成功，运行时才发现协议不支持”的误配。
- 风险：sub2api 现有模型映射语义需要先盘点，避免破坏已有账号配置。
- 建议：先做只读校验和 UI 提示，再考虑写入约束。

### 3. hybrid smart routing

- 来源：`backend/src/domain/api-key-hybrid-routing.ts`
- 设计：按评分模型给请求分 1-10 级，配置 level routes、cache TTL、affinity TTL、质量检查和 repair/upgrade action。
- 价值：可以把“便宜模型先答，风险高再升档”产品化。
- 风险：这是新产品，不是小补丁。它需要评分模型、质量回查、成本策略、用户可解释性和失败处理。
- 建议：暂不进第一批实现。先写设计 RFC，明确触发场景、指标、费用和回退。

## Phase 4：暂不建议本轮吸收

这些不是不好，而是不属于本轮“网关稳定性吸收”目标。

- Grok 订阅/OAuth/配额探测：产品面大，依赖新 provider 全链路。
- 订阅推广返利、支付币种、provider supported_types、订单展示：和当前目标无关。
- Ops 趋势卡片 UI、README、多语言、partner logos：不是网关可靠性。
- GitHub CI/workflow/security scan：本项目 AGENTS 明确本地 AI 自动验证，不把 CI 作为本轮目标。
- OpenAI 周限额手动重置 admin 功能：有价值，但需要明确运营权限和 UI 交互，不和 Phase 1 混合。

## 推荐实施顺序

### Commit 1：上游响应体解压和坏 2xx failover

- zstd 解压。
- 非 JSON 2xx failover。
- SSE `event:error` 保真。
- 测试：repository 解压测试、gateway 2xx bad body、handler already-communicated。

### Commit 2：OpenAI 图片路径可靠性

- `response.incomplete` 图片路径解析。
- 无 output 软失败摘要。
- content_filter 400。
- 服务器错误复用 body failover。
- 测试：图片 OAuth 非流式/流式/API Key fallback。

### Commit 3：provider 兼容补丁

- Vertex beta filter。
- Gemini schema cleanup。
- GLM/DeepSeek/thinking effort 规范化。
- 测试：每个 provider 的转换表格测试。

### Commit 4：token refresh 与 scheduler outbox

- 不可重试错误补齐。
- scheduler outbox dedup/cleanup。
- 若涉及 SQL migration，独立提交。
- 测试：service 表格测试、repository/migration 测试。

### Commit 5：Codex 身份硬化观测模式

- 全局策略结构。
- engine fingerprint 解析和校验。
- 先日志/诊断，不默认扩大拒绝面。
- 测试：官方 UA、originator、trailer clientInfo、伪造 UA、Claude Code。

### Commit 6：Codex 身份硬化强制模式

- `/v1/chat/completions` 强制 `codex_cli_only`。
- app-server allow。
- GPT-5.5 instructions。
- PAT auth 如果需要，拆成单独提交。

### Commit 7：juhe 路由小设计

- API key 多组 `round_robin` / `weighted`。
- 只做 ordering，不改账号 scheduler。
- 测试：状态裁剪、权重比例、禁用 group 过滤。

### Commit 8：模型映射协议族校验

- 后端校验。
- 前端提示。
- 只读提示先行，写入硬约束后置。

## 验收标准

每个 commit 必须满足：

- 只改同一优化簇文件，不混入无关 UI、支付、Grok、README。
- Go 代码 `gofmt`。
- 有对应聚焦单元测试或 handler/service 编译切片。
- `git diff --check` 对本轮触达文件通过。
- 涉及前端类型时运行 `corepack pnpm typecheck` 或对应 Vitest。
- 涉及迁移时运行 migration/repository 测试，并记录无法线上执行的风险。

如果进入发布：

- 先提交，再用提交后的 HEAD 构建不可变镜像。
- 记录 active/idle 颜色和回滚镜像。
- 只部署 idle 容器。
- 候选端口和代理切流后都跑健康、静态资源、未登录 admin 401、未登录 `/responses` 401、日志关键字扫描。
- 发布日志写入 `docs/releases/`，并回填 `verification.md`、`.codex/testing.md`、`.codex/operations-log.md`、`docs/feature_list.jsonl`、`docs/process_list.jsonl`。

## 当前决策

- 已选择 D：写全量 umbrella plan。
- Phase 1 已完成并上线，下一步进入 Phase 2 的 Codex 身份硬化。
- 暂不建议把 juhe 的路由/协议族设计和 Phase 2 强制拒绝混在一起。
- 需要用户下一次确认：是否开始实施 Phase 2。

## 本次计划验证

- 已确认 sub2api 三个远端标签对象。
- 已确认 juhe-ai 分支 HEAD 与本地路径。
- 已用 CodeGraph 确认当前分支的 OpenAI gateway、Codex client restriction、scheduler/provider 相关入口。
- 已用 PowerShell 定向抽样当前代码和上游 diff。
- 本次只生成计划文档，未改运行时代码，未运行 Go/Vitest。

## 2026-07-01 v0.1.140 补充吸收记录

执行者：Devil。

本次按用户要求继续查看 `v0.1.140`，不做整 tag 合并，只吸收与本地目标一致、冲突低、偏稳定性/性能/缓存友好的后端切片。`v0.1.140` 中支付、前端表格、ops 日志筛选、风险控制、Grok CLI compat、README 和版本同步等内容暂不吸收，原因是它们不是当前缓存命中/稳定性主线，且容易和本地已完成的阶段内容混在一起。

已吸收切片：

- 图片输出计费误判修正：`backend/internal/service/image_output_accounting.go` 只把 `data[]` 中带 `url` 或 `b64_json` 的对象计为图片产物；`image_generation.completed` 没有 `result`、`url`、`b64_json` 时不再计费。对应测试在 `backend/internal/service/image_generation_intent_test.go`。
- Codex 图片桥接 `tool_choice=auto`：`backend/internal/service/openai_codex_transform.go` 增加 `ensureOpenAIResponsesImageGenerationToolChoiceAuto`；HTTP `/responses` 路径和 WS v2 入站路径都会在 Codex image bridge 启用且未显式传入 `tool_choice` 时补 `auto`，保留用户已有选择并跳过 Spark 模型。对应测试在 `openai_codex_transform_test.go`、`openai_gateway_service_test.go`、`openai_ws_forwarder_success_test.go`。
- OpenAI 上下文窗口错误不切号：`backend/internal/service/openai_gateway_service.go` 增加 `isOpenAIContextWindowError`，让 502/stream `response.failed` 中的 context window/context length/token limit 类错误作为用户请求过大透传，不触发账号 failover、runtime block 或 stream failover。对应测试覆盖普通错误响应、Chat Completions buffered/streaming 和 Responses streaming。
- fallback 定价告警去重与 GLM 兜底：`backend/internal/service/billing_service.go` 用 `sync.Map` 按模型去重 fallback pricing 日志，避免热路径重复刷日志；补入 `glm-5` / `glm-4.6` 兜底定价，`glm-5.2` 归入 GLM-5。对应单元测试使用 `-tags unit`。

验证结果：

- `go test ./internal/service -run 'TestOpenAIImageOutputCounter(SkipsTextOnlyDataArrays|RequiresCompletedImageResult)' -count=1` 通过。
- `go test ./internal/service -run 'Test(EnsureOpenAIResponsesImageGenerationToolChoiceAuto|OpenAIGatewayServiceForward_CodexImageInjectionRespectsGroupCapability|OpenAIGatewayServiceForward_ChannelBridgeOverrideEnablesCodexInjection|OpenAIGatewayServiceForward_CodexBridgePreservesExistingToolChoice|OpenAIGatewayService_Forward_WSv2_ImageGenerationCountsOutputs)' -count=1` 通过。
- `go test ./internal/service -run 'Test(IsOpenAIContextWindowError|ShouldFailoverOpenAIUpstreamResponseContextWindow502|OpenAIHandleErrorResponse_ContextWindow502KeepsMessageWithoutFailover|ForwardAsChatCompletions_.*ContextWindow|OpenAIStreamingContextWindowResponseFailedBeforeOutputPassesThrough)' -count=1` 通过。
- `go test -tags unit ./internal/service -run 'TestGetModelPricing_(FallbackWarnLoggedOncePerModel|FallbackWarnPerModelNotGlobal|GLM52FallsBackToGLM5Price)' -count=1` 通过。
- `go test ./internal/service -run 'TestNoSuchTest' -count=1` 与 `go test ./cmd/server -run 'TestNoSuchTest' -count=1` 通过。
- scoped `git diff --check` 通过。

边界：本轮未提交、未构建镜像、未部署、未执行真实上游请求。下一次如果进入发布，需要把大版本和镜像小版本一起提升到用户指定的版本线，并按蓝绿发布规则构建 committed HEAD。
