// SPDX-License-Identifier: Apache-2.0

package budget

import "time"

// BudgetDimension identifies which budget limit was breached.
type BudgetDimension string

const (
	// BudgetDimensionWallClock indicates the wall-clock duration limit was breached.
	BudgetDimensionWallClock BudgetDimension = "wall_clock"

	// BudgetDimensionTokens indicates the token count limit was breached.
	BudgetDimensionTokens BudgetDimension = "tokens"

	// BudgetDimensionToolCalls indicates the tool call count limit was breached.
	BudgetDimensionToolCalls BudgetDimension = "tool_calls"

	// BudgetDimensionCost indicates the cost limit was breached.
	BudgetDimensionCost BudgetDimension = "cost"
)

// BudgetSnapshot is a point-in-time view of resource consumption for an invocation.
//
// The zero value is valid and represents no consumption. All fields are safe
// to read without initialization.
type BudgetSnapshot struct {
	// ElapsedWallClock is the wall-clock duration since the invocation started.
	ElapsedWallClock time.Duration

	// InputTokensUsed is the cumulative number of input tokens consumed.
	InputTokensUsed int64

	// OutputTokensUsed is the cumulative number of output tokens generated.
	OutputTokensUsed int64

	// ToolCallsUsed is the cumulative number of tool calls dispatched.
	ToolCallsUsed int64

	// CostMicrodollars is the estimated cost in micro-dollars (1 USD = 1,000,000).
	CostMicrodollars int64

	// ExceededDimension identifies which dimension was breached, if any.
	// Zero value ("") indicates no breach.
	ExceededDimension BudgetDimension
}

// Config holds the budget limits for an invocation.
//
// A zero value for any field means "no limit" for that dimension.
type Config struct {
	// MaxWallClock is the maximum wall-clock duration in nanoseconds.
	MaxWallClock int64

	// MaxInputTokens is the maximum number of input tokens allowed.
	MaxInputTokens int64

	// MaxOutputTokens is the maximum number of output tokens allowed.
	MaxOutputTokens int64

	// MaxToolCalls is the maximum number of tool calls allowed.
	MaxToolCalls int64

	// MaxCostMicrodollars is the maximum cost in micro-dollars allowed.
	MaxCostMicrodollars int64
}

// TokenDirection distinguishes input from output tokens for pricing.
type TokenDirection string

const (
	// TokenDirectionInput represents input (prompt) tokens.
	TokenDirectionInput TokenDirection = "input"

	// TokenDirectionOutput represents output (completion) tokens.
	TokenDirectionOutput TokenDirection = "output"
)

// PriceKey identifies a specific pricing rate for a provider/model/direction
// combination.
type PriceKey struct {
	Provider  string
	Model     string
	Direction TokenDirection
}
