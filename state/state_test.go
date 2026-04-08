// SPDX-License-Identifier: Apache-2.0

package state

import (
	"fmt"
	"testing"
)

func TestStateCount(t *testing.T) {
	if got := Count(); got != 14 {
		t.Errorf("Count() = %d, want 14", got)
	}
}

func TestStateConstants(t *testing.T) {
	// Verify exact iota values are load-bearing.
	tests := []struct {
		state State
		want  uint8
	}{
		{Created, 0},
		{Initializing, 1},
		{PreHook, 2},
		{LLMCall, 3},
		{ToolDecision, 4},
		{ToolCall, 5},
		{PostToolFilter, 6},
		{LLMContinuation, 7},
		{PostHook, 8},
		{Completed, 9},
		{Failed, 10},
		{Cancelled, 11},
		{BudgetExceeded, 12},
		{ApprovalRequired, 13},
	}
	for _, tt := range tests {
		if got := uint8(tt.state); got != tt.want {
			t.Errorf("%s = %d, want %d", tt.state, got, tt.want)
		}
	}
}

func TestStateString(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{Created, "Created"},
		{Initializing, "Initializing"},
		{PreHook, "PreHook"},
		{LLMCall, "LLMCall"},
		{ToolDecision, "ToolDecision"},
		{ToolCall, "ToolCall"},
		{PostToolFilter, "PostToolFilter"},
		{LLMContinuation, "LLMContinuation"},
		{PostHook, "PostHook"},
		{Completed, "Completed"},
		{Failed, "Failed"},
		{Cancelled, "Cancelled"},
		{BudgetExceeded, "BudgetExceeded"},
		{ApprovalRequired, "ApprovalRequired"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("State(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestStateStringUnknown(t *testing.T) {
	unknown := State(255)
	want := "State(255)"
	if got := unknown.String(); got != want {
		t.Errorf("State(255).String() = %q, want %q", got, want)
	}
}

func TestStateStringerInterface(_ *testing.T) {
	// Verify State implements fmt.Stringer.
	var _ fmt.Stringer = Created
}

func TestIsTerminal(t *testing.T) {
	nonTerminal := []State{Created, Initializing, PreHook, LLMCall, ToolDecision, ToolCall, PostToolFilter, LLMContinuation, PostHook}
	for _, s := range nonTerminal {
		if s.IsTerminal() {
			t.Errorf("%s.IsTerminal() = true, want false", s)
		}
	}

	terminal := []State{Completed, Failed, Cancelled, BudgetExceeded, ApprovalRequired}
	for _, s := range terminal {
		if !s.IsTerminal() {
			t.Errorf("%s.IsTerminal() = false, want true", s)
		}
	}
}

func TestIsTerminalBranchlessProperty(t *testing.T) {
	// The branchless property: IsTerminal iff value >= Completed (9).
	for i := 0; i < 256; i++ {
		s := State(i) //nolint:gosec // i is bounded by 256 which fits in uint8
		got := s.IsTerminal()
		want := i >= int(Completed)
		if got != want {
			t.Errorf("State(%d).IsTerminal() = %v, want %v", i, got, want)
		}
	}
}

func TestTerminalStatesCount(t *testing.T) {
	if got := len(TerminalStates()); got != 5 {
		t.Errorf("len(TerminalStates()) = %d, want 5", got)
	}
}

func TestNonTerminalStatesCount(t *testing.T) {
	if got := len(NonTerminalStates()); got != 9 {
		t.Errorf("len(NonTerminalStates()) = %d, want 9", got)
	}
}

func TestAllStatesCount(t *testing.T) {
	all := All()
	if got := len(all); got != 14 {
		t.Errorf("len(All()) = %d, want 14", got)
	}
	for i, s := range all {
		if int(s) != i {
			t.Errorf("All()[%d] = %d, want %d", i, s, i)
		}
	}
}

func TestTerminalStatesHaveNoOutgoingEdges(t *testing.T) {
	for _, s := range TerminalStates() {
		if tr := Transitions(s); tr != nil {
			t.Errorf("Transitions(%s) = %v, want nil", s, tr)
		}
	}
}

func TestNonTerminalStatesHaveOutgoingEdges(t *testing.T) {
	for _, s := range NonTerminalStates() {
		if tr := Transitions(s); len(tr) == 0 {
			t.Errorf("Transitions(%s) has no outgoing edges", s)
		}
	}
}

func TestTransitionsCreated(t *testing.T) {
	got := Transitions(Created)
	want := []State{Initializing}
	assertStatesEqual(t, "Created", got, want)
}

func TestTransitionsInitializing(t *testing.T) {
	got := Transitions(Initializing)
	want := []State{PreHook, Failed}
	assertStatesEqual(t, "Initializing", got, want)
}

func TestTransitionsPreHook(t *testing.T) {
	got := Transitions(PreHook)
	want := []State{LLMCall, Failed, ApprovalRequired}
	assertStatesEqual(t, "PreHook", got, want)
}

func TestTransitionsLLMCall(t *testing.T) {
	got := Transitions(LLMCall)
	want := []State{ToolDecision, Failed, Cancelled, BudgetExceeded}
	assertStatesEqual(t, "LLMCall", got, want)
}

func TestTransitionsToolDecision(t *testing.T) {
	got := Transitions(ToolDecision)
	want := []State{ToolCall, PostHook, Failed, Cancelled, BudgetExceeded}
	assertStatesEqual(t, "ToolDecision", got, want)
}

func TestTransitionsToolCall(t *testing.T) {
	got := Transitions(ToolCall)
	want := []State{PostToolFilter, Failed, Cancelled}
	assertStatesEqual(t, "ToolCall", got, want)
}

func TestTransitionsPostToolFilter(t *testing.T) {
	got := Transitions(PostToolFilter)
	want := []State{LLMContinuation, Failed, Cancelled}
	assertStatesEqual(t, "PostToolFilter", got, want)
}

func TestTransitionsLLMContinuation(t *testing.T) {
	got := Transitions(LLMContinuation)
	want := []State{ToolDecision, Failed, Cancelled, BudgetExceeded}
	assertStatesEqual(t, "LLMContinuation", got, want)
}

func TestTransitionsPostHook(t *testing.T) {
	got := Transitions(PostHook)
	want := []State{Completed, Failed, ApprovalRequired, Cancelled}
	assertStatesEqual(t, "PostHook", got, want)
}

func TestTransitionsOutOfRange(t *testing.T) {
	if got := Transitions(State(255)); got != nil {
		t.Errorf("Transitions(255) = %v, want nil", got)
	}
}

func TestIsLegalTransition(t *testing.T) {
	// Spot-check legal transitions.
	legal := []struct{ from, to State }{
		{Created, Initializing},
		{Initializing, PreHook},
		{PreHook, LLMCall},
		{LLMCall, ToolDecision},
		{ToolDecision, ToolCall},
		{ToolCall, PostToolFilter},
		{PostToolFilter, LLMContinuation},
		{LLMContinuation, ToolDecision},
		{PostHook, Completed},
		{PreHook, ApprovalRequired},
		{LLMCall, BudgetExceeded},
		{ToolCall, Cancelled},
		{Initializing, Failed},
	}
	for _, tt := range legal {
		if !IsLegalTransition(tt.from, tt.to) {
			t.Errorf("IsLegalTransition(%s, %s) = false, want true", tt.from, tt.to)
		}
	}
}

func TestIsLegalTransitionIllegal(t *testing.T) {
	// Spot-check illegal transitions.
	illegal := []struct{ from, to State }{
		{Created, Completed},
		{Created, Failed},
		{Created, PreHook},
		{Completed, Failed},
		{Failed, Created},
		{Cancelled, Initializing},
		{PostHook, LLMCall},
		{ToolCall, LLMCall},
	}
	for _, tt := range illegal {
		if IsLegalTransition(tt.from, tt.to) {
			t.Errorf("IsLegalTransition(%s, %s) = true, want false", tt.from, tt.to)
		}
	}
}

func TestEveryNonTerminalCanReachFailed(t *testing.T) {
	// D16: every non-terminal state except Created can transition to Failed.
	for _, s := range NonTerminalStates() {
		if s == Created {
			continue
		}
		if !IsLegalTransition(s, Failed) {
			t.Errorf("%s cannot transition to Failed", s)
		}
	}
}

func TestFailedReachableFromCreatedViaInitializing(t *testing.T) {
	// Created -> Initializing -> Failed
	if !IsLegalTransition(Created, Initializing) {
		t.Fatal("Created -> Initializing should be legal")
	}
	if !IsLegalTransition(Initializing, Failed) {
		t.Fatal("Initializing -> Failed should be legal")
	}
}

func assertStatesEqual(t *testing.T, name string, got, want []State) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("Transitions(%s): got %d states, want %d", name, len(got), len(want))
		return
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("Transitions(%s)[%d] = %s, want %s", name, i, got[i], w)
		}
	}
}

func BenchmarkIsTerminal(b *testing.B) {
	states := All()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = states[i%len(states)].IsTerminal()
	}
}

func BenchmarkIsLegalTransition(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = IsLegalTransition(LLMContinuation, ToolDecision)
	}
}

func BenchmarkString(b *testing.B) {
	states := All()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = states[i%len(states)].String()
	}
}
