// SPDX-License-Identifier: Apache-2.0

package llm

// LLMResponse is the provider-agnostic output from a single LLM call.
type LLMResponse struct {
	// Message is the assistant's response, containing text parts and/or
	// tool-call parts.
	Message Message

	// StopReason identifies why the LLM stopped generating.
	StopReason StopReason

	// Usage reports token consumption for budget accounting.
	Usage TokenUsage
}

// StopReason identifies the completion reason for an LLM response.
type StopReason string

const (
	StopReasonEndTurn      StopReason = "end_turn"
	StopReasonToolUse      StopReason = "tool_use"
	StopReasonMaxTokens    StopReason = "max_tokens"
	StopReasonStopSequence StopReason = "stop_sequence"
)

// String returns the string representation of the StopReason.
func (s StopReason) String() string { return string(s) }

// TokenUsage reports token counts for a single LLM call.
// Used by the orchestrator to record against budget.Guard.
type TokenUsage struct {
	// InputTokens is the number of input tokens consumed.
	InputTokens int64
	// OutputTokens is the number of output tokens generated.
	OutputTokens int64
	// CachedInputTokens is the number of input tokens served from cache.
	// Zero if the provider does not support or report prompt caching.
	CachedInputTokens int64
}

// TotalTokens returns InputTokens + OutputTokens.
func (u TokenUsage) TotalTokens() int64 {
	return u.InputTokens + u.OutputTokens
}

// LLMStreamChunk is a single chunk in a streaming LLM response.
type LLMStreamChunk struct {
	// Delta is the incremental text content for this chunk.
	// Empty for non-text chunks.
	Delta string

	// ToolCallDelta carries a partial tool call for this chunk.
	// Non-nil when the LLM is streaming a tool-call argument payload.
	ToolCallDelta *LLMToolCallDelta

	// Final is true on the last chunk. The Final chunk carries the
	// complete LLMResponse summary (usage, stop reason).
	Final bool

	// Response is populated only on the Final chunk.
	Response *LLMResponse

	// Err is non-nil if an error occurred after streaming began.
	// When non-nil, no further chunks follow and the channel is closed.
	Err error
}
