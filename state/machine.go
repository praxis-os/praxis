// SPDX-License-Identifier: Apache-2.0

package state

import "fmt"

// TransitionError is returned when a state transition is illegal.
// It captures the source state and the rejected target state.
//
// This type will be replaced by *errors.SystemError once the typed error
// taxonomy (T4.1/T4.2) is implemented. Code should check for this error
// using errors.As rather than type-asserting directly.
type TransitionError struct {
	From State
	To   State
	Msg  string
}

func (e *TransitionError) Error() string {
	return fmt.Sprintf("state: %s: from %s to %s", e.Msg, e.From, e.To)
}

// Machine is the state machine instance for a single invocation.
//
// Machine is not safe for concurrent use from multiple goroutines;
// it is owned exclusively by the invocation loop goroutine (D24,
// sole-producer rule). External readers observe state indirectly via
// InvocationEvent.State.
//
// Machine is exported so that property-based tests can construct
// instances directly.
type Machine struct {
	current     State
	transitions uint64
}

// NewMachine creates a Machine in the Created state.
func NewMachine() *Machine {
	return &Machine{current: Created}
}

// State returns the current state.
func (m *Machine) State() State {
	return m.current
}

// TransitionCount returns the number of successful transitions performed.
func (m *Machine) TransitionCount() uint64 {
	return m.transitions
}

// Transition attempts to move the machine from its current state to next.
//
// Returns nil on success.
// Returns a *TransitionError if:
//   - next is not a legal transition from the current state (D16).
//   - the current state is already terminal (terminal immutability rule).
//
// After a successful transition to a terminal state, all subsequent
// Transition calls return a *TransitionError. Terminal state is immutable
// for the machine's lifetime.
func (m *Machine) Transition(next State) error {
	if m.current.IsTerminal() {
		return &TransitionError{
			From: m.current,
			To:   next,
			Msg:  "terminal state is immutable",
		}
	}
	if !IsLegalTransition(m.current, next) {
		return &TransitionError{
			From: m.current,
			To:   next,
			Msg:  "illegal transition",
		}
	}
	m.current = next
	m.transitions++
	return nil
}
