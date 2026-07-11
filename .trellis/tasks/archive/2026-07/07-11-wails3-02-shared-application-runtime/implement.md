# Shared Application Runtime Implementation Plan

## 1. Start Gate

- [ ] Child 01 archived evidence is readable, especially `E-C01-WEB-SERVER-DOCKER-REGRESSION`.
- [ ] PRD stable IDs and DES-C02 IDs pass trace review.
- [ ] `implement.jsonl` and `check.jsonl` contain real context and `task.py validate` passes.
- [ ] Execution environment is fixed:
  - Windows 11 workstation: non-race Go tests, frontend suite/lint/build, focused process smoke.
  - Linux CI/runner: `go test ./... -race`, `go list -deps`, Docker build/smoke.
  - Missing Docker/Linux tooling locally is recorded as an environment limitation, never treated as pass.
- [ ] Fixtures are fixed before implementation: migrated temp SQLite DB, fault-index Runtime builder, controlled scheduler refresh/clock, blocking registered writer, close marker, pprof on/off server process, route/auth/static/stream snapshot.
- [ ] Evidence directory and files are created only when real commands are run.

## 2. Generator Cutover — AC-C02-02 / DES-C02-03

- [ ] Write negative tests for invalid node, uninitialized repository construction, second initialization on one bootstrap owner, and isolation between independent test generators.
- [ ] Implement instance `Generator` plus one-shot host bootstrap owner.
- [ ] Inject generator into every repository/service creating persisted IDs.
- [ ] Migrate every production/test callsite; delete package-level mutable `Init`/`NextID` state without shim.
- [ ] Add activation marker proving failures occur before scheduler/backfill/writer start.

Focused validation:

```bash
cd backend && go test ./pkg/snowflake ./internal/repository/...
```

Evidence: `evidence/E-C02-SNOWFLAKE.md` records callsite inventory, negative cases, commands, observed outputs, and activation ordering.

Rollback point: generator implementation and all constructor callsites revert together.

## 3. Writer Inventory and Registry — AC-C02-03/05 / DES-C02-04

- [ ] Inventory all goroutine launch sites and classify local-data writers as background or request/task-bound; separately prove non-writers.
- [ ] Implement register-before-launch, exactly-once completion, class-aware cancellation, wait, deadline, and race-safe state.
- [ ] Cover scheduler refresh, startup icon backfill, OPML post-import refresh/backfill, import task, and asynchronous AI stream/cache persistence paths.
- [ ] Move admission before success publication/HTTP response. Define stable service/handler error mapping for rejected admission; do not return 200 then drop work.
- [ ] Build linked contexts preserving initiating request/task cancellation; layer scheduler timeout; quiesce cancels background immediately but drains request/task-bound work until deadline, then force-cancels.
- [ ] Test request cancel, task cancel, background quiesce cancel, in-flight SSE/NDJSON graceful drain, deadline force-cancel, and unchanged stream framing independently.
- [ ] Add controlled scheduler refresh/fake clock tests for immediate, periodic, stop-cancel, and wait semantics.
- [ ] Verify every excluded goroutine is request-scoped or cannot write local state.

Focused validation:

```bash
go -C backend test -race ./internal/application ./internal/scheduler ./internal/service/...
```

Evidence: `evidence/E-C02-WRITER-REGISTRY.md` includes the full inventory, state transitions, rejection behavior, deadline/race results, and writer callsite anchors.

Rollback point: registry plus every migrated launch path revert together; no dual admission path remains.

## 4. Runtime Build/Activate — AC-C02-01/04 / DES-C02-01/02/05

- [ ] Add Runtime options, owner object, lifecycle state, and private cleanup stack.
- [ ] Move DB/repository/service/handler/router construction from `cmd/server` without changing business options or route registration.
- [ ] Prepare scheduler/workers during fallible Build without launching them.
- [ ] Activate only after all fallible resources succeed; activation uses pre-reserved/non-failing start tokens.
- [ ] Inject a failure after each build resource and assert reverse cleanup, no partial Runtime, zero writers, zero listener/pprof starts.
- [ ] Keep DB/scheduler/repositories/private services unexported unless a current adapter requires them.

Focused validation:

```bash
go -C backend test ./internal/application ./internal/http ./internal/handler/...
```

Rollback point: composition moves back to server as one unit; no shared helper remains if the cutover is reverted.

## 5. Lifecycle and Server Cutover — AC-C02-04/06 / DES-C02-06/07/08

- [ ] Implement Open→Quiescing→Quiesced→Closing→Closed synchronization.
- [ ] Ensure caller deadline does not become a cached terminal close result; later callers can resume waiting/closing.
- [ ] Prove nil Close means all writers/services stopped and DB closed last.
- [ ] Reduce `cmd/server` to config/logger/generator bootstrap/listener/signal/pprof plus one Runtime.
- [ ] Route SIGINT and SIGTERM through one 10-second shutdown function: immediately reject writers/cancel background work; run Echo drain, pprof shutdown, and request/task-bound writer drain for at most 8 seconds; force-cancel only at expiry; reserve 2 seconds for final service/SQLite close.
- [ ] Build process fixtures for both signals using temporary data, pprof off/on, a completing in-flight SSE/NDJSON request, a near-deadline HTTP/blocking writer path, unchanged framing assertions, and an explicit Runtime close marker.

Focused validation:

```bash
go -C backend test -race ./internal/application ./internal/scheduler ./cmd/server
```

## 6. Server Contract Matrix — AC-C02-06

Windows/local commands, each executed from repository root:

```bash
go -C backend test ./...
go -C backend vet ./...
bun --cwd frontend run test
bun --cwd frontend run lint
bun --cwd frontend run build
bun test scripts/version.test.ts
```

Linux CI/runner commands, each executed from repository root:

```bash
go -C backend test ./... -race -coverprofile=coverage.out -covermode=atomic
go -C backend build -v ./...
(cd backend && golangci-lint run)
(cd backend && deps="$(go list -deps ./cmd/server ./internal/application/...)" && if printf '%s\n' "$deps" | grep -E 'github.com/wailsapp/|/internal/desktop|golang.org/x/sys/windows' ; then exit 1; fi)
```

Docker verification uses the existing validator and workflow smoke contract from repository root:

```bash
VERSION="$(bun scripts/version.ts)"
IMAGE="gist:c02"
docker build --build-arg VERSION="$VERSION" -t "$IMAGE" -f docker/Dockerfile .
test "$(docker image inspect --format '{{ index .Config.Labels "org.opencontainers.image.version" }}' "$IMAGE")" = "$VERSION"
docker run -d --name gist-c02 -p 127.0.0.1:18080:8080 "$IMAGE"
trap 'docker rm -f gist-c02 >/dev/null 2>&1 || true' EXIT
for attempt in $(seq 1 30); do if curl --fail --silent http://127.0.0.1:18080/api/auth/status >/dev/null; then ready=true; break; fi; sleep 1; done
test "${ready:-false}" = "true"
docker top gist-c02 -eo user,comm | grep -E '^gist[[:space:]]+gist-server$'
test "$(docker inspect --format '{{json .Config.ExposedPorts}}' gist-c02)" = '{"8080/tcp":{}}'
docker exec gist-c02 test -d /app/data
docker rm -f gist-c02
trap - EXIT
```

Any local absence of Linux race, golangci-lint, or Docker is recorded as unexecuted and must be supplied by CI/runner evidence.

Contract fixture coverage:

- complete route registration and auth public/protected boundary
- representative status/body for auth, normal JSON, icons, Swagger option
- static asset and SPA fallback
- existing SSE/NDJSON framing snapshots without normalization
- migration/open behavior and `GIST_DATA_DIR`/`GIST_DB_PATH`
- scheduler immediate/periodic/stop behavior
- SIGINT and SIGTERM through the same 10-second path: budget-completing SSE/NDJSON retains framing, near-deadline HTTP/request-bound writer is cancelled only after drain expiry, background writer cancels immediately, and final close marker is written

Evidence:

- `evidence/E-C02-SERVER-CONTRACT.md`
- `evidence/E-C02-WEB-SERVER-DOCKER-REGRESSION.md`

## 7. Rollback Drill — AC-C02-07 / DES-C02-10

In an isolated worktree:

1. Record the completed Child 02 commit.
2. Revert the full Child 02 cutover.
3. Run the Child 01 backend/router/frontend baseline appropriate to the environment.
4. Reapply the Child 02 commit.
5. Run focused generator/writer/runtime/server tests and the available platform matrix.
6. Search/inspect for residual package-global generator API, duplicate server composition, duplicate writer launch paths, schema/migration changes, and data-conversion code.

Evidence: `evidence/E-C02-ROLLBACK.md` records worktree refs, commands, outputs, environment limitations, and the no-residue inspection.

Rollback is code-only. No data, schema, credential, registry, shortcut, install directory, or CI/release state is migrated by this Child.

## 8. AC → Design → Validation → Evidence

| AC | Design | Required validation | Evidence |
|---|---|---|---|
| AC-C02-01 | DES-C02-01/02/08 | Runtime constructor graph; server graph; dependency denylist | E-C02-SERVER-CONTRACT |
| AC-C02-02 | DES-C02-03/05 | generator negative suite; activation marker | E-C02-SNOWFLAKE |
| AC-C02-03 | DES-C02-04/06 | inventory; admission/quiesce/deadline/race | E-C02-WRITER-REGISTRY |
| AC-C02-04 | DES-C02-05/06 | per-stage fault injection; concurrent/retry close; DB-last | E-C02-WRITER-REGISTRY |
| AC-C02-05 | DES-C02-04/06 | controlled scheduler refresh/fake clock | E-C02-WRITER-REGISTRY |
| AC-C02-06 | DES-C02-07/08 | route/auth/static/stream snapshots; SIGTERM; Linux/Windows/Docker | E-C02-SERVER-CONTRACT; E-C02-WEB-SERVER-DOCKER-REGRESSION |
| AC-C02-07 | DES-C02-10 | isolated revert/reapply and no-residue inspection | E-C02-ROLLBACK |

## 9. Archive Gate

- [ ] Every REQ-C02/AC-C02 ID maps to design, real validation, and non-placeholder evidence URI.
- [ ] Behavior, boundary, invariant, failure, concurrency, deadline, and rollback cases pass.
- [ ] Rollback drill preserves Child 01 evidence.
- [ ] Web/PWA HTTP/SSE/NDJSON/file/PWA behavior and server/Docker paths/listener/Linux graph remain unchanged.
- [ ] Logs/errors/evidence contain no sensitive data.
- [ ] Trellis quality check passes; reusable implementation contracts are added to backend specs; implementation/spec/evidence/task changes are committed before archive.
- [ ] Child 03 starts only after Child 02 is archived.