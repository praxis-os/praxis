// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"context"

	"github.com/praxis-os/praxis"
)

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
	// Emit delivers a lifecycle event to the sink. A non-nil error signals
	// a delivery failure; the orchestrator logs it but does not halt.
	Emit(ctx context.Context, event praxis.InvocationEvent) error
}

// AttributeEnricher adds caller-specific attributes to telemetry metadata.
//
// The orchestrator calls Enrich at the start of each invocation so
// consumer-specific identifiers (e.g., tenant, user, request correlation ID)
// can be injected without coupling the framework to any particular schema.
//
// Implementations must be safe for concurrent use.
//
// Stability: frozen-v1.0.
type AttributeEnricher interface {
	// Enrich returns caller-specific attributes to be attached to all
	// telemetry for the current invocation.
	Enrich(ctx context.Context) map[string]string
}

// Compile-time interface checks.
var _ LifecycleEventEmitter = NullEmitter{}
var _ AttributeEnricher = NullEnricher{}

// NullEmitter is a [LifecycleEventEmitter] that silently discards all events.
// Used as the default when no telemetry sink is configured.
type NullEmitter struct{}

// Emit discards the event without side effects and returns nil.
func (NullEmitter) Emit(_ context.Context, _ praxis.InvocationEvent) error { return nil }

// NullEnricher is an [AttributeEnricher] that returns an empty map.
// Used as the default when no enrichment is configured.
type NullEnricher struct{}

// Enrich returns an empty attribute map.
func (NullEnricher) Enrich(_ context.Context) map[string]string {
	return map[string]string{}
}
