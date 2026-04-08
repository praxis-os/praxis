# Phase 3 — Telemetry Interfaces

**Stability tier:** `frozen-v1.0`
**Decisions:** D31, D32
**Packages:** `MODULE_PATH_TBD/event` (EventType, InvocationEvent),
`MODULE_PATH_TBD/telemetry` (LifecycleEventEmitter, AttributeEnricher)

---

## Overview

Two complementary interfaces drive observability: `LifecycleEventEmitter`
delivers framework-defined events to a caller's observability backend, and
`AttributeEnricher` contributes caller-specific attributes to every span and
event without the framework needing to know their names.

The `EventType` constants and `InvocationEvent` struct live in the `event/`
sub-package (D32, amended) so event concerns are cleanly separated from the
root package.

---

## `EventType` constants (D31)

```go
// EventType identifies the kind of InvocationEvent. It is a typed string
// for grep-friendliness, JSON serializability, and readable log output.
//
// All 21 event types are defined here (D52b). The prefix conventions are:
//   - "invocation.*" for invocation-lifecycle events.
//   - "prehook.*", "posthook.*" for policy hook events.
//   - "llmcall.*", "llmcontinuation.*" for LLM call events.
//   - "tooldecision.*", "toolcall.*", "posttoolfilter.*" for tool-cycle events.
//   - "budget.*" for budget events.
//   - "approval.*" for human-approval events.
//   - "filter.*" for content-analysis events from filter chains (D52b).
//
// EventType is exported from the praxis root package (D32).
// Stability: frozen-v1.0.
type EventType string

const (
    // --- Non-terminal events (14) ---

    // EventTypeInvocationStarted is the first event on every stream.
    // Emitted at Created -> Initializing transition.
    EventTypeInvocationStarted EventType = "invocation.started"

    // EventTypeInitialized is emitted at Initializing -> PreHook.
    // Agent config is resolved; PriceProvider snapshot taken (D26);
    // wall-clock started (D25).
    EventTypeInitialized EventType = "invocation.initialized"

    // EventTypePreHookStarted is emitted at PreHook state entry.
    EventTypePreHookStarted EventType = "prehook.started"

    // EventTypePreHookCompleted is emitted at PreHook -> LLMCall.
    // All pre-invocation policy hooks returned VerdictAllow.
    EventTypePreHookCompleted EventType = "prehook.completed"

    // EventTypeLLMCallStarted is emitted at LLMCall state entry.
    // Pre-LLM filters applied; LLM request in flight.
    EventTypeLLMCallStarted EventType = "llmcall.started"

    // EventTypeLLMCallCompleted is emitted at LLMCall -> ToolDecision.
    // LLM response received.
    EventTypeLLMCallCompleted EventType = "llmcall.completed"

    // EventTypeToolDecisionStarted is emitted at ToolDecision state entry.
    // No matching *Completed event: ToolDecision is a synchronous
    // in-loop check (budget arithmetic + tool-call inspection), not an
    // I/O-bound operation. The next observable event is either
    // EventTypeToolCallStarted (tool cycle continues),
    // EventTypePostHookStarted (end of turn), or a terminal event.
    // (C2: this asymmetry is deliberate — see D18 rationale.)
    EventTypeToolDecisionStarted EventType = "tooldecision.started"

    // EventTypeToolCallStarted is emitted at ToolDecision -> ToolCall.
    // One per tool call in a batch. Under parallel dispatch, all
    // EventTypeToolCallStarted events for a batch are emitted before
    // any tool sub-goroutine runs (D24).
    // InvocationEvent.ToolCallID and InvocationEvent.ToolName are set.
    EventTypeToolCallStarted EventType = "toolcall.started"

    // EventTypeToolCallCompleted is emitted at ToolCall -> PostToolFilter.
    // Under parallel dispatch, all EventTypeToolCallCompleted events for
    // a batch are emitted after all sub-goroutines complete (C2).
    // InvocationEvent.ToolCallID is set.
    EventTypeToolCallCompleted EventType = "toolcall.completed"

    // EventTypePostToolFilterStarted is emitted at PostToolFilter state entry.
    // One per tool result. InvocationEvent.ToolCallID is set.
    EventTypePostToolFilterStarted EventType = "posttoolfilter.started"

    // EventTypePostToolFilterCompleted is emitted at PostToolFilter -> LLMContinuation.
    // InvocationEvent.ToolCallID is set.
    EventTypePostToolFilterCompleted EventType = "posttoolfilter.completed"

    // EventTypeLLMContinuationStarted is emitted at LLMContinuation state entry.
    // Tool results injected; next LLM call prepared.
    EventTypeLLMContinuationStarted EventType = "llmcontinuation.started"

    // EventTypePostHookStarted is emitted at PostHook state entry.
    EventTypePostHookStarted EventType = "posthook.started"

    // EventTypePostHookCompleted is emitted on any terminal entry from PostHook.
    // All post-invocation policies returned a terminal-advancing decision.
    EventTypePostHookCompleted EventType = "posthook.completed"

    // --- Content-analysis events (2, D52b) ---
    // These are emitted by the filter chain during LLMCall or PostToolFilter
    // states. They are not state-transition events; they are content-analysis
    // events triggered by FilterDecision values (D42). Phase 4 defines when
    // exactly these are emitted and what InvocationEvent fields are populated.

    // EventTypePIIRedacted is emitted when a filter redacts PII from content.
    // Corresponds to FilterActionRedact with a PII-related reason (seed §5).
    EventTypePIIRedacted EventType = "filter.pii_redacted"

    // EventTypePromptInjectionSuspected is emitted when a filter detects a
    // suspected prompt injection attempt. May accompany FilterActionBlock or
    // FilterActionLog depending on the filter's configuration (seed §5).
    EventTypePromptInjectionSuspected EventType = "filter.prompt_injection_suspected"

    // --- Terminal events (5) ---

    // EventTypeInvocationCompleted is the terminal event for state.Completed.
    EventTypeInvocationCompleted EventType = "invocation.completed"

    // EventTypeInvocationFailed is the terminal event for state.Failed.
    // InvocationEvent.Err is set to a non-nil TypedError.
    EventTypeInvocationFailed EventType = "invocation.failed"

    // EventTypeInvocationCancelled is the terminal event for state.Cancelled.
    EventTypeInvocationCancelled EventType = "invocation.cancelled"

    // EventTypeBudgetExceeded is the terminal event for state.BudgetExceeded.
    // InvocationEvent.BudgetSnapshot.ExceededDimension identifies the breach.
    EventTypeBudgetExceeded EventType = "budget.exceeded"

    // EventTypeApprovalRequired is the terminal event for state.ApprovalRequired.
    // InvocationEvent.ApprovalSnapshot is set to the resumption packet (D39).
    EventTypeApprovalRequired EventType = "approval.required"
)
```

---

## `InvocationEvent` (canonical definition)

The canonical definition lives in `02-orchestrator-api.md`. Reproduced here for
telemetry-package consumers.

Key telemetry-relevant fields:

- `Type EventType` — always set, the primary dispatch key.
- `InvocationID string` — correlates with traces and audit logs.
- `At time.Time` — wall-clock emission timestamp.
- `Err error` — non-nil only on `EventTypeInvocationFailed`, implements
  `errors.TypedError`.
- `BudgetSnapshot budget.BudgetSnapshot` — zero-value-safe.
- `ApprovalSnapshot *errors.ApprovalSnapshot` — non-nil only on
  `EventTypeApprovalRequired`.

---

## `LifecycleEventEmitter` interface

```go
// LifecycleEventEmitter delivers framework-defined lifecycle events to
// the caller's observability backend.
//
// The orchestrator calls Emit once per InvocationEvent before sending
// the event to the InvokeStream channel. For terminal events, Emit is
// called under the detached emission context (Layer 4, D22/D23):
// derived from context.Background() with a 5-second deadline and the
// invocation's OTel span re-attached. This ensures terminal events
// reach the emitter even when the invocation context has been cancelled.
//
// Emit must not modify the event. The event is value-copied before
// each call; modifications are silently discarded.
//
// LifecycleEventEmitter implementations must be safe for concurrent use.
//
// Stability: frozen-v1.0.
type LifecycleEventEmitter interface {
    // Emit delivers a single InvocationEvent to the observability backend.
    //
    // The ctx is the orchestrator's emission context (Layer 4 for terminal
    // events, Layer 3 for non-terminal events). Implementations must
    // respect ctx.Done() and return promptly on cancellation.
    //
    // Errors returned from Emit are logged at WARN level by the orchestrator
    // and do not affect the invocation's terminal state or the channel close
    // protocol. Emit failures are non-fatal (D22).
    Emit(ctx context.Context, event InvocationEvent) error
}
```

---

## `AttributeEnricher` interface

```go
// AttributeEnricher contributes caller-specific attributes to every
// span and lifecycle event in an invocation.
//
// The orchestrator calls Enrich once at invocation start (Initializing
// state) to collect the attribute set for this invocation. The attributes
// are attached to every OTel span and every InvocationEvent emitted
// during the invocation. The framework attaches whatever the enricher
// provides without inspecting, validating, or transforming any key or value.
//
// This is the decoupling contract's enforcement point for caller-contributed
// identity attribution (seed §6.3): tenant, agent, user, and request
// identifiers live here, not as hardcoded framework fields.
//
// AttributeEnricher implementations must be safe for concurrent use.
//
// Stability: frozen-v1.0.
type AttributeEnricher interface {
    // Enrich returns the attribute set for the current invocation.
    //
    // Attributes is a flat key-value map. Keys and values are strings.
    // The framework imposes no naming convention; callers define their
    // own attribute taxonomy.
    //
    // Returns an empty (non-nil) map if no attributes are relevant.
    // Never returns nil.
    //
    // The ctx carries the invocation context (Layer 2, D23), including
    // the OTel span context, so the enricher may read span baggage.
    Enrich(ctx context.Context) map[string]string
}
```

---

## Default (null) implementations

```go
// NullEmitter is the default LifecycleEventEmitter.
// It discards all events without error.
// Safe for concurrent use.
//
// Package: MODULE_PATH_TBD/telemetry
var NullEmitter LifecycleEventEmitter = nullEmitter{}

// NullEnricher is the default AttributeEnricher.
// It returns an empty map for every Enrich call.
// Safe for concurrent use.
//
// Package: MODULE_PATH_TBD/telemetry
var NullEnricher AttributeEnricher = nullEnricher{}
```

---

## Concurrency contract

Both `LifecycleEventEmitter` and `AttributeEnricher` implementations must be
safe for concurrent use. The orchestrator may call `Emit` from the loop goroutine
of any concurrent invocation, and `Enrich` at the start of any concurrent
invocation.

---

## Event ordering and Phase 4 handoff

The OTel span tree design, Prometheus metrics, slog redaction, and the
emission semantics for `EventTypePIIRedacted` and
`EventTypePromptInjectionSuspected` are Phase 4's scope. Phase 3 defines the
21 `EventType` constants (D52b added the two content-analysis events from
seed §5) and the interface shapes. Phase 4 defines when the content-analysis
events are emitted, what `InvocationEvent` fields are populated, and how they
correlate with `FilterDecision` values from D42.

The C2 concern (parallel tool-call completion ordering) is documented in the
`EventTypeToolCallCompleted` godoc above: under parallel dispatch, per-tool
completion ordering is not individually observable — the batch is observed as
a unit.
