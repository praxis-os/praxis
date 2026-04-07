// SPDX-License-Identifier: Apache-2.0

package orchestrator_test

import (
	"context"
	"testing"

	"github.com/praxis-os/praxis/budget"
	"github.com/praxis-os/praxis/credentials"
	"github.com/praxis-os/praxis/errors"
	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/invocation"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/mock"
	"github.com/praxis-os/praxis/orchestrator"
	"github.com/praxis-os/praxis/state"
	"github.com/praxis-os/praxis/telemetry"
	"github.com/praxis-os/praxis/tools"
)

// --- stub implementations used only in tests ---

type stubToolInvoker struct{}

func (stubToolInvoker) Invoke(_ context.Context, call llm.LLMToolCall) (llm.LLMToolResult, error) {
	return llm.LLMToolResult{CallID: call.CallID, Content: "stub"}, nil
}

type stubPolicyHook struct{}

func (stubPolicyHook) Evaluate(_ context.Context, _ hooks.Phase, _ map[string]string) (hooks.Decision, error) {
	return hooks.Allow(), nil
}

type stubPreLLMFilter struct{}

func (stubPreLLMFilter) Filter(_ context.Context, _ *llm.LLMRequest) error { return nil }

type stubPostToolFilter struct{}

func (stubPostToolFilter) Filter(_ context.Context, _ *llm.LLMToolResult) error { return nil }

type stubBudgetGuard struct{}

func (stubBudgetGuard) Check(_ context.Context, _ budget.Usage) error { return nil }

type stubPriceProvider struct{}

func (stubPriceProvider) InputPricePer1K(_ string) float64  { return 0 }
func (stubPriceProvider) OutputPricePer1K(_ string) float64 { return 0 }

type stubLifecycleEmitter struct{}

func (stubLifecycleEmitter) Emit(_ context.Context, _ telemetry.LifecycleEvent) {}

type stubAttributeEnricher struct{}

func (stubAttributeEnricher) Enrich(_ context.Context, attrs map[string]string) map[string]string {
	return attrs
}

type stubCredentialResolver struct{}

func (stubCredentialResolver) Fetch(_ context.Context, _ string) (credentials.Credential, error) {
	return credentials.Credential{Value: []byte("stub")}, nil
}

type stubIdentitySigner struct{}

func (stubIdentitySigner) Sign(_ context.Context, _ map[string]any) (string, error) {
	return "stub-token", nil
}

type stubClassifier struct{}

func (stubClassifier) Classify(err error) errors.TypedError {
	return errors.NewSystemError(err.Error(), err)
}

// --- nil-argument validation tests ---

func TestWithToolInvoker_NilReturnsError(t *testing.T) {
	_, err := orchestrator.New(mock.NewSimple("ok"), orchestrator.WithToolInvoker(nil))
	if err == nil {
		t.Fatal("expected error for nil ToolInvoker")
	}
}

func TestWithPolicyHook_NilReturnsError(t *testing.T) {
	_, err := orchestrator.New(mock.NewSimple("ok"), orchestrator.WithPolicyHook(nil))
	if err == nil {
		t.Fatal("expected error for nil PolicyHook")
	}
}

func TestWithPreLLMFilter_NilReturnsError(t *testing.T) {
	_, err := orchestrator.New(mock.NewSimple("ok"), orchestrator.WithPreLLMFilter(nil))
	if err == nil {
		t.Fatal("expected error for nil PreLLMFilter")
	}
}

func TestWithPostToolFilter_NilReturnsError(t *testing.T) {
	_, err := orchestrator.New(mock.NewSimple("ok"), orchestrator.WithPostToolFilter(nil))
	if err == nil {
		t.Fatal("expected error for nil PostToolFilter")
	}
}

func TestWithBudgetGuard_NilReturnsError(t *testing.T) {
	_, err := orchestrator.New(mock.NewSimple("ok"), orchestrator.WithBudgetGuard(nil))
	if err == nil {
		t.Fatal("expected error for nil BudgetGuard")
	}
}

func TestWithPriceProvider_NilReturnsError(t *testing.T) {
	_, err := orchestrator.New(mock.NewSimple("ok"), orchestrator.WithPriceProvider(nil))
	if err == nil {
		t.Fatal("expected error for nil PriceProvider")
	}
}

func TestWithLifecycleEmitter_NilReturnsError(t *testing.T) {
	_, err := orchestrator.New(mock.NewSimple("ok"), orchestrator.WithLifecycleEmitter(nil))
	if err == nil {
		t.Fatal("expected error for nil LifecycleEmitter")
	}
}

func TestWithAttributeEnricher_NilReturnsError(t *testing.T) {
	_, err := orchestrator.New(mock.NewSimple("ok"), orchestrator.WithAttributeEnricher(nil))
	if err == nil {
		t.Fatal("expected error for nil AttributeEnricher")
	}
}

func TestWithCredentialResolver_NilReturnsError(t *testing.T) {
	_, err := orchestrator.New(mock.NewSimple("ok"), orchestrator.WithCredentialResolver(nil))
	if err == nil {
		t.Fatal("expected error for nil CredentialResolver")
	}
}

func TestWithIdentitySigner_NilReturnsError(t *testing.T) {
	_, err := orchestrator.New(mock.NewSimple("ok"), orchestrator.WithIdentitySigner(nil))
	if err == nil {
		t.Fatal("expected error for nil IdentitySigner")
	}
}

func TestWithErrorClassifier_NilReturnsError(t *testing.T) {
	_, err := orchestrator.New(mock.NewSimple("ok"), orchestrator.WithErrorClassifier(nil))
	if err == nil {
		t.Fatal("expected error for nil ErrorClassifier")
	}
}

// --- non-nil construction tests (table-driven) ---

func TestWithOptions_NonNilSucceeds(t *testing.T) {
	tests := []struct {
		name string
		opt  orchestrator.Option
	}{
		{"WithToolInvoker", orchestrator.WithToolInvoker(stubToolInvoker{})},
		{"WithPolicyHook", orchestrator.WithPolicyHook(stubPolicyHook{})},
		{"WithPreLLMFilter", orchestrator.WithPreLLMFilter(stubPreLLMFilter{})},
		{"WithPostToolFilter", orchestrator.WithPostToolFilter(stubPostToolFilter{})},
		{"WithBudgetGuard", orchestrator.WithBudgetGuard(stubBudgetGuard{})},
		{"WithPriceProvider", orchestrator.WithPriceProvider(stubPriceProvider{})},
		{"WithLifecycleEmitter", orchestrator.WithLifecycleEmitter(stubLifecycleEmitter{})},
		{"WithAttributeEnricher", orchestrator.WithAttributeEnricher(stubAttributeEnricher{})},
		{"WithCredentialResolver", orchestrator.WithCredentialResolver(stubCredentialResolver{})},
		{"WithIdentitySigner", orchestrator.WithIdentitySigner(stubIdentitySigner{})},
		{"WithErrorClassifier", orchestrator.WithErrorClassifier(stubClassifier{})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o, err := orchestrator.New(mock.NewSimple("ok"), tt.opt)
			if err != nil {
				t.Fatalf("New with %s: unexpected error: %v", tt.name, err)
			}
			if o == nil {
				t.Fatalf("New with %s: returned nil orchestrator", tt.name)
			}
		})
	}
}

// --- TestNew_DefaultsAreNullImplementations ---

// TestNew_DefaultsAreNullImplementations verifies that New(provider) with no
// options produces a functional orchestrator that can complete a simple
// invocation. The null defaults must not cause panics or unexpected errors.
func TestNew_DefaultsAreNullImplementations(t *testing.T) {
	p := mock.NewSimple("hello")
	o, err := orchestrator.New(p, orchestrator.WithDefaultModel("test-model"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if o == nil {
		t.Fatal("New returned nil orchestrator")
	}

	// A basic invocation must succeed with all null defaults in place.
	result, err := o.Invoke(context.Background(), invocation.InvocationRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}}},
	})
	if err != nil {
		t.Fatalf("Invoke with defaults: %v", err)
	}
	if result.FinalState != state.Completed {
		t.Errorf("FinalState: want Completed, got %v", result.FinalState)
	}
}

// --- TestOptions_LastWins ---

func TestOptions_LastWins(t *testing.T) {
	first := stubToolInvoker{}
	second := tools.NullInvoker{}

	// Both options accepted — last one must not error.
	o, err := orchestrator.New(
		mock.NewSimple("ok"),
		orchestrator.WithToolInvoker(first),
		orchestrator.WithToolInvoker(second),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if o == nil {
		t.Fatal("returned nil orchestrator")
	}
}

// --- TestOptions_ComposeCorrectly ---

func TestOptions_ComposeCorrectly(t *testing.T) {
	o, err := orchestrator.New(
		mock.NewSimple("ok"),
		orchestrator.WithDefaultModel("my-model"),
		orchestrator.WithMaxIterations(5),
		orchestrator.WithToolInvoker(stubToolInvoker{}),
		orchestrator.WithPolicyHook(stubPolicyHook{}),
		orchestrator.WithPreLLMFilter(stubPreLLMFilter{}),
		orchestrator.WithPostToolFilter(stubPostToolFilter{}),
		orchestrator.WithBudgetGuard(stubBudgetGuard{}),
		orchestrator.WithPriceProvider(stubPriceProvider{}),
		orchestrator.WithLifecycleEmitter(stubLifecycleEmitter{}),
		orchestrator.WithAttributeEnricher(stubAttributeEnricher{}),
		orchestrator.WithCredentialResolver(stubCredentialResolver{}),
		orchestrator.WithIdentitySigner(stubIdentitySigner{}),
		orchestrator.WithErrorClassifier(stubClassifier{}),
	)
	if err != nil {
		t.Fatalf("New with all options: %v", err)
	}
	if o == nil {
		t.Fatal("returned nil orchestrator")
	}

	// A basic invocation with all options wired must still complete.
	result, err := o.Invoke(context.Background(), invocation.InvocationRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}}},
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result.FinalState != state.Completed {
		t.Errorf("FinalState: want Completed, got %v", result.FinalState)
	}
}

// --- error propagation: first nil option aborts construction ---

func TestNew_FirstNilOptionAbortsConstruction(t *testing.T) {
	// If the first option fails, the second (valid) option is never applied.
	// New must return the error from the first failing option.
	_, err := orchestrator.New(
		mock.NewSimple("ok"),
		orchestrator.WithToolInvoker(nil),           // fails
		orchestrator.WithDefaultModel("my-model"),   // should never run
	)
	if err == nil {
		t.Fatal("expected error from nil ToolInvoker option")
	}
}
