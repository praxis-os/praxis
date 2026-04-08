// SPDX-License-Identifier: Apache-2.0

package budget

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

// Compile-time interface check.
var _ Guard = (*BudgetGuard)(nil)

// BudgetGuard is a concrete [Guard] that enforces multi-dimensional budget
// limits using atomic counters for lock-free concurrent access.
//
// Create with [NewBudgetGuard]. A zero-valued [Config] field means "no limit"
// for that dimension.
type BudgetGuard struct {
	cfg Config

	inputTokens  atomic.Int64
	outputTokens atomic.Int64
	toolCalls    atomic.Int64
	costMicros   atomic.Int64

	wallClockStart time.Time
}

// NewBudgetGuard creates a BudgetGuard with the given limits.
// A zero value for any Config field means no limit for that dimension.
func NewBudgetGuard(cfg Config) *BudgetGuard {
	return &BudgetGuard{cfg: cfg}
}

// Start sets the wall-clock start time. Called by the orchestrator at
// Initializing state entry. Not part of the [Guard] interface.
func (g *BudgetGuard) Start(t time.Time) {
	g.wallClockStart = t
}

// RecordTokens adds the given token counts to the running totals.
func (g *BudgetGuard) RecordTokens(_ context.Context, inputTokens, outputTokens int64) error {
	g.inputTokens.Add(inputTokens)
	g.outputTokens.Add(outputTokens)
	return nil
}

// RecordToolCall increments the tool call counter by one.
func (g *BudgetGuard) RecordToolCall(_ context.Context) error {
	g.toolCalls.Add(1)
	return nil
}

// RecordCost adds the given cost in micro-dollars to the running total.
func (g *BudgetGuard) RecordCost(_ context.Context, microdollars int64) error {
	g.costMicros.Add(microdollars)
	return nil
}

// BudgetBreachError is returned by Check when a budget dimension is exceeded.
// The orchestrator maps this to errors.BudgetExceededError.
type BudgetBreachError struct {
	Dimension BudgetDimension
	Limit     string
	Actual    string
}

func (e *BudgetBreachError) Error() string {
	return fmt.Sprintf("budget exceeded: %s (limit: %s, actual: %s)", e.Dimension, e.Limit, e.Actual)
}

// Check evaluates all budget dimensions in order: WallClock → Tokens →
// ToolCalls → Cost. The first breach wins (D21).
func (g *BudgetGuard) Check(_ context.Context) (BudgetSnapshot, error) {
	snap := g.snapshot()

	if g.cfg.MaxWallClock > 0 && snap.ElapsedWallClock > time.Duration(g.cfg.MaxWallClock) {
		snap.ExceededDimension = BudgetDimensionWallClock
		return snap, &BudgetBreachError{BudgetDimensionWallClock,
			fmt.Sprintf("%v", time.Duration(g.cfg.MaxWallClock)),
			fmt.Sprintf("%v", snap.ElapsedWallClock)}
	}

	if g.cfg.MaxInputTokens > 0 && snap.InputTokensUsed > g.cfg.MaxInputTokens {
		snap.ExceededDimension = BudgetDimensionTokens
		return snap, &BudgetBreachError{BudgetDimensionTokens,
			fmt.Sprintf("%d", g.cfg.MaxInputTokens),
			fmt.Sprintf("%d", snap.InputTokensUsed)}
	}

	if g.cfg.MaxOutputTokens > 0 && snap.OutputTokensUsed > g.cfg.MaxOutputTokens {
		snap.ExceededDimension = BudgetDimensionTokens
		return snap, &BudgetBreachError{BudgetDimensionTokens,
			fmt.Sprintf("%d", g.cfg.MaxOutputTokens),
			fmt.Sprintf("%d", snap.OutputTokensUsed)}
	}

	if g.cfg.MaxToolCalls > 0 && snap.ToolCallsUsed > g.cfg.MaxToolCalls {
		snap.ExceededDimension = BudgetDimensionToolCalls
		return snap, &BudgetBreachError{BudgetDimensionToolCalls,
			fmt.Sprintf("%d", g.cfg.MaxToolCalls),
			fmt.Sprintf("%d", snap.ToolCallsUsed)}
	}

	if g.cfg.MaxCostMicrodollars > 0 && snap.CostMicrodollars > g.cfg.MaxCostMicrodollars {
		snap.ExceededDimension = BudgetDimensionCost
		return snap, &BudgetBreachError{BudgetDimensionCost,
			fmt.Sprintf("%d", g.cfg.MaxCostMicrodollars),
			fmt.Sprintf("%d", snap.CostMicrodollars)}
	}

	return snap, nil
}

// Snapshot returns the current resource consumption without checking limits.
func (g *BudgetGuard) Snapshot(_ context.Context) BudgetSnapshot {
	return g.snapshot()
}

func (g *BudgetGuard) snapshot() BudgetSnapshot {
	var elapsed time.Duration
	if !g.wallClockStart.IsZero() {
		elapsed = time.Since(g.wallClockStart)
	}
	return BudgetSnapshot{
		ElapsedWallClock: elapsed,
		InputTokensUsed:  g.inputTokens.Load(),
		OutputTokensUsed: g.outputTokens.Load(),
		ToolCallsUsed:    g.toolCalls.Load(),
		CostMicrodollars: g.costMicros.Load(),
	}
}
