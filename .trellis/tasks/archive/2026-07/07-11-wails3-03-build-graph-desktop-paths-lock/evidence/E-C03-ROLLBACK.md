# E-C03-ROLLBACK

Environment: Windows 11 amd64, Go 1.26.5, 2026-07-12.

## Executed scope verification

```powershell
go test ./internal/desktop/... ./cmd/desktop ./cmd/server ./internal/application ./internal/config ./internal/service -count=1
go build ./...
$env:GOOS='linux'; $env:GOARCH='amd64'; $env:CGO_ENABLED='0'; go build ./...
```

Results: PASS (`artifact://134`; both builds exited 0).

The cutover remains isolated to `backend/internal/desktop/**`, the Windows-only `backend/cmd/desktop` seam, the application dependency guard, and this task's artifacts. There is no Wails dependency, runnable fake desktop product, second Runtime composition root, frontend mode, schema/migration, server environment change, file-lock fallback, or code that deletes the real desktop root. Rollback removes the Child 03 code/tests/guards as one unit and must leave `%LOCALAPPDATA%\\Gist`, canonical recovery records, and recovery tombstones untouched.

## External evidence requirement

The isolated-worktree commit revert/reapply drill is UNEXECUTED because the active Child 03 changes have no completed commit and this check agent may not commit or mutate lifecycle state. It remains an external fixed-point evidence requirement, not a code blocker and not claimed as passed.
