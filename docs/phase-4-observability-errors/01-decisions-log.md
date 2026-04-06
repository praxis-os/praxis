# Phase 4 — Decisions Log

**Phase:** 4 — Observability and Error Model
**Decision range:** D53–D66
**Status:** adopted working position

---

## Amendability

All decisions in this log follow the amendment protocol in
`docs/phase-1-api-scope/01-decisions-log.md`. They are working positions, not
immutable commitments. Later phases that discover contradictions or missed cases
may open new decision IDs with `Supersedes:` annotations.

---

## D53 — OTel span tree structure

**Status:** decided
**Summary:** One root span per invocation; child spans for I/O-bound and
policy-evaluation phases; synchronous check phases share the parent span.

**Decision.** The span tree is structured as follows:

- **Root span** (`praxis.invocation`): created at `Initializing` state entry,
  closed at terminal state entry (after the terminal lifecycle event is emitted
  and the OTel span status is set). The root span carries the full invocation
  attribute set, enricher attributes, and terminal status.

- **Child spans for I/O-bound and policy-evaluation phases:**
  - `praxis.prehook` — created at `PreHook` entry, closed at `PreHook` exit
  - `praxis.llmcall` — created at `LLMCall` entry, closed at `LLMCall` exit
  - `praxis.toolcall` — created at `ToolCall` entry (one per tool call in a
    batch, all created before dispatch per D24), closed when the tool goroutine
    completes. Under parallel dispatch, each tool call has its own child span.
  - `praxis.posttoolfilter` — created at `PostToolFilter` entry, closed at
    `PostToolFilter` exit (one span per tool result in the filter loop)
  - `praxis.llmcontinuation` — created at `LLMContinuation` entry, closed at
    `LLMContinuation` exit (captures budget re-check + message assembly)
  - `praxis.posthook` — created at `PostHook` entry, closed at `PostHook` exit

- **No child span for `ToolDecision`:** `ToolDecision` is a synchronous
  in-process check (budget arithmetic, tool-call inspection). It produces no
  I/O and runs in sub-microsecond time. Instrumenting it with a span adds
  allocation pressure on the hot path without adding diagnostic value. The
  `EventTypeToolDecisionStarted` lifecycle event is sufficient observability.
  See `02-span-tree.md` for rationale details.

- **No child span for `Created`:** `Created` is the allocation state before
  any work begins. The root span is opened at `Initializing` entry.

**Span status mapping:**

| Terminal state | OTel span status |
|---|---|
| `Completed` | `StatusOK` |
| `Failed` | `StatusError` (description: `error_kind` attribute value) |
| `Cancelled` | `StatusError` (description: `"cancellation"`) |
| `BudgetExceeded` | `StatusError` (description: `"budget_exceeded"`) |
| `ApprovalRequired` | `StatusOK` (not a failure; a deliberate pause) |

**Rationale.** `ApprovalRequired` is semantically equivalent to a successful
checkpoint, not an error. Setting it to `StatusError` would pollute error-rate
dashboards with intentional approval pauses. Callers who need to distinguish
`Completed` from `ApprovalRequired` can filter on the `praxis.terminal_state`
span attribute.

See `02-span-tree.md` for the complete attribute sets and Mermaid diagrams.

---

## D54 — C1 resolution: `DetachedWithSpan` contract

**Status:** decided
**Summary:** `DetachedWithSpan` carries the full `trace.Span` (not just
`SpanContext`) so the terminal lifecycle emitter can add attributes before the
span is ended. It accepts a `deadline` parameter for testability.

**Decision.** The `internal/ctxutil.DetachedWithSpan` function accepts a
`trace.Span` and a `deadline time.Duration`. It returns a new context derived
from `context.Background()` (so cancellation is detached) with:

1. A deadline of the given duration (the caller passes 5 seconds for
   production use, per D22/D23; tests may pass shorter values).
2. The full `trace.Span` re-attached via `trace.ContextWithSpan`.

The full span (not merely `SpanContext`) is carried so that:

- The terminal lifecycle event emitter can call `span.SetAttributes(...)` to
  add terminal-phase attributes (e.g., `praxis.terminal_state`,
  `praxis.error_kind`) before the span is ended.
- The `LifecycleEventEmitter` implementation can access the live span via
  `trace.SpanFromContext(ctx)` without special handling.
- The emitter can record an exception on the span for error terminals.

**Why not just `SpanContext`?** If only `SpanContext` is stored, the terminal
emitter cannot write attributes to the span. The span would be closed from the
non-detached context path without the terminal attributes, and those attributes
would be missing from the exported trace. Losing terminal attributes is a
meaningful diagnostic regression.

**Why the full Span does not risk a goroutine leak:** the Span reference is
passed into the Layer 4 context only for the duration of the terminal emission
sequence. After `Emit` returns, the orchestrator calls `span.End()` and drops
the reference. The deadline on the Layer 4 context caps the window.

**Why `deadline` is a parameter (go-architect recommendation):** the
go-architect validation recommended making the deadline injectable rather than
hardcoding 5 seconds inside the function. This improves testability (tests can
pass 1ms to avoid slow-test penalties) and avoids embedding a magic number in
an internal helper. The 5-second production value is a constant in the
orchestrator's loop code, not in `ctxutil`.

**Contract:**

```go
// DetachedWithSpan returns a context derived from context.Background()
// with the given deadline and the given span re-attached. The returned
// context is not cancelled when parent is cancelled, enabling terminal
// lifecycle emission after the caller's context is done.
//
// The caller is responsible for calling span.End() after the detached
// context has been used for terminal emission. DetachedWithSpan does not
// call span.End().
//
// The production caller passes 5*time.Second as the deadline (D22).
// Test callers may pass shorter values for fast-test execution.
//
// Package: internal/ctxutil
func DetachedWithSpan(span trace.Span, deadline time.Duration) (context.Context, context.CancelFunc)
```

Note: `parent` context is intentionally not a parameter. The detached context
always starts from `context.Background()` to guarantee isolation from caller
cancellation. Any baggage or values the orchestrator needs in the Layer 4
context must be copied explicitly before calling `DetachedWithSpan`. The
returned `CancelFunc` must be deferred by the caller to release the timer.

---

## D55 — CP1 nested span semantics: child span vs. span link

**Status:** decided
**Summary:** Nested orchestrator invocations use span links, not child spans.
The inner orchestrator creates its own root span with a link to the outer span.

**Decision.** When a `tools.Invoker` implementation internally calls a nested
`AgentOrchestrator`, the inner invocation opens a new root span
(`praxis.invocation`) that carries a `SpanLink` to the outer invocation's span
context (`tools.InvocationContext.SpanContext`, defined in CP1).

The inner span is **not** a child of the outer span. Rationale:

1. **Lifetime mismatch.** Child spans must end before their parent. An inner
   invocation may outlive the outer span's context window (e.g., with approval
   pause), making child-of semantics incorrect.
2. **Backend compatibility.** Some tracing backends impose depth limits on
   span trees. Arbitrarily nested orchestrators with child-of semantics could
   hit those limits or produce misleadingly deep trees.
3. **Fan-out clarity.** When multiple tool calls each spawn inner
   orchestrators, child-of semantics produce a deep, sequential-looking tree.
   Span links express the "triggered by" relationship while allowing backends
   to render each inner invocation independently.

**Link attribute:** the link carries the attribute `praxis.link_kind = "nested_invocation"` to distinguish from other link uses (e.g., async
resume).

**Implementation note:** the outer orchestrator does not create the inner span.
The inner orchestrator's initialization path reads `SpanContext` from
`tools.InvocationContext` and attaches the link when starting the root span.

---

## D56 — CP2 `parent_invocation_id` propagation

**Status:** decided
**Summary:** `parent_invocation_id` is a framework-injected span attribute on
the inner invocation's root span. The framework reads it from
`tools.InvocationContext.ParentInvocationID` (set by the outer orchestrator).

**Decision.** The framework sets the span attribute
`praxis.parent_invocation_id` on the inner invocation's root span if
`tools.InvocationContext.ParentInvocationID` is non-empty. This is a
framework-defined attribute, not an `AttributeEnricher` concern.

The `InvocationContext.ParentInvocationID` field is populated by the outer
orchestrator before calling the `tools.Invoker`. Callers who bypass the outer
orchestrator and invoke nested orchestrators directly are responsible for
populating this field.

`parent_invocation_id` is a span attribute only. It is never a Prometheus
metric label (unbounded cardinality; see D57 cardinality budget).

The `InvocationEvent.InvocationID` field on the inner invocation's events
carries the inner invocation's own ID. Consumers correlating inner and outer
invocations use the span link (D55) and the `praxis.parent_invocation_id`
attribute on the inner root span.

---

## D57 — Prometheus metric set

**Status:** decided
**Summary:** 10 metrics with `praxis_` prefix; label cardinality cap; no
`AttributeEnricher` attributes on metric labels.

**Decision.** The metric set is intentionally small. Every label must have a
statically bounded value set. Enricher attributes never appear on metric
labels. See `03-metrics.md` for the full metric table with bucket boundaries
and cardinality estimates.

**Hard cardinality rule:** no metric label may have more than ~50 distinct
values in a typical deployment. Labels that would exceed this (invocation ID,
user ID, model version sub-strings) are span attributes only.

**AttributeEnricher and metrics:** there is an intentional hard boundary.
Enricher attributes go to OTel spans only. They never appear on Prometheus
metric labels. Callers who need per-tenant or per-user metrics must aggregate
from spans or wire their own metric pipeline via `LifecycleEventEmitter`.

---

## D58 — slog integration contract

**Status:** decided
**Summary:** The framework emits structured log records via a caller-provided
`slog.Handler`. A default `RedactingHandler` wraps the caller's handler and
strips credential material, raw message content, and PII markers from log
records before forwarding. The `RedactingHandler` lives in the
`telemetry/slog` sub-package per the go-architect recommendation.

**Decision.** See `04-slog-redaction.md` for the complete log-level mapping,
field taxonomy, and redaction rules.

**Package placement:** `RedactingHandler` is placed in the `telemetry/slog/`
sub-package, not in `telemetry/` directly. This follows the go-architect
validation recommendation (§3, §9):
- The handler is independently useful — callers may want redaction without
  OTel spans.
- It avoids mixing two distinct concerns (span emission and log redaction)
  in the `telemetry` package.
- The sub-package imports only `log/slog` from stdlib, placing it at Level 2
  in the import graph with no intra-praxis dependency.

Key commitments:
- Log-level mapping from `EventType` is framework-defined and documented.
- The `RedactingHandler` is opt-in at construction time (callers may pass
  `slog.Default()` directly). The framework itself never calls
  `slog.SetDefault`.
- Raw LLM response content and raw tool output are never logged at any level
  by default. Callers may opt in by implementing their own `LifecycleEventEmitter`.
- Credential values (API keys, JWT material) are never logged. This rule is
  enforced structurally by the Phase 5 credential isolation design; no
  credential value reaches a log call site.

---

## D59 — `FilterDecision` to content-analysis event mapping

**Status:** decided
**Summary:** `FilterActionRedact` with a PII-related reason emits
`EventTypePIIRedacted`. `FilterActionLog` or `FilterActionBlock` with an
injection-related reason emits `EventTypePromptInjectionSuspected`. Both
events are emitted in addition to (not instead of) the state-transition events
for the enclosing state.

**Decision.** The mapping logic is reason-driven, not purely action-driven.
The same `FilterAction` may or may not trigger a content-analysis event
depending on whether the `FilterDecision.Reason` field signals a recognized
content-analysis concern. See `06-filter-event-mapping.md` for the complete
trigger conditions, precedence rules, and `InvocationEvent` field population.

**Emission point:** content-analysis events are emitted by the orchestrator
immediately after the filter chain returns, before the enclosing
state-transition events (`EventTypeLLMCallStarted` or
`EventTypePostToolFilterStarted`/`Completed`). This ordering ensures that if
a `FilterActionBlock` causes a terminal transition, the content-analysis event
appears in the event stream before the terminal event.

**Weak assumption amendment:** the plan's assumption that no new `EventType`
constants are needed beyond D52b's 21 is confirmed. The two content-analysis
event types cover all `FilterDecision` combinations. `VerdictLog` (D64) also
maps to an existing event type.

---

## D60 — `AttributeEnricher` flow: when and how attributes attach

**Status:** decided
**Summary:** `Enrich` is called once per invocation at `Initializing` entry.
The returned map is stored in the orchestrator's per-invocation state and
attached to every OTel span (as `attribute.String` key-value pairs) and every
`InvocationEvent` (via `InvocationEvent.EnricherAttributes`). Enricher
attributes never appear on Prometheus metric labels.

**Decision.** The attribute attachment sequence at span start:
1. Framework-defined attributes (e.g., `praxis.invocation_id`,
   `praxis.model`) are set first.
2. Enricher attributes are set second. If an enricher attribute key collides
   with a framework key (same string), the framework attribute wins and the
   enricher attribute for that key is silently dropped. This prevents enrichers
   from overwriting core observability signals.

**`InvocationEvent.EnricherAttributes` field:** this field is a
`map[string]string` carrying the enricher snapshot for the invocation.
Phase 3's `InvocationEvent` struct definition must be amended to add this
field. The amendment is backward-compatible (zero value is nil, treated as
empty by the framework).

**Cardinality boundary:**
- OTel spans: enricher attributes attached freely. Unbounded-cardinality
  attributes (per-user IDs, per-request IDs) are acceptable on spans because
  the cardinality budget is managed by the tracing backend (sampling, retention
  limits), not by the framework.
- Prometheus metrics: no enricher attributes ever appear as labels. The
  framework-defined label set is closed (see D57 and `03-metrics.md`).
- `LifecycleEventEmitter`: the full enricher snapshot is available on every
  `InvocationEvent.EnricherAttributes`. Callers who want per-tenant metrics
  can implement a `LifecycleEventEmitter` that reads `EnricherAttributes` and
  updates their own metric pipeline.

---

## D61 — Error-to-terminal-event mapping

**Status:** decided
**Summary:** Each `ErrorKind` maps to exactly one terminal `EventType`. The
mapping is 1:1 and framework-enforced. `ApprovalRequired` maps to a terminal
event but is not classified as an error.

**Decision.** See `05-error-event-mapping.md` for the full mapping table and
edge cases. The 1:1 mapping is:

| `ErrorKind` | Terminal `EventType` |
|---|---|
| `transient_llm` (exhausted retries) | `EventTypeInvocationFailed` |
| `permanent_llm` | `EventTypeInvocationFailed` |
| `tool` | `EventTypeInvocationFailed` |
| `policy_denied` | `EventTypeInvocationFailed` |
| `budget_exceeded` | `EventTypeBudgetExceeded` |
| `cancellation` | `EventTypeInvocationCancelled` |
| `system` | `EventTypeInvocationFailed` |
| `approval_required` | `EventTypeApprovalRequired` |

**Multiple-error edge cases** (e.g., tool error concurrent with context
cancel): the orchestrator uses terminal-state immutability (D15/Phase 2 §4)
as the arbitration rule. The first error to drive a terminal state transition
wins. Subsequent errors are logged at WARN level but do not alter the terminal
event. See `05-error-event-mapping.md` §4 for the precedence table.

---

## D62 — C3 token-overshoot documentation

**Status:** decided
**Summary:** `BudgetExceededError` godoc is amended with an explicit
token-overshoot caveat. `BudgetSnapshot` fields already provide the overshoot
amount; no new fields are needed.

**Decision.** The token-dimension overshoot (C3) occurs because
`Guard.RecordTokens` is called after the LLM provider returns, then
`Guard.Check` is called at the `ToolDecision` or `LLMContinuation` boundary.
Between the `RecordTokens` call and the `Check` call, actual output tokens may
have already exceeded the `MaxOutputTokens` limit by up to one LLM call's
output token count.

The `BudgetExceededError.Snapshot.OutputTokensUsed` field already contains
the post-call value, which may exceed `MaxOutputTokens`. No new field is
needed to communicate the overshoot — the excess is
`Snapshot.OutputTokensUsed - MaxOutputTokens`.

The godoc amendment is in `05-error-event-mapping.md` §3. The key clause to
add to `BudgetExceededError`:

> Token-dimension limits may be overshot by up to one LLM call's output
> token count before breach detection. `Snapshot.OutputTokensUsed` reflects
> the actual consumed value including the overshoot. Callers who require strict
> token limits must set `MaxOutputTokens` to a value lower than their true
> ceiling by the maximum expected single-call output token count.

---

## D63 — CP5 classifier precedence rules

**Status:** decided
**Summary:** The classifier applies an `errors.As` identity check first. If
the input error is already a `TypedError`, it is returned as-is without
re-wrapping. Heuristic classification applies only to plain errors.

**Decision.** The `DefaultClassifier.Classify` precedence order (from
Phase 3 `07-errors-and-classifier.md`, formalized here):

1. **Identity rule:** `errors.As(err, &typed)` — if any error in the chain
   implements `TypedError`, return that `TypedError` unchanged. This is the
   CP5 contract: a `BudgetExceededError` propagated via `ToolResult.Err`
   survives the classifier unchanged and drives `BudgetExceeded` terminal
   state, not `Failed`.
2. **Context cancellation:** `errors.Is(err, context.Canceled)` or
   `errors.Is(err, context.DeadlineExceeded)` → `CancellationError`.
3. **HTTP status heuristic:** if the error value exposes an HTTP status code,
   classify as `TransientLLMError` (5xx, 429) or `PermanentLLMError` (4xx).
4. **Default fallback:** wrap in `SystemError`.

See `05-error-event-mapping.md` §5 for worked examples of CP5 propagation
through `ToolResult.Err`.

---

## D64 — `VerdictLog` lifecycle event emission

**Status:** decided
**Summary:** A `VerdictLog` decision from a `PolicyHook` emits an
`InvocationEvent` with type `EventTypePreHookCompleted` or
`EventTypePostHookCompleted` (whichever is the enclosing hook phase), with the
`FilterDecision`-equivalent fields populated in a new optional field
`AuditNote` on `InvocationEvent`. No new `EventType` constant is introduced.

**Decision.** `VerdictLog` means "proceed, but note this in the audit trail."
The orchestrator:
1. Proceeds as if `VerdictAllow` was returned.
2. Emits the normal state-transition event for the hook phase completion
   (e.g., `EventTypePreHookCompleted`).
3. Populates `InvocationEvent.AuditNote` with the `Decision.Reason` from the
   `VerdictLog` decision.

**`AuditNote` field amendment:** `InvocationEvent` gains an `AuditNote string`
field. It is non-empty only when the event was produced in response to a
`VerdictLog` decision. The zero value (empty string) means no audit note.
This is a backward-compatible addition.

**Why not a new EventType?** A new `EventTypePolicyAuditNote` constant would
require consumers to handle 22 event types and would break the 21-constant
count established in D52b. More importantly, the audit note is inseparable
from the hook-phase outcome: the invocation proceeded (which is what the
`*HookCompleted` event communicates). Embedding the audit note in the
completion event preserves that semantic unity.

**Emission point:** `AuditNote` is set before the event is sent to the channel
and before `Emit` is called. `LifecycleEventEmitter` implementations receive
the note on the same event as the phase completion.

See `06-filter-event-mapping.md` §4 for the complete `VerdictLog` emission
flow and the `AuditNote` field contract.

---

## D65 — Phase 3 amendments: `InvocationEvent` fields and `MetricsRecorder` option

**Status:** decided
**Supersedes (additive):** Phase 3 `02-orchestrator-api.md` `InvocationEvent`
struct definition; Phase 3 `02-orchestrator-api.md` option inventory.
**Summary:** Phase 4 adds six backward-compatible fields to `InvocationEvent`
and one new orchestrator option (`WithMetricsRecorder`). All additions are
zero-value-safe and do not break existing consumers.

**Decision.** The following amendments to Phase 3 artifacts are formally
recorded:

### `InvocationEvent` struct additions

Six new fields are added to the `InvocationEvent` struct defined in Phase 3
`02-orchestrator-api.md`. All are zero-value-safe (empty string, nil map):

| Field | Type | Decision | Populated on |
|---|---|---|---|
| `FilterPhase` | `string` | D59 | `EventTypePIIRedacted`, `EventTypePromptInjectionSuspected` |
| `FilterField` | `string` | D59 | Same as above |
| `FilterReason` | `string` | D59 | Same as above |
| `FilterAction` | `string` | D59 | Same as above |
| `AuditNote` | `string` | D64 | `prehook.completed`, `posthook.completed` with `VerdictLog` |
| `EnricherAttributes` | `map[string]string` | D60 | All events after `EventTypeInvocationStarted` |

These additions do not modify any existing field. The zero value of each new
field is semantically equivalent to "not applicable." Consumers that
exhaustively read `InvocationEvent` fields will see new zero-value fields but
no behavioral change.

### Orchestrator option addition

One new option is added to the `02-orchestrator-api.md` option inventory:

```go
// WithMetricsRecorder injects the telemetry.MetricsRecorder.
// Default: telemetry.NullMetricsRecorder (discards all observations).
func WithMetricsRecorder(recorder telemetry.MetricsRecorder) Option
```

This brings the total option count from 12 (Phase 3) to 13. The
`MetricsRecorder` interface is defined in `03-metrics.md` §5.1 with
stability tier `frozen-v1.0`.

**Amendment protocol compliance:** this decision uses the amendment protocol
from `docs/phase-1-api-scope/01-decisions-log.md`. The Phase 3 canonical
`InvocationEvent` definition in `02-orchestrator-api.md` is amended by
reference — the struct gains six fields as specified here. A back-reference
annotation ("Amended by D65 in Phase 4") should be added to the Phase 3
file at implementation time.

---

## D66 — Signal-term stability commitment

**Status:** decided
**Summary:** The signal-term constant lists that gate content-analysis event
emission carry a `frozen-v1.0` stability commitment. Terms are never removed
in v1.x; new terms may be added in minor releases.

**Decision.** The signal-term lists in `06-filter-event-mapping.md` §2.1 are:

- **PII signal terms:** `pii`, `personal`, `ssn`, `credit card`, `email`,
  `phone`, `address`, `dob`, `date of birth`, `passport`, `national id`
- **Injection signal terms:** `injection`, `prompt injection`, `jailbreak`

These lists are shipped as exported string-slice constants in the `telemetry`
package. They carry a `frozen-v1.0` stability commitment:

- **No removals in v1.x.** Removing a term changes which events fire — a
  behavioral breaking change. Removal requires v2.
- **Additions permitted in v1.x minor releases.** Adding a term widens
  detection. This may cause new events to fire for existing filter reasons
  that previously did not trigger content-analysis events. This is documented
  as an expected behavior change in minor releases.
- **Callers who need custom detection** implement a `LifecycleEventEmitter`
  that inspects `FilterDecision` values. The built-in detection is a
  convenience, not the sole mechanism.

**Rationale for narrow injection list:** broad terms like `"override"` and
`"ignore previous"` were considered and rejected because they appear in
common non-security filter reasons (e.g., "override default temperature").
The injection list targets terms that are unambiguously associated with
injection attempts when combined with a non-`Pass` filter action.
