# Wails 3 桌面客户端技术设计

> 状态：revised，已纳入 2026-07-11 用户确认与 P1 审查结论；仍为规划文档，不授权启动实现
> 日期：2026-07-11
> 目标平台：Windows 10/11 x64

## 1. 决策摘要

以下稳定设计 ID 供 PRD/实施计划/验证证据引用；编号不因章节移动而改变。

1. **DES-01 交付隔离**：保留 `backend/cmd/server`、现有 HTTP API、Docker 和 Web/PWA；新增 `backend/cmd/desktop`，桌面构建使用独立 entry，禁用 PWA/Service Worker，且绝不监听 localhost/LAN。
2. **DES-02 数据独占与 runtime**：桌面进程在创建 Wails application 或 `application.Runtime` 前，先取得当前 Windows 用户跨会话的数据目录锁；Wails SingleInstance 仅负责同会话第二次启动的窗口唤醒。runtime 唯一拥有 SQLite、repository、service、handler、scheduler、回填与所有后台 writer。
3. **DES-03 模式事务**：`ModeManager` 同时维护不可变 active repository 与独立 candidate transition；只有候选验证、认证与任务门禁全部成功才原子提交并递增 generation。
4. **DES-04 传输与认证**：普通非认证 `/api` 与 `/icons` 经 Wails AssetServer 的模式感知 `http.Handler` 路由；全部认证/profile/token 操作只能经过 `DesktopAuth`，token 永不返回 WebView。四类流式能力使用 operation-specific Wails 事件，Web/PWA 继续使用原 SSE/NDJSON。
5. **DES-05 操作协调**：单一 `OperationCoordinator` 仲裁模式切换、完全退出、更新安装、完整备份与恢复；后台 scheduler 不作为前台任务，但所有 runtime-owned writer 必须进入停写/排空协议。
6. **DES-06 崩溃安全**：配置迁移、数据库迁移、备份恢复与更新均先持久化并 fsync 恢复 journal，再执行可破坏步骤；启动先恢复未完成事务，再打开数据库。
7. **DES-07 Windows 生命周期**：正常关闭走交互式 close coordinator；`WM_QUERYENDSESSION/WM_ENDSESSION` 走无对话框、有时限、数据库一致性优先的系统结束路径。
8. **DES-08 更新安全**：签名 NSIS、稳定 launcher、版本化 payload、单调 anti-replay 状态、data-format fence 与首次启动健康检查共同阻止重放、降级和旧程序打开新数据。
9. **DES-09 可选能力**：`api.major` 定义远程基础必需契约；capability 只表示 optional capability，缺失时仅禁用对应可选功能，绝不能掩盖同 major 必需端点/字段/行为缺失。
10. **DES-10 feasibility gate**：锁定 Wails CLI/Go module `v3.0.0-alpha2.117` 与 `@wailsio/runtime@3.0.0-alpha.97`。完整 body/upload、乱序事件与连续 ACK/背压、关闭 hook、通知激活和 launcher 交接原型全部通过并留存证据前，不得扩展为产品实现；失败时修订设计，不得绕到 localhost listener。
## 2. 目标与非目标

### 目标

- 最大限度复用现有业务 service、repository、SQLite schema 和 React UI。
- 本地与远程模式使用相同桌面前端，但始终访问各自独立的内容资料库和认证状态。
- 保留 Web 端全部 HTTP/SSE/NDJSON/PWA 契约。
- 让模式切换、窗口关闭、更新、备份和恢复具备可测试的事务边界。
- 把 Wails 依赖限制在桌面入口和适配层，避免进入 domain/service 层。

### 非目标

- 不让桌面本地模式成为可被浏览器或局域网访问的第二个服务端。
- 不加载远程服务托管的 Web/PWA 页面。
- 不同步本地与远程内容、设置或认证状态。
- 不提供多个本地资料库、多个远程连接、远程离线副本或写入队列。
- 不在首版交付便携包、macOS、Linux 桌面包或 Windows ARM64。
- 不在本任务中直接实现；评审后按独立实施切片创建/启动子任务。

## 3. 总体架构

```text
                          GitHub Release / NSIS
                                   |
                         Desktop Update Service
                                   |
+---------------- Wails desktop process ----------------+
|                                                        |
|  React/Vite desktop build                              |
|       |                                                |
|       +-- ordinary fetch('/api', '/icons') ------------+---+
|       |                                                |   |
|       +-- DesktopAuth / DesktopTasks / NativeFiles ----+   |
|                                                        |   |
|  Wails AssetServer + bound services/events             |   |
|       |                                                |   |
|  ModeManager ---- CredentialStore ---- DesktopSettings |   |
|       |                                                |   |
|       +-- local ---- shared ApplicationRuntime <-------+   |
|       |              Echo + services + SQLite              |
|       |              scheduler always running              |
|       |                                                     |
|       +-- remote --- RemoteClient --------------------------+
|                      HTTPS/HTTP + compatibility handshake
+--------------------------------------------------------+
                              |
                     separately deployed Gist
                     API + remote content library

Existing browser/PWA ---------- existing cmd/server ---------- SQLite
```

## 4. 模块与所有权

建议边界如下，最终文件名可在实现中微调，但依赖方向不得反转。

| 模块 | 职责 | 禁止事项 |
|---|---|---|
| `internal/application` | 构造/关闭共享 local runtime；拥有 DB、scheduler、回填、writer registry 和 Snowflake generator | 引用 Wails、窗口或 Windows API |
| `cmd/server` | 环境变量、HTTP listener、signal、pprof | 重复装配 repository/service |
| `cmd/desktop` | 先取得数据锁，再恢复 journal/初始化 Snowflake/runtime，最后创建 Wails app | 承载业务规则或先创建 Wails runtime |
| `internal/desktop/operation` | `OperationCoordinator`；切换、退出、更新、backup/restore 的统一任务/writer 门禁 | 各入口维护私有任务判断 |
| `internal/desktop/mode` | active repository、candidate transition、generation、原子提交 | 复制资料库数据或让候选污染 active |
| `internal/desktop/transport` | AssetServer 限额、普通请求路由、RemoteClient、资源 namespace | 开启网络 listener；代理认证/profile 路由 |
| `internal/desktop/tasks` | operation-specific 事件、reorder/ACK、取消、通知结果 | 改写 Web SSE handler 契约 |
| `internal/desktop/auth` | `DesktopAuth` 唯一 token 边界、Credential Manager 注入 | 把 token 返回 WebView/普通 proxy |
| `internal/desktop/platform/windows` | 用户级数据锁、Credential Manager、自启动、session-ending、签名/单实例辅助 | 进入共享业务包 |
| `internal/desktop/recovery` | fsynced journal、startup recovery、data-format fence | 在 db.Open 后才判断未完成事务 |
| `internal/desktop/backup` | 一致性快照、加密备份、验证、原子恢复 | 读取远程资料库 |
| `internal/desktop/update` | manifest、anti-replay、下载验证、launcher 恢复事务 | 直接替换运行中的 Wails exe |
| `frontend/src/platform` | browser/desktop ports 与明确 build entry | 在业务组件中散布 `window.wails` 判断 |

所有组件通过构造函数注入；不新增可变包级全局状态。`Runtime.Close(ctx)`、任务取消、journal recovery 和桌面关闭必须可重复调用且顺序确定。
## 5. 共享 Application Runtime

### 5.1 前置数据锁与构造契约

桌面数据锁是当前 Windows 用户范围、跨 Terminal Services session 的 OS-backed exclusive lock，身份由规范化 `%LOCALAPPDATA%\Gist\data` 派生。进程启动只能先派生固定锁名与激活 IPC 名称，不得读取配置或资料库；随后立即尝试取得数据锁。首实例持锁后建立不访问资料库的用户级 activation IPC，再继续 recovery/config/Runtime/Wails 初始化。后续实例锁失败时只能连接该 IPC：同一交互会话请求已有窗口唤醒/聚焦；跨 session 因 Windows 前台隔离只获得“另一会话正在使用资料库”的明确占用响应。IPC 不接受任意命令、路径或凭据，连接失败也只显示占用错误。任何锁失败进程都不得读取/迁移 `desktop.json`、replay journal、初始化 Snowflake、调用 `db.Open` 或启动 scheduler。异常退出由 OS 自动释放锁，不以永久 lock file 作为所有权依据。Wails `SingleInstanceOptions` 仅作为同会话窗口唤醒的纵深机制，不能作为数据安全锁。

持锁后按顺序执行：建立日志 sink -> replay/finish recovery journal -> 读取桌面配置 -> 初始化 Credential Store -> 初始化一次全进程 Snowflake node/generator -> `application.NewRuntime(options)`。Snowflake 初始化失败必须在任何写 service 或 scheduler 启动前失败关闭；server 入口保留其既有初始化语义。

`application.NewRuntime` 返回单个拥有者对象，拥有 router、DB、scheduler、图标回填、background context、Snowflake generator 引用及 `WriterRegistry`。所有会修改 local data 的异步工作必须由 runtime 启动并注册，禁止 service 脱离 runtime 创建不可枚举 goroutine。

```go
type RuntimeOptions struct {
    DataDir           string
    DBPath            string
    StaticDir         string
    EnableSwagger     bool
    SchedulerInterval time.Duration
    StartScheduler    bool
    IDGenerator       snowflake.Generator
}

type Runtime struct {
    Router      http.Handler
    Auth        service.AuthService
    AI          service.AIService
    OPML        service.OPMLService
    ImportTasks service.ImportTaskService
    Writers     WriterRegistry
}

func NewRuntime(ctx context.Context, options RuntimeOptions) (*Runtime, error)
func (r *Runtime) Quiesce(ctx context.Context) error
func (r *Runtime) Close(ctx context.Context) error
```

- 构造失败按逆序关闭已经创建的资源；`Close` 幂等。
- `Quiesce` 拒绝新 local writes，取消或排空已注册 writer，并在 backup/restore/update snapshot 前给出可验证的静止点。
- `cmd/server` 继续以 `StartScheduler=true` 创建 runtime 后启动 Echo listener；HTTP、signal、pprof 与目录行为不变。
- desktop 固定使用 `%LOCALAPPDATA%\Gist\data`；active repository 为 remote 时 local runtime/scheduler 仍运行，但不重复调度 remote。
- desktop 不启动 pprof、Echo listener 或任何 loopback listener。

### 5.2 启动与关闭顺序

交互启动：用户级数据锁 -> 日志 -> recovery journal -> 配置/凭据 -> Snowflake -> local runtime/迁移 -> Wails services/AssetServer -> SingleInstance/window/tray -> 后台更新检查。任一步失败逆序释放；数据锁最后释放。

正常退出由 `OperationCoordinator` 执行：拒绝新操作 -> 前台任务确认/取消 -> runtime `Quiesce` -> 停 scheduler -> 关闭 service/回填/SQLite -> fsync 必需状态 -> 结束 Wails app -> 释放数据锁。backup/restore 和更新安装复用同一 coordinator，不能有旁路关闭顺序。

Windows `WM_QUERYENDSESSION/WM_ENDSESSION` 不显示关闭选择或任务确认，也不等待网络/通知等非关键工作：立即拒绝新写、取消可取消任务，在固定 deadline 内 quiesce/关闭 SQLite 并落盘 journal；超时记录脱敏状态后允许系统结束。测试必须证明该路径不会调用任何 modal UI。
## 6. 模式状态机与资料库隔离

`ModeManager` 不用单一 `status` 同时表达当前资料库与切换中间态，而是维护两个正交状态：

```ts
interface ActiveRepository {
  mode: 'unselected' | 'local' | 'remote'
  generation: number
  sourceLabel: string
  insecureHTTP: boolean
  status: 'auth-required' | 'active' | 'error'
  errorCode?: string
}

interface CandidateTransition {
  id: string
  target: 'local' | 'remote'
  phase: 'task-gate' | 'validating-address' | 'compatibility' | 'auth' | 'ready-to-commit'
  targetLabel: string
  error?: DesktopError
}

interface ModeSnapshot {
  active: ActiveRepository
  candidate: CandidateTransition | null
}
```

切换事务固定为：创建 candidate -> `OperationCoordinator` 前台任务门禁 -> 验证目标地址/TLS/HTTP 风险 -> compatibility（先于凭据）-> 验证目标 token 或进入目标 auth -> 停止并取消 active generation 的 UI 请求 -> 原子提交 active + 持久启动模式 -> `generation++` -> 丢弃 candidate。候选任何步骤失败或用户取消，只清除/保留可重试 candidate 错误；active repository、来源标识、启动模式、token 和 cache 均不变。

generation 是资料库身份的一部分。所有普通请求、任务事件、图片 URL、Query key、持久缓存 namespace 和通知结果捕获 generation；旧 generation 结果不得 reducer 到新 active。提交后根 repository shell 以 `key={generation}` 强制 remount，并按顺序执行：abort requests/streams -> `queryClient.cancelQueries()` -> `queryClient.clear()` -> 清理当前 generation 的 IndexedDB/resource cache -> 重置 translation、lightbox、selection、scroll、route-derived detail 与所有含 entry/feed/task ID 的 store -> 发布新 active -> 重新 bootstrap。不得只 invalidate。

用穷举测试维护 repository-scoped 状态清单；新增含资料库 ID 的 store/cache 时必须加入共享 reset registry。两套 fixture 刻意复用相同 Snowflake ID，验证 DOM、请求、图片、通知和缓存只来自 active generation。远程不可达、认证失效或 API 不兼容仍显示 remote active/error，不自动回退 local。
## 7. 传输设计

### 7.1 Browser transport 保持不变

browser/PWA build 继续使用当前 `fetch`、Bearer/localStorage、SSE/NDJSON、文件 input/download、Service Worker 与浏览器资源缓存。服务端路径、状态码、JSON、上传/下载、流式响应、Swagger 和新标签外链行为保持兼容；desktop 分支只能由 build entry/port 选择，不能改变 browser 默认值。

### 7.2 Desktop AssetServer 普通请求与资源

`DesktopAssetHandler` 包装内置静态资源 handler，并在读取 body 前按路由施加明确限额：JSON/form 普通请求、OPML upload、其他 upload、普通响应、图片代理响应分别使用独立常量；超过 request 限额返回稳定 `request_too_large`，超过 remote response/image 限额中止读取并返回 `response_too_large`，不得把无限 remote body 缓存在内存。实现应流式 copy 到有界 writer；完整 body/upload、取消和超限行为属于 DES-10 feasibility gate。

| 路径 | desktop 行为 |
|---|---|
| `/api/auth/**`、认证状态/profile/password/token 相关路径 | 拒绝普通 fetch，返回 `desktop_auth_required`；只能调用 `DesktopAuth` |
| `/api/**` 非流式且非认证 | 按 active repository/generation 路由 local Echo 或 RemoteClient |
| `/icons/**` 与图片代理 | 按 active generation 路由；响应使用 generation namespace 或 `Cache-Control: no-store`，禁止跨来源命中 |
| 四类流式路径 | 返回 `desktop_stream_required` |
| desktop 静态资源 | 内置 SPA fallback；immutable asset key 含 desktop build hash |

本地请求注入仅当前模式 token 后调用 Echo `ServeHTTP`；remote client 只接受已验证 base URL，过滤 hop-by-hop、`Set-Cookie` 与未经允许的 redirect。普通 transport 永不读取、返回、刷新或删除 token；认证拒绝由 `DesktopAuth`/AuthPort 统一分类。私有网段仍是正式 NAS/LAN 场景，但不扫描发现服务。

### 7.3 DesktopAuth 唯一 token 边界

注册、登录、saved-token 验证、认证状态、登出、profile/password 修改和 token 清理由 `DesktopAuth` Wails service 独占。local 直接调用共享 AuthService；remote 必须在地址/TLS/HTTP 确认与 compatibility 成功后调用远程 auth API。Credential Manager target 分离 local 与规范化 remote address；返回值只含用户、安全认证状态和稳定错误，不含 token。只有后端明确认证拒绝才删除对应凭据；network/TLS/compatibility/5xx 保留凭据。browser `AuthPort` 和现有 localStorage 完全不变。

### 7.4 operation-specific 流事件

调用方先生成 UUID `taskId` 并注册 listener/reorder state，再调用绑定。OPML adapter 还必须把该 caller task ID 传入/映射到现有全局 ImportTask ID；status、cancel、通知和终态始终携带 caller ID，禁止前端猜测后端全局 ID。

```ts
type TaskKey = { taskId: string; generation: number }
type TaskEnvelope<O extends string, K extends string, P> = {
  schemaVersion: 1; taskId: string; generation: number; sequence: number
  mode: 'local' | 'remote'; operation: O; kind: K; payload: P
}

type SummaryEvent =
  | TaskEnvelope<'summary','started',{}> | TaskEnvelope<'summary','cached',SummaryResult>
  | TaskEnvelope<'summary','chunk',{text:string}> | TaskEnvelope<'summary','completed',SummaryResult>
  | TaskTerminal<'summary'>
type TranslateEvent =
  | TaskEnvelope<'translate','init',TranslateInit> | TaskEnvelope<'translate','block',TranslateBlock>
  | TaskEnvelope<'translate','warning',TranslateNonTerminalError>
  | TaskEnvelope<'translate','completed',TranslateResult> | TaskTerminal<'translate'>
type BatchTranslateEvent =
  | TaskEnvelope<'batch-translate','started',BatchInit> | TaskEnvelope<'batch-translate','item',BatchItem>
  | TaskEnvelope<'batch-translate','completed',BatchResult> | TaskTerminal<'batch-translate'>
type OPMLImportEvent =
  | TaskEnvelope<'opml-import','snapshot',ImportTask> | TaskEnvelope<'opml-import','progress',ImportTask>
  | TaskEnvelope<'opml-import','completed',ImportTask> | TaskTerminal<'opml-import'>
```

`TaskTerminal<O>` 只能是该 operation 的 `cancelled` 或 `failed`，每任务终态唯一。translate `init` 是必达首个业务事件；单 block 的非终态错误用 `warning`，不得错误地结束整任务；远程协议的 done/error 映射必须与现有 Web 语义一致。

Go 为事件分配单调 sequence，但 WebView 交付可能乱序。前端按 `(generation, taskId)` 建有界 reorder buffer：只立即处理 `nextExpected`，暂存更大 sequence，重复/小于 next 的事件幂等丢弃；gap/窗口/时间上限触发 `event_gap` 并取消任务。`AckTask(taskId, generation, highestContiguousSequence)` 只 ACK 已连续 reducer 的最大 sequence，禁止 ACK 最大已见值。Go 仅释放 `<= highestContiguous` 的窗口；未确认窗口达到上限时暂停 producer/remote read（或按 operation 允许的 progress coalesce），超时明确失败，禁止丢 text/item 或无界内存。完整乱序、连续 ACK、背压与 reload 测试属于 feasibility gate。

`AbortSignal -> CancelTask`：local cancel context；remote cancel HTTP；OPML 同时桥接现有 DELETE。模式切换、退出、update、backup/restore 查询同一 `OperationCoordinator`。Web handler 仍是协议源，desktop adapter 不改 SSE/NDJSON。
## 8. 远程兼容性契约

无需认证的 `GET /api/compatibility` 返回 schema/product/API major 与 optional capabilities。`api.major` 是进入主界面的基础必需契约：该 major 对应的必需 endpoint、method、field、status 和 error semantics 全部由契约测试定义，缺任一项即 compatibility failure。capability 只能声明不属于基础契约的 optional capability；缺失时隐藏/禁用该功能并给出原因，不阻止基础主界面。不得定义“必需 capability”。

```json
{
  "schemaVersion": 1,
  "productVersion": "1.3.0",
  "api": { "major": 1 },
  "capabilities": ["optional.example.v1"]
}
```

客户端维护 supported-major 集合与每个 major 的基础 contract suite；引入新 major 后连续两个稳定产品版本同时验证新旧 major。productVersion 仅诊断提示。缺握手、非法响应、major 不支持或基础契约 probe 失败都在发送凭据前阻止 remote candidate；optional capability 缺失只影响对应可选入口。

RemoteClient 使用结构化 URL join，支持显式端口/path prefix，拒绝 userinfo/query/fragment。只自动跟随同 scheme/host/port 且留在确认 prefix 内的有界 redirect；跨地址或 HTTPS->HTTP 返回需重新确认的新 candidate，绝不携带凭据。TLS 使用 Windows 系统信任链且无绕过。
## 9. 数据、配置、凭据与恢复 journal

| 路径/存储 | 内容 | 备份策略 |
|---|---|---|
| `%LOCALAPPDATA%\Gist\data` | SQLite、icons、AI/readability cache、data-format marker | 纳入本地完整备份和升级恢复点 |
| `%LOCALAPPDATA%\Gist\desktop.json` | active 模式、remote URL/HTTP 同意、窗口/关闭/自启动/更新偏好 | 用户备份只导出非 remote 偏好；升级恢复点包含完整文件 |
| `%LOCALAPPDATA%\Gist\recovery` | fsynced operation journal、rollback pointers | 不纳入用户备份；完成并验证后清理 |
| `%LOCALAPPDATA%\Gist\logs` | 脱敏轮转日志 | 不纳入备份；14 天/50 MiB |
| `%LOCALAPPDATA%\Gist\updates` | 下载与临时恢复点 | 不纳入用户备份 |
| `%LOCALAPPDATA%\Gist\webview` | 固定 WebView2 user data | 非业务数据；不备份 |
| Windows Credential Manager | local token、按规范化地址绑定的 remote token | 不备份；按登出/移除/卸载清理 |

`desktop.json` 使用 schema version 与同目录 temp-write + flush/fsync + atomic replace。任何会跨文件改变 data/config/active version 的操作先写 `{transactionId, operation, phase, old/new paths, expected hashes/dataFormat}` journal 并 fsync 目录；每一不可逆步骤后推进 phase。进程启动在 `db.Open` 前 replay：能完成则幂等完成，不能完成则恢复旧指针/快照；恢复失败保留材料并停止，禁止继续迁移。

网络代理设置仍只属于 active repository 后端：local UI 直接读写 local SettingsService，remote UI 在线读写 remote API；失败不产生本地副本。Wails shell、RemoteClient、更新检查不读取资料库代理。
## 10. 前端构建与状态隔离

- Vite 使用两个显式 entry/config contract：`web` entry 保留现有 PWA plugin、manifest、Service Worker、浏览器 auth/file/download；`desktop` entry 从编译图中排除 PWA 注册/更新 UI，并输出 Go embed 目录。不得用运行时 UA 猜测平台。
- `src/platform` 定义 `AuthPort`、`StreamPort`、`FilePort`、`ShellPort`；业务 hooks 不引用 Wails。browser port 保留现状，desktop port 才引入精确锁定 runtime/bindings。
- repository root 由 active `generation` keyed remount。Query key、图片/图标 URL、task key、IndexedDB/object URL/cache namespace 都包含 generation；无法 namespace 的资源使用 `no-store`。
- commit 使用第 6 节穷举 reset registry，清除 Query、Zustand、translation/lightbox/selection/scroll/router detail/任务通知/object URL；旧 generation callback 在 reducer 边界丢弃。
- desktop build 不读写 `gist_auth_token`；browser/PWA 的 localStorage 行为不变。
- 来源标识只由 active repository 驱动；candidate 未提交前继续显示原来源。
## 11. Windows 桌面生命周期

- 数据独占依赖 DES-02 用户级跨会话锁；Wails `SingleInstanceOptions` 只在 Wails application 创建后处理同会话 second-launch Restore/Focus，初始化期间的竞争仍由数据锁拒绝第二 runtime。
- 正常 `WindowClosing`/tray/`ShouldQuit` 共用 `OperationCoordinator`：关闭偏好决定询问、隐藏或完全退出；完全退出需前台任务二次确认，隐藏不取消任务。
- Windows session ending 使用独立 native hook 处理 `WM_QUERYENDSESSION/WM_ENDSESSION`，标记 non-interactive shutdown 并走第 5.2 节 deadline 路径；不得弹 close/task/update/backup 对话框，不得因用户未确认而 veto 系统结束。
- 登录自启动默认关闭；启用后用当前用户启动项和 `--background`，先完成 lock/journal/runtime，再隐藏到 tray，不因 remote 错误抢焦点。
- 通知只用于隐藏状态下更新与用户前台任务终态；payload 只有 task ID/type/result/generation，不含正文或凭据。激活先寻找已有实例，Restore/Focus 后按 generation 导航；来源已不可进入时显示保留的明确结果。
- 原生对话框承担 OPML、backup、restore、diagnostic；取消无副作用，remote import 只传内容。
- WebView 只允许内置资源/内部路由；用户触发 HTTP(S) 外链交给默认浏览器，其他或程序化外部导航阻止。
## 12. 本地完整备份与恢复

备份格式使用版本化 envelope：固定 magic/header + 通过用户密码创建的 `age` scrypt recipient + 加密的 tar payload。payload 包含 manifest、文件大小/哈希、local data snapshot 与不含远程信息的桌面偏好。采用成熟库完成 KDF、认证加密和流式处理，不自行设计密码算法。

创建备份与恢复都由 `OperationCoordinator` 仲裁并复用 runtime `Quiesce`/reopen；backup/restore 不得只检查 UI task 而遗漏 scheduler、图标回填或其他 writer。创建：检查空间 -> journal/fsync -> 阻止新 local writes -> quiesce 全部 runtime-owned writer -> 一致性 snapshot -> 流式加密到同目录临时文件 -> fsync/原子改名 -> reopen/清 journal。失败时幂等删除临时输出并恢复 runtime。

恢复：解密到 staging -> 完整 AEAD/manifest/path/size/hash/data-format 验证 -> 覆盖确认 -> journal/fsync -> quiesce -> rollback snapshot -> 原子替换 -> reopen/integrity check -> commit/清 journal。崩溃后 startup replay 依据 journal 完成或回滚；远程连接/token/资料库不参与。

所有归档路径 clean + secure join，拒绝绝对路径、`..`、符号链接和超限内容。格式兼容由显式 data-format version/range 判定，不以程序版本猜测。

## 13. 更新、NSIS 与自动回滚

### 13.1 更新 manifest

同一稳定 GitHub Release 发布签名 NSIS 与 detached signed manifest。manifest 至少包含 schema、product/version、channel、release sequence（或等价单调版本）、publishedAt、Windows x64 URL/size/SHA-256、payload data-format min/max、支持 API majors、签名 key ID。客户端先验证 Ed25519 trust set，再验证 SHA-256、detached signature 与 WinVerifyTrust；任一步失败拒绝执行且保留脱敏证据。

客户端持久化已接受 stable release 的最高单调序列/版本与 manifest digest。自动或手动检查都拒绝较低序列、同序列不同 digest、已撤销 key、prerelease/test channel 和签名时间/元数据不一致；网络重放旧的合法 manifest 不得重新进入 pending。只有明确设计并签名的 recovery policy 才能放宽，首版无 UI 降级。

启动时距上次自动检查超过 24 小时才异步检查，持续运行每 24 小时，手动检查不受间隔；检查失败不阻断。manifest 的 API majors 仅用于升级后 remote 提示，不是安装门禁。

公钥轮换采用重叠信任：先发布 old/new trust set，再切 CI key，最后经兼容窗口移除 old。测试 key/channel 与 stable 隔离。

### 13.2 版本化安装与恢复事务

固定目录采用稳定 launcher + 版本化 payload：

```text
%LOCALAPPDATA%\Programs\Gist\
  Gist.exe                 # 小型、签名、稳定 launcher
  versions\1.3.0\...       # Wails app payload
  versions\1.2.0\...       # 仅在待验证升级期间保留
  current.json             # active/pending version + transaction id
```

NSIS 是唯一正式安装入口。首次确认可下载；无前台任务且第二次确认后，当前 app 创建一致性恢复点和 update transaction，优雅退出并启动 NSIS。NSIS 安装新版本到独立目录，把它标为 pending，再由 launcher 启动。

安装器使用当前用户级上下文、固定 `%LOCALAPPDATA%\Programs\Gist`，不显示目录选择且正常安装/升级/卸载不请求管理员权限。所有注册表、自启动、快捷方式和卸载信息写入当前用户范围。

新版本必须在超时内完成：launcher 验证 pending payload 签名与 transaction -> app 在 `db.Open` 前核对 payload 声明的 data-format fence -> config/data migration -> integrity/smoke query -> Wails 主窗口或 background runtime ready -> 以 transaction ID 提交 health。只有 commit 后才切 active 并清理旧 payload/恢复点。

进程无法启动、提前退出、fence 不匹配、迁移/完整性失败或超时，由外部 launcher/coordinator 终止 pending、恢复 previous active pointer、恢复 local data/config snapshot并启动上一正式版本。launcher 与 app 都读取同一 fsynced journal；回滚失败保留材料并停止循环。

禁止依赖 ACL 阻止旧 exe。每个 payload 声明 `minReadableDataFormat/maxReadableDataFormat`，data 目录持久化当前 format；launcher 在创建目标进程前、app 在 `db.Open` 前双重检查。旧 payload 或手工启动若不支持当前 data format 必须失败关闭。正常 health commit 后不提供任意降级。

### 13.3 统一发布

- 根 `VERSION` 作为产品版本源，CI 要求 Tag `vX.Y.Z` 与其一致，并向 Go、Vite、compatibility endpoint、NSIS 和 Docker label 注入同一值。
- Release 先运行 Linux backend/Web/Docker 与 Windows desktop 全部测试，按 digest/暂存资产构建；NSIS 签名、manifest 签名和安装矩阵通过后才发布非 draft GitHub Release 与稳定 Docker tags。
- Windows runner 验证 clean install、覆盖升级、默认卸载保留、勾选删除、自动回滚和二次更新确认。
- 卸载器默认只删除程序与 Gist 创建的当前用户集成；“删除所有 Gist 用户数据”默认不勾选，勾选并再次确认后才删除列明的 data/config/log/update/webview 目录和 Gist Credential Manager 项。
- Wails 依赖升级单独提交并运行完整桌面矩阵，不在普通功能提交中顺带升级。

## 14. 错误、日志与诊断

Desktop binding 使用稳定 `DesktopError { code, message, retryable, field?, operationId? }`；HTTP API 错误保持原样。错误至少区分 lock、recovery、mode/candidate、auth、network/TLS/redirect/compatibility、credential-store、body-limit、task-gap/backpressure、backup、update-replay、data-format 和 rollback。

所有 Go/Wails/launcher/installer 日志进入同一字段级 redaction policy：默认拒绝未知结构，允许列举 event/code/version/phase/host hash 等安全字段；禁止 Authorization/Cookie、token/password/key、完整 URL/query/path、request/response body、文章/翻译文本、OPML 内容、备份内容与本机文件路径。错误链在写 sink 前统一脱敏，不能依赖调用者记得处理。文件 sink 14 天/50 MiB 轮转；诊断导出只含版本、Windows/WebView2 摘要和已脱敏日志，不上传。
## 15. 验证策略

### 15.1 共享回归

- Linux：现有 server `go test ./... -race`、lint、Web build/test/typecheck、Docker 构建继续通过，Docker build graph 不含 Wails/Windows。
- Windows：先修复既有两项平台基线；验证 Snowflake 单次初始化、server/desktop runtime ownership、关闭顺序及 desktop 无监听 socket。
- 同一普通 API contract suite 比对 browser handler、desktop local 与 remote proxy；Web/PWA HTTP/SSE/NDJSON/PWA 行为不可变。

### 15.2 可执行契约

- **DES-02/06**：两个不同 Windows session/进程争夺同一 data dir，仅一方可在 migration/db.Open 前继续；kill 注入后锁自动释放。对 journal 每个 phase kill，重启在 db.Open 前完成或回滚。
- **DES-03**：candidate 各阶段失败均保留 active；commit 恰好 generation+1。相同 Snowflake ID fixture 验证 keyed remount/reset、Query/Zustand/IndexedDB/object URL/图片/通知无串库。
- **DES-04**：认证/profile 路由普通 fetch 全拒绝；DesktopAuth 成功/401/network/TLS 矩阵验证 token 不出 Credential boundary。AssetServer 覆盖 body/upload/response/image 上限、取消和常量级内存。
- **DES-04 流**：每 operation 验证事件合法集合；translate init、warning 非终态、唯一终态；注入乱序/重复/gap，证明 reducer 顺序和 `highestContiguousSequence` ACK；慢/失联前端证明有界背压且 text/item 不丢。OPML caller ID 到 global task 的 status/cancel 映射一致。
- **DES-05**：模式切换、退出、update、backup、restore 对同一前台 task/runtime writer fixture 得出一致门禁；quiesce 后无 DB writer。
- **DES-07**：正常 close 有交互；模拟 `WM_QUERYENDSESSION/WM_ENDSESSION` 无 modal、deadline 内关闭 DB；通知 activation 唤醒已有实例并按 generation 导航。
- **DES-08**：旧合法 manifest replay、同序列异 digest、测试 key/channel、payload 降级、手工旧 exe、每个 launcher 交接点故障均失败关闭且保留恢复材料。

### 15.3 feasibility gate 与 Windows E2E

在任何功能扩展前，独立 tracer prototype 必须在锁定 `.117/.97` 上提交可复现证据：完整 JSON body、OPML upload、超限 body/image、operation event 吞吐、乱序 reorder、连续 ACK/背压、取消、WindowClosing/session-ending hook、toast activation、launcher handoff/health timeout。任一项失败即阻止后续 child，回到设计；localhost listener 不得作为替代。

最终 E2E 覆盖 clean profile、local/remote auth与切换、remote UI 下 local scheduler、single instance/tray/background、文件对话框、backup/restore、签名 update/rollback/uninstall，以及 Windows 10/11 x64。
## 16. 实施切片与追踪

设计到已确认 PRD 的基线映射如下；实施计划可细分但不得删除这些边：

| Design | PRD acceptance anchors |
|---|---|
| DES-01 | AC-PROD-01、AC-PROD-17..20 |
| DES-02、DES-06 | AC-PROD-05（并以 AC-PROD-01 保护 server/no-listener） |
| DES-03 | AC-PROD-02..05 |
| DES-04、DES-09 | AC-PROD-06..10 |
| DES-05 | AC-PROD-09..10、AC-PROD-15 |
| DES-07 | AC-PROD-11..14 |
| DES-08 | AC-PROD-17..20 |
| DES-01、DES-07（日志/诊断与生命周期合同） | AC-PROD-16 |
| DES-10 | AC-PROD-21 |
| 全部设计与 child 追踪 | AC-PLAN-03..05；启动仍受 AC-PLAN-06 门禁 |

父任务只维护设计和集成验收；评审后创建约 13–15 个小型 child，每个 child 具备单一可观察合同、独立失败边界和 focused verification。建议按以下能力拆分，具体编号由 `implement.md` 固化：Windows 基线/版本源；用户级数据锁与 startup recovery；共享 runtime/Snowflake/writer registry；Wails feasibility prototype；desktop build/shell；compatibility/RemoteClient；ModeManager/DesktopAuth/前端隔离；流式 operation adapters；Windows lifecycle/notification/native files；OperationCoordinator；backup/restore；launcher/update security；release/E2E。

实施计划必须维护 `PRD AC ID -> DES-xx -> Child ID -> verification command/scenario -> evidence path` 矩阵。没有 AC/DES 映射、依赖门禁或证据位置的 child 不得启动。每个 child 都保护 Web/PWA/Docker；不得以“后续会修”为由合入回归。
## 17. 主要取舍与硬门禁

- 混合 transport 增加适配层，但保留 Web 流协议并避免 localhost 攻击面。
- 跨会话数据锁必须早于 Runtime/Wails；Wails single instance 只解决用户体验，不承担数据完整性。
- active + candidate transaction 使验证失败不污染当前资料库；generation keyed remount/reset 以明确成本换取不串库保证。
- `DesktopAuth` 双实现避免 token 进入 WebView；ordinary proxy 因而必须排除全部认证/profile 路由。
- runtime-owned writer registry 与统一 `OperationCoordinator` 增加生命周期纪律，但让 switch/exit/update/backup/restore 使用同一可验证静止点。
- launcher 增加安装复杂度，但能覆盖新 exe 无法启动；anti-replay、journal 与 data-format fence 分别处理旧清单重放、崩溃中间态和旧程序误开新数据，不能由 ACL 替代。
- DES-10 列出的完整 Wails body/upload、乱序事件/连续 ACK/背压、关闭 hook、通知激活和 launcher 交接 prototype 是独立硬门禁，不是“实现时再看”的软验证；证据缺失即不得继续产品实现。
