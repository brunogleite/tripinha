# Definition of Ready Validation: Correlation Engine

**Date**: 2026-04-28
**Validator**: Luna (nw-product-owner, review mode)
**Wave**: DISCUSS → DESIGN handoff gate

---

## US-CE-01: Trigger Candidates Schema

| DoR Item | Status | Evidence |
|---|---|---|
| Problem statement clear, domain language | PASS | "The correlation engine needs a durable store for its output. Without a persistent trigger_candidates table, no downstream feature can be built." |
| User/persona with specific characteristics | PASS | IBS user Sofia Andrade (indirect); direct actor is engine service; brownfield Go + PostgreSQL; sequential migration pattern |
| 3+ domain examples with real data | PASS | 3 examples: happy path (sofia/fructose/6), re-run update (count=8), idempotent migration |
| UAT scenarios in Given/When/Then (3-7) | PASS | 4 scenarios; all use real data |
| AC derived from UAT | PASS | 6 AC items mapping directly to scenarios |
| Right-sized (1-3 days, 3-7 scenarios) | PASS | ~0.5 day effort (schema + migration); 4 scenarios |
| Technical notes: constraints/dependencies | PASS | Migration numbering, UPSERT conflict key, window_hours rationale, no FK rationale, dependencies stated |
| Dependencies resolved or tracked | PASS | #4 and #5 both done per issue; US-CE-01 has no other user story dependencies |
| Outcome KPIs defined with measurable targets | PASS | 100% persistence rate; measured by integration test |

### DoR Status: PASSED

---

## US-CE-02: Core Correlation Engine

| DoR Item | Status | Evidence |
|---|---|---|
| Problem statement clear, domain language | PASS | "Sofia has been logging meals and symptoms for weeks but cannot identify which ingredient causes her flare-ups — too many variables." |
| User/persona with specific characteristics | PASS | Sofia Andrade, IBS user; indirect beneficiary; engine service is direct actor; Go API with pgx/v5 |
| 3+ domain examples with real data | PASS | 4 examples: fructose at threshold (count=7), gluten below threshold (count=4), empty window (no meals), independent symptom types |
| UAT scenarios in Given/When/Then (3-7) | PASS | 6 scenarios; concrete data throughout |
| AC derived from UAT | PASS | 12 AC items; each traces to a scenario or decision |
| Right-sized (1-3 days, 3-7 scenarios) | PASS | ~1.5 days effort (package + SQL + test); 6 scenarios |
| Technical notes: constraints/dependencies | PASS | Interface pattern, cumulative count decision, LOOKBACK read-once, EXPLAIN ANALYZE note, symptom_type not validated, advisory lock reference |
| Dependencies resolved or tracked | PASS | US-CE-01 listed as dependency; upstream #4 and #5 done |
| Outcome KPIs defined with measurable targets | PASS | 80% of eligible users receive ≥1 candidate; measured by integration test + SQL count |

### DoR Status: PASSED

---

## US-CE-03: GET /triggers Endpoint

| DoR Item | Status | Evidence |
|---|---|---|
| Problem statement clear, domain language | PASS | "Sofia has no way to retrieve correlation results. GET /triggers bridges background computation with Sofia's daily dietary decisions." |
| User/persona with specific characteristics | PASS | Sofia Andrade, authenticated IBS user; Auth0 JWT; chi router; pattern from POST /meals |
| 3+ domain examples with real data | PASS | 4 examples: two candidates (fructose/bloating + onion/cramping), empty pre-run, DB down, Marco's data excluded |
| UAT scenarios in Given/When/Then (3-7) | PASS | 5 scenarios; user-scoped security test included |
| AC derived from UAT | PASS | 10 AC items; all map to observable behavior |
| Right-sized (1-3 days, 3-7 scenarios) | PASS | ~0.5 day effort (handler + route + test); 5 scenarios |
| Technical notes: constraints/dependencies | PASS | Route registration path, nil vs empty slice, RFC 3339, no pagination deferred, dependencies stated |
| Dependencies resolved or tracked | PASS | US-CE-01 and US-CE-02 listed |
| Outcome KPIs defined with measurable targets | PASS | 100% scoped response; zero cross-user contamination; measured by integration test |

### DoR Status: PASSED

---

## US-CE-04: Background Scheduler

| DoR Item | Status | Evidence |
|---|---|---|
| Problem statement clear, domain language | PASS | "Without a scheduler, the correlation feature only works during developer testing. Sofia's trigger list would never update post-deployment." |
| User/persona with specific characteristics | PASS | Sofia Andrade (IBS user, indirect); Lena Fischer (ops engineer, direct); graceful shutdown pattern from cmd/main.go |
| 3+ domain examples with real data | PASS | 4 examples: 6h auto-run (11:30 bloating → processed by 12:00), concurrent run skipped for Hiroshi, DB down (logged+continues), SIGTERM mid-run |
| UAT scenarios in Given/When/Then (3-7) | PASS | 5 scenarios; concurrency + graceful shutdown both covered |
| AC derived from UAT | PASS | 8 AC items tracing to scenarios |
| Right-sized (1-3 days, 3-7 scenarios) | PASS | ~1 day effort (goroutine + advisory lock + startup log + tests); 5 scenarios |
| Technical notes: constraints/dependencies | PASS | Graceful shutdown ctx source, advisory lock SQL, eligible users query, CRON_INTERVAL format, fatal on unparseable, dependencies stated |
| Dependencies resolved or tracked | PASS | US-CE-02 listed as dependency |
| Outcome KPIs defined with measurable targets | PASS | 100% users processed per tick; max staleness = 1 interval; monitoring alert trigger defined |

### DoR Status: PASSED

---

## Summary

| Story | DoR Status | Notes |
|---|---|---|
| US-CE-01: Trigger Candidates Schema | PASSED | All 9 items pass |
| US-CE-02: Core Correlation Engine | PASSED | All 9 items pass |
| US-CE-03: GET /triggers Endpoint | PASSED | All 9 items pass |
| US-CE-04: Background Scheduler | PASSED | All 9 items pass |

**All 4 stories pass DoR. Feature is ready for DESIGN wave handoff.**

---

## Peer Review: Self-Review Dimensions

Applying `nw-po-review-dimensions` dimensions:

### Dimension 0: Elevator Pitch Test

| Story | Present | Real Entry Point | Concrete Output | Job Connection |
|---|---|---|---|---|
| US-CE-01 | YES | `migration 005` applied via `psql` or migrate tool | Table exists with correct schema | Developer can build engine against stable data contract |
| US-CE-02 | YES | `POST /admin/correlations/run` (HTTP endpoint) | trigger_candidates rows visible in DB; GET /triggers returns results | Sofia sees actionable ingredient-symptom correlations |
| US-CE-03 | YES | `GET /triggers` (HTTP endpoint) | JSON response with ingredient, symptom_type, correlation_count | Sofia decides which ingredients to eliminate |
| US-CE-04 | YES | Background goroutine visible via startup logs and scheduled behavior | Service logs confirm tick completion; trigger_candidates refreshed | Ops team trusts results stay current without manual intervention |

All elevator pitches pass. No infrastructure-only stories in any release slice.

### Dimension 1: Confirmation Bias

- **Technology bias**: No technology prescribed (e.g., no "use gocron library" — DESIGN wave chooses). Advisory lock is a behavioral requirement (concurrency safety), not a technology prescription.
- **Happy path bias**: Each story includes at least one error path and one edge case. US-CE-02 has 6 scenarios covering threshold boundary, empty window, and re-run. US-CE-04 covers DB failure and concurrent run.
- **Verdict**: No confirmation bias detected.

### Dimension 2: Completeness

- **Stakeholder perspectives**: IBS user (Sofia), ops engineer (Lena), developer (implicit). No compliance/legal gaps identified at this stage (data is health data — flagged in Technical Notes for DESIGN wave to address).
- **Error scenarios**: DB down (US-CE-03, US-CE-04), empty window (US-CE-02), unauthenticated request (US-CE-03), concurrent run (US-CE-04), threshold misconfiguration (wave-decisions.md).
- **NFRs**: GET /triggers p99 < 500ms (outcome-kpis.md guardrail); zero cross-user leakage (AC item in US-CE-03); no goroutine leak (AC item in US-CE-04).
- **Verdict**: Completeness adequate for this slice. Health data privacy (GDPR/HIPAA) flagged as a risk for DESIGN wave — out of scope for this AC-driven fast conversion.

### Dimension 3: Clarity

- All thresholds are concrete: CORRELATION_THRESHOLD=5, LOOKBACK_WINDOW_HOURS=24, CRON_INTERVAL=6h.
- No qualitative performance language ("fast", "efficient") — replaced with measurable guardrail (p99 < 500ms).
- **Verdict**: No clarity issues.

### Dimension 4: Testability

- All scenarios have observable outcomes: table exists, row count, HTTP status code, log message presence.
- No AC describes internal state — all describe observable behavior.
- **Verdict**: All AC are testable.

### Dimension 5: Priority Validation

- Q1 (largest bottleneck): YES — no correlation results exist today; this is the only path to actionable dietary insights.
- Q2 (simpler alternatives considered): YES — walking skeleton first (schema + engine + endpoint) before scheduler; on-demand trigger before automated scheduler.
- Q3 (constraint prioritization): CORRECT — advisory lock is a correctness constraint, not a performance constraint; not dominating the design.
- Q4 (data justified): ADEQUATE for a new feature with no baseline; KPIs establish measurement from day 1.
- **Verdict**: PASS

### Overall Review Verdict: APPROVED

All 4 stories meet DoR. No blocking issues. Feature package is ready for handoff to solution-architect (DESIGN wave).
