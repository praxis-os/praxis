// SPDX-License-Identifier: Apache-2.0

package ctxutil_test

import (
	"context"
	"testing"
	"time"

	"github.com/praxis-os/praxis/internal/ctxutil"
	"go.opentelemetry.io/otel/trace"
)

func TestDetachedWithSpan_IndependentOfParent(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	cancel() // Cancel the parent immediately.

	ctx, detachedCancel := ctxutil.DetachedWithSpan(parent, 5*time.Second)
	defer detachedCancel()

	// The detached context should NOT be cancelled even though parent is.
	if ctx.Err() != nil {
		t.Errorf("detached context should not inherit parent cancellation, got %v", ctx.Err())
	}
}

func TestDetachedWithSpan_HasTimeout(t *testing.T) {
	parent := context.Background()
	ctx, cancel := ctxutil.DetachedWithSpan(parent, 10*time.Millisecond)
	defer cancel()

	// Wait for the timeout.
	<-ctx.Done()

	if ctx.Err() != context.DeadlineExceeded {
		t.Errorf("expected DeadlineExceeded, got %v", ctx.Err())
	}
}

func TestDetachedWithSpan_PreservesSpanContext(t *testing.T) {
	// Create a parent with a valid span context.
	traceID := trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	spanID := trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8}
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})

	parent := trace.ContextWithSpanContext(context.Background(), sc)
	ctx, cancel := ctxutil.DetachedWithSpan(parent, 5*time.Second)
	defer cancel()

	// The detached context should carry the same span context.
	got := trace.SpanContextFromContext(ctx)
	if got.TraceID() != traceID {
		t.Errorf("TraceID: want %v, got %v", traceID, got.TraceID())
	}
	if got.SpanID() != spanID {
		t.Errorf("SpanID: want %v, got %v", spanID, got.SpanID())
	}
}

func TestDetachedWithSpan_NoSpanContext(t *testing.T) {
	// Parent without span context should not panic.
	parent := context.Background()
	ctx, cancel := ctxutil.DetachedWithSpan(parent, 5*time.Second)
	defer cancel()

	got := trace.SpanContextFromContext(ctx)
	if got.IsValid() {
		t.Error("expected invalid span context when parent has none")
	}
}
