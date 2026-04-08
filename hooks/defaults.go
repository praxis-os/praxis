// SPDX-License-Identifier: Apache-2.0

package hooks

import (
	"context"

	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/tools"
)

// Compile-time interface checks.
var _ PolicyHook = AllowAllPolicyHook{}
var _ PreLLMFilter = PassThroughPreLLMFilter{}
var _ PostToolFilter = PassThroughPostToolFilter{}

// AllowAllPolicyHook is a [PolicyHook] that unconditionally permits every
// operation. Used as the default when no policy is configured.
type AllowAllPolicyHook struct{}

// Evaluate always returns [Allow] with a nil error.
func (AllowAllPolicyHook) Evaluate(_ context.Context, _ Phase, _ PolicyInput) (Decision, error) {
	return Allow(), nil
}

// PassThroughPreLLMFilter is a [PreLLMFilter] that passes every message list
// through without modification. Used as the default when no pre-LLM filter
// is configured.
type PassThroughPreLLMFilter struct{}

// Filter returns the messages unchanged with no decisions.
func (PassThroughPreLLMFilter) Filter(_ context.Context, messages []llm.Message) ([]llm.Message, []FilterDecision, error) {
	return messages, nil, nil
}

// PassThroughPostToolFilter is a [PostToolFilter] that passes every tool
// result through without modification. Used as the default when no post-tool
// filter is configured.
type PassThroughPostToolFilter struct{}

// Filter returns the result unchanged with no decisions.
func (PassThroughPostToolFilter) Filter(_ context.Context, result tools.ToolResult) (tools.ToolResult, []FilterDecision, error) {
	return result, nil, nil
}
