# Phase: Core Runtime Design

**Phase number:** 2
**State:** in-progress
**Decision range:** D15–D30 (allocated, unused IDs released at close)
**Depends on:** Phase 1 (approved — D01–D14 adopted)

## Goal

Specify the invocation runtime — state machine, lifecycle, streaming transport,
cancellation, context propagation, and concurrency model — precisely enough that
Phase 3 can translate every transition and boundary into a concrete Go interface
signature without further runtime questions.

## Scope

**In scope** (refines seed §4.2, §4.3, §4.4, §4.5):

- Reconciling the state machine count: seed §1/§8 say 11 states, seed §4.2
  enumerates 13, and D07 (approved) adds `ApprovalRequired` as a ninth terminal
  exit. Phase 2 must produce a single authoritative state list with
  allow-listed transitions.
- Terminal representation for `ApprovalRequired`: is it a distinct terminal
  state (parallel to `Completed`, `Failed`, `Cancelled`, `BudgetExceeded`), or
  a sub-status of `Failed` carrying a structured `ApprovalRequiredError`? D07
  is ambiguous on runtime placement; Phase 2 decides.
- Streaming transport contract: event types emitted on the `<-chan
  InvocationEvent`, ordering guarantees, buffer semantics (seed §4.4 says
  16), drain-on-terminal rules, close-channel protocol, backpressure
  interaction with cancellation.
- Cancellation semantics: soft vs. hard cancel boundaries per seed §4.5,
  behaviour on approval terminals, interaction with the synchronous terminal
  lifecycle emission on a derived background context (seed §4.5 invariant),
  and how D07's approval-pending terminal composes with soft-cancel grace
  windows.
- Context propagation: parent `context.Context` vs. the derived background
  context used for terminal lifecycle emission; which operations run on
  which; bounded deadline inheritance rules.
- Concurrency model: one goroutine per invocation vs. bounded orchestrator
  worker pool; concurrent-safety contract of `AgentOrchestrator`
  (seed §5 says safe for concurrent use); ownership of the streaming
  producer goroutine.
- Budget-clock interaction: when does the wall-clock dimension start and
  stop, does it freeze on approval terminals, and how does
  `budget.PriceProvider` per-invocation snapshot (D08) interact with mid-loop
  tool cycles.
- Zero-wiring streaming path (D12 boundary): `InvokeStream` must remain
  constructible with only `llm.Provider` plus defaults; define what the
  channel emits when all hooks, filters, budget, telemetry are null.
- Property-based test invariants the phase artifacts commit to (seed §10
  already mandates gopter — Phase 2 writes the invariant list).

**Out of scope** (deferred to later phases):

- Final Go method signatures for any interface (Phase 3).
- OTel span tree and metric cardinality (Phase 4).
- Credential and identity mechanics (Phase 5).
- Release gating, deprecation windows (Phase 6).
- Adapter-specific retry tuning (belongs with `llm.Provider` adapters, not
  runtime).

## Key Questions

1. **State count reconciliation.** What is the authoritative state list?
   Candidates: (a) 11 states as seed §1/§8; (b) 13 states as seed §4.2 with
   four terminals; (c) 13 non-terminal + 5 terminal with `ApprovalRequired`
   as a new terminal from D07; (d) 13 states with `ApprovalRequired` modelled
   as a sub-status of `Failed`. What does the allow-listed transition table
   look like under the chosen option?
2. **Where does `ApprovalRequired` sit?** Is it a terminal state of its own,
   or a structured error carried out of `Failed`? The answer drives the
   streaming event set, the `InvokeStream` close protocol, and whether
   resume is a new invocation or a continuation (D07 says caller owns
   persistence and resume — Phase 2 must say precisely what the orchestrator
   emits at the approval point).
3. **What `InvocationEvent` types are emitted, and in what order?** Enumerate
   every event type. Specify ordering guarantees (e.g., all `Tool*` events
   for a given tool call are emitted before the following
   `LLMContinuation` event). Specify the final event on every terminal path.
4. **Channel close semantics on `InvokeStream`.** Who closes the channel,
   under what condition, and is the terminal event emitted before or after
   close? What does a cancelled stream look like to the consumer — does the
   consumer see a terminal `Cancelled` event, or just a closed channel?
5. **Soft-cancel grace window under approval.** If a soft cancel arrives while
   the invocation is stalled waiting to emit an approval-required terminal,
   which wins? Does the 500 ms grace apply? Does approval emission get the
   same bounded 5 s background-context guarantee as other terminal events?
6. **Concurrency model.** Is each invocation its own goroutine tree owned by
   the orchestrator, or does the orchestrator hold a bounded worker pool
   shared across invocations? What is the concurrent-safety guarantee on a
   single `AgentOrchestrator` instance — is it `N` concurrent `Invoke` calls
   with linear memory, or does it share state? What does cancellation of one
   invocation guarantee about others?
7. **Streaming producer ownership.** For `InvokeStream`, which goroutine
   produces events into the 16-buffered channel, and which goroutine runs
   the state machine loop? Single goroutine doing both, or producer +
   consumer split? Implication for backpressure fairness across concurrent
   streams.
8. **Budget wall-clock boundary.** Does wall-clock start at `Created` or at
   the first `LLMCall`? Does it stop at the terminal state entry or at
   terminal lifecycle event emission completion? Does it include the 5 s
   synchronous terminal-emission window or exclude it?
9. **Zero-wiring streaming behaviour.** With `llm.Provider` as the sole
   dependency and all else null, what does `InvokeStream` emit on the
   channel? At minimum: `InvocationStarted`, one `LLMCall`-ish event, and
   a terminal `Completed`? Or richer?
10. **Property-based invariant set.** What is the list of invariants we
    commit to fuzzing with gopter in CI? (e.g., "no illegal transition",
    "every terminal path emits exactly one terminal lifecycle event",
    "no path bypasses `PreHook`", "no path reaches `LLMCall` before
    `Initializing`".)

## Decisions Required

(Allocated range `D15–D30`. Unused IDs are released at phase close per the
Phase 1 amendment protocol.)

- **D15 — Authoritative state list.** Produce the canonical enumeration,
  reconcile seed §1/§8 vs §4.2, and integrate D07.
- **D16 — Transition allow-list.** The full adjacency table for the state
  list from D15. Lists every legal `(from, to)` pair.
- **D17 — Placement of `ApprovalRequired`.** Terminal state of its own or
  sub-status of `Failed`. If terminal, how does it interact with resume
  semantics from D07.
- **D18 — `InvocationEvent` type enumeration.** Complete set of event types
  emitted on the `InvokeStream` channel, with 1:1 or 1:N mapping to state
  transitions declared explicitly.
- **D19 — Streaming channel close protocol.** Who closes, when, and in what
  order relative to the terminal event. Buffer is 16 per seed §4.4 (not
  re-opened).
- **D20 — Backpressure and stuck-consumer semantics.** When the 16-buffer
  fills, orchestrator blocks — confirm, and specify the interaction with
  cancellation deadlines (seed §4.4 says `ctx.Done()` bounds the block).
- **D21 — Soft vs. hard cancel precedence rules.** Including what happens
  when soft cancel arrives during approval-required, budget breach, or
  tool-call-in-flight.
- **D22 — Terminal lifecycle-emission invariant.** Confirm and lock the
  derived-background-context + bounded 5 s deadline rule from seed §4.5, and
  extend it to cover `ApprovalRequired` if D17 makes it terminal.
- **D23 — Context propagation model.** Which `context.Context` each
  transition runs under, rules for deriving background contexts.
- **D24 — Concurrency model.** Goroutine topology per invocation and
  concurrent-safety guarantee of `AgentOrchestrator`. Single-goroutine loop
  vs. producer/consumer split for streaming.
- **D25 — Budget wall-clock boundary.** Exact start and stop points of the
  wall-clock dimension.
- **D26 — `PriceProvider` snapshot boundary.** D08 says per-invocation
  snapshot; Phase 2 pins the exact state at which the snapshot is taken
  (e.g., entry to `Initializing`).
- **D27 — Zero-wiring streaming event set.** The minimum event set emitted
  by `InvokeStream` under the D12 zero-wiring constraint.
- **D28 — Property-based invariant list.** The gopter invariant set for the
  v0.3.0 state machine property tests.

D29 and D30 are held as reviewer reserves, released unused at close unless
the reviewer subagent surfaces an additional runtime decision.

## Assumptions

- D07's approval semantics (caller owns persistence and resume) are stable;
  Phase 2 only decides the runtime shape, not whether resume is supported.
- Seed §4.4's 16-event buffer is a working default, not a tunable in v1.0.
  If Phase 2 challenges this it must do so explicitly as an amendment to
  seed §4.4.
- Seed §4.5's soft/hard cancel model and the 5 s bounded background-context
  emission window are load-bearing invariants, not drafts. Phase 2 confirms
  and extends them rather than redesigning.
- `AgentOrchestrator` concurrent-safety from seed §5 means one instance
  serves N concurrent `Invoke` / `InvokeStream` calls. Not multi-tenancy —
  just thread safety.
- **Weak assumption flagged:** the state-count discrepancy in the seed is a
  clerical error, not a hidden semantic disagreement. If Phase 2 uncovers
  an actual semantic divergence, the reconciliation becomes an amendment
  to seed §4.2 or §1/§8, not just a Phase 2 decision.

## Risks

**Critical (blocks v1.0 or the decoupling contract):**

- **State list churn after Phase 3 starts.** If D15/D16 are not airtight,
  Phase 3 will be forced to redesign interface signatures mid-phase.
  Mitigation: reviewer subagent exit criterion is "state list is stable".
- **Hidden coupling between approval semantics and streaming close.** If
  D17 places `ApprovalRequired` as a terminal and D19 does not specify the
  final event precisely, the caller cannot distinguish "approval needed,
  resumable" from "failed, not resumable" just from the stream. This is a
  correctness bug waiting to ship.
- **Decoupling leakage via event names.** `InvocationEvent` types enumerated
  in D18 must all live under the neutral `praxis.Event*` namespace
  (seed §6.2). A single event name like `GovernanceEventX` would fail the
  banned-identifier grep. Reviewer check required.

**Secondary:**

- Property-based invariant set is subjective; D28 risks under- or
  over-specifying. Mitigation: cross-check against seed §10 requirements
  and Phase 3's interface shape.
- Concurrency model (D24) may be hard to validate without benchmark data
  not available until v0.3.0. Phase 2 locks the _shape_ (single goroutine
  vs. pool) and leaves tuning to implementation.

## Deliverables

All files in `docs/phase-2-core-runtime/`, numbered for reading order:

- `00-plan.md` — this file.
- `01-decisions-log.md` — adopted decisions D15–D28 (plus any released
  reserves) with rationale, amendability note, and cross-references to the
  seed and to Phase 1.
- `02-state-machine.md` — canonical state list, Mermaid state diagram, full
  allow-listed transition table, terminal rules, and the D17 resolution for
  `ApprovalRequired`.
- `03-streaming-and-events.md` — `InvocationEvent` taxonomy (Mermaid
  sequence or state diagram), ordering guarantees, channel close protocol,
  backpressure rules, zero-wiring minimum event set.
- `04-cancellation-and-context.md` — soft vs. hard cancel matrix, terminal
  lifecycle emission invariant, context propagation rules, budget wall-clock
  boundary.
- `05-concurrency-model.md` — goroutine topology, concurrent-safety
  guarantee, `InvokeStream` producer ownership.
- `06-state-machine-invariants.md` — the D28 property-based invariant list,
  written in a form gopter generators can target.
- `REVIEW.md` — reviewer verdict. Unnumbered. Last file in the phase
  directory.

## Recommended Subagents

- **go-architect** — owns the goroutine topology, `context.Context`
  propagation rules, and the streaming producer/consumer split. D23 and D24
  are load-bearing Go runtime decisions that need a Go concurrency
  specialist, not just an interface designer.
- **api-designer** — owns the shape of the `InvocationEvent` taxonomy
  (D18/D19) and the D17 placement of `ApprovalRequired`, because these
  decisions constrain Phase 3's interface signatures. Without this subagent
  we risk decisions that read well in prose but cannot be cleanly expressed
  in Go types.

The reviewer subagent is always invoked. solution-researcher,
observability-architect, security-architect, and dx-designer are _not_
recommended for this phase — they lead later phases where their expertise
is load-bearing.

## Exit Criteria

1. D15–D28 all have adopted working positions in `01-decisions-log.md`.
   Unused decision IDs from the D15–D30 range explicitly released at phase
   close.
2. `02-state-machine.md` contains a single canonical state list with no
   residual contradiction between seed §1/§8 and seed §4.2. Any amendment
   to the seed is recorded explicitly under the Phase 1 amendment protocol.
3. `03-streaming-and-events.md` enumerates every `InvocationEvent` type and
   maps each to a state transition. No event name collides with the
   banned-identifier list (seed §6.1).
4. `04-cancellation-and-context.md` covers every combination in the
   soft/hard cancel × approval/budget/tool-in-flight matrix.
5. `06-state-machine-invariants.md` lists at least the invariants already
   mandated by seed §10, plus any new invariants introduced by D15–D17.
6. `reviewer` subagent returns PASS on decoupling-contract grep and on
   runtime coherence. No banned identifiers appear in any Phase 2 artifact.
7. `review-phase` skill returns `REVIEW.md` verdict **READY**.
8. `roadmap-status` updated to reflect Phase 2 approval and the next phase
   is Phase 3 — Interface Contracts.
