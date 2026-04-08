// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/praxis-os/praxis"
	"github.com/praxis-os/praxis/internal/ctxutil"
)

// InvokeStream runs a single agent invocation and returns a channel of
// lifecycle events.
//
// The returned channel has a buffer of 16 (D18). The orchestrator goroutine
// is the sole producer and sole closer (D19, D24). The caller must drain
// the channel; failure to do so will block the orchestrator goroutine until
// the context is cancelled.
//
// The last event on the channel is always a terminal event. The channel is
// closed immediately after the terminal event is sent.
//
// InvokeStream respects ctx cancellation. A cancelled context causes the
// invocation to terminate with EventTypeInvocationCancelled.
func (o *Orchestrator) InvokeStream(ctx context.Context, req praxis.InvocationRequest) <-chan praxis.InvocationEvent {
	ch := make(chan praxis.InvocationEvent, 16)

	model := req.Model
	if model == "" {
		model = o.defaultModel
	}

	maxTurns := req.MaxTurns
	if maxTurns <= 0 {
		maxTurns = o.maxTurns
	}

	go func() {
		var once sync.Once
		defer once.Do(func() { close(ch) })

		// Handle missing model before entering the loop.
		if model == "" {
			ch <- praxis.InvocationEvent{
				Type: praxis.EventTypeInvocationFailed,
				At:   time.Now(),
				Err:  fmt.Errorf("orchestrator: no model configured: set WithDefaultModel or InvocationRequest.Model"),
			}
			return
		}

		sink := func(sinkCtx context.Context, e praxis.InvocationEvent) {
			if e.Type.IsTerminal() {
				// Terminal events use a detached Layer 4 context (D22/D23)
				// to ensure delivery even if the parent is cancelled.
				termCtx, cancel := ctxutil.DetachedWithSpan(sinkCtx, 5*time.Second)
				defer cancel()
				select {
				case ch <- e:
				case <-termCtx.Done():
				}
			} else {
				select {
				case ch <- e:
				case <-ctx.Done():
				}
			}
		}

		o.runLoop(ctx, req, model, maxTurns, sink)
	}()

	return ch
}
