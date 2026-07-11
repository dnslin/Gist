# Wails 3 桌面客户端实施计划

> 状态：approved-for-child-planning；当前父任务只维护规划、追踪与最终集成验收，不创建或启动 child
> 目标平台：Windows 10/11 x64；现有 Web/PWA/server/Docker 契约始终是并列回归面

## 1. 执行模型与术语

- `active repository`：当前已提交、仍对 UI 和请求生效的本地或远程资料库。
- `candidate transition`：模式切换中尚未提交的候选状态；验证失败不得改变 active repository、启动模式、凭据或缓存。
- `generation`：active repository 每次成功提交后递增的代数；旧 generation 的请求、任务和通知结果不得进入当前 UI。
- `optional capability`：只控制可选功能是否可用；API major 定义基础必需契约。缺少 optional capability 不得把兼容服务判为连接失败。
- `OperationCoordinator`：模式切换、退出、备份/恢复和更新安装共享的活动操作查询、确认、取消与排空契约；各能力在其所属 child 接入，不预设未来实现。
- `feasibility gate`：实现业务 child 前必须通过的 Wails/Windows/launcher 原型硬门禁；失败时先修订 research、design/ADR 和本计划，禁止用 localhost listener 绕过。

每个 child 是独立 Trellis 任务：可单独规划、启动、验证、回滚、检查和归档；不得依赖尚未存在的类型、identity、安装布局或 coordinator。下面列出的“依赖”全部归档且证据可读后，下一 child 才可启动。

## 2. 父任务与 Child 状态迁移

1. 父任务保持 `planning`，完成 PRD/design/ADR/implement 评审和追踪矩阵，不承载产品实现。
2. 用户明确批准后，按本文件创建 14 个 child，均使用 `--no-start`；本文件不创建目录、不运行 `task.py start`。
3. 每个 child 先完成自己的 PRD、design、implement，以及真实 spec/research 条目的 `implement.jsonl`、`check.jsonl`；`task.py validate <child>` 只验证 Trellis context 完整性，不算产品证据。
4. child 经独立设计评审且所有依赖已归档后，才可 `task.py start <child>`；执行、check、修复、归档均在该 child 内完成。
5. 一次只启动一个会改变共享产品契约的 child。原型或 VM 取证可并行准备，但不得跨过依赖门禁。
6. 14 个 child 全部归档后，补齐父任务 context，显式启动父任务，只执行追踪矩阵审计和最终集成证据复核；不在父任务补做缺失实现。矩阵存在空项则重开对应 owner child。
7. 父任务通过最终 check 后归档；任何 ADR 被取代时，矩阵必须指向替代 ADR。

### 每个 Child 的统一 Start Gate

- 依赖 child 已归档，且其 evidence/rollback 记录可读取。
- child PRD/设计/实施清单已把父级 Requirement/AC ID 收窄到本切片。
- `implement.jsonl` 和 `check.jsonl` 含真实 spec/research/context 条目并通过 `task.py validate`。
- 验收命令、Windows 环境、fixture、故障注入点和证据保存位置在启动前已确定；不得以 `task.py validate`、`git diff --check`、编译成功代替行为证据。
- rollback 已说明代码、数据、凭据、注册表、快捷方式、安装目录、CI/release 状态如何恢复，且不依赖后续 child。

## 3. Child 01：Windows 基线与统一版本源

**依赖**：无。

**范围**：修复现有 Windows 测试分歧；建立根 `VERSION`，向 Go、Vite、Swagger/compatibility metadata、Docker label 和后续 NSIS 注入，不引入 Wails。

**验收**：Windows 现有后端测试恢复可信；Linux race、frontend、Docker 行为不变；Tag、package metadata 和 `VERSION` 不一致时 CI 失败关闭。

**真实验证类别**：Linux/Windows 单元与构建；router/API snapshot；Docker image metadata；故意制造版本不一致的 CI negative test。

**Rollback point**：无 schema/API 变更；共同消费失败即回退统一版本提交并保留原常量，不允许留下第二版本真相。

**Trellis context/start gate**：收录 Windows 路径失败证据、现行 release/version 注入点和 Web/Docker 基线；通过统一 Start Gate 后启动。

## 4. Child 02：共享 Application Runtime

**依赖**：Child 01。

**范围**：从 `cmd/server` 抽取平台无关 `application.Runtime`，唯一拥有 repository/service/handler/scheduler/backfill/DB、进程级 Snowflake generator 引用、`WriterRegistry` 与关闭顺序；所有修改 local data 的异步工作通过 runtime root context 注册，禁止不可枚举 writer。server 仍唯一拥有 listener、signal、pprof。

**验收**：Snowflake 只初始化一次且失败先于任何写服务；构造失败逆序清理、Close/Quiesce 幂等；WriterRegistry 能拒绝新写、等待/取消已注册 writer 并在 deadline 返回明确结果；server 的路由、状态、body、迁移、scheduler 首次/周期语义和 SIGTERM 不变；Docker build graph 不含 Wails/Windows 包。

**真实验证类别**：Snowflake 未初始化/重复初始化负向测试；构造故障注入；WriterRegistry admission/quiesce/deadline/race；route/API contract；scheduler fake clock；SIGTERM 进程 smoke；Linux race、Windows Go test、Docker build。证据至少包含 `E-C02-SNOWFLAKE` 与 `E-C02-WRITER-REGISTRY`。

**Rollback point**：纯重构；任一可观察差异则整体回退 runtime extraction，禁止在 desktop child 修补 server 回归。

**Trellis context/start gate**：收录当前 composition root、资源所有权与关闭顺序证据；依赖归档后启动。

## 5. Child 03：Linux Build Graph 隔离与 DesktopPaths/Data Lock

**依赖**：Child 02。

**范围**：在引入 Wails 前确定 desktop 独立 Go module或等价 Windows build constraints；平台无关核心仍可在 Linux 测试。实现 `DesktopPaths`，固定 `%LOCALAPPDATA%\Gist\data|desktop.json|logs|updates|webview`，忽略 server 的 `GIST_DATA_DIR`/`GIST_DB_PATH`。进程只先派生固定数据锁与 activation IPC 名称，然后立即获取当前用户级、跨会话、异常退出可恢复的数据所有权锁；锁失败进程不得访问配置、journal、Runtime 或 SQLite，只能通过最小 activation IPC 在同一会话请求已有窗口唤醒，跨会话返回明确占用状态。首实例持锁并建立 IPC 后，才允许 recovery、配置、Snowflake、Runtime 与 Wails 初始化。建立通用 `RecoveryJournal` 原语：持久化 phase/transaction ID、fsync、启动恢复完成后才允许 DB open；本 child 只用最小 fixture 证明顺序，后续 settings/backup/update 各自定义 payload。

**验收**：Ubuntu `go build/test ./...` 不解析 Wails WebView/CGO/GTK/WebKit 依赖；desktop 无论 cwd/server env 如何都只打开规范化自有 data；并发会话/第二进程在 Runtime 创建前失败；异常终止后可恢复，不共享 server DB；每个 journal phase kill 后重启先完成或回滚事务再打开 DB。

**真实验证类别**：Linux dependency/build graph inspection；Windows temp-profile/path/env matrix；两进程/跨会话 lock integration；kill/crash recovery；DB open spy 证明 lock/journal-recovery-before-open；journal phase fault injection 与 fsync trace。

**Rollback point**：删除 desktop graph/paths/lock/journal primitive，不迁移或删除任何用户数据；测试 journal 可清理，发现真实未完成 journal 时必须先恢复而非直接删除；server 配置语义保持原样。

**Trellis context/start gate**：启动前记录 module/build-constraint 决策、Windows lock API 语义、路径威胁模型、journal durable-write/恢复状态机和 DB-open ordering。

## 6. Child 04：锁定 Wails 工具链与最小 Shell

**依赖**：Child 03。

**范围**：精确锁定经批准的 Wails CLI/Go module/runtime tuple；增加 desktop Vite mode（禁用 PWA/SW）、embed SPA、最小 AssetHandler JSON round-trip、固定 WebView2 目录与日志 adapter；不接真实业务 runtime/模式/凭据。

**验收**：空白 profile 可打开内置首屏和 tracer endpoint；关闭退出；无监听 socket；Linux/Web/PWA/Docker graph 不受影响；lock 在任何 Runtime/Wails app 初始化前取得。

**真实验证类别**：Windows desktop build/launch smoke；DOM/JSON round-trip；进程 socket inspection；desktop bundle 无 SW/manifest；Linux/Web regression。

**Rollback point**：删除 Wails module/build产物和 desktop mode；不触碰业务数据/schema/identity。

**Trellis context/start gate**：记录精确 tuple、上游 API 证据、生成目录/lockfile 策略；不得以浮动版本启动。

## 7. Child 05：Wails/Windows 可行性硬门禁

**依赖**：Child 04。

**范围**：只做可丢弃的最小 prototype 与量化证据：AssetHandler request body 和 multipart upload；Wails event 乱序压力、严格 sequence、连续 ACK、有界窗口、背压/失联；窗口关闭/退出 hook；Windows 通知激活并恢复既有窗口；最小签名/测试 identity 下稳定 launcher → versioned payload → NSIS handoff 与 child-process ownership。原型不进入产品业务实现。

**验收**：全部项目有可重复 runner、阈值和原始证据；handler 不丢 body/upload；事件无 text/item 丢失或重排且内存有界；hook/activation/handoff 覆盖冷/热启动与失败；任何失败均阻断 Child 06–14。

**真实验证类别**：专用 Windows harness；大文件/取消 upload；事件 flood/ACK stall/reload；窗口消息与通知 activation E2E；launcher/NSIS process trace、故障注入和退出码。

**Rollback point**：删除 prototype 产物/测试 identity/临时安装状态；保留 research 结论。若结论推翻设计，回到 design/ADR 评审，不保留产品 shim。

**Trellis context/start gate**：必须先定义量化阈值、Win10/11 取证环境、research 输出路径和失败决策人；归档条件是 feasibility review 明确“pass”。

## 8. Child 06：Compatibility、RemoteClient 与候选连接

**依赖**：Child 05。

**范围**：公开 `/api/compatibility`；API major 作为基础必需契约，optional capability 只做功能降级。实现无状态 `RemoteClient`、结构化 URL、HTTP 风险确认、系统 TLS、redirect、超时、脱敏、独立系统网络 client。实现持久设置中的 saved remote configuration 与 candidate remote configuration：候选完整验证后原子替换；二者只是远程连接配置，不表示 UI 的 active repository/candidate transition。本 child 不实现 UI 模式切换或 mode-aware handler。

**验收**：仅当 `api.major` 受支持且该 major 的基础必需 endpoint/field/status/error contract probe 或 suite 全部通过时才可进入；缺 optional capability 只禁用对应功能；缺握手、非法响应、基础契约缺失或 major 不兼容均阻断且凭据未发送；地址替换失败保留 saved remote configuration；RemoteClient 不读取任一资料库 proxy；修改/移除规则不误删 local data。

**真实验证类别**：公开 endpoint contract/Swagger；多版本 server fixtures；URL/path-prefix/TLS/certificate/HTTP/redirect matrix；credential-leak assertions；proxy env与repository proxy isolation；原子文件故障注入。

**Rollback point**：移除 endpoint/client/schema migration 并恢复旧 desktop settings；不删除现有 remote token，未提交 candidate 可安全丢弃。

**Trellis context/start gate**：收录 supported-major policy、capability 分类表、TLS/redirect threat model和 settings migration fixture。

## 9. Child 07：Active/Candidate Mode Transaction、认证与普通 Transport

**依赖**：Child 06。

**范围**：实现 active repository + candidate transition 双状态事务、generation、ModeManager、Credential Manager local/remote identity、DesktopAuth 和 mode-aware 普通 `/api`/`/icons` handler。请求捕获 generation；切换提交前 active 始终有效。事务 commit 接受显式、一次性的 precondition permit，本 child 只验证状态/凭据/transport 原子性；不引用尚未交付的 OperationCoordinator。desktop settings migration 使用 Child 03 RecoveryJournal，并在 DB/Runtime 打开前恢复。

**验收**：候选验证/认证/凭据保存/缓存准备任一步失败不改变 active、启动模式或来源；成功提交原子递增 generation；token 不进入 WebView持久存储；只明确 401 清当前凭据；local/remote response contract 与 browser 普通 API 等价；流式路径稳定返回 `desktop_stream_required`；settings migration 每 phase crash 均恢复一致状态。

**真实验证类别**：事务步骤与 journal phase 故障注入；Credential Manager integration + fake；local/remote same-contract suite；late-generation response drop；WebView storage inspection；network/401/certificate/compatibility error matrix。

**Rollback point**：按 RecoveryJournal 恢复迁移前 desktop.json，再回退 schema/manager/transport；仅删除本 child 创建且可证明 ownership 的 credential targets，不接触用户既有凭据或 local DB。

**Trellis context/start gate**：明确 active/candidate invariants、one-shot precondition permit、credential target namespace、settings journal payload/恢复 fixture；OperationCoordinator 集成明确留给 Child 09。

## 10. Child 08：前端 Runtime Ports、资料库隔离与代理设置

**依赖**：Child 07。

**范围**：browser/desktop `AuthPort` 与 runtime bootstrap；首次选择、常驻来源、错误页、显式切换。提交时 cancel/clear Query，再重置所有 repository-scoped Zustand/IndexedDB/UI state。代理设置界面按 active repository 读/测/写；remote 保存只在服务确认后更新，失败保留已生效值；切换后重读。Shell、RemoteClient、updater 使用独立系统网络 client，不继承 repository proxy。

**验收**：相同 entry/feed ID 的两库切换不串 DOM/cache/image/task/UI；candidate 未提交前仍显示 active 来源；远程错误不自动回 local；local scheduler 在 remote UI 仍运行；代理不复制、不同步或产生伪成功；Web/PWA auth/localStorage/proxy UI 保持原行为。

**真实验证类别**：React integration/E2E 双 fixture；Query/Zustand/IndexedDB inspection；代理 backend spy 与失败注入；shell/remote/update outbound client isolation；restart/auto-login/error-state E2E；accessibility text-source assertion。

**Rollback point**：切回 browser ports/旧 UI，清理 desktop build 的 repository-scoped caches；不更改已提交 desktop settings、凭据或后端 proxy 设置。

**Trellis context/start gate**：枚举全部 repository-scoped state owner、代理 API 读测写契约、browser baseline和 generation 传播路径。

## 11. Child 09：流式任务与 OperationCoordinator

**依赖**：Child 08。

**范围**：生成类型化 `DesktopTaskEvent`；实现任务 owner、generation、sequence、唯一终态、连续 ACK、有界窗口、背压、取消和活动操作查询。local 复用 service，remote 解析现有 text/SSE/NDJSON；frontend `StreamPort` 保持业务接口。OperationCoordinator 在本 child 正式拥有活动任务契约并接入模式切换；不预先接入未来退出/update。

**验收**：四类流在 local/remote 均增量；text/item 不丢不重排，progress 可合并；ACK stall/reload 不导致无界内存；旧 generation 事件不可见；取消竞态和远程不可撤回状态如实表达；Web SSE/NDJSON 不变。

**真实验证类别**：首事件时延与 throughput benchmark；乱序/重复/缺口/ACK stall/flood/reload property/integration tests；UTF-8 分块、cache、error、cancel race；remote OPML DELETE；双库 late event E2E。

**Rollback point**：移除 desktop task bindings/listeners，取消并排空本 child task；普通 API与 Web 流协议继续可用，不留下事件订阅或临时状态。

**Trellis context/start gate**：引用 Child 05 实测阈值，固定 event schema/ACK policy/内存上限/不可撤回分类和 OperationCoordinator contract。

## 12. Child 10：稳定 Launcher、最小 NSIS 与安装 Identity

**依赖**：Child 09。

**范围**：前置最小稳定 `Gist.exe` launcher、`versions/<version>`、current-user固定 NSIS布局、AppUserModelID、Start Menu shortcut、卸载 identity和测试签名路径；只完成 clean install/launch/uninstall-preserve，不实现在线更新事务。`current.json` 明确由 launcher/update transaction owner 管理，应用不得随意写 active/pending。

**验收**：非管理员固定目录安装；launcher 唯一入口且唤起 payload；identity 在覆盖安装后稳定；卸载默认保留 data/settings/credentials；禁止 machine-wide状态；Win10/11 均有可重置 VM 证据。

**真实验证类别**：clean-profile Win10/11 VM installer E2E；registry/shortcut/AppUserModelID inspection；non-admin/policy-denied negative tests；repair/overwrite identity；process handoff trace；default uninstall data diff。

**Rollback point**：卸载测试产品并只删除本 child 创建的程序目录、HKCU uninstall/shortcut/identity；保留所有用户数据/凭据；恢复 release workflow 到未发布状态。

**Trellis context/start gate**：锁定 installer ownership 表、identity 常量、测试证书、VM reset snapshot、launcher/current.json schema。

## 13. Child 11：生命周期、单实例、托盘、关闭与自启动

**依赖**：Child 10。

**范围**：数据锁继续是 Runtime 前安全边界；Wails SingleInstance 仅在已有进程内 Restore/Focus。实现 close coordinator、托盘、记住选择、background、自启动。退出路径接入 OperationCoordinator；自启动使用 Child 10 稳定 launcher identity。

**验收**：第二启动不创建第二 Runtime/DB/scheduler；托盘隐藏不断任务/本地刷新；所有完全退出入口同一确认语义；远程不可撤回任务说明准确；自启动默认关、`--background` 不弹远程错误且立即启动本地维护；卸载移除 owned autorun。

**真实验证类别**：双进程/跨会话/data-lock E2E；tray/window hook automation；任务中 hide/quit；close preference reset；Windows login VM automation；registry ownership；shutdown/logoff bounded close。

**Rollback point**：删除 owned HKCU autorun/tray/close preference，恢复标准窗口关闭；不移除 launcher identity、数据锁或用户数据。

**Trellis context/start gate**：记录窗口 hook prototype、OperationCoordinator 退出协议、自启动 registry ownership和系统关机非交互策略。

## 14. Child 12：通知、导航、NativeFiles 与诊断

**依赖**：Child 11。

**范围**：用稳定 identity 实现更新/前台任务通知、激活恢复与 pending result ownership；实现只允许内置资源的 navigation policy、系统浏览器外链、`NativeFiles` facade、脱敏有界日志和离线诊断。只接 OPML 与诊断；为 backup/restore 暴露已验证的原生打开/保存能力。

**验收**：隐藏窗口才通知用户前台任务，visible/后台维护不通知；通知不可用时结果保留并在下次打开显示；点击恢复已有窗口/正确 generation，不能恢复页面时显示明确结果；危险/程序化导航阻止；远程文件流程只传内容不传路径；取消对话框零副作用；日志 14 天/50 MiB 且不含敏感内容。

**真实验证类别**：Win10/11 toast activation E2E（冷/热/禁用/失败/重复）；notification queue persistence；navigation adversarial tests；原生 dialog automation；remote upload body/path spy；日志敏感语料和 retention/size tests；诊断包 content audit。

**Rollback point**：注销/移除本 child notification state与 handlers，取消 pending activation；保留 Start Menu identity；删除仅测试产生的日志/诊断包，不删除用户导出文件。

**Trellis context/start gate**：固定 notification pending-state owner/schema、activation URI allowlist、NativeFiles trust boundary和日志脱敏语料。

## 15. Child 13：本地完整备份与恢复

**依赖**：Child 12。

**范围**：锁定成熟 age 实现；版本化 envelope、限额、secure join、一致性 snapshot、密码加密、staging 验证、覆盖确认、rollback snapshot、Runtime 重开。备份 native save 必须先写同目录临时文件、fsync、原子替换；失败/取消不得截断用户已有目标文件。恢复使用 Child 03 RecoveryJournal 记录停写、snapshot、swap、reopen/commit phase，启动必须先恢复未完成事务。仅 local data和允许的本地设置，排除 remote、token、logs、updates、webview。

**验收**：跨用户/设备还原完整本地状态；错误密码/篡改/截断/未知版本/路径穿越/压缩炸弹/空间不足在修改 active data 前失败；每个 journal phase kill 或故障均自动恢复；quiesce 后无 runtime-owned writer；remote connection/token/repository不变；用户已有备份目标在失败时原样保留。

**真实验证类别**：格式 golden/compatibility；安全属性与恶意 archive；真实 NativeFiles temp/atomic replace；DB consistency；逐 journal phase kill/fault/power-loss simulation；OperationCoordinator task+writer fixture；跨 Windows profile/VM restore E2E。

**Rollback point**：按 durable journal 恢复 rollback snapshot和原 desktop settings，重开旧 Runtime；仅在 commit/恢复完成后清理 journal、staging/temp；保留失败证据及用户原备份文件；不触碰 remote state。

**Trellis context/start gate**：收录库审查、文件 allowlist/limits、native atomic-save contract、OperationCoordinator quiesce协议、journal payload和恢复故障矩阵。

## 16. Child 14：签名更新事务、统一 Release 与 Win10/11 E2E

**依赖**：Child 13。

**范围**：signed manifest、Ed25519 trust set、SHA-256、WinVerifyTrust、stable GitHub Release检查、24h/manual策略、下载与二次授权。update coordinator接入OperationCoordinator。launcher 独占 `current.json` active/pending/transaction；NSIS只安装 pending payload；Child 03 RecoveryJournal 在任何破坏步骤前持久化并 fsync app/data/config恢复事务。增加单调 release sequence、manifest digest绑定与 anti-replay 状态；data-format fence 阻止旧 payload 打开新格式数据。health commit/rollback同时恢复 app、local data与desktop config。建立统一Tag的draft→验证→publish流水线和 Win10/11 实机/自托管 VM E2E。

**验收**：前台任务可下载验证但阻止 installer；第二次确认才退出安装；暂缓保留经验证 pending state且不取消任务；remote API不兼容不阻止更新；旧合法 manifest replay、同 sequence 异 digest、payload downgrade与手工旧 exe均失败关闭；每个 journal/launcher phase故障自动回滚，失败时保留恢复材料并停止循环；正式/测试 key与channel隔离；任一资产、测试、签名失败不发布 stable Release/Docker tag。

**真实验证类别**：checked-in desktop E2E runner 在可重置的非管理员 Windows 10 与 Windows 11 x64 VM/self-hosted matrix 执行 clean install、local/remote、upgrade、migration failure、crash、integrity failure、timeout、rollback failure、repair、uninstall keep/delete；逐 journal/launcher phase kill；replay/same-sequence-different-digest/downgrade/data-format-fence tests；test cert/key自动化与正式 release signing ceremony；manifest/tamper/wrong-key/prerelease negative tests；Linux race/frontend/Web/PWA/Docker/API-major contract矩阵。证据必须包含 runner版本、OS build、WebView2、安装器/资产哈希、视频或结构化日志和 JUnit/trace；`task.py validate`、`git diff --check`、hosted Windows Server job均不能替代 Win10/11 产品证据。

**Rollback point**：保持 release 为 draft、不推 stable Docker tag；按 durable journal 恢复 previous active/data/config snapshot；仅在恢复/commit完成后删除未提交 pending payload/下载和 journal；回滚失败保留材料并停止启动；密钥和已发布资产不得由回滚脚本删除。

**Trellis context/start gate**：启动前必须已有可调用的 Win10/11 VM runner、测试签名基础设施、key/channel/anti-replay policy、data-format compatibility表、update journal状态机、release failure-close演练和完整 E2E 命令；缺任一项不得启动。

## 17. AC → Design → Child → Evidence 追踪矩阵

PRD 的 Requirement/Acceptance Criterion 已使用稳定 ID。`E-Cxx-*` 是计划证据槽位，owner child 归档前必须替换为实际 CI run、JUnit、trace、截图/视频或人工记录 URI；空槽位视为失败。每个 child PRD 还必须逐项列出本行关联的 `REQ-*`，不得只复制 AC 文本。

| PRD stable ID | 关联 Requirements | Design ID | Owner child | 验证命令/场景类别 | 必需 evidence |
|---|---|---|---:|---|---|
| `AC-PLAN-01` | `REQ-SCOPE-*`, `REQ-COMPAT-*` | `DES-01` | 父任务；01–02 提供基线 | 仓库证据审查；Windows/Linux/Web/Docker baseline | `E-PLAN-CURRENT-SYSTEM`, `E-C01-WIN-BASELINE`, `E-C02-SERVER-CONTRACT` |
| `AC-PLAN-02` | `REQ-COMPAT-04`, `REQ-GATE-01`～`05` | `DES-10` | 父任务；05 提供原型 | 一手资料审查；五项 prototype runner | `E-PLAN-WAILS-RESEARCH`, `E-C05-FEASIBILITY` |
| `AC-PLAN-03` | 全部稳定 ID | `DES-01`～`10` | 父任务 | trace completeness script + 人工孤项审查 | `E-PLAN-TRACE-AUDIT` |
| `AC-PLAN-04` | `REQ-DATA-06`～`07`, `REQ-MODE-01`～`03`, `REQ-REMOTE-07`, `REQ-TASK-03` | `DES-02`, `03`, `05`, `09` | 父任务；03/06/07/09 提供设计证据 | ownership/invariant review | `E-PLAN-OWNERSHIP-REVIEW` |
| `AC-PLAN-05` | `AC-PROD-01`～`21` | `DES-01`～`10` | 父任务 | dependency DAG + rollback/evidence audit | `E-PLAN-CHILD-DEPENDENCY-AUDIT` |
| `AC-PLAN-06` | `REQ-NONGOAL-05` | `DES-01`～`10` | 父任务 | 用户评审与 ADR 状态审查 | 用户评审记录与 accepted/replaced ADR URI |
| `AC-PROD-01` | `REQ-COMPAT-01`～`04`, `REQ-DATA-02`, `REQ-NONGOAL-03` | `DES-01`, `02`, `04` | 01–14（14 汇总） | 每 child Linux/Windows/Web/PWA/Docker回归；desktop 固定数据目录与 socket inspection | 每个 `E-Cxx-WEB-SERVER-DOCKER-REGRESSION`，`E-C03-DESKTOP-PATHS`，`E-C14-COMPAT-SUMMARY` |
| `AC-PROD-02` | `REQ-DATA-01`, `REQ-DATA-05`, `REQ-MODE-01`～`06`, `REQ-NONGOAL-02` | `DES-02`, `03`, `04` | 08；09/14 汇总 | 双库同 ID E2E；cache/state/image/write inspection；remote active 时 local scheduler 继续且 remote scheduler 不重复 | `E-C08-DUAL-REPO`, `E-C08-LOCAL-SCHEDULER`, `E-C09-GENERATION`, `E-C14-LOCAL-REMOTE-E2E` |
| `AC-PROD-03` | `REQ-MODE-01`～`05` | `DES-03` | 07–08 | candidate 每阶段 fault injection；restart/late-result E2E | `E-C07-CANDIDATE-ATOMICITY`, `E-C08-MODE-UX`, `E-C08-LATE-RESULT` |
| `AC-PROD-04` | `REQ-DATA-03`～`04` | `DES-03`, `04` | 08 | proxy read/test/write backend spy；shell/remote/update client isolation | `E-C08-PROXY-ROUTING`, `E-C08-NETWORK-CLIENT-ISOLATION` |
| `AC-PROD-05` | `REQ-DATA-06`～`07`, `REQ-SHELL-01` | `DES-02`, `06`, `07` | 03，11 | 跨 session 双进程；同 session activation IPC；跨 session 占用提示；kill recovery；DB-open spy | `E-C03-LOCK-BEFORE-DB`, `E-C03-ACTIVATION-IPC`, `E-C03-CRASH-RECOVERY`, `E-C11-SECOND-LAUNCH` |
| `AC-PROD-06` | `REQ-AUTH-01`～`05` | `DES-04` | 07–08 | Credential Manager + WebView storage；401/network/TLS matrix | `E-C07-CREDENTIALS`, `E-C07-AUTH-ERRORS`, `E-C08-WEB-AUTH-REGRESSION` |
| `AC-PROD-07` | `REQ-REMOTE-01`～`05` | `DES-03`, `04` | 06–07 | URL/TLS/redirect/path-prefix matrix；credential leak assertions | `E-C06-URL-TLS-REDIRECT`, `E-C06-CANDIDATE-PERSISTENCE`, `E-C07-NO-CREDENTIAL-LEAK` |
| `AC-PROD-08` | `REQ-REMOTE-06`～`08` | `DES-09` | 06；14 汇总 | supported-major contract suite；missing optional capability UI | `E-C06-MAJOR-MATRIX`, `E-C06-OPTIONAL-DEGRADE`, `E-C14-SUPPORTED-MAJOR-RELEASE` |
| `AC-PROD-09` | `REQ-COMPAT-03`, `REQ-TASK-01`～`02` | `DES-04` | 09 | event flood/乱序/重复/gap/ACK stall/reload/UTF-8/cancel | `E-C09-FIRST-EVENT`, `E-C09-EVENT-ORDER`, `E-C09-ACK-BACKPRESSURE`, `E-C09-RELOAD` |
| `AC-PROD-10` | `REQ-TASK-03`～`05` | `DES-05` | 09，11，13，14 | 同一 task/writer fixture 的 switch/exit/restore/update gate matrix | `E-C09-OPERATION-CONTRACT`, `E-C11-EXIT-GATE`, `E-C13-RESTORE-GATE`, `E-C14-UPDATE-GATE` |
| `AC-PROD-11` | `REQ-SHELL-02`～`03` | `DES-05`, `07` | 11 | tray/window/autostart/login/logoff VM automation | `E-C11-TRAY-CLOSE`, `E-C11-AUTOSTART`, `E-C11-LOGOFF` |
| `AC-PROD-12` | `REQ-SHELL-04`～`05` | `DES-07` | 12；14 补更新 | Win10/11 toast visible/hidden/disabled/cold/hot/duplicate activation | `E-C12-TOAST-ACTIVATION`, `E-C12-PENDING-RESULT`, `E-C14-UPDATE-NOTIFICATION` |
| `AC-PROD-13` | `REQ-SHELL-06`, `REQ-NONGOAL-04` | `DES-01`, `07` | 12 | navigation adversarial suite；external-open failure | `E-C12-NAVIGATION`, `E-C12-EXTERNAL-OPEN-FAILURE` |
| `AC-PROD-14` | `REQ-FILE-01` | `DES-04`, `07` | 12–13 | native dialog cancel；remote path spy；atomic save failure | `E-C12-NATIVE-PATH`, `E-C12-DIALOG-CANCEL`, `E-C13-NATIVE-SAVE` |
| `AC-PROD-15` | `REQ-FILE-02`～`05` | `DES-05`, `06` | 13 | cross-profile restore；malicious archive；每 phase kill/fault recovery journal | `E-C13-CROSS-DEVICE`, `E-C13-MALICIOUS-INPUT`, `E-C13-ROLLBACK`, `E-C13-NATIVE-SAVE` |
| `AC-PROD-16` | `REQ-PRIVACY-01`～`03` | `DES-01`, `07` | 12 | telemetry inventory；secret corpus；retention/diagnostic audit | `E-C12-NO-TELEMETRY`, `E-C12-LOG-REDACTION`, `E-C12-RETENTION`, `E-C12-DIAGNOSTIC` |
| `AC-PROD-17` | `REQ-RELEASE-01`～`02`, `REQ-NONGOAL-01` | `DES-01`, `08` | 10；14 汇总 | 非管理员 Win10/11 x64 clean/repair/upgrade/uninstall VM matrix；unsupported target/portable artifact absence | `E-C10-WIN10-INSTALL`, `E-C10-WIN11-INSTALL`, `E-C10-IDENTITY`, `E-C14-INSTALL-MATRIX`, `E-C14-SUPPORTED-TARGETS` |
| `AC-PROD-18` | `REQ-RELEASE-03`～`04`, `REQ-RELEASE-08`, `REQ-TASK-05` | `DES-05`, `08` | 14 | fake clock；two-consent；task gate；remote-incompatible E2E | `E-C14-CHECK-CADENCE`, `E-C14-TWO-CONSENTS`, `E-C14-REMOTE-INCOMPATIBLE` |
| `AC-PROD-19` | `REQ-RELEASE-05` | `DES-06`, `08` | 14 | install/payload/crash/migration/integrity/timeout/rollback fault matrix | `E-C14-WIN10-ROLLBACK`, `E-C14-WIN11-ROLLBACK`, `E-C14-ROLLBACK-FAILURE` |
| `AC-PROD-20` | `REQ-RELEASE-06`～`07`, `REQ-SCOPE-04` | `DES-08` | 14 | Authenticode/app-sign wrong/missing/tamper；anti-replay/channel/release-close | `E-C14-SIGNING`, `E-C14-ANTI-REPLAY`, `E-C14-CHANNEL-ISOLATION`, `E-C14-RELEASE-CLOSE` |
| `AC-PROD-21` | `REQ-GATE-01`～`05` | `DES-10` | 05 | 五项独立 prototype runner 与 go/no-go review | `E-C05-HANDLER`, `E-C05-EVENT-ACK`, `E-C05-CLOSE-HOOK`, `E-C05-NOTIFICATION`, `E-C05-LAUNCHER-HANDOFF` |

> 同步规则：表中不得使用 AC 通配符；`REQ-*` 仅用于把同一 AC 的需求范围带入 child。跨 child 的 AC 由最右侧最后一个 owner 汇总，但每个分段 owner 仍须独立产出自己的 evidence。

## 18. 跨 Child 归档门禁

每个 child 归档前必须同时满足：

- 范围内全部 PRD ID 均映射到 design 决策、owner child、真实验证和 evidence URI；无“后续补证”。
- 验收覆盖行为、边界、不变量、失败和回滚；`task.py validate` 只证明 context，不证明产品。
- rollback 在该 child 的测试环境实际演练，回滚后上一 child 的产品证据仍成立。
- active repository、candidate transition、generation、凭据、proxy、pending notification/update和文件 ownership没有跨边界漂移。
- Web/PWA继续原 HTTP/SSE/NDJSON/file/PWA行为；server/Docker目录、listener和Linux build graph不变。
- 新 API 同步 Swagger/DTO/frontend types/contract；新 UI 字符串同步中英文且完成无障碍检查。
- Wails、加密、签名、NSIS依赖精确锁定；日志/错误/证据均经过敏感信息审查。

## 19. 父任务完成条件

- [ ] 14 个 child 均按依赖顺序独立规划、启动、验证、回滚演练并归档。
- [ ] PRD 每个稳定 Requirement/AC ID 在矩阵中有唯一 owner（跨阶段回归可有多个）和非占位 evidence URI。
- [ ] ADR-0002/0003 已 accepted，或矩阵指向明确取代它们的 accepted ADR。
- [ ] Child 05 feasibility gate 的完整 Wails body/upload、event ordering/ACK/backpressure、close hook、notification activation和launcher handoff均通过。
- [ ] 可重置非管理员 Windows 10/11 x64 VM 完成安装、本地/远程、更新、回滚、卸载与 native E2E；Windows Server hosted runner不冒充产品支持证据。
- [ ] 同一稳定 Tag 的签名 NSIS/manifest、server/Docker和Web/PWA资产完成失败关闭发布演练，且可追溯到同一提交/版本。
- [ ] 父任务显式 start 后只复核矩阵和集成证据；空项重开 owner child，全部通过后 check并归档父任务。
