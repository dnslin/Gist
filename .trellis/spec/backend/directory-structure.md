# Directory Structure

> How backend code is organized in this project.

---

## Overview

The backend is the Go module `gist/backend` (`backend/go.mod`). Production business dependencies are assembled once in `backend/internal/application.NewRuntime`: SQLite and infrastructure first, then repositories, services, handlers, and `internal/http.NewRouter`. `backend/cmd/server/main.go` is the process host for configuration, logging, Snowflake bootstrap, listener, signals, and pprof; handlers and repositories do not construct the application graph.

The prevalent request flow is handler → service → repository. Handlers own Echo transport concerns, services own use cases and business validation, and repositories own direct SQLite access. `backend/internal/handler/feed_handler.go` → `backend/internal/service/feed_service.go` → `backend/internal/repository/feed_repository.go` is the representative trace.

---

## Directory Layout

```text
backend/
├── cmd/server/main.go          # process host: config, listener, signals, pprof
├── internal/
│   ├── application/            # shared Runtime composition, writers, lifecycle
│   ├── config/                 # environment/default configuration
│   ├── db/                     # SQLite opening and schema migrations
│   ├── handler/                # Echo routes, binding, DTOs, status mapping
│   ├── http/                   # router, middleware, auth, static serving
│   ├── model/                  # persistence/domain structs
│   ├── repository/             # database/sql adapters
│   ├── scheduler/              # background refresh work
│   ├── service/                # use cases and external orchestration
│   ├── hashutil/               # module-private hashing helpers
│   └── urlutil/                # module-private URL normalization helpers
└── pkg/                        # reusable logger, network, ID, sanitizer, OPML packages
```

Concrete infrastructure examples are `backend/internal/config/config.go`, `backend/internal/db/db.go`, `backend/internal/http/router.go`, `backend/internal/scheduler/scheduler.go`, and `backend/pkg/logger/logger.go`.
`backend/internal/hashutil/` and `backend/internal/urlutil/` are backend-module utilities used by migrations, repositories, and services. Put helpers in `internal/*util` when they are implementation details of this Go module; use `pkg/*` only for packages intentionally reusable outside the module-private application layers. Existing examples of the reusable side are `backend/pkg/network`, `pkg/snowflake`, and `pkg/sanitizer`.

---

## Module Organization

- Handlers depend on exported service interfaces and receive them through `New…Handler`; for example, `FeedHandler` stores `service.FeedService` in `backend/internal/handler/feed_handler.go`.
- Services depend on repository or service interfaces while their concrete implementations remain private. `feedService` in `backend/internal/service/feed_service.go` stores `FeedRepository`, `FolderRepository`, and `EntryRepository` dependencies.
- Repositories expose an interface and constructor beside a private implementation, as in `FeedRepository`, `feedRepository`, and `NewFeedRepository` in `backend/internal/repository/feed_repository.go`.
- Complex integrations may use a service subpackage: `backend/internal/service/ai/` and `backend/internal/service/anubis/` contain provider/solver details. This is not universal; feeds, folders, and entries remain flat files in `internal/service/`.
- Handler-private request/response DTOs use camelCase JSON tags and convert models at the HTTP boundary. Snowflake `int64` IDs are emitted as strings for JavaScript safety in `backend/internal/handler/feed_handler.go`, `entry_handler.go`, and `response.go`.

## Outbound HTTP Contracts

The tracked root `.rules` makes the following package and execution boundaries mandatory for network work:

- Outbound HTTP code **MUST** obtain clients or transports through an injected `network.ClientFactory`; services MUST NOT construct ad hoc `http.Client` instances. `backend/internal/application.NewRuntime` constructs one factory from settings-backed proxy/IP-stack providers and injects it into feed, refresh, icon, readability, proxy, settings, and Anubis services; `backend/pkg/network/client.go` owns the concrete HTTP and Azure-session construction.
- Ordinary feed requests **MUST** start with `config.GistUserAgent` (currently exposed as `config.DefaultUserAgent`). When the origin blocks that request, the retry **MUST** use `config.ChromeUserAgent`, and retry control must prevent an unbounded loop.
- Feed refresh **MUST** implement conditional GET: send persisted ETag and Last-Modified values as `If-None-Match` and `If-Modified-Since`, treat `304 Not Modified` as success without parsing or rewriting entries, and persist non-empty validators from a successful response. `refreshService.refreshFeedWithCookie` and `processParsedFeed` in `backend/internal/service/refresh_service.go` are the positive reference.

### Existing outbound-HTTP deviation

`backend/internal/service/feed_service.go` and `refresh_service.go` currently retry blocked feed requests only with the arbitrary `general.fallback_user_agent` returned by `SettingsService.GetFallbackUserAgent`; an empty setting skips the retry. `backend/internal/service/settings_service.go` persists that configurable value. This differs from the mandatory Gist-then-`config.ChromeUserAgent` contract and MUST be treated as existing source debt, not as a configurable-fallback convention.

## AI Execution Boundaries

- Summary and translation execution **MUST** remain isolated from feed refresh: it must run asynchronously and MUST NOT block feed fetching, parsing, validator persistence, or entry persistence. Keep `refreshService` free of `AIService`/AI-provider dependencies. `backend/internal/application.NewRuntime` constructs refresh and AI services as sibling dependencies; AI does not enter the feed-refresh graph.
- All summary and translation provider requests **MUST** pass through one process-wide, injected `ai.RateLimiter`; per-operation worker semaphores MAY additionally cap concurrency but MUST NOT replace the shared request limiter. `backend/internal/application.NewRuntime` creates one limiter for `SettingsService` and `AIService`; `backend/internal/service/ai_service.go` calls it for summary, block translation, and batch title/summary provider work, while `backend/internal/service/ai/rate_limiter.go` owns synchronized limit updates.


---

## Naming and Tests

Production filenames are lower snake case by layer or concern: `feed_handler.go`, `feed_service.go`, `feed_repository.go`, and `domain_rate_limit_repository.go`. Tests are colocated as `*_test.go`, commonly using external packages such as `handler_test`, `service_test`, and `repository_test`.

Generated GoMock implementations live in same-layer `mock/` directories. Their interfaces carry `//go:generate mockgen …`, for example in `backend/internal/service/feed_service.go` and `backend/internal/repository/feed_repository.go`. Repeated `export_test.go` files (`internal/handler`, `internal/service`, `internal/db`) intentionally expose private helpers to external tests.

---

## Scenario: Shared Application Runtime

### 1. Scope / Trigger

Any host that needs the local Gist application graph MUST construct `backend/internal/application.Runtime`. `cmd/server` owns process hosting only: environment configuration, logging, Snowflake bootstrap, listener, signals, and pprof. Repositories, services, handlers, router, scheduler, SQLite, and local-data writers MUST NOT be assembled in a second host.

### 2. Signatures

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

func NewRuntime(ctx context.Context, options RuntimeOptions) (*Runtime, error)
func (r *Runtime) Quiesce(ctx context.Context) error
func (r *Runtime) Close(ctx context.Context) error

type WriterLauncher interface {
    LaunchWriter(context.Context, WriterClass, func(context.Context)) error
}
```

Snowflake repositories receive `snowflake.Generator` through constructors. Process hosts create it through a one-shot `snowflake.BootstrapOwner`; package-level mutable `Init`/`NextID` APIs are prohibited.

### 3. Contracts

- Runtime validates generator and DB path before opening SQLite or activating writers.
- Runtime owns DB → repositories → services → handlers/router plus scheduler and startup icon backfill.
- Runtime reads no environment variables and starts no listener or pprof server.
- Every asynchronous operation that can write local data registers before goroutine launch.
- Background writers cancel immediately on quiesce. Request/task-bound writers preserve initiating cancellation and may drain until the quiesce deadline; deadline then force-cancels them.
- OPML import reserves its task writer before HTTP 200. Refresh/backfill tail work stays inside that same reservation; nested admission after success is prohibited.
- `Close` is retryable after caller timeout. Only a nil result guarantees writer quietness, closed services, and SQLite closed last.

### 4. Validation & Error Matrix

| Condition | Required result |
|---|---|
| Missing generator | `NewRuntime` returns `ErrMissingIDGenerator`; DB and writers remain unopened |
| Empty DB path | `NewRuntime` returns `ErrMissingDBPath` |
| Invalid scheduler interval | `NewRuntime` returns `ErrInvalidInterval` |
| Writer admission after quiesce | `ErrWriterAdmissionClosed`; goroutine is not launched and caller cannot report success |
| Quiesce deadline | remaining bound writers are cancelled; deadline error is returned; later call may continue waiting |
| Parent import context cancelled | matching running task atomically becomes `cancelled`; stale watcher cannot alter a replacement task |
| Close caller timeout | shared close work remains retryable; timeout is not cached as terminal close result |

### 5. Good / Base / Bad Cases

- Good: host creates one generator, one Runtime, starts its own listener, and calls Runtime quiesce/close from a bounded shutdown path.
- Base: tests create isolated generators and Runtime instances over temporary SQLite databases with scheduler disabled when irrelevant.
- Bad: service starts a `context.Background()` writer, OPML launches a nested writer after returning HTTP 200, a host duplicates repository/service assembly, or a repository reaches package-global ID state.

### 6. Tests Required

- Generator: invalid node, one-shot owner, concurrent single winner, independent instances, repository injection.
- WriterRegistry: admission, exactly-once completion, request/task cancellation, background quiesce cancellation, graceful drain, deadline force-cancel, retry, and concurrent race.
- OPML/AI: rejection occurs before success/channel publication; task cancellation updates observable status; stream framing remains unchanged.
- Scheduler: immediate run, periodic run, registered refresh, Stop cancellation, and wait.
- Runtime: invalid options before DB open, construction cleanup, activation, idempotent/retryable lifecycle, writer quiet point, and DB-last close.
- Host: route/auth/static/stream contract plus SIGINT/SIGTERM bounded graceful shutdown.

### 7. Wrong vs Correct

#### Wrong

```go
go refreshService.RefreshAll(context.Background())
snowflake.NextID()
```

The writer is untracked and the ID source is mutable global state.

#### Correct

```go
if err := writers.LaunchWriter(ctx, service.WriterBackground, func(writerCtx context.Context) {
    _ = refreshService.RefreshAll(writerCtx)
}); err != nil {
    return err
}

id := generator.NextID()
```

## Known Gaps and Cautions

- Windows race builds currently depend on a working cgo toolchain; Linux CI remains the authoritative race gate.
- A Windows subprocess `SIGTERM` may terminate rather than exercise Go's graceful signal path. Use a real console/process signal fixture on a supported CI runner.
- Do not introduce a second production wiring location. Shared business assembly belongs only in `internal/application`; hosts own transport and process lifecycle.

