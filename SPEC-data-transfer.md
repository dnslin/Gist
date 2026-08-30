# Spec: data-transfer

> 状态：Approved（2026-08-30）
> Module ID：`data-transfer`
> 依赖：`connection`、`reader-workspace`
> 来源：[`CAPABILITY-MAP.md`](./CAPABILITY-MAP.md)

## Objective

让 Web 与 Wails 桌面客户端通过现有“设置 → 数据”页面完成 OPML 订阅迁移：选择文件、导入、查看进度、停止导入，以及导出当前订阅与文件夹。

完成本 Spec 后：

- Web 与桌面继续复用同一个 `DataControl` 组件和同一组 OPML API 类型。
- 用户可以选择 `.opml` 或 `.xml` 文件，并把其中的订阅与文件夹导入当前 Gist 服务。
- 页面能够读取当前服务上的导入任务，并在运行期间每秒显示一次最新状态；同一页面可以连续多次导入。
- 用户点击“停止”后，服务端停止后续处理，但保留已经创建的文件夹与订阅。
- 导入中途失败时同样保留已经成功创建的数据，并显示服务端错误。
- Web 继续以浏览器下载方式导出 `gist.opml`。
- 桌面端弹出系统“另存为”窗口，默认文件名为 `gist.opml`，并把服务端导出的内容写入用户选择的位置。

本模块只拥有现有 `DataControl` 中的 OPML 区域。AI 与 Readability cache 由 `content-tools` 验收；文章、图标和 Anubis 清理由 `settings-profile` 验收。

本模块不增加完整数据备份、导入预览、字段映射、冲突选择、导入历史、任务中心、离线副本、定时导入或云端同步。

## Confirmed Decisions

- Web 与桌面统一使用一次性 JSON 状态接口。前端挂载时读取一次，并在任务 `running` 时每秒查询，不保留 Web SSE，也不为桌面建立 Wails Stream。
- 桌面导出使用系统“另存为”窗口，默认文件名是 `gist.opml`。用户取消窗口时不请求导出，也不写文件。
- 用户停止导入或导入中途失败时，已经创建的文件夹与订阅保留，不执行事务回滚或补偿删除。
- 导入是 Gist 服务端后台任务。关闭数据页、退出桌面、退出登录或切换服务只停止当前客户端观察，不主动调用取消接口；当前页面只有明确点击“停止”才调用取消接口。
- 重新打开数据页时立即读取当前服务的任务状态。服务仍在导入时恢复进度；任务已结束时显示最后状态；服务重启后没有内存任务则显示空闲。
- `done` 表示 OPML 中的文件夹与订阅记录已经处理完成。新订阅的正文刷新与图标补全可以继续在服务端后台执行，不把这些网络任务并入 OPML 进度。
- 导入继续使用现有隐藏的 `<input type="file">`。它已经返回浏览器 `File` 并可直接走现有 multipart 上传，不增加原生选文件 binding。
- 服务端继续只有一个当前导入任务；界面在上传中或看到 `running` 时禁用再次选择文件。另一个客户端同时发起时只返回 `409`，不增加队列、多任务列表或客户端 task ownership。
- 导入文件继续以文件内容的 `5 MiB` 为上限：`<= 5 MiB` 可以进入解析，`> 5 MiB` 返回 `413`。multipart 边界字节不计入文件大小。文件扩展名只用于选择器提示，XML/OPML 合法性仍由服务端判断。
- 只修复已经确认的 OPML 状态与错误处理问题，不借机重构整个 API client、设置页或后端任务系统。

## Existing UI Contract

### Entry point and layout

入口继续是现有 `SettingsModal` 的“数据”页签。`DataControl` 内的现有顺序保持不变：

1. OPML 导入。
2. OPML 导出。
3. 各 capability 分别拥有的 cache 清理区域。

不增加独立 OPML 页面、向导、拖放区、历史列表或桌面专属组件副本。现有按钮、进度条、成功卡片、停止卡片和错误卡片继续使用当前 React、Tailwind 与 i18n 样式。

### Select and start import

- “选择文件”继续触发现有隐藏文件输入。
- `accept` 继续是 `.opml,.xml`。
- 没有选择文件或用户取消系统选择器时不发请求，也不显示错误。
- 选择文件后清除上一次本地成功与错误呈现，并以 multipart 字段 `file` 上传。
- 文件一经选择即进入本地 uploading 状态，并取消正在进行的旧状态请求；上传完成前按钮保持禁用，旧响应不得覆盖新任务状态。
- 上传请求成功只表示服务端接受并建立后台任务，不表示导入完成。
- 上传成功或返回 `409` 后，把现有 status Query 暂置为 `{status:"running",total:0,current:0}` 并立即重新读取当前任务。这样首次 refetch 临时失败时仍会继续每秒查询；真实快照到达后直接替换占位状态。`409` 不得取消或覆盖已经运行的任务。
- 上传失败显示服务端错误；`401` 使用 `connection` 已批准的统一未授权清理。
- uploading 或 `running` 时按钮显示现有 loading 状态并禁用再次选择文件。
- 一个任务进入终态后，用户可以在不关闭设置页的情况下选择另一个文件；新任务必须重新开始状态查询。
- 每次上传尝试结束后重置 file input，使用户可以再次选择同一个文件。

前端不读取本地文件内容做第二次 XML 解析，也不在上传前计算 Feed 数量。服务端是导入结果与进度的唯一真源。

### Progress and terminal states

任务状态继续使用现有五种值：

| 状态 | 页面行为 |
| --- | --- |
| `idle` | 可选择文件，不显示进度或旧错误 |
| `running` | 显示当前 Feed、`current/total`、进度条与“停止” |
| `done` | 显示创建与跳过的文件夹、订阅数量 |
| `cancelled` | 显示已停止以及停止时的 `current/total` |
| `error` | 显示服务端错误，不伪装成成功或取消 |

进度满足：

- `total` 使用与真实导入相同的 Feed 判定规则，不再用 `xmlUrl` 文本出现次数近似。
- `current` 表示已经完成创建或跳过处理的 Feed 数量，并保持 `0 <= current <= total`；失败项不能提前计入“已导入”。
- `total = 0` 时不计算百分比；页面仍可显示任务状态。
- 终态保留在服务端内存中，直到下一次导入或服务重启；前端重新进入数据页时可以显示它。

### Polling lifecycle

`GET /api/opml/import/status` 改为返回一次 JSON 快照。前端直接使用仓库已有的 TanStack Query，不手写 timer 状态机：

1. `DataControl` 挂载时立即请求一次。
2. Query data 是 `running`，或最近一次 status 请求失败时，设置约 `1s` 的 `refetchInterval`；有效 `idle` 或终态且没有查询错误时不持续轮询。
3. 本地上传前取消当前 status Query。上传成功或 `409` 后写入最小 running 占位并立即 `refetch`，即使页面先前已经看到终态。
4. status 请求失败时保留旧的 running data 或 Query error，因此临时错误恢复后仍会继续观察；成功快照直接替换占位或旧状态。
5. Query function 使用 TanStack Query 提供的 `AbortSignal`。切换页签、关闭设置、退出、`401` 或更换服务时取消请求；迟到响应不得写入新任务或新服务。
6. 停止观察不调用远端取消接口，服务端任务继续运行。

status Query 的错误与上传/取消操作错误分别保存，避免一次恢复成功错误地清除另一项失败。两者都显示在现有导入错误区域，不增加全局错误中心、指数退避、离线队列或持久化重试器。

### Stop import

- 只有用户点击现有“停止”按钮时才发送 `DELETE /api/opml/import`。
- 返回 `cancelled:true` 时等待下一次状态快照显示 `cancelled`。
- 返回 `cancelled:false` 时重新读取状态；若任务已结束则显示该终态，若服务已回到 idle 则正常回到空闲，不把这个竞态当作错误。
- 请求失败显示错误，并保持当前任务状态；不得假装已经停止。
- 取消不会撤销已经创建的数据。
- `cancelled` 或 `error` 终态都失效已经受部分导入影响的 `folders`、`feeds` 与 `unreadCounts` Query，使保留下来的项目能够出现在共享工作区。

### Import completion and Query state

收到 `done` 终态时：

- 显示 `foldersCreated`、`foldersSkipped`、`feedsCreated` 与 `feedsSkipped`。
- 失效现有 `['folders']`、`['feeds']` 与 `['unreadCounts']` Query。
- 不建立 data-transfer 专用客户端副本或统一 invalidation service。
- 服务端继续在后台刷新新 Feed 与补全图标，但 data-transfer 不新增完成信号，也不接管 `reading` 已拥有的 `useRefreshStatus` 行为。

`cancelled` 或 `error` 不返回虚构的完整 ImportResult。它们只显示真实进度或错误，并失效上述导航 Query，以反映已经保留的部分结果。

## OPML Behavior

### Import semantics

继续保留现有服务端行为：

- 支持根级订阅与嵌套文件夹。
- 文件夹标题优先读取 `title`，否则读取 `text`；空标题使用 `Untitled`。
- Feed 标题优先读取 `title`，否则读取 `text`。
- Feed URL 使用 `xmlUrl`。
- `xmlUrl` 非空，或 `type` 是 `rss`、`atom`、`feed` 时，outline 被视为 Feed。
- 新文件夹使用现有默认 `article` 类型；同层已有同名文件夹时复用它及其现有类型。
- Feed 按现有完整 URL 规则去重。重复 Feed 计入 `feedsSkipped`，不移动到新文件夹，也不覆盖标题。
- 没有可用 URL 的 Feed outline 计入 skipped。
- 不导入 `htmlUrl`、description、Gist 内容类型、摘要提示词或设置。
- 成功完成后，服务端继续异步刷新本次新建 Feed 并补全图标。

若取消或错误发生在中途，已经提交的创建操作保留。这个行为必须在测试和错误文案中明确，不增加数据库事务、导入批次表或补偿删除。

### Export semantics

导出继续生成现有 OPML：

- XML 内容是 OPML `2.0`。
- head title 是 `Gist Subscriptions`。
- `dateCreated` 与 `dateModified` 使用导出时的 UTC 时间。
- 文件夹保留嵌套层级并输出 `text/title`。
- Feed 输出 `text/title/type="rss"/xmlUrl`；有站点 URL 时输出 `htmlUrl`。
- 每层按标题大小写不敏感排序，文件夹排在 Feed 前。
- 找不到父节点的文件夹或 Feed 提升到根节点。
- 不导出文章、已读/收藏状态、缓存、AI 设置、网络设置、用户资料或其他非 OPML 数据。

空订阅库仍导出合法的 OPML 文件，不把“没有订阅”当作错误。

## API Contract

四个接口继续位于当前受认证的 `/api` 路由组：

| Method | Path | 成功响应 | 用途 |
| --- | --- | --- | --- |
| `POST` | `/api/opml/import` | `200 {"status":"started"}` | 接受文件并建立后台任务 |
| `GET` | `/api/opml/import/status` | `200 ImportTask` JSON | 返回一次当前任务快照 |
| `DELETE` | `/api/opml/import` | `200 {"cancelled":boolean}` | 停止当前运行任务 |
| `GET` | `/api/opml/export` | `200 application/xml` | 返回 `gist.opml` 内容 |

`ImportTask` 的共享形状是：

```ts
export interface ImportTask {
  id?: string;
  status: "idle" | "running" | "done" | "error" | "cancelled";
  total: number;
  current: number;
  feed?: string;
  result?: ImportResult;
  error?: string;
  createdAt?: string;
}
```

空闲响应也必须给出稳定数字字段：

```json
{"status":"idle","total":0,"current":0}
```

状态接口设置 `Cache-Control: no-store`，不再设置 `text/event-stream`、不保持连接、不要求 `http.Flusher`，也不保留第二个兼容 SSE endpoint。Web 与桌面在同一次变更中一起迁移到 JSON 快照。

文件上传只接受 multipart `file`，删除没有已确认调用方的原始 XML body 分支。缺文件或 multipart 无效返回 `400`；文件内容 `> 5 MiB` 返回 `413`。空文件或无效 OPML 可以先返回 `started`，但随后必须进入 `error` 终态，不得永远停在 `running`。

`POST` 返回 `started` 前，新的当前任务必须已经可由状态接口读取。这样上传成功后的第一次查询不会因 goroutine 尚未调用 `Start` 而错误地看到旧的 `idle`。若已有任务仍是 `running`，建立任务的同一个原子操作返回 `409 {"error":"import already running"}`，不能隐式取消或替换它。客户端重新读取当前状态并继续显示原任务。

这仍然只是一个当前任务：不新增队列、历史表、客户端 task ownership 或按 task ID 取消。`ImportTaskService.Start` 的运行中检查与任务建立必须在同一锁内完成，避免两个客户端同时通过检查。

`401` 继续触发 `connection` 的统一未授权清理。其他非成功响应转换成现有 `ApiError` 并保留服务端错误文本。不得为 OPML 建立第二套认证或连接逻辑。

## Transport Contract

`DataControl` 继续只调用公开的 `exportOPML(): Promise<void>`。Web/desktop 选择留在 API 边界，并复用已批准的 `VITE_DESKTOP` 构建标记；组件 JSX 不判断运行环境。两端共享一个 authenticated `getOPMLExport(): Promise<string>` 获取 XML。

### Web and desktop import/status/cancel

以下请求在 Web 使用现有 `VITE_API_URL`，在桌面使用 `connection` 的相对 `/api` 同源代理：

- multipart import。
- JSON status snapshot。
- JSON cancel response。

它们都是有限请求/响应，属于 `connection` 已批准的代理范围。移除 SSE 后，data-transfer 不注册 `HandleStream`、不使用 `JSONStream`，也不启动 localhost server。

### Web export

Web 保持现有有限响应下载：

1. 使用现有 authenticated `request<string>` 获取 `/api/opml/export` XML；该 helper 已负责统一 `401`。
2. 从 XML 字符串创建 `application/xml` Blob。
3. 创建临时 object URL。
4. 使用 `download="gist.opml"` 的临时 `<a>` 触发浏览器下载。
5. 移除元素并释放 object URL。

失败显示在导出区域，不再静默吞掉；`401` 继续统一退出。

### Desktop export

桌面端不能依赖 WebView 自动下载位置。点击现有“导出”按钮后：

1. 调用 Wails `Dialogs.SaveFile`，默认文件名 `gist.opml`。
2. 用户取消时结束，不请求远端、不写文件、不显示错误。
3. 用户选择路径后，通过与 Web 相同的 authenticated `request<string>` 和现有 `/api` 代理取得 XML；远端 `401` 与其他 HTTP 错误继续走统一 API 处理。
4. XML 获取成功后，调用具体的 `DataTransferService.SaveOPML(path, xml)` binding。
5. Go 只把收到的 XML 字节写入用户选择的路径；写文件失败交回现有导出错误区域，不显示成功。

这个 binding 不读取 connection、不发 HTTP，也不接受 URL、method、header 或 Token。它只解决浏览器下载在桌面 WebView 中没有确定保存位置这一真实差异，不建立通用下载器或文件系统 service。

保存窗口负责覆盖确认。实现不增加路径 allowlist、扩展名强制改写、临时文件封装、hash、签名或下载记录。默认文件名是提示，不修改用户最终选择的路径。

## Tech Stack

继续使用仓库已有依赖：

| 层 | 现有技术 |
| --- | --- |
| Shared frontend | React、现有 `DataControl`、i18next、TanStack Query |
| Web/desktop finite transport | `fetch` 与 `connection` 的 Wails 同源代理 |
| Desktop save dialog | `@wailsio/runtime` `3.0.0-beta.12` `Dialogs.SaveFile` |
| Desktop export host | Wails `v3.0.0-beta.12` binding、Go `os.WriteFile` |
| Backend | Echo、现有 `OPMLService`、`ImportTaskService` 与 repositories |
| OPML codec | 现有 `backend/pkg/opml` XML parser/encoder |
| Tests | Vitest、Testing Library、Go `testing`、testify、GoMock、`httptest` |

本模块不增加生产依赖、数据库 migration、前端请求库、XML 库、任务队列、Worker、状态管理库、下载库或原生文件系统插件。

## Commands

以下命令是本模块实施后必须成立的验证契约。

### Targeted frontend tests

```bash
cd frontend
bun run test -- \
  src/api/data-transfer.test.ts \
  src/desktop/data-transfer.test.ts \
  src/components/settings/tabs/DataControl.data-transfer.test.tsx
```

### Targeted backend tests

```bash
cd backend
go test ./internal/handler ./internal/service ./pkg/opml
```

### Targeted desktop tests

```bash
cd desktop
go test ./...
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

手工测试使用一台已初始化且可登录的 Gist 服务，以及小型、重复项、嵌套文件夹、无效 XML 和接近 `5 MiB` 边界的确定性 fixture。自动测试不得依赖真实公网 Feed。

## Project Structure

只修改实现所需位置；测试继续与对应行为相邻：

```text
desktop/
├── main.go                                  # 注册具体 DataTransferService
├── data_transfer.go                         # 把已获取的 OPML 写入用户路径
└── data_transfer_test.go

frontend/src/
├── api/
│   ├── index.ts                             # import/status/cancel/Web export
│   └── data-transfer.test.ts
├── components/settings/tabs/
│   ├── DataControl.tsx                      # 继续承载原有 OPML UI
│   └── DataControl.data-transfer.test.tsx
├── desktop/
│   ├── data-transfer.ts                     # SaveFile + generated binding adapter
│   └── data-transfer.test.ts
└── types/
    └── api.ts                               # ImportTask / ImportResult

backend/
├── internal/handler/
│   ├── opml_handler.go                      # 有限 JSON status 与现有 endpoints
│   ├── opml_handler_test.go
│   └── response.go                          # idle JSON response
├── internal/service/
│   ├── import_task.go
│   ├── import_task_test.go
│   ├── opml_service.go
│   └── opml_service_test.go
└── pkg/opml/
    ├── opml.go
    └── opml_test.go

frontend/public/locales/{en,zh}/common.json  # 复用区域所需错误文案
```

`DataControl.tsx`、`frontend/src/api/index.ts` 与 `desktop/main.go` 同时被其他 capability 使用。后续任务拆分应按依赖协调这些文件，但不得为了避免文件重叠复制组件、创建 capability registry 或拆出通用 transport framework。

## Code Style

### TypeScript and React

- 沿用 strict TypeScript、`@/` 别名、现有 i18n 与组件本地状态。
- 状态快照使用一个具体的 `['opmlImportStatus']` TanStack Query；data 是 `running` 或 Query 有 status error 时设置 `refetchInterval`。
- Query function 使用 TanStack Query 提供的 signal。上传前取消旧 status Query，成功后主动 refetch。
- uploading、status error、import/cancel error 与 export error 各自只保留所需的局部状态，不建立统一错误 store。
- 非 Abort 错误显示在对应 import/export 区域，不静默忽略。
- Web/桌面差异只留在导出 API 边界，不在 `DataControl` JSX 中散布运行模式判断。

```ts
const statusQuery = useQuery({
  queryKey: ["opmlImportStatus"],
  queryFn: ({ signal }) => getImportStatus(signal),
  staleTime: 0,
  refetchInterval: (query) =>
    query.state.data?.status === "running" || query.state.error
      ? 1_000
      : false,
});
```

桌面导出 adapter 保持具体：

```ts
const path = await Dialogs.SaveFile({ Filename: "gist.opml" });
if (!path) return;

const xml = await getOPMLExport();
await SaveOPML(path, xml);
```

### Go

- 使用 `gofmt`，保持现有 Echo handler → service → repository 分层。
- 状态 handler 只读取 `ImportTaskService.Get()` 并返回一个有限 JSON 对象。
- idle 也返回 `total:0`、`current:0`，不让前端猜缺失字段。
- 进度总数与真实 Feed 判定使用同一规则；不再扫描原始 XML 字符串估算。
- 桌面导出 service 是具体 struct，不为一个调用点建立 interface、factory、download manager 或 transport registry。
- 桌面 `SaveOPML` 只使用 `os.WriteFile` 保存已经由认证 API helper 获取的 XML，不读取 connection 或重复发 HTTP。
- 日志可记录 task ID、状态、计数和保存错误；不得记录 Token 或完整 OPML 内容。
- 不手工编辑生成的 Wails bindings 或 GoMock 文件。

## Testing Strategy

### Frontend API and component tests

至少覆盖：

- 挂载数据页立即读取 idle、running 与各终态。
- running 或 status error 时每约一秒继续查询；取得有效 idle 或终态后不永久轮询。
- 状态接口改为 JSON 后不再创建 SSE reader、`EventSource` 或 Wails Stream。
- 组件卸载、退出、`401` 或切换服务时取消 status Query，迟到结果不写入状态。
- 第一次任务结束后不关闭页面，选择第二个文件会立即重新查询并正确显示第二个任务。
- multipart 字段名是 `file`，取消文件选择不发请求。
- 文件选择后立即进入 uploading 并禁用按钮；running 时继续禁用。上传前取消旧 status request，旧 idle 不覆盖新任务。
- POST 成功或 `409` 后先写入最小 running 占位；首个 status refetch 即使临时失败，也会继续每秒查询并最终由真实快照替换。
- running 且 `total=0` 时仍显示运行与停止，不计算 `NaN`、`Infinity` 或百分比。
- done 显示结果，并失效 `folders`、`feeds`、`unreadCounts`。
- cancelled/error 保留部分结果语义并失效相同 Query，不伪造 ImportResult。
- 取消 true、取消 false、取消请求失败分别刷新或显示正确状态；false + idle 不是错误。
- status error 与 import/cancel error 独立；其中一项恢复不会错误清除另一项。
- status 网络错误与上传错误不再被吞掉；恢复后可以继续显示任务。
- import/status/cancel 的 `401` 复用统一未授权清理。
- Web export 通过 authenticated text request 获取 XML，生成并释放 object URL，下载名为 `gist.opml`；失败显示错误。
- 桌面导出调用 SaveFile；取消不请求 XML、不调用 binding；选择路径后通过相同 authenticated request 获取 XML，再调用 `SaveOPML(path, xml)`。
- 桌面导出的远端 `401`、其他 HTTP 错误和本地写文件错误分别得到正确页面状态。

### Backend handler and service tests

至少覆盖：

- status idle 返回 JSON、`Cache-Control: no-store` 与稳定的 `total/current = 0`，不再返回 SSE headers。
- running、done、error、cancelled 返回完整可序列化快照。
- import 成功响应返回前，status 已能看到新的当前任务。
- 已有 running 任务时第二个 import 原子返回 `409`，不取消或替换旧任务。
- 进度 total 与实际 Feed 判定一致，`current` 只在创建或跳过完成后增加，失败项不提前计数且 current 不超过 total。
- 只接受 multipart `file`；文件内容恰好 `5 MiB` 可进入解析，超过一个字节返回 `413`，multipart framing 不计入限制。
- 空文件与无效 XML 在 accepted 后进入 error 终态，不停在 running。
- cancel 在 running 时返回 true 并取消 context；无运行任务或终态时返回 false。
- 取消与中途错误保留已经创建的文件夹和 Feed，不执行回滚。
- 嵌套文件夹、`Untitled`、已有文件夹、重复 Feed、缺 URL Feed 与成功计数。
- 成功导入后只对新 Feed 触发现有后台 refresh/icon 逻辑；done 不等待它们完成。
- 取消在最后一个 Feed 附近发生时再次检查 context，不启动本应停止的后续 refresh/icon。
- export 的 OPML 2.0、嵌套层级、排序、孤儿提升、`htmlUrl` 与空库。
- 四个 endpoint 继续位于认证路由；非认证请求保持现有 `401`。

本模块新增的确定性测试不依赖个人电脑上的固定 OPML 路径、真实公网 Feed 或 `time.Sleep`。

### Desktop host tests

使用临时目录覆盖：

- `SaveOPML(path, xml)` 把 UTF-8 XML 字节原样写入用户路径。
- 本地写入失败返回原始可诊断错误，不伪装成功。
- service 不读取 connection、不发 HTTP，也不注册 OPML stream、通用下载 service 或额外文件 API。

自动测试 mock `Dialogs.SaveFile`；不自动操纵真实系统窗口。

### Manual regression

在 Web 与当前开发平台桌面完成完整流程：

1. 导入包含根级 Feed、嵌套文件夹和重复 Feed 的 OPML。
2. 查看进度，完成后看到创建/跳过计数以及更新后的侧栏。
3. 不关闭设置页再次导入，确认第二次进度正常出现。
4. 导入中关闭并重新打开数据页，确认任务继续且进度恢复。
5. 点击停止，确认后续处理停止、已创建项保留并出现在界面。
6. 导入无效 XML，确认任务进入 error 且可以再次选择文件。
7. 模拟 status、cancel 与 export 错误，确认错误可见且不会伪装成功。
8. Web 导出并重新解析 `gist.opml`。
9. 桌面导出时确认系统“另存为”、取消行为、默认文件名和最终 XML 内容。
10. 退出或切换服务时旧状态请求停止；原服务端任务没有被客户端自动取消。

Windows、macOS 和 Linux 各自只需完成短冒烟：选择一个小型 OPML、看到至少一次状态更新、停止或完成、使用系统“另存为”导出。完整异常矩阵不在三个平台重复执行。

不要求截图测试、性能压测、大文件基准、多客户端并发矩阵、真实公网 Feed、安装包或签名验证。

## Boundaries

### Always

- 复用现有 `DataControl` 与 OPML UI，不创建桌面副本。
- Web 与桌面共用 JSON 状态快照和业务状态。
- 导入、状态和取消继续走 `connection` 的普通认证代理。
- 客户端关闭页面、退出或切换服务时不主动取消；运行中新的 POST 返回 `409`，不隐式替换任务。
- 取消或失败保留已创建项，并刷新受影响的现有 Query。
- 桌面保存路径由系统“另存为”返回；用户取消不写文件。
- Web 与桌面都通过现有代理访问当前服务的固定 OPML endpoint；桌面 binding 只写入已获取的 XML。
- `401` 使用统一会话清理，其他错误保留可诊断消息。
- 运行相关定向测试、完整前后端回归和当前平台桌面构建后才能提交实现。

### Ask First

- 恢复 SSE、增加 Wails Stream、WebSocket、本地 HTTP server 或其他进度 transport。
- 改变 `5 MiB` 文件上限、OPML 字段映射、去重规则、文件夹合并规则或导出格式。
- 改成关闭页面/退出应用/切换服务时自动取消远端导入。
- 改成取消或错误时回滚已经创建的数据。
- 支持多个并发任务、导入队列、任务历史或按 task ID 取消。
- 增加导入预览、冲突选择、拖放、目录导入或完整数据备份。
- 新增依赖、数据库表、migration、CI 或发布流程。

### Never

- 把 OPML 进度塞进 `content-tools` Stream 或建立通用流/HTTP transport。
- 复制 `DataControl`、API client 或 OPML 业务逻辑到桌面目录。
- 向保存 binding 传入远端 URL、Token、任意 header，或增加读、删、列目录等通用文件操作。
- 为用户选择的保存路径增加没有真实威胁目标的 allowlist、hash、签名或安全包装层。
- 静默吞掉 import、status、cancel 或 export 的非 Abort 错误。
- 把 `done` 错误解释为正文与图标已经全部刷新完成。
- 在 data-transfer 中实现 Feed/Folder CRUD、AI/cache 清理、用户设置、离线或多连接。
- 为未来格式或云服务建立 adapter registry、插件系统、repository 层或任务框架。

## Success Criteria

- [ ] Web 与桌面继续使用同一个 `DataControl` OPML 区域，没有新增页面或 UI 副本。
- [ ] 状态 endpoint 返回有限 JSON；Web/桌面不再使用 OPML SSE 或 Wails Stream。
- [ ] 页面挂载后读取当前状态；running 或 status error 时每秒刷新，取得有效 idle/终态后停止，卸载时正确取消 Query。
- [ ] 同一页面连续两次导入都能显示各自进度和终态。
- [ ] uploading 与 running 都禁用重复选择；POST 成功或 `409` 后的 running 占位保证临时 status 错误不会停止观察。
- [ ] 运行中第二个 POST 返回 `409`，不取消旧任务。
- [ ] 进度 total 与真实导入判定一致，current 只统计已完成处理且不超过 total。
- [ ] 点击“停止”调用取消；关闭页面、退出或切换服务只停止观察。
- [ ] 取消或失败保留已创建项，并让 folders/feeds/unreadCounts 反映部分结果。
- [ ] done 显示四项真实计数；后台 Feed 刷新与 reading 的刷新观察不被并入本模块。
- [ ] import/status/cancel/export 的错误可见；`401` 复用 connection 的统一清理。
- [ ] Web 下载文件名是 `gist.opml`，并正确释放 object URL。
- [ ] 桌面弹出系统“另存为”；取消不请求、不写文件；成功写入用户路径。
- [ ] 桌面通过现有代理读取固定 `/api/opml/export`，`SaveOPML` 不重复连接逻辑，也没有通用文件或下载 binding。
- [ ] 导入和导出保持现有 OPML 层级、去重、排序与字段语义。
- [ ] 未新增生产依赖、database migration、任务中心、队列、历史、备份、离线或安全包装层。
- [ ] 定向测试、完整前后端验证、前端 Web/desktop build 与当前平台 Wails build 全部通过。
- [ ] Windows、macOS 和 Linux 的短冒烟覆盖选择、进度和原生保存。

## Open Questions

无。本文件已经批准。全部 capability Spec 已完成；按照用户要求，暂不进入任务拆分。

## References

- [`CAPABILITY-MAP.md`](./CAPABILITY-MAP.md)
- [`SPEC-connection.md`](./SPEC-connection.md)
- [`SPEC-reader-workspace.md`](./SPEC-reader-workspace.md)
- [`SPEC-library.md`](./SPEC-library.md)
- [`SPEC-content-tools.md`](./SPEC-content-tools.md)
- [`SPEC-settings-profile.md`](./SPEC-settings-profile.md)
- [`frontend/src/components/settings/tabs/DataControl.tsx`](./frontend/src/components/settings/tabs/DataControl.tsx)
- [`frontend/src/api/index.ts`](./frontend/src/api/index.ts)
- [`frontend/src/types/api.ts`](./frontend/src/types/api.ts)
- [`backend/internal/handler/opml_handler.go`](./backend/internal/handler/opml_handler.go)
- [`backend/internal/service/import_task.go`](./backend/internal/service/import_task.go)
- [`backend/internal/service/opml_service.go`](./backend/internal/service/opml_service.go)
- [`backend/pkg/opml/opml.go`](./backend/pkg/opml/opml.go)
- [Wails beta.12 Windows asset response writer](https://github.com/wailsapp/wails/blob/v3.0.0-beta.12/v3/internal/assetserver/webview/responsewriter_windows.go)
- [Wails beta.12 file dialog guide](https://github.com/wailsapp/wails/blob/v3.0.0-beta.12/docs/src/content/docs/features/dialogs/file.mdx)
- [Wails beta.12 Dialogs runtime source](https://github.com/wailsapp/wails/blob/v3.0.0-beta.12/v3/internal/runtime/desktop/@wailsio/runtime/src/dialogs.ts)
