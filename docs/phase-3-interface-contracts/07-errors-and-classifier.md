# Phase 3 — Errors and Classifier

**Stability tier:** `frozen-v1.0`
**Decisions:** D07, D17, D39, D44
**Package:** `MODULE_PATH_TBD/errors`

---

## Overview

The `errors` package defines the typed error taxonomy for `praxis`. Every error
returned from a framework method implements `TypedError`. Eight concrete types
ship with `praxis`; callers may define additional types that implement
`TypedError` for their own adapter layers.

`Classifier` maps raw `error` values into the taxonomy and drives the
differentiated retry policy. It is the orchestrator's sole error-routing
mechanism.

---

## `TypedError` interface

```go
// TypedError is the base interface for all errors returned by the praxis
// framework. Every error surfaced to callers implements TypedError.
//
// TypedError is compatible with errors.Is and errors.As: the concrete types
// implement Unwrap() and the standard library's error-wrapping chain is
// preserved throughout.
//
// Stability: frozen-v1.0.
type TypedError interface {
    error

    // Kind returns the stable error classification. The Kind value drives
    // retry decisions and event routing.
    Kind() ErrorKind

    // HTTPStatusCode returns a hint for HTTP callers mapping this error
    // to a response status. These are hints, not requirements; callers
    // may map differently for their use case.
    HTTPStatusCode() int

    // Unwrap returns the underlying error, enabling errors.Is and errors.As
    // to traverse the error chain. May return nil.
    Unwrap() error
}
```

---

## `ErrorKind` enum

```go
// ErrorKind is the stable error classification used by Classifier and
// TypedError.Kind().
type ErrorKind string

const (
    // ErrorKindTransientLLM represents a retryable LLM provider failure
    // (rate limit, 5xx, network timeout). Retry: 3x with exponential
    // backoff and jitter.
    ErrorKindTransientLLM ErrorKind = "transient_llm"

    // ErrorKindPermanentLLM represents a non-retryable LLM provider failure
    // (4xx, invalid request, unsupported model). Retry: never.
    ErrorKindPermanentLLM ErrorKind = "permanent_llm"

    // ErrorKindTool represents a tool invocation failure. Sub-kinds are
    // further classified via ToolError.SubKind. Retry: never by default;
    // callers may choose to retry at the Invoker level.
    ErrorKindTool ErrorKind = "tool"

    // ErrorKindPolicyDenied represents a policy hook rejection (VerdictDeny).
    // Retry: never.
    ErrorKindPolicyDenied ErrorKind = "policy_denied"

    // ErrorKindBudgetExceeded represents a budget dimension breach.
    // Retry: never (terminal).
    ErrorKindBudgetExceeded ErrorKind = "budget_exceeded"

    // ErrorKindCancellation represents context cancellation.
    // Retry: never (terminal).
    ErrorKindCancellation ErrorKind = "cancellation"

    // ErrorKindSystem represents an internal framework error (illegal state
    // transition, nil-pointer in framework code). Retry: never.
    ErrorKindSystem ErrorKind = "system"

    // ErrorKindApprovalRequired represents a human-approval checkpoint.
    // Not a failure. Retry: never; caller handles out-of-process resume.
    // Added by D07.
    ErrorKindApprovalRequired ErrorKind = "approval_required"
)
```

---

## Eight concrete error types

### `TransientLLMError`

```go
// TransientLLMError wraps a retryable LLM provider failure.
// The orchestrator retries 3 times with exponential backoff and jitter
// before routing to Failed.
type TransientLLMError struct {
    // Provider is the Name() of the llm.Provider that returned the error.
    Provider string
    // StatusCode is the HTTP status code from the provider, if available.
    StatusCode int
    // cause is the underlying error from the provider adapter.
    cause error
}

func (e *TransientLLMError) Kind() ErrorKind       { return ErrorKindTransientLLM }
func (e *TransientLLMError) HTTPStatusCode() int   { return 503 }
func (e *TransientLLMError) Unwrap() error          { return e.cause }
func (e *TransientLLMError) Error() string          { ... }
```

### `PermanentLLMError`

```go
// PermanentLLMError wraps a non-retryable LLM provider failure.
type PermanentLLMError struct {
    Provider   string
    StatusCode int
    cause      error
}

func (e *PermanentLLMError) Kind() ErrorKind       { return ErrorKindPermanentLLM }
func (e *PermanentLLMError) HTTPStatusCode() int   { return 502 }
func (e *PermanentLLMError) Unwrap() error          { return e.cause }
func (e *PermanentLLMError) Error() string          { ... }
```

### `ToolError`

```go
// ToolSubKind further classifies tool failures.
type ToolSubKind string

const (
    // ToolSubKindNetwork is a network-level failure reaching the tool.
    ToolSubKindNetwork ToolSubKind = "network"

    // ToolSubKindServerError is a 5xx or equivalent from the tool.
    ToolSubKindServerError ToolSubKind = "server_error"

    // ToolSubKindCircuitOpen is a circuit-breaker rejection.
    ToolSubKindCircuitOpen ToolSubKind = "circuit_open"

    // ToolSubKindSchemaViolation is an argument validation failure.
    ToolSubKindSchemaViolation ToolSubKind = "schema_violation"
)

// ToolError wraps a tool invocation failure.
type ToolError struct {
    // ToolName is the name of the failing tool.
    ToolName string
    // CallID is the ToolCall.CallID for correlation.
    CallID string
    // SubKind further classifies the failure.
    SubKind ToolSubKind
    cause   error
}

func (e *ToolError) Kind() ErrorKind       { return ErrorKindTool }
func (e *ToolError) HTTPStatusCode() int   { return 502 }
func (e *ToolError) Unwrap() error          { return e.cause }
func (e *ToolError) Error() string          { ... }
```

### `PolicyDeniedError`

```go
// PolicyDeniedError is returned when a PolicyHook returns VerdictDeny.
type PolicyDeniedError struct {
    // Phase is the hook phase at which the denial occurred.
    Phase hooks.Phase
    // Reason is Decision.Reason from the denying hook.
    Reason string
}

func (e *PolicyDeniedError) Kind() ErrorKind       { return ErrorKindPolicyDenied }
func (e *PolicyDeniedError) HTTPStatusCode() int   { return 403 }
func (e *PolicyDeniedError) Unwrap() error          { return nil }
func (e *PolicyDeniedError) Error() string          { ... }
```

### `BudgetExceededError`

```go
// BudgetExceededError is returned when a budget.Guard.Check() detects
// a dimension breach.
type BudgetExceededError struct {
    // Snapshot is the BudgetSnapshot at the time of breach.
    // ExceededDimension is set to the offending dimension.
    Snapshot budget.BudgetSnapshot
}

func (e *BudgetExceededError) Kind() ErrorKind       { return ErrorKindBudgetExceeded }
func (e *BudgetExceededError) HTTPStatusCode() int   { return 429 }
func (e *BudgetExceededError) Unwrap() error          { return nil }
func (e *BudgetExceededError) Error() string          { ... }
```

### `CancellationError`

```go
// CancellationKind distinguishes soft from hard cancellation.
type CancellationKind string

const (
    CancellationKindSoft CancellationKind = "soft"
    CancellationKindHard CancellationKind = "hard"
)

// CancellationError is returned when the invocation context is cancelled.
type CancellationError struct {
    // cancelKind distinguishes soft from hard cancellation. Access via
    // CancelKind() accessor method.
    cancelKind CancellationKind
    cause      error
}

// CancelKind returns the cancellation flavor (soft or hard).
func (e *CancellationError) CancelKind() CancellationKind { return e.cancelKind }

func (e *CancellationError) Kind() ErrorKind       { return ErrorKindCancellation }
func (e *CancellationError) HTTPStatusCode() int   { return 499 }
func (e *CancellationError) Unwrap() error          { return e.cause }
func (e *CancellationError) Error() string          { ... }
```

### `SystemError`

```go
// SystemError represents an internal framework error: an illegal state
// transition, an invariant violation, or a nil-pointer in framework code.
// These are bugs; callers should report them.
type SystemError struct {
    // Message is a description of the internal failure.
    Message string
    cause   error
}

func (e *SystemError) Kind() ErrorKind       { return ErrorKindSystem }
func (e *SystemError) HTTPStatusCode() int   { return 500 }
func (e *SystemError) Unwrap() error          { return e.cause }
func (e *SystemError) Error() string          { ... }
```

### `ApprovalRequiredError` (D07, D39)

```go
// ApprovalSnapshot holds the conversation state needed for caller-owned
// out-of-process resume after a human-approval checkpoint (D07, D17).
//
// ApprovalSnapshot is not a failure record; it is a resumption packet.
// Credential material is never included (enforced by Phase 5 review).
type ApprovalSnapshot struct {
    // Messages is the full conversation history up to the approval point,
    // in provider-agnostic format. Includes all tool calls and results.
    Messages []llm.Message

    // OriginalRequest is sufficient to reconstruct the InvocationRequest
    // for the resuming Invoke call. Contains the model, tools, budget
    // config, system prompt, and metadata.
    OriginalRequest InvocationRequest

    // BudgetAtApproval is the budget snapshot at the moment of the
    // ApprovalRequired transition. Callers may use this to configure a
    // budget offset for the resuming invocation.
    BudgetAtApproval budget.BudgetSnapshot

    // ApprovalMetadata is the opaque Decision.Metadata from the
    // PolicyHook that triggered this requirement. Framework-opaque;
    // callers define its schema (approval rule IDs, approver hints, etc.).
    ApprovalMetadata map[string]any
}

// ApprovalRequiredError is returned by Invoke (and sent as
// EventTypeApprovalRequired by InvokeStream) when a PolicyHook returns
// VerdictRequireApproval.
//
// HTTP status 202 (Accepted) distinguishes approval-pending from denial
// (403) and internal error (500).
//
// Resume after approval: the caller appends the human decision as a new
// message to Snapshot.Messages and calls Invoke with a new
// InvocationRequest. See D50.
type ApprovalRequiredError struct {
    Snapshot ApprovalSnapshot
}

func (e *ApprovalRequiredError) Kind() ErrorKind       { return ErrorKindApprovalRequired }
func (e *ApprovalRequiredError) HTTPStatusCode() int   { return 202 }
func (e *ApprovalRequiredError) Unwrap() error          { return nil }
func (e *ApprovalRequiredError) Error() string          { ... }
```

---

## `Classifier` interface (D44)

```go
// Classifier maps a raw error into the errors.TypedError taxonomy.
// It is used by the orchestrator's retry logic and event routing.
//
// CP5 — Classifier precedence for propagated typed errors: the default
// classifier checks errors.As(err, &typed) before applying any
// heuristic classification. This ensures that a BudgetExceededError
// propagated through a tools.ToolResult.Err is returned as-is rather
// than collapsed into a ToolError.
//
// Classifier implementations must be safe for concurrent use.
//
// Stability: frozen-v1.0.
type Classifier interface {
    // Classify maps err into the TypedError taxonomy.
    //
    // Never returns nil. If err is already a TypedError (via errors.As),
    // returns it unchanged (identity rule). If err is a plain error,
    // applies heuristic classification and wraps in the nearest concrete
    // TypedError.
    //
    // Errors in classification are not propagated; the classifier falls
    // back to SystemError for inputs it cannot classify.
    Classify(err error) TypedError
}

// DefaultClassifier is the default Classifier implementation shipped
// with praxis. It applies heuristic rules in this precedence order:
//
//  1. errors.As check: if err is already a TypedError, return it as-is.
//  2. Context check: if errors.Is(err, context.Canceled) or
//     errors.Is(err, context.DeadlineExceeded), return CancellationError.
//  3. HTTP status heuristic: if the provider error carries an HTTP status,
//     classify as TransientLLMError (5xx, 429) or PermanentLLMError (4xx).
//  4. Default: wrap in SystemError.
//
// DefaultClassifier is safe for concurrent use.
//
// Package: MODULE_PATH_TBD/errors
var DefaultClassifier Classifier = defaultClassifier{}
```

---

## Error construction helpers

```go
// NewTransientLLMError constructs a TransientLLMError.
func NewTransientLLMError(provider string, statusCode int, cause error) *TransientLLMError

// NewPermanentLLMError constructs a PermanentLLMError.
func NewPermanentLLMError(provider string, statusCode int, cause error) *PermanentLLMError

// NewToolError constructs a ToolError with the given sub-kind.
func NewToolError(toolName, callID string, subKind ToolSubKind, cause error) *ToolError

// NewSystemError constructs a SystemError.
func NewSystemError(message string, cause error) *SystemError
```

---

## Retry policy matrix

The following matrix is enforced by the orchestrator's internal retry logic.
It is derived from the `ErrorKind` returned by `Classifier.Classify()`.

| `ErrorKind` | Retry behavior |
|---|---|
| `ErrorKindTransientLLM` | 3 retries, exponential backoff with full jitter, base 500 ms |
| `ErrorKindPermanentLLM` | No retry |
| `ErrorKindTool` | No retry (retry is the Invoker's responsibility) |
| `ErrorKindPolicyDenied` | No retry |
| `ErrorKindBudgetExceeded` | No retry (terminal) |
| `ErrorKindCancellation` | No retry (terminal) |
| `ErrorKindSystem` | No retry |
| `ErrorKindApprovalRequired` | No retry (terminal, not a failure) |
