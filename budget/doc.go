// SPDX-License-Identifier: Apache-2.0

// Package budget defines the multi-dimensional budget enforcement interfaces
// and their null implementations.
//
// [Guard] enforces budget limits across input tokens, output tokens, tool
// calls, estimated cost, and wall-clock elapsed time. The orchestrator calls
// [Guard.Check] at each turn boundary; a non-nil error halts the invocation.
//
// [PriceProvider] supplies per-model pricing so the orchestrator can compute
// cost estimates for [Usage.CostMicros].
//
// [NullGuard] never rejects — used when no budget is configured.
// [NullPriceProvider] returns zero prices — used when no pricing data is
// available.
//
// Stability: frozen-v1.0.
package budget
