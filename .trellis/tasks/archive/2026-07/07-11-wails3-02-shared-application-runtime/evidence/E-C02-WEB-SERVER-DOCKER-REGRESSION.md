# E-C02-WEB-SERVER-DOCKER-REGRESSION

## Passed locally

- Backend full non-race suite: 16 packages passed, 7 packages had no tests (`artifact://155`).
- Backend `go vet ./...`: passed with no output.
- Backend `go build -v ./...`: passed.
- Integration-tag service suite: passed (`artifact://154`).
- Shutdown force-close ordering test passed 20 consecutive runs (`artifact://153`).
- Runtime, WriterRegistry, OPML, AI, scheduler, handler, and router focused suites passed (`artifact://117`).
- Linux-target server dependency denylist passed in `cmd/server`: no Wails, `internal/desktop`, or `golang.org/x/sys/windows` dependency.
- Frontend: 38 test files and 442 tests passed.
- Frontend ESLint: passed.
- Frontend TypeScript + Vite production build passed; PWA generated 57 precache entries (`artifact://143`).
- Root version suite: 18 tests passed (`artifact://141`).

## Contract changes and preserved behavior

- No frontend source or DTO, schema, migration, config environment variable, Dockerfile, or workflow changed.
- OPML success behavior remains HTTP 200 with a synchronously published task; admission rejection remains HTTP 500 and is now represented in Swagger.
- OPML core completion publishes `done` before refresh/icon tail work, while the same admitted reservation remains alive through the tail.
- `cmd/server` remains the only listener/pprof/signal host; Runtime contains no listener.
- Listener bind failures retain the baseline return behavior rather than forcing a new process exit status.
- The 8-second drain timeout now force-closes HTTP handlers before Runtime/SQLite close.

## Environment limitations / CI requirements

- `go test ./... -race -count=1` cannot build locally because `runtime/cgo` reports `cgo.exe exit status 2` (`artifact://147`); Linux race remains required in CI.
- `golangci-lint` is not installed locally; configured CI lint remains authoritative.
- Docker is not installed locally, so image build, OCI label, non-root process, `/app/data`, exposed port, and `/api/auth/status` container smoke remain CI-required.
- The Windows subprocess environment cannot deliver a targetable graceful console SIGINT/SIGTERM event. Local deterministic tests exercise the exact shared runner for both signals with pprof on/off; Linux process-level signal smoke remains CI-required.