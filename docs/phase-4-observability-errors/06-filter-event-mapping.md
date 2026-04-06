# Phase 4 — FilterDecision to Event Mapping

**Decisions:** D59, D60, D64
**Cross-references:** `01-decisions-log.md`, `02-span-tree.md` (span attributes),
`04-slog-redaction.md` (log levels), Phase 3 `04-hooks-and-filters.md`
(FilterDecision definition), Phase 3 `08-telemetry-interfaces.md`
(AttributeEnricher, InvocationEvent)

---

## 1. Overview

Filter chains (`PreLLMFilter` and `PostToolFilter`) return `[]FilterDecision`
from each `Filter` call. The orchestrator inspects these decisions to:

1. Apply the filter action (block, redact, log, or pass — Phase 3 D42).
2. Emit content-analysis lifecycle events (`EventTypePIIRedacted`,
   `EventTypePromptInjectionSuspected`) where applicable.
3. Record content-analysis span attributes on the enclosing child span.

Policy hooks (`PolicyHook`) return a `Decision` with a `Verdict`. The
`VerdictLog` verdict results in an audit note on the enclosing hook event
(D64).

The `AttributeEnricher` flow is also finalized here: when `Enrich` is called,
how the attributes attach to spans and events, and where the cardinality
boundary is enforced.

---

## 2. FilterDecision → content-analysis event mapping (D59)

### 2.1 Trigger logic

Content-analysis events are triggered by the combination of `FilterAction` and
the semantic content of `FilterDecision.Reason`. The reason field is
human-written and free-form; the framework applies a case-insensitive substring
check against a small set of recognized signal terms.

**Recognized PII signal terms** (for `EventTypePIIRedacted`):
`pii`, `personal`, `ssn`, `credit card`, `email`, `phone`, `address`, `dob`,
`date of birth`, `passport`, `national id`

**Recognized injection signal terms** (for `EventTypePromptInjectionSuspected`):
`injection`, `prompt injection`, `jailbreak`

The signal-term lists are framework-defined and shipped as constants in the
`telemetry` package. They carry a `frozen-v1.0` stability commitment: terms
are not removed from the lists in v1.x (removal would be a breaking change
since it changes which events fire). New terms may be added in v1.x minor
releases, as this only widens detection. Callers who need custom signal
detection implement a `LifecycleEventEmitter` that inspects `FilterDecision`
values from `InvocationEvent` fields.

**False-positive mitigation:** the signal-term list is intentionally narrow.
Broad terms like `"override"` and `"ignore previous"` were considered and
rejected because they appear in common non-security usage (e.g., "override
default model temperature"). The list targets terms that are unambiguously
associated with injection attempts when combined with a non-`Pass` filter
action.

### 2.2 Decision-to-event mapping table

| `FilterAction` | Reason contains PII signal | Reason contains injection signal | Event emitted |
|---|---|---|---|
| `FilterActionRedact` | Yes | No | `EventTypePIIRedacted` |
| `FilterActionRedact` | No | Yes | `EventTypePromptInjectionSuspected` |
| `FilterActionRedact` | Yes | Yes | Both events |
| `FilterActionRedact` | Neither | — | No content-analysis event |
| `FilterActionLog` | — | Yes | `EventTypePromptInjectionSuspected` |
| `FilterActionLog` | Yes | No | `EventTypePIIRedacted` |
| `FilterActionLog` | Neither | — | No content-analysis event |
| `FilterActionBlock` | — | Yes | `EventTypePromptInjectionSuspected` |
| `FilterActionBlock` | Yes | No | `EventTypePIIRedacted` |
| `FilterActionBlock` | Neither | — | No content-analysis event |
| `FilterActionPass` | Any | Any | No content-analysis event |

**Key design choice:** `FilterActionPass` never emits a content-analysis event,
even if the reason contains a signal term. A passing filter is not acting on
the content; logging a content-analysis event for a pass decision would create
false positives in security dashboards.

**Multiple decisions from one Filter call:** a single `Filter` call may return
multiple `FilterDecision` values (e.g., a filter that redacts PII and logs an
injection suspicion). Each decision is processed independently. The orchestrator
iterates the slice and emits one content-analysis event per qualifying decision,
in slice order.

### 2.3 Emission point (D59)

Content-analysis events are emitted by the orchestrator immediately after the
filter chain call returns, before the enclosing state-transition lifecycle
event. Specifically:

**For `PreLLMFilter`:**
```
[LLMCall state entry]
  → call PreLLMFilter.Filter(ctx, messages)
  → for each qualifying FilterDecision: emit EventTypePIIRedacted or EventTypePromptInjectionSuspected
  → emit EventTypeLLMCallStarted
  → dispatch LLM request
```

**For `PostToolFilter`:**
```
[PostToolFilter state entry]
  → emit EventTypePostToolFilterStarted
  → call PostToolFilter.Filter(ctx, result)
  → for each qualifying FilterDecision: emit EventTypePIIRedacted or EventTypePromptInjectionSuspected
  → emit EventTypePostToolFilterCompleted
```

**Rationale for emission ordering:** emitting content-analysis events before
the enclosing state-transition events ensures that if `FilterActionBlock`
causes a terminal transition, the content-analysis event appears in the event
stream before the terminal `EventTypeInvocationFailed`. This gives consumers
the full picture: "injection suspected, then blocked" rather than "blocked"
alone.

### 2.4 InvocationEvent fields for content-analysis events

Both `EventTypePIIRedacted` and `EventTypePromptInjectionSuspected` populate
the following fields on `InvocationEvent`:

| Field | Type | Value |
|---|---|---|
| `Type` | `EventType` | `"filter.pii_redacted"` or `"filter.prompt_injection_suspected"` |
| `InvocationID` | `string` | Per-invocation ID |
| `At` | `time.Time` | Wall-clock emission time |
| `BudgetSnapshot` | `budget.BudgetSnapshot` | Current snapshot at emission time |
| `EnricherAttributes` | `map[string]string` | Enricher snapshot from `Initializing` |
| `FilterPhase` | `string` | `"pre_llm"` or `"post_tool"` (new field — see §2.5) |
| `FilterField` | `string` | `FilterDecision.Field` (dot-path to acted-on content element) |
| `FilterReason` | `string` | `FilterDecision.Reason` |
| `FilterAction` | `string` | `FilterDecision.Action` string value |
| `ToolCallID` | `string` | Non-empty when `FilterPhase == "post_tool"` |

### 2.5 `InvocationEvent` struct amendments

The following fields must be added to `InvocationEvent` (Phase 3 `02-orchestrator-api.md` canonical definition). These are backward-compatible additions (zero
values are valid for all existing event types).

```go
// FilterPhase is the filter chain phase that produced this event.
// Non-empty only on EventTypePIIRedacted and EventTypePromptInjectionSuspected.
// Values: "pre_llm" (from PreLLMFilter), "post_tool" (from PostToolFilter).
FilterPhase string

// FilterField is the dot-path to the content element acted on.
// Non-empty only on EventTypePIIRedacted and EventTypePromptInjectionSuspected.
// Examples: "messages[2].parts[0].text", "result.content"
FilterField string

// FilterReason is the human-readable reason from the FilterDecision.
// Non-empty only on EventTypePIIRedacted and EventTypePromptInjectionSuspected.
FilterReason string

// FilterAction is the FilterAction string value from the FilterDecision.
// Non-empty only on EventTypePIIRedacted and EventTypePromptInjectionSuspected.
FilterAction string

// AuditNote is the reason from a VerdictLog decision.
// Non-empty only on prehook.completed or posthook.completed events where
// the PolicyHook returned VerdictLog (D64).
AuditNote string

// EnricherAttributes is the attribute map from AttributeEnricher.Enrich,
// captured at Initializing state entry. Non-nil for all events after
// Initializing; nil for EventTypeInvocationStarted (emitted before Enrich).
EnricherAttributes map[string]string
```

---

## 3. FilterActionBlock behavior

`FilterActionBlock` causes the orchestrator to transition to `Failed` with a
`PolicyDeniedError`. The content-analysis event (if any) is emitted before the
terminal event, as described in §2.3.

The `PolicyDeniedError` for a filter block is constructed as:

```go
&PolicyDeniedError{
    Phase:  hooks.PhasePreLLMInput,   // or PhasePostToolOutput
    Reason: "filter blocked: " + filterDecision.Reason,
}
```

The `Phase` field distinguishes filter-initiated denials from policy-hook
denials. Callers can inspect `PolicyDeniedError.Phase` to determine whether
the denial originated from a filter chain or a policy hook.

---

## 4. VerdictLog event emission (D64)

### 4.1 Emission contract

When a `PolicyHook.Evaluate` returns a `Decision` with `Verdict == VerdictLog`,
the orchestrator:

1. Proceeds as if `VerdictAllow` was returned (execution continues).
2. Emits the normal hook-phase completion event (`EventTypePreHookCompleted`
   or `EventTypePostHookCompleted`).
3. Sets `InvocationEvent.AuditNote = Decision.Reason` on that completion event.
4. Sets `InvocationEvent.AuditNote` on the enclosing `praxis.prehook` or
   `praxis.posthook` OTel span attribute `praxis.audit_note`.

### 4.2 AuditNote field contract

```go
// AuditNote is the reason from a VerdictLog decision. It is set on the
// prehook.completed or posthook.completed InvocationEvent when a PolicyHook
// returned VerdictLog during hook evaluation.
//
// The AuditNote is forwarded without modification from Decision.Reason.
// It is non-empty if and only if the hook phase completed with at least one
// VerdictLog decision.
//
// If multiple PolicyHooks in the chain return VerdictLog, AuditNote
// contains the concatenation of all Decision.Reason values, separated by
// "; ". Order is determined by the PolicyHook evaluation order.
AuditNote string
```

### 4.3 VerdictLog with multiple hooks

A hook chain may evaluate multiple `PolicyHook` implementations. If more than
one returns `VerdictLog`, all `Decision.Reason` values are concatenated in
evaluation order, separated by `"; "`:

```
AuditNote = "high-value customer flagged for review; model is experimental"
```

### 4.4 VerdictLog and other verdicts

If the hook chain includes both a `VerdictLog` and a `VerdictAllow` from
different hooks, `AuditNote` carries the `VerdictLog` reason and execution
proceeds normally.

If the hook chain includes a `VerdictLog` followed by a `VerdictDeny` from
a later hook, the `VerdictDeny` wins (deny takes precedence over log). The
`AuditNote` is still captured and emitted on the enclosing span
`praxis.prehook.audit_note`, but the terminal event is `EventTypeInvocationFailed`
not the hook completion event.

### 4.5 Why no new EventType for VerdictLog

A dedicated `EventTypePolicyAuditNote` constant was considered and rejected
(D64 rationale). The reasons:

1. **Semantic unity:** `VerdictLog` means "proceed AND note." The `*HookCompleted`
   event already communicates "proceed." Adding the note to the completion event
   preserves this unity without requiring consumers to correlate two events for
   the same hook phase.
2. **21-constant stability:** D52b established 21 `EventType` constants.
   Adding a 22nd is a minor breaking change for consumers that exhaustively
   switch on event types. The `AuditNote` field approach is additive.
3. **Ordering simplicity:** a separate event would need to be ordered relative
   to the hook completion event. The `AuditNote` field avoids this ambiguity.

---

## 5. AttributeEnricher flow contract (D60)

### 5.1 When Enrich is called

`AttributeEnricher.Enrich` is called exactly once per invocation, at
`Initializing` state entry, after the root OTel span is opened. This ensures
that the `ctx` passed to `Enrich` carries the invocation's OTel span context,
honoring the Phase 3 contract that enricher implementations can "read span
baggage" from `trace.SpanFromContext(ctx)`.

**Sequence at `Initializing` entry:**
```
1. Open root span "praxis.invocation"
   → ctx2 now carries the root span via trace.ContextWithSpan
2. Set framework-defined span attributes (praxis.invocation_id, praxis.model, ...)
3. Call AttributeEnricher.Enrich(ctx2)
   → returns enricherAttrs map[string]string
   → enricher can call trace.SpanFromContext(ctx2) to read span baggage
4. Set enricher attributes on root span (iterate enricherAttrs)
   → if key collides with praxis.* key, drop enricher value, log at DEBUG
5. Store enricherAttrs in per-invocation state for attachment to all
   subsequent InvocationEvent.EnricherAttributes fields
```

### 5.2 How attributes attach to spans

Enricher attributes are attached to the root span only, not to child spans.
Child spans inherit the root span's attributes through the trace context in
most tracing backends. Setting them on every child span would be redundant and
would increase span export size.

If a caller needs enricher attributes on a specific child span for backend
filtering, they can read them from `InvocationEvent.EnricherAttributes` in
their `LifecycleEventEmitter` implementation and use the OTel SDK's baggage API
to propagate attributes as needed.

### 5.3 How attributes attach to InvocationEvent

`InvocationEvent.EnricherAttributes` is set to the captured `enricherAttrs`
map on every event emitted after `Initializing` entry. The first event
(`EventTypeInvocationStarted`, emitted at `Created -> Initializing`) has
`EnricherAttributes == nil` because it is emitted before `Enrich` is called.

The framework stores a shallow copy of the `enricherAttrs` map at `Initializing`
entry. The same copy is referenced by all events; it is not modified after
capture.

### 5.4 Cardinality boundary

The cardinality boundary between spans and metrics is enforced by design:

**Spans (high-cardinality OK):**
- Enricher attributes are set as `attribute.String(key, value)` on the root
  span via `span.SetAttributes`.
- No cardinality constraint is imposed by the framework. The tracing backend
  manages cardinality through sampling and retention.
- This is the appropriate place for per-user IDs, per-request IDs, and other
  high-cardinality caller identifiers.

**Prometheus metrics (enricher attributes forbidden):**
- The `MetricsRecorder` implementation (backed by the 10 registered metrics in
  `03-metrics.md`) has no access to `enricherAttrs`.
- The metric recording path calls `MetricsRecorder.RecordXxx(provider, model,
  ...)` with only framework-defined, bounded-cardinality values.
- This is enforced structurally: the `MetricsRecorder` interface methods do not
  accept a `map[string]string` parameter.

**`LifecycleEventEmitter` (caller's responsibility):**
- Callers who implement `LifecycleEventEmitter` receive `EnricherAttributes`
  on every `InvocationEvent`.
- They may use these attributes to update their own Prometheus counters, write
  to a datastore, or feed a streaming analytics pipeline.
- The cardinality consequences of doing so are the caller's responsibility,
  not the framework's.

### 5.5 Enricher error handling

If `AttributeEnricher.Enrich` panics or returns a nil map, the orchestrator:
1. Recovers from any panic, logs at `ERROR` with `praxis.enricher_panic = true`.
2. Treats a nil map as an empty map (no enricher attributes).
3. Proceeds with the invocation normally.

`Enrich` is not expected to fail in normal operation. Errors from `Enrich`
should be handled inside the implementation (by returning an empty map). The
framework's panic recovery is belt-and-suspenders protection.

---

## 6. Span attributes for content-analysis events

When a content-analysis event is emitted, the enclosing child span (either
`praxis.llmcall` or `praxis.posttoolfilter`) is updated with the following
attributes:

**`praxis.llmcall` span (for `FilterPhase == "pre_llm"`):**
- `praxis.pii_redacted`: bool, `true` if at least one `EventTypePIIRedacted`
  was emitted during this LLM call's pre-filter phase
- `praxis.injection_suspected`: bool, `true` if at least one
  `EventTypePromptInjectionSuspected` was emitted

**`praxis.posttoolfilter` span (for `FilterPhase == "post_tool"`):**
- `praxis.pii_redacted`: bool (same semantics)
- `praxis.injection_suspected`: bool (same semantics)

These boolean span attributes are set to `false` by default and updated to
`true` if any qualifying `FilterDecision` is encountered during the filter
chain call. They allow tracing queries like "show me all LLM calls where PII
was redacted" without parsing event streams.
