# Phase 4 — Error-to-Event Mapping

**Decisions:** D61, D62, D63
**Cross-references:** `01-decisions-log.md`, Phase 3 `07-errors-and-classifier.md`,
Phase 2 `02-state-machine.md` (terminal-state immutability)

---

## 1. Overview

Every error that causes a terminal transition is classified by
`errors.Classifier` and maps to exactly one terminal `EventType`. The mapping
is deterministic, 1:1, and enforced by the orchestrator's terminal-routing
logic. No error can produce multiple terminal events; the first error to lock
the terminal state wins.

This document defines:
- The canonical `ErrorKind` → `EventType` mapping (§2).
- The `BudgetExceededError` token-overshoot documentation amendment (§3, C3).
- Multiple-error edge cases and precedence (§4).
- CP5 classifier precedence rules with worked examples (§5).

---

## 2. ErrorKind → terminal EventType mapping (D61)

| `ErrorKind` | Terminal state | Terminal `EventType` | `InvocationEvent.Err` |
|---|---|---|---|
| `transient_llm` (retries exhausted) | `Failed` | `EventTypeInvocationFailed` | `*TransientLLMError` |
| `permanent_llm` | `Failed` | `EventTypeInvocationFailed` | `*PermanentLLMError` |
| `tool` | `Failed` | `EventTypeInvocationFailed` | `*ToolError` |
| `policy_denied` | `Failed` | `EventTypeInvocationFailed` | `*PolicyDeniedError` |
| `budget_exceeded` | `BudgetExceeded` | `EventTypeBudgetExceeded` | `*BudgetExceededError` |
| `cancellation` | `Cancelled` | `EventTypeInvocationCancelled` | `*CancellationError` |
| `system` | `Failed` | `EventTypeInvocationFailed` | `*SystemError` |
| `approval_required` | `ApprovalRequired` | `EventTypeApprovalRequired` | `*ApprovalRequiredError` |

**Notes:**

- `approval_required` is listed for completeness. It is returned as an error
  from `AgentOrchestrator.Invoke` but is not a failure. `InvocationEvent.Err`
  is set to `*ApprovalRequiredError` on `EventTypeApprovalRequired` only when
  using the `Invoke` (non-streaming) path; on the `InvokeStream` path, it is
  carried as `InvocationEvent.ApprovalSnapshot` instead.

- `transient_llm` enters `Failed` only after the orchestrator exhausts its
  retry budget (3 retries per Phase 3 retry matrix). During active retries,
  the invocation stays in the `LLMCall` state and does not emit a terminal
  event.

- `budget_exceeded` is the only `ErrorKind` that maps to a non-`Failed`
  terminal state. This distinction is intentional: budget breaches have their
  own terminal state (`BudgetExceeded`) and their own event type
  (`EventTypeBudgetExceeded`) to make them unambiguous in dashboards and
  audit logs.

### 2.1 `InvocationEvent` fields on terminal events

**All terminal events:**
- `Type`: the terminal `EventType`
- `InvocationID`: the invocation ID
- `At`: wall-clock emission time
- `BudgetSnapshot`: current snapshot (may reflect overshoot for `budget_exceeded`)
- `EnricherAttributes`: enricher snapshot from `Initializing`

**`EventTypeInvocationFailed` additionally:**
- `Err`: non-nil `TypedError` implementing the appropriate concrete type

**`EventTypeBudgetExceeded` additionally:**
- `Err`: `*BudgetExceededError` with `Snapshot.ExceededDimension` set
- `BudgetSnapshot`: the snapshot at breach (same as `Err.Snapshot`)

**`EventTypeInvocationCancelled` additionally:**
- `Err`: `*CancellationError` with `CancelKind` set (`soft` or `hard`)

**`EventTypeApprovalRequired` additionally:**
- `ApprovalSnapshot`: non-nil `*ApprovalSnapshot` resumption packet
- `Err`: `nil` on the streaming path; `*ApprovalRequiredError` on the blocking path

---

## 3. C3: BudgetExceededError token-overshoot documentation (D62)

### 3.1 Mechanism

Token-dimension budget limits may be overshot by up to one LLM call's output
token count before breach detection. This is a consequence of the check timing:

```
RecordTokens(inputTokens, outputTokens)   // called after provider returns
Check()                                    // called at ToolDecision or LLMContinuation entry
```

The breach is detected at `Check` time, which is after the LLM provider has
already returned and tokens have been recorded. There is no mechanism to
pre-check the token budget before the LLM call without knowing in advance how
many output tokens the model will produce.

### 3.2 BudgetExceededError godoc amendment

The following text must be added to `BudgetExceededError` in the errors
package (Phase 3 `07-errors-and-classifier.md` defines the base struct; this
is an amendment per D62):

```go
// BudgetExceededError is returned when a budget.Guard.Check() detects
// a dimension breach.
//
// Token-dimension overshoot (C3): the token-dimension limit
// (MaxInputTokens or MaxOutputTokens) may be exceeded by up to one LLM
// call's token count before this error is returned. RecordTokens is called
// after the LLM provider returns, and Check is called at the next
// budget-gated state boundary (ToolDecision or LLMContinuation entry).
// In the window between RecordTokens and Check, actual token consumption
// may already exceed the configured limit.
//
// The Snapshot.OutputTokensUsed field reflects the actual consumed value
// including the overshoot. The overshoot magnitude is:
//   Snapshot.OutputTokensUsed - (the caller's configured MaxOutputTokens)
//
// Callers who require strict token limits must set MaxOutputTokens to a
// value lower than their true ceiling by the maximum expected single-call
// output token count (a model-specific value, typically 1,000–8,192 tokens
// depending on the configured max_tokens parameter).
type BudgetExceededError struct {
    // Snapshot is the BudgetSnapshot at the time of breach.
    // ExceededDimension is set to the offending dimension.
    // For token-dimension breaches, Snapshot.OutputTokensUsed may exceed
    // budget.Config.MaxOutputTokens by one call's output tokens (C3).
    Snapshot budget.BudgetSnapshot
}
```

### 3.3 Overshoot visibility on spans and events

The `praxis.output_tokens_total` root span attribute and the
`BudgetSnapshot.OutputTokensUsed` field in `EventTypeBudgetExceeded` both
reflect the post-overshoot value. Callers can compute the overshoot from these
values combined with their configured `MaxOutputTokens` limit.

The `praxis_llm_tokens_total` Prometheus counter (direction=`output`) also
reflects the post-overshoot cumulative count, since it is incremented at
`RecordTokens` time.

---

## 4. Multiple-error edge cases (D61)

### 4.1 Arbitration rule

When multiple errors arrive concurrently or in rapid succession, terminal-state
immutability (Phase 2 §4) is the arbitration mechanism. The first error to
drive the state machine into a terminal state wins. Subsequent errors are:

1. Logged at `WARN` level with the field `praxis.discarded_error_kind`.
2. Discarded from the terminal event (not included in `Err`).
3. Visible in the OTel span via `span.AddEvent("praxis.discarded_error",
   attribute("praxis.discarded_error_kind", err.Kind()))`.

### 4.2 Precedence table for simultaneous errors

In practice, errors arrive in sequence (the orchestrator's loop goroutine is
single-threaded per invocation). The following ordering applies when the loop
detects multiple error conditions at the same state boundary:

| Priority | Condition | Winner |
|---|---|---|
| 1 (highest) | Context already cancelled at state boundary entry | `CancellationError` |
| 2 | `budget.Guard.Check()` returns `BudgetExceededError` | `BudgetExceededError` |
| 3 | PolicyHook returns `VerdictDeny` | `PolicyDeniedError` |
| 4 | Filter chain returns `FilterActionBlock` | `PolicyDeniedError` |
| 5 | LLM provider returns error (after retries) | `TransientLLMError`/`PermanentLLMError` |
| 6 | Tool invocation returns error | `ToolError` |
| 7 (lowest) | All other cases | `SystemError` |

**Rationale for priority 1 (cancellation first):** if the caller cancelled
the context, it has signalled intent to stop. Reporting a budget breach or
policy denial on a context that was already cancelled would be misleading —
the invocation was stopped by the caller, not by a policy or budget check.

**Rationale for priority 2 (budget before policy):** budget checks run before
policy hooks at the same state boundary. A budget breach short-circuits the
policy check. This ordering is load-bearing: it prevents a policy hook from
running when the budget is already exhausted (which could produce a spurious
`PolicyDeniedError` in addition to the budget breach).

### 4.3 ToolError concurrent with context cancel

This is the most common race in practice. The tool goroutine returns a
`ToolError` at the same time the caller cancels the context.

**Outcome:** the orchestrator detects context cancellation at the
`ToolCall -> PostToolFilter` boundary. It classifies the error as
`CancellationError` (priority 1). The `ToolError` is logged at `WARN` with
`praxis.discarded_error_kind = "tool"`.

---

## 5. CP5 classifier precedence with worked examples (D63)

### 5.1 Precedence rules

The `DefaultClassifier.Classify` precedence order (formalized from Phase 3
`07-errors-and-classifier.md` §Classifier interface):

1. **Identity rule:** `errors.As(err, &typed)` where `typed` is `errors.TypedError`.
   If any error in the chain satisfies `TypedError`, return that `TypedError`
   unchanged. This is the CP5 contract.

2. **Context cancellation:** `errors.Is(err, context.Canceled)` or
   `errors.Is(err, context.DeadlineExceeded)` → `&CancellationError{cancelKind: CancellationKindHard}`.
   Note: context cancellation is checked after the identity rule because a
   `CancellationError` in the chain (from identity rule) may have
   `CancellationKindSoft`, which is semantically different from a raw
   `context.Canceled`.

3. **HTTP status heuristic:** if the error exposes an HTTP status code via
   a recognized interface or error type (e.g., `interface { StatusCode() int }`),
   classify as `TransientLLMError` (5xx, 429) or `PermanentLLMError` (4xx).

4. **Default fallback:** wrap in `SystemError{Message: err.Error(), cause: err}`.

### 5.2 Worked examples

**Example 1: BudgetExceededError propagated through ToolResult.Err**

A shared `budget.Guard` (CP3) is used by a nested orchestrator tool. The inner
orchestrator's budget is exhausted and it returns a `*BudgetExceededError`
through the tool result.

```
tools.Invoker.Invoke returns: ToolResult{
    Err: &BudgetExceededError{Snapshot: {ExceededDimension: "tokens", ...}},
}
```

The outer orchestrator calls `Classifier.Classify(toolResult.Err)`:

1. `errors.As(toolResult.Err, &typed)` succeeds — `*BudgetExceededError`
   implements `TypedError`.
2. Identity rule fires: return `*BudgetExceededError` unchanged.

**Outcome:** outer orchestrator routes to `BudgetExceeded` terminal state,
not `Failed`. The `EventTypeBudgetExceeded` event is emitted with the inner
invocation's budget snapshot. This is the correct behavior — the shared budget
has been exhausted, and the outer invocation should also stop.

**Example 2: Plain error from a tool (no TypedError in chain)**

A tool's HTTP client returns a connection error:

```
tools.Invoker.Invoke returns: ToolResult{
    Err: &url.Error{Op: "Get", URL: "...", Err: io.EOF},
}
```

`Classifier.Classify(&url.Error{...})`:

1. `errors.As` check: `*url.Error` does not implement `TypedError`. Chain
   inspection: `io.EOF` does not implement `TypedError`. Identity rule does
   not fire.
2. Context check: `errors.Is(&url.Error{...}, context.Canceled)` → false.
3. HTTP status heuristic: `*url.Error` does not expose a status code. Heuristic
   does not fire.
4. Default fallback: wrap in `&SystemError{Message: "url.Error: ...", cause: &url.Error{...}}`.

**Outcome:** `SystemError` with `kind = "system"`. The orchestrator routes to
`Failed`. This is appropriate — the tool invoker should have wrapped the error
in a `ToolError` with a `ToolSubKind`; the `SystemError` fallback signals that
the invoker is not wrapping correctly, which is a bug in the `Invoker`
implementation.

**Example 3: TypedError wrapped in another error**

A tool adapter wraps a `*TransientLLMError` (returned by a nested LLM call)
in a standard `fmt.Errorf` wrapper:

```go
return ToolResult{Err: fmt.Errorf("inner llm failed: %w", transientLLMErr)}
```

`Classifier.Classify(wrappedErr)`:

1. `errors.As(wrappedErr, &typed)`: Go's `errors.As` traverses the `%w` chain
   and finds `*TransientLLMError`. Identity rule fires: return
   `*TransientLLMError` unchanged.

**Outcome:** the outer orchestrator sees `ErrorKindTransientLLM` and applies
its retry policy (3 retries with exponential backoff). The `fmt.Errorf` wrapper
does not obscure the `TypedError` classification.

**Example 4: Mixed TypedError and context cancel**

A `CancellationError` (soft cancel, from the orchestrator's own cancel
mechanism) is in the error chain, and the context is also cancelled:

```go
err := &CancellationError{cancelKind: CancellationKindSoft, cause: context.Canceled}
```

`Classifier.Classify(err)`:

1. `errors.As(err, &typed)` succeeds — `*CancellationError` implements
   `TypedError`. Identity rule fires: return `*CancellationError` unchanged,
   with `CancelKind() == CancellationKindSoft`.

**Outcome:** `CancellationKindSoft` is preserved. The context-cancellation
check (step 2) is never reached because the identity rule fired first. This
prevents a soft cancel from being reclassified as a hard cancel.

### 5.3 Classifier interface amendment (CP5 explicit contract)

The `Classifier` interface godoc in Phase 3 already documents the identity
rule. Phase 4 formalizes it as CP5. The `DefaultClassifier` godoc must include
the following:

> The identity rule (step 1) uses `errors.As`, not a type assertion. This
> means any `TypedError` implementation anywhere in the error chain — including
> wrapped via `%w` — will be returned unchanged. Callers who implement custom
> `TypedError` types in adapter layers benefit from this: errors do not need to
> be unwrapped and re-wrapped before passing to `Classify`.
