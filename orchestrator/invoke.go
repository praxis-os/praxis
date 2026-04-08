// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"

	"github.com/praxis-os/praxis"
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
	var events []praxis.InvocationEvent
	var terminalErr error

	sink := func(_ context.Context, e praxis.InvocationEvent) {
		events = append(events, e)
		if e.Type.IsTerminal() && e.Err != nil {
			terminalErr = e.Err
		}
	}

	result := o.runLoop(ctx, req, model, maxTurns, sink)
	result.Events = events
	return result, terminalErr
}
