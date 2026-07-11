# Component Guidelines

> Component patterns observed in the current frontend.

---

## Component and Props Shape

Business components prevalently use named exports, as shown by `frontend/src/components/add-feed/FeedUrlForm.tsx`, `frontend/src/components/entry-list/EntryList.tsx`, and `frontend/src/components/settings/SettingsModal.tsx`. Named export usage is an observed repository pattern, not the source of the Props typing requirement.

The tracked `.rules` contract independently requires **all component Props and API response shapes to be defined with interfaces**. Every new or modified component **MUST** declare a named `interface <Component>Props` rather than an inline object annotation or a Props type alias; local component interfaces stay beside the component, while shared contracts belong under `frontend/src/types/`. Props **MUST** use precise domain types and callback signatures; optional props use `?`, and defaults may be assigned during destructuring. API response interfaces follow the boundary placement rules in `type-safety.md`. Existing violations are deviations, not alternate conventions.

## Composition

`frontend/src/App.tsx` is the root composition boundary: it obtains authentication, selection, and layout state, then passes data and events into `Sidebar`, `EntryList`, `PictureMasonry`, and `EntryContent`. Similar container-style composition appears in `components/add-feed/AddFeedPage.tsx` and `components/entry-content/EntryContent.tsx`. `App.tsx` also lazy-loads entry content with `lazy` and `Suspense`.

Wrappers around third-party primitives preserve the primitive prop and ref types and forward refs. See `components/ui/dialog.tsx`, `components/ui/dropdown-menu.tsx`, and `components/ui/switch.tsx`. `components/entry-list/EntryListItem.tsx` also uses `forwardRef` for virtual-list measurement.

## Styling

Components use Tailwind utility classes. Conditional classes are merged with `cn()`, for example in `FeedUrlForm.tsx`, `EntryListItem.tsx`, and `components/ui/dialog.tsx`. Global design tokens, dark-theme behavior, safe-area rules, and shared animations live in `frontend/src/index.css`.

Behaviorally complex shared controls use Radix primitives rather than reimplementing dialogs, menus, and switches; the concrete wrappers are under `components/ui/`.

## Safe HTML and Internationalization Contracts

These are documented `.rules` requirements, not merely prevalent patterns:

- Treat externally supplied article HTML as untrusted. Parse it through the `unified` + `rehype` sanitizing pipeline and render the resulting HAST with `hast-util-to-jsx-runtime`; do not render unsanitized content with `dangerouslySetInnerHTML`. `frontend/src/lib/parse-html.ts` implements the pipeline with `rehype-parse`, removal of non-content elements, `rehype-sanitize`, and `toJsxRuntime`; `frontend/src/components/ui/article-content.tsx` consumes `parseHtml()` rather than injecting HTML.
- Every new translation key must be added to both `frontend/public/locales/en/common.json` and `frontend/public/locales/zh/common.json`. Components then read the shared key through `useTranslation().t()`.

```tsx
// Required for untrusted HTML
const rendered = parseHtml(content, { components });
return rendered.toContent();

// Forbidden unless the source has already passed the required pipeline
return <div dangerouslySetInnerHTML={{ __html: content }} />;
```

## Text and Accessibility

User-visible text is usually translated with `useTranslation().t()`, including `components/auth/LoginPage.tsx`, `components/add-feed/AddFeedPage.tsx`, and `components/entry-content/EntryContentHeader.tsx`. Several icon-only controls use translated `aria-label` values, including controls in `FeedUrlForm.tsx`, `SettingsModal.tsx`, and `EntryContentHeader.tsx`. Decorative feed images use `alt=""` in `EntryListItem.tsx`.

## Inconsistencies and Cautions

- Accessible naming is not consistent: `components/entry-list/EntryListHeader.tsx` uses `title` for icon-only buttons, while other components use `aria-label`.
- The text input in `FeedUrlForm.tsx` has a placeholder but no associated label.
- No shared Button or Input primitive and no reusable form-validation layer were found. Do not document those abstractions as established conventions.
- Named exports are prevalent for business components but are not mandatory for every UI primitive or root file. This export-style observation does not weaken the mandatory `.rules` requirement that all Props and API response shapes use interfaces; current type aliases and inline Props objects are deviations listed in `type-safety.md`.
