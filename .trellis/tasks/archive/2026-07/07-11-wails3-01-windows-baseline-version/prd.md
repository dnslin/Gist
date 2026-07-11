# Child 01：Windows 测试基线与统一产品版本源

> 状态：planning；尚未启动实现
> 父任务：`.trellis/tasks/07-10-wails3-client-planning`

## 1. 目标

本 Child 在引入 Wails、桌面 shell、共享 Runtime 或安装器之前，建立可归因、可重复的 Windows 迁移前测试基线，并把仓库根 `VERSION` 建立为当前产品版本的唯一来源。完成后，现有 Go、Vite/前端 package metadata、Swagger/未来 compatibility metadata 与 Docker image metadata 具有统一、失败关闭的版本注入机制；现有 Web/PWA/server/Docker 行为保持不变。

## 2. 继承范围

- **REQ-C01-BASELINE-01**（继承 `REQ-COMPAT-01`、`AC-PROD-01`）：在任何桌面或 Wails 代码进入构建图之前，复现并记录现有 Windows 后端测试结果。研究中已观察到的两项失败必须逐项分类、固定输入与命令，并建立回归测试；不得跳过、隔离、静默放行或归因于 Wails：
  1. `internal/config.TestLoad` 使用 Unix 字面路径，Windows `filepath.Clean/Join` 产生 Windows 路径；
  2. `internal/service.TestIconService_IsValidIconPath` 对 `/abs/icon.png` 的平台语义与断言不一致。
- **REQ-C01-BASELINE-02**（继承 `REQ-COMPAT-01`、`AC-PROD-01`）：两项失败的修复必须明确区分“测试夹具不可移植”与“生产路径安全合同缺陷”。配置测试应断言宿主平台的规范化路径；图标路径校验应保持跨平台拒绝绝对路径、父目录逃逸和非本平台分隔符绕过的安全合同。不得仅删除断言、按 `runtime.GOOS` 跳过或放宽路径校验。
- **REQ-C01-VERSION-01**（继承 `REQ-SCOPE-04`、`REQ-COMPAT-04`、`AC-PROD-20`）：仓库根 `VERSION` 是稳定产品版本的唯一人工维护来源，内容为单行、无 `v` 前缀的 SemVer `X.Y.Z`。其他位置不得继续作为可独立编辑的产品版本真相。
- **REQ-C01-VERSION-02**：当前已存在消费者必须由 `VERSION` 注入或校验：Go `config.AppVersion`/user agent、Vite 构建常量、`frontend/package.json` 版本、Swagger metadata，以及 Docker OCI image version label。消费者得到的值必须逐字一致。
- **REQ-C01-VERSION-03**：compatibility metadata 的产品版本边界必须复用同一 Go 构建版本值；本 Child 只提供可被后续 compatibility endpoint 消费的版本访问机制，不创建 endpoint、API major 或 capability。后续 NSIS 只能从发布流水线接收同一已验证版本；本 Child 不创建 NSIS、Wails 配置、launcher 或虚构安装器文件。
- **REQ-C01-VERSION-04**：CI/脚本必须在根 `VERSION`、稳定 Tag `vX.Y.Z`、前端 package metadata、Go/Vite 注入值或 Docker label 不一致时失败关闭；非 Tag 的普通构建仍从 `VERSION` 获得确定版本，不依赖 Git 可用性或工作目录。
- **REQ-C01-COMPAT-01**（继承 `REQ-COMPAT-01`～`04`、`AC-PROD-01`）：不得改变 HTTP 路由、认证、状态码、JSON/SSE/NDJSON、SQLite schema/迁移、scheduler、server listener、`GIST_*` 配置、PWA/Service Worker、Docker entrypoint、数据目录或现有镜像平台。
- **REQ-C01-RELEASE-BOUNDARY-01**（继承 `REQ-RELEASE-01`～`02`、`AC-PROD-17`）：本 Child 仅为后续 Windows 安装资产提供统一版本输入；不声明、实现或验收 fixed install path、repair、upgrade、uninstall、UAC、registry、shortcut 或数据清理行为。
- **REQ-C01-RELEASE-BOUNDARY-02**（继承 `REQ-RELEASE-06`～`07`、`AC-PROD-20`）：本 Child 只验收“同版本、同提交、失败关闭”的版本前置条件；Authenticode、应用内签名、stable Release 原子发布与完整资产矩阵仍由 Child 14 负责。

## 3. 非目标

- 不引入 Wails CLI、Go module、runtime、bindings、WebView、桌面入口或 shell。
- 不抽取 `application.Runtime`，不调整 server composition root、生命周期或 scheduler。
- 不创建 compatibility endpoint、API major/capability 协议、NSIS、launcher、更新器、签名或发布资产。
- 不修改产品业务行为、数据库、HTTP API、PWA 或 Docker 运行契约。
- 不启动本 Child，不处理 Child 02 及后续任务。

## 4. 验收标准

- [ ] **AC-C01-01 / E-C01-WIN-BASELINE**：在 Windows 上用固定命令先复现研究记录的两项失败，并保存测试名、输入、期望/实际结果、Go/Windows 版本；修复后同一命令连续两次通过，且没有 skip、平台条件放行或测试删除。
- [ ] **AC-C01-02 / E-C01-PATH-CONTRACT**：表驱动测试覆盖配置路径的宿主平台规范化，以及图标路径的空值、普通文件名、`.`、`..`、父目录逃逸、POSIX 绝对路径、Windows drive/UNC 绝对路径和混合分隔符；所有平台保持同一安全意图。
- [ ] **AC-C01-03 / E-C01-VERSION-CONSUMERS**：根 `VERSION` 是唯一人工维护源；Go、Vite、`frontend/package.json`、Swagger 与构建后的 OCI label 均报告同一版本。compatibility/NSIS 只有明确消费接口或流水线输入合同，没有伪造组件。
- [ ] **AC-C01-04 / E-C01-VERSION-NEGATIVE**：至少逐项故意制造 `VERSION`↔Tag、`VERSION`↔package metadata、缺失/非法 `VERSION`、Go/Vite 注入遗漏和 Docker label 不一致，focused check 均非零退出且不产生可发布结果；测试恢复后通过。
- [ ] **AC-C01-05 / E-C01-WEB-SERVER-DOCKER-REGRESSION**：Windows focused Go tests、Linux `go test ./... -race`、前端 test/build、router/API snapshot 与 Docker build/metadata inspection 通过；现有 server/Web/PWA/Docker 可观察行为不变。
- [ ] **AC-C01-06 / E-C01-ROLLBACK**：统一版本变更可整体回退到上一个提交而不涉及 schema/API/data migration；回退演练确认不存在第二个继续生效的版本源。若共同消费机制不能一次切换全部当前消费者，则不得合入部分方案。

## 5. 完成与启动边界

本规划产物无开放问题。实现仅可在父任务批准 Child 规划且显式执行 Trellis start 后开始。`task.json` 在此阶段必须保持 `planning`；`task.py validate` 只验证规划上下文，不构成上述产品证据。
