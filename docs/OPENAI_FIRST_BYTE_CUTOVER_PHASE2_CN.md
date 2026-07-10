# OpenAI 首字节主动切换 Phase 2 实现说明

- 日期：2026-07-09
- 执行者：Devil
- 参考会话：`019f46ad-5e52-7b71-ac37-c147758cad8d`
- 范围：OpenAI `/responses` HTTP/SSE 转发、passthrough 转发、request-phase failover、OpenAI path health 首字节慢降级
- 状态：Phase 2 已完成本地实现和聚焦验证；未执行提交、镜像构建、部署或线上切流

## 背景

参考会话已经完成 Phase 1：网关能观测 OpenAI 首字节慢样本，把慢账号或慢 baseURL 写入短 TTL 降级状态，并在后续调度中把慢路径排到健康路径之后。Phase 1 的重点是“发现慢并影响后续请求”。

Phase 2 的目标是“受保护的主动切换”：当当前请求还没有向客户端写出真实 OpenAI 内容，但 request-phase 等待响应头或首字节已经超时，网关可以把这次等待视为慢首字节切换样本，返回 `UpstreamFailoverError` 交给 handler 选择下一个候选账号或路径。

## 行为边界

会主动切换的条件：

- 请求处于 OpenAI `/responses` HTTP/SSE 或 passthrough request-phase。
- 上游错误被分类为响应头等待超时或首字节等待超时。
- 客户端请求上下文尚未取消。
- Codex stability policy 允许 request-phase failover。
- handler 侧确认 `OpenAIRealClientOutputStarted(c)` 仍为 false。

不会主动切换的条件：

- 已经向客户端写出真实输出，handler 会直接耗尽本次 failover，不做透明换号。
- 客户端已经取消或断开，本次请求不再切号，也不再尝试写错误响应。
- 当前候选耗尽或超过最大切换次数，handler 使用现有 exhausted/probe 逻辑。
- HTTP `/responses` 携带 `previous_response_id` 的续链请求仍按现有逻辑提前拒绝，避免跨账号破坏上游私有状态。

## 实现入口

主要改动：

- `backend/internal/service/openai_path_health.go`
  - 新增 `RecordFirstByteSlow`，表示调用方已经判定这是慢首字节样本。
  - 该方法只累计 `first_byte_slow_*` 状态，不增加 `FailureCount`、`WindowFailures` 或 circuit breaker 计数。
  - 成功请求的 TTFT 仍走原有阈值判断和快样本恢复逻辑。

- `backend/internal/service/openai_gateway_service.go`
  - 普通 OpenAI HTTP/SSE 分支和 passthrough 分支在 request-phase 错误处统一计算 `first_byte_wait_ms`。
  - 首字节等待超时时调用 `recordOpenAIRequestPhaseFirstByteCutover`，同时写账号、baseURL、聚合 bucket 的慢首字节样本。
  - `newOpenAIRequestPhaseFailoverError` 增加可选 metadata 合并，并自动标记首字节切换语义。

- `backend/internal/handler/gateway_handler_warmup_intercept_unit_test.go`
  - 补齐测试 fake 对 `ConcurrencyCache` 新接口的空实现，让 handler 聚焦测试能重新编译。

## 可观测字段

request-phase failover 的 `ActionMetadata` 会包含：

| 字段 | 含义 |
| --- | --- |
| `first_byte_cutover_allowed=true` | 本次错误满足输出前主动切换条件 |
| `first_byte_cutover_phase=request_header_wait` | 切换发生在等待响应头/首字节阶段 |
| `first_byte_wait_ms` | 本次等待样本，Codex wait guard 截断时体现 guard 后的等待值 |
| `degraded_account_id` | 被记录慢样本的账号 |
| `degraded_base_url` / `request_base_url` | 被记录慢样本的调度 baseURL |
| `degraded_transport=http_sse` | 被记录慢样本的传输类型 |

Ops upstream error 事件会额外记录 `kind=first_byte_cutover`，便于管理端或日志把这类错误和普通 transport error 区分开。

## 与 Phase 1 的关系

Phase 1 的成功慢样本来自“请求最终成功但首字节很慢”。Phase 2 的慢样本来自“请求还没输出就已经被 wait guard 或响应头等待超时切走”。

两者共用 `FirstByteDegradedUntil`、`FirstByteSlowCount`、`LastFirstByteSlowMs` 等字段，所以当前请求触发切换后，后续调度会立即看到同账号、同 baseURL 或同聚合 bucket 的短 TTL 降级状态。

## 验证结果

已执行并通过：

```powershell
cd D:\sub2api-src\backend
go test -tags unit ./internal/service -run "TestOpenAIPathHealthFirstByte|TestOrderedOpenAIRequestBaseURLsDemotesFirstByteDegradedURL" -count=1
go test -tags unit ./internal/service -run "TestOpenAIGatewayServiceRequestPhaseFailoverCarriesActionMetadata|TestOpenAIGatewayService_ForwardRequestHeaderTimeoutReturnsFailover|TestOpenAIGatewayService_ForwardRequestPhaseContextCanceled|TestOpenAIGatewayService_APIKeyRequestBaseURLDoesNotFailoverBeforeAccountFailover" -count=1
go test -tags unit ./internal/service -run "TestOpenAIPathHealth|TestBuildOpenAIAccountLoadPlan|TestOpenAIAccountScheduleProfile" -count=1
go test -tags unit ./internal/handler -run "TestOpenAIHandleFailoverExhausted|TestOpenAIEnsureForwardErrorResponse|TestOpenAIHandleStreamingAwareError" -count=1
go test -tags unit ./cmd/server -run TestNonExistent -count=0
git diff --check
```

`git diff --check` 仅输出既有 CRLF 提示：`.codegraph/daemon.pid`、`docs/feature_list.jsonl`、`docs/process_list.jsonl`。

## 未覆盖范围

- 本轮未做真实网络压测、镜像构建、候选容器验证或线上切流。
- 本轮未新增跨进程 retry key 避让；仍复用当前 handler 的 `failedAccountIDs` 和 path health 短 TTL 降级。
- 本轮未改变已输出后的保护策略；已输出后仍不透明切号。
