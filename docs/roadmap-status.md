# Roadmap Status

**Last updated:** 2026-04-05
**Target:** `praxis v1.0.0` — stable public Go API for enterprise agent orchestration

## Phase Status

| # | Phase | Status | Artifacts |
|---|-------|--------|-----------|
| 0 | Seed Context | starting baseline (amendable via decision-log amendment) | 1 (`docs/PRAXIS-SEED-CONTEXT.md`) |
| 1 | API Scope and Positioning | **approved** | 7 (`00-plan.md`, `01-decisions-log.md`, `02-positioning-and-principles.md`, `03-non-goals.md`, `04-v1-freeze-surface.md`, `05-seed-question-resolutions.md`, `REVIEW.md`) |
| 2 | Core Runtime Design | not-started | — |
| 3 | Interface Contracts | not-started | — |
| 4 | Observability and Error Model | not-started | — |
| 5 | Security and Trust Boundaries | not-started | — |
| 6 | Release, Versioning and Community Governance | not-started | — |

## Adopted Decisions

**14 of 14 allocated Phase 1 decisions adopted.** Phase 1 owns the D01–D15
range; D15 was held as a reviewer reserve and released unused.

- **Phase 1 (approved):** D01–D14 adopted as working positions. D15 released.
- **Phase 2–6:** no decisions allocated yet. Next range opens at D15.

Adopted decisions are working positions, **amendable in later phases** when
new evidence, downstream constraints, or external changes justify revisiting
them. See the Amendment protocol in
[`docs/phase-1-api-scope/01-decisions-log.md`](phase-1-api-scope/01-decisions-log.md#amendment-protocol)
for how amendments are recorded.

## Completed Work

**Phase 1 — API Scope and Positioning (approved):**

- **D01** — README-ready positioning statement adopted: praxis is an
  invocation kernel for a single LLM agent call with enterprise guardrails
  built in by construction.
- **D02** — Final design principles adopted: eight principles, seven
  carried from seed §3 plus a new **zero-wiring smoke path** principle.
- **D03** — Target consumer archetype (platform / ML infra teams with
  LLM already in production needing audit/cost/security by construction)
  and three explicit anti-personas adopted.
- **D04** — v1.0 freeze surface adopted: **12 of 14 interfaces at
  `frozen-v1.0`**, 2 at `stable-v0.x-candidate` (`budget.PriceProvider`
  gated on D08, `identity.Signer` gated on Phase 5). `frozen-v1.0` is a
  semver-level commitment to downstream consumers; it is separate from
  the amendability of Phase 1's methodological decisions.
- **D05** — Seven explicit v1.x non-goals catalogued with the
  "interface that would exist if reversed" anchor for each.
- **D06** — `tools.Invoker` tool-name placement adopted: name lives on
  `ToolCall`, not `InvocationContext` (resolves seed §13.1).
- **D07** — `requires_approval` semantics adopted: orchestrator returns a
  structured `ApprovalRequiredError` and treats the condition as
  terminal; caller owns persistence and resume (resolves seed §13.2).
  Amends the seed §5 error taxonomy to eight concrete types.
- **D08** — `budget.PriceProvider` hot-reload policy adopted:
  per-invocation snapshot, no live re-read, no mid-invocation re-pricing
  (resolves seed §13.3).
- **D09** — "No plugins in v1" re-confirmed with recorded rationale
  (resolves seed §13.4).
- **D10** — Name `praxis` / module `github.com/praxis-go/praxis` adopted
  conditionally, with a Phase 3 tripwire: preconditions (GitHub org
  acquisition and brand review vs `usepraxis.app`) must be resolved
  before Phase 3 embeds the module path into godoc, or Phase 3 treats
  the path as `MODULE_PATH_TBD`.
- **D11** — Seven positioning gaps praxis will not close relative to
  LangChainGo, Google ADK for Go, Eino, and direct SDK use catalogued.
- **D12** — Zero-wiring smoke-test promise adopted: `AgentOrchestrator`
  must be constructible with `llm.Provider` as the single required
  dependency, all other dependencies defaulting to null/noop
  implementations.
- **D13** — Three-tier interface stability policy adopted
  (`frozen-v1.0`, `stable-v0.x-candidate`, `post-v1`).
- **D14** — Azure OpenAI parity is best-effort (tested, not v1.0
  guaranteed); compatibility matrix owned by the `openai/` adapter.

All four seed §13 open questions have an adopted working resolution.
Decoupling contract grep is clean across all Phase 1 artifacts.
`REVIEW.md` verdict: **READY**.

## Open Decisions

No open decisions in Phase 1. All decisions from Phase 2 onwards remain
unallocated:

- **Phase 2 (Core Runtime Design) — open.** Must allocate: state
  machine reconciliation (seed's 11 vs 13 vs +1 for `ApprovalRequired`),
  terminal-state representation for `ApprovalRequired`, streaming
  channel contract details, cancellation soft-vs-hard semantics on
  approval terminals, budget-clock interaction with D07, zero-wiring
  streaming path behavior (D12 boundary).
- **Phase 3 (Interface Contracts) — open.** Must allocate: final method
  signatures for all 14 interfaces, `tools.Invoker` method name and
  `InvocationContext` field set, `ApprovalRequiredError` concrete field
  set, `budget.PriceProvider` signature confirmation and promotion, Azure
  OpenAI compatibility matrix ownership, D10 module-path resolution
  before any godoc is written.
- **Phase 4 (Observability and Error Model) — open.** Must allocate:
  OTel span tree, Prometheus metric set and cardinality constraints,
  slog redaction rules, error-to-event mapping, error taxonomy updated
  to eight concrete types per D07.
- **Phase 5 (Security and Trust) — open.** Must allocate: credential
  zero-on-close mechanics, `identity.Signer` JWT claim set and key
  lifecycle, promotion of `identity.Signer` from candidate to
  `frozen-v1.0`.
- **Phase 6 (Release and Governance) — open.** Must allocate: semver
  deprecation windows, release-please configuration, CI pipeline
  including banned-identifier grep, bus-factor mitigation (seed §14.1),
  D10 preconditions verification or execution of rename before v0.1.0.

## Risks / Blockers

- **D10 external dependency.** Name/module-path resolution is decided
  conditional and cannot be fully resolved inside a design phase. Phase
  3 tripwire mitigates godoc exposure, but v0.1.0 tag is gated on
  resolution. Largest forward exposure in the current plan.
- **State-count discrepancy in seed.** Seed §1/§8 ("11-state machine")
  and seed §4.2 (13 numbered states) are not self-consistent. Phase 2
  must reconcile and D07 will add `ApprovalRequired`. Flagged as a
  Phase 2 exit criterion.
- **Soft freeze promotions.** `budget.PriceProvider` (Phase 3) and
  `identity.Signer` (Phase 5) promotions to `frozen-v1.0` are soft
  commitments. Phase 6 release must hard-gate both before v0.5.0 tag.
- **Bus factor (seed §14.1).** Early maintainer set is small. Not
  addressed in Phase 1 artifacts (correctly out of scope); flagged for
  Phase 6.
- **First production consumer dependency (seed §8 v1.0.0 criterion).**
  v1.0.0 tag is gated on a production consumer shipping against v0.5.x.
  Go-to-market question outside design phase scope; flagged for Phase 6.

## Decoupling Contract Health

**PASS.** A case-insensitive word-bounded grep against the seed §6.1
banned-identifier set returns zero matches across all Phase 1 artifacts.
Verified twice: once by the reviewer subagent, once independently in the
`review-phase` pass. A false-positive substring collision from non-goal
heading numbering was caught pre-review and fixed by renaming non-goal
headings to `Non-goal N`. The literal banned-identifier pattern is not
embedded in this document; the authoritative list lives in seed §6.1.

The decoupling contract is a correctness invariant, not an amendable
decision — it is not subject to the decision-log amendment protocol.

## Next Step

Invoke `plan-phase` on **Phase 2 — Core Runtime Design**. The plan should
claim as exit criteria: (a) state-count reconciliation, (b) terminal
representation for `ApprovalRequired`, (c) streaming channel contract,
(d) cancellation semantics on approval terminals, (e) zero-wiring
streaming boundary behavior.

## Overall Status

Planning is on track: Phase 1 of 6 is approved with a clean charter and a
clean decoupling contract; five design phases remain before implementation
begins. **v0.1.0** (first working invocation) requires Phases 2–3
complete plus D10 resolution. **v0.5.0** (feature complete) requires all
six phases approved plus both `stable-v0.x-candidate` interfaces promoted
to `frozen-v1.0`. **v1.0.0** (API freeze) further requires a production
consumer to ship against a v0.5.x tag — a dependency outside any design
phase. Adopted decisions in every phase remain amendable via the protocol
recorded in the relevant decision log.
