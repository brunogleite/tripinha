# Wave Decisions: Correlation Engine DISCUSS

**Feature**: correlation-engine (issue #7)
**Wave**: DISCUSS
**Date**: 2026-04-28
**Author**: Luna (nw-product-owner)

---

## Decisions Made in This Wave

### Decision 1: trigger_candidates table schema

**Context**: The issue AC mentions `trigger_candidates` but does not specify the schema.

**Decision**: Columns are `id, user_id, ingredient_name, symptom_type, correlation_count, window_hours, last_evaluated_at`. Unique constraint on `(user_id, ingredient_name, symptom_type)`. Index on `(user_id)`.

**Rationale**:
- Compound unique key enables idempotent UPSERT on re-runs without generating duplicate rows
- `window_hours` stored per-row because the env var may be reconfigured; historical rows should reflect the window at time of computation
- `last_evaluated_at` enables future cache-invalidation logic and user-facing "last updated" display
- No FK to a users table — `user_id` is an Auth0 subject string (consistent with all existing tables in the project)

**Alternatives considered**: Storing only the most recent count vs. all-time cumulative count. Decision: all-time cumulative (simpler; partial runs do not undercount). DESIGN wave may revisit if time-windowed decay is needed.

---

### Decision 2: Correlation count is cumulative across all symptom windows

**Context**: The issue says "counts ingredient occurrences in the N-hour window before it" — ambiguous whether this is per-window or total.

**Decision**: `correlation_count` is the total number of symptom windows (for a given symptom type) in which the ingredient appeared — not a per-window count.

**Rationale**: A threshold of 5 on a per-window count would mean an ingredient appearing 5 times in one window counts the same as appearing once in 5 separate windows. Cumulative across windows better reflects "how many times has this ingredient preceded this symptom type" — which is the user-relevant signal.

---

### Decision 3: On-demand endpoint is POST /admin/correlations/run

**Context**: Issue says "Engine can be triggered on-demand (for testing)" — no endpoint path specified.

**Decision**: `POST /admin/correlations/run` with body `{"user_id": "..."}`. Admin-only (enforced by auth middleware or separate admin role — DESIGN wave decides the exact auth mechanism). Returns HTTP 202 Accepted.

**Rationale**: `POST` is correct (mutation — triggers a run). `/admin/` prefix signals internal/testing use and can be excluded from public API documentation. 202 Accepted is appropriate for potentially async operations.

---

### Decision 4: Scheduler interval default = 6 hours

**Context**: Issue does not specify the background schedule interval.

**Decision**: Default `CRON_INTERVAL=6h`.

**Rationale**: 6 hours balances freshness (users log 3-4 meals/day; a 6h window captures same-day patterns) against DB load (correlation queries do a cross-join of meal_events and symptom_events — on early user data volumes, 6h is safe). Configurable via env var so ops can tune without a code change.

---

### Decision 5: Advisory lock per user_id for concurrent run protection

**Context**: Issue does not specify concurrency behavior.

**Decision**: Use PostgreSQL `pg_try_advisory_xact_lock(hashtext(user_id))` to skip (not block) concurrent runs for the same user.

**Rationale**: Non-blocking (try) is safer than blocking — avoids queue buildup if engine is slow for a user with large history. Skipped runs are logged as warnings. DESIGN wave may upgrade to a proper job queue if run volume warrants it.

---

### Decision 6: symptom_type is not validated by the engine

**Context**: `symptom_events.type` is a free text column with no constraint. The issue mentions "bloating, cramping, etc." as examples.

**Decision**: The engine does not validate or normalize symptom_type values. It groups by whatever strings are in `symptom_events.type`.

**Rationale**: Normalization of symptom vocabulary is out of scope for this slice. Flagged as a tech note for DESIGN wave — a CHECK constraint or enum on `symptom_events.type` is the right fix, but belongs to the symptoms domain, not the correlation engine.

**Risk**: If `symptom_events.type` contains typos or variants ("bloat" vs "bloating"), per-type correlation counts are split. Accepted for now — the symptom logging feature (#4) should enforce vocabulary there.

---

### Decision 7: No DIVERGE wave artifacts present

**Context**: Issue #7 has no `docs/feature/correlation-engine/diverge/` directory.

**Decision**: Proceeded with DISCUSS wave using the issue AC as ground truth (user-confirmed). Job statement bootstrapped from issue user story and included in `docs/product/jobs.yaml`.

**Risk**: No formal JTBD/ODI validation was performed. The job statement "identify food triggers for IBS symptoms" is reasonable but untested with real users. Flagged in `jobs.yaml` as in_progress. Recommend running DISCOVER → DIVERGE on the broader IBS management product opportunity.

---

### Decision 8: No pagination on GET /triggers

**Context**: Issue does not mention pagination.

**Decision**: GET /triggers returns all trigger_candidates for the user in a single response (no pagination) in this slice.

**Rationale**: At current data volumes (small user base, ≤50 trigger candidates per user in realistic scenarios), pagination adds complexity without benefit. DESIGN wave should flag this for review if user scale changes assumptions.

---

## Risks

| Risk | Probability | Impact | Mitigation |
|---|---|---|---|
| Query performance on large meal history | LOW (early stage) | HIGH | Index on meal_events(user_id, scanned_at) exists; add EXPLAIN ANALYZE during DESIGN wave |
| symptom_type typos splitting correlation counts | MEDIUM | MEDIUM | Accepted for this slice; fix in symptoms domain (#4 follow-up) |
| Advisory lock not sufficient at scale | LOW | MEDIUM | Acceptable for current user volumes; revisit with job queue in future release |
| No JTBD validation | MEDIUM | MEDIUM | Bootstrap job statement from issue; recommend DISCOVER wave for full validation |
| CORRELATION_THRESHOLD misconfigured to 0 | LOW | HIGH | Guardrail: engine must reject threshold < 1 at startup |
