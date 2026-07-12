# Desktop Host Guidelines

## Scenario: Windows Desktop Data Ownership Boundary

### 1. Scope / Trigger

Use this contract whenever code under `backend/internal/desktop` or `backend/cmd/desktop` derives local paths, acquires desktop data ownership, activates an existing instance, replays recovery state, or constructs `application.Runtime`.

The desktop host is a Windows-only adapter around the existing platform-neutral Runtime. It must not introduce Wails, Windows or desktop dependencies into `cmd/server` or `internal/application`.

### 2. Signatures

```go
// Desktop paths are resolved once and then injected.
type paths.Paths struct {
    Root, DataDir, DBPath, ConfigPath string
    RecoveryDir, LogsDir, UpdatesDir, WebViewDir string
}
func paths.Resolve(LocalAppDataResolver) (paths.Paths, error)

// An OS handle is the ownership authority.
type ownership.Acquirer interface {
    Acquire(context.Context, ownership.Identity) (ownership.Acquisition, error)
}
type ownership.Lease interface { Close() error }

// Activation is deliberately data-independent.
type ownership.ActivationSink interface {
    Activate(context.Context) error
}

// Recovery must finish before Runtime construction.
type recovery.Store interface {
    Load(context.Context) ([]byte, error)
    Replace(context.Context, []byte) error
    Remove(context.Context) error
}
type recovery.Handler interface {
    Recover(context.Context, recovery.Record) (recovery.Decision, error)
}

// Bootstrap owns ordering and reverse cleanup.
func bootstrap.Start(context.Context, bootstrap.Dependencies) (*bootstrap.Host, error)
func (*bootstrap.Host).Close(context.Context) error
```

### 3. Contracts

#### Paths

- Resolve LocalAppData through the Windows known-folder adapter.
- Fixed layout: `Gist/data/gist.db`, `desktop.json`, `recovery`, `logs`, `updates`, `webview`.
- Never read cwd, `GIST_DATA_DIR`, `GIST_DB_PATH`, or `config.Load()` for desktop paths.
- Resolution is pure: no directory creation, file open, migration, or config read.

#### Ownership and activation

- Derive object identity from canonical data root plus current user SID; expose only hashes in object names/logs.
- Use a `Global\` named mutex with protected current-user + SYSTEM DACL. The mutex handle is the only ownership authority.
- Store owner PID/session/nonce in a hashed HKCU registry key protected by the same DACL. Registry metadata only classifies same-session versus other-session contention; stale/missing metadata never permits startup.
- On an existing mutex, perform a zero-time wait to distinguish active ownership, available-race acquisition, and `WAIT_ABANDONED`.
- Same-session contender may send one bounded `activate` request over the per-session named pipe. Other-session contenders do not open the pipe.
- Named-pipe connect/read/write use overlapped I/O, event waits, bounded deadlines, and `CancelIoEx`. `os.File.SetDeadline` on a synchronous Windows pipe is not an accepted deadline implementation.
- Activation protocol is one length-prefixed UTF-8 JSON frame, maximum 1024 bytes, version `1`, action `activate`; unknown fields/actions/versions, trailing frames and invalid UTF-8 fail closed.

#### Recovery

- Envelope fields: `schemaVersion`, `transactionId`, `operation`, `phase`, `metadata`.
- Supported generic phases: `prepared`, `applied`, `committed`, `rollback_required`, `rolled_back`.
- Reads are bounded to `MaxRecordSize + 1`; JSON is strict, UTF-8, single-value and unknown-field rejecting.
- `rollback_required` may only produce `DecisionRollback`.
- Durable replace is same-directory temp write → file sync → atomic `MoveFileEx(REPLACE_EXISTING|WRITE_THROUGH)` → directory sync attempt.
- Durable cleanup moves the canonical journal to a fixed tombstone before sync/removal. A cleanup failure must preserve canonical or tombstone evidence; never destructively delete the only recovery record first.
- Recovery directories and temporary files use explicit current-user + SYSTEM DACL on Windows.

#### Bootstrap

Required order:

```text
resolve paths -> derive identity -> acquire mutex -> activation endpoint
-> logs -> recovery replay -> config -> credentials -> generator
-> application.NewRuntime(DataDir, DBPath, generator)
```

Lock contenders perform zero data-stage work. Every successfully created partial resource must be closed even when its constructor also returns an error. Failure cleanup is reverse order; the data lease is last. A successful `Host.Close` is retryable: if an earlier resource close fails, retain remaining cleanup entries and the lock for a later call.

### 4. Validation & Error Matrix

| Condition | Required behavior |
|---|---|
| LocalAppData missing/relative/root-only/malformed | `desktop_paths_unavailable`; no side effects |
| Mutex actively owned in same session | send bounded activation; return `data_owned_same_session`; zero data access |
| Mutex actively owned in another session | return `data_owned_other_session`; do not open activation pipe |
| Metadata missing/stale/corrupt | return owner unreachable/occupied; never enter bootstrap |
| Mutex becomes available between create and wait | acquire lease and continue safely |
| `WAIT_ABANDONED` | acquire lease, record abandoned diagnostic, run recovery before DB |
| Pipe timeout/oversize/invalid/trailing frame | cancel I/O and return `activation_protocol_invalid` |
| Journal missing | replay succeeds without handler or DB side effects |
| Journal corrupt/oversize/unknown schema or phase | preserve evidence and return `recovery_corrupt`/`recovery_unsupported` |
| `rollback_required` handler asks to finish | return `recovery_failed`; retain journal |
| Durable replace/cleanup boundary fails | preserve last durable record/tombstone; fail closed |
| Stage returns closer/runtime plus error | close partial resource, join errors, then reverse-clean prior stages |
| Runtime close fails | retain Runtime and earlier cleanup entries, including lock, for retry |

### 5. Good / Base / Bad Cases

- **Good:** first Windows process acquires the Global mutex, publishes HKCU owner metadata, starts activation, replays a killed `applied` journal, then constructs exactly one Runtime.
- **Base:** no journal exists; recovery is a no-op and Runtime still starts only after ownership/config/credential/generator stages.
- **Bad:** a second process sees the mutex and calls `config.Load`, opens SQLite, or relies on WAL/busy timeout. It must stop at activation/occupied routing with zero data-stage calls.
- **Bad:** journal cleanup deletes `journal.json` before directory durability is established. Move to a tombstone first so evidence survives failure.

### 6. Tests Required

- Black-box path matrix: temporary LocalAppData, changed cwd, conflicting `GIST_*`, malformed roots, and no filesystem side effects.
- Real Windows helper processes: same-session contention, activation, owner kill/reacquire, abandoned mutex, DACL inspection and second-frame rejection.
- Bootstrap fault table: every stage, partial closer/runtime plus error, nil generator/runtime, contender routing, reverse cleanup and retryable close.
- Recovery state/fault table: every generic phase, strict/oversize payloads, each durable I/O boundary, tombstone cleanup and replay-before-DB marker.
- Real process-kill recovery fixtures for `prepared`, `applied`, `rollback_required`, `committed`, and `rolled_back`.
- Build guards: Windows `go build ./...`, Linux cross-build, and dependency closure proving server/application exclude desktop, Wails and Windows-only host packages.
- External completion evidence: same-user second Terminal Services session; native Linux race/Docker; isolated fixed-commit rollback drill. Never relabel unexecuted external evidence as pass.

### 7. Wrong vs Correct

#### Wrong

```go
// Wrong: server environment and SQLite concurrency are not desktop ownership.
cfg := config.Load()
db, _ := db.Open(cfg.DBPath)
if databaseIsLocked(err) { focusExistingWindow() }
```

```go
// Wrong: synchronous named pipe deadlines are not enforced by SetDeadline on Windows.
_ = pipe.SetDeadline(time.Now().Add(time.Second))
```

#### Correct

```go
p, err := paths.Resolve(localAppDataResolver)
identity, err := ownership.DeriveIdentity(p.DataDir, currentSID, sessionID)
acquired, err := acquirer.Acquire(ctx, identity)
// Only an acquired Lease may proceed to recovery and Runtime construction.
```

```go
// Correct: Windows pipe operations use OVERLAPPED + event wait + CancelIoEx.
err := performOverlapped(ctx, pipeHandle, operation, deadline)
```

## Design Decisions

- Keep desktop in the existing `backend` module with Windows build constraints. A separate root module cannot import `backend/internal/application` without creating a public compatibility seam.
- Use a Global mutex for cross-session ownership and HKCU for metadata. Global file mappings require privileges normal desktop processes may not hold; metadata is classification only, not ownership.
- Keep Wails out of this boundary. Wails SingleInstance may later deepen same-session UX, but it cannot replace the data mutex.
