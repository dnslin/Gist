# Type Safety

> Type organization and runtime-safety boundaries in the frontend.

---

## Compiler Configuration

`frontend/tsconfig.app.json` applies to production code under `src` and enables `strict`, `noUnusedLocals`, `noUnusedParameters`, `noUncheckedIndexedAccess`, `verbatimModuleSyntax`, and `erasableSyntaxOnly`. JSX uses `react-jsx`.

## Type Organization

Shared API/resource models live in `frontend/src/types/api.ts`, including `ContentType`, `Entry`, `Feed`, `Folder`, request parameters, and paginated responses. Settings contracts live in `src/types/settings.ts`, including provider/proxy unions and settings request/response types.

Transport-specific types and guards remain beside transport code in `src/api/index.ts`, such as `AuthUser`, streaming event unions, and `isTranslate*` guards. Component- and hook-private contracts stay with their implementation; examples include `EntryListProps`, `FeedUrlFormProps`, and `UseAddFeedReturn` in their respective files.

## Established Type Patterns

String-literal and discriminated unions model finite states: `ContentType` and `ImportTask.status` in `types/api.ts`, `SelectionType` in `hooks/useSelection.ts`, and `TranslateEvent` in `api/index.ts`. Generics express transport results and query-cache values, including `request<T>` in `api/index.ts` and `getQueryData<Entry>`/`setQueriesData` usage in `useEntries.ts`.

`api/index.ts` centralizes token/header handling, response parsing, non-2xx conversion to `ApiError`, and the 401 callback. Endpoint functions return explicit `Promise<Model>` values. Hooks and stores import this API layer, as shown by `hooks/useFeeds.ts`, `hooks/useAddFeed.ts`, and `stores/auth-store.ts`. Streaming AI responses use typed `AsyncGenerator` values and local type guards in the same API module.

## Required Type and API Boundary Contracts

The tracked `.rules` policy requires the following:

- Define component Props and API response shapes with interfaces; keep local component interfaces beside the component and shared backend DTO/resource interfaces under `frontend/src/types/`.
- Do not use `any`. Prefer concrete domain types, `unknown` plus narrowing, generics, or narrow non-`any` assertions when a library boundary requires one.
- Represent backend Snowflake IDs as `string`; JavaScript numbers cannot safely represent the full identifier range.
- Treat backend timestamps as UTC RFC3339 strings at the transport boundary.
- When a backend DTO changes, update the corresponding frontend contract in `frontend/src/types/` in the same change.

`frontend/src/types/api.ts` and `frontend/src/types/settings.ts` are the primary shared boundary definitions; transport-specific interfaces may remain in `frontend/src/api/index.ts`.

### Pagination Boundary

Paginated list endpoints **MUST** request `limit + 1` records from the data layer, set `hasMore` from whether the actual result count is greater than `limit`, and crop the returned items to at most `limit` before serializing the response. The frontend response interface **MUST** retain the resulting boolean rather than infer it from a full page. This is required by `.rules` line 76 and implemented concretely by `backend/internal/handler/entry_handler.go` (`queryParams.Limit = params.Limit + 1`, `len(entries) > params.Limit`, then `entries[:params.Limit]`); `frontend/src/types/api.ts` exposes the matching `EntryListResponse { entries: Entry[]; hasMore: boolean }`, and `frontend/src/hooks/useEntries.ts` uses `lastPage.hasMore` to decide whether another page exists.

### AI Cached JSON / Uncached SSE Branch

AI streaming clients **MUST** branch on the response `Content-Type`: a cache hit is `application/json` and must be parsed as the endpoint's response interface; a cache miss is the streaming branch and must be consumed as SSE/stream data rather than JSON. This is the API contract in `.rules` line 99. `frontend/src/api/index.ts` provides both concrete examples: `streamSummary()` parses `SummarizeResponse` when `Content-Type` includes `application/json` and otherwise reads the response body incrementally; `streamTranslateBlocks()` parses `TranslateResponse` for JSON and otherwise delegates to `readSSEEvents<TranslateEvent>()`. New AI endpoints **MUST** preserve this two-branch contract and define interfaces for cached JSON response shapes and typed events for the uncached stream.

```ts
const contentType = response.headers.get("Content-Type") ?? "";
if (contentType.includes("application/json")) {
  const cached = (await response.json()) as SummarizeResponse;
  return cached;
}
return readSSEEvents<SummaryEvent>(response);
```

## Runtime Validation Reality

No Zod, Yup, Valibot, io-ts, or Ajv dependency is present in `frontend/package.json`. Network JSON is parsed as `unknown` and returned as `as T` by `api/index.ts`; `components/settings/tabs/AISettings.tsx` parses JSON and asserts `RequestOptions`.

Runtime checks are local and targeted rather than schema-wide: `isErrorResponse` and translation-event guards in `api/index.ts`, protocol checking in `lib/url.ts`, and rehype-schema sanitization in `lib/parse-html.ts`. Do not claim that all network payloads receive runtime schema validation.

## Assertions and Observed Deviations

Production uses controlled non-`any` assertions at boundaries, including API response `as T` casts in `api/index.ts`, select/JSON casts in `AISettings.tsx`, and a `CSSProperties` cast in `components/layout/three-column-layout.tsx`. These assertions do not remove the requirement to model Props and API responses explicitly, and they do not establish runtime schema validation.

Tests contain `as any` suppressions and DOM-mock casts, for example in `EntryList.test.tsx` and `PictureMasonry.test.tsx`. These are observed deviations from the `.rules` no-`any` policy, not permission to introduce new `any` usage. The separate runtime-validation gap remains: generic `as T` transport casts trust payload shape unless a local guard validates it.

The tracked interface rule also has current source violations that must remain classified as deviations:

- `frontend/src/components/ui/article-content.tsx` defines `ArticleContentProps` as a `type` union instead of an interface-based Props contract.
- Inline component Props object types appear in `frontend/src/App.tsx` (`EntryContentPlaceholder`), `components/i18n-provider.tsx` (`I18nProvider`), `components/entry-content/EntryContent.tsx` (`EntryContentEmpty`), `components/picture-masonry/PictureMasonry.tsx` (`MasonryItemContent`), `components/sidebar/Sidebar.tsx` (`SidebarScrollArea`), and `components/sidebar/SidebarHeader.tsx` (`GistLogo`).
- `frontend/src/api/index.ts` uses inline API response shapes in `fetchReadableContent()` (`request<{ readableContent: string }>`), `cancelImportOPML()` (`request<{ cancelled: boolean }>`), and similar generic calls rather than named response interfaces.

These examples violate `.rules` line 62; they are migration targets, not permission to use type aliases or inline object shapes for new Props or API responses.
