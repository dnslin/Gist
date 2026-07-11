---
status: accepted
---

# 将 Wails 3 用于正式桌面版

尽管 Wails 3 上游仍标记为 Alpha，Gist 仍将基于它的 Windows 桌面版作为正式产品开发、签名、发布和支持，而不标记为 Preview 或等待上游稳定版。项目接受上游预发布状态带来的变化风险，并通过精确锁定并验证 Wails CLI、Go module 与前端 runtime 的兼容版本组合，以及每次升级前执行完整桌面回归、NSIS 安装和更新验证来控制风险；这些组件使用各自的上游版本线，不要求版本号字面一致。

## Considered Options

- 将桌面版作为 Preview 发布。
- 只开发但不公开发布，等待 Wails 3 稳定版。
- 按正式产品交付，并以版本锁定和发布门禁承担风险（采用）。

## Consequences

- Wails 升级不得自动跟随 `latest`，必须作为显式、可回滚的依赖升级处理。
- 框架升级未通过 Windows 10/11 x64、本地/远程模式、流式能力和 NSIS 升级回归时，不得进入正式 GitHub Release。
