// SPDX-License-Identifier: Apache-2.0

package invocation

import (
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/state"
)

// InvocationResult is the value returned by the orchestrator after an
// invocation reaches a terminal state.
//
// When Error is nil the invocation completed successfully and Response holds
// the final LLM output. When Error is non-nil the invocation failed, was
// cancelled, exceeded its budget, or was held for approval; FinalState
// identifies which terminal condition occurred.
type InvocationResult struct {
	// Response is the final [llm.LLMResponse] produced by the LLM.
	// Populated when FinalState is [state.Completed]; may be zero-valued
	// for other terminal states.
	Response llm.LLMResponse

	// FinalState is the terminal [state.State] reached at the end of the
	// invocation. It is always one of [state.Completed], [state.Failed],
	// [state.Cancelled], [state.BudgetExceeded], or [state.ApprovalRequired].
	FinalState state.State

	// Iterations is the total number of LLM round-trips that occurred during
	// the invocation, including the initial call and any tool-use continuations.
	Iterations int

	// TokenUsage is the cumulative token consumption across all LLM round-trips
	// in this invocation.
	TokenUsage TokenUsage

	// Error is the error that caused a non-[state.Completed] terminal state.
	// Nil when FinalState is [state.Completed].
	Error error
}

// TokenUsage records cumulative token consumption across all LLM round-trips
// in a single invocation.
//
// This is an invocation-level aggregate. Per-call counts are available on
// each [llm.LLMResponse] via its [llm.TokenUsage] field.
type TokenUsage struct {
	// InputTokens is the total number of input tokens consumed across all
	// LLM calls in the invocation.
	InputTokens int64

	// OutputTokens is the total number of output tokens generated across all
	// LLM calls in the invocation.
	OutputTokens int64

	// TotalTokens is InputTokens + OutputTokens.
	TotalTokens int64
}
