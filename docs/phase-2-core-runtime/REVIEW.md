# Review: Phase 2 — Core Runtime Design

## Overall Assessment

Phase 2 delivers a coherent and precise runtime contract for the `praxis`
invocation kernel. The 14-state machine reconciles the seed's internal
count discrepancy cleanly, the 19-event streaming taxonomy is fully
specified against the state transition graph, and the cancellation,
context propagation, and concurrency models are defined at a granularity
Phase 3 can consume without re-opening runtime questions. The in-loop
reviewer subagent surfaced eight findings (four IMPORTANT, four MINOR),
all of which have been addressed in the artifacts before this review.

## Critical Issues

None.

## Important Weaknesses

1. **Parallel tool-call per-completion ordering visibility is a residual
   v1.0 limitation** (Concern C2, `01-decisions-log.md` D24,
   `05-concurrency-model.md` §4). Even after the reviewer-driven fix
   (emitting all `EventTypeToolCallStarted` events **before** dispatch
   rather than after), consumers still cannot tell which tool in a
   parallel batch finished first from the event stream — all `*Completed`
   events arrive in a burst after the slowest tool returns. This is
   documented and justified as a sole-producer-invariant trade-off, but
   Phase 3's `InvocationEvent` godoc must state it explicitly or v1.0
   users will be surprised. Not blocking, but must not be forgotten.

2. **Token-dimension budget overshoot is inherent, not a bug** (Concern
   C3, `01-decisions-log.md` D25 line 548, `04-cancellation-and-context.md`
   §5.4). The framework can only check token cost after the LLM call
   returns, so a tight token budget can be exceeded by one call's output
   tokens. This is correctly flagged as a Phase 4 documentation
   requirement for `BudgetExceededError`, but it is the only budget
   dimension where "over-budget-by-design" is possible — Phase 4 must not
   let this slip.

3. **Layer 4 terminal emission span re-attachment is load-bearing for
   Phase 4** (Concern C1, `01-decisions-log.md` D22, `internal/ctxutil`
   helper). If `internal/ctxutil.DetachedWithSpan` is not implemented
   correctly, terminal lifecycle events arrive at the collector without
   parent spans and are unattributable in distributed trace UIs. This is
   a single code path in `internal/ctxutil`, which is the correct place
   for it, but Phase 4's OTel span tree design must treat the helper as
   a load-bearing pre-requisite rather than an implementation detail.

4. **`hooks.PolicyHook.Decision` type shape is a Phase 3 decision with
   Phase 2 constraints.** D17 (as amended during review) pins three
   requirements on the `Decision` type: approval-required must be a
   value-variant, not an error; it may carry opaque caller metadata; it
   must support at minimum `Allow`/`Deny`/`RequireApproval`. Phase 3 must
   honor all three. The constraint section in D17 is clear, but it lives
   inside a decision log entry rather than a dedicated "Phase 3 handoff"
   section, so Phase 3's plan-phase run must read the full D17 text.

## Open Questions

1. **Under parallel errgroup dispatch, can a mid-dispatch tool failure
   produce a `Cancelled` outcome instead of `Failed`?** D24 states the
   terminal is `Failed` when a tool returns an error. But the errgroup
   context cancellation of remaining tools will cause those tools to see
   `context.Canceled` and return that as an error via `Invoke`. If the
   `errors.Classifier` categorizes such errors as `CancellationError`,
   could the terminal classifier produce `Cancelled` instead of `Failed`?
   Phase 3 or Phase 4 should confirm the classifier respects the "first
   failure wins" rule and does not let cancellation bubble out of a
   cascaded sub-goroutine into the terminal classification.

2. **What is the concrete Go shape of `BudgetSnapshot` given it appears
   on every `InvocationEvent`?** D28 INV-20 and D25 reference a
   `BudgetSnapshot` struct with per-dimension values and an
   `ExceededDimension` field, but the struct is Phase 3 work. The high
   event frequency (once per state transition) means the struct must be
   value-copyable and cheap to allocate. Phase 3's first task on this is
   ensuring it does not become a pointer-heavy type.

3. **Does resume of an `ApprovalRequired` invocation use a new `Invoke`
   call with a re-seated conversation, or is there a first-class
   `Resume` API?** D07 (Phase 1) says the caller owns persistence and
   resume, and the D17 snapshot constraint in this phase confirms the
   caller receives the full message history and configuration. This
   implies resume is a fresh `Invoke`. Phase 3 should explicitly state
   this — otherwise a reader might assume a dedicated `Resume` method is
   coming.

## Decoupling Contract Check

**PASS.**

Banned-identifier grep run against all seven Phase 2 artifacts for:
`custos`, `reef`, `governance_event`, `GovernanceEvent`, `org.id`,
`agent.id`, `user.id`, `tenant.id`, milestone codes, consumer file paths.

Four occurrences found, all are negation-mentions inside compliance
declarations:

- `00-plan.md:190` — "A single event name like `GovernanceEventX` would
  fail the banned-identifier grep" (reviewer risk statement).
- `03-streaming-and-events.md:276-278` — §7 compliance declaration
  explicitly naming the banned identifiers to assert absence.

No identifier is used as an actual identifier anywhere in the artifacts.
No consumer brand name appears. No hardcoded attribute key appears in any
event type, state name, struct field, or snapshot shape. The
`EventType*` prefix under the `praxis.*` namespace honors seed §6.2.

Caller-specific attribution reaches the stream exclusively via
`telemetry.AttributeEnricher` (Phase 4), which is an interface the
framework does not inspect.

## Recommendations

- **Carry Concerns C1–C5 forward into Phase 3 and Phase 4 planning as
  explicit inputs.** The decisions log §"Concerns surfaced by the
  go-architect subagent" section is the right location, but Phase 3 and
  Phase 4 `plan-phase` runs should read it first.
- **Phase 3 must read D17's Decision-type constraint block in full.** The
  three pinned requirements (value-variant, opaque metadata forwarding,
  `Allow`/`Deny`/`RequireApproval` minimum) are not redundant with the
  seed §5 interface sketch.
- **Phase 4 must treat `internal/ctxutil.DetachedWithSpan` as load-bearing
  for the OTel span tree contract.** A single helper is the entire
  attribution mechanism for terminal lifecycle events.
- **The conversation snapshot Go type placement is a Phase 3 decision**
  with Phase 5 redaction assessment. Flag it as joint work in the Phase 3
  plan so the type is not invented in `orchestrator/` without Phase 5 seeing
  the content first.
- **Update `docs/roadmap-status.md`** to reflect Phase 2 approval and the
  next phase (Phase 3 — Interface Contracts). Decision ID range opens at
  D31 (D29/D30 released unused per Phase 2 close).
- **Delete `tmp-reviewer-report.md`** after this review is written.

## Verdict: READY

All stated Phase 2 exit criteria are met: D15–D28 are adopted with
rationale and amendment notes, the 14-state machine is canonical and
Mermaid-rendered, the 19 `InvocationEvent` types are enumerated and
namespace-clean, the cancel/context/concurrency boundaries are precise
enough for Phase 3 to generate interface signatures, the 21 property-test
invariants trace back to specific decisions, and the decoupling contract
grep passes. The four IMPORTANT reviewer findings have been addressed
in-place; the residual weaknesses are documentation and coordination
concerns for later phases, not open runtime questions.
