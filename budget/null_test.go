// SPDX-License-Identifier: Apache-2.0

package budget_test

import (
	"context"
	"testing"

	"github.com/praxis-os/praxis/budget"
)

func TestInterfaces(_ *testing.T) {
	var _ budget.Guard = budget.NullGuard{}
	var _ budget.PriceProvider = budget.NullPriceProvider{}
}

func TestNullGuard_RecordTokens(t *testing.T) {
	g := budget.NullGuard{}
	if err := g.RecordTokens(context.Background(), 1000, 500); err != nil {
		t.Errorf("RecordTokens() unexpected error: %v", err)
	}
}

func TestNullGuard_RecordToolCall(t *testing.T) {
	g := budget.NullGuard{}
	if err := g.RecordToolCall(context.Background()); err != nil {
		t.Errorf("RecordToolCall() unexpected error: %v", err)
	}
}

func TestNullGuard_RecordCost(t *testing.T) {
	g := budget.NullGuard{}
	if err := g.RecordCost(context.Background(), 99_000_000); err != nil {
		t.Errorf("RecordCost() unexpected error: %v", err)
	}
}

func TestNullGuard_Check(t *testing.T) {
	g := budget.NullGuard{}
	snap, err := g.Check(context.Background())
	if err != nil {
		t.Errorf("Check() unexpected error: %v", err)
	}
	if snap.InputTokensUsed != 0 {
		t.Errorf("Check() InputTokensUsed = %d, want 0", snap.InputTokensUsed)
	}
}

func TestNullGuard_Snapshot(t *testing.T) {
	g := budget.NullGuard{}
	snap := g.Snapshot(context.Background())
	if snap.InputTokensUsed != 0 {
		t.Errorf("Snapshot() InputTokensUsed = %d, want 0", snap.InputTokensUsed)
	}
}

func TestNullPriceProvider_PriceForToken(t *testing.T) {
	models := []string{"", "gpt-4o", "claude-3-opus-20240229", "unknown-model"}
	p := budget.NullPriceProvider{}

	for _, model := range models {
		t.Run(model, func(t *testing.T) {
			price, err := p.PriceForToken(context.Background(), "provider", model, budget.TokenDirectionInput)
			if err != nil {
				t.Errorf("PriceForToken() unexpected error: %v", err)
			}
			if price != 0 {
				t.Errorf("PriceForToken(%q, input) = %d, want 0", model, price)
			}

			price, err = p.PriceForToken(context.Background(), "provider", model, budget.TokenDirectionOutput)
			if err != nil {
				t.Errorf("PriceForToken() unexpected error: %v", err)
			}
			if price != 0 {
				t.Errorf("PriceForToken(%q, output) = %d, want 0", model, price)
			}
		})
	}
}
