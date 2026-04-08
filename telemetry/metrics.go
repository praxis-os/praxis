// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"context"
	"time"
)

// MetricsRecorder records Prometheus-style metrics for invocations.
//
// Implementations are expected to be safe for concurrent use.
//
// v0.3.0 ships a noop implementation; full Prometheus metrics come in v0.5.0.
//
// Stability: stable-v0.3.
type MetricsRecorder interface {
	// RecordInvocationDuration records the wall-clock duration of a completed
	// invocation. provider and model identify the LLM backend; terminalState
	// is the string representation of the terminal [state.State].
	RecordInvocationDuration(ctx context.Context, provider, model, terminalState string, duration time.Duration)

	// RecordTokensUsed records token consumption for an LLM call.
	// direction is "input" or "output".
	RecordTokensUsed(ctx context.Context, provider, model, direction string, count int64)

	// RecordToolCallDuration records the wall-clock duration of a tool call.
	// status is "ok" or "error".
	RecordToolCallDuration(ctx context.Context, toolName, status string, duration time.Duration)
}

// Compile-time interface check.
var _ MetricsRecorder = NoopMetricsRecorder{}

// NoopMetricsRecorder is a [MetricsRecorder] whose methods are all no-ops.
// Used as the default when no metrics backend is configured.
type NoopMetricsRecorder struct{}

// RecordInvocationDuration is a no-op.
func (NoopMetricsRecorder) RecordInvocationDuration(_ context.Context, _, _, _ string, _ time.Duration) {
}

// RecordTokensUsed is a no-op.
func (NoopMetricsRecorder) RecordTokensUsed(_ context.Context, _, _, _ string, _ int64) {}

// RecordToolCallDuration is a no-op.
func (NoopMetricsRecorder) RecordToolCallDuration(_ context.Context, _, _ string, _ time.Duration) {}
