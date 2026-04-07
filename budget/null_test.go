// SPDX-License-Identifier: Apache-2.0

package budget_test

import (
	"context"
	"testing"
	"time"

	"github.com/praxis-os/praxis/budget"
)

func TestInterfaces(t *testing.T) {
	// Compile-time checks documented as runtime assertions.
	var _ budget.Guard = budget.NullGuard{}
	var _ budget.PriceProvider = budget.NullPriceProvider{}
}

func TestNullGuard_Check(t *testing.T) {
	tests := []struct {
		name  string
		usage budget.Usage
	}{
		{
			name:  "zero usage",
			usage: budget.Usage{},
		},
		{
			name: "high token usage",
			usage: budget.Usage{
				InputTokens:  1_000_000,
				OutputTokens: 500_000,
				ToolCalls:    1000,
				CostMicros:   99_000_000,
				Elapsed:      24 * time.Hour,
			},
		},
	}

	g := budget.NullGuard{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := g.Check(context.Background(), tt.usage); err != nil {
				t.Errorf("Check() unexpected error: %v", err)
			}
		})
	}
}

func TestNullPriceProvider(t *testing.T) {
	models := []string{"", "gpt-4o", "claude-3-opus-20240229", "unknown-model"}
	p := budget.NullPriceProvider{}

	for _, model := range models {
		t.Run(model, func(t *testing.T) {
			if got := p.InputPricePer1K(model); got != 0 {
				t.Errorf("InputPricePer1K(%q) = %v, want 0", model, got)
			}
			if got := p.OutputPricePer1K(model); got != 0 {
				t.Errorf("OutputPricePer1K(%q) = %v, want 0", model, got)
			}
		})
	}
}
