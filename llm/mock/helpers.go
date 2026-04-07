// SPDX-License-Identifier: Apache-2.0

package mock

import (
	"time"

	"github.com/praxis-os/praxis/llm"
)

// Response is a scripted response entry for the mock provider.
// Exactly one of LLMResponse or Err should be set per entry.
// If Err is non-nil it is returned instead of LLMResponse.
// Delay, if positive, causes the provider to pause before returning,
// honoring context cancellation during the wait.
type Response struct {
	// LLMResponse is the response to return when Err is nil.
	LLMResponse llm.LLMResponse

	// Err is the error to return. When non-nil, LLMResponse is ignored.
	Err error

	// Delay is an optional pause before the response is delivered.
	// The delay is interrupted immediately if the context is cancelled.
	Delay time.Duration
}

// NewSimple creates a mock provider that always returns a single text response
// with StopReasonEndTurn. Subsequent calls after the first exhaust the script
// and return an error.
//
// Callers that need the provider to respond to many calls should use [New] with
// multiple [Response] entries.
func NewSimple(text string) *Provider {
	return New(Response{
		LLMResponse: llm.LLMResponse{
			Message: llm.Message{
				Role:  llm.RoleAssistant,
				Parts: []llm.MessagePart{llm.TextPart(text)},
			},
			StopReason: llm.StopReasonEndTurn,
			Usage:      llm.TokenUsage{InputTokens: 10, OutputTokens: int64(len(text))},
		},
	})
}

// NewWithToolCalls creates a mock provider that returns a single response
// containing the provided tool calls with StopReasonToolUse.
func NewWithToolCalls(calls ...*llm.LLMToolCall) *Provider {
	parts := make([]llm.MessagePart, 0, len(calls))
	for _, c := range calls {
		parts = append(parts, llm.ToolCallPart(c))
	}
	return New(Response{
		LLMResponse: llm.LLMResponse{
			Message: llm.Message{
				Role:  llm.RoleAssistant,
				Parts: parts,
			},
			StopReason: llm.StopReasonToolUse,
			Usage:      llm.TokenUsage{InputTokens: 10, OutputTokens: int64(len(calls))},
		},
	})
}
