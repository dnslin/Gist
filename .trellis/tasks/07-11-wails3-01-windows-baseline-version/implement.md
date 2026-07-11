# Child 01 实施计划：Windows 基线与统一产品版本源

> 状态：planning；未启动。以下步骤只在显式 `task.py start` 后执行。

## 1. Start gate

- 父任务 Child 01 范围、`AC-PROD-01/17/20` 分段责任和 `E-C01-*` 槽位已批准。
- 本 Child 的 `prd.md`、`design.md`、本计划和两份 curated JSONL 通过独立评审。
- Windows 原生环境可运行 Go tests；Linux runner、Bun 与 Docker/Buildx 可用于回归。
- evidence 目录/CI artifact 名称在实现时固定；不得以 Trellis validate、编译或 skip 代替行为证据。

## 2. 执行顺序

### Phase A — 固化迁移前 Windows 基线

1. 记录 `go version`、`go env GOOS GOARCH` 和 Windows build。
2. 在未修改代码前运行并保存：

   ```powershell
   cd backend
   go test ./internal/config -run '^TestLoad$' -count=1 -v
   go test ./internal/service -run '^TestIconService_IsValidIconPath$' -count=1 -v
   ```

3. 将每项失败分类为 fixture/断言不可移植或生产安全合同缺陷，并记录触发输入、实际值、期望值。证据槽：`E-C01-WIN-BASELINE`。
4. 若实际失败与 research 不同，停止修改，更新 Child research/设计并重新评审；不得把新失败并入既有两项描述。

### Phase B — 修复且锁定跨平台路径合同

1. 配置测试改用宿主平台路径构造与精确 DB path 断言；保持 server `GIST_*` 语义不变。
2. 为 icon path validator 增加表驱动安全矩阵，先证明 Windows drive、UNC、POSIX root、父目录与混合分隔符可绕过的 plausible bug，再在生产入口统一拒绝。
3. 重跑 focused tests 两次，确认无 skip/条件放行：

   ```powershell
   cd backend
   go test ./internal/config -run '^TestLoad' -count=2 -v
   go test ./internal/service -run '^TestIconService_IsValidIconPath$' -count=2 -v
   ```

4. 运行受影响包：

   ```powershell
   cd backend
   go test ./internal/config ./internal/service -count=1
   ```

证据槽：`E-C01-PATH-CONTRACT`。

### Phase C — 建立根版本源与校验器

1. 创建根 `VERSION`，初始化为当前已存在 metadata 的 `1.2.0`。
2. 实现单一只读校验入口：解析稳定 SemVer、校验 optional Tag、校验 frontend package 与 Swagger metadata；所有错误非零退出。
3. 为解析/校验增加 focused positive 与 negative tests；测试使用临时目录，不改真实 `VERSION`。
4. 将 release workflow 的 Tag 解析改为调用该入口；Tag 不一致时必须在 build/push/release 前失败。

### Phase D — 接入所有当前消费者

1. **Go**：使 `config.AppVersion` 可由 linker 注入，并让 user agent 从最终值派生；增加注入值/派生值合同测试。
2. **Vite/frontend**：从已验证 `VERSION` 注入 typed build constant；保持 package metadata 等于根版本并增加可观察 build test。
3. **Swagger**：同步注释和生成物，使产品版本等于根版本；运行现有生成命令后校验无漂移。
4. **Docker**：Dockerfile 接收版本 build arg，同一值注入 Go binary 并写 OCI version label；CI 与本地 build 显式传值。
5. 在代码与 CI 中搜索 `1.2.0`/产品版本定义，逐项分类：消费者、测试 fixture 或无关协议版本。产品消费者不得保留独立真相。
6. compatibility/NSIS 仅在接口注释或 release input contract 中指向统一值；不得创建 endpoint、installer、Wails config 或 placeholder。

证据槽：`E-C01-VERSION-CONSUMERS`。

### Phase E — 失败关闭验证

在临时 fixture 上逐项运行：

```text
missing VERSION
empty VERSION
v-prefixed/partial/prerelease/multiline VERSION
Tag != v+VERSION
frontend package version != VERSION
Swagger metadata != VERSION
Go injected version != VERSION or omitted from release build
Vite injected version != VERSION or omitted
Docker OCI label != VERSION or absent
```

每个场景必须证明命令非零退出并且 release job 不进入 build/push/publish。恢复正确 fixture 后同一检查通过。证据槽：`E-C01-VERSION-NEGATIVE`。

### Phase F — focused 兼容回归

实现工作证明可用后，执行最后清理与回归：

```powershell
# Windows
cd backend
go test ./internal/config ./internal/service ./internal/http ./internal/handler -count=1
go test ./... -count=1

# Frontend (Bun only)
cd frontend
bun run test
bun run build

# Swagger generation/check, from backend
swag init -g cmd/server/main.go --parseDependency --parseInternal
```

```sh
# Linux CI-equivalent backend regression
cd backend
go test ./... -race

# Docker, VERSION supplied by the single validator
# exact wrapper/variable is fixed during implementation
VERSION_VALUE=<validated-value>
docker build --build-arg VERSION="$VERSION_VALUE" -f docker/Dockerfile -t gist:c01-version .
docker image inspect gist:c01-version
```

Docker inspection必须断言 `org.opencontainers.image.version`、server startup、`/api/auth/status`/router snapshot、port、entrypoint、non-root user 和 `/app/data` 契约。前端 build artifact/version test 同时证明 PWA/Service Worker 构建仍存在。证据槽：`E-C01-WEB-SERVER-DOCKER-REGRESSION`。

### Phase G — rollback 演练与归档准备

1. 在临时分支/工作树整体撤回 Child 01 版本提交，运行旧 baseline smoke，确认无 schema/data/API 恢复动作。
2. 再应用完整提交并运行版本一致性检查；禁止演练“只回退某个消费者”的半状态。
3. 记录 rollback 命令、前后版本报告与无残留第二来源的搜索结果到 `E-C01-ROLLBACK`。
4. 只有所有 `E-C01-*` 槽位替换为真实 artifact URI 后才可 check/archive。

## 3. AC → Design → 验证 → Evidence

| Child AC | Parent slice | Design | 关键验证 | Evidence |
|---|---|---|---|---|
| `AC-C01-01` | `AC-PROD-01` | `DES-C01-01` | Windows 原始两项失败 + 修复后 count=2 | `E-C01-WIN-BASELINE` |
| `AC-C01-02` | `AC-PROD-01` | `DES-C01-01` | 跨平台路径安全表驱动 negative matrix | `E-C01-PATH-CONTRACT` |
| `AC-C01-03` | `AC-PROD-20` 前置 | `DES-C01-02~04` | Go/Vite/package/Swagger/Docker 同值 | `E-C01-VERSION-CONSUMERS` |
| `AC-C01-04` | `AC-PROD-20` 前置 | `DES-C01-03` | Tag/metadata/injection/label drift 全部失败关闭 | `E-C01-VERSION-NEGATIVE` |
| `AC-C01-05` | `AC-PROD-01`; `AC-PROD-17` 回归前置 | `DES-C01-05` | Windows/Linux race/frontend/router/Docker | `E-C01-WEB-SERVER-DOCKER-REGRESSION` |
| `AC-C01-06` | 父 Child 01 rollback | `DES-C01-05` | 整体回退、无 schema/API/data、无第二来源 | `E-C01-ROLLBACK` |

`AC-PROD-17` 的安装矩阵和 `AC-PROD-20` 的签名/完整发布仍分别由 Child 10/14 验收；本 Child 不提前关闭这些父级 AC。

## 4. 停止条件

出现任一条件立即停止并保持 task 为进行中/阻塞，不带部分机制进入后续 Child：

- 两项 Windows 基线不能按记录命令重复；
- 路径校验只能通过放宽安全合同或平台 skip 修复；
- 任一当前版本消费者无法切换到根源或一致性校验；
- negative test 仍能形成可推送 image/release；
- Web/server/Docker/PWA 可观察行为发生变化；
- rollback 后仍有第二个生效版本源。

## 5. Rollback point

唯一 rollback point 是 Child 01 完整提交之前。回滚单位包含根 `VERSION`、校验入口、Go/Vite/Swagger/Docker 注入、CI workflow 与相关测试。没有数据库迁移、用户数据、凭据、注册表、快捷方式、安装目录或发布资产需要恢复。失败时允许保留基线/evidence 记录，但不允许保留部分消费者或兼容别名。
