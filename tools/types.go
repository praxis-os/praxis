// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"github.com/praxis-os/praxis/budget"
	"go.opentelemetry.io/otel/trace"
)

// ToolStatus indicates the outcome of a tool invocation.
type ToolStatus string

const (
	// ToolStatusSuccess indicates the tool executed successfully.
	ToolStatusSuccess ToolStatus = "success"

	// ToolStatusDenied indicates the tool invocation was denied by policy.
	ToolStatusDenied ToolStatus = "denied"

	// ToolStatusNotImplemented indicates no invoker is configured for this tool.
	ToolStatusNotImplemented ToolStatus = "not_implemented"

	// ToolStatusError indicates the tool invocation failed with an error.
	ToolStatusError ToolStatus = "error"
)

// ToolCall represents a single tool invocation requested by the LLM.
type ToolCall struct {
	// CallID is the unique identifier for this tool call, assigned by the LLM.
	CallID string

	// Name is the tool name as declared in the tool definition.
	Name string

	// ArgumentsJSON is the raw JSON arguments produced by the LLM.
	ArgumentsJSON []byte
}

// ToolResult carries the output of a tool invocation back to the LLM.
type ToolResult struct {
	// Status indicates the outcome of the tool invocation.
	Status ToolStatus

	// Content is the tool's output, presented to the LLM in the next turn.
	Content string

	// Err carries a typed error from the tool invocation. May be non-nil
	// even when Status is ToolStatusSuccess (e.g., for nested invocation
	// errors that were handled).
	Err error

	// CallID is the tool call identifier, echoed from the ToolCall.
	CallID string
}

// InvocationContext carries ambient invocation state to tool implementations.
//
// Tool invokers receive this context so they can participate in budget
// tracking, tracing, and identity propagation without needing a direct
// reference to the orchestrator.
type InvocationContext struct {
	// InvocationID is the unique identifier for the current invocation.
	InvocationID string

	// Budget is a snapshot of the current budget consumption.
	Budget budget.BudgetSnapshot

	// SpanContext is the OpenTelemetry span context for the current invocation.
	SpanContext trace.SpanContext

	// SignedIdentity is the signed identity token for the current invocation,
	// if identity signing is configured. Empty otherwise.
	SignedIdentity string

	// Metadata is caller-supplied key-value pairs from the InvocationRequest.
	Metadata map[string]string
}
