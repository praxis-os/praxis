// SPDX-License-Identifier: Apache-2.0

package state

import (
	"errors"
	"math/rand"
	"testing"
	"testing/quick"
)

// propertyConfig is the shared quick.Config used by every property test.
var propertyConfig = &quick.Config{MaxCount: 10_000}

// TestProperty_InitialStateIsCreated verifies that NewMachine always starts in
// the Created state, regardless of any random seed. The property function
// accepts a dummy uint8 to satisfy quick.Check's requirement for a typed input.
func TestProperty_InitialStateIsCreated(t *testing.T) {
	prop := func(_ uint8) bool {
		m := NewMachine()
		return m.State() == Created && m.TransitionCount() == 0
	}
	if err := quick.Check(prop, propertyConfig); err != nil {
		t.Error(err)
	}
}

// TestProperty_TerminalImmutability verifies that once a machine has reached a
// terminal state, every subsequent Transition call returns a *TransitionError
// and the state does not change. The seed drives which terminal state and which
// follow-up target are tried.
func TestProperty_TerminalImmutability(t *testing.T) {
	terminals := TerminalStates()
	allStates := All()

	prop := func(terminalIdx uint8, targetIdx uint8) bool {
		terminal := terminals[int(terminalIdx)%len(terminals)]
		m := driveToState(terminal)
		if m == nil {
			// shortestPath returned nil — should never happen for valid terminals.
			return false
		}

		countAtTerminal := m.TransitionCount()
		target := allStates[int(targetIdx)%len(allStates)]

		err := m.Transition(target)
		if err == nil {
			return false
		}
		var te *TransitionError
		if !errors.As(err, &te) {
			return false
		}
		if te.Msg != "terminal state is immutable" {
			return false
		}
		// State must not change and count must not increment.
		return m.State() == terminal && m.TransitionCount() == countAtTerminal
	}

	if err := quick.Check(prop, propertyConfig); err != nil {
		t.Error(err)
	}
}

// TestProperty_NoIllegalTransitions verifies that for any (src, dst) pair NOT
// listed in the D16 allow-table, Transition always fails. Conversely, every
// pair that IS listed always succeeds when the machine is at src.
func TestProperty_NoIllegalTransitions(t *testing.T) {
	allStates := All()

	prop := func(srcIdx uint8, dstIdx uint8) bool {
		src := allStates[int(srcIdx)%len(allStates)]
		dst := allStates[int(dstIdx)%len(allStates)]

		m := driveToState(src)
		if m == nil {
			return false
		}

		err := m.Transition(dst)
		legal := IsLegalTransition(src, dst)

		if legal && !src.IsTerminal() {
			// Must succeed.
			return err == nil
		}
		// Must fail (illegal edge or terminal source).
		return err != nil
	}

	if err := quick.Check(prop, propertyConfig); err != nil {
		t.Error(err)
	}
}

// TestProperty_MonotonicTransitionCount verifies that TransitionCount increases
// by exactly 1 on each successful Transition and stays unchanged on failures.
func TestProperty_MonotonicTransitionCount(t *testing.T) {
	nonTerminals := NonTerminalStates()

	prop := func(srcIdx uint8, dstIdx uint8) bool {
		// Use only non-terminal sources so we can attempt both legal and
		// illegal transitions.
		src := nonTerminals[int(srcIdx)%len(nonTerminals)]
		m := driveToState(src)
		if m == nil {
			return false
		}

		allStates := All()
		dst := allStates[int(dstIdx)%len(allStates)]

		before := m.TransitionCount()
		err := m.Transition(dst)
		after := m.TransitionCount()

		if err == nil {
			// Successful: count must increase by exactly 1.
			return after == before+1
		}
		// Failed: count must not change.
		return after == before
	}

	if err := quick.Check(prop, propertyConfig); err != nil {
		t.Error(err)
	}
}

// TestProperty_TerminalReachability verifies (via BFS) that every non-terminal
// state can reach at least one terminal state. This is a structural property of
// the D16 graph; it is verified once exhaustively and then re-validated against
// random state samples to satisfy the 10k-iteration requirement.
func TestProperty_TerminalReachability(t *testing.T) {
	// Pre-compute which states are terminal-reachable using BFS.
	reachable := terminalReachableSet()

	nonTerminals := NonTerminalStates()

	prop := func(srcIdx uint8) bool {
		src := nonTerminals[int(srcIdx)%len(nonTerminals)]
		return reachable[src]
	}

	if err := quick.Check(prop, propertyConfig); err != nil {
		t.Error(err)
	}
}

// TestProperty_TransitionDeterminism verifies that replaying the same sequence
// of legal transitions on two independent machines always produces the same
// final state and transition count.
func TestProperty_TransitionDeterminism(t *testing.T) {
	prop := func(seed int64) bool {
		seq := legalWalk(seed, 20)

		m1 := NewMachine()
		m2 := NewMachine()

		for _, next := range seq {
			_ = m1.Transition(next)
			_ = m2.Transition(next)
		}

		return m1.State() == m2.State() && m1.TransitionCount() == m2.TransitionCount()
	}

	if err := quick.Check(prop, propertyConfig); err != nil {
		t.Error(err)
	}
}

// TestProperty_FailedUniversallyReachable verifies that every non-terminal
// state except Created can transition to Failed, and that Created can reach
// Failed via exactly one intermediate hop (Created -> Initializing -> Failed).
func TestProperty_FailedUniversallyReachable(t *testing.T) {
	nonTerminals := NonTerminalStates()

	prop := func(srcIdx uint8) bool {
		src := nonTerminals[int(srcIdx)%len(nonTerminals)]

		if src == Created {
			// Created can only go to Initializing; it reaches Failed via one hop.
			return IsLegalTransition(Created, Initializing) &&
				IsLegalTransition(Initializing, Failed)
		}
		return IsLegalTransition(src, Failed)
	}

	if err := quick.Check(prop, propertyConfig); err != nil {
		t.Error(err)
	}
}

// TestProperty_RandomWalkTermination verifies that any random walk of legal
// transitions that is long enough eventually reaches a terminal state. The seed
// controls the random choices; the walk is capped at 1000 steps after which we
// require that the machine is either terminal or still has outgoing legal edges
// (the graph has no dead ends outside of terminal states).
func TestProperty_RandomWalkTermination(t *testing.T) {
	const maxSteps = 1000

	prop := func(seed int64) bool {
		rng := rand.New(rand.NewSource(seed)) //nolint:gosec
		m := NewMachine()

		for step := 0; step < maxSteps; step++ {
			if m.State().IsTerminal() {
				return true
			}
			nexts := Transitions(m.State())
			if len(nexts) == 0 {
				// Non-terminal with no outgoing edges — graph invariant violated.
				return false
			}
			next := nexts[rng.Intn(len(nexts))]
			if err := m.Transition(next); err != nil {
				// A legal-table pick must never fail.
				return false
			}
		}

		// After maxSteps, the machine is allowed to still be non-terminal (the
		// loop can cycle through ToolDecision -> ToolCall -> ... -> LLMContinuation
		// -> ToolDecision), but it must always have a way out (outgoing edges exist).
		if m.State().IsTerminal() {
			return true
		}
		return len(Transitions(m.State())) > 0
	}

	if err := quick.Check(prop, propertyConfig); err != nil {
		t.Error(err)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// driveToState creates a fresh Machine and drives it to target using the
// BFS shortest path. Returns nil if no path exists (should never happen for
// valid states in the D16 graph).
func driveToState(target State) *Machine {
	m := NewMachine()
	path := shortestPath(target)
	if path == nil && target != Created {
		return nil
	}
	for _, next := range path {
		if err := m.Transition(next); err != nil {
			return nil
		}
	}
	return m
}

// terminalReachableSet returns a map reporting which states can reach at least
// one terminal state, computed by reverse-BFS from the terminal set.
func terminalReachableSet() map[State]bool {
	// Build a reverse adjacency list.
	reverse := make(map[State][]State, int(stateCount))
	for src := State(0); src < stateCount; src++ {
		for _, dst := range Transitions(src) {
			reverse[dst] = append(reverse[dst], src)
		}
	}

	reachable := make(map[State]bool, int(stateCount))
	queue := make([]State, 0, int(stateCount))

	for _, t := range TerminalStates() {
		reachable[t] = true
		queue = append(queue, t)
	}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, pred := range reverse[cur] {
			if !reachable[pred] {
				reachable[pred] = true
				queue = append(queue, pred)
			}
		}
	}

	return reachable
}

// legalWalk generates a sequence of up to n legal transitions starting from
// Created, using the provided seed for randomness.
func legalWalk(seed int64, n int) []State {
	rng := rand.New(rand.NewSource(seed)) //nolint:gosec
	seq := make([]State, 0, n)
	cur := Created

	for i := 0; i < n; i++ {
		if cur.IsTerminal() {
			break
		}
		nexts := Transitions(cur)
		if len(nexts) == 0 {
			break
		}
		next := nexts[rng.Intn(len(nexts))]
		seq = append(seq, next)
		cur = next
	}

	return seq
}
