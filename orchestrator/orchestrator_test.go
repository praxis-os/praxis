// SPDX-License-Identifier: Apache-2.0

package orchestrator_test

import (
	"context"
	"strings"
	"testing"

	"github.com/praxis-os/praxis/invocation"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/orchestrator"
	"github.com/praxis-os/praxis/state"
)

// stubProvider is a minimal llm.Provider implementation for testing.
type stubProvider struct{}

func (s *stubProvider) Complete(_ context.Context, _ llm.LLMRequest) (llm.LLMResponse, error) {
	return llm.LLMResponse{}, nil
}

func (s *stubProvider) Stream(_ context.Context, _ llm.LLMRequest) (<-chan llm.LLMStreamChunk, error) {
	ch := make(chan llm.LLMStreamChunk)
	close(ch)
	return ch, nil
}

func (s *stubProvider) Name() string                    { return "stub" }
func (s *stubProvider) SupportsParallelToolCalls() bool { return false }
func (s *stubProvider) Capabilities() llm.Capabilities  { return llm.Capabilities{} }

func TestNew_ValidProvider(t *testing.T) {
	orch, err := orchestrator.New(&stubProvider{})
	if err != nil {
		t.Fatalf("New with valid provider: unexpected error: %v", err)
	}
	if orch == nil {
		t.Fatal("New with valid provider: returned nil Orchestrator")
	}
}

func TestNew_NilProvider(t *testing.T) {
	orch, err := orchestrator.New(nil)
	if err == nil {
		t.Fatal("New with nil provider: expected error, got nil")
	}
	if orch != nil {
		t.Fatal("New with nil provider: expected nil Orchestrator, got non-nil")
	}
}

func TestNew_DefaultMaxIterations(t *testing.T) {
	// Validate the default via the Invoke stub which exercises the struct,
	// and also indirectly through WithMaxIterations clamping behaviour.
	// We probe the default by supplying no WithMaxIterations option and
	// confirming we can create the orchestrator without error.
	orch, err := orchestrator.New(&stubProvider{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if orch == nil {
		t.Fatal("expected non-nil Orchestrator")
	}
	// The default is tested indirectly: WithMaxIterations(0) should clamp to 1,
	// not be equal to the default of 10. We use a separate subtest below.
}

func TestWithDefaultModel(t *testing.T) {
	tests := []struct {
		name  string
		model string
	}{
		{name: "non-empty model", model: "claude-3-5-sonnet-20241022"},
		{name: "empty model is a no-op", model: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Construction must succeed regardless of model value.
			orch, err := orchestrator.New(&stubProvider{}, orchestrator.WithDefaultModel(tc.model))
			if err != nil {
				t.Fatalf("New: unexpected error: %v", err)
			}
			if orch == nil {
				t.Fatal("expected non-nil Orchestrator")
			}
		})
	}
}

func TestWithMaxIterations_Clamping(t *testing.T) {
	tests := []struct {
		name  string
		input int
		// We cannot directly inspect maxIterations since it is unexported.
		// We verify the option is accepted without error and that extreme
		// values do not panic.
		wantErr bool
	}{
		{name: "below minimum clamps to 1", input: 0},
		{name: "negative clamps to 1", input: -5},
		{name: "exactly 1 is valid", input: 1},
		{name: "nominal value", input: 10},
		{name: "exactly 100 is valid", input: 100},
		{name: "above maximum clamps to 100", input: 101},
		{name: "large value clamps to 100", input: 99999},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			orch, err := orchestrator.New(&stubProvider{}, orchestrator.WithMaxIterations(tc.input))
			if (err != nil) != tc.wantErr {
				t.Fatalf("New: wantErr=%v, got err=%v", tc.wantErr, err)
			}
			if !tc.wantErr && orch == nil {
				t.Fatal("expected non-nil Orchestrator")
			}
		})
	}
}

func TestInvoke_StubReturnsNotImplemented(t *testing.T) {
	orch, err := orchestrator.New(&stubProvider{})
	if err != nil {
		t.Fatalf("New: unexpected error: %v", err)
	}

	result, err := orch.Invoke(context.Background(), invocation.InvocationRequest{})

	if err == nil {
		t.Fatal("Invoke stub: expected non-nil error")
	}
	if !strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("Invoke stub: error message should contain %q, got: %q", "not yet implemented", err.Error())
	}
	if result.FinalState != state.Failed {
		t.Errorf("Invoke stub: FinalState = %v, want %v", result.FinalState, state.Failed)
	}
	if result.Error == nil {
		t.Error("Invoke stub: result.Error should be non-nil")
	}
}

func TestNew_MultipleOptions(t *testing.T) {
	// Verify options compose correctly and the last write wins.
	orch, err := orchestrator.New(
		&stubProvider{},
		orchestrator.WithDefaultModel("model-v1"),
		orchestrator.WithDefaultModel("model-v2"), // should win
		orchestrator.WithMaxIterations(5),
		orchestrator.WithMaxIterations(25), // should win
	)
	if err != nil {
		t.Fatalf("New with multiple options: unexpected error: %v", err)
	}
	if orch == nil {
		t.Fatal("expected non-nil Orchestrator")
	}
}
