// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"

	"github.com/praxis-os/praxis"
	"github.com/praxis-os/praxis/event"
)

// runInvocation is the sync invocation entry point.
// It collects all lifecycle events into the result and extracts the
// terminal error for the caller.
func runInvocation(
	ctx context.Context,
	o *Orchestrator,
	model string,
	maxTurns int,
	req praxis.InvocationRequest,
) (*praxis.InvocationResult, error) {
	// Preallocate event buffer: a no-tool invocation emits ~8 events;
	// tool paths emit more.
	eventCap := 8
	if len(req.Tools) > 0 {
		eventCap = 16
	}
	events := make([]event.InvocationEvent, 0, eventCap)
	var terminalErr error

	sink := func(_ context.Context, e event.InvocationEvent) {
		events = append(events, e)
		if e.Type.IsTerminal() && e.Err != nil {
			terminalErr = e.Err
		}
	}

	result := o.runLoop(ctx, req, model, maxTurns, sink)
	result.Events = events
	return result, terminalErr
}
