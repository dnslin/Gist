# E-C02-SNOWFLAKE

## Contract

- Snowflake IDs are produced by injected `snowflake.Generator` instances.
- One `BootstrapOwner` permits one successful initialization; a second initialization fails.
- No package-level mutable `Init` / `NextID` compatibility path remains.
- Runtime requires a generator before opening SQLite or starting writers.

## Implementation evidence

- `backend/pkg/snowflake/snowflake.go`: instance generator and one-shot bootstrap owner.
- ID-producing folder, feed, entry, AI summary/translation/list-translation, and domain-rate-limit repositories receive a generator through constructors.
- Repository tests and integration-tag service fixtures construct isolated generators; no shared test-global Snowflake state remains.
- `backend/internal/application/runtime.go` validates `IDGenerator` before `db.Open` and before activation.

## Verification

Passed on Windows 11 / Go 1.26.5:

```text
go test ./pkg/snowflake ./internal/repository/...
PASS

go test -tags=integration ./internal/service
PASS (artifact://141)

go test ./...
15 packages passed, 8 packages had no tests (artifact://140)
```

Focused negative tests cover invalid node, repeated initialization on one owner, failure then retry, concurrent single winner, independent generator isolation, monotonic unique IDs, and repository use of the injected generator.

## Environment limitation

`go test -race` could not build on this Windows workstation because `runtime/cgo` failed with `cgo.exe exit status 2`. Linux race remains required CI evidence.