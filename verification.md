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
