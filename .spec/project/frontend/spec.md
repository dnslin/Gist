---
title: Frontend
status: active
hue: 150
desc: React reader UI, settings, local UI state, sanitized article rendering, and API-facing TypeScript contracts.
code:
  - frontend/src
---
# Frontend

The frontend is the primary reading surface for Gist. It renders the feed/sidebar/navigation shell,
article lists, entry content, picture masonry, settings, authentication flows, and AI summary or
translation controls against the backend API.

Article content must be rendered through the sanitized HTML pipeline rather than unsafe raw HTML. UI state
that affects repeated reading workflows should be predictable across themes, mobile layouts, selection,
scrolling, and transient network failures.

TypeScript API contracts live with the client and must represent backend IDs, timestamps, settings, and
streaming behavior accurately. New user-facing copy is localized in both supported languages.

Frontend tests should stay near the source files they cover and focus on behavior visible through the
component or hook API.
