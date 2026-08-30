# Spec: content-tools

> 状态：Approved（2026-08-30）
> Module ID：`content-tools`
> 依赖：`connection`、`reader-workspace`
> 来源：[`CAPABILITY-MAP.md`](./CAPABILITY-MAP.md)

## Objective

让桌面客户端通过 `connection` 已建立的 Gist 服务连接，完整复用现有正文提取与 AI 内容工具。

完成本 Spec 后，Web 与桌面端都应当能够：

- 手动或按现有设置自动获取 Readability 正文，并在 RSS 正文与提取正文之间切换。
- 手动或按现有设置自动生成 AI 摘要，并在生成过程中逐步显示内容。
- 手动或按现有设置翻译文章标题与正文，并在翻译过程中逐块显示正文。
- 在自动翻译开启时，按现有可见范围批量翻译列表标题与摘要。
- 取消正在执行的摘要或翻译，并在切换文章、列表或连接时终止不再需要的请求。
- 使用和清除现有服务端 Readability、摘要、正文翻译与列表翻译缓存。

桌面端使用 Wails `v3.0.0-beta.12` Streams 保留现有逐步显示和取消行为。Web 继续使用现有 HTTP `fetch`。两端共享同一组 React Hooks、组件、状态和业务规则。

本模块不重新设计工具栏或正文，不编辑 AI/通用设置，不内嵌 AI Provider，不在桌面端执行 Readability，不增加本地模型、离线缓存、任务中心、通用流代理或新的通知系统。

## Confirmed Decisions

- 摘要被取消时丢弃本次不完整内容并回到无摘要状态，不显示错误。
- 摘要生成失败时丢弃本次不完整内容，使用现有正文区域显示一条可重试错误；再次点击摘要按钮即重试。
- 单篇翻译被取消时丢弃本次标题和正文译文，立即恢复完整原文，不显示错误。
- 单篇翻译任一部分失败时丢弃本次标题和正文译文，恢复完整原文并显示一条可重试错误；不把“部分译文 + 部分原文”当成完成结果。
- 上述“全部回退”只适用于一份摘要或一篇详情翻译。自动列表 batch 中每篇文章是独立结果，成功文章不因其他文章失败而回退。
- 手动点击“翻译文章”同时翻译详情标题和当前正文模式，不再只翻译正文。
- 翻译完成后点击“显示原文”，标题与正文一起恢复；该选择在当前已认证服务的本次应用会话内对该文章持续有效，用户再次主动点击翻译才恢复译文。
- “显示原文”只在当前已认证服务的本次应用会话内有效，不写入 `localStorage` 或服务端；退出、`401`、更换服务或重启应用后清除，并重新遵守当前自动翻译设置。
- 自动列表翻译仍只翻译可见项与当前选中项的标题和摘要，不新增批量翻译按钮。
- 列表批量翻译以文章为独立结果：已完成文章保留译文，失败文章保留原文并允许后续重试，不把整批改成事务。
- 自动 Readability、自动摘要和自动翻译同时开启时，先等待 Readability 成功或失败，再只对最终显示正文执行一次摘要与翻译。
- 桌面端保留逐步显示；使用一个只接受 `summary`、`translate`、`batch` 三种枚举操作的 Wails Stream，不建立任意 URL、任意 path 或通用协议转发器。
- AI 与 Readability 继续完全在 Gist 服务端执行。桌面端不读取 AI API Key，也不直接连接 OpenAI、Anthropic 或兼容 Provider。
- AI Provider、API Key、Base URL、模型、请求参数、速率、目标语言与自动开关的编辑属于 `settings-profile`；本模块只读取并执行当前设置。
- 每次 AI 操作在服务端开始时固定一次目标语言；cache lookup、prompt、生成和 cache save 必须使用同一个值，不在完成保存时重新读取可能已经变化的设置。
- 前端观察到目标语言变化时取消仍在执行的摘要、详情翻译和列表 batch，清除旧语言的当前呈现，再由之后的操作使用新语言；设置保存后及时更新 Query 的责任仍属于 `settings-profile`。
- Feed 的 `summaryPromptReminder` 由 `library` 编辑和保存，由现有后端摘要服务在缓存未命中时使用。
- Provider、模型、请求参数或摘要提示词变化不自动改写已有缓存。用户继续通过现有数据控制入口手动清除缓存；不增加 cache version、自动级联或后台重算。

## Existing Content Tools Contract

### Readability

Readability 保持现有入口和服务端执行位置：

- 文章存在原文 URL 时，正文工具栏显示现有 Readability 按钮；继续由服务端判断 URL 是否可抓取，不在前端重复增加“安全 URL”规则。
- 已有 `entry.readableContent` 时，点击按钮只在缓存正文与 RSS 正文间切换，不发请求。
- 尚无缓存时，点击按钮调用 `POST /api/entries/:id/fetch-readable`；成功后显示提取正文，失败后保留 RSS 正文并显示现有范围内的错误。
- `general.autoReadability` 开启时，选择文章后自动使用缓存；没有缓存且存在 URL 时自动抓取一次。
- 每篇文章每次挂载只自动尝试一次。自动失败后不因 React effect 重跑而立即重复请求；用户仍可点击按钮手动重试。
- 切换文章、卸载详情页、退出或更换服务时，取消请求或忽略迟到结果。旧文章的结果不得覆盖当前文章。
- 没有 URL 时不发请求，继续显示原有正文。服务端抓取后发现没有可提取内容时按失败处理，保留 RSS 正文并显示现有错误状态。

Gist 服务端继续负责：

1. 从 `entries.readable_content` 读取缓存。
2. 缓存未命中时使用现有网络设置抓取条目的 HTTP/HTTPS URL。
3. 使用现有 Readability 解析、lazy image 修正和 metadata 清理。
4. 成功后把 HTML 写回 `entries.readable_content`。

桌面 WebView 不直接访问文章站点，也不复制服务端 Readability、网络代理、Anubis 或 HTML 处理逻辑。

### AI summary

摘要始终针对当前正文模式：

- Readability 未启用时使用 `entry.content`。
- Readability 已启用时使用当前 `readableContent`，并发送 `isReadability: true`。
- 目标语言继续读取 `ai.summary_language`，缺少设置时使用现有默认值 `zh-CN`。
- Feed 的 `summaryPromptReminder` 由后端根据 entry 与 feed 关系读取，前端不把该字段复制进请求。
- 缓存命中时一次显示完整摘要；缓存未命中时逐段追加到现有 `AiSummaryBox`。
- 生成期间按钮保持现有取消语义；完成后按钮保持现有隐藏摘要语义。
- 手动隐藏完整摘要只影响当前挂载状态，不删除服务端缓存。
- `ai.autoSummary` 开启时，每篇文章对最终正文模式自动尝试一次；失败后等待用户手动重试，不进入自动重试循环。
- 正文模式在用户已经请求或显示摘要后发生变化时，取消旧请求，并读取或生成新模式对应的摘要。

为了可靠表达用户已确认的失败语义，未缓存摘要的响应统一为结构化 SSE，而不是当前“`text/event-stream` Content-Type + 裸文本正文”的混合格式：

```text
data: {"delta":"第一段增量"}

data: {"delta":"第二段增量"}

data: {"done":true}

```

失败终态为：

```text
data: {"error":"provider error"}

```

客户端只在收到 `done` 后把本次结果视为完成。`error`、连接中断、协议损坏或缺少终态都丢弃本次增量并显示错误。取消不显示错误。服务端只在正常完成且内容非空时保存摘要；失败或取消不得保存不完整缓存。

缓存命中继续返回现有 JSON：

```json
{"summary":"完整摘要","cached":true}
```

### Article translation

单篇翻译针对当前正文模式，并保持现有 HTML 分块规则：

- 普通模式使用 `entry.content` 与 `isReadability: false`。
- Readability 模式使用当前 `readableContent` 与 `isReadability: true`。
- 代码、`pre`、数学内容和媒体继续使用现有不翻译或占位恢复规则。
- 缓存未命中时先接收所有原始块，再按完成顺序逐块替换；未完成块在加载期间继续显示原文。
- 缓存命中时一次显示完整译文。
- 当前内容已经是目标语言时保持现有禁用按钮和提示，不发送 AI 请求。

详情页的一次“翻译文章”操作包含：

1. 使用现有批量翻译接口请求当前文章的标题与列表摘要。
2. 使用现有正文翻译接口请求当前普通或 Readability 正文。
3. 在加载期间允许逐步显示已经成功返回的内容。
4. 两部分都正常完成后，标题和正文共同进入完成状态。
5. 任一部分失败、流缺少完成终态或协议无法解析时，终止仍在执行的同次请求，回到完整原文并显示一条错误。

标题或正文为空时只处理实际存在的部分；两者都为空时不发 AI 请求。Batch 正常结束但没有返回当前 entry 所需的标题结果，也属于详情标题部分失败。即使标题先写入共享 translation store，详情页也必须由本次组合操作的完成状态控制其显示；正文随后失败时，当前页面和重新选择该文章后都不得泄漏这次未完成操作的标题译文。

这里复用现有三个服务端缓存，不增加“详情翻译事务”或新的组合 endpoint。原子性只属于当前详情页的呈现状态；已经由服务端成功写入的标题、摘要或正文缓存不做补偿删除，下次重试可以直接命中它们。

正文翻译 SSE 保持现有事件形状：

```text
data: {"blocks":[{"index":0,"html":"<p>...</p>","needTranslate":true}]}

data: {"index":0,"html":"<p>译文</p>"}

data: {"done":true}

```

错误事件继续使用 `{"error":"..."}`，但前端不得再忽略它。收到第一个错误后，详情操作按已确认规则回退并关闭当前流。只有所有需要翻译的块成功且请求未取消时，后端才保存完整正文翻译缓存。

点击“显示原文”时：

- 清除当前详情页显示的标题与正文译文，立即显示原文。
- 使用现有会话内 translation store 记录该 entry 已被用户关闭自动翻译。
- 重新选择同一 entry 时读取该记录，不再次自动翻译。
- 用户再次点击“翻译文章”时清除该记录，并优先使用已有服务端或会话缓存。
- 切换普通/Readability 模式时继续尊重“显示原文”，不得因模式变化重新自动翻译。重新启用翻译后，两种模式仍分别使用现有按模式缓存，不把一种模式的正文译文当成另一种模式的结果。

### List batch translation

列表批量翻译保持现有自动行为：

- 仅在 `ai.autoTranslate` 开启且列表处于活动状态时运行。
- 只调度进入可见区域及当前选中的文章；没有 `IntersectionObserver` 时保持现有前 `20` 条回退范围。
- 使用现有 `500ms` 合并窗口，并遵守服务端每批最多 `100` 篇的限制。
- 在请求前使用现有语言检测；已经是目标语言的条目不发送。
- 请求只包含字符串 entry ID、标题和现有最多 `200` 个纯文本字符的列表摘要。
- NDJSON 每返回一篇就更新该篇的会话缓存，结果顺序不作保证。
- 切换列表 selection（all/feed/folder/starred）、内容类型、卸载列表、退出或更换服务时，取消该列表拥有的 batch 请求。
- 一个列表 batch 的取消不得取消详情页标题或正文翻译；详情页取消也不得取消不相关的列表 batch。
- 已成功文章保留译文；没有结果或失败的文章继续显示原文，并在之后新的可见性会话中允许重试。
- 失败不得触发紧密自动重试，也不新增列表级弹窗、toast、进度条或任务队列。

列表中的“标题/摘要翻译”与详情中的“标题/正文翻译”共享服务端 list translation cache，但拥有各自具体的 AbortController。现有单一全局 `batchAbortController` 必须收窄到调用方所有权；不得用通用任务管理器替代。

### Auto execution order

自动 Readability、自动摘要和自动翻译同时开启时使用以下固定顺序：

1. 读取选中 entry 与当前设置。
2. 若自动 Readability 开启，先使用已有缓存；没有缓存时等待本次抓取成功或失败。
3. Readability 成功时以提取正文作为最终模式；失败、没有 URL 或无法运行时以 RSS 正文作为最终模式。
4. 对最终模式各自启动一次自动摘要与自动正文翻译。

等待期间不先对 RSS 正文调用 AI。摘要和翻译在第 4 步可以并行；不建立统一队列。手动按钮不等待其他工具完成，每个工具只取消自己拥有的请求。

切换 entry 或正文模式时，详情 Hooks 只清理详情拥有的 Readability、摘要、标题与正文请求，不取消当前列表仍拥有的 batch。退出、`401` 或更换服务时取消全部 content-tools 请求，清空 translation store 的译文数据与 disabled 集合。迟到的 chunk、block、batch item 或 Readability 响应不得写入新的 entry、模式、用户或连接。

## API Contract

本模块使用以下现有服务端接口，并只对未缓存摘要的流事件作上述结构化修正：

| Method | Path | 响应 | 用途 |
| --- | --- | --- | --- |
| `POST` | `/api/entries/:id/fetch-readable` | JSON | 读取或生成 Readability 正文 |
| `POST` | `/api/ai/summarize` | 缓存 JSON；未缓存 SSE | 读取或生成当前模式摘要 |
| `POST` | `/api/ai/translate` | 缓存 JSON；未缓存 SSE | 读取或生成当前模式正文译文 |
| `POST` | `/api/ai/translate/batch` | NDJSON | 翻译列表标题与摘要 |
| `DELETE` | `/api/ai/cache` | JSON counts | 清除摘要、正文和列表翻译缓存 |
| `DELETE` | `/api/entries/readability-cache` | JSON count | 清除 Readability 缓存 |

现有请求字段保持不变：

```ts
interface SummarizeRequest {
  entryId: string;
  content: string;
  title?: string;
  isReadability?: boolean;
}

interface TranslateRequest {
  entryId: string;
  content: string;
  title?: string;
  isReadability?: boolean;
}

interface BatchTranslateArticle {
  id: string;
  title: string;
  summary: string;
}
```

Snowflake ID 继续使用字符串。目标语言、Provider、模型、请求参数和 Feed 摘要提示词继续由服务端设置与数据关系决定，不加入这些请求体。

普通 JSON Readability 与 cache-clear 请求继续通过 `connection` 的同源 asset middleware。AI 三种流在 Web 使用现有同源 HTTP，在桌面使用下一节的 Wails Stream；不得让同一次桌面 AI 请求同时走两条 transport。

`401` 继续调用 `connection` 已批准的统一未授权清理。其他开始前的非成功状态转换成现有 `ApiError`，保留状态与服务端错误文本。流开始后的错误通过明确终态交给对应 Hook，不能伪装成正常 EOF。

## Desktop AI Stream Contract

### Why a concrete stream is required

Wails `v3.0.0-beta.12` 的 Windows asset response writer 会缓冲完整响应，直到请求结束。`connection` 因此只承诺有限响应，并明确把 AI 三种流留给本 Spec。

桌面端使用 beta.12 已有的 `application.HandleStream` 与前端 `JSONStream`。关闭 `JSONStream` 会取消 Go stream context；Go 必须把该 context 传给发往当前 Gist 服务的 `http.Request`，从而实际终止上游摘要、翻译或批量请求。

### Narrow operation mapping

`desktop/main.go` 注册一个固定名称的 stream，例如：

```go
app.HandleStream("content-tools.ai", contentToolsStream.Handle)
```

客户端连接后发送的第一条 JSON 只包含：

```ts
type ContentToolsStreamRequest =
  | { operation: "summary"; body: SummarizeRequest }
  | { operation: "translate"; body: TranslateRequest }
  | { operation: "batch"; body: { articles: BatchTranslateArticle[] } };
```

Go 使用固定映射，并把 path 追加到 `ConnectionService` 当前 target 的可选路径前缀之后：

| operation | Method | 上游 path |
| --- | --- | --- |
| `summary` | `POST` | `/api/ai/summarize` |
| `translate` | `POST` | `/api/ai/translate` |
| `batch` | `POST` | `/api/ai/translate/batch` |

例如当前规范化服务地址是 `https://host/gist` 时，summary 必须请求 `https://host/gist/api/ai/summarize`，不能丢掉 `/gist` 后请求 origin 根路径。

stream 请求不接收 URL、path、method、header 或 Token。Go 从同一个 `ConnectionService` 获取当前目标与 Token 快照，设置 `Content-Type: application/json`，并使用与同源代理一致的 Bearer 规则。这里是对三个已知流的必要适配，不扩展 `ConnectionService` 为通用 HTTP client。

每次 API generator 调用独占一个新的 `JSONStream`：连接后只发送一条 request，正常结束、错误或取消后立即关闭。不复用长连接，不增加 request ID，也不在一个 socket 上多路复用操作。

### Stream messages

Go 先发送响应头结果，再逐条发送完整的 SSE record 或 NDJSON line：

```ts
type ContentToolsStreamMessage =
  | { type: "response"; status: number; contentType: string }
  | { type: "record"; data: string }
  | { type: "end" }
  | { type: "error"; status?: number; message: string };
```

规则：

- 上游 `2xx` 时，`response` 是第一条服务端消息；上游非 `2xx` 时直接发送一个带 status/message 的 `error` 并结束，不先伪造成功 response。
- 缓存 JSON 作为一个完整 `record` 发送，随后发送 `end`。
- SSE 按完整 event record 发送；NDJSON 按完整非空行发送。不得依赖 HTTP chunk 与 UTF-8 字符边界重合。
- Go 在上游正常 EOF 时发送 `end`；前端公开 generator 仍须看到 summary/translation 的业务 `done` 才能判定成功。Batch 的正常 EOF 本身就是该批终态。
- 上游非成功状态或读取失败使用 `error`；`401` 还触发现有统一未授权清理。Go 只处理 HTTP 状态和 SSE/NDJSON 记录边界，业务 JSON 的 done/error/malformed/missing-done 只由前端 generator 判定。
- 客户端取消时直接关闭 socket。取消不需要先发送另一个 Cancel binding，也不显示错误。
- 切换服务的前端流程先 Abort content-tools Hooks 并关闭 JSONStream，再执行 `ConnectionService.Clear()`；不假设 Clear 能取消已经取得 target 快照的请求，也不为此增加 Go stream registry。
- Go handler 在 socket 关闭或应用退出后停止读取并取消上游 request context。
- 消息只承载当前操作所需的响应数据，不发送连接地址、Token、Provider Key 或诊断对象。

前端现有 `streamSummary`、`streamTranslateBlocks` 与 `streamBatchTranslate` 保持公开签名。桌面构建时它们分别委托给 `frontend/src/desktop/content-tools-stream.ts`；Web 构建继续执行现有 fetch。这个选择只存在于 API 边界，不把 `isDesktop` 分支散布到 Hooks 或组件。

桌面 Vite 配置把 build-time `VITE_DESKTOP` 设为字符串 `"true"`，API 边界只检查 `import.meta.env.VITE_DESKTOP === "true"`；Web 构建不设置该值。它只用于选择这三个已确认的流实现，不建立运行时 capability registry 或可替换 transport interface。

### Cache and error responses

缓存响应仍由前端公开 generator 转成现有值：

- Summary：`{cached:true, summary}`。
- Article translation：`{cached:true, content}`。
- Batch：每条 `{id,title,summary,cached?}`。

非缓存响应转成现有 summary delta、translation init/block/done 和 batch item。协议 record 解析失败必须终止当前操作；不得像当前 parser 一样静默跳过 malformed SSE/NDJSON 后继续宣称成功。

本模块不启动 localhost HTTP server，不使用 WebSocket 服务端，不修改 Gist 部署协议，也不把 Wails Stream 包装成全项目通用 transport。

## Cache Contract

服务端缓存键保持现状：

| Cache | Key |
| --- | --- |
| AI summary | entry ID + Readability mode + target language |
| Article translation | entry ID + Readability mode + target language |
| List translation | entry ID + target language |
| Readability | entry ID 上的 `readable_content` |

缓存键不增加 Provider、model、request options、文章内容 hash、Feed `summaryPromptReminder` 或 schema version。

缓存清理继续使用现有 `DataControl` 中的两个区域，但本模块只拥有这些区域的内容工具行为；OPML 属于 `data-transfer`，其他 cache 区域由对应 capability 验收。

清除 AI cache 成功后：

- 显示现有删除计数结果。
- 清空前端 translation store 中的标题、摘要与两种正文模式译文。
- 保留“显示原文”的会话选择；清缓存不是重新启用自动翻译。
- 当前正在查看的已完成摘要或译文可以保留到用户隐藏、切换文章或重新发起操作；它只是当前呈现状态，不再作为之后的客户端 cache 命中。

清除 Readability cache 成功后：

- 显示现有删除计数结果。
- 失效 entries 查询，使之后重新选择文章时不再命中已删除的 `readableContent`。
- 当前正在查看的已提取正文可以保留到用户切回 RSS 正文或离开文章；重新选择后，下一次手动或新的自动尝试重新访问服务端。

AI cache 清理只清 translation store 的 `data`，保留 `disabled`；而 logout、`401` 和更换服务同时清 `data` 与 `disabled`。实现只需增加这两个具体 store action 并失效现有 Query，不为强制刷新当前画面增加跨组件缓存协调服务、事件总线或持久化 client cache。

## Tech Stack

继续使用仓库已有依赖：

| 层 | 现有技术 |
| --- | --- |
| Shared frontend | React、现有正文组件与 Hooks、TanStack Query、Zustand |
| Web stream | Fetch、ReadableStream、SSE 与 NDJSON parser |
| Desktop stream | Wails `v3.0.0-beta.12` `HandleStream` / `JSONStream` |
| Desktop upstream | Go `net/http`、同一个 `ConnectionService` target/token |
| Backend AI | 现有 AIService、Provider、rate limiter 与 SQLite repositories |
| Readability | 现有 ReadabilityService 与 entries cache |
| Tests | Vitest、Testing Library、Go `testing`、testify、GoMock、`httptest` |

本模块不增加生产依赖、数据库 migration、AI SDK、前端请求库、状态管理库、队列、Worker、localhost server 或桌面凭据存储。

## Commands

以下命令是本模块实施后必须成立的验证契约。

### Targeted frontend tests

```bash
cd frontend
bun run test -- \
  src/api/content-tools.test.ts \
  src/desktop/content-tools-stream.test.ts \
  src/hooks/useReadability.test.ts \
  src/hooks/useAISummary.test.ts \
  src/hooks/useAITranslation.test.ts \
  src/services/translation-service.test.ts \
  src/stores/translation-store.test.ts \
  src/lib/language-detect.test.ts \
  src/components/entry-list/EntryList.test.tsx \
  src/components/settings/tabs/DataControl.content-tools.test.tsx
```

### Targeted backend tests

```bash
cd backend
go test ./internal/handler ./internal/service ./internal/service/ai ./internal/repository
```

### Desktop stream tests

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

手工 AI 验收需要一台已经初始化、可登录并已配置可用 AI Provider 的 Gist 服务。自动测试不得要求真实 Provider Key。Readability 自动测试使用 `httptest.Server` fixture，不把公网文章站点作为 CI 门槛。

## Project Structure

```text
desktop/
├── main.go                              # 注册固定 content-tools.ai stream
├── content_tools_stream.go              # 三种操作映射、上游请求与记录转发
└── content_tools_stream_test.go

frontend/src/
├── api/
│   ├── index.ts                         # 现有公开 generators 与结构化流解析
│   └── content-tools.test.ts
├── components/
│   ├── entry-content/
│   │   ├── EntryContent.tsx
│   │   ├── EntryContentHeader.tsx
│   │   ├── EntryContentBody.tsx
│   │   └── AiSummaryBox.tsx
│   ├── entry-list/
│   │   ├── EntryList.tsx
│   │   └── EntryListItem.tsx
│   └── settings/tabs/
│       ├── DataControl.tsx              # 只调整 AI/Readability cache 区域
│       └── DataControl.content-tools.test.tsx
├── desktop/
│   ├── content-tools-stream.ts          # JSONStream 的三个具体 generator
│   └── content-tools-stream.test.ts
├── hooks/
│   ├── useReadability.ts
│   ├── useReadability.test.ts
│   ├── useAISummary.ts
│   ├── useAISummary.test.ts
│   ├── useAITranslation.ts
│   └── useAITranslation.test.ts
├── services/
│   ├── translation-service.ts
│   └── translation-service.test.ts
├── stores/
│   ├── translation-store.ts
│   └── translation-store.test.ts
└── vite.desktop.config.ts               # VITE_DESKTOP="true"

backend/internal/
├── handler/
│   ├── ai_handler.go                    # 结构化 summary SSE 与现有翻译流
│   ├── ai_handler_test.go
│   ├── entry_handler.go
│   └── entry_handler_test.go
├── service/
│   ├── ai_service.go
│   ├── ai_service_test.go
│   ├── readability_service.go
│   └── readability_service_test.go
└── repository/
    ├── ai_summary_repository.go
    ├── ai_translation_repository.go
    └── ai_list_translation_repository.go
```

测试继续与对应源码相邻。不得为了本模块移动整个 API client、拆分 `EntryContent`/`EntryList` 组合、复制组件到 `desktop/`，或重排后端现有 handler/service/repository 分层。

## Code Style

### TypeScript / React

- 沿用 strict TypeScript、现有别名、Hooks、Zustand actions、TanStack Query 和 i18n 约定。
- 三个公开 async generator 的调用签名保持不变；桌面分支只存在于 API 边界。
- 每个 Hook 拥有并清理自己的 AbortController、request sequence 和 loading/error 状态；不建立全局请求 registry。
- 只在正常业务终态后发布完成状态。Abort、error 或 malformed stream 不能落入 success 分支。
- 使用现有 `AiSummaryBox` 与正文/工具栏范围显示错误，不增加 toast Provider、全局错误中心或新页面。
- translation store 继续只存在于当前会话；按 entry、target language 与 Readability mode 使用现有槽位。
- 自动尝试使用最小的 per-entry ref/set 防止紧密重试，不增加持久化重试策略或退避调度器。
- 不静默吞掉非 Abort 错误；背景列表 batch 可以保留原文，但必须清理 in-flight 标记并允许之后的新会话重试。

### Go

- 使用 `gofmt`，沿用 `net/http`、现有日志和 connection 状态。
- `contentToolsStream` 是具体结构，不为一个调用点建立 interface、factory 或 transport registry。
- operation 使用显式 `switch` 映射固定 method/path；不接受任意 URL、path、method 或 header。
- 上游请求使用 Wails stream context；客户端关闭后不得改用 `context.Background()` 继续执行。
- SSE/NDJSON 按完整记录读取并转发，保留 UTF-8 与 JSON 边界；不假设一次 `Read` 就是一条业务消息。
- 日志可以记录 operation、上游状态和错误，但不得记录 Token、API Key、文章正文、摘要或译文。
- 不手工编辑生成的 Wails bindings 或 GoMock 文件。

### Backend

- 保持现有 handler/service/repository 分层和 Provider 接口。
- Summary handler 只把未缓存响应改成结构化 SSE；缓存 JSON、请求字段、语言、提示词和 repository 保持不变。
- 翻译块错误继续停止完整缓存写入；客户端负责按已确认呈现规则回退。
- 不为了详情标题与正文的呈现原子性增加数据库事务、组合表或新 endpoint。

## Testing Strategy

### Frontend Hooks and UI

至少覆盖：

- Readability 缓存命中只切换显示，不发请求。
- 手动 Readability 成功、失败和手动重试。
- 自动 Readability 每篇只尝试一次，失败不形成请求循环。
- 切换 entry 后旧 Readability 结果不能覆盖新文章。
- 自动 Readability 与自动摘要/翻译同时开启时，等待 Readability 结束后只对最终正文模式各执行一次 AI 请求。
- Summary 缓存命中、SSE delta 累加、正常 `done`、隐藏和再次显示。
- Summary 取消会清除部分内容且不显示错误；失败、malformed stream 或缺少 `done` 会清除部分内容、停止 loading 并允许重试。
- Summary 在 entry 或 Readability 模式变化时取消旧流，旧 delta 不覆盖当前结果。
- Article translation 的 cached、init、逐 block 与 done 路径。
- `autoTranslate=false` 时手动翻译仍同时显示标题与正文；取消或任一部分失败后共同回到原文。
- 标题 batch 先成功但正文随后失败，或 batch EOF 没有当前 entry 结果时，当前详情和重新选择文章后都保持完整原文，不泄漏共享 store 中的标题译文。
- 翻译 error event、协议错误或缺少 done 不再被当作成功，也不保留部分译文。
- 点击“显示原文”后，同一认证服务会话重新选择 entry、切换普通/Readability 模式仍显示原文；再次点击翻译才恢复。
- 重置 store、退出、更换服务或模拟应用重启后，“显示原文”选择被清除，并重新遵守当前自动翻译设置。
- 普通与 Readability 正文译文按模式隔离。
- 自动翻译失败每篇只自动尝试一次，不形成请求循环。
- 列表只调度可见项与选中项，继续去重、限制范围并逐 item 更新。
- 切换列表只取消该列表 batch，不取消详情翻译；详情取消也不影响列表 batch。
- Batch 部分成功保留成功文章，失败文章保留原文并在后续会话可重试。
- 清除 AI cache 同步清 translation store；清 Readability cache 同步失效 entries。当前画面可保留到用户隐藏或离开，不再作为之后的 cache 命中。
- 清 AI cache 保留 disabled；logout、`401` 和更换服务同时清除 translation data 与 disabled。

### API and desktop stream

至少覆盖：

- Web summary parser 接受 cached JSON 与结构化 SSE，并拒绝 error、malformed record 和缺少终态。
- Web translation parser 接受 cached JSON、init/block/done，并把 error 与缺少 done 作为失败。
- Web batch parser 处理任意网络 chunk 边界下的 NDJSON，不静默跳过 malformed line。
- Desktop adapter 对三种 operation 发送固定 request，解析同样的业务结果，并保持公开 generator 签名。
- Go stream 只映射三个固定上游 endpoint，使用当前 connection target/token，正确转发请求体和非成功状态。
- 上游 record 在任意网络分片下仍逐条到达前端，不因 Windows asset writer 缓冲。
- AbortSignal 关闭 JSONStream，Wails context 取消上游 HTTP request。
- 旧 stream 的迟到消息不能写入新 entry、模式或连接。
- `401` 复用 connection 的统一未授权清理；其他错误保留状态和消息。
- 不接受未知 operation，也没有任意 path、URL、header 或 Token 输入。
- 服务地址带路径前缀时，三个固定 path 都追加在该前缀之后。
- 超过 `64 KiB` 的单个 SSE record 仍能完整到达，不被默认 Scanner 限制截断。

### Backend

至少覆盖：

- Summary cache hit 保持 JSON；cache miss 使用 delta/done SSE。
- Summary Provider 中途失败发送明确 error 终态，不把错误文本混进摘要，不保存部分缓存。
- Summary context 取消停止 Provider stream 且不保存缓存。
- Feed `summaryPromptReminder` 继续进入新摘要 prompt；缓存命中不重新生成。
- 同一次请求的 cache lookup、prompt、生成与 cache save 使用同一个目标语言快照；设置中途变化不能把旧语言结果保存到新语言 key。
- Article translation cache hit、init/block/done、块错误、取消和完整缓存语义保持正确。
- 代码、数学和媒体块规则保持现状。
- Batch cache hit、最多 `100` 篇、逐 item 返回、部分失败与取消保持现有独立文章语义。
- AI cache 清除继续返回三类删除计数。
- Readability 使用确定性 `httptest.Server` 覆盖 fetch、extract、persist 与第二次 cache hit。
- Readability 的非法 ID、无 URL、非 200、解析失败与 context 取消映射保持现状。
- 不增加数据库 migration。

### Manual regression

完整行为先在 Web 与至少一个桌面平台验证：

1. 手动开启/关闭 Readability，验证缓存命中与失败重试。
2. 自动 Readability + 自动摘要 + 自动翻译同时开启时，等待 Readability 结束，并且只处理最终正文一次。
3. 手动生成摘要，看到增量；取消后部分摘要消失；失败后显示错误并可重试。
4. 手动翻译文章，看到标题与正文逐步进入译文；“显示原文”同时恢复两者。
5. 重新选择同一文章后仍尊重本次会话的“显示原文”；再次点击翻译恢复。
6. 在翻译中取消或模拟一个 block 失败，确认详情完整回到原文并显示正确错误状态。
7. 自动翻译列表，只翻译可见项和选中项；快速切换列表后旧结果不写入当前列表。
8. 切换文章、Readability 模式、退出和更换服务时，旧流停止且不污染新状态。
9. 清除 AI 与 Readability cache 后，当前客户端状态和之后的服务端请求一致。
10. 在桌面端确认摘要、正文翻译和列表翻译都逐步到达，而不是等待完整响应后一次出现。

Windows、macOS 和 Linux 各自只需完成短冒烟：Readability 有限响应可用；摘要、正文翻译和列表翻译逐步到达；取消其中一个流会停止该流且应用继续可用。完整错误注入、全部回退与清缓存流程不要求在三个平台重复执行。

不设置人为覆盖率百分比，不要求真实 OpenAI/Anthropic Key、真实公网 Readability 网站、截图测试、性能压测或 Provider × 平台的全组合 CI。

## Boundaries

### Always

- 复用现有工具栏、正文、摘要框、列表、Hooks、translation store 与相对 `/api` 合约。
- 保持 Web 与桌面共享业务实现，Web 使用 fetch，桌面仅为三个 AI 流使用 Wails Stream。
- Readability 与 AI 始终在 Gist 服务端执行。
- 对取消、失败、entry/mode/connection 切换执行明确清理，避免不完整或迟到结果被当作完成。
- 自动 Readability 完成或失败后再确定自动摘要与正文翻译的输入。
- 手动翻译同时处理详情标题与正文；失败或取消时共同回到原文。
- 本次会话内尊重用户对单篇文章选择的“显示原文”。
- 清服务端 cache 时同步清理对应客户端缓存状态。
- 退出、`401` 或更换服务时清空 translation store 的 data 与 disabled，不把一台服务的 entry 状态带到下一台服务或下一位用户。
- 只修复本模块已经确认的流、状态和重试错误，不增加无目标防御层。

### Ask First

- 改变已确认的“失败/取消全部回退”、标题与正文共同翻译或会话内显示原文语义。
- 改变自动 Readability、摘要、翻译的触发顺序或列表可见项范围。
- 改变 API 请求字段、流事件、缓存键、缓存失效或 Provider prompt 语义。
- 新增局部结果保留、逐块重试、自动退避、任务历史、进度中心或全局通知。
- 新增本地 AI、桌面直连 Provider、离线正文、预生成或后台批处理。
- 把 AI/通用设置编辑、Feed 管理、阅读路由或 OPML 并入本模块。
- 把三个固定 Wails 流扩大成通用 HTTP/stream transport。
- 增加生产依赖、数据库 migration 或新的服务端 endpoint。

### Never

- 复制 EntryContent、EntryList、工具栏或设置 UI 到 `desktop/`。
- 从桌面 WebView 或 Wails service 直接调用 AI Provider。
- 把 Provider API Key、任意 URL、任意 path 或 Token 放进 content-tools stream 请求。
- 使用普通 asset proxy 声称在 Windows beta.12 已实现逐步 SSE/NDJSON。
- 启动 localhost HTTP server 绕过 Wails Stream，或建立通用 WebSocket/事件总线。
- 静默忽略 summary/translation 的协议错误、缺少终态或非 Abort 异常后宣称成功。
- 缓存失败、取消或不完整的摘要与正文翻译。
- 为 Provider/model/prompt 变化增加自动 cache versioning、后台重算或级联工作流。
- 在本模块新增 AI Settings、General Settings、OPML、Profile 或独立 DataControl 页面。
- 为未来多连接、插件、离线或多 Provider 并发建立 registry、factory、队列或扩展点。
- 通过删除测试、降低断言或吞掉错误让验证通过。

## Success Criteria

- [ ] Web 与桌面复用现有 Readability、摘要、单篇翻译和列表翻译 UI 与业务 Hooks。
- [ ] Readability 缓存、手动切换、自动执行、失败与重试行为正确。
- [ ] 自动 Readability 失败不会形成请求循环，切换文章后旧结果不会覆盖新文章。
- [ ] 自动 Readability 与自动 AI 同时开启时，只对最终显示正文执行一次摘要和一次正文翻译。
- [ ] 摘要缓存命中一次显示；未命中逐步显示，并以结构化 done/error 结束。
- [ ] 摘要取消或失败不保留部分结果；失败显示可重试错误且不写缓存。
- [ ] 单篇翻译逐块显示；取消或任一部分失败后标题和正文共同回到原文，失败可重试。
- [ ] 即使 `autoTranslate=false`，手动翻译也同时处理详情标题与当前模式正文。
- [ ] 标题先成功、正文后失败时，当前详情和重新选择文章后都不泄漏标题译文。
- [ ] 点击“显示原文”后，同一认证服务会话重新选择该文章或切换正文模式仍显示原文；再次点击翻译恢复译文。
- [ ] 退出、更换服务或重启应用会清除“显示原文”选择，并重新遵守当前自动翻译设置。
- [ ] 普通与 Readability 正文译文继续按模式隔离。
- [ ] 自动列表翻译只处理可见项与选中项，逐篇更新，并保持最多 `100` 篇的服务端限制。
- [ ] Batch 部分成功保留成功文章，失败文章保留原文；列表与详情请求互不误取消。
- [ ] Web 三种流继续使用同源 fetch；桌面三种流使用固定 Wails Stream 并逐步到达。
- [ ] 关闭桌面 JSONStream 会取消 Go context 与上游 Gist 请求。
- [ ] Desktop stream 只接受三个枚举操作，不接受任意 URL、path、method、headers 或 Token。
- [ ] `401` 复用 connection 的统一清理，其他流错误不会伪装成完成。
- [ ] 退出、`401` 和更换服务会清空 translation store 的译文与“显示原文”标记；新服务相同 entry ID 不复用旧状态。
- [ ] AI cache 清理后 translation store 不再命中旧译文；Readability cache 清理后重新选择文章不再命中旧提取正文。
- [ ] Provider/model/request options/prompt 变化不自动重算缓存；未增加 cache versioning。
- [ ] AI/通用设置编辑仍属于 `settings-profile`，Feed 摘要提示词编辑仍属于 `library`，OPML 仍属于 `data-transfer`。
- [ ] 未新增业务页面、全局通知、任务中心、本地 AI、离线缓存、生产依赖或 database migration。
- [ ] 定向测试、完整前后端回归和当前平台 Wails 构建全部通过。
- [ ] Web 与至少一个桌面平台完成全部行为回归，包括失败回退与清缓存。
- [ ] Windows、macOS 和 Linux 各自完成 Readability、三种 AI 流逐步到达与取消的短冒烟。

## Open Questions

无。用户已确认：取消或失败不保留不完整摘要/单篇译文；手动翻译同时处理标题与正文；“显示原文”在同一认证服务的本次应用会话内持续有效；三个自动功能同时开启时先等待 Readability，再只对最终正文调用一次 AI。

本文件已经批准。按照既定工作流，继续完成 `settings-profile` 与 `data-transfer` Spec；全部 capability Spec 就绪后再统一进入任务拆分。

## References

- [`CAPABILITY-MAP.md`](./CAPABILITY-MAP.md)
- [`SPEC-reader-workspace.md`](./SPEC-reader-workspace.md)
- [`SPEC-connection.md`](./SPEC-connection.md)
- [`SPEC-reading.md`](./SPEC-reading.md)
- [`SPEC-library.md`](./SPEC-library.md)
- [Wails beta.12 Streams guide](https://github.com/wailsapp/wails/blob/v3.0.0-beta.12/docs/src/content/docs/guides/streams.mdx)
- [Wails beta.12 stream probe host at fixed commit](https://github.com/dnslin/Gist/blob/d9dbf54/prototypes/wails-v2-v3-hard-gates/v3/GistWailsV3Probe/main.go)
- [Wails beta.12 stream probe frontend at fixed commit](https://github.com/dnslin/Gist/blob/d9dbf54/prototypes/wails-v2-v3-hard-gates/v3/GistWailsV3Probe/frontend/src/runtime-adapter.ts)
- [`frontend/src/components/entry-content/EntryContent.tsx`](./frontend/src/components/entry-content/EntryContent.tsx)
- [`frontend/src/hooks/useReadability.ts`](./frontend/src/hooks/useReadability.ts)
- [`frontend/src/hooks/useAISummary.ts`](./frontend/src/hooks/useAISummary.ts)
- [`frontend/src/hooks/useAITranslation.ts`](./frontend/src/hooks/useAITranslation.ts)
- [`frontend/src/components/entry-list/EntryList.tsx`](./frontend/src/components/entry-list/EntryList.tsx)
- [`frontend/src/services/translation-service.ts`](./frontend/src/services/translation-service.ts)
- [`frontend/src/stores/translation-store.ts`](./frontend/src/stores/translation-store.ts)
- [`frontend/src/api/index.ts`](./frontend/src/api/index.ts)
- [`backend/internal/handler/ai_handler.go`](./backend/internal/handler/ai_handler.go)
- [`backend/internal/service/ai_service.go`](./backend/internal/service/ai_service.go)
- [`backend/internal/service/readability_service.go`](./backend/internal/service/readability_service.go)
