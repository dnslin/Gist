---
status: accepted
---

# 使用版本化启动器完成 NSIS 升级回滚

新版本可能在 exe 启动、桌面配置迁移或 SQLite 迁移阶段失败，此时新应用本身无法可靠回滚。Gist 因此仍只分发签名 NSIS，但在固定程序目录中使用小型稳定 launcher 和版本化 app payload：新版本先作为 pending 启动，只有首次健康检查提交后才成为 active；失败则由进程外 coordinator 恢复上一 payload 与升级前本地资料库、桌面配置快照。

launcher 交接必须先通过独立 feasibility gate：原型需证明 NSIS 能安装版本化 payload、稳定 launcher 能以 transaction ID 启动 pending、接收超时内健康提交，并在 exe 缺失、启动崩溃、迁移失败、完整性失败和健康超时时恢复 previous active 与升级前数据。原型失败时先修订设计和本 ADR，不得先实现完整更新功能。

## Considered Options

- 依赖新版本自行回滚：无法覆盖新 exe 根本不能启动或启动即崩溃。
- 只保留恢复点并给出人工说明：不满足已确认的自动回滚要求。
- 直接使用 Wails updater 替换 exe：不符合 NSIS 唯一正式交付物和安装元数据要求。
- NSIS + 稳定 launcher + 版本化 payload（采用）。

## Consequences

- 安装目录结构和 release 流程更复杂，launcher、NSIS、app 与恢复事务必须共同测试和签名。
- launcher 只能从已验证的 update transaction 启动 pending，并校验 signed manifest、payload 完整性、版本与 channel。active 已完成健康提交后，较旧 payload、旧 manifest、直接路径启动或伪造 `current.json` 均不得把 active 指针回退；不提供 downgrade UI。
- 数据必须带独立的 data-format version fence。每个 payload 声明可读取/迁移的格式范围；launcher 在启动前阻止 active/pending/previous 中任何不兼容 payload 打开当前数据。回滚只能在未提交 transaction 内同时恢复 previous payload 和与其匹配的升级前快照，不能让旧程序打开已提交的新格式数据。
- 上一正式 payload 与升级前快照只保留到新版本首次健康检查成功；提交后清理 transaction 恢复材料，之后不提供任意降级。回滚失败时保留材料并停止循环启动。
- launcher 必须保持极小、无业务依赖，并拥有独立的交接原型、故障注入、防降级与 data-format fence 测试。

