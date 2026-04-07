// SPDX-License-Identifier: Apache-2.0

// Package state — invariant tests for the 21 state machine invariants defined in
// docs/phase-2-core-runtime/06-state-machine-invariants.md.
//
// Tests are named TestInvariant_INVnn_<slug> where nn is the two-digit invariant
// number.
//
// Group A (INV-01 to INV-10): fully implemented — structural/transition-table
// verification.
// Group B (INV-11 to INV-17): skipped — requires InvokeStream (v0.3.0).
// Group C (INV-18 to INV-21): skipped — requires terminal event types (v0.3.0).
package state

import (
	"errors"
	"testing"
)

// ---------------------------------------------------------------------------
// Graph helpers used across multiple invariant tests.
// ---------------------------------------------------------------------------

// allAcyclicPaths returns every acyclic path from `start` to `target` through
// the D16 adjacency table. Paths are represented as state slices including both
// endpoints. Cycles are broken by tracking visited sets per path.
func allAcyclicPaths(start, target State) [][]State {
	var results [][]State
	var dfs func(current State, path []State, visited map[State]bool)
	dfs = func(current State, path []State, visited map[State]bool) {
		if current == target {
			cp := make([]State, len(path))
			copy(cp, path)
			results = append(results, cp)
			return
		}
		for _, next := range Transitions(current) {
			if visited[next] {
				continue
			}
			visited[next] = true
			dfs(next, append(path, next), visited)
			delete(visited, next)
		}
	}
	visited := map[State]bool{start: true}
	dfs(start, []State{start}, visited)
	return results
}

// incomingEdges builds a map of dst → []src for all edges in the D16 table.
func incomingEdges() map[State][]State {
	inc := make(map[State][]State, Count())
	for _, src := range All() {
		for _, dst := range Transitions(src) {
			inc[dst] = append(inc[dst], src)
		}
	}
	return inc
}

// pathContains reports whether target appears in path.
func pathContains(path []State, target State) bool {
	for _, s := range path {
		if s == target {
			return true
		}
	}
	return false
}

// countOccurrences counts how many times target appears in path.
func countOccurrences(path []State, target State) int {
	n := 0
	for _, s := range path {
		if s == target {
			n++
		}
	}
	return n
}

// ---------------------------------------------------------------------------
// Group A — State machine structural invariants
// ---------------------------------------------------------------------------

// TestInvariant_INV01_TransitionAllowList verifies that:
//  1. Every state accepted by Machine.Transition is listed in the D16 table.
//  2. Any transition not in the allow-list returns a *TransitionError and
//     leaves the machine state unchanged.
//
// This is an exhaustive structural check: for every non-terminal source state
// we iterate all 14 states, assert legal ones succeed and illegal ones fail
// without mutation.
func TestInvariant_INV01_TransitionAllowList(t *testing.T) {
	for _, src := range NonTerminalStates() {
		src := src
		t.Run("from_"+src.String(), func(t *testing.T) {
			legalSet := make(map[State]bool)
			for _, dst := range Transitions(src) {
				legalSet[dst] = true
			}

			for _, dst := range All() {
				dst := dst
				m := reachState(t, src)

				err := m.Transition(dst)
				if legalSet[dst] {
					// Legal: must succeed and advance state.
					if err != nil {
						t.Errorf("INV-01: legal Transition(%s→%s) returned error: %v", src, dst, err)
					}
					if m.State() != dst {
						t.Errorf("INV-01: after Transition(%s→%s) state = %s, want %s", src, dst, m.State(), dst)
					}
				} else {
					// Illegal: must return *TransitionError and leave state unchanged.
					if err == nil {
						t.Errorf("INV-01: illegal Transition(%s→%s) succeeded", src, dst)
						continue
					}
					var te *TransitionError
					if !errors.As(err, &te) {
						t.Errorf("INV-01: illegal Transition(%s→%s) returned %T, want *TransitionError", src, dst, err)
					}
					if m.State() != src {
						t.Errorf("INV-01: state mutated after rejected Transition(%s→%s): got %s", src, dst, m.State())
					}
				}
			}
		})
	}
}

// TestInvariant_INV02_TerminalImmutability verifies that once a Machine enters
// any terminal state, every subsequent Transition call returns a *TransitionError
// and the state does not change.
func TestInvariant_INV02_TerminalImmutability(t *testing.T) {
	for _, terminal := range TerminalStates() {
		terminal := terminal
		t.Run(terminal.String(), func(t *testing.T) {
			m := reachState(t, terminal)
			countBefore := m.TransitionCount()

			for _, next := range All() {
				next := next
				err := m.Transition(next)
				if err == nil {
					t.Errorf("INV-02: Transition(%s→%s) succeeded from terminal state", terminal, next)
					continue
				}
				var te *TransitionError
				if !errors.As(err, &te) {
					t.Errorf("INV-02: Transition(%s→%s) returned %T, want *TransitionError", terminal, next, err)
				}
				if te.Msg != "terminal state is immutable" {
					t.Errorf("INV-02: TransitionError.Msg = %q, want %q", te.Msg, "terminal state is immutable")
				}
				if m.State() != terminal {
					t.Errorf("INV-02: state changed after rejected transition: got %s, want %s", m.State(), terminal)
				}
				if m.TransitionCount() != countBefore {
					t.Errorf("INV-02: TransitionCount changed after rejected transition: %d → %d",
						countBefore, m.TransitionCount())
				}
			}
		})
	}
}

// TestInvariant_INV03_CompletedPathVisitsMandatoryStates verifies that every
// acyclic path from Created to Completed visits PreHook, LLMCall, and PostHook
// at least once.
func TestInvariant_INV03_CompletedPathVisitsMandatoryStates(t *testing.T) {
	paths := allAcyclicPaths(Created, Completed)
	if len(paths) == 0 {
		t.Fatal("INV-03: no paths found from Created to Completed")
	}

	mandatory := []State{PreHook, LLMCall, PostHook}

	for i, path := range paths {
		for _, required := range mandatory {
			if !pathContains(path, required) {
				t.Errorf("INV-03: path %d to Completed missing %s: %v", i, required, path)
			}
		}
	}
}

// TestInvariant_INV04_PreHookVisitedExactlyOnce verifies two properties:
//  1. Every acyclic path from Created to Completed contains PreHook exactly once.
//  2. No state that is reachable after PreHook has PreHook as an allowed next
//     transition — meaning the transition table structurally prevents re-entry.
func TestInvariant_INV04_PreHookVisitedExactlyOnce(t *testing.T) {
	// Part 1: path enumeration.
	paths := allAcyclicPaths(Created, Completed)
	for i, path := range paths {
		n := countOccurrences(path, PreHook)
		if n != 1 {
			t.Errorf("INV-04: path %d to Completed contains PreHook %d times (want 1): %v", i, n, path)
		}
	}

	// Part 2: verify the transition table has no edge back to PreHook from any
	// state that can be visited after PreHook. States reachable after PreHook
	// are those reachable from LLMCall (the only non-error, non-terminal exit
	// from PreHook).
	statesAfterPreHook := []State{
		LLMCall, ToolDecision, ToolCall, PostToolFilter, LLMContinuation, PostHook,
	}
	for _, src := range statesAfterPreHook {
		for _, dst := range Transitions(src) {
			if dst == PreHook {
				t.Errorf("INV-04: transition table has edge %s→PreHook, allowing re-entry", src)
			}
		}
	}
}

// TestInvariant_INV05_LLMCallRequiresInitializingAndPreHook verifies that
// LLMCall is only reachable from PreHook in the transition table. No other
// source state has LLMCall as an allowed next state.
func TestInvariant_INV05_LLMCallPrecedence(t *testing.T) {
	inc := incomingEdges()
	sources := inc[LLMCall]

	if len(sources) != 1 {
		t.Fatalf("INV-05: LLMCall has %d incoming edges, want exactly 1: incoming = %v", len(sources), sources)
	}
	if sources[0] != PreHook {
		t.Errorf("INV-05: LLMCall's only incoming edge is from %s, want PreHook", sources[0])
	}

	// Also exhaustively verify: for every state that is not PreHook,
	// LLMCall is not in its allow-list.
	for _, src := range All() {
		if src == PreHook {
			continue
		}
		for _, dst := range Transitions(src) {
			if dst == LLMCall {
				t.Errorf("INV-05: %s has edge to LLMCall but is not PreHook", src)
			}
		}
	}
}

// TestInvariant_INV06_TerminalStateSetIsExact verifies that the set of terminal
// states is exactly {Completed, Failed, Cancelled, BudgetExceeded,
// ApprovalRequired} — no more, no fewer.
func TestInvariant_INV06_TerminalStateSetIsExact(t *testing.T) {
	want := map[State]bool{
		Completed:        true,
		Failed:           true,
		Cancelled:        true,
		BudgetExceeded:   true,
		ApprovalRequired: true,
	}

	// Every state in TerminalStates() must be in want.
	for _, s := range TerminalStates() {
		if !want[s] {
			t.Errorf("INV-06: TerminalStates() contains unexpected state %s", s)
		}
	}

	// Every state in want must appear in TerminalStates().
	termSet := make(map[State]bool, len(TerminalStates()))
	for _, s := range TerminalStates() {
		termSet[s] = true
	}
	for s := range want {
		if !termSet[s] {
			t.Errorf("INV-06: expected terminal state %s missing from TerminalStates()", s)
		}
	}

	// Total count must be exactly 5.
	if got := len(TerminalStates()); got != 5 {
		t.Errorf("INV-06: len(TerminalStates()) = %d, want 5", got)
	}

	// Non-terminal states must not appear in TerminalStates().
	for _, s := range NonTerminalStates() {
		if termSet[s] {
			t.Errorf("INV-06: non-terminal state %s appears in TerminalStates()", s)
		}
	}

	// IsTerminal() must agree with TerminalStates() for all 14 defined states.
	for _, s := range All() {
		if s.IsTerminal() != termSet[s] {
			t.Errorf("INV-06: %s.IsTerminal() = %v but termSet[s] = %v", s, s.IsTerminal(), termSet[s])
		}
	}
}

// TestInvariant_INV07_ApprovalRequiredSources verifies that ApprovalRequired is
// reachable only from PreHook or PostHook. No other source state has a legal
// edge to ApprovalRequired.
func TestInvariant_INV07_ApprovalRequiredSources(t *testing.T) {
	allowed := map[State]bool{PreHook: true, PostHook: true}
	inc := incomingEdges()

	for _, src := range inc[ApprovalRequired] {
		if !allowed[src] {
			t.Errorf("INV-07: ApprovalRequired reachable from %s, want only PreHook or PostHook", src)
		}
	}

	// Exhaustive: for every state not in allowed, ApprovalRequired must not
	// appear in its transition list.
	for _, src := range All() {
		if allowed[src] {
			continue
		}
		for _, dst := range Transitions(src) {
			if dst == ApprovalRequired {
				t.Errorf("INV-07: %s→ApprovalRequired edge exists but %s is not PreHook or PostHook", src, src)
			}
		}
	}

	// Sanity: both PreHook and PostHook must have ApprovalRequired as a valid next.
	for src := range allowed {
		if !IsLegalTransition(src, ApprovalRequired) {
			t.Errorf("INV-07: expected %s→ApprovalRequired to be legal but IsLegalTransition returned false", src)
		}
	}
}

// TestInvariant_INV08_BudgetExceededSources verifies that BudgetExceeded is
// reachable only from LLMCall, ToolDecision, or LLMContinuation.
func TestInvariant_INV08_BudgetExceededSources(t *testing.T) {
	allowed := map[State]bool{
		LLMCall:         true,
		ToolDecision:    true,
		LLMContinuation: true,
	}
	inc := incomingEdges()

	for _, src := range inc[BudgetExceeded] {
		if !allowed[src] {
			t.Errorf("INV-08: BudgetExceeded reachable from %s, want only LLMCall/ToolDecision/LLMContinuation", src)
		}
	}

	// Exhaustive: for every state not in allowed, BudgetExceeded must not appear.
	for _, src := range All() {
		if allowed[src] {
			continue
		}
		for _, dst := range Transitions(src) {
			if dst == BudgetExceeded {
				t.Errorf("INV-08: unexpected edge %s→BudgetExceeded", src)
			}
		}
	}

	// Sanity: each allowed source must have the edge.
	for src := range allowed {
		if !IsLegalTransition(src, BudgetExceeded) {
			t.Errorf("INV-08: %s→BudgetExceeded should be legal but IsLegalTransition returned false", src)
		}
	}
}

// TestInvariant_INV09_CancelledNotReachableFromEarlyStates verifies that
// Cancelled is not reachable from Created, Initializing, PreHook, or any
// terminal state.
func TestInvariant_INV09_CancelledNotReachableFromForbiddenSources(t *testing.T) {
	forbidden := []State{Created, Initializing, PreHook}
	for _, src := range TerminalStates() {
		forbidden = append(forbidden, src)
	}

	for _, src := range forbidden {
		src := src
		t.Run("from_"+src.String(), func(t *testing.T) {
			for _, dst := range Transitions(src) {
				if dst == Cancelled {
					t.Errorf("INV-09: %s→Cancelled edge exists, want forbidden", src)
				}
			}
			if IsLegalTransition(src, Cancelled) {
				t.Errorf("INV-09: IsLegalTransition(%s, Cancelled) = true, want false", src)
			}
		})
	}

	// Cross-check: states from which Cancelled IS reachable.
	allowedSources := map[State]bool{
		LLMCall: true, ToolDecision: true, ToolCall: true,
		PostToolFilter: true, LLMContinuation: true, PostHook: true,
	}
	inc := incomingEdges()
	for _, src := range inc[Cancelled] {
		if !allowedSources[src] {
			t.Errorf("INV-09: Cancelled is reachable from unexpected source %s", src)
		}
	}
}

// TestInvariant_INV10_ToolCycleRepeatsArbitrarily verifies that the tool cycle
// ToolDecision→ToolCall→PostToolFilter→LLMContinuation→ToolDecision may repeat
// any finite number of times. We test N = 1, 5, 10, 50, 100 cycles, each
// followed by ToolDecision→PostHook→Completed to confirm Machine accepts the
// full walk.
func TestInvariant_INV10_ToolCycleRepeatsArbitrarily(t *testing.T) {
	cycleCounts := []int{1, 5, 10, 50, 100}

	for _, n := range cycleCounts {
		n := n
		t.Run("cycles_"+itoa(n), func(t *testing.T) {
			m := NewMachine()

			// Reach ToolDecision via the shortest prefix.
			prefix := []State{Initializing, PreHook, LLMCall, ToolDecision}
			for i, s := range prefix {
				if err := m.Transition(s); err != nil {
					t.Fatalf("cycle_count=%d prefix step %d Transition(%s): %v", n, i, s, err)
				}
			}

			// Repeat the tool cycle n times.
			cycle := []State{ToolCall, PostToolFilter, LLMContinuation, ToolDecision}
			for i := 0; i < n; i++ {
				for _, s := range cycle {
					if err := m.Transition(s); err != nil {
						t.Fatalf("cycle_count=%d cycle %d Transition(%s): %v", n, i+1, s, err)
					}
				}
			}

			// Finish via PostHook→Completed.
			suffix := []State{PostHook, Completed}
			for _, s := range suffix {
				if err := m.Transition(s); err != nil {
					t.Fatalf("cycle_count=%d suffix Transition(%s): %v", n, s, err)
				}
			}

			if m.State() != Completed {
				t.Errorf("INV-10: final state = %s, want Completed", m.State())
			}

			// Expected transition count:
			// prefix(4) + cycles(n*4) + suffix(2)
			want := uint64(4 + n*4 + 2)
			if m.TransitionCount() != want {
				t.Errorf("INV-10: TransitionCount = %d, want %d", m.TransitionCount(), want)
			}
		})
	}
}

// itoa converts an int to a decimal string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}

// ---------------------------------------------------------------------------
// Group B — Streaming event invariants (v0.3.0)
// ---------------------------------------------------------------------------

// TestInvariant_INV11_StreamBeginsWithInvocationStarted skips until InvokeStream exists.
func TestInvariant_INV11_StreamBeginsWithInvocationStarted(t *testing.T) {
	t.Skip("requires InvokeStream/event system (v0.3.0)")
}

// TestInvariant_INV12_StreamEndsWithExactlyOneTerminalEvent skips until InvokeStream exists.
func TestInvariant_INV12_StreamEndsWithExactlyOneTerminalEvent(t *testing.T) {
	t.Skip("requires InvokeStream/event system (v0.3.0)")
}

// TestInvariant_INV13_ChannelClosedExactlyOnce skips until InvokeStream exists.
func TestInvariant_INV13_ChannelClosedExactlyOnce(t *testing.T) {
	t.Skip("requires InvokeStream/event system (v0.3.0)")
}

// TestInvariant_INV14_ToolCallStartedHasMatchingCompleted skips until InvokeStream exists.
func TestInvariant_INV14_ToolCallStartedHasMatchingCompleted(t *testing.T) {
	t.Skip("requires InvokeStream/event system (v0.3.0)")
}

// TestInvariant_INV15_ToolCycleEventsOrdered skips until InvokeStream exists.
func TestInvariant_INV15_ToolCycleEventsOrdered(t *testing.T) {
	t.Skip("requires InvokeStream/event system (v0.3.0)")
}

// TestInvariant_INV16_PreHookEventsAppearExactlyOnce skips until InvokeStream exists.
func TestInvariant_INV16_PreHookEventsAppearExactlyOnce(t *testing.T) {
	t.Skip("requires InvokeStream/event system (v0.3.0)")
}

// TestInvariant_INV17_ZeroWiringPathProducesExactlyTenEvents skips until InvokeStream exists.
func TestInvariant_INV17_ZeroWiringPathProducesExactlyTenEvents(t *testing.T) {
	t.Skip("requires InvokeStream/event system (v0.3.0)")
}

// ---------------------------------------------------------------------------
// Group C — Terminal path invariants (v0.3.0)
// ---------------------------------------------------------------------------

// TestInvariant_INV18_EveryTerminalPathEmitsExactlyOneTerminalEvent skips until
// telemetry.LifecycleEventEmitter exists.
func TestInvariant_INV18_EveryTerminalPathEmitsExactlyOneTerminalEvent(t *testing.T) {
	t.Skip("requires InvokeStream/event system (v0.3.0)")
}

// TestInvariant_INV19_ApprovalRequiredEventCarriesRequiredFields skips until
// the event type system exists.
func TestInvariant_INV19_ApprovalRequiredEventCarriesRequiredFields(t *testing.T) {
	t.Skip("requires InvokeStream/event system (v0.3.0)")
}

// TestInvariant_INV20_BudgetExceededEventCarriesDimensionField skips until
// the budget event type exists.
func TestInvariant_INV20_BudgetExceededEventCarriesDimensionField(t *testing.T) {
	t.Skip("requires InvokeStream/event system (v0.3.0)")
}

// TestInvariant_INV21_InvocationFailedEventCarriesTypedError skips until
// the typed error taxonomy (Phase 3/4) exists.
func TestInvariant_INV21_InvocationFailedEventCarriesTypedError(t *testing.T) {
	t.Skip("requires InvokeStream/event system (v0.3.0)")
}
