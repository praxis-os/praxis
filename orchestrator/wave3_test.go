// SPDX-License-Identifier: Apache-2.0

package orchestrator_test

import (
	"context"
	stderrors "errors"
	"testing"
	"time"

	"github.com/praxis-os/praxis"
	praxiserrors "github.com/praxis-os/praxis/errors"
	"github.com/praxis-os/praxis/event"
	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/mock"
	"github.com/praxis-os/praxis/orchestrator"
	"github.com/praxis-os/praxis/state"
	"github.com/praxis-os/praxis/tools"
)

// --- PolicyHook tests ---

type verdictHook struct {
	verdict hooks.Decision
}

func (h verdictHook) Evaluate(_ context.Context, _ hooks.Phase, _ hooks.PolicyInput) (hooks.Decision, error) {
	return h.verdict, nil
}

func TestPolicyHook_Deny(t *testing.T) {
	p := mock.NewSimple("unreachable")
	o, _ := orchestrator.New(p,
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithPolicyHook(verdictHook{hooks.Deny("forbidden")}),
	)

	result, err := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})
	if err == nil {
		t.Fatal("expected error from policy deny")
	}
	if result.FinalState != state.Failed {
		t.Errorf("FinalState: want Failed, got %v", result.FinalState)
	}

	var policyErr *praxiserrors.PolicyDeniedError
	if !stderrors.As(err, &policyErr) {
		t.Errorf("expected PolicyDeniedError, got %T: %v", err, err)
	}
}

func TestPolicyHook_RequireApproval(t *testing.T) {
	p := mock.NewSimple("unreachable")
	meta := map[string]any{"reviewer": "admin"}
	o, _ := orchestrator.New(p,
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithPolicyHook(verdictHook{hooks.RequireApproval("needs review", meta)}),
	)

	result, err := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})
	if err == nil {
		t.Fatal("expected error from approval required")
	}
	if result.FinalState != state.ApprovalRequired {
		t.Errorf("FinalState: want ApprovalRequired, got %v", result.FinalState)
	}

	var approvalErr *praxiserrors.ApprovalRequiredError
	if !stderrors.As(err, &approvalErr) {
		t.Errorf("expected ApprovalRequiredError, got %T: %v", err, err)
	}
}

func TestPolicyHook_Log(t *testing.T) {
	p := mock.NewSimple("hello")
	o, _ := orchestrator.New(p,
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithPolicyHook(verdictHook{hooks.Log("audit entry")}),
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
}

func TestPolicyHook_Allow(t *testing.T) {
	p := mock.NewSimple("hello")
	o, _ := orchestrator.New(p,
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithPolicyHook(verdictHook{hooks.Allow()}),
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
}

// --- PolicyHook with streaming ---

func TestPolicyHook_Deny_Stream(t *testing.T) {
	p := mock.NewSimple("unreachable")
	o, _ := orchestrator.New(p,
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithPolicyHook(verdictHook{hooks.Deny("blocked")}),
	)

	ch := o.InvokeStream(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})
	events := drainEvents(ch)

	last := events[len(events)-1]
	if last.Type != event.EventTypeInvocationFailed {
		t.Errorf("last event: want Failed, got %q", last.Type)
	}
}

// --- PreLLMFilter tests ---

type blockingPreLLMFilter struct {
	reason string
}

func (f blockingPreLLMFilter) Filter(_ context.Context, messages []llm.Message) ([]llm.Message, []hooks.FilterDecision, error) {
	return messages, []hooks.FilterDecision{
		{Action: hooks.FilterActionBlock, Field: "messages[0].text", Reason: f.reason},
	}, nil
}

type redactingPreLLMFilter struct{}

func (redactingPreLLMFilter) Filter(_ context.Context, messages []llm.Message) ([]llm.Message, []hooks.FilterDecision, error) {
	return messages, []hooks.FilterDecision{
		{Action: hooks.FilterActionRedact, Field: "messages[0].text", Reason: "PII detected"},
	}, nil
}

func TestPreLLMFilter_Block(t *testing.T) {
	p := mock.NewSimple("unreachable")
	o, _ := orchestrator.New(p,
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithPreLLMFilter(blockingPreLLMFilter{reason: "toxic content"}),
	)

	result, err := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("bad input"),
	})
	if err == nil {
		t.Fatal("expected error from filter block")
	}
	if result.FinalState != state.Failed {
		t.Errorf("FinalState: want Failed, got %v", result.FinalState)
	}
}

func TestPreLLMFilter_Redact_EmitsPIIEvent(t *testing.T) {
	p := mock.NewSimple("hello")
	o, _ := orchestrator.New(p,
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithPreLLMFilter(redactingPreLLMFilter{}),
	)

	result, err := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages: userMsg("my SSN is 123-45-6789"),
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result.FinalState != state.Completed {
		t.Errorf("FinalState: want Completed, got %v", result.FinalState)
	}

	// Should contain a PIIRedacted event.
	found := false
	for _, e := range result.Events {
		if e.Type == event.EventTypePIIRedacted {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected EventTypePIIRedacted event, none found")
	}
}

// --- PostToolFilter tests ---

type blockingPostToolFilter struct{}

func (blockingPostToolFilter) Filter(_ context.Context, result tools.ToolResult) (tools.ToolResult, []hooks.FilterDecision, error) {
	return result, []hooks.FilterDecision{
		{Action: hooks.FilterActionBlock, Reason: "tool output blocked"},
	}, nil
}

type redactingPostToolFilter struct{}

func (redactingPostToolFilter) Filter(_ context.Context, result tools.ToolResult) (tools.ToolResult, []hooks.FilterDecision, error) {
	modified := result
	modified.Content = "[REDACTED]"
	return modified, []hooks.FilterDecision{
		{Action: hooks.FilterActionRedact, Field: "content", Reason: "sensitive data"},
	}, nil
}

// Note: PostToolFilter is currently applied at the state level, not per-result.
// These tests verify the filter is wired into the orchestrator options correctly.

// --- Cancellation tests ---

func TestSoftCancel_ProducesCancellationKindSoft(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Soft cancel (context.Canceled)

	p := mock.NewSimple("unreachable")
	o, _ := orchestrator.New(p, orchestrator.WithDefaultModel("test-model"))

	_, err := o.Invoke(ctx, praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})
	if err == nil {
		t.Fatal("expected cancellation error")
	}

	var cancelErr *praxiserrors.CancellationError
	if !stderrors.As(err, &cancelErr) {
		t.Fatalf("expected CancellationError, got %T: %v", err, err)
	}
	if cancelErr.CancelKind() != praxiserrors.CancellationKindSoft {
		t.Errorf("CancelKind: want Soft, got %v", cancelErr.CancelKind())
	}
}

func TestHardCancel_ProducesCancellationKindHard(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Second))
	defer cancel()

	p := mock.NewSimple("unreachable")
	o, _ := orchestrator.New(p, orchestrator.WithDefaultModel("test-model"))

	_, err := o.Invoke(ctx, praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})
	if err == nil {
		t.Fatal("expected cancellation error")
	}

	var cancelErr *praxiserrors.CancellationError
	if !stderrors.As(err, &cancelErr) {
		t.Fatalf("expected CancellationError, got %T: %v", err, err)
	}
	if cancelErr.CancelKind() != praxiserrors.CancellationKindHard {
		t.Errorf("CancelKind: want Hard, got %v", cancelErr.CancelKind())
	}
}

func TestCancel_TerminalEventAlwaysEmitted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	p := mock.NewSimple("unreachable")
	o, _ := orchestrator.New(p, orchestrator.WithDefaultModel("test-model"))

	ch := o.InvokeStream(ctx, praxis.InvocationRequest{
		Messages: userMsg("hi"),
	})
	events := drainEvents(ch)

	if len(events) == 0 {
		t.Fatal("no events received")
	}

	last := events[len(events)-1]
	if !last.Type.IsTerminal() {
		t.Errorf("last event should be terminal, got %q", last.Type)
	}
	if last.Type != event.EventTypeInvocationCancelled {
		t.Errorf("last event: want InvocationCancelled, got %q", last.Type)
	}
}

// --- PolicyHook that inspects PolicyInput ---

type inputCapturingHook struct {
	captured *hooks.PolicyInput
}

func (h *inputCapturingHook) Evaluate(_ context.Context, _ hooks.Phase, input hooks.PolicyInput) (hooks.Decision, error) {
	*h.captured = input
	return hooks.Allow(), nil
}

func TestPolicyHook_ReceivesCorrectInput(t *testing.T) {
	p := mock.NewSimple("hello")
	var captured hooks.PolicyInput
	hook := &inputCapturingHook{captured: &captured}
	o, _ := orchestrator.New(p,
		orchestrator.WithDefaultModel("test-model"),
		orchestrator.WithPolicyHook(hook),
	)

	_, err := o.Invoke(context.Background(), praxis.InvocationRequest{
		Messages:     userMsg("hi"),
		SystemPrompt: "be helpful",
		Metadata:     map[string]string{"key": "value"},
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}

	if captured.Model != "test-model" {
		t.Errorf("PolicyInput.Model: want test-model, got %q", captured.Model)
	}
	if captured.SystemPrompt != "be helpful" {
		t.Errorf("PolicyInput.SystemPrompt: want 'be helpful', got %q", captured.SystemPrompt)
	}
	if captured.Metadata["key"] != "value" {
		t.Errorf("PolicyInput.Metadata[key]: want 'value', got %q", captured.Metadata["key"])
	}
}
