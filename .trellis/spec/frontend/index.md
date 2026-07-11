# Frontend Development Guidelines

> Source-backed conventions for the React frontend in `frontend/`.

---

## Overview

These guides describe the repository as it exists today. They distinguish prevalent patterns from local exceptions rather than imposing generic React or TypeScript rules.

## Guidelines Index

| Guide | Description | Status |
|-------|-------------|--------|
| [Directory Structure](./directory-structure.md) | Module organization and file layout | Filled |
| [Component Guidelines](./component-guidelines.md) | Component patterns, props, composition | Filled |
| [Hook Guidelines](./hook-guidelines.md) | Custom hooks and data fetching patterns | Filled |
| [State Management](./state-management.md) | Local, URL, client, and server state | Filled |
| [Quality Guidelines](./quality-guidelines.md) | Executable quality tooling and observed practices | Filled |
| [Type Safety](./type-safety.md) | Type organization and runtime-validation boundaries | Filled |

## Scope

The application is a Vite React frontend under `frontend/`. `frontend/src/main.tsx` installs application providers; `frontend/src/App.tsx` owns authentication gating, routing, and responsive composition. The same frontend supports the PWA described in `README.md` and the Wails desktop product described in `docs/adr/0001-use-wails-3-for-production-desktop.md`.

**Language:** English.
