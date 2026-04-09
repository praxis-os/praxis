// SPDX-License-Identifier: Apache-2.0

// Package llm defines the provider-agnostic interface for LLM adapters.
//
// The [Provider] interface is the boundary between the praxis orchestrator
// and LLM backends (Anthropic, OpenAI, etc.). Each adapter translates the
// generic [LLMRequest] into provider-specific API calls and returns a
// generic [LLMResponse].
//
// Stability: frozen-v1.0 (D41).
package llm

import "context"

// Provider is the provider-agnostic interface over LLM adapters.
//
// Each method may perform network I/O. All methods take a [context.Context]
// as the first parameter; implementations must respect context cancellation.
//
// Implementations must be safe for concurrent use. The orchestrator may
// call Complete or Stream from multiple goroutines on the same Provider
// instance.
//
// Stability: frozen-v1.0.
type Provider interface {
	// Complete sends a request to the LLM and blocks until the full response
	// is available. Use [Provider.Stream] for streaming responses.
	//
	// Returns a classified TypedError on failure:
	//   - TransientLLMError for retryable failures (rate limit, 5xx, timeout).
	//   - PermanentLLMError for non-retryable failures (invalid request, 4xx).
	//
	// The orchestrator's retry policy (3x exponential backoff for transient
	// errors) operates at this boundary.
	Complete(ctx context.Context, req LLMRequest) (LLMResponse, error)

	// Stream sends a request to the LLM and returns a channel of stream
	// chunks. The channel is closed when the response is complete or when
	// an error occurs.
	//
	// The returned error is non-nil only for setup failures (auth, network
	// before first token). Errors after the channel opens are delivered as
	// [LLMStreamChunk] with a non-nil Err field.
	//
	// The returned channel is closed by the provider when the response is
	// complete. Callers must drain the channel to avoid leaks.
	//
	// Adapters that do not natively support streaming must implement Stream
	// by calling Complete and delivering a single chunk containing the
	// full response.
	Stream(ctx context.Context, req LLMRequest) (<-chan LLMStreamChunk, error)

	// Name returns the provider's canonical name (e.g., "anthropic", "openai").
	// Used in budget.PriceProvider lookups.
	Name() string

	// SupportsParallelToolCalls reports whether the provider can process
	// multiple tool calls returned in a single response concurrently.
	// The orchestrator uses this to gate parallel dispatch (D24).
	SupportsParallelToolCalls() bool

	// Capabilities returns a snapshot of the provider's supported features.
	// The snapshot is immutable for the lifetime of the provider instance.
	Capabilities() Capabilities
}

// Capabilities is a snapshot of a provider's supported features.
// Returned by [Provider.Capabilities]; immutable for the provider's lifetime.
type Capabilities struct {
	SupportedStopReasons      []StopReason
	MaxContextTokens          int64
	SupportsStreaming         bool
	SupportsParallelToolCalls bool
	SupportsSystemPrompt      bool
}
