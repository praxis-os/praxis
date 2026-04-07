// SPDX-License-Identifier: Apache-2.0

package invocation

import "github.com/praxis-os/praxis/llm"

// InvocationRequest carries the inputs for a single orchestrator invocation.
//
// The zero value is valid for fields marked optional; the orchestrator applies
// defaults for any zero-valued optional field before the first LLM call.
type InvocationRequest struct {
	// Messages is the conversation history passed to the LLM on the first
	// round-trip. Required; must contain at least one message.
	Messages []llm.Message

	// Model is the provider-specific model identifier (e.g. "claude-3-5-sonnet-20241022").
	// Required; the orchestrator returns an error if empty.
	Model string

	// Tools is the set of tool definitions made available to the LLM.
	// Optional; nil or empty means the LLM receives no tool definitions.
	Tools []llm.ToolDefinition

	// MaxIterations caps the number of LLM round-trips (including tool-use
	// loops) for this invocation. Optional; zero means use the orchestrator's
	// configured default.
	MaxIterations int

	// Metadata is caller-supplied key-value pairs attached to this invocation.
	// The orchestrator forwards Metadata to hooks and telemetry enrichers but
	// does not interpret the values itself. Optional; nil is treated as empty.
	Metadata map[string]string
}
