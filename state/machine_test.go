// SPDX-License-Identifier: Apache-2.0

package state

import (
	"errors"
	"testing"
)

func TestNewMachine(t *testing.T) {
	m := NewMachine()
	if m.State() != Created {
		t.Errorf("NewMachine().State() = %s, want Created", m.State())
	}
	if m.TransitionCount() != 0 {
		t.Errorf("NewMachine().TransitionCount() = %d, want 0", m.TransitionCount())
	}
}

func TestMachineHappyPath(t *testing.T) {
	// Walk the shortest path: Created -> Initializing -> PreHook -> LLMCall ->
	// ToolDecision -> PostHook -> Completed.
	m := NewMachine()
	steps := []State{Initializing, PreHook, LLMCall, ToolDecision, PostHook, Completed}
	for i, next := range steps {
		if err := m.Transition(next); err != nil {
			t.Fatalf("step %d: Transition(%s) = %v", i, next, err)
		}
	}
	if m.State() != Completed {
		t.Errorf("final state = %s, want Completed", m.State())
	}
	if m.TransitionCount() != uint64(len(steps)) {
		t.Errorf("TransitionCount() = %d, want %d", m.TransitionCount(), len(steps))
	}
}

func TestMachineToolLoop(t *testing.T) {
	// Created -> Initializing -> PreHook -> LLMCall -> ToolDecision ->
	// ToolCall -> PostToolFilter -> LLMContinuation -> ToolDecision ->
	// PostHook -> Completed.
	m := NewMachine()
	steps := []State{
		Initializing, PreHook, LLMCall, ToolDecision,
		ToolCall, PostToolFilter, LLMContinuation, ToolDecision,
		PostHook, Completed,
	}
	for i, next := range steps {
		if err := m.Transition(next); err != nil {
			t.Fatalf("step %d: Transition(%s) = %v", i, next, err)
		}
	}
	if m.State() != Completed {
		t.Errorf("final state = %s, want Completed", m.State())
	}
}

func TestMachineIllegalTransition(t *testing.T) {
	m := NewMachine()
	err := m.Transition(Completed)
	if err == nil {
		t.Fatal("Transition(Completed) from Created should fail")
	}
	var te *TransitionError
	if !errors.As(err, &te) {
		t.Fatalf("error type = %T, want *TransitionError", err)
	}
	if te.From != Created || te.To != Completed {
		t.Errorf("TransitionError = {From: %s, To: %s}, want {Created, Completed}", te.From, te.To)
	}
	if te.Msg != "illegal transition" {
		t.Errorf("Msg = %q, want %q", te.Msg, "illegal transition")
	}
	// State should not have changed.
	if m.State() != Created {
		t.Errorf("state after illegal transition = %s, want Created", m.State())
	}
	if m.TransitionCount() != 0 {
		t.Errorf("TransitionCount() = %d, want 0", m.TransitionCount())
	}
}

func TestMachineTerminalImmutability(t *testing.T) {
	terminals := TerminalStates()
	for _, terminal := range terminals {
		t.Run(terminal.String(), func(t *testing.T) {
			m := reachTerminal(t, terminal)

			// All subsequent transitions must fail.
			for _, next := range All() {
				err := m.Transition(next)
				if err == nil {
					t.Errorf("Transition(%s) from terminal %s should fail", next, terminal)
				}
				var te *TransitionError
				if !errors.As(err, &te) {
					t.Fatalf("error type = %T, want *TransitionError", err)
				}
				if te.Msg != "terminal state is immutable" {
					t.Errorf("Msg = %q, want %q", te.Msg, "terminal state is immutable")
				}
			}

			// State must not have changed.
			if m.State() != terminal {
				t.Errorf("state after rejected transitions = %s, want %s", m.State(), terminal)
			}
		})
	}
}

func TestMachineTerminalTransitionCountFrozen(t *testing.T) {
	m := reachTerminal(t, Completed)
	countBefore := m.TransitionCount()

	_ = m.Transition(Failed) // should fail

	if m.TransitionCount() != countBefore {
		t.Errorf("TransitionCount changed after rejected transition: %d -> %d",
			countBefore, m.TransitionCount())
	}
}

func TestMachineFailedFromEveryNonTerminal(t *testing.T) {
	// Every non-terminal state except Created can transition to Failed.
	for _, s := range NonTerminalStates() {
		if s == Created {
			continue
		}
		t.Run(s.String(), func(t *testing.T) {
			m := reachState(t, s)
			if err := m.Transition(Failed); err != nil {
				t.Errorf("Transition(Failed) from %s = %v", s, err)
			}
		})
	}
}

func TestMachineCancelledFromLLMCallOnward(t *testing.T) {
	cancellable := []State{LLMCall, ToolDecision, ToolCall, PostToolFilter, LLMContinuation, PostHook}
	for _, s := range cancellable {
		t.Run(s.String(), func(t *testing.T) {
			m := reachState(t, s)
			if err := m.Transition(Cancelled); err != nil {
				t.Errorf("Transition(Cancelled) from %s = %v", s, err)
			}
		})
	}
}

func TestMachineBudgetExceededStates(t *testing.T) {
	budgetStates := []State{LLMCall, ToolDecision, LLMContinuation}
	for _, s := range budgetStates {
		t.Run(s.String(), func(t *testing.T) {
			m := reachState(t, s)
			if err := m.Transition(BudgetExceeded); err != nil {
				t.Errorf("Transition(BudgetExceeded) from %s = %v", s, err)
			}
		})
	}
}

func TestMachineApprovalRequiredStates(t *testing.T) {
	approvalStates := []State{PreHook, PostHook}
	for _, s := range approvalStates {
		t.Run(s.String(), func(t *testing.T) {
			m := reachState(t, s)
			if err := m.Transition(ApprovalRequired); err != nil {
				t.Errorf("Transition(ApprovalRequired) from %s = %v", s, err)
			}
		})
	}
}

func TestTransitionErrorMessage(t *testing.T) {
	err := &TransitionError{From: Created, To: Completed, Msg: "illegal transition"}
	want := "state: illegal transition: from Created to Completed"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestTransitionErrorAs(t *testing.T) {
	m := NewMachine()
	err := m.Transition(Completed)

	var te *TransitionError
	if !errors.As(err, &te) {
		t.Fatal("errors.As should match *TransitionError")
	}
}

func BenchmarkMachineTransition(b *testing.B) {
	// Benchmark a full happy-path cycle.
	steps := []State{Initializing, PreHook, LLMCall, ToolDecision, PostHook, Completed}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := NewMachine()
		for _, s := range steps {
			_ = m.Transition(s)
		}
	}
}

func BenchmarkMachineTransitionReject(b *testing.B) {
	m := NewMachine()
	_ = m.Transition(Initializing)
	_ = m.Transition(PreHook)
	_ = m.Transition(LLMCall)
	_ = m.Transition(ToolDecision)
	_ = m.Transition(PostHook)
	_ = m.Transition(Completed)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.Transition(Failed) // always rejected (terminal)
	}
}

// reachTerminal drives a new Machine to the given terminal state.
func reachTerminal(t *testing.T, terminal State) *Machine {
	t.Helper()
	m := reachState(t, terminal)
	if !m.State().IsTerminal() {
		t.Fatalf("reachTerminal: %s is not terminal", terminal)
	}
	return m
}

// reachState drives a new Machine to the given state via the shortest legal path.
func reachState(t *testing.T, target State) *Machine {
	t.Helper()
	m := NewMachine()
	path := shortestPath(target)
	if path == nil {
		t.Fatalf("no path to %s", target)
	}
	for _, next := range path {
		if err := m.Transition(next); err != nil {
			t.Fatalf("reachState(%s): Transition(%s) from %s: %v", target, next, m.State(), err)
		}
	}
	if m.State() != target {
		t.Fatalf("reachState: ended at %s, want %s", m.State(), target)
	}
	return m
}

// shortestPath returns the transition sequence from Created to reach the target.
// Uses BFS over the adjacency table.
func shortestPath(target State) []State {
	if target == Created {
		return []State{}
	}
	type node struct {
		state State
		path  []State
	}
	visited := make(map[State]bool)
	queue := []node{{state: Created, path: nil}}
	visited[Created] = true

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		for _, next := range Transitions(curr.state) {
			if visited[next] {
				continue
			}
			newPath := make([]State, len(curr.path)+1)
			copy(newPath, curr.path)
			newPath[len(curr.path)] = next
			if next == target {
				return newPath
			}
			visited[next] = true
			queue = append(queue, node{state: next, path: newPath})
		}
	}
	return nil
}
