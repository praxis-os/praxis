// SPDX-License-Identifier: Apache-2.0

package praxis

import (
	"github.com/praxis-os/praxis/budget"
	"github.com/praxis-os/praxis/llm"
)

// InvocationRequest carries the inputs for a single orchestrator invocation.
//
// The zero value is valid for fields marked optional; the orchestrator applies
// defaults for any zero-valued optional field before the first LLM call.
type InvocationRequest struct {
	Metadata     map[string]string
	Model        string
	SystemPrompt string
	ParentToken  string
	Messages     []llm.Message
	Tools        []llm.ToolDefinition
	BudgetConfig budget.Config
	MaxTurns     int
}
