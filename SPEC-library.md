# Spec: library

> 状态：Approved（2026-08-30）
> Module ID：`library`
> 依赖：`connection`、`reader-workspace`
> 来源：[`CAPABILITY-MAP.md`](./CAPABILITY-MAP.md)

## Objective

让桌面客户端通过 `connection` 已建立的同源连接，完整复用现有 Gist 订阅与文件夹管理功能。

完成本 Spec 后，Web 与桌面端都应当能够：

- 预览并添加 RSS/Atom 订阅。
- 为订阅设置自定义标题，并选择已有文件夹或在添加订阅时创建文件夹。
- 编辑订阅标题、文件夹归属和现有的摘要自定义提示词字段。
- 修改订阅或文件夹的内容类型。
- 单个或批量删除订阅与文件夹。
- 在设置页手动执行现有的“全部更新”。
- 在管理操作完成后立即反映新的导航、文章归属和未读数。

本模块不重新设计现有 UI，不新增独立文件夹管理能力、单订阅刷新、OPML、AI 执行逻辑、离线订阅副本或桌面专属业务 API。

## Confirmed Decisions

- 保持现有文件夹 UI：文件夹只在添加订阅时顺带创建；继续支持改类型、单个删除和批量删除。
- 不新增独立创建、重命名、拖拽、嵌套或树形文件夹管理界面。
- 文件夹内的单个订阅改成另一内容类型时，订阅自动移出原文件夹，成为新类型下的未分类订阅。
- 当前正在查看的订阅或文件夹改类型后，继续保留该选择、当前文章和“只看未读”状态，只把路由内容类型切换到目标类型。
- 删除当前正在查看的订阅或文件夹后，返回当前内容类型的“全部”视图。
- 删除继续沿用现有立即执行行为，不新增确认弹窗。
- 手动刷新只保留现有“全部更新”，不接通单订阅刷新。
- 添加订阅预览中的站点链接继续保留现有 `_blank` 标记；桌面宿主如何打开该链接由 `reading` 独立验收，`library` 不增加第二套外链或窗口方案。
- 继续复用同一份 React UI 和相对 `/api` 请求；不增加 Wails binding、桌面 API client 或数据 Adapter。

## Existing Library Contract

### UI entry points

本模块直接复用以下现有入口：

- 侧栏加号打开的 `AddFeedPage`。
- 侧栏中订阅和文件夹的右键菜单。
- 设置弹窗中的“订阅源”和“文件夹”面板。
- 订阅编辑使用的 `EditFeedDialog`。

`reader-workspace` 继续拥有这些组件的组合和弹窗入口。`library` 只验收其中的订阅、文件夹和手动刷新行为，不拆分组件，也不增加 capability flag。

### Feed preview and subscription

添加订阅保持现有流程：

1. 输入地址经现有前端逻辑规范化；缺少 scheme 时补 `https://`，`feed://` 转为 `https://`。
2. `GET /api/feeds/preview` 拉取并解析 RSS/Atom，页面显示现有加载、预览和错误状态。
3. 用户可以填写自定义标题，并选择“不使用文件夹”、已有文件夹或输入名称创建文件夹。
4. 选择已有文件夹时，新订阅继承该文件夹的内容类型。
5. 创建文件夹时，新文件夹和订阅使用当前目标内容类型。
6. 如需新文件夹，继续先创建文件夹再创建订阅；订阅创建失败时不回滚已经创建的空文件夹。
7. 订阅成功后关闭添加页，并返回打开添加页时所在内容类型的“全部”视图。

重复订阅继续使用服务端 `409` 响应中的 `error: "feed_exists"` 显示现有本地化提示。预览与正式添加继续是两次独立抓取；本模块不建立预览结果缓存或事务式添加流程。

预览卡中的站点链接继续保留 `_blank`。桌面宿主对 `_blank` 的处理属于 `reading`；本模块只保证没有删除标记或实现第二套 handler。

### Feed management

保持现有订阅管理能力：

- 编辑标题；标题裁剪后不得为空。
- 编辑 `summaryPromptReminder`；空白值表示清除，最多 `2000` 个 Unicode 字符。
- 在同一内容类型的文件夹之间移动，或移到未分类。
- 修改订阅内容类型。
- 单个删除和设置页批量删除。
- 设置页按标题、创建时间或更新时间排序和选择。

`summaryPromptReminder` 作为订阅元数据由本模块保存。它如何影响 AI 摘要属于 `content-tools`。

订阅 URL 继续只读；本模块不增加修改 URL、复制订阅、暂停订阅或单独刷新入口。

### Folder management

保持现有文件夹管理能力：

- 添加订阅时按名称创建根文件夹。
- 修改文件夹内容类型，并同步修改该文件夹直接包含的订阅类型。
- 单个删除和设置页批量删除。
- 设置页按名称、创建时间或更新时间排序和选择。

本模块不在 UI 中暴露后端已有但当前前端未使用的 `PUT /api/folders/:id`、`parentId`、重命名或层级编辑。

删除当前 UI 创建的顶层文件夹时，继续删除该文件夹直接包含的订阅和文章，并保留设置页现有警告。已有嵌套文件夹继续使用当前扁平展示和后端删除语义；本模块不新增层级 UI 或递归删除规则。

### Manual refresh

设置页“全部更新”继续调用 `POST /api/feeds/refresh`：

- 请求成功后刷新订阅、文章和未读数。
- 请求期间按钮保持现有禁用和加载状态。
- 已有刷新正在执行时，`409` 继续显示现有“刷新进行中”提示。
- 其他失败继续显示现有“刷新失败”提示。
- 单个订阅抓取失败继续写入该订阅的 `errorMessage`，不把整轮刷新改成失败。
- `lastRefreshedAt` 表示整轮处理结束，不保证每个订阅都成功。

`reading` 继续负责通过 `GET /api/feeds/refresh` 观察后台刷新状态。`library` 不实现第二个轮询器，也不接通当前未使用的单订阅 `onRefresh` 属性。

### Query and route consistency

各项操作成功后使用以下固定 Query invalidation：

| 操作 | 失效的 Query families |
| --- | --- |
| 添加订阅 | `feeds`、`entries`、`unreadCounts`；同时创建文件夹时再加 `folders` |
| 编辑或移动订阅 | `feeds`、`entries` |
| 修改单个订阅类型 | `feeds`、`entries` |
| 修改文件夹类型 | `folders`、`feeds`、`entries` |
| 删除订阅或文件夹 | `feeds`、`folders`、`entries`、`unreadCounts` |
| 全部更新 | `feeds`、`entries`、`unreadCounts` |

这里继续使用现有 Query keys，不增加统一 invalidation registry 或事件层。

删除操作从侧栏和设置页发起时使用同一规则：

- 若删除集合包含当前 `/feed/:feedId` 选择，返回 `/all?type=<当前内容类型>`。
- 若删除集合包含当前 `/folder/:folderId` 选择，返回 `/all?type=<当前内容类型>`。
- 返回“全部”时不保留已删除条目的 `entryId`。
- 继续保留当前的 `unread=true` 筛选状态，并使用 replace 导航，不把已删除地址留在历史记录中。
- 删除其他对象时保留当前选择。

批量删除继续使用现有接口和错误提示。本 Spec 只定义成功响应后的导航与 Query 更新，不增加全有或全无事务，也不定义部分成功协调机制。

## API Contract

本模块继续使用现有 HTTP API：

| Method | Path | 本模块用途 |
| --- | --- | --- |
| `GET` | `/api/feeds/preview?url=...` | 预览 RSS/Atom |
| `GET` | `/api/feeds` | 添加、侧栏和设置页所需订阅数据 |
| `POST` | `/api/feeds` | 添加订阅 |
| `PUT` | `/api/feeds/:id` | 修改标题、文件夹和摘要提示词 |
| `PATCH` | `/api/feeds/:id/type` | 实际改变类型时移出原文件夹 |
| `DELETE` | `/api/feeds/:id` | 删除单个订阅 |
| `DELETE` | `/api/feeds` | 批量删除订阅 |
| `POST` | `/api/feeds/refresh` | 手动执行全部更新 |
| `GET` | `/api/folders` | 添加、侧栏和设置页所需文件夹数据 |
| `POST` | `/api/folders` | 添加订阅时创建文件夹 |
| `PATCH` | `/api/folders/:id/type` | 修改文件夹及其直接订阅的类型 |
| `DELETE` | `/api/folders/:id` | 删除单个文件夹及其内容 |
| `DELETE` | `/api/folders` | 批量删除文件夹及其内容 |

这些请求继续通过 `connection` 提供的同源 `/api` 代理和 Bearer Token 运行。JSON 字段、字符串 Snowflake ID、状态码和现有错误响应保持不变。下面两项修复只改变成功后的数据或页面语义，不改变接口形状。

`PUT /api/folders/:id` 继续是现有后端接口，但不是本模块 UI 或验收范围。`GET /api/feeds/refresh` 属于 `reading` 的刷新观察契约。

## Correctness Fixes

### Feed type change detaches from folder

当前 `PATCH /api/feeds/:id/type` 只修改 `feed.type`，但保留原 `folder_id`。前端会把订阅显示在新类型的“未分类”中，数据库却仍把它挂在旧类型文件夹下；以后删除旧文件夹时，该订阅仍会被删除。

修复后的行为：

- 目标类型与当前类型不同时，在同一条持久化更新中修改类型并把 `folder_id` 设为 `NULL`。
- 目标类型与当前类型相同时保持现有文件夹归属，不产生意外移动。
- 订阅的标题、URL、摘要提示词、抓取元数据和文章保持不变。
- 返回状态继续是 `204`，请求体仍只有现有 `type` 字段。
- 修改文件夹类型仍使用 `PATCH /api/folders/:id/type`，并继续同步该文件夹直接包含的订阅；它不会把这些订阅移出文件夹。
- 不增加新 endpoint、请求字段、数据库 migration 或通用“类型迁移”抽象。

### Type change keeps the current selection

当前正在查看的订阅或文件夹改类型后，旧的 `type` 查询参数会使对象从当前侧栏消失，但正文仍停留在原路由。

修复后的行为：

- 当前 selection 就是被修改的订阅时，继续保留 `/feed/:feedId/:entryId?`，并用 replace 把 `type` 改成目标类型。
- 当前 selection 就是被修改的文件夹时，继续保留 `/folder/:folderId/:entryId?`，并用 replace 把 `type` 改成目标类型。
- 保留当前 `entryId` 和 `unread=true` 状态。
- 修改非当前订阅或文件夹时不改变路由。
- 继续使用现有 wouter、`buildPath` 和 selection 状态，不增加路由 store 或类型切换协调层。

### Delete current selection returns to All

当前删除正在查看的订阅或文件夹后，路由仍可能指向已经不存在的 ID，并继续显示缓存中的文章。

修复后的行为：

- 侧栏单删和设置页批量删除都检查删除集合是否包含当前选择。
- 删除成功后失效相关 Query，并返回当前内容类型的“全部”；本 Spec 不规定两者的内部执行先后。
- 导航使用现有 wouter 路由与 selection helper，不新增全局路由 store。
- 删除非当前对象时不改变路由。

## Tech Stack

继续使用仓库已有依赖：

| 层 | 现有技术 |
| --- | --- |
| Shared frontend | React、wouter、TanStack Query、现有组件与 CSS |
| Desktop connection | `connection` 的 Wails 同源代理 |
| Backend | Echo、现有 service/repository、SQLite |
| Tests | Vitest、Testing Library、Go `testing`、testify、GoMock |

本模块不增加生产依赖、状态管理库、通知系统、桌面 binding、数据库 migration 或本地订阅存储。

## Commands

以下命令是本模块实施后必须成立的验证契约。

### Targeted backend tests

```bash
cd backend
go test ./internal/handler ./internal/service ./internal/repository
```

### Targeted frontend tests

```bash
cd frontend
bun run test -- \
  src/hooks/useAddFeed.test.tsx \
  src/lib/url.test.ts \
  src/hooks/useFeeds.test.tsx \
  src/hooks/useFolders.test.tsx \
  src/components/add-feed/AddFeedPage.test.tsx \
  src/components/settings/tabs/EditFeedDialog.test.tsx \
  src/components/settings/tabs/FeedsSettings.test.tsx \
  src/components/settings/tabs/FoldersSettings.test.tsx \
  src/components/sidebar/Sidebar.library.test.tsx
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

### Manual Web and desktop verification

```bash
cd frontend
bun run dev

cd ../desktop
wails3 dev
```

Web 与桌面开发命令分别运行，不要求在同一个终端同时启动。

手工验收前必须准备一个已初始化且可登录的 Gist 服务。服务部署与首次初始化不属于本模块。

## Project Structure

```text
frontend/src/
├── api/index.ts                              # 现有 feed/folder/refresh 请求
├── components/
│   ├── add-feed/
│   │   ├── AddFeedPage.tsx
│   │   ├── FeedUrlForm.tsx
│   │   └── FeedPreviewCard.tsx
│   ├── settings/tabs/
│   │   ├── EditFeedDialog.tsx
│   │   ├── FeedsSettings.tsx
│   │   └── FoldersSettings.tsx
│   └── sidebar/
│       ├── Sidebar.tsx
│       ├── FeedCategory.tsx
│       └── FeedItem.tsx
├── hooks/
│   ├── useAddFeed.ts
│   ├── useFeeds.ts
│   └── useFolders.ts
└── types/api.ts                              # 现有 Feed、Folder 与请求类型

backend/internal/
├── handler/
│   ├── feed_handler.go
│   └── folder_handler.go
├── service/
│   ├── feed_service.go
│   ├── folder_service.go
│   └── refresh_service.go
└── repository/
    ├── feed_repository.go
    └── folder_repository.go
```

测试继续与对应源码相邻。不得为了本模块移动、复制或重写现有添加页、侧栏、设置页、Hooks、API client 或 backend 分层。

## Code Style

### TypeScript / React

- 沿用 strict TypeScript、现有别名、ESLint、TanStack Query 和 wouter 约定。
- 在现有 mutation hooks 中完成 Query invalidation，不增加事件总线或第二个缓存协调层。
- 删除后的导航复用当前 selection 与 content type，不从 DOM 或 URL 字符串重复解析状态。
- 继续使用现有组件内联错误区域；不为了统一提示新增全局 toast 系统。

示例：删除成功后只在删除对象就是当前选择时导航。

```tsx
if (selection.type === "feed" && deletedIds.has(selection.feedId)) {
  onSelectAll?.(contentType);
}
```

### Go

- 使用 `gofmt`，沿用现有 handler、service、repository 和错误映射。
- 单订阅改类型通过现有 endpoint 和一次持久化更新同时修改 `type` 与 `folder_id`。
- 文件夹删除复用现有 service/repository 行为，不建立通用树仓储、工作流引擎或后台任务。
- 不手工编辑 GoMock 生成文件。

## Testing Strategy

### Backend

至少覆盖：

- Feed preview 的成功、非法 URL 和抓取失败映射保持现状。
- 添加订阅的默认类型、文件夹类型匹配、重复 URL 和抓取失败后保存错误状态保持现状。
- 修改订阅标题、文件夹和摘要提示词继续工作，`2000` 字符限制保持不变。
- 实际修改单个订阅类型会清除 `folder_id`，但保留其他订阅字段和文章；提交相同类型不会移出文件夹。
- 修改文件夹类型继续更新该文件夹直接包含的订阅类型，并保留归属关系。
- 删除订阅继续级联删除其文章和 AI 缓存。
- 删除当前 UI 创建的顶层文件夹会删除其直接订阅和文章。
- 单个与批量删除的现有状态码和错误映射保持可用。
- “全部更新”的并发 `409`、完成状态和单订阅失败语义保持现状。
- 不增加数据库 migration。

### Frontend

至少覆盖：

- URL 规范化、预览加载/错误和重复订阅提示。
- 无文件夹、已有文件夹和添加时创建文件夹三种订阅流程。
- 已有文件夹决定新订阅的内容类型；新文件夹继承目标类型。
- 编辑标题与摘要提示词，移动到同类型文件夹或未分类。
- 订阅改类型后重新显示为新类型下的未分类订阅。
- 文件夹改类型后，文件夹和其中订阅共同移动到目标类型。
- 当前订阅或文件夹改类型后保留 selection、entry 和未读筛选，只把路由类型替换为目标类型；修改其他对象不导航。
- 单删与批量删除会刷新受影响的 `feeds`、`folders`、`entries` 和 `unreadCounts`。
- 删除当前订阅或文件夹返回当前类型的“全部”；删除其他对象不改变路由。
- “全部更新”请求期间保持禁用和加载状态；成功后刷新相关 Query，`409` 和其他失败分别显示现有提示。
- 预览站点链接继续保留 `_blank`；本模块不实现桌面链接 handler。
- 现有列表排序、选择、加载、空状态和内联错误行为保持不变。

### Manual regression

在 Web 验证一次，并在 Windows、macOS 和 Linux 当前架构分别完成短冒烟：

1. 预览并添加无文件夹订阅。
2. 添加到已有文件夹，以及添加时创建文件夹。
3. 编辑标题、摘要提示词和文件夹归属。
4. 把当前订阅改成另一类型后，当前文章和未读筛选保持，侧栏切到目标类型且订阅位于未分类区域。
5. 修改当前文件夹类型后，当前文章和未读筛选保持，侧栏切到目标类型且其中订阅仍属于该文件夹。
6. 删除当前订阅或文件夹后返回当前类型的“全部”，不再显示旧文章。
7. 单个和批量删除后，侧栏、设置列表和未读数一致。
8. 执行“全部更新”，确认请求中的加载状态、并发刷新提示和普通失败提示。
9. Web 预览站点链接继续按现有 `_blank` 行为打开。

不新增人为覆盖率阈值，也不为真实 WebView 建立额外的窗口 mock 层。

## Boundaries

### Always

- 复用现有 AddFeedPage、侧栏、设置面板、Hooks、Query keys 和 HTTP API。
- 使用 `connection` 提供的相对 `/api` 与 `/icons` 请求链路。
- 保持 Web 与桌面使用同一份业务实现。
- 订阅单独改类型时移出旧文件夹，保持订阅和文件夹类型关系一致。
- 当前订阅或文件夹改类型时保留 selection、entry 和未读筛选，并把路由切换到目标类型。
- 删除当前选择后返回当前内容类型的“全部”，并清除陈旧 Query。
- 当前 UI 创建的文件夹删除结果与现有级联警告一致。
- 只修复已经确认的数据和页面状态错误，不增加无目标的防御设计。

### Ask First

- 新增独立创建、重命名、嵌套、拖拽或树形文件夹 UI。
- 改变文件夹删除的级联语义或增加删除确认弹窗。
- 新增单订阅刷新、定时刷新设置或后台同步能力。
- 允许修改订阅 URL，或改变重复订阅判断方式。
- 改变 Query keys、路由格式、内容类型或缓存策略。
- 把 AI 摘要执行、OPML、其他设置面板或阅读行为并入本模块。
- 改变现有嵌套文件夹的扁平展示或删除语义。
- 增加生产依赖、数据库 migration、Wails binding 或桌面专属 API client。

### Never

- 复制一份订阅管理 UI 到 `desktop/`。
- 为桌面请求建立第二套 feed/folder API client 或业务 Adapter。
- 把后端已有但 UI 未使用的文件夹重命名和层级能力自动暴露出来。
- 把未接线的单订阅刷新属性当成已确认产品功能。
- 在本模块新增嵌套文件夹的树形 UI 或递归删除规则。
- 为添加订阅增加分布式事务、补偿框架或预览缓存层。
- 在本模块实现 OPML、AI 摘要/翻译、设置资料或离线订阅数据库。
- 为未来多连接、插件或同步场景增加 registry、factory、事件总线或扩展点。
- 通过吞掉错误、删除测试或降低断言让验证通过。

## Success Criteria

- [ ] Web 与桌面复用同一套订阅和文件夹管理 UI、Hooks 与 API client。
- [ ] Feed preview、URL 规范化、自定义标题和重复订阅提示保持现状。
- [ ] 可以添加无文件夹订阅、选择已有文件夹，或在添加订阅时创建文件夹。
- [ ] 可以编辑订阅标题、文件夹归属和摘要提示词。
- [ ] 单个订阅改类型时自动移出原文件夹，并显示在新类型的未分类区域。
- [ ] 文件夹改类型时，其直接订阅同步改类型并继续保留在文件夹中。
- [ ] 当前订阅或文件夹改类型后继续显示当前 selection 和文章，保留未读筛选，并用目标类型替换路由；修改其他对象不导航。
- [ ] 可以单个和批量删除订阅与文件夹；删除当前 UI 创建的文件夹会删除其直接订阅和文章。
- [ ] 删除当前正在查看的订阅或文件夹后返回当前类型的“全部”，不显示陈旧文章。
- [ ] 管理操作后 feeds、folders、entries 和 unreadCounts 与服务端结果一致。
- [ ] 设置页“全部更新”的加载、成功、并发 `409` 和普通失败状态保持现状；未新增单订阅刷新。
- [ ] 预览站点链接保留 `_blank`；library 未实现第二套桌面链接 handler。
- [ ] 未新增独立文件夹管理 UI、确认弹窗、生产依赖、migration、Wails binding 或桌面 Adapter。
- [ ] 未把 OPML、AI 执行逻辑、其他设置或离线能力并入 library。
- [ ] 定向测试、完整前后端回归和当前平台 Wails 构建全部通过。
- [ ] Windows、macOS 和 Linux 完成添加、编辑、改类型、删除与全部刷新冒烟。

## Open Questions

无。用户已确认：保持现有文件夹 UI；单订阅改类型时自动移出旧文件夹；当前订阅或文件夹改类型后保留选择并切换路由类型；删除当前对象后返回当前类型的“全部”。

本文件已经批准。按照已确认的工作流，继续完成其余 capability Spec；全部 Spec 就绪后再统一进入任务拆分。

## References

- [`CAPABILITY-MAP.md`](./CAPABILITY-MAP.md)
- [`SPEC-reader-workspace.md`](./SPEC-reader-workspace.md)
- [`SPEC-connection.md`](./SPEC-connection.md)
- [`SPEC-reading.md`](./SPEC-reading.md)
- [`frontend/src/hooks/useAddFeed.ts`](./frontend/src/hooks/useAddFeed.ts)
- [`frontend/src/hooks/useFeeds.ts`](./frontend/src/hooks/useFeeds.ts)
- [`frontend/src/hooks/useFolders.ts`](./frontend/src/hooks/useFolders.ts)
- [`frontend/src/components/add-feed/AddFeedPage.tsx`](./frontend/src/components/add-feed/AddFeedPage.tsx)
- [`frontend/src/components/sidebar/Sidebar.tsx`](./frontend/src/components/sidebar/Sidebar.tsx)
- [`frontend/src/components/settings/tabs/FeedsSettings.tsx`](./frontend/src/components/settings/tabs/FeedsSettings.tsx)
- [`frontend/src/components/settings/tabs/FoldersSettings.tsx`](./frontend/src/components/settings/tabs/FoldersSettings.tsx)
- [`backend/internal/service/feed_service.go`](./backend/internal/service/feed_service.go)
- [`backend/internal/service/folder_service.go`](./backend/internal/service/folder_service.go)
- [`backend/internal/repository/feed_repository.go`](./backend/internal/repository/feed_repository.go)
- [`backend/internal/repository/folder_repository.go`](./backend/internal/repository/folder_repository.go)
