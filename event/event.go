// SPDX-License-Identifier: Apache-2.0

// Package event defines the lifecycle event types emitted by the praxis
// orchestrator during an invocation.
//
// Consumers of [orchestrator.InvokeStream] import this package to switch
// on [EventType] constants and read [InvocationEvent] fields.
package event

import (
	"time"

	"github.com/praxis-os/praxis/budget"
	"github.com/praxis-os/praxis/errors"
	"github.com/praxis-os/praxis/state"
)

// EventType identifies the kind of lifecycle event emitted during an
// invocation. The full set of 21 event type constants is defined in
// event_types.go.
type EventType string

// InvocationEvent represents a lifecycle event emitted by the orchestrator
// during an invocation. Events are delivered to the configured
// [telemetry.LifecycleEventEmitter] and, on the streaming path, sent
// through the InvokeStream channel.
type InvocationEvent struct {
	// Type identifies the event kind.
	Type EventType

	// InvocationID is the unique identifier for the invocation that
	// produced this event.
	InvocationID string

	// State is the invocation state at the time of the event.
	State state.State

	// At is the wall-clock time at which the event occurred.
	At time.Time

	// Err is a non-nil error for failure/cancellation terminal events.
	Err error

	// ToolCallID is the tool call identifier, populated for tool-related events.
	ToolCallID string

	// ToolName is the tool name, populated for tool-related events.
	ToolName string

	// BudgetSnapshot is the budget consumption at the time of the event.
	BudgetSnapshot budget.BudgetSnapshot

	// ApprovalSnapshot is populated only for EventTypeApprovalRequired.
	ApprovalSnapshot *errors.ApprovalSnapshot

	// AuditNote is an optional human-readable annotation attached by policy hooks
	// or filters to provide audit trail context for this event. It is empty when
	// no annotation was provided.
	AuditNote string

	// FilterPhase is the filter chain phase that produced this event.
	// Non-empty only on EventTypePIIRedacted and EventTypePromptInjectionSuspected.
	// Values: "pre_llm" (from PreLLMFilter), "post_tool" (from PostToolFilter).
	FilterPhase string

	// FilterField is the dot-path to the content element acted on.
	// Non-empty only on content-analysis events.
	FilterField string

	// FilterReason is the human-readable reason from the FilterDecision.
	// Non-empty only on content-analysis events.
	FilterReason string

	// FilterAction is the FilterAction string value from the FilterDecision.
	// Non-empty only on content-analysis events.
	FilterAction string
}
