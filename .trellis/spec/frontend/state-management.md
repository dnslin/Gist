# State Management

> State boundaries currently used by the frontend.

---

## State Categories

- **Local UI and form state:** component or hook `useState`, as in `components/add-feed/FeedUrlForm.tsx`, `components/settings/tabs/GeneralSettings.tsx`, and `hooks/useAddFeed.ts`.
- **Shareable selection state:** URL state handled by Wouter and `hooks/useSelection.ts`, with parsing/building in `lib/router.ts`.
- **Persistent UI preferences:** localStorage-backed module external stores in `hooks/useTheme.ts` and `hooks/useUISettings.ts`.
- **Cross-tree client state:** Zustand stores under `src/stores/`.
- **Server state:** TanStack React Query, installed at the root by `frontend/src/main.tsx`.

## Zustand Stores

Stores such as `stores/lightbox-store.ts`, `image-preview-store.ts`, and `translation-store.ts` define an explicit State interface, an `initialState`, and action methods. Consumers generally subscribe with selectors to the smallest field or action they need; examples include `components/picture-masonry/PictureItem.tsx`, `components/entry-list/EntryListItem.tsx`, and `hooks/useAITranslation.ts`.

Authentication is also a Zustand store, but `stores/auth-store.ts` includes asynchronous API side effects and exports `authActions` for callers outside React. `hooks/useAuth.ts` provides the component-facing adapter.

## Server State

`frontend/src/lib/queryClient.ts` configures queries with `staleTime: 30_000` and `retry: 1`. Reads are primarily exposed by hooks such as `useFeeds.ts`, `useFolders.ts`, and `useEntries.ts`; `useRefreshStatus.ts` implements polling.

Writes invalidate the resource and affected derived data. Feed/folder mutations update feeds, folders, and unread-count caches in `useFeeds.ts` and `useFolders.ts`. Adding a feed invalidates folders, feeds, entries, and unread counts in `useAddFeed.ts`. The entry-read flow in `useEntries.ts` additionally performs optimistic cache changes and rollback. `main.tsx` invalidates queries when the page is restored from the back-forward cache.

## Inconsistencies and Cautions

- Query keys are inline arrays across `useEntries.ts`, `useFeeds.ts`, and `components/settings/tabs/DataControl.tsx`; no centralized key factory exists.
- Settings screens including `AISettings.tsx`, `FeedsSettings.tsx`, and `DataControl.tsx` sometimes combine direct API calls with local loading/error state.
- Consequently, not every server write is represented by a reusable `useMutation` hook. Document that as an inconsistency, not as a forbidden pattern already enforced by the codebase.
