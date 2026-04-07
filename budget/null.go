// SPDX-License-Identifier: Apache-2.0

package budget

import (
	"context"
	"time"
)

// Usage carries the current resource consumption for a single invocation.
// The orchestrator accumulates these values across turns and passes a snapshot
// to [Guard.Check] at each turn boundary.
type Usage struct {
	// InputTokens is the cumulative number of input (prompt) tokens consumed.
	InputTokens int64

	// OutputTokens is the cumulative number of output (completion) tokens produced.
	OutputTokens int64

	// ToolCalls is the cumulative number of tool calls dispatched.
	ToolCalls int

	// CostMicros is the estimated cost in micro-dollars (1 USD = 1,000,000 CostMicros),
	// computed from [PriceProvider] rates applied to InputTokens and OutputTokens.
	CostMicros int64

	// Elapsed is the wall-clock duration since the invocation started.
	Elapsed time.Duration
}

// Guard enforces multi-dimensional budget limits for an invocation.
//
// The orchestrator calls Check at each turn boundary (after token counts and
// cost estimates are updated). A non-nil error halts the invocation with a
// budget-exceeded error.
//
// Implementations must be safe for concurrent use.
//
// Stability: frozen-v1.0.
type Guard interface {
	// Check evaluates the current [Usage] against the configured limits.
	// Returns nil if the invocation may continue, or a non-nil error
	// (typically a typed budget error) if a limit has been reached.
	Check(ctx context.Context, usage Usage) error
}

// PriceProvider supplies per-model pricing so the orchestrator can compute
// cost estimates for [Usage.CostMicros].
//
// Implementations must be safe for concurrent use.
//
// Stability: frozen-v1.0.
type PriceProvider interface {
	// InputPricePer1K returns the cost in USD per 1,000 input tokens for
	// the given model identifier. Returns 0 if the model is unknown.
	InputPricePer1K(model string) float64

	// OutputPricePer1K returns the cost in USD per 1,000 output tokens for
	// the given model identifier. Returns 0 if the model is unknown.
	OutputPricePer1K(model string) float64
}

// Compile-time interface checks.
var _ Guard = NullGuard{}
var _ PriceProvider = NullPriceProvider{}

// NullGuard is a [Guard] that never rejects any usage. Used as the default
// when no budget is configured.
type NullGuard struct{}

// Check always returns nil, allowing the invocation to continue.
func (NullGuard) Check(_ context.Context, _ Usage) error { return nil }

// NullPriceProvider is a [PriceProvider] that returns zero for all models.
// Used as the default when no pricing data is configured. Cost estimates
// will be zero for all invocations.
type NullPriceProvider struct{}

// InputPricePer1K returns 0 for all models.
func (NullPriceProvider) InputPricePer1K(_ string) float64 { return 0 }

// OutputPricePer1K returns 0 for all models.
func (NullPriceProvider) OutputPricePer1K(_ string) float64 { return 0 }
