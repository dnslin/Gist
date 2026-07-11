# E-C02-WRITER-REGISTRY

## Contract

`application.WriterRegistry` owns admission and quiet-point tracking for asynchronous local-data writers.

Writer classes:

- background: scheduler refresh and startup icon backfill; cancelled immediately during quiesce
- request/task-bound: OPML import and AI stream/cache work; initiating cancellation is preserved and graceful drain is allowed before deadline force-cancel

All registrations happen before goroutine launch. Admission rejection prevents a success response or channel publication.

## Writer inventory

- Scheduler `RefreshAll`: background registration per refresh; Stop cancels and waits.
- Startup icon backfill: Runtime activation background registration.
- OPML import: handler reserves one task-bound writer before HTTP 200; task context inherits the writer context. Refresh/backfill tail work completes inside the same reservation.
- AI `TranslateBlocks` and `TranslateBatch`: request-bound registration before channels are returned; stream and cache persistence share the linked context.
- Provider/network-only goroutines remain beneath their registered request operation and do not own independent local-data lifetimes.

## Defects found and closed during review

1. Nested OPML follow-up initially inherited the outer writer context and was cancelled when the outer token completed. It now runs synchronously inside the already admitted OPML task writer.
2. Parent-context cancellation initially left import status at `running`. A task-ID-scoped watcher now marks only the matching running task cancelled; stale cancellation cannot overwrite a replacement task.

Independent reviewer confirmed both findings closed with no remaining P0/P1 finish blocker.

## Verification

```text
go test ./internal/application -count=20
PASS

go test ./internal/service ./internal/handler ./internal/scheduler
PASS

go test -tags=integration ./internal/service
PASS (artifact://141)

go test ./...
15 packages passed, 8 packages had no tests (artifact://140)
```

Tests cover admission, exactly-once completion, invalid class, linked request/task cancellation, immediate background cancellation, graceful bound drain, deadline force-cancel, deadline retry, concurrent registration/completion/quiesce, OPML rejection-before-200, stale task cancellation, AI admission rejection, scheduler immediate/periodic registration and Stop wait.

## Environment limitation

Windows race build is unavailable because `runtime/cgo` fails with `cgo.exe exit status 2`; Linux race remains required.