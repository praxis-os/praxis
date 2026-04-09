// SPDX-License-Identifier: Apache-2.0

package hooks

import (
	"context"

	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/tools"
)

// PolicyHook evaluates invocation state at defined lifecycle phases and
// returns a [Decision] indicating whether the operation may proceed.
//
// Implementations must be safe for concurrent use.
//
// Stability: frozen-v1.0.
type PolicyHook interface {
	// Evaluate is called at the given phase with the current invocation
	// state. The returned [Decision] determines whether the invocation
	// continues, is denied, requires approval, or is logged.
	Evaluate(ctx context.Context, phase Phase, input PolicyInput) (Decision, error)
}

// PreLLMFilter can inspect or mutate the message list before it is sent
// to the LLM provider. Returns the filtered messages, a list of per-field
// decisions describing what was changed, and any error.
//
// A [FilterDecision] with [FilterActionBlock] causes the invocation to fail.
//
// Implementations must be safe for concurrent use.
//
// Stability: frozen-v1.0.
type PreLLMFilter interface {
	Filter(ctx context.Context, messages []llm.Message) (filtered []llm.Message, decisions []FilterDecision, err error)
}

// PreToolFilter can inspect, modify, or block a [tools.ToolCall] before it is
// dispatched to the tool invoker. Returns the (potentially modified) call, a
// list of per-field decisions, and any error.
//
// A [FilterDecision] with [FilterActionBlock] causes the invocation to fail.
//
// Implementations must be safe for concurrent use.
//
// Stability: frozen-v1.0.
type PreToolFilter interface {
	Filter(ctx context.Context, call tools.ToolCall) (filtered tools.ToolCall, decisions []FilterDecision, err error)
}

// PostToolFilter can inspect or mutate a [tools.ToolResult] before it is
// included in the next LLM turn. Returns the filtered result, a list of
// per-field decisions, and any error.
//
// A [FilterDecision] with [FilterActionBlock] causes the invocation to fail.
//
// Implementations must be safe for concurrent use.
//
// Stability: frozen-v1.0.
type PostToolFilter interface {
	Filter(ctx context.Context, result tools.ToolResult) (filtered tools.ToolResult, decisions []FilterDecision, err error)
}
