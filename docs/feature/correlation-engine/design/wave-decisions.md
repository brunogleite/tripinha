# Wave Decisions: Correlation Engine DESIGN

**Feature**: correlation-engine (issue #7)
**Wave**: DESIGN
**Date**: 2026-04-28
**Author**: Morgan (nw-solution-architect)
**Interaction mode**: PROPOSE
**Builds on**: `docs/feature/correlation-engine/discuss/wave-decisions.md`

---

## Reuse Analysis

Scan of `internal/` for components that overlap with the correlation engine's requirements. Decision: EXTEND (reuse) vs CREATE NEW.

| Existing Component | Location | Overlaps With | Decision | Rationale |
|---|---|---|---|---|
| `auth.UserID(ctx)` | `internal/auth/middleware.go` | All handlers need user_id from JWT | **EXTEND** (use as-is) | Exact function required; no changes needed |
| `auth.NewMiddleware(...)` | `internal/auth/middleware.go` | All routes need JWT validation | **EXTEND** (use as-is) | Existing middleware applied to `/admin/correlations/run` and `GET /triggers` |
| `consent.RequireConsent(...)` | `internal/consent/middleware.go` | `GET /triggers` is a health data read | **EXTEND** (use as-is) | Apply to `GET /triggers`; not applied to `/admin/` route (engine is a system actor) |
| `meals.Store.Save` | `internal/meals/store.go` | Source table `meal_events` + `meal_ingredients` | **READ-ONLY NEW QUERY** | Correlation engine queries these tables directly via its own `CorrelationReader` interface; does not call `meals.Store` (avoids cross-package coupling) |
| `symptoms.Store.Save` | `internal/symptoms/store.go` | Source table `symptom_events` | **READ-ONLY NEW QUERY** | Same rationale as meals — own query via `CorrelationReader` |
| `pgxpool.Pool` | `cmd/main.go` | All store adapters share one pool | **EXTEND** (pass pool to `correlations.NewStore`) | Consistent with all existing package stores |
| `cmd/main.go` graceful shutdown pattern | `cmd/main.go` | Scheduler needs context cancellation | **EXTEND** (pass ctx to scheduler.Start) | Existing `signal.NotifyContext` ctx passed to scheduler — identical pattern to HTTP server goroutine |
| `mustEnv` / `envOr` helpers | `cmd/main.go` | Env var parsing for new config | **EXTEND** (use as-is) | New env vars (`CRON_INTERVAL`, `CORRELATION_THRESHOLD`, etc.) parsed with existing helpers |
| `consent.Storer` interface pattern | `internal/consent/store.go` | Interface-backed store pattern | **EXTEND** (replicate pattern) | Two new interfaces follow the same interface-backed adapter pattern |
| Handler + `json.NewEncoder` pattern | `internal/symptoms/handler.go`, `internal/meals/handler.go` | HTTP handler implementation | **EXTEND** (replicate pattern) | `correlations.Handler` follows identical HTTP handler structure |

**Summary**: No new packages beyond `internal/correlations/`. No new external libraries. All existing auth, consent, DB pool, and graceful shutdown infrastructure reused unchanged.

---

## Architecture Options Proposed

Three options were analysed. Option A was recommended. The user should confirm or select an alternative.

---

### Option A: Single-Port Adapter (RECOMMENDED)

**Structure inside `internal/correlations/`:**

```
correlation.go     — TriggerCandidate, Config, sentinel errors
ports.go           — CorrelationReader interface, CandidateWriter interface
service.go         — Service struct (depends on interfaces only, no pgx imports)
store.go           — pgxStore struct (implements CorrelationReader + CandidateWriter)
scheduler.go       — Scheduler struct (wraps Service, time.NewTicker)
handler.go         — Handler struct (GET /triggers, POST /admin/correlations/run)
```

**Two interfaces:**
- `CorrelationReader` — `ReadSymptomEvents(ctx, userID)` + `ReadMealIngredients(ctx, userID, occurredAt, windowHours)`
- `CandidateWriter` — `UpsertCandidates(ctx, []TriggerCandidate)` + `ListEligibleUsers(ctx)` + `ListByUser(ctx, userID)`

**SQL query shape:** Single correlated query per user (all aggregation in PostgreSQL, threshold in `HAVING` clause).

**Scheduler:** In-process `Scheduler` struct; `Start(ctx)` goroutine; `time.NewTicker`.

**Admin auth:** Static `ADMIN_SECRET` env var; `X-Admin-Secret` header; `subtle.ConstantTimeCompare`; fatal at startup if missing.

**Trade-offs:**

| Quality attribute | Assessment |
|---|---|
| Testability | Excellent — `Service` has zero pgx imports; fakes implement two clean interfaces |
| Maintainability | Excellent — mirrors `internal/meals` pattern exactly |
| Correctness | Excellent — user_id always from JWT context; single SQL query avoids application-side bugs |
| Simplicity | Good — 6 files, consistent pattern |
| Admin security | Acceptable for internal testing endpoint; not auditable per-caller |

---

### Option B: Separate Read/Write Adapters

**Variation on Option A** — split `pgxStore` into two concrete structs: `pgxCorrelationReader` and `pgxCandidateStore`. Each implements exactly one interface.

**Trade-offs vs Option A:**

| Concern | Option B vs Option A |
|---|---|
| Single-responsibility | Better: each adapter owns one concern |
| Testability | Identical — fakes implement same interfaces |
| Consistency with codebase | Worse — existing pattern (`internal/meals`) uses one Store struct implementing multiple interfaces (`Storer` + `FlaggedLogger`) |
| Wiring complexity | Slightly higher — two adapter structs to construct in `cmd/main.go` |
| File count | +1 file |

**Recommendation**: Option B is architecturally valid but adds separation without quality-attribute benefit at this team size. Option A is preferred for codebase consistency.

---

### Option C: Direct Store in Handler (No Service Layer)

Handler calls `pgxStore` directly. Correlation logic embedded in handler methods.

**Trade-offs vs Option A:**

| Concern | Option C vs Option A |
|---|---|
| Testability | Poor — business logic (threshold, window, count accumulation) is inseparable from HTTP layer; requires HTTP test context for every engine test |
| Maintainability | Poor — handler grows to ~200 lines; scheduler cannot call the engine without importing the handler |
| Correctness | Neutral — same SQL |
| Simplicity | Superficially simpler (fewer files), actually harder to test and evolve |

**Recommendation**: Rejected. Violates Testability priority (ranked #2) and the established project pattern for non-trivial service logic.

---

## DESIGN Wave Decisions

### Decision D-01: Option A Selected

**Decision**: Implement `internal/correlations/` following Option A — two interfaces, one `pgxStore` adapter, in-process `Scheduler` struct.

**Rationale**: Maximises testability and maintainability at minimum complexity. Consistent with existing package pattern.

**Alternatives rejected**: Option B (unnecessary split), Option C (untestable).

---

### Decision D-02: SQL Query — Single Correlated Query

**Decision**: One SQL query per user using `JOIN meal_events ON scanned_at BETWEEN (occurred_at - window) AND occurred_at`, `JOIN meal_ingredients`, `GROUP BY ingredient_name, symptom_type`, `HAVING COUNT(DISTINCT symptom_event_id) >= threshold`.

**Rationale**: Single round-trip; all aggregation in PostgreSQL; avoids N+1 degradation as user history grows.

**Alternatives rejected**: Application-side loop (N+1 queries per user), materialised staging table (unnecessary schema complexity).

---

### Decision D-03: Admin Endpoint Auth — Static Secret

**Decision**: `POST /admin/correlations/run` protected by `X-Admin-Secret` header checked against `ADMIN_SECRET` env var via `subtle.ConstantTimeCompare`. Fatal at startup if `ADMIN_SECRET` is not set.

**Rationale**: Simplest viable protection for an internal testing endpoint. No Auth0 config changes required.

**Open question**: If the endpoint needs to be called from automated pipelines or audited per-caller, upgrade to Auth0 M2M (ADR-001 Option B for Decision 3).

**Alternatives rejected**: Auth0 M2M (over-engineering for testing endpoint), IP allowlist only (no defence-in-depth).

---

### Decision D-04: Scheduler — In-Process

**Decision**: `Scheduler` struct inside `internal/correlations/`. Started from `cmd/main.go`. `time.NewTicker` with `CRON_INTERVAL` duration. Exits on context cancellation.

**Rationale**: Zero new dependencies. Consistent with existing goroutine lifecycle pattern in `cmd/main.go`.

**Alternatives rejected**: External cron/Kubernetes CronJob (operational complexity), PostgreSQL job queue library (over-engineering for this scale).

---

### Decision D-05: Architecture Enforcement

**Decision**: Add `go-arch-lint` to CI. Rule: `service.go` in `internal/correlations` must not import `github.com/jackc/pgx`. This enforces the ports/adapters boundary at compile time in CI.

**Rationale**: Architecture rules without enforcement erode. `go-arch-lint` is MIT-licensed, fast, and YAML-configured.

---

## Handoff Package — For DISTILL Wave

**Primary deliverable**: `docs/product/architecture/brief.md`

**ADR**: `docs/product/architecture/adr-001-correlation-engine.md`

**New package**: `internal/correlations/` (to be created by software-crafter)

**Migration**: `migrations/005_trigger_candidates.sql` (to be created by software-crafter)

**Env vars required** (add to `.env.example`):
- `CRON_INTERVAL` — Go duration string, default `6h`
- `CORRELATION_THRESHOLD` — integer, default `5`, fatal if < 1
- `LOOKBACK_WINDOW_HOURS` — integer, default `24`
- `ADMIN_SECRET` — string, no default, fatal if empty

**Routes to register** in `cmd/main.go`:
- `GET /triggers` — under `authMiddleware` + `consent.RequireConsent`
- `POST /admin/correlations/run` — under `authMiddleware` + `requireAdminSecret`

**No external integrations** are introduced. Contract test annotation: not applicable for this feature.

**Paradigm**: OOP — consistent with existing Go codebase (`Service` struct, interface-based dependency injection).

**Architecture enforcement**: Add `go-arch-lint` to CI.
