// SPDX-License-Identifier: Apache-2.0

package event

import (
	"testing"

	"github.com/praxis-os/praxis/errors"
)

func TestTerminalEventTypeForError(t *testing.T) {
	tests := []struct {
		kind errors.ErrorKind
		want EventType
	}{
		{errors.ErrorKindTransientLLM, EventTypeInvocationFailed},
		{errors.ErrorKindPermanentLLM, EventTypeInvocationFailed},
		{errors.ErrorKindTool, EventTypeInvocationFailed},
		{errors.ErrorKindPolicyDenied, EventTypeInvocationFailed},
		{errors.ErrorKindBudgetExceeded, EventTypeBudgetExceeded},
		{errors.ErrorKindCancellation, EventTypeInvocationCancelled},
		{errors.ErrorKindSystem, EventTypeInvocationFailed},
		{errors.ErrorKindApprovalRequired, EventTypeApprovalRequired},
	}

	for _, tc := range tests {
		t.Run(string(tc.kind), func(t *testing.T) {
			got := TerminalEventTypeForError(tc.kind)
			if got != tc.want {
				t.Errorf("TerminalEventTypeForError(%q) = %q, want %q", tc.kind, got, tc.want)
			}
		})
	}
}

func TestTerminalEventTypeForError_AllKindsCovered(t *testing.T) {
	allKinds := []errors.ErrorKind{
		errors.ErrorKindTransientLLM,
		errors.ErrorKindPermanentLLM,
		errors.ErrorKindTool,
		errors.ErrorKindPolicyDenied,
		errors.ErrorKindBudgetExceeded,
		errors.ErrorKindCancellation,
		errors.ErrorKindSystem,
		errors.ErrorKindApprovalRequired,
	}

	for _, k := range allKinds {
		if _, ok := errorKindToEventType[k]; !ok {
			t.Errorf("ErrorKind %q missing from mapping", k)
		}
	}

	if len(errorKindToEventType) != len(allKinds) {
		t.Errorf("mapping has %d entries, want %d (one per ErrorKind)", len(errorKindToEventType), len(allKinds))
	}
}

func TestTerminalEventTypeForError_UnknownKindReturnsFailed(t *testing.T) {
	got := TerminalEventTypeForError(errors.ErrorKind("unknown_kind"))
	if got != EventTypeInvocationFailed {
		t.Errorf("unknown kind = %q, want %q", got, EventTypeInvocationFailed)
	}
}

func TestTerminalEventTypeForError_AllResultsAreTerminal(t *testing.T) {
	for kind, evtType := range errorKindToEventType {
		if !evtType.IsTerminal() {
			t.Errorf("TerminalEventTypeForError(%q) = %q which is NOT terminal", kind, evtType)
		}
	}
}
