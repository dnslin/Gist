# Wails 3 技术评估

> 调研日期：2026-07-10
> 官方版本基线：`v3.0.0-alpha2.117`
> 官方提交：`3440ae554b8ae3c0fffb7f7a778ba778b6bbf1e5`
> 本机 CLI：`v3.0.0-alpha2.115`

## 当前成熟度

Wails 官方 README 仍将 v3 标为 **Alpha**，不是稳定版。官方 GitHub 最新 v3 release 是 2026-07-08 发布的 `v3.0.0-alpha2.117`；其 tag 指向提交 `3440ae5`。这意味着：

- 可以开发和验证，但公开发布前必须接受预发布 API/工具链变化风险。
- CLI、`github.com/wailsapp/wails/v3` Go 模块和 `@wailsio/runtime` 必须分别精确锁定为经过联合验证的兼容组合，禁止 CI 使用无锁定的 `@latest`。Go/CLI 与 npm runtime 使用不同版本线，不应要求版本号字面一致。
- Wails 代码应隔离在桌面入口与适配层，不能让频繁升级扩散到现有 domain/service 层。

## 2026-07-11 版本复核

- GitHub 官方 Releases API 仍将 `v3.0.0-alpha2.117` 列为最新 v3 release，发布时间为 2026-07-08；本任务继续以它作为 Go module 与 CLI 目标基线，不自动跟随之后的新版本。
- 已将 `v3.0.0-alpha2.117` 下载到本机 Go module cache，校验和为 `h1:udyjqPG3AIgkod5QDR/WblCkpV8R86BFPSrsWxSyt5Y=`。
- `alpha2.115` 与 `alpha2.117` 的 `application_options.go`、`single_instance.go` 和 Windows AssetServer ResponseWriter 相关源码哈希一致，因此本机 `.115` 示例所使用的 AssetServer、单实例与生命周期 API 在目标 `.117` 中未发生变化。
- `alpha2.117` 源码内置 runtime 的 `package.json` 明确声明 `@wailsio/runtime@3.0.0-alpha.97`，npm registry 当前也发布该精确版本；部分旧测试夹具仍保留 `^3.0.0-alpha.79`。首个验证组合因此定为 CLI/Go module `v3.0.0-alpha2.117` + runtime `3.0.0-alpha.97`，并须通过绑定生成、事件、取消和打包 smoke test；不得使用 `latest` 或范围依赖。

## 技术模型

Wails 3 使用 Go 后端和 Web 前端，但不捆绑 Chromium；桌面窗口使用操作系统 WebView：

- Windows：WebView2。
- macOS：WKWebView/WebKit。
- Linux：默认 GTK4 + WebKitGTK 6.0；官方仍提供临时 GTK3/WebKit2GTK 4.1 build tag。

核心能力包括：

- Go service 方法到 TypeScript/JavaScript 的生成绑定。
- Go 与前端之间的事件系统。
- 原生窗口、菜单、对话框、剪贴板、屏幕、单实例和生命周期 API。
- 以 `http.Handler` 作为 AssetServer handler/middleware，或把实现 `http.Handler` 的 service 挂到路由。
- `WebviewWindowOptions.URL` 可以加载指定 URL。
- `server` build tag 可将同一 Wails app 构建成无原生窗口的 HTTP server，但这不是接入桌面的前置条件。

## 平台和构建

官方当前列出的主要支持范围：

- Windows 10/11 amd64/arm64。
- macOS 10.15+ amd64、macOS 11+ arm64。
- Ubuntu 24.04 amd64/arm64，其他 Linux 可能可用。

官方要求 Go 1.24+。项目通过 Task 驱动，常用命令为 `wails3 dev`、`wails3 build` 与 `wails3 package`；包产物包括 Windows NSIS、macOS `.app`、Linux AppImage/deb/rpm。Windows 默认可无 CGO 构建；macOS/Linux WebView 集成需要 CGO，官方建议发布使用各平台原生 CI runner 以完成签名与打包。

本机 `wails3 doctor` 已确认 Windows WebView2 可用且开发环境无阻断问题；签名工具尚未配置，这只影响正式分发，不影响原型验证。

## 与 Gist 的直接适配能力

### 可直接复用的部分

- React/Vite 前端可以继续使用，无需改写 UI 框架。
- Echo 实现 `http.Handler`，可以由 Wails AssetServer middleware 调用，或包装为带 route 的 Wails service。
- 现有 repository/service 层与 Wails 无关，可以由新的桌面组合根原样构造。
- 原生窗口生命周期可承接 scheduler、数据库、后台任务和资源关闭。

### 不能直接原样复用的部分：流式 HTTP

Wails 3 AssetServer 当前明确拒绝 WebSocket。更关键的是，`v3/internal/assetserver/webview.ResponseWriter` 只要求 `http.ResponseWriter + Finish + Code`；Windows、macOS、Linux 的实现均未实现 `http.Flusher`。

Gist 的 Echo handler 会调用 `c.Response().Flush()`。Echo 4.15.2 在底层 writer 不支持 `http.Flusher` 时会 panic。因此，仅把现有 Echo router 挂入 Wails AssetServer，普通 JSON API 可能工作，但 AI SSE/NDJSON 与 OPML 状态流会回归。这是从官方源码和本项目代码共同推导出的结论，必须通过桌面原型测试验证，不能忽略。

## 候选集成方式

### A. 全量改成 Wails bindings/events

优点：无本地端口、类型生成、原生 IPC。
缺点：需要改造全部前端 API 调用、鉴权、上传/下载和流式语义；为了保留 Web/PWA 还要维护第二套 transport。首版回归面最大，不推荐。

### B. 全量把 Echo 挂入 AssetServer

优点：普通 `/api` 路径改动最小，官方支持 `http.Handler`。
缺点：当前 AssetServer 不支持 Gist 所需的 Flush/WebSocket 语义；不能满足“功能不受影响”。排除作为完整方案。

### C. Wails 窗口加载进程内 loopback Echo server

优点：真实 `net/http` 保留所有 REST、SSE、NDJSON、上传下载和 Echo 中间件语义，最接近当前 Web 运行时。
缺点：要处理端口占用、单实例、origin 稳定性、CORS、JWT/cookie、Service Worker 和本地攻击面。随机端口还会改变 localStorage origin；固定端口又存在冲突。可作为低改动原型或回退方案，但正式方案需额外安全设计。

### D. 普通 API 复用 Echo handler，流式能力使用桌面 transport adapter

优点：绝大多数 `/api` 和前端逻辑不变；仅对四类流式操作建立 Wails binding/event 适配，Web 版继续使用原 SSE/NDJSON；不开放本地端口。
缺点：需要证明事件顺序、取消、错误、背压和窗口关闭行为，并在前端 API 边界选择 transport。

在“桌面版本地运行后端、Web/Docker 完全保留”的前提下，D 是当前更稳健的正式方向。用户随后确认桌面不开放 localhost、本地与远程资料库独立且 Web/PWA 必须保留，因此 proposed design 采用 D；C 不再作为正式回退路径，只保留为已拒绝的研究对照。

## 已收敛的设计边界

- 保留 `backend/cmd/server` 和现有 Docker/PWA 发布链。
- 新增独立桌面入口，例如 `backend/cmd/desktop`，Wails 依赖只进入桌面构建图。
- 抽取共享 application kernel，统一创建 DB、repositories、services、router、scheduler 与 closers。
- 桌面专属代码只负责 Wails window、native lifecycle、数据目录、单实例和 transport adapter。
- Web 前端默认行为不变；桌面构建通过明确的 capability/config 选择流式 adapter，并禁用不适合桌面壳的 PWA 更新注册。
- 对 Wails 版本组合做精确锁定，并建立 Windows 10/11 x64 原生构建与 smoke-test 矩阵；其他桌面平台不进入首版支持范围。

## Updater 与 NSIS 的边界

`v3.0.0-alpha2.117` 已内置 `app.Updater`，支持 GitHub Releases 等 provider、更新状态机、下载进度、摘要或公钥签名校验、暂存、重启和 Windows 二进制替换。官方默认窗口仍要求用户执行安装/重启动作，并支持跳过版本或稍后提醒。

需要注意：官方 updater 的默认交付模型是替换单个可执行文件或单顶层归档，不是重新运行 NSIS 安装包。Gist 已确认首版只正式支持 NSIS，因此在没有证明安装目录权限、卸载信息、版本元数据和失败回滚都正确前，不应直接把“Wails 可执行文件自交换”等同于“NSIS 覆盖升级”。较保守的首版路径是自动检查并提示，由用户明确启动新的 NSIS 安装包；框架自更新可在独立原型验证后再启用。

## 官方一手资料

- Wails 官方仓库与版本状态：https://github.com/wailsapp/wails
- `v3.0.0-alpha2.117` release：https://github.com/wailsapp/wails/releases/tag/v3.0.0-alpha2.117
- 安装与平台支持：https://v3.wails.io/getting-started/installation/
- 架构：https://v3.wails.io/concepts/architecture/
- 构建：https://v3.wails.io/guides/build/building/
- 跨平台构建：https://v3.wails.io/guides/build/cross-platform/
- Server build：https://v3.wails.io/guides/server-build/
- 官方 Gin/http.Handler 集成示例：https://v3.wails.io/guides/gin-services/
- `AssetOptions` 官方源码：https://github.com/wailsapp/wails/blob/v3.0.0-alpha2.117/v3/pkg/application/application_options.go
- AssetServer WebView 请求处理：https://github.com/wailsapp/wails/blob/v3.0.0-alpha2.117/v3/internal/assetserver/assetserver_webview.go
- WebView ResponseWriter 接口：https://github.com/wailsapp/wails/blob/v3.0.0-alpha2.117/v3/internal/assetserver/webview/responsewriter.go
- Windows ResponseWriter 实现：https://github.com/wailsapp/wails/blob/v3.0.0-alpha2.117/v3/internal/assetserver/webview/responsewriter_windows.go
- Wails 3 Updater：https://v3.wails.io/guides/updater/
- Updater 官方源码：https://github.com/wailsapp/wails/tree/v3.0.0-alpha2.117/v3/pkg/updater
- Echo `Flush()` 行为：https://github.com/labstack/echo/blob/v4.15.2/response.go
