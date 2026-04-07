// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"context"
	"time"
)

// LifecycleEvent represents a state change in an invocation.
//
// Events are emitted by the orchestrator at each invocation state transition
// and delivered to the configured [LifecycleEventEmitter].
type LifecycleEvent struct {
	// InvocationID is the unique identifier for the invocation that produced
	// this event.
	InvocationID string

	// State is the name of the new invocation state (e.g., "running",
	// "tool_call", "complete", "error").
	State string

	// Timestamp is the wall-clock time at which the state transition occurred.
	Timestamp time.Time

	// Attributes is a set of key-value pairs enriched by the caller's
	// [AttributeEnricher]. Keys and values are plain strings.
	Attributes map[string]string
}

// LifecycleEventEmitter receives invocation lifecycle events from the
// orchestrator.
//
// Implementations may forward events to an OpenTelemetry backend, a metrics
// system, an audit log, or any other sink. Emit must not block the
// orchestrator's hot path; implementations that perform I/O should buffer
// asynchronously.
//
// Implementations must be safe for concurrent use.
//
// Stability: frozen-v1.0.
type LifecycleEventEmitter interface {
	// Emit delivers a lifecycle event to the sink. Emit must not return an
	// error; failures must be handled internally (e.g., dropped with a log
	// line) so the orchestrator is not interrupted by telemetry failures.
	Emit(ctx context.Context, event LifecycleEvent)
}

// AttributeEnricher adds caller-specific attributes to telemetry metadata.
//
// The orchestrator calls Enrich at the start of each invocation so
// consumer-specific identifiers (e.g., tenant, user, request correlation ID)
// can be injected without coupling the framework to any particular schema.
//
// The returned map must not share memory with the input map; implementations
// should copy and extend rather than mutate in place.
//
// Implementations must be safe for concurrent use.
//
// Stability: frozen-v1.0.
type AttributeEnricher interface {
	// Enrich receives the current attribute map and returns an enriched copy.
	// The orchestrator uses the returned map for all subsequent telemetry
	// within the invocation.
	Enrich(ctx context.Context, attrs map[string]string) map[string]string
}

// Compile-time interface checks.
var _ LifecycleEventEmitter = NullEmitter{}
var _ AttributeEnricher = NullEnricher{}

// NullEmitter is a [LifecycleEventEmitter] that silently discards all events.
// Used as the default when no telemetry sink is configured.
type NullEmitter struct{}

// Emit discards the event without side effects.
func (NullEmitter) Emit(_ context.Context, _ LifecycleEvent) {}

// NullEnricher is an [AttributeEnricher] that returns the attribute map
// unchanged. Used as the default when no enrichment is configured.
type NullEnricher struct{}

// Enrich returns the input map without modification.
func (NullEnricher) Enrich(_ context.Context, attrs map[string]string) map[string]string {
	return attrs
}
