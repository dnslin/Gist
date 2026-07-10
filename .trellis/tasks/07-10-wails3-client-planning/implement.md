# Wails 3 桌面客户端实施计划

> 状态：proposed，等待 `prd.md`、`design.md` 与 ADR 整体评审
> 当前任务角色：父级规划与集成验收，不作为单次巨型实现任务启动

## 1. 执行原则

- 未经用户评审确认，不运行当前任务或任何 child task 的 `task.py start`。
- 评审后把本计划的十个阶段创建为当前任务的 child tasks；一次只启动一个具备独立验收面的 child。
- 每个 child 必须先写自己的精简 PRD/设计/实施清单，显式继承父任务相关要求。
- 每个 child 同时保护现有 Web/PWA/Docker；不能以“后续 child 会修复”为由合入回归。
- 新增桌面能力先完成测试或 tracer-bullet，再扩展到全功能。
- 所有 Wails、runtime、加密、签名和打包依赖精确锁定，禁止 `latest`、通配版本和未提交 lockfile 变化。
- 不开放 localhost/LAN listener，不在业务 service 中加入 Wails 判断。

## 2. 评审与启动门禁

- [ ] 用户接受 `prd.md` 的完整需求边界。
- [ ] 用户接受 `design.md` 的混合 transport、模式状态机、版本化 launcher 和数据边界。
- [ ] `docs/adr/0002-use-hybrid-transport-for-desktop-streaming.md` 从 `proposed` 改为 `accepted`。
- [ ] `docs/adr/0003-use-versioned-launcher-for-nsis-rollback.md` 从 `proposed` 改为 `accepted`。
- [ ] 使用 `task.py create ... --parent 07-10-wails3-client-planning` 创建 child tasks，并写入下述依赖顺序。
- [ ] 只对第一个 child 执行 `task.py start`；父任务保持规划/集成角色。

## 3. Child 1：Windows 基线与统一版本源

### 目标

先让未引入 Wails 的现有代码在 Windows 上具有可信基线，并建立所有产物共用的产品版本源。

### 工作项

- [ ] 为 `internal/config.TestLoad` 使用平台无关路径断言，保留真实路径清理语义。
- [ ] 修复 `IconService_IsValidIconPath` 的 Windows 路径断言/实现分歧并增加回归测试。
- [ ] 新增根 `VERSION` 与只读 version package；替换 `config.AppVersion` 的硬编码来源。
- [ ] Vite、Go、Docker label、Swagger/compatibility metadata 和后续 NSIS 均从同一版本源注入。
- [ ] CI 校验稳定 Tag `vX.Y.Z` 与 `VERSION`、前端 package metadata 一致。
- [ ] 后端 CI 增加 Windows 非 race test；保留 Linux race test。

### 验证

```powershell
Set-Location backend
go build ./...
go test ./...
go vet ./...

Set-Location ../frontend
bun install --frozen-lockfile
bun run build
bun run test
bun run lint
```

### 回滚点

本阶段不改 schema/API。若统一版本源无法被 Docker/Vite 共同消费，先保留现有常量并停止，不在多个位置新增第二套版本真相。

## 4. Child 2：共享 Application Runtime

### 目标

把 `cmd/server` 的组合根抽为可测试 runtime，同时证明 server/Docker 行为零变化。

### 工作项

- [ ] 先为 runtime 构造失败清理、Start/Close 幂等和关闭顺序写测试。
- [ ] 建立 `internal/application.RuntimeOptions`、`Runtime` 和构造函数。
- [ ] 将 repository/service/handler/scheduler/backfill 装配迁入 runtime。
- [ ] `cmd/server` 只保留 config/log/snowflake、signal、pprof 和 listener。
- [ ] router/static/API 路由、scheduler 15 分钟语义和立即首次刷新保持不变。
- [ ] 验证 Docker build graph 未引入 Wails 或 Windows 包。

### 验证

- `go test ./... -race`（Linux CI）。
- `go test ./...`（Windows CI）。
- router route snapshot、migration、scheduler start/stop、SIGTERM smoke test。
- `docker build -f docker/Dockerfile .`。

### 回滚点

runtime extraction 必须是纯重构。任何 API/status/body/scheduler 差异都先回退该 child，不与桌面 scaffold 混合修复。

## 5. Child 3：锁定 Wails 工具链与最小 Desktop Shell

### 目标

完成不带业务扩展的 Windows tracer bullet：Wails 加载桌面前端、普通本地 JSON API 可用、没有监听端口。

### 工作项

- [ ] 将 Wails CLI/Go module 精确锁定到 `v3.0.0-alpha2.117`。
- [ ] 将 `@wailsio/runtime` 精确锁定到 `3.0.0-alpha.97`，提交 Bun lockfile。
- [ ] 新增 Wails Taskfile/build 配置和 `cmd/desktop`。
- [ ] Vite 增加 `web`/`desktop` build mode；desktop 禁用 PWA/Service Worker并输出到 Go embed 目录。
- [ ] 创建最小 `DesktopAssetHandler`：内置 SPA + 非流式 local `/api` tracer endpoint。
- [ ] 固定 WebView2 user data 到 Gist 自有目录。
- [ ] 配置 Wails 日志 adapter，所有应用日志仍通过 `pkg/logger`。
- [ ] 添加进程端口检查，证明 desktop 不调用 `ListenAndServe`。

### 验证

```powershell
wails3 version
wails3 doctor
bun --cwd frontend run build --mode desktop
wails3 build
```

- 校验 Go module、CLI、npm runtime 与记录 tuple 完全一致。
- 启动 app，检查首屏非空、普通 JSON round trip、窗口关闭和进程退出。
- 使用 `Get-NetTCPConnection -OwningProcess <pid>` 验证无监听 socket。
- Web build/PWA 测试仍通过。

### 原型门禁

在扩展功能前，把 handler body/upload、Wails event、取消、窗口 hook、通知激活结果写入父任务 `research/desktop-tracer-bullet.md`。任何不符合 `.117/.97` 的结论先修订设计。

## 6. Child 4：兼容性握手、RemoteClient 与普通请求

### 目标

让桌面前端通过同一 `/api` 边界访问 local 或一个已验证 remote，暂不处理流式端点。

### 工作项

- [ ] 新增公开 `GET /api/compatibility`、DTO、Swagger 与契约测试。
- [ ] 建立 API major/capability 常量和所有稳定产物的 product version 注入。
- [ ] 实现远程服务地址 parser：HTTP(S)、端口、path prefix、无 userinfo/query/fragment。
- [ ] 实现系统 TLS、HTTP 风险确认、重定向门禁、超时与脱敏错误。
- [ ] 实现 local Echo dispatch 与 remote response proxy；过滤 hop-by-hop/Set-Cookie。
- [ ] `/api`、`/icons` 按 ModeSnapshot 路由；流式路径明确返回 `desktop_stream_required`。
- [ ] 远程候选连接通过验证后才原子保存；替换失败保留旧连接。
- [ ] 远程 API major 不兼容只阻止 remote 进入，不影响本地模式或桌面更新。

### 验证

- `httptest.Server` 覆盖 root/path-prefix、HTTP、有效 TLS、自签名、过期/主机名错误、同源/跨源/循环 redirect。
- 对 local handler 与 remote proxy 运行相同普通 API contract suite。
- 缺少 handshake、非法 JSON、major 不支持、可选 capability 缺失分别映射稳定错误。
- Web server 路由和 Swagger regenerated diff 经过审查。

## 7. Child 5：ModeManager、Credential Manager 与前端隔离

### 目标

完成模式选择/切换、独立认证、持久启动模式和不会串库的前端状态边界。

### 工作项

- [ ] `desktop.json` schema、原子写入、迁移和损坏恢复测试。
- [ ] Windows Credential Manager adapter + in-memory fake；local/remote target name 分离。
- [ ] DesktopAuth local/remote 实现，返回前端的结果不含 token。
- [ ] 凭据写入失败只保留内存会话，不落明文/localStorage。
- [ ] 实现 ModeManager 状态机、generation 与候选切换事务。
- [ ] 前端建立 browser/desktop `AuthPort` 与 runtime bootstrap。
- [ ] 增加首次模式选择、常驻来源标识、远程错误/兼容错误页面和显式切换入口。
- [ ] 切换时 cancel/clear TanStack Query，重置 translation/lightbox/selection/scroll/IndexedDB 中的资料库相关状态。
- [ ] remote 网络失败不清 token；明确 401 才清当前模式 token。
- [ ] local scheduler 在 remote UI 下继续运行的集成测试。

### 验证

- 两套 fixture 使用相同 entry/feed ID，切换后 DOM、Query cache、Zustand 和图片 URL 只来自目标资料库。
- 重启自动进入上次模式并验证有效 token；失效 token 只回到对应登录页。
- 修改/移除 remote 清理正确 Credential Manager 项且不影响 local。
- Web/PWA auth store/localStorage 行为的既有测试不变。

## 8. Child 6：流式任务事件适配

### 目标

让四类流式能力在 local/remote desktop 都保持增量、取消和错误语义，同时 Web 协议不变。

### 工作项

- [ ] 定义生成 Go/TypeScript 判别联合 `DesktopTaskEvent`，禁止 `any`。
- [ ] 实现 TaskRegistry、generation、sequence、唯一终态、取消和活动任务查询。
- [ ] 实现有界 ACK window、文本小批合并、进度合并和前端失联超时。
- [ ] local summary/translate/batch/OPML adapters 直接调用现有 service/task manager。
- [ ] remote adapters 解析原文本/SSE/NDJSON，并保持现有 error/cache 语义。
- [ ] frontend `StreamPort` 维持现有 async generator/callback API，业务 hooks 不直接订阅 Wails。
- [ ] AbortSignal -> CancelTask；OPML remote 取消调用 DELETE。
- [ ] 模式切换/退出/update coordinator 使用同一活动任务门禁。
- [ ] 隐藏窗口下的终态写入通知队列；通知 UI 留给 Child 7。

### 验证

- 首事件时延上限、连续多块、UTF-8 分块、缓存命中、服务错误、取消竞态和终态唯一。
- sequence 无缺口/重排；progress 可合并但 text/item 不丢失。
- 超过 ACK window 不产生无界内存；前端 reload 后任务按策略取消/恢复 OPML status。
- Web SSE/NDJSON tests 与 desktop event contract tests 同时通过。

## 9. Child 7：Windows 生命周期与原生集成

### 目标

完成单实例、托盘、关闭、自启动、通知、外链、原生文件流程和本地诊断。

### 工作项

- [ ] Wails SingleInstance second-launch Restore/Focus，初始化期间重复启动不创建第二 runtime。
- [ ] close coordinator、记住选择、任务二次确认、tray show/quit。
- [ ] 当前用户自启动开关默认关闭，`--background` 隐藏启动且不弹 remote 错误。
- [ ] Windows 通知：更新和用户前台任务；可见窗口不重复，后台刷新不通知。
- [ ] 通知点击恢复已有窗口和相关任务结果。
- [ ] WebView navigation policy 与系统浏览器 external open。
- [ ] OPML/backup/restore/diagnostic 原生对话框 facade；本 child 只接 OPML/diagnostic，backup 由 Child 8 接入。
- [ ] 本地日志脱敏、14 天/50 MiB 轮转、离线诊断包。
- [ ] NSIS/应用创建的 Start Menu identity 满足 Windows toast 激活。

### 验证

- 第二次启动、托盘隐藏/恢复、close preference reset、任务中退出、Windows 登录 background。
- 通知禁用/点击/重复合并，payload 无敏感内容。
- HTTP(S) 外链系统打开，危险/程序化导航被阻止。
- 文件对话框取消无副作用，remote import 不发送本机路径。

## 10. Child 8：本地完整备份与恢复

### 目标

实现用户主动、密码保护、跨设备可恢复且失败自动回滚的 local-only 备份。

### 工作项

- [ ] 锁定并审查成熟 `age` 实现；定义 magic/header/manifest/format version。
- [ ] 定义允许文件集合、大小/数量上限和 secure-join 解包器。
- [ ] 创建备份前停止 local 写入并生成一致性 snapshot。
- [ ] 将 local data、local account/AI/proxy/cache/icons 与非远程桌面偏好流式加密到临时文件后原子保存。
- [ ] 明确排除 remote URL/token、logs、updates、diagnostic、WebView cache。
- [ ] 恢复先完整解密/认证/哈希/版本验证到 staging，再请求覆盖确认。
- [ ] 创建恢复前 rollback snapshot，原子交换并重开 runtime；失败恢复旧数据。
- [ ] 密码不保存、不找回、不写日志。

### 验证

- 跨 Windows 用户/机器恢复；错误密码、篡改、截断、未知版本、路径穿越、压缩炸弹、空间不足。
- 数据库/图标/AI cache/settings 与本地账号完整还原。
- 失败注入覆盖每个停机/交换/重开步骤，原资料库和 remote 状态不变。

## 11. Child 9：NSIS、签名更新与首次启动回滚

### 目标

完成当前用户固定目录 NSIS、双重签名验证、两阶段更新授权和无法启动也能自动回滚的外部事务。

### 工作项

- [ ] 构建最小稳定 `Gist.exe` launcher 和版本化 `versions/<version>` payload。
- [ ] NSIS 固定 `%LOCALAPPDATA%\Programs\Gist`、current-user、无 UAC、无目录选择。
- [ ] clean install、pending install、active pointer 和卸载元数据。
- [ ] 定义 signed manifest、Ed25519 trust set、SHA-256、WinVerifyTrust Authenticode。
- [ ] GitHub stable-only 24 小时检查、手动检查、托盘状态和下载断点/清理。
- [ ] 第一次确认下载，第二次确认退出安装；前台任务阻止 installer launch。
- [ ] 创建 pre-update local data + desktop config recovery snapshot。
- [ ] launcher/coordinator health timeout、transaction ID、commit 与自动 restore。
- [ ] 已保存 remote API 不兼容不阻止安装，升级后 remote handshake 提示。
- [ ] 卸载默认保留数据；显式复选框清理 Gist 目录、Credential Manager 与自启动。
- [ ] 测试/正式 key/channel 隔离，记录生产 key provisioning 和 rotation runbook。

### 验证

- test certificate/key 下完成完整自动化；正式 Release 前再以真实 Authenticode/manifest key 验证。
- 故障注入：安装中断、exe 缺失、启动崩溃、config migration 失败、DB migration 失败、integrity failure、health timeout、rollback failure。
- 回滚成功恢复旧 app/旧 local data；回滚失败保留恢复材料且不循环启动。
- 正常 health commit 后旧 payload/恢复点清理且无 downgrade UI。

## 12. Child 10：统一 Release 与端到端回归

### 目标

让同一稳定 Tag 只在所有 Web/Docker/desktop 资产通过后发布，并建立正式支持矩阵。

### 工作项

- [ ] release workflow 增加 Windows x64 desktop test/build/package/sign jobs。
- [ ] Docker 先 push digest，不在最终门禁前创建 stable tags。
- [ ] GitHub Release 先 draft/暂存资产；所有签名与一致性校验通过后才 publish。
- [ ] 同一版本注入 server handshake、desktop UI、NSIS metadata、manifest、Docker labels。
- [ ] API supported-major matrix；引入新 major 时连续两个稳定版本覆盖前一 major。
- [ ] Windows 10/11 clean/install/upgrade/rollback/uninstall E2E。
- [ ] 本地/远程功能矩阵、四类流式、scheduler、backup、native integration 全量回归。
- [ ] Wails upgrade runbook 与 dependency tuple check。
- [ ] 发布失败关闭：任何必需资产、测试或签名失败时不形成部分稳定 Release。

### 验证

```powershell
python ./.trellis/scripts/task.py validate <child-task>
git diff --check
```

加上 CI 中的 Linux backend race/lint、frontend test/typecheck/build、Docker multi-arch、Windows desktop build/E2E/NSIS/signature。

## 13. 跨阶段检查清单

每个 child 合并前都检查：

- [ ] 本地与远程资料库、token、Query cache 和任务 generation 未串用。
- [ ] Web/PWA 仍使用原 HTTP/SSE/NDJSON/文件/PWA 行为。
- [ ] Docker build graph 不包含 Wails/Windows 运行依赖。
- [ ] desktop 进程无 listener，remote 目标只来自已确认 URL。
- [ ] 所有日志和错误经过敏感信息审查。
- [ ] 新 API 更新 Swagger、Go DTO、前端类型与契约测试。
- [ ] 新字符串同时更新中英文翻译。
- [ ] Wails/加密/签名/NSIS 依赖精确锁定。
- [ ] `go build ./...`、Go tests、frontend build/tests/lint/typecheck 和 `git diff --check` 通过。
- [ ] 对应 child 的 rollback point 可执行，不依赖未完成 child。

## 14. 父任务完成条件

- [ ] 十个 child tasks 均已独立验收并归档。
- [ ] `prd.md` 全部 acceptance criteria 有自动化或记录完备的人工证据。
- [ ] 两份 proposed ADR 在评审后 accepted，或由新的 ADR 明确取代。
- [ ] Windows 10/11 x64 正式签名 NSIS 与同 Tag Docker/Web 资产完成发布演练。
- [ ] 现有 Web/PWA/Docker 无回归，本地/远程模式及资料库隔离通过端到端测试。
- [ ] 执行父任务最终集成 review，再归档父任务。
