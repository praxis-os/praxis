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
	At                 time.Time
	Err                error
	EnricherAttributes map[string]string
	ApprovalSnapshot   *errors.ApprovalSnapshot
	BudgetSnapshot     budget.BudgetSnapshot
	ToolName           string
	Type               EventType
	ToolCallID         string
	AuditNote          string
	FilterPhase        string
	FilterField        string
	FilterReason       string
	FilterAction       string
	InvocationID       string
	State              state.State
}
