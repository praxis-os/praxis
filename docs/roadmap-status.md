# Roadmap Status

**Last updated:** 2026-04-05
**Target:** `praxis v1.0.0` — stable public Go API for enterprise agent orchestration

## Phase Status

| # | Phase | Status | Artifacts |
|---|-------|--------|-----------|
| 0 | Seed Context | starting baseline (amendable via decision-log amendment) | 1 (`docs/PRAXIS-SEED-CONTEXT.md`) |
| 1 | API Scope and Positioning | **approved** | 7 (`00-plan.md`, `01-decisions-log.md`, `02-positioning-and-principles.md`, `03-non-goals.md`, `04-v1-freeze-surface.md`, `05-seed-question-resolutions.md`, `06-composition-patterns.md`, `REVIEW.md`) |
| 2 | Core Runtime Design | **approved** | 8 (`00-plan.md`, `01-decisions-log.md`, `02-state-machine.md`, `03-streaming-and-events.md`, `04-cancellation-and-context.md`, `05-concurrency-model.md`, `06-state-machine-invariants.md`, `REVIEW.md`) |
| 3 | Interface Contracts | not-started | — |
| 4 | Observability and Error Model | not-started | — |
| 5 | Security and Trust Boundaries | not-started | — |
| 6 | Release, Versioning and Community Governance | not-started | — |

## Adopted Decisions

**28 of 30 allocated decisions adopted** across Phases 1–2. Phase 1 owns
D01–D15 (D15 released unused); Phase 2 owns D15–D30 (D15–D28 adopted,
D29/D30 released unused). Next range opens at **D31** for Phase 3.

- **Phase 1 (approved):** D01–D14 adopted. D15 released.
- **Phase 2 (approved):** D15–D28 adopted (note: Phase 2's D15 is a
  distinct decision from the Phase 1 reserve of the same ID — Phase 2
  allocated a fresh D15 as the first decision of its own range). D29, D30
  released.
- **Phase 3–6:** no decisions allocated yet.

Adopted decisions remain **amendable** via the protocol recorded in
`docs/phase-1-api-scope/01-decisions-log.md#amendment-protocol`. The
three-tier stability policy (D13) governs interface freezes separately
from methodological-decision amendability.

## Completed Work

**Phase 1 — API Scope and Positioning (approved).** 14 decisions (D01–D14):
positioning statement, eight design principles (seven from seed + the
zero-wiring smoke path principle), target consumer archetype and three
anti-personas, v1.0 freeze surface (12/14 interfaces at `frozen-v1.0`,
two at `stable-v0.x-candidate`), seven non-goals, tool-name placement on
`ToolCall` (seed §13.1), `ApprovalRequiredError` terminal semantics (seed
§13.2), `PriceProvider` per-invocation snapshot policy (seed §13.3), "no
plugins in v1" re-confirmed (seed §13.4), conditional adoption of the name
`praxis` / module `github.com/praxis-go/praxis` with a Phase 3 tripwire,
positioning gaps catalogued, zero-wiring smoke-test promise, three-tier
stability policy, Azure OpenAI best-effort parity. All four seed §13 open
questions resolved. Decoupling grep clean. `REVIEW.md` verdict: **READY**.

**Phase 2 — Core Runtime Design (approved).** 14 decisions (D15–D28):

- **D15 — 14-state machine** (9 non-terminal + 5 terminal). Reconciles
  seed §1/§8's "11 states" and §4.2's 13-state enumeration; adds
  `ApprovalRequired` per D07. Recorded as a seed amendment.
- **D16 — Transition allow-list.** Full adjacency table with Mermaid
  state diagram; `ApprovalRequired` reachable only from `PreHook`/`PostHook`;
  `BudgetExceeded` reachable only from `LLMCall`/`ToolDecision`/`LLMContinuation`.
- **D17 — `ApprovalRequired` is a distinct terminal state** (not a
  `Failed` sub-status). Pins three constraints on the `hooks.PolicyHook`
  `Decision` type (value-variant, opaque metadata forwarding,
  `Allow`/`Deny`/`RequireApproval` minimum) and a minimum conversation
  snapshot content specification.
- **D18 — 19 `InvocationEvent` types** under the `EventType*` namespace
  (seed §6.2 compliant). Ordering guarantees for tool cycles, hook
  brackets, and terminal events.
- **D19 — Streaming channel close protocol.** Loop goroutine is sole
  owner; `sync.Once`-guarded close after terminal event; panic-safe.
- **D20 — Backpressure via `select + ctx.Done()`.** No non-blocking
  sends; seed §4.4 16-event buffer retained.
- **D21 — Soft vs. hard cancel precedence matrix.** Terminal state is
  immutable; budget/approval always preempt soft cancel; 500 ms grace
  applies only to in-flight I/O, never to stuck-consumer sends.
- **D22 — Terminal lifecycle-emission invariant** extended from seed
  §4.5's four terminals to all five (including `ApprovalRequired`), with
  the 5-second bounded background-context rule and single-point
  `internal/ctxutil` helper.
- **D23 — Four-layer context propagation** (caller / invocation /
  operation / terminal emission). `context.WithoutCancel` used on the
  soft-cancel grace path.
- **D24 — One goroutine per invocation, sole-producer rule** on the
  stream channel. Parallel tool dispatch via `golang.org/x/sync/errgroup`;
  `ToolCallStarted` events emitted before dispatch to honor the name
  semantic (revised during review).
- **D25 — Budget wall-clock boundary:** starts at `Initializing`, stops
  at terminal state entry (5-second emission window excluded). Token
  dimension can overshoot by one call's worth (physical consequence of
  the provider API, documented for Phase 4).
- **D26 — `PriceProvider` snapshot taken at `Initializing` entry**,
  co-located with wall-clock start.
- **D27 — Zero-wiring streaming event set:** exactly 10 events on a
  one-turn path, 18 events on a one-tool two-turn path.
- **D28 — 21 property-based invariants** across state-machine structural,
  streaming event, and terminal path groups. Targets `gopter` at 10k
  iterations in CI, 100k nightly (seed §10).
- **D29, D30** released unused at Phase 2 close.

Phase 2 also recorded five forward-carried concerns for Phase 3/4/5:

- **C1** — OTel span re-attachment in `internal/ctxutil.DetachedWithSpan`
  (load-bearing for Phase 4 span tree).
- **C2** — Parallel tool-call completion ordering not visible in stream.
- **C3** — Token-dimension budget can overshoot; Phase 4 must document.
- **C4** — Soft-cancel grace must pass `context.WithoutCancel` to
  `credentials.Resolver.Fetch` (Phase 5 contract requirement).
- **C5** — `golang.org/x/sync/errgroup` recorded as a runtime dependency.

Phase 2 addressed all four IMPORTANT reviewer findings (parallel dispatch
event semantics, `PolicyHook.Decision` constraint, cancel-during-`PreHook`
event sequence pin, soft/hard discriminant correction) in-place before
`REVIEW.md` was issued. Verdict: **READY**.

## Open Decisions

No open decisions in Phases 1–2. All decisions from Phase 3 onwards
remain unallocated.

- **Phase 3 (Interface Contracts) — open.** Must allocate: final Go
  method signatures for all 14 v1.0 interfaces, the `hooks.PolicyHook`
  `Decision` type shape per D17 constraints, the `InvocationEvent` struct
  field set per D18, the `BudgetSnapshot` struct (value-copyable, cheap
  to allocate), the `state.State` Go representation with the D16 adjacency
  table, the `ApprovalRequiredError` concrete type with conversation
  snapshot field, confirmation of whether resume is a fresh `Invoke` or a
  dedicated method, D10 module-path resolution before any godoc is
  written.
- **Phase 4 (Observability and Error Model) — open.** Must allocate:
  OTel span tree and C1 resolution for `internal/ctxutil.DetachedWithSpan`,
  Prometheus metric set and cardinality constraints, slog redaction rules,
  error-to-event mapping, taxonomy updated to eight concrete types per
  D07, `BudgetExceededError` documentation of the C3 token-dimension
  overshoot caveat.
- **Phase 5 (Security and Trust) — open.** Must allocate: credential
  zero-on-close mechanics, `credentials.Resolver.Fetch` contract including
  C4 `context.WithoutCancel` requirement on soft-cancel grace, conversation
  snapshot redaction assessment (joint with Phase 3 per D17 constraint),
  `identity.Signer` JWT claim set and key lifecycle, promotion to
  `frozen-v1.0`.
- **Phase 6 (Release and Governance) — open.** Must allocate: semver
  deprecation windows, release-please configuration, CI pipeline including
  banned-identifier grep, D10 tripwire enforcement, bus-factor mitigation
  (seed §14.1), first-production-consumer gating for v1.0.0.

## Risks / Blockers

- **D10 external dependency** (carried from Phase 1). Name/module-path
  resolution is conditional; Phase 3 tripwire mitigates godoc exposure,
  but v0.1.0 tag is gated on resolution.
- **Parallel tool-call completion visibility gap** (Phase 2 C2). Phase
  3's `InvocationEvent` godoc must state explicitly that per-tool
  completion ordering is not preserved in the event stream under parallel
  dispatch. If Phase 3 forgets to document this, v1.0 consumers will be
  surprised.
- **Soft freeze promotions.** `budget.PriceProvider` (Phase 3) and
  `identity.Signer` (Phase 5) promotions to `frozen-v1.0` are soft
  commitments. Phase 6 release must hard-gate both before v0.5.0.
- **Bus factor** (seed §14.1). Flagged for Phase 6.
- **First production consumer dependency** (seed §8 v1.0.0 criterion).
  v1.0.0 tag is gated on a production consumer shipping against v0.5.x.
  Flagged for Phase 6.

## Decoupling Contract Health

**PASS.** A case-insensitive word-bounded grep against the seed §6.1
banned-identifier set returns zero matches across all Phase 1 and Phase 2
artifacts as actual identifiers. Four occurrences exist in Phase 2 files —
all are negation-mentions inside compliance declarations in
`03-streaming-and-events.md` §7 and one risk statement in `00-plan.md`.
Verified twice: once by the reviewer subagent, once independently in the
`review-phase` pass. The literal banned-identifier pattern is not embedded
in any document; the authoritative list lives in seed §6.1.

The decoupling contract is a correctness invariant, not an amendable
decision.

## Next Step

Invoke `plan-phase` on **Phase 3 — Interface Contracts**. Phase 3 must
consume Phase 2's runtime contract in full, especially D17's
`Decision`-type constraint block, D18's `InvocationEvent` field set, D25's
`BudgetSnapshot` shape requirement (value-copyable, cheap), and the C1–C5
concerns carried forward. Decision range opens at D31.

## Overall Status

Planning is on track: Phases 1 and 2 of 6 are approved with a clean
decoupling contract and 28 adopted decisions; four design phases remain
before implementation begins. **v0.1.0** (first working invocation)
requires Phase 3 complete plus D10 resolution. **v0.5.0** (feature
complete) requires all six phases approved plus both
`stable-v0.x-candidate` interfaces promoted to `frozen-v1.0`. **v1.0.0**
(API freeze) further requires a production consumer to ship against a
v0.5.x tag — a dependency outside any design phase. Adopted decisions in
every phase remain amendable via the protocol recorded in each phase's
decision log.
