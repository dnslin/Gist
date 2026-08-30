# Spec: settings-profile

> 状态：Approved（2026-08-30）
> Module ID：`settings-profile`
> 依赖：`connection`、`reader-workspace`
> 来源：[`CAPABILITY-MAP.md`](./CAPABILITY-MAP.md)

## Objective

让桌面客户端通过 `connection` 已建立的 Gist 服务连接，完整复用现有个人资料与设置界面。

完成本 Spec 后，Web 与桌面端都应当能够：

- 查看用户名，并分别修改昵称、邮箱和密码。
- 修改界面语言、主题、自动 Readability、滚动标记已读和备用 User-Agent。
- 调整内容类型的显示、隐藏和顺序。
- 配置 Gist 服务端的代理与 IP 栈，并测试代理连接。
- 配置 AI Provider、模型、请求参数、目标语言、自动行为和请求速率，并测试 Provider 连接。
- 管理现有域名请求间隔。
- 从现有数据控制页清除文章缓存、图标缓存和 Anubis Cookie。

现有 `ProfileModal` 与 `SettingsModal` 继续共享同一份 React 源码。所有远程设置继续由当前 Gist 服务保存；桌面构建通过相对 `/api` 同源代理访问，Web 保留现有 `VITE_API_URL` 行为，不增加设置专用 Wails binding。

本模块不重新设计设置 UI，不增加桌面本机代理、原生偏好存储、账号注册、头像上传、用户名修改、邮箱验证、账号删除、多用户设置、跨服务偏好同步或新的设置框架。

## Confirmed Decisions

- `settings-profile` 同时验收现有“高级”域名限速，以及数据控制页中的文章缓存、图标缓存和 Anubis Cookie 清理。
- 数据控制页中的 AI 与 Readability 缓存继续由 `content-tools` 验收；OPML 导入导出继续由 `data-transfer` 验收。
- 当前密码错误继续沿用现有 `401`。桌面端按 `connection` 的统一未授权规则清除会话并返回登录页，不为 Profile 单独绕过 `401`。
- 测试已保存的认证代理时，掩码密码表示复用服务端已保存密码，不把掩码文本当成真实密码，也不要求用户每次重新输入。
- 网络设置只控制已连接 Gist 服务的出站请求，不控制桌面客户端到 Gist 服务的连接，也不修改系统代理。
- 主题和界面语言继续保存在当前 WebView 的 `localStorage`；退出、`401` 或更换服务时保留。
- 通用、外观、网络、AI、高级和维护数据继续保存在当前 Gist 服务。更换服务后，Query 数据重新读取；Network 与 Advanced 在组件重挂载后重新请求。
- 个人资料继续使用独立 `ProfileModal`；不合并到设置侧栏。
- API Key 与代理密码继续由服务端保存并以掩码返回。空值或掩码值表示保留现有秘密；本轮不新增清除秘密的操作。
- 修改密码成功后必须等待 `connection` 的统一 Token helper 保存新 Token。若同步或本地保存失败，密码已经在服务端修改且旧 Token 已失效，因此清理本地会话并提示使用新密码重新登录，不重试修改密码请求。
- AI 设置保存成功后立即用服务端响应更新 `['aiSettings']` Query；测试连接不修改 Query，保存失败保留旧 Query。
- 不增加通用表单层、设置事件总线、缓存协调服务、桌面设置 Adapter 或生产依赖。

## Existing UI Contract

### Entry points and ownership

侧栏用户菜单继续提供两个独立入口：

- “个人资料”打开 `ProfileModal`。
- “设置”打开 `SettingsModal`，每次打开默认选择“通用”。

`SettingsModal` 继续保留现有八个页签。业务归属如下：

| 页签或区域 | 负责的 Spec |
| --- | --- |
| 通用、网络、外观、AI、高级 | `settings-profile` |
| 订阅源、文件夹 | `library` |
| 数据控制：文章、图标、Anubis | `settings-profile` |
| 数据控制：AI、Readability | `content-tools` |
| 数据控制：OPML | `data-transfer` |

这种归属只用于验收现有组件中的行为。不得拆成多个设置弹窗、增加 capability flag，或复制 `DataControl`。

桌面宽度继续使用现有侧栏弹窗布局，窄窗口继续使用现有全屏设置布局和下拉页签。响应式断点、尺寸、图标、样式和文案结构保持不变。

### Profile

个人资料保持以下行为：

- 用户名只读，不提供修改入口。
- 昵称与邮箱各自拥有独立输入和保存按钮；任一保存成功后更新 auth store 中的 `user`。
- 保存一个字段不得覆盖另一个字段尚未保存的草稿。
- 前端裁剪昵称和邮箱并阻止空值提交；API 中缺省或空值继续表示不修改该字段。不新增邮箱验证邮件或格式确认流程。
- 密码表单包含当前密码、新密码和确认密码。
- 前端在提交前确认两次新密码一致且至少 `6` 个字符。
- 服务端继续要求当前密码，并拒绝与旧密码相同的新密码。

修改密码的结果按以下顺序处理：

1. `PUT /api/auth/profile` 成功并返回新 Token。
2. 显式 `await setAuthToken(newToken)`，先同步桌面 `ConnectionService`，再保存到 `localStorage`。
3. Token 保存成功后清空密码输入并显示成功状态，当前登录保持有效。
4. Token 同步或保存失败时，不把密码修改描述成失败，也不重复提交；清空密码输入，执行 `connection` 的本地会话清理，并在登录页提示“密码已修改，请使用新密码重新登录”。

用户输入错误当前密码时，服务端继续返回 `401`。按已确认选择，这与其他受保护请求的 `401` 使用同一清理流程；Profile 不设置 `handleUnauthorized: false`，也不增加专用错误状态码。

### General settings

通用设置保持现有字段与保存方式：

| 设置 | 存储位置 | 保存行为 |
| --- | --- | --- |
| 界面语言 `zh` / `en` | `localStorage['gist-lang']` | 选择后立即应用 |
| 自动 Readability | Gist `general.auto_readability` | 切换后立即保存 |
| 滚动标记已读 | Gist `general.mark_read_on_scroll` | 切换后立即保存 |
| 备用 User-Agent | Gist `general.fallback_user_agent` | 点击保存 |

语言没有本地值时继续按浏览器语言选择中文或英文。改变语言立即更新 i18n 与根元素 `lang`，不请求服务端。

远程字段继续通过一个完整 `GeneralSettings` 对象保存。成功后立即把服务端响应写入 `['generalSettings']` Query；失败时恢复最后一次服务端值并显示现有样式的行内错误。加载失败时显示可重试错误，不允许用户把默认空表单误保存到服务端。

本模块不改变 `autoReadability` 与 `markReadOnScroll` 的业务执行规则；它们分别由 `content-tools` 与 `reading` 使用。

### Appearance settings

外观设置保持两类不同持久化来源：

- 主题 `system`、`light`、`dark` 保存在 `localStorage['gist-theme']`，选择后立即应用。
- 有序的内容类型子集保存在当前 Gist 服务的 `appearance.content_types`。

内容类型只允许 `article`、`picture`、`notification`，去重后至少保留一种。用户可以拖动排序、隐藏或重新加入；成功后立即更新 `['appearanceSettings']` Query。

远程内容类型加载失败时显示可重试错误，并禁用排序、隐藏和保存；不得用默认三种类型覆盖服务端设置。主题是独立本地偏好，仍可正常切换。

当前内容类型被隐藏时，继续由现有 `App` 逻辑使用 replace 导航切换到第一个可见类型。改变顺序只影响之后的默认类型，不主动改变仍然可见的当前类型。

现有 `150ms` 合并保存可以保留，但关闭弹窗、切换页签或卸载组件不得丢失最后一次修改。保存失败时恢复最后一次服务端顺序并显示行内错误，不允许界面继续展示尚未保存的顺序。

主题继续跟随系统深浅色变化。主题和语言不上传到 Gist，也不按服务地址增加命名空间。

### Network settings

网络页继续配置远端 Gist 服务使用的出站网络：

- 启用或关闭代理。
- HTTP 或 SOCKS5。
- 主机与端口。
- 可选用户名与密码。
- `default`、`ipv4` 或 `ipv6` IP 栈。

“启用代理”和 IP 栈继续在选择后立即保存当前完整 `NetworkSettings` 草稿；因此同一表单中尚未点击保存的代理字段也会随这次请求提交。代理类型、主机、端口和认证字段仍可通过现有保存按钮提交。测试按钮使用当前表单草稿，不要求先保存。

网络页加载失败时显示可重试错误，并禁用保存与测试；不得以默认空配置替代加载失败的真实服务端配置。

代理密码契约保持简单：

- `GET /api/settings/network` 只返回掩码。
- PUT 中空值或掩码值保留服务端已保存密码。
- 输入新值时替换服务端密码。
- 本轮不增加“清除代理密码”按钮或额外请求字段。
- POST 测试中的掩码值由服务端解析为已保存密码；没有保存密码时按无密码测试。

代理关闭时服务端继续直连。代理测试继续使用服务端固定测试地址；业务失败返回 `200` 与 `{success:false,error}`，请求结构错误返回 `400`。测试结果不自动保存表单。

本页不修改 `gist_service_url`，不影响 Wails `ConnectionService`，也不读取操作系统代理设置。

### AI settings

AI 设置继续提供：

- Provider：`openai`、`anthropic`、`compatible`。
- API Key、Base URL、模型和 JSON object 请求参数。
- 摘要/翻译目标语言：`zh-CN`、`zh-TW`、`en-US`、`ja`、`ko`、`es`、`fr`、`de`。
- 自动翻译与自动摘要开关。
- 每秒请求速率 `1` 到 `100`。

`openai` 与 `compatible` 继续要求非空 Base URL；`anthropic` 可以为空。请求参数必须是 JSON object，不接受数组、标量或损坏 JSON。

API Key 使用与代理密码一致的掩码语义：读取时只返回掩码；API 的保存请求中空值或掩码值保留服务端已有 Key；测试时掩码值复用服务端已有 Key；输入新值时替换。现有表单不允许空 API Key 的草稿保存或测试。桌面 WebView 和 Wails Go 进程都不得读取服务端真实 Key。

“测试”使用当前草稿调用现有接口，不保存设置。Provider 测试失败继续返回 `200` 与 `{success:false,error}`；表单或请求不合法返回 `400`。

“保存”成功后：

```ts
const saved = await updateAISettings(payload);
queryClient.setQueryData(["aiSettings"], saved);
setSettings(saved);
```

服务端响应是唯一已保存真值。保存失败时保留旧 Query，不让其他阅读组件观察到未保存草稿。目标语言变化后的执行行为继续由 `content-tools` 的已批准契约负责。

Provider、模型、请求参数或目标语言变化不自动清除已有 AI 缓存，也不触发后台重算。

### Advanced domain rate limits

“高级”页继续使用现有域名请求间隔 CRUD：

- 列出所有规则。
- 输入有效域名、IP 或 `localhost` 创建规则。
- 修改既有规则的间隔秒数。
- 经现有确认后删除规则。
- `0` 秒表示不增加等待。

创建、修改和删除成功后重新读取当前列表即可；不要求迁移到 TanStack Query。失败时保留当前列表和编辑草稿，并显示行内错误，不再静默忽略。

域名规则继续作用于 Gist 服务端抓取流程。本模块不增加通配符、路径规则、规则优先级、导入导出或桌面本地限速。

### Remaining DataControl actions

现有数据控制页继续保留所有区域。本模块只验收以下三项：

| 操作 | 服务端行为 | 成功后的客户端行为 |
| --- | --- | --- |
| 清除文章缓存 | 删除未收藏文章，保留收藏文章，并重置 Feed 条件请求元数据 | 显示删除数；失效 `['entries']`、`['entry']`、`['unreadCounts']` |
| 清除图标缓存 | 删除服务端图标缓存，清除 Feed 图标路径并重置 Feed 条件请求元数据 | 显示删除文件数；失效 `['feeds']` |
| 清除 Anubis Cookie | 删除服务端保存的 Anubis 设置记录 | 显示删除的设置记录数；不额外失效业务 Query |

操作期间只禁用当前按钮。成功或失败沿用现有结果区域，不增加全局通知、维护任务队列、撤销、事务或自动刷新 Feed。

AI/Readability 清理继续遵守 `content-tools`；OPML 继续由 `data-transfer` 定义。本模块不得重复定义它们的请求、状态或缓存语义。

## API Contract

本模块继续使用现有受 JWT 保护的有限 JSON API：

| Method | Path | 用途 |
| --- | --- | --- |
| `GET` | `/api/auth/me` | 读取当前用户 |
| `PUT` | `/api/auth/profile` | 修改昵称、邮箱或密码 |
| `GET` / `PUT` | `/api/settings/general` | 读取或保存通用设置 |
| `GET` / `PUT` | `/api/settings/appearance` | 读取或保存内容类型顺序 |
| `GET` / `PUT` | `/api/settings/network` | 读取或保存服务端网络设置 |
| `POST` | `/api/settings/network/test` | 测试当前代理草稿 |
| `GET` / `PUT` | `/api/settings/ai` | 读取或保存 AI 设置 |
| `POST` | `/api/settings/ai/test` | 测试当前 AI 草稿 |
| `GET` | `/api/domain-rate-limits` | 列出域名规则 |
| `POST` | `/api/domain-rate-limits` | 创建域名规则 |
| `PUT` | `/api/domain-rate-limits/:host` | 修改域名规则 |
| `DELETE` | `/api/domain-rate-limits/:host` | 删除域名规则 |
| `DELETE` | `/api/entries/cache` | 清除未收藏文章 |
| `DELETE` | `/api/icons/cache` | 清除图标缓存 |
| `DELETE` | `/api/settings/anubis-cookies` | 清除 Anubis Cookie |

桌面构建继续通过 `connection` 的相对 `/api` 同源代理和 Bearer Token；Web 继续使用现有 `VITE_API_URL` 与请求路径。不得增加业务 Wails binding、第二套桌面 API client、CORS 或桌面跨域直连。

现有 JSON 字段、状态码和默认值保持不变，只有以下已确认修正：

- 网络测试接收掩码密码时复用已保存密码。
- Profile 返回新 Token 时等待异步 Token helper。
- 当前密码错误仍返回 `401`，并触发统一未授权清理。

AI 与代理测试的业务失败继续用成功 HTTP 响应中的 `success:false` 表达。其他接口的非成功状态继续转换成现有 `ApiError`，保留状态和服务端错误文本。

## Persistence and State Contract

| 数据 | 真源 | 客户端状态 |
| --- | --- | --- |
| 当前用户 | Gist `user.*` settings | auth store `user` |
| 主题 | 当前 WebView `localStorage` | `useTheme` |
| 界面语言 | 当前 WebView `localStorage` | i18n |
| General | 当前 Gist 服务 | `['generalSettings']` |
| Appearance | 当前 Gist 服务 | `['appearanceSettings']` |
| AI | 当前 Gist 服务 | `['aiSettings']` |
| Network、Advanced | 当前 Gist 服务 | 当前设置组件状态 |
| API Key、代理密码 | 当前 Gist 服务 | 仅掩码草稿 |
| JWT | `connection` 统一 Token helper | WebView + 桌面运行期同步 |

退出、`401` 或更换服务时，远程 Query 按 `connection` 清理。主题和语言不清理。不得为单连接模式增加 service ID、setting revision、同步冲突或多连接命名空间。

## Tech Stack

继续使用仓库已有依赖：

| 层 | 现有技术 |
| --- | --- |
| Shared frontend | React、TanStack Query、Zustand、i18next、现有 Dialog 与表单组件 |
| Desktop transport | `connection` 的 Wails 同源代理 |
| Backend | Echo、现有 Auth/Settings/DomainRateLimit service 与 repositories、SQLite |
| Tests | Vitest、Testing Library、Go `testing`、testify、GoMock |

本模块不增加表单库、schema validator、设置状态库、通知库、HTTP client、Keyring、数据库 migration 或 Wails service。

## Commands

以下命令是本模块实施后必须成立的验证契约。

### Targeted frontend tests

```bash
cd frontend
bun run test -- src/components/settings src/stores/auth-store.test.ts src/api/index.test.ts
```

### Targeted backend tests

```bash
cd backend
go test ./internal/handler ./internal/service ./internal/repository
```

### Full frontend verification

```bash
cd frontend
bun run lint
bun run test
bun run build
```

### Full backend verification

```bash
cd backend
go test ./...
```

### Current-platform desktop build

```bash
cd desktop
wails3 build
```

## Project Structure

以下是本模块可能涉及的现有位置。只修改实现所需文件，并让新增测试与对应组件相邻：

```text
frontend/src/
├── api/index.ts
├── components/settings/
│   ├── ProfileModal.tsx
│   ├── SettingsModal.tsx
│   ├── SettingsSidebar.tsx
│   └── tabs/
│       ├── ProfileSettings.tsx
│       ├── GeneralSettings.tsx
│       ├── AppearanceSettings.tsx
│       ├── NetworkSettings.tsx
│       ├── AISettings.tsx
│       ├── AdvancedSettings.tsx
│       └── DataControl.tsx
├── hooks/
│   ├── useTheme.ts
│   ├── useGeneralSettings.ts
│   ├── useAppearanceSettings.ts
│   └── useAISettings.ts
├── stores/
│   └── auth-store.ts
└── types/
    └── settings.ts

backend/internal/
├── handler/
│   ├── auth_handler.go
│   ├── settings_handler.go
│   └── domain_rate_limit_handler.go
├── service/
│   ├── auth_service.go
│   ├── settings_service.go
│   └── domain_rate_limit_service.go
└── repository/
    ├── settings_repository.go
    └── domain_rate_limit_repository.go
```

测试继续与对应源码相邻。不得为了本模块拆分整个 API client、移动设置目录、复制组件到 `desktop/`，或重排后端现有 handler/service/repository 分层。

## Code Style

### Frontend

- 沿用 strict TypeScript、`@/` 别名、现有 i18n、TanStack Query 和 Zustand 约定。
- 组件状态只保存当前表单草稿；服务端 Query 继续是远程设置真源。
- 使用服务端返回对象更新 Query，不手工拼接另一份“已保存设置”。
- 失败必须保留服务端错误文本或现有本地化错误，不再静默忽略。

```ts
const saved = await updateGeneralSettings(payload);
queryClient.setQueryData(["generalSettings"], saved);
setDraft(saved);
```

### Backend

- 保持 Echo handler → service → repository 分层。
- 保持现有 JSON camelCase、错误映射和日志字段。
- 只为掩码代理测试补充具体的已保存密码解析，不建立 secret manager。
- 不记录 JWT、API Key、代理密码或完整认证 URL。

## Testing Strategy

### Frontend component tests

使用 Vitest、Testing Library 和现有 Provider wrapper 覆盖可观察行为：

- Profile 分别保存昵称与邮箱，更新 auth store，并保留另一个未保存草稿。
- 密码本地校验、新 Token 的异步保存顺序、成功状态和 Token 同步失败后的重新登录流程。
- 当前密码错误 `401` 触发统一未授权清理。
- General 加载、即时开关、备用 UA 保存、Query 更新、失败回滚和错误显示。
- 语言只写 `gist-lang`，主题只写 `gist-theme`，二者不发 API 请求。
- Appearance 排序、隐藏、至少保留一项、最后一次保存不因卸载丢失、失败回滚和当前类型切换。
- Network 加载、即时字段、显式保存、测试草稿、掩码密码复用以及失败结果。
- AI JSON object 校验、Provider/Base URL 约束、测试不改 Query、保存立即更新 `['aiSettings']`。
- Advanced 列表、创建、修改、确认删除和错误保留。
- 三项 DataControl 操作的 loading、计数、错误与精确 Query invalidation。

组件测试模拟 API 与 Token helper，不启动 Wails，不访问真实 Gist、代理、Apple 测试地址或 AI Provider。

### Backend tests

使用现有 Go 测试工具覆盖：

- Profile 字段更新、密码验证、JWT secret 轮换与新 Token 返回。
- 当前密码错误继续映射为 `401`。
- General 默认值与完整保存。
- Appearance 默认顺序、去重、非法值和至少保留一项。
- Network 默认值、掩码保存、掩码测试复用真实密码和业务失败响应。
- AI 默认值、掩码 Key、Provider/Base URL、JSON options、速率更新和测试响应。
- 域名规则的有效 host、非负间隔和 CRUD。
- 文章清理返回删除的文章数，图标清理返回删除的文件数，Anubis 清理返回删除的设置记录数。

所有出站测试使用 mock、fake client 或 `httptest.Server`。不要求真实 API Key、认证代理或公网网络。

### Manual verification

在当前开发平台的桌面构建完成完整流程：

1. 修改昵称和邮箱，确认侧栏资料立即更新。
2. 修改密码，确认新 Token 保持当前登录；再用错误当前密码验证已确认的退出行为。
3. 修改语言、主题、General 和内容类型，重启后确认各自持久化来源正确。
4. 保存并测试无认证代理、已保存认证代理和错误代理。
5. 测试并保存 AI 设置，确认阅读端立即观察新目标语言和自动开关。
6. 创建、修改并删除域名限速规则。
7. 分别执行文章、图标和 Anubis 清理，确认计数和页面状态更新。

Web 只需完成入口、资料显示和一次 General 保存冒烟。设置 UI 使用同一份 React 源码，本模块不重复 `desktop-shell` 与 `connection` 已定义的三平台验收。

## Boundaries

### Always

- 复用现有 Profile、Settings 和 DataControl 组件。
- 桌面远程设置通过 `connection` 的相对 `/api` 代理访问；Web 保留现有 `VITE_API_URL`。
- 保存成功后以服务端响应更新对应 Query 或组件状态。
- 修改密码返回的新 Token 必须等待统一 Token helper。
- 主题与语言保留为当前设备偏好。
- API Key 与代理密码只向客户端返回掩码。
- 对加载、保存、测试和维护失败显示可诊断错误。
- 保持 Web 与桌面共享行为，除 `connection` 已定义的 Token 同步差异外不增加宿主分支。

### Ask First

- 改变 Profile 与 Settings 的入口、页签或布局。
- 让主题、语言或其他设置跨客户端同步。
- 改变当前密码错误触发退出的已确认行为。
- 增加桌面本机代理、系统代理读取或连接代理。
- 增加 Provider、AI 字段、内容类型、域名规则类型或 secret 清除语义。
- 修改现有 Query keys、API 字段、状态码、数据库 schema 或认证流程。
- 把新的 DataControl 行为加入本模块。

### Never

- 复制设置 UI 到 `desktop/`。
- 把真实 API Key 或代理密码保存到 WebView、Wails Go 状态、日志或桌面配置。
- 从桌面端直接调用 AI Provider、订阅源或代理测试地址。
- 为设置增加通用 Adapter、repository、schema registry、事件总线或插件系统。
- 增加多连接设置命名空间、revision、冲突合并或离线设置副本。
- 为当前单用户设置增加角色权限、审计系统或无明确需求的安全防御。
- 静默吞掉设置加载、保存、测试或清理失败。
- 自动清除 AI 缓存、后台重算内容或修改其他 capability 的缓存语义。

## Success Criteria

- [ ] Profile 与 Settings 继续从现有侧栏入口打开，UI 未复制或重新设计。
- [ ] 用户名只读；昵称、邮箱和密码按现有独立操作保存。
- [ ] 保存昵称或邮箱不会覆盖另一个未保存草稿。
- [ ] 修改密码成功后新 Token 同步到桌面运行期与本地存储，并保持登录。
- [ ] 新 Token 同步失败时清理失效会话，并提示使用新密码登录。
- [ ] 错误当前密码继续返回 `401` 并触发统一退出。
- [ ] 语言与主题只保存在本机，退出或更换服务后保留。
- [ ] General 保存后立即更新 `['generalSettings']`，失败时回滚并显示错误。
- [ ] 内容类型保持有效有序子集，最后一次修改不会因关闭或切页丢失。
- [ ] 隐藏当前内容类型后切换到第一个可见类型。
- [ ] 网络设置只影响 Gist 服务端出站请求，不影响桌面连接或系统代理。
- [ ] 已保存认证代理可以在不重新输入密码时正确测试。
- [ ] AI 测试不保存；AI 保存立即更新 `['aiSettings']`，后续执行行为继续遵守 `content-tools`。
- [ ] API Key 与代理密码只显示掩码，空值/掩码值保留已有秘密。
- [ ] 域名限速现有 CRUD 在 Web 与桌面可用。
- [ ] 文章、图标和 Anubis 清理显示各接口定义的删除数并完成对应 Query 更新。
- [ ] AI/Readability 清理仍由 `content-tools` 定义，OPML 仍由 `data-transfer` 定义。
- [ ] 加载失败不会用默认空表单覆盖服务端设置；保存和测试失败不会伪装成成功。
- [ ] 未新增业务 Wails binding、桌面设置副本、生产依赖、数据库 migration 或通用设置架构。
- [ ] 定向测试、完整前后端测试、前端 lint/build 和当前平台桌面 build 全部通过。
- [ ] 当前开发平台桌面完成完整手工回归，Web 完成入口与一次保存冒烟。

## Open Questions

无。本文件已经批准。按照既定工作流，继续完成 `data-transfer` Spec；全部 capability Spec 就绪后再统一进入任务拆分。

## References

- [`CAPABILITY-MAP.md`](./CAPABILITY-MAP.md)
- [`SPEC-connection.md`](./SPEC-connection.md)
- [`SPEC-reader-workspace.md`](./SPEC-reader-workspace.md)
- [`SPEC-reading.md`](./SPEC-reading.md)
- [`SPEC-library.md`](./SPEC-library.md)
- [`SPEC-content-tools.md`](./SPEC-content-tools.md)
- [`frontend/src/components/settings/SettingsModal.tsx`](./frontend/src/components/settings/SettingsModal.tsx)
- [`frontend/src/components/settings/ProfileModal.tsx`](./frontend/src/components/settings/ProfileModal.tsx)
- [`frontend/src/components/settings/tabs/ProfileSettings.tsx`](./frontend/src/components/settings/tabs/ProfileSettings.tsx)
- [`frontend/src/components/settings/tabs/GeneralSettings.tsx`](./frontend/src/components/settings/tabs/GeneralSettings.tsx)
- [`frontend/src/components/settings/tabs/AppearanceSettings.tsx`](./frontend/src/components/settings/tabs/AppearanceSettings.tsx)
- [`frontend/src/components/settings/tabs/NetworkSettings.tsx`](./frontend/src/components/settings/tabs/NetworkSettings.tsx)
- [`frontend/src/components/settings/tabs/AISettings.tsx`](./frontend/src/components/settings/tabs/AISettings.tsx)
- [`frontend/src/components/settings/tabs/AdvancedSettings.tsx`](./frontend/src/components/settings/tabs/AdvancedSettings.tsx)
- [`frontend/src/components/settings/tabs/DataControl.tsx`](./frontend/src/components/settings/tabs/DataControl.tsx)
- [`backend/internal/handler/auth_handler.go`](./backend/internal/handler/auth_handler.go)
- [`backend/internal/handler/settings_handler.go`](./backend/internal/handler/settings_handler.go)
- [`backend/internal/handler/domain_rate_limit_handler.go`](./backend/internal/handler/domain_rate_limit_handler.go)
- [`backend/internal/service/auth_service.go`](./backend/internal/service/auth_service.go)
- [`backend/internal/service/settings_service.go`](./backend/internal/service/settings_service.go)
