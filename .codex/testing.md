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
