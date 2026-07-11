---
status: accepted
---

# 桌面普通 API 与流式任务使用混合传输

Wails AssetServer 可以承载 `http.Handler`，但其 WebView ResponseWriter 不实现 `http.Flusher`；直接挂载现有 Echo router 会让依赖 `Flush()` 的 AI 与 OPML 流式 handler 失败。Gist 桌面版因此让普通 `/api` 和 `/icons` 请求继续通过进程内模式感知 `http.Handler`，只把摘要、翻译、批量翻译和 OPML 进度适配为类型化 Wails 任务事件；现有 Web/PWA 仍使用原 SSE/NDJSON，桌面也不开放 localhost 端口。

此决策以独立 feasibility gate 为实施前提：必须使用锁定的 Wails CLI/Go module `v3.0.0-alpha2.117` 与 `@wailsio/runtime@3.0.0-alpha.97` 完成原型，实测 AssetServer 普通请求的完整 request/response body 与文件 upload、事件首块与连续传输、取消、窗口关闭 hook 和通知激活。原型结论必须记录到当前规划任务的 research 文档；任一关键行为不成立时先修订设计和本 ADR，不得把未经验证的框架假设带入功能实现。

## Considered Options

- 全量改成 Wails bindings/events：会重写全部 API、上传下载与认证，并长期维护两套传输。
- 全量挂载 Echo：普通 JSON 可用，但当前 AssetServer 无法保证流式 `Flush()`。
- 启动隐藏 loopback HTTP server：协议最接近现状，但引入端口、origin、CORS、cookie 和本机攻击面。
- 普通 HTTP handler + 流式事件适配（采用）。

## Consequences

- 前端必须在单一 transport 边界选择 browser 或 desktop stream port，业务组件不能自行判断运行环境。
- local 与 remote stream adapter 必须共享类型化事件、`generation`、严格递增 `sequence`、取消和唯一终态契约；Web/PWA 的原 SSE/NDJSON 顺序与错误语义保持不变。
- 前端只能按连续 sequence 推进 ACK。乱序到达的事件必须暂存于有界窗口并在缺口补齐后按序交付；重复或已确认事件必须忽略，不能通过 ACK 越过缺口。缺口超时、窗口溢出或前端长期不可用时必须明确取消或失败，不能静默丢弃、重排或形成无界缓存。
- 文本事件允许有界小批合并，进度事件允许合并到最新状态，但 text/item 事件不得丢失；背压、reload、取消竞态与终态唯一性必须由契约测试覆盖。
- 每次 Wails 升级都要重新运行 feasibility gate；只有上游提供并验证等价 Flusher 语义后，才重新评估是否合并传输路径。

