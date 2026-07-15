# 操作日志

## 2026-07-12 Devil

- 对比 `v0.1.150..v0.1.151`，按本地架构吸收 9 个非合并提交的生产行为。
- 使用 TDD 验证 cache-write 显式零值、request_type alias 和 bare GPT-5.6 展示。
- 执行后端与前端全量验证，修正 151 行为变化对应的旧断言。
- 核对发布前 active green v0.1.150.2 与 idle blue v0.1.150.1 均 healthy。

## 2026-07-13 Devil

- 通过 Obsidian Local REST、项目流水、Memory 和 CodeGraph 注入项目约束，确认采用方案 1 的提交簇选择性融合边界。
- 在隔离工作树 `D:\sub2api-src-152`、分支 `codex/merge-v0.1.152` 中完成 13 个上游修复簇的 cherry-pick 与冲突融合，原工作树保持不变。
- 使用 `apply_patch` 融合 Grok CLI 头部、OAuth 默认地址、Responses 请求清理、Grok fallback 计费和本地前端组件模式；使用 `gofmt` 格式化 Go 文件。
- 使用 Go 聚焦/包级测试、Vue typecheck、Vitest 聚焦/全量测试验证；记录 handler 6 个既有失败，不扩大本轮修改范围。
- 更新主版本唯一来源 `backend/cmd/server/VERSION` 为 `v0.1.152`；核对 alpha/search migration、按次计费测试和 VersionBadge 单 `v` 行为仍保留。
- 本轮未执行镜像构建、部署、Git push、registry push 或真实认证上游请求。

## 2026-07-13 Devil - v0.1.152.1 发布

- 复核 committed HEAD、active green、idle blue 和回滚镜像，确认目标不可变标签不存在。
- 从 Git archive 构建 `sub2api:v0.1.152.1`，核验 OCI 标签和二进制版本。
- 备份部署 `.env` 与 `active.conf`，仅更新 `SUB2API_BLUE_IMAGE` 并重建 `sub2api-blue`。
- 完成 `18083` 冒烟；因启动期一次 `pq` 取消保留 green active，直到第二个独立 65 秒窗口完全干净。
- 修改 upstream 到 blue，执行 nginx 配置检查和 reload。
- 完成 `8080/18081/18083` 冒烟、前端 chunk 哈希一致性和切流后 65 秒 blue/proxy 观察。
- 最终 active blue=`sub2api:v0.1.152.1`；rollback green=`sub2api:v0.1.151.2`；PostgreSQL、Redis 未重启。
- 从 committed HEAD `dd489bb537d1` 构建并校验不可变镜像 `sub2api:v0.1.151.2`。
- 备份部署配置，仅将 idle green 更新为新镜像，完成候选冒烟与独立健康日志观察。
- 通过 nginx 配置检查后将 active 从 blue 切换到 green，完成三入口冒烟和切流后观察，保留 blue 作为回滚目标。

## 2026-07-13 Devil - OpenAI 模拟测验 Responses streaming

- 在现有 `D:\sub2api-src` / `codex/merge-v0.1.134-updates` 上继续工作，未创建新 worktree；同步后的 HEAD 为 `4aa66e0b4`。
- 通过 Obsidian Local REST、Memory、项目流水与本地 CodeGraph CLI 注入上下文；CodeGraph 索引为最新状态。
- 按 TDD 先将旧 Chat 路径测试改为 Responses streaming 预期并观察 RED，再移除人工测试的 Chat 分流。
- 显式设置 `Accept: text/event-stream`，迁移旧路径的成功与错误测试断言，并完成聚焦验证。

## 2026-07-13 Devil - v0.1.152.2 发布

- 提交协议改动 `9b42afcdf0e7`，从 committed archive 构建并核验不可变镜像 `sub2api:v0.1.152.2`。
- 备份部署 `.env` 与 `active.conf`，仅更新并重建 idle green，blue active 全程保持运行。
- 完成候选 `18082` 冒烟和 75 秒独立健康/日志观察。
- 通过 nginx 配置检查后从 blue 切流到 green，完成三入口冒烟、静态资源哈希核对和 77 秒切流后观察。
- 最终 active green=`sub2api:v0.1.152.2`；rollback blue=`sub2api:v0.1.152.1`；PostgreSQL、Redis 未重启。

## 2026-07-13 Devil - 君公益 403 根因诊断

- 按 systematic-debugging 流程检查 active green 日志、账号非敏感配置、历史 usage、无凭证直连请求和主机网络路由。
- 定位账号 `494/君公益` 的 `/v1/responses` 请求被 `muyuan.do` Cloudflare 返回 HTML 403，记录 Ray ID 并确认非 sub2api 本地拒绝。
- 对首页和 Responses 路径分别使用普通 UA、Codex UA 无凭证复现同一封禁页，排除 API Key 和协议 payload。
- 确认账号无独立代理，主机默认流量经 Mihomo TUN/Clash 出站；未修改代理节点、账号配置或调度状态。

## 2026-07-13 Devil - 简化账号新增与编辑表单

- 读取项目 AGENTS、Obsidian 项目/开发知识库入口、会话 JSONL 流水，并使用 CodeGraph 定位账号表单相关组件；CodeGraph 结果不足后按规则使用 PowerShell 定向读取 Vue 文件。
- 通过 `frontend-design` 的渐进披露原则设计“基本设置 / 更多设置”两级表单；`shrimp-task-manager` 当前未暴露，改用内置计划工具维护步骤。
- 先补 Tab、基本信息字段拆分、API Key 凭证分区和编辑弹窗行为测试，确认 RED 后实现共享 `AccountFormTabs` 与两弹窗字段分区。
- 使用 `apply_patch` 完成源码、测试和 i18n 修改；未改后端请求类型、表单状态结构或提交载荷。
- 运行聚焦 Vitest、ESLint、Vue TypeScript 检查、diff check 和 CodeGraph 重索引；应用内浏览器用临时假数据预览默认编辑弹窗后删除预览文件。
- 浏览器默认态截图确认 1280x720 下无重叠或裁切；Browser 插件跨标签点击状态不一致，未把其高级 Tab 点击截图作为验证证据，交互由自动化测试覆盖。

## 2026-07-13 Devil - v0.1.152.3 发布

- 提交账号表单简化改动 `edacdd5e729b`，从 committed archive 构建并核验不可变镜像 `sub2api:v0.1.152.3`。
- 备份部署 `.env` 与 `active.conf`，仅更新并重建 idle blue，green active 全程保持运行。
- 完成候选 `18083` 冒烟；启动期一次 `pq` 查询取消后，追加独立约 66 秒健康/日志观察并通过。
- 通过 nginx 配置检查后从 green 切流到 blue，完成三入口冒烟、静态资源哈希核对和切流后约 66 秒观察。
- 使用管理员登录态 Chrome 页面验证版本展示、添加弹窗和编辑弹窗的默认 Tab 与切换行为，未保存账号数据并释放用户标签页。
- 最终 active blue=`sub2api:v0.1.152.3`；rollback green=`sub2api:v0.1.152.2`；PostgreSQL、Redis 未重启。

## 2026-07-13 Devil - 模型设置独立 Tab

- 根据用户追加要求，将模型限制和模型映射从基本/更多设置中移出，定义为独立第三个 Tab。
- 使用 CodeGraph 定位 API Key、OAuth、Vertex、Bedrock 和 Antigravity 的 10 个模型配置渲染入口，并按 TDD 先提交失败复现测试 `f855c186d`。
- 扩展共享 `AccountFormTabs`，拆分 Bedrock 模型区域，隐藏模型 Tab 中的 API Key 凭证区域，并同步中英文文案。
- 按 `frontend-design` / `teach-impeccable` 生成项目设计上下文，保持管理后台克制、可靠、清晰的工具型风格。
- 运行 5 个聚焦测试文件、目标 ESLint、Vue TypeScript 检查、diff check 和 CodeGraph 重索引，全部通过。

## 2026-07-13 Devil - v0.1.152.5 发布

- 先拒绝二进制未注入 `image_version` 的 `sub2api:v0.1.152.4`，该镜像未进入候选部署。
- 从业务提交 `022bc50d240a` 构建并核验不可变镜像 `sub2api:v0.1.152.5`。
- 备份部署 `.env` 与 `active.conf`，仅更新并重建 idle green，blue active 全程保持运行。
- 完成候选 `18082` 冒烟和独立健康/日志观察，通过 nginx 配置检查后从 blue 切流到 green。
- 完成三入口完整冒烟、静态资源哈希核对、70 秒切流后观察和精确严重日志过滤。
- 使用管理员登录态 Chrome 验证主版本、镜像版本以及新增/编辑账号三个 Tab，未保存账号数据并释放浏览器标签页。
- 最终 active green=`sub2api:v0.1.152.5`；rollback blue=`sub2api:v0.1.152.3`；PostgreSQL、Redis 未重启。

## 2026-07-14 Devil - 添加 aiaiai 签到站点

- 读取 Obsidian 前馈入口、项目流水和运行中 PostgreSQL 配置，确认 `aiaiai` 尚不存在。
- 使用 CodeGraph 与源码确认 NewAPI 上游认证头和 `/api/user/self`、`/api/user/checkin` 调用方式。
- 对 `api.aiaiai001.com` 的 9 个账号执行身份验证和令牌列表只读探测，不记录明文凭据。
- 备份 8 张签到相关表后，在单一事务中幂等写入站点和 9 个账号，分配 `ip-slot-36` 至 `ip-slot-44`。
- 真实签到验证 9 个账号均返回“今日已签到”；令牌接口确认仅账号 525、619 已有生成的 `codex` API Key。
- 未创建上游 API Key，未重启应用、PostgreSQL 或 Redis，未修改 schema。

## 2026-07-14 Devil - 全站点 Key 脱敏复核

- 从 PostgreSQL 读取 5 个签到站点及 42 个账号的实时配置。
- 请求 5 个站点 `/api/status`，确认站点可达和额度显示类型。
- 限制并发请求 42 个账号 `/api/token/`；3 个 dawclaudecode 账号首次超时后仅对失败项重试并成功。
- 汇总签到 access key 与生成 API Key，统一仅保留前 2 位和后 4 位；未在日志中记录完整凭据。
- 结果为 25 个账号已有 API Key、17 个账号暂无 API Key；全程只读，无数据库或上游写操作。

## 2026-07-14 Devil - 签到工具展示脱敏 API Key

- 复用既有 NewApi access key 请求各账号 `/api/token/?p=1&size=100`，新增独立只读管理接口，限制并发为 6。
- 后端只输出 Key 名称及 `sk-前2位***后4位`，完整上游 Key 不写入 PostgreSQL、不返回前端。
- 公共签到 HTML 与管理端嵌入生成文件同步增加 API Key 列，并采用后台异步加载，避免拖慢主页面数据读取。
- 以 service、handler 和真实嵌入页面测试覆盖有 Key、无 Key、请求头、URL、`sk-` 前缀及完整值不泄露。
- 运行 Go 聚焦测试、server 编译切片、Vitest、Vue typecheck 和 diff check，全部通过。
- 本轮未执行 Git commit、镜像构建、蓝绿部署或线上管理员页面验证。

## 2026-07-14 Devil - v0.1.152.6 发布

- 提交签到工具 API Key 脱敏展示改动 `ef9d5c851e30`，从 committed archive 构建并核验不可变镜像 `sub2api:v0.1.152.6`。
- 备份部署 `.env` 与 `active.conf`，仅更新并重建 idle blue，green active 全程保持运行。
- 完成候选 `18083` 冒烟；启动期一次 `pq` 查询取消后，追加独立健康和关键日志观察并通过。
- 通过 nginx 配置检查后从 green 切流到 blue，完成三入口冒烟和静态资源哈希核对。
- 切流后连续 62 秒采样 `8080/18081/18083`，全部为 200；blue/proxy restart 0，关键日志匹配 0。
- 使用管理员登录态 Chrome 验证主版本、镜像版本和签到平台目录 API Key 脱敏展示，未执行签到、创建 Key 或配置写入。
- 最终 active blue=`sub2api:v0.1.152.6`；rollback green=`sub2api:v0.1.152.5`；PostgreSQL、Redis 未重启。

## 2026-07-14 Devil - 修复签到总览卡片响应式布局

- 使用 CodeGraph、PowerShell 定向读取和截图确认固定双列站点网格、固定三列统计以及金额强制断词共同导致窄容器样式崩坏。
- 先新增嵌入样式回归测试并执行 RED，再修改公共签到 HTML，机械同步 `newapiCheckinLegacy.generated.ts`。
- 将站点卡片和统计项改为容器感知的 `auto-fit/minmax` 布局；标题区允许换行，金额保持单行。
- 运行目标 Vitest、Vue TypeScript 检查、生产构建和 diff 检查，全部通过。
- 启动本地 Vite 服务并用 Playwright 验证 1600、1024、768、390px，无页面、卡片或金额溢出。

## 2026-07-15 Devil - 增加 sub2api 只读数据源和手工账号标识

- 使用 CodeGraph 和源码定向读取确认 `/v1/usage` 已通过 API Key 识别用户，但当前响应契约不返回 username/email；现有 Key 调用用户登录态接口会返回 401。
- 在签到工具增加 `sub2api` provider，只读取 `/v1/usage?days=30` 和 `/v1/models`，保存并展示套餐、余额、用量、到期时间及模型分组，不执行签到或月度同步。
- 增加 `POST /admin/newapi-checkin/account-display-name`，平台目录提供“编辑标识”，允许管理员手工填写用户名或邮箱并写入现有 `display_name` 配置字段。
- 未增加账号密码字段或模拟登录流程；未调用目标站点签到、模型生成或 Key 创建接口。
- 用户明确要求停止检测后，仅完成代码、格式化、公共 HTML 到嵌入 TS 的机械同步和差异检查；未继续运行测试、浏览器检查、构建或部署。
- 停止本轮遗留的本地 `3001/3002` 监听进程。

## 2026-07-15 Devil - v0.1.155.1 提交构建部署验证

- 提交业务改动 `ab937dd48b7f`，从 committed HEAD 构建并核验不可变镜像 `sub2api:v0.1.155.1`。
- 备份部署 `.env` 和 `active.conf`，仅重建 idle blue；候选 `18083` 冒烟和稳定观察通过后，从 green 切流到 blue。
- 完成 `8080/18081/18083` 冒烟、静态资源 SHA-256 核对和切流后连续 60 秒健康/日志观察。
- 确认 active blue=`sub2api:v0.1.155.1`、rollback green=`sub2api:v0.1.155`，两者均 healthy/restart 0；PostgreSQL、Redis 未重启。
- 确认目标 sub2 站点尚未入库后，先备份配置表，再幂等写入 `sub-vcnovb` 和 `primary` 账号；只记录 Key 长度与 `sk-90***84cd` 脱敏值。
- 浏览器检查公开嵌入页静态布局；部署环境管理员密码为空，未修改密码或伪造令牌，登录态页面验证保留为遗留边界。

## 2026-07-15 Devil - sub-vcnovb 账号扩容与完整 Key 按需展示

- 读取项目指导、Obsidian 项目/开发知识库入口、历史过程流水与签到工具既有实现边界。
- 备份生产签到配置表后，向 `sub-vcnovb` 幂等写入 8 个账号，保存邮箱标识与 `ip-slot-sub2-01` 至 `ip-slot-sub2-08`。
- 仅通过 SQL 脱敏复核账号数、Key 长度和首尾字符；未请求 `https://sub.vcnovb.cn/`，未执行签到或刷新。
- 后端配置摘要增加完整 `access_key`；公共签到页面和管理端生成副本增加逐账号“显示/隐藏”，默认仍只渲染脱敏值。
- 增加 service、handler、Vitest 契约与交互测试；完成 Go 聚焦测试、server 构建、Vue typecheck、前端生产构建。
- 启动本地 mock 预览，使用 Chrome 验证桌面与 390x844 页面、DOM 显示/隐藏、窄屏溢出和控制台错误；截图保存在系统临时目录，随后停止预览进程并删除临时脚本。
- 本轮未执行 Git commit、镜像构建、蓝绿部署或上游 API 调用。

## 2026-07-15 Devil - v0.1.155.2 提交构建部署验证

- 提交完整 sub2 Key 按需展示改动 `9e055eeba143`，只包含本轮代码、测试和审计记录；排除 daemon、live probe 和临时请求文件。
- 预提交运行后端聚焦测试、server build、Vitest、Vue typecheck 和前端 production build。
- 备份部署 `.env` 与 active upstream；首次镜像因主版本参数遗漏在候选前拒绝并删除，随后从同一 committed HEAD 正确重建 `sub2api:v0.1.155.2`。
- 仅更新并重建 idle green；候选冒烟、65 秒稳定观察和关键日志检查通过。
- 通过 nginx 配置检查后从 blue 切流到 green，完成三入口冒烟、资源 SHA-256 和切流后 65 秒健康/日志观察。
- 验证线上静态主资源包含显示/隐藏实现且不含真实 Key，生产库仍为 9 个启用账号；未请求上游或签到。
- Chrome 验证公开签到页桌面/移动布局；管理登录态过期且无可用部署密码，未修改认证配置。
- 最终 active green=`sub2api:v0.1.155.2`，rollback blue=`sub2api:v0.1.155.1`；PostgreSQL、Redis 未重启。

## 2026-07-15 Devil - v0.1.156 tag 更新评估

- 读取 Obsidian 前馈入口、Memory、项目流水、当前工作树和 worktree 列表；确认主工作树存在用户原有修改并保持不动。
- 从 origin 精确拉取 `v0.1.156`，校验 annotated tag 和目标提交。
- 使用 first-parent log、stat、name-status 和逐提交三方补丁检查拆解 48 个上游变更组。
- 发现实际开发/发布主线为 `f33701edc`，不包含独立 worktree 的 `f1ffeacc`，据此重跑可应用性评估。
- CodeGraph 在 `D:\sub2api-src-155` 未初始化，改用已初始化的主项目索引确认 `FailoverState` 影响多个网关入口。
- 创建一次性 detached worktree 组合验证低冲突候选；连续三次组合冲突后停止扩大范围并收敛到已成功组合项。
- 移除存在测试依赖缺口的 Ops 投影组，剩余五项通过 repository、handler 编译、apicompat 和 service 测试。
- 验证完成后清理一次性 worktree；未合并当前分支、未构建镜像、未部署。
