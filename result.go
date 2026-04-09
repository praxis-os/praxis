// SPDX-License-Identifier: Apache-2.0

package praxis

import (
	"github.com/praxis-os/praxis/budget"
	"github.com/praxis-os/praxis/event"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/state"
)

// InvocationResult is the value returned by the orchestrator after an
// invocation reaches a terminal state.
type InvocationResult struct {
	Response       *llm.Message
	BudgetSnapshot budget.BudgetSnapshot
	InvocationID   string
	SignedIdentity string
	Events         []event.InvocationEvent
	FinalState     state.State
}
