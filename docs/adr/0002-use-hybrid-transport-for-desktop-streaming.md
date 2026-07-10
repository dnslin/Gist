---
status: proposed
---

# 桌面普通 API 与流式任务使用混合传输

Wails AssetServer 可以承载 `http.Handler`，但其 WebView ResponseWriter 不实现 `http.Flusher`；直接挂载现有 Echo router 会让依赖 `Flush()` 的 AI 与 OPML 流式 handler 失败。Gist 桌面版因此让普通 `/api` 和 `/icons` 请求继续通过进程内模式感知 `http.Handler`，只把摘要、翻译、批量翻译和 OPML 进度适配为类型化 Wails 任务事件；现有 Web/PWA 仍使用原 SSE/NDJSON，桌面也不开放 localhost 端口。

## Considered Options

- 全量改成 Wails bindings/events：会重写全部 API、上传下载与认证，并长期维护两套传输。
- 全量挂载 Echo：普通 JSON 可用，但当前 AssetServer 无法保证流式 `Flush()`。
- 启动隐藏 loopback HTTP server：协议最接近现状，但引入端口、origin、CORS、cookie 和本机攻击面。
- 普通 HTTP handler + 流式事件适配（采用）。

## Consequences

- 前端必须在单一 transport 边界选择 browser 或 desktop stream port，业务组件不能自行判断运行环境。
- local 与 remote stream adapter 必须共享类型化事件、顺序、取消、背压和终态契约。
- 每次 Wails 升级都要重新验证 AssetServer；只有上游提供并验证等价 Flusher 语义后，才重新评估是否合并传输路径。

