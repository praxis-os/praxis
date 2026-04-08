// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/praxis-os/praxis/event"
	"github.com/praxis-os/praxis/state"
	"github.com/praxis-os/praxis/telemetry"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

func TestOTelEmitter_ImplementsInterface(_ *testing.T) {
	var _ telemetry.LifecycleEventEmitter = (*telemetry.OTelEmitter)(nil)
}

func TestOTelEmitter_EmitNoSpan(t *testing.T) {
	// With no span in context, Emit should not panic.
	emitter := telemetry.NewOTelEmitter(noop.NewTracerProvider().Tracer("test"))
	err := emitter.Emit(context.Background(), event.InvocationEvent{
		Type:         event.EventTypeInvocationStarted,
		InvocationID: "inv-1",
		State:        state.Initializing,
		At:           time.Now(),
	})
	if err != nil {
		t.Errorf("Emit() = %v, want nil", err)
	}
}

func TestOTelEmitter_EmitWithSpan(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	emitter := telemetry.NewOTelEmitter(tracer)

	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	events := []event.InvocationEvent{
		{Type: event.EventTypeInvocationStarted, InvocationID: "inv-1", State: state.Initializing, At: time.Now()},
		{Type: event.EventTypeLLMCallStarted, InvocationID: "inv-1", State: state.LLMCall, At: time.Now()},
		{Type: event.EventTypeToolCallStarted, InvocationID: "inv-1", State: state.ToolCall, ToolCallID: "c1", ToolName: "search", At: time.Now()},
		{Type: event.EventTypeInvocationCompleted, InvocationID: "inv-1", State: state.Completed, At: time.Now()},
	}

	for _, e := range events {
		if err := emitter.Emit(ctx, e); err != nil {
			t.Errorf("Emit(%s) = %v, want nil", e.Type, err)
		}
	}
}

func TestOTelEmitter_EmitTerminalWithError(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	emitter := telemetry.NewOTelEmitter(tracer)

	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	err := emitter.Emit(ctx, event.InvocationEvent{
		Type:         event.EventTypeInvocationFailed,
		InvocationID: "inv-1",
		State:        state.Failed,
		At:           time.Now(),
		Err:          fmt.Errorf("something failed"),
	})
	if err != nil {
		t.Errorf("Emit() = %v, want nil", err)
	}
}

func TestOTelEmitter_EmitTerminalWithoutError(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	emitter := telemetry.NewOTelEmitter(tracer)

	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	// Terminal but no error (e.g., Completed) should not call RecordError.
	err := emitter.Emit(ctx, event.InvocationEvent{
		Type:         event.EventTypeInvocationCompleted,
		InvocationID: "inv-1",
		State:        state.Completed,
		At:           time.Now(),
	})
	if err != nil {
		t.Errorf("Emit() = %v, want nil", err)
	}
}

func TestNewOTelEmitter(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	emitter := telemetry.NewOTelEmitter(tracer)
	if emitter == nil {
		t.Fatal("NewOTelEmitter returned nil")
	}

	// Verify it satisfies the interface through a variable assignment.
	var _ telemetry.LifecycleEventEmitter = emitter

	// Ensure the noop tracer doesn't cause issues.
	_ = trace.SpanFromContext(context.Background())
}
