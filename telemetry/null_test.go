// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"context"
	"testing"
	"time"

	"github.com/praxis-os/praxis/event"
	"github.com/praxis-os/praxis/telemetry"
)

func TestInterfaces(_ *testing.T) {
	var _ telemetry.LifecycleEventEmitter = telemetry.NullEmitter{}
	var _ telemetry.AttributeEnricher = telemetry.NullEnricher{}
}

func TestNullEmitter_Emit(t *testing.T) {
	emitter := telemetry.NullEmitter{}

	events := []event.InvocationEvent{
		{},
		{
			InvocationID: "inv-1",
			At:           time.Now(),
		},
		{
			InvocationID: "inv-2",
			At:           time.Now(),
		},
	}

	for _, event := range events {
		if err := emitter.Emit(context.Background(), event); err != nil {
			t.Errorf("Emit() unexpected error: %v", err)
		}
	}
}

func TestNullEnricher_Enrich(t *testing.T) {
	enricher := telemetry.NullEnricher{}

	got := enricher.Enrich(context.Background())
	if got != nil {
		t.Errorf("Enrich() returned %v, want nil", got)
	}
}
