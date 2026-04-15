// SPDX-License-Identifier: Apache-2.0

package orchestrator_test

import (
	"context"
	"strings"
	"testing"

	"github.com/praxis-os/praxis"
	"github.com/praxis-os/praxis/budget"
	"github.com/praxis-os/praxis/event"
	"github.com/praxis-os/praxis/credentials"
	"github.com/praxis-os/praxis/errors"
	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/mock"
	"github.com/praxis-os/praxis/orchestrator"
	"github.com/praxis-os/praxis/state"
	"github.com/praxis-os/praxis/tools"
)

// --- stub implementations used only in tests ---

type stubToolInvoker struct{}

func (stubToolInvoker) Invoke(_ context.Context, _ tools.InvocationContext, call tools.ToolCall) (tools.ToolResult, error) {
	return tools.ToolResult{CallID: call.CallID, Content: "stub", Status: tools.ToolStatusSuccess}, nil
}

type stubPolicyHook struct{}

func (stubPolicyHook) Evaluate(_ context.Context, _ hooks.Phase, _ hooks.PolicyInput) (hooks.Decision, error) {
	return hooks.Allow(), nil
}

type stubPreLLMFilter struct{}

func (stubPreLLMFilter) Filter(_ context.Context, messages []llm.Message) ([]llm.Message, []hooks.FilterDecision, error) {
	return messages, nil, nil
}

type stubPostToolFilter struct{}

func (stubPostToolFilter) Filter(_ context.Context, result tools.ToolResult) (tools.ToolResult, []hooks.FilterDecision, error) {
	return result, nil, nil
}

type stubBudgetGuard struct{}

func (stubBudgetGuard) RecordTokens(_ context.Context, _, _ int64) error        { return nil }
func (stubBudgetGuard) RecordToolCall(_ context.Context) error                   { return nil }
func (stubBudgetGuard) RecordCost(_ context.Context, _ int64) error              { return nil }
func (stubBudgetGuard) Check(_ context.Context) (budget.BudgetSnapshot, error)   { return budget.BudgetSnapshot{}, nil }
func (stubBudgetGuard) Snapshot(_ context.Context) budget.BudgetSnapshot         { return budget.BudgetSnapshot{} }

type stubPriceProvider struct{}

func (stubPriceProvider) PriceForToken(_ context.Context, _, _ string, _ budget.TokenDirection) (int64, error) {
	return 0, nil
}

type stubLifecycleEmitter struct{}

func (stubLifecycleEmitter) Emit(_ context.Context, _ event.InvocationEvent) error { return nil }

type stubAttributeEnricher struct{}

func (stubAttributeEnricher) Enrich(_ context.Context) map[string]string {
	return map[string]string{}
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

func TestNew_DefaultsAreNullImplementations(t *testing.T) {
	p := mock.NewSimple("hello")
	o, err := orchestrator.New(p, orchestrator.WithDefaultModel("test-model"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if o == nil {
		t.Fatal("New returned nil orchestrator")
	}

	result, err := o.Invoke(context.Background(), praxis.InvocationRequest{
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

	result, err := o.Invoke(context.Background(), praxis.InvocationRequest{
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
	_, err := orchestrator.New(
		mock.NewSimple("ok"),
		orchestrator.WithToolInvoker(nil),
		orchestrator.WithDefaultModel("my-model"),
	)
	if err == nil {
		t.Fatal("expected error from nil ToolInvoker option")
	}
}

// --- WithSystemPromptFragment tests ---

func TestWithSystemPromptFragment_SingleFragment(t *testing.T) {
	o, err := orchestrator.New(
		mock.NewSimple("ok"),
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithSystemPromptFragment("skill-a", "You are a code reviewer."),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got := o.ComposedSystemPrompt("base prompt")
	want := "base prompt\n\n--- Skills ---\n\nYou are a code reviewer."
	if got != want {
		t.Errorf("ComposedSystemPrompt:\n got: %q\nwant: %q", got, want)
	}
}

func TestWithSystemPromptFragment_MultipleFragments_OrderPreserved(t *testing.T) {
	o, err := orchestrator.New(
		mock.NewSimple("ok"),
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithSystemPromptFragment("skill-a", "Fragment A"),
		orchestrator.WithSystemPromptFragment("skill-b", "Fragment B"),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got := o.ComposedSystemPrompt("")
	want := "\n\n--- Skills ---\n\nFragment A\n\n--- Skills ---\n\nFragment B"
	if got != want {
		t.Errorf("ComposedSystemPrompt:\n got: %q\nwant: %q", got, want)
	}
}

func TestWithSystemPromptFragment_NoFragments_ReturnsBase(t *testing.T) {
	o, err := orchestrator.New(
		mock.NewSimple("ok"),
		orchestrator.WithDefaultModel("test-model"),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got := o.ComposedSystemPrompt("base")
	if got != "base" {
		t.Errorf("ComposedSystemPrompt: got %q, want %q", got, "base")
	}
}

func TestWithSystemPromptFragment_DuplicateName_Panics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for duplicate fragment name")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value is %T, want string", r)
		}
		if !strings.Contains(msg, "duplicate") || !strings.Contains(msg, "skill-a") {
			t.Errorf("panic message %q should mention 'duplicate' and 'skill-a'", msg)
		}
	}()

	_, _ = orchestrator.New(
		mock.NewSimple("ok"),
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithSystemPromptFragment("skill-a", "first"),
		orchestrator.WithSystemPromptFragment("skill-a", "second"),
	)
}

func TestWithSystemPromptFragment_FragmentReachesLLM(t *testing.T) {
	// Verify that the composed system prompt reaches the LLM provider.
	p := mock.NewSimple("response")
	o, err := orchestrator.New(
		p,
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithSystemPromptFragment("reviewer", "Review code carefully."),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := o.Invoke(context.Background(), praxis.InvocationRequest{
		SystemPrompt: "You are helpful.",
		Messages:     []llm.Message{{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}}},
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result.FinalState != state.Completed {
		t.Fatalf("FinalState: want Completed, got %v", result.FinalState)
	}

	calls := p.Calls()
	if len(calls) == 0 {
		t.Fatal("expected at least one LLM call")
	}
	gotPrompt := calls[0].SystemPrompt
	wantPrompt := "You are helpful.\n\n--- Skills ---\n\nReview code carefully."
	if gotPrompt != wantPrompt {
		t.Errorf("LLM received SystemPrompt:\n got: %q\nwant: %q", gotPrompt, wantPrompt)
	}
}
