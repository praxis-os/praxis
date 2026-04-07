// SPDX-License-Identifier: Apache-2.0

// Package state defines the invocation state machine for praxis.
//
// An invocation progresses through 14 states: 9 non-terminal (Created through
// PostHook) and 5 terminal (Completed, Failed, Cancelled, BudgetExceeded,
// ApprovalRequired). The ordering is load-bearing: non-terminal states occupy
// values 0–8 and terminal states 9–13, enabling a branchless IsTerminal check.
//
// The transition allow-list (D16) defines which state transitions are legal.
// Terminal states have no outgoing edges and are immutable once entered.
package state

import "fmt"

// State represents an invocation lifecycle state as a uint8.
// Non-terminal states: 0–8; terminal states: 9–13.
// This ordering is LOAD-BEARING for the branchless IsTerminal predicate (D28).
type State uint8

const (
	// Non-terminal states (0–8).

	Created         State = iota // Invocation object allocated; no work begun.
	Initializing                 // Agent config resolved; PriceProvider snapshot taken; wall-clock starts.
	PreHook                      // Pre-invocation policy hook chain (PreInvocation phase).
	LLMCall                      // Pre-LLM filters applied; LLM request in flight.
	ToolDecision                 // LLM response received; tool calls inspected against budget.
	ToolCall                     // Tool invoker dispatching; credentials fetched; identity signing optional.
	PostToolFilter               // Post-tool filter chain scrubs untrusted output.
	LLMContinuation              // Tool results injected; budget re-check; next LLM call prepared.
	PostHook                     // Post-invocation policy hook chain (PostInvocation phase).

	// Terminal states (9–13). Once entered, no further transitions are legal.

	Completed        // LLM returned final answer; post-hook chain passed.
	Failed           // Unrecoverable error at any non-terminal state.
	Cancelled        // Context cancelled (soft or hard cancel).
	BudgetExceeded   // Any budget dimension breached (tokens, cost, duration, tool calls).
	ApprovalRequired // Policy hook returned Deny with approval-required signal.

	// stateCount is an unexported sentinel for iteration and table sizing.
	stateCount
)

// stateNames maps each State to its string representation.
var stateNames = [stateCount]string{
	Created:          "Created",
	Initializing:     "Initializing",
	PreHook:          "PreHook",
	LLMCall:          "LLMCall",
	ToolDecision:     "ToolDecision",
	ToolCall:         "ToolCall",
	PostToolFilter:   "PostToolFilter",
	LLMContinuation:  "LLMContinuation",
	PostHook:         "PostHook",
	Completed:        "Completed",
	Failed:           "Failed",
	Cancelled:        "Cancelled",
	BudgetExceeded:   "BudgetExceeded",
	ApprovalRequired: "ApprovalRequired",
}

// String returns the human-readable name of the state.
// Unknown state values return "State(<N>)".
func (s State) String() string {
	if s < stateCount {
		return stateNames[s]
	}
	return fmt.Sprintf("State(%d)", s)
}

// IsTerminal reports whether the state is a terminal state (no outgoing edges).
// Terminal states are Completed, Failed, Cancelled, BudgetExceeded, and
// ApprovalRequired (values >= 9).
func (s State) IsTerminal() bool {
	return s >= Completed
}

// transitions is the D16 adjacency table defining legal state transitions.
// Terminal states map to nil (no outgoing edges).
var transitions = [stateCount][]State{
	Created:          {Initializing},
	Initializing:     {PreHook, Failed},
	PreHook:          {LLMCall, Failed, ApprovalRequired},
	LLMCall:          {ToolDecision, Failed, Cancelled, BudgetExceeded},
	ToolDecision:     {ToolCall, PostHook, Failed, Cancelled, BudgetExceeded},
	ToolCall:         {PostToolFilter, Failed, Cancelled},
	PostToolFilter:   {LLMContinuation, Failed, Cancelled},
	LLMContinuation:  {ToolDecision, Failed, Cancelled, BudgetExceeded},
	PostHook:         {Completed, Failed, ApprovalRequired, Cancelled},
	Completed:        nil,
	Failed:           nil,
	Cancelled:        nil,
	BudgetExceeded:   nil,
	ApprovalRequired: nil,
}

// Transitions returns the legal next states from s.
// Terminal states return nil.
func Transitions(s State) []State {
	if s >= stateCount {
		return nil
	}
	return transitions[s]
}

// IsLegalTransition reports whether transitioning from src to dst is permitted
// by the D16 adjacency table.
func IsLegalTransition(src, dst State) bool {
	for _, allowed := range Transitions(src) {
		if allowed == dst {
			return true
		}
	}
	return false
}

// Count returns the total number of defined states (14).
func Count() int {
	return int(stateCount)
}

// All returns a slice of all 14 defined states in order.
func All() []State {
	all := make([]State, stateCount)
	for i := range all {
		all[i] = State(i)
	}
	return all
}

// TerminalStates returns a slice of all 5 terminal states.
func TerminalStates() []State {
	return []State{Completed, Failed, Cancelled, BudgetExceeded, ApprovalRequired}
}

// NonTerminalStates returns a slice of all 9 non-terminal states.
func NonTerminalStates() []State {
	return []State{Created, Initializing, PreHook, LLMCall, ToolDecision, ToolCall, PostToolFilter, LLMContinuation, PostHook}
}
