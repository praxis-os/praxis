// SPDX-License-Identifier: Apache-2.0

package orchestrator_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/praxis-os/praxis/invocation"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/mock"
	"github.com/praxis-os/praxis/orchestrator"
	"github.com/praxis-os/praxis/state"
)

func TestNew_ValidProvider(t *testing.T) {
	o, err := orchestrator.New(mock.NewSimple("hello"))
	if err != nil {
		t.Fatalf("New: unexpected error: %v", err)
	}
	if o == nil {
		t.Fatal("New: returned nil orchestrator")
	}
}

func TestNew_NilProvider(t *testing.T) {
	_, err := orchestrator.New(nil)
	if err == nil {
		t.Fatal("New(nil): expected error, got nil")
	}
}

func TestInvoke_SimpleTextResponse(t *testing.T) {
	p := mock.NewSimple("the answer is 42")
	o, _ := orchestrator.New(p, orchestrator.WithDefaultModel("test-model"))

	result, err := o.Invoke(context.Background(), invocation.InvocationRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("question")}}},
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result.FinalState != state.Completed {
		t.Errorf("FinalState: want Completed, got %v", result.FinalState)
	}
	if result.Iterations != 1 {
		t.Errorf("Iterations: want 1, got %d", result.Iterations)
	}
	if result.Response.StopReason != llm.StopReasonEndTurn {
		t.Errorf("StopReason: want EndTurn, got %v", result.Response.StopReason)
	}
}

func TestInvoke_ToolUseStubThenComplete(t *testing.T) {
	p := mock.New(
		mock.Response{
			LLMResponse: llm.LLMResponse{
				Message: llm.Message{
					Role: llm.RoleAssistant,
					Parts: []llm.MessagePart{
						llm.ToolCallPart(&llm.LLMToolCall{CallID: "call-1", Name: "search", ArgumentsJSON: []byte(`{}`)}),
					},
				},
				StopReason: llm.StopReasonToolUse,
				Usage:      llm.TokenUsage{InputTokens: 100, OutputTokens: 20},
			},
		},
		mock.Response{
			LLMResponse: llm.LLMResponse{
				Message: llm.Message{
					Role:  llm.RoleAssistant,
					Parts: []llm.MessagePart{llm.TextPart("done")},
				},
				StopReason: llm.StopReasonEndTurn,
				Usage:      llm.TokenUsage{InputTokens: 150, OutputTokens: 10},
			},
		},
	)
	o, _ := orchestrator.New(p, orchestrator.WithDefaultModel("test-model"))

	result, err := o.Invoke(context.Background(), invocation.InvocationRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("do something")}}},
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result.FinalState != state.Completed {
		t.Errorf("FinalState: want Completed, got %v", result.FinalState)
	}
	if result.Iterations != 2 {
		t.Errorf("Iterations: want 2, got %d", result.Iterations)
	}
	if result.TokenUsage.InputTokens != 250 {
		t.Errorf("InputTokens: want 250, got %d", result.TokenUsage.InputTokens)
	}
	if result.TokenUsage.OutputTokens != 30 {
		t.Errorf("OutputTokens: want 30, got %d", result.TokenUsage.OutputTokens)
	}
}

func TestInvoke_ProviderError(t *testing.T) {
	p := mock.New(mock.Response{Err: fmt.Errorf("provider down")})
	o, _ := orchestrator.New(p, orchestrator.WithDefaultModel("test-model"))

	result, err := o.Invoke(context.Background(), invocation.InvocationRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}}},
	})
	if err == nil {
		t.Fatal("expected error from provider failure")
	}
	if result.FinalState != state.Failed {
		t.Errorf("FinalState: want Failed, got %v", result.FinalState)
	}
}

func TestInvoke_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	p := mock.NewSimple("won't reach this")
	o, _ := orchestrator.New(p, orchestrator.WithDefaultModel("test-model"))

	result, err := o.Invoke(ctx, invocation.InvocationRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}}},
	})
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if result.FinalState != state.Cancelled {
		t.Errorf("FinalState: want Cancelled, got %v", result.FinalState)
	}
}

func TestInvoke_MaxIterationsExceeded(t *testing.T) {
	responses := make([]mock.Response, 5)
	for i := range responses {
		responses[i] = mock.Response{
			LLMResponse: llm.LLMResponse{
				Message: llm.Message{
					Role: llm.RoleAssistant,
					Parts: []llm.MessagePart{
						llm.ToolCallPart(&llm.LLMToolCall{CallID: fmt.Sprintf("c%d", i), Name: "tool", ArgumentsJSON: []byte(`{}`)}),
					},
				},
				StopReason: llm.StopReasonToolUse,
				Usage:      llm.TokenUsage{InputTokens: 10, OutputTokens: 5},
			},
		}
	}
	p := mock.New(responses...)
	o, _ := orchestrator.New(p, orchestrator.WithDefaultModel("test-model"), orchestrator.WithMaxIterations(3))

	result, err := o.Invoke(context.Background(), invocation.InvocationRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}}},
	})
	if err == nil {
		t.Fatal("expected error from max iterations exceeded")
	}
	if result.FinalState != state.Failed {
		t.Errorf("FinalState: want Failed, got %v", result.FinalState)
	}
	if result.Iterations != 3 {
		t.Errorf("Iterations: want 3, got %d", result.Iterations)
	}
}

func TestInvoke_DefaultModelUsed(t *testing.T) {
	p := mock.NewSimple("ok")
	o, _ := orchestrator.New(p, orchestrator.WithDefaultModel("my-model"))

	_, err := o.Invoke(context.Background(), invocation.InvocationRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}}},
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	calls := p.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Model != "my-model" {
		t.Errorf("Model: want my-model, got %q", calls[0].Model)
	}
}

func TestInvoke_RequestModelOverridesDefault(t *testing.T) {
	p := mock.NewSimple("ok")
	o, _ := orchestrator.New(p, orchestrator.WithDefaultModel("default-model"))

	_, err := o.Invoke(context.Background(), invocation.InvocationRequest{
		Model:    "override-model",
		Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}}},
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	calls := p.Calls()
	if calls[0].Model != "override-model" {
		t.Errorf("Model: want override-model, got %q", calls[0].Model)
	}
}

func TestInvoke_NoModelConfigured(t *testing.T) {
	p := mock.NewSimple("ok")
	o, _ := orchestrator.New(p)

	_, err := o.Invoke(context.Background(), invocation.InvocationRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}}},
	})
	if err == nil {
		t.Fatal("expected error when no model configured")
	}
}

func TestWithMaxIterations_Clamping(t *testing.T) {
	tests := []struct {
		name  string
		input int
	}{
		{"below minimum", -5},
		{"zero", 0},
		{"exactly 1", 1},
		{"nominal", 10},
		{"exactly 100", 100},
		{"above maximum", 200},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := orchestrator.New(mock.NewSimple("ok"), orchestrator.WithMaxIterations(tt.input), orchestrator.WithDefaultModel("m"))
			if err != nil {
				t.Fatalf("New: %v", err)
			}
		})
	}
}
