# E-C02-SERVER-CONTRACT

## Ownership

- `backend/internal/application.Runtime` is the only DB/repository/service/handler/router/scheduler composition root.
- `backend/cmd/server` retains config, logger, Snowflake host bootstrap, Echo listener, signal handling, and pprof.
- Runtime does not read environment variables or start a network listener.
- Shared Runtime imports no Wails or Windows-only package.

## Shutdown

SIGINT and SIGTERM enter the same host shutdown path:

1. fixed 10-second total deadline
2. 8-second concurrent Echo drain, pprof shutdown, and Runtime quiesce
3. Runtime writer admission closes immediately; background writers cancel; request-bound streams may drain until deadline
4. remaining time closes services and SQLite last
5. main waits for shutdown completion before reporting server stopped

Runtime lifecycle tests cover retryable deadlines, idempotent close, writer quieting, scheduler stop, and DB-last close.

## Verification

```text
go test ./cmd/server ./internal/application ./internal/http ./internal/service ./internal/handler ./internal/scheduler
PASS (artifact://89)

go test ./...
15 packages passed, 8 packages had no tests (artifact://140)

go vet ./...
PASS, no output

go test -tags=integration ./internal/service
PASS (artifact://141)
```

A real temporary-data server smoke reached `GET /api/auth/status` with HTTP 200 and body `{"exists":false}` and created the expected SQLite database. Windows `SIGTERM` maps to forced process termination in the local Python subprocess API, and targeted console-control delivery was unavailable; graceful signal behavior is therefore defended by the shared shutdown path and focused lifecycle tests, while Linux process-signal smoke remains CI-required.

## Web/PWA regression

```text
bun --cwd frontend run test
38 files, 442 tests passed

bun --cwd frontend run lint
PASS

bun --cwd frontend run build
PASS; Vite production build and PWA generated 57 precache entries (artifact://98)

bun test scripts/version.test.ts
18 tests passed (artifact://96)
```

No frontend source, API DTO, schema, migration, route contract, server environment variable, Dockerfile, or workflow was changed.