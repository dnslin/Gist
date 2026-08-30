# Spec: desktop-shell

> 状态：Approved（2026-08-29）
> 修订：2026-08-30，与 `reading` 已确认的单一原文窗口对齐
> Module ID：`desktop-shell`
> 来源：[`CAPABILITY-MAP.md`](./CAPABILITY-MAP.md)

## Objective

为 Gist 桌面客户端建立最小可运行的 Wails v3 外壳。

完成本 Spec 后，项目应当具备：

- 一个可在 Windows、macOS 和 Linux 上原生构建与运行的桌面工程。
- 一个使用系统原生边框的主窗口；应用启动时只创建该主窗口。
- 一条独立于 Web/PWA 构建的桌面前端构建链路。
- 开发模式下的前端热更新和 Go 代码重编译。
- 生产构建时嵌入可执行文件的前端静态资源。
- 供后续 `reader-workspace` 和 `connection` 使用的运行入口。

本模块只证明桌面外壳与前端入口能够工作，不实现现有阅读 UI、Gist 服务连接或任何业务功能。用于验证资源加载的最小页面是开发阶段的技术入口，不是新增产品页面。

## Decisions

- Wails 工程位于 `desktop/`，与现有 `backend/`、`frontend/` 并列。
- React 业务源码仍只有 `frontend/src/` 一份，不复制到 `desktop/`。
- 现有 Web 构建继续输出 `frontend/dist/`。
- 桌面前端单独输出 `desktop/frontend/dist/`，供 Wails 嵌入。
- 应用名称为 `Gist`，Go module 为 `github.com/dnslin/Gist/desktop`。
- 主窗口使用系统标题栏和系统窗口按钮，不实现自定义标题栏。
- 主窗口初始尺寸为 `1440 × 900`，可调整大小并在首次打开时居中。
- `desktop-shell` 本身只创建主窗口。后续 `reading` 可以在用户打开外部链接时，按需创建最多一个可复用的原文窗口。
- 关闭原文窗口不影响主窗口；关闭主窗口后应用进程退出，并结束仍打开的原文窗口。

选择 `1440 × 900` 是为了让首次打开宽度超过现有 UI 的 `1366px` 桌面断点。窗口仍可缩小，现有响应式布局负责小尺寸显示。

## Tech Stack

| 项目 | 约束 |
| --- | --- |
| Desktop framework | Wails `v3.0.0-beta.12` |
| Go | Go `1.25.5`；不得低于 Wails 要求的 Go 1.25 |
| Frontend | 复用现有 React `19.2.6`、TypeScript `6.0.3`、Vite `8.0.14` |
| Package manager | Bun；继续使用 `frontend/bun.lock` |
| Build orchestration | Wails v3 生成的 Taskfile |
| Tests | Go `testing`、现有 Vitest 与 Testing Library |
| Desktop assets | Wails `embed.FS`，生产运行时不依赖前端开发服务器 |

`desktop/go.mod` 必须精确依赖：

```go
require github.com/wailsapp/wails/v3 v3.0.0-beta.12
```

如果 Wails 脚手架向 `frontend/package.json` 加入 `@wailsio/runtime`，必须精确锁定为 `3.0.0-beta.12`，不得保留模板中的 `latest`。本模块没有使用该运行时的实际需求时，不为了保持模板原样而增加它。

不得为本模块增加新的状态管理、路由、HTTP 客户端、UI 组件库或通用桥接层。

## First-stage Platform Validation

第一阶段在以下三类原生环境中验证：

| 平台 | 验证边界 |
| --- | --- |
| Windows | 实施阶段可用的 Windows 环境及其当前 CPU 架构 |
| macOS | 实施阶段可用的 macOS 环境及其当前 CPU 架构 |
| Linux | 实施阶段可用的 Linux 环境及其当前 CPU 架构 |

每个环境必须先通过 `wails3 doctor`，再完成原生构建和启动验证。本 Spec 不定义最低操作系统版本，不要求同时验证 amd64 与 arm64，也不要求跨平台交叉编译、移动端或安装包制作。

## Commands

以下命令是本模块完成后必须成立的工程契约。

### 安装并核对固定版本

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.12
wails3 version
wails3 doctor
```

### 安装依赖

```bash
cd frontend
bun install --frozen-lockfile

cd ../desktop
go mod download
```

### 桌面开发

```bash
cd desktop
wails3 dev
```

`wails3 dev` 必须通过桌面专用 Vite 配置启动前端，并支持热更新。

### 前端验证

```bash
cd frontend
bun run lint
bun run test
bun run build
bun run build:desktop
bun run verify:desktop-assets
```

其中：

- `bun run build` 保持现有 Web/PWA 构建行为，输出到 `frontend/dist/`。
- `bun run build:desktop` 执行 TypeScript 检查和桌面 Vite 构建，输出到 `desktop/frontend/dist/`。
- `bun run verify:desktop-assets` 在桌面构建后检查入口文件和禁止出现的 PWA 产物。

### Go 验证

桌面静态资源必须先生成，因为 `go:embed` 在编译阶段需要目标目录存在。

```bash
cd frontend
bun run build:desktop
bun run verify:desktop-assets

cd ../desktop
go fmt ./...
go vet ./...
go test ./...
```

### 原生构建

```bash
cd desktop
wails3 build
```

产物位于 `desktop/bin/`。本模块不执行 `wails3 package`，因为 `.app`、安装器、签名和发布不在当前范围。

## Project Structure

```text
desktop/
├── go.mod                     # 独立的桌面 Go module
├── go.sum
├── main.go                    # Wails 应用启动与主窗口
├── Taskfile.yml               # Wails 开发和构建入口
├── build/
│   ├── config.yml             # Gist 产品与二进制元数据
│   ├── Taskfile.yml           # 公共构建任务
│   ├── darwin/                # macOS 构建资源
│   ├── linux/                 # Linux 构建资源
│   └── windows/               # Windows 构建资源
├── frontend/
│   └── dist/                  # 桌面前端生成物，不提交
└── bin/                       # Wails 构建产物，不提交

frontend/
├── desktop/
│   └── index.html             # 无 PWA 启动逻辑的桌面 HTML 入口
├── src/
│   └── desktop/
│       ├── DesktopShell.tsx   # 最小资源加载探针
│       ├── DesktopShell.test.tsx
│       └── main.tsx           # 桌面 React 入口
├── scripts/
│   └── verify-desktop-assets.ts # 构建后的桌面资产检查
├── vite.config.ts             # 现有 Web/PWA 配置，行为保持不变
├── vite.desktop.config.ts     # 无 PWA 的桌面构建配置
├── tsconfig.node.json         # 同时检查桌面 Vite 配置
├── package.json               # 增加 dev:desktop/build:desktop
└── bun.lock                   # 唯一前端锁文件
```

不得复制现有 React 组件、Hooks 或样式到 `desktop/`。后续 `reader-workspace` 直接从现有 `frontend/src/` 接入阅读 UI。

Wails 默认模板中的示例服务、定时事件、示例 React 应用、移动端任务和 server/docker 任务不属于本模块。实施时从生成结果中删除它们，只保留 Windows、macOS、Linux 桌面构建实际需要的模板内容。

## Frontend Build Contract

桌面专用 Vite 构建必须满足：

- `dev:desktop` 使用 `vite.desktop.config.ts`，固定使用 Wails 开发链路约定的端口并启用 `strictPort`。
- `build:desktop` 先执行 `tsc -b`，再执行桌面 Vite 构建。
- 输出目录固定为 `desktop/frontend/dist/`，每次构建前清空该目录。
- 最终入口文件名为 `index.html`。
- 不启用 `VitePWA`。
- 不复制 `frontend/public/` 中的 PWA 资源。
- 不导入或渲染 `UpdateNotice`。
- 不注册、查询或清理 Service Worker。
- 不启动现有 `App`，因此不会请求相对路径 `/api`。
- 不改变 `bun run build` 的 Web/PWA 行为。

桌面入口暂时只渲染能够识别资源加载成功的最小内容。`reader-workspace` 只建立共享 UI 入口；等 `connection` 配置服务地址并建立有效会话后，桌面宿主才挂载该工作区。

## Wails Taskfile Contract

Wails `v3.0.0-beta.12` 的默认任务假定 `package.json` 位于 Wails 工程自己的 `frontend/`。本项目的 React 工程位于仓库级 `frontend/`，因此不得原样使用默认前端任务。

`desktop/Taskfile.yml` 与 `desktop/build/Taskfile.yml` 必须按以下方式调整：

- 包管理器固定为 Bun。
- 根 Taskfile 只包含 `common`、`windows`、`darwin` 和 `linux` 桌面任务。
- `common:install:frontend:deps:bun` 的工作目录改为 `../frontend`，并使用现有 `bun.lock`。
- `common:build:frontend` 在 `../frontend` 中执行 `bun run build:desktop`。
- `common:dev:frontend` 在 `../frontend` 中执行 `bun run dev:desktop`，并传入与 Wails 相同的 `VITE_PORT` 和 `strictPort`。
- `desktop/build/config.yml` 的开发流程继续调用 `common:dev:frontend`，使 `wails3 dev` 能启动桌面 Vite 入口。
- `common:build:frontend` 的生成物声明指向 `desktop/frontend/dist/`。
- 本模块没有 Go service，不生成前端 bindings，也不保留默认示例服务。
- 不保留 iOS、Android、server build 或 Docker build 的 include、task 和生成目录。

最终用户只需要执行 `wails3 dev` 和 `wails3 build`，不需要手工分别启动 Wails 与 Vite。

## Code Style

### Go

- 使用 `gofmt`。
- 在只实施本模块时，`main.go` 只负责创建应用、配置资产、创建主窗口和运行应用。后续已批准的 capability 可以注册自己的具体 service 与最小生命周期调用点。
- 没有第二个调用点时，不提前建立接口、工厂层或配置抽象。
- 运行失败必须返回非零退出状态并保留原始错误信息。

示例结构：

```go
app := application.New(application.Options{
	Name: "Gist",
	Assets: application.AssetOptions{
		Handler: application.AssetFileServerFS(assets),
	},
	Mac: application.MacOptions{
		ApplicationShouldTerminateAfterLastWindowClosed: true,
	},
})

window := app.Window.NewWithOptions(application.WebviewWindowOptions{
	Name:      "main",
	Title:     "Gist",
	Width:     1440,
	Height:    900,
})
window.Center()

if err := app.Run(); err != nil {
	log.Fatal(err)
}
```

该示例只表示尚未接入 `reading` 时的初始外壳。加入原文窗口后，不能只依赖 `ApplicationShouldTerminateAfterLastWindowClosed`，因为原文窗口仍打开时，主窗口不是“最后一个窗口”。届时必须在主窗口关闭事件中明确退出应用；具体 Wails beta.12 回调写法在实施时以实际 API 为准，不在本 Spec 中凭假设固定。

### TypeScript / React

- 沿用现有 TypeScript strict 配置和 ESLint 规则。
- React 组件使用 PascalCase，函数和变量使用 camelCase。
- 不使用无说明的 `any`。
- 不为最小入口增加状态、路由或设计系统组件。

示例：

```tsx
export function DesktopShell() {
  return <main aria-label="Gist desktop shell">Gist</main>;
}
```

## Testing Strategy

### 自动验证

- `DesktopShell.test.tsx` 验证桌面入口能够渲染，并且挂载时不发起网络请求。
- `bun run verify:desktop-assets` 在 `build:desktop` 之后验证 `desktop/frontend/dist/index.html` 存在。
- 同一资产检查验证产物中不存在 `manifest.webmanifest`、Service Worker 和 Workbox 文件。
- 现有 `bun run test`、`bun run lint` 和 Web `bun run build` 必须继续通过。
- `go vet ./...` 与 `go test ./...` 验证桌面 Go module。
- `wails3 build` 验证当前平台的完整前端构建、嵌入与 Go 编译链路。

不增加人为覆盖率阈值。纯声明式 Wails 配置不为了覆盖率而拆成额外抽象。

### 手工冒烟验证

在至少一个当前开发环境中执行：

1. 运行 `wails3 dev`。
2. 确认启动时只打开一个名为 `Gist` 的原生主窗口。
3. 确认窗口初始尺寸、调整大小和居中行为符合本 Spec。
4. 确认窗口显示最小桌面入口，而不是空白页。
5. 修改入口内容，确认前端热更新生效。
6. 关闭主窗口，确认应用进程退出。

在 Windows、macOS 和 Linux 的当前 CPU 架构上分别执行：

1. 运行 `wails3 doctor`。
2. 运行 `wails3 build` 并启动 `desktop/bin/` 中的产物。
3. 确认启动时只打开一个名为 `Gist` 的原生主窗口，初始尺寸为 `1440 × 900`，可调整大小。
4. 在 Gist 服务未运行时，确认内嵌入口仍能显示。
5. 关闭主窗口，确认应用进程退出。
6. 确认生产进程没有启动 Gist 服务，也没有监听应用自建的 HTTP 端口。

## Boundaries

### Always

- Wails CLI、Go module 以及实际使用的 Wails 前端 runtime 保持在 `v3.0.0-beta.12`。
- 保持现有 Web 构建、Docker 静态资源目录和 Web/PWA 测试不变。
- 始终只有一个主窗口；后续 `reading` 只可按需增加最多一个原生原文窗口。窗口使用系统原生边框并可调整大小，关闭主窗口即退出应用。
- 桌面生产构建只使用嵌入静态资源，不依赖 Vite 或外部前端服务器。
- 生成物与二进制必须被 Git 忽略。
- 只实现本模块验收所需的最小代码。

### Ask First

- 改变 `desktop/` 或现有 `frontend/` 的所有权边界。
- 升级 Wails、Go、React、TypeScript、Vite 或 Bun 依赖。
- 增加新的生产依赖。
- 改变应用名称、module path、主窗口数量、尺寸或窗口边框。
- 增加第二个原文窗口、其他类型窗口、分栏、页签、地址栏或浏览器历史。
- 添加平台专属业务逻辑。
- 修改 `backend/`、Docker 或现有 Web/PWA 行为。
- 新增或修改 GitHub Actions。
- 加入打包、签名、发布或自动更新。

### Never

- 在桌面进程中启动、嵌入或复制 Gist 服务。
- 实现连接地址、登录、会话恢复或首次注册。
- 实现或迁移阅读、订阅、设置及其他业务页面。
- 在桌面构建中包含 PWA、Service Worker 或 Web 更新提示。
- 添加托盘、通知、离线、多连接、自动更新、安装器或签名。
- 添加自定义标题栏、插件层、通用桥接层或其他未被当前验收条件使用的抽象。
- 为没有明确故障或真实系统边界的情况增加额外加密、证书固定、权限隔离、进程锁或防御层。
- 通过删除测试、降低断言或忽略错误让验证通过。

## Success Criteria

- [ ] `desktop/` 是独立 Wails 工程，Go module 为 `github.com/dnslin/Gist/desktop`。
- [ ] `desktop/go.mod` 精确依赖 Wails `v3.0.0-beta.12`。
- [ ] `wails3 version` 输出 `v3.0.0-beta.12`。
- [ ] `wails3 dev` 启动时只创建一个原生、可调整大小的 `Gist` 主窗口。
- [ ] 主窗口初始为 `1440 × 900`，可调整大小，首次打开时居中。
- [ ] 关闭主窗口后进程退出，不驻留后台；后续原文窗口存在时也遵守这一行为。
- [ ] 开发模式支持前端热更新。
- [ ] `bun run build:desktop` 输出 `desktop/frontend/dist/index.html`。
- [ ] `bun run verify:desktop-assets` 在桌面构建后通过。
- [ ] 桌面产物不包含 PWA manifest、Service Worker、Workbox 或 Web 更新提示。
- [ ] 桌面入口不发出 Gist API 请求。
- [ ] `wails3 doctor` 和 `wails3 build` 在 Windows、macOS 和 Linux 的当前原生环境中通过。
- [ ] 三个平台的构建产物均能启动，并在没有 Vite 和 Gist 服务的情况下显示嵌入入口。
- [ ] `bun run lint`、`bun run test`、Web `bun run build`、`go vet ./...` 和 `go test ./...` 全部通过。
- [ ] 现有 Web `frontend/dist/` 和 Web/PWA 行为没有改变，Docker 相关文件没有修改。
- [ ] 没有复制现有 React 业务源码，也没有修改 `backend/`。
- [ ] 没有实现本模块范围外的桌面或业务功能。

## Open Questions

无。任何后续范围变化先更新本 Spec，再进入 Plan。

## References

- [Wails v3.0.0-beta.12 Release](https://github.com/wailsapp/wails/releases/tag/v3.0.0-beta.12)
- [Wails beta.12 React template](https://github.com/wailsapp/wails/tree/v3.0.0-beta.12/v3/internal/templates/react)
- [Wails beta.12 common project template](https://github.com/wailsapp/wails/tree/v3.0.0-beta.12/v3/internal/templates/_common)
- [Wails beta.12 generated build Taskfile](https://github.com/wailsapp/wails/blob/v3.0.0-beta.12/v3/internal/commands/build_assets/Taskfile.tmpl.yml)
- [Wails v3 build documentation](https://v3.wails.io/guides/build/building/)
- [Wails v3 window basics](https://v3.wails.io/features/windows/basics/)
