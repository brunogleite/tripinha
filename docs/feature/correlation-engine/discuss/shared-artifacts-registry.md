# Shared Artifacts Registry: Correlation Engine

## Purpose

Every variable that appears in more than one step of the correlation engine journey must have a single source of truth. This registry documents that source and all consumers. Untracked artifacts are the primary cause of integration failures.

---

## Registry

### CORRELATION_THRESHOLD

| Field | Value |
|---|---|
| Source of truth | Environment variable `CORRELATION_THRESHOLD` |
| Default | `5` |
| Type | Integer, minimum value: 1 |
| Consumers | Engine threshold comparison; implicit in trigger_candidates (only rows meeting threshold are written) |
| Owner | correlation engine service |
| Integration risk | HIGH — if accidentally set to 0, every ingredient becomes a candidate; guardrail: reject values < 1 at startup |
| Validation | Engine logs effective threshold value at startup |

### LOOKBACK_WINDOW_HOURS

| Field | Value |
|---|---|
| Source of truth | Environment variable `LOOKBACK_WINDOW_HOURS` |
| Default | `24` |
| Type | Integer (hours), minimum value: 1 |
| Consumers | Engine window query (SQL interval); `trigger_candidates.window_hours` column; GET /triggers response body |
| Owner | correlation engine service |
| Integration risk | HIGH — window stored in trigger_candidates must match window used in query; if env changes between runs, stored rows reflect old window |
| Validation | trigger_candidates.window_hours must equal env value at time of write |

### CRON_INTERVAL

| Field | Value |
|---|---|
| Source of truth | Environment variable `CRON_INTERVAL` |
| Default | `6h` |
| Type | Duration string (Go format: "6h", "1h30m") |
| Consumers | Background scheduler tick |
| Owner | scheduler (within correlation engine service) |
| Integration risk | LOW — only affects frequency, not correctness |
| Validation | Engine logs effective interval at startup |

### user_id

| Field | Value |
|---|---|
| Source of truth | Auth0 JWT, decoded by `internal/auth` middleware |
| Consumers | meal_events.user_id; symptom_events.user_id; trigger_candidates.user_id; all engine queries (WHERE user_id = ?); GET /triggers handler (scopes response) |
| Owner | auth middleware |
| Integration risk | CRITICAL — if user_id is not consistently applied, cross-user data leakage occurs; also produces empty results for mismatched IDs |
| Validation | All SQL queries include explicit user_id filter; tests must verify no cross-user rows returned |

### symptom_type

| Field | Value |
|---|---|
| Source of truth | `symptom_events.type` column (free text, de-facto vocabulary: "bloating", "cramping", "nausea", "pain") |
| Consumers | Engine per-type grouping loop; trigger_candidates.symptom_type; GET /triggers response |
| Owner | symptoms service (upstream) |
| Integration risk | MEDIUM — if symptom_events.type contains typos or variants ("bloat" vs "bloating"), per-type isolation breaks and counts are split |
| Validation | No validation enforced at engine layer in this slice; flagged as tech note for DESIGN wave |

### trigger_candidates (table)

| Field | Value |
|---|---|
| Source of truth | trigger_candidates table (written by correlation engine in step 4) |
| Consumers | GET /triggers handler (reads and returns); future notification service (deferred) |
| Owner | correlation engine |
| Integration risk | MEDIUM — GET /triggers must handle empty table as valid state (200 + []), not as error |
| Validation | GET /triggers integration test covers empty-table scenario |

---

## Integration Checkpoints

1. `LOOKBACK_WINDOW_HOURS` env var must be read once at engine startup and passed through consistently — never re-read mid-run
2. All SQL queries must include `WHERE user_id = $1` — no query touches all-user data
3. `trigger_candidates` UPSERT key: `(user_id, ingredient_name, symptom_type)` — prevents duplicates on re-run
4. GET /triggers handler must not JOIN trigger_candidates with any other table to avoid accidental cross-user exposure
