# Spec: reading

> 状态：Approved（2026-08-30）
> Module ID：`reading`
> 依赖：`connection`、`reader-workspace`
> 来源：[`CAPABILITY-MAP.md`](./CAPABILITY-MAP.md)

## Objective

让桌面客户端在 `connection` 已建立的同源连接上，完整复用现有 Gist 在线阅读闭环。

完成本 Spec 后，桌面端应当能够：

- 读取订阅源、文件夹和未读数，使用现有导航选择内容。
- 浏览 `article`、`picture` 和 `notification` 三类条目。
- 保留现有筛选、无限分页、滚动位置、响应式布局和空状态。
- 查看正文、正文图片、图片墙、Lightbox 和 ImagePreview。
- 保留单条、批量、滚动和当前范围的“标为已读”行为。
- 保留收藏与取消收藏行为。
- 修复收藏页“全部标为已读”误把非收藏条目也标为已读的问题。
- 在一个按需创建、可反复复用的 Wails 原文窗口中打开桌面端外部链接。

现有 Web 客户端继续使用原来的页面、路由和 `_blank` 浏览器行为。

本模块不重新设计 Reader UI，不实现离线阅读、搜索、订阅管理、内容工具、设置编辑或 OPML。

## Confirmed Decisions

- 收藏页的“全部标为已读”只处理已收藏且未读的条目。
- 收藏仍是跨 `article`、`picture` 和 `notification` 的全局视图，不增加按类型分组的收藏页。
- 桌面端外部链接使用一个单独的 Wails 原文窗口。
- 原文窗口按需创建，同时最多存在一个；打开下一个链接时复用窗口并替换 URL。
- 原文窗口使用系统原生边框并允许调整大小；初始尺寸和位置不作为本模块验收条件。
- 关闭原文窗口不影响主窗口；之后再次打开链接时重新创建。
- 关闭主窗口时退出整个应用，即使原文窗口仍然打开。
- 原文窗口的 URL 和窗口尺寸不跨重启保存。
- 不实现分栏、应用内页签、地址栏、前进后退栏、收藏夹或浏览历史。
- 不增加新的产品 UI、生产依赖、通用窗口管理器或浏览器抽象。

## Existing Reading Contract

### Routes and selection

保持现有路由：

| 选择 | 路径 |
| --- | --- |
| 全部 | `/all/:entryId?` |
| 订阅源 | `/feed/:feedId/:entryId?` |
| 文件夹 | `/folder/:folderId/:entryId?` |
| 收藏 | `/starred/:entryId?` |

保持现有查询参数：

- `unread=true`
- `type=article|picture|notification`

筛选映射保持现状：

- `all` 把当前 `type` 作为后端 `contentType`。
- `feed` 和 `folder` 不额外发送 `contentType`，因为它们本身已经具有内容类型。
- `starred` 只发送 `starredOnly=true`，不把当前 `type` 发送给后端。
- 收藏页中的 `type` 只决定现有前端呈现模式和滚动位置；图片模式还会发送 `hasThumbnail=true`。

因此，收藏语义始终是全局跨类型。图片模式仍只展示具有缩略图的收藏条目。本模块不新增桌面专属路由、筛选状态或路由适配层。

### Navigation data

阅读模块只读取：

- feeds 的标题、类型、文件夹关系和图标路径。
- folders 的名称、层级和类型。
- 按 feed 返回的未读数。
- 调度刷新状态，用于发现服务端刷新已经完成。

现有 `GET /api/starred-count` 和 `useStarredCount` 保持可用，但当前 Reader UI 没有显示收藏总数。本模块不新增收藏计数 UI，也不为了它增加查询调用。

Feed 与 folder 的新增、编辑、删除以及手动触发刷新属于 `library`。

### Lists and pagination

- `GET /api/entries` 保持现有筛选参数和响应结构。
- 前端默认每页请求 `50` 条。
- 下一页 `offset` 使用实际已加载条目数，不使用固定页码相乘。
- 合并页面时继续按条目 ID 去重。
- 后端排序保持 `published_at DESC, id DESC`。
- 保留现有无限滚动、加载骨架、分页指示、空列表和当前错误呈现。
- 保留按 selection、内容类型和现有 key 保存的滚动位置。
- 图片模式继续附加 `hasThumbnail=true`，并使用现有 Masonry 布局。

本模块不新增错误页、重试 UI、预取、响应缓存或本地内容数据库。

### Entry detail and resources

- 选择条目后继续通过 `GET /api/entries/:id` 获取完整内容。
- Feed 图标继续使用 `/icons/:filename`。
- 正文图片和缩略图继续使用 `/api/proxy/image/:encoded?ref=...`。
- 这些根相对路径继续由 `connection` 的同源代理处理。
- Snowflake ID 继续在 JSON 和 TypeScript 中表示为字符串。
- 现有 HTML 清理、图片预览、代码高亮和内容样式保持不变。

### Read state

- 打开一篇未读条目后继续自动调用单条已读接口。
- 在“只看未读”中，当前正文不会立刻从列表消失；切换条目或卸载正文后才移除。
- `markReadOnScroll` 继续读取现有通用设置。
- 滚动标记继续使用现有等待、批量提交、乐观更新、失败回滚和滚动补偿行为。
- Lightbox 打开未读图片时继续标为已读，并避免在查看过程中刷新整个列表。
- 单次批量接口继续去重 ID，并遵守后端现有最多 `1000` 个 ID 的限制。

本模块不改变自动已读时机、未读列表移除时机、滚动锚点或缓存失效规则。

### Starred state

- 条目继续通过现有接口收藏或取消收藏。
- 成功后继续更新单条缓存并失效 entries 查询。
- 收藏页保持全类型视图，不新增收藏文件夹、标签或本地收藏副本。
- 现有收藏计数查询不是当前 UI 的验收项，不为了本模块添加显示位置。

### Refresh observation

`useRefreshStatus` 继续每 `15` 秒读取 `GET /api/feeds/refresh`。当 `lastRefreshedAt` 从已知值发生变化时，只失效现有：

- `entries`
- `unreadCounts`
- `feeds`

手动调用 `POST /api/feeds/refresh` 属于 `library`，不在本模块实现。

### Settings and content-tools boundary

- 阅读可以读取现有外观设置、可见内容类型和 `markReadOnScroll`。
- 设置的编辑与保存属于 `settings-profile`。
- Readability、AI 摘要、单篇翻译和批量翻译属于 `content-tools`。
- 现有共享组件中的这些按钮和 Hook 接缝保持原样，不为了拆分 Spec 隐藏按钮或增加 capability flag。
- 阅读测试把 `autoReadability`、`autoSummary` 和 `autoTranslate` 设为关闭，或直接 stub 对应 Hook；不能把内容工具接口计入 reading 验收。

## API Contract

本模块直接使用以下现有接口：

| Method | Path | 用途 |
| --- | --- | --- |
| `GET` | `/api/feeds` | 只读导航数据 |
| `GET` | `/api/folders` | 只读导航数据 |
| `GET` | `/api/feeds/refresh` | 观察调度刷新状态 |
| `GET` | `/api/entries` | 筛选与分页列表 |
| `GET` | `/api/entries/:id` | 单篇正文 |
| `PATCH` | `/api/entries/:id/read` | 单篇已读状态 |
| `PATCH` | `/api/entries/read` | 批量已读状态 |
| `POST` | `/api/entries/mark-read` | 当前选择全部标为已读 |
| `PATCH` | `/api/entries/:id/starred` | 收藏状态 |
| `GET` | `/api/unread-counts` | 各 feed 未读数 |
| `GET` | `/icons/:filename` | Feed 图标 |
| `GET` | `/api/proxy/image/:encoded?ref=...` | 正文、缩略图和预览图片 |

除下面的收藏 mark-all 修复外，接口继续沿用现有 JSON 类型、字符串 ID、认证、状态码和错误响应。本模块不新增 typed Go business client，也不把普通业务请求改成 Wails bindings。

## Starred Mark-all Fix

当前前端收藏 selection 已产生 `{starredOnly: true}`，但 `MarkAllReadParams` 没有声明该字段，后端请求结构也会忽略它。结果是收藏页执行 mark-all 时，后端落入无筛选分支，把所有未读条目都标为已读。

最小修复是在现有请求中增加一个字段：

```ts
export interface MarkAllReadParams {
  feedId?: string;
  folderId?: string;
  contentType?: ContentType;
  starredOnly?: boolean;
}
```

后端把同一布尔值沿现有 handler、service、repository 传递：

```go
MarkAllAsRead(
	ctx context.Context,
	feedID *int64,
	folderID *int64,
	contentType *string,
	starredOnly bool,
) error
```

行为契约：

- 请求体为 `{"starredOnly":true}` 时，只更新 `starred = 1 AND read = 0` 的条目。
- 非收藏的未读条目保持未读。
- 已读收藏条目保持已读，不做无意义改写。
- 收藏范围横跨三种内容类型。
- 省略或传入 `starredOnly:false` 时，现有 feed、folder、content type 和全局行为保持不变。
- 现有 UI 每次只发送一个 selection scope。本 Spec 只增加独立的收藏分支，不定义多个筛选字段组合的新语义。
- 不增加新 endpoint、数据库字段、migration、filter object 或通用筛选引擎。

repository 继续保留现有 feed、folder、content type 的判断顺序，并在这些范围都未提供时处理 `starredOnly`；最后才进入无筛选的全局分支。生成的 service 与 repository mocks 必须同步更新。

## Desktop Original Page Window Contract

### Frontend interception

桌面入口安装一个具体的、无 UI 的 `DesktopOriginalLinks` click handler。它集中处理当前共享 UI 中的 `a[target="_blank"]`，避免在每个阅读组件中加入桌面运行模式判断。

现有阅读链接包括：

- 正文标题。
- 正文头部的“打开原文”。
- 正文 HTML 内链接。
- Lightbox 的原文链接和视频链接。

行为：

1. Web 构建不安装该 handler，继续执行浏览器原生 `_blank`。
2. 桌面 handler 只拦截解析后为绝对 `http` 或 `https` 的链接。
3. handler 阻止 WebView 默认打开行为，并调用生成的 `OriginalPageService.Open(url)` binding。
4. binding 失败时保持 Reader route 和主窗口不变，并在开发日志中保留错误；不静默改用系统浏览器。
5. 非链接点击和其他 scheme 不调用 binding，继续使用既有行为。

该 handler 是具体的桌面宿主行为，不建立通用 link adapter、事件总线或可插拔导航系统。

### Go service

桌面进程增加一个具体的 `OriginalPageService`：

```go
func (s *OriginalPageService) Open(rawURL string) error
```

`Open` 的行为：

1. 使用 `net/url` 解析绝对 URL，并只接受 `http` 与 `https`。
2. 第一次调用时创建名称固定为 `original` 的原生、可调整大小的 `WebviewWindow`，直接加载该 URL。
3. 已存在 `original` 窗口时，不创建新窗口；改用 `SetURL` 切换内容，并把窗口显示、恢复和聚焦到前台。
4. 用户关闭 `original` 后不保留空引用；下次调用重新创建。
5. 关闭原文窗口不改变认证、Reader route 或主窗口生命周期。
6. 主窗口关闭时退出应用，并同时结束原文窗口。

URL 中的普通 path、query 和 fragment 必须保留。这里不增加域名 allowlist、证书固定、重复 DNS 检查、内容扫描或额外风险评分。URL scheme 和 host 检查只是远程 WebView 的真实输入契约。

原文窗口直接访问第三方页面：

- 不经过 Gist `/api` 或 `/icons` 代理。
- 不转发 Gist Token、Cookie 或其他连接状态。
- 不向远程页面注册 Gist services、`ReaderWorkspace` 或自定义注入脚本。
- 不提供应用页签、分栏、地址栏、导航按钮、下载管理或历史存储。
- 原文页面内部自己的导航与弹窗行为交给 WebView 默认行为；Gist 不建立站点兼容层。

`OriginalPageService` 直接使用 Wails 的窗口管理能力，不增加 `WindowManager` interface、factory、registry 或第二套生命周期框架。

## Tech Stack

继续使用现有依赖：

| 层 | 现有技术 |
| --- | --- |
| Shared frontend | React、wouter、TanStack Query、Zustand、现有组件与 CSS |
| Desktop host | Wails `v3.0.0-beta.12` 与生成的 TypeScript bindings |
| Backend | Echo、现有 service/repository、SQLite |
| Tests | Vitest、Testing Library、Go `testing`、testify、GoMock |

不增加生产依赖、嵌入式浏览器库、窗口状态库、API adapter 或离线存储库。

## Commands

以下命令是本模块实施后必须成立的验证契约。

### Regenerate backend mocks

```bash
cd backend
make gen
```

只接受由现有 `go:generate` 声明产生的机械 mock 更新，不手工编辑生成文件。

### Targeted backend tests

```bash
cd backend
go test ./internal/handler ./internal/service ./internal/repository
```

### Targeted frontend tests

```bash
cd frontend
bun run test -- \
  src/api/entries.test.ts \
  src/lib/router.test.ts \
  src/lib/entry-pagination.test.ts \
  src/components/entry-list/EntryList.test.tsx \
  src/components/entry-list/EntryListItem.test.tsx \
  src/components/picture-masonry/PictureMasonry.test.tsx \
  src/components/picture-masonry/Lightbox.test.tsx \
  src/components/entry-content/EntryContentBody.test.tsx \
  src/components/ui/image-preview.test.tsx \
  src/desktop/DesktopOriginalLinks.test.tsx
```

### Full verification

```bash
cd backend
make test
make lint

cd ../frontend
bun install --frozen-lockfile
bun run lint
bun run test
bun run build
bun run build:desktop
bun run verify:desktop-assets

cd ../desktop
go fmt ./...
go vet ./...
go test ./...
wails3 build
```

### Manual desktop verification

```bash
cd desktop
wails3 dev
```

## Project Structure

```text
desktop/
├── main.go                         # 注册 OriginalPageService 与主窗口退出行为
├── original_page.go                # 创建、复用和聚焦原文窗口
└── original_page_test.go           # URL 输入与可独立验证的 service 行为

frontend/
├── bindings/                       # 增加生成的 OriginalPageService binding
└── src/
    ├── api/
    │   ├── index.ts                # 现有 mark-all 请求
    │   └── entries.test.ts
    ├── types/api.ts                # MarkAllReadParams 增加 starredOnly
    ├── hooks/
    │   ├── useEntries.ts           # 保留现有缓存失效
    │   └── useSelection.ts         # 保留现有 starredOnly 映射
    └── desktop/
        ├── DesktopOriginalLinks.tsx
        └── DesktopOriginalLinks.test.tsx

backend/internal/
├── handler/
│   ├── entry_handler.go
│   └── entry_handler_test.go
├── service/
│   ├── entry_service.go
│   ├── entry_service_test.go
│   └── mock/entry_service.go       # 重新生成
└── repository/
    ├── entry_repository.go
    ├── entry_repository_test.go
    └── mock/entry_repository.go    # 重新生成
```

不移动、复制或拆分现有 `EntryList`、`EntryContent`、`PictureMasonry`、`Lightbox`、router、stores 或 Query Hooks。`DesktopOriginalLinks` 不渲染产品 UI。

## Code Style

### TypeScript / React

- 沿用 strict TypeScript、现有别名和 ESLint 规则。
- 保持现有 Query keys、API helper 和组件 props。
- 桌面链接 handler 只负责识别 anchor、调用 binding 和清理 event listener。
- 不在共享组件内读取 Wails 全局对象，也不增加 `isDesktop` 条件分支。
- 不使用未等待的 binding promise；失败必须保留原始错误上下文。

### Go

- 使用 `gofmt`，沿用现有 handler、service、repository 命名和错误返回方式。
- `starredOnly` 沿现有函数链传递，不为一个布尔值创建 filter struct。
- `OriginalPageService` 使用具体 Wails application 实例，不创建 interface 或 factory。
- 远程 URL 窗口与主窗口生命周期写在桌面模块，不进入 backend。
- 不手工编辑 GoMock 或 Wails 生成文件。

## Testing Strategy

### Backend

至少覆盖：

- handler 能从 `{"starredOnly":true}` 解析并向 service 传递 `true`。
- service 向 repository 传递同一值，原有 feed/folder 存在性检查保持不变。
- repository 中收藏未读变为已读，非收藏未读保持未读，已读收藏保持已读。
- 收藏分支横跨三种 feed type。
- 没有收藏时成功返回且不影响其他条目。
- 原有 feed、folder、content type 和无筛选 mark-all 测试继续通过。
- 不增加数据库 migration。

### Frontend

至少覆盖：

- 收藏 selection 继续产生 `{starredOnly:true}`。
- `markAllAsRead` 请求 JSON 包含 `starredOnly:true`。
- 成功后继续只失效 `entries` 与 `unreadCounts`；不新增无用的收藏计数失效。
- 文章列表、图片模式、自动已读、滚动批量已读、失败回滚和滚动补偿的现有测试继续通过。
- Web 链接继续具有现有 `target="_blank"`。
- 桌面点击受支持链接只调用一次 `OriginalPageService.Open`。
- 非链接点击和非 HTTP(S) 链接不调用 binding。
- binding 失败不改变 Reader route 或卸载工作区，并保留可诊断错误。

### Desktop service and lifecycle

自动测试验证不依赖真实 WebView 的 URL 输入与 service 逻辑。窗口创建、复用、聚焦和生命周期通过当前原生环境的手工冒烟验证，不为了 mock Wails 窗口再建立 interface。

在 Windows、macOS 和 Linux 当前架构分别确认：

1. 导航、列表、分页、正文、图片和收藏正常。
2. Feed 图标与代理图片能够加载。
3. 打开正文或 Lightbox 会按现有规则标为已读。
4. “只看未读”和滚动标为已读保持现有行为。
5. 收藏页 mark-all 不影响非收藏未读条目。
6. 第一次点击原文创建一个原生窗口。
7. 再点击不同链接会复用同一窗口、加载新 URL 并聚焦，不出现第二个原文窗口。
8. 关闭原文窗口后主窗口继续运行。
9. 再次点击链接能够重新创建原文窗口。
10. 原文窗口仍打开时关闭主窗口，应用完全退出。

不新增人为覆盖率阈值。

## Boundaries

### Always

- 复用现有 Reader UI、路由、Query keys、stores 和缓存行为。
- 使用 `connection` 提供的相对 `/api` 与 `/icons` 请求链路。
- 保持现有 Web 阅读与 `_blank` 行为。
- 修复收藏 mark-all 的实际数据错误。
- 保持一个主窗口；桌面原文窗口按需存在且最多一个。
- 保留足够诊断信息，不增加无具体故障目标的防御层。

### Ask First

- 改变路由、筛选语义、分页大小、排序或 Query keys。
- 改变自动已读、滚动标记、未读列表移除时机或滚动锚点。
- 改变收藏视图的跨类型语义。
- 新增阅读错误页、重试 UI、数据缓存或搜索。
- 将原文窗口改成分栏、多页签或自定义浏览器。
- 增加第二个原文窗口、其他辅助窗口、地址栏、导航栏、下载管理或历史持久化。
- 把 Readability、AI、订阅管理、设置编辑或 OPML 并入 reading。
- 增加生产依赖或通用 API/window adapter。

### Never

- 复制 Reader UI 到 `desktop/`。
- 新增离线数据库、全文索引、预取系统或同步层。
- 把 Gist Token、connection state 或 bindings 注入远程原文页面。
- 在原文窗口挂载 `ReaderWorkspace`。
- 为第三方网站增加 allowlist、证书固定、广告拦截、内容检查或浏览器安全套件。
- 在 reading 中实现 feed/folder CRUD、手动刷新、AI、Readability、设置编辑或 OPML。
- 为未来多窗口、多连接或插件场景建立 registry、factory、事件总线或扩展框架。
- 通过删除测试、降低断言或吞掉 binding 错误让验证通过。

## Success Criteria

- [ ] 桌面端完成现有在线阅读闭环，Web 行为没有回归。
- [ ] 现有路由、查询参数和 selection 语义保持不变。
- [ ] feed/folder 导航和未读数正常；未新增无用的收藏计数 UI。
- [ ] 三类内容列表、50 条分页、去重、滚动位置和空状态正常。
- [ ] 正文、Feed 图标、代理图片、图片模式、Lightbox 和 ImagePreview 正常。
- [ ] 自动已读、滚动已读、批量更新、失败回滚和未读移除时机保持现状。
- [ ] 收藏与取消收藏保持现状。
- [ ] 收藏页 mark-all 只更新收藏中的未读条目，非收藏未读条目不受影响。
- [ ] 桌面阅读链接打开一个可复用的原文窗口。
- [ ] 连续打开不同链接不会创建第二个 Gist 原文窗口。
- [ ] 关闭原文窗口不影响主窗口；关闭主窗口退出整个应用。
- [ ] Web 链接继续使用当前浏览器 `_blank` 行为。
- [ ] 原文页面不获得 Gist Token、connection state、Reader UI 或 bindings。
- [ ] 不新增业务 UI、生产依赖、离线能力或通用抽象。
- [ ] 定向测试、完整前后端回归和当前平台 Wails 构建全部通过。
- [ ] Windows、macOS 和 Linux 完成阅读与原文窗口冒烟。

## Open Questions

无。用户已确认：收藏页只处理收藏未读条目；桌面端使用一个可复用的 Wails 原文窗口。

本文件已经批准。按照已确认的工作流，继续完成其余 capability Spec；全部 Spec 就绪后再统一进入任务拆分。

## References

- [`CAPABILITY-MAP.md`](./CAPABILITY-MAP.md)
- [`SPEC-desktop-shell.md`](./SPEC-desktop-shell.md)
- [`SPEC-reader-workspace.md`](./SPEC-reader-workspace.md)
- [`SPEC-connection.md`](./SPEC-connection.md)
- [Wails beta.12 window manager](https://github.com/wailsapp/wails/blob/v3.0.0-beta.12/v3/pkg/application/window_manager.go)
- [Wails beta.12 WebviewWindow](https://github.com/wailsapp/wails/blob/v3.0.0-beta.12/v3/pkg/application/webview_window.go)
- [Wails beta.12 WebviewWindow options](https://github.com/wailsapp/wails/blob/v3.0.0-beta.12/v3/pkg/application/webview_window_options.go)
