// SPDX-License-Identifier: Apache-2.0

// Package telemetry defines the observability interfaces and their null
// implementations.
//
// [LifecycleEventEmitter] receives invocation lifecycle events emitted by
// the orchestrator at each state transition. Implementations may forward
// events to OpenTelemetry, a metrics backend, or an audit log.
//
// [AttributeEnricher] adds caller-specific attributes to telemetry metadata.
// The orchestrator calls Enrich at the start of each invocation so
// consumer-specific identifiers (tenant, user, etc.) can be injected without
// coupling the framework to any particular schema.
//
// [NullEmitter] silently discards all events.
// [NullEnricher] returns attributes unchanged.
//
// Stability: frozen-v1.0.
package telemetry
