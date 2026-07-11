# E-C01-ROLLBACK

Verified on 2026-07-11 using a detached temporary worktree at pre-Child commit `3b19c1d`.

- The previous configuration test reproduced the recorded Windows fixture failure exactly (`artifact://61`).
- The previous icon validator test reproduced the recorded cross-platform security failure exactly (`artifact://63`).
- The previous router package passed (`artifact://62`), confirming the rollback does not require route/API restoration work.
- The temporary worktree was removed after verification.
- Reapplying the current working tree restores both focused tests and the unified version check; current focused/full results are recorded in the other `E-C01-*` evidence files.
- No database migration, schema, user data, credential, registry, shortcut, installer directory, or release asset was introduced, so rollback has no data-plane procedure.
- The version mechanism is a clean cutover: root `VERSION`, validator, Go/Vite/Swagger/Docker consumers, CI wiring, and tests must be reverted or applied together. Partial consumer rollback is unsupported and intentionally fails consistency checks.
