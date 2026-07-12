# Child 03 Implementation Plan

## 1. Start Gate

- [ ] Child 01/02 archived evidence is readable; Child 02 Runtime remains the only application composition root.
- [ ] `prd.md` requirement/AC IDs and `design.md` DES IDs pass trace review.
- [ ] `implement.jsonl` and `check.jsonl` contain real spec/research entries; `_example` rows are removed.
- [ ] User reviews planning artifacts and explicitly approves `task.py start`.
- [ ] Branch is recorded as `feat/wails3-build-graph-desktop-paths-lock`, base `main`.
- [ ] Windows evidence environment is fixed: Windows 11 amd64, current-user same-session multi-process harness, and a controlled second Terminal Services session or Windows runner capable of cross-session execution.
- [ ] Linux evidence environment is fixed for build/dependency isolation; missing Docker/Linux/second-session tooling is recorded as unexecuted, never passed.
- [ ] Evidence files are created only after real commands/scenarios run.

## 2. Build Graph Guard — AC-C03-01 / DES-C03-01

- [ ] Add failing dependency-closure tests proving `cmd/server` and `internal/application` exclude `internal/desktop`, Wails and Windows-only host packages.
- [ ] Establish `internal/desktop` package boundaries and Windows build tags without adding Wails dependencies.
- [ ] Add a Windows-only desktop bootstrap test target; do not ship a fake runnable desktop product.
- [ ] Prove platform-neutral desktop state/value packages remain Linux-testable where applicable.

Focused validation:

```bash
go -C backend test ./cmd/server ./internal/application
go -C backend build ./...
```

Linux dependency check from `backend/`:

```bash
deps="$(go list -deps ./cmd/server ./internal/application/...)"
if printf '%s\n' "$deps" | grep -E 'github.com/wailsapp/|gist/backend/internal/desktop|golang.org/x/sys/windows'; then exit 1; fi
```

Evidence: `evidence/E-C03-WEB-SERVER-DOCKER-REGRESSION.md`.

Rollback point: build tags/package boundaries and graph assertions revert together; no public Runtime shim remains.

## 3. DesktopPaths — AC-C03-02 / DES-C03-02

- [ ] Write table-driven black-box tests for empty/relative/malformed resolver output and expected fixed layout.
- [ ] Add tests changing cwd and setting conflicting `GIST_DATA_DIR`/`GIST_DB_PATH`; assert identical desktop paths.
- [ ] Assert resolution performs no mkdir/open/write.
- [ ] Implement LocalAppData known-folder resolver and immutable `Paths` value.
- [ ] Centralize fixed component names and identity normalization.
- [ ] Re-run server config/path tests to prove no semantic change.

Focused validation:

```bash
go -C backend test ./internal/desktop/paths ./internal/config ./internal/service
```

Evidence: `evidence/E-C03-DESKTOP-PATHS.md` records test matrix, canonical paths, environment isolation and no-side-effect proof.

Rollback point: paths package and all consumers revert together; no fallback to server config remains.

## 4. Windows Data Ownership — AC-C03-03 / DES-C03-03

- [ ] Add injected identity/security/session interfaces and negative tests before Windows API code.
- [ ] Implement current-user SID lookup, canonical root hash, Global mutex name and explicit DACL.
- [ ] Add minimal HKCU owner metadata containing protocol version, owner PID, Terminal Services session ID and nonce; protect it with an explicit current-user + SYSTEM DACL, and avoid privileged Global file mappings so standard-user startup remains valid.
- [ ] Implement acquisition outcomes: acquired, acquired-after-abandon, owned-same-session, owned-other-session, access denied and invalid identity.
- [ ] Ensure OS handle, not metadata, is the ownership fact; stale/corrupt metadata never permits data access.
- [ ] Hold ownership across bootstrap/runtime and release it last with idempotent close.
- [ ] Build same-session two-process and cross-session integration fixtures with DB/config/journal/Snowflake call counters.
- [ ] Kill the owner and prove a third process acquires without artifact deletion.

Focused validation on Windows:

```powershell
go -C backend test ./internal/desktop/ownership/... -count=1
go -C backend test ./internal/desktop/ownership/... -run 'TestProcess' -count=1 -v
```

Windows race coverage is run only where the installed CGO/race toolchain supports it; absence is not a pass.

Evidence: `evidence/E-C03-LOCK-BEFORE-DB.md` includes object names in hashed/redacted form, DACL/session facts, process traces, zero DB-open loser count and kill/reacquire output.

Rollback point: mutex, metadata object and all bootstrap ownership callsites revert together; no file-lock fallback remains.

## 5. Activation IPC — AC-C03-04 / DES-C03-04

- [ ] Write protocol tests for accepted activation, unknown version/action/field, invalid UTF-8, oversize, trailing frame and deadlines.
- [ ] Implement current-session pipe identity, current-user/SYSTEM DACL and one-frame request/response server.
- [ ] Restrict handler dependency to `ActivationSink`; add a test that fails if Runtime/config/journal dependencies enter the constructor.
- [ ] Route same-session lock losers to the pipe, cross-session losers to `occupied_other_session`, and unreachable same-session owners to `occupied_unreachable`.
- [ ] Verify arbitrary paths, URLs, credentials and business commands cannot be represented or dispatched.

Focused validation on Windows:

```powershell
go -C backend test ./internal/desktop/ownership/... -run 'TestActivation' -count=1 -v
```

Evidence: `evidence/E-C03-ACTIVATION-IPC.md` records protocol fixtures, bounds/deadlines, DACL and session-routing results.

Rollback point: protocol, pipe server/client and lock-loser routing revert together.

## 6. RecoveryJournal — AC-C03-05 / DES-C03-05

- [ ] Write envelope/state-machine tests for absent/prepared/applied/committed/rollback-required/rolled-back paths.
- [ ] Add negative tests for unknown schema/operation/phase, duplicate transaction, corrupt/truncated/oversize JSON and missing recovery material.
- [ ] Implement bounded versioned envelope and handler registry with typed operation ownership.
- [ ] Implement injected durable store: same-directory temp, full write, file sync, atomic replace, directory sync, durable remove.
- [ ] Fault every write/sync/replace/remove boundary; preserve last durable record/material on failure.
- [ ] Add process-kill fixtures between generic phases and prove replay is idempotent.
- [ ] Add DB factory spy proving replay/rollback completes before first `application.NewRuntime`/`db.Open` path.
- [ ] Keep operation-specific settings/backup/update payloads absent.

Focused validation:

```bash
go -C backend test ./internal/desktop/recovery -count=1
```

Windows process/durability validation:

```powershell
go -C backend test ./internal/desktop/recovery -run 'TestCrash|TestDurable' -count=1 -v
```

Evidence: `evidence/E-C03-CRASH-RECOVERY.md` records state transitions, fault table, real kill phases, sync trace and DB-before-replay assertion.

Rollback point: journal envelope/store/replay and bootstrap integration revert together; real unfinished journals are never deleted by rollback.

## 7. Bootstrap Integration — AC-C03-06 / DES-C03-06

- [ ] Write stage-order tests before production orchestration.
- [ ] Implement explicit dependencies and LIFO cleanup stack.
- [ ] Integrate: paths → identity → mutex → activation → directories/logs → recovery → config seam → credential seam → generator → one Runtime.
- [ ] Fault every stage and prove reverse cleanup, joined errors, no early Runtime and mutex-last release.
- [ ] Prove success passes `DataDir`, `DBPath` and generator to Child 02 Runtime, starts no listener/pprof/Wails app, and exposes a Runtime-ready host seam only.
- [ ] Do not merge fake production config/credential implementations; use test doubles only until their owning Child lands.

Focused validation:

```powershell
go -C backend test ./internal/desktop/bootstrap/... ./cmd/desktop -count=1
```

Evidence is appended to `E-C03-LOCK-BEFORE-DB.md` and `E-C03-CRASH-RECOVERY.md`.

## 8. Compatibility Matrix — AC-C03-01/06

Windows/local:

```powershell
go -C backend test ./...
go -C backend vet ./...
bun --cwd frontend run test
bun --cwd frontend run lint
bun --cwd frontend run build
bun test scripts/version.test.ts
```

Linux/CI:

```bash
go -C backend build -v ./...
go -C backend test ./... -race -coverprofile=coverage.out -covermode=atomic
(cd backend && golangci-lint run)
(cd backend && deps="$(go list -deps ./cmd/server ./internal/application/...)" && if printf '%s\n' "$deps" | grep -E 'github.com/wailsapp/|gist/backend/internal/desktop|golang.org/x/sys/windows'; then exit 1; fi)
```

Docker uses the existing Child 02 build/smoke contract and confirms no desktop/Wails artifact enters the image. Environment limitations remain explicit.

Evidence: `evidence/E-C03-WEB-SERVER-DOCKER-REGRESSION.md`.

## 9. Rollback Drill — AC-C03-06

In an isolated worktree:

1. Record the completed Child 03 commit.
2. Revert the complete Child 03 code/test cutover.
3. Run Child 02 backend/server/frontend/Docker baseline available in the environment.
4. Reapply Child 03.
5. Run focused paths/ownership/IPC/recovery/bootstrap tests and dependency denylist.
6. Verify no Wails dependency, second Runtime composition root, server env change, schema/migration change, file-lock fallback or real-data cleanup code remains.

Evidence: `evidence/E-C03-ROLLBACK.md`.

## 10. AC → Design → Validation → Evidence

| AC | Design | Required validation | Evidence |
|---|---|---|---|
| `AC-C03-01` | `DES-C03-01/07` | Linux build/test/dependency denylist | `E-C03-WEB-SERVER-DOCKER-REGRESSION` |
| `AC-C03-02` | `DES-C03-02` | LocalAppData/cwd/env/no-side-effect matrix | `E-C03-DESKTOP-PATHS` |
| `AC-C03-03` | `DES-C03-03/06` | same/cross-session process, kill/reacquire, zero DB spy | `E-C03-LOCK-BEFORE-DB` |
| `AC-C03-04` | `DES-C03-04` | protocol/session/negative matrix | `E-C03-ACTIVATION-IPC` |
| `AC-C03-05` | `DES-C03-05/06` | state/fault/kill/sync/DB-order matrix | `E-C03-CRASH-RECOVERY` |
| `AC-C03-06` | `DES-C03-06/07` | stage faults, compatibility and rollback | `E-C03-ROLLBACK`; `E-C03-WEB-SERVER-DOCKER-REGRESSION` |

## 11. Check Gate

- [ ] Every requirement and AC maps to a design decision, real validation and evidence file.
- [ ] Windows evidence includes real processes and cross-session execution; mocks cover logic but do not replace OS proof.
- [ ] Linux graph proves absence of desktop/Wails dependencies.
- [ ] Lock loser performs zero data/config/journal/Runtime access.
- [ ] Recovery always precedes first DB open and failures preserve material.
- [ ] No fake production shell, Wails dependency, frontend desktop mode or future business journal schema is shipped.
- [ ] Trellis check passes before Phase 3 spec update and commit.
