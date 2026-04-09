// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// prometheusRecorder implements [MetricsRecorder] backed by 10 Prometheus
// metrics with bounded cardinality.
//
// All metric names use the "praxis_" prefix. Label sets are closed and
// framework-defined; [AttributeEnricher] attributes never appear on metric
// labels (D57, D60).
type prometheusRecorder struct {
	invocationsTotal   *prometheus.CounterVec
	invocationDuration *prometheus.HistogramVec
	llmCallsTotal      *prometheus.CounterVec
	llmCallDuration    *prometheus.HistogramVec
	llmTokensTotal     *prometheus.CounterVec
	toolCallsTotal     *prometheus.CounterVec
	toolCallDuration   *prometheus.HistogramVec
	budgetExceeded     *prometheus.CounterVec
	errorsTotal        *prometheus.CounterVec
}

// Compile-time interface check.
var _ MetricsRecorder = (*prometheusRecorder)(nil)

// Histogram bucket boundaries per Phase 4 §3.

var invocationBuckets = []float64{
	0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0, 60.0, 120.0, 300.0,
}

var llmCallBuckets = []float64{
	0.1, 0.25, 0.5, 1.0, 2.0, 5.0, 10.0, 20.0, 30.0, 60.0,
}

var toolCallBuckets = []float64{
	0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0, 5.0, 10.0, 30.0,
}

// NewPrometheusRecorder registers all 10 praxis metrics with the given
// Prometheus registerer and returns a [MetricsRecorder] backed by them.
//
// Callers who use the default Prometheus global registry:
//
//	recorder := telemetry.NewPrometheusRecorder(prometheus.DefaultRegisterer)
//
// Callers with isolated registries:
//
//	reg := prometheus.NewRegistry()
//	recorder := telemetry.NewPrometheusRecorder(reg)
//
// The returned recorder is safe for concurrent use.
//
// NewPrometheusRecorder panics if reg is nil.
func NewPrometheusRecorder(reg prometheus.Registerer) MetricsRecorder {
	if reg == nil {
		panic("telemetry: NewPrometheusRecorder: registerer must not be nil")
	}

	r := &prometheusRecorder{
		invocationsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "praxis_invocations_total",
			Help: "Total number of completed invocations.",
		}, []string{"provider", "model", "terminal_state"}),

		invocationDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "praxis_invocation_duration_seconds",
			Help:    "Wall-clock duration of invocations in seconds.",
			Buckets: invocationBuckets,
		}, []string{"provider", "model", "terminal_state"}),

		llmCallsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "praxis_llm_calls_total",
			Help: "Total number of LLM provider calls.",
		}, []string{"provider", "model", "status"}),

		llmCallDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "praxis_llm_call_duration_seconds",
			Help:    "Wall-clock duration of individual LLM calls in seconds.",
			Buckets: llmCallBuckets,
		}, []string{"provider", "model", "status"}),

		llmTokensTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "praxis_llm_tokens_total",
			Help: "Total LLM tokens consumed.",
		}, []string{"provider", "model", "direction"}),

		toolCallsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "praxis_tool_calls_total",
			Help: "Total number of tool invocations.",
		}, []string{"tool_name", "status"}),

		toolCallDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "praxis_tool_call_duration_seconds",
			Help:    "Wall-clock duration of individual tool calls in seconds.",
			Buckets: toolCallBuckets,
		}, []string{"tool_name", "status"}),

		budgetExceeded: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "praxis_budget_exceeded_total",
			Help: "Total number of budget breaches by dimension.",
		}, []string{"dimension"}),

		errorsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "praxis_errors_total",
			Help: "Total errors by kind.",
		}, []string{"error_kind"}),
	}

	// Register all collectors. MustRegister panics on duplicate registration,
	// which is the correct behavior: double-registering metrics is a
	// programming error.
	reg.MustRegister(
		r.invocationsTotal,
		r.invocationDuration,
		r.llmCallsTotal,
		r.llmCallDuration,
		r.llmTokensTotal,
		r.toolCallsTotal,
		r.toolCallDuration,
		r.budgetExceeded,
		r.errorsTotal,
	)

	return r
}

func (r *prometheusRecorder) RecordInvocation(provider, model, terminalState string, duration time.Duration) {
	r.invocationsTotal.WithLabelValues(provider, model, terminalState).Inc()
	r.invocationDuration.WithLabelValues(provider, model, terminalState).Observe(duration.Seconds())
}

func (r *prometheusRecorder) RecordLLMCall(provider, model, status string, duration time.Duration) {
	r.llmCallsTotal.WithLabelValues(provider, model, status).Inc()
	r.llmCallDuration.WithLabelValues(provider, model, status).Observe(duration.Seconds())
}

func (r *prometheusRecorder) RecordLLMTokens(provider, model, direction string, count int64) {
	r.llmTokensTotal.WithLabelValues(provider, model, direction).Add(float64(count))
}

func (r *prometheusRecorder) RecordToolCall(toolName, status string, duration time.Duration) {
	r.toolCallsTotal.WithLabelValues(toolName, status).Inc()
	r.toolCallDuration.WithLabelValues(toolName, status).Observe(duration.Seconds())
}

func (r *prometheusRecorder) RecordBudgetExceeded(dimension string) {
	r.budgetExceeded.WithLabelValues(dimension).Inc()
}

func (r *prometheusRecorder) RecordError(errorKind string) {
	r.errorsTotal.WithLabelValues(errorKind).Inc()
}
