# Phase 3 — Budget Interfaces

**Stability tiers:** `budget.Guard` — `frozen-v1.0`; `budget.PriceProvider` — promoted to `frozen-v1.0` (D47)
**Decisions:** D34, D35, D47
**Package:** `MODULE_PATH_TBD/budget`
**Composition property:** CP3 (shared Guard across nested orchestrators)

---

## Overview

The `budget` package enforces four-dimensional budget limits on every
invocation: wall-clock duration, total tokens (input + output), total tool
call count, and total estimated cost in micro-dollars. Any dimension breach
causes the invocation to transition to `BudgetExceeded`.

The key composition property (CP3): a single `Guard` instance may be shared
across an outer and one or more inner orchestrators to enforce a combined
budget ceiling. The `Guard` interface is designed with this in mind — it
assumes no one-to-one mapping between a Guard instance and an invocation.

---

## `BudgetSnapshot` (D35)

```go
// BudgetDimension identifies a budget enforcement dimension.
type BudgetDimension string

const (
    BudgetDimensionWallClock BudgetDimension = "wall_clock"
    BudgetDimensionTokens    BudgetDimension = "tokens"
    BudgetDimensionToolCalls BudgetDimension = "tool_calls"
    BudgetDimensionCost      BudgetDimension = "cost"
)

// BudgetSnapshot is a point-in-time read of consumed and limit values
// across all four budget dimensions.
//
// BudgetSnapshot is a value type: no pointer fields, safe to copy and
// store in channels and event histories without aliasing concerns.
//
// The zero value is valid and means no consumption has been recorded
// (appropriate when no budget.Guard is configured).
type BudgetSnapshot struct {
    // ElapsedWallClock is the wall-clock time elapsed since the Guard
    // was started for this invocation (D25: starts at Initializing entry).
    ElapsedWallClock time.Duration

    // InputTokensUsed is the cumulative input tokens consumed.
    InputTokensUsed int64

    // OutputTokensUsed is the cumulative output tokens consumed.
    OutputTokensUsed int64

    // ToolCallsUsed is the number of tool calls invoked so far.
    ToolCallsUsed int64

    // CostMicrodollars is the cumulative estimated cost in micro-dollars
    // (millionths of a USD). Computed from PriceProvider snapshot (D26).
    CostMicrodollars int64

    // ExceededDimension is the dimension that caused a budget breach.
    // Empty string when no breach has occurred. Set by Check() when it
    // returns a BudgetExceededError.
    ExceededDimension BudgetDimension
}
```

---

## `Config` struct

```go
// Config holds per-invocation budget limits. Zero value means unlimited
// on all dimensions. Applied on top of any shared Guard's configuration.
type Config struct {
    // MaxWallClock is the maximum allowed wall-clock duration.
    // Zero means no wall-clock limit.
    MaxWallClock time.Duration

    // MaxInputTokens is the maximum allowed input token consumption.
    // Zero means no input-token limit.
    MaxInputTokens int64

    // MaxOutputTokens is the maximum allowed output token consumption.
    // Zero means no output-token limit.
    MaxOutputTokens int64

    // MaxToolCalls is the maximum number of tool calls allowed.
    // Zero means no tool-call limit.
    MaxToolCalls int64

    // MaxCostMicrodollars is the maximum estimated cost allowed, in
    // micro-dollars. Zero means no cost limit.
    MaxCostMicrodollars int64
}
```

---

## `Guard` interface (D34)

```go
// Guard enforces four-dimensional budget limits for an invocation.
//
// The orchestrator calls Record* methods to accumulate consumed resources
// at each state where budget accounting happens (D16). It calls Check to
// detect breaches at budget-gated transitions. Snapshot is called to
// populate InvocationEvent.BudgetSnapshot on non-terminal events.
//
// CP3 — Shared instance across nested orchestrators: Guard implementations
// must be safe for concurrent use from multiple simultaneous invocations.
// The interface makes no assumption that a Guard is bound to one invocation.
// A shared Guard accumulates resources from all invocations using it;
// Check() reports against the total. Callers who share a Guard to enforce
// a combined budget ceiling must be aware that all nested invocations
// draw from the same pool.
//
// Stability: frozen-v1.0.
type Guard interface {
    // RecordTokens records token consumption from a single LLM call.
    // Called after every LLMCall or LLMContinuation turn.
    // inputTokens and outputTokens must be >= 0.
    RecordTokens(ctx context.Context, inputTokens, outputTokens int64) error

    // RecordToolCall records one tool invocation.
    // Called once per tool dispatched at ToolCall state entry.
    RecordToolCall(ctx context.Context) error

    // RecordCost records an estimated cost in micro-dollars.
    // Called after each LLM call using the PriceProvider snapshot.
    // microdollars must be >= 0.
    RecordCost(ctx context.Context, microdollars int64) error

    // Check returns the current BudgetSnapshot and a non-nil
    // *errors.BudgetExceededError if any configured limit is breached.
    // When no limit is breached, the returned error is nil.
    //
    // The BudgetExceededError carries the snapshot with ExceededDimension
    // set to the first dimension found in breach (checked in order:
    // WallClock, Tokens, ToolCalls, Cost).
    //
    // Check is called at budget-gated transitions (D16): at ToolDecision
    // entry (after LLM response received), and at LLMContinuation entry
    // (after tool results collected). The LLMCall → BudgetExceeded edge
    // in D16 fires when Check detects a breach after the LLM response is
    // received and tokens have been recorded via RecordTokens — i.e., at
    // the LLMCall → ToolDecision boundary, not before the LLM call.
    // Token-dimension overshoot (C3) is a consequence of this timing:
    // RecordTokens is called after the provider returns, so the breach is
    // detected post-call, not pre-call.
    Check(ctx context.Context) (BudgetSnapshot, error)

    // Snapshot returns the current BudgetSnapshot without performing a
    // breach check. Used to populate InvocationEvent.BudgetSnapshot.
    // Returns the zero BudgetSnapshot if no recording has occurred.
    Snapshot(ctx context.Context) BudgetSnapshot
}
```

---

## `PriceProvider` interface (D47, promoted to `frozen-v1.0`)

```go
// PriceProvider maps (provider, model, direction) to a per-token price
// in micro-dollars (millionths of a USD).
//
// PriceProvider is consulted once per invocation start (at Initializing
// entry, D26) for each (provider, model, direction) tuple required by
// the invocation's configured model. The result is cached in the
// orchestrator's per-invocation state; PriceForToken is not called
// again during the invocation. Mid-process price updates affect new
// invocations only (D08).
//
// There is no bundled price table for any specific LLM provider.
// Callers own the commercial relationship with the LLM vendor and
// therefore own the pricing table. The framework ships
// StaticPriceProvider and NullPriceProvider as convenience
// implementations.
//
// PriceProvider implementations must be safe for concurrent use.
//
// Stability: frozen-v1.0. (Promoted from stable-v0.x-candidate at
// Phase 3 close per D47.)
type PriceProvider interface {
    // PriceForToken returns the price per token in micro-dollars for the
    // given provider name, model identifier, and token direction.
    //
    // Returns 0, nil if the price is unknown (equivalent to NullPriceProvider).
    // Returns a non-nil error only for lookup failures (I/O error, config
    // parse failure) — not for unknown models.
    //
    // The ctx parameter is included because implementations may perform
    // I/O (remote price API, config file read).
    PriceForToken(ctx context.Context, provider, model string, direction TokenDirection) (int64, error)
}

// TokenDirection distinguishes input from output tokens for pricing.
type TokenDirection string

const (
    // TokenDirectionInput is the per-token price for prompt/input tokens.
    TokenDirectionInput TokenDirection = "input"

    // TokenDirectionOutput is the per-token price for completion/output tokens.
    TokenDirectionOutput TokenDirection = "output"
)
```

---

## Default (null) implementations

```go
// NullGuard is the default budget.Guard implementation.
// It records nothing, never signals a breach, and returns a zero
// BudgetSnapshot from every call.
//
// Use NullGuard for zero-wiring construction (D12) and for invocations
// where budget enforcement is the caller's responsibility outside the
// framework.
//
// NullGuard is safe for concurrent use.
//
// Package: MODULE_PATH_TBD/budget
var NullGuard Guard = nullGuard{}

// NullPriceProvider is the default budget.PriceProvider implementation.
// It returns 0 micro-dollars for all token price lookups.
//
// NullPriceProvider is safe for concurrent use.
//
// Package: MODULE_PATH_TBD/budget
var NullPriceProvider PriceProvider = nullPriceProvider{}

// NewStaticPriceProvider constructs a PriceProvider backed by a caller-supplied
// price table. The table maps (provider, model, direction) to micro-dollars
// per token.
//
// Unknown entries return 0, nil (same as NullPriceProvider).
// StaticPriceProvider is safe for concurrent use; the table is read-only
// after construction.
//
// Package: MODULE_PATH_TBD/budget
func NewStaticPriceProvider(table map[PriceKey]int64) PriceProvider

// PriceKey is the lookup key for StaticPriceProvider.
type PriceKey struct {
    Provider  string
    Model     string
    Direction TokenDirection
}

// NewBudgetGuard constructs a Guard that enforces the given Config limits.
// Wall-clock tracking begins when the Guard is started (Start is an internal
// call made by the orchestrator at Initializing entry, D25; it is not part
// of the public Guard interface).
//
// Package: MODULE_PATH_TBD/budget
func NewBudgetGuard(cfg Config) Guard
```

---

## Concurrency contract

All `Guard` and `PriceProvider` implementations must be safe for concurrent
use. The orchestrator may call `RecordTokens`, `RecordToolCall`, `RecordCost`,
`Check`, and `Snapshot` from multiple concurrent invocations' loop goroutines
when a shared instance is injected via `WithBudgetGuard`.

The default `BudgetGuard` implementation uses atomic operations for all
accumulator fields to satisfy this contract without locking.

---

## Composition rules (CP3)

The CP3 composition property requires that sharing a `Guard` instance across
nested orchestrators enforces a combined budget ceiling:

1. Both outer and inner orchestrators call `RecordXxx` and `Check` on the same
   `Guard` instance.
2. The `Guard` accumulates resources from all invocations atomically.
3. `Check` reports against the combined total; a breach in the inner
   invocation is visible to the outer Guard and vice versa.
4. Neither orchestrator resets or initializes the Guard's internal state
   at invocation start — initialization is the caller's responsibility
   before constructing the orchestrators.

The Guard interface's method surface makes no assumption about invocation
lifecycle. There is no `Start(invocationID)` or `Stop()` method. This is a
deliberate design choice for CP3 compatibility.
