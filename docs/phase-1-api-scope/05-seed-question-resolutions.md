# Phase 1 — Seed Open Question Resolutions

The seed document (`docs/PRAXIS-SEED-CONTEXT.md` §13) carries four open
questions forward from the pre-extraction planning phase into praxis
Phase 1. This document answers each, one-to-one, and cross-references the
numbered decision in `01-decisions-log.md` where the full rationale lives.

Each section below restates the seed question verbatim and then gives the
Phase 1 answer. For the reasoning and alternatives considered, follow the
decision-log pointer.

---

## Seed §13.1 — Final Go type name and signature for `tools.Invoker`

**Seed question.** *"The interface shape is locked conceptually, but the
exact method name, the precise `InvocationContext` struct it receives, and
whether the tool name lives on the context or the call struct are open.
This matters because `tools.Invoker` is the single seam through which any
consumer connects its own tool execution infrastructure, and its shape
cannot drift after v1.0."*

**Phase 1 answer.** The tool name lives on the `ToolCall` struct passed
into `tools.Invoker.Invoke`. `InvocationContext` carries invocation-level
state (budget, span, policy decision carry-through) and does not carry the
tool name or per-call routing information.

**Decision reference.** [`D06` in `01-decisions-log.md`](01-decisions-log.md#d06--toolsinvoker-tool-name-placement-resolves-seed-131).

**Residual for Phase 3.** The exact method name (`Invoke` vs `Execute` vs
`Call`) and the precise `InvocationContext` field set are delegated to
Phase 3 (Interface Contracts). Phase 1 decides the placement question only,
because that is the question that has semantic consequences for concurrency.

---

## Seed §13.2 — Semantics of `requires_approval` in hook results

**Seed question.** *"When a `PolicyHook` returns a decision that says 'this
invocation can proceed only after a human approves', the framework has two
plausible options: stall the invocation in-process and surface an
approval-pending event to the caller, or defer the stall entirely to the
caller and return an error-like decision that the caller handles. Each
choice has implications for cancellation, budget accounting, and the
channel contract in `InvokeStream`. One must be chosen and documented."*

**Phase 1 answer.** The orchestrator returns a structured "approval
required" decision to the caller. The invocation transitions to a terminal
state (sibling of `Failed`, `Cancelled`, `BudgetExceeded`), emits its
terminal lifecycle event, closes the `InvokeStream` channel cleanly, and
returns a typed `ApprovalRequiredError` (`errors.As`-compatible). The
caller owns persistence, human notification, and resume. Each resume is a
new invocation with its own state machine, span tree, and budget.

**Decision reference.** [`D07` in `01-decisions-log.md`](01-decisions-log.md#d07--requires_approval-hook-result-semantics-resolves-seed-132).

**Implications propagated forward.**
- **Phase 2 (Core Runtime).** Adds `ApprovalRequired` as a terminal state
  (or expresses the equivalent via a typed error on the same path as
  `Failed`; the exact representation is a Phase 2 call). Raises terminal
  state count from four to five, or keeps it at four with the typed-error
  representation.
- **Phase 2 (Streaming model).** The `InvokeStream` channel contract is
  unchanged: the channel closes on reaching the new terminal state, exactly
  as it does for `Completed`, `Failed`, `Cancelled`, and `BudgetExceeded`.
- **Phase 2 (Cancellation).** No new semantics. Approval-required
  invocations are terminal; there is nothing to cancel mid-approval because
  the orchestrator has released its hold on the invocation by then.
- **Phase 4 (Lifecycle events).** Adds a new lifecycle event constant for
  the approval-required terminal transition, under the neutral
  `praxis.Event*` namespace.
- **Phase 4 (Error taxonomy amendment).** `ApprovalRequiredError` becomes
  the eighth concrete type implementing `errors.TypedError` (seed §5
  declares seven; D07 amends to eight). It is classified as non-retryable
  and shares the retry posture of `PolicyDeniedError`. Phase 4 inherits
  this amendment via D07.

---

## Seed §13.3 — `budget.Guard` / `budget.PriceProvider` hot-reload semantics

**Seed question.** *"The first implementation assumes prices are loaded at
orchestrator construction time and are stable for the process lifetime.
Whether the interface should support mid-process pricing updates (e.g., a
provider mid-quarter price cut), and if so whether updates apply only to
new invocations or also to in-flight ones, is open."*

**Phase 1 answer.** Per-invocation snapshot. `PriceProvider` is consulted
at invocation start for the `(provider, model, direction)` tuples required
by that invocation, and the resolved rates are cached in
`InvocationContext` for the life of the invocation. All token accounting
within the invocation uses the cached rates. Mid-process price updates
(a new `PriceProvider` state or a refreshed `StaticPriceProvider`) take
effect for **new invocations only**; in-flight invocations are never
re-priced. There is no framework-level notification to in-flight
invocations that prices have changed.

**Decision reference.** [`D08` in `01-decisions-log.md`](01-decisions-log.md#d08--budgetpriceprovider-hot-reload-policy-resolves-seed-133).

**Implications propagated forward.**
- **Phase 3 (Interface Contracts).** `budget.PriceProvider` method surface
  settles on a simple lookup method; no re-read, no change notification,
  no subscription interface. This is the condition that promotes
  `PriceProvider` from `stable-v0.x-candidate` to `frozen-v1.0` at the
  Phase 3 close (see `04-v1-freeze-surface.md`).
- **Phase 4 (Observability).** Final `cost_estimate_micros` reported on
  terminal lifecycle events is reproducible from the snapshot cached in
  the invocation — a property Phase 4's audit-trail design can rely on.

---

## Seed §13.4 — Confirm the no-plugins position for v1

**Seed question.** *"The design locks 'no plugin system in v1', with
extension via Go interface implementation as the only extension path.
This is the correct default, but it should be re-confirmed explicitly at
phase 1 review so that future 'should we add a plugin system' discussions
have a clean artifact to point at."*

**Phase 1 answer.** Re-confirmed. praxis v1.x will not ship any plugin,
extension, or dynamic loading mechanism. No `plugin.Open`, no WASM host,
no reflection registry, no RPC-based extensibility, no runtime-loaded
third-party code path. Every extension point is a Go interface defined in
the stable surface (`docs/PRAXIS-SEED-CONTEXT.md` §5). Consumers extend
the library by implementing those interfaces in their own packages and
injecting them at `AgentOrchestrator` construction time.

**Decision reference.** [`D09` in `01-decisions-log.md`](01-decisions-log.md#d09--re-confirmation-of-no-plugins-in-v1-resolves-seed-134).

**Forward record.** This document and `01-decisions-log.md` D09 are the
artifact that future RFCs proposing a v2+ plugin system must cite and
address explicitly. No plugin system will be added in v1.x under any
circumstances.

---

## Summary

| Seed question | Phase 1 decision | Status |
|---|---|---|
| §13.1 — `tools.Invoker` tool-name placement | D06 — on `ToolCall` | decided |
| §13.2 — `requires_approval` semantics | D07 — returned-to-caller, terminal | decided |
| §13.3 — `PriceProvider` hot-reload | D08 — per-invocation snapshot | decided |
| §13.4 — No plugins in v1 | D09 — re-confirmed | decided |

All four seed open questions have an adopted working resolution. Phase 2
(Core Runtime Design) proceeds from these positions; any of them may be
revisited via the amendment protocol in `01-decisions-log.md` if later
phases surface a concrete reason to reopen.
