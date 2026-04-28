# DESIGN Wave Handoff Package: Correlation Engine

**From**: Luna (nw-product-owner, DISCUSS wave)
**To**: solution-architect (DESIGN wave)
**Date**: 2026-04-28
**Issue**: #7 (brunogleite/tripinha)
**DoR gate**: PASSED (all 4 stories, see dor-validation.md)

---

## What Was Built in This Wave

Four user stories, all DoR-PASSED, covering the complete correlation engine slice:

| Story | Title | Effort | Key Output |
|---|---|---|---|
| US-CE-01 | Trigger Candidates Schema | ~0.5 day | migration 005_trigger_candidates.sql |
| US-CE-02 | Core Correlation Engine | ~1.5 days | internal/correlation Service + Store interface |
| US-CE-03 | GET /triggers Endpoint | ~0.5 day | HTTP handler + route registration |
| US-CE-04 | Background Scheduler | ~1 day | goroutine + advisory lock + startup config log |

**Total estimated effort**: 3.5 days

---

## Key Design Decisions for DESIGN Wave

The following decisions were made in requirements and must be respected in architecture. See `wave-decisions.md` for full rationale.

1. **trigger_candidates UPSERT key**: `(user_id, ingredient_name, symptom_type)` — do not change without updating the conflict resolution logic
2. **Correlation count is cumulative across all symptom windows** — not per-window, not time-decayed (in this slice)
3. **LOOKBACK_WINDOW_HOURS read once per engine run** — never per-symptom-event; store in trigger_candidates.window_hours
4. **Advisory lock**: `pg_try_advisory_xact_lock(hashtext(user_id))` — non-blocking; skip + warn on contention
5. **On-demand endpoint**: `POST /admin/correlations/run` — admin/internal only; returns 202 Accepted
6. **symptom_type is not validated**: engine groups by raw `symptom_events.type` value — normalization is out of scope (risk: typos split counts; see wave-decisions.md Decision 6)
7. **GET /triggers returns empty array, not 404** when no candidates exist
8. **No pagination** on GET /triggers in this slice — deferred

---

## Open Questions for DESIGN Wave

1. **Admin auth for POST /admin/correlations/run**: What auth mechanism protects this endpoint? Separate admin role in Auth0? Internal network restriction? IP allowlist? Requires a decision before implementation.

2. **Health data privacy (GDPR/HIPAA)**: `trigger_candidates` stores health-derived inferences (ingredient → IBS symptom correlation). Does this table require data retention limits, right-to-erasure hooks, or data processing agreements? This was out of scope for requirements but must be addressed before production.

3. **On-demand endpoint sync vs async**: Should `POST /admin/correlations/run` run synchronously (blocks until complete, returns 200 with result) or asynchronously (202 Accepted, runs in background)? Requirements say 202; DESIGN wave chooses the execution model.

4. **Eligible users query at scale**: `SELECT DISTINCT user_id FROM symptom_events` is simple and correct for current data volumes. At high user scale, a dedicated users table or a pre-filtered query may be needed. Flag for architecture review.

5. **Correlation count decay**: The current model is all-time cumulative. If an ingredient was problematic 6 months ago but not recently, it would remain a trigger candidate indefinitely. Time-windowed decay or a staleness TTL on trigger_candidates rows is a product decision — flagged for future iteration.

---

## Artifact Index

| Artifact | Path | Purpose |
|---|---|---|
| Journey visual | `docs/product/journeys/correlation-engine-visual.md` | Data flow ASCII + emotional arc + error paths |
| Journey schema | `docs/product/journeys/correlation-engine.yaml` | Structured journey with embedded Gherkin per step |
| Jobs registry | `docs/product/jobs.yaml` | Bootstrap — job-001: identify food triggers |
| Story map | `docs/feature/correlation-engine/discuss/story-map.md` | Backbone + walking skeleton + release slices |
| Shared artifacts | `docs/feature/correlation-engine/discuss/shared-artifacts-registry.md` | All shared variables with sources and consumers |
| Wave decisions | `docs/feature/correlation-engine/discuss/wave-decisions.md` | 8 decisions + risk register |
| User stories (combined) | `docs/feature/correlation-engine/discuss/user-stories.md` | All 4 stories with AC, system constraints, elevator pitches |
| Slices (individual) | `docs/feature/correlation-engine/slices/US-CE-0{1-4}-*.md` | Standalone story files for handoff to engineers |
| Outcome KPIs | `docs/feature/correlation-engine/discuss/outcome-kpis.md` | 4 KPIs with measurement plan + hypothesis |
| DoR validation | `docs/feature/correlation-engine/discuss/dor-validation.md` | 9-item gate + peer review dimensions |
| This file | `docs/feature/correlation-engine/discuss/handoff-design.md` | DESIGN wave handoff summary |

---

## Handoff to Acceptance Designer (DISTILL wave)

When DESIGN wave completes, hand the following to acceptance-designer:

- `docs/product/journeys/correlation-engine.yaml` — Gherkin scenarios embedded per journey step
- Integration points from shared-artifacts-registry.md (user_id scoping, UPSERT conflict key, window_hours consistency)
- Outcome KPIs guardrails from outcome-kpis.md: GET /triggers p99 < 500ms, zero cross-user contamination, engine error rate < 1%
