# Phase 3 — State Types

**Stability tier:** `frozen-v1.0` (the 14 states and adjacency table are
load-bearing and not re-opened without a D15/D16 amendment)
**Decision:** D43
**Package:** `MODULE_PATH_TBD/state`

---

## Overview

The `state` package exports the `State` type, the 14 state constants, the
`IsTerminal()` predicate, the `Transitions()` function (adjacency table), and
the `Machine` type used by the orchestrator's internal loop. The `Machine` type
is exported so that callers can embed it in property-based tests using the same
state-machine instance as the runtime.

---

## `State` type (D43)

```go
// State is the type of a single state in the praxis 14-state invocation
// machine (D15). It is a typed uint8 to allow efficient storage and a
// branchless terminal predicate.
//
// Non-terminal states are ordered first (0–8); terminal states last (9–13).
// This ordering enables IsTerminal() to be a single range comparison.
//
// Stability: frozen-v1.0. The 14-state count and ordering are load-bearing
// for the property-based test suite (D28) and may not change without a D15
// amendment.
type State uint8

const (
    // Non-terminal states (0–8)

    // Created: invocation object allocated; no work begun.
    Created State = iota // 0

    // Initializing: agent config and tool list resolved; PriceProvider
    // snapshot taken (D26); wall-clock started (D25).
    Initializing // 1

    // PreHook: pre-invocation policy hook chain (PhasePreInvocation).
    PreHook // 2

    // LLMCall: pre-LLM filters applied; LLM request in flight.
    LLMCall // 3

    // ToolDecision: LLM response received; tool calls inspected against budget.
    ToolDecision // 4

    // ToolCall: tool invoker dispatching; credentials fetched; identity signing.
    ToolCall // 5

    // PostToolFilter: post-tool filter chain scrubs untrusted output.
    PostToolFilter // 6

    // LLMContinuation: tool results injected; budget re-check; next LLM call.
    LLMContinuation // 7

    // PostHook: post-invocation policy hook chain (PhasePostInvocation).
    PostHook // 8

    // Terminal states (9–13) — IsTerminal() returns true for all of these.

    // Completed: terminal success.
    Completed // 9

    // Failed: terminal failure with classified TypedError.
    Failed // 10

    // Cancelled: terminal via ctx.Done().
    Cancelled // 11

    // BudgetExceeded: terminal via budget dimension breach.
    BudgetExceeded // 12

    // ApprovalRequired: terminal; human approval required before resume (D07).
    ApprovalRequired // 13

    // stateCount is the total number of states. Unexported sentinel.
    stateCount // 14
)

// IsTerminal reports whether s is a terminal state.
// Terminal states are immutable: no further transitions are legal.
// The implementation relies on the ordering invariant that all terminal
// states have values >= Completed.
func (s State) IsTerminal() bool { return s >= Completed }

// String returns the human-readable name of s.
// Implements fmt.Stringer.
func (s State) String() string {
    switch s {
    case Created:          return "Created"
    case Initializing:     return "Initializing"
    case PreHook:          return "PreHook"
    case LLMCall:          return "LLMCall"
    case ToolDecision:     return "ToolDecision"
    case ToolCall:         return "ToolCall"
    case PostToolFilter:   return "PostToolFilter"
    case LLMContinuation:  return "LLMContinuation"
    case PostHook:         return "PostHook"
    case Completed:        return "Completed"
    case Failed:           return "Failed"
    case Cancelled:        return "Cancelled"
    case BudgetExceeded:   return "BudgetExceeded"
    case ApprovalRequired: return "ApprovalRequired"
    default:               return fmt.Sprintf("State(%d)", uint8(s))
    }
}
```

---

## `Transitions` function (adjacency table)

```go
// Transitions returns the set of states legally reachable from s in one
// transition step. Returns nil for terminal states (no outgoing edges).
//
// This function is the single authoritative source for the D16 adjacency
// table. Both the runtime state machine and the property-based test suite
// (D28) use this function to determine legal transitions.
//
// The returned slice must not be modified by callers; it is freshly
// allocated on each call to prevent accidental mutation of the canonical
// table.
func Transitions(s State) []State {
    switch s {
    case Created:
        return []State{Initializing}
    case Initializing:
        return []State{PreHook, Failed}
    case PreHook:
        return []State{LLMCall, Failed, ApprovalRequired}
    case LLMCall:
        return []State{ToolDecision, Failed, Cancelled, BudgetExceeded}
    case ToolDecision:
        return []State{ToolCall, PostHook, Failed, Cancelled, BudgetExceeded}
    case ToolCall:
        return []State{PostToolFilter, Failed, Cancelled}
    case PostToolFilter:
        return []State{LLMContinuation, Failed, Cancelled}
    case LLMContinuation:
        return []State{ToolDecision, Failed, Cancelled, BudgetExceeded}
    case PostHook:
        return []State{Completed, Failed, ApprovalRequired, Cancelled}
    default:
        // Terminal states and unknown states have no outgoing edges.
        return nil
    }
}

// IsLegalTransition reports whether the transition from -> to is legal
// per the D16 adjacency table.
func IsLegalTransition(from, to State) bool {
    for _, s := range Transitions(from) {
        if s == to {
            return true
        }
    }
    return false
}
```

---

## `Machine` type

```go
// Machine is the state machine instance for a single invocation.
//
// Machine is not safe for concurrent use from multiple goroutines;
// it is owned exclusively by the invocation loop goroutine (D24,
// sole-producer rule). External readers (e.g., event consumers) observe
// state indirectly via InvocationEvent.State.
//
// Machine is exported so that property-based tests can construct
// instances directly.
type Machine struct {
    // unexported fields: current State, transition count
}

// NewMachine creates a Machine in the Created state.
func NewMachine() *Machine

// State returns the current state.
func (m *Machine) State() State

// Transition attempts to move the machine from its current state to next.
//
// Returns nil on success.
// Returns a non-nil *errors.SystemError if:
//   - next is not a legal transition from the current state (D16).
//   - the current state is already terminal (terminal immutability rule).
//
// After a successful transition to a terminal state, all subsequent
// Transition calls return a SystemError. Terminal state is immutable.
func (m *Machine) Transition(next State) error
```

---

## Terminal immutability rule

Once `Machine.Transition` succeeds into any terminal state
(`Completed`, `Failed`, `Cancelled`, `BudgetExceeded`, `ApprovalRequired`),
all subsequent `Transition` calls return a `*errors.SystemError` with
`ErrorKindSystem`. The machine does not panic. The error is logged by the
orchestrator at ERROR level and delivered to the
`telemetry.LifecycleEventEmitter` for investigation.

This rule is the foundation for the soft/hard cancel precedence decisions
in D21: a cancel signal arriving after `ApprovalRequired` or `BudgetExceeded`
is entered cannot overwrite the terminal state. The governance audit trail
records the true terminal.

---

## Property-based test obligations (D28)

The 21 invariants in `docs/phase-2-core-runtime/06-state-machine-invariants.md`
are validated against `Machine` and `Transitions`. The conformance is:

- INV-01 to INV-10 (structural): use `IsLegalTransition` and `IsTerminal`.
- INV-11 to INV-17 (streaming event): use `Machine.Transition` with a
  simulated event emitter; assert event ordering.
- INV-18 to INV-21 (terminal path): assert exactly one terminal event per
  terminal transition; assert `BudgetExceededError.Snapshot.ExceededDimension`
  is set.

The property suite runs 10k iterations in CI and 100k in nightly builds.
