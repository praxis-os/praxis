# Phase 2 — Decisions Log

**Phase:** 2 — Core Runtime Design
**Decision range:** D15–D30 allocated. D15–D28 adopted below. D29, D30 released
unused at phase close.
**Status:** under-review

## Amendability

Every adopted decision in this log is a **working position** — an explicit
commitment for the current planning cycle that can be amended in a later
phase or milestone if new evidence, downstream constraints, or external
changes justify revisiting it. Amendments follow the protocol recorded in
`docs/phase-1-api-scope/01-decisions-log.md#amendment-protocol`: a new
decision ID in a later phase supersedes the earlier one, the earlier entry
is annotated with a back-reference, and the rationale is recorded.

The three-tier stability policy (D13) is separate from amendability.
Interfaces carrying `frozen-v1.0` in Phase 3 and beyond are semver-load-bearing
for downstream consumers — they are not amendable in the same sense as
methodological decisions in a decisions log.

---

## D15 — Authoritative state list: 14 states (9 non-terminal + 5 terminal)

**Decision.** The canonical `praxis` invocation state machine has 14 states:
nine non-terminal (`Created`, `Initializing`, `PreHook`, `LLMCall`,
`ToolDecision`, `ToolCall`, `PostToolFilter`, `LLMContinuation`, `PostHook`)
and five terminal (`Completed`, `Failed`, `Cancelled`, `BudgetExceeded`,
`ApprovalRequired`).

**Rationale.** Seed §1 and §8 describe an "11-state machine" in narrative
text; seed §4.2 enumerates 13 numbered states. D07 (Phase 1) adds
`ApprovalRequired` as a new terminal arising from policy-hook approval
decisions. The most faithful reconciliation is: the narrative count was a
clerical rounding, §4.2's 13-state table was the accurate reference, and
D07 adds the fifth terminal — producing 14.

**Seed amendment note.** Seed §1, §8, and the §4.1 component diagram label
("11 states") are superseded by D15 per the Phase 1 amendment protocol. The
seed document text is not edited in place; D15 is the authoritative record.
All future Phase 2–6 artifacts, implementation, and documentation use
"14-state machine" from this point.

**Trade-offs rejected.**

- (a) Collapsing `PostToolFilter` and `LLMContinuation` into one state to
  reach the narrative's "11" count — rejected. These states own semantically
  disjoint concerns (output filter execution vs. tool-result injection and
  budget re-check before the next LLM call). Collapsing them would force one
  hook phase to straddle two unrelated responsibilities.
- (b) Treating `Cancelled` and `BudgetExceeded` as error sub-statuses of
  `Failed` — rejected. Callers must distinguish these terminal classes
  without unwrapping errors. The streaming event vocabulary (D18) depends
  on distinct terminal states.

Full enumeration, transition rules, and Mermaid state diagram are in
`02-state-machine.md`.

---

## D16 — Transition allow-list

**Decision.** The complete legal `(from, to)` adjacency table is enumerated
in `02-state-machine.md` §2 and rendered as a Mermaid `stateDiagram-v2` in
§3. Every edge not listed is illegal. The allow-list is expressible as a
static Go `map[State][]State` literal checked on every transition.

**Key structural properties.**

- `ApprovalRequired` is reachable from exactly `PreHook` and `PostHook`.
  Policy-hook privilege only; tool invokers and filters cannot transition
  directly to `ApprovalRequired`.
- `BudgetExceeded` is reachable from exactly `LLMCall`, `ToolDecision`, and
  `LLMContinuation`. These are the three points at which token-cost and
  tool-count accounting happen.
- `Cancelled` is reachable from `LLMCall`, `ToolDecision`, `ToolCall`,
  `PostToolFilter`, `LLMContinuation`, and `PostHook`. It is **not**
  reachable from `Created`, `Initializing`, or `PreHook` — cancellation
  before substantive work routes to `Failed` via `Initializing -> Failed`
  if needed.
- The tool cycle (`ToolDecision -> ToolCall -> PostToolFilter ->
  LLMContinuation -> ToolDecision`) may repeat arbitrarily. The state
  machine does not cap cycle count; cycle budgeting is enforced by
  `budget.Guard` on the tool-call-count dimension.
- Terminal states have no outgoing edges.

**Trade-offs rejected.**

- Allowing `ToolCall -> ApprovalRequired` — rejected. Tool invokers express
  denials as `ToolResult{Status: StatusDenied}` per seed §5. The approval
  terminal is a policy-hook privilege, not a tool-invoker privilege.
- Allowing `LLMCall -> ApprovalRequired` — rejected. Pre-LLM filters can
  `Block`, which routes to `Failed`. They do not carry approval semantics.

---

## D17 — `ApprovalRequired` is a distinct terminal state

**Decision.** `ApprovalRequired` is a terminal state in its own right,
parallel to `Completed`, `Failed`, `Cancelled`, and `BudgetExceeded`. It is
**not** a sub-status of `Failed`.

**Caller-observable semantics.**

- **Synchronous `Invoke`:** returns a non-nil error that is
  `errors.As`-compatible with a new concrete `ApprovalRequiredError` type
  (Phase 3 places it in the `errors` package). The `InvocationResult` is
  non-nil and carries the terminal state, invocation ID, budget consumed,
  and the conversation snapshot needed for out-of-process resume.
- **Streaming `InvokeStream`:** emits exactly one `EventTypeApprovalRequired`
  event carrying the same fields as above. The channel is closed immediately
  after. Callers distinguish "approval needed, resumable" from "failed, not
  resumable" by the event type on the final event — not by unwrapping an
  error from a generic `EventTypeInvocationFailed`.

**Rationale.** Placing `ApprovalRequired` as a sub-status of `Failed` would
conflate genuine errors with human-approval checkpoints, forcing streaming
consumers to pattern-match errors attached to `EventTypeInvocationFailed`.
D17 makes the terminal observable directly as a state and as an event type.
The cost is one additional terminal state and one additional event type —
negligible against the clarity gain.

**Seed amendment note.** Seed §4.2 lists four terminals. D07 anticipated a
fifth; D17 confirms and locks the five-terminal set:
`{Completed, Failed, Cancelled, BudgetExceeded, ApprovalRequired}`. Seed
§4.2 is superseded by D15 + D17 per the amendment protocol.

**Constraint on `hooks.PolicyHook.Decision` (carried to Phase 3).**
D17 requires that the `Decision` value returned by
`hooks.PolicyHook.Evaluate(ctx, phase, input) (Decision, error)` carry an
**approval-required variant** expressible without adding a new method to
the `PolicyHook` interface. Phase 3 chooses the concrete Go shape (struct
with a discriminator field, enum + optional metadata, tagged union, or
equivalent), but Phase 2 pins three requirements:

1. The loop goroutine must be able to detect "approval required" from the
   `Decision` value alone, without type-asserting an error or unwrapping
   a sentinel — so the hook path returns `(Decision{ApprovalRequired},
   nil)`, **not** `(Decision{}, ApprovalRequiredError{})`. Errors returned
   from hooks are reserved for hook-internal failures (e.g., policy engine
   unreachable) and route to `Failed`.
2. The `Decision` value may carry an optional, opaque "approval metadata"
   payload that the orchestrator forwards into the terminal
   `ApprovalRequiredError` and `EventTypeApprovalRequired` payload
   without interpretation. This metadata is caller-defined (reason, human
   approver hint, policy rule ID, etc.) and the framework must not
   inspect it.
3. The `Decision` type must support at minimum: `Allow`, `Deny` (routes to
   `Failed`), and `RequireApproval` (routes to `ApprovalRequired`). A
   `Log`/`Audit` variant is permitted if Phase 3 finds it useful but is
   not required by Phase 2.

Phase 3 allocates the next available decision ID for the concrete
`Decision` type shape. This constraint exists so that Phase 3 does not
invent a `Decision` type that cannot express the D17 terminal.

**Constraint on the approval conversation snapshot (Finding 5).**
Both the synchronous `ApprovalRequiredError` and the streaming
`EventTypeApprovalRequired` must carry a conversation snapshot sufficient
for caller-owned out-of-process resume (D07). Phase 2 pins the minimum
content:

- The full message history up to the approval point, in the
  provider-agnostic `[]llm.Message` format (seed §5 `llm.Provider`
  contract). Tool results, tool calls, and `MessagePart` values are all
  included.
- The `InvocationRequest` (or an equivalent projection) sufficient to
  re-invoke with the same configuration: agent definition reference,
  tool list, budget ceilings, and any caller-provided metadata.
- The `BudgetSnapshot` at the approval point, so the resuming caller can
  re-seat the budget.

Credential material is **never** included in the snapshot (enforced by
Phase 5). The snapshot is a value type; Phase 3 decides the Go struct
name and placement. Phase 5 assesses redaction requirements on the
message content (prompt-injection markers, PII) before the snapshot
leaves the framework boundary.

**Trade-offs rejected.** Option (b) sub-status of `Failed` — rejected for
the reasons above.

---

## D18 — `InvocationEvent` type enumeration: 19 event types

**Decision.** The full `InvocationEvent` type set is enumerated in
`03-streaming-and-events.md` §2. There are 19 event types, all prefixed
`EventType*` and exported from the `praxis` package per seed §6.2. Every
event type maps to one or two state transitions (a `*Started`/`*Completed`
bracket for each non-trivial state). Five terminal event types correspond
1:1 to the five terminal states.

**Key ordering guarantees** (full list in `03-streaming-and-events.md` §3):

1. `EventTypeInvocationStarted` is always the first event.
2. Exactly one terminal event is always the last event before channel close.
3. For each tool call ID, `EventTypeToolCallStarted`,
   `EventTypeToolCallCompleted`, `EventTypePostToolFilterStarted`, and
   `EventTypePostToolFilterCompleted` are paired and contiguous before the
   next `EventTypeLLMContinuationStarted`.
4. Parallel tool-call events may interleave across call IDs but each call
   ID's bracket is preserved.
5. No event is emitted after channel close.

**Namespace compliance.** All event type constants use the `EventType*`
prefix under `praxis.*`. None contain any banned identifier (seed §6.1). No
event name embeds a consumer-specific product namespace.

**Trade-offs rejected.**

- Emitting only terminal events — rejected. `InvokeStream` must provide
  per-state observability to justify its existence over `Invoke + goroutine`.
- Combining `*Started`/`*Completed` into a single event with a `Phase` field
  — rejected. SSE consumers need a "tool call in progress" signal emitted
  before the tool returns.
- A single `EventTypeStateTransition(From, To)` tuple — rejected. Consumers
  binding to lifecycle events need semantic names, not raw state pairs.

Field shape for `InvocationEvent` is prose-fixed in
`03-streaming-and-events.md` §5 and finalized in Phase 3.

---

## D19 — Streaming channel close protocol

**Decision.** The invocation loop goroutine is the **sole owner** of the
`<-chan InvocationEvent`. It closes the channel exactly once, via a
`sync.Once`-guarded wrapper in a deferred call, **after** the terminal event
has been sent. No other code path may close the channel.

Ordering contract, enforced by a single code path:

1. Terminal event is sent to the channel (blocking until buffer absorbs it
   or a timeout under the terminal emission context).
2. Channel is closed via the `sync.Once` wrapper.
3. No further send is attempted.

**Rationale.** Placing close ownership exclusively in the loop goroutine
with a `sync.Once` guard is the only panic-safe design in Go. Any scheme
with two closing paths needs the same guard anyway. Emitting the terminal
event before closing means a consumer reading until drain always receives
the terminal event — the close is a redundant-but-clean final signal.

**Trade-offs rejected.**

- Consumer closes the channel — impossible in Go without a producer panic.
- Close without a terminal event — consumer cannot distinguish clean
  termination from a bug-induced silent close.
- Separate `done` channel — adds API surface with no benefit over the
  terminal-event-before-close pattern.

---

## D20 — Backpressure and stuck-consumer semantics

**Decision.** Every channel send in the loop goroutine uses the canonical
pattern:

```
select {
case ch <- event:
case <-ctx.Done():
}
```

No non-blocking sends. No event drops. The 16-event buffer from seed §4.4 is
retained as the working default.

**Cancellation-while-blocked behavior.**

- On parent-context cancellation while the loop is blocked on a non-terminal
  send: the loop unblocks, transitions to `Cancelled`, and attempts a
  terminal `EventTypeInvocationCancelled` send under the terminal-emission
  context (D22 — derived from `context.Background()` with a 5s deadline).
- If the consumer resumes reading within the 5s window: the terminal event
  is delivered, then the channel is closed.
- If the consumer is still not reading at 5s: the terminal event is
  dropped; the channel is still closed. This is the only safe outcome for a
  genuinely dead consumer.

**Invariant.** Under normal (responsive) cancellation, the consumer always
receives the terminal `EventTypeInvocationCancelled` event before the
channel closes. Only a catastrophically stuck consumer sees a close without
a prior terminal event.

**Trade-offs rejected.**

- Non-blocking send with event drop — silent event loss is unacceptable for
  the audit contract.
- Per-event timeout separate from the invocation context — complexity
  without benefit.
- Larger buffer — delays backpressure without eliminating it. Seed §4.4's
  16 stands.

---

## D21 — Soft vs. hard cancel precedence matrix

**Decision.** Precedence rules governing cancel × state interactions. The
full matrix is in `04-cancellation-and-context.md` §2. The four load-bearing
rules are:

1. **Terminal state is immutable once entered.** Any cancel signal arriving
   after a terminal transition (`ApprovalRequired`, `BudgetExceeded`,
   `Failed`, `Completed`) is ignored. Terminal state cannot be overwritten
   by cancel.
2. **Soft cancel grace (500 ms) applies only to in-flight LLM calls and
   tool calls.** It does **not** apply to buffer-blocked channel sends. A
   stuck consumer is effectively a hard-cancel trigger.
3. **Budget breach always preempts soft cancel.** If a soft cancel and a
   budget dimension breach arrive at the same loop iteration,
   `BudgetExceeded` is the terminal — budget is a correctness invariant,
   soft cancel is a UX feature.
4. **Approval-required always preempts cancel.** If a soft cancel arrives
   while `PostHook` is evaluating an approval-required decision, the
   terminal is `ApprovalRequired`. The governance audit trail requires
   that approval checkpoints are not silently rewritten as cancellations.

**Rationale.** The asymmetry between "operation in flight" (gets grace) and
"terminal already entered" (no override) is the coherent invariant.
Allowing cancel to overwrite a governance terminal would produce an audit
trail that records `Cancelled` for an invocation that actually required
human review — a correctness bug in the governance model.

**Go version dependency.** Soft-cancel grace uses `context.WithoutCancel`
(Go 1.21+, available at the project minimum Go 1.23+) to derive a
non-cancellable context for in-flight operation completion, then layers a
500 ms `context.WithTimeout` on top.

---

## D22 — Terminal lifecycle-emission invariant (extends seed §4.5 to 5 terminals)

**Decision.** Every terminal state entry — `Completed`, `Failed`,
`Cancelled`, `BudgetExceeded`, `ApprovalRequired` — triggers a synchronous
terminal lifecycle event emission running on a context derived from
`context.Background()` with a bounded 5-second deadline. The parent
invocation context is **not** in the emission derivation chain;
cancellation of the invocation context cannot prevent the terminal event
from reaching the `telemetry.LifecycleEventEmitter`.

**Sequence per terminal.**

1. State machine transitions to terminal state (immutable).
2. Emission context derived from `context.Background()` +
   `WithTimeout(5s)`, with the invocation's OTel span attached via
   `trace.ContextWithSpanContext`. (See Concern C1 below — this span
   re-attachment is load-bearing for Phase 4.)
3. `telemetry.LifecycleEventEmitter.Emit(emissionCtx, terminalEvent)` is
   called synchronously on the loop goroutine.
4. If `Emit` returns within 5s: the terminal `InvocationEvent` is sent to
   the stream channel under the same emission context; channel is closed.
5. If `Emit` times out: the timeout is logged via `slog` at WARN level, the
   terminal `InvocationEvent` is still sent to the channel, and the channel
   is still closed. Emitter failure is non-fatal to the close protocol.

**Seed extension.** Seed §4.5 originally referenced four terminals. D22
extends the invariant to all five terminals (adding `ApprovalRequired`)
with no change to the 5s deadline or the derivation rule.

**The `internal/ctxutil` package** provides the single helper that
implements this derivation so the pattern cannot be reimplemented
incorrectly elsewhere. Phase 3 and implementation must route every terminal
emission through this helper.

**Trade-offs rejected.**

- Run emission on the original invocation context — defeats the invariant.
- Separate goroutine for emission — adds lifecycle complexity; 5s blocking
  on the loop goroutine is acceptable given it only delays channel close
  to the consumer.
- Make the 5s deadline caller-configurable — not justified in v1.0.

---

## D23 — Context propagation model: three layers plus detached emission

**Decision.** Context propagation follows four layers. The full table, rules,
and Mermaid diagram are in `04-cancellation-and-context.md` §3.

| Layer | Source | Derivation |
|---|---|---|
| 1 — Caller context | `ctx` passed to `Invoke`/`InvokeStream` | n/a |
| 2 — Invocation context | Layer 1 + `WithCancel` + `WithValue` (span, enricher values) | orchestrator holds the cancel handle |
| 3 — Operation context | Layer 2 + `WithTimeout` per operation | per LLM call / tool call / hook evaluation |
| 4 — Emission context | `context.Background()` + `WithTimeout(5s)` + span re-attached | terminal lifecycle emission only |

**Rules.**

- All `ctx.Done()` checks in the loop run against Layer 2, not Layer 1
  directly. The orchestrator holds the Layer 2 cancel handle so it can
  self-terminate on budget breach or approval transitions without waiting
  for the caller to cancel.
- Layer 3 operation contexts inherit Layer 2 cancellation. When soft-cancel
  grace is active, operations receive a `context.WithoutCancel`-derived
  context layered with `WithTimeout(500 ms)` so the in-flight operation can
  complete cleanly. This applies to LLM calls, tool calls, and
  `credentials.Resolver.Fetch` (see Concern C4).
- Layer 4 is independent of Layers 1–3 for cancellation purposes but carries
  the invocation's OTel span via `trace.ContextWithSpanContext` so the
  terminal lifecycle event remains linked to the invocation trace.

**Go version dependency.** `context.WithoutCancel` requires Go 1.21+.
Project minimum is Go 1.23+ — available.

**Trade-offs rejected.**

- Pass caller context directly to every operation — the orchestrator loses
  its self-terminate handle; budget and approval terminals become
  error-returns instead of context cancellations, breaking "every blocking
  op respects `ctx.Done()`".
- One context per state (fresh `WithValue` chain per transition) — allocates
  on the hot path, and value scoping mid-invocation is unnecessary.

---

## D24 — Concurrency model: one goroutine per invocation, sole-producer rule

**Decision.** The orchestrator runs **one state-machine loop goroutine per
invocation**. The loop goroutine is the sole producer on the stream
channel. There is no shared worker pool. There is no producer/consumer
split within a single invocation.

**Per-invocation topology.** Illustrated as a Mermaid diagram in
`05-concurrency-model.md` §2.

**`Invoke` path.** Spawns the loop goroutine, drains the channel internally,
blocks until terminal (channel close), returns `(InvocationResult, error)`.

**`InvokeStream` path.** Spawns the loop goroutine, immediately returns the
receive channel to the caller. Caller drains in its own goroutine(s). Loop
goroutine closes the channel when terminal per D19.

**Concurrent-safety guarantee of `AgentOrchestrator`.** A single
`AgentOrchestrator` instance serves N concurrent `Invoke`/`InvokeStream`
calls safely because:

1. All mutable state (state machine instance, budget snapshot, stream
   channel, invocation context) is per-invocation and allocated fresh on
   every call.
2. The orchestrator's injected dependencies (`llm.Provider`,
   `hooks.PolicyHook`, `tools.Invoker`, `budget.Guard`, etc.) are read-only
   after construction. Their implementations must be safe for concurrent
   use — this requirement is recorded in every interface's Phase 3 godoc.
3. Cancellation of one invocation's context is isolated to that
   invocation's loop goroutine. Other concurrent invocations are
   unaffected.

**Parallel tool calls (interaction with D06).** When the LLM returns
multiple tool calls and the provider advertises
`SupportsParallelToolCalls`, the loop goroutine dispatches them
concurrently using `golang.org/x/sync/errgroup` (recorded dependency —
see D24 trade-offs). The event emission order (revised during Phase 2
review to fix a naming-contract gap on `EventTypeToolCallStarted`) is:

1. Loop emits **all** `EventTypeToolCallStarted` events for the batch, in
   call-ID order, **before** dispatching any sub-goroutine. The "has
   started" semantic of the event name is honored.
2. Loop spawns N sub-goroutines via `errgroup`; each invokes
   `tools.Invoker.Invoke`.
3. Loop waits on `errgroup.Wait()`.
4. After the wait, the loop emits `EventTypeToolCallCompleted`,
   `EventTypePostToolFilterStarted`, `EventTypePostToolFilterCompleted`
   for each call ID in call-ID order.

The sole-producer invariant is preserved throughout. The residual
observability gap (per-tool completion ordering is not visible because
the loop waits for all sub-goroutines before emitting any `*Completed`
event) is documented as Concern C2 and must appear in Phase 3's
`InvocationEvent` godoc.

**Parallel errgroup partial-failure semantics.** If one sub-goroutine
returns an error while others are still running, `errgroup` cancels the
group's context, causing the remaining sub-goroutines' `Invoke` calls to
see a cancelled Layer 3 operation context. Per `tools.Invoker` semantics
(seed §5), the invoker returns the partial result or error. The loop then:

- Emits `EventTypeToolCallCompleted` for every call ID in the batch,
  including those that were cancelled mid-dispatch. A cancelled sub-tool's
  event carries the cancellation as a `TypedError` in a dedicated field
  (Phase 3 finalizes the shape).
- Transitions `ToolCall -> Failed` with the classified error from the
  originating failure. The transition is not `Cancelled`, because the
  failure was orchestrator-internal (a tool returned an error) rather than
  caller-driven.
- If a caller-driven soft cancel arrives concurrently with a tool failure,
  the tool failure wins — `ToolCall -> Failed` is the terminal, and the
  soft-cancel grace window does not apply because the loop is already
  terminating on the error path.

**Trade-offs rejected.**

- Shared orchestrator worker pool — introduces shared mutable state,
  cross-invocation backpressure coupling, and lifecycle management that
  does not map onto an invocation. Rejected for v1.0.
- Producer/consumer split within an invocation — adds one goroutine and one
  internal channel per invocation with no observable benefit; the loop
  goroutine must still wait on external backpressure to preserve event
  ordering.
- Sending tool events directly from per-tool goroutines — breaks the
  sole-producer rule and introduces non-deterministic event ordering.

---

## D25 — Budget wall-clock boundary

**Decision.**

- **Start:** wall-clock begins at entry to `Initializing`, the same instant
  the `PriceProvider` snapshot is taken (D26). `Created` is allocation-only
  and is excluded.
- **Stop:** wall-clock stops at terminal state entry (the moment the state
  machine transitions to any terminal). The 5-second terminal lifecycle
  emission window (D22) is **excluded** from the measured duration.

**Rationale.**

- Starting at `Initializing` (not `Created`) excludes orchestrator
  scheduling latency from the caller's budget — cleaner accounting.
- Stopping at terminal state entry (not at emission completion) prevents a
  meta-circular `BudgetExceeded` failure mode where the orchestrator's own
  5s emission bookkeeping tips an otherwise-within-budget invocation over
  its wall-clock limit.

**`ApprovalRequired` interaction.** Wall-clock stops the instant the state
machine enters `ApprovalRequired`. The caller's out-of-process approval wait
(minutes, hours, days) is not billed to this invocation. The resume
invocation — a separate `Invoke` call from the caller per D07 — has its own
wall-clock from its own `Initializing` entry.

**Token-cost dimension caveat.** Unlike wall-clock, tool-call count, and
cost in micro-dollars, the token dimension can only be checked after the
LLM call returns (the framework does not know output token counts before
the provider reports them). An invocation with a tight token budget may
exceed it by one LLM call's output-token contribution before the check
fires. Phase 4 must document this in `BudgetExceededError` semantics: the
`amount_exceeded` value may reflect a post-call discovery, not a pre-call
rejection. Recorded here to avoid Phase 4 re-discovering it. (Concern C3.)

---

## D26 — `PriceProvider` snapshot boundary

**Decision.** The `budget.PriceProvider` snapshot is taken **at entry to
`Initializing`**, co-located with the D25 wall-clock start. The snapshot is
immutable for the lifetime of the invocation.

This resolves the D08 hot-reload semantics (per-invocation snapshot, no
mid-invocation re-pricing) at the precise state-machine location. An
invocation that enters `LLMContinuation` after a provider price change
mid-flight still sees the original snapshot.

**Rationale.** Co-locating the snapshot with the wall-clock start gives the
invocation a single consistent baseline for both time and pricing, taken at
the first moment work begins. Any earlier (e.g., at `Created`) would snapshot
under allocation-latency conditions; any later (e.g., at `LLMCall`) would
risk mid-loop drift if an implementation re-reads the provider per turn.

---

## D27 — Zero-wiring streaming event set

**Decision.** With `llm.Provider` as the only injected dependency and all
other wiring at null defaults (D12), `InvokeStream` on a single-turn,
no-tools completion emits exactly 10 events in this order:

1. `EventTypeInvocationStarted`
2. `EventTypeInitialized`
3. `EventTypePreHookStarted`
4. `EventTypePreHookCompleted`
5. `EventTypeLLMCallStarted`
6. `EventTypeLLMCallCompleted`
7. `EventTypeToolDecisionStarted`
8. `EventTypePostHookStarted`
9. `EventTypePostHookCompleted`
10. `EventTypeInvocationCompleted`

Then channel close.

**Rationale.** Every state the path traverses emits at least one event.
Null hooks are called, return `Allow` immediately, and the bracket events
are still emitted — the event sequence is determined by state machine
structure, not by whether a hook is wired. Seed §3 principle 5 ("mandatory
observability, silent paths are a bug") applies at zero wiring.

**One-tool, two-turn zero-wiring path:** 18 events total (10 single-turn
events plus an inserted tool-cycle subsequence of 8). Full subsequence in
`03-streaming-and-events.md` §4.

**Trade-offs rejected.** Minimal-mode constructor that skips hook events
when hooks are null — rejected. Would create a behavioral split between
minimal and full orchestrators and conflict with the zero-wiring smoke-path
principle (D12).

---

## D28 — Property-based invariant list (21 invariants)

**Decision.** The v0.3.0 `gopter` property test suite commits to 21
invariants organized into three groups:

- **State-machine structural** (INV-01 through INV-10): transitions obey
  D16, terminals are immutable, completion paths visit required states,
  `ApprovalRequired` and `BudgetExceeded` reachability is bounded.
- **Streaming event** (INV-11 through INV-17): stream begins and ends with
  known events, exactly one terminal event, tool-cycle event brackets,
  D27 zero-wiring path is exact.
- **Terminal path** (INV-18 through INV-21): every terminal emits exactly
  one lifecycle event, `EventTypeBudgetExceeded` identifies the offending
  dimension, `EventTypeInvocationFailed` carries a `TypedError`.

Full text of each invariant and gopter targeting guidance (10k iterations
in CI, 100k in nightly per seed §10) is in `06-state-machine-invariants.md`.

**Trade-offs rejected.**

- Limiting to the seed §10 baseline — rejected. D15–D17 introduce new
  reachability constraints that must be tested.
- Writing invariants as code in Phase 2 — deferred to Phase 3 and
  implementation. Prose invariants are more amendable.

---

## D29, D30 — Reviewer reserves, released unused

D29 and D30 were held against surfacing of additional runtime decisions by
the reviewer subagent. Both released unused at phase close. The next phase
(Phase 3 — Interface Contracts) opens at D31.

---

## Concerns surfaced by the go-architect subagent (carried forward)

These are not decisions but constraints on Phase 3 and the implementation.
They are recorded here so the next phase does not re-discover them.

**C1 — OTel span re-attachment in the terminal emission context.**
The D22/D23 Layer 4 emission context is derived from
`context.Background()` and therefore does not carry the invocation's OTel
span automatically. The `internal/ctxutil` helper must attach the span via
`trace.ContextWithSpanContext` before passing the emission context to
`telemetry.LifecycleEventEmitter.Emit`. If this is not done, terminal
lifecycle events arrive at the collector without a parent span and are
unattributable in distributed trace UIs. Phase 4 must design its span tree
contract with this helper as load-bearing.

**C2 — Parallel tool-call event ordering visibility gap.** Under D24, events
for a parallel tool-call batch are not sent until the slowest tool in the
batch returns. Consumers building real-time progress UIs will not see per-
tool events interleaved with batch execution — the batch is observed as a
unit. This is correct for v1.0 (simplicity + determinism); Phase 3's
`InvocationEvent` godoc must state this explicitly.

**C3 — Token-cost budget dimension is inherently post-call.** Documented
under D25. Phase 4's `BudgetExceededError` model must state that the token
dimension may overshoot by up to one LLM call's output-token contribution
before the breach is detected.

**C4 — Soft-cancel grace must pass `context.WithoutCancel` to credentials
resolution.** During the 500 ms soft-cancel grace window, the loop
goroutine must pass a `context.WithoutCancel`-derived context (layered
with the 500 ms timeout) to any in-flight operation that respects context
cancellation — including `credentials.Resolver.Fetch`. Without this, a soft
cancel is effectively a hard cancel for any tool call requiring a
credential fetch, because the resolver will see the cancelled parent
context and fail immediately. Phase 5 must document this explicitly in the
`credentials.Resolver` contract.

**C5 — `golang.org/x/sync/errgroup` is a recorded dependency.** Used for
parallel tool-call dispatch under D24. In-scope per the project's stance
on `golang.org/x/` extensions. Recording the choice here avoids
re-litigation during implementation.
