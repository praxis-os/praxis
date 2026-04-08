// SPDX-License-Identifier: Apache-2.0

package budget

import "context"

// Guard enforces multi-dimensional budget limits for an invocation.
//
// The orchestrator calls RecordTokens, RecordToolCall, and RecordCost as
// resources are consumed, then calls Check at each turn boundary to detect
// breaches.
//
// Implementations must be safe for concurrent use. A single Guard instance
// may be shared across nested orchestrators.
//
// Stability: frozen-v1.0.
type Guard interface {
	// RecordTokens adds the given token counts to the running totals.
	RecordTokens(ctx context.Context, inputTokens, outputTokens int64) error

	// RecordToolCall increments the tool call counter by one.
	RecordToolCall(ctx context.Context) error

	// RecordCost adds the given cost in micro-dollars to the running total.
	RecordCost(ctx context.Context, microdollars int64) error

	// Check evaluates all budget dimensions and returns the current snapshot.
	// A non-nil error (typically *errors.BudgetExceededError) indicates a
	// breach. The snapshot is populated even on error.
	Check(ctx context.Context) (BudgetSnapshot, error)

	// Snapshot returns the current resource consumption without checking limits.
	Snapshot(ctx context.Context) BudgetSnapshot
}

// PriceProvider supplies per-model pricing so the orchestrator can compute
// cost estimates.
//
// Implementations must be safe for concurrent use.
//
// Stability: frozen-v1.0.
type PriceProvider interface {
	// PriceForToken returns the price in micro-dollars per token for the
	// given provider, model, and direction. Returns 0 if the model is unknown.
	PriceForToken(ctx context.Context, provider, model string, direction TokenDirection) (int64, error)
}
