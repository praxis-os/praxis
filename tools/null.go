// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"

	"github.com/praxis-os/praxis/llm"
)

// Invoker dispatches tool calls requested by the LLM.
//
// The orchestrator routes each [llm.LLMToolCall] produced by a provider
// response through the configured Invoker and collects the resulting
// [llm.LLMToolResult] values to include in the next LLM turn.
//
// Implementations must be safe for concurrent use. The orchestrator may
// invoke multiple tool calls in parallel when the provider supports it.
//
// Stability: frozen-v1.0.
type Invoker interface {
	// Invoke executes a single tool call and returns its result.
	//
	// A non-nil error indicates a framework-level failure (e.g., the
	// invoker itself is broken). Tool-level errors should be signalled
	// via [llm.LLMToolResult.IsError] instead, so the LLM can observe
	// and potentially recover from them.
	Invoke(ctx context.Context, call llm.LLMToolCall) (llm.LLMToolResult, error)
}

// Compile-time interface check.
var _ Invoker = NullInvoker{}

// NullInvoker is a no-op tool invoker that returns a safe error result for
// every call. Used as the default when no tools are configured.
//
// It never returns a framework error; instead it sets [llm.LLMToolResult.IsError]
// so the LLM receives a well-formed response indicating no invoker is present.
type NullInvoker struct{}

// Invoke returns an error result indicating that no tool invoker is configured.
// The framework error return value is always nil.
func (NullInvoker) Invoke(_ context.Context, call llm.LLMToolCall) (llm.LLMToolResult, error) {
	return llm.LLMToolResult{
		CallID:  call.CallID,
		Content: "no tool invoker configured",
		IsError: true,
	}, nil
}
