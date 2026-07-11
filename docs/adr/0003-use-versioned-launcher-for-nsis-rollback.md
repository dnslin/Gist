---
status: proposed
---

# 使用版本化启动器完成 NSIS 升级回滚

新版本可能在 exe 启动、桌面配置迁移或 SQLite 迁移阶段失败，此时新应用本身无法可靠回滚。Gist 因此仍只分发签名 NSIS，但在固定程序目录中使用小型稳定 launcher 和版本化 app payload：新版本先作为 pending 启动，只有首次健康检查提交后才成为 active；失败则由进程外 coordinator 恢复上一 payload 与升级前本地资料库快照。

## Considered Options

- 依赖新版本自行回滚：无法覆盖新 exe 根本不能启动或启动即崩溃。
- 只保留恢复点并给出人工说明：不满足已确认的自动回滚要求。
- 直接使用 Wails updater 替换 exe：不符合 NSIS 唯一正式交付物和安装元数据要求。
- NSIS + 稳定 launcher + 版本化 payload（采用）。

## Consequences

- 安装目录结构和 release 流程更复杂，launcher、NSIS、app 与恢复事务必须共同测试和签名。
- 上一正式 payload 与本地快照只保留到新版本首次健康检查成功；之后不提供任意降级。
- launcher 必须保持极小、无业务依赖，并拥有独立的故障注入和回滚测试。

