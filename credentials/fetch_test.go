// SPDX-License-Identifier: Apache-2.0

package credentials_test

import (
	"context"
	"testing"
	"time"

	"github.com/praxis-os/praxis/credentials"
)

func TestSoftCancelFetchCtx_ParentNotCancelled(t *testing.T) {
	parent := context.Background()
	ctx, cancel := credentials.SoftCancelFetchCtx(parent, credentials.GraceTimeout)
	defer cancel()

	// Context should not be done.
	select {
	case <-ctx.Done():
		t.Fatal("context should not be done when parent is not cancelled")
	default:
	}

	// Should be the same parent context (no timeout imposed).
	if ctx.Err() != nil {
		t.Errorf("ctx.Err() = %v, want nil", ctx.Err())
	}
}

func TestSoftCancelFetchCtx_ParentCancelled_GracePeriod(t *testing.T) {
	parent, parentCancel := context.WithCancel(context.Background())
	parentCancel() // Cancel parent to simulate soft-cancel.

	ctx, cancel := credentials.SoftCancelFetchCtx(parent, 100*time.Millisecond)
	defer cancel()

	// The detached context should NOT be done immediately.
	select {
	case <-ctx.Done():
		t.Fatal("detached context should not be done immediately")
	default:
	}

	// Wait for the grace timeout to expire.
	<-ctx.Done()
	if ctx.Err() != context.DeadlineExceeded {
		t.Errorf("ctx.Err() = %v, want DeadlineExceeded", ctx.Err())
	}
}

func TestSoftCancelFetchCtx_ParentCancelled_GraceExpires(t *testing.T) {
	parent, parentCancel := context.WithCancel(context.Background())
	parentCancel()

	grace := 50 * time.Millisecond
	ctx, cancel := credentials.SoftCancelFetchCtx(parent, grace)
	defer cancel()

	start := time.Now()
	<-ctx.Done()
	elapsed := time.Since(start)

	// Should expire within a reasonable window around the grace timeout.
	if elapsed < grace/2 {
		t.Errorf("expired too early: %v (grace=%v)", elapsed, grace)
	}
}

func TestSoftCancelFetchCtx_ParentValues_Preserved(t *testing.T) {
	type ctxKey string
	parent := context.WithValue(context.Background(), ctxKey("trace"), "span-123")

	// Cancel parent.
	pCtx, pCancel := context.WithCancel(parent)
	pCancel()

	ctx, cancel := credentials.SoftCancelFetchCtx(pCtx, credentials.GraceTimeout)
	defer cancel()

	// Values from parent must be accessible.
	if got := ctx.Value(ctxKey("trace")); got != "span-123" {
		t.Errorf("value = %v, want %q", got, "span-123")
	}
}

func TestGraceTimeout_Is500ms(t *testing.T) {
	if credentials.GraceTimeout != 500*time.Millisecond {
		t.Errorf("GraceTimeout = %v, want 500ms", credentials.GraceTimeout)
	}
}
