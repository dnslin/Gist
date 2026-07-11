# Shared Application Runtime Design

## DES-C02-01 — Ownership Boundary

```text
cmd/server
  config/logger/listener/signal/pprof
                 |
                 v
internal/application.Runtime
  generator ref -> SQLite -> repositories -> services -> handlers/router
                                      |
                                      +-> WriterRegistry
                                           |- scheduler refresh
                                           |- startup icon backfill
                                           |- OPML post-import work
                                           |- import task
                                           `- AI async cache/persist work
```

`internal/application` owns the local application kernel and no host transport. `cmd/server` translates existing config into Runtime options, starts/stops Echo and pprof, and handles signals. It cannot construct repositories/services after cutover.

Runtime exports only the router and services required by current server or a known later adapter. DB, scheduler, repositories, closeable concrete services, worker contexts, and cleanup machinery remain private.

## DES-C02-02 — Runtime Contract

```go
type RuntimeOptions struct {
    DataDir           string
    DBPath            string
    StaticDir         string
    EnableSwagger     bool
    SchedulerInterval time.Duration
    StartScheduler    bool
    IDGenerator       snowflake.Generator
}

type Runtime struct {
    Router      *echo.Echo
    Auth        service.AuthService
    AI          service.AIService
    OPML        service.OPMLService
    ImportTasks service.ImportTaskService
    Writers     *WriterRegistry
}

func NewRuntime(ctx context.Context, options RuntimeOptions) (*Runtime, error)
func (r *Runtime) Quiesce(ctx context.Context) error
func (r *Runtime) Close(ctx context.Context) error
```

Options are explicit values; Runtime never calls `config.Load` or reads `GIST_*`.

## DES-C02-03 — Generator Ownership

Replace mutable package-level Snowflake state with an instance:

```go
type Generator interface { NextID() int64 }
func NewGenerator(node int64) (Generator, error)
```

A process bootstrap owner enforces one successful initialization for that host. Server bootstrap calls it once before `NewRuntime`; a second call on the same owner returns a stable error. Tests create isolated owners/generators and never overwrite a process global. Every repository that creates persisted IDs receives `Generator` through its constructor. All package-level `Init`/`NextID` production and test callsites are removed in one cutover.

Generator validation happens before Runtime build. No DB writer, scheduler, or worker can start without a valid injected instance.

## DES-C02-04 — Writer Inventory and Admission

A writer is asynchronous work that can mutate local SQLite or repository-owned files after its initiating stack returns.

| Existing work | Classification | Runtime contract |
|---|---|---|
| scheduler `RefreshAll` | background writer | scheduler trigger registers before refresh goroutine starts; quiesce cancels immediately |
| startup icon backfill | background writer | activate registers then launches; quiesce cancels immediately |
| OPML import follow-up refresh/backfill | task/request-bound writer | admission occurs before successful response; initiating cancel is preserved |
| import task execution | task-bound writer | task start registers before task publication/launch; task cancel is preserved |
| AI translation/batch streaming plus cache persistence | request-bound writer | one registration follows the accepted request; graceful server drain allows the stream to finish, while request cancellation stops stream/cache work |
| pure event fan-out or network-only worker | non-writer | excluded only when test proves no local persistence path |

Implementation must produce a final inventory from all goroutine launch sites and record every exclusion with reason.

Registry state:

```text
Accepting --begin quiesce--> Rejecting --active=0--> Quiet
```

Rules:

- registration and active-count increment happen before goroutine launch
- registration failure means no launch and no success response
- each registration owns an exactly-once completion token, a class (`background` or `request/task-bound`), and a linked context
- initiating request/task cancellation always cancels its writer; scheduler writers additionally layer their configured refresh timeout
- quiesce atomically rejects new registration and cancels background writers immediately, but waits for request/task-bound writers to finish under their initiating contexts
- when the quiesce/drain deadline expires, Runtime cancellation force-cancels remaining request/task-bound writers and returns a stable deadline error without pretending quietness
- concurrent register/complete/quiesce, independent caller/task cancellation, graceful drain, and forced deadline cancellation are synchronized and race-tested

## DES-C02-05 — Build Then Activate

`NewRuntime` has two internal phases.

### Build: fallible, no asynchronous work

1. Validate options and generator reference.
2. Create root context, lifecycle state, and WriterRegistry.
3. Open SQLite and run existing migrations.
4. Construct repositories with explicit generator injection.
5. Construct services.
6. Construct handlers and the existing router.
7. Construct scheduler/backfill/import/AI worker launch descriptors without starting them.
8. Seal a fully built Runtime.

Every successful resource creation adds one inverse cleanup to a LIFO stack. A failure executes the stack in reverse, joins cleanup errors, returns no Runtime, and has started zero writers/listeners/pprof.

### Activate: non-fallible publication

After all fallible build steps complete, activate performs only in-memory state transitions and prevalidated register/start operations. StartScheduler=false skips scheduler activation. Startup writers use already-reserved registration tokens so activation cannot fail halfway.

If a dependency cannot support non-fallible activation, its fallible preparation belongs in Build and activation publishes only the prepared result.

## DES-C02-06 — Lifecycle State Machine

```text
Open -> Quiescing -> Quiesced -> Closing -> Closed
  |          |            |          |
  +----------+------------+----------+-- concurrent callers observe synchronized state
```

- `Quiesce(ctx)` transitions Open→Quiescing once, closes writer admission, stops scheduler triggers, cancels background writers, and waits for request/task-bound writers under their initiating contexts.
- If the quiesce deadline expires, Runtime force-cancels remaining linked writers and returns that caller a deadline error; shared state remains Quiescing until active=0, and later callers may continue waiting. Once active=0, state becomes Quiesced.
- `Close(ctx)` ensures quiesce, then performs close-once resource shutdown. A timed-out caller does not cache timeout as the final shared result; later calls may continue.
- Only `Close` returning nil guarantees Closed: scheduler/workers/services stopped and SQLite closed last.
- Concurrent callers share actual resource-close work but use their own wait contexts.
- Repeated calls after Closed return the stored terminal resource-close result without re-closing.

Close order:

1. reject admission, stop scheduler triggering, and cancel background writers
2. drain request/task-bound writers, force-cancelling only at deadline, then wait for active=0
3. cancel Runtime root context
4. close readability/proxy and other closeable services
5. close SQLite last

## DES-C02-07 — Server Hosting and Shutdown

Server remains host owner:

1. load existing config/logger and construct one generator owner
2. build/activate one Runtime
3. optionally start pprof
4. start Echo on the existing address
5. route SIGINT and SIGTERM into the same shutdown function with one fixed 10-second overall deadline
6. immediately reject new writer admission, stop scheduler triggers, and cancel background writers; concurrently run Echo graceful drain, pprof shutdown, and wait for request/task-bound writers within an 8-second drain budget
7. request-scoped SSE/NDJSON accepted before the signal keeps its request context and may finish with unchanged framing during that budget; only budget expiry force-cancels remaining requests/writers
8. after HTTP handlers and writers are quiet, or after forced cancellation has completed, use the reserved final 2 seconds to close Runtime services and SQLite; SQLite remains last

The fixed total remains the existing 10 seconds. Fixtures cover a stream completing inside the drain budget and a near-deadline stream/writer forced-cancel path, proving original framing/graceful completion when possible and preserving final Runtime close time. SIGINT and SIGTERM both run these fixtures; pprof runs disabled and enabled. An explicit Runtime close marker prevents process exit from falsely proving shutdown.

## DES-C02-08 — Compatibility and Build Isolation

Unchanged contracts:

- complete route/auth middleware set, status/body, SSE/NDJSON, icons, Swagger option, static SPA fallback
- DB path/parent creation/migrations/schema
- `GIST_*`, Web/LAN listener, PWA and Docker
- scheduler immediate/periodic/stop semantics

Dependency verification uses `go list -deps` denylisting Wails and Windows-only packages from server/application Linux graphs.

## DES-C02-09 — Verification and Evidence

| AC | Design | Verification | Evidence |
|---|---|---|---|
| AC-C02-01 | DES-C02-01/02/08 | constructor graph tests; `go list -deps` denylist | E-C02-SERVER-CONTRACT |
| AC-C02-02 | DES-C02-03/05 | generator negative tests; build activation marker | E-C02-SNOWFLAKE |
| AC-C02-03 | DES-C02-04/06 | writer inventory; admission/deadline/race tests | E-C02-WRITER-REGISTRY |
| AC-C02-04 | DES-C02-05/06 | fault table; concurrent/retry lifecycle tests | E-C02-WRITER-REGISTRY |
| AC-C02-05 | DES-C02-04/06 | controlled refresh/fake clock | E-C02-WRITER-REGISTRY |
| AC-C02-06 | DES-C02-07/08 | route snapshot; SIGINT/SIGTERM near-deadline fixtures; Linux/Windows/Docker matrix | E-C02-SERVER-CONTRACT / E-C02-WEB-SERVER-DOCKER-REGRESSION |
| AC-C02-07 | all | isolated worktree revert/reapply and baseline reruns | E-C02-ROLLBACK |

## DES-C02-10 — Rollback Shape

The cutover is atomic: generator constructors/callsites, writer admission, Runtime composition, and server delegation revert together. No schema/data/credentials/registry/shortcuts/install artifacts change. Rollback validation rejects residual global generator shims, duplicate composition roots, and duplicate writer launch paths.