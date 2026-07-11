# E-C01-VERSION-NEGATIVE

Verified on 2026-07-11.

## Fixture matrix

`bun test scripts/version.test.ts` passed 18 tests (`artifact://75`). The temporary-fixture suite covers stable LF/CRLF input plus:

- missing and empty `VERSION`;
- `v1.2.0`, partial, prerelease, and multiline values;
- frontend package, Go fallback, Swagger source, Swagger JSON, Swagger YAML, and Swagger Go drift;
- Git Tag, Go linker value, Vite value, and Docker label drift.

Every negative case throws before returning a validated product version; the fixtures are deleted after each test.

## CLI/build failure evidence

All commands below exited with code 1:

- `bun scripts/version.ts --tag v1.2.1` → tag expected `v1.2.0`.
- `bun scripts/version.ts --go-version 1.2.1` → Go injection expected `1.2.0`.
- `bun scripts/version.ts --docker-version 1.2.1` → image version expected `1.2.0`.
- `GIST_PRODUCT_VERSION=1.2.1 bun run build` from `frontend/` → Vite config rejected the mismatch before producing a build.

The positive command `bun scripts/version.ts --tag v1.2.0 --go-version 1.2.0 --vite-version 1.2.0 --docker-version 1.2.0` returned `1.2.0`.

## Docker limitation

The workstation has no `docker` executable, so the Dockerfile's missing-ARG failure and built-image label inspection could not be executed locally. The Dockerfile now fails at `RUN test -n "$VERSION"`, injects the same ARG into Go/Vite, and labels the final image; both Docker workflows pass only the validated value.
