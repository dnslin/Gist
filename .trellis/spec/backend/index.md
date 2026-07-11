# Backend Development Guidelines

> Best practices for backend development in this project.

---

## Overview

This directory records the conventions currently visible in the Go backend under `backend/`. The guides distinguish repeated patterns from known inconsistencies; they are not a generic Go style guide.

---

## Guidelines Index

| Guide | Description | Status |
|-------|-------------|--------|
| [Directory Structure](./directory-structure.md) | Module organization and file layout | Filled |
| [Database Guidelines](./database-guidelines.md) | SQLite queries, repositories, and migrations | Filled |
| [Error Handling](./error-handling.md) | Error types, propagation, and HTTP mapping | Filled |
| [Quality Guidelines](./quality-guidelines.md) | Enforced tooling and test patterns | Filled |
| [Logging Guidelines](./logging-guidelines.md) | Structured logging and observed log levels | Filled |

---

## How to Fill These Guidelines

For each guideline file:

1. Document your project's **actual conventions** (not ideals)
2. Include **code examples** from your codebase
3. List **forbidden patterns** and why
4. Add **common mistakes** your team has made

The goal is to help AI assistants and new team members understand how YOUR project works.

---

**Language**: All documentation should be written in **English**.
