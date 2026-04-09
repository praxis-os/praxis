// SPDX-License-Identifier: Apache-2.0

package event

import "github.com/praxis-os/praxis/errors"

// errorKindToEventType maps each ErrorKind to its terminal EventType per D61.
// The mapping is 1:1: every ErrorKind maps to exactly one terminal EventType.
var errorKindToEventType = map[errors.ErrorKind]EventType{
	errors.ErrorKindTransientLLM:     EventTypeInvocationFailed,
	errors.ErrorKindPermanentLLM:     EventTypeInvocationFailed,
	errors.ErrorKindTool:             EventTypeInvocationFailed,
	errors.ErrorKindPolicyDenied:     EventTypeInvocationFailed,
	errors.ErrorKindBudgetExceeded:   EventTypeBudgetExceeded,
	errors.ErrorKindCancellation:     EventTypeInvocationCancelled,
	errors.ErrorKindSystem:           EventTypeInvocationFailed,
	errors.ErrorKindApprovalRequired: EventTypeApprovalRequired,
}

// TerminalEventTypeForError returns the terminal EventType that corresponds
// to the given ErrorKind per D61.
//
// If the ErrorKind is not recognized, EventTypeInvocationFailed is returned
// as a safe default (unknown errors are treated as system failures).
//
// The mapping is 1:1 and framework-enforced. The first error to drive a
// terminal state transition wins; subsequent errors do not alter the
// terminal event (enforced by state machine immutability, D15).
func TerminalEventTypeForError(kind errors.ErrorKind) EventType {
	if evtType, ok := errorKindToEventType[kind]; ok {
		return evtType
	}
	return EventTypeInvocationFailed
}
