# 上游中转站稳定性、性能与缓存优化开发计划

> 日期：2026-07-16
> 执行者：Devil
> 状态：PR 1 基础观测、PR 2 缓存正确性、PR 3 failover 回归已完成本地实现与验证；PR 4-7 待实施
> 适用范围：以 OpenAI `/v1/responses`、`/v1/chat/completions` 为主，同时覆盖共用调度、Redis、PostgreSQL 和 HTTP 上游连接池

## 0. 2026-07-16 并行实施记录

> 执行者：Devil
> 验证边界：仅本地自动化测试；未写数据库、未提交、未构建、未部署、未推送。

### 已完成

- PR 1 基础观测：新增固定维度、原子累加的进程内指标注册表和 `GET /api/v1/admin/ops/runtime/metrics`。已接入调度快照 hit/miss、selection 耗时及结果、实际 failover、BillingCache hit/miss/write/write_drop/write_error；动态输入统一归并 `unknown`。
- PR 2 缓存正确性：queue full、closed、worker Redis error/timeout 时，余额、订阅和 API Key 限流缓存键均标记为本进程 unsafe；后续读取以 3 秒上限经 singleflight 回源主数据。完整快照仅能清除同版本 marker，避免早期快照覆盖后续失败。
- PR 3 failover 回归：Responses 和 Chat Completions 在首次选号前安装请求级调度快照；契约测试同时覆盖 Anthropic Messages。8 个候选故障风暴下，候选列表、窗口费用、用量和 RPM 批量预取均为一次读取。

### 验证证据

- `go test ./internal/service -count=1` 通过，37.472 秒。
- `go test -tags unit ./internal/service -run '^TestBillingCacheService' -count=1` 通过。
- 快照、流输出保护和指标 handler 聚焦测试通过；故障风暴基准（`-benchtime=1x`）为 `572600 ns/op`、`69808 B/op`、`400 allocs/op`。
- `go test ./... -run '^$' -count=1` 与 `git diff --check` 通过。

### 后续边界

- PR 1 的 DB/Redis pool wait、HTTP client create/evict、header wait、TTFT 和 stream producer 仍由 PR 4/6 的实际连接池与上游路径改造接入；本次没有伪造空指标。
- unsafe marker 目前仅在进程内生效；多实例共享 Redis 的强一致 epoch/失效协议属于后续独立设计，不在请求热路径对每次 drop 同步写 Redis。

## 1. 目标开发者与真实任务

### 目标开发者

主要使用者是接入多家上游中转站的 sub2api 网关维护者。日常任务不是从零学习 API，而是：

- 上游出现 429、5xx、EOF、响应头超时或首输出变慢时，快速判断是账号、代理、连接池、Redis、数据库还是上游本身。
- 在不破坏粘性会话、计费正确性和流式连续性的前提下完成 failover。
- 通过缓存降低热路径开销，但不让过期缓存或异步写丢失掩盖真实状态。
- 用可重复的压测和运行指标决定连接池、TTL、并发与超时参数，不凭经验直接调大。

### 维护者视角

目前项目已经具备多账号调度、并发槽位、短窗口退避、路径健康、HTTP/2 回退、请求时间线和多级缓存。真正的摩擦点是这些能力分散在 handler、service、repository 和配置层：单个请求能成功，不等于维护者可以证明连续 failover 时没有放大 Redis/SQL 开销，也不等于缓存写入拥塞时仍保持计费判断正确。

本计划把“能转发”提升为“能解释、能量化、能回滚”。目标体验是：拿到一个 request ID 后，5 分钟内回答请求为什么慢、为什么切号、缓存是否命中、连接是否复用、最终失败发生在哪一层。

### 维护者旅程

| 阶段 | 维护者动作 | 当前摩擦 | 计划后的结果 |
| --- | --- | --- | --- |
| 发现 | 从告警、客户端错误或用户反馈获得 request ID | 错误与耗时信息分散 | 一个 request ID 进入统一诊断 |
| 复现 | 判断是单账号、单代理、单站点还是全局问题 | 缺少标准故障 fixture 和阶段基准 | 使用正常/429/503/timeout/EOF fixture 自动复现 |
| 定位 | 查日志、timeline、Redis、SQL 和连接池 | 缓存命中、pool wait、failover 成本不在同一视图 | 阶段 waterfall 与低基数指标统一口径 |
| 决策 | 调整账号、代理、TTL、连接池或超时 | 容易用“调大参数”掩盖根因 | 每个参数变更由基线、容量公式和回滚值支撑 |
| 验证 | 压测、候选部署、对比错误率和延迟 | 平均值容易掩盖尾延迟 | 固定输出 P50/P95/P99、调用次数和资源占用 |
| 发布 | idle 候选、切流、观察和回滚 | 业务健康与缓存/pool 指标未统一进闸门 | 发布闸门同时检查健康、延迟、drop、wait 和 entry 数 |

### 第一视角困惑报告

```text
T+0:00  收到“Codex 一直转圈”与 request ID，先查请求日志。
T+0:30  能看到一次上游失败，但还要分别确认选号等待、连接建立和响应头等待。
T+1:00  找到 failover 记录，仍需判断下一轮选号是否重复执行缓存/SQL 回源。
T+2:00  查看 Redis 和 PostgreSQL，缺少与该请求同口径的 pool wait、cache miss 和 queue drop。
T+3:00  暂时可以判断“上游慢”，但不能证明连接池、缓存或调度没有放大延迟。
T+5:00  计划目标：诊断页直接给出阶段 waterfall、缓存层级、连接复用、错误分类和切号结果。
```

### 参考体验基准

本计划不是复制外部产品，而是采用成熟开发者工具的共同做法作为质量基准：

| 参考模式 | 可复用体验 | sub2api 目标 |
| --- | --- | --- |
| Stripe 风格 request ID | 一个 ID 串起请求与结构化错误 | 一个 request ID 串起调度、缓存、连接和上游阶段 |
| Vercel 风格阶段日志 | 清楚显示卡在哪个构建阶段 | 清楚显示卡在 selection、slot、connect、header、TTFT 或 stream |
| Grafana 风格 waterfall | 用阶段耗时定位尾延迟 | 在管理端展示请求阶段 waterfall，并能下钻低基数指标 |

### Magical Moment

维护者粘贴 request ID 后，系统立即给出一句可执行结论，例如：

```text
账号 A 的上游响应头等待 60.0s 后超时；本请求复用了调度快照，
未发生额外 SQL 回源；已切换账号 B，并在 1.8s 后收到首个语义输出。
```

这就是本计划的关键体验：不用跨多个日志和缓存系统拼接事实，也能在 5 分钟内定位并决定下一步。

## 2. 当前代码基线

### 已确认存在，不重复建设

| 能力 | 当前证据 | 本轮处理 |
| --- | --- | --- |
| 请求入口与调度 | `gateway_handler_responses.go:22` 读取并校验请求，调用 `GatewayService.SelectAccountWithLoadAwareness` | 保留，只增加阶段指标和回归断言 |
| 请求级调度预取 | `gateway_service.go:2647` 已有 `withRequestSchedulingPrefetch`，窗口费用与 RPM 可在请求 context 内复用 | 不再实现“新快照”；验证所有 failover/协议入口都能复用 |
| 窗口费用批量查询 | `gateway_service.go:2752` 先批量读缓存，缺失时按窗口批查 SQL | 保留批量路径；消除失败时逐账号回退放大 |
| 有界 failover | `failover_loop.go:97` 已处理客户端取消、同账号有限重试、临时封禁与可取消等待 | 保留语义；补充失败风暴基准和耗时预算 |
| 统一上游错误分类 | `upstream_error_classifier.go:45` 已分类 401、429、WAF、EOF、header timeout、5xx、quota 等 | 复用分类标签，不再新增平行错误体系 |
| 路径健康 | `openai_path_health.go` 已记录健康、冷却、半开探测和首字节退化 | 增加聚合指标与运行基线，不改状态机语义 |
| HTTP 上游客户端缓存 | `http_upstream.go:136` 按账号/代理/base URL/协议缓存客户端，跟踪 in-flight 与 last-used | 验证容量、淘汰、连接复用和 HTTP/2 回退成本 |
| Redis Context Journal | `context_journal_redis.go:21` 已实现 TTL、最大会话字节、response 绑定和 replay 安全判断 | 不再建设 Redis 版本；做延迟预算、故障注入和重启验证 |
| API Key 两级缓存 | L1 默认 15 秒，L2 默认 300 秒，负缓存 30 秒，启用 jitter 与 singleflight | 先观测命中率和失效延迟，再决定 TTL |
| 调度负载短缓存 | `gateway.scheduling.load_batch_cache_ttl_ms` 默认 200ms | 不盲目延长；验证稳定性与新鲜度的平衡 |
| 数据库连接池 | 示例值 `max_open=256`、`max_idle=128`、lifetime 30m、idle 5m | 结合 PostgreSQL 上限和 WaitDuration 调优，不直接复制默认值 |
| Redis 连接池 | 示例值 pool 1024、min idle 128 | 结合实际并发、连接等待和服务器 maxclients 调优 |

### 已完成优化的边界

`docs/OPTIMIZATION_TODO.md` 已记录 2026-06-17 完成请求级调度快照复用、选号热路径省查库、RPM/窗口费用批量预取和 failover 有界退避。本计划把这些内容视为已交付基线，只增加回归保护和运行量化，不建立重复实现。

## 3. 核心结论

1. 当前最优先事项不是继续增加缓存层，而是把热路径的缓存命中、失效、写队列拥塞和回源成本变成可观测指标。
2. 上游连续失败时，性能风险主要来自重复选号、缓存/SQL 回源、同账号等待和连接池重建的组合放大，需要用故障风暴基准验证上界。
3. `BillingCacheService` 的异步写队列满时会返回失败并记录节流日志。必须区分“可丢的派生缓存”和“影响余额、订阅、限流判断的关键缓存”，确保队列拥塞不会留下可被继续信任的旧正缓存。
4. HTTP 客户端缓存默认最多 5000 个条目，账号、代理、base URL 和协议会共同放大 key 基数。应先记录命中率、条目数、in-flight、淘汰和新建成本，再选择隔离策略和容量。
5. Context Journal 已经是 Redis 实现，下一步价值在于证明 Redis 慢、断连、服务重启和大上下文时不会阻塞主请求或造成静默切坏会话。
6. 数据库和 Redis 池参数都已配置化，但高默认值不等于适合当前部署。调优必须受服务端连接上限、CPU、请求并发和真实等待时间约束。

## 4. 范围与非目标

### 本轮范围

- OpenAI HTTP Responses 与 Chat Completions 的认证后热路径。
- 选号、并发槽位、failover、路径健康、HTTP 客户端缓存。
- API Key、计费、订阅、RPM/窗口费用、Context Journal 缓存。
- PostgreSQL、Redis 和上游连接池的运行指标与容量边界。
- 管理端最小诊断视图，仅展示已采集数据。

### NOT In Scope

- 不重写整个调度器，不更换 Redis/PostgreSQL，不新增自研缓存框架。
- 不改变计费公式、分组倍率、账号优先级和粘性会话业务语义。
- 不把实时余额最终判断改成本地缓存；最终切号仍使用已有真实上游确认规则。
- 不以延长 TTL 掩盖数据库慢查询，也不以无界扩容连接池掩盖等待和泄漏。
- 不在没有基线数据时改变全局超时、最大切号次数或连接池隔离策略。

## 5. 可观测基线与目标

### Phase 0 必须先采集的指标

所有指标标签保持低基数，只使用协议、阶段、结果、错误分类和缓存层级；不得使用 request ID、账号 ID、完整 URL 或用户 ID 作为指标标签。

| 维度 | 指标 |
| --- | --- |
| 请求阶段 | auth、body parse、selection、slot wait、upstream connect、header wait、TTFT、stream duration |
| 调度 | 候选数、excluded 数、每请求选号次数、failover 次数、等待次数、最终原因 |
| 缓存 | L1/L2 hit/miss、negative hit、singleflight shared、回源耗时、写队列深度、drop、invalidate 失败 |
| PostgreSQL | open/in-use/idle、WaitCount、WaitDuration、query P95、事务错误 |
| Redis | pool hits/misses/timeouts、连接等待、命令 P95、Context Journal P95、并发槽位错误 |
| HTTP 客户端池 | cache entries、hit/miss/create/evict、in-flight、HTTP/2 回退、TLS/代理建连耗时 |
| 上游 | status/error category、header P95、TTFT P95、stream silent、同账号重试、切号成功率 |

### 验收目标

- 任何一次失败请求都能从 request timeline 区分 selection、slot wait、connect、header wait、TTFT 和 stream 阶段。
- 10 个候选连续 503 的故障基准中，窗口费用/RPM 批量回源次数不随 failover 次数线性增长。
- 计费缓存写队列满时，不继续信任可能过期的余额、订阅或限流正缓存。
- Redis Context Journal P95 超过预算或 Redis 不可用时，普通转发不被同步阻塞；连续性功能返回明确 reason。
- HTTP 客户端缓存淘汰只关闭无 in-flight 的 idle entry；高基数场景条目数保持配置上限以内。
- PostgreSQL 与 Redis 池调优后，等待时间下降且服务端连接数不越过预留上限。
- 维护者从 request ID 到定位根因的时间目标不超过 5 分钟。

## 6. 分阶段开发计划

### PR 1：运行基线与请求阶段指标（P0）

**目标**：只增加观测，不改变调度和缓存行为。

**主要文件**：

- `backend/internal/handler/gateway_handler_responses.go`
- `backend/internal/handler/openai_gateway_handler.go`
- `backend/internal/service/gateway_service.go`
- `backend/internal/service/ops_metrics_collector.go`
- `backend/internal/handler/admin/ops_handler.go`

**工作项**：

1. 为 selection、slot wait、header wait、TTFT、stream 和 failover 增加统一阶段计时。
2. 暴露调度预取复用次数、批量回源次数、逐账号 fallback 次数。
3. 暴露 BillingCache 写队列深度、drop 原因和任务类型。
4. 暴露 DB/Redis pool 等待与 HTTP 客户端 cache create/evict。
5. 在现有 request timeline 中引用同一错误分类和阶段名，避免日志与指标口径漂移。

**验收**：

- 正常、429、503、header timeout、EOF、client canceled 各有稳定指标和 timeline。
- 指标无高基数标签。
- 观测开启前后相同基准吞吐差异不超过 2%，P95 不显著回退。

### PR 2：计费缓存拥塞正确性（P0）

**目标**：缓存写队列满、关闭或 Redis 异常时，不留下可继续信任的旧状态。

**主要文件**：

- `backend/internal/service/billing_cache_service.go`
- `backend/internal/repository/billing_cache.go`
- `backend/internal/service/subscription_service.go`
- 对应 service/repository 测试

**工作项**：

1. 分类 `SetBalance`、`DeductBalance`、订阅 usage、API Key 限流等任务的正确性等级。
2. 确认主数据先持久化，缓存只作为派生状态；把这一顺序写成测试契约。
3. 队列失败时使相关正缓存失效或标记 dirty，下一次读取通过 singleflight 回源。
4. 禁止对每次 drop 无条件同步写 Redis；仅对明确的正确性路径执行有超时的修复。
5. 对 shutdown、queue full、worker timeout、Redis error 增加独立测试。

**验收**：

- queue full 后下一次资格检查不会使用旧余额/订阅/限流值。
- 同 key 并发回源被 singleflight 合并。
- 修复路径有明确超时，不把 Redis 故障扩散为网关请求长时间阻塞。

### PR 3：failover 快照回归保护与故障风暴上界（P1）

**目标**：证明已经完成的请求级调度快照在所有主要协议入口持续生效。

**主要文件**：

- `backend/internal/service/gateway_service.go`
- `backend/internal/handler/gateway_handler_responses.go`
- `backend/internal/handler/gateway_handler_chat_completions.go`
- `backend/internal/handler/gateway_handler.go`
- 现有 handler/service 测试

**工作项**：

1. 为 Responses、Chat Completions、Anthropic Messages 建立相同的 failover 调用计数基准。
2. 断言一次请求只构建一次候选 map 和预取快照；excludedIDs 可变，但快照对象不重建。
3. 每次重新选中前仍实时检查账号 runtime block、并发槽位和路径健康，避免把快照变成陈旧可用性判断。
4. 批量 DB 查询失败时限制逐账号 fallback 的数量和总耗时。

**验收**：

- N 个候选失败时，候选列表与批量预取调用保持 O(1)，账号尝试保持 O(N)。
- 客户端取消后不再选号、等待或记录账号耗尽。
- 已向客户端输出语义内容后不透明切号。

### PR 4：上游 HTTP 客户端池容量与复用（P1）

**目标**：减少 TLS/代理重复建连，并防止账号 × 代理 × base URL × 协议导致客户端缓存膨胀。

**主要文件**：

- `backend/internal/repository/http_upstream.go`
- `backend/internal/config/config.go`
- `deploy/config.example.yaml`
- 对应 repository/config 测试

**工作项**：

1. 增加 client cache hit/miss/create/evict、entry 数、in-flight 和 idle age 指标。
2. 用三种真实拓扑建立配置基准：少代理多账号、账号独享代理、多 base URL 中转站。
3. 验证 LRU/idle eviction 永不关闭 in-flight entry，并验证代理变更后旧连接及时清理。
4. 验证 HTTP/2 fallback key 和 TTL 不产生永久协议降级。
5. 根据运行数据给出 `connection_pool_isolation`、`max_upstream_clients`、每 host 连接数的部署配置，而不是修改通用默认值。

**验收**：

- 稳态连接复用率可测，client cache 条目受上限约束。
- 代理切换、base URL 变更和 HTTP/2 回退都有清理测试。
- 压测结束后无持续增长的 idle client 或 goroutine。

### PR 5：Context Journal 与缓存故障注入（P1）

**目标**：证明跨重启连续性和 Redis 异常边界，而不是再实现一套 Journal。

**主要文件**：

- `backend/internal/service/context_journal.go`
- `backend/internal/service/context_journal_redis.go`
- `backend/internal/service/openai_gateway_service.go`
- 对应 service 集成测试

**工作项**：

1. 增加 Redis 读写延迟预算和 context deadline。
2. 覆盖 Redis timeout、断连、响应绑定过期、会话 overflow、损坏 JSON、服务重启。
3. 确认已输出、function call 依赖、encrypted reasoning 等不安全场景始终返回 protected reason。
4. 记录 journal backend、coverage reason 和 replay decision，但不记录完整请求体。

**验收**：

- 服务重启后 TTL 内绑定仍可读取。
- Redis 故障不造成普通请求无界等待。
- 不安全 replay 不发生静默账号切换。

### PR 6：数据库与 Redis 容量调优（P2）

**目标**：依据 PR 1 指标调整部署参数，不改变代码语义。

**工作项**：

1. 核对 PostgreSQL `max_connections`、其他应用连接和管理预留，计算 sub2api `max_open_conns`。
2. 以 WaitCount/WaitDuration、CPU、慢查询和事务时长决定 open/idle 比例。
3. 核对 Redis `maxclients`、内存、命令 P95 与 pool wait，计算 pool/min-idle。
4. 对比调整前后至少三个负载档位，不以单次峰值结果定参数。

**验收**：

- 池等待时间下降，不出现 PostgreSQL/Redis 连接拒绝。
- 空闲连接不长期占满服务端额度。
- 配置和回滚值写入对应发布记录。

### PR 7：维护者诊断闭环（P2）

**目标**：把已有数据汇总成一个请求诊断入口。

**工作项**：

1. request ID 页面展示阶段耗时、账号选择原因、缓存命中层级、上游错误分类和 failover 结果。
2. 仅提供聚合/采样数据，不在管理端同步触发上游探测。
3. 从账号异常、path health、缓存 drop 告警跳转到相关请求。

**验收**：

- 维护者不查原始日志即可区分缓存回源、连接池等待、上游 header timeout 和 stream silent。
- 页面数据获取失败不影响网关转发和账号列表。

## 7. 测试与基准矩阵

### 单元测试

```powershell
Set-Location backend
go test ./internal/service -run 'Test.*(BillingCache|ContextJournal|PathHealth|Gateway)' -count=1
go test ./internal/repository -run 'Test.*(DBPool|HTTPUpstream|ContextJournal)' -count=1
go test ./internal/handler -run 'Test.*(Responses|ChatCompletions|Failover)' -count=1
```

### 集成测试

- PostgreSQL/Redis 使用项目现有 testcontainers 测试。
- 三个可控上游 fixture：正常、固定 503、header timeout/EOF。
- 覆盖单候选、多候选、全部候选失败、客户端取消、已输出后断流。
- 进程重启前写入 Redis Journal，重启后读取并验证 TTL 与绑定。

### 性能基准

| 场景 | 重点观察 |
| --- | --- |
| 正常单账号持续流式 | 吞吐、header P95、TTFT P95、连接复用 |
| 10 账号全部 503 | 每请求选号/预取/SQL/Redis 次数、failover 总耗时 |
| 50% 429 + 50% 正常 | 同账号重试、临时封禁、切号成功率 |
| Redis 注入 50/200/1000ms 延迟 | auth、billing、slot、journal 的阻塞边界 |
| BillingCache 队列打满 | drop、dirty/invalidate、DB 回源、资格判断正确性 |
| 5000 client cache key churn | entry 上限、evict、in-flight、安全关闭、内存 |
| PostgreSQL 连接池饱和 | WaitDuration、query P95、错误率、恢复时间 |

性能报告必须同时给出吞吐、P50/P95/P99、错误率、资源占用和调用次数。只给平均值不通过验收。

## 8. 发布与回滚

1. PR 1 先仅发布观测，建立至少一个完整业务周期基线。
2. 每个行为改动使用独立配置开关或可回滚策略，禁止把多个缓存/调度改动捆成一次切流。
3. 按项目既定流程从 committed HEAD 构建不可变镜像，只部署 idle 颜色。
4. 候选和切流后除健康/日志外，增加：请求 P95、failover 次数、cache drop、DB/Redis wait、client cache entries。
5. 任一指标超过基线阈值时切回旧颜色，不通过延长观察掩盖回退信号。

## 9. 维护者 DX 评分

| 维度 | 当前 | 目标 | 说明 |
| --- | ---: | ---: | --- |
| 请求路径可理解性 | 7/10 | 9/10 | 代码分层清楚，但一次请求证据仍分散 |
| 错误与 failover | 8/10 | 9/10 | 分类和保护较完整，缺运行上界证明 |
| 缓存可解释性 | 5/10 | 9/10 | 缓存层多，缺统一 hit/drop/invalidate 视图 |
| 性能测量 | 6/10 | 9/10 | 有局部计数和 ops，缺跨层基准合同 |
| 配置可调优性 | 7/10 | 9/10 | 参数齐全，缺基于拓扑的推荐与验证 |
| 故障定位时间 | 未量化 | <= 5 分钟 | 用 request ID 贯通阶段、缓存与上游 |
| 综合 | 6.6/10 | 9/10 | 重点是闭环与证明，不是堆叠新组件 |

## 10. 实施任务清单

- [~] **T1（P0，人力约 2 天 / Codex 约 3 小时）**：已完成低基数注册表、管理端快照、调度与 BillingCache 生产调用；连接池与完整上游阶段 producer 留给 PR 4/6。
- [x] **T2（P0，人力约 2 天 / Codex 约 3 小时）**：已修复 BillingCache 队列拥塞时的旧正缓存信任问题。
- [x] **T3（P1，人力约 1.5 天 / Codex 约 2 小时）**：已为所有协议入口补请求级调度快照回归和故障风暴基准。
- [x] **T4（P1，人力约 2 天 / Codex 约 3 小时）**：量化并收敛 HTTP 上游客户端缓存容量、复用与淘汰。
- [x] **T5（P1，人力约 1.5 天 / Codex 约 2 小时）**：完成 Redis Context Journal 故障注入和跨重启验证。
- [~] **T6（P2，人力约 1 天 / Codex 约 1 小时）**：已接入 PostgreSQL、Redis 与 HTTP 客户端池运行时快照；线上容量参数保持不变，待观察 wait/timeout 基线后再调整。
- [x] **T7（P2，人力约 2 天 / Codex 约 4 小时）**：形成面向维护者的一站式请求诊断视图。

### DX 实施检查表

- [ ] request ID 可贯通认证、调度、缓存、连接、上游和流式终态。
- [ ] 每个错误至少包含问题、原因、下一步和稳定错误分类。
- [ ] 正常/429/503/timeout/EOF fixture 可以一条命令运行。
- [ ] 性能报告固定输出 P50/P95/P99、吞吐、错误率、资源和调用次数。
- [ ] 缓存指标可以区分 L1、L2、negative、singleflight、回源、drop 和 invalidate。
- [ ] 队列拥塞和 Redis 故障不会留下可继续信任的旧正缓存。
- [ ] failover 不会重复构建请求级调度快照或线性放大批量回源。
- [ ] 上游 client cache 受容量上限约束，不关闭 in-flight entry。
- [ ] Context Journal 跨重启可验证，危险 replay 返回明确保护原因。
- [ ] 数据库和 Redis pool 参数有容量计算、基线、目标值与回滚值。
- [ ] 每个行为变更都有聚焦单测、故障注入、性能基准和候选观察证据。
- [ ] 文档、配置示例、发布记录与实际版本同步。

## 11. 推荐实施顺序

```text
PR 1 观测基线
  -> PR 2 计费缓存正确性
  -> PR 3 failover 回归与故障上界
  -> PR 4 HTTP 客户端池
  -> PR 5 Context Journal 故障注入
  -> PR 6 运行参数调优
  -> PR 7 管理端诊断闭环
```

PR 1 是其余工作的前置条件。没有可比较基线时，不开始修改 TTL、连接池大小、超时和最大切号次数。

## 12. 决策与遗留问题

### 已决定

- 采用 DX POLISH 模式：保持现有架构，完善稳定性、性能与维护体验。
- 复用统一上游错误分类、路径健康、请求级调度快照和 Redis Context Journal。
- 缓存优化以正确性和可观测性优先，不以增加层级或延长 TTL 为目标。
- 所有性能结论必须来自自动化基准或真实运行指标。

### 实施前需用数据确认

1. 实际上游拓扑中账号数、代理数、base URL 数和连接池 cache key 基数。
2. failover 次数分布及 429/5xx/EOF/header timeout 占比。
3. BillingCache queue full 是否发生、涉及哪些任务类型。
4. PostgreSQL 和 Redis 当前 WaitDuration、连接上限与峰值使用率。
5. Context Journal 真实启用比例、P95 延迟与 overflow 比例。

以上问题不阻塞 PR 1；PR 1 的目的就是自动生成这些答案。

## GSTACK REVIEW REPORT

| Review | Trigger | Why | Runs | Status | Findings |
| --- | --- | --- | ---: | --- | --- |
| CEO Review | `/plan-ceo-review` | Scope & strategy | 0 | NOT RUN | 本轮保持既定优化范围 |
| Codex Review | `/codex review` | Independent 2nd opinion | 0 | NOT RUN | 本轮未执行独立代码审查 |
| Eng Review | `/plan-eng-review` | Architecture & tests (required) | 0 | REQUIRED | 实施前需锁定指标、数据流和测试边界 |
| Design Review | `/plan-design-review` | UI/UX gaps | 0 | NOT RUN | 仅 PR 7 涉及管理端诊断界面 |
| DX Review | `/plan-devex-review` | Developer experience gaps | 1 | CLEAR | score: 6.6/10 -> 9/10, time-to-explain: unmeasured -> 5 min or less |

**VERDICT:** DX CLEARED；进入实施前仍需执行 Eng Review。

NO UNRESOLVED DECISIONS

## 2026-07-16 实施回填 - Devil

### 完成范围

- PR 4：HTTP 上游客户端池提供命中、未命中、创建、淘汰、条目数、容量、in-flight 与最长空闲时间快照；活动条目淘汰后延迟关闭，HTTP/2 fallback 到期后可恢复。
- PR 5：Context Journal Redis 操作默认总预算为 100ms；覆盖 timeout、断连、过期、overflow、损坏 JSON、重启恢复与 replay 保护。
- PR 6：管理端运行时快照接入 PostgreSQL `sql.DB.Stats()`、Redis `PoolStats()` 与 HTTP 客户端池快照。2026-07-16 只读审计确认 PostgreSQL 实际 `max_connections=100`、预留 3、总连接 16（15 idle、1 审计查询），两个应用实例均为 `max_open=256`、`max_idle=128`；Redis 实际 `maxclients=10000`、connected=528、blocked=0，两个实例均为 `pool_size=4096`、`min_idle=256`。部署 `.env` 声明的 PostgreSQL 1024 与 Redis 50000 未生效，且当前镜像的三入口 runtime metrics 路由均为 404，无法取得 Wait/Timeout 指标；因此不自动修改部署参数。先校正服务端限额与声明配置漂移，再在 idle 候选以 DB 32/16、Redis 1024/128 做三档负载验证并以 Wait、Timeout、P95 决定正式值。
- PR 7：错误详情与请求时间线展示请求阶段、缓存、failover 与连接池摘要；摘要请求失败时静默隐藏，切换或关闭弹窗会取消旧请求。

### 稳态基准

命令：`go test .\internal\repository -run '^$' -bench '^BenchmarkHTTPUpstreamPoolTopologies$' -benchtime=1s -count=1`。

| 拓扑 | ns/op | B/op | allocs/op | 稳态 entries | 命中率 |
| --- | ---: | ---: | ---: | ---: | ---: |
| 共享代理，128 账号 | 652.4 | 280 | 5 | 1 | 100% |
| 独享代理，64 账号 | 925.4 | 296 | 6 | 64 | 100% |
| 多 Base URL，16 地址 | 963.4 | 280 | 9 | 16 | 100% |

### 验证边界

- 后端 service、repository、admin handler、全仓编译切片与前端 Vitest、类型检查、生产构建均通过；完整 `go test ./internal/handler` 的 retry-window 与 WebSocket stub 失败是本轮外既有基线，未混入修复。
- Browser 在已登录 `localhost:8080` 验证已部署基线的列表加载、打开/关闭追踪弹窗、桌面与移动布局，页面无 `8080` 控制台错误。当前源码的 `localhost:3000` 因端口隔离的登录态被鉴权守卫拦截；后续创建的隔离 Vite 夹具已确认 HTML 与目标组件入口模块均返回 200、无 Vite 编译错误，但应用内 Browser 的旧标签与当前会话不匹配，无法取得可操作 DOM 或截图。故 PR 7 新摘要的真实视觉验收仍未完成，其状态切换、取消、失败隐藏和四类摘要由 Vitest、类型检查和生产构建覆盖。
- `-race` 未执行：当前 Go 环境 `CGO_ENABLED=0`，启用后缺少 gcc。未提交、未部署、未写数据库、未推送。
