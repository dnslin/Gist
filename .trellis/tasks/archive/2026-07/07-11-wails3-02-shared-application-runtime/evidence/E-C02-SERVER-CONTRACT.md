# E-C02-SERVER-CONTRACT

## Ownership

- `backend/internal/application.Runtime` is the only DB/repository/service/handler/router/scheduler composition root.
- `backend/cmd/server` retains config, logger, Snowflake host bootstrap, Echo listener, signal handling, and pprof.
- Runtime does not read environment variables or start a network listener.
- Shared Runtime imports no Wails or Windows-only package.

## Shutdown

SIGINT and SIGTERM enter the same tested host shutdown runner:

1. one fixed 10-second overall context
2. an 8-second concurrent Echo drain, pprof shutdown, and Runtime quiesce
3. writer admission closes immediately; background writers cancel while accepted request-bound writers drain
4. drain expiry force-closes HTTP and pprof, then waits for tracked handlers to exit before Runtime close begins
5. the remaining overall budget closes Runtime services and SQLite last

`cmd/server` tests inject short deterministic budgets and cover SIGINT/SIGTERM with pprof enabled and disabled. The forced-drain test failed before the fix with `runtime close began before HTTP handler exited`, then passed 20 consecutive runs after the fix. Listener bind failure is returned without a new `os.Exit(1)` path.

Runtime tests cover per-build-stage reverse cleanup with zero activation, controlled scheduler time, concurrent/idempotent Quiesce and Close, retry after caller deadline, writer quieting, and DB-last close.

## Verification

```text
go test ./cmd/server -run TestShutdownRunnerForcesHTTPHandlersBeforeRuntimeClose -count=20
PASS (artifact://153)

go test ./internal/application ./internal/service ./internal/scheduler ./internal/handler ./internal/http -count=1
PASS (artifact://117)

go test ./... -count=1
16 packages passed, 7 packages had no tests (artifact://155)

go vet ./...
PASS, no output

go build -v ./...
PASS

go test -tags=integration ./internal/service -count=1
PASS (artifact://154)
```

`TestServerDependenciesExcludeDesktopAndWindowsHosts` runs `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go list -deps .` and rejects Wails, `internal/desktop`, and `golang.org/x/sys/windows` dependencies.

The Windows workstation cannot target a real process with graceful SIGINT/SIGTERM using the available subprocess API. The same injected runner is covered for both signals locally; Linux process-signal smoke remains a CI requirement.

## Web/PWA regression

```text
bun --cwd frontend run test
38 files, 442 tests passed

bun --cwd frontend run lint
PASS

bun --cwd frontend run build
PASS; Vite production build and PWA generated 57 precache entries (artifact://143)

bun test scripts/version.test.ts
18 tests passed (artifact://141)
```

No frontend source or DTO, schema, migration, server environment variable, Dockerfile, or workflow changed. The OPML route set and success/error runtime behavior remain unchanged; its Swagger source and generated artifacts now explicitly document the existing admission-failure HTTP 500 response.