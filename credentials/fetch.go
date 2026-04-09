// SPDX-License-Identifier: Apache-2.0

package credentials

import (
	"context"
	"time"
)

// GraceTimeout is the default soft-cancel grace period for credential
// fetching. During a soft-cancel, Fetch continues for up to this duration
// after the parent context is cancelled (D69).
const GraceTimeout = 500 * time.Millisecond

// SoftCancelFetchCtx builds a context for [Resolver.Fetch] that survives
// parent cancellation for up to graceTimeout.
//
// If the parent context is not yet cancelled, the parent is returned
// unchanged (no additional timeout is imposed).
//
// If the parent context is already cancelled (soft-cancel in progress),
// a detached context is created via [context.WithoutCancel] with a
// graceTimeout deadline. This allows credential resolution to complete
// within the grace window rather than failing immediately. Values
// propagated through the parent (trace spans, baggage) are preserved.
//
// The caller must call the returned CancelFunc when done to release
// resources.
func SoftCancelFetchCtx(parent context.Context, graceTimeout time.Duration) (context.Context, context.CancelFunc) {
	if parent.Err() == nil {
		return parent, func() {}
	}
	detached := context.WithoutCancel(parent)
	return context.WithTimeout(detached, graceTimeout)
}
