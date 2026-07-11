# Quality Guidelines

> Executable frontend quality tooling and observed test practices.

---

## Commands and Configuration

The tracked `.rules` policy makes **Bun the mandatory package manager**. Dependency installation, lockfile updates, and frontend scripts **MUST** use Bun; npm, pnpm, and Yarn commands or lockfiles must not be introduced. `bun.lock` is the repository lockfile, `frontend/package.json` defines the scripts, and `.github/workflows/frontend-test.yml` installs Bun and uses a frozen lockfile.

The supported commands are defined in `frontend/package.json`:

```sh
bun run lint   # eslint .
bun run test   # vitest run
bun run build  # tsc -b && vite build
bun run test:coverage # vitest run --coverage
```

`README.md` also lists the frontend test and lint commands. `frontend/vitest.config.ts` uses jsdom and globals, includes `src/**/*.{test,spec}.{ts,tsx}`, and configures V8 text, JSON, and HTML coverage while excluding declaration files and `main.tsx`.

The tracked `.rules` test contract is stricter: frontend tests **MUST** be colocated with their source and named `*.test.ts` (`.rules` lines 81-83). `frontend/vitest.config.ts` currently broadens discovery to `.spec.ts`, `.spec.tsx`, and `.test.tsx`; that include pattern is a deviation and **MUST NOT** be treated as permission for new filenames. Existing `.test.tsx` files—including `hooks/useAddFeed.test.tsx`, `components/entry-list/EntryList.test.tsx`, `components/entry-list/EntryListItem.test.tsx`, `components/picture-masonry/PictureMasonry.test.tsx`, `components/picture-masonry/Lightbox.test.tsx`, `components/entry-content/EntryContentBody.test.tsx`, `components/settings/tabs/EditFeedDialog.test.tsx`, `components/layout/three-column-layout.test.tsx`, and `components/ui/image-preview.test.tsx`—are tracked-rule deviations to migrate when touched.

`frontend/eslint.config.js` is a flat configuration combining JavaScript recommended, typescript-eslint recommended, react-hooks recommended, and react-refresh's Vite rules. It ignores `dist` and `coverage`. The only observed file-specific lint exception disables `react-hooks/incompatible-library` for `EntryList.tsx`.

## Continuous Integration and Review Requirements

`.github/workflows/frontend-test.yml` is the established frontend CI gate for pushes and pull requests affecting `frontend/**` or the workflow itself. It uses Bun with a frozen lockfile and runs:

- **Test job:** `bun run build`, then `bun run test:coverage`; `frontend/coverage/coverage-final.json` is uploaded to Codecov with `fail_ci_if_error: false`, so upload failure is non-blocking while build and coverage-test failures block the job.
- **Lint job:** `bun run tsc -b`, then `bun run lint` as a separate type-check/lint gate.

Frontend review must enforce the tracked `.rules` contracts: Props and API responses use interfaces; no new `any`; `@/` is used for `src/` imports; untrusted HTML uses the sanitizing `unified`/`rehype`/`hast-util-to-jsx-runtime` path; streaming resources are cleaned up and replacement batches cancel prior work; each translation key updates both English and Chinese resources; tests use Vitest + jsdom and remain colocated. Observed repository deviations must be called out explicitly rather than weakening these requirements.

## Test Organization and Scope

Tests are colocated with source and cover multiple layers:

- `lib/router.test.ts` exercises route and query boundaries.
- `stores/lightbox-store.test.ts` covers initial state, boundary behavior, and close-versus-reset regressions.
- `hooks/useAddFeed.test.tsx` mocks the API and verifies folder typing and failure results.
- `components/entry-list/EntryList.test.tsx` covers component behavior.
- `api/entries.test.ts` covers API behavior.

Several tests preserve explicit historical contracts: `lightbox-store.test.ts` ensures close does not clear animation state; `router.test.ts` ensures content type remains represented in the URL; `components/picture-masonry/Lightbox.test.tsx` covers a click-region regression. Component tests also mock i18n, stores, and DOM observers where needed, as seen in `EntryListItem.test.tsx` and `EntryList.test.tsx`.

## Accessibility and UX Practices

Radix-backed primitives provide the base behavior for dialogs, switches, and dropdowns in `components/ui/dialog.tsx`, `switch.tsx`, and `dropdown-menu.tsx`. Several icon buttons use translated `aria-label` values in `FeedUrlForm.tsx`, `SettingsModal.tsx`, and `EntryContentHeader.tsx`. `frontend/src/index.css` respects `prefers-reduced-motion` for dropdown and AI-thinking animations and defines PWA safe-area tokens used by `components/ui/dialog.tsx` and `App.tsx`.

## Known Gaps and Cautions

- No Prettier, Biome, or `.editorconfig` configuration was found; do not prescribe an unconfigured formatter as repository policy.
- No dedicated accessibility-test dependency or configuration appears in `frontend/package.json` or `vitest.config.ts`.
- Some tests assert source/class strings, including `src/app-shell.test.ts`, `EntryListItem.test.tsx`, and `EntryList.test.tsx`; observable behavior is prevalent but not universal.
- CI does not configure a dedicated accessibility-test gate or a formatter check beyond the build, coverage-test, type-check, and ESLint jobs described above.
