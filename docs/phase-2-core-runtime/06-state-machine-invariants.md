# Phase 2 — State Machine Invariants

**Scope:** property-based invariant set for the `praxis` v0.3.0 `gopter`
test suite (seed §10).
**Binds:** D28 in `01-decisions-log.md`.
**Consumers:** the v0.3.0 implementation, the property-test authors, and
Phase 3's go-architect (who will translate these into gopter generators).

---

## 1. Overview

This document pins the full invariant set the state machine and streaming
event system must satisfy under property-based testing. The set has 21
invariants organized into three groups:

- **Group A — State machine structural** (INV-01 through INV-10): the
  transition graph obeys D16, terminal states are immutable, and
  `ApprovalRequired` / `BudgetExceeded` reachability is bounded.
- **Group B — Streaming event** (INV-11 through INV-17): the event stream
  begins and ends with known events, tool-cycle brackets are preserved,
  and the D27 zero-wiring path matches exactly.
- **Group C — Terminal path** (INV-18 through INV-21): every terminal emits
  exactly one lifecycle event, and terminal events carry the fields the
  contract promises.

Each invariant carries a source tag: `[seed]` (from seed §10 baseline),
`[D15]` / `[D16]` / `[D17]` / `[D18]` / `[D27]` (introduced or extended by
that Phase 2 decision).

Seed §10 mandates 10,000 iterations in CI and 100,000 in nightly. Phase 2
does not change these numbers.

---

## 2. Group A — State machine structural invariants

| ID | Invariant | Source |
|---|---|---|
| INV-01 | Every transition accepted by a `state.Machine` instance is listed in the D16 adjacency table for its source state. Any `Transition(src, dst)` call where `dst` is not in the allow-list for `src` returns an error and does not modify the instance. | [seed] + [D16] |
| INV-02 | Once a `state.Machine` instance enters any terminal state, no further transition succeeds. Any subsequent `Transition` call returns an error. Terminal state is immutable for the lifetime of the instance. | [seed] + [D15] |
| INV-03 | Every path from `Created` that reaches `Completed` visits `PreHook`, `LLMCall`, and `PostHook` at least once. No path can complete without these three states on the path. | [seed] |
| INV-04 | `PreHook` is visited **exactly once** per invocation. It is never re-entered after the first exit. This is the "PreHook runs exactly once" invariant from seed §4.2. | [seed] |
| INV-05 | No path reaches `LLMCall` before visiting `Initializing` and `PreHook` (in that order). `LLMCall` is unreachable from `Created`, `Initializing`, or directly after `Created`. | [seed] |
| INV-06 | The set of terminal states of any reachable invocation is exactly `{Completed, Failed, Cancelled, BudgetExceeded, ApprovalRequired}`. Every path that terminates exits through exactly one of these states. No other terminal state exists. | [D15] + [D17] |
| INV-07 | `ApprovalRequired` is reachable only from `PreHook` or `PostHook`. No other source state has a legal transition to `ApprovalRequired`. | [D16] + [D17] |
| INV-08 | `BudgetExceeded` is reachable only from `LLMCall`, `ToolDecision`, or `LLMContinuation`. No other source state has a legal transition to `BudgetExceeded`. | [D16] |
| INV-09 | `Cancelled` is not reachable from `Created`, `Initializing`, `PreHook`, or any terminal state. | [D16] |
| INV-10 | The tool cycle `ToolDecision -> ToolCall -> PostToolFilter -> LLMContinuation -> ToolDecision` may repeat any finite number of times. The state machine itself imposes no maximum cycle count; cycle budgeting is enforced by the budget tool-call-count dimension, not by the state machine structure. | [D16] |

### Group A targeting guidance

Gopter generators should:

- Generate random sequences of `Transition` calls on fresh
  `state.Machine` instances and verify INV-01 holds for every call,
  INV-02 holds after every terminal entry, and illegal calls return an
  error without side effects.
- Enumerate all acyclic paths from `Created` to each terminal in the D16
  graph and verify INV-03, INV-06, INV-07, INV-08, INV-09 hold for every
  enumerated path.
- For INV-04, generate paths that attempt to re-enter `PreHook` (e.g., via
  synthetic illegal transitions) and assert rejection.
- For INV-10, generate paths with arbitrary cycle counts (N up to some
  large bound) and verify the state machine accepts them structurally.
  Budget enforcement is out of scope for this invariant — it is tested
  separately.

---

## 3. Group B — Streaming event invariants

These invariants require a mock `llm.Provider` and a mock `tools.Invoker`
to drive the loop to produce event sequences. They are tested against the
output of `AgentOrchestrator.InvokeStream`.

| ID | Invariant | Source |
|---|---|---|
| INV-11 | Every `InvokeStream` invocation begins with `EventTypeInvocationStarted` as the first event received on the channel. | [D18] |
| INV-12 | Every `InvokeStream` invocation ends with exactly one terminal event (`EventTypeInvocationCompleted`, `EventTypeInvocationFailed`, `EventTypeInvocationCancelled`, `EventTypeBudgetExceeded`, or `EventTypeApprovalRequired`) received immediately before channel close. (Exception: a consumer that stalls for >5s during terminal emission may observe a bare close; this is the D22 degraded-consumer path. Property tests use responsive consumers, so the exception does not fire.) | [D18] |
| INV-13 | No events are received on the channel after close. The channel is closed exactly once for the lifetime of the `InvokeStream` return value. | [D18] + [D19] |
| INV-14 | For every tool call ID `C` that appears on `EventTypeToolCallStarted`, a matching `EventTypeToolCallCompleted{C}` appears later in the stream, unless a terminal event intervenes (e.g., cancel mid-tool). | [D18] |
| INV-15 | All tool-cycle events for a given call ID (`EventTypeToolCallStarted`, `EventTypeToolCallCompleted`, `EventTypePostToolFilterStarted`, `EventTypePostToolFilterCompleted`) appear in that order and before the next `EventTypeLLMContinuationStarted` on the stream. | [D18] |
| INV-16 | `EventTypePreHookStarted` appears exactly once per stream. `EventTypePreHookCompleted` appears at most once and does not appear if the stream terminates during or before `PreHook -> LLMCall` (e.g., `PreHook -> Failed` or `PreHook -> ApprovalRequired`). | [D18] |
| INV-17 | On the zero-wiring, one-turn, no-tools path (`llm.Provider` injected, everything else null), the stream contains **exactly 10 events** in the order specified in D27: `InvocationStarted, Initialized, PreHookStarted, PreHookCompleted, LLMCallStarted, LLMCallCompleted, ToolDecisionStarted, PostHookStarted, PostHookCompleted, InvocationCompleted`. | [D27] |

### Group B targeting guidance

Gopter generators should:

- Drive a mock `llm.Provider` that returns randomized responses: one-turn
  (no tool calls), multi-turn (N tool calls per response, sequential),
  multi-turn with a parallel batch (N tool calls in a single response).
- Drive a mock `tools.Invoker` that returns results with randomized
  latency to exercise the sole-producer batch-emission rule under
  parallel dispatch (C2).
- Record the sequence of events received on the channel and verify each
  invariant against the recording.
- Exercise terminal divergence by injecting policy denials, budget
  breaches, cancellations, and approval-required decisions at random
  points.
- For INV-17, pin the exact zero-wiring case as a targeted test, not a
  fuzz: set `llm.Provider` to return a single completion with no tool
  calls and assert the 10-event sequence exactly.

---

## 4. Group C — Terminal path invariants

| ID | Invariant | Source |
|---|---|---|
| INV-18 | Every terminal path emits exactly one `telemetry.LifecycleEventEmitter` terminal event — regardless of whether the terminal is `Completed`, `Failed`, `Cancelled`, `BudgetExceeded`, or `ApprovalRequired`, and regardless of whether the parent context was cancelled. The emission runs on the Layer 4 emission context (D22) with a 5s deadline. | [seed] + [D17] + [D22] |
| INV-19 | `EventTypeApprovalRequired`, when it appears, carries a non-empty `InvocationID`, a non-zero `At` timestamp, and a non-nil conversation snapshot for caller-owned resume (D07). | [D17] |
| INV-20 | `EventTypeBudgetExceeded`, when it appears, carries a `BudgetSnapshot` whose `ExceededDimension` field identifies exactly one of the four budget dimensions (wall-clock, tokens, tool-call count, cost). | [D18] |
| INV-21 | `EventTypeInvocationFailed`, when it appears, carries a non-nil `Err` that satisfies `errors.As(err, &typedErr)` for some concrete `errors.TypedError` type from the Phase 3 taxonomy. | [D18] |

### Group C targeting guidance

Gopter generators should:

- For INV-18, drive the loop to every terminal state and verify the
  `LifecycleEventEmitter` mock receives exactly one terminal event per
  invocation, including when the parent context is cancelled during
  non-terminal execution.
- For INV-19, drive a `PolicyHook` mock that returns approval-required at
  `PreInvocation` and at `PostInvocation` phases; verify the
  `EventTypeApprovalRequired` fields are populated.
- For INV-20, drive a `budget.Guard` mock that breaches each dimension in
  turn and verify the correct dimension is reported.
- For INV-21, inject failures at each non-terminal state and verify the
  `Err` field is typed correctly per seed §5 and Phase 4's taxonomy.

---

## 5. Invariants deliberately **not** in the list (for v0.3.0)

The following candidate invariants were considered and excluded. They are
recorded here so Phase 3 and Phase 4 do not re-propose them without an
amendment.

- **"No two invocations emit the same `InvocationID`."** Uniqueness is a
  quality-of-identifier concern, not a state machine invariant. It is the
  responsibility of whatever generator the orchestrator uses
  (UUIDv7 or similar, decided in Phase 3). Testing this belongs with the
  ID generator, not the state machine.
- **"Every event carries a monotonically increasing `At` timestamp."**
  System clocks are non-monotonic in general; `At` uses `time.Now()` which
  is subject to NTP adjustments. The correct invariant — "events within a
  stream appear in causal order" — is already covered by INV-11 through
  INV-17, which test event ordering independent of wall-clock timestamp.
- **"Budget checks are called at every transition."** This is an
  implementation detail, not a state machine structural property. Phase 4
  may add an observability invariant (e.g., "every state transition
  produces a `BudgetSnapshot` on the emitted event") but it belongs there.
- **"Spans are closed in LIFO order."** This is a Phase 4 OTel invariant,
  not a Phase 2 runtime invariant. Included in Phase 4's scope.

---

## 6. Cross-reference map

For ease of tracing each invariant back to its driving decision:

- **D15** (state list) → INV-02, INV-06
- **D16** (transition allow-list) → INV-01, INV-07, INV-08, INV-09, INV-10
- **D17** (ApprovalRequired placement) → INV-06, INV-07, INV-18, INV-19
- **D18** (event enumeration) → INV-11 through INV-15, INV-20, INV-21
- **D19** (close protocol) → INV-13
- **D22** (terminal emission) → INV-18
- **D27** (zero-wiring path) → INV-17
- **seed §10** (baseline property tests) → INV-03, INV-04, INV-05, INV-18
- **seed §4.2** ("PreHook runs exactly once") → INV-04

No invariant depends on a decision outside the Phase 2 range or the seed.
