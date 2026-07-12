# Child 03 Phase 2.2 Check

Checked on Windows 11 amd64 with Go 1.26.5 on branch `feat/wails3-build-graph-desktop-paths-lock`.

## Blocking findings fixed

1. **Global owner metadata required an unavailable standard-user privilege.** A `Global\\` file mapping created outside session zero requires `SeCreateGlobalPrivilege`. Owner metadata now uses a hashed HKCU registry key with a protected current-user + SYSTEM DACL; the Global named mutex handle remains the sole ownership authority.
2. **Existing-mutex handling conflated object existence with active ownership.** Acquisition now performs a zero-time wait, distinguishes active, available-race, and abandoned outcomes, and has a real abandoned-mutex process fixture.
3. **Named-pipe deadlines were ineffective.** `os.File.SetDeadline` is unsupported for the original synchronous Windows pipe handle. Connect/read/write now use overlapped I/O, events, cancellation, and fixed deadlines. Message mode plus strict framing rejects invalid UTF-8 and trailing frames.
4. **Same-session lock losers were not integrated with activation routing.** Bootstrap now activates the owner before returning `data_owned_same_session`; other-session and unreachable paths remain fail-closed and touch no data stages.
5. **Bootstrap leaked resources returned alongside constructor errors.** Activation, ordinary stages, generator, and Runtime partial resources are closed before earlier stages; nil generator/Runtime results fail closed; activation receives the derived ownership identity.
6. **Recovery decoding and storage were not fully bounded or strict.** Reads are limited to `MaxRecordSize+1`; trailing JSON, invalid UTF-8, malformed identifiers, null metadata, unknown envelope values, and rollback-required/finish conflicts fail closed.
7. **Durable removal could erase the canonical journal before reporting directory-sync failure.** Cleanup now uses the same write-through atomic rename to a fixed tombstone, preserving evidence and idempotence. Windows recovery paths and temp files receive explicit current-user + SYSTEM DACLs.
8. **Crash evidence covered only `prepared`.** Real forced-termination fixtures now cover prepared, applied, rollback-required, committed, and rolled-back phases before the DB marker.
9. **Tests did not cover partial cleanup, actual DACLs, abandoned ownership, or multiple pipe frames.** Focused fixtures now cover each contract.

## Non-blocking external evidence requirements

- A second Terminal Services session for the same user is unavailable. Native cross-session mutex/activation evidence remains required on a suitable Windows runner and is not claimed as passed.
- Native Ubuntu race/test and Docker smoke remain required externally. Windows and Linux cross-builds plus Linux dependency-closure guards passed locally. The Windows race probe cannot compile because the installed cgo toolchain exits with status 2.
- The isolated-worktree rollback revert/reapply drill requires a completed Child 03 commit; no commit exists and this agent may not create one.

These items require environment or lifecycle fixed points, not further implementation changes discovered in this review.

## Verification

- `go test ./internal/desktop/... ./cmd/desktop ./cmd/server ./internal/application ./internal/config ./internal/service -count=1` — PASS, 9 packages (`artifact://134`).
- `go test ./internal/desktop/ownership -run "TestProcess|TestAbandoned|TestActivation" -count=1 -v` — PASS (`artifact://131`).
- `go test ./internal/desktop/recovery -run "TestCrash|TestDurable|TestJournalState|TestRollbackRequired" -count=1 -v` — PASS (`artifact://136`).
- `go test ./cmd/server ./internal/application -run "TestServerDependencies|TestLinuxDependencyClosure" -count=1 -v` — PASS (`artifact://132`).
- `go build ./...` — PASS.
- `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./...` — PASS.
- `python .trellis/scripts/task.py validate .trellis/tasks/07-11-wails3-03-build-graph-desktop-paths-lock` — PASS; 10 implement and 11 check context entries.
- `python .trellis/scripts/task.py validate-lifecycle .trellis/tasks/07-11-wails3-03-build-graph-desktop-paths-lock completion --allow-legacy` — PASS with the existing legacy-contract warning; no lifecycle state was mutated.

## Disposition

No fixable blocking finding remains. Standards and implementation/spec review axes pass with the external evidence requirements above recorded explicitly. Recommended next transition: `finish.spec-update`.
