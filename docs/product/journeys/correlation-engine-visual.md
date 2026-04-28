# Journey: Correlation Engine — Data Flow Visual

**Feature**: correlation-engine (issue #7)
**Type**: Backend service — data flow perspective (no UI, no CLI)
**Persona**: IBS user (Sofia Andrade) — benefits indirectly via GET /triggers
**Emotional arc**: Uncertainty → Discovery → Confidence

---

## High-Level Data Flow

```
[meal_events]          [symptom_events]
  + meal_ingredients        (bloating, cramping, …)
        |                         |
        |   joined by user_id     |
        +----------+  +-----------+
                   |  |
                   v  v
        +------------------------------+
        |  Correlation Engine          |
        |  (internal/correlation)      |
        |                              |
        |  For each symptom_event:     |
        |    window = [occurred_at     |
        |              - 24h,          |
        |              occurred_at]    |
        |                              |
        |  Count ingredient occurrences|
        |  in that window              |
        |                              |
        |  If count >= threshold (5):  |
        |    upsert trigger_candidates |
        +------------------------------+
                   |
                   v
        +------------------------------+
        |  trigger_candidates table    |
        |  (persisted results)         |
        +------------------------------+
                   |
                   v
        +------------------------------+
        |  GET /triggers               |
        |  (reads table, returns JSON) |
        +------------------------------+
                   |
                   v
        [IBS user sees trigger list]
```

---

## Step-by-Step Data Flow (with emotional annotations)

### Step 1: Data Ingestion (already done — #4, #5)

```
+-- Step 1: Data Available ----------------------------------------+
|                                                                   |
|  meal_events                  symptom_events                     |
|  ┌─────────────────────┐      ┌────────────────────────┐        |
|  │ id=1, user=sofia     │      │ id=7, user=sofia        │       |
|  │ barcode=3017620422003│      │ type=bloating           │       |
|  │ scanned_at=10:30     │      │ severity=3              │       |
|  └─────────────────────┘      │ occurred_at=14:45       │       |
|                                └────────────────────────┘        |
|  meal_ingredients                                                 |
|  ┌─────────────────────────────────────────────┐                 |
|  │ meal_event_id=1, ingredient_name=lactose     │                 |
|  │ meal_event_id=1, ingredient_name=gluten      │                 |
|  └─────────────────────────────────────────────┘                 |
|                                                                   |
|  Sofia's emotional state: UNCERTAIN — "I feel bad often but      |
|  don't know why."                                                 |
+-------------------------------------------------------------------+
```

### Step 2: Engine Triggered (on-demand or scheduled)

```
+-- Step 2: Correlation Run Initiated ------------------------------+
|                                                                   |
|  Trigger sources:                                                 |
|    A) Background scheduler  →  every 6 hours (CRON_INTERVAL=6h)  |
|    B) On-demand HTTP call   →  POST /admin/correlations/run      |
|                                  (admin-only, for testing)        |
|                                                                   |
|  Engine receives: user_id=sofia                                   |
|  (runs once per user, per trigger)                                |
|                                                                   |
|  Decision: Run per symptom_type independently                    |
|    → bloating  correlation run                                    |
|    → cramping  correlation run                                    |
|    → nausea    correlation run (if events exist)                  |
|                                                                   |
+-------------------------------------------------------------------+
```

### Step 3: Window Query (core logic)

```
+-- Step 3: Time Window Correlation Query --------------------------+
|                                                                   |
|  For symptom_event id=7 (bloating, occurred_at=14:45):           |
|                                                                   |
|  Window: [14:45 - 24h = 14:45 yesterday] → [14:45 today]        |
|                                                                   |
|  SQL (conceptual):                                                |
|  SELECT mi.ingredient_name, COUNT(*) as occurrences              |
|  FROM   meal_ingredients mi                                       |
|  JOIN   meal_events me ON me.id = mi.meal_event_id               |
|  WHERE  me.user_id = 'sofia'                                      |
|  AND    me.scanned_at BETWEEN (occurred_at - interval '24h')     |
|                           AND  occurred_at                        |
|  GROUP BY mi.ingredient_name                                      |
|                                                                   |
|  Result:                                                          |
|    lactose  → 3 windows with bloating                            |
|    gluten   → 1 window with bloating                             |
|    fructose → 6 windows with bloating  ← exceeds threshold=5!   |
|                                                                   |
|  Sofia's data is being analyzed — she doesn't know it yet.       |
+-------------------------------------------------------------------+
```

### Step 4: Threshold Check and Upsert

```
+-- Step 4: Confidence Threshold Gate -----------------------------+
|                                                                   |
|  CORRELATION_THRESHOLD = 5 (configurable via env)                |
|  LOOKBACK_WINDOW       = 24h (configurable via env)              |
|                                                                   |
|  For each ingredient x symptom_type combination:                 |
|    fructose + bloating → count=6 → 6 >= 5 → CANDIDATE           |
|    lactose  + bloating → count=3 → 3 < 5  → skip                |
|    gluten   + bloating → count=1 → 1 < 5  → skip                |
|                                                                   |
|  Upsert to trigger_candidates:                                   |
|  ┌──────────────────────────────────────────────────┐            |
|  │ user_id=sofia                                     │            |
|  │ ingredient_name=fructose                          │            |
|  │ symptom_type=bloating                             │            |
|  │ correlation_count=6                               │            |
|  │ window_hours=24                                   │            |
|  │ last_evaluated_at=now()                           │            |
|  └──────────────────────────────────────────────────┘            |
|                                                                   |
+-------------------------------------------------------------------+
```

### Step 5: Results Surface via GET /triggers

```
+-- Step 5: IBS User Reads Trigger Candidates ----------------------+
|                                                                   |
|  GET /triggers                                                    |
|                                                                   |
|  Response (HTTP 200):                                             |
|  {                                                                |
|    "triggers": [                                                  |
|      {                                                            |
|        "ingredient": "fructose",                                  |
|        "symptom_type": "bloating",                               |
|        "correlation_count": 6,                                    |
|        "window_hours": 24,                                        |
|        "last_evaluated_at": "2026-04-28T14:45:00Z"              |
|      }                                                            |
|    ]                                                              |
|  }                                                                |
|                                                                   |
|  Sofia's emotional state: CONFIDENT — "Now I know it's fructose, |
|  not just 'certain foods'. I can act on this."                    |
+-------------------------------------------------------------------+
```

---

## Error Paths

```
+-- Error: No meal data in window ---------------------------------+
|  Symptom event exists but zero meal_events in lookback window   |
|  → engine completes with 0 correlations for this event          |
|  → no trigger_candidates written                                 |
|  → silent success (not an error)                                 |
+-----------------------------------------------------------------+

+-- Error: DB unavailable during run -----------------------------+
|  Engine logs error, marks run as failed                          |
|  Scheduler retries on next interval                              |
|  On-demand endpoint returns HTTP 503 with retry-after header    |
+-----------------------------------------------------------------+

+-- Error: User has insufficient history -------------------------+
|  Fewer than threshold symptom events exist                      |
|  → correlation counts never reach threshold                     |
|  → no trigger_candidates written (expected behavior)            |
|  → GET /triggers returns {"triggers": []}                       |
+-----------------------------------------------------------------+

+-- Error: Concurrent runs for same user -------------------------+
|  Scheduler fires while prior run still in progress              |
|  → advisory lock per user_id prevents double processing         |
|  → second trigger skips and logs a warning                      |
+-----------------------------------------------------------------+
```

---

## Integration Points

| Upstream | Provides | Risk |
|---|---|---|
| meal_events + meal_ingredients | Ingredient consumption history | HIGH — missing join index causes slow scans |
| symptom_events | Symptom timestamps and types | HIGH — must have occurred_at index |
| Auth middleware | user_id extracted from JWT | MEDIUM — user_id mismatch produces empty results |

| Downstream | Consumes | Risk |
|---|---|---|
| GET /triggers handler | Reads trigger_candidates | MEDIUM — must handle empty table gracefully |
| Future: notification service | Subscribes to new trigger_candidates | LOW — deferred |

---

## Shared Artifacts Registry (summary)

| Artifact | Source | Consumers |
|---|---|---|
| `CORRELATION_THRESHOLD` | env var (default: 5) | Engine threshold check, GET /triggers response metadata |
| `LOOKBACK_WINDOW_HOURS` | env var (default: 24) | Engine window query, trigger_candidates.window_hours |
| `CRON_INTERVAL` | env var (default: 6h) | Background scheduler |
| `user_id` | Auth0 JWT → auth middleware | All queries, trigger_candidates |
| `symptom_type` | symptom_events.type | Per-type grouping, trigger_candidates |
