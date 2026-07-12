# Implementation Plan

1. 汇总仓库探索结果并按顺序询问仍需用户决定的事项。
2. 确认问题跟踪器；推荐 GitHub Issues。
3. 确认是否保留五个默认分诊标签。
4. 生成 `CLAUDE.md` 区块和三个 `docs/agents/*.md` 文件的完整草稿。
5. 请求用户审阅并吸收修改意见。
6. 用户批准后，将区块追加到 `CLAUDE.md`，创建 `docs/agents/` 配置文件。
7. 定向检查四个文件，确认链接、布局、标签和唯一性。

## Validation

- 读取 `CLAUDE.md`，确认只有一个 `## Agent skills` 标题。
- 读取三个配置文件，核对 tracker、标签映射、single-context 规则。
- 检查所有相对路径均指向仓库内实际或约定位置。

## Risk Points

- 不得编辑 `AGENTS.md`；`CLAUDE.md` 已存在并具有优先级。
- 不得覆盖 Trellis 管理块或 `.rules`。
- 在用户批准草稿前不得写入最终配置文件。
