# Phase 8 — Decisions Log

> **Status:** scaffolded — no decisions adopted yet.
>
> This file is a placeholder. Decision IDs for Phase 8 will be allocated
> contiguously **starting immediately after Phase 7's last decision ID**,
> which is itself unknown until Phase 7 closes its range. Phase 8 cannot
> pre-allocate its first decision ID.
>
> Until then, this file exists only to make the phase directory
> recognisable to the `roadmap-status` skill and to hold the decision
> range reservation.

## Decision Range Reservation

- **First decision ID:** `TBD` — set to `D(Phase7.last + 1)` when
  Phase 7 reaches `approved`.
- **Last decision ID:** `TBD` — set when the phase reaches `under-review`.
- **Contiguity rule:** Phase 8 owns a contiguous range that begins
  immediately after Phase 7's range.
- **Ordering rule:** Phase 8 must not adopt any decision while Phase 7
  is still `in-progress` or `under-review`, because several Phase 8
  decisions depend on Phase 7 outputs (namespacing convention,
  credential flow, error taxonomy mapping).

## Adopted Decisions

*None yet.*

## Amendment Protocol

Once decisions are adopted, amendments follow the protocol recorded in
`docs/phase-1-api-scope/01-decisions-log.md#amendment-protocol` — the
same protocol used by all Phase 1–6 decisions.
