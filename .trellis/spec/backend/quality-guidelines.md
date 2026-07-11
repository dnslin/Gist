# Quality Guidelines

> Code quality standards for backend development.

---

## Overview

The enforced backend quality surface is defined by `backend/Makefile`, `backend/.golangci.yml`, and `.github/workflows/backend-test.yml`. From `backend/`, the available commands are:

```sh
make test   # go test ./... -race -v
make lint   # golangci-lint run
make build  # go build -o gist-server ./cmd/server
make cover  # writes coverage.out and renders coverage
make gen    # go generate ./...
```

The root `README.md` directs backend contributors to `make test` and `make lint`.

---

## Enforced Checks

CI downloads modules, runs `go build -v ./...`, then `go test ./... -race -coverprofile=coverage.out -covermode=atomic`. Coverage is uploaded with `fail_ci_if_error: false`; neither CI nor the Makefile sets a minimum threshold. A separate job runs the latest golangci-lint.

`backend/.golangci.yml` uses golangci-lint v2 with readonly module downloads. The enabled linters are only `govet`, `ineffassign`, `staticcheck`, `unused`, and `misspell`, with unlimited issue caps. Although the file contains settings or exclusions for `errcheck`, `revive`, and `gocritic`, those linters are not enabled and must not be described as enforced.

---

## Test Patterns

- Colocate tests as `*_test.go`, commonly in external packages, and use `testify/require` for observable results, errors, HTTP statuses, and JSON. Examples: `backend/internal/service/feed_service_test.go`, `internal/handler/feed_handler_test.go`, and `internal/repository/feed_repository_test.go`.
- Handler and middleware tests use Echo plus `httptest`; shared helpers are in `internal/handler/test_helpers_test.go`. `internal/handler/response_test.go` and `internal/http/middleware_test.go` assert client-visible contracts.
- Repository tests use real migrated in-memory SQLite via `internal/repository/testutil/db.go`; migration behavior is tested directly in `internal/db/migrations_test.go`.
- Interfaces with `//go:generate mockgen …` produce same-layer `mock/` files. Tests configure them with `gomock.NewController(t)`. Regenerate with `make gen` rather than editing generated mocks manually.

Existing contract-focused examples cover service validation/error conversion, response error mapping, auth middleware status/cookies, router registration, and migration transformation/idempotence (`feed_service_test.go`, `response_test.go`, `middleware_test.go`, `router_test.go`, `migrations_test.go`). These examples are not evidence of a repository-wide coverage rule.

---

## Documented Review Requirements

The tracked root `.rules` requires backend review to verify all of the following, whether or not a linter enforces them:

- Unit tests use black-box `package name_test` packages and validate behavior through public APIs.
- Tests use `testify/require`, not `testify/assert`; GoMock files remain generated in the defining package's `mock/` directory.
- Dependencies are injected through constructors. Global state variables are prohibited except configuration objects, and only necessary APIs are exported.
- State shared between goroutines is synchronized with `sync/atomic`, `sync.Mutex`, or channels; unsynchronized writes are prohibited.
- File operations clean paths with `filepath.Clean`, and source contains no hard-coded secrets.
- Servers handle SIGINT and SIGTERM with graceful shutdown rather than directly terminating with `os.Exit`.
- Before submission, run `make test` and `make lint` from `backend/`.

## API Change Checklist

For every new or changed API endpoint or DTO, review and complete each applicable item below. These are **MUST** contracts from the tracked root `.rules`, not optional cleanup:

1. **UTC RFC3339 timestamps:** Every timestamp exposed by a handler **MUST** be serialized in UTC using RFC3339 (RFC3339Nano is compatible where sub-second precision is required). Positive boundary mappers are `toEntryResponse`, `toFeedResponse`, and `toFolderResponse` in `backend/internal/handler/entry_handler.go`, `feed_handler.go`, and `folder_handler.go`, which call `UTC().Format(time.RFC3339)`.
2. **AI cache/stream branch:** AI cache hits **MUST** return a JSON response that identifies the cached result; uncached AI execution **MUST** use a correctly framed SSE response. `AIHandler.Summarize` and `AIHandler.Translate` in `backend/internal/handler/ai_handler.go` demonstrate the cache lookup and content-type branch; translation's `data:` events are the positive SSE framing reference. Tests MUST cover both content types and event framing.
3. **Cross-layer DTO synchronization:** A backend handler request/response DTO change **MUST** update the matching frontend types under `frontend/src/types/` in the same change, including Snowflake IDs as `string` and timestamps as strings. `frontend/src/types/api.ts` mirrors Folder, Feed, and Entry handler responses; `frontend/src/types/settings.ts` mirrors settings and domain-rate-limit DTOs. The typed calls in `frontend/src/api/index.ts` are consumers that MUST continue to compile against those contracts.
4. **Swagger source and generated artifacts:** Changed handlers **MUST** carry accurate swag annotations, using paths relative to the global `@BasePath /api` in `backend/cmd/server/main.go` and declaring every produced media type. After annotation changes, run exactly from `backend/`:

   ```sh
   swag init -g cmd/server/main.go --parseDependency --parseInternal
   ```

   The regenerated `backend/docs/docs.go`, `swagger.json`, and `swagger.yaml` MUST match the actual routes registered below `internal/http.NewRouter`'s `/api` group. `AIHandler.Summarize` and `Translate` are positive examples of colocated annotations declaring JSON and `text/event-stream`.

### Existing API deviations

- Uncached summary currently sets `text/event-stream` but writes plain text chunks in `backend/internal/handler/ai_handler.go`; only translation emits properly framed `data:` events. This is an existing violation of the SSE contract, not permission to call arbitrary chunked text SSE.
- AI transport DTOs currently live in `frontend/src/api/index.ts` rather than the policy-required `frontend/src/types/`. This is existing placement debt, not a second synchronization convention.
- Domain-rate-limit and proxy Swagger annotations include `/api` even though global `@BasePath /api` already supplies it, producing duplicate-prefixed generated paths. New or changed annotations MUST use relative paths.

---

## Observed Exceptions and Enforcement Gaps

- Repeated `export_test.go` aliases/accessors in `internal/handler`, `internal/service`, and `internal/db` let external test packages reach private helpers. They are current exceptions to strict public-API-only black-box testing, not a replacement for the documented rule.
- `backend/cmd/server/main.go` handles normal shutdown signals gracefully but calls `os.Exit(1)` on fatal startup failures. Those startup exits are observed exceptions to the documented no-direct-`os.Exit` requirement.
- Do not claim formatter enforcement: no formatter command or `.editorconfig` was found in the repository conventions inspected by the backend scout.
- No static rule enforces layer boundaries, error wrapping, logging attributes, constructor injection, synchronization, path cleaning, lifecycle behavior, or test presence; review them against `.rules` and nearby source patterns.
- Do not hand-maintain generated mocks or describe disabled linters and a coverage threshold as active gates.
- `backend/pkg/snowflake/snowflake.go` holds a mutable package-level `node`, overwritten by `Init` and consumed by `NextID`; `backend/cmd/server/main.go` initializes it before repository use. This violates the `.rules` prohibition on non-configuration global state and constructor-injection requirement. Treat it as existing global-state debt, not an approved ID-generation pattern; new mutable package globals MUST NOT be introduced.

---

## Scenario: Unified Product Version

### 1. Scope / Trigger

Any product-version bump or build/release change touches the repository root, Go, Vite, generated Swagger metadata, and Docker. Root `VERSION` is the only manually maintained product version source; package metadata and code fallbacks are checked consumers.

### 2. Signatures

```sh
bun scripts/version.ts [--root <repository>] [--tag vX.Y.Z] \
  [--go-version X.Y.Z] [--vite-version X.Y.Z] [--docker-version X.Y.Z]
```

Go release builds inject `gist/backend/internal/config.AppVersion` with linker `-X`. Vite exposes the typed `__GIST_VERSION__` build constant. Docker accepts the required `VERSION` build argument and writes `org.opencontainers.image.version`.

### 3. Contracts

- `VERSION`: UTF-8 stable SemVer `X.Y.Z`, no `v`, prerelease, metadata, spaces, or additional lines; LF and CRLF endings are accepted.
- `frontend/package.json`, Go fallback, Swagger source, `docs.go`, `swagger.json`, and `swagger.yaml` MUST equal `VERSION`.
- Stable tags MUST equal `v` + `VERSION`.
- `GistUserAgent` and `DefaultUserAgent` MUST derive from the final linker-injected `AppVersion`; runtime code MUST NOT read repository files.
- Docker workflows MUST validate metadata and container smoke contracts before pushing an image.

### 4. Validation & Error Matrix

| Condition | Required result |
|---|---|
| Missing, empty, multiline, partial, `v`-prefixed, or prerelease `VERSION` | Validator exits nonzero |
| Package, Go fallback, or any Swagger metadata differs | Validator exits nonzero |
| Tag or supplied Go/Vite/Docker value differs | Validator exits nonzero |
| Docker `VERSION` argument is empty | Docker build fails before application compilation |
| OCI label, HTTP startup, or non-root server process check fails | Workflow MUST NOT push the image |

### 5. Good/Base/Bad Cases

- Good: `VERSION=1.2.0`, tag `v1.2.0`, every consumer reports `1.2.0`.
- Base: local non-release Go and Vite builds use checked deterministic values without Git access.
- Bad: deriving the version from a tag/package, embedding a second test expectation, or publishing before inspecting the built image.

### 6. Tests Required

- `bun test scripts/version.test.ts`: valid LF/CRLF input plus every invalid/drift case.
- Go focused test with linker `-X`: assert `GistUserAgent` contains the injected value.
- Frontend test: compare `productVersion` with root `VERSION`, never a copied literal.
- Docker workflow: assert OCI label, `/api/auth/status`, and the `gist-server` process user before push.
- Run `actionlint` after changing GitHub Actions workflows.

### 7. Wrong vs Correct

#### Wrong

```yaml
run: echo "value=$(bun scripts/version.ts)" >> "$GITHUB_OUTPUT"
```

`echo` can mask the validator's nonzero exit status.

#### Correct

```yaml
run: |
  VERSION="$(bun scripts/version.ts)"
  echo "value=$VERSION" >> "$GITHUB_OUTPUT"
```

The assignment preserves command-substitution failure and stops the workflow.

## Cross-Platform Persisted Path Contract

Persisted icon paths MUST be portable single filenames, not host-native relative paths. Validation rejects both slash styles, colon-bearing drive/device syntax, absolute roots, `.` and `..`. Producers MUST normalize before persistence: IPv6 hostnames use the deterministic `ipv6-` plus hex-encoded hostname form. Tests MUST cover foreign-platform path syntax on the current host; `filepath.IsAbs` alone is insufficient because it only understands the host OS grammar.
