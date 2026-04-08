// SPDX-License-Identifier: Apache-2.0

package tools

import "context"

// Compile-time interface check.
var _ Invoker = NullInvoker{}

// NullInvoker is a no-op tool invoker that returns a not-implemented result
// for every call. Used as the default when no tools are configured.
//
// It never returns a framework error; instead it sets [ToolStatusNotImplemented]
// so the LLM receives a well-formed response indicating no invoker is present.
type NullInvoker struct{}

// Invoke returns a not-implemented result indicating that no tool invoker is
// configured. The framework error return value is always nil.
func (NullInvoker) Invoke(_ context.Context, _ InvocationContext, call ToolCall) (ToolResult, error) {
	return ToolResult{
		Status:  ToolStatusNotImplemented,
		Content: "no tool invoker configured",
		CallID:  call.CallID,
	}, nil
}
