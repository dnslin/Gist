# E-C01-VERSION-CONSUMERS

Verified on 2026-07-11.

- Root `VERSION`: `1.2.0`.
- `bun scripts/version.ts`: returned `1.2.0` after checking frontend package metadata, Go fallback, Swagger source, and all generated Swagger formats.
- Go linker injection: `go test -ldflags=-X=gist/backend/internal/config.AppVersion=9.9.9 ./internal/config -run '^TestGistUserAgent_DerivesAppVersion$' -count=1 -v` passed (`artifact://27`), proving the final user agent derives from the injected value.
- Frontend typed constant: focused Vitest passed; production `bun run build` passed and regenerated the PWA (`artifact://51`).
- Swagger: regenerated with pinned `swaggo/swag@v1.16.6`; source, `docs.go`, JSON, and YAML all report `1.2.0`.
- Dockerfile: requires `VERSION`, supplies it to Vite, injects it into `config.AppVersion`, and writes `org.opencontainers.image.version` on the final image.
- Release and develop Docker workflows call the same validator before build/push and pass only its output as the Docker build ARG.
- Product-version literal audit found only the root source, checked consumers, tests/fixtures, and the README tag example; dependency versions were classified as unrelated.

Compatibility endpoint, NSIS, Wails configuration, launcher, and installer assets were not created.
