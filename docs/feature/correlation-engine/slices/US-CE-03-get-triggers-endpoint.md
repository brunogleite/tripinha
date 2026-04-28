# US-CE-03: GET /triggers Endpoint

## Elevator Pitch

- **Before**: Correlation results exist in the database but Sofia has no way to retrieve them — the `trigger_candidates` table is write-only from her perspective.
- **After**: `GET /triggers` returns Sofia's ingredient-symptom correlations as JSON, scoped to her user_id from her Auth0 JWT.
- **Decision enabled**: Sofia can see which specific ingredients are statistically associated with each of her symptom types and decide which ones to eliminate or reduce in her diet.

## Problem

Sofia Andrade has been managing IBS through trial and error for two years. Even with meal and symptom logging in place, she has no way to retrieve the patterns the engine has discovered. The `GET /triggers` endpoint is the bridge between background computation and the user's daily decision-making.

## Who

- **User type**: IBS user (Sofia Andrade) — authenticated via Auth0 JWT
- **Context**: Brownfield Go + chi router API; pattern established by `POST /meals` and `POST /symptoms`; consent middleware already wired
- **Motivation**: Sofia wants a simple, reliable endpoint she can call from any client to see her current trigger candidates without re-running the engine herself

## Solution

Implement `GET /triggers` handler in a new `internal/correlation` package (or `internal/triggers`). Handler reads `trigger_candidates` scoped to the authenticated `user_id` and returns JSON. Follows the existing handler pattern from `internal/meals/handler.go`.

## Domain Examples

### 1: Happy Path — Sofia has candidates after engine runs

Sofia calls `GET /triggers`. Her `trigger_candidates` table has two rows: fructose/bloating (count=7) and onion/cramping (count=6). Response:
```json
{
  "triggers": [
    {"ingredient": "fructose", "symptom_type": "bloating", "correlation_count": 7, "window_hours": 24, "last_evaluated_at": "2026-04-28T10:00:00Z"},
    {"ingredient": "onion", "symptom_type": "cramping", "correlation_count": 6, "window_hours": 24, "last_evaluated_at": "2026-04-28T10:00:00Z"}
  ]
}
```

### 2: Edge Case — Engine has not run yet

Sofia just registered and logged her first week of data. The engine has not completed a run for her user. Response:
```json
{"triggers": []}
```
HTTP 200 — not 404. An empty result is not an error.

### 3: Error Path — DB unavailable

The PostgreSQL server is temporarily unreachable. Response: HTTP 503 with a `Retry-After: 30` header. No stack trace exposed.

### 4: Security Boundary — User can only see their own triggers

Marco is also a user. His `trigger_candidates` rows exist. When Sofia calls `GET /triggers`, Marco's rows are never returned. The query is always `WHERE user_id = $1` using the JWT-derived user_id.

## UAT Scenarios (BDD)

### Scenario: Authenticated user retrieves their trigger candidates

```gherkin
Given Sofia is authenticated (valid Auth0 JWT)
And trigger_candidates contains rows for Sofia: (fructose, bloating, count=7) and (onion, cramping, count=6)
When Sofia calls GET /triggers
Then the response is HTTP 200
And the body contains both fructose/bloating and onion/cramping entries
And each entry includes correlation_count and window_hours
```

### Scenario: No candidates yet returns empty list

```gherkin
Given Sofia is authenticated
And trigger_candidates has no rows for Sofia
When Sofia calls GET /triggers
Then the response is HTTP 200
And the body is {"triggers": []}
```

### Scenario: Triggers are user-scoped

```gherkin
Given Sofia is authenticated as user_id "auth0|sofia"
And Marco has trigger_candidates rows in the table (user_id "auth0|marco")
When Sofia calls GET /triggers
Then the response contains only Sofia's triggers
And Marco's triggers do not appear
```

### Scenario: Unauthenticated request is rejected

```gherkin
Given a request to GET /triggers with no Authorization header
When the request is received
Then the response is HTTP 401
```

### Scenario: Database unavailable returns retriable error

```gherkin
Given the database is unreachable
When Sofia calls GET /triggers
Then the response is HTTP 503
And the response includes a Retry-After header
```

## Acceptance Criteria

- [ ] Route `GET /triggers` is registered in `cmd/main.go` under the auth middleware group (consent required)
- [ ] Handler reads `trigger_candidates` filtered by the JWT-derived `user_id`
- [ ] Response is HTTP 200 with `Content-Type: application/json`
- [ ] Response body has shape `{"triggers": [...]}` — array is present even when empty (never null)
- [ ] Each trigger entry includes: `ingredient`, `symptom_type`, `correlation_count`, `window_hours`, `last_evaluated_at`
- [ ] No cross-user data leakage — query always includes `WHERE user_id = $1`
- [ ] Missing or invalid JWT returns HTTP 401 (handled by existing auth middleware)
- [ ] DB error returns HTTP 503 with `Retry-After: 30` header
- [ ] Handler follows the interface pattern from `internal/meals` (Store interface injected, not hardcoded)
- [ ] At least one integration test covers the empty-result case (HTTP 200 + `{"triggers": []}`)

## Outcome KPIs

- **Who**: Authenticated IBS users with at least one completed engine run
- **Does what**: Successfully retrieve their trigger candidate list in a single API call
- **By how much**: 100% of calls for users with existing candidates return the correct scoped list (zero cross-user contamination in any test run)
- **Measured by**: Integration tests with two distinct user_ids; production audit log monitoring for unexpected cross-user query patterns
- **Baseline**: Endpoint does not exist today

## Technical Notes

- Route registration: add `r.Get("/triggers", triggerHandler.Get)` inside the `consent.RequireConsent` group in `cmd/main.go` — consistent with `/meals` and `/symptoms`
- Empty array must be `[]` not `null` in JSON — use `[]TriggerCandidate{}` not `nil` slice when no results
- `last_evaluated_at` should be returned in RFC 3339 format (UTC) — consistent with `scanned_at` in meal events
- Dependencies: US-CE-01 (table schema), US-CE-02 (engine produces rows to read)
- No pagination required in this slice — deferred. If user has >100 triggers, all are returned. Flag as a tech debt item for DESIGN wave if needed.
