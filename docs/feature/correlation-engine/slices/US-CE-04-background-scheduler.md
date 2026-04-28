# US-CE-04: Background Scheduler for Correlation Engine

## Elevator Pitch

- **Before**: The correlation engine only runs when a developer manually calls `POST /admin/correlations/run` — Sofia's trigger candidates are never updated in production without manual intervention.
- **After**: The engine runs automatically on a configurable interval (default: every 6 hours) for all users with unprocessed symptom data, keeping `trigger_candidates` current without any manual action.
- **Decision enabled**: Sofia trusts that her trigger list reflects recent data — she does not need to know the engine exists; it works quietly in the background.

## Problem

Lena Fischer (operations engineer) needs the correlation engine to run continuously in production without manual triggering. Without a scheduler, the correlation feature only works during developer testing. Sofia's trigger candidates would never update after initial setup, making the feature useless in practice.

## Who

- **User type**: IBS user (Sofia Andrade) — indirect beneficiary. Direct actor: the background scheduler within the correlation engine service
- **Context**: Go service with graceful shutdown (pattern already established in `cmd/main.go`); environment variables for configuration
- **Motivation**: Sofia's trigger list must stay current as she logs new meals and symptoms — stale results erode trust in the app

## Solution

Implement a background goroutine (ticker-based) within the correlation engine service. On each tick, the engine processes all users who have symptom events. Configurable via `CRON_INTERVAL` env var. Respects graceful shutdown via context cancellation. Advisory lock per user_id prevents concurrent runs.

## Domain Examples

### 1: Happy Path — Scheduler runs automatically every 6 hours

At 06:00, 12:00, 18:00, and 00:00 the scheduler fires. All users with symptom data are processed. Sofia logged a bloating event at 11:30 — by 12:00 the engine has processed it and her `trigger_candidates` reflect the latest data.

### 2: Edge Case — Scheduler fires while previous run still in progress

The 12:00 tick fires. The engine is still processing Hiroshi's large dataset from the 06:00 run. The advisory lock for Hiroshi's user_id causes his 12:00 run to be skipped and logged as a warning. Sofia's 12:00 run proceeds normally.

### 3: Error Path — DB unavailable during scheduled run

The 06:00 run begins. The DB connection pool returns an error. The engine logs the failure with context, marks all users as failed for this run, and waits for the next tick (12:00). The service does not crash.

### 4: Edge Case — Graceful shutdown during a run

The service receives SIGTERM mid-run. The context is cancelled. The engine stops processing after the current user completes (or at the next safe checkpoint). Partial results for completed users remain in `trigger_candidates`. The service exits cleanly.

## UAT Scenarios (BDD)

### Scenario: Engine runs automatically on configured interval

```gherkin
Given CRON_INTERVAL is set to "6h"
And users with symptom events exist in the database
When 6 hours have elapsed since the last run
Then the engine runs a correlation pass for all eligible users
And trigger_candidates is updated with fresh correlation counts
```

### Scenario: Interval is configurable via environment variable

```gherkin
Given CRON_INTERVAL is set to "1h" (non-default)
When the service starts
Then the scheduler ticks every 1 hour (not 6)
And the engine logs the effective interval at startup
```

### Scenario: Graceful shutdown stops the scheduler cleanly

```gherkin
Given the service is running with the scheduler active
When the service receives SIGTERM
Then the scheduler stops accepting new ticks
And the current in-progress run completes before the service exits (or is cancelled by context)
And no panic or incomplete write is produced
```

### Scenario: Concurrent run for same user is skipped safely

```gherkin
Given the engine is currently processing a correlation run for Sofia
When the scheduler fires again before Sofia's run completes
Then the second run for Sofia is skipped with a log warning
And no duplicate or corrupted rows are written to trigger_candidates
```

### Scenario: DB failure during scheduled run does not crash the service

```gherkin
Given the database is unreachable when the scheduler fires
When the engine attempts a correlation run
Then the engine logs the DB error with context
And the service continues running (does not exit)
And the next scheduled run proceeds normally when the DB recovers
```

## Acceptance Criteria

- [ ] Background goroutine starts in `cmd/main.go` alongside the HTTP server (mirrors graceful shutdown pattern already established)
- [ ] Scheduler uses `time.NewTicker` with interval from `CRON_INTERVAL` env var; defaults to `6h` if unset
- [ ] Scheduler goroutine exits cleanly when the context is cancelled (SIGTERM/SIGINT)
- [ ] Engine processes all users with at least one symptom_event on each tick
- [ ] Advisory lock (PostgreSQL `pg_try_advisory_xact_lock` keyed on user_id hash) prevents concurrent per-user runs
- [ ] Per-user run failure is logged with context and does not abort other users' runs in the same tick
- [ ] Service startup logs: effective `CRON_INTERVAL`, `CORRELATION_THRESHOLD`, and `LOOKBACK_WINDOW_HOURS`
- [ ] No goroutine leak: scheduler goroutine does not outlive the service process

## Outcome KPIs

- **Who**: IBS users with active meal and symptom logging
- **Does what**: Have their trigger candidates refreshed automatically without manual intervention
- **By how much**: Trigger candidates updated within 1 interval (default 6h) of new symptom data being logged — 100% of users processed per tick
- **Measured by**: Service logs showing per-tick run completion; monitoring alert if no tick fires for >2 intervals
- **Baseline**: 0% automated runs today (feature does not exist)

## Technical Notes

- Graceful shutdown integration: the scheduler must receive the same `ctx` passed to `server.Shutdown` in `cmd/main.go` — same pattern as the existing shutdown logic
- Advisory lock approach: `SELECT pg_try_advisory_xact_lock(hashtext(user_id))` — if false, skip and log; does not block other users
- "All eligible users" query: `SELECT DISTINCT user_id FROM symptom_events` — simple and correct for current data volumes; revisit if user count grows large
- CRON_INTERVAL format: Go duration string (e.g., "6h", "1h30m") — use `time.ParseDuration`; log a fatal error at startup if unparseable
- Dependencies: US-CE-02 (engine core must exist before scheduler can invoke it)
