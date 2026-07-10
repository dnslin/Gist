# Wails 3 桌面客户端技术设计

> 状态：proposed，等待用户整体评审
> 日期：2026-07-11
> 目标平台：Windows 10/11 x64

## 1. 决策摘要

1. 保留 `backend/cmd/server`、现有 HTTP API、Docker 和 Web/PWA 交付路径；新增 `backend/cmd/desktop`，不以桌面入口替换服务端入口。
2. 从当前 `cmd/server` 抽取可复用的 application runtime，统一装配 SQLite、repository、service、handler、scheduler 和关闭顺序；server 与 desktop 只负责各自平台生命周期。
3. 桌面普通 `/api` 与 `/icons` 请求经 Wails AssetServer 中的模式感知 `http.Handler` 路由到本地 Echo 或远程 Gist 服务；不监听 localhost/LAN 端口。
4. AssetServer 无 `http.Flusher`，因此摘要、翻译、批量翻译和 OPML 进度改走类型化 Wails 任务事件；Web/PWA 继续使用原 SSE/NDJSON，业务 service 不复制。
5. 模式、远程连接、桌面偏好和令牌由 Go 桌面层持有。令牌进入 Windows 凭据管理器，WebView 不持久化令牌。
6. 本地 runtime 在桌面进程存续期间始终运行；切到远程模式只改变当前资料库和请求路由，不停止本地 scheduler。
7. Windows 生命周期由 Wails 单实例、窗口 hook、托盘、通知和原生对话框承载；桌面构建禁用 PWA Service Worker。
8. 正式更新仍由签名 NSIS 完成。固定启动器和版本化程序目录负责首次启动健康检查与自动回滚，不使用 Wails 可执行文件自交换。
9. 首个锁定工具链组合为 Wails CLI/Go module `v3.0.0-alpha2.117` 与 `@wailsio/runtime@3.0.0-alpha.97`；所有依赖使用精确版本并经过联合 smoke test。

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
| `internal/application` | 构造和关闭共享本地 runtime | 引用 Wails、窗口或 Windows API |
| `cmd/server` | 环境变量、HTTP listener、signal、pprof | 重复装配 repository/service |
| `cmd/desktop` | embed 桌面前端、创建 Wails app | 承载业务规则 |
| `internal/desktop/mode` | 模式状态机、来源 generation、切换门禁 | 复制资料库数据 |
| `internal/desktop/transport` | AssetServer 路由、本地 handler、远程 client | 开启网络 listener |
| `internal/desktop/tasks` | 流式任务、事件、取消、背压、通知状态 | 改写 Web SSE handler 契约 |
| `internal/desktop/auth` | 本地/远程认证 facade 与令牌注入 | 把令牌返回给持久化前端存储 |
| `internal/desktop/platform/windows` | Credential Manager、自启动、签名验证、单实例辅助 | 进入共享业务包 |
| `internal/desktop/backup` | 一致性快照、加密备份、验证、原子恢复 | 读取远程资料库 |
| `internal/desktop/update` | manifest、下载、验证、恢复事务 | 直接替换运行中的 Wails exe |
| `frontend/src/platform` | browser/desktop ports 与运行时选择 | 在业务组件中散布 `window.wails` 判断 |

所有组件通过构造函数注入；不新增可变包级全局状态。`Runtime.Close(ctx)`、任务取消和桌面关闭必须可重复调用且顺序确定。

## 5. 共享 Application Runtime

### 5.1 构造契约

`application.NewRuntime(options)` 负责当前 `cmd/server/main.go` 中从打开数据库到创建 router 的装配工作，返回单个拥有者对象：

```go
type RuntimeOptions struct {
    DataDir           string
    DBPath            string
    StaticDir         string
    EnableSwagger     bool
    SchedulerInterval time.Duration
    StartScheduler    bool
}

type Runtime struct {
    Router            http.Handler
    Auth              service.AuthService
    AI                service.AIService
    OPML              service.OPMLService
    ImportTasks       service.ImportTaskService
    // private scheduler, db, background contexts and closers
}

func NewRuntime(ctx context.Context, options RuntimeOptions) (*Runtime, error)
func (r *Runtime) Close(ctx context.Context) error
```

- 构造失败必须按逆序关闭已经创建的资源。
- scheduler、图标回填和数据库由 runtime 唯一拥有。
- `cmd/server` 继续以 `StartScheduler=true` 创建 runtime，再启动 Echo listener；外部 HTTP 行为不变。
- `cmd/desktop` 固定使用 `%LOCALAPPDATA%\Gist\data`，并在 UI 模式为远程时仍保持本地 runtime 与 scheduler 运行。
- 桌面不启动 pprof 或 Echo listener；server 的 pprof 与 signal 行为保持原样。

### 5.2 启动与关闭顺序

桌面启动顺序：单实例锁 -> 路径/日志 -> 桌面配置 -> 凭据存储 -> 本地 runtime 与迁移 -> Wails services/AssetServer -> tray/window -> 后台更新检查。

关闭顺序：拒绝新任务 -> 取消/等待前台任务 -> 停止 scheduler -> 关闭 service -> 停止回填 -> 关闭 SQLite -> 写入桌面偏好 -> 结束 Wails app。Windows 注销/关机不弹交互对话框，只执行有时限的最佳努力关闭；数据库一致性优先于等待非关键后台工作。

## 6. 模式状态机与资料库隔离

```text
Unselected
   | choose
   +--> LocalValidating --> LocalAuth --> LocalActive
   |
   +--> RemoteValidating --> RemoteAuth --> RemoteActive
              |                  |
              +---- RemoteError -+

Active -- switch requested --> task gate --> validate target --> atomic commit
```

`ModeManager` 暴露不可变快照：

```ts
interface ModeSnapshot {
  mode: 'unselected' | 'local' | 'remote'
  generation: number
  status: 'validating' | 'auth-required' | 'active' | 'error'
  sourceLabel: string
  insecureHTTP: boolean
  errorCode?: string
}
```

- 每次成功切换递增 `generation`。请求和任务捕获启动时 generation；旧 generation 的迟到结果不能进入 UI。
- 切换提交前保留当前模式。候选连接验证失败时不改变当前模式、启动模式或查询缓存。
- 切换提交时先取消普通请求，再清空 TanStack Query、translation/lightbox/selection 等资料库相关 Zustand 状态，最后发布新快照并重新加载。
- 前台任务由 `TaskRegistry` 提供门禁。用户未确认取消时不提交切换；后台 scheduler 不参与门禁。
- 远程错误、令牌失效和 API 不兼容均保留远程来源，不自动进入本地。

## 7. 传输设计

### 7.1 Browser transport 保持不变

浏览器构建继续使用当前 `fetch`、Bearer/localStorage、SSE/NDJSON、文件 input/download 和 Service Worker。服务端路由、状态码、JSON、流式响应与 Swagger 均保持兼容。

### 7.2 Desktop 普通请求

`DesktopAssetHandler` 包装 `application.BundledAssetFileServer`：

| 路径 | 桌面行为 |
|---|---|
| `/api/**`（非流式） | 按 ModeSnapshot 路由到 local Echo 或 RemoteClient |
| `/icons/**` | 按模式路由图标资源 |
| 四类流式路径 | 返回结构化 `desktop_stream_required`，防止误走无 Flusher 路径 |
| 其他路径 | 交给内置静态前端并支持 SPA fallback |

本地请求克隆后注入本地令牌，再调用 Echo `ServeHTTP`。远程请求由专用 client 重新构造 URL、复制允许的 header/body、注入远程令牌并把 status/header/body 写回 AssetServer。两条路径均不向 WebView暴露持久令牌。

远程代理必须移除 hop-by-hop header、远程 `Set-Cookie` 和未经允许的重定向；只接受已验证 base URL 下的 `/api` 与 `/icons`。私有网段不被禁止，因为 NAS/LAN 是正式场景，但所有目标只能来自用户确认的远程服务地址。

### 7.3 桌面认证端口

登录、注册、令牌验证、登出和密码修改使用 `DesktopAuth` Wails service，而不让前端解析后再持久化 JWT：

- local：直接调用共享 `AuthService`，成功后写本地 Credential Manager 项。
- remote：通过已验证 RemoteClient 调用现有 auth API，成功后写绑定规范化地址的远程凭据项。
- 返回前端的数据只包含用户、认证状态和错误，不包含持久令牌。
- 普通 API 401 由桌面 transport 分类；仅明确认证拒绝才删除当前模式凭据。
- browser `AuthPort` 继续使用当前 API/localStorage，desktop `AuthPort` 使用生成绑定。

### 7.4 流式任务事件

桌面前端启动时先注册一个全局 `gist:task-event` listener。调用方生成 UUID task ID，再调用对应 Wails service，因此订阅先于首个事件，不存在“返回 task ID 前丢首块”的竞态。

事件使用判别联合而不是 `any`：

```ts
interface TaskEventBase {
  schemaVersion: 1
  taskId: string
  mode: 'local' | 'remote'
  generation: number
  sequence: number
  operation: 'summary' | 'translate' | 'batch-translate' | 'opml-import'
}

type DesktopTaskEvent =
  | (TaskEventBase & { kind: 'started' })
  | (TaskEventBase & { kind: 'cached'; payload: CachedPayload })
  | (TaskEventBase & { kind: 'chunk'; payload: TextChunk })
  | (TaskEventBase & { kind: 'item'; payload: TranslateOrBatchItem })
  | (TaskEventBase & { kind: 'progress'; payload: ImportTask })
  | (TaskEventBase & { kind: 'completed'; payload?: CompletionPayload })
  | (TaskEventBase & { kind: 'cancelled' })
  | (TaskEventBase & { kind: 'failed'; error: DesktopError })
```

| 操作 | local adapter | remote adapter | 前端保持的语义 |
|---|---|---|---|
| summary | 调用 AI service channel | 解析远程原始文本流/缓存 JSON | 增量文本或 cached result |
| translate | 调用 block translation service | 解析远程 SSE JSON | init、block、done/error |
| batch translate | 调用 batch service | 解析远程 NDJSON | 每篇结果按到达顺序 |
| OPML progress | 订阅/轮询 ImportTaskService | 解析远程 SSE | 初始状态、进度、终态 |

- 每个 task 由单 goroutine 分配严格递增 sequence；终态只能出现一次。
- 相邻文本块可以按短时间/大小阈值合并，但不能丢失或重排；进度事件允许合并为最新值。
- Go 侧维持有界未确认窗口，前端按 sequence 回 ACK；超过窗口或前端长期不可用时明确取消/失败，禁止无界缓存。
- `AbortSignal` 由 desktop adapter 映射到 `CancelTask(taskId)`。local 取消 context；remote 取消 HTTP request；OPML 还调用现有 DELETE 取消端点。
- 模式切换、完全退出和更新安装均查询同一 TaskRegistry，避免 UI 与后台对“是否有前台任务”得出不同结论。
- Web handler 仍是协议兼容源；local desktop adapter 直接复用同一 service，remote adapter 只解析既有远程协议。

## 8. 远程兼容性契约

新增无需认证的 `GET /api/compatibility`：

```json
{
  "schemaVersion": 1,
  "productVersion": "1.3.0",
  "api": { "major": 1 },
  "capabilities": [
    "ai.summary.stream.v1",
    "ai.translate.stream.v1",
    "ai.batch.stream.v1",
    "opml.import.progress.v1"
  ]
}
```

- `api.major` 定义必需契约；同一 major 只允许兼容性新增。
- capability 只控制可选功能，不能掩盖必需端点缺失。
- 客户端维护显式 supported-major 集合。引入新 major 的连续两个稳定产品版本同时运行新旧契约测试。
- product version 只用于诊断和提示，不作为连接相等条件。
- 缺少 endpoint、响应非法、major 不支持或必需 capability 缺失时，在发送凭据前阻止远程主界面。

RemoteClient 使用结构化 URL join，支持端口和反向代理 path prefix，拒绝 userinfo/query/fragment。只有同 scheme/host/port 且仍在确认 prefix 内的有界重定向可自动跟随；其余重定向转成“确认新地址”结果。TLS 使用 Windows 系统信任链，不提供绕过。

## 9. 数据、配置与凭据

| 路径/存储 | 内容 | 备份策略 |
|---|---|---|
| `%LOCALAPPDATA%\Gist\data` | SQLite、icons、AI/readability cache | 纳入本地完整备份和升级恢复点 |
| `%LOCALAPPDATA%\Gist\desktop.json` | 模式、远程 URL、HTTP 同意、窗口/关闭/自启动/更新偏好 | 备份只导出非远程偏好；升级恢复点包含完整文件 |
| `%LOCALAPPDATA%\Gist\logs` | 脱敏轮转日志 | 不纳入完整备份；14 天/50 MiB |
| `%LOCALAPPDATA%\Gist\updates` | 下载、事务 marker、临时恢复点 | 不纳入用户备份；事务后清理 |
| `%LOCALAPPDATA%\Gist\webview` | 固定 WebView2 user data | 不作为业务数据或备份源 |
| Windows Credential Manager | local token、按地址绑定的 remote token | 不纳入备份；按登出/移除/卸载规则清理 |

`desktop.json` 使用 schema version 与原子 temp-write/replace。敏感令牌不得进入文件；远程地址和 HTTP 同意虽然非秘密，日志中仍只记录脱敏 host。

网络代理设置只属于当前资料库后端：local UI 读写 local SettingsService，remote UI 读写 remote `/api/settings/network`。Wails 外壳、RemoteClient 和更新服务不读取该设置，也不存在本地/远程配置同步副本。

## 10. 前端构建与状态隔离

- `vite.config.ts` 根据明确 build mode 生成 Web 与 desktop 两个产物。Web 保留 PWA plugin；desktop 禁用 manifest、Service Worker 注册和 PWA 更新提示。
- desktop 产物写入可供 Go `embed` 的生成目录，例如 `backend/cmd/desktop/assets`；该目录由构建任务生成，不手工维护。
- `src/platform` 提供 `AuthPort`、`StreamPort`、`FilePort`、`ShellPort` 和 runtime detection。业务 hooks 依赖 ports，不直接依赖 Wails runtime。
- browser port 保持现有实现；desktop port 只在 desktop build 中引入精确锁定的 `@wailsio/runtime` 和生成 bindings。
- 模式切换必须 `cancelQueries` 后 `queryClient.clear()`，并重置所有含 entry/feed ID 的 Zustand/IndexedDB/UI 状态；不能只 invalidate，因为本地与远程可能复用相同 Snowflake ID。
- 非敏感 UI 偏好可以继续存在受控桌面偏好中；`gist_auth_token` 在 desktop build 中不得读取或写入。
- 当前资料库来源在主界面常驻显示，并由 ModeSnapshot 驱动，不能从 URL 或缓存猜测。

## 11. Windows 桌面生命周期

- Wails `SingleInstanceOptions` 使用稳定应用 ID；第二次启动只 Restore/Focus 已有窗口，不再打开数据库或 scheduler。由 OS 锁处理异常退出，不自建永久 lock file。
- `WindowClosing` hook 取消默认关闭并调用统一 close coordinator。用户选择托盘时 Hide；完全退出时先执行前台任务门禁，再 `app.Quit()`。
- `ShouldQuit` 与托盘“退出”共用同一 coordinator，防止不同入口绕过任务确认。
- close coordinator 从 `desktop.json` 读取“每次询问/托盘/退出”偏好；设置重置后恢复每次询问，模式切换不改变该全局偏好。
- 登录自启动默认关闭且安装器不得开启。用户启用后使用当前用户级 Windows 启动项并传入 `--background`；Wails window 以 Hidden 启动，本地 runtime 仍先完成迁移和 scheduler 启动。
- 托盘提供显示和退出；更新状态可改变 tooltip/menu，不主动抢焦点。
- Windows 通知只用于隐藏状态下的更新提示和用户前台任务终态，payload 不含内容或凭据；点击后恢复已有窗口。
- 原生对话框承担 OPML、备份、恢复和诊断包文件选择。远程导入只上传内容，不发送本机路径。
- WebView navigation hook 只允许内置资源；用户触发的 HTTP/HTTPS 外链交给 Windows 默认浏览器。

## 12. 本地完整备份与恢复

备份格式使用版本化 envelope：固定 magic/header + 通过用户密码创建的 `age` scrypt recipient + 加密的 tar payload。payload 包含 manifest、文件大小/哈希、local data snapshot 与不含远程信息的桌面偏好。采用成熟库完成 KDF、认证加密和流式处理，不自行设计密码算法。

创建备份：检查空间 -> 阻止新的本地前台写任务 -> 停止 scheduler/等待数据库访问 -> 关闭/快照 local data -> 流式加密到目标同目录临时文件 -> fsync/原子改名 -> 重开 runtime。失败时删除临时输出并恢复 runtime。

恢复：解密到 staging -> 完整读取并验证 AEAD、manifest、路径、大小、哈希和支持版本 -> 显示覆盖确认 -> 停止 local runtime -> 创建受保护 rollback snapshot -> 原子替换 local data/本地偏好 -> 重开和完整性检查。任一步失败都恢复 rollback snapshot；远程连接、远程 token 和远程资料库不参与事务。

所有归档路径必须 clean + secure join，拒绝绝对路径、`..`、符号链接和超出限额的解压内容。

## 13. 更新、NSIS 与自动回滚

### 13.1 更新 manifest

同一稳定 GitHub Release 发布签名 NSIS 与 detached signed manifest。manifest 至少包含 schema、product/version、channel、publishedAt、Windows x64 asset URL/size/SHA-256、支持 API majors、签名 key ID。客户端先用内置 Ed25519 trust set 验证 manifest，再验证下载文件 SHA-256、detached signature 和 WinVerifyTrust Authenticode；任一步失败即删除待安装状态并拒绝执行。

客户端只查询非 prerelease 的稳定 GitHub Release。启动时距上次自动检查超过 24 小时才异步检查，持续运行时每 24 小时检查一次，手动检查不受间隔限制。托盘状态只通知，不抢焦点。

manifest 中的 API 支持范围只用于升级后远程握手提示，不构成安装门禁；即使已保存远程服务将不兼容，用户仍可安装并继续使用本地模式，原远程连接和令牌保留。

公钥轮换采用重叠信任：先发布同时信任 old/new 的客户端，再切换 CI 私钥，最后经过兼容窗口移除 old key。测试产物使用隔离 key/channel，不得进入 stable manifest。

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

新版本必须在超时内完成：进程启动 -> desktop config migration -> local DB migration -> `PRAGMA integrity_check`/必要 smoke query -> Wails 主窗口或 background runtime ready。成功后向 launcher/update coordinator 提交带 transaction ID 的 health signal，才原子切换 active 并清理旧 payload/恢复点。

进程无法启动、提前退出、迁移失败、完整性失败或超时都由 app 外部的 launcher/coordinator 执行：终止 pending -> 恢复 previous active pointer -> 恢复 local data 与 desktop config snapshot -> 启动上一正式版本。回滚自身失败时保留全部恢复材料并停止循环启动。

正常 health commit 后不提供任意降级。手工直接运行旧 payload 被 launcher/ACL/路径设计阻止，不允许旧 schema 打开新数据。

### 13.3 统一发布

- 根 `VERSION` 作为产品版本源，CI 要求 Tag `vX.Y.Z` 与其一致，并向 Go、Vite、compatibility endpoint、NSIS 和 Docker label 注入同一值。
- Release 先运行 Linux backend/Web/Docker 与 Windows desktop 全部测试，按 digest/暂存资产构建；NSIS 签名、manifest 签名和安装矩阵通过后才发布非 draft GitHub Release 与稳定 Docker tags。
- Windows runner 验证 clean install、覆盖升级、默认卸载保留、勾选删除、自动回滚和二次更新确认。
- 卸载器默认只删除程序与 Gist 创建的当前用户集成；“删除所有 Gist 用户数据”默认不勾选，勾选并再次确认后才删除列明的 data/config/log/update/webview 目录和 Gist Credential Manager 项。
- Wails 依赖升级单独提交并运行完整桌面矩阵，不在普通功能提交中顺带升级。

## 14. 错误、日志与诊断

Desktop binding 使用统一 `DesktopError { code, message, retryable, field? }`，MarshalError 输出稳定 JSON；HTTP API 错误格式保持原样。错误码至少区分 mode、auth、network、certificate、compatibility、redirect、credential-store、task、backup、update 和 rollback。

应用日志继续通过 `pkg/logger`，Wails 所需 `slog.Logger` 使用 adapter 写入同一脱敏 sink。日志禁止完整 URL、令牌、密码、AI key、文章正文和备份内容。桌面文件 sink 按 14 天/50 MiB 轮转；诊断导出只打包版本/环境摘要和脱敏日志，不自动上传。

## 15. 验证策略

### 15.1 共享回归

- Linux：现有 `go test ./... -race`、lint、Web build/test/typecheck、Docker 构建继续通过。
- Windows：先修复当前两项平台测试基线，再增加 `go test ./...`、desktop build 和 Wails toolchain pin 检查。
- server composition extraction 前后运行 router、scheduler、migration 与 API contract 测试，证明 HTTP 行为不变。

### 15.2 Transport contract

- 用同一测试向 browser HTTP handler、desktop local handler 和 desktop remote proxy 发出普通 API 请求，比对 status/header/body。
- 四类流式操作验证首事件时延、事件顺序、缓存命中、增量、取消、错误、终态唯一和背压上限。
- 远程测试覆盖 HTTP 同意、系统 TLS、证书错误、path prefix、同源 redirect、跨地址 redirect、API major/capability 和无握手旧服务。
- 模式测试使用相同 ID 的两套假资料库，证明 Query/Zustand/图片/任务结果不会串库。

### 15.3 Windows E2E

- clean profile 首启模式选择、本地注册登录、远程登录、切换与重启自动登录。
- 单实例、托盘、关闭偏好、自启动 background、任务通知、外链和原生文件对话框。
- 本地 scheduler 在 remote UI/托盘状态继续运行，remote scheduler 不重复运行。
- NSIS 签名、更新下载验证、任务门禁、首次启动 commit、故障自动回滚、卸载数据选择。
- 备份跨 Windows 用户/设备恢复、错误密码、篡改、空间不足和中途失败回滚。

## 16. 实施切片

该范围不应作为单个巨型实现提交。评审后建议让当前任务作为父级规划，按以下可独立验收的 child tasks 执行：

1. Windows 基线修复与统一版本源。
2. 共享 application runtime 抽取，server/Docker 零行为变化。
3. Wails 锁定工具链、desktop build 和最小 shell tracer bullet。
4. compatibility endpoint、RemoteClient 与普通 mode-aware transport。
5. ModeManager、Credential Manager、desktop auth 与前端缓存隔离。
6. 四类流式任务事件适配与取消/背压契约。
7. 托盘、关闭、单实例、自启动、通知、外链和原生文件集成。
8. 本地完整备份/恢复。
9. NSIS、签名更新、版本化 launcher 和自动回滚。
10. 统一 Release 流水线与 Windows 端到端回归。

每个 child task 必须保持 Web/PWA 回归门禁；后续切片不得以“最终会修复”为由破坏已完成切片。

## 17. 主要取舍与待验证点

- 混合 transport 比全量 bindings 多一层适配，但显著减少普通 API 改动并保留 Web 流式协议。
- 版本化 launcher 增加安装复杂度，但这是覆盖“新 exe 根本无法启动”自动回滚的必要外部执行边界。
- local runtime 始终运行会使用后台资源，但符合已确认的本地定时刷新语义。
- desktop auth bridge 增加少量双实现，但避免令牌进入 WebView 持久存储。
- 实现前需用最小 prototype 验证 Wails `.117/.97` 的 handler body/upload、event 吞吐/ACK、窗口关闭 hook、通知激活和 NSIS launcher 进程交接；prototype 结果写入 research，失败则回到本设计修订，不绕到 localhost listener。
