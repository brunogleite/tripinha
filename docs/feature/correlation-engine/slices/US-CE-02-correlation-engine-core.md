# US-CE-02: Core Correlation Engine

## Elevator Pitch

- **Before**: Sofia's meal and symptom data sit in separate tables — there is no mechanism to connect them or identify patterns.
- **After**: Calling `POST /admin/correlations/run` (or the scheduler) produces rows in `trigger_candidates` for ingredients that correlate with Sofia's symptoms above the configured threshold.
- **Decision enabled**: Sofia (and her care team) can query `GET /triggers` to see which ingredients are statistically associated with her flare-ups — enabling dietary adjustments with evidence rather than guesswork.

## Problem

Sofia Andrade has been logging meals and symptoms for weeks. She knows she reacts badly to certain foods but cannot identify which ingredient is the culprit — there are too many variables and too many meals to manually correlate. The app needs to do this computation for her, automatically.

## Who

- **User type**: IBS user (Sofia Andrade) — indirect beneficiary; the engine runs on her behalf
- **Context**: Brownfield Go API; PostgreSQL; engine runs as a service method called by scheduler and on-demand HTTP endpoint
- **Motivation**: Sofia wants to know which specific ingredients (not just products) precede her flare-ups so she can make targeted dietary changes

## Solution

Implement `internal/correlation` package with a `Service` that:
1. For each user's symptom events, queries `meal_ingredients` in the lookback window
2. Counts ingredient occurrences per symptom type across all windows
3. Upserts to `trigger_candidates` when count >= threshold

## Domain Examples

### 1: Happy Path — Fructose identified as bloating trigger

Sofia has 12 bloating events over 3 weeks. In 7 of those events, she ate fructose-containing food within 24 hours. CORRELATION_THRESHOLD=5. The engine runs: fructose gets a count of 7 for bloating. 7 >= 5 → fructose is upserted as a candidate for Sofia.

### 2: Edge Case — Ingredient just below threshold

Sofia ate gluten in 4 of 12 bloating windows. 4 < 5 → gluten is not written to `trigger_candidates`. If Sofia logs more meals and the next run finds 5 windows, gluten becomes a candidate then.

### 3: Error Path — No meal data in any window

Sofia logged 5 symptom events this month but forgot to scan meals during that period. Every symptom window returns zero meal ingredients. Engine completes with 0 candidate rows. No error is raised — empty result is valid.

### 4: Edge Case — Multiple symptom types treated independently

Sofia has bloating (12 events) and cramping (6 events). Fructose correlates with 7 bloating windows but only 2 cramping windows. → fructose is a candidate for bloating; not for cramping. Both computations run independently.

## UAT Scenarios (BDD)

### Scenario: Ingredient exceeding threshold becomes a trigger candidate

```gherkin
Given Sofia has 7 bloating symptom events
And in each of those 7 windows, she ate fructose
And CORRELATION_THRESHOLD is 5
When the correlation engine runs for Sofia
Then trigger_candidates contains a row for (sofia, fructose, bloating)
And correlation_count is 7
```

### Scenario: Ingredient below threshold is excluded

```gherkin
Given Sofia has 7 bloating symptom events
And gluten appears in only 3 of those windows
And CORRELATION_THRESHOLD is 5
When the correlation engine runs for Sofia
Then trigger_candidates does not contain a row for (sofia, gluten, bloating)
```

### Scenario: Window boundary is respected

```gherkin
Given Sofia has a bloating event at 2026-04-28T14:45:00Z
And LOOKBACK_WINDOW_HOURS is 24
And she ate food containing fructose at 2026-04-27T14:00:00Z (within window)
And she ate food containing lactose at 2026-04-26T20:00:00Z (outside window, 42h before)
When the engine runs the window query for this symptom event
Then fructose is counted for this window
And lactose is not counted for this window
```

### Scenario: Symptom types are correlated independently

```gherkin
Given Sofia has both bloating and cramping events
And fructose appears in 7 bloating windows but only 2 cramping windows
And CORRELATION_THRESHOLD is 5
When the engine runs
Then trigger_candidates contains fructose for bloating
And trigger_candidates does not contain fructose for cramping
```

### Scenario: Empty meal history produces empty candidates gracefully

```gherkin
Given Sofia has 5 bloating events
And she has no meal_events in any of those lookback windows
When the engine runs for Sofia
Then no rows are written to trigger_candidates
And the engine completes without error
```

### Scenario: Re-running engine updates existing counts

```gherkin
Given trigger_candidates already has (sofia, fructose, bloating, count=7)
And a new run finds fructose in 9 bloating windows
When the engine runs again for Sofia
Then trigger_candidates is updated with correlation_count=9
And last_evaluated_at is refreshed
And no duplicate row is created
```

## Acceptance Criteria

- [ ] `internal/correlation` package exists with a `Service` struct
- [ ] Engine queries `meal_ingredients` JOIN `meal_events` using the configured lookback window, scoped by `user_id`
- [ ] Engine accumulates counts per `(ingredient_name, symptom_type)` across all symptom windows for the user
- [ ] Only ingredients with count >= CORRELATION_THRESHOLD are written to `trigger_candidates`
- [ ] Each symptom type is processed independently (no cross-contamination between symptom types)
- [ ] Engine completes without error when no meal events exist in any window
- [ ] UPSERT prevents duplicate rows; re-runs update existing rows
- [ ] `window_hours` stored in each upserted row matches the env configuration at time of run
- [ ] Engine is invocable from a `POST /admin/correlations/run` endpoint (accepts `{"user_id": "..."}`)
- [ ] On-demand endpoint returns HTTP 202 Accepted immediately (processing may be async or sync — DESIGN wave decides)
- [ ] CORRELATION_THRESHOLD and LOOKBACK_WINDOW_HOURS are read from environment variables with documented defaults (5 and 24)
- [ ] Engine is unit-testable with a mock store (follows existing `internal/meals` interface pattern)

## Outcome KPIs

- **Who**: IBS users with at least 5 logged symptom events and corresponding meal data
- **Does what**: Receive at least one ingredient-symptom correlation candidate after engine runs
- **By how much**: >0 trigger candidates produced for users with sufficient history (≥5 symptom events + meal data in windows)
- **Measured by**: Integration test seeded with controlled data; production monitoring via `SELECT COUNT(*) FROM trigger_candidates GROUP BY user_id`
- **Baseline**: 0 candidates produced today (feature does not exist)

## Technical Notes

- Follow the interface pattern from `internal/meals`: define a `Store` interface consumed by `Service` — enables test doubles without mocking the DB directly
- The correlation count is the total across ALL symptom windows for a user, not per-window. Example: if fructose appears in 7 out of 12 bloating windows, correlation_count = 7
- `LOOKBACK_WINDOW_HOURS` must be read once and passed consistently through the entire run — no mid-run re-reads
- Performance note for DESIGN wave: the window query joins meal_events and meal_ingredients. The index on `meal_events(user_id, scanned_at)` already exists (migration 002). Verify query plan with EXPLAIN ANALYZE on realistic data volumes before production
- `symptom_type` is not validated by the engine — it reflects whatever is in `symptom_events.type`. Normalization of symptom vocabulary is out of scope for this slice
- Dependencies: US-CE-01 (trigger_candidates table must exist before first upsert)
