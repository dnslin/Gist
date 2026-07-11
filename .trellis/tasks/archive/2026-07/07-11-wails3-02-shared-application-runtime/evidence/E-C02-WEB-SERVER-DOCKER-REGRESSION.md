# E-C02-WEB-SERVER-DOCKER-REGRESSION

## Passed locally

- Backend full non-race suite: 15 packages passed, 8 packages had no tests (`artifact://140`).
- Backend `go vet ./...`: passed with no output.
- Integration-tag service suite: passed (`artifact://141`).
- Runtime/router/server focused suite: passed (`artifact://156`).
- Frontend: 38 test files and 442 tests passed.
- Frontend ESLint: passed.
- Frontend TypeScript + Vite production build: passed; PWA generated 57 precache entries (`artifact://98`).
- Root version suite: 18 tests passed (`artifact://96`).
- Temporary-data server returned HTTP 200 from `/api/auth/status` and created its SQLite database.
- Independent concurrency/lifecycle review found no remaining P0/P1 or finish blocker after OPML lifecycle fixes.

## Unchanged contracts

- No frontend source or DTO changed.
- No HTTP route, handler response, schema, migration, config environment variable, Dockerfile, or workflow changed.
- `cmd/server` remains the only listener/pprof/signal host; Runtime contains no listener.
- Build graph contains no Wails dependency introduced by this child.

## Environment limitations / CI requirements

- Windows `go test -race` cannot build locally because `runtime/cgo` exits with `cgo.exe exit status 2`; Linux race is required in CI.
- `golangci-lint` is not installed locally; configured CI lint remains authoritative.
- Docker is not installed locally, so image build, OCI label, non-root process, `/app/data`, exposed port, and `/api/auth/status` container smoke remain CI-required through the existing Docker workflow.
- Windows subprocess APIs used locally could not deliver a targetable graceful SIGINT/SIGTERM console event; Linux signal-process smoke remains CI-required.