// SPDX-License-Identifier: Apache-2.0

package telemetry

import "time"

// MetricsRecorder records metric observations for invocations.
//
// The orchestrator calls the appropriate method at each metric-relevant
// state transition. Implementations must be safe for concurrent use.
//
// MetricsRecorder methods accept only framework-defined, bounded-cardinality
// values as parameters. [AttributeEnricher] attributes are intentionally
// excluded from all method signatures to prevent cardinality explosion
// in Prometheus (D57, D60).
//
// Stability: frozen-v1.0.
type MetricsRecorder interface {
	// RecordInvocation records a completed invocation.
	// Called once at terminal state entry.
	// terminalState is one of: "completed", "failed", "cancelled",
	// "budget_exceeded", "approval_required".
	RecordInvocation(provider, model, terminalState string, duration time.Duration)

	// RecordLLMCall records a single LLM call.
	// Called after each LLM provider call returns.
	// status is "ok", "transient_error", or "permanent_error".
	RecordLLMCall(provider, model, status string, duration time.Duration)

	// RecordLLMTokens records token consumption from a single LLM call.
	// direction is "input" or "output".
	RecordLLMTokens(provider, model, direction string, count int64)

	// RecordToolCall records a single tool invocation.
	// status is "ok", "error", or "denied".
	RecordToolCall(toolName, status string, duration time.Duration)

	// RecordBudgetExceeded records a budget breach.
	// dimension is one of: "wall_clock", "tokens", "tool_calls", "cost".
	RecordBudgetExceeded(dimension string)

	// RecordError records an error by kind.
	// errorKind is one of the ErrorKind string values.
	RecordError(errorKind string)
}

// Compile-time interface check.
var _ MetricsRecorder = NoopMetricsRecorder{}

// NoopMetricsRecorder is a [MetricsRecorder] that silently discards all
// observations. Used as the default when no metrics backend is configured.
type NoopMetricsRecorder struct{}

func (NoopMetricsRecorder) RecordInvocation(_, _, _ string, _ time.Duration) {}
func (NoopMetricsRecorder) RecordLLMCall(_, _, _ string, _ time.Duration)    {}
func (NoopMetricsRecorder) RecordLLMTokens(_, _, _ string, _ int64)          {}
func (NoopMetricsRecorder) RecordToolCall(_, _ string, _ time.Duration)      {}
func (NoopMetricsRecorder) RecordBudgetExceeded(_ string)                    {}
func (NoopMetricsRecorder) RecordError(_ string)                             {}
