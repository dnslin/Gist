# E-C02-WRITER-REGISTRY

## Contract

`application.WriterRegistry` owns admission and quiet-point tracking for asynchronous local-data writers.

Writer classes:

- background: scheduler refresh and startup icon backfill; cancelled immediately during quiesce
- request/task-bound: OPML import and AI stream/cache work; initiating cancellation is preserved and graceful drain is allowed before deadline force-cancel

All registrations happen before goroutine launch. Admission rejection prevents a success response or channel publication.

## Writer inventory

- Scheduler `RefreshAll`: reserves a background writer per refresh; Stop cancels and waits. Periodic execution uses an injected instance clock in tests.
- Startup icon backfill: reservation is acquired during Runtime build and launched only after the complete graph is built.
- OPML import: handler reserves admission, synchronously publishes the task, transfers ownership from the HTTP request, launches, then returns HTTP 200. Task cancellation and Runtime cancellation remain linked. Core completion marks the task done before refresh/backfill tail work; the same reservation stays active until the tail exits.
- AI `TranslateBlocks` and `TranslateBatch`: request-bound reservation occurs before channels are published; stream and cache persistence share the linked context.
- Provider/network-only goroutines remain beneath their registered request operation and do not own independent local-data lifetimes.

## Defects found and closed during review

1. `LaunchWriter` coupled admission and goroutine launch, allowing HTTP 200 before task publication. It was replaced by the single `ReserveWriter -> Publish -> Launch/Release` path.
2. OPML used `context.Background()`, discarding pre-accept request cancellation. Reservations now retain initiating cancellation until synchronous task publication, after which the accepted task survives normal HTTP handler return while task/Runtime cancellation remain effective.
3. Refresh/backfill delayed observable task completion. Import now returns a follow-up descriptor; the handler completes the task first and executes tail work under the same admitted reservation without a nested untracked goroutine.
4. `WriterClass` existed independently in service and application. `service.WriterClass` is now canonical and the conversion switch is removed.
5. Parent-context cancellation could leave import status at `running`. The task-ID-scoped watcher marks only the matching running task cancelled, so stale cancellation cannot overwrite a replacement task.

## Verification

```text
go test ./internal/application ./internal/service ./internal/scheduler ./internal/handler ./internal/http -count=1
PASS (artifact://117)

go test ./cmd/server -run TestShutdownRunnerForcesHTTPHandlersBeforeRuntimeClose -count=20
PASS (artifact://153)

go test -tags=integration ./internal/service -count=1
PASS (artifact://154)

go test ./... -count=1
16 packages passed, 7 packages had no tests (artifact://155)
```

Tests cover reservation admission, publish ownership transfer, safe unlaunched release, exactly-once completion, invalid class, pre-publication initiating cancellation, accepted-task survival after HTTP completion, task/root cancellation, immediate background cancellation, graceful bound drain, deadline force-cancel and retry, concurrent lifecycle calls, OPML publish-before-200, task-done-before-tail, rejection-before-success, AI admission/cancellation, controlled scheduler ticks, and Stop cancel/wait.

## Environment limitation

Windows race build is unavailable because `runtime/cgo` fails with `cgo.exe exit status 2` (`artifact://147`); Linux race remains required.