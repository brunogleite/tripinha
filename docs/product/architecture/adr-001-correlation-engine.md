# ADR-001: Correlation Engine — Architecture Decisions

**Status**: Accepted
**Date**: 2026-04-28
**Author**: Morgan (nw-solution-architect)
**Feature**: correlation-engine (issue #7)
**Supersedes**: —
**Superseded by**: —

---

## Context

The correlation engine (issue #7) requires a new `internal/correlations` package in a Go 1.21 modular monolith. Four decisions require explicit architectural trade-off analysis before implementation:

1. **Package / component boundary design** — how many structs, how many interfaces, what owns what
2. **Scheduler placement** — where the background ticker lives
3. **Admin endpoint authentication** — no admin role currently exists; Auth0 JWT is the only auth mechanism
4. **SQL query shape** — how the correlation computation is expressed in SQL

The team is a small team (single developer at this stage). The codebase follows a consistent `Service` + `Store interface` + `pgx Store impl` + `Handler` pattern established in `internal/meals` and `internal/symptoms`. All quality-attribute priorities in order: Correctness > Testability > Maintainability > Performance.

---

## Decision 1: Package Structure and Component Boundaries

### Options Considered

**Option A — Single-port adapter (two interfaces, one pgxStore)**

Two interfaces in the package:
- `CorrelationReader`: read-only access to `meal_ingredients` and `symptom_events`
- `CandidateWriter`: write/read access to `trigger_candidates` + eligibility query

One concrete adapter struct `pgxStore` that implements both interfaces. `Service` struct depends on both interfaces and imports no `pgx` types. `Handler` depends on `CandidateWriter` (for GET /triggers) and `Service` (for RunForUser).

Mirrors the structure of `internal/meals` where `Storer` and `FlaggedLogger` are separate interfaces but the same `*Store` struct satisfies both.

*Pros*: Consistent with existing codebase pattern. Service is fully testable without any DB. Minimal indirection — two interfaces map directly to two logical concerns (read domain data / write correlation results). No new packages.

*Cons*: `pgxStore` has multiple responsibilities (reads two tables, writes one); some engineers prefer single-responsibility adapters.

**Option B — Three separate interfaces (one per table group)**

Three interfaces: `SymptomReader`, `MealReader`, `CandidateStore`. Three separate implementations, or one implementation of all three. `Service` depends on all three.

*Pros*: Maximum single-responsibility at the interface level.

*Cons*: Over-engineering for this scope. Three interfaces for what is effectively one store means three fakes in every test. Diverges from the established two-interface pattern in `internal/meals`. No quality-attribute benefit justifies the added indirection for a small team.

**Option C — No Service layer (handler calls store directly)**

Handler contains business logic directly; no Service struct.

*Pros*: Fewer files; simple for trivial CRUD.

*Cons*: Engine logic (threshold filtering, window computation, advisory lock) is business logic, not HTTP logic. Moving it into the handler makes it untestable without an HTTP request context. Directly contradicts the Testability priority and the existing pattern for non-trivial service logic (see `internal/meals/service.go`).

### Decision

**Option A** — Single-port adapter with two interfaces and one `pgxStore` adapter.

*Rationale*: Directly consistent with the established codebase pattern. Maximises testability of `Service` with minimal interface count. Two interfaces cleanly separate the read domain concern from the write results concern. Option B adds interfaces without adding testability or clarity. Option C violates Testability priority.

---

## Decision 2: Scheduler Placement

### Options Considered

**Option A — In-process Scheduler struct in `internal/correlations`**

A `Scheduler` struct in the package wraps `Service.RunForAllUsers` in a `time.NewTicker` loop. Started from `cmd/main.go` via `scheduler.Start(ctx)`. Shares the graceful-shutdown `context.Context` from `signal.NotifyContext`. No external process or library needed.

*Pros*: Zero new dependencies. Consistent with the existing `cmd/main.go` goroutine pattern (HTTP server is already managed via goroutine + context). Simple, auditable, testable with a short interval. Lifecycle tied to binary — no orphaned worker processes.

*Cons*: If the binary crashes, the scheduler stops. Acceptable: the binary already has graceful shutdown and a process supervisor (systemd/container runtime) will restart it. A missed tick does not corrupt data — next tick re-runs and upserts.

**Option B — External job scheduler (e.g., cron job or Kubernetes CronJob)**

Run `correlations run` as a separate invocation via OS cron or Kubernetes CronJob.

*Pros*: Independent restartability; no in-process goroutine.

*Cons*: Requires separate process, separate binary entry point or subcommand, separate deployment config, and external scheduler coordination. The DISCUSS wave decision explicitly chose `CRON_INTERVAL` env var (in-process semantics). This team does not have Kubernetes operational maturity in scope. Significantly more operational complexity for identical reliability at current scale.

**Option C — Database-backed job queue (e.g., `pgqueue` or `river`)**

A PostgreSQL-backed job queue with worker.

*Pros*: Retry semantics, observability, distributed scheduling.

*Cons*: Introduces a new library dependency (OSS, but non-trivial: `river` is Apache 2.0, active). DISCUSS wave decision explicitly noted "DESIGN wave may upgrade to a proper job queue if run volume warrants it" — meaning it does not warrant it now. This is resume-driven development at current scale (few users, 6h interval, single instance).

### Decision

**Option A** — In-process `Scheduler` struct.

*Rationale*: Zero new dependencies. Consistent with existing goroutine lifecycle management in `cmd/main.go`. The advisory lock (pg_try_advisory_xact_lock) already handles concurrent run safety. Options B and C introduce operational or dependency complexity that exceeds the benefit for this team and scale.

---

## Decision 3: Admin Endpoint Authentication

### Options Considered

**Option A — Static shared secret (`ADMIN_SECRET` header)**

A new middleware `requireAdminSecret(secret string)` reads the `X-Admin-Secret` header and compares it against `ADMIN_SECRET` env var (constant-time comparison via `subtle.ConstantTimeCompare`). Returns 401 on mismatch. Applied only to the `/admin/` route group. If `ADMIN_SECRET` is not set, the endpoint is disabled at startup (fatal).

*Pros*: Zero additional dependencies. Consistent with the project's env-var-only configuration policy. Straightforward to rotate by changing the env var and redeploying. Does not require Auth0 M2M client setup. Suitable for a testing/internal endpoint.

*Cons*: Static secret is less sophisticated than M2M token. Secret rotation requires redeployment. Not auditable per-caller (all callers share one secret). Appropriate only for internal/developer use (which this endpoint is).

**Option B — Auth0 Machine-to-Machine (M2M) token**

Require an Auth0 M2M JWT with a specific scope (e.g., `run:correlations`) on the `/admin/` route.

*Pros*: Auditable, rotatable without redeployment, consistent with the Auth0 investment already made.

*Cons*: Requires creating an Auth0 M2M application, adding scope validation to the existing JWT middleware or creating a second middleware, and updating deployment with M2M credentials. Significant Auth0 configuration overhead for an endpoint used only by developers during testing. The DISCUSS wave explicitly flagged this as "TBD for DESIGN wave" with no existing admin role in the codebase.

**Option C — No auth on admin endpoint (IP allowlist via reverse proxy)**

Rely on network-layer controls (reverse proxy allowlist for `/admin/` path).

*Pros*: Zero application code change.

*Cons*: Requires reverse proxy configuration discipline; no protection if deployed without a proxy. Violates defence-in-depth. Explicitly risky if the service is ever exposed directly.

### Decision

**Option A** — Static shared secret via `X-Admin-Secret` header with `subtle.ConstantTimeCompare`.

*Rationale*: Simplest viable option that provides application-level protection for an internal developer endpoint. Option B's Auth0 M2M setup is disproportionate overhead for a testing endpoint. Option C is not acceptable (no defence-in-depth). The `ADMIN_SECRET` env var is fatal-on-missing at startup — the endpoint is not accidentally unprotected. **Open question for user**: if the endpoint needs to be auditable per-caller or called from automated pipelines, escalate to Option B.

---

## Decision 4: SQL Query Shape for Correlation Computation

### Options Considered

**Option A — Single correlated SQL query (recommended)**

One SQL query computes all correlations for a given user in a single database round-trip.

**Join pattern**: `symptom_events` joined to `meal_events` on a time-window predicate (`meal_events.scanned_at` within `LOOKBACK_WINDOW_HOURS` before each `symptom_events.occurred_at`), then joined to `meal_ingredients` to resolve ingredient names.

**Aggregation semantics**: group by `(ingredient_name, symptom_type)`, counting **distinct symptom event IDs** per group — this is the cumulative "how many windows did this ingredient appear in" count decided in DISCUSS wave Decision 2.

**Threshold application**: filter groups in the database (`HAVING count >= CORRELATION_THRESHOLD`) — below-threshold pairs are never returned to the application layer.

**Parameters passed from application**: `user_id` (from JWT context), `LOOKBACK_WINDOW_HOURS` (env config), `CORRELATION_THRESHOLD` (env config).

The exact SQL text (parameterisation, interval casting, pgx argument format) is the software-crafter's responsibility.

*Pros*: Single round-trip per user. All filtering and aggregation in PostgreSQL (optimised query planner). Service code is simple: execute query, iterate result rows, build upsert slice. Threshold filtering in `HAVING` avoids materialising below-threshold rows. Easily testable with known seed data.

*Cons*: More complex SQL to read. Team must be comfortable with correlated joins and HAVING clauses.

**Option B — Application-side loop (fetch symptom events, then query meals per event)**

Fetch all symptom events for user. For each symptom event, execute a second query to find meal ingredients in the window. Accumulate in a Go map. After all events processed, filter by threshold and upsert.

*Pros*: SQL queries are simpler individually (each is a straightforward SELECT).

*Cons*: N+1 query problem — one SQL round-trip per symptom event. A user with 100 symptom events generates 101 DB queries per engine run. At 6h intervals with growing user history, this degrades engine performance. Advisory lock prevents concurrent runs but does not reduce per-user query count. Contradicts the Performance quality attribute and the cross-join risk noted in DISCUSS wave.

**Option C — Materialised intermediate table**

Write intermediate counts into a staging table; query staging table for threshold filtering.

*Pros*: Can be incrementally updated.

*Cons*: Adds a staging table to the schema, a two-phase write, and a cleanup job. DISCUSS wave chose cumulative upsert semantics — a staging table is unnecessary complexity for this. Rejected as over-engineering.

### Decision

**Option A** — Single correlated SQL query.

*Rationale*: Single round-trip, all aggregation in PostgreSQL, threshold applied in `HAVING`. Option B introduces N+1 queries which degrade monotonically with user history length. Option C adds schema and lifecycle complexity with no quality-attribute benefit for this scope.

---

## Consequences

### Positive
- Zero new external dependencies
- `Service` is fully unit-testable without a database
- Pattern consistent with existing packages — new contributors can orient quickly
- Admin endpoint protected without Auth0 configuration changes
- Single SQL query per user keeps engine run time bounded at DB level

### Negative
- `ADMIN_SECRET` rotation requires redeployment (accepted; internal testing endpoint)
- Single SQL query is more complex than application-side loop — requires SQL competency in the team
- `pgxStore` implements two interfaces — tests must either stub both or use a single fake struct that implements both (minor)
- Monotonically increasing `correlation_count` means re-running with a shorter window does not reduce counts (DISCUSS decision — accepted risk)

### Risks
- If `ADMIN_SECRET` leaks, any caller can trigger arbitrary user correlation runs (mitigated by rate of damage: run is idempotent and read-only of source data)
- Advisory lock uses `hashtext(user_id)` — hash collision between two distinct user_ids would cause one to skip; collision probability is 1/(2^31) per pair — negligible at this scale
- `SELECT DISTINCT user_id FROM symptom_events` for eligible users query is a full-table scan without an index on `symptom_events(user_id)` — migration 005 should add this index (see Open Question OQ-3 in brief.md)
