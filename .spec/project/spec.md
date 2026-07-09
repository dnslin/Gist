---
title: Gist
status: active
hue: 45
desc: A self-hosted RSS reader with a Go API, React client, local persistence, and BYOK AI reading tools.
---
# Gist

Gist is a lightweight self-hosted RSS reader. It fetches and stores feeds locally, presents articles
through a responsive web client, and layers optional BYOK AI features such as summary and translation
without making AI a requirement for basic reading.

The project is split into a Go backend and a React/TypeScript frontend. The backend owns persistence,
feed refresh, network access, API contracts, authentication, and AI-provider integration. The frontend
owns the reader experience, settings surfaces, offline-friendly UI state, localization, and sanitized
article rendering.

Specs under this tree describe present product intent. A code change that alters a behavior contract
should update the nearest governing spec in the same change; mechanical edits can be acknowledged when
the contract is unchanged.
