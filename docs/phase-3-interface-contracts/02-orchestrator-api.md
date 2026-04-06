# Phase 3 — Orchestrator API

**Stability tier:** `frozen-v1.0`
**Decisions:** D32, D37, D38, D48, D49, D50, D51
**Packages:**
- `MODULE_PATH_TBD` (root `praxis` package) — shared types:
  `InvocationRequest`, `InvocationResult`, `InvocationEvent`, `EventType`
  constants.
- `MODULE_PATH_TBD/orchestrator` — facade: `*Orchestrator`, `New`,
  all `With*` option functions.

---

## Overview

The orchestrator API is split across two packages per D51:

- The **root `praxis` package** holds the shared types that callers construct
  and inspect: `InvocationRequest`, `InvocationResult`, `InvocationEvent`, and
  all `EventType` constants. This package imports leaf packages (`state`,
  `budget`, `errors`, `llm`) but does NOT import `hooks`, `tools`, or any
  interface package — this is the cycle-breaking constraint.
- The **`orchestrator` package** holds the facade struct, constructor, and
  functional options. It imports everything (root, all interface packages,
  `internal/loop`).

---

## `*Orchestrator`

```go
// Orchestrator is the public facade for a praxis agent invocation kernel.
// It drives a single LLM agent call from request to terminal state,
// enforcing policy, cost, security, and observability contracts by
// construction through a 14-state machine (D15).
//
// Each call to Invoke or InvokeStream is a fully independent invocation:
// fresh state machine, fresh invocation ID, fresh budget accounting.
// Orchestrator itself holds no per-invocation state.
//
// Orchestrator is safe for concurrent use. Multiple goroutines may call
// Invoke or InvokeStream simultaneously on the same Orchestrator instance.
//
// Stability: frozen-v1.0.
type Orchestrator struct {
    // unexported fields
}

// NewOrchestrator constructs an Orchestrator with the given LLM provider
// as the sole required dependency. All other dependencies default to their
// null/noop implementations if not supplied via options.
//
// The zero-wiring form (only provider supplied) produces a fully functional
// orchestrator suitable for smoke tests and examples:
//
//	orch := praxis.NewOrchestrator(provider)
//	result, err := orch.Invoke(ctx, req)
//
// See the With* option functions for dependency injection.
func NewOrchestrator(provider llm.Provider, opts ...Option) *Orchestrator

// Invoke executes a single agent invocation synchronously.
//
// It returns when the invocation reaches one of the five terminal states:
// Completed, Failed, Cancelled, BudgetExceeded, or ApprovalRequired.
//
// The returned InvocationResult is always non-nil. FinalState identifies the
// terminal reached. Response is non-nil only when FinalState == state.Completed.
//
// The returned error is non-nil for non-Completed terminals:
//   - state.Failed:          err is errors.TypedError with Kind() indicating cause.
//   - state.Cancelled:       err is *errors.CancellationError.
//   - state.BudgetExceeded:  err is *errors.BudgetExceededError.
//   - state.ApprovalRequired: err is *errors.ApprovalRequiredError.
//
// InvocationResult.Events contains the ordered list of all InvocationEvents
// emitted during the invocation. On the streaming path (InvokeStream),
// Events is nil; callers drain the channel directly.
//
// Cancellation: ctx cancellation routes the invocation to state.Cancelled.
// The soft-cancel grace window (500 ms, D21) applies to in-flight LLM and
// tool calls. Hard deadlines bypass the grace window.
func (o *Orchestrator) Invoke(ctx context.Context, req InvocationRequest) (*InvocationResult, error)

// InvokeStream executes a single agent invocation and returns a channel of
// InvocationEvents for real-time observation.
//
// The returned channel is buffered (size 16, seed §4.4). The orchestrator's
// loop goroutine is the sole producer and sole closer. The caller is
// responsible for draining the channel.
//
// Ordering guarantees:
//   - EventTypeInvocationStarted is always the first event.
//   - Exactly one terminal event (EventTypeInvocationCompleted,
//     EventTypeInvocationFailed, EventTypeInvocationCancelled,
//     EventTypeBudgetExceeded, or EventTypeApprovalRequired) is always
//     the last event before channel close.
//   - See docs/phase-2-core-runtime/03-streaming-and-events.md §3 for
//     the complete set of ordering guarantees.
//
// Under parallel tool dispatch, per-tool completion events are batched:
// all EventTypeToolCallStarted events for a batch are emitted before any
// tool sub-goroutine runs; all EventTypeToolCallCompleted and filter events
// are emitted after all sub-goroutines complete (C2 — per-tool completion
// ordering is not individually observable).
//
// The channel is always closed, even when the context is cancelled. A
// responsive consumer always receives the terminal event before close; a
// consumer stalled for >5 seconds may see a bare close (D20).
//
// InvokeStream is safe for concurrent use on the same Orchestrator instance.
// Each call returns an independent channel with its own goroutine.
func (o *Orchestrator) InvokeStream(ctx context.Context, req InvocationRequest) <-chan InvocationEvent
```

---

## `InvocationRequest`

```go
// InvocationRequest is the input to a single agent invocation.
//
// InvocationRequest is a value type. Callers may reuse and modify a request
// struct between invocations without affecting in-flight invocations.
type InvocationRequest struct {
    // Messages is the conversation history in provider-agnostic format.
    // Must be non-empty. The last message is typically a user turn.
    // All provider-specific formats (tool-use blocks, thinking blocks)
    // are represented via llm.MessagePart.
    Messages []llm.Message

    // Model is the model identifier for this invocation.
    // Format is provider-specific (e.g., "claude-opus-4-5", "gpt-4o").
    // Must be non-empty.
    Model string

    // Tools is the list of tools available to the LLM for this invocation.
    // Empty slice means no tool use. The orchestrator does not validate
    // tool definitions; the LLM provider validates on the wire.
    Tools []llm.ToolDefinition

    // BudgetConfig configures per-invocation budget limits for all four
    // dimensions: wall-clock duration, tokens, tool calls, and cost.
    // Zero value means no limits (the Guard still records consumption).
    // BudgetConfig is applied on top of any shared budget.Guard injected
    // via WithBudgetGuard; limits in BudgetConfig override the Guard's
    // configured limits for this invocation only.
    BudgetConfig budget.Config

    // SystemPrompt is an optional system prompt prepended to the conversation.
    // Empty string means no system prompt.
    SystemPrompt string

    // Metadata is an opaque key-value map forwarded to tools.InvocationContext.
    // The framework never inspects the keys or values. Callers use this to
    // carry request-scoped context (correlation IDs, caller identifiers)
    // to their tools.Invoker implementation.
    Metadata map[string]string

    // MaxTurns limits the number of LLM-tool cycles.
    // Zero means no limit enforced at this layer. Use BudgetConfig.MaxToolCalls
    // to enforce a budget-dimension limit.
    MaxTurns int
}
```

---

## `InvocationResult`

```go
// InvocationResult is the output of a synchronous Invoke call.
// Always non-nil on return from Invoke.
type InvocationResult struct {
    // InvocationID is the UUIDv7 (D49) assigned by the orchestrator.
    // Unique per invocation. Present on all InvocationEvents for correlation.
    InvocationID string

    // FinalState is the terminal state reached. One of:
    //   state.Completed, state.Failed, state.Cancelled,
    //   state.BudgetExceeded, state.ApprovalRequired.
    FinalState state.State

    // Response is the LLM's final response in provider-agnostic format.
    // Non-nil only when FinalState == state.Completed.
    Response *llm.Message

    // BudgetSnapshot is the budget state at the moment of terminal transition.
    // Zero-value-safe when no budget.Guard is configured.
    BudgetSnapshot budget.BudgetSnapshot

    // Events is the ordered list of all InvocationEvents emitted during the
    // invocation, in emission order. Only populated by the synchronous
    // Invoke path. Nil on the InvokeStream path.
    Events []InvocationEvent
}
```

---

## `InvocationEvent`

```go
// InvocationEvent is a single observable event emitted by the orchestrator
// during an invocation. It is the element type of the InvokeStream channel
// and the element type of InvocationResult.Events.
//
// InvocationEvent is a value type (no pointer fields transfer ownership).
// Consumers may store events in buffers without aliasing concerns.
//
// See the EventType constants for the complete set of 21 event types (D52b)
// and their emission points.
type InvocationEvent struct {
    // Type identifies the event. Always set.
    Type EventType

    // InvocationID is the UUIDv7 of the invocation that emitted this event.
    // Always non-empty. Matches InvocationResult.InvocationID.
    InvocationID string

    // State is the state.State from which this event was emitted.
    // Always set.
    State state.State

    // At is the wall-clock time at event emission.
    At time.Time

    // Err is non-nil only on EventTypeInvocationFailed. The error
    // implements errors.TypedError. For other event types, Err is nil.
    Err error

    // ToolCallID is non-empty only on tool-cycle events:
    //   EventTypeToolCallStarted, EventTypeToolCallCompleted,
    //   EventTypePostToolFilterStarted, EventTypePostToolFilterCompleted.
    // Matches tools.ToolCall.CallID.
    ToolCallID string

    // ToolName is non-empty only on EventTypeToolCallStarted.
    // Identifies the tool being invoked (matches tools.ToolCall.Name).
    ToolName string

    // BudgetSnapshot is the budget state at the time of this event.
    // Zero-value-safe when no budget.Guard is configured.
    // Under parallel tool dispatch, the snapshot reflects budget state
    // after the full batch completes, not per-tool (C2).
    BudgetSnapshot budget.BudgetSnapshot

    // ApprovalSnapshot is non-nil only on EventTypeApprovalRequired.
    // Contains the conversation snapshot for caller-owned resume (D07).
    ApprovalSnapshot *errors.ApprovalSnapshot
}
```

---

## Functional options

```go
// Option is a functional option for NewOrchestrator.
// Options are applied in order; later options override earlier ones
// for the same dependency.
type Option func(*options)

// WithToolInvoker injects the tools.Invoker implementation.
// Default: tools.NullInvoker (returns ToolStatusNotImplemented for all tools).
func WithToolInvoker(invoker tools.Invoker) Option

// WithPolicyHook injects the hooks.PolicyHook implementation.
// Default: hooks.AllowAllPolicyHook (returns VerdictAllow for all phases).
func WithPolicyHook(hook hooks.PolicyHook) Option

// WithPreLLMFilter injects the hooks.PreLLMFilter implementation.
// Default: hooks.PassThroughPreLLMFilter (pass-through, no decisions).
func WithPreLLMFilter(filter hooks.PreLLMFilter) Option

// WithPostToolFilter injects the hooks.PostToolFilter implementation.
// Default: hooks.PassThroughPostToolFilter (pass-through, no decisions).
func WithPostToolFilter(filter hooks.PostToolFilter) Option

// WithBudgetGuard injects the budget.Guard implementation.
// Default: budget.NullGuard (records nothing, never signals breach).
func WithBudgetGuard(guard budget.Guard) Option

// WithPriceProvider injects the budget.PriceProvider implementation.
// Default: budget.NullPriceProvider (returns 0 micro-dollars for all tokens).
func WithPriceProvider(pp budget.PriceProvider) Option

// WithLifecycleEventEmitter injects the telemetry.LifecycleEventEmitter.
// Default: telemetry.NullEmitter (discards all events).
func WithLifecycleEventEmitter(emitter telemetry.LifecycleEventEmitter) Option

// WithAttributeEnricher injects the telemetry.AttributeEnricher.
// Default: telemetry.NullEnricher (contributes no attributes).
func WithAttributeEnricher(enricher telemetry.AttributeEnricher) Option

// WithCredentialResolver injects the credentials.Resolver.
// Default: credentials.NullResolver (returns error for all credential refs).
func WithCredentialResolver(resolver credentials.Resolver) Option

// WithIdentitySigner injects the identity.Signer.
// Default: identity.NullSigner (returns empty string for all sign calls).
func WithIdentitySigner(signer identity.Signer) Option

// WithClassifier injects the errors.Classifier.
// Default: errors.DefaultClassifier (heuristic classification).
func WithClassifier(classifier errors.Classifier) Option

// WithInvocationIDFunc overrides the default UUIDv7 invocation ID generator.
// The provided function must return a non-empty string and be safe for
// concurrent use. Default: UUIDv7 via stdlib crypto/rand.
func WithInvocationIDFunc(f func() string) Option
```

**Canonical option inventory.** The above list is the authoritative set.
All option functions live in the `orchestrator` package (D51). The
go-architect document's option names (`WithLifecycleEmitter`,
`WithInvoker`) are superseded by this canonical list.
