# Logging Guidelines

> How logging is done in this project.

---

## Overview

Application code MUST use the project wrapper `gist/backend/pkg/logger`. The tracked root `.rules` prohibits the standard `log` package, direct `log/slog` calls, and `fmt.Print*` for logging. Direct `slog` construction and calls are confined to `backend/pkg/logger/logger.go`, which is the only allowed facade implementation; all handlers, services, middleware, schedulers, and process startup code call `logger.Debug`, `Info`, `Warn`, or `Error`. Startup invokes `logger.Init(logger.ParseLevel(cfg.LogLevel))` in `backend/cmd/server/main.go`. `backend/pkg/logger/logger.go` configures a lowercase-level `slog.NewTextHandler` writing to stdout. `GIST_LOG_LEVEL` is loaded in `backend/internal/config/config.go`; documented values are `debug`, `info`, `warn`, and `error` (`README.md`, `docker/Dockerfile`).

---

## Log Levels

- **Debug:** high-volume successful reads, validation failures, cache hits, and static serving. See `backend/internal/http/middleware.go`, `internal/handler/ai_handler.go`, and `internal/http/static.go`.
- **Info:** successful state changes, lifecycle events, and scheduled work. See `internal/handler/feed_handler.go`, `internal/scheduler/scheduler.go`, and `cmd/server/main.go`.
- **Warn:** expected or recoverable rejection/failure paths such as missing auth, conflicts, fetch failures, and cancellation. See `internal/http/middleware.go`, `internal/service/feed_service.go`, and `internal/handler/auth_handler.go`.
- **Error:** unexpected operation failures and HTTP 5xx results. See `internal/handler/response.go`, `internal/http/middleware.go`, and `cmd/server/main.go`.

---

## Mandatory Logging API Boundary

- Production code **MUST NOT** import or call the standard `log` package for application logging.
- Production code outside `backend/pkg/logger` **MUST NOT** import or call `log/slog` directly. The wrapper alone may construct handlers and invoke slog.
- Production code **MUST NOT** use `fmt.Print`, `fmt.Printf`, or `fmt.Println` as logging substitutes. `fmt.Fprintf` to an HTTP response body, such as AI stream writes in `backend/internal/handler/ai_handler.go`, is transport output rather than logging and does not relax this rule.
- New log behavior **MUST** be added through `pkg/logger`; do not create a second facade or pass raw slog loggers through the application graph.

`backend/pkg/logger/logger.go` is the positive policy boundary, and current production source routes logging through it. These prohibitions come from tracked `.rules` even though the linter configuration does not mechanically enforce them.

---

## Structured Fields

Project policy in the tracked root `.rules` requires business logs to include `module`, `action`, `resource`, and `result`; failed operations add `error`. Field names must use snake_case. Domain context commonly adds `feed_id`, `folder_id`, `entry_id`, `host`, `provider`, `model`, `count`, or `type`; observed `module` values include `handler`, `service`, `http`, and `scheduler`, and observed results include `ok`, `failed`, `cancelled`, `skipped`, and `hit`.

Current source is not mechanically universal: some startup and shutdown logs in `backend/cmd/server/main.go` contain only a message and sometimes `error`. These are observed exceptions, not permission to omit the four required fields from new business logs.

---

## Request Logging

`RequestLoggerMiddleware` in `backend/internal/http/middleware.go` emits one log after the downstream handler. It selects Error for status >=500, Warn for 4xx, and Debug otherwise, and includes `method`, `path`, `status_code`, `duration_ms`, `remote_ip`, and `user_agent` alongside common fields. `backend/internal/http/router.go` installs both Echo recovery and this middleware.

---

## Sensitive Data, Layer Boundary, and Observed Exceptions

The tracked `.rules` policy requires URL logging to use `network.ExtractHost()` rather than full URLs, prohibits logging password, token, or API-key values, and prohibits repository-layer logging: repositories return errors for service or higher layers to log. These are review requirements even though no automated check enforces them.

Observed logs include account identifiers as `actor`, plus request `remote_ip` and `user_agent`; no observed call logs passwords, JWTs, or API keys. Treat those identifier fields as current source behavior, not as an exception to the secret prohibition. Startup/shutdown field omissions described above are also existing deviations from the required business-log schema.
