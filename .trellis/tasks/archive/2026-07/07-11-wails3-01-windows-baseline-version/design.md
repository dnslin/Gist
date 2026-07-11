# Child 01 技术设计：Windows 基线与统一产品版本源

> 状态：planning；本设计可独立评审，不授权实现

## 1. 设计边界与追踪

| Design ID | 决策 | 需求/验收 |
|---|---|---|
| `DES-C01-01` | 先取证、分类、锁定 Windows 基线，再修改 | `REQ-C01-BASELINE-01~02`, `AC-C01-01~02` |
| `DES-C01-02` | 根 `VERSION` 是唯一人工维护产品版本源 | `REQ-C01-VERSION-01`, `AC-C01-03~04` |
| `DES-C01-03` | 单次解析、显式注入、消费者校验 | `REQ-C01-VERSION-02~04`, `AC-C01-03~04` |
| `DES-C01-04` | compatibility/NSIS 只定义消费边界，不创建未来组件 | `REQ-C01-VERSION-03`, `REQ-C01-RELEASE-BOUNDARY-*` |
| `DES-C01-05` | Web/server/Docker 行为与回滚保持可观察等价 | `REQ-C01-COMPAT-01`, `AC-C01-05~06` |

## 2. Windows 基线设计

### 2.1 先记录后修复

实现开始后的第一个行为步骤必须在 Windows 工作区运行两条精确 focused test，并保存原始输出到 Child evidence 目录：

```powershell
cd backend
go test ./internal/config -run '^TestLoad$' -count=1 -v
go test ./internal/service -run '^TestIconService_IsValidIconPath$' -count=1 -v
```

记录项包括 `go version`、Windows build、`GOOS/GOARCH`、测试名、fixture、期望值、实际值和失败分类。只有基线证据写入后才允许改测试或生产代码。任何 `t.Skip`、build tag、CI allow-failure、测试重命名逃避或删除断言均判定失败。

### 2.2 分类与合同

1. **配置路径**：生产代码使用 `filepath.Clean/Join`，测试却把 `/tmp/gist` 当作跨平台结果。此项初始分类为“不可移植 fixture/断言”；测试输入应由 `t.TempDir()` 或 `filepath.Join` 构造，并断言 `cfg.DataDir` 与 `cfg.DBPath` 的宿主平台规范化值。不得改变 `GIST_DATA_DIR`、`GIST_DB_PATH` 的 server 语义。
2. **图标路径**：`filepath.IsAbs("/abs/icon.png")` 在 Windows 不表达跨平台攻击输入，生产校验若只依赖宿主 OS 语法会接受另一平台的绝对路径或混合分隔符。此项初始分类为“跨平台安全合同缺口”，需要平台无关地拒绝：空路径、`.`、`..`、任何父目录组件、POSIX root、Windows drive absolute、UNC/device root、混合分隔符逃逸；允许单个安全相对文件名。分类必须由 focused negative tests 证实，不能以修改期望为 `true` 解决。

测试采用表驱动、外部测试包与 `testify/require`，遵循 backend quality guidance。测试断言可观察返回值，不断言源码文本。

## 3. 统一版本数据流

```text
repository root VERSION (X.Y.Z)
          |
          +--> version validation/check command
          |       +--> stable Git tag vX.Y.Z comparison
          |       +--> frontend/package.json consistency
          |       +--> generated Swagger metadata consistency
          |
          +--> Go linker variable --> config.AppVersion --> GistUserAgent
          |                                +--> future compatibility.productVersion
          |
          +--> Vite define/env --> typed frontend build constant
          |
          +--> Docker build ARG --> OCI org.opencontainers.image.version label
          |
          +--> future release input --> NSIS/manifest (not created in Child 01)
```

### 3.1 `VERSION` 格式

- 路径固定为仓库根 `VERSION`。
- UTF-8 单行文本；允许文件末尾一个换行；解析后必须完全匹配稳定 SemVer `^[0-9]+\.[0-9]+\.[0-9]+$`。
- 禁止 `v` 前缀、prerelease/build metadata、空白内部字符、多行或从 Git Tag 自动反写文件。
- 当前版本从既有 Go 与 frontend metadata 的 `1.2.0` 初始化；Tag 只校验，不成为回退源。

### 3.2 版本校验所有权

仓库只建立一个版本解析/校验入口，供本地命令和 CI 调用。它必须：

1. 从显式 repository root 读取 `VERSION`，不依赖调用者 cwd；
2. 校验格式并输出裸 `X.Y.Z`；
3. 在提供 Tag 时校验其严格等于 `v` + `VERSION`；
4. 校验 `frontend/package.json` 声明等于 `VERSION`；
5. 对缺文件、空值、非法值、读失败或不一致返回非零退出；
6. 不修改文件，不从 package/tag 猜测替代版本。

具体脚本语言在实现评审时服从现有 CI 环境，优先使用仓库已具备的 Go/Bun，不引入只为版本校验存在的新 runtime dependency。

### 3.3 当前消费者

- **Go**：`config.AppVersion` 从 linker `-X` 可覆盖变量取得；未注入的本地 `go test` 使用由 `VERSION` 同步校验的确定 fallback。`GistUserAgent` 必须从最终 `AppVersion` 派生，禁止保留独立字面版本。
- **Vite**：构建命令在启动 Vite 前读取已验证的 `VERSION`，注入只读的 typed build constant。不得由浏览器在运行时读取文件或 Git。现有 PWA 配置、输出和路由不因版本注入分叉。
- **package metadata**：`frontend/package.json` 是生态工具消费者而非第二来源；本 Child 采用一致性校验，不允许发布流程从它反推根版本。`bun.lock` 仅在 Bun 对 root package metadata 的规范行为要求时同步，不能成为版本来源。
- **Swagger**：`@version` 与生成的 `docs.go/swagger.json/swagger.yaml` 必须显示相同产品版本。生成物仍遵循现有 `swag init` 合同；校验防止只更新注释或只更新生成物。
- **Docker**：Docker build 显式接收 `VERSION` build arg，并写 `org.opencontainers.image.version=$VERSION`。server binary 的 linker value 与 image label 来自同一个已验证变量。entrypoint、environment、ports、用户和 linux/amd64+arm64 graph 不变。

## 4. Future consumer 边界

### 4.1 Compatibility metadata

Child 01 不创建 `/api/compatibility`。它只保证 Go 层有单一、可注入、可测试的产品版本访问点。Child 06 创建 endpoint 时必须读取该值作为 `productVersion`；不得解析 Git、读取 package.json 或增加新常量。API major 与 optional capabilities 不属于本 Child。

### 4.2 NSIS 与统一 Release

Child 01 不创建 NSIS 文件或 Wails config。版本校验入口必须能在后续 release job 中输出裸 `X.Y.Z`；Child 10/14 将同一值传给 NSIS、launcher payload directory 和 signed manifest。当前 JSONL/文档不得引用不存在的 installer 路径作为已实现消费者。

## 5. 失败关闭与负向验证

负向测试在临时工作树/临时复制的 metadata 上运行，不污染开发者工作区：

| 故障 | 预期 |
|---|---|
| `VERSION` 缺失、空、`v1.2.0`、`1.2`、`1.2.0-beta`、多行 | 校验非零退出，无构建/发布输出 |
| Tag `v1.2.1` 对 `VERSION=1.2.0` | release gate 非零退出 |
| `frontend/package.json` 改为不同值 | metadata check 非零退出 |
| Go linker 注入遗漏或不同值 | binary/version contract test 失败 |
| Vite 注入遗漏或不同值 | frontend build/version test 失败 |
| Docker label 缺失或不同值 | image inspection 失败 |
| Swagger source/generated metadata 漂移 | generated-artifact check 失败 |

## 6. 兼容性与回滚

- 不改变 API/schema/data；版本信息变化仅替换已存在的 metadata 值及其注入方式。
- router/API snapshot 确认路由、status、body schema 不变；版本字段只在既有 Swagger metadata 或后续明确消费者出现。
- Docker smoke 确认原 entrypoint、端口、`GIST_DATA_DIR=/app/data` 与非 root 用户保持不变。
- rollback point 是统一版本提交之前：整体回退 `VERSION`、校验入口、注入和消费者改动。不得只回退某个消费者而留下两套真相；无数据、schema、credential、registry 或安装目录需要恢复。

## 7. 设计验收边界

本设计完成只代表以下内容可独立评审：两项 Windows 失败的归因流程与安全合同、根版本源格式、现有消费者数据流、未来消费者边界、负向失败关闭矩阵和整体 rollback point。它不代表产品实现、安装器或统一稳定 Release 已完成。
