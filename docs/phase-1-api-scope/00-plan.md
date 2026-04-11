# Phase: API Scope and Positioning

**Status:** in-progress
**Phase directory:** `docs/phase-1-api-scope/`
**Starting baseline:** `docs/PRAXIS-SEED-CONTEXT.md` (amendable via the
decision-log amendment protocol in a phase that discovers the need; the
seed file itself is not edited directly)

## Goal

Adopt a working charter for the scope, positioning, design principles, and
v1.0 commitment boundary of `praxis` so that every subsequent design phase
(Core Runtime, Interface Contracts, Observability, Security, Release) has
an unambiguous reference point to refine. Phase 1 decisions are adopted
working positions, not immutable; later phases may amend them with
justification per the amendment protocol in
[`01-decisions-log.md`](01-decisions-log.md#amendment-protocol).

## Scope

**In scope** — refines `PRAXIS-SEED-CONTEXT.md` §1 (Vision), §2 (Positioning),
§3 (Design principles), §5 (interface surface at the naming level only), §13 (Open
questions for phase 1):

- Final one-paragraph positioning statement of what praxis *is* and what it is *not*.
- Confirmed list of design principles (7 in seed) — kept, reworded, or cut.
- Target-consumer profile: the archetype team that should reach for praxis, and the
  teams that should not.
- The v1.0 commitment boundary: which interfaces in §5 are part of the v1.0 freeze,
  which ship as "stable but not frozen" in v0.x, and which are explicitly post-v1.
- Explicit non-goals list (things praxis will never be in v1.x).
- Resolution of the four seed open questions (seed §13):
  1. `tools.Invoker` final method shape and where the tool name lives.
  2. `requires_approval` hook-result semantics — in-process stall vs caller-deferred.
  3. `budget.Guard` / `PriceProvider` hot-reload policy.
  4. Re-confirmation of "no plugins in v1".
- Positioning table vs LangChainGo, Google ADK for Go, Eino, direct SDKs — refined
  into decisions about which gaps praxis will not try to close.
- Module path and project name confirmation (seed risk §14.3: name collision check).

**Out of scope** — deferred to later phases:

- Concrete method signatures and struct fields for any interface (→ Phase 3).
- State machine transitions and lifecycle event names (→ Phase 2, Phase 4).
- Error taxonomy surface and retry classification logic (→ Phase 4).
- OTel span shape, metric names, slog redaction rules (→ Phase 4).
- Credential zeroing, Ed25519 key lifecycle, identity JWT shape (→ Phase 5).
- Semver deprecation windows, release-please config, CI grep rules (→ Phase 6).
- Any implementation code.

## Key Questions

1. Is the seed §1 vision statement ("enterprise guardrails built in") the right
   hook for the v1.0 README, or does it need to be sharpened to avoid sounding
   like a governance platform rather than a library?
2. Which of the 7 seed design principles survive verbatim, which need rewording,
   and is there a missing principle (e.g. "zero-config smoke test path")?
3. Who is the target consumer, concretely? Platform teams inside enterprises?
   Agent-product startups past prototyping? ML infra groups? A precise archetype
   determines API ergonomic trade-offs in Phase 3.
4. What is the anti-persona — teams for whom praxis would be the wrong choice,
   stated plainly so the README can redirect them (e.g. "if you want a
   Python-equivalent LangChain experience, use LangChainGo")?
5. Are all 14 interfaces listed in seed §5 part of the v1.0 freeze, or are any
   ("stable but not frozen") — e.g. `identity.Signer`, `budget.PriceProvider`
   hot-reload surface?
6. For `tools.Invoker`: does the tool name live on the `ToolCall` struct or on
   `InvocationContext`? (Seed §13.1.) The answer affects how multi-tool-call
   parallel invocation is represented.
7. For `requires_approval`: does the orchestrator stall in-process (requiring a
   new `ApprovalPending` state and a streaming event) or does it return a
   structured "approval needed" decision and delegate the stall to the caller?
   (Seed §13.2.) This decision propagates into Phase 2 state machine design.
8. For budget pricing hot-reload: is `PriceProvider` consulted on every token
   accounting call (allowing live updates), or resolved once at invocation start
   (stable for the life of that invocation)? If live, are in-flight invocations
   re-priced? (Seed §13.3.)
9. Is "no plugins in v1" re-confirmed, and is the rationale recorded in a
   decision that future RFCs can point at? (Seed §13.4.)
10. Is the module path `github.com/praxis-os/praxis` confirmed available on
    GitHub, pkg.go.dev, and clean from trademark conflict? (Seed risk §14.3.)
11. Are there *any* non-goals not already explicit in the seed — for example:
    no built-in HTTP handler, no multi-agent orchestration, no prompt template
    engine, no vector store interface, no memory/RAG primitives?

## Decisions Required

Phase 1 owns the D01–D15 range. Sequential allocation; reviewer may compress.

- **D01** — Final positioning statement (one paragraph, README-ready).
- **D02** — Final design principles list (count, wording, ordering).
- **D03** — Target consumer archetype and explicit anti-persona.
- **D04** — Set of interfaces covered by the v1.0 freeze commitment (subset of
  seed §5, or all 14).
- **D05** — Non-goals list for v1.x (things praxis will never ship).
- **D06** — `tools.Invoker` tool-name placement: on `ToolCall` or
  `InvocationContext`. (Resolves seed §13.1.)
- **D07** — `requires_approval` semantics: in-process stall vs caller-deferred.
  (Resolves seed §13.2.)
- **D08** — `budget.PriceProvider` hot-reload policy: per-invocation snapshot
  vs live lookup. (Resolves seed §13.3.)
- **D09** — "No plugins in v1" re-confirmation, with recorded rationale.
  (Resolves seed §13.4.)
- **D10** — Module path and project name confirmation (or rename).
- **D11** — Positioning gaps that praxis explicitly will *not* close relative
  to LangChainGo, ADK, Eino, and direct SDKs.
- **D12** — Smoke-test promise: can an `Orchestrator` be constructed and used
  with zero caller-supplied wiring (all null defaults)? Yes/no is a design
  principle and drives Phase 3 constructor shape.
- **D13** — Interface stability tiering: "frozen v1.0" vs "stable v0.x,
  candidate for v1.0", for any interface where the seed is ambiguous.
- **D14** — Whether `openai.Provider` covering Azure OpenAI is a v1.0 promise
  or a best-effort ("we test it, we don't guarantee parity").
- **D15** — *Reserve.* Left unallocated for a reviewer-introduced decision;
  renumber if unused at phase close.

## Assumptions

- The seed document's 11-state machine, 4-dimension budget, 7-error taxonomy,
  and 14-interface surface are directionally correct and this phase does not
  reopen them. **(Strong.)**
- The target consumer is a platform or infra team at an organisation where
  LLM calls already exist in production, and the pain is auditability, cost,
  and security rather than "first LLM integration". **(Medium — this is the
  frame that justifies the design, but it has not been validated with
  external users. Flagged.)**
- Go 1.23+ is an acceptable floor. **(Strong — supported by the toolchain
  era and by generics-free interface choices in the seed.)**
- There will be a first production consumer willing to ship against v0.5.x
  before v1.0 is tagged, gating the freeze. **(Medium — documented in seed
  §8 v1.0.0 criterion but outside phase 1's control.)**
- The banned-identifier list in seed §6.1 is complete enough to enforce the
  decoupling contract; Phase 6 will formalize the CI check. **(Weak — new
  leakage categories are likely to be discovered. Flagged.)**
- `github.com/praxis-os/praxis` is available as a module path and `praxis`
  is clear for use. **(Unvalidated — D10 must check this.)**

## Risks

**Critical (block v1.0 or break the decoupling contract):**

- **R1 — Positioning drift.** If the positioning statement is vague ("a Go
  library for LLM agents"), Phase 3 interface design will expand to cover
  everything and v1.0 will never freeze. Phase 1 must produce a statement
  narrow enough that adding a feature can be rejected on scope grounds.
- **R2 — Leaky consumer assumptions.** Phase 1 is the first artifact after
  extraction; it is the highest-risk point for consumer-specific vocabulary
  to slip into design language. Reviewer must grep phase artifacts against
  the banned list before approval.
- **R3 — Name collision on `praxis`.** A late rename (post-v0.1.0) costs
  module path churn and breaks early consumers. D10 must confirm before any
  subsequent phase writes the name into interface godoc.
- **R4 — Interface freeze scope ambiguity.** If D04/D13 are not crisp,
  Phase 3 will relitigate "is this interface v1.0-frozen or not" on every
  method. Decision must enumerate each interface explicitly.

**Secondary:**

- **R5 — Anti-persona missing.** Without a stated anti-persona, users
  arriving from a LangChain/Python background will file bugs against
  missing features that are deliberately out of scope.
- **R6 — `requires_approval` under-specification.** Deferring D07 to
  Phase 2 is tempting but pollutes the state machine design; resolving it
  here keeps Phase 2 clean.
- **R7 — Hot-reload ambiguity** (D08). The wrong default locks out a
  legitimate use case (provider price cuts mid-process); the wrong call in
  the other direction introduces race hazards in token accounting.
- **R8 — Smoke-test promise regression.** If D12 goes to "no", the
  30-line README example in seed §7 becomes hard to write, and DX suffers.

## Deliverables

All files land in `docs/phase-1-api-scope/`. Numbered prefix enforces reading
order. `REVIEW.md` is unnumbered and always last.

- `00-plan.md` — this file.
- `01-decisions-log.md` — D01–D15 with full rationale, alternatives considered,
  and a one-line summary suitable for the `roadmap-status` skill.
- `02-positioning-and-principles.md` — README-ready positioning paragraph,
  final design principles list, target consumer archetype, anti-persona,
  positioning gaps not addressed (D11).
- `03-non-goals.md` — explicit non-goals for v1.x; structured so each entry
  has a rationale and a pointer to the interface that *would* exist if the
  non-goal were reversed.
- `04-v1-freeze-surface.md` — enumeration of the 14 seed interfaces with a
  per-interface verdict: `frozen-v1.0`, `stable-v0.x-candidate`, or
  `post-v1`; includes D14 (OpenAI/Azure parity) and D12 (zero-wiring smoke
  path) as interface-level implications.
- `05-seed-question-resolutions.md` — direct answers to seed §13.1–§13.4,
  i.e. decisions D06, D07, D08, D09, with the rationale for each.
- `REVIEW.md` — reviewer's verdict: `READY` or `BLOCKED`, the banned-identifier
  grep output, and a checklist mapping exit criteria to evidence.

## Recommended Subagents

- **api-designer** — Phase 1 is the charter for every public interface in the
  library; the API designer must own D04, D11, D12, D13, D14 and the v1.0
  freeze-surface document. This is the single most load-bearing recommendation
  of the phase.
- **solution-researcher** — The positioning table vs LangChainGo, ADK, Eino
  and direct SDKs needs current data (what each library added or dropped since
  the seed was written) and availability checks for the `praxis` name on
  GitHub and pkg.go.dev (D10). Research output feeds D01, D11, and D10
  directly.

(No security, observability, go-architect, or dx-designer involvement this
phase — each has a dedicated later phase where their input is load-bearing,
and pulling them in now would expand scope. Reviewer is always invoked and is
not listed here.)

## Exit Criteria

1. D01–D14 are each recorded in `01-decisions-log.md` with rationale and
   alternatives. D15 is either allocated or explicitly released.
2. Seed §13 questions 1–4 are each resolved by a decision in D06–D09 and
   cross-referenced in `05-seed-question-resolutions.md`.
3. `04-v1-freeze-surface.md` assigns every interface from seed §5 to exactly
   one tier (`frozen-v1.0`, `stable-v0.x-candidate`, `post-v1`).
4. `03-non-goals.md` contains at least the non-goals implied by seed §2 and
   §3 plus any introduced during phase discussion.
5. `02-positioning-and-principles.md` contains a README-ready positioning
   paragraph and an explicit anti-persona.
6. Banned-identifier grep over `docs/phase-1-api-scope/**` returns zero
   matches against the seed §6.1 list. Reviewer records the grep command and
   result in `REVIEW.md`.
7. Module path `github.com/praxis-os/praxis` is confirmed available, or a
   rename is recorded in D10 and propagated to the seed via amendment note.
8. `reviewer` subagent returns PASS with no unresolved blockers.
9. `REVIEW.md` verdict is `READY`.
10. `roadmap-status` reflects Phase 1 as `approved` and Phase 2 as eligible
    to start.
