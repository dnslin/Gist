# Error Handling

> How errors are handled in this project.

---

## Overview

Errors generally move through three layers: repositories return SQL/operational failures, services validate and classify domain outcomes, and handlers map service errors to HTTP responses. `FeedService.Add` in `backend/internal/service/feed_service.go` is the representative path: it returns `ErrInvalid`, translates a missing folder to `ErrNotFound`, returns `*FeedConflictError` for an existing feed, and wraps unexpected repository failures. `FeedHandler.Create` handles the typed conflict; other service errors go through `writeServiceError` in `backend/internal/handler/response.go`.

---

## Error Types and Propagation

Shared sentinels `ErrInvalid`, `ErrNotFound`, `ErrConflict`, and `ErrFeedFetch` live in `backend/internal/service/errors.go`. Callers classify them with `errors.Is`. `FeedConflictError.Is` matches `ErrConflict` while retaining the existing feed. Auth, proxy, and AI provider code also define feature-local sentinels in `auth_service.go`, `proxy_service.go`, and `service/ai/provider.go`.

Unexpected lower-level errors are commonly wrapped with an operation prefix and `%w`, preserving causal matching: `fmt.Errorf("check feed url: %w", err)` and `fmt.Errorf("get feed: %w", err)` in `feed_service.go`, and `fmt.Errorf("create db dir: %w", err)` in `internal/db/db.go`. Direct returns still occur in `settings_repository.go` and `domain_rate_limit_repository.go`, so wrapping is prevalent rather than universal.

Handlers reject malformed request bodies, path values, and query values before service calls (`feed_handler.go`, `entry_handler.go`, `proxy_handler.go`). Services enforce business validation and relationship existence (`feed_service.go`, `folder_service.go`, `domain_rate_limit_service.go`).

---

## API Error Responses

The general envelope is `{"error":"…"}`, represented by `errorResponse` in `backend/internal/handler/response.go`. `writeServiceError` maps:

- `ErrInvalid` → 400, `invalid request`
- `ErrNotFound` → 404, `resource not found`
- `ErrConflict` → 409, `conflict`
- `ErrFeedFetch` → 502, `feed fetch failed`
- unclassified errors → logged 500, `internal error`

`backend/internal/handler/response_test.go` verifies this mapping. Feature-specific handlers keep local mappings where the response contract differs: auth returns credential/account-specific statuses in `auth_handler.go`; proxy maps protocol, timeout, image, and upstream failures in `proxy_handler.go`; feed creation returns `feed_exists` plus `existingFeed` for `FeedConflictError`.

---

## Known Gaps and Cautions

Client-facing error disclosure is inconsistent. Cache-clear endpoints in `entry_handler.go`, `icon_handler.go`, and `settings_handler.go` return `err.Error()` in 500 JSON; readability returns raw upstream text in a 502; AI streaming sends raw error text in an SSE event. Other paths deliberately return generic messages. Therefore the repository does not establish a universal “never expose raw errors” rule; match the specific endpoint contract and avoid silently standardizing unrelated behavior.

Repository missing-row behavior is also mixed. Some methods preserve `sql.ErrNoRows`, while others return `(nil, nil)`; service translation must follow the called method's actual contract.
