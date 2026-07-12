# Fix PR #2 Code Review Findings

## Goal

修复 PR #2 双轴 review 已确认的 Windows desktop ownership、activation IPC、RecoveryJournal、DesktopPaths、bootstrap 和 acceptance-evidence 缺陷，使实现重新满足 Child 03 的原始 PRD/design 与 backend desktop-host code-spec；补齐能在真实 Windows seam 捕获这些缺陷的确定性回归测试，并更新 PR #2。

## Background

- Standards review 报告 8 个硬性违反和 2 个判断性 smell；Spec review 报告 4 个实现问题和 3 个验收/证据缺口。
- planning diagnosis 已确认：mutex thread affinity、Acquire `error + Lease` 泄漏、abandoned diagnostic 缟失、overlapped cancel 未 drain、transport error classification、per-connection trailing-frame race、真实 timeout fixture 缺失、UNC share root、tombstone startup、directory-sync failure suppression、recovery path leakage、真实 bootstrap contender zero-data fixture 和 evidence status 均需处理。
- `byteReader` 与 DACL duplication 是低风险维护项；successful cleanup 是否必须删除 fixed tombstone 存在文档冲突，本任务以“tombstone 是待完成 cleanup evidence，启动必须识别并完成/失败关闭”为合同，不要求无条件删除唯一 marker。

## Requirements

### REQ-RF-01 — Red-capable feedback loops

- 每个产品缺陷必须先有 focused regression test，在修复前真实变红、修复后变绿。
- 反馈环必须覆盖真实 seam：Windows mutex OS thread、真实 named-pipe stalled I/O、真实两进程 bootstrap contender、Windows UNC path、RecoveryJournal fault/tombstone，而非仅断言浅层 helper。
- 测试必须确定、秒级、无需人工操作；真实第二 Terminal Services session、native Linux/Docker、power-loss 仍作为外部 evidence，不伪造。

### REQ-RF-02 — Thread-affine Windows mutex ownership

- Global named mutex 的 create/wait、metadata publish、release/handle close 必须由同一个锁定 OS thread owner 执行；调用 `Lease.Close` 的 goroutine/thread 不影响正确释放。
- `Close` 幂等、可等待并返回 owner-thread release 结果；process termination 仍由 Windows 自动释放。
- zero-time available race、`WAIT_ABANDONED`、active same/other-session outcome 与 DACL 合同保持不变。

### REQ-RF-03 — Bootstrap ownership cleanup and diagnostics

- `Acquire` 同时返回 `Lease + error` 时，bootstrap 必须精确关闭 lease 一次并 join acquisition/close errors，且进入零 data stage。
- malformed/non-acquired outcome 若携带 lease 也必须防御性关闭；有效 acquired outcome 才可将 lease 放入 lock-last cleanup。
- `OutcomeAcquiredAbandoned` 必须在 logs 打开后、recovery 前通过窄 diagnostic recorder 记录稳定 `previous_owner_abandoned`；记录失败必须失败关闭。
- 补齐 acquired-without-lease、generic stage partial closer、nil generator/runtime、partial runtime 和 retryable Host.Close 回归矩阵。

### REQ-RF-04 — Race-free activation request boundary

- server 在调用 `ActivationSink` 前必须确定该请求连接不会再提供第二 frame；不得以瞬时 `PeekNamedPipe` 作为 whole-connection 完成证明。
- 协议改为 request/response 分离：request pipe 接收单个 bounded request envelope（含一次性 correlation nonce 与受限 response endpoint identity），client 完整关闭 request channel 后，server 才可执行 activation，再通过独立 response pipe 返回结果。
- response identity 必须从既有 hashed user/session/root identity 与随机 nonce 派生；不得接受任意路径、命令、URL 或 credential。
- 协议仍为 version 1、UTF-8 strict JSON、1 KiB 上限、unknown/trailing/oversize fail-closed。

### REQ-RF-05 — Safe overlapped I/O and stable errors

- connect/read/write/cancel/close 全部使用 overlapped I/O；任何 `CancelIoEx` 后必须等待该 OVERLAPPED 获得终态，再释放 event、buffer 或 handle。
- stalled connect/read/write 必须在 bounded deadline 内返回；server timeout 后仍能接受下一次正常 activation。
- transport/protocol timeout、malformed、oversize、trailing 和 unexpected I/O 对外稳定分类为 `activation_protocol_invalid`；startup routing 才可将 endpoint unavailable 映射为 `data_owner_unreachable`，底层 Win32 cause 不进入业务分支。

### REQ-RF-06 — DesktopPaths root validation

- 拒绝 Windows drive root、UNC share root（有/无尾随分隔符）、empty/relative/root-only known-folder output。
- 允许合法的 redirected UNC LocalAppData 子目录；不得粗暴禁止所有 UNC path。
- 保持 path resolution 无文件系统副作用且忽略 cwd/`GIST_*`。

### REQ-RF-07 — Recovery durability and fail-closed startup

- originating Child 03 PRD 的 strict durability 优先：directory open/flush/sync error 不得转为成功；`Journal.Save/Replay` 必须返回 `recovery_failed` 并保留 canonical/tombstone evidence。
- canonical 缺失但 fixed tombstone 存在时，startup 必须读取、严格验证并完成 pending cleanup；fault 时继续保留 marker、阻止 config/generator/Runtime，成功重试必须幂等。
- cleanup atomic replace、directory sync 和 tombstone retry 均须独立 fault seam；不得把 unexecuted boundary 记录为 pass。
- Journal 对外错误只暴露稳定 category + safe phase/validated operation ID；完整 LocalAppData path、raw `PathError` 文本、metadata 和 handler secret 不得出现在 error string，同时可通过 safe wrapper 保留 `errors.Is` cause。
- 删除单次使用的 `byteReader`，改用 `bytes.NewReader`；不得改变记录 bytes 或 allocation boundary。

### REQ-RF-08 — Real process and security evidence

- 新增 Windows bootstrap 两进程 same-session fixture：owner 使用真实 WindowsAcquirer + activation server；contender 使用真实 bootstrap/activation client，并证明 logs/recovery/config/credentials/generator/Runtime factory/DB path 均为零。
- 新增真实 stalled client/server named-pipe timeout、different-thread lease close、delayed second request、pipe/recovery DACL 行为测试。
- DACL helper 仅在行为测试保护后抽取到 desktop-wide Windows security owner；ownership 与 recovery 保持各自资源操作，不产生反向依赖。

### REQ-RF-09 — Honest acceptance and PR update

- 修正归档 Child 03 的 `check.json`/evidence：focused command 可标 passed，但整体状态不得在 required external evidence 未完成时标 complete/passed。
- 明确 pending：第二 Terminal Services session、native Ubuntu race/lint、Docker smoke、Windows race toolchain、fixed-commit rollback drill、真实 power-loss durability。
- 当前工作站可执行的 focused tests、Windows build、Linux cross-build、dependency guard 必须通过；更新 PR #2 body/check evidence，不得关闭或新建替代 PR。

## Constraints

- 不引入 Wails、frontend desktop mode、schema/migration、server env/listener、Docker graph 或第二 Runtime composition root。
- 不通过削弱原 PRD/code-spec 来消除 finding；若 Windows strict directory sync 在受支持 LocalAppData filesystem 无法实现，必须保留红灯并记录 blocking trade-off，而不是吞错。
- 所有 debug instrumentation 使用唯一 `[DEBUG-*]` 前缀并在完成前删除。
- Windows API 资源生命周期优先于抽象简洁性；不允许 pending OVERLAPPED 引用已释放内存。

## Out of Scope

- Wails shell、mode/auth、backup/update 业务 journal、NSIS、frontend。
- 提供第二 Terminal Services session、Ubuntu/Docker runner 或真实断电环境。
- 修改 Child 03 产品范围或宣称上述外部 evidence 已通过。

## Acceptance Criteria

- [ ] 所有新增 regression tests 已证明修复前 red、修复后 green，并在 task evidence 记录命令与症状。
- [ ] mutex 不受 goroutine OS-thread migration 影响；different-thread close、abandoned、kill/reacquire、same/other-session outcome 通过。
- [ ] activation request completion 无 `PeekNamedPipe` TOCTOU；delayed second request 不触发 sink；所有 cancel path drain pending I/O；真实 timeout 后服务可恢复。
- [ ] UNC bare/trailing share root 均拒绝，UNC child path 保留。
- [ ] Acquire partial lease、nil/partial/retry bootstrap matrix 与真实 two-process zero-data fixture 通过。
- [ ] tombstone startup/fault/cleanup、strict directory-sync failure、path redaction、all phase process recovery 通过；recovery failure 阻止 Runtime。
- [ ] pipe/mutex/registry/recovery artifacts 的 protected current-user + SYSTEM DACL 行为通过；共享 helper 不改变资源 ownership。
- [ ] Windows focused suites/build、Linux cross-build、dependency guards 通过；external evidence 保持 pending，不标 pass。
- [ ] Trellis check 无 blocker，规范与 PR #2 描述同步，修复提交 push 到原分支。
