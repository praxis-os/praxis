// SPDX-License-Identifier: Apache-2.0

package praxis

import (
	"github.com/praxis-os/praxis/budget"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/state"
)

// InvocationResult is the value returned by the orchestrator after an
// invocation reaches a terminal state.
type InvocationResult struct {
	// InvocationID is the unique identifier assigned to this invocation.
	InvocationID string

	// FinalState is the terminal [state.State] reached at the end of the
	// invocation. It is always one of [state.Completed], [state.Failed],
	// [state.Cancelled], [state.BudgetExceeded], or [state.ApprovalRequired].
	FinalState state.State

	// Response is the final LLM message produced by the model.
	// Non-nil when FinalState is [state.Completed]; may be nil for other
	// terminal states.
	Response *llm.Message

	// BudgetSnapshot is the final budget consumption snapshot.
	BudgetSnapshot budget.BudgetSnapshot

	// Events is the ordered list of lifecycle events emitted during the
	// invocation. Populated on the sync path (Invoke); nil on the stream
	// path (InvokeStream).
	Events []InvocationEvent
}
