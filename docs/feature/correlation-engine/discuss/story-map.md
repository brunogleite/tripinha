# Story Map: Correlation Engine

## User
IBS user (Sofia Andrade) — indirect beneficiary. Direct actors: the correlation engine service and the background scheduler.

## Goal
Sofia wants to know which ingredients consistently precede her IBS symptoms so she can make confident dietary adjustments.

## Backbone

| Persist Results | Query Correlation | Apply Threshold | Surface Results | Enable Scheduling |
|---|---|---|---|---|
| Design trigger_candidates schema | Query meal_ingredients in lookback window | Filter by correlation_count >= threshold | GET /triggers reads table | Background scheduler runs engine |
| Write migration 005 | Scope query by user_id and symptom_type | Upsert to trigger_candidates | Return scoped JSON (user_id filter) | On-demand POST endpoint for testing |
| Implement CorrelationStore | Accumulate counts across all symptom windows | Refresh last_evaluated_at | Handle empty results (200 + []) | Env-configurable interval + threshold + window |

---

### Walking Skeleton

The thinnest end-to-end slice that produces a verifiable result:

1. **Persist Results** — `trigger_candidates` table exists (migration 005)
2. **Query Correlation** — engine queries `meal_ingredients` in a 24h window for one user, one symptom type
3. **Apply Threshold** — counts meeting threshold are upserted to `trigger_candidates`
4. **Surface Results** — `GET /triggers` reads `trigger_candidates` and returns the list
5. **Enable Scheduling** — on-demand `POST /admin/correlations/run` triggers a run (scheduler deferred to Release 1)

Walking Skeleton = US-CE-01 + US-CE-02 + US-CE-03 (schema + core engine + read endpoint)

---

### Release 1: Engine is reliable in production

Tasks included:
- Background scheduler with configurable interval (US-CE-04)
- Advisory lock per user_id to prevent concurrent runs
- Env-configurable: CORRELATION_THRESHOLD, LOOKBACK_WINDOW_HOURS, CRON_INTERVAL

Outcome KPI targeted: Correlation results available for all users within 6 hours of new symptom data — without manual intervention.

---

## Priority Rationale

| Priority | Slice | Rationale |
|---|---|---|
| 1 | Walking Skeleton (US-CE-01 to US-CE-03) | Validates the riskiest assumption: can we produce non-trivial correlations from real meal + symptom data? Without this, everything else is speculation. |
| 2 | Release 1 (US-CE-04) | Operationalizes the engine. Without a scheduler, correlations only exist during developer testing — no production value. |

Walking skeleton beats Release 1 because it de-risks the core algorithm and the DB schema before investing in operational infrastructure.

## Scope Assessment: PASS

4 stories, 2 bounded contexts (correlation engine + HTTP handler), estimated 3-4 days total (≤1 day each), single independent user outcome: IBS user sees ingredient-symptom correlations.
