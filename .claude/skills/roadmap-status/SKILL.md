---
name: roadmap-status
description: >
  Track planning progress across the 6 praxis design phases and produce a compact status
  snapshot. Use after completing a phase step, after a review, or when checking overall
  planning readiness for v1.0. Shows current phase, completed work, open decisions,
  blockers, next step, and overall status.
---

# Roadmap Status

Produce a compact status snapshot of praxis library planning progress.

## Input

Read the current state of planning artifacts in the repository. If a `docs/roadmap-status.md`
file exists, update it. Otherwise, create it.

## Phase Detection

Determine each phase's status by scanning the repository for artifacts.

### Artifact Locations

Each phase stores artifacts in `docs/phase-<N>-<slug>/`:

| # | Phase | Directory |
|---|-------|-----------|
| 0 | Seed Context (locked at extraction) | `docs/PRAXIS-SEED-CONTEXT.md` (single file, not a phase dir) |
| 1 | API Scope and Positioning | `docs/phase-1-api-scope/` |
| 2 | Core Runtime Design | `docs/phase-2-core-runtime/` |
| 3 | Interface Contracts | `docs/phase-3-interface-contracts/` |
| 4 | Observability and Error Model | `docs/phase-4-observability-errors/` |
| 5 | Security and Trust Boundaries | `docs/phase-5-security-trust/` |
| 6 | Release, Versioning and Community Governance | `docs/phase-6-release-governance/` |

All phase output files use a two-digit numbered prefix (e.g., `01-`, `02-`) to enforce
reading order. `REVIEW.md` is the only unnumbered file.

### State Detection Rules

Scan the phase directory and apply these rules in order:

1. **`approved`** — a `REVIEW.md` file exists in the phase directory with verdict
   `READY` or `PASS`
2. **`under-review`** — a `REVIEW.md` file exists with verdict `NOT READY` or `FAIL`,
   or the review-phase skill has been invoked but no READY verdict yet
3. **`in-progress`** — the phase directory exists and contains at least one artifact
   file (excluding REVIEW.md)
4. **`blocked`** — the phase directory contains a `BLOCKED.md` file describing the
   blocker
5. **`not-started`** — the phase directory does not exist or is empty

When a phase is `in-progress`, also assess **completeness** by counting numbered
artifacts present. Report as `in-progress (N artifacts)`.

## Phase List and State Model

Phases (planning):
1. API Scope and Positioning
2. Core Runtime Design
3. Interface Contracts
4. Observability and Error Model
5. Security and Trust Boundaries
6. Release, Versioning and Community Governance

Valid states: `not-started` | `in-progress` | `under-review` | `blocked` | `approved`

Phase 0 (Seed Context) is always `locked` once `docs/PRAXIS-SEED-CONTEXT.md` exists. It
is not a working phase — it is the frozen baseline that all phases refine.

## Output Structure

Use this exact structure:

```
# Roadmap Status

**Last updated:** <date>
**Target:** `praxis v1.0.0` — stable public Go API for enterprise agent orchestration

## Phase Status

| # | Phase | Status | Artifacts |
|---|-------|--------|-----------|
| 0 | Seed Context | locked | 1 (docs/PRAXIS-SEED-CONTEXT.md) |
| 1 | API Scope and Positioning | <status> | <count or —> |
| 2 | Core Runtime Design | <status> | <count or —> |
| 3 | Interface Contracts | <status> | <count or —> |
| 4 | Observability and Error Model | <status> | <count or —> |
| 5 | Security and Trust Boundaries | <status> | <count or —> |
| 6 | Release, Versioning and Community Governance | <status> | <count or —> |

## Locked Decisions
Count of decisions locked across all phases. Reference the decision ID ranges per phase
(e.g., Phase 1: D01–D06 locked).

## Completed Work
Bullet list of what has been decided and delivered so far, phase by phase.

## Open Decisions
Bullet list of decisions that remain unresolved, tagged by phase.

## Risks / Blockers
Bullet list. Flag anything that prevents forward progress toward v1.0.

## Decoupling Contract Health
One sentence on whether phase artifacts currently pass the banned-identifier grep.
If any phase has violations, list them.

## Next Step
One sentence: what should happen next and which skill/subagent to invoke.

## Overall Status
One sentence assessment of planning readiness for v0.1.0 (first working invocation),
v0.5.0 (feature complete), and v1.0.0 (API freeze).
```

## Output File

Write the status snapshot to `docs/roadmap-status.md`. If the file already exists,
overwrite it. After writing the file, print a one-line summary to the conversation:
the current phase and overall status.

## Guardrails

- Do not generate a fake sense of progress.
- Do not mark a phase as `approved` if core questions remain open or the decoupling
  contract check is failing.
- Do not blur planning status with implementation status. Implementation begins only
  after Phase 6 is approved.
- Keep the snapshot compact and scannable.
- Always scan the filesystem — never rely on memory or assumptions about what exists.
- Cross-reference `docs/PRAXIS-SEED-CONTEXT.md` for the frozen baseline. If any phase
  artifact contradicts the seed context without an explicit amendment, flag it under
  Risks / Blockers.
