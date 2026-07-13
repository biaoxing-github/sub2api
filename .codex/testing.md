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
