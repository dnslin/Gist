# Directory Structure

> How backend code is organized in this project.

---

## Overview

The backend is the Go module `gist/backend` (`backend/go.mod`). Production dependencies are assembled in `backend/cmd/server/main.go`: configuration and infrastructure are initialized first, followed by repositories, services, handlers, and `internal/http.NewRouter`. This is the observed composition root; handlers and repositories do not construct the application graph.

The prevalent request flow is handler → service → repository. Handlers own Echo transport concerns, services own use cases and business validation, and repositories own direct SQLite access. `backend/internal/handler/feed_handler.go` → `backend/internal/service/feed_service.go` → `backend/internal/repository/feed_repository.go` is the representative trace.

---

## Directory Layout

```text
backend/
├── cmd/server/main.go          # process entrypoint and dependency wiring
├── internal/
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

- Outbound HTTP code **MUST** obtain clients or transports through an injected `network.ClientFactory`; services MUST NOT construct ad hoc `http.Client` instances. `backend/cmd/server/main.go` constructs one factory from settings-backed proxy/IP-stack providers and injects it into feed, refresh, icon, readability, proxy, settings, and Anubis services; `backend/pkg/network/client.go` owns the concrete HTTP and Azure-session construction.
- Ordinary feed requests **MUST** start with `config.GistUserAgent` (currently exposed as `config.DefaultUserAgent`). When the origin blocks that request, the retry **MUST** use `config.ChromeUserAgent`, and retry control must prevent an unbounded loop.
- Feed refresh **MUST** implement conditional GET: send persisted ETag and Last-Modified values as `If-None-Match` and `If-Modified-Since`, treat `304 Not Modified` as success without parsing or rewriting entries, and persist non-empty validators from a successful response. `refreshService.refreshFeedWithCookie` and `processParsedFeed` in `backend/internal/service/refresh_service.go` are the positive reference.

### Existing outbound-HTTP deviation

`backend/internal/service/feed_service.go` and `refresh_service.go` currently retry blocked feed requests only with the arbitrary `general.fallback_user_agent` returned by `SettingsService.GetFallbackUserAgent`; an empty setting skips the retry. `backend/internal/service/settings_service.go` persists that configurable value. This differs from the mandatory Gist-then-`config.ChromeUserAgent` contract and MUST be treated as existing source debt, not as a configurable-fallback convention.

## AI Execution Boundaries

- Summary and translation execution **MUST** remain isolated from feed refresh: it must run asynchronously and MUST NOT block feed fetching, parsing, validator persistence, or entry persistence. Keep `refreshService` free of `AIService`/AI-provider dependencies and schedule AI work separately. The separate `NewRefreshService` and `NewAIServiceWithFeedContext` wiring in `backend/cmd/server/main.go`, plus the absence of AI dependencies from `refreshService` in `backend/internal/service/refresh_service.go`, is the positive boundary.
- All summary and translation provider requests **MUST** pass through one process-wide, injected `ai.RateLimiter`; per-operation worker semaphores MAY additionally cap concurrency but MUST NOT replace the shared request limiter. `backend/cmd/server/main.go` creates one limiter for `SettingsService` and `AIService`; `backend/internal/service/ai_service.go` calls it for summary, block translation, and batch title/summary provider work, while `backend/internal/service/ai/rate_limiter.go` owns synchronized limit updates.


---

## Naming and Tests

Production filenames are lower snake case by layer or concern: `feed_handler.go`, `feed_service.go`, `feed_repository.go`, and `domain_rate_limit_repository.go`. Tests are colocated as `*_test.go`, commonly using external packages such as `handler_test`, `service_test`, and `repository_test`.

Generated GoMock implementations live in same-layer `mock/` directories. Their interfaces carry `//go:generate mockgen …`, for example in `backend/internal/service/feed_service.go` and `backend/internal/repository/feed_repository.go`. Repeated `export_test.go` files (`internal/handler`, `internal/service`, `internal/db`) intentionally expose private helpers to external tests.

---

## Known Gaps and Cautions

No backend architecture document establishes package dependency rules; these conventions are derived from current source. Do not infer a mandatory one-package-per-feature layout from the two specialized service subpackages, and do not introduce a second production wiring location without matching an existing need.

