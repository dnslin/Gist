---
title: Backend
status: active
hue: 205
desc: Go services, repositories, handlers, scheduler, and shared packages for local RSS and AI-backed reading workflows.
code:
  - backend/cmd
  - backend/internal
  - backend/pkg
---
# Backend

The backend exposes Gist's HTTP API and owns durable server-side behavior. It stores feeds, entries,
folders, settings, authentication state, import/export data, refresh progress, rate limits, and AI task
state in the local SQLite database.

Feed and network behavior must favor safe, repeatable background work: refreshes use the shared network
client conventions, preserve security boundaries around URLs and secrets, and avoid blocking unrelated
reading workflows. API handlers validate all boundary input and keep the frontend contract in sync when
DTOs change.

AI summary and translation are optional BYOK capabilities layered over the reader. Provider integrations
must handle streaming and cached responses deliberately, respect rate limits, and keep failures localized
so a broken AI provider does not break feed reading.

Tests should exercise public behavior through package boundaries where practical, with generated mocks
kept near the interfaces they represent.
