# 配置 Matt Pocock 工程技能

## Goal

为本仓库建立 Matt Pocock 工程技能依赖的仓库级配置，使相关技能能够确定问题跟踪器、分诊标签词汇及领域文档读取规则。

## Background

- 仓库远程地址为 `https://github.com/dnslin/Gist.git`；用户已确认使用 GitHub Issues，并通过 `gh` CLI 操作。
- 根目录同时存在 `CLAUDE.md` 与 `AGENTS.md`；按技能规则应编辑 `CLAUDE.md`。
- 已安装 `triage` 技能，因此需要配置五个标准分诊角色的标签映射。
- 根目录已有 `CONTEXT.md` 和 `docs/adr/`，无 `CONTEXT-MAP.md`，且未发现技能定义的 monorepo 信号；领域文档采用 single-context 布局。
- `docs/agents/` 尚不存在，未发现此前运行该设置技能的输出。

## Requirements

1. 使用用户已确认的 GitHub Issues 作为问题跟踪器。
2. 使用用户已确认的默认分诊标签：`needs-triage`、`needs-info`、`ready-for-agent`、`ready-for-human`、`wontfix`。
3. 采用由仓库事实确定的 single-context 领域文档布局。
4. 写入前展示 `CLAUDE.md` 的 `## Agent skills` 区块，以及 `docs/agents/issue-tracker.md`、`docs/agents/domain.md`、`docs/agents/triage-labels.md` 的完整草稿。
5. 用户批准草稿后再写入；保留 `CLAUDE.md` 中 Trellis 管理区块及其他现有内容。
6. 使用设置技能目录中的模板作为文档种子；GitHub 配置保持“PRs as a request surface: no”。

## Acceptance Criteria

- [ ] `CLAUDE.md` 恰好包含一个 `## Agent skills` 区块，并链接三个配置文档。
- [ ] `docs/agents/issue-tracker.md` 准确描述用户确认的问题跟踪器工作流。
- [ ] `docs/agents/triage-labels.md` 映射五个标准分诊角色。
- [ ] `docs/agents/domain.md` 记录 single-context 消费规则并匹配现有 `CONTEXT.md` 与 `docs/adr/`。
- [ ] 写入内容与用户批准的草稿一致，现有周边内容未被覆盖。
- [ ] 最终检查确认文件存在、链接路径正确、无重复 Agent skills 区块。

## Out of Scope

- 创建或修改远程 GitHub 标签。
- 创建 GitHub Issues。
- 修改现有 `CONTEXT.md` 或 ADR 内容。
- 启用 Pull Requests 作为分诊请求入口。
