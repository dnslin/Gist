# Capability Map: Gist 桌面客户端

## 目标

使用 Wails v3 构建跨平台桌面客户端，复用现有 React UI，并覆盖现有 Gist Web 客户端的完整在线功能。

## 能力地图

| Module ID | 职责 | 依赖 |
| --- | --- | --- |
| `desktop-shell` | Wails v3 工程、桌面窗口、前端入口与构建 | — |
| `reader-workspace` | 原样复用现有阅读 UI、样式、Hooks 和 Query 行为 | — |
| `connection` | Gist 服务地址、登录与会话恢复；只连接已初始化的服务 | `desktop-shell` |
| `reading` | 导航数据、未读数、文章列表、正文、图片模式、已读、收藏与桌面原文窗口 | `connection`, `reader-workspace` |
| `library` | 添加、编辑和删除订阅；订阅时创建文件夹，修改文件夹类型或删除文件夹；执行全部刷新 | `connection`, `reader-workspace` |
| `content-tools` | Readability、AI 摘要、翻译和批量翻译 | `connection`, `reader-workspace` |
| `settings-profile` | 用户资料、通用设置、外观设置、网络设置和 AI 设置 | `connection`, `reader-workspace` |
| `data-transfer` | OPML 导入、进度、取消和导出 | `connection`, `reader-workspace` |

## 构建顺序

`desktop-shell` 与 `reader-workspace` 可并行。`desktop-shell` 完成后即可实现 `connection`，不需要等待 `reader-workspace`。其余五个在线能力可在各自依赖稳定后并行实施。

## 固定边界

- Wails 精确锁定为 `v3.0.0-beta.12`。
- Gist 服务单独部署，不嵌入桌面客户端。
- 直接复用现有页面源码；只有连接页面属于新增 UI。
- 桌面构建不包含 PWA、Service Worker 和 Web 更新提示。
- 当前范围不包含首次注册、离线、多连接、托盘、通知、自动更新、签名和公开发布。
- 只实现当前需求，不为假设中的未来需求提前增加抽象。
- 只针对明确的故障或真实系统边界采取必要措施，不增加无具体目标的安全防御设计。
