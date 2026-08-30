# Spec: reader-workspace

> 状态：Approved（2026-08-29）
> Module ID：`reader-workspace`
> 来源：[`CAPABILITY-MAP.md`](./CAPABILITY-MAP.md)

## Objective

把现有 Web 客户端的完整已登录工作区机械抽取为共享的 `ReaderWorkspace` 组件，使 Web 客户端继续使用同一份实现，并为后续桌面端接入建立唯一的 UI 入口。

本模块完成后：

- 现有 `AuthenticatedApp` 被重命名并移动为可复用的 `ReaderWorkspace`。
- Web 客户端在认证成功后改为渲染 `ReaderWorkspace`。
- 现有侧栏、文章列表、正文、图片模式、添加订阅入口、资料入口和设置入口全部保留。
- 现有路由、响应式布局、样式、Hooks、stores、Query keys 和交互行为保持不变。
- 若 `desktop-shell` 已经实施，桌面端继续显示技术入口；只有 `connection` 建立服务配置和有效会话后，桌面入口才能挂载 `ReaderWorkspace`。

本模块只建立共享组件边界，不宣称阅读、订阅、内容工具、设置或 OPML 已经完成桌面端接入。那些能力分别由 `reading`、`library`、`content-tools`、`settings-profile` 和 `data-transfer` Spec 验收。

## Decisions

- `ReaderWorkspace` 的真实来源是 `frontend/src/App.tsx` 中现有的 `AuthenticatedApp`，不是整个 `App`。
- 完整抽取当前已登录工作区，不把它拆成空的三栏骨架或可插拔槽位。
- `ReaderWorkspace` 保持零参数组件：

  ```tsx
  export function ReaderWorkspace() {
    // 现有 AuthenticatedApp 内容
  }
  ```

- 组件继续使用现有 Hooks、Zustand stores、TanStack Query 和 wouter，不新增数据适配层或 Provider 抽象。
- 宿主必须已经提供 `Router`、`QueryClientProvider`、`I18nProvider` 和 `TooltipProvider`，加载共享 `index.css`，提供 `.app-shell` 全高容器，并且已经进入认证成功状态。
- Web 的认证、登录、注册、网络错误和重试流程继续由 `AppContent` 负责。
- Web 的 `UpdateNotice`、Service Worker、BFCache 启动逻辑和 PWA 元数据不进入 `ReaderWorkspace`。
- 桌面端在 `connection` 完成以前不得挂载真实工作区，也不提供 fixture、模拟数据或临时相对 `/api` 请求。
- 现有 `<768px` 移动布局、`768–1365px` 平板布局和 `>=1366px` 桌面布局原样保留。Wails 窗口缩小时继续使用这些断点。
- 现有 URL 路径和查询参数继续作为选择状态来源，不改用新的状态容器或桌面专属路由。
- 列宽、侧栏显示和文件夹展开等非敏感 UI 偏好继续使用当前 WebView `localStorage`，不增加原生偏好存储。

## Tech Stack

本模块只使用仓库已有依赖，不增加生产依赖或开发依赖。

| 项目 | 约束 |
| --- | --- |
| UI | React `19.2.6`、现有组件与 `index.css` |
| Routing | wouter `3.10.0`，保留现有 path/query 语义 |
| Server state | TanStack Query `5.100.14`，保留现有 QueryClient、query keys 和失效行为 |
| Local state | 现有 React state 与 Zustand `5.0.13` stores |
| i18n | i18next `26.2.0`、react-i18next `17.0.8` |
| Motion | 现有 Motion、Framer Motion 和 CSS transition |
| Tests | Vitest `4.1.7`、Testing Library、jsdom |

不得为了共享组件引入新的路由、状态管理、依赖注入、Context、API client、设计系统或桌面桥接层。

## Component Boundary

### 进入 `ReaderWorkspace`

从现有 `AuthenticatedApp` 一起移动：

- `defaultContentTypes`。
- `LazyEntryContent`。
- `EntryContentPlaceholder`。
- `EntryContentFallback`。
- 完整移动端、平板和桌面工作区组合。
- 侧栏、列表、正文、图片模式、添加订阅页面及全部现有弹窗入口。
- 当前的路由跳转、选择状态、标题计算和 UI 偏好读取逻辑。

### 保留在 Web 外壳

- `LoadingScreen`。
- `AppContent` 及 `useAuth` 驱动的认证状态选择。
- `LoginPage`、`RegisterPage` 和 `NetworkErrorPage`。
- `UpdateNotice`。
- `main.tsx` 中的 Service Worker、BFCache、boot 日志和 Web 启动逻辑。
- Web 的 Vite PWA 配置和 `index.html` 启动保护。

### 宿主运行前提

`ReaderWorkspace` 自身不检查以下条件。Web 或桌面宿主负责在挂载前满足：

- Router 已创建。
- QueryClient、i18n 和 Tooltip Provider 已挂载。
- 共享 `index.css` 已加载，工作区位于 `.app-shell` 全高容器内。
- API 运行地址已经配置。
- 已经存在有效认证会话。

Web 客户端继续使用现有宿主。桌面客户端的 API 地址与会话由后续 `connection` Spec 定义。

## Commands

以下命令是本模块实施后必须成立的验证契约。

### 安装依赖

```bash
cd frontend
bun install --frozen-lockfile
```

### 定向测试

```bash
cd frontend
bun run test -- \
  src/App.test.tsx \
  src/components/reader-workspace/ReaderWorkspace.test.tsx \
  src/app-shell.test.ts \
  src/components/layout/three-column-layout.test.tsx
```

### 完整 Web 验证

```bash
cd frontend
bun run lint
bun run test
bun run build
```

### 手工 Web 验证

```bash
cd frontend
bun run dev
```

本模块不要求运行桌面工作区测试，因为桌面入口仍不挂载 `ReaderWorkspace`。本模块不得创建或修改桌面入口。

## Project Structure

```text
frontend/src/
├── App.tsx
│   ├── App                         # Web 外壳与 UpdateNotice
│   ├── AppContent                  # Web 认证状态选择
│   └── LoadingScreen               # Web 认证加载状态
├── App.test.tsx                    # Web 认证分支与共享工作区接入
├── components/
│   └── reader-workspace/
│       ├── ReaderWorkspace.tsx     # 原 AuthenticatedApp 完整内容
│       ├── ReaderWorkspace.test.tsx
│       └── index.ts                # 公共导出
├── main.tsx                        # Web/PWA 启动逻辑，保持原位
├── app-shell.test.ts               # 分别检查 Web 外壳与工作区滚动约束
└── index.css                       # Web 与未来桌面共用样式
```

现有 `components/`、`hooks/`、`stores/`、`lib/`、`api/` 和 `types/` 目录保持原位。不得为了目录整齐移动或复制它们。

## Shared UI Contract

机械抽取必须保持以下行为：

- 根路径等待外观设置加载完成后，再重定向到第一个可见内容类型。
- 路由仍保留当前 selection、entry、unread 和 content type 查询语义。
- 移动端列表与正文继续共同挂载，通过现有 CSS 和 route 切换显示。
- 平板继续使用可隐藏侧栏。
- 桌面继续显示现有三栏布局；图片模式和添加订阅继续使用当前两栏变体。
- 列宽调整、侧栏显示、文件夹展开、滚动位置和图片预览状态继续使用当前实现。
- `useRefreshStatus`、feeds、folders、entries、appearance settings 等 Query Hook 的调用顺序和条件保持不变。
- 现有 lazy loading、Suspense fallback、Lightbox、ImagePreview、Sheet 和 portal 层级保持不变。
- Web 的动态 `document.title` 行为继续保留；本模块不通过 Wails API 修改原生窗口标题。
- `index.css` 保持单一共享样式源，不创建桌面样式副本。

现有工作区使用 `/logo.svg`。未来桌面挂载时只需把该本地品牌资源纳入桌面产物；不得因此复制全部 PWA public 资源或建立通用资产系统。`connection` 负责统一服务地址，`reading`、`library` 等后续业务 Spec 分别验收各自使用的 `/icons` 和 `/api/proxy/image` 等远端资源。

## Code Style

- 保持现有 TypeScript strict、ESLint、别名和组件命名规则。
- `ReaderWorkspace.tsx` 只承载从 `AuthenticatedApp` 移出的现有组合逻辑。
- 不因为文件较长而提前拆分 Hooks、控制器、ViewModel 或 Provider。
- 不在移动过程中顺便重命名现有变量、重写 JSX、调整 Tailwind class 或改变注释含义。
- 公共导出使用已有 barrel-file 风格。

Web 外壳的调用方式：

```tsx
if (isAuthenticated) {
  return <ReaderWorkspace />;
}
```

若 `desktop-shell` 已经实施，桌面端在本模块阶段仍保持：

```tsx
createRoot(root).render(<DesktopShell />);
```

只有 `connection` 确认 API 与会话已准备好后，桌面宿主才可以改为渲染 `ReaderWorkspace`。

## Testing Strategy

### 自动验证

- 新增 `ReaderWorkspace.test.tsx`，验证共享组件在既有 Providers 与受控 Hook 数据下能够渲染主要工作区结构。
- 新增 `App.test.tsx`，验证 Web 的认证成功分支渲染 `ReaderWorkspace`，而登录、注册和网络错误分支仍由 `AppContent` 处理。
- 更新 `app-shell.test.ts`，继续从 `App.tsx` 检查 `.app-shell`，并从 `ReaderWorkspace.tsx` 检查移动文档滚动约束。
- 保留 `three-column-layout.test.tsx` 及现有组件、Hooks、stores、API 和路由测试。
- 完整 `bun run test`、`bun run lint` 和 `bun run build` 必须通过。

测试只证明共享抽取没有改变现有 Web 行为。本模块不新增针对连接、业务 API 或桌面平台的模拟验收。

### 手工回归

使用现有 Web 客户端和可用 Gist 服务，在以下宽度各执行一次：

| 宽度 | 预期布局 |
| --- | --- |
| `1440px` | 现有桌面三栏布局 |
| `1024px` | 现有平板布局与侧栏开关 |
| `390px` | 现有移动列表、正文切换和侧栏 Sheet |

每个宽度至少确认：

1. 登录成功后进入工作区。
2. 内容类型、订阅源、文件夹和收藏导航仍可选择。
3. 文章列表与正文选择仍反映到现有 URL 状态。
4. 图片模式、添加订阅入口、资料入口和设置入口仍能打开。
5. 退出登录后返回现有 Web 登录流程。

这里验证入口和布局没有因抽取而损坏，不在本模块重新验收各业务表单或远端操作的完整正确性。

## Boundaries

### Always

- 完整复用现有已登录工作区，不重新设计 UI。
- Web 和未来桌面只引用同一个 `ReaderWorkspace` 实现。
- 保持现有 Router、QueryClient、i18n、Tooltip、Hooks、stores 和 CSS 行为。
- 保持现有三个响应式区间及窗口缩小时的布局变化。
- 保持 Web 认证流程和 PWA 构建行为不变。
- 只做建立共享组件边界所需的机械移动和测试更新。

### Ask First

- 改变 `ReaderWorkspace` 的范围、props 或 Provider 所有权。
- 改变路由格式、Query keys、缓存策略、响应式断点或 UI 偏好存储方式。
- 移动或拆分现有业务组件、Hooks、stores、API 或类型。
- 改变任何用户可见布局、文案、动效、键盘或触摸行为。
- 在 `connection` 完成前把真实工作区挂到桌面入口。
- 增加依赖、通用 Adapter、Context、桥接层或桌面专属 UI。

### Never

- 复制一份 Reader UI 到 `desktop/`。
- 把整个 Web `App` 或 `main.tsx` 作为桌面入口。
- 把 `UpdateNotice`、Service Worker 或 PWA 启动逻辑放进共享工作区。
- 为桌面预览加入 fixture、模拟服务、硬编码用户或假数据。
- 在本模块修改服务地址、认证 Token、401 处理、CORS 或 API transport。
- 在本模块实现或重写阅读、订阅、AI、设置、资料或 OPML 业务。
- 为未知的未来需求增加扩展点或防御层。
- 通过删除测试、降低断言或忽略错误让验证通过。

## Success Criteria

- [ ] `frontend/src/components/reader-workspace/ReaderWorkspace.tsx` 导出零参数的 `ReaderWorkspace`。
- [ ] `ReaderWorkspace` 包含原 `AuthenticatedApp` 的完整已登录工作区组合。
- [ ] `App.tsx` 的认证成功分支渲染共享 `ReaderWorkspace`。
- [ ] `App.tsx` 继续负责登录、注册、网络错误、加载状态和 Web `UpdateNotice`。
- [ ] Web/PWA `main.tsx`、Vite PWA 配置和启动行为没有进入共享组件。
- [ ] 现有组件、Hooks、stores、API、types 和 `index.css` 没有复制到 `desktop/`。
- [ ] 现有 URL 选择语义、Query keys、缓存失效、UI 偏好和响应式断点没有改变。
- [ ] `1440px`、`1024px` 和 `390px` 下的工作区布局与交互入口保持现状。
- [ ] 本模块没有创建或修改桌面入口；若 `desktop-shell` 已实施，其入口仍渲染 `DesktopShell`，没有发送工作区 API 请求。
- [ ] 没有新增 API client、Provider 抽象、模拟数据或生产依赖。
- [ ] 定向测试、完整 `bun run test`、`bun run lint` 和 `bun run build` 全部通过。
- [ ] 没有实现或重新设计其他 capability 的业务行为。

## Open Questions

无。任何范围变化先更新本文件，再进入后续阶段。

## References

- [`CAPABILITY-MAP.md`](./CAPABILITY-MAP.md)
- [`SPEC-desktop-shell.md`](./SPEC-desktop-shell.md)
- [`frontend/src/App.tsx`](./frontend/src/App.tsx)
- [`frontend/src/main.tsx`](./frontend/src/main.tsx)
- [`frontend/src/hooks/useMobileLayout.ts`](./frontend/src/hooks/useMobileLayout.ts)
- [`frontend/src/lib/router.ts`](./frontend/src/lib/router.ts)
- [`frontend/src/lib/queryClient.ts`](./frontend/src/lib/queryClient.ts)
- [`frontend/src/api/index.ts`](./frontend/src/api/index.ts)
