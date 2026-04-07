// SPDX-License-Identifier: Apache-2.0

package invocation_test

import (
	"errors"
	"testing"

	"github.com/praxis-os/praxis/invocation"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/state"
)

func TestInvocationResult_ZeroValue(t *testing.T) {
	// The zero value must be constructable without panicking.
	var result invocation.InvocationResult

	if result.FinalState != state.Created {
		t.Errorf("zero-value FinalState: want %v, got %v", state.Created, result.FinalState)
	}
	if result.Iterations != 0 {
		t.Errorf("zero-value Iterations: want 0, got %d", result.Iterations)
	}
	if result.Error != nil {
		t.Errorf("zero-value Error: want nil, got %v", result.Error)
	}
}

func TestInvocationResult_SuccessfulCompletion(t *testing.T) {
	resp := llm.LLMResponse{
		Message:    llm.Message{Role: llm.RoleAssistant, Parts: []llm.MessagePart{llm.TextPart("done")}},
		StopReason: llm.StopReasonEndTurn,
		Usage: llm.TokenUsage{
			InputTokens:  100,
			OutputTokens: 50,
		},
	}

	result := invocation.InvocationResult{
		Response:   resp,
		FinalState: state.Completed,
		Iterations: 1,
		TokenUsage: invocation.TokenUsage{
			InputTokens:  100,
			OutputTokens: 50,
			TotalTokens:  150,
		},
		Error: nil,
	}

	if result.FinalState != state.Completed {
		t.Errorf("FinalState: want %v, got %v", state.Completed, result.FinalState)
	}
	if !result.FinalState.IsTerminal() {
		t.Errorf("FinalState %v should be terminal", result.FinalState)
	}
	if result.Iterations != 1 {
		t.Errorf("Iterations: want 1, got %d", result.Iterations)
	}
	if result.TokenUsage.InputTokens != 100 {
		t.Errorf("TokenUsage.InputTokens: want 100, got %d", result.TokenUsage.InputTokens)
	}
	if result.TokenUsage.OutputTokens != 50 {
		t.Errorf("TokenUsage.OutputTokens: want 50, got %d", result.TokenUsage.OutputTokens)
	}
	if result.TokenUsage.TotalTokens != 150 {
		t.Errorf("TokenUsage.TotalTokens: want 150, got %d", result.TokenUsage.TotalTokens)
	}
	if result.Error != nil {
		t.Errorf("Error: want nil, got %v", result.Error)
	}
}

func TestInvocationResult_FailedState(t *testing.T) {
	sentinel := errors.New("provider unavailable")

	result := invocation.InvocationResult{
		FinalState: state.Failed,
		Iterations: 2,
		Error:      sentinel,
	}

	if result.FinalState != state.Failed {
		t.Errorf("FinalState: want %v, got %v", state.Failed, result.FinalState)
	}
	if !result.FinalState.IsTerminal() {
		t.Errorf("FinalState %v should be terminal", result.FinalState)
	}
	if !errors.Is(result.Error, sentinel) {
		t.Errorf("Error: want sentinel, got %v", result.Error)
	}
}

func TestInvocationResult_AllTerminalStates(t *testing.T) {
	terminalStates := []state.State{
		state.Completed,
		state.Failed,
		state.Cancelled,
		state.BudgetExceeded,
		state.ApprovalRequired,
	}

	for _, s := range terminalStates {
		t.Run(s.String(), func(t *testing.T) {
			result := invocation.InvocationResult{FinalState: s}
			if !result.FinalState.IsTerminal() {
				t.Errorf("FinalState %v should be terminal", result.FinalState)
			}
		})
	}
}

func TestTokenUsage_ZeroValue(t *testing.T) {
	var u invocation.TokenUsage

	if u.InputTokens != 0 {
		t.Errorf("zero-value InputTokens: want 0, got %d", u.InputTokens)
	}
	if u.OutputTokens != 0 {
		t.Errorf("zero-value OutputTokens: want 0, got %d", u.OutputTokens)
	}
	if u.TotalTokens != 0 {
		t.Errorf("zero-value TotalTokens: want 0, got %d", u.TotalTokens)
	}
}

func TestTokenUsage_MultiIteration(t *testing.T) {
	// Simulate aggregating token usage across 3 LLM round-trips.
	calls := []llm.TokenUsage{
		{InputTokens: 100, OutputTokens: 40},
		{InputTokens: 150, OutputTokens: 60},
		{InputTokens: 200, OutputTokens: 80},
	}

	var agg invocation.TokenUsage
	for _, c := range calls {
		agg.InputTokens += c.InputTokens
		agg.OutputTokens += c.OutputTokens
		agg.TotalTokens += c.InputTokens + c.OutputTokens
	}

	if agg.InputTokens != 450 {
		t.Errorf("aggregated InputTokens: want 450, got %d", agg.InputTokens)
	}
	if agg.OutputTokens != 180 {
		t.Errorf("aggregated OutputTokens: want 180, got %d", agg.OutputTokens)
	}
	if agg.TotalTokens != 630 {
		t.Errorf("aggregated TotalTokens: want 630, got %d", agg.TotalTokens)
	}
}
