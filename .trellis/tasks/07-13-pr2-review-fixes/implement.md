# PR #2 Review Fixes Implementation Plan

## 1. Start Gate

- [ ] PRD/design/implement reviewed and approved.
- [ ] `implement.jsonl` / `check.jsonl` contain real backend specs, Child 03 artifacts, review and diagnosis context.
- [ ] Branch remains `feat/wails3-build-graph-desktop-paths-lock`; PR #2 remains open.
- [ ] No product edit begins before red-capable tests are run and captured.

## 2. Red Loops: Pure Defects

Write and run before fixes:

```powershell
go -C backend test ./internal/desktop/paths -run TestResolveRejectsUNCShareRoot -count=1
go -C backend test ./internal/desktop/bootstrap -run TestAcquireErrorClosesReturnedLease -count=1
go -C backend test ./internal/desktop/ownership -run TestLeaseCloseFromDifferentThread -count=1 -v
go -C backend test ./internal/desktop/recovery -run 'TestTombstoneReplayRetriesCleanup|TestJournalErrorsRedactRecoveryPath' -count=1
```

Evidence records exact red symptom and artifact URI.

## 3. Mutex and Bootstrap Fixes

- [ ] Implement dedicated locked-OS-thread mutex owner worker.
- [ ] Preserve available-race, abandoned, metadata/DACL and kill/reacquire behavior.
- [ ] Close `error + Lease` and malformed-outcome leases; join errors.
- [ ] Add typed abandoned diagnostic and post-log/pre-recovery recorder.
- [ ] Add acquired-without-lease, generic partial stage, nil generator/runtime, partial runtime and retryable close tests.

Focused validation:

```powershell
go -C backend test ./internal/desktop/bootstrap ./internal/desktop/ownership -run 'Test(Acquire|Lease|Abandoned|Construction|Nil|HostClose)' -count=1 -v
```

## 4. Activation Protocol and I/O

- [ ] Add red native stalled-response, idle-client, delayed-second-request and cancel-order tests.
- [ ] Replace Peek-based duplex contract with request pipe + nonce-derived response pipe.
- [ ] Implement strict request EOF before sink invocation.
- [ ] Centralize cancel-and-drain; collect final OVERLAPPED completion before event/handle release.
- [ ] Normalize stable protocol errors while preserving unwrap causes.
- [ ] Verify timeout recovery accepts the next valid activation.

Focused validation:

```powershell
go -C backend test ./internal/desktop/ownership -run 'Test(Activation|DelayedSecond|CancelDrains|Stalled|IdleClient)' -count=1 -v
```

## 5. Paths and Recovery

- [ ] Fix bare UNC share-root predicate; retain nested UNC support.
- [ ] Make directory sync strict and capture environment behavior.
- [ ] Recognize tombstone on Load; fail closed on dual/malformed/nonterminal state.
- [ ] Add cleanup atomic-replace and directory-sync checkpoints.
- [ ] Add safe recovery error wrapper and path/metadata redaction tests.
- [ ] Replace `byteReader` with `bytes.NewReader`.
- [ ] Add real-Journal bootstrap test proving unresolved tombstone blocks all later stages.

Focused validation:

```powershell
go -C backend test ./internal/desktop/paths ./internal/desktop/recovery ./internal/desktop/bootstrap -run 'Test(Resolve|Tombstone|TerminalCleanup|JournalErrors|Recovery)' -count=1 -v
```

## 6. Real Process and DACL Evidence

- [ ] Add full bootstrap same-session owner/contender helper with real mutex/activation and zero data/Runtime spies.
- [ ] Add pipe and recovery artifact DACL inspection.
- [ ] Extract shared protected descriptor helper only after behavior tests pass.
- [ ] Re-run existing process kill/abandoned/recovery-phase fixtures.

Focused validation:

```powershell
go -C backend test ./internal/desktop/... ./cmd/desktop -run 'TestProcess|TestDACL|TestCrash' -count=1 -v
```

## 7. Evidence and Regression

- [ ] Capture every red→green command in current task evidence.
- [ ] Correct archived Child 03 overall evidence disposition to pending external evidence while retaining observed command results.
- [ ] Run touched suites, Windows build, Linux cross-build and dependency guards.

```powershell
go -C backend test ./internal/desktop/... ./cmd/desktop ./cmd/server ./internal/application ./internal/config ./internal/service -count=1
go -C backend build ./...
$env:GOOS='linux'; $env:GOARCH='amd64'; $env:CGO_ENABLED='0'; go -C backend build ./...
Remove-Item Env:GOOS,Env:GOARCH,Env:CGO_ENABLED
```

Dependency guards:

```powershell
go -C backend test ./cmd/server ./internal/application -run 'TestServerDependencies|TestLinuxDependencyClosure' -count=1 -v
```

External pending evidence is listed, not waived.

## 8. Quality Check and PR Update

- [ ] Dispatch Trellis check against all REQ-RF/AC.
- [ ] Apply spec update for final mutex/activation/recovery contracts.
- [ ] Commit coherent fixes, push the existing branch, update PR #2 body and verification section.
- [ ] Re-run code review or record that all prior findings have a fix/evidence mapping.

## Rollback

The fix commits revert independently from the original Child 03 foundation. No schema/data conversion occurs. Protocol changes are not yet released, so clean cutover is required: migrate client/server/tests together with no compatibility shim.
