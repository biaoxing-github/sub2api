# OpenAI 路由健康自驾仪开发计划

更新日期：2026-05-27

## 一句话目标

把现在的“能切账号、能切 BaseURL”升级成“会判断、会解释、会提前避开坏线路”的本地 OpenAI / Codex API 中转。

目标不是再堆几个按钮，而是建立一条闭环：

```text
真实请求和主动测速产生样本
  -> 按 account + base_url 形成健康分
  -> 调度优先选择最快最稳的路径
  -> 请求级记录为什么这么选
  -> 卡住时按安全边界抢跑或保护会话
  -> 页面能直接看出问题发生在哪里
```

## 当前基础

项目已经具备这些能力，本计划必须复用，不另起一套平行系统：

- 多 `request_base_urls`：OpenAI API Key 账号已支持多个请求 BaseURL。
- 独立 `balance_base_url`：余额刷新和正式请求 URL 已拆开。
- 账号切换：额度不足、401/429、请求阶段失败等场景已有账号 failover 基础。
- 慢账号冷却：已有按错误次数和窗口冷却慢账号 / 坏路径的方向。
- 后台测速体检：已有主动测速、批量测试、报告页、得分项和扣分项。
- Ops 指标：已有请求耗时、TTFT、错误、请求详情、timeline 等基础。
- 会话连续性：已有 Context Journal、安全重放、保护错误、`previous_response_id` / `session_hash` 相关逻辑。
- 本地自用部署：可以记录更完整的请求上下文、连续性快照和调度证据；这些数据只在本地服务内使用，不发往第三方外部系统。

## 上下文连续性的事实边界

这些优化可以显著提高切换后的可用性，但不能凭空保证上游私有上下文不断。

### 不能保证的部分

如果客户端只靠 `previous_response_id` 续上下文，新账号或新上游通常不认识旧账号生成的 `response_id`。这个状态存在于上游私有会话里，网关无法凭空复制。

因此跨账号切换只有三种可靠程度：

- 同账号、同上游、同 response chain：最强，优先保持。
- 本地有完整请求快照：可以安全重放，等价于重新发起一次完整上下文请求。
- 只有 `previous_response_id` 但目标账号不认识：不能保证，必须保护或降级。

### 当前代码事实

HTTP `/v1/responses` 对 `previous_response_id` 支持受限。当前 handler 会拒绝 HTTP 入站携带 `previous_response_id`，并提示只支持 Responses WebSocket v2。

服务层 HTTP fallback 也保持保守策略：非 WSv2 转发会移除 `previous_response_id`。这避免把一个目标上游不认识的 `response_id` 直接发过去造成更隐蔽的上下文错误。

WSv2 是更强连续性的路径，因为它有 `response_id -> account_id`、连接状态、turn state 和 Context Journal 这些绑定能力。

### 本计划能增强什么

本计划不承诺“任意切账号上下文不断”。它要实现的是：

- 切换前判断连续性等级。
- 能原账号续就原账号续。
- 能同上游 / 同 BaseURL 续就优先同上游 / 同 BaseURL。
- 有完整快照时才允许跨账号安全重放。
- 没有快照或已经向客户端输出时，不透明切换，返回保护错误或保持原请求策略。
- 页面明确展示当前会话属于 `strong`、`replayable`、`weak` 还是 `unsafe`。

### 基于 Codex `/responses` 抓包的补充结论

本地 Debug 抓到的 Codex Desktop HTTP `/responses` 请求显示：

- 普通长上下文请求没有携带 `previous_response_id`。
- 上下文主要放在请求体 `input` 中，压缩前请求体约 800KB 以上，包含 `message`、`function_call`、`function_call_output`、`reasoning`、`custom_tool_call`、`custom_tool_call_output` 等完整历史结构。
- 压缩后请求体约 165KB，`input` 仍然存在，但历史工具链和 reasoning 结构被折叠，只剩较少的 `message` 项。
- 因此 Codex HTTP 路径当前更像“客户端携带上下文”，不是纯依赖上游私有 `response_id`。

这带来一个关键调度策略：

```text
不带 previous_response_id + input 足够完整
  -> 可以跨账号 / 跨 BaseURL 重放，连续性取决于 input 完整度

不带 previous_response_id + input 是压缩摘要
  -> 可以跨账号 / 跨 BaseURL 续，但只能保证摘要连续，不能保证完整历史细节

带 previous_response_id + input 很薄
  -> 强依赖旧上游私有上下文，不能随便跨账号
```

所以后续调度不能只看“账号是否可用”，还要先判断“这次请求的上下文是否可迁移”。

## 优化 0：请求级上下文迁移判定

### 目标

在账号切换、BaseURL 切换、抢跑请求、失败重放之前，对每个 `/responses` 请求先计算一个迁移分类，避免把“能重发请求”和“能保持上下文”混为一谈。

### 迁移分类

`portable_full`：

- 没有 `previous_response_id`。
- `input` / `messages` 项较多。
- 包含完整历史消息、工具调用输出或足够大的上下文体积。
- 允许跨账号或跨 BaseURL 重放。

`portable_summary`：

- 没有 `previous_response_id`。
- `input` 存在，但更像压缩摘要。
- 通常只有 `message` 项，缺少历史工具调用输出和 reasoning 结构。
- 允许跨账号或跨 BaseURL 重放，但页面标记为“摘要连续”。

`upstream_bound`：

- 携带 `previous_response_id`。
- `input` 很薄，缺少完整上下文或完整快照。
- 优先原账号、原 BaseURL、同上游链路；默认不跨账号透明切换。

`snapshot_replayable`：

- 原始请求当前可能依赖上游私有上下文。
- 但本地已保存完整快照，且尚未向客户端输出。
- 允许移除旧 `previous_response_id` 后，用完整 `input` / `messages` 在新账号重新发起。

`unsafe_after_stream`：

- 上游已经开始返回，或者网关已经向客户端输出 token。
- 不允许透明重放，不允许后台悄悄切账号生成第二段回答。

### 判定信号

- `has_previous_response_id`
- `previous_response_id` 类型和长度。
- `input_item_count`
- `input_type_counts`
- `request_body_bytes`
- 是否包含 `function_call_output` / `custom_tool_call_output`
- 是否包含 reasoning 历史项。
- 是否存在本地完整快照。
- 是否已经收到上游响应头。
- 是否已经向客户端输出。
- `Session-Id`、`Thread-Id`、`prompt_cache_key`、`session_hash` 是否稳定。

### 切换动作矩阵

```text
portable_full
  -> 401 / 余额不足 / header timeout / EOF 时允许换账号或换 BaseURL 重放

portable_summary
  -> 允许换账号或换 BaseURL，但优先同模型、同上游、健康分高路径，并标记为摘要连续

upstream_bound
  -> 优先原账号原 BaseURL；没有快照时不跨账号透明切换

snapshot_replayable
  -> 可跨账号重放，但要用快照重建请求，并移除旧 previous_response_id

unsafe_after_stream
  -> 不透明重放；记录保护原因，返回可解释错误或保持当前流失败状态
```

### 对 BaseURL 的影响

BaseURL 健康分之外新增“上下文亲和度”：

- 同账号同 BaseURL：最高。
- 同账号同上游集群不同 BaseURL：较高。
- 同账号未知上游集群：中等。
- 不同账号但请求是 `portable_full`：允许。
- 不同账号但请求是 `portable_summary`：允许但标记弱连续。
- 不同账号且 `upstream_bound`：默认禁止。

### 主要改动

- 新增 `openai_request_continuity_observation` 或类似分析器。
- 在 `/responses` 入站解析阶段产出 `context_migration_class`。
- 将迁移分类写入 RouteDecisionTrace、Ops 请求详情和 Context Journal。
- 调度器、BaseURL failover、账号 failover、首包抢跑都读取该分类。
- 前端请求详情展示迁移分类、判定信号和切换限制。

### 验收

- Codex 压缩前请求能识别为 `portable_full` 或接近完整可迁移状态。
- Codex 压缩后请求能识别为 `portable_summary`，并展示“摘要连续”。
- 带 `previous_response_id` 且缺少完整快照的请求识别为 `upstream_bound` 或 `unsafe`。
- 已输出 token 的流式请求识别为 `unsafe_after_stream`。
- 账号切换和 BaseURL 切换都能记录“为什么允许 / 为什么禁止”。

## 优化 1：BaseURL 级别调度

### 目标

现在多 BaseURL 主要是“当前 URL 请求失败后，再尝试下一个”。下一步要变成“请求前就知道哪个 BaseURL 最近最快、最稳、最可能保上下文，优先走它”。

### 数据维度

每个 OpenAI API Key 账号的每个 `request_base_url` 单独维护健康分：

- `account_id`
- `base_url`
- `success_count`
- `failure_count`
- `success_rate`
- `ttft_ewma_ms`
- `p95_latency_ms`
- `header_wait_ewma_ms`
- `eof_count`
- `header_timeout_count`
- `context_deadline_count`
- `recent_failure_count_5m`
- `state`: `healthy` / `degraded` / `open` / `half_open`
- `cooldown_until`
- `last_failure_reason`
- `continuity_affinity_score`
- `same_upstream_cluster`
- `last_response_chain_hit`
- `supports_wsv2_continuity`

### 调度规则

调度不再只产出账号，而是产出：

```text
selected account + selected request_base_url
```

推荐公式：

```text
base_url_score =
  success_rate_weight
  + freshness_weight
  + continuity_affinity_weight
  - ttft_penalty
  - p95_latency_penalty
  - header_wait_penalty
  - eof_penalty
  - header_timeout_penalty
  - cooldown_penalty
```

规则：

- `open` 状态默认不参与新请求。
- `half_open` 只允许少量探测请求。
- 无样本 BaseURL 保留探索比例，避免永远没机会。
- 老会话优先连续性，新会话优先健康和 TTFT。
- 携带 `previous_response_id`、`session_hash` 或 Context Journal 命中的请求，优先选择同账号、同上游、同 BaseURL。
- BaseURL 如果属于同一个上游集群，可以作为弱连续候选；如果只是不同中转域名但实际同一上游，需要通过配置或探测标记 `same_upstream_cluster`。
- 同账号多个 BaseURL 都不可用时，再进入账号 failover。

### 主要改动

- `backend/internal/service/account.go`
- `backend/internal/service/openai_gateway_service.go`
- `backend/internal/service/openai_account_scheduler.go`
- `backend/internal/service/account_probe.go`
- 新增或扩展 `backend/internal/service/openai_path_health.go`

### 验收

- 一个账号配置两个 BaseURL，一个快一个慢，新请求优先走快 URL。
- 主 URL 连续 EOF / header timeout 后进入冷却，新请求直接走备用 URL。
- 半开恢复只放少量请求，不把 Codex 全量打回坏 URL。
- 没有样本的新 URL 会获得少量探索机会。
- 有会话连续性信号时，调度优先选择更可能认识旧 `response_id` 的 BaseURL。

## 优化 2：首包超时抢跑请求，谨慎版

### 目标

降低 Codex “一直卡思考”的体感。当流式请求长时间等不到响应头时，启动一个备用请求，谁先返回用谁，另一个取消。

### 启用条件

必须同时满足：

- Codex 稳定模式开启。
- 请求是流式请求。
- 原请求尚未收到上游响应头。
- 尚未向客户端输出任何内容。
- 当前请求可安全取消。
- 有备用 BaseURL 或备用账号。
- header wait 超过阈值，例如 3000-5000ms。

### 限制

- 单请求最多抢跑 1 次。
- 默认优先同账号备用 BaseURL，其次备用账号。
- 长上下文请求默认更保守，缺少快照时不跨账号抢跑。
- 只有 `strong` 或 `replayable` 连续性状态允许跨账号抢跑；`weak` 只能同账号 BaseURL 抢跑；`unsafe` 不抢跑。
- 记录 `hedge_attempt=true`、`hedge_winner`、`hedge_extra_cost_risk`。
- 增加每日额外抢跑计数，避免不知不觉多消耗。

### 数据流

```text
primary request started
  -> header_wait_ms > threshold
  -> start hedge request
  -> first response headers wins
  -> cancel loser
  -> write route trace and health samples
```

### 主要改动

- `backend/internal/service/openai_gateway_service.go`
- `backend/internal/service/openai_gateway_transport.go`
- `backend/internal/service/context_journal.go`
- `backend/internal/service/ops_request_details.go`
- 设置项接入 Codex 稳定模式。

### 验收

- 慢 BaseURL header wait 超阈值时触发一次抢跑。
- 已经向客户端输出后不触发抢跑。
- 抢跑失败不会吞掉原请求的错误证据。
- 关闭 Codex 稳定模式后不抢跑。
- 缺少完整请求快照时，不进行跨账号抢跑。

## 优化 3：请求级调度解释器

### 目标

现在功能多了，但出问题时还要靠猜。需要一个“最近请求路由追踪”页面，让每次请求的调度过程可解释。

页面能回答：

```text
请求进来
  -> 候选账号有哪些
  -> 哪些账号被跳过，为什么
  -> 选中了哪个账号
  -> 选中了哪个 BaseURL
  -> 是否切换 BaseURL
  -> 是否切换账号
  -> 首包耗时多少
  -> 最终状态是什么
```

### 后端数据结构

新增或扩展 `RouteDecisionTrace`：

- `request_id`
- `session_hash`
- `previous_response_id`
- `requested_model`
- `candidate_accounts`
- `candidate_base_urls`
- `skipped_reasons`
- `selected_account_id`
- `selected_base_url`
- `account_score`
- `base_url_score`
- `continuity_state`
- `balance_check_result`
- `failover_events`
- `hedge_events`
- `header_wait_ms`
- `first_token_ms`
- `duration_ms`
- `final_status`
- `final_error_category`

### 前端页面

新增或扩展运维页面：

- 最近请求列表。
- 筛选：账号、BaseURL、模型、状态、是否抢跑、是否切账号、错误类型。
- 详情弹窗：
  - 候选账号排序。
  - 跳过原因。
  - BaseURL 分数。
  - 连续性状态。
  - 余额确认结果。
  - timeline 事件。

### 主要改动

- `backend/internal/service/ops_request_details.go`
- `backend/internal/service/ops_models.go`
- `backend/internal/handler/admin/*ops*`
- `frontend/src/views/admin/ops/*`
- `frontend/src/api/admin/ops.ts`

### 验收

- 一次 `context deadline exceeded` 能在页面看到是上游超时、BaseURL 冷却、账号满并发、余额不足还是本地等待上限。
- 同一请求在日志、timeline、路由追踪页面的 request id 一致。
- 不展示完整 prompt、API key、Authorization、Cookie。

## 优化 4：真实流量反哺测速分

### 目标

主动测速解决“我想测一下”。真实流量解决“它刚刚真的慢了”。

最终调度应该以真实请求样本为主，主动测速为辅。真实流量反哺必须是实时的，但不能影响主请求性能。

### 关键原则

真实流量反哺做成“实时但异步”：

```text
请求完成
  -> 内存 channel 投递 HealthSample
  -> 主请求立即结束，不等统计写入
  -> 后台 worker 批量聚合
  -> 更新内存 / Redis 热状态
  -> 低频落库做历史报告
```

主链路只允许多一次轻量内存投递。channel 满了就丢低价值样本，不阻塞 Codex 请求。

### HealthSample 字段

- `request_id`
- `account_id`
- `base_url`
- `model`
- `transport`
- `stream`
- `success`
- `status_code`
- `error_category`
- `header_wait_ms`
- `first_token_ms`
- `duration_ms`
- `input_tokens`
- `output_tokens`
- `failover_count`
- `hedge_attempted`
- `created_at`

### 双层存储

实时调度层：

- 内存 map 或 Redis。
- 保存 EWMA、近窗口失败数、最近状态。
- 调度立即使用。

历史分析层：

- 30-60 秒批量写库。
- 用于报告页、趋势图、复盘。
- 不参与每次请求的同步路径。

### 性能保护

- 不在请求链路同步写 DB。
- 不在请求链路等待 Redis。
- `HealthSample` channel 有上限。
- channel 满时丢弃样本并增加 `dropped_sample_count`。
- worker 批量 flush。
- 调度读取健康缓存失败时降级到原逻辑。
- 只记录指标和错误分类，不记录完整请求正文。

### 主动测速与真实流量权重

推荐：

```text
final_health_score =
  real_traffic_score * 0.75
  + probe_score * 0.25
```

如果真实样本不足，则提高 probe 权重。真实样本足够时，主动测速只做补充和冷启动。

### 主要改动

- 新增 `backend/internal/service/openai_health_sample_collector.go`
- 扩展 `backend/internal/service/openai_path_health.go`
- 扩展 `backend/internal/service/account_probe.go`
- 扩展 `backend/internal/service/openai_gateway_service.go`
- 扩展 `backend/internal/service/ops_metrics_collector.go`

### 验收

- 真实请求成功后，BaseURL 健康分在短时间内更新。
- 真实请求 EOF/header timeout 后，新请求调度能看到降权。
- 大量请求下统计 worker 不拖慢 TTFT。
- channel 满时请求仍正常返回。
- 报告页能区分样本来源：真实流量 / 主动测速。

## 优化 5：会话连续性体检

### 目标

把“切号后上下文怎么办”的隐式逻辑变成可见状态。平时就知道哪些会话能安全续，哪些会话有风险。

这项功能用于判断和保护上下文，不承诺跨账号继承上游私有 `previous_response_id`。

### 状态定义

- `strong`：同账号、同上游、同 response chain，可直接续。
- `replayable`：本地有完整请求快照，可安全重放到新账号或新 BaseURL。
- `weak`：只能依赖客户端原始 messages，或只能判断同上游可能连续。
- `unsafe`：缺少快照，或已经向客户端输出，不能安全重放。

### 连续性判断矩阵

```text
同账号 + 同 BaseURL + WSv2 response_id 命中
  -> strong

同账号 + 同上游集群 + response_id 绑定存在
  -> strong 或 weak，取决于 WSv2 / 上游支持

跨账号 + Context Journal 有完整请求快照 + 尚未向客户端输出
  -> replayable

跨账号 + 只有 previous_response_id
  -> unsafe，不能假设新账号认识旧 response_id

HTTP /v1/responses + previous_response_id
  -> 当前代码拒绝或移除，标记为 unsupported_http_previous_response_id

已向客户端输出任何内容
  -> unsafe，不透明切换会破坏客户端看到的流
```

### 页面展示

在请求详情或路由追踪里展示：

- 当前连续性状态。
- 使用的 `previous_response_id` / `session_hash`。
- 是否有完整 snapshot。
- 是否已经绑定 response id。
- 切号时采用的策略。
- 不能安全重放的原因。
- 当前请求协议：HTTP / stream / WSv2。
- `previous_response_id` 在当前路径是否被支持、拒绝或移除。
- BaseURL 是否同上游集群，以及连续性亲和分。

### 调度策略

```text
strong
  -> 优先原账号原链路

replayable
  -> 可在请求阶段切账号或抢跑

weak
  -> 优先同账号 / 同上游 / 同 BaseURL；不跨账号抢跑

unsafe
  -> 不透明切号，返回保护错误或保持原请求等待策略
```

### 主要改动

- `backend/internal/service/context_journal.go`
- `backend/internal/service/openai_gateway_service.go`
- `backend/internal/service/ops_request_details.go`
- `frontend/src/views/admin/ops/*`

### 验收

- 每个 Codex 请求详情能看到连续性状态。
- 缺少快照时显示明确原因，不只写“不可重放”。
- 已向客户端输出后的请求不会被后台悄悄切号破坏上下文。
- HTTP 携带 `previous_response_id` 时能显示“当前路径不支持”，而不是误导成可续。
- 跨账号只有 `previous_response_id` 时标记 `unsafe`，不承诺上下文不断。
- 有完整快照时，跨账号重放会移除旧 `previous_response_id`，用完整 input 重建上下文。

## 优化 6：策略模板

### 目标

把散落的稳定性和速度设置收成几个自用模式。你不需要每次理解一堆参数。

### 模板

`Codex 稳定优先`：

- 更长流式静默等待。
- 更积极避开 EOF/header timeout 线路。
- 更保守重放。
- 默认允许请求阶段 BaseURL failover。

`首字速度优先`：

- 低 TTFT BaseURL 优先。
- 允许首包抢跑。
- 探索比例更低。
- 对 header wait 权重更高。

`省额度优先`：

- 不抢跑。
- 少主动测速。
- 真实流量样本为主。
- 尽量单请求完成。

`长上下文优先`：

- 连续性权重最高。
- 缺少 snapshot 时不跨账号。
- 抢跑默认只限同账号 BaseURL。

`自定义`：

- 保留当前高级参数。

### 实现方式

模板只写入现有 settings，不把策略写死在调度代码中。

前端展示：

- 设置页新增“Codex 路由策略”区域。
- 选择模板后显示将要修改的关键参数。
- 用户确认后保存。
- 如果手动改过参数，显示为“自定义”。

### 主要改动

- `backend/internal/service/setting_service.go`
- `backend/internal/service/settings_view.go`
- `backend/internal/config/config.go`
- `frontend/src/views/admin/SettingsView.vue`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

### 验收

- 四个模板能一键保存。
- 保存后调度行为对应变化。
- 手动修改参数后显示为自定义。
- 关闭 Codex 稳定模式后不触发抢跑和激进切换。

## 优化 7：本地请求上下文快照与连续性观测

### 目标

因为这是自用、本地部署，不存在多租户隐私顾虑，可以把请求上下文和连续性证据记录得更完整。这样后面排查“Codex 为什么卡住”“切号后上下文有没有断”“客户端到底有没有带 `previous_response_id`”时，不再只能靠临时日志和猜测。

这项功能服务于三件事：

- 请求详情页能看到客户端实际带来的连续性信号。
- 切账号 / 切 BaseURL 前能判断是否可安全重放。
- 失败后能复盘：是客户端没带上下文、HTTP 路径不支持、缺快照、还是上游不认识旧 `response_id`。

### 记录内容

建议分两层记录。

轻量字段，写入 `usage_logs`、`ops_error_logs`、请求详情和路由追踪：

- `has_previous_response_id`
- `previous_response_id_kind`: `empty` / `response_id` / `message_id` / `unknown`
- `previous_response_id_len`
- `has_full_input`
- `input_item_count`
- `request_body_bytes`
- `continuity_state`: `strong` / `replayable` / `weak` / `unsafe` / `unknown`
- `continuity_reason`
- `request_type`: `sync` / `stream` / `ws_v2`
- `snapshot_available`
- `snapshot_id`
- `snapshot_replayable`
- `snapshot_replay_block_reason`

完整快照，写入单独的本地表，不直接塞进高频日志表：

- 原始请求 JSON。
- 规范化后的 `input` / `messages`。
- 请求模型、stream、tools、tool outputs、reasoning、service tier 等关键字段。
- `previous_response_id`、`session_hash`、Context Journal 绑定信息。
- 选中的账号、BaseURL、上游 endpoint。
- 请求开始时间、首次响应头时间、首 token 时间。
- 是否已经向客户端输出。

### 存储边界

虽然自用不担心隐私，但仍然要避免工程风险：

- 不记录 Authorization、API Key、Cookie 等认证头。
- 原始请求 body 设置大小上限，超过上限只记录截断标记和摘要。
- 快照表单独做保留期，例如 7-30 天。
- 支持按 request id / session hash 手动清理。
- 高频 `usage_logs` 只放可筛选字段，不放大 JSON。
- 完整快照只用于本地排查、重放判断和连续性保护，不参与每次同步调度计算。

### 连续性判断

```text
WSv2 + response_id 命中 + 同账号同链路
  -> strong

有完整 input / messages 快照 + 尚未向客户端输出
  -> replayable

HTTP /v1/responses 携带 previous_response_id
  -> unsafe 或 unsupported_http_previous_response_id

只有 previous_response_id，没有完整快照
  -> weak 或 unsafe，取决于是否同账号同上游

已经向客户端输出
  -> unsafe，不允许透明重放
```

### 页面展示

在“最近请求路由追踪”和“请求详情”里展示：

- 客户端是否带了 `previous_response_id`。
- `previous_response_id` 是 `resp_` 还是 `msg_` / `message_`。
- 当前请求有没有完整 input。
- 当前请求是否有本地快照。
- 为什么判定为 `strong` / `replayable` / `weak` / `unsafe`。
- 如果切号失败，展示“新账号不认识旧 response_id”或“缺少完整快照”。
- 如果 HTTP 路径携带 `previous_response_id`，展示“当前 HTTP 路径不支持，只能 WSv2 或快照重放”。

### 对切号的帮助

这项功能不能让新账号凭空继承旧账号的上游私有上下文，但能让系统知道什么时候可以保护会话：

- 有完整快照：可以移除旧 `previous_response_id`，用完整 input 在新账号重新发起。
- 没有完整快照：不能透明切号，避免客户端误以为上下文仍在。
- 同账号多 BaseURL：优先选更可能同上游、同 response chain 的 BaseURL。
- 已开始输出：不重放，只记录风险并走保护策略。

### 主要改动

- 新增 `openai_request_continuity_observation` 分析器。
- 扩展 `usage_logs`、`ops_error_logs` 的轻量连续性字段。
- 新增本地快照表，例如 `openai_request_snapshots`。
- 扩展 Context Journal，把 snapshot id 和 replayable 状态关联进去。
- 扩展 Ops 请求详情和路由追踪 API。
- 前端在请求详情弹窗展示连续性证据。

### 验收

- 能在页面看到 Codex 请求是否携带 `previous_response_id`。
- 能区分 HTTP 路径不支持、WSv2 可强连续、完整快照可重放、缺快照不可安全重放。
- 切号时若有完整快照，系统可明确标记 `replayable`。
- 切号时若只有旧 `previous_response_id`，系统明确标记 `unsafe`，不假装上下文不断。
- 快照不会记录认证密钥。
- 快照超过保留期会自动清理。

## 2026-05-27 执行状态

本轮优先完成不增加额外真实请求、不做大表结构迁移的低风险闭环。

已落地：

- 请求级上下文迁移判定：已产出 `context_migration_class`，覆盖 `portable_full`、`portable_summary`、`upstream_bound`、`snapshot_replayable`、`unsafe_after_stream`，并接入调度决策、连续性保护、响应头和 Ops Codex 诊断。
- BaseURL 级健康调度：真实 `request_base_url` 作为 PathHealth 分桶，HTTP Responses 和 passthrough 请求会按实际 URL 记录 header wait、EOF/header timeout、401/429/5xx 和流式 first token。
- 请求前 BaseURL 排序：开启 PathHealth 时会先避开 `open_circuit` / `degraded` 路径；开启 Fast Lane 时再叠加 TTFT / header wait EWMA 加权，不把坏线路冷却绑定到首字加速开关。
- 主动测速反哺：上游测速体检样本会写入对应账号 + BaseURL 的 PathHealth，成功样本记录耗时和首 token，失败样本按错误原因降权。
- 请求级解释增强：Ops Codex 诊断能归类展示上下文迁移、连续性、候选路由、BaseURL failover、线路健康状态和最后失败 URL / 原因。
- 策略模板：设置页已提供 Codex 稳定优先、首字速度优先、省额度优先、长上下文优先四个模板，复用现有 OpenAI Fast/Flex Policy 规则保存。

暂缓：

- 首包超时抢跑请求：会额外发起真实上游请求并可能消耗 token，必须单独做额度预算、开关、每日上限和真实流量验证。
- 持久化完整快照表：需要新增迁移、保留期清理、敏感字段边界和页面检索能力，当前只先落地轻量迁移分类与 Context Journal 可重放信号。
- 独立“最近请求路由追踪”新页面：现阶段复用 Ops 请求详情和 Codex 诊断弹窗展示；后续如需列表级筛选，再扩 `RouteDecisionTrace` 持久化和前端页面。

## 推荐开发顺序

第一批，先做质变：

1. BaseURL 级别健康分和调度。
2. 请求级调度解释器。
3. 真实流量实时异步反哺测速分。

这三项做完，系统会从“失败后补救”变成“提前避开坏路”。

第二批，再做提速和易用：

4. 首包超时抢跑请求。
5. 会话连续性体检。
6. 策略模板。
7. 本地请求上下文快照与连续性观测。

首包抢跑对速度提升最大，但有额外请求和 token 风险，所以必须放在健康分、路由追踪、连续性证据之后。

## 总体验收

- 多 BaseURL 账号能优先走最近最快最稳的 URL。
- 坏 URL 不再每次都先撞一次才切换。
- 真实请求样本能实时影响下一批调度。
- 请求详情能解释候选、跳过、选择、切换、抢跑、连续性状态。
- 抢跑只在安全边界内发生，不破坏已输出的会话。
- 策略模板能减少手动调参成本。
- 本地请求快照能解释客户端是否带上下文，以及切号是否可安全重放。
- 主请求链路不被统计写入拖慢。

## 建议测试

后端：

```powershell
go test -tags unit ./internal/service -run "Test.*BaseURL|Test.*PathHealth|Test.*RouteDecision|Test.*HealthSample|Test.*Hedge|Test.*Continuity"
go test ./internal/repository -run "Test.*PathHealth|Test.*RouteTrace|Test.*Probe"
go test ./internal/handler/admin -run "Test.*Ops|Test.*RouteTrace|Test.*Probe"
```

前端：

```powershell
.\node_modules\.bin\vue-tsc.cmd --noEmit
pnpm test -- AccountProbeReportsView
pnpm test -- Ops
```

部署验证：

```powershell
docker build --pull=false -t sub2api:multi-key-local --build-arg COMMIT=$(git rev-parse --short HEAD) .
docker compose -f D:\sub2api-deploy\docker-compose.yml up -d sub2api
docker exec sub2api /app/sub2api --version
Invoke-RestMethod http://127.0.0.1:8080/health
```
