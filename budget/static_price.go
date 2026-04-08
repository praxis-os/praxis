// SPDX-License-Identifier: Apache-2.0

package budget

import "context"

// Compile-time interface check.
var _ PriceProvider = (*StaticPriceProvider)(nil)

// StaticPriceProvider is a [PriceProvider] backed by a static lookup table.
// Useful for testing and fixed-pricing scenarios.
type StaticPriceProvider struct {
	table map[PriceKey]int64
}

// NewStaticPriceProvider creates a StaticPriceProvider from a price table.
// The table maps (provider, model, direction) tuples to micro-dollar prices
// per token.
func NewStaticPriceProvider(table map[PriceKey]int64) *StaticPriceProvider {
	return &StaticPriceProvider{table: table}
}

// PriceForToken returns the micro-dollar price per token for the given key.
// Returns 0 if the key is not in the table.
func (p *StaticPriceProvider) PriceForToken(_ context.Context, provider, model string, direction TokenDirection) (int64, error) {
	key := PriceKey{Provider: provider, Model: model, Direction: direction}
	return p.table[key], nil
}
