# E-C03-CRASH-RECOVERY

Environment: Windows 11 amd64 on NTFS, Go 1.26.5, 2026-07-12.

## Executed

```powershell
go test ./internal/desktop/recovery -run "TestCrash|TestDurable|TestJournalState|TestRollbackRequired" -count=1 -v
go test ./internal/desktop/... ./cmd/desktop ./cmd/server ./internal/application ./internal/config ./internal/service -count=1
```

Results: PASS (`artifact://136`, `artifact://134`).

## Behavioral evidence

- The state table covers `prepared`, `applied`, `rollback_required`, `committed`, and `rolled_back`; rollback-required records cannot be committed.
- Real helper processes durably write each generic phase, are forcibly terminated, and are replayed before the DB-factory marker. Finish, rollback, terminal cleanup, and a second idempotent replay all pass.
- Envelope decoding is bounded before allocation and rejects empty/truncated/oversized data, invalid UTF-8, trailing JSON, unknown fields/schema/operation/phase, missing metadata, and unstable identifiers. Handler failure preserves the canonical record and blocks Runtime.
- Durable replace traces `temp_create -> temp_write -> file_sync -> atomic_replace -> directory_sync`; every boundary has an injected failure test. Recovery directory and temp files receive protected current-user + SYSTEM DACLs on Windows.
- Windows replacement uses same-directory `MoveFileEx(MOVEFILE_REPLACE_EXISTING | MOVEFILE_WRITE_THROUGH)`. A backup-semantics directory flush with write access is attempted; Windows `ERROR_ACCESS_DENIED`/`ERROR_INVALID_HANDLE` is accepted only after the write-through rename because unprivileged Windows does not generally expose a flushable directory handle.
- Durable cleanup moves the canonical journal with the same write-through primitive to a fixed tombstone. A directory-sync failure therefore leaves recovery evidence instead of reporting success after destructive removal; the next replay remains idempotent.
- Bootstrap ordering is activation -> logs -> recovery -> config -> credentials -> generator -> Runtime. Replay failure prevents the first Runtime/SQLite path.

No settings, backup, restore, update, installer, or other operation-specific payload was introduced.

## External evidence requirement

The generic crash-phase matrix is complete on this NTFS workstation. Native Linux filesystem/race execution remains an external CI requirement and is not claimed here.
