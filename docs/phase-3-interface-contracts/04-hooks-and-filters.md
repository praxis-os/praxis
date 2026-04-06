# Phase 3 — Hooks and Filters

**Stability tier:** `frozen-v1.0`
**Decisions:** D33, D42
**Package:** `MODULE_PATH_TBD/hooks`

---

## Overview

The `hooks` package defines three interfaces: `PolicyHook`, `PreLLMFilter`,
and `PostToolFilter`. Together they constitute the policy evaluation and
content-filtering seams at every security-sensitive boundary in the
orchestrator.

---

## Policy lifecycle phases

The orchestrator owns four policy-hook phases:

| Phase constant | Triggered at | Notes |
|---|---|---|
| `PhasePreInvocation` | `PreHook` state entry | Runs once per invocation before any LLM call |
| `PhasePreLLMInput` | `LLMCall` state entry | Runs before each LLM call (applies to all turns) |
| `PhasePostToolOutput` | `PostToolFilter` state entry | Runs after tool output filter chain |
| `PhasePostInvocation` | `PostHook` state entry | Runs once per invocation after all turns |

A fifth conceptual phase, mid-tool-call policy, is explicitly out of scope for
the orchestrator; it belongs to the `tools.Invoker` implementation (seed §5,
seed §12 REVIEW.md).

---

## `PolicyHook` interface

```go
// PolicyHook evaluates policy at named lifecycle phases.
//
// Evaluate is called by the orchestrator at each of the four hook phases.
// The returned Decision drives the next state transition:
//   - VerdictAllow:           proceed to the next state.
//   - VerdictDeny:            transition to Failed with PolicyDeniedError.
//   - VerdictRequireApproval: transition to ApprovalRequired (D17).
//   - VerdictLog:             proceed; the orchestrator logs the decision
//                             but does not alter the execution path.
//
// Errors returned from Evaluate are hook-internal failures (policy engine
// unreachable, timeout) and are distinct from policy decisions. A non-nil
// error always routes the invocation to Failed, regardless of any partial
// Decision value returned alongside it. Callers must not return
// (Decision{VerdictAllow}, someErr) — the error wins.
//
// PolicyHook implementations must be safe for concurrent use.
//
// Stability: frozen-v1.0.
type PolicyHook interface {
    Evaluate(ctx context.Context, phase Phase, input PolicyInput) (Decision, error)
}

// Phase identifies the lifecycle point at which a PolicyHook is evaluated.
type Phase string

const (
    PhasePreInvocation  Phase = "pre_invocation"
    PhasePreLLMInput    Phase = "pre_llm_input"
    PhasePostToolOutput Phase = "post_tool_output"
    PhasePostInvocation Phase = "post_invocation"
)

// PolicyInput is the context provided to PolicyHook.Evaluate.
// It carries the relevant state snapshot for the evaluation phase.
//
// PolicyInput uses projection fields (D51) instead of embedding
// InvocationRequest directly, to avoid an import cycle between hooks
// and the praxis root package. The orchestrator populates these fields
// from InvocationRequest before calling Evaluate.
//
// Fields are populated depending on Phase:
//   - PreInvocation:  InvocationID, Model, SystemPrompt, Metadata set.
//                     Messages nil; ToolResult, LLMResponse nil.
//   - PreLLMInput:    All above plus Messages.
//   - PostToolOutput: All above plus Messages and ToolResult.
//   - PostInvocation: All fields set.
type PolicyInput struct {
    // InvocationID is the unique ID for the current invocation.
    InvocationID string

    // Model is the model identifier for this invocation (from InvocationRequest).
    Model string

    // SystemPrompt is the system prompt for this invocation (from InvocationRequest).
    SystemPrompt string

    // Messages is the current conversation history at the time of evaluation.
    // Nil during PhasePreInvocation (no turns have occurred yet).
    Messages []llm.Message

    // ToolResult is the most recent tool result.
    // Non-nil only during PhasePostToolOutput.
    ToolResult *tools.ToolResult

    // LLMResponse is the most recent LLM response.
    // Non-nil only during PhasePostInvocation.
    LLMResponse *llm.LLMResponse

    // Metadata is the opaque key-value map from InvocationRequest.Metadata.
    // The framework forwards it unchanged.
    Metadata map[string]string
}
```

---

## `Decision` type (D33)

```go
// Verdict is the outcome of a PolicyHook evaluation.
type Verdict string

const (
    // VerdictAllow permits the invocation to proceed.
    VerdictAllow Verdict = "allow"

    // VerdictDeny rejects the invocation. The orchestrator transitions
    // to Failed with a PolicyDeniedError.
    VerdictDeny Verdict = "deny"

    // VerdictRequireApproval suspends the invocation pending human
    // approval. The orchestrator transitions to ApprovalRequired (D17).
    // The Decision.Metadata and Decision.Reason fields are forwarded
    // opaquely into ApprovalRequiredError.Snapshot.ApprovalMetadata
    // and surfaced on EventTypeApprovalRequired.
    VerdictRequireApproval Verdict = "require_approval"

    // VerdictLog allows the invocation to proceed while emitting an
    // audit note. The orchestrator logs the Decision.Reason at INFO
    // level and emits a lifecycle event; execution continues as Allow.
    VerdictLog Verdict = "log"
)

// Decision is the value returned by PolicyHook.Evaluate.
//
// Decision is a value type. The loop detects the Verdict from the value
// alone, without type assertions or error inspection (D17 requirement).
//
// Constructors are provided for ergonomic use; direct struct literals
// are also valid.
type Decision struct {
    // Verdict is the policy outcome. Required; zero value is not valid
    // (treated as a SystemError by the orchestrator).
    Verdict Verdict

    // Metadata is an opaque map forwarded without inspection to the
    // ApprovalRequiredError and EventTypeApprovalRequired payload.
    // Relevant only when Verdict == VerdictRequireApproval.
    // Ignored for other verdicts.
    Metadata map[string]any

    // Reason is a human-readable explanation.
    // Optional; may be empty. Logged at INFO for VerdictLog,
    // embedded in PolicyDeniedError.Error() for VerdictDeny.
    Reason string
}

// Allow returns a Decision with VerdictAllow.
func Allow() Decision { return Decision{Verdict: VerdictAllow} }

// Deny returns a Decision with VerdictDeny and an optional reason.
func Deny(reason string) Decision { return Decision{Verdict: VerdictDeny, Reason: reason} }

// RequireApproval returns a Decision with VerdictRequireApproval,
// the given reason, and optional opaque metadata.
func RequireApproval(reason string, metadata map[string]any) Decision {
    return Decision{Verdict: VerdictRequireApproval, Reason: reason, Metadata: metadata}
}

// Log returns a Decision with VerdictLog and an audit reason.
func Log(reason string) Decision { return Decision{Verdict: VerdictLog, Reason: reason} }
```

---

## `PreLLMFilter` interface (D42)

```go
// PreLLMFilter filters the message list immediately before each LLM call.
//
// Filter receives the complete message list for the upcoming LLM call.
// It returns:
//   - filtered: the (possibly modified) message list to send to the LLM.
//   - decisions: one FilterDecision per filter action taken. May be empty.
//   - err: non-nil only on filter-internal failure (I/O error, timeout).
//
// The orchestrator inspects the returned decisions:
//   - Any FilterActionBlock causes the invocation to transition to Failed.
//   - FilterActionRedact and FilterActionLog are passed to the
//     telemetry.LifecycleEventEmitter for Phase 4 event mapping.
//   - FilterActionPass decisions are informational; no action is taken.
//
// Filter is called on every LLM call (both the initial call and
// LLMContinuation turns). Implementations must be safe for concurrent use.
//
// Stability: frozen-v1.0.
type PreLLMFilter interface {
    Filter(ctx context.Context, messages []llm.Message) (filtered []llm.Message, decisions []FilterDecision, err error)
}
```

---

## `PostToolFilter` interface (D42)

```go
// PostToolFilter filters tool results after invocation and before they
// are appended to the conversation history.
//
// Tool outputs are treated as untrusted by contract (seed §5).
// This filter is the primary defense against prompt injection via tool
// responses.
//
// Filter receives one tools.ToolResult at a time. It returns:
//   - filtered: the (possibly modified) ToolResult to inject into the
//               conversation.
//   - decisions: one FilterDecision per filter action taken.
//   - err: non-nil only on filter-internal failure.
//
// Any FilterActionBlock causes the invocation to transition to Failed.
//
// PostToolFilter implementations must be safe for concurrent use.
// Under parallel tool dispatch (D24), each tool result is filtered
// sequentially after the batch completes, not concurrently.
//
// Stability: frozen-v1.0.
type PostToolFilter interface {
    Filter(ctx context.Context, result tools.ToolResult) (filtered tools.ToolResult, decisions []FilterDecision, err error)
}
```

---

## `FilterDecision` type (D42)

```go
// FilterAction is the action taken by a filter on a piece of content.
type FilterAction string

const (
    // FilterActionPass means the content is forwarded without modification.
    FilterActionPass FilterAction = "pass"

    // FilterActionRedact means a sensitive element was removed or replaced.
    // The filter must return the redacted content in the filtered return value.
    FilterActionRedact FilterAction = "redact"

    // FilterActionLog means the filter noted a concern but allowed the
    // content through. The orchestrator emits a lifecycle event.
    FilterActionLog FilterAction = "log"

    // FilterActionBlock means the content is rejected entirely. The
    // orchestrator transitions the invocation to Failed with a
    // PolicyDeniedError.
    FilterActionBlock FilterAction = "block"
)

// FilterDecision records a single filter action on a content element.
//
// A single Filter call may return multiple FilterDecisions — for example,
// a filter that both redacts a PII field and logs a prompt-injection
// suspicion returns two decisions.
type FilterDecision struct {
    // Action is the filter outcome for this element.
    Action FilterAction

    // Field is the dot-path to the content element acted on.
    // Empty string means the decision applies to the whole input.
    // Examples: "messages[2].parts[0].text", "result.content".
    Field string

    // Reason is a human-readable explanation. May be empty.
    // Logged alongside the filter action.
    Reason string
}
```

---

## Default (null) implementations

```go
// AllowAllPolicyHook is the default PolicyHook implementation.
// It returns Decision{VerdictAllow} for every evaluation.
// Safe for concurrent use.
//
// Package: MODULE_PATH_TBD/hooks
var AllowAllPolicyHook PolicyHook = allowAllHook{}

// PassThroughPreLLMFilter is the default PreLLMFilter implementation.
// It returns the input messages unchanged with no decisions.
// Safe for concurrent use.
//
// Package: MODULE_PATH_TBD/hooks
var PassThroughPreLLMFilter PreLLMFilter = passThroughPreFilter{}

// PassThroughPostToolFilter is the default PostToolFilter implementation.
// It returns the input ToolResult unchanged with no decisions.
// Safe for concurrent use.
//
// Package: MODULE_PATH_TBD/hooks
var PassThroughPostToolFilter PostToolFilter = passThroughPostFilter{}
```

---

## Concurrency contract

All `hooks` interface implementations must be safe for concurrent use. The
orchestrator may evaluate the same `PolicyHook` or filter from multiple
concurrent invocations' goroutines.
