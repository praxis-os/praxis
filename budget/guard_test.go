// SPDX-License-Identifier: Apache-2.0

package budget_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/praxis-os/praxis/budget"
)

func TestBudgetGuard_NoLimits(t *testing.T) {
	g := budget.NewBudgetGuard(budget.Config{})
	g.Start(time.Now())

	_ = g.RecordTokens(context.Background(), 1_000_000, 500_000)
	_ = g.RecordToolCall(context.Background())
	_ = g.RecordCost(context.Background(), 99_000_000)

	snap, err := g.Check(context.Background())
	if err != nil {
		t.Fatalf("Check() unexpected error: %v", err)
	}
	if snap.InputTokensUsed != 1_000_000 {
		t.Errorf("InputTokensUsed = %d, want 1000000", snap.InputTokensUsed)
	}
	if snap.ExceededDimension != "" {
		t.Errorf("ExceededDimension = %q, want empty", snap.ExceededDimension)
	}
}

func TestBudgetGuard_TokenBreach(t *testing.T) {
	g := budget.NewBudgetGuard(budget.Config{MaxInputTokens: 100})
	g.Start(time.Now())

	_ = g.RecordTokens(context.Background(), 150, 0)

	snap, err := g.Check(context.Background())
	if err == nil {
		t.Fatal("expected budget breach error")
	}
	if snap.ExceededDimension != budget.BudgetDimensionTokens {
		t.Errorf("ExceededDimension = %q, want %q", snap.ExceededDimension, budget.BudgetDimensionTokens)
	}

	var breach *budget.BudgetBreachError
	if ok := err.(*budget.BudgetBreachError); ok == nil {
		t.Fatalf("expected *BudgetBreachError, got %T", err)
	} else {
		breach = ok
	}
	if breach.Dimension != budget.BudgetDimensionTokens {
		t.Errorf("Dimension = %q, want %q", breach.Dimension, budget.BudgetDimensionTokens)
	}
}

func TestBudgetGuard_ToolCallBreach(t *testing.T) {
	g := budget.NewBudgetGuard(budget.Config{MaxToolCalls: 2})
	g.Start(time.Now())

	_ = g.RecordToolCall(context.Background())
	_ = g.RecordToolCall(context.Background())
	_ = g.RecordToolCall(context.Background())

	snap, err := g.Check(context.Background())
	if err == nil {
		t.Fatal("expected budget breach error")
	}
	if snap.ExceededDimension != budget.BudgetDimensionToolCalls {
		t.Errorf("ExceededDimension = %q, want %q", snap.ExceededDimension, budget.BudgetDimensionToolCalls)
	}
}

func TestBudgetGuard_CostBreach(t *testing.T) {
	g := budget.NewBudgetGuard(budget.Config{MaxCostMicrodollars: 1000})
	g.Start(time.Now())

	_ = g.RecordCost(context.Background(), 1500)

	_, err := g.Check(context.Background())
	if err == nil {
		t.Fatal("expected budget breach error")
	}
}

func TestBudgetGuard_WallClockBreach(t *testing.T) {
	g := budget.NewBudgetGuard(budget.Config{MaxWallClock: int64(10 * time.Millisecond)})
	g.Start(time.Now().Add(-20 * time.Millisecond)) // Started 20ms ago.

	snap, err := g.Check(context.Background())
	if err == nil {
		t.Fatal("expected wall clock breach")
	}
	if snap.ExceededDimension != budget.BudgetDimensionWallClock {
		t.Errorf("ExceededDimension = %q, want %q", snap.ExceededDimension, budget.BudgetDimensionWallClock)
	}
}

func TestBudgetGuard_FirstBreachWins(t *testing.T) {
	// Both tokens and tool calls are over limit. WallClock > Tokens > ToolCalls.
	g := budget.NewBudgetGuard(budget.Config{
		MaxInputTokens: 10,
		MaxToolCalls:   1,
	})
	g.Start(time.Now())

	_ = g.RecordTokens(context.Background(), 100, 0)
	_ = g.RecordToolCall(context.Background())
	_ = g.RecordToolCall(context.Background())

	snap, err := g.Check(context.Background())
	if err == nil {
		t.Fatal("expected breach")
	}
	// Tokens is checked before ToolCalls.
	if snap.ExceededDimension != budget.BudgetDimensionTokens {
		t.Errorf("ExceededDimension = %q, want tokens (first breach)", snap.ExceededDimension)
	}
}

func TestBudgetGuard_Snapshot(t *testing.T) {
	g := budget.NewBudgetGuard(budget.Config{})
	g.Start(time.Now())

	_ = g.RecordTokens(context.Background(), 100, 50)
	_ = g.RecordToolCall(context.Background())
	_ = g.RecordCost(context.Background(), 500)

	snap := g.Snapshot(context.Background())
	if snap.InputTokensUsed != 100 {
		t.Errorf("InputTokensUsed = %d, want 100", snap.InputTokensUsed)
	}
	if snap.OutputTokensUsed != 50 {
		t.Errorf("OutputTokensUsed = %d, want 50", snap.OutputTokensUsed)
	}
	if snap.ToolCallsUsed != 1 {
		t.Errorf("ToolCallsUsed = %d, want 1", snap.ToolCallsUsed)
	}
	if snap.CostMicrodollars != 500 {
		t.Errorf("CostMicrodollars = %d, want 500", snap.CostMicrodollars)
	}
}

func TestBudgetGuard_ConcurrentRecordTokens(t *testing.T) {
	g := budget.NewBudgetGuard(budget.Config{})
	g.Start(time.Now())

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = g.RecordTokens(context.Background(), 10, 5)
		}()
	}
	wg.Wait()

	snap := g.Snapshot(context.Background())
	if snap.InputTokensUsed != 1000 {
		t.Errorf("InputTokensUsed = %d, want 1000", snap.InputTokensUsed)
	}
	if snap.OutputTokensUsed != 500 {
		t.Errorf("OutputTokensUsed = %d, want 500", snap.OutputTokensUsed)
	}
}

func TestStaticPriceProvider(t *testing.T) {
	table := map[budget.PriceKey]int64{
		{Provider: "anthropic", Model: "claude-3", Direction: budget.TokenDirectionInput}:  3,
		{Provider: "anthropic", Model: "claude-3", Direction: budget.TokenDirectionOutput}: 15,
	}
	p := budget.NewStaticPriceProvider(table)

	price, err := p.PriceForToken(context.Background(), "anthropic", "claude-3", budget.TokenDirectionInput)
	if err != nil {
		t.Fatalf("PriceForToken: %v", err)
	}
	if price != 3 {
		t.Errorf("input price = %d, want 3", price)
	}

	price, err = p.PriceForToken(context.Background(), "anthropic", "claude-3", budget.TokenDirectionOutput)
	if err != nil {
		t.Fatalf("PriceForToken: %v", err)
	}
	if price != 15 {
		t.Errorf("output price = %d, want 15", price)
	}

	// Unknown key returns 0.
	price, err = p.PriceForToken(context.Background(), "openai", "gpt-4", budget.TokenDirectionInput)
	if err != nil {
		t.Fatalf("PriceForToken: %v", err)
	}
	if price != 0 {
		t.Errorf("unknown key price = %d, want 0", price)
	}
}
