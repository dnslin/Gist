# Gist 当前项目基线

> 调研日期：2026-07-10
> 工作分支：`dev`
> 调研时 HEAD：`63dec1a`

## 产品定位

Gist 是一个轻量级、单实例单用户、自托管的 RSS 阅读器。当前交付物是可独立部署的 Web 应用，不是桌面客户端：Go 进程同时提供 HTTP API 与 React 静态资源，官方发布路径是多架构 Docker 镜像，前端同时支持 PWA 安装。

当前用户可见能力包括：

- RSS 2.0、Atom、JSON Feed 订阅、预览、增删改与手动/定时刷新。
- Article、Picture、Notification 三种内容类型及同类型文件夹层级。
- 文章列表、全文检索、未读/已读、批量标记、收藏与计数。
- Readability 正文抓取、缓存与清理。
- AI 摘要、文章翻译、列表批量翻译和缓存；支持 OpenAI、Anthropic 与兼容接口，用户自行提供密钥。
- OPML 导入、取消、进度跟踪与导出。
- Feed 图标缓存、图片代理、Anubis 防护处理、代理/IP 栈与按域名限速。
- 单用户注册、登录、JWT、个人资料与密码更新。
- 浅色/深色/跟随系统、多语言、响应式三栏/移动布局、PWA 更新提示。

依据：`README.md`、`frontend/src/App.tsx`、`frontend/src/components/settings/`、`backend/internal/handler/*_handler.go`。

## 当前运行架构

### 后端

`backend/cmd/server/main.go` 是组合根，同时负责：

1. 读取 `GIST_*` 环境变量并初始化日志和 Snowflake ID。
2. 打开并自动迁移 SQLite 数据库。
3. 创建 repository、service、handler 与 Echo router。
4. 启动图标回填后台任务和每 15 分钟一次的 Feed scheduler。
5. 启动 HTTP 服务并处理 SIGINT/SIGTERM 优雅退出。

业务层已经按 `handler -> service -> repository -> SQLite` 分层，构造函数注入明确；真正需要共享的是组合根和生命周期，不需要重写现有业务服务。

数据库使用 `modernc.org/sqlite`，不依赖 CGO，默认文件为 `./data/gist.db`，启用 WAL、外键、30 秒 busy timeout 与 synchronous NORMAL。schema 包含 folders、feeds、entries、FTS5、settings、AI 缓存和 domain rate limits；启动时执行向前迁移。

Echo router 的契约为：

- `/api/auth/status|register|login|logout` 为公开 API。
- 其余 `/api/**` 经 JWT 中间件保护。
- `/icons/**` 由独立静态图标路由提供。
- 非 API 路径回退到 React `index.html`，从而支持前端 history 路由。

### 前端

前端是 React 19 + TypeScript + Vite 8：

- TanStack Query 管理服务器状态，Zustand 管理认证和局部 UI 状态。
- Wouter 管理路由，Radix UI/Tailwind 提供组件与样式。
- `frontend/src/api/index.ts` 是主要传输边界，默认 `API_BASE_URL` 为空，因此全部使用同源 `/api/**`。
- JWT 存在 `localStorage.gist_auth_token`，请求使用 Bearer header；后端也设置 cookie 以支持图片等浏览器资源请求。
- AI 与导入进度依赖流式 `fetch`：SSE 或 NDJSON 通过 `ReadableStream.getReader()` 增量读取。
- Vite PWA 生成 Service Worker；PWA 更新提示、资源缓存和移动端恢复逻辑是现有 Web 版行为的一部分。

## 流式接口是兼容性硬边界

以下现有能力依赖服务端及时 `Flush()`：

- `POST /api/ai/summarize`
- `POST /api/ai/translate`
- `POST /api/ai/translate/batch`
- `GET /api/opml/import/status`

对应代码位于 `backend/internal/handler/ai_handler.go` 与 `backend/internal/handler/opml_handler.go`。任何桌面承载方式都必须验证首块到达时间、连续增量、取消、错误事件和完成事件，不能只验证最终响应正文。

## 既有交付与兼容面

“不影响现有功能”至少需要保护以下独立兼容面：

1. **Web/Docker 交付**：现有 `gist-server`、Docker Compose、环境变量和 PWA 继续工作。
2. **HTTP API**：路径、方法、鉴权、状态码、JSON、SSE/NDJSON 和文件上传/下载保持兼容。
3. **数据**：现有 SQLite 文件可继续升级和读取；桌面版不能意外占用或迁移 Web 服务的数据目录。
4. **后台行为**：15 分钟调度、手动刷新、图标回填、取消与优雅关闭保持语义。
5. **浏览器行为**：PWA、Service Worker、移动布局与同源资源访问不被桌面条件分支破坏。
6. **发布行为**：原有 Linux amd64/arm64 Docker CI 不被桌面构建依赖拖入 CGO 或 GUI 依赖。

## 当前自动化基线

静态盘点：

- 后端：72 个 `_test.go` 文件，约 545 个测试/示例/基准函数。
- 前端：37 个测试文件；本次实际执行为 441 个 Vitest 测试。
- CI：后端 build + race test + lint；前端 build + coverage test + lint/typecheck。

2026-07-10 本机验证：

- `go build ./...`：通过。
- `go test ./...`：失败，存在两个迁移前基线问题：
  - `internal/config.TestLoad` 写死 Unix 路径，Windows 上实际为 `\\tmp\\gist`。
  - `internal/service.TestIconService_IsValidIconPath` 有一个 Windows 路径判定断言失败。
- `bun run build`：通过，有既有的大 chunk 与 browserslist 数据陈旧警告。
- `bun run test`：37 个文件、441 个测试全部通过。

桌面改造开始前应先决定：把两个 Windows 测试缺口单独修复为前置任务，还是把它们明确列为已知基线并在桌面 CI 中隔离。不能把它们事后误归因于 Wails。

## 对后续设计的直接约束

- 保留 `cmd/server` 作为原 Web/Docker 入口；新增桌面入口比替换入口风险更低。
- 将现有组合根抽成可复用的 application kernel/runtime，供 server 与 desktop 两个入口装配；业务 service/repository 应继续复用。
- 桌面数据目录、认证方式、流式传输和目标平台需要逐项确认，它们不能从“客户端”一词推断。
- 桌面条件逻辑应集中在适配层或构建入口，避免在已有业务服务和 Web UI 中散布平台判断。
