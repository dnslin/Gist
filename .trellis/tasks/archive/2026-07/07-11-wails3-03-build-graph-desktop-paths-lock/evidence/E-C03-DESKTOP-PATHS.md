# E-C03-DESKTOP-PATHS

Environment: Windows 11 amd64 (`windows/amd64`), Go 1.26.5, 2026-07-12.

## Executed

```powershell
go test ./internal/desktop/... ./cmd/desktop ./cmd/server ./internal/application ./internal/config ./internal/service -count=1
```

Result: PASS, 9 packages (`artifact://134`).

`paths_test.go` changes cwd, sets conflicting `GIST_DATA_DIR` and `GIST_DB_PATH`, resolves from an injected LocalAppData root, compares every fixed path, and verifies that resolution creates no root or child directory. Empty, dot, relative, filesystem-root, volume-root, embedded-NUL, and resolver-error inputs fail with `desktop_paths_unavailable`. Production Windows resolution uses `windows.KnownFolderPath(FOLDERID_LocalAppData, KF_FLAG_DONT_VERIFY)` and does not consult server configuration or environment fallback logic.

## Disposition

The deterministic known-folder boundary and pure path matrix are complete on this workstation. A second Windows profile is not required for this pure value-object contract.
