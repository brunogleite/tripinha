# US-CE-01: Trigger Candidates Schema

## Elevator Pitch

- **Before**: The correlation engine has no place to persist its results — computed correlations exist only in memory and are lost.
- **After**: A `trigger_candidates` table exists with the correct schema, constraints, and indexes; `GET /triggers` can be implemented against it.
- **Decision enabled**: Developer (and later the IBS user) can trust that engine results survive restarts and are queryable without re-running the engine.

## Problem

The correlation engine needs a durable store for its output. Without a persistent `trigger_candidates` table with the correct schema, no downstream feature (GET /triggers, notifications, export) can be built. This story establishes the data contract that all other correlation-engine stories depend on.

## Who

- **User type**: IBS user (Sofia Andrade) — indirect; direct actor is the engine service
- **Context**: Brownfield Go + PostgreSQL API; migrations use sequential numbered SQL files
- **Motivation**: Engine results must survive service restarts and be queryable by user_id without re-running computation

## Solution

Create migration `005_trigger_candidates.sql` that defines the `trigger_candidates` table with primary key, unique constraint on `(user_id, ingredient_name, symptom_type)`, and query-supporting indexes.

## Domain Examples

### 1: Happy Path — Schema in place, engine can upsert

Sofia's engine run completes. It attempts to upsert `(user_id=sofia, ingredient_name=fructose, symptom_type=bloating, correlation_count=6, window_hours=24, last_evaluated_at=now())`. The table exists, the UPSERT succeeds, and the row is visible to `GET /triggers` immediately.

### 2: Edge Case — Re-run with updated count

Sofia's engine runs again the next day. Fructose now has a count of 8. The UPSERT updates `correlation_count=8` and `last_evaluated_at` on the existing row — no duplicate row is created.

### 3: Error / Boundary — Old migration file

A developer accidentally runs migration 004 twice. The `IF NOT EXISTS` clauses prevent failure. The `trigger_candidates` migration is idempotent.

## UAT Scenarios (BDD)

### Scenario: Trigger candidates table is created by migration

```gherkin
Given the database has migrations 001 through 004 applied
When migration 005_trigger_candidates.sql is applied
Then the trigger_candidates table exists
And it has columns: id, user_id, ingredient_name, symptom_type, correlation_count, window_hours, last_evaluated_at
And a unique constraint exists on (user_id, ingredient_name, symptom_type)
```

### Scenario: Engine can upsert a new candidate row

```gherkin
Given the trigger_candidates table exists
And no rows exist for user_id=sofia, ingredient_name=fructose, symptom_type=bloating
When the engine upserts (sofia, fructose, bloating, count=6, window_hours=24)
Then a new row is created with correlation_count=6
And last_evaluated_at is set to the current timestamp
```

### Scenario: Re-run updates existing candidate without duplication

```gherkin
Given trigger_candidates has a row (sofia, fructose, bloating, count=6)
When the engine upserts the same key with count=8
Then only one row exists for (sofia, fructose, bloating)
And correlation_count is updated to 8
And last_evaluated_at is refreshed
```

### Scenario: Migration is idempotent

```gherkin
Given migration 005 has already been applied
When migration 005 is applied a second time
Then no error is raised
And the table structure is unchanged
```

## Acceptance Criteria

- [ ] Migration file `005_trigger_candidates.sql` exists in `migrations/`
- [ ] Table includes: `id SERIAL PRIMARY KEY`, `user_id TEXT NOT NULL`, `ingredient_name TEXT NOT NULL`, `symptom_type TEXT NOT NULL`, `correlation_count INTEGER NOT NULL DEFAULT 0`, `window_hours INTEGER NOT NULL`, `last_evaluated_at TIMESTAMPTZ NOT NULL`
- [ ] Unique constraint on `(user_id, ingredient_name, symptom_type)`
- [ ] Index on `(user_id)` for efficient GET /triggers queries
- [ ] UPSERT (ON CONFLICT ... DO UPDATE) updates correlation_count and last_evaluated_at
- [ ] Migration uses `CREATE TABLE IF NOT EXISTS` (idempotent)

## Outcome KPIs

- **Who**: Correlation engine service
- **Does what**: Persists correlation results durably after each run
- **By how much**: 100% of engine runs that produce candidates result in persisted rows (zero data loss on restart)
- **Measured by**: Integration test: upsert N rows, restart service, verify N rows present
- **Baseline**: No persistent store exists today (0%)

## Technical Notes

- Migration numbering: `005_trigger_candidates.sql` (sequential after 004_ingredient_tables.sql)
- UPSERT conflict key: `(user_id, ingredient_name, symptom_type)` — compound unique constraint
- `window_hours` is stored per-row because the engine may be reconfigured; historical rows should reflect the window at time of computation
- No foreign key to `users` table — user_id is a string (Auth0 subject), consistent with existing tables
- Dependencies: #4 (symptom_events — done), #5 (ingredient tables — done)
