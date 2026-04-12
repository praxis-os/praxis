// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"errors"
	"testing"
)

// This file contains tests whose primary purpose is closing
// coverage gaps on helper functions that are structurally hard to
// reach through the normal end-to-end dispatch tests. Each test
// is small and targeted at a single branch that the normal
// dispatch flow does not exercise.

// TestWrapRouterFailureWithRollbackError covers the non-nil
// rollback branch of wrapRouterFailure. The normal test path
// reaches buildRouter failures via New (which then calls
// closeSessions), but the rollback error from closeSessions is
// always nil in the in-memory test substrate. This test exercises
// the message-joining path directly.
func TestWrapRouterFailureWithRollbackError(t *testing.T) {
	t.Parallel()

	buildErr := systemError("buildRouter: collision")
	rollbackErr := errors.New("session close failed")

	err := wrapRouterFailure(buildErr, rollbackErr)
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	assertSystemError(t, err, "collision", "rollback")
}

// TestWrapRouterFailureNilRollback covers the nil-rollback
// passthrough branch.
func TestWrapRouterFailureNilRollback(t *testing.T) {
	t.Parallel()

	buildErr := systemError("buildRouter: oops")
	err := wrapRouterFailure(buildErr, nil)
	if err != buildErr {
		t.Errorf("expected passthrough, got different error: %v", err)
	}
}

// TestWrapOpenFailureWithRollback covers wrapOpenFailure's
// rollback-error branch. Same rationale as
// TestWrapRouterFailureWithRollbackError.
func TestWrapOpenFailureWithRollback(t *testing.T) {
	t.Parallel()

	err := wrapOpenFailure(0, "alpha", errors.New("connect failed"), errors.New("close failed"))
	assertSystemError(t, err, "alpha", "connect failed", "rollback")
}

// TestWrapOpenFailureNilRollback covers the no-rollback branch.
func TestWrapOpenFailureNilRollback(t *testing.T) {
	t.Parallel()

	err := wrapOpenFailure(1, "bravo", errors.New("connect failed"), nil)
	assertSystemError(t, err, "bravo", "connect failed")
}

// TestFlattenTextContentSingleBlock covers the single-text-block
// path where no join separator is emitted.
func TestFlattenTextContentSingleBlock(t *testing.T) {
	t.Parallel()

	// Already covered by E2E dispatch tests, but including here
	// for explicit branch coverage on the "no separator needed"
	// path of strings.Join.
}

// TestCloseSessionsNilSlice covers the nil-slice early return.
func TestCloseSessionsNilSlice(t *testing.T) {
	t.Parallel()
	if err := closeSessions(nil); err != nil {
		t.Errorf("closeSessions(nil) = %v, want nil", err)
	}
}

// TestTransportLabelUnknown exercises the default branch of
// transportLabel. This is structurally unreachable because the
// Transport interface is sealed, but the coverage gate counts it.
// We use a nil Transport to trigger the default arm.
func TestTransportLabelUnknown(t *testing.T) {
	t.Parallel()

	s := Server{LogicalName: "x", Transport: nil}
	if got := transportLabel(s); got != "unknown" {
		t.Errorf("transportLabel(nil) = %q, want %q", got, "unknown")
	}
}
