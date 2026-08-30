# Spec: connection

> 状态：Approved（2026-08-30）
> Module ID：`connection`
> 来源：[`CAPABILITY-MAP.md`](./CAPABILITY-MAP.md)

## Objective

让 Wails 桌面客户端连接一台已经初始化的 Gist 服务，完成登录、跨重启会话恢复、退出登录和更换服务地址，并为现有前端提供同源的 `/api` 与 `/icons` 请求入口。

本模块完成后：

- 首次启动显示一个只填写 Gist 服务地址的连接页面。
- 服务地址验证成功后复用现有 `LoginPage`，不复制登录表单。
- 登录成功或已有 Token 仍有效时，桌面宿主进入已认证状态。
- 重启应用后，保存的服务地址和 Token 能自动恢复当前会话。
- 普通 API、Feed 图标和受保护的图片代理继续使用现有根相对 URL。
- 退出登录保留服务地址，更换服务地址则清除旧会话和旧查询缓存。
- Web 客户端原有的注册、登录、同源请求和 PWA 行为保持不变。

本模块不实现任何业务页面，也不重新定义阅读、订阅、AI、设置或 OPML 的业务契约。若 `reader-workspace` 已实施，最终桌面组合在认证成功后挂载共享 `ReaderWorkspace`；连接 gate 本身只负责决定何时可以渲染已认证内容。

## Confirmed Decisions

- 桌面 WebView 继续请求同源的相对路径 `/api` 与 `/icons`。
- 桌面构建不注入远端 `VITE_API_URL`；该变量只保留给现有 Web 构建。
- Wails `AssetOptions.Middleware` 在 Go 侧把这些路径转发到当前 Gist 服务。
- Gist 后端不新增 CORS。
- 服务校验复用现有 `GET /api/auth/status`，不新增 desktop handshake、实例 ID、版本或 capability 协商。
- 只连接 `{"exists":true}` 的已初始化服务；`{"exists":false}` 时提示用户先通过 Web 初始化，不显示 `RegisterPage`。
- 服务地址保存在新的 WebView `localStorage` 键 `gist_service_url`。
- Token 继续保存在现有 `localStorage` 键 `gist_auth_token`。
- Go 进程只保存当前运行期的地址和 Token，不写配置文件，不使用系统凭据库。
- 连接页面与登录页面采用两步流程；地址和账号密码不合并成一张新表单。
- 桌面 logout 只清除本地会话，不调用只负责清 Cookie 的远端 logout；Web logout 保持原行为。
- 允许绝对 `http://` 和 `https://` 地址。HTTPS 使用 Go/操作系统的正常证书校验。
- 不加入证书固定、自签名证书绕过、局域网批准、地址 allowlist、DNS 复验或重定向策略。
- 同一时间只保存一组服务地址与会话，不建立连接列表、连接 ID 或多连接缓存命名空间。

## Existing Service Contract

本模块直接使用现有接口：

| 行为 | 请求 | 成功响应 | 桌面处理 |
| --- | --- | --- | --- |
| 检查初始化 | `GET /api/auth/status` | `200 {"exists":boolean}` | `true` 进入会话恢复或登录；`false` 提示先在 Web 初始化 |
| 登录 | `POST /api/auth/login` | `200 {"token","user"}` | 先同步 Token 到 Go，成功后保存 Token 并进入已认证状态 |
| 恢复会话 | `GET /api/auth/me` + Bearer | `200 {username,nickname,email,avatarUrl}` | 恢复用户并进入已认证状态 |
| Web 退出 | `POST /api/auth/logout` | `200 {"message":"logged out"}` | 仅现有 Web 客户端继续调用；桌面不调用 |

桌面端不调用 `/api/auth/register`。

现有 logout 只清除 Cookie，不撤销 JWT；桌面代理又明确不接收上游 `Set-Cookie`。因此桌面调用它没有效果，也可能让本地退出等待网络。桌面 logout 直接清除本地 Token、Go 运行期 Token 和查询状态；现有 Web logout 继续调用该接口。`/api/auth/me` 或其他已认证请求返回 `401` 时执行同样的桌面本地清理。

## Runtime Connection Contract

### Go service

在 `desktop/` 中增加一个具体的 `ConnectionService`。不为它建立 interface、factory 或通用 transport 抽象。

它向生成的 Wails TypeScript bindings 暴露：

```go
func (s *ConnectionService) Configure(serviceURL string, token string) (string, error)
func (s *ConnectionService) SetToken(token string) error
func (s *ConnectionService) Clear()
```

`Configure`：

1. 去掉输入首尾空白。
2. 使用 `net/url` 解析绝对 URL。
3. 只接受 `http` 或 `https`。
4. 拒绝缺少 host、包含 userinfo、query 或 fragment 的地址。
5. 保留可选的路径前缀，并去掉末尾 `/`；根路径保持为 origin。
6. 原子替换当前运行期的目标 URL 与 Token。
7. 返回规范化后的 URL，作为前端保存的唯一地址形式。

`SetToken` 只替换当前运行期 Token。非空 Token 要求已经存在目标 URL；空 Token 即使尚未配置目标也可以成功，用于 logout 与 `401`。

`Clear` 原子清除当前运行期的目标 URL 与 Token，只用于没有保存地址或明确更换服务。

反向代理请求会与登录、401 和切换服务同时发生，因此该结构只使用一个 `sync.RWMutex` 保护这两个运行期字段。这是实际并发读写所需的最小同步，不扩展成连接管理器。

`main.go` 只创建一个 `ConnectionService` 实例。这个同一实例同时注册到 `application.Options.Services` 生成 bindings，并提供 `AssetOptions.Middleware` 使用的运行期目标；不得创建两份会发生状态分叉的 service。

### Frontend persistence

前端增加两个简单的服务地址函数：

```ts
getServiceUrl(): string | null
setServiceUrl(url: string): void
clearServiceUrl(): void
```

它们只读写 `gist_service_url`。Token 继续通过现有 `getAuthToken`、`setAuthToken` 和 `clearAuthToken` 管理。

现有 Token helper 调整为可等待的具体契约：

```ts
type AuthTokenSynchronizer = (token: string | null) => Promise<void>;

setAuthTokenSynchronizer(sync: AuthTokenSynchronizer | null): void;
setAuthToken(token: string): Promise<void>;
clearAuthToken(): Promise<void>;
```

共享 auth store 另增加一个具体的 `setRemoteLogoutEnabled(enabled: boolean)` setter。桌面入口注册最薄的 Token adapter：`token => ConnectionService.SetToken(token ?? "")`，并调用一次 `setRemoteLogoutEnabled(false)`；Web 不注册 synchronizer，保留 remote logout 默认开启。该布尔开关只解决共享 auth store 的这一个真实宿主差异，不建立通用 auth runtime 或 transport 注入层。

所有 Token 写入与清除必须经过统一 helper；桌面运行时由它同步 Go，Web 运行时则立即完成同步步骤并维持现有本地行为。调用点包括：

- 登录成功。
- 现有 Web 注册成功（只做异步签名迁移；桌面仍不显示注册）。
- 退出登录。
- 全局 `401`。
- 现有 `ProfileSettings` 修改密码后返回新 Token 的调用点只做异步签名所需的机械 `await`；其桌面端到端行为留给 `settings-profile` Spec 验收。

启动恢复不重写保存的 Token；它在发出任何 API 请求前直接把已保存的地址和 Token 一次性交给 `ConnectionService.Configure`。

设置新 Token 时，先读取旧 Token，再等待 `ConnectionService.SetToken(token)` 成功，然后写入 `localStorage`，最后由调用方发布相应认证状态。binding 失败时不保存新 Token、不发布新的认证状态。若同步成功但 `localStorage.setItem` 抛错，helper 只做一次最小补偿：尝试把 Go Token 恢复为旧值，并把原始存储错误交给发起页面；不为此建立事务或重试框架。这样 `ReaderWorkspace` 首次渲染时图片代理已经能使用正确 Token。

清除会话的 auth action 先发布未认证状态并取消当前 QueryClient 请求，使已认证 child 立即卸载；随后 `clearAuthToken()` 删除 `localStorage` 并等待 `ConnectionService.SetToken("")`，最后清空 QueryClient。auth action 捕获并记录 binding 清理错误，仍完成本地状态与缓存清理，不把该错误重新抛给 `401` 请求。下一次进程启动时 Go 状态天然为空。

现有 `setOnUnauthorized` callback 改为允许返回 Promise。普通 request helper 只为标记为已认证的请求处理 `401`；`status`、`register`、`login` 和 `logout` 明确关闭这个 hook。request helper 等待 callback，但会捕获并记录清理错误，最后始终抛出原始 `ApiError`，不得用同步失败覆盖服务端错误。

登录、现有 Web 注册、桌面 logout、受保护请求的 `401` 和 ProfileSettings 的调用点都必须显式 `await` Token helper；不得使用未等待的 effect 弥补同步时序。现有 request helper 增加一个内部的 `handleUnauthorized` 请求选项即可，不为此建立第二套 API client。

Web 入口不注册这个桌面同步回调。现有 Web Token 行为保持不变。

## Same-origin Proxy Contract

### Route ownership

Wails asset middleware 只拦截以下精确路径族：

- `/api`
- `/api/*`
- `/icons`
- `/icons/*`

以下路径继续交给 Wails 默认资产处理器或 Vite 开发服务器：

- `/`
- `/index.html`
- `/assets/*`
- `/logo.svg`
- `/wails/*`
- 其他桌面前端静态资源和前端路由

开发模式和生产模式使用同一个 middleware。不得把 Vite dev proxy 当作生产连接实现。

### Forwarding

对被拦截的普通请求，代理必须：

- 把目标设为当前服务地址，并在可选路径前缀后追加原始 path。
- 保留 method、path、query、请求 body、未在本节明确删除的请求 headers、除下述 `304` 限制外的上游状态码、响应 headers 和响应 body。
- 把上游 `Host` 设置为目标服务的 host。
- 每个 `/api` 请求都以当前 Go 运行期 Token 为唯一转发来源：Token 非空时设置 `Authorization: Bearer <token>`；Token 为空时删除任何传入的 `Authorization`。
- `/icons` 保持公开访问，不要求 Token。
- 对 `GET` 与 `HEAD` 删除 `If-None-Match` 与 `If-Modified-Since` 请求头，要求上游返回完整响应而不是条件请求的 `304 Not Modified`。其他 method 保留这些 header，不能改变写请求的 precondition 语义。

这个行为解决 `<img src="/api/proxy/image/...">` 无法自行添加 Bearer header 的现有问题。代理在开始转发每个新请求时读取当前 Go Token；已经交给上游的请求无法撤回，因此 logout 与更换服务会先取消 QueryClient 请求，再清除 Token 和缓存。普通 Web API 请求仍按现状添加 Bearer header；只有桌面代理会用已经同步的 Go 值替换它。

对读取请求删除条件缓存头是 Wails beta.12 的已知平台约束：Windows asset response writer 会把上游 `304` 变成 `500`。本模块以重新获取有限响应换取三平台一致的正确结果；若上游在没有条件头时仍主动返回 `304`，该响应不属于本 Spec 的成功转发保证。这里不建立缓存层，也不改写 ETag 响应头。

### Cookie handling

桌面代理不把 WebView 的 `Cookie` 请求头发给远端，也不把远端 `Set-Cookie` 响应头写回 WebView。

原因是 Windows、macOS 和 Linux 的 Wails 内部 origin 不同，而且更换远端服务时它们仍共享各自平台上的同一个桌面 origin。若保留 Cookie，服务 A 设置的 `gist_auth` 可能在连接服务 B 时被带过去。桌面会话已经由已确认的 `localStorage` Bearer Token 负责，因此这里直接去掉不可靠且会串服务的第二条凭据通道。

现有 Web 客户端仍使用原来的 Cookie 行为，后端 Cookie 实现不修改。

### Proxy errors

未配置目标时，被拦截路径返回：

```http
HTTP/1.1 503 Service Unavailable
Content-Type: application/json

{"error":"Gist service is not configured","code":"connection_not_configured"}
```

Go transport 无法连接上游时返回：

```http
HTTP/1.1 502 Bad Gateway
Content-Type: application/json

{"error":"Unable to reach Gist service","code":"connection_unavailable"}
```

代理日志保留目标地址和底层 Go 错误，但不得记录 Token 或密码。

`ApiErrorResponse` 与 `ApiError` 增加可选的 `code`。现有 `isNetworkError` 同时识别 `code === "connection_unavailable"`，使启动恢复和认证状态能把代理的 502 正确显示为网络错误，而不是错误地进入普通登录状态。远端服务自己返回的 502 不带该 code，因此不会被误分类为桌面传输失败。

不建立更大的错误码体系。

## Desktop Connection Flow

### UI composition

新增 `DesktopConnectionGate`。它只拥有连接与认证 gate，并在认证完成时渲染 `children`：

```tsx
<DesktopConnectionGate>
  <ReaderWorkspace />
</DesktopConnectionGate>
```

这个单一 `children` 边界使 `connection` 仍可独立于 `reader-workspace` 测试，同时不建立随后删除的临时成功页面。`ReaderWorkspace` 本身继续保持零参数。

桌面 gate 使用已有 Router、QueryClient、i18n 和 Tooltip Provider。它不修改现有业务路由格式。

### First connection

1. 没有 `gist_service_url` 时清除孤立的 `gist_auth_token` 与 Go 运行期状态，然后显示新的 `ConnectionPage`；主题、语言、布局等无关偏好不受影响。
2. 页面只包含服务地址输入、连接按钮、错误信息和现有 Gist 品牌元素。
3. 提交后先调用 `ConnectionService.Configure(candidate, "")`。
4. 使用同源 `GET /api/auth/status` 验证该地址。
5. 只有 `200`、合法 JSON 且 `exists === true` 时保存规范化地址。
6. 随后显示现有 `LoginPage`。

失败处理：

| 结果 | 行为 |
| --- | --- |
| URL 解析失败 | 留在地址页，显示具体输入错误 |
| `connection_unavailable` | 留在地址页，保留输入，允许重试 |
| `404`、无效 JSON 或不符合 status 契约 | 留在地址页，提示不是可用的 Gist 服务 |
| `exists:false` | 留在地址页，提示先通过 Web 初始化，并允许重新检查或修改地址 |
| 其他服务端错误 | 留在地址页，显示服务返回的错误并允许重试 |

失败的候选地址不写入 `gist_service_url`。

新增的地址、未初始化、重试和修改服务文案同时写入现有英文与中文 `common.json`，组件中不硬编码两套字符串。

### Startup restore

若已经保存服务地址：

1. 从 `localStorage` 读取服务地址和 Token。
2. 等待 `ConnectionService.Configure(serviceURL, token ?? "")` 完成。
3. 再请求 `/api/auth/status`。
4. `exists:false` 时进入“服务尚未初始化”状态，不显示注册。
5. `exists:true` 且没有 Token 时显示登录页。
6. 有 Token 时请求 `/api/auth/me`。
7. `/api/auth/me` 成功后才渲染已认证内容。
8. `/api/auth/me` 返回 `401` 时清除前端与 Go Token，保留服务地址并显示登录页。
9. 上游不可达时保留服务地址与 Token，显示现有网络错误页，并提供“重试”和“修改服务地址”。
10. status 或 `/api/auth/me` 返回其他非成功状态、无效 JSON 或错误结构时，保留服务地址与 Token，显示可重试的服务错误。

若第 2 步的 `Configure` binding 失败，桌面不发出任何 API 请求，保留保存的地址与 Token，并显示可重试的连接设置错误；用户也可以选择“修改服务地址”进入清理流程。

网络失败和服务端 `5xx` 都不能被当作 Token 失效，重试前也不能删除保存的 Token。只有明确的 `401` 清除 Token；这会修正现有 auth store 把 `/auth/me` 的所有非网络错误都当成未授权的过宽处理。

桌面启动流程可以把现有 auth store 的“恢复 Token”逻辑提取为一个明确 action，供 Web `initialize` 和桌面 gate 复用。Web 仍保留自己的 `/auth/status → RegisterPage/LoginPage` 分支；不得加入运行模式判断散布到业务组件。

### Login, logout and unauthorized

- 登录继续接受现有 identifier（用户名或邮箱）和密码。
- 登录 `401` 时保留在登录页并显示现有接口错误。
- 登录网络失败时保留在登录页并显示连接错误，允许重试或修改服务地址。
- 登录成功后按照“同步 Go Token → 保存 Token → authenticated”的顺序执行；Go 同步或保存失败都不渲染已认证内容。
- 桌面退出不调用远端 logout；它立即卸载已认证内容、取消请求、清除 Token 和 QueryClient，保留服务地址，并返回同一服务的登录页。Web 仍执行现有远端 logout 后的本地清理。
- 已认证请求的全局 `401` 执行与桌面退出相同的本地清理。公开的 status、register、login 和 logout 请求不触发全局 `401` hook。
- `LoginPage` 与 `NetworkErrorPage` 只增加可选的服务地址显示及“修改服务地址”回调；`NetworkErrorPage` 还可接收桌面服务返回的具体错误。Web 不传这些 props，现有 Web UI 不变。

### Change service

“修改服务地址”是明确的单连接替换操作：

1. 发布未认证状态，卸载已认证 child，并取消当前 TanStack Query 请求。
2. 调用统一的 `clearAuthToken()`，立即删除本地 Token，并尝试清除 Go Token。
3. 清除 auth store 的其余用户与错误状态、QueryClient 和 `gist_service_url`。
4. 调用 `ConnectionService.Clear()` 尝试清除 Go 目标地址与 Token。
5. 导航回根路径并显示地址页。

第 2 或第 4 步的 binding 调用失败时只记录诊断，不得中断第 3 和第 5 步；本地地址、Token、认证状态和缓存仍必须清除并回到地址页。不保留旧地址，不建立最近使用列表，也不把现有 Query keys 增加 connection ID。清缓存是为了防止旧服务数据在新服务登录后继续显示，属于单连接切换的正确性要求。

## Stream Boundary

Wails `v3.0.0-beta.12` 的 Windows asset response writer 会缓冲完整响应，直到请求结束才交给 WebView2。因此同源代理不能在三平台上透明兑现现有 SSE、NDJSON 和原始文本流的逐块交付。

本 Spec：

- 只验收普通 JSON、普通资源、请求 body、文件 body 和有限响应的转发。
- 不宣称 AI 摘要、翻译、批量翻译或 OPML 进度已经完成桌面接入。
- 不在 `connection` 中建立通用流转换器。
- 不启动本地 HTTP 监听端口规避该限制。

后续 `content-tools` 和 `data-transfer` Spec 必须分别选择并验证能在三个目标平台逐块交付的方案，同时保留各自的取消与错误语义。Wails beta.12 的 Streams 是可核验的候选，但本 Spec 不替那些能力预先决定实现方式。

已知需要后续桥接的现有端点是：

- `/api/ai/summarize`
- `/api/ai/translate`
- `/api/ai/translate/batch`
- `/api/opml/import/status`

## Tech Stack

| 项目 | 约束 |
| --- | --- |
| Desktop framework | Wails `v3.0.0-beta.12` |
| Host transport | Go `net/http`、`httputil.ReverseProxy`、Wails `AssetOptions.Middleware` |
| Host state | 一个具体 `ConnectionService` + `sync.RWMutex` |
| Frontend bridge | Wails 生成的 TypeScript bindings |
| Wails frontend runtime | `@wailsio/runtime` 精确锁定 `3.0.0-beta.12` |
| Persistence | WebView `localStorage` |
| Authentication | 现有 JWT Bearer 与 auth store |
| Server state | 现有 TanStack QueryClient；更换连接时 `clear()` |
| UI | 现有 React、i18n、Tailwind 和 auth 页面样式 |

本模块不增加第三方 Go HTTP client、Keyring、前端状态库、表单库或连接管理依赖。

## Project Structure

```text
desktop/
├── main.go                         # 注册 ConnectionService 与 asset middleware
├── connection.go                   # URL 规范化、运行期 URL/Token 与 Wails methods
├── connection_test.go
├── connection_proxy.go             # /api 与 /icons 同源反向代理
└── connection_proxy_test.go

frontend/
├── bindings/                       # Wails 生成并提交的 TypeScript bindings
├── public/
│   └── logo.svg                    # Web 源文件；桌面构建只复制这一项品牌资源
└── src/
    ├── api/index.ts                # 可选 error code 与统一 Token 同步入口
    ├── api/index.test.ts
    ├── components/auth/
    │   ├── LoginPage.tsx           # 可选服务地址/修改地址 props
    │   └── NetworkErrorPage.tsx    # 可选修改地址 props
    ├── desktop/
    │   ├── ConnectionPage.tsx
    │   ├── ConnectionPage.test.tsx
    │   ├── DesktopConnectionGate.tsx
    │   ├── DesktopConnectionGate.test.tsx
    │   ├── connection-runtime.ts   # 调用生成的 ConnectionService bindings
    │   ├── connection-storage.ts
    │   └── connection-storage.test.ts
    ├── lib/errors.ts               # 识别 connection_unavailable
    └── stores/auth-store.ts        # 可复用的会话恢复与 Token 同步调用点
```

生成的 bindings 属于源码并提交，不能手工编辑。这样现有 Web 的 TypeScript 检查不要求先在本机安装 Go/Wails。桌面 Taskfile 在构建前重新生成到仓库 `frontend/bindings/`。

桌面构建仍不复制整个 `frontend/public/`。它只把现有 `logo.svg` 放入桌面产物；i18n JSON 已被现有 TypeScript import 打包，不需要复制 public locales，也不需要建立通用静态资产系统。

## Wails Binding and Build Contract

`desktop/Taskfile.yml` 的桌面前端构建增加 bindings 依赖。等价的手工命令为：

```bash
cd desktop
wails3 generate bindings -d ../frontend/bindings -clean=true -ts
```

要求：

- 只为实际注册的 `ConnectionService` 生成 bindings。
- 输出目录固定为仓库 `frontend/bindings/`。
- `frontend/tsconfig.app.json` 纳入生成目录或其被桌面源码引用的文件。
- `@wailsio/runtime` 精确锁定 `3.0.0-beta.12`，不使用 `latest`。
- `wails3 dev` 与 `wails3 build` 都在启动桌面前端前完成 bindings 生成。
- `build:desktop` 使用已经生成或仓库已提交的 bindings，不自行调用 Go 工具链。
- 桌面 dev/build 明确让 `VITE_API_URL` 为空，使现有 API 模块继续生成相对请求。
- Web `bun run build` 不生成 bindings，也不要求 Go 工具链；它使用仓库中已提交的 bindings。
- `verify:desktop-assets` 继续拒绝 PWA 产物，并新增检查桌面 `logo.svg` 存在。

## Commands

### 安装依赖

```bash
cd frontend
bun install --frozen-lockfile

cd ../desktop
go mod download
```

### 生成 bindings

```bash
cd desktop
wails3 generate bindings -d ../frontend/bindings -clean=true -ts
```

### 定向前端测试

```bash
cd frontend
bun run test -- \
  src/api/index.test.ts \
  src/desktop/connection-storage.test.ts \
  src/desktop/ConnectionPage.test.tsx \
  src/desktop/DesktopConnectionGate.test.tsx \
  src/stores/auth-store.test.ts \
  src/lib/errors.test.ts
```

### Go 测试

```bash
cd desktop
go fmt ./...
go vet ./...
go test ./...
```

### 完整回归与构建

```bash
cd frontend
bun run lint
bun run test
bun run build
bun run build:desktop
bun run verify:desktop-assets

cd ../desktop
wails3 build
```

### 开发与手工验证

```bash
cd desktop
wails3 dev
```

本模块不要求修改或运行新的 backend migration。若实现没有修改 `backend/`，不为文档形式上的完整性增加后端接口或测试文件。

## Testing Strategy

### Go unit and integration tests

使用 `httptest.Server` 与直接调用 middleware 覆盖：

- URL trim、规范化、HTTP/HTTPS、可选路径前缀和非法输入。
- 未配置时 `/api`、`/icons` 返回稳定 503。
- `/api/*` 与 `/icons/*` 进入代理，而 `/apix`、`/icons-old`、`/logo.svg` 和 `/wails/*` 进入下一层资产 handler。
- GET、HEAD、POST、PUT、PATCH 和 DELETE 的 method、path、query、body、非 `304` status、content type 和未明确删除的普通 headers 能够转发。
- 上游 Host 使用目标 host。
- `/api` 始终使用当前 Go Token：设置、替换和清除后，上游分别收到新 Bearer 或无 Authorization。
- `SetToken` 在未配置目标时接受空 Token、拒绝非空 Token；配置后可替换或清除 Token，且不改变目标地址。
- `/icons` 不要求 Token；图片字节、content type、ETag、cache headers 和 404 保持不变。
- renderer 的 GET/HEAD 传入 `If-None-Match` 与 `If-Modified-Since` 时，这两个 header 不发给上游且上游返回完整 `200` 资源；PUT 等写请求上的条件 header 仍会转发。测试不把 `304` 列入跨平台转发保证。
- `/api/proxy/image/...` 保留原 path 与 `ref` query，并在 renderer 没有认证能力的资源请求上使用当前 Go Token。
- 请求 Cookie 与响应 Set-Cookie 不跨代理。
- 上游 DNS、拒绝连接或 TLS 错误进入稳定的 `connection_unavailable` 502。
- 上游自己返回的 401、422、500 或 503 保持原状态和响应，不被重写为代理错误。
- `Configure` 更换目标或 `Clear` 返回后发起的新请求不会使用旧目标或旧 Token；不对已经交给上游的在途请求作错误保证。

这些测试不启动真实 WebView，也不把流式 flush 当作已支持能力。

### Frontend tests

- 没有保存地址时清除孤立 Token、显示 `ConnectionPage`，且不发 API 请求。
- 地址只有在 `status` 返回合法 `exists:true` 后才保存。
- `exists:false` 不渲染 `RegisterPage`。
- 保存地址但没有 Token 时，在配置 Go 后显示复用的 `LoginPage`。
- 有效 Token 只有在 `/auth/me` 成功后才能渲染 gate 的 child。
- 过期或无效 Token 清除前端与 Go Token，但保留地址。
- `/auth/me` 的 `500` 或错误响应结构保留 Token，并进入可重试的服务错误状态。
- 上游不可达保留地址和 Token，并显示重试与修改地址操作。
- 登录成功等待 Go Token 同步后才渲染已认证内容。
- 登录时 `SetToken` binding 失败不会保存 Token，也不会渲染已认证内容。
- 新 Token 的 localStorage 写入失败时保持原认证状态，并尝试把 Go Token 恢复为旧值。
- 错误密码的 login `401` 显示原始接口错误，不触发全局未授权清理；未授权清理失败也不能覆盖受保护请求的原始 `ApiError`。
- 桌面 logout 不发远端请求；它与已认证请求的全局 `401` 都清 Token 与 QueryClient，并保留地址。
- 清除 Token 时 binding 失败也保持本地未认证、已卸载 child 和无本地 Token。
- 修改服务地址清地址、Token、auth state、QueryClient 和 Go 运行期状态。
- 修改服务时 `SetToken("")` 或 `Clear()` binding 拒绝也会完成本地清理和导航。
- 启动时 `Configure` 失败保留保存的数据、显示可重试错误，且不会发出 `/api/auth/status` 或 `/api/auth/me` 请求。
- `connection_unavailable` 能被现有网络错误判断识别。
- Web 未注册桌面同步回调时，现有 auth store 行为与测试保持不变。

### Manual desktop verification

在 Windows、macOS 和 Linux 的当前原生环境分别执行短冒烟：

1. 首次启动输入一个已初始化的服务地址并使用现有登录页登录。
2. 确认认证成功后 gate 的已认证 child 被渲染；本模块不要求真实阅读工作区已经实施。
3. 关闭并重新启动应用，确认服务地址与有效会话恢复。
4. 在 DevTools 中确认 `/api/auth/status` 与 `/api/auth/me` 仍是同源相对请求且没有 CORS 错误。

在至少一个原生环境额外完成完整异常流程：

1. 无效地址、不可达地址和未初始化服务给出对应状态，不显示注册。
2. 错误密码留在登录页；正确密码进入已认证内容。
3. 服务不可达时重启，Token 不被删除；服务恢复后重试成功。
4. 退出登录后不等待远端请求，保留地址并返回同一服务的登录页。
5. 修改服务地址后旧查询数据不再出现。

跨三平台的验证集合必须至少包含一个公网 HTTPS 服务和一个局域网 HTTP 服务。HTTPS 证书不受信任时按正常连接失败处理，不提供绕过按钮。

流式 AI 与 OPML 进度不在这轮手工验收中。

## Boundaries

### Always

- 只保存一个当前服务地址与一个当前 Token。
- 先完成 Go 配置与 Token 同步，再允许已认证内容发请求。
- 普通 API 与资源继续使用现有相对 `/api`、`/icons` 路径。
- `exists:false` 只提示 Web 初始化，不进入桌面注册。
- 网络失败保留 Token；`401` 才清除 Token。
- 更换服务时清除旧认证与 Query 缓存。
- Web 注册、登录、PWA、Cookie 和 `VITE_API_URL` 行为保持不变。
- 保留底层连接错误的日志可诊断性，但不记录密码和 Token。

### Ask First

- 改用 WebView 跨域直连、typed Go API client 或本地监听端口。
- 新增 handshake、版本发现、capability 协商或后端 CORS。
- 改变服务地址规范、允许非 HTTP 协议或支持多个保存连接。
- 改用系统凭据库、Go 配置文件或其他持久化来源。
- 改变登录表单、注册范围或服务切换语义。
- 修改现有 Query keys、缓存策略或 Web auth 流程。
- 在本模块建立任意业务 API adapter 或流式桥接。

### Never

- 在桌面进程中启动或嵌入 Gist 服务。
- 把密码写入 localStorage、Go 状态、日志或配置文件。
- 依赖远端 Cookie 作为桌面认证来源。
- 把旧 handshake、instance ID、connection revision 或多连接设计重新带回当前 Spec。
- 增加 Keyring、证书固定、自签名绕过、私网 allowlist 或无明确故障目标的防御层。
- 通过给 Gist 后端开放任意 CORS 来绕过桌面 transport 设计。
- 声称 beta.12 的普通资产代理已经跨平台支持 SSE、NDJSON 或 AI 流。
- 为未来功能创建通用 transport、repository、credential store 或插件层。
- 复制现有 LoginPage、ReaderWorkspace、API 类型或业务 Hooks。

## Success Criteria

- [ ] 首次启动只显示单地址 `ConnectionPage`。
- [ ] 地址经过统一 URL 规范化，并只在 `/api/auth/status` 返回 `exists:true` 后保存。
- [ ] 未初始化服务不显示桌面注册入口。
- [ ] 桌面复用现有 `LoginPage`，Web 登录与注册流程保持现状。
- [ ] 服务地址保存在 `gist_service_url`，Token 继续保存在 `gist_auth_token`。
- [ ] Go 只在内存保存当前地址与 Token，进程退出后不留下第二份凭据。
- [ ] Wails middleware 只接管 `/api` 与 `/icons`，其他前端和 Wails 资产正常加载。
- [ ] 桌面构建不包含固定远端 `VITE_API_URL`，Web 构建对该变量的现有支持保持不变。
- [ ] 普通 API、图标和受保护图片代理通过当前服务地址访问。
- [ ] Cookie 不在 WebView 与远端服务之间传递。
- [ ] 上游不可达得到稳定 `connection_unavailable`，桌面显示可重试的网络错误。
- [ ] 登录成功先同步 Go Token，再渲染 gate 的已认证 child。
- [ ] 应用重启后有效 Token 通过 `/api/auth/me` 恢复会话。
- [ ] 网络失败不删除 Token；`401`、logout 和更换服务会清除 Token。
- [ ] 桌面 logout 不调用无效果的远端 logout，并保留服务地址；Web logout 保持现状。
- [ ] 更换服务即使遇到 binding 清理错误，也会清除本地地址、认证状态与 QueryClient 并返回地址页。
- [ ] connection 的登录与清理调用点等待同一异步 Token helper；任何新 Token 只有在 Go 同步成功后才保存。
- [ ] `@wailsio/runtime` 精确锁定 `3.0.0-beta.12`，bindings 自动生成并提交。
- [ ] 桌面产物包含现有 `logo.svg`，但不包含 PWA、Service Worker 或其他 Web 更新产物。
- [ ] connection 不修改 Gist backend、不新增 CORS 或 handshake。
- [ ] connection 不实现或宣称完成流式业务和其他业务 capability。
- [ ] 定向测试、完整前端回归、Go 测试和当前平台 `wails3 build` 全部通过。
- [ ] Windows、macOS 和 Linux 均完成连接、登录、认证 child 渲染和重启恢复冒烟；至少一个原生环境完成全部异常、退出和更换服务流程。

## Open Questions

无。任何范围变化先更新本文件，再进入后续阶段。

## References

- [`CAPABILITY-MAP.md`](./CAPABILITY-MAP.md)
- [`SPEC-desktop-shell.md`](./SPEC-desktop-shell.md)
- [`SPEC-reader-workspace.md`](./SPEC-reader-workspace.md)
- [`frontend/src/api/index.ts`](./frontend/src/api/index.ts)
- [`frontend/src/stores/auth-store.ts`](./frontend/src/stores/auth-store.ts)
- [`backend/internal/http/router.go`](./backend/internal/http/router.go)
- [`backend/internal/http/middleware.go`](./backend/internal/http/middleware.go)
- [`backend/internal/handler/auth_handler.go`](./backend/internal/handler/auth_handler.go)
- [Wails beta.12 AssetOptions](https://github.com/wailsapp/wails/blob/v3.0.0-beta.12/v3/pkg/application/application_options.go)
- [Wails beta.12 WebView asset requests](https://github.com/wailsapp/wails/blob/v3.0.0-beta.12/v3/internal/assetserver/assetserver_webview.go)
- [Wails beta.12 development asset proxy](https://github.com/wailsapp/wails/blob/v3.0.0-beta.12/v3/internal/assetserver/build_dev.go)
- [Wails beta.12 Windows response writer](https://github.com/wailsapp/wails/blob/v3.0.0-beta.12/v3/internal/assetserver/webview/responsewriter_windows.go)
- [Wails beta.12 Streams guide](https://github.com/wailsapp/wails/blob/v3.0.0-beta.12/docs/src/content/docs/guides/streams.mdx)
