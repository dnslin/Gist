# E-C01-WIN-BASELINE

Recorded before Child 01 production or test edits on 2026-07-11.

## Environment

- OS: Microsoft Windows 10.0.22631.6199 (Windows 11 build family)
- Go: `go version go1.26.5 windows/amd64`
- `GOOS`: `windows`
- `GOARCH`: `amd64`

## Configuration path baseline

Command, from `backend/`:

```powershell
go test ./internal/config -run '^TestLoad$' -count=1 -v
```

Result: failed, exit code 1. Raw harness output: `artifact://11`.

Observed contract mismatch:

- Test: `TestLoad`
- Fixture: `GIST_DATA_DIR=/tmp/gist`
- Expected: `/tmp/gist`
- Actual: `\tmp\gist`
- Classification: non-portable test fixture/assertion. Production uses host-platform `filepath.Clean` semantics.

## Icon path baseline

Command, from `backend/`:

```powershell
go test ./internal/service -run '^TestIconService_IsValidIconPath$' -count=1 -v
```

Result: failed, exit code 1. Raw harness output: `artifact://12`.

Observed contract mismatch:

- Test: `TestIconService_IsValidIconPath`
- Fixture: `/abs/icon.png`
- Expected: rejected (`false`)
- Actual: accepted (`true`)
- Classification: cross-platform security contract defect. Windows `filepath.IsAbs` does not recognize POSIX-rooted paths, so host-only path parsing permits foreign-platform absolute syntax.

No tests were skipped, renamed, conditionally allowed, or deleted while recording this baseline.
