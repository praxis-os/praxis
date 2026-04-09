// SPDX-License-Identifier: Apache-2.0

package hooks

import (
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/tools"
)

// Phase represents the lifecycle phase at which a [PolicyHook] is evaluated.
type Phase string

const (
	// PhasePreInvocation is evaluated before the first LLM call of an invocation.
	PhasePreInvocation Phase = "pre_invocation"

	// PhasePreLLMInput is evaluated immediately before each LLM call.
	PhasePreLLMInput Phase = "pre_llm_input"

	// PhasePostToolOutput is evaluated after each tool result is collected.
	PhasePostToolOutput Phase = "post_tool_output"

	// PhasePostInvocation is evaluated after the invocation completes
	// (regardless of outcome).
	PhasePostInvocation Phase = "post_invocation"
)

// Verdict indicates the outcome of a policy evaluation.
type Verdict string

const (
	// VerdictAllow permits the operation to proceed.
	VerdictAllow Verdict = "allow"

	// VerdictDeny halts the invocation with a policy error.
	VerdictDeny Verdict = "deny"

	// VerdictRequireApproval suspends the invocation pending human approval.
	VerdictRequireApproval Verdict = "require_approval"

	// VerdictLog permits the operation but records an audit log entry.
	VerdictLog Verdict = "log"

	// VerdictContinue forces the orchestrator to perform an additional LLM turn
	// instead of completing. Only meaningful at [PhasePostInvocation]; at other
	// phases it behaves identically to [VerdictAllow].
	VerdictContinue Verdict = "continue"
)

// Decision is the verdict returned by a [PolicyHook].
type Decision struct {
	// Verdict is the policy outcome.
	Verdict Verdict

	// Metadata carries arbitrary key-value data associated with the decision.
	// Forwarded to telemetry and the ApprovalSnapshot for RequireApproval.
	Metadata map[string]any

	// Reason is a human-readable explanation of the decision.
	Reason string
}

// Allow returns a Decision that permits the operation to proceed.
func Allow() Decision { return Decision{Verdict: VerdictAllow} }

// Deny returns a Decision that halts the operation with the given reason.
func Deny(reason string) Decision { return Decision{Verdict: VerdictDeny, Reason: reason} }

// RequireApproval returns a Decision that suspends the invocation pending
// human approval.
func RequireApproval(reason string, metadata map[string]any) Decision {
	return Decision{Verdict: VerdictRequireApproval, Reason: reason, Metadata: metadata}
}

// Log returns a Decision that permits the operation but records an audit entry.
func Log(reason string) Decision { return Decision{Verdict: VerdictLog, Reason: reason} }

// Continue returns a Decision that forces an additional LLM turn at
// [PhasePostInvocation]. At other phases it behaves like [Allow].
func Continue(reason string) Decision { return Decision{Verdict: VerdictContinue, Reason: reason} }

// PolicyInput carries invocation state to a [PolicyHook] for evaluation.
type PolicyInput struct {
	// InvocationID is the unique identifier for the current invocation.
	InvocationID string

	// Model is the LLM model identifier being used.
	Model string

	// SystemPrompt is the system prompt, if any.
	SystemPrompt string

	// Messages is the current conversation history.
	Messages []llm.Message

	// ToolResult is the most recent tool result. Non-nil only at
	// PhasePostToolOutput.
	ToolResult *tools.ToolResult

	// LLMResponse is the final LLM response. Non-nil only at
	// PhasePostInvocation.
	LLMResponse *llm.LLMResponse

	// Metadata is caller-supplied key-value pairs from the InvocationRequest.
	Metadata map[string]string
}

// FilterAction indicates what action a filter took on a field.
type FilterAction string

const (
	// FilterActionPass indicates the field was not modified.
	FilterActionPass FilterAction = "pass"

	// FilterActionRedact indicates the field was redacted.
	FilterActionRedact FilterAction = "redact"

	// FilterActionLog indicates the field was flagged for logging but not modified.
	FilterActionLog FilterAction = "log"

	// FilterActionBlock indicates the entire operation should be blocked.
	FilterActionBlock FilterAction = "block"
)

// FilterDecision records the action a filter took on a specific field.
type FilterDecision struct {
	// Action is the filter action taken.
	Action FilterAction

	// Field identifies which field was affected (e.g., "messages[2].text").
	Field string

	// Reason explains why this action was taken.
	Reason string
}
