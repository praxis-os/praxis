// SPDX-License-Identifier: Apache-2.0

package llm

// LLMRequest is the provider-agnostic input to a single LLM call.
type LLMRequest struct {
	// Messages is the full conversation history for this call, including
	// any tool results from previous turns.
	Messages []Message

	// Model is the provider-specific model identifier.
	Model string

	// Tools is the list of tools the LLM may call in this turn.
	Tools []ToolDefinition

	// SystemPrompt is the system-level instruction.
	// Empty string means no system prompt.
	SystemPrompt string

	// MaxTokens limits the response length. Zero means provider default.
	MaxTokens int

	// Temperature controls sampling randomness. Zero uses provider default.
	Temperature float64

	// ExtraParams is an opaque key-value map for provider-specific
	// parameters not covered by the standard fields. The orchestrator
	// forwards ExtraParams to the adapter unchanged.
	ExtraParams map[string]any
}
