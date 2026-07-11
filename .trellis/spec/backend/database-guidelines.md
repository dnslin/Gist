# Database Guidelines

> Database patterns and conventions for this project.

---

## Overview

The backend uses SQLite through `database/sql` and `modernc.org/sqlite`, not an ORM. `backend/internal/db/db.go` opens the database and calls `Migrate` before returning; `backend/cmd/server/main.go` uses this entrypoint. The DSN enables WAL, foreign keys, a 30-second busy timeout, and synchronous `NORMAL`; `backend/internal/db/db_test.go` checks this connection contract.

Repositories contain handwritten SQL and accept `context.Context`. Handler request contexts flow through services to `ExecContext`, `QueryContext`, and `QueryRowContext`, as shown across `backend/internal/handler/feed_handler.go`, `internal/service/feed_service.go`, and `internal/repository/feed_repository.go`.

---

## Query and Mapping Patterns

- Bind values with `?` parameters. Batch `IN` operations build only the placeholder list dynamically and pass values separately; examples are `FeedRepository.GetByIDs`/`DeleteBatch` and `EntryRepository.UpdateManyReadStatus`.
- List queries defer `rows.Close()` and check `rows.Err()` after scanning in `feed_repository.go`, `folder_repository.go`, and `domain_rate_limit_repository.go`.
- Creates generally assign Snowflake IDs and UTC timestamps in repositories. `backend/internal/repository/sql_helpers.go` centralizes nullable-pointer conversion and RFC3339Nano serialization.
- Multi-statement atomic work uses an explicit transaction, deferred rollback, then commit. `SettingsRepository.SetMany` in `settings_repository.go` uses `BeginTx(ctx, nil)`; migration 17 in `internal/db/migrations.go` uses a transaction for merge/backfill/index work. No generic unit-of-work helper exists.
- Under `modernc.org/sqlite`, delete FTS5 rows with direct SQL such as `DELETE FROM entries_fts WHERE rowid = ?`. Do not use the unsupported special form `INSERT INTO fts(fts, ...) VALUES ('delete', ...)`. Migration 15 in `backend/internal/db/migrations.go` replaces that form by recreating `entries_ad` with `DELETE FROM entries_fts WHERE rowid = old.id`.

Absence behavior is inconsistent and callers must inspect the specific repository contract. `FeedRepository.GetByID` preserves `sql.ErrNoRows`, while `FindByURL`, `SettingsRepository.Get`, and `DomainRateLimitRepository.GetByHost` return `(nil, nil)`. Domain-rate-limit update/delete synthesize `sql.ErrNoRows` when `RowsAffected` is zero. Services commonly translate `sql.ErrNoRows` to `service.ErrNotFound`.

## Mandatory Database Change Contracts

The tracked root `.rules` is authoritative for database work. The following are requirements, not merely prevalent source patterns:

- **Schema synchronization MUST accompany every schema change.** Update `backend/internal/db/migrations.go` (base schema and/or ordered idempotent migration as appropriate) and synchronize `.trellis/spec/backend/database-guidelines.md` in the same change so the documented schema and migration procedure remain current. `backend/internal/db/db.go` calls `Migrate` on every open; `backend/internal/db/migrations.go` demonstrates guarded `pragma_table_info` checks, `IF [NOT] EXISTS`, and transactional data migration, while `backend/internal/db/migrations_test.go` verifies prior-schema transformation and idempotence.
- **Batch mutations MUST use one SQL statement over the complete ID set, never a loop that executes one mutation per ID.** Return without SQL for an empty set, build only bound `?` placeholders dynamically, and execute one `... WHERE id IN (...)`. Positive examples are `FeedRepository.DeleteBatch` in `backend/internal/repository/feed_repository.go` and `EntryRepository.UpdateManyReadStatus` in `backend/internal/repository/entry_repository.go`.
- **Updates MUST preserve an existing valid value when the incoming value is empty, absent, or zero unless clearing the field is an explicit API operation.** Encode that precedence in validation or SQL rather than blindly overwriting. `EntryRepository.CreateOrUpdate` in `backend/internal/repository/entry_repository.go` uses `COALESCE(entries.published_at, excluded.published_at)` so an absent incoming timestamp cannot erase an existing one.
- **Every hierarchy parent change MUST reject direct and indirect cycles before persistence.** Self-parenting, an ancestor becoming a descendant, and a pre-existing loop encountered during traversal are invalid. `folderService.detectCycle` and its use from `Update` in `backend/internal/service/folder_service.go` are the positive implementation: walk ancestors with a visited-ID set and return `ErrInvalid` when a cycle is found.

For these changes, tests MUST assert the observable contract: migration from the previous schema and a second idempotent run; one batch call handling empty and multiple IDs; preservation under empty/zero input plus replacement by a valid input; and direct/indirect folder-cycle rejection without a persisted parent change.


---

## Migrations and Schema Names

`backend/internal/db/migrations.go` embeds the base `CREATE TABLE IF NOT EXISTS` schema and an ordered, idempotent `runMigrations` sequence. Additive migrations inspect `pragma_table_info` before `ALTER TABLE`; indexes and triggers use `IF [NOT] EXISTS` or `DROP … IF EXISTS`. Migration 17 demonstrates a transactional data migration. Prior-schema transformation and idempotence are exercised in `backend/internal/db/migrations_test.go`.

Observed schema naming is snake_case: plural tables such as `folders`, `feeds`, `entries`, `ai_summaries`, and `domain_rate_limits`; singular foreign keys such as `feed_id`; and indexes such as `idx_<table>_<columns>`. IDs are supplied Snowflake integers rather than SQLite autoincrement values (`migrations.go`, `feed_repository.go`, `backend/pkg/snowflake/snowflake.go`).

---

## Verification Examples

Repository tests use real migrated in-memory SQLite databases through `backend/internal/repository/testutil.NewTestDB`, then assert persisted behavior (`feed_repository_test.go`, `entry_repository_test.go`). Migration tests operate directly on SQLite in `backend/internal/db/migrations_test.go`.

---

## Known Gaps and Cautions

- Time serialization is mixed: shared helpers use RFC3339Nano, while older settings/domain-rate-limit code uses RFC3339.
- Error wrapping is prevalent in feed/folder repositories but some settings/domain-rate-limit operations return driver errors directly.
- Missing-row semantics are not uniform. Do not document or assume one universal repository convention, and do not interpolate data values into SQL.

### Existing policy deviation

- `FolderHandler.DeleteBatch` in `backend/internal/handler/folder_handler.go` loops over request IDs and calls `service.Delete` once per folder. This violates the tracked `.rules` single-SQL batch requirement; it is existing source debt, not a convention or an approved template. New batch endpoints MUST NOT copy it.
