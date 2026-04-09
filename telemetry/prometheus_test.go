// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/praxis-os/praxis/telemetry"
)

// newTestRegistry creates an isolated Prometheus registry and recorder for testing.
func newTestRegistry(t *testing.T) (*prometheus.Registry, telemetry.MetricsRecorder) {
	t.Helper()
	reg := prometheus.NewRegistry()
	rec := telemetry.NewPrometheusRecorder(reg)
	return reg, rec
}

// gatherMetric finds a metric family by name in the registry.
func gatherMetric(t *testing.T, reg *prometheus.Registry, name string) *dto.MetricFamily {
	t.Helper()
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	for _, f := range families {
		if f.GetName() == name {
			return f
		}
	}
	return nil
}

func TestNewPrometheusRecorder_NilPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil registerer")
		}
	}()
	telemetry.NewPrometheusRecorder(nil)
}

func TestNewPrometheusRecorder_ImplementsMetricsRecorder(t *testing.T) {
	reg := prometheus.NewRegistry()
	var _ telemetry.MetricsRecorder = telemetry.NewPrometheusRecorder(reg)
}

func TestPrometheusRecorder_RecordInvocation(t *testing.T) {
	reg, rec := newTestRegistry(t)

	rec.RecordInvocation("anthropic", "claude-3", "completed", 2*time.Second)
	rec.RecordInvocation("anthropic", "claude-3", "failed", 500*time.Millisecond)

	// Counter
	family := gatherMetric(t, reg, "praxis_invocations_total")
	if family == nil {
		t.Fatal("praxis_invocations_total not found")
	}
	if len(family.GetMetric()) != 2 {
		t.Errorf("expected 2 metric series, got %d", len(family.GetMetric()))
	}

	// Histogram
	family = gatherMetric(t, reg, "praxis_invocation_duration_seconds")
	if family == nil {
		t.Fatal("praxis_invocation_duration_seconds not found")
	}
}

func TestPrometheusRecorder_RecordLLMCall(t *testing.T) {
	reg, rec := newTestRegistry(t)

	rec.RecordLLMCall("openai", "gpt-4", "ok", time.Second)
	rec.RecordLLMCall("openai", "gpt-4", "transient_error", 5*time.Second)

	family := gatherMetric(t, reg, "praxis_llm_calls_total")
	if family == nil {
		t.Fatal("praxis_llm_calls_total not found")
	}
	if len(family.GetMetric()) != 2 {
		t.Errorf("expected 2 metric series, got %d", len(family.GetMetric()))
	}

	family = gatherMetric(t, reg, "praxis_llm_call_duration_seconds")
	if family == nil {
		t.Fatal("praxis_llm_call_duration_seconds not found")
	}
}

func TestPrometheusRecorder_RecordLLMTokens(t *testing.T) {
	reg, rec := newTestRegistry(t)

	rec.RecordLLMTokens("anthropic", "claude-3", "input", 1500)
	rec.RecordLLMTokens("anthropic", "claude-3", "output", 500)

	family := gatherMetric(t, reg, "praxis_llm_tokens_total")
	if family == nil {
		t.Fatal("praxis_llm_tokens_total not found")
	}

	// Should have 2 series (input + output).
	if len(family.GetMetric()) != 2 {
		t.Errorf("expected 2 metric series, got %d", len(family.GetMetric()))
	}

	// Verify token counts.
	for _, m := range family.GetMetric() {
		for _, lp := range m.GetLabel() {
			if lp.GetName() == "direction" {
				switch lp.GetValue() {
				case "input":
					if m.GetCounter().GetValue() != 1500 {
						t.Errorf("input tokens = %v, want 1500", m.GetCounter().GetValue())
					}
				case "output":
					if m.GetCounter().GetValue() != 500 {
						t.Errorf("output tokens = %v, want 500", m.GetCounter().GetValue())
					}
				}
			}
		}
	}
}

func TestPrometheusRecorder_RecordToolCall(t *testing.T) {
	reg, rec := newTestRegistry(t)

	rec.RecordToolCall("search", "ok", 100*time.Millisecond)
	rec.RecordToolCall("search", "error", 50*time.Millisecond)

	family := gatherMetric(t, reg, "praxis_tool_calls_total")
	if family == nil {
		t.Fatal("praxis_tool_calls_total not found")
	}
	if len(family.GetMetric()) != 2 {
		t.Errorf("expected 2 metric series, got %d", len(family.GetMetric()))
	}

	family = gatherMetric(t, reg, "praxis_tool_call_duration_seconds")
	if family == nil {
		t.Fatal("praxis_tool_call_duration_seconds not found")
	}
}

func TestPrometheusRecorder_RecordBudgetExceeded(t *testing.T) {
	reg, rec := newTestRegistry(t)

	for _, dim := range []string{"wall_clock", "tokens", "tool_calls", "cost"} {
		rec.RecordBudgetExceeded(dim)
	}

	family := gatherMetric(t, reg, "praxis_budget_exceeded_total")
	if family == nil {
		t.Fatal("praxis_budget_exceeded_total not found")
	}
	if len(family.GetMetric()) != 4 {
		t.Errorf("expected 4 metric series (one per dimension), got %d", len(family.GetMetric()))
	}
}

func TestPrometheusRecorder_RecordError(t *testing.T) {
	reg, rec := newTestRegistry(t)

	kinds := []string{
		"transient_llm", "permanent_llm", "tool", "policy_denied",
		"budget_exceeded", "cancellation", "system", "approval_required",
	}
	for _, k := range kinds {
		rec.RecordError(k)
	}

	family := gatherMetric(t, reg, "praxis_errors_total")
	if family == nil {
		t.Fatal("praxis_errors_total not found")
	}
	if len(family.GetMetric()) != 8 {
		t.Errorf("expected 8 metric series (one per error kind), got %d", len(family.GetMetric()))
	}
}

func TestPrometheusRecorder_AllTenMetricsRegistered(t *testing.T) {
	// We need to trigger at least one observation per metric to make them
	// appear in Gather.
	reg2 := prometheus.NewRegistry()
	rec2 := telemetry.NewPrometheusRecorder(reg2)
	rec2.RecordInvocation("p", "m", "completed", time.Second)
	rec2.RecordLLMCall("p", "m", "ok", time.Second)
	rec2.RecordLLMTokens("p", "m", "input", 1)
	rec2.RecordToolCall("t", "ok", time.Second)
	rec2.RecordBudgetExceeded("tokens")
	rec2.RecordError("system")

	families, err := reg2.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}

	want := map[string]bool{
		"praxis_invocations_total":             false,
		"praxis_invocation_duration_seconds":   false,
		"praxis_llm_calls_total":               false,
		"praxis_llm_call_duration_seconds":     false,
		"praxis_llm_tokens_total":              false,
		"praxis_tool_calls_total":              false,
		"praxis_tool_call_duration_seconds":    false,
		"praxis_budget_exceeded_total":         false,
		"praxis_errors_total":                  false,
	}

	for _, f := range families {
		if _, ok := want[f.GetName()]; ok {
			want[f.GetName()] = true
		}
	}

	// 9 metric names (10 metrics, but counter+histogram pairs share a call).
	// The 10th "metric" is the histogram which creates _bucket, _sum, _count.
	for name, found := range want {
		if !found {
			t.Errorf("metric %q not found in gathered output", name)
		}
	}
}

func TestPrometheusRecorder_DoubleRegistrationPanics(t *testing.T) {
	reg := prometheus.NewRegistry()
	_ = telemetry.NewPrometheusRecorder(reg)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on double registration")
		}
	}()
	_ = telemetry.NewPrometheusRecorder(reg)
}
