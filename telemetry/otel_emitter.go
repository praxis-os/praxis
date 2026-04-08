// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/praxis-os/praxis/event"
)

// Compile-time interface check.
var _ LifecycleEventEmitter = (*OTelEmitter)(nil)

// OTelEmitter is a [LifecycleEventEmitter] that records invocation lifecycle
// events as span events on the current OpenTelemetry span.
//
// Each call to [OTelEmitter.Emit] resolves the active span from the context via
// [trace.SpanFromContext] and adds a span event whose name is the string value
// of the [event.EventType]. Core attributes (praxis.invocation_id,
// praxis.state) are always attached; tool-specific attributes
// (praxis.tool_call_id, praxis.tool_name) are attached only when the event
// carries tool-call context.
//
// For terminal events that carry a non-nil error, RecordError and
// codes.Error are applied to the span so that tracing backends surface the
// failure at the root span level.
//
// OTelEmitter does not own the tracer's lifecycle; callers are responsible for
// flushing and shutting down the underlying [trace.TracerProvider].
//
// Implementations must be safe for concurrent use. OTelEmitter is safe because
// it holds no mutable state; all span mutations are delegated to the span
// obtained from the caller-supplied context.
//
// Stability: stable-v0.3.
type OTelEmitter struct {
	tracer trace.Tracer
}

// NewOTelEmitter constructs an [OTelEmitter] backed by the given tracer.
// The tracer is used for future span creation if the emitter is extended in
// later versions; current behaviour resolves the active span from the context
// on each [Emit] call.
func NewOTelEmitter(tracer trace.Tracer) *OTelEmitter {
	return &OTelEmitter{tracer: tracer}
}

// Emit records the invocation lifecycle event as a span event on the current
// span. If no span is active in ctx, the event is added to the no-op span
// returned by [trace.SpanFromContext] and is silently discarded by the SDK.
//
// For terminal events with a non-nil Err, Emit additionally calls
// span.RecordError and sets the span status to codes.Error so that
// observability backends propagate the failure to the root span.
//
// Emit always returns nil; span operations are best-effort and must not
// interrupt the orchestrator's hot path.
func (e *OTelEmitter) Emit(ctx context.Context, ev event.InvocationEvent) error {
	span := trace.SpanFromContext(ctx)

	attrs := []attribute.KeyValue{
		attribute.String("praxis.invocation_id", ev.InvocationID),
		attribute.String("praxis.state", ev.State.String()),
	}

	if ev.ToolCallID != "" {
		attrs = append(attrs, attribute.String("praxis.tool_call_id", ev.ToolCallID))
	}
	if ev.ToolName != "" {
		attrs = append(attrs, attribute.String("praxis.tool_name", ev.ToolName))
	}

	span.AddEvent(string(ev.Type), trace.WithAttributes(attrs...))

	if ev.Type.IsTerminal() && ev.Err != nil {
		span.RecordError(ev.Err)
		span.SetStatus(codes.Error, ev.Err.Error())
	}

	return nil
}
