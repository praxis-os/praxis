# Roadmap Status

**Last updated:** 2026-04-06
**Target:** `praxis v1.0.0` — stable public Go API for enterprise agent orchestration

## Phase Status

| # | Phase | Status | Artifacts |
|---|-------|--------|-----------|
| 0 | Seed Context | starting baseline (amendable via decision-log amendment) | 1 (`docs/PRAXIS-SEED-CONTEXT.md`) |
| 1 | API Scope and Positioning | **approved** | 7 (`00-plan.md`, `01-decisions-log.md`, `02-positioning-and-principles.md`, `03-non-goals.md`, `04-v1-freeze-surface.md`, `05-seed-question-resolutions.md`, `06-composition-patterns.md`, `REVIEW.md`) |
| 2 | Core Runtime Design | **approved** | 8 (`00-plan.md`, `01-decisions-log.md`, `02-state-machine.md`, `03-streaming-and-events.md`, `04-cancellation-and-context.md`, `05-concurrency-model.md`, `06-state-machine-invariants.md`, `REVIEW.md`) |
| 3 | Interface Contracts | **approved** | 14 (`00-plan.md`, `01-decisions-log.md`, `02-orchestrator-api.md`, `03-llm-provider.md`, `04-hooks-and-filters.md`, `05-budget-interfaces.md`, `06-tools-and-invocation-context.md`, `07-errors-and-classifier.md`, `08-telemetry-interfaces.md`, `09-credentials-and-identity.md`, `10-state-types.md`, `11-defaults-and-construction.md`, `go-architect-package-layout.md`, `REVIEW.md`) |
| 4 | Observability and Error Model | not-started | — |
| 5 | Security and Trust Boundaries | not-started | — |
| 6 | Release, Versioning and Community Governance | not-started | — |

## Adopted Decisions

**50 of 52 allocated decisions adopted** across Phases 1–3. Phase 1 owns
D01–D15 (D15 released unused); Phase 2 owns D15–D30 (D15–D28 adopted,
D29/D30 released unused); Phase 3 owns D31–D52 (D31–D52 adopted).
Next range opens at **D53** for Phase 4.

- **Phase 1 (approved):** D01–D14 adopted. D15 released.
- **Phase 2 (approved):** D15–D28 adopted. D29, D30 released.
- **Phase 3 (approved):** D31–D52 adopted. D51 resolved the package layout
  (facade in `orchestrator/` sub-package, types in root). D52 recorded
  three seed amendments (interface→struct, 21-event vocabulary, PriceProvider
  promotion).
- **Phase 4–6:** no decisions allocated yet.

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

Phase 2 addressed all four IMPORTANT reviewer findings in-place before
`REVIEW.md` was issued. Verdict: **READY**.

**Phase 3 — Interface Contracts (approved).** 22 decisions (D31–D52):

- **D31 — `EventType` typed string** (`type EventType string`). Grep-friendly,
  JSON-serializable. 21 named constants (19 transition + 2 content-analysis).
- **D32 — `InvocationEvent` in root package** (amended by D51: facade moved
  to `orchestrator/` sub-package; types remain in root).
- **D33 — `Decision` struct** with `Verdict` enum + `Metadata map[string]any`.
  Four verdicts: Allow, Deny, RequireApproval, Log.
- **D34 — `budget.Guard`** dimension-additive (`RecordTokens`, `RecordToolCall`,
  `RecordCost`) plus `Check` and `Snapshot`. CP3-compliant.
- **D35 — `BudgetSnapshot`** value struct: 5 consumption fields + `ExceededDimension`.
- **D36 — `InvocationContext`** struct: ID, budget, span, identity, metadata. No
  tool name (D06).
- **D37 — Constructor** `orchestrator.New(provider, opts...)` with 12 `With*`
  options. Concrete `*Orchestrator` struct (D52a amends seed §5 interface).
- **D38 — Request/Result** shapes: 7-field request, 5-field result.
- **D39 — `ApprovalRequiredError`** with `ApprovalSnapshot` (messages, original
  request, budget, approval metadata). HTTP 202.
- **D40 — `ToolResult`** with `Err error` for CP5 typed-error propagation.
- **D41 — `llm.Provider`** 5 methods. `Stream` returns channel.
- **D42 — Filter returns** `(filtered, []FilterDecision, error)`.
- **D43 — `state.State`** `uint8` iota; `Transitions(s)` function.
- **D44 — `Classifier.Classify(error) TypedError`** with identity rule for CP5.
- **D45 — `credentials.Resolver`** `Fetch(ctx, CredentialRef) (Credential, error)`.
- **D46 — `identity.Signer`** `Sign(ctx, invocationID, toolName) (string, error)`
  (stable-v0.x-candidate).
- **D47 — `PriceProvider`** promoted to `frozen-v1.0`.
- **D48 — No `Close()`** on Orchestrator.
- **D49 — UUIDv7** invocation IDs.
- **D50 — Resume** = fresh `Invoke` (confirms D07).
- **D51 — Package layout** facade in `orchestrator/`; OPEN-1 import cycle
  resolved via PolicyInput projection fields.
- **D52 — Seed amendments** interface→struct, 21 events, PriceProvider promotion.

Phase 3 reviewer surfaced 2 blockers and 7 important findings; all resolved
in-place via D51/D52. Verdict: **READY**.

## Open Decisions

No open decisions in Phases 1–3.

- **Phase 4 (Observability and Error Model) — open.** Must allocate:
  OTel span tree and C1 resolution for `internal/ctxutil.DetachedWithSpan`,
  Prometheus metric set and cardinality constraints, slog redaction rules,
  error-to-event mapping, `BudgetExceededError` C3 token-overshoot
  documentation, emission semantics for `EventTypePIIRedacted` and
  `EventTypePromptInjectionSuspected` (D52b), `FilterDecision` → event
  mapping, CP1 span child-of semantics, CP2 `parent_invocation_id`,
  CP5 classifier precedence rules.
- **Phase 5 (Security and Trust) — open.** Must allocate: credential
  zero-on-close mechanics, `credentials.Resolver.Fetch` soft-cancel
  `context.WithoutCancel` requirement (C4), `identity.Signer` JWT claim
  set and key lifecycle, promotion to `frozen-v1.0`, CP6 identity chaining.
- **Phase 6 (Release and Governance) — open.** Must allocate: semver
  deprecation windows, release-please configuration, CI pipeline including
  banned-identifier grep, D10 tripwire enforcement, bus-factor mitigation
  (seed §14.1), first-production-consumer gating for v1.0.0.

## Risks / Blockers

- **D10 external dependency** (carried from Phase 1). Name/module-path
  resolution is conditional. Phase 3 used `MODULE_PATH_TBD` throughout;
  v0.1.0 tag is gated on resolution.
- **C2 documented** (Phase 2). Phase 3 `InvocationEvent` godoc in
  `08-telemetry-interfaces.md` explicitly states that per-tool completion
  ordering is not preserved under parallel dispatch. Risk mitigated.
- **`identity.Signer` promotion** remains the sole `stable-v0.x-candidate`.
  Phase 5 must finalize JWT claim set and promote before v0.5.0. Phase 6
  release must hard-gate this.
- **Bus factor** (seed §14.1). Flagged for Phase 6.
- **First production consumer dependency** (seed §8 v1.0.0 criterion).
  v1.0.0 tag is gated on a production consumer shipping against v0.5.x.
  Flagged for Phase 6.

## Decoupling Contract Health

**PASS.** A case-insensitive word-bounded grep against the seed §6.1
banned-identifier set returns zero matches across all Phase 1, Phase 2,
and Phase 3 artifacts as actual identifiers. Occurrences are limited to
negation-mentions inside compliance declarations in review files. Verified
by the reviewer subagent in each phase pass.

The decoupling contract is a correctness invariant, not an amendable
decision.

## Next Step

Invoke `plan-phase` on **Phase 4 — Observability and Error Model**. Phase 4
must consume Phase 3's interface contracts in full, especially the 21
`EventType` constants (D52b), the `LifecycleEventEmitter.Emit` contract,
the `AttributeEnricher.Enrich` contract, the `Classifier` precedence rules
(CP5), the `FilterDecision` type (D42), and the forward-carried concerns
C1 (span re-attachment), C3 (token overshoot), CP1 (nested span child-of),
and CP2 (parent_invocation_id). Decision range opens at D53.

## Overall Status

Planning is on track: Phases 1–3 of 6 are approved with a clean
decoupling contract and 50 adopted decisions; three design phases remain
before implementation begins. **v0.1.0** (first working invocation)
requires Phase 4 minimum plus D10 resolution. **v0.5.0** (feature
complete) requires all six phases approved plus `identity.Signer` promoted
to `frozen-v1.0` (the sole remaining candidate). **v1.0.0** (API freeze)
further requires a production consumer to ship against a v0.5.x tag — a
dependency outside any design phase. Adopted decisions in every phase
remain amendable via the protocol recorded in each phase's decision log.
