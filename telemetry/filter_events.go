// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"strings"

	"github.com/praxis-os/praxis/event"
	"github.com/praxis-os/praxis/hooks"
)

// PII signal terms (case-insensitive substring match). Frozen-v1.0: terms
// are not removed in v1.x; new terms may be added in minor releases.
var piiSignalTerms = []string{
	"pii", "personal", "ssn", "credit card", "email", "phone",
	"address", "dob", "date of birth", "passport", "national id",
}

// Injection signal terms (case-insensitive substring match). Frozen-v1.0.
var injectionSignalTerms = []string{
	"injection", "prompt injection", "jailbreak",
}

// ClassifyFilterDecision returns the content-analysis event types that
// should be emitted for the given FilterDecision, per D59.
//
// FilterActionPass never produces events. For other actions, the reason
// is checked against PII and injection signal terms using case-insensitive
// substring matching. Both event types may be returned if both signal
// categories match.
func ClassifyFilterDecision(d hooks.FilterDecision) []event.EventType {
	if d.Action == hooks.FilterActionPass {
		return nil
	}

	reason := strings.ToLower(d.Reason)
	var types []event.EventType

	if containsAny(reason, piiSignalTerms) {
		types = append(types, event.EventTypePIIRedacted)
	}
	if containsAny(reason, injectionSignalTerms) {
		types = append(types, event.EventTypePromptInjectionSuspected)
	}

	return types
}

// containsAny reports whether s contains any of the given substrings.
func containsAny(s string, terms []string) bool {
	for _, t := range terms {
		if strings.Contains(s, t) {
			return true
		}
	}
	return false
}
