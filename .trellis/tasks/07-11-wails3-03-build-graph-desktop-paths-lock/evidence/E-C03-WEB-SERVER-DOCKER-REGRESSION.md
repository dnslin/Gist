# E-C03-WEB-SERVER-DOCKER-REGRESSION

Environment: Windows 11 amd64, Go 1.26.5, with Linux cross-build through `GOOS=linux GOARCH=amd64 CGO_ENABLED=0`, 2026-07-12.

## Executed

```powershell
go test ./internal/desktop/... ./cmd/desktop ./cmd/server ./internal/application ./internal/config ./internal/service -count=1
go test ./cmd/server ./internal/application -run "TestServerDependencies|TestLinuxDependencyClosure" -count=1 -v
go build ./...
$env:GOOS='linux'; $env:GOARCH='amd64'; $env:CGO_ENABLED='0'; go build ./...
```

Results: PASS. The touched suite passed 9 packages (`artifact://134`); both Linux dependency-closure guards passed (`artifact://132`); native Windows and Linux cross-builds exited 0.

`cmd/server` and `internal/application` reject `github.com/wailsapp/`, `gist/backend/internal/desktop`, and `golang.org/x/sys/windows` from their Linux dependency closures. The desktop seam remains Windows build-tagged. No Wails dependency, frontend desktop mode, Bun lock change, Dockerfile change, server environment change, DB schema/migration change, listener/route change, or second application composition root was introduced.

## External evidence requirements

Native Ubuntu test/race execution and Docker build/smoke are UNEXECUTED on this Windows workstation. The requested Linux cross-build and dependency closure were executed; they are not represented as substitutes for native Linux/Docker evidence. A focused Windows race probe was attempted but the installed toolchain failed before package compilation with `runtime/cgo ... cgo.exe: exit status 2` (`artifact://123`); Linux CI race remains required.
