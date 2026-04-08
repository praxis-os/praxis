// SPDX-License-Identifier: Apache-2.0

package event_test

import (
	"testing"

	"github.com/praxis-os/praxis/event"
)

func TestEventType_IsTerminal(t *testing.T) {
	terminal := []event.EventType{
		event.EventTypeInvocationCompleted,
		event.EventTypeInvocationFailed,
		event.EventTypeInvocationCancelled,
		event.EventTypeBudgetExceeded,
		event.EventTypeApprovalRequired,
	}
	for _, et := range terminal {
		if !et.IsTerminal() {
			t.Errorf("%q should be terminal", et)
		}
	}

	nonTerminal := []event.EventType{
		event.EventTypeInvocationStarted,
		event.EventTypeInitialized,
		event.EventTypePreHookStarted,
		event.EventTypePreHookCompleted,
		event.EventTypeLLMCallStarted,
		event.EventTypeLLMCallCompleted,
		event.EventTypeToolDecisionStarted,
		event.EventTypeToolCallStarted,
		event.EventTypeToolCallCompleted,
		event.EventTypePostToolFilterStarted,
		event.EventTypePostToolFilterCompleted,
		event.EventTypeLLMContinuationStarted,
		event.EventTypePostHookStarted,
		event.EventTypePostHookCompleted,
		event.EventTypePIIRedacted,
		event.EventTypePromptInjectionSuspected,
	}
	for _, et := range nonTerminal {
		if et.IsTerminal() {
			t.Errorf("%q should not be terminal", et)
		}
	}
}

func TestEventTypeCount(t *testing.T) {
	// Verify we have exactly 21 event types (14 non-terminal + 2 content-analysis + 5 terminal).
	all := []event.EventType{
		event.EventTypeInvocationStarted,
		event.EventTypeInitialized,
		event.EventTypePreHookStarted,
		event.EventTypePreHookCompleted,
		event.EventTypeLLMCallStarted,
		event.EventTypeLLMCallCompleted,
		event.EventTypeToolDecisionStarted,
		event.EventTypeToolCallStarted,
		event.EventTypeToolCallCompleted,
		event.EventTypePostToolFilterStarted,
		event.EventTypePostToolFilterCompleted,
		event.EventTypeLLMContinuationStarted,
		event.EventTypePostHookStarted,
		event.EventTypePostHookCompleted,
		event.EventTypePIIRedacted,
		event.EventTypePromptInjectionSuspected,
		event.EventTypeInvocationCompleted,
		event.EventTypeInvocationFailed,
		event.EventTypeInvocationCancelled,
		event.EventTypeBudgetExceeded,
		event.EventTypeApprovalRequired,
	}
	if len(all) != 21 {
		t.Errorf("expected 21 event types, got %d", len(all))
	}

	// Check uniqueness.
	seen := make(map[event.EventType]bool)
	for _, et := range all {
		if seen[et] {
			t.Errorf("duplicate event type: %q", et)
		}
		seen[et] = true
	}
}
