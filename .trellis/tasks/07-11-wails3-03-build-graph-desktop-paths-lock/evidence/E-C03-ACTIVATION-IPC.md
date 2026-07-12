# E-C03-ACTIVATION-IPC

Environment: Windows 11 amd64 (`windows/amd64`), Go 1.26.5, same interactive session, 2026-07-12.

## Executed

```powershell
go test ./internal/desktop/ownership -run "TestProcess|TestAbandoned|TestActivation" -count=1 -v
```

Result: PASS (`artifact://131`).

The fixture covers a real named-pipe round trip using a protected current-user + SYSTEM DACL. The transport uses message-mode named pipes plus explicit little-endian length framing, a 1024-byte body limit, and overlapped connect/read/write operations with two-second deadlines. Unknown version/action/fields, invalid UTF-8, malformed or oversized frames, trailing JSON, and a second frame in the same pipe message fail closed. The activation sink is called exactly once for the valid request and is not called for the trailing-frame fixture.

Same-session contention accepts only `activate`, maps connection/protocol failure to `occupied_unreachable`, and never permits bootstrap. Other-session contention returns `occupied_other_session` without opening a pipe. The server constructor accepts only `Identity` and `ActivationSink`; it has no Runtime, SQLite, config, journal, path payload, URL, credential, or business-command dependency.

## External evidence requirement

Real cross-session window focus is UNEXECUTED because no second Terminal Services session was available and Child 03 intentionally contains no Wails window. This is external product-evidence work, not claimed by the protocol fixture.
