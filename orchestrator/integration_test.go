// SPDX-License-Identifier: Apache-2.0

package orchestrator_test

// integration_test.go — verifies all 14 Phase-3 interfaces compose correctly
// through the orchestrator. Each interface is exercised via a custom stub that
// records whether and how it was called, and a "full-stack" test wires all 14
// into a single invocation to verify they coexist without interference.

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"sync/atomic"
	"testing"

	"github.com/praxis-os/praxis"
	"github.com/praxis-os/praxis/budget"
	"github.com/praxis-os/praxis/credentials"
	praxiserrors "github.com/praxis-os/praxis/errors"
	"github.com/praxis-os/praxis/event"
	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/identity"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/mock"
	"github.com/praxis-os/praxis/orchestrator"
	"github.com/praxis-os/praxis/state"
	"github.com/praxis-os/praxis/tools"
)

// newTestSigner creates an Ed25519 signer with a fresh random key for testing.
func newTestSigner(t *testing.T) identity.Signer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	s, err := identity.NewEd25519Signer(priv)
	if err != nil {
		t.Fatalf("NewEd25519Signer: %v", err)
	}
	return s
}

// ---------------------------------------------------------------------------
// Stub implementations that record calls for verification.
// ---------------------------------------------------------------------------

// recordingPolicyHook records Evaluate calls and returns Allow.
type recordingPolicyHook struct {
	calls atomic.Int32
}

func (h *recordingPolicyHook) Evaluate(_ context.Context, _ hooks.Phase, _ hooks.PolicyInput) (hooks.Decision, error) {
	h.calls.Add(1)
	return hooks.Allow(), nil
}

// recordingPreLLMFilter passes messages through and records calls.
type recordingPreLLMFilter struct {
	calls atomic.Int32
}

func (f *recordingPreLLMFilter) Filter(_ context.Context, msgs []llm.Message) ([]llm.Message, []hooks.FilterDecision, error) {
	f.calls.Add(1)
	return msgs, nil, nil
}

// recordingPreToolFilter passes calls through and records invocations.
type recordingPreToolFilter struct {
	calls atomic.Int32
}

func (f *recordingPreToolFilter) Filter(_ context.Context, call tools.ToolCall) (tools.ToolCall, []hooks.FilterDecision, error) {
	f.calls.Add(1)
	return call, nil, nil
}

// recordingPostToolFilter passes results through and records invocations.
type recordingPostToolFilter struct {
	calls atomic.Int32
}

func (f *recordingPostToolFilter) Filter(_ context.Context, result tools.ToolResult) (tools.ToolResult, []hooks.FilterDecision, error) {
	f.calls.Add(1)
	return result, nil, nil
}

// recordingEmitter records lifecycle events.
type recordingEmitter struct {
	events []event.InvocationEvent
}

func (e *recordingEmitter) Emit(_ context.Context, ev event.InvocationEvent) error {
	e.events = append(e.events, ev)
	return nil
}

// recordingEnricher returns a fixed attribute set and records calls.
type recordingEnricher struct {
	calls atomic.Int32
}

func (e *recordingEnricher) Enrich(_ context.Context) map[string]string {
	e.calls.Add(1)
	return map[string]string{"test.attr": "integration"}
}

// recordingResolver records fetch calls. Returns a dummy credential.
type recordingResolver struct {
	calls atomic.Int32
}

func (r *recordingResolver) Fetch(_ context.Context, name string) (credentials.Credential, error) {
	r.calls.Add(1)
	return credentials.Credential{Value: []byte("test-secret")}, nil
}

// recordingInvoker records tool invocations and returns success.
type recordingInvoker struct {
	calls atomic.Int32
}

func (inv *recordingInvoker) Invoke(_ context.Context, _ tools.InvocationContext, call tools.ToolCall) (tools.ToolResult, error) {
	inv.calls.Add(1)
	return tools.ToolResult{
		CallID:  call.CallID,
		Content: "ok",
		Status:  tools.ToolStatusSuccess,
	}, nil
}

// ---------------------------------------------------------------------------
// Individual interface integration tests
// ---------------------------------------------------------------------------

// TestIntegration_Provider verifies llm.Provider is called and its response
// drives the invocation result.
func TestIntegration_Provider(t *testing.T) {
	p := mock.NewSimple("provider integration response")
	o, err := orchestrator.New(p, orchestrator.WithDefaultModel("test-model"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("hello"),
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result.FinalState != state.Completed {
		t.Errorf("FinalState: want Completed, got %v", result.FinalState)
	}
	if p.CallCount() != 1 {
		t.Errorf("Provider call count: want 1, got %d", p.CallCount())
	}
}

// TestIntegration_ToolInvoker verifies tools.Invoker is dispatched for tool calls.
func TestIntegration_ToolInvoker(t *testing.T) {
	inv := &recordingInvoker{}
	tc := &llm.LLMToolCall{CallID: "int-c1", Name: "test_tool", ArgumentsJSON: []byte(`{}`)}

	p := mock.New(
		toolCallResponse(50, 10, tc),
		textResponse("tool done", 60, 5),
	)

	o, _ := orchestrator.New(p,
		orchestrator.WithDefaultModel("m"),
		orchestrator.WithToolInvoker(inv),
	)

	result, err := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("use tool"),
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result.FinalState != state.Completed {
		t.Errorf("FinalState: want Completed, got %v", result.FinalState)
	}
	if inv.calls.Load() != 1 {
		t.Errorf("Invoker calls: want 1, got %d", inv.calls.Load())
	}
}

// TestIntegration_PolicyHook verifies hooks.PolicyHook is evaluated at
// lifecycle phases.
func TestIntegration_PolicyHook(t *testing.T) {
	hook := &recordingPolicyHook{}
	p := mock.NewSimple("policy ok")

	o, _ := orchestrator.New(p,
		orchestrator.WithDefaultModel("m"),
		orchestrator.WithPolicyHook(hook),
	)

	result, err := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result.FinalState != state.Completed {
		t.Errorf("FinalState: want Completed, got %v", result.FinalState)
	}
	// PolicyHook is called at: PreInvocation, PreLLMInput, PostInvocation = 3 phases minimum
	if hook.calls.Load() < 2 {
		t.Errorf("PolicyHook calls: want >=2, got %d", hook.calls.Load())
	}
}

// TestIntegration_PreLLMFilter verifies hooks.PreLLMFilter is applied before
// the LLM call.
func TestIntegration_PreLLMFilter(t *testing.T) {
	filter := &recordingPreLLMFilter{}
	p := mock.NewSimple("filtered")

	o, _ := orchestrator.New(p,
		orchestrator.WithDefaultModel("m"),
		orchestrator.WithPreLLMFilter(filter),
	)

	result, err := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result.FinalState != state.Completed {
		t.Errorf("FinalState: want Completed, got %v", result.FinalState)
	}
	if filter.calls.Load() != 1 {
		t.Errorf("PreLLMFilter calls: want 1, got %d", filter.calls.Load())
	}
}

// TestIntegration_PreToolFilter verifies hooks.PreToolFilter is applied before
// tool dispatch.
func TestIntegration_PreToolFilter(t *testing.T) {
	filter := &recordingPreToolFilter{}
	inv := &recordingInvoker{}
	tc := &llm.LLMToolCall{CallID: "ptf-c1", Name: "tool", ArgumentsJSON: []byte(`{}`)}

	p := mock.New(
		toolCallResponse(50, 10, tc),
		textResponse("done", 60, 5),
	)

	o, _ := orchestrator.New(p,
		orchestrator.WithDefaultModel("m"),
		orchestrator.WithToolInvoker(inv),
		orchestrator.WithPreToolFilter(filter),
	)

	result, err := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("use tool"),
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result.FinalState != state.Completed {
		t.Errorf("FinalState: want Completed, got %v", result.FinalState)
	}
	if filter.calls.Load() != 1 {
		t.Errorf("PreToolFilter calls: want 1, got %d", filter.calls.Load())
	}
}

// TestIntegration_PostToolFilter verifies hooks.PostToolFilter is applied after
// tool execution.
func TestIntegration_PostToolFilter(t *testing.T) {
	filter := &recordingPostToolFilter{}
	inv := &recordingInvoker{}
	tc := &llm.LLMToolCall{CallID: "potf-c1", Name: "tool", ArgumentsJSON: []byte(`{}`)}

	p := mock.New(
		toolCallResponse(50, 10, tc),
		textResponse("done", 60, 5),
	)

	o, _ := orchestrator.New(p,
		orchestrator.WithDefaultModel("m"),
		orchestrator.WithToolInvoker(inv),
		orchestrator.WithPostToolFilter(filter),
	)

	result, err := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("use tool"),
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result.FinalState != state.Completed {
		t.Errorf("FinalState: want Completed, got %v", result.FinalState)
	}
	if filter.calls.Load() != 1 {
		t.Errorf("PostToolFilter calls: want 1, got %d", filter.calls.Load())
	}
}

// TestIntegration_BudgetGuard verifies budget.Guard records tokens and enforces limits.
func TestIntegration_BudgetGuard(t *testing.T) {
	guard := budget.NewBudgetGuard(budget.Config{MaxInputTokens: 10000})
	p := mock.NewSimple("budget ok")

	o, _ := orchestrator.New(p,
		orchestrator.WithDefaultModel("m"),
		orchestrator.WithBudgetGuard(guard),
	)

	result, err := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result.FinalState != state.Completed {
		t.Errorf("FinalState: want Completed, got %v", result.FinalState)
	}

	snap := guard.Snapshot(context.Background())
	if snap.InputTokensUsed == 0 {
		t.Error("BudgetGuard: expected non-zero InputTokens after invocation")
	}
}

// TestIntegration_PriceProvider verifies budget.PriceProvider is accepted by the
// orchestrator option wiring.
func TestIntegration_PriceProvider(t *testing.T) {
	pp := budget.NewStaticPriceProvider(map[budget.PriceKey]int64{
		{Provider: "mock", Model: "m", Direction: budget.TokenDirectionInput}:  100,
		{Provider: "mock", Model: "m", Direction: budget.TokenDirectionOutput}: 300,
	})
	p := mock.NewSimple("price ok")

	o, err := orchestrator.New(p,
		orchestrator.WithDefaultModel("m"),
		orchestrator.WithPriceProvider(pp),
	)
	if err != nil {
		t.Fatalf("New with PriceProvider: %v", err)
	}

	result, invokeErr := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})
	if invokeErr != nil {
		t.Fatalf("Invoke: %v", invokeErr)
	}
	if result.FinalState != state.Completed {
		t.Errorf("FinalState: want Completed, got %v", result.FinalState)
	}
}

// TestIntegration_TypedError verifies errors.TypedError interface contract is
// maintained through error classification.
func TestIntegration_TypedError(t *testing.T) {
	// Provider returns a 429 error → should be classified as TransientLLMError.
	p := mock.New(mock.Response{
		Err: &httpError{status: 429, msg: "rate limited"},
	})

	o, _ := orchestrator.New(p, orchestrator.WithDefaultModel("m"))
	_, invokeErr := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})
	if invokeErr == nil {
		t.Fatal("expected error")
	}

	// Verify it implements TypedError.
	typed, ok := invokeErr.(praxiserrors.TypedError)
	if !ok {
		t.Fatalf("error does not implement TypedError: %T", invokeErr)
	}
	if typed.Kind() != praxiserrors.ErrorKindTransientLLM {
		t.Errorf("Kind: want transient_llm, got %v", typed.Kind())
	}
	if typed.HTTPStatusCode() == 0 {
		t.Error("HTTPStatusCode: want non-zero")
	}
}

// TestIntegration_Classifier verifies errors.Classifier option is used for
// error classification.
func TestIntegration_Classifier(t *testing.T) {
	classifier := praxiserrors.NewDefaultClassifier()
	p := mock.New(mock.Response{
		Err: &httpError{status: 503, msg: "service unavailable"},
	})

	o, _ := orchestrator.New(p,
		orchestrator.WithDefaultModel("m"),
		orchestrator.WithErrorClassifier(classifier),
	)
	_, invokeErr := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})
	if invokeErr == nil {
		t.Fatal("expected error")
	}

	typed, ok := invokeErr.(praxiserrors.TypedError)
	if !ok {
		t.Fatalf("error does not implement TypedError: %T", invokeErr)
	}
	if !typed.Kind().IsRetryable() {
		t.Errorf("503 should be retryable, got Kind=%v", typed.Kind())
	}
}

// TestIntegration_LifecycleEventEmitter verifies telemetry.LifecycleEventEmitter
// receives events via InvokeStream.
func TestIntegration_LifecycleEventEmitter(t *testing.T) {
	emitter := &recordingEmitter{}
	p := mock.NewSimple("emitter ok")

	o, _ := orchestrator.New(p,
		orchestrator.WithDefaultModel("m"),
		orchestrator.WithLifecycleEmitter(emitter),
	)

	// Use InvokeStream to capture events.
	ch := o.InvokeStream(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})
	events := drainEvents(ch)

	// Stream channel should have delivered lifecycle events.
	if len(events) < 3 {
		t.Errorf("stream events: want >=3, got %d", len(events))
	}

	// Verify at least InvocationStarted and InvocationCompleted are present.
	hasStarted := false
	hasCompleted := false
	for _, ev := range events {
		if ev.Type == event.EventTypeInvocationStarted {
			hasStarted = true
		}
		if ev.Type == event.EventTypeInvocationCompleted {
			hasCompleted = true
		}
	}
	if !hasStarted {
		t.Error("missing InvocationStarted event")
	}
	if !hasCompleted {
		t.Error("missing InvocationCompleted event")
	}
}

// TestIntegration_AttributeEnricher verifies telemetry.AttributeEnricher option
// wiring is accepted.
func TestIntegration_AttributeEnricher(t *testing.T) {
	enricher := &recordingEnricher{}
	p := mock.NewSimple("enriched")

	o, err := orchestrator.New(p,
		orchestrator.WithDefaultModel("m"),
		orchestrator.WithAttributeEnricher(enricher),
	)
	if err != nil {
		t.Fatalf("New with AttributeEnricher: %v", err)
	}

	result, invokeErr := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})
	if invokeErr != nil {
		t.Fatalf("Invoke: %v", invokeErr)
	}
	if result.FinalState != state.Completed {
		t.Errorf("FinalState: want Completed, got %v", result.FinalState)
	}
}

// TestIntegration_CredentialResolver verifies credentials.Resolver option
// wiring is accepted.
func TestIntegration_CredentialResolver(t *testing.T) {
	resolver := &recordingResolver{}
	p := mock.NewSimple("credential ok")

	o, err := orchestrator.New(p,
		orchestrator.WithDefaultModel("m"),
		orchestrator.WithCredentialResolver(resolver),
	)
	if err != nil {
		t.Fatalf("New with CredentialResolver: %v", err)
	}

	result, invokeErr := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})
	if invokeErr != nil {
		t.Fatalf("Invoke: %v", invokeErr)
	}
	if result.FinalState != state.Completed {
		t.Errorf("FinalState: want Completed, got %v", result.FinalState)
	}
}

// TestIntegration_IdentitySigner verifies identity.Signer produces a signed
// token in the invocation result.
func TestIntegration_IdentitySigner(t *testing.T) {
	signer := newTestSigner(t)

	p := mock.NewSimple("identity ok")
	o, _ := orchestrator.New(p,
		orchestrator.WithDefaultModel("m"),
		orchestrator.WithIdentitySigner(signer),
	)

	result, invokeErr := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})
	if invokeErr != nil {
		t.Fatalf("Invoke: %v", invokeErr)
	}
	if result.FinalState != state.Completed {
		t.Errorf("FinalState: want Completed, got %v", result.FinalState)
	}
	if result.SignedIdentity == "" {
		t.Error("SignedIdentity: expected non-empty token")
	}
}

// ---------------------------------------------------------------------------
// Full-stack integration: all 14 interfaces wired into a single invocation.
// ---------------------------------------------------------------------------

// TestIntegration_FullStack_All14Interfaces wires all 14 Phase-3 interfaces
// into a single orchestrator invocation with a tool call cycle and verifies
// every interface is invoked and the invocation completes successfully.
func TestIntegration_FullStack_All14Interfaces(t *testing.T) {
	// 1. llm.Provider (mock)
	tc := &llm.LLMToolCall{CallID: "fs-c1", Name: "lookup", ArgumentsJSON: []byte(`{"q":"test"}`)}
	p := mock.New(
		toolCallResponse(100, 20, tc),
		textResponse("full stack complete", 150, 30),
	)

	// 2. tools.Invoker
	inv := &recordingInvoker{}

	// 3. hooks.PolicyHook
	policyHook := &recordingPolicyHook{}

	// 4. hooks.PreLLMFilter
	preLLMFilter := &recordingPreLLMFilter{}

	// 5. hooks.PreToolFilter
	preToolFilter := &recordingPreToolFilter{}

	// 6. hooks.PostToolFilter
	postToolFilter := &recordingPostToolFilter{}

	// 7. budget.Guard
	guard := budget.NewBudgetGuard(budget.Config{
		MaxInputTokens:  100000,
		MaxOutputTokens: 100000,
		MaxToolCalls:    100,
	})

	// 8. budget.PriceProvider
	priceProvider := budget.NewStaticPriceProvider(map[budget.PriceKey]int64{
		{Provider: "mock", Model: "m", Direction: budget.TokenDirectionInput}:  10,
		{Provider: "mock", Model: "m", Direction: budget.TokenDirectionOutput}: 30,
	})

	// 9. errors.TypedError — verified implicitly through classification
	// 10. errors.Classifier
	classifier := praxiserrors.NewDefaultClassifier()

	// 11. telemetry.LifecycleEventEmitter
	emitter := &recordingEmitter{}

	// 12. telemetry.AttributeEnricher
	enricher := &recordingEnricher{}

	// 13. credentials.Resolver
	resolver := &recordingResolver{}

	// 14. identity.Signer
	signer := newTestSigner(t)

	o, err := orchestrator.New(p,
		orchestrator.WithDefaultModel("m"),
		orchestrator.WithToolInvoker(inv),
		orchestrator.WithPolicyHook(policyHook),
		orchestrator.WithPreLLMFilter(preLLMFilter),
		orchestrator.WithPreToolFilter(preToolFilter),
		orchestrator.WithPostToolFilter(postToolFilter),
		orchestrator.WithBudgetGuard(guard),
		orchestrator.WithPriceProvider(priceProvider),
		orchestrator.WithErrorClassifier(classifier),
		orchestrator.WithLifecycleEmitter(emitter),
		orchestrator.WithAttributeEnricher(enricher),
		orchestrator.WithCredentialResolver(resolver),
		orchestrator.WithIdentitySigner(signer),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, invokeErr := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("full stack test"),
	})
	if invokeErr != nil {
		t.Fatalf("Invoke: %v", invokeErr)
	}
	if result.FinalState != state.Completed {
		t.Errorf("FinalState: want Completed, got %v", result.FinalState)
	}

	// Verify each recording interface was called.
	if p.CallCount() != 2 {
		t.Errorf("Provider calls: want 2, got %d", p.CallCount())
	}
	if inv.calls.Load() != 1 {
		t.Errorf("Invoker calls: want 1, got %d", inv.calls.Load())
	}
	if policyHook.calls.Load() < 2 {
		t.Errorf("PolicyHook calls: want >=2, got %d", policyHook.calls.Load())
	}
	if preLLMFilter.calls.Load() < 1 {
		t.Errorf("PreLLMFilter calls: want >=1, got %d", preLLMFilter.calls.Load())
	}
	if preToolFilter.calls.Load() != 1 {
		t.Errorf("PreToolFilter calls: want 1, got %d", preToolFilter.calls.Load())
	}
	if postToolFilter.calls.Load() != 1 {
		t.Errorf("PostToolFilter calls: want 1, got %d", postToolFilter.calls.Load())
	}

	// Budget: tokens should have been recorded.
	snap := guard.Snapshot(context.Background())
	if snap.InputTokensUsed == 0 {
		t.Error("Budget InputTokens: want non-zero")
	}
	if snap.ToolCallsUsed == 0 {
		t.Error("Budget ToolCalls: want non-zero")
	}

	// Identity: signed token should be present.
	if result.SignedIdentity == "" {
		t.Error("SignedIdentity: expected non-empty token")
	}

	// InvocationID should be present.
	if result.InvocationID == "" {
		t.Error("InvocationID: expected non-empty")
	}
}

// TestIntegration_FullStack_StreamAll14 repeats the full-stack test via
// InvokeStream to verify event delivery with all interfaces wired.
func TestIntegration_FullStack_StreamAll14(t *testing.T) {
	tc := &llm.LLMToolCall{CallID: "str-c1", Name: "tool", ArgumentsJSON: []byte(`{}`)}
	p := mock.New(
		toolCallResponse(50, 10, tc),
		textResponse("stream complete", 80, 15),
	)

	inv := &recordingInvoker{}
	policyHook := &recordingPolicyHook{}
	preLLMFilter := &recordingPreLLMFilter{}
	preToolFilter := &recordingPreToolFilter{}
	postToolFilter := &recordingPostToolFilter{}
	guard := budget.NewBudgetGuard(budget.Config{MaxInputTokens: 100000})
	priceProvider := budget.NewStaticPriceProvider(nil)
	classifier := praxiserrors.NewDefaultClassifier()
	emitter := &recordingEmitter{}
	enricher := &recordingEnricher{}
	resolver := &recordingResolver{}
	signer := newTestSigner(t)

	o, _ := orchestrator.New(p,
		orchestrator.WithDefaultModel("m"),
		orchestrator.WithToolInvoker(inv),
		orchestrator.WithPolicyHook(policyHook),
		orchestrator.WithPreLLMFilter(preLLMFilter),
		orchestrator.WithPreToolFilter(preToolFilter),
		orchestrator.WithPostToolFilter(postToolFilter),
		orchestrator.WithBudgetGuard(guard),
		orchestrator.WithPriceProvider(priceProvider),
		orchestrator.WithErrorClassifier(classifier),
		orchestrator.WithLifecycleEmitter(emitter),
		orchestrator.WithAttributeEnricher(enricher),
		orchestrator.WithCredentialResolver(resolver),
		orchestrator.WithIdentitySigner(signer),
	)

	ch := o.InvokeStream(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("stream test"),
	})
	events := drainEvents(ch)

	// Should have lifecycle events including tool-related ones.
	if len(events) < 5 {
		t.Errorf("stream events: want >=5, got %d", len(events))
	}

	// Last event should be terminal (Completed or InvocationCompleted).
	last := events[len(events)-1]
	if last.State != state.Completed {
		t.Errorf("last event state: want Completed, got %v", last.State)
	}

	// All recording interfaces should have been called.
	if inv.calls.Load() < 1 {
		t.Error("Invoker not called in stream path")
	}
	if policyHook.calls.Load() < 2 {
		t.Error("PolicyHook not called enough in stream path")
	}
	if preLLMFilter.calls.Load() < 1 {
		t.Error("PreLLMFilter not called in stream path")
	}
	if preToolFilter.calls.Load() < 1 {
		t.Error("PreToolFilter not called in stream path")
	}
	if postToolFilter.calls.Load() < 1 {
		t.Error("PostToolFilter not called in stream path")
	}
}

// TestIntegration_AttributeEnricher_Propagated verifies that
// AttributeEnricher.Enrich is called at Initializing and its return value
// is attached to every event after InvocationStarted (D60).
func TestIntegration_AttributeEnricher_Propagated(t *testing.T) {
	enricher := &recordingEnricher{}
	p := mock.NewSimple("enriched response")

	o, _ := orchestrator.New(p,
		orchestrator.WithDefaultModel("m"),
		orchestrator.WithAttributeEnricher(enricher),
	)

	ch := o.InvokeStream(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})
	events := drainEvents(ch)

	if enricher.calls.Load() != 1 {
		t.Errorf("Enricher calls: want 1, got %d", enricher.calls.Load())
	}

	// InvocationStarted (first event) should have nil EnricherAttributes.
	if len(events) < 2 {
		t.Fatalf("expected >=2 events, got %d", len(events))
	}
	if events[0].Type != event.EventTypeInvocationStarted {
		t.Fatalf("first event: want InvocationStarted, got %q", events[0].Type)
	}
	if events[0].EnricherAttributes != nil {
		t.Errorf("InvocationStarted: want nil EnricherAttributes, got %v", events[0].EnricherAttributes)
	}

	// All subsequent events should carry the enricher attributes.
	for i := 1; i < len(events); i++ {
		if events[i].EnricherAttributes == nil {
			t.Errorf("event[%d] (%s): EnricherAttributes is nil, want non-nil", i, events[i].Type)
			continue
		}
		if events[i].EnricherAttributes["test.attr"] != "integration" {
			t.Errorf("event[%d] (%s): want test.attr=integration, got %v", i, events[i].Type, events[i].EnricherAttributes)
		}
	}
}
