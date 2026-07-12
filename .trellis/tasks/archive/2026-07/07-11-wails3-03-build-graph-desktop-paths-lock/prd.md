# Wails3 Build Graph、DesktopPaths 与数据所有权锁

## Goal

在引入 Wails 依赖和桌面 UI 之前，建立一个不会污染现有 Linux/server 构建图的 Windows desktop host 边界；提供固定的桌面数据路径、数据库访问前的跨会话排他所有权锁、最小 activation IPC，以及可在 `db.Open` 前完成恢复的通用 RecoveryJournal。现有 Web/PWA/server/Docker 行为保持不变。

## Background

- Child 01 已建立 Windows 路径安全基线与根 `VERSION` 单一来源；本任务不得创建第二套版本来源或改变 server 的 `GIST_DATA_DIR` / `GIST_DB_PATH` 语义。
- Child 02 已建立唯一平台无关 `internal/application.Runtime` composition root；desktop 必须复用该 Runtime，不得复制 repository/service/router/scheduler 装配。
- 当前仓库没有 `cmd/desktop`、Wails dependency、desktop Vite mode 或 Windows 数据所有权锁；SQLite WAL/busy timeout 不是跨进程资料库所有权机制。
- Child 04 才引入并锁定 Wails CLI/Go/runtime tuple 与最小 shell；Child 07/13/14 才定义 settings、backup/restore、update 的具体 journal payload。

## Requirements

### REQ-C03-GRAPH-01 — Desktop 构建图隔离

- 在现有 `backend` Go module 内新增 desktop-only host seam，优先采用 Windows build constraints；不得为了本任务引入第二个 composition root 或公开 `internal/application` 的兼容 shim。
- 本任务不得引入 Wails CLI、Go module、JS runtime、bindings、WebView、Taskfile/Wails config、desktop Vite build 或窗口实现。
- `cmd/server` 与 `internal/application` 的 Linux 依赖闭包不得解析 desktop、Wails、Windows WebView、GTK/WebKit 或 desktop-only CGO 依赖。
- Ubuntu 上既有 `go build ./...`、backend tests、Docker server graph 与 frontend Web/PWA build 必须保持可用。

### REQ-C03-PATHS-01 — 固定且不可变的 DesktopPaths

- `DesktopPaths` 只从当前 Windows 用户的 LocalAppData known folder 派生规范化根 `%LOCALAPPDATA%\Gist`，并提供 `data`、`data\gist.db`、`desktop.json`、`recovery`、`logs`、`updates` 与 `webview`。
- DesktopPaths 不读取 cwd、`GIST_DATA_DIR`、`GIST_DB_PATH`、server `config.Load()` 或 frontend static-dir 探测作为 fallback。
- 路径计算无数据副作用；只允许在成功取得数据锁后，由显式 bootstrap 阶段创建所需目录。
- 所有 desktop local-data consumers 必须接收同一个不可变 DesktopPaths 值，禁止复制根路径、文件名或环境回退逻辑。

### REQ-C03-LOCK-01 — 数据访问前的 OS-backed 所有权锁

- 锁身份由规范化 desktop data root 的稳定 identity 派生，并限定为当前 Windows 用户；锁必须跨 Terminal Services session 排他。
- 使用 OS handle 表示所有权；异常退出由 OS 自动释放。永久 `.lock` 文件、SQLite busy timeout、进程内 bool/mutex 或 Wails SingleInstance 均不得作为所有权事实。
- 进程只能先计算 DesktopPaths 与 lock/IPC identity，随后立即尝试获取锁。锁成功之前不得读取 desktop config 或 journal、初始化 credential/Snowflake/Runtime、调用 `db.Open`/migration、启动 scheduler/writer 或创建 Wails application。
- 启动后锁必须保持到所有 Runtime/data resources 已关闭；失败清理与正常退出均最后释放锁。

### REQ-C03-IPC-01 — 最小 activation IPC

- 首实例取得数据锁后建立 user-scoped activation endpoint；endpoint 不访问资料库。
- 后续实例锁失败时只能请求 `activate`：同一交互 session 请求已有窗口恢复/聚焦；跨 session 返回稳定的 `occupied_other_session`；连接失败返回稳定占用错误。
- IPC 不接受任意命令、路径、URL、凭据或业务 payload；请求/响应有固定版本、大小上限、deadline 与拒绝未知字段/动作的失败关闭行为。
- 本任务只提供 activation contract 与受测 endpoint primitive，不接入 Wails window；Child 11 才完成产品级窗口/托盘/SingleInstance 生命周期。

### REQ-C03-RECOVERY-01 — 通用 durable RecoveryJournal

- RecoveryJournal 记录 schema version、transaction ID、operation、phase 和恢复所需的最小 opaque metadata；operation-specific payload 由后续任务定义。
- journal 更新采用同目录临时文件、file sync、atomic replace 与 directory sync 的 durable write sequence；禁止仅依赖 buffered flush 或覆盖写。
- desktop bootstrap 必须在第一次 `db.Open` 前 replay：可幂等完成则完成；否则回滚到旧 pointer/snapshot；恢复失败保留材料并失败关闭。
- journal 状态机和 replay 对重复启动幂等；未知 schema/operation/phase、损坏内容、缺失恢复材料或 sync/rename 失败均返回稳定错误，不得静默删除。
- 本任务只用最小 fixture 证明 prepare/apply/commit/rollback 顺序，不预建 settings、backup、restore、update 或 installer 领域协议。

### REQ-C03-BOOT-01 — 确定的 desktop bootstrap 顺序

- 顺序固定为：DesktopPaths → lock/IPC identity → acquire data lock → start activation IPC → logs → recovery replay → desktop config seam → credential seam → Snowflake bootstrap → `application.NewRuntime`。
- 本任务实现到 Runtime-ready host seam；不创建 Wails app/window。尚未实现的 config/credential adapter 以显式依赖 seam 表达，不得交付 no-op 生产 fallback。
- 任一步失败按逆序关闭已创建资源；只有 data lock 在最后释放。Runtime 构造必须接收 DesktopPaths 的 `DataDir`/`DBPath` 与 Child 02 的显式 generator。
- Desktop host 不启动 Echo/pprof/localhost/LAN listener。

### REQ-C03-COMPAT-01 — 兼容与依赖锁不变

- server 的 config/listener/signal/pprof、HTTP/API、SSE/NDJSON、SQLite schema/migration、scheduler、Web/PWA、Docker entrypoint/data path 和现有 Bun/Go lockfile 行为不变。
- `backend/go.sum` 与 `frontend/bun.lock` 仅因本任务真实依赖变化而更新；本任务不引入 Wails，因此不得产生 Wails/runtime lock 条目。
- Wails 目标 tuple `v3.0.0-alpha2.117` + `@wailsio/runtime 3.0.0-alpha.97` 仅作为 Child 04 输入记录，不在本任务安装或验证。

## Constraints

- Windows 11 amd64 是首个实现与行为验证平台；Linux CI 必须继续验证共享/server 构建图。
- 所有依赖通过构造函数注入；禁止新的可变 package global。
- 锁、IPC 与 journal 的错误必须稳定、可分类且不泄漏完整用户路径、token、credential 或 journal payload。
- 当前 CI 只有 Ubuntu；跨 Terminal Services session、进程 kill 后自动释放与真实 Windows durable-write 行为必须由 Windows 原生 runner/受控环境形成证据，不能用单进程 mock 替代。

## Out of Scope

- Wails app/window、AssetServer、bindings、events、desktop Vite/PWA-disable、toolchain locking 与 frontend runtime isolation。
- mode/auth/RemoteClient、Credential Manager 的产品实现、stream adapter、OperationCoordinator。
- Wails SingleInstance 的产品接入、tray、autostart、session-ending UI、notification/native files。
- backup/restore/update/launcher/NSIS 的业务 journal schema、payload、数据迁移或发布策略。
- 修改 server 的环境变量、默认数据目录、DB schema、migration、API 或 Docker contract。

## Acceptance Criteria

### AC-C03-01 — Build graph isolation

- [ ] Linux `cmd/server`/`internal/application` dependency denylist 无 Wails、desktop 和 Windows-only host 依赖；Ubuntu `go build ./...` 与 focused backend tests 通过。
- [ ] 仓库没有因本任务新增 Wails CLI/module/runtime、desktop frontend build 或第二个 application composition root。

### AC-C03-02 — DesktopPaths ownership

- [ ] 在临时 LocalAppData 下改变 cwd，并设置冲突 `GIST_DATA_DIR`/`GIST_DB_PATH`，DesktopPaths 仍只产生规范化 `%LOCALAPPDATA%\Gist` 子路径；路径计算不创建目录或打开文件。
- [ ] server config/path focused regression 证明既有环境变量与相对路径语义不变。

### AC-C03-03 — Lock before data access

- [ ] Windows 两进程同 session fixture 证明首进程持锁时第二进程无法进入 config/journal/Snowflake/Runtime/SQLite，且 DB-open/migration spy 计数为零。
- [ ] 跨 Terminal Services session fixture 证明同一用户仍互斥；强制终止 owner 后第三进程可重新取得锁，无人工删除 lock artifact。

### AC-C03-04 — Activation IPC boundary

- [ ] 同 session后续实例只能发出 bounded `activate` 并收到成功/稳定失败；跨 session 返回 `occupied_other_session`，不会尝试跨 session 聚焦。
- [ ] 未知动作、超限/损坏 frame、超时、endpoint 不可达均失败关闭；测试证明 IPC handler 不访问 Runtime/SQLite/config/journal。

### AC-C03-05 — Crash recovery before DB

- [ ] 每个最小 journal phase 都有 fault/kill fixture；下一次启动在 DB factory 第一次调用前完成幂等 finish 或 rollback。
- [ ] durable write trace 覆盖 temp write、file sync、atomic replace、directory sync；损坏/未知/缺材料与恢复失败均保留证据并阻止 Runtime。

### AC-C03-06 — Bootstrap cleanup and compatibility

- [ ] 每个 bootstrap 阶段 fault injection 证明逆序清理、Runtime 未提前创建、data lock 最后释放；成功路径只构造一个 Child 02 Runtime 且不开 listener。
- [ ] Web/PWA/server/Docker focused regression 通过；无 schema/data conversion；整体回退只删除本任务代码和测试，不删除真实 desktop data/recovery material。

## Required Evidence

- `E-C03-DESKTOP-PATHS`
- `E-C03-LOCK-BEFORE-DB`
- `E-C03-ACTIVATION-IPC`
- `E-C03-CRASH-RECOVERY`
- `E-C03-WEB-SERVER-DOCKER-REGRESSION`
- `E-C03-ROLLBACK`

## Open Questions

无阻塞产品问题。具体 Windows primitive、IPC transport 与 journal encoding 在 `design.md` 中固定，并以 Windows 原生 integration evidence 验证。
