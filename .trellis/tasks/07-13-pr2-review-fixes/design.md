# PR #2 Review Fixes Design

## 1. Diagnostic Feedback Loops

The implementation starts with focused tests that exercise the real failing seam:

| Bug | Red-capable command |
|---|---|
| UNC bare share root accepted | `go -C backend test ./internal/desktop/paths -run TestResolveRejectsUNCShareRoot -count=1` |
| Acquire error leaks returned lease | `go -C backend test ./internal/desktop/bootstrap -run TestAcquireErrorClosesReturnedLease -count=1` |
| mutex released from non-owner OS thread | `go -C backend test ./internal/desktop/ownership -run TestLeaseCloseFromDifferentThread -count=1 -v` |
| cancel returns before OVERLAPPED completion | `go -C backend test ./internal/desktop/ownership -run TestCancelDrainsOverlappedBeforeEventClose -count=1` |
| stalled activation returns raw timeout | `go -C backend test ./internal/desktop/ownership -run TestActivationClientStalledResponse -count=1 -v` |
| delayed second request reaches sink | `go -C backend test ./internal/desktop/ownership -run TestDelayedSecondRequestRejectedBeforeActivation -count=1 -v` |
| tombstone ignored on startup | `go -C backend test ./internal/desktop/recovery -run TestTombstoneReplayRetriesCleanup -count=1` |
| recovery error leaks path | `go -C backend test ./internal/desktop/recovery -run TestJournalErrorsRedactRecoveryPath -count=1` |
| OS contender evidence is ownership-only | `go -C backend test ./internal/desktop/bootstrap -run TestProcessContenderDoesZeroDataWork -count=1 -v` |

Each red result is captured before production edits. Current-green coverage gaps are added after the two known unit defects are fixed.

## 2. Ranked Hypotheses

1. **Mutex close fails because ownership is thread-affine.** Prediction: acquisition on pinned thread A followed by `Close` on pinned thread B yields `ERROR_NOT_OWNER`; a dedicated owner-thread worker removes the failure.
2. **Activation lifetime bugs come from treating cancellation as completion.** Prediction: scripted pending I/O shows event/handle close before `GetOverlappedResult`; cancel-and-drain changes ordering and stabilizes timeout recovery.
3. **Per-connection trailing rejection is impossible on one duplex pipe without request completion.** Prediction: delayed second write after `PeekNamedPipe` reaches the sink; separating request and response channels lets the server wait for request EOF before activation.
4. **Recovery false-success comes from treating tombstone/dir-sync as optional.** Prediction: canonical-absent + tombstone-present currently returns nil; recognizing the tombstone and propagating sync failure blocks bootstrap.
5. **Acceptance drift comes from aggregating focused command success into overall pass.** Prediction: machine-readable evidence lists pending external artifacts while `check.json` says passed; changing disposition to pending resolves the contradiction without changing product code.

## 3. Thread-affine Mutex Owner

`WindowsAcquirer.Acquire` creates a dedicated owner goroutine which immediately calls `runtime.LockOSThread`. That worker owns the complete mutex lifecycle:

```text
owner thread:
  CreateMutex / WaitForSingleObject
  -> publish HKCU owner metadata
  -> reply Acquisition
  -> wait close request
  -> clear metadata
  -> ReleaseMutex
  -> CloseHandle
  -> runtime.UnlockOSThread
```

`windowsLease.Close` sends an idempotent close request and waits for the final result. It never calls `ReleaseMutex` itself. Acquisition failure before publishing a lease cleans all owner-thread resources before returning. Process termination still releases the kernel mutex.

The worker returns a typed diagnostic on `WAIT_ABANDONED`; bootstrap records it after logs and before recovery through:

```go
type DiagnosticRecorder interface {
    Record(context.Context, ownership.Diagnostic) error
}
```

No logger, Runtime or DB dependency enters `ownership`.

## 4. Bootstrap Cleanup

The ownership result is normalized before routing:

- `err + Lease`: close lease, join errors, zero stages.
- non-acquired outcome + unexpected Lease: close defensively, then route/error.
- acquired outcome without Lease: fail closed.
- acquired abandoned: activation/logs, record diagnostic, then recovery.

Existing LIFO stack remains. Regression tests drive real constructor result combinations instead of relying only on `Checkpoint`.

## 5. Activation Protocol v1 Completion

The current one-duplex-pipe design cannot prove that a client will not write again before executing the sink. The revised v1 transport separates channels:

1. Client creates a nonce-scoped response pipe using the existing hashed user/session/root identity.
2. Client connects to the fixed request pipe and sends one strict request:

```json
{"version":1,"action":"activate","nonce":"<128-bit hex>"}
```

3. Client flushes and closes the request pipe connection.
4. Server reads until connection EOF. Any second frame/bytes fail before side effects.
5. Server validates nonce and derives the response pipe name itself; no path is accepted from the request.
6. Server invokes `ActivationSink` only after EOF proves request completion.
7. Server connects to the derived response pipe and writes the strict result frame.

The nonce is a domain type, fixed lowercase hex length, generated by the client. Response pipe ACL remains current-user + SYSTEM. Replay/guessing is bounded by random nonce and one-shot pipe lifetime.

## 6. Overlapped I/O Lifetime

All pending operations use one helper with injected WinAPI seams for deterministic lifetime tests:

```text
start OVERLAPPED
-> wait event/context
-> on cancel: CancelIoEx
-> GetOverlappedResult(wait=true)
-> classify final result
-> close event/handle
```

`ERROR_NOT_FOUND` from `CancelIoEx` is accepted only as a completion race, then final completion is still collected. Public activation errors use a safe wrapper:

- malformed/oversize/trailing/deadline/unexpected I/O → `ErrActivationProtocolInvalid`
- fixed owner endpoint absent during contender routing → `ErrOwnerUnreachable`

Raw Win32 errors remain unwrap causes for diagnostics but their text is not used as business behavior.

## 7. DesktopPaths Root Predicate

After `Clean` and `Abs`, reject root-only when:

```go
volume := filepath.VolumeName(base)
rootOnly := volume != "" && (base == volume || base == volume+string(filepath.Separator))
```

Windows tests cover `C:\`, `\\server\share`, `\\server\share\`, and valid `\\server\share\redirected\Local`.

## 8. Recovery Protocol

### 8.1 Strict sync

`syncDirectory` returns every open/flush error. `ACCESS_DENIED` and `ERROR_INVALID_HANDLE` are no longer success. If the target LocalAppData filesystem cannot provide the required operation, recovery remains fail-closed and the limitation is surfaced as evidence instead of weakening the PRD.

### 8.2 Tombstone startup

`FileStore.Load` has three states:

```text
canonical exists -> load canonical
canonical absent + tombstone exists -> load tombstone as pending cleanup
both absent -> ErrAbsent
both exist -> ErrFailed/corrupt, preserve both
```

A terminal tombstone bypasses operation handlers and retries the outstanding cleanup directory sync. A malformed/nonterminal tombstone fails closed. Successful retry is idempotent; the fixed tombstone remains terminal evidence unless a separately durable delete protocol is introduced.

### 8.3 Cleanup faults

`Remove` exposes explicit checkpoints for cleanup atomic replace and directory sync. A move fault leaves canonical intact. A sync fault leaves tombstone intact. Bootstrap cannot continue while either cleanup step is unresolved.

### 8.4 Safe errors

A recovery wrapper stores the unwrap cause but renders only stable category and safe phase. It never formats `os.PathError.Path`, raw metadata, credential data, or handler text. `errors.Is` continues to work.

`byteReader` is replaced with `bytes.NewReader` after behavior tests pass.

## 9. Windows Security Helper

After pipe and recovery DACL behavior tests exist, extract only protected SYSTEM + current-user descriptor construction into `internal/desktop/security`. Resource-specific registry, mutex, named-pipe and filesystem security application remains with the owning package.

## 10. Evidence Disposition

The archived Child 03 evidence is corrected without rewriting historical command results:

- each observed command retains `passed`;
- overall disposition becomes `pending_external_evidence`;
- pending artifacts name second TS session, native Ubuntu race/lint, Docker smoke, Windows race, rollback drill and power-loss boundary;
- new PR-fix task evidence records red/green feedback loops and current environment verification.

PR #2 remains the only PR and receives the new commits/body updates.
