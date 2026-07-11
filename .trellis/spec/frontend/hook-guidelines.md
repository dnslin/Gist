# Hook Guidelines

> Custom-hook and data-fetching patterns used by the frontend.

---

## Placement and Naming

Custom hooks begin with `use`. Reusable browser/UI hooks and server-data hooks coexist in `frontend/src/hooks/`, including `useTheme.ts`, `useUISettings.ts`, `useSwipeGesture.ts`, `useFeeds.ts`, `useEntries.ts`, and `useAppearanceSettings.ts`. Domain-specific interaction hooks may be colocated with components, as in `components/entry-list/useScrollMarkRead.ts` and `components/picture-masonry/useMasonryScrollMarkRead.ts`.

## Stateful Composition

Hooks hide stateful behavior and return data/actions suited to component use:

- `hooks/useAddFeed.ts` owns preview, loading, error handling, mutations, and cache invalidation.
- `hooks/useSelection.ts` turns URL route state into a selection and navigation API.
- `hooks/useAuth.ts` adapts the authentication Zustand store into booleans and actions needed by pages.

Listener lifecycle depends on the target; do not apply one universal ref pattern:

- **Element listeners:** hooks such as `hooks/useSwipeGesture.ts` accept an element ref, stabilize event handlers with `useCallback`, attach listeners to `elementRef.current`, and remove the same handlers in the effect cleanup. `components/entry-list/useScrollMarkRead.ts` follows the same element-scoped cleanup shape.
- **Global/browser listeners:** hooks without an element ref attach to the relevant global object and remove the listener during cleanup. `hooks/useTheme.ts` subscribes to a `MediaQueryList` change event, while `hooks/useScrollToTop.ts` and `hooks/useMobileLayout.ts` clean up `window` listeners.
- **Async streams and batches:** the tracked `.rules` contract requires streaming SSE/WebSocket work to be closed or aborted from the owning `useEffect` cleanup, requires an existing batch to be cancelled before a replacement starts, and forbids module-level variables from carrying request context (`.rules`, resource-management rules). Request-scoped state such as an `AbortController`, signal, reader, entry ID, or batch token **MUST NOT** be owned by a module singleton. A React hook **MUST** keep its active controller in a `useRef`; a non-hook service **MUST** keep it on an explicitly created service/job instance whose lifetime matches the request owner. The owner **MUST** abort before replacement and on disposal/unmount. `hooks/useAITranslation.ts` and `hooks/useAISummary.ts` show ref-owned controllers and replacement aborts.

Current exceptions must not be normalized into the contract. `services/translation-service.ts` declares the module-level mutable `batchAbortController`; although it aborts before replacement and exposes `cancelAllBatchTranslations()`, its module ownership violates the `.rules` prohibition on module-level request context and **MUST** be migrated to ref/instance ownership when touched. `api/index.ts`'s `watchImportStatus()` returns cleanup that only sets a `cancelled` flag; `reader.cancel()` occurs only after a terminal event. The current `useAITranslation.ts` and `useAISummary.ts` effects also abort on entry/replacement transitions but do not return an unmount cleanup. These are cleanup gaps, not patterns to copy. `useSelection.ts` separately memoizes parsed route state with `useMemo` and declares navigation callback dependencies explicitly.

## Server Data

Server-data hooks return TanStack Query results directly. Observed keys include `["feeds"]` in `useFeeds.ts`, `["appearanceSettings"]` in `useAppearanceSettings.ts`, and `["entries", params]` in `useEntries.ts`; infinite queries also live in `useEntries.ts`.

Mutation hooks invalidate affected caches on success in `useFeeds.ts`, `useFolders.ts`, and `useEntries.ts`. The entry-read mutation in `useEntries.ts` is the repository's fuller optimistic-update example: cancel and snapshot queries, update item/list caches, roll back on error, then invalidate after success. `useRefreshStatus.ts` encapsulates polling intended to be mounted once at application level.

## Tests

Hook tests use Vitest and Testing Library `renderHook`, `act`, and `waitFor`. `hooks/useAddFeed.test.tsx` supplies an isolated `QueryClientProvider`; browser-interaction examples include `hooks/useSwipeGesture.test.ts` and `hooks/useScrollToTop.test.ts`.

## Inconsistencies and Cautions

Some settings components call the API and React Query directly instead of going through a domain hook: `components/settings/tabs/FoldersSettings.tsx`, `GeneralSettings.tsx`, and `AppearanceSettings.tsx`. Therefore, “components never call APIs or React Query” is not a current repository rule. Query keys are also inline rather than governed by a central key factory.
