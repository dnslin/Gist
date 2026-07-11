# E-C01-WEB-SERVER-DOCKER-REGRESSION

Verified on Windows 10.0.22631.6199 / Go 1.26.5 / Bun 1.3.14 on 2026-07-11.

## Passed

- Windows focused path tests, two consecutive runs: `artifact://15`, `artifact://17`.
- Affected backend packages (`config`, `service`, `http`, `handler`): `artifact://52`.
- Final full Windows backend suite: 14 packages passed, 8 packages had no tests (`artifact://109`).
- `go vet ./...`: passed with no output.
- Frontend suite: 38 files, 442 tests passed.
- Frontend ESLint: passed.
- Frontend TypeScript + Vite production build: passed; PWA generated 57 precache entries plus `sw.js` and Workbox (`artifact://110`).
- Root version suite: 18 tests passed (`artifact://108`); product-version consistency returned `1.2.0`.
- `actionlint` passed all GitHub Actions workflows with no findings.
- Independent reviewer confirmed no remaining P0/P1 blocker after fixes to pre-push image verification, validator exit propagation, frontend single-source testing, and IPv6 favicon filenames.
- Existing router package passed in both current and rollback worktrees; no route, status, schema, listener, database, scheduler, or runtime configuration code was changed.

## Environment limitations

- `go test ./... -race` could not build because Windows cgo exited with status 2 (`artifact://53`). The non-race full suite and `go vet` passed. Linux race execution remains a CI requirement.
- `golangci-lint` is not installed on this workstation; the configured CI lint job remains authoritative.
- `docker` is not installed, so local image build, OCI label inspection, container startup, non-root process, entrypoint, port, `/app/data`, and `/api/auth/status` smoke checks remain unexecuted. Both Docker workflows now build locally, verify the label plus HTTP/non-root process contracts, and only then push.
