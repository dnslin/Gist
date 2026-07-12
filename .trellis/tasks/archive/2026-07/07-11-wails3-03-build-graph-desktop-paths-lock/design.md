# Child 03 技术设计：Build Graph、DesktopPaths、Lock 与 Recovery

## 1. Design Decisions

| Design ID | Decision | Requirements / AC |
|---|---|---|
| `DES-C03-01` | 保留单一 `backend` Go module，以 Windows build constraints 隔离 desktop host | `REQ-C03-GRAPH-01`, `AC-C03-01` |
| `DES-C03-02` | DesktopPaths 从 Windows LocalAppData known folder 单次派生为不可变值 | `REQ-C03-PATHS-01`, `AC-C03-02` |
| `DES-C03-03` | 使用当前用户 ACL 保护的 Global named mutex 表达跨 session 数据所有权 | `REQ-C03-LOCK-01`, `AC-C03-03` |
| `DES-C03-04` | 使用 per-session named pipe 承载固定 activation frame | `REQ-C03-IPC-01`, `AC-C03-04` |
| `DES-C03-05` | RecoveryJournal 采用版本化 JSON envelope 与 durable replace protocol | `REQ-C03-RECOVERY-01`, `AC-C03-05` |
| `DES-C03-06` | Bootstrap 以显式 stage/dependency graph 保证 lock-before-data 与逆序清理 | `REQ-C03-BOOT-01`, `AC-C03-06` |
| `DES-C03-07` | Wails、frontend desktop mode 与业务 journal schema 全部留给后续 Child | `REQ-C03-COMPAT-01`, `AC-C03-01/06` |

## 2. Module Boundary

```text
backend/                              # existing module gist/backend
├─ cmd/server                         # unchanged host: env/listener/signal/pprof
├─ internal/application              # unchanged platform-neutral composition root
├─ cmd/desktop                       # windows-only bootstrap executable, no Wails yet
└─ internal/desktop
   ├─ paths                           # platform-neutral immutable value + windows resolver
   ├─ ownership                       # interfaces, errors, identity and activation protocol
   │  └─ platform/windows             # named mutex, session identity, named pipe
   ├─ recovery                        # journal envelope/state machine/durable store
   └─ bootstrap                       # stage ordering and reverse cleanup
```

`cmd/desktop` and Windows implementations use `//go:build windows`. Platform-neutral value objects, state machines and test doubles remain buildable on Linux when they do not import Windows APIs. `internal/application` remains unaware of desktop.

A separate root Go module is rejected: it cannot directly import `backend/internal/application` because of Go `internal` visibility. Moving Runtime behind a new public adapter would deepen no useful module and create a second compatibility surface.

## 3. Build Graph Isolation

The production graph remains two disjoint host branches sharing only `internal/application`:

```text
cmd/server  ──> config + application.Runtime ──> HTTP listener

cmd/desktop [windows] ──> desktop bootstrap ──> application.Runtime
                                           └─> Child 04 Wails shell (future)
```

Rules:

1. `cmd/server` cannot import `internal/desktop`.
2. `internal/application` cannot import desktop, Wails or Windows packages.
3. Child 03 adds no Wails module/runtime dependency.
4. Linux verification inspects dependency closure, not only compile success.
5. Current `frontend/dist`, Vite/PWA config, `frontend/bun.lock`, Dockerfile and server Makefile remain unchanged.

## 4. DesktopPaths

### 4.1 Contract

```go
type Paths struct {
    Root        string
    DataDir     string
    DBPath      string
    ConfigPath  string
    RecoveryDir string
    LogsDir     string
    UpdatesDir  string
    WebViewDir  string
}

type LocalAppDataResolver interface {
    ResolveLocalAppData() (string, error)
}

func Resolve(resolver LocalAppDataResolver) (Paths, error)
```

Windows production resolution uses the LocalAppData known folder API rather than cwd or server configuration. The returned root is cleaned and converted to an absolute canonical lexical path before adding the fixed `Gist` component. The resolver rejects empty, relative, root-only and malformed results.

`Paths` is pure data. `Resolve` performs no mkdir, open, migration or config read. Directory creation is a later held-lock bootstrap stage.

### 4.2 Owned Layout

```text
%LOCALAPPDATA%\Gist\
├─ data\
│  └─ gist.db
├─ desktop.json
├─ recovery\
├─ logs\
├─ updates\
└─ webview\
```

Fixed names live in one package. Consumers receive the whole immutable `Paths` value or the specific derived path; they do not reconstruct strings.

## 5. Ownership Identity and Data Lock

### 5.1 Identity

The lock identity is derived from:

- schema prefix `gist.desktop.data.v1`
- canonical, case-folded data-root identity
- current Windows user SID

The path is hashed before entering object names. Raw profile paths are not exposed in mutex/pipe names or logs.

```text
hash     = SHA-256(lowercase(canonicalDataRoot))[:16]
mutex    = Global\Gist.Data.v1.<userSidHash>.<hash>
metadata = HKCU\Software\Gist\Desktop\Ownership\v1\<userSidHash>.<hash>
pipe     = \\.\pipe\Gist.Activate.v1.<userSidHash>.<sessionId>.<hash>
```

The explicit user component avoids cross-user collisions while the mutex and metadata DACLs restrict access to the current user and SYSTEM. The `Global\` mutex provides cross-Terminal-Services-session exclusion for one Windows user. Owner metadata is a small current-user registry value containing only protocol version, owner PID, owner session ID and a random owner nonce while the mutex is held. A Global file-mapping object is deliberately rejected because Microsoft requires `SeCreateGlobalPrivilege` to create one outside session zero; the desktop host must run correctly as a standard user. Registry metadata contains no data path or credential and is never the ownership authority.
### 5.2 Mutex Primitive

Production uses a Windows named mutex created/opened with an explicit security descriptor. Acquisition rules:

- initial ownership requested atomically;
- `ERROR_ALREADY_EXISTS` opens the existing mutex; a zero-time wait distinguishes actively owned, newly available and abandoned outcomes without treating metadata as authority;
- abandoned mutex acquisition is ownership success plus a stable `previous_owner_abandoned` diagnostic; recovery still runs before DB open;
- the held OS handle is the ownership fact;
- `Close` is idempotent and releases/closes only once;
- process termination lets Windows release ownership without deleting an artifact.

- after acquisition, publish owner metadata before opening activation IPC or any data resource;
- on contention, read metadata only to classify same-session versus other-session activation; missing, stale or corrupt metadata yields occupied/unreachable and never permits startup;
- clear metadata during orderly shutdown before releasing the mutex; crash leftovers are ignored once mutex acquisition succeeds and are overwritten by the new owner;

The implementation must distinguish access denied, invalid security descriptor and already-owned outcomes. It must not fall back to a file lock or a local-session mutex.

## 6. Activation IPC

### 6.1 Transport

A per-session named pipe is selected because it supports Windows ACLs, bounded request/response framing and deterministic timeout. The data mutex stays cross-session; the pipe is deliberately session-specific because a process in another interactive session must not try to focus that session's window.

### 6.2 Protocol

```json
{"version":1,"action":"activate"}
```

Responses:

```json
{"version":1,"result":"accepted"}
{"version":1,"result":"occupied_other_session"}
{"version":1,"result":"occupied_unreachable"}
```

Protocol rules:

- one length-prefixed UTF-8 JSON frame per connection;
- maximum 1 KiB request and response;
- fixed connect/read/write deadlines;
- reject trailing frames, unknown fields, unknown versions/actions and invalid UTF-8;
- no arbitrary command, path, URL, credential or business payload;
- server callback is only an injected `ActivationSink.Activate()`; it has no Runtime, DB, config or journal dependency.

On mutex contention, the second process compares owner/session metadata exposed by the ownership primitive. Same-session attempts the pipe. Other-session returns `occupied_other_session` without opening a pipe. Missing/unreachable same-session endpoint returns `occupied_unreachable`; it never proceeds to data access.

Child 03 tests the callback and protocol but does not focus a real Wails window. Child 11 supplies that adapter.

## 7. RecoveryJournal

### 7.1 Envelope

```go
type Record struct {
    SchemaVersion int             `json:"schemaVersion"`
    TransactionID string          `json:"transactionId"`
    Operation     string          `json:"operation"`
    Phase         string          `json:"phase"`
    Metadata      json.RawMessage `json:"metadata"`
}
```

Core validates envelope structure, schema version, stable identifiers and size. It does not interpret operation-specific metadata. Later operation packages register typed recovery handlers through constructor injection; unknown operations fail closed.

### 7.2 State Machine

The primitive recognizes generic durable milestones:

```text
Absent -> Prepared -> Applied -> Committed -> Absent
                  \-> RollbackRequired -> RolledBack -> Absent
```

Operation handlers decide whether a `Prepared`/`Applied` record can be finished or must be rolled back. Replaying the same record must be idempotent. `Committed` cleanup may remove only the journal after the committed target state is durably visible.

### 7.3 Durable Replace

For every transition:

1. serialize and validate to a bounded byte slice;
2. create a unique temp file in `RecoveryDir` with restrictive permissions;
3. write all bytes and `Sync` the temp file;
4. atomically replace the canonical journal file;
5. sync the recovery directory through a platform filesystem adapter;
6. only then report the transition committed.

Delete follows the same durability rule: remove canonical journal, then sync the directory. A failure leaves the last known durable record or temp evidence; startup does not guess or silently delete.

The storage layer is injected so unit tests can fault every write/sync/replace/remove boundary. Windows integration tests exercise real filesystem behavior and process termination between phases.

## 8. Bootstrap Ordering

```go
type Dependencies struct {
    ResolvePaths    func() (paths.Paths, error)
    AcquireOwner    ownership.Acquirer
    StartActivation ownership.ServerFactory
    OpenLogs        Stage
    Recover         recovery.Runner
    LoadConfig      Stage
    OpenCredentials Stage
    NewGenerator    Stage
    NewRuntime      RuntimeFactory
}
```

Execution:

```text
resolve paths
  -> derive identity
  -> acquire data mutex
  -> start activation endpoint
  -> create required directories / open logs
  -> replay journal
  -> load desktop config seam
  -> open credential seam
  -> create Snowflake generator
  -> application.NewRuntime(DataDir, DBPath, generator)
  -> return Runtime-ready Host
```

Every successful stage pushes one cleanup function. Failure executes cleanup in reverse and joins errors. Runtime closes before credentials/logs/IPC; mutex releases last. The ownership-loss path never instantiates the remaining dependencies, which makes lock-before-DB observable through call counters.

Child 03 may use test-only config/credential stages to verify ordering, but production `cmd/desktop` must not ship fake/no-op implementations. Until Child 04/07 provide real downstream adapters, the executable may expose only an integration-test bootstrap harness behind a non-production test entry; no misleading runnable product shell is delivered.

## 9. Error Model and Logging

Stable categories:

- `desktop_paths_unavailable`
- `data_owned_same_session`
- `data_owned_other_session`
- `data_owner_unreachable`
- `activation_protocol_invalid`
- `recovery_corrupt`
- `recovery_unsupported`
- `recovery_failed`
- `bootstrap_failed`

Logs use `backend/pkg/logger`, structured fields and hashes/phase identifiers. They never include full profile paths, credentials or raw journal metadata.

## 10. Compatibility and Rollback

No schema, migration, API, server config, frontend build or user data conversion occurs. Rollback removes the desktop-only packages, tests and build-graph assertions as one code change. It does not delete `%LOCALAPPDATA%\Gist`, a real recovery record or user data. If a real unfinished journal exists, the compatible implementation must recover it before removal; planning/fixture journals use isolated temp roots.

## 11. Verification Mapping

| AC | Design | Verification | Evidence |
|---|---|---|---|
| `AC-C03-01` | `DES-C03-01/07` | Linux build/test and dependency denylist | `E-C03-WEB-SERVER-DOCKER-REGRESSION` |
| `AC-C03-02` | `DES-C03-02` | LocalAppData/cwd/env/path purity matrix | `E-C03-DESKTOP-PATHS` |
| `AC-C03-03` | `DES-C03-03/06` | same/cross-session processes, kill/reacquire, DB spy | `E-C03-LOCK-BEFORE-DB` |
| `AC-C03-04` | `DES-C03-04` | protocol negative matrix and session routing | `E-C03-ACTIVATION-IPC` |
| `AC-C03-05` | `DES-C03-05/06` | fault/kill matrix, sync trace, DB-before replay assertion | `E-C03-CRASH-RECOVERY` |
| `AC-C03-06` | `DES-C03-06/07` | stage fault injection, regression and rollback drill | `E-C03-ROLLBACK` |
