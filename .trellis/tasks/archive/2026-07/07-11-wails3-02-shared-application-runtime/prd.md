# Wails3-Shared-Application-Runtime

## Goal

将 `cmd/server` 的平台无关应用装配与本地数据生命周期抽取为唯一的 `application.Runtime`，供现有 server 与后续 desktop host 复用；现有 Web/PWA/Docker/server 行为保持不变。

## Parent Scope

本 Child 收窄以下父级合同：

- `REQ-COMPAT-01`：HTTP API、JWT、status/JSON、SSE/NDJSON、SQLite schema/migration、scheduler、Web/PWA、Docker/server 无破坏性变化。
- `REQ-COMPAT-02`：server 的数据目录、`GIST_*`、listener、Linux Docker 与 PWA 契约保持不变。
- `REQ-NONGOAL-03`：共享 Runtime 不创建第二个 localhost/LAN server。
- `AC-PROD-01`：本 Child 产出 `E-C02-WEB-SERVER-DOCKER-REGRESSION`，证明上述回归合同。
- `REQ-TASK-03`～`05` / `AC-PROD-10`：本 Child 只建立后续 OperationCoordinator 所需的 Runtime writer 静止点，不实现切换、退出、恢复或更新门禁。

Child 01 已归档；Child 03 依赖本任务。DesktopPaths、Windows data lock、activation IPC 与 recovery journal 属于 Child 03。

## Requirements

### REQ-C02-RUNTIME-01 — 唯一平台无关组合根

- 新增 `backend/internal/application`，由显式 options 构造单个 Runtime owner。
- Runtime 唯一拥有 SQLite、repositories、services、handlers/router、scheduler、全部 local-data asynchronous writers、root context、进程级 Snowflake generator 引用与 `WriterRegistry`。
- `cmd/server` 只保留环境配置、logger、HTTP listener、signal 与 pprof，不再装配 repository/service/handler。
- 共享包不得引用 Wails、窗口、Windows API 或 desktop-only 包。

### REQ-C02-ID-01 — 唯一 Snowflake generator

- 进程 bootstrap 创建唯一 generator owner，并将不可变引用注入 Runtime/repositories；不得保留可重复覆盖的包级 node/`NextID` 状态。
- 同一 bootstrap owner 的第二次初始化明确失败；并行测试通过独立 generator 实例隔离，而非修改进程全局。
- 未初始化、非法 node、重复初始化与 Runtime 构造失败必须在 scheduler、回填或任何写任务启动前失败。
- ID 数值格式与持久化合同保持不变。

### REQ-C02-WRITER-01 — 全量可枚举 local-data writer

- 所有在发起调用返回后仍可能修改 local SQLite/文件的异步工作，都必须在 goroutine 启动前通过 `WriterRegistry` 登记。
- writer context 同时链接 Runtime root 与 initiating request/task context：initiating cancel 必须取消 writer；scheduler 另叠加其 refresh timeout。
- writer 分为 background 与 request/task-bound：quiesce 立即取消 background，但对 signal 前已接受的 request/task-bound writer 先随请求优雅 drain，只有 drain deadline 到达才强制取消。
- writer inventory 至少覆盖 scheduler refresh、启动 icon backfill、OPML import 后 refresh/backfill、import task，以及 AI translation/batch/cache 等异步持久化路径。
- 只有纯网络、纯事件或严格受请求 context 约束且不写 local data 的 goroutine 可以排除；排除项必须在设计与测试中列明。
- admission rejection 必须在对外成功响应前可观察；不得先返回 HTTP 200 再静默丢弃后台写入。

### REQ-C02-LIFECYCLE-01 — 两阶段构造与确定状态机

- Runtime 采用 build → activate：所有可能失败的资源先完成构造；仅在 Runtime 可发布后，以无失败步骤统一登记/启动 scheduler 与 writers。
- build 失败按资源创建逆序清理，且不启动任何 writer、listener 或 pprof。
- Runtime 状态至少区分 Open、Quiescing、Quiesced、Closing、Closed；并发调用同步且无数据竞争。
- `Quiesce(ctx)` 停止 admission/调度并取消 background writer；request/task-bound writer 先随 initiating context drain，deadline 时才强制取消并返回明确错误，状态可继续重试。
- `Close(ctx)` 仅在返回 nil 时保证 scheduler/workers/services/SQLite 全部关闭；caller deadline 不冻结共享关闭结果，后续调用可继续完成关闭。
- SQLite 最后关闭；成功 Close 后无 Runtime-owned writer。

### REQ-C02-SERVER-01 — server 零回归

- 路由集合、认证边界、status/body、SSE/NDJSON、静态资源/SPA fallback 保持不变。
- DB 路径、目录创建、migration、schema、`GIST_*` 与 listener 语义保持不变。
- scheduler 保持立即刷新、周期刷新、停止取消并等待当前 refresh 的语义。
- SIGINT/SIGTERM 必须进入同一受测 shutdown path；沿用 10 秒总 deadline；signal 前已接受的 SSE/NDJSON 在 drain 预算内保持原 framing 并允许完成，超时才取消，同时为最终 Runtime/SQLite close 保留明确预算。
- Linux/Windows server 与 Docker build graph 不得引入 Wails/Windows-only 包。

## Constraints

- 纯重构：不修改 schema、migration、HTTP/API、业务规则、server 配置语义或用户数据。
- 构造函数注入；禁止 service locator、新可变全局状态、双 composition root、双 writer admission path 或兼容 shim。
- 任一可观察差异必须在本 Child 回退；后续 desktop child 不得修补本 Child 回归。

## Out of Scope

Wails/`cmd/desktop`、desktop module/build graph、DesktopPaths/lock/IPC/journal、RemoteClient/mode/auth/transport、frontend runtime isolation、desktop streaming/OperationCoordinator、launcher/NSIS/lifecycle/native integrations/backup/update。

## Acceptance Criteria

### AC-C02-01 — Runtime ownership

- [ ] Runtime 唯一装配并拥有 REQ-C02-RUNTIME-01 列出的资源；server 无重复业务装配；共享依赖图无 Wails/Windows-only 包。

### AC-C02-02 — Generator fail-closed

- [ ] 唯一 owner、非法 node、未初始化、重复初始化与构造故障均有负向测试；任何失败都先于 local-data writer 启动；无包级 generator shim。

### AC-C02-03 — WriterRegistry quiet point

- [ ] 完整 writer inventory 有 owner/class/登记/取消/等待证据；register/complete/quiesce/deadline/race 通过；request cancel、task cancel、background quiesce cancel、in-flight stream graceful drain 与 deadline force-cancel 分别验证；admission failure 不产生伪成功；成功 close 后无 writer。

### AC-C02-04 — Runtime lifecycle

- [ ] 每个 build 阶段 fault injection 证明逆序清理且 activate 未发生；状态机覆盖重复/并发 Quiesce/Close、caller deadline 后重试、DB-last-close。

### AC-C02-05 — Scheduler contract

- [ ] controlled refresh/fake clock 证明立即执行、周期执行、停止取消与等待；scheduler writer 受 Runtime registry 管理。

### AC-C02-06 — Server contract

- [ ] Runtime-constructed router 的 route/auth/status/body/static/stream snapshot 与现状一致；SIGINT 与 SIGTERM runner 在 pprof on/off 下证明：预算内 SSE/NDJSON 完整结束且 framing 不变，near-deadline HTTP/blocking writer 仅在超时后取消，同一 10 秒 path 保留最终 close 预算并写出 Runtime close marker；Linux race、Windows baseline 与 Docker smoke 通过。

### AC-C02-07 — Rollback

- [ ] 在隔离 worktree 中整体 revert/reapply 并复跑旧/新基线；证明无 schema/data 处理、无双 composition root、双 admission path 或 generator shim。

## Required Evidence

- `E-C02-SNOWFLAKE`
- `E-C02-WRITER-REGISTRY`
- `E-C02-SERVER-CONTRACT` / `E-C02-WEB-SERVER-DOCKER-REGRESSION`
- `E-C02-ROLLBACK`

## Rollback

整体回退 Runtime extraction、generator cutover 与 writer admission cutover，恢复 Child 01 已验证的 server composition root。无 schema/data/credential/registry/shortcut/install/CI-release 状态需要迁移或恢复。