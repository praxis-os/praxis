// SPDX-License-Identifier: Apache-2.0

package tools

import "context"

// Invoker dispatches tool calls requested by the LLM.
//
// The orchestrator routes each [ToolCall] produced by a provider response
// through the configured Invoker and collects the resulting [ToolResult]
// values to include in the next LLM turn.
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
	// via [ToolResult.Err] and an appropriate [ToolStatus] instead, so
	// the LLM can observe and potentially recover from them.
	Invoke(ctx context.Context, ictx InvocationContext, call ToolCall) (ToolResult, error)
}

// InvokerFunc is an adapter to allow the use of ordinary functions as
// [Invoker] implementations.
type InvokerFunc func(ctx context.Context, ictx InvocationContext, call ToolCall) (ToolResult, error)

// Invoke calls f(ctx, ictx, call).
func (f InvokerFunc) Invoke(ctx context.Context, ictx InvocationContext, call ToolCall) (ToolResult, error) {
	return f(ctx, ictx, call)
}
