// SPDX-License-Identifier: Apache-2.0

package praxis_test

import (
	"testing"

	"github.com/praxis-os/praxis"
)

func TestEventType_IsTerminal(t *testing.T) {
	terminal := []praxis.EventType{
		praxis.EventTypeInvocationCompleted,
		praxis.EventTypeInvocationFailed,
		praxis.EventTypeInvocationCancelled,
		praxis.EventTypeBudgetExceeded,
		praxis.EventTypeApprovalRequired,
	}
	for _, et := range terminal {
		if !et.IsTerminal() {
			t.Errorf("%q should be terminal", et)
		}
	}

	nonTerminal := []praxis.EventType{
		praxis.EventTypeInvocationStarted,
		praxis.EventTypeInitialized,
		praxis.EventTypePreHookStarted,
		praxis.EventTypePreHookCompleted,
		praxis.EventTypeLLMCallStarted,
		praxis.EventTypeLLMCallCompleted,
		praxis.EventTypeToolDecisionStarted,
		praxis.EventTypeToolCallStarted,
		praxis.EventTypeToolCallCompleted,
		praxis.EventTypePostToolFilterStarted,
		praxis.EventTypePostToolFilterCompleted,
		praxis.EventTypeLLMContinuationStarted,
		praxis.EventTypePostHookStarted,
		praxis.EventTypePostHookCompleted,
		praxis.EventTypePIIRedacted,
		praxis.EventTypePromptInjectionSuspected,
	}
	for _, et := range nonTerminal {
		if et.IsTerminal() {
			t.Errorf("%q should not be terminal", et)
		}
	}
}

func TestEventTypeCount(t *testing.T) {
	// Verify we have exactly 21 event types (14 non-terminal + 2 content-analysis + 5 terminal).
	all := []praxis.EventType{
		praxis.EventTypeInvocationStarted,
		praxis.EventTypeInitialized,
		praxis.EventTypePreHookStarted,
		praxis.EventTypePreHookCompleted,
		praxis.EventTypeLLMCallStarted,
		praxis.EventTypeLLMCallCompleted,
		praxis.EventTypeToolDecisionStarted,
		praxis.EventTypeToolCallStarted,
		praxis.EventTypeToolCallCompleted,
		praxis.EventTypePostToolFilterStarted,
		praxis.EventTypePostToolFilterCompleted,
		praxis.EventTypeLLMContinuationStarted,
		praxis.EventTypePostHookStarted,
		praxis.EventTypePostHookCompleted,
		praxis.EventTypePIIRedacted,
		praxis.EventTypePromptInjectionSuspected,
		praxis.EventTypeInvocationCompleted,
		praxis.EventTypeInvocationFailed,
		praxis.EventTypeInvocationCancelled,
		praxis.EventTypeBudgetExceeded,
		praxis.EventTypeApprovalRequired,
	}
	if len(all) != 21 {
		t.Errorf("expected 21 event types, got %d", len(all))
	}

	// Check uniqueness.
	seen := make(map[praxis.EventType]bool)
	for _, et := range all {
		if seen[et] {
			t.Errorf("duplicate event type: %q", et)
		}
		seen[et] = true
	}
}
