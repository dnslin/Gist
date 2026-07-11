# Directory Structure

> How frontend code is organized in this repository.

---

## Application Boundaries

The frontend is an independent Vite React application in `frontend/`. `frontend/src/main.tsx` installs React Query and i18n providers. `frontend/src/App.tsx` handles authentication gating, Wouter routing, and desktop/mobile layout composition. The `@` alias resolves to `frontend/src`, as configured in `frontend/vite.config.ts`.

## Directory Layout

```text
frontend/
├── public/
│   └── locales/{en,zh}/ # Translation resources; each contains common.json
└── src/
    ├── api/                 # Central HTTP and streaming transport
    ├── components/
    │   ├── ui/              # Shared Radix-based primitives
    │   ├── layout/          # Application layouts
    │   ├── auth/            # Authentication screens
    │   └── <domain>/        # Feature UI such as entry-list or settings/tabs
    ├── hooks/               # Reusable stateful and server-data hooks
    ├── i18n/                # i18next setup; imports resources from public/locales
    ├── lib/                 # Pure utilities and browser adapters
    ├── services/            # Service logic; currently translation-service.ts
    ├── stores/              # Zustand client stores
    └── types/               # Shared API and settings models
```

Concrete examples include `frontend/public/locales/en/common.json`, `frontend/public/locales/zh/common.json`, `frontend/src/i18n/index.ts`, `frontend/src/api/index.ts`, `frontend/src/lib/router.ts`, `frontend/src/lib/parse-html.ts`, `frontend/src/hooks/useEntries.ts`, `frontend/src/stores/auth-store.ts`, and `frontend/src/types/api.ts`.

## Module Organization

- Shared UI primitives live in `frontend/src/components/ui/`, for example `dialog.tsx`, `switch.tsx`, and `dropdown-menu.tsx`.
- Business UI is grouped by domain: `components/entry-list/`, `components/picture-masonry/`, and `components/settings/tabs/`.
- Layout and authentication components have dedicated `components/layout/` and `components/auth/` directories.
- Some domains expose a barrel entry point: `components/sidebar/index.tsx`, `components/entry-list/index.tsx`, and `components/picture-masonry/index.ts`. `frontend/src/App.tsx` consumes these barrels.
- Component-specific interaction hooks may remain beside their domain, as in `components/entry-list/useScrollMarkRead.ts`; broadly reused hooks live in `src/hooks/`.
- Tests are colocated with the implementation, for example `hooks/useAddFeed.test.tsx`, `stores/lightbox-store.test.ts`, and `components/entry-list/EntryList.test.tsx`. There is no separate frontend test tree.

## Naming Conventions

Prevalent naming is contextual rather than universal:

- Product components commonly use PascalCase files and matching named exports: `FeedUrlForm.tsx`, `EntryList.tsx`, and `SettingsModal.tsx`.
- Hooks use camelCase `use*.ts`: `useEntries.ts`, `useTheme.ts`, and `useSwipeGesture.ts`.
- Zustand stores use kebab-case `*-store.ts`: `auth-store.ts` and `translation-store.ts`.
- UI primitives, layout files, and many library modules use kebab-case: `dropdown-menu.tsx`, `three-column-layout.tsx`, and `parse-html.ts`.

`frontend/src/lib/queryClient.ts` is a camelCase exception among library files. Do not introduce a blanket claim that every component-related file must use PascalCase; the repository does not follow that rule.

## Known Gaps

There is no dedicated frontend architecture document defining stricter component or module boundaries. `README.md` documents product capabilities, and `docs/adr/0001-use-wails-3-for-production-desktop.md` documents the Wails desktop decision and regression gates, but neither establishes a universal feature-folder scheme.
