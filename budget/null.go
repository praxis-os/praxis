// SPDX-License-Identifier: Apache-2.0

package budget

import "context"

// Compile-time interface checks.
var _ Guard = NullGuard{}
var _ PriceProvider = NullPriceProvider{}

// NullGuard is a [Guard] that never rejects any usage. Used as the default
// when no budget is configured.
type NullGuard struct{}

// RecordTokens is a no-op.
func (NullGuard) RecordTokens(_ context.Context, _, _ int64) error { return nil }

// RecordToolCall is a no-op.
func (NullGuard) RecordToolCall(_ context.Context) error { return nil }

// RecordCost is a no-op.
func (NullGuard) RecordCost(_ context.Context, _ int64) error { return nil }

// Check always returns a zero snapshot and nil error.
func (NullGuard) Check(_ context.Context) (BudgetSnapshot, error) {
	return BudgetSnapshot{}, nil
}

// Snapshot always returns a zero snapshot.
func (NullGuard) Snapshot(_ context.Context) BudgetSnapshot {
	return BudgetSnapshot{}
}

// NullPriceProvider is a [PriceProvider] that returns zero for all models.
// Used as the default when no pricing data is configured.
type NullPriceProvider struct{}

// PriceForToken returns 0 for all models.
func (NullPriceProvider) PriceForToken(_ context.Context, _, _ string, _ TokenDirection) (int64, error) {
	return 0, nil
}
