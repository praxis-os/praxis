// SPDX-License-Identifier: Apache-2.0

package llm

// LLMRequest is the provider-agnostic input to a single LLM call.
type LLMRequest struct {
	ExtraParams  map[string]any
	Model        string
	SystemPrompt string
	Messages     []Message
	Tools        []ToolDefinition
	MaxTokens    int
	Temperature  float64
}
