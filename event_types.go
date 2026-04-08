// SPDX-License-Identifier: Apache-2.0

package praxis

// Event type constants define the 21 lifecycle events emitted by the
// orchestrator during an invocation (D18, D31, D52b).
//
// Conventions:
//   - "invocation.*" for invocation-lifecycle events
//   - "prehook.*", "posthook.*" for policy hook events
//   - "llmcall.*", "llmcontinuation.*" for LLM call events
//   - "tooldecision.*", "toolcall.*", "posttoolfilter.*" for tool-cycle events
//   - "budget.*" for budget events
//   - "approval.*" for human-approval events
//   - "filter.*" for content-analysis events from filter chains (D52b)
//
// Stability: frozen-v1.0.
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
	// No matching *Completed event: ToolDecision is a synchronous in-loop
	// check, not an I/O-bound operation (D18 rationale).
	EventTypeToolDecisionStarted EventType = "tooldecision.started"

	// EventTypeToolCallStarted is emitted at ToolDecision -> ToolCall.
	// InvocationEvent.ToolCallID and InvocationEvent.ToolName are set.
	EventTypeToolCallStarted EventType = "toolcall.started"

	// EventTypeToolCallCompleted is emitted at ToolCall -> PostToolFilter.
	// InvocationEvent.ToolCallID is set.
	EventTypeToolCallCompleted EventType = "toolcall.completed"

	// EventTypePostToolFilterStarted is emitted at PostToolFilter state entry.
	// InvocationEvent.ToolCallID is set.
	EventTypePostToolFilterStarted EventType = "posttoolfilter.started"

	// EventTypePostToolFilterCompleted is emitted at PostToolFilter -> LLMContinuation.
	// InvocationEvent.ToolCallID is set.
	EventTypePostToolFilterCompleted EventType = "posttoolfilter.completed"

	// EventTypeLLMContinuationStarted is emitted at LLMContinuation state entry.
	// Tool results injected; next LLM call prepared.
	EventTypeLLMContinuationStarted EventType = "llmcontinuation.started"

	// EventTypePostHookStarted is emitted at PostHook state entry.
	EventTypePostHookStarted EventType = "posthook.started"

	// EventTypePostHookCompleted is emitted on terminal entry from PostHook.
	EventTypePostHookCompleted EventType = "posthook.completed"

	// --- Content-analysis events (2, D52b) ---

	// EventTypePIIRedacted is emitted when a filter redacts PII from content.
	EventTypePIIRedacted EventType = "filter.pii_redacted"

	// EventTypePromptInjectionSuspected is emitted when a filter detects a
	// suspected prompt injection attempt.
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

// terminalEventTypes is the set of event types that terminate a stream.
var terminalEventTypes = map[EventType]bool{
	EventTypeInvocationCompleted: true,
	EventTypeInvocationFailed:    true,
	EventTypeInvocationCancelled: true,
	EventTypeBudgetExceeded:      true,
	EventTypeApprovalRequired:    true,
}

// IsTerminal reports whether this event type represents a terminal state.
func (t EventType) IsTerminal() bool {
	return terminalEventTypes[t]
}
