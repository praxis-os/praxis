// SPDX-License-Identifier: Apache-2.0

package llm

// ToolDefinition describes a tool the LLM may call.
type ToolDefinition struct {
	// Name is the tool identifier the LLM uses in tool-call requests.
	// Must match the Name returned by tools.Invoker.
	Name string

	// Description is a human-readable description of the tool's purpose.
	// LLMs use this for tool selection. Required.
	Description string

	// InputSchema is the JSON Schema for the tool's input parameters,
	// serialized as a raw JSON message. The framework forwards this to
	// the provider without interpretation.
	InputSchema []byte
}

// LLMToolCall is a single tool invocation requested by the LLM.
type LLMToolCall struct {
	// CallID is the provider-assigned unique ID for this call.
	// Used to correlate LLMToolResult with LLMToolCall in multi-tool turns.
	CallID string

	// Name is the tool name from ToolDefinition.Name.
	Name string

	// ArgumentsJSON is the tool's input arguments as a raw JSON object.
	ArgumentsJSON []byte
}

// LLMToolCallDelta is an incremental update to a tool call in a streaming
// response. Arguments are delivered in chunks and must be concatenated.
type LLMToolCallDelta struct {
	// CallID is the provider-assigned unique ID for this call.
	CallID string
	// Name is the tool name (may only be present on the first delta).
	Name string
	// ArgumentsDelta is the incremental argument payload.
	ArgumentsDelta string
}

// LLMToolResult carries the output of a tool invocation back to the LLM.
type LLMToolResult struct {
	// CallID matches LLMToolCall.CallID.
	CallID string

	// Content is the tool's output as a string. May be structured JSON
	// or plain text; the LLM interprets it based on context.
	Content string

	// IsError indicates that the tool returned an error state.
	// The LLM will be informed the tool call failed.
	IsError bool
}
