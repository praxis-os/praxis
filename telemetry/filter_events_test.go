// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"testing"

	"github.com/praxis-os/praxis/event"
	"github.com/praxis-os/praxis/hooks"
)

func TestClassifyFilterDecision(t *testing.T) {
	tests := []struct {
		name string
		d    hooks.FilterDecision
		want []event.EventType
	}{
		{
			name: "pass never emits",
			d:    hooks.FilterDecision{Action: hooks.FilterActionPass, Reason: "pii detected"},
			want: nil,
		},
		{
			name: "redact with PII reason",
			d:    hooks.FilterDecision{Action: hooks.FilterActionRedact, Reason: "PII: email found"},
			want: []event.EventType{event.EventTypePIIRedacted},
		},
		{
			name: "redact with injection reason",
			d:    hooks.FilterDecision{Action: hooks.FilterActionRedact, Reason: "prompt injection detected"},
			want: []event.EventType{event.EventTypePromptInjectionSuspected},
		},
		{
			name: "redact with both PII and injection",
			d:    hooks.FilterDecision{Action: hooks.FilterActionRedact, Reason: "PII email with injection attempt"},
			want: []event.EventType{event.EventTypePIIRedacted, event.EventTypePromptInjectionSuspected},
		},
		{
			name: "redact with no signal terms",
			d:    hooks.FilterDecision{Action: hooks.FilterActionRedact, Reason: "general cleanup"},
			want: nil,
		},
		{
			name: "block with injection reason",
			d:    hooks.FilterDecision{Action: hooks.FilterActionBlock, Reason: "jailbreak attempt"},
			want: []event.EventType{event.EventTypePromptInjectionSuspected},
		},
		{
			name: "log with PII reason",
			d:    hooks.FilterDecision{Action: hooks.FilterActionLog, Reason: "ssn detected"},
			want: []event.EventType{event.EventTypePIIRedacted},
		},
		{
			name: "case insensitive matching",
			d:    hooks.FilterDecision{Action: hooks.FilterActionRedact, Reason: "CREDIT CARD number found"},
			want: []event.EventType{event.EventTypePIIRedacted},
		},
		{
			name: "block with no signal",
			d:    hooks.FilterDecision{Action: hooks.FilterActionBlock, Reason: "content too long"},
			want: nil,
		},
		{
			name: "log with jailbreak",
			d:    hooks.FilterDecision{Action: hooks.FilterActionLog, Reason: "possible Jailbreak"},
			want: []event.EventType{event.EventTypePromptInjectionSuspected},
		},
		{
			name: "passport signal",
			d:    hooks.FilterDecision{Action: hooks.FilterActionRedact, Reason: "passport number redacted"},
			want: []event.EventType{event.EventTypePIIRedacted},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyFilterDecision(tc.d)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d events, want %d: %v", len(got), len(tc.want), got)
			}
			for i, g := range got {
				if g != tc.want[i] {
					t.Errorf("event[%d] = %q, want %q", i, g, tc.want[i])
				}
			}
		})
	}
}
