# Phase 3 — Tools and Invocation Context

**Stability tier:** `frozen-v1.0`
**Decisions:** D06, D36, D40
**Package:** `MODULE_PATH_TBD/tools`
**Composition properties:** CP3 (budget Guard not exposed via InvocationContext),
CP5 (ToolResult admits typed-error embedding), CP6 (signed identity readable
from InvocationContext)

---

## Overview

The `tools` package defines the `Invoker` interface, the `ToolCall` and
`ToolResult` data types, and `InvocationContext`. Together they constitute the
complete seam between the orchestration kernel and any tool execution
infrastructure a caller provides.

The orchestrator does not know how tools are resolved, authenticated, or
routed. All of that is the invoker's concern.

---

## `InvocationContext` (D36)

```go
// InvocationContext carries framework-maintained state for a single
// invocation. It is passed to every Invoker.Invoke call.
//
// InvocationContext is a read-only data container. Implementations of
// Invoker must not mutate any field.
//
// Under parallel tool dispatch (D24), multiple Invoke calls may receive
// the same InvocationContext concurrently. InvocationContext is designed
// for concurrent read access: it carries no mutable state and no
// synchronization primitives.
//
// InvocationContext does not carry the tool name (D06). The tool name
// lives on the ToolCall struct so that each concurrent Invoke call is
// self-contained.
type InvocationContext struct {
    // InvocationID is the UUIDv7 (D49) for the current invocation.
    // Non-empty on every call. Use this to correlate Invoker-emitted
    // spans and events with the orchestrator's trace.
    InvocationID string

    // Budget is the BudgetSnapshot at the time this tool call was
    // dispatched. It is a read-only value; the Guard is not exposed
    // to the invoker. The invoker must not record budget against the
    // outer Guard.
    Budget budget.BudgetSnapshot

    // SpanContext is the OTel span context for the current invocation.
    // Invokers that want to emit child spans should start them with
    // this as the parent via trace.ContextWithRemoteSpanContext or
    // otel.Tracer.Start(ctx, ...).
    //
    // May be the zero SpanContext if telemetry is not configured.
    SpanContext trace.SpanContext

    // SignedIdentity is the JWT produced by identity.Signer.Sign for
    // this tool call. Empty string if identity signing is not configured.
    //
    // Invokers may forward this as a Bearer token to downstream services
    // to assert the identity of the invoking agent.
    //
    // For nested orchestrator wiring (CP6), the invoker reads
    // SignedIdentity to construct the inner invocation's Signer.
    SignedIdentity string

    // Metadata is the caller-controlled key-value map from
    // InvocationRequest.Metadata. The framework forwards it unchanged.
    // The framework never inspects the keys or values.
    Metadata map[string]string
}
```

---

## `ToolCall`

```go
// ToolCall describes a single tool invocation requested by the LLM.
// It is constructed by the orchestrator from the LLM response and
// passed to Invoker.Invoke.
//
// The tool name lives on ToolCall (D06), not on InvocationContext,
// so each concurrent Invoke call under parallel dispatch is
// self-contained.
type ToolCall struct {
    // CallID is the provider-assigned unique ID for this tool call.
    // Matches llm.LLMToolCall.CallID.
    CallID string

    // Name is the tool identifier from the LLM's response.
    // Matches llm.ToolDefinition.Name.
    Name string

    // ArgumentsJSON is the raw JSON arguments payload from the LLM.
    // The invoker is responsible for unmarshaling and validating.
    ArgumentsJSON []byte
}
```

---

## `ToolResult` (D40, CP5)

```go
// ToolStatus is the outcome classification of a tool invocation.
type ToolStatus string

const (
    // ToolStatusSuccess means the tool executed and returned output.
    ToolStatusSuccess ToolStatus = "success"

    // ToolStatusDenied means the invoker's own policy rejected the call.
    // The orchestrator injects a structured denial back into the
    // conversation rather than routing to Failed.
    ToolStatusDenied ToolStatus = "denied"

    // ToolStatusNotImplemented means no handler was found for the
    // requested tool name. Returned by NullInvoker for all calls.
    ToolStatusNotImplemented ToolStatus = "not_implemented"

    // ToolStatusError means the tool execution failed.
    // Err must be non-nil when Status == ToolStatusError.
    ToolStatusError ToolStatus = "error"
)

// ToolResult is the output of a single Invoker.Invoke call.
//
// CP5 — Typed-error propagation: when a nested invocation (agent-as-tool
// pattern) returns a TypedError (BudgetExceededError, PolicyDeniedError,
// ApprovalRequiredError, CancellationError), the Invoker should set
// Status to ToolStatusError and Err to the original TypedError. The
// outer orchestrator's Classifier inspects Err via errors.As before
// applying any reclassification, so the original ErrorKind is preserved
// through the tool-result boundary (D44 classifier precedence rule).
//
// ToolResult is a value type.
type ToolResult struct {
    // Status indicates the outcome of the tool invocation.
    Status ToolStatus

    // Content is the tool's output. May be structured JSON or plain text.
    // Empty string is valid and expected for Denied and NotImplemented.
    Content string

    // Err is non-nil only when Status == ToolStatusError.
    //
    // CP5: if the tool is a nested praxis invocation, Err carries the
    // original TypedError from that invocation. The outer orchestrator's
    // Classifier preserves the Kind() via errors.As inspection.
    //
    // For local tool errors, Err wraps the underlying failure.
    // The orchestrator classifies it via errors.Classifier.
    Err error

    // CallID echoes ToolCall.CallID so the orchestrator can correlate
    // results with the original call. Must be set by the invoker.
    CallID string
}
```

---

## `Invoker` interface

```go
// Invoker is the tool execution seam between the orchestration kernel
// and caller-provided tool infrastructure.
//
// The orchestrator calls Invoke once per tool call in the LLM response.
// Under parallel dispatch (D24, D06), it calls Invoke concurrently for
// each tool call in the batch. Each Invoke call receives a distinct
// ToolCall (different Name and CallID) but the same InvocationContext.
//
// The invoker is responsible for:
//   - Routing the call by ToolCall.Name.
//   - Fetching credentials via credentials.Resolver (if needed).
//   - Executing the tool.
//   - Returning a ToolResult with an appropriate Status.
//
// Policy denials at the tool level are expressed as
// ToolResult{Status: ToolStatusDenied} — not as an error return.
// The orchestrator injects denied results back into the conversation.
//
// Invoker implementations must be safe for concurrent use.
//
// Stability: frozen-v1.0.
type Invoker interface {
    Invoke(ctx context.Context, ictx InvocationContext, call ToolCall) (ToolResult, error)
}

// InvokerFunc is an adapter that lets an ordinary function serve as
// an Invoker. Useful for single-tool implementations and tests.
type InvokerFunc func(ctx context.Context, ictx InvocationContext, call ToolCall) (ToolResult, error)

func (f InvokerFunc) Invoke(ctx context.Context, ictx InvocationContext, call ToolCall) (ToolResult, error) {
    return f(ctx, ictx, call)
}
```

---

## Default (null) implementation

```go
// NullInvoker is the default Invoker implementation.
// It returns ToolResult{Status: ToolStatusNotImplemented} for every call.
// Used when zero-wiring an Orchestrator (D12).
//
// NullInvoker is safe for concurrent use.
//
// Package: MODULE_PATH_TBD/tools
var NullInvoker Invoker = nullInvoker{}
```

---

## Concurrency contract

`Invoker` implementations must be safe for concurrent use. The orchestrator
may call `Invoke` from multiple goroutines simultaneously — both from parallel
tool dispatch within a single invocation (D24) and from multiple concurrent
invocations on the same `*Orchestrator` instance.

---

## CP5 usage example

When implementing the agent-as-tool pattern, the invoker wraps the inner
orchestrator's error to preserve typed-error provenance:

```go
// Illustrative caller-level code — not part of the praxis API.
func (iv *agentInvoker) Invoke(
    ctx context.Context,
    ictx tools.InvocationContext,
    call tools.ToolCall,
) (tools.ToolResult, error) {
    if call.Name != "ask_b" {
        return tools.ToolResult{Status: tools.ToolStatusNotImplemented, CallID: call.CallID}, nil
    }
    result, err := iv.innerOrch.Invoke(ctx, buildRequest(ictx, call))
    if err != nil {
        // Preserve the TypedError from the inner invocation (CP5).
        // The outer classifier will errors.As into the original Kind.
        return tools.ToolResult{
            Status: tools.ToolStatusError,
            CallID: call.CallID,
            Err:    err,
        }, nil
    }
    return tools.ToolResult{
        Status:  tools.ToolStatusSuccess,
        Content: result.Response.Parts[0].Text,
        CallID:  call.CallID,
    }, nil
}
```
