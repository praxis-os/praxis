// SPDX-License-Identifier: Apache-2.0

package llm

// LLMResponse is the provider-agnostic output from a single LLM call.
type LLMResponse struct {
	StopReason StopReason
	Message    Message
	Usage      TokenUsage
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
	Err           error
	ToolCallDelta *LLMToolCallDelta
	Response      *LLMResponse
	Delta         string
	Final         bool
}
