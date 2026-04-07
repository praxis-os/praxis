// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"context"
	"testing"
	"time"

	"github.com/praxis-os/praxis/telemetry"
)

func TestInterfaces(t *testing.T) {
	// Compile-time checks documented as runtime assertions.
	var _ telemetry.LifecycleEventEmitter = telemetry.NullEmitter{}
	var _ telemetry.AttributeEnricher = telemetry.NullEnricher{}
}

func TestNullEmitter_Emit(t *testing.T) {
	emitter := telemetry.NullEmitter{}

	// Emit must not panic for any input.
	events := []telemetry.LifecycleEvent{
		{},
		{
			InvocationID: "inv-1",
			State:        "running",
			Timestamp:    time.Now(),
			Attributes:   map[string]string{"key": "value"},
		},
		{
			InvocationID: "inv-2",
			State:        "error",
			Timestamp:    time.Now(),
			Attributes:   nil,
		},
	}

	for _, event := range events {
		// Should not panic.
		emitter.Emit(context.Background(), event)
	}
}

func TestNullEnricher_Enrich(t *testing.T) {
	enricher := telemetry.NullEnricher{}

	tests := []struct {
		name  string
		attrs map[string]string
	}{
		{
			name:  "nil map is returned unchanged",
			attrs: nil,
		},
		{
			name:  "empty map is returned unchanged",
			attrs: map[string]string{},
		},
		{
			name:  "populated map is returned unchanged",
			attrs: map[string]string{"trace_id": "abc123", "span_id": "xyz"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := enricher.Enrich(context.Background(), tt.attrs)

			// NullEnricher returns the same map reference.
			if tt.attrs == nil {
				if got != nil {
					t.Errorf("Enrich(nil) = %v, want nil", got)
				}
				return
			}

			if len(got) != len(tt.attrs) {
				t.Errorf("Enrich() returned map with %d entries, want %d", len(got), len(tt.attrs))
			}
			for k, v := range tt.attrs {
				if got[k] != v {
					t.Errorf("Enrich() key %q = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}
