// SPDX-License-Identifier: Apache-2.0

// Package ctxutil provides context utilities for the praxis orchestrator.
package ctxutil

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// DetachedWithSpan creates a new context derived from context.Background()
// with a timeout, re-attaching the OTel span context from the parent.
//
// This is the Layer 4 emission context (D22/D23): independent of the
// caller's cancellation so terminal events are always delivered, but
// preserving the span context so traces are not broken.
func DetachedWithSpan(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	// Re-attach the OTel span context if present.
	if sc := trace.SpanContextFromContext(parent); sc.IsValid() {
		ctx = trace.ContextWithSpanContext(ctx, sc)
	}

	return ctx, cancel
}
