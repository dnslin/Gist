# Design

## Boundaries

本任务仅配置工程技能的仓库级消费约定，不改变产品代码、领域模型或远程问题跟踪器状态。

## File Layout

- `CLAUDE.md`: 追加或原位更新唯一的 `## Agent skills` 区块。
- `docs/agents/issue-tracker.md`: 问题跟踪器操作约定。
- `docs/agents/triage-labels.md`: 标准角色到实际标签字符串的映射。
- `docs/agents/domain.md`: 领域文档发现和词汇消费规则。

## Decisions

- 配置入口选择 `CLAUDE.md`，因为它已存在且技能规则优先于 `AGENTS.md`。
- 领域文档使用 single-context：根目录 `CONTEXT.md` 加 `docs/adr/`。
- GitHub 模板保留 `PRs as a request surface: no`。
- 仅在用户确认后固化问题跟踪器与标签选择。

## Compatibility

保留 `CLAUDE.md` 的 Trellis 管理块和 `.rules` 内容。若未来重新运行技能，应更新现有 `## Agent skills` 区块而不是追加重复区块。

## Rollback

删除新增的 `docs/agents/` 文件，并从 `CLAUDE.md` 移除本任务新增的 `## Agent skills` 区块即可完全回滚。
