# Outcome KPIs: Correlation Engine

## Feature: correlation-engine

### Objective

By end of Q2 2026, IBS users who have logged at least 2 weeks of meal and symptom data can identify at least one dietary trigger candidate — enabling them to make a confident, evidence-based dietary adjustment without relying solely on elimination diets.

### Outcome KPIs

| # | Who | Does What | By How Much | Baseline | Measured By | Type |
|---|---|---|---|---|---|---|
| 1 | IBS users with ≥5 logged symptom events + meal data | Retrieve at least one trigger candidate via GET /triggers | >0 candidates returned for 80% of eligible users after first full engine run | 0% (feature does not exist) | SQL: `SELECT COUNT(DISTINCT user_id) FROM trigger_candidates` / eligible user count | Leading |
| 2 | IBS users with active trigger candidates | Return to the app within 7 days to view updated triggers | 40% week-1 return rate for users who have seen at least one trigger | Unknown | Analytics: GET /triggers call frequency per user_id, cohort by first-trigger-seen date | Leading |
| 3 | IBS users | Report dietary adjustments based on trigger candidates (survey) | 30% of users with trigger candidates report removing or reducing a flagged ingredient within 4 weeks | Unknown | In-app survey or follow-up email (4-week post-feature cohort) | Lagging |
| 4 | Correlation engine service | Complete a full correlation pass for all users within 1 scheduler interval | 100% of users processed per tick, 0 skipped due to errors | N/A | Service logs: per-tick completion rate | Leading |

### Metric Hierarchy

- **North Star**: Percentage of eligible IBS users with at least one trigger candidate (KPI #1)
- **Leading Indicators**: Engine runs successfully on schedule (KPI #4); users return to check triggers (KPI #2)
- **Guardrail Metrics**: GET /triggers p99 latency < 500ms; zero cross-user data leakage (confirmed by audit); DB error rate during engine runs < 1%

### Measurement Plan

| KPI | Data Source | Collection Method | Frequency | Owner |
|---|---|---|---|---|
| #1: Eligible users with ≥1 candidate | trigger_candidates table | SQL query: distinct user_id count | Weekly | Engineering |
| #2: 7-day return rate | API access logs | COUNT(GET /triggers) per user_id grouped by cohort | Weekly | Engineering |
| #3: Dietary adjustments | User survey | 4-week post-release survey (email or in-app) | Once (4-week mark) | Product |
| #4: Engine run completeness | Service logs | Log lines: "tick complete: N users processed, M skipped" | Per run (every 6h) | Engineering |

### Hypothesis

We believe that surfacing statistically correlated ingredient-symptom candidates for IBS users with sufficient history will achieve a 40% week-1 return rate and 30% reported dietary adjustments within 4 weeks. We will know this is true when 80% of eligible users receive at least one trigger candidate after their first engine run.

### Baseline Measurement Requirements

Before release:
1. Count users with ≥5 symptom events AND corresponding meal data (eligible population baseline)
2. Establish current GET /triggers latency baseline (endpoint does not exist — start measuring from day 1)
3. No prior dietary-adjustment survey data — Q2 2026 cohort is the first measurement point
