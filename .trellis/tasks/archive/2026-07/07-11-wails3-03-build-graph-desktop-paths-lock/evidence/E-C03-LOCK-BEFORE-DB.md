# E-C03-LOCK-BEFORE-DB

Environment: Windows 11 amd64 (`windows/amd64`), Go 1.26.5, same interactive session, 2026-07-12.

## Executed

```powershell
go test ./internal/desktop/ownership -run "TestProcess|TestAbandoned|TestActivation" -count=1 -v
go test ./internal/desktop/... ./cmd/desktop ./cmd/server ./internal/application ./internal/config ./internal/service -count=1
```

Results: PASS (`artifact://131`, `artifact://134`).

## Observed contracts

- A real child process acquired `Global\\Gist.Data.v1.<sid-hash>.<root-hash>` with an explicit protected current-user + SYSTEM DACL. The test re-opened the kernel object and inspected its security descriptor.
- Owner metadata is stored under hashed `HKCU\\Software\\Gist\\Desktop\\Ownership\\v1\\<sid-hash>.<root-hash>` with the same protected DACL. It contains only version, PID, Terminal Services session ID, and random nonce. Raw SID/profile paths are absent.
- The implementation deliberately does not create a Global file mapping: Microsoft requires `SeCreateGlobalPrivilege` for that operation outside session zero, which would break standard-user desktop startup. The OS mutex handle remains the sole ownership fact.
- `ERROR_ALREADY_EXISTS` is followed by a zero-time mutex wait. Active ownership, an available race, and `WAIT_ABANDONED` are distinct outcomes. `TestAbandonedMutexIsAcquiredAndDiagnosed` keeps a spectator handle alive, kills the owner, and observes `OutcomeAcquiredAbandoned`.
- A same-session loser invokes only the activation client and still returns `data_owned_same_session`; an other-session classification does not contact a pipe. Bootstrap tests record zero logs/recovery/config/credential/generator/Runtime calls for losers.
- Owner termination releases the mutex automatically. A third acquisition succeeds without deleting a lock artifact. Lease close is idempotent.
- Bootstrap success and every injected failure clean up in reverse order; partial resources returned alongside construction errors are closed, and `lock_close` remains last.

## External evidence requirement

A real second Terminal Services session for the same Windows user was unavailable. The production mutex is in the Global namespace and session routing is fail-closed, but the native cross-session process fixture remains UNEXECUTED. This is an external Windows-runner evidence requirement, not a known code blocker and not represented as passed.
