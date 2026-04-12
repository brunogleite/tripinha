---
name: prd
description: >
  Creates a PRD through structured user interview, codebase exploration, and module
  design, then submits it as a GitHub issue. Use when user says "write a PRD",
  "create a product requirements document", "plan a new feature", or invokes /prd.
---

# PRD

Four phases: interview → codebase exploration → module design → GitHub issue.
Don't skip phases. Don't write the PRD until the interview is complete.

---

## Phase 1 — Interview

One question at a time. Cover these branches in order — each must be resolved before writing:

1. **Problem** — What pain does this solve? For whom? Why now?
2. **Success metric** — How do you measure this worked? Name a specific, observable outcome.
3. **Scope** — What is explicitly *out of scope*? Force this answer — "we'll see" is not accepted.
4. **Constraints** — Deadline, tech stack limits, compliance, dependencies on other teams.
5. **Alternatives considered** — Why not X? Forces the user to defend the approach.

Rules (same as grill-me):
- One question at a time.
- Lead with your recommendation, then ask if they agree.
- "We'll handle that later" = unresolved branch. Push back.
- Mark `✓ resolved` when a branch closes.

---

## Phase 2 — Codebase Exploration

After interview is complete, explore the repo silently. Don't ask the user what already exists — go look.

Find:
- Existing modules this feature touches
- Current data models / schema relevant to the feature
- API surface (routes, handlers) that need to change or be added
- Tests that will need updating
- Any existing partial implementation or related code

Report findings as a short "Codebase context" block before writing the PRD. Flag anything surprising.

---

## Phase 3 — Module Design

Based on interview + codebase context, design:

| Module | Action | Notes |
|--------|--------|-------|
| `existing/module` | modify | what changes |
| `new/module` | create | what it owns |

Define:
- **New API endpoints** — method, path, request/response shape
- **Data model changes** — new tables/columns, index changes, migrations needed
- **Inter-module dependencies** — what calls what

Keep it minimal. Don't design for hypothetical future requirements.

---

## Phase 4 — PRD Output + GitHub Issue

Write the PRD in this structure:

```markdown
## Problem

[1–3 sentences. What breaks or hurts without this feature, and for whom.]

## Goals

- [ ] [Measurable outcome 1]
- [ ] [Measurable outcome 2]

## Non-Goals

- [Explicitly out of scope item]

## User Stories

- As a [user], I want [action] so that [outcome].

## Technical Design

### Affected Modules
[From Phase 3 module design table]

### API Changes
[Endpoints added/modified]

### Data Model Changes
[Schema changes, migrations]

## Acceptance Criteria

- [ ] [Specific, testable condition]

## Open Questions

- [Unresolved item with owner if known]
```

Then submit as a GitHub issue:
- Use `gh issue create` with the PRD as the body
- Title = feature name, concise (≤60 chars)
- Add label `enhancement` (and `prd` if that label exists in the repo)
- Print the issue URL when done

---

## Boundaries

- Don't write implementation code. Design only.
- Don't create files outside of the GitHub issue.
- If repo has no GitHub remote or `gh` is not authenticated, output the PRD as markdown and tell the user to submit manually.
- "stop prd" or "normal mode": exit, revert to regular assistant behavior.
- If user hasn't described a feature yet: ask "What are we building?" and wait.
