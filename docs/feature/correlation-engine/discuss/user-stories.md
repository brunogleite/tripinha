<!-- markdownlint-disable MD024 -->
# User Stories: Correlation Engine

## System Constraints

- All routes require a valid Auth0 JWT (existing `authMiddleware` in `cmd/main.go`)
- Health data routes additionally require consent (existing `consent.RequireConsent` middleware)
- `user_id` is always derived from the JWT — never accepted from the request body
- Configuration via environment variables only; no config files in this slice
- Follow existing Go interface pattern: `Service` struct depends on `Store` interface (enables test doubles)
- Database: PostgreSQL via pgx/v5 pool — consistent with existing code
- No technology choices made here; all implementation decisions belong to DESIGN wave

---

## US-CE-01: Trigger Candidates Schema

### Elevator Pitch

- **Before**: The correlation engine has no place to persist its results — computed correlations are lost on restart.
- **After**: A `trigger_candidates` table exists with correct schema and UPSERT support; `GET /triggers` can be built against it.
- **Decision enabled**: The team can build and test the correlation engine against a real, stable data contract.

### Problem

The correlation engine needs a durable store for its output. Without a persistent `trigger_candidates` table, no downstream feature (GET /triggers, notifications, export) can be built. This story establishes the data contract all other correlation-engine stories depend on.

### Who

- IBS user (Sofia Andrade) — indirect; direct actor is the engine service
- Brownfield Go + PostgreSQL API; migrations use sequential numbered SQL files
- Engine results must survive service restarts and be queryable per user_id

### Solution

Create migration `005_trigger_candidates.sql` defining the `trigger_candidates` table with primary key, unique constraint on `(user_id, ingredient_name, symptom_type)`, and query-supporting index.

### Domain Examples

#### 1: Happy Path — Schema in place, engine can upsert

Sofia's engine run completes. It upserts `(user_id=sofia, ingredient_name=fructose, symptom_type=bloating, correlation_count=6, window_hours=24, last_evaluated_at=now())`. The UPSERT succeeds and the row is immediately visible to GET /triggers.

#### 2: Edge Case — Re-run updates existing count

Sofia's engine runs again. Fructose now has count=8. The UPSERT updates `correlation_count=8` and refreshes `last_evaluated_at` — no duplicate row created.

#### 3: Error / Boundary — Idempotent migration

Migration 005 is applied twice (operator error). `CREATE TABLE IF NOT EXISTS` prevents failure. Table structure is unchanged.

### UAT Scenarios (BDD)

```gherkin
Scenario: Trigger candidates table is created by migration
  Given the database has migrations 001 through 004 applied
  When migration 005_trigger_candidates.sql is applied
  Then the trigger_candidates table exists with columns: id, user_id, ingredient_name,
       symptom_type, correlation_count, window_hours, last_evaluated_at
  And a unique constraint exists on (user_id, ingredient_name, symptom_type)

Scenario: Engine can persist a new candidate row
  Given the trigger_candidates table exists and is empty for Sofia
  When the engine upserts (sofia, fructose, bloating, count=6, window_hours=24)
  Then a new row is created with correlation_count=6
  And last_evaluated_at is set to approximately now

Scenario: Re-run updates existing candidate without duplication
  Given trigger_candidates has (sofia, fructose, bloating, count=6)
  When the engine upserts the same key with count=8
  Then only one row exists for (sofia, fructose, bloating)
  And correlation_count is 8 and last_evaluated_at is refreshed

Scenario: Migration is idempotent
  Given migration 005 has already been applied
  When migration 005 is applied a second time
  Then no error is raised and the table structure is unchanged
```

### Acceptance Criteria

- [ ] Migration `005_trigger_candidates.sql` exists in `migrations/`
- [ ] Columns: `id SERIAL PRIMARY KEY`, `user_id TEXT NOT NULL`, `ingredient_name TEXT NOT NULL`, `symptom_type TEXT NOT NULL`, `correlation_count INTEGER NOT NULL DEFAULT 0`, `window_hours INTEGER NOT NULL`, `last_evaluated_at TIMESTAMPTZ NOT NULL`
- [ ] Unique constraint on `(user_id, ingredient_name, symptom_type)`
- [ ] Index on `(user_id)` for GET /triggers query performance
- [ ] UPSERT updates `correlation_count` and `last_evaluated_at` on conflict
- [ ] `CREATE TABLE IF NOT EXISTS` (idempotent)

### Outcome KPIs

- **Who**: Correlation engine service
- **Does what**: Persists correlation results after each run
- **By how much**: 100% of engine runs that produce candidates result in persisted rows
- **Measured by**: Integration test — upsert N rows, restart, verify N rows present
- **Baseline**: No persistent store exists (0%)

### Technical Notes

- Migration sequence: `005_trigger_candidates.sql` (after `004_ingredient_tables.sql`)
- UPSERT conflict key: `(user_id, ingredient_name, symptom_type)`
- `window_hours` stored per-row — reflects the window configured at time of computation
- No FK to a users table — `user_id` is an Auth0 subject string (consistent with existing tables)
- Dependencies: #4 (symptom_events, done), #5 (ingredient tables, done)

---

## US-CE-02: Core Correlation Engine

### Elevator Pitch

- **Before**: Sofia's meal and symptom data sit in separate tables with no mechanism connecting them.
- **After**: `POST /admin/correlations/run` produces rows in `trigger_candidates` for ingredients that correlate with Sofia's symptoms above the configured threshold.
- **Decision enabled**: Sofia can query `GET /triggers` to see which ingredients are statistically associated with her flare-ups and adjust her diet with evidence.

### Problem

Sofia Andrade has logged meals and symptoms for weeks but cannot identify which ingredient causes her flare-ups — there are too many variables. The app needs to compute this pattern for her automatically.

### Who

- IBS user (Sofia Andrade) — indirect; engine runs on her behalf
- Brownfield Go API; engine runs as a service method called by scheduler and on-demand HTTP
- Sofia needs specific ingredient names (not just product names) to make targeted dietary changes

### Solution

Implement `internal/correlation` package with a `Service` that queries `meal_ingredients` in the lookback window for each symptom event, accumulates counts per `(ingredient_name, symptom_type)`, and upserts to `trigger_candidates` when count >= threshold.

### Domain Examples

#### 1: Happy Path — Fructose identified as bloating trigger

Sofia has 12 bloating events. In 7, she ate fructose within 24 hours. CORRELATION_THRESHOLD=5. Engine runs: fructose count=7 >= 5 → upserted as candidate.

#### 2: Edge Case — Just below threshold

Sofia ate gluten in 4 of 12 bloating windows. 4 < 5 → gluten not written. Next run may promote it if count rises.

#### 3: Error Path — No meal data in any window

Sofia logged 5 symptom events but no meals that period. Engine finds 0 correlations. Completes without error — empty result is valid.

#### 4: Edge Case — Multiple symptom types independent

Fructose correlates with 7 bloating windows, 2 cramping windows. THRESHOLD=5. → candidate for bloating; not for cramping.

### UAT Scenarios (BDD)

```gherkin
Scenario: Ingredient exceeding threshold becomes a trigger candidate
  Given Sofia has 7 bloating symptom events
  And in each of those 7 windows, she ate fructose
  And CORRELATION_THRESHOLD is 5
  When the correlation engine runs for Sofia
  Then trigger_candidates contains (sofia, fructose, bloating, count=7)

Scenario: Ingredient below threshold is excluded
  Given Sofia has 7 bloating symptom events
  And gluten appears in only 3 of those windows
  And CORRELATION_THRESHOLD is 5
  When the correlation engine runs for Sofia
  Then trigger_candidates does not contain a row for (sofia, gluten, bloating)

Scenario: Window boundary is respected
  Given Sofia has a bloating event at 2026-04-28T14:45:00Z
  And LOOKBACK_WINDOW_HOURS is 24
  And she ate fructose at 2026-04-27T14:00:00Z (within window)
  And she ate lactose at 2026-04-26T20:00:00Z (42h before — outside window)
  When the engine runs the window query for this symptom event
  Then fructose is counted and lactose is not counted

Scenario: Symptom types are correlated independently
  Given Sofia has bloating (7 windows with fructose) and cramping (2 windows with fructose)
  And CORRELATION_THRESHOLD is 5
  When the engine runs
  Then fructose is a candidate for bloating but not for cramping

Scenario: Empty meal history produces no candidates without error
  Given Sofia has 5 bloating events and no meal_events in any lookback window
  When the engine runs for Sofia
  Then no rows are written to trigger_candidates
  And the engine completes without error

Scenario: Re-running updates existing counts
  Given trigger_candidates has (sofia, fructose, bloating, count=7)
  And a new run finds fructose in 9 bloating windows
  When the engine runs again
  Then correlation_count is updated to 9 and last_evaluated_at is refreshed
  And no duplicate row is created
```

### Acceptance Criteria

- [ ] `internal/correlation` package with `Service` struct
- [ ] Engine queries `meal_ingredients` JOIN `meal_events` within the lookback window, scoped by `user_id`
- [ ] Accumulates counts per `(ingredient_name, symptom_type)` across all symptom windows
- [ ] Only ingredients with count >= `CORRELATION_THRESHOLD` are written to `trigger_candidates`
- [ ] Each symptom type processed independently
- [ ] Engine completes without error when no meal events exist in any window
- [ ] UPSERT prevents duplicates; re-runs update existing rows
- [ ] `window_hours` stored matches env configuration at time of run
- [ ] On-demand endpoint `POST /admin/correlations/run` accepts `{"user_id": "..."}` and returns HTTP 202
- [ ] `CORRELATION_THRESHOLD` and `LOOKBACK_WINDOW_HOURS` read from env with defaults (5 and 24)
- [ ] Engine is unit-testable with a mock store (interface pattern from `internal/meals`)

### Outcome KPIs

- **Who**: IBS users with ≥5 logged symptom events and meal data in windows
- **Does what**: Receive ≥1 ingredient-symptom correlation candidate after engine runs
- **By how much**: >0 candidates for 80% of eligible users after first full run
- **Measured by**: Integration test with controlled seed data; production SQL count
- **Baseline**: 0 candidates produced today

### Technical Notes

- Store interface + Service pattern follows `internal/meals` — enables test doubles
- Correlation count is total across ALL symptom windows, not per-window
- `LOOKBACK_WINDOW_HOURS` read once at engine startup, not per-symptom-event
- Performance: window query joins `meal_events` + `meal_ingredients`; index on `meal_events(user_id, scanned_at)` exists (migration 002); verify with EXPLAIN ANALYZE on realistic data
- `symptom_type` not validated by engine — reflects raw `symptom_events.type` value
- Advisory lock on user_id prevents concurrent runs (see US-CE-04)
- Dependencies: US-CE-01 (table schema)

---

## US-CE-03: GET /triggers Endpoint

### Elevator Pitch

- **Before**: Correlation results exist in the database but Sofia has no way to retrieve them.
- **After**: `GET /triggers` returns Sofia's trigger candidates as JSON, scoped to her Auth0 user_id.
- **Decision enabled**: Sofia can see which specific ingredients are statistically associated with each symptom type and decide which to eliminate or reduce in her diet.

### Problem

Sofia Andrade manages IBS through trial and error. Even with correlation results persisted, she has no endpoint to access them. GET /triggers bridges background computation with Sofia's daily dietary decisions.

### Who

- IBS user (Sofia Andrade) — authenticated via Auth0 JWT
- Brownfield Go + chi router; follows `POST /meals` and `POST /symptoms` handler pattern
- Sofia needs a reliable, user-scoped endpoint accessible from any client

### Solution

Implement `GET /triggers` handler in the correlation package. Reads `trigger_candidates` scoped to the authenticated `user_id`. Returns JSON. Registered under auth + consent middleware.

### Domain Examples

#### 1: Happy Path — Sofia has two candidates

Sofia calls `GET /triggers`. Two rows in `trigger_candidates`: fructose/bloating (count=7), onion/cramping (count=6). Response: `{"triggers": [{"ingredient": "fructose", "symptom_type": "bloating", "correlation_count": 7, "window_hours": 24, "last_evaluated_at": "2026-04-28T10:00:00Z"}, ...]}`.

#### 2: Edge Case — No candidates yet

Engine has not run. Response: `{"triggers": []}` with HTTP 200. Not 404.

#### 3: Error Path — DB unavailable

PostgreSQL unreachable. Response: HTTP 503 with `Retry-After: 30`. No stack trace.

#### 4: Security — User-scoped

Marco's trigger rows exist. Sofia's GET /triggers returns only Sofia's rows.

### UAT Scenarios (BDD)

```gherkin
Scenario: Authenticated user retrieves their trigger candidates
  Given Sofia is authenticated (valid Auth0 JWT)
  And trigger_candidates has (sofia, fructose, bloating, count=7) and (sofia, onion, cramping, count=6)
  When Sofia calls GET /triggers
  Then the response is HTTP 200
  And the body contains both entries with ingredient, symptom_type, correlation_count, window_hours, last_evaluated_at

Scenario: No candidates yet returns empty list
  Given Sofia is authenticated
  And trigger_candidates has no rows for Sofia
  When Sofia calls GET /triggers
  Then the response is HTTP 200
  And the body is {"triggers": []}

Scenario: Triggers are user-scoped
  Given Sofia (auth0|sofia) and Marco (auth0|marco) both have trigger_candidates rows
  When Sofia calls GET /triggers
  Then only Sofia's triggers appear in the response

Scenario: Unauthenticated request is rejected
  Given a request to GET /triggers with no Authorization header
  When the request is received
  Then the response is HTTP 401

Scenario: Database unavailable returns retriable error
  Given the database is unreachable
  When Sofia calls GET /triggers
  Then the response is HTTP 503 with a Retry-After header
```

### Acceptance Criteria

- [ ] Route `GET /triggers` registered in `cmd/main.go` under auth + consent middleware group
- [ ] Handler reads `trigger_candidates` filtered by JWT-derived `user_id`
- [ ] Response is HTTP 200 with `Content-Type: application/json`
- [ ] Response body: `{"triggers": [...]}` — array never null (use empty slice, not nil)
- [ ] Each entry includes: `ingredient`, `symptom_type`, `correlation_count`, `window_hours`, `last_evaluated_at`
- [ ] No cross-user leakage — query always includes `WHERE user_id = $1`
- [ ] Missing/invalid JWT returns HTTP 401 (existing auth middleware)
- [ ] DB error returns HTTP 503 with `Retry-After: 30`
- [ ] Handler uses injected Store interface
- [ ] Integration test covers empty-result case (HTTP 200 + `{"triggers": []}`)

### Outcome KPIs

- **Who**: Authenticated IBS users with ≥1 completed engine run
- **Does what**: Retrieve trigger candidate list in a single API call
- **By how much**: 100% of calls return correct scoped list; zero cross-user contamination
- **Measured by**: Integration tests with two distinct user_ids; production audit logs
- **Baseline**: Endpoint does not exist

### Technical Notes

- Register: `r.Get("/triggers", triggerHandler.Get)` inside `consent.RequireConsent` group
- Empty array: `[]TriggerCandidate{}` not `nil` (avoids JSON null)
- `last_evaluated_at` in RFC 3339 UTC — consistent with `scanned_at` in meal events
- No pagination in this slice — deferred. Flag for DESIGN wave if user scale warrants it
- Dependencies: US-CE-01 (table schema), US-CE-02 (engine produces rows to read)

---

## US-CE-04: Background Scheduler

### Elevator Pitch

- **Before**: The engine only runs on manual `POST /admin/correlations/run` — trigger candidates are never updated in production without developer intervention.
- **After**: The engine runs automatically every 6 hours (configurable), keeping `trigger_candidates` current for all users without any manual action.
- **Decision enabled**: Operations team can rely on up-to-date trigger data for all users without manual scheduling; Sofia's triggers reflect her most recent logged data.

### Problem

Lena Fischer (operations engineer) needs the engine to run continuously in production. Without a scheduler, the correlation feature only works during developer testing. Sofia's trigger list would never update post-deployment.

### Who

- IBS user (Sofia Andrade) — indirect; direct actor is the background scheduler
- Go service with graceful shutdown (established pattern in `cmd/main.go`); env var configuration
- Sofia's trust in the feature depends on results staying current as she logs new data

### Solution

Background goroutine (ticker-based) within the correlation engine service. On each tick, processes all users with symptom events. Configurable via `CRON_INTERVAL`. Respects graceful shutdown via context cancellation. Advisory lock per user_id prevents concurrent runs.

### Domain Examples

#### 1: Happy Path — Runs every 6 hours

At 06:00, 12:00, 18:00, 00:00 the scheduler fires. Sofia logged a bloating event at 11:30. By 12:00 her `trigger_candidates` reflect the new data.

#### 2: Edge Case — Concurrent run skipped

12:00 tick fires while 06:00 run for Hiroshi (large dataset) is still in progress. Advisory lock causes Hiroshi's 12:00 run to be skipped and logged. Sofia's 12:00 run proceeds normally.

#### 3: Error Path — DB unavailable

06:00 run begins. DB returns error. Engine logs failure with context, marks users as failed for this tick, waits for 12:00. Service does not crash.

#### 4: Edge Case — SIGTERM during run

Service receives SIGTERM mid-run. Context cancelled. Engine stops after current user completes. Partial results for completed users remain persisted. Service exits cleanly.

### UAT Scenarios (BDD)

```gherkin
Scenario: Engine runs automatically on configured interval
  Given CRON_INTERVAL is "6h"
  And users with symptom events exist
  When 6 hours have elapsed since the last run
  Then the engine runs a correlation pass for all eligible users
  And trigger_candidates is updated with fresh counts

Scenario: Interval is configurable via environment variable
  Given CRON_INTERVAL is set to "1h"
  When the service starts
  Then the scheduler ticks every 1 hour and logs the effective interval at startup

Scenario: Graceful shutdown stops scheduler cleanly
  Given the service is running with the scheduler active
  When the service receives SIGTERM
  Then the scheduler stops accepting new ticks
  And no panic or incomplete write is produced

Scenario: Concurrent run for same user is skipped safely
  Given the engine is processing a run for Sofia
  When the scheduler fires again before her run completes
  Then the second run for Sofia is skipped with a log warning
  And no duplicate or corrupted rows appear in trigger_candidates

Scenario: DB failure during scheduled run does not crash the service
  Given the database is unreachable when the scheduler fires
  When the engine attempts a run
  Then the service logs the error and continues running
  And the next scheduled run proceeds normally when the DB recovers
```

### Acceptance Criteria

- [ ] Background goroutine starts in `cmd/main.go` alongside HTTP server
- [ ] `time.NewTicker` with interval from `CRON_INTERVAL` env var; defaults to `6h`
- [ ] Scheduler goroutine exits cleanly on context cancellation
- [ ] All users with ≥1 symptom_event processed on each tick
- [ ] Advisory lock (`pg_try_advisory_xact_lock` keyed on user_id) prevents concurrent per-user runs
- [ ] Per-user failure logged with context; does not abort other users in the same tick
- [ ] Startup log includes effective `CRON_INTERVAL`, `CORRELATION_THRESHOLD`, `LOOKBACK_WINDOW_HOURS`
- [ ] No goroutine leak — scheduler goroutine does not outlive the service process

### Outcome KPIs

- **Who**: IBS users with active meal and symptom logging
- **Does what**: Have trigger candidates refreshed automatically
- **By how much**: 100% of users processed per tick; max staleness = 1 interval (6h default)
- **Measured by**: Service logs — per-tick completion; monitoring alert if no tick fires for >2 intervals
- **Baseline**: 0% automated runs today

### Technical Notes

- Graceful shutdown: scheduler receives the same `ctx` from `signal.NotifyContext` in `cmd/main.go`
- Advisory lock: `SELECT pg_try_advisory_xact_lock(hashtext($1))` — non-blocking; if false, skip + warn
- Eligible users query: `SELECT DISTINCT user_id FROM symptom_events` (simple; revisit at scale)
- `CRON_INTERVAL` format: Go duration string (`time.ParseDuration`); fatal at startup if unparseable
- Dependencies: US-CE-02 (engine core must exist before scheduler can invoke it)
