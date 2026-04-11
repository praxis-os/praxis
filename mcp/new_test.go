// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	praxiserrors "github.com/praxis-os/praxis/errors"
	"github.com/praxis-os/praxis/tools"
)

// validServer returns a fresh valid Server with the given
// LogicalName. The transport is always a stdio variant with a
// trivial command, so the only distinguishing field across test
// cases is the name itself.
//
// Tests that want validation-only behaviour (construction shape,
// rollback, option binding) pair validServer with
// [withSessionOpener] and [nullSessionOpener] to bypass the
// real SDK-backed session-opening path — the default New flow
// would otherwise fail at [exec.LookPath] on the synthetic
// command name used here.
func validServer(name string) Server {
	return Server{
		LogicalName: name,
		Transport:   TransportStdio{Command: "mcp-test-server"},
	}
}

// assertSystemError asserts that err is a typed
// [praxiserrors.SystemError] whose Error() string contains every
// fragment in wantFragments. It fails the current test with Fatalf
// on the first mismatch.
func assertSystemError(t *testing.T, err error, wantFragments ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected a typed system error, got nil")
	}
	var typed praxiserrors.TypedError
	if !errors.As(err, &typed) {
		t.Fatalf("error is not a TypedError: %T: %v", err, err)
	}
	if typed.Kind() != praxiserrors.ErrorKindSystem {
		t.Fatalf("error Kind() = %q, want %q", typed.Kind(), praxiserrors.ErrorKindSystem)
	}
	var sysErr *praxiserrors.SystemError
	if !errors.As(err, &sysErr) {
		t.Fatalf("error is not a *SystemError: %T: %v", err, err)
	}
	msg := err.Error()
	for _, frag := range wantFragments {
		if !strings.Contains(msg, frag) {
			t.Errorf("error message missing fragment %q; got: %s", frag, msg)
		}
	}
}

// assertToolError asserts that err is a typed
// [*praxiserrors.ToolError] whose Error() string contains every
// fragment in wantFragments. It fails the current test with
// Fatalf on the first structural mismatch. Used by the S32 router
// dispatch tests to check that tool-level failures (unknown tool,
// server-side tool error) are classified as ErrorKindTool rather
// than ErrorKindSystem.
func assertToolError(t *testing.T, err error, wantFragments ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected a typed tool error, got nil")
	}
	var typed praxiserrors.TypedError
	if !errors.As(err, &typed) {
		t.Fatalf("error is not a TypedError: %T: %v", err, err)
	}
	if typed.Kind() != praxiserrors.ErrorKindTool {
		t.Fatalf("error Kind() = %q, want %q", typed.Kind(), praxiserrors.ErrorKindTool)
	}
	var toolErr *praxiserrors.ToolError
	if !errors.As(err, &toolErr) {
		t.Fatalf("error is not a *ToolError: %T: %v", err, err)
	}
	msg := err.Error()
	for _, frag := range wantFragments {
		if !strings.Contains(msg, frag) {
			t.Errorf("error message missing fragment %q; got: %s", frag, msg)
		}
	}
}

// TestNewHappyPath asserts that valid inputs produce a non-nil
// Invoker, that the returned value satisfies the public Invoker
// interface shape, and that Close is idempotent on it. Uses
// nullSessionOpener to bypass the real SDK session-opening path
// — this test is about construction shape, not session lifecycle.
func TestNewHappyPath(t *testing.T) {
	t.Parallel()

	inv, err := New(context.Background(), []Server{validServer("probe")}, withSessionOpener(nullSessionOpener))
	if err != nil {
		t.Fatalf("New returned unexpected error: %v", err)
	}
	if inv == nil {
		t.Fatal("New returned nil Invoker with nil error")
	}

	// Interface-shape sanity: the returned value must satisfy both
	// projections of the public Invoker interface.
	var _ tools.Invoker = inv
	if err := inv.Close(); err != nil {
		t.Errorf("Close: unexpected error: %v", err)
	}
	if err := inv.Close(); err != nil {
		t.Errorf("Close (idempotent): unexpected error: %v", err)
	}
}

// TestNewInvokeUnknownToolRoutes documents the S32 router-miss
// contract: a composed tool name not present in the routing table
// surfaces as ErrorKindTool/ToolSubKindServerError via
// ToolResult.Err, with a nil framework error. Uses
// nullSessionOpener because the router is empty anyway (nil
// sessions yield no tools) — the test asserts the router-miss
// code path, not session dispatch. The CallID is echoed on
// the ToolResult.
func TestNewInvokeUnknownToolRoutes(t *testing.T) {
	t.Parallel()

	inv, err := New(context.Background(), []Server{validServer("probe")}, withSessionOpener(nullSessionOpener))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = inv.Close() }()

	result, frameworkErr := inv.Invoke(context.Background(), tools.InvocationContext{}, tools.ToolCall{
		CallID: "call-1",
		Name:   "probe__anything",
	})
	if frameworkErr != nil {
		t.Fatalf("Invoke framework error: got %v, want nil (router miss routes via ToolResult.Err)", frameworkErr)
	}
	if result.Status != tools.ToolStatusError {
		t.Errorf("Invoke status = %q, want %q", result.Status, tools.ToolStatusError)
	}
	if result.CallID != "call-1" {
		t.Errorf("Invoke CallID = %q, want %q", result.CallID, "call-1")
	}
	if result.Err == nil {
		t.Fatal("Invoke ToolResult.Err is nil; router miss must route failures via ToolResult.Err")
	}
	assertToolError(t, result.Err, "unknown tool", "probe__anything")
}

// TestNewServerListPinned asserts T30.4's "server list is pinned for
// the lifetime of the Invoker" requirement. Mutating the caller's
// slice after New returns must not affect the Invoker. Uses
// nullSessionOpener — this test is about slice-pinning, not
// session lifecycle.
func TestNewServerListPinned(t *testing.T) {
	t.Parallel()

	servers := []Server{validServer("probe")}
	inv, err := New(context.Background(), servers, withSessionOpener(nullSessionOpener))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = inv.Close() }()

	// Post-construction mutation: rename the caller's server.
	servers[0].LogicalName = "mutated-after-construction"

	// The internal copy owned by the Invoker should still carry the
	// original name. Read it back via the unexported field to pin
	// the invariant; this white-box test is the simplest way to
	// assert internal pinning before the public Invoker exposes a
	// reflective accessor (which it never will — the slice is
	// implementation-private by design).
	concrete, ok := inv.(*invoker)
	if !ok {
		t.Fatalf("New did not return the expected concrete type: %T", inv)
	}
	if got := concrete.servers[0].LogicalName; got != "probe" {
		t.Errorf("pinned server name: got %q, want %q (caller mutation leaked into Invoker)", got, "probe")
	}
}

// TestNewServerListValidation drives the full T30.4 + T30.6
// validation matrix as a table-driven test. Each case supplies a
// server slice (or transformer) and asserts the resulting error is
// a typed SystemError containing the expected message fragment.
func TestNewServerListValidation(t *testing.T) {
	t.Parallel()

	longName := strings.Repeat("x", 65)

	cases := []struct {
		name         string
		servers      []Server
		wantFragment string
	}{
		{
			name:         "empty slice",
			servers:      []Server{},
			wantFragment: "slice is empty",
		},
		{
			name:         "nil slice",
			servers:      nil,
			wantFragment: "slice is empty",
		},
		{
			name: "empty LogicalName",
			servers: []Server{
				{Transport: TransportStdio{Command: "mcp-test"}},
			},
			wantFragment: "LogicalName is empty",
		},
		{
			name: "LogicalName contains double underscore",
			servers: []Server{
				validServer("my__server"),
			},
			wantFragment: `reserved "__"`,
		},
		{
			name: "LogicalName too long",
			servers: []Server{
				validServer(longName),
			},
			wantFragment: "length 1-64",
		},
		{
			name: "LogicalName has illegal char (space)",
			servers: []Server{
				validServer("invalid name"),
			},
			wantFragment: "alphanumeric",
		},
		{
			name: "LogicalName has illegal char (slash)",
			servers: []Server{
				validServer("a/b"),
			},
			wantFragment: "alphanumeric",
		},
		{
			name: "nil Transport",
			servers: []Server{
				{LogicalName: "probe", Transport: nil},
			},
			wantFragment: "Transport is nil",
		},
		{
			name: "duplicate LogicalName",
			servers: []Server{
				validServer("probe"),
				validServer("probe"),
			},
			wantFragment: "duplicate LogicalName",
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			inv, err := New(context.Background(), c.servers)
			if inv != nil {
				t.Errorf("expected nil Invoker on validation failure, got %T", inv)
			}
			assertSystemError(t, err, c.wantFragment)
		})
	}
}

// TestNewServerCap asserts that the MaxServers cap is enforced at
// exactly the boundary: 32 servers is accepted, 33 is rejected with
// a typed error referencing the cap. Uses nullSessionOpener so the
// happy-path branch does not try to open 32 real sessions — this
// test is about boundary enforcement, not session lifecycle.
func TestNewServerCap(t *testing.T) {
	t.Parallel()

	// Boundary: exactly MaxServers entries — must succeed.
	boundary := make([]Server, MaxServers)
	for i := range boundary {
		boundary[i] = validServer(fmt.Sprintf("server-%d", i))
	}
	inv, err := New(context.Background(), boundary, withSessionOpener(nullSessionOpener))
	if err != nil {
		t.Errorf("New with %d servers (at cap): unexpected error: %v", MaxServers, err)
	}
	if inv != nil {
		_ = inv.Close()
	}

	// Over-cap: MaxServers + 1 — must fail with a typed error
	// before the opener is ever consulted (validation runs first).
	overCap := make([]Server, MaxServers+1)
	for i := range overCap {
		overCap[i] = validServer(fmt.Sprintf("server-%d", i))
	}
	inv2, err2 := New(context.Background(), overCap, withSessionOpener(nullSessionOpener))
	if inv2 != nil {
		t.Errorf("expected nil Invoker on over-cap, got %T", inv2)
	}
	assertSystemError(t, err2, "per-Invoker cap is 32")
	assertSystemError(t, err2, "33 entries")
}

// TestNewOptionNilIsIgnored asserts that a nil Option in the
// variadic list is silently ignored rather than panicking. This
// matches the default-safe posture of the With* constructors
// themselves (see options_test.go). Uses nullSessionOpener so the
// test does not depend on a real session-opening path.
func TestNewOptionNilIsIgnored(t *testing.T) {
	t.Parallel()

	inv, err := New(
		context.Background(),
		[]Server{validServer("probe")},
		nil,
		WithMaxResponseBytes(1024),
		nil,
		withSessionOpener(nullSessionOpener),
	)
	if err != nil {
		t.Fatalf("New: unexpected error with nil options: %v", err)
	}
	defer func() { _ = inv.Close() }()
	concrete, ok := inv.(*invoker)
	if !ok {
		t.Fatalf("New returned unexpected type %T", inv)
	}
	if got := concrete.cfg.maxResponseBytes; got != 1024 {
		t.Errorf("maxResponseBytes after mixed nil/valid options: got %d, want 1024", got)
	}
}

// TestMaxServersConstant pins the exported cap to its D115 value.
// A future change to this constant requires re-reading the Phase 4
// D60 cardinality budget before editing the test — that is the
// whole point of pinning it.
func TestMaxServersConstant(t *testing.T) {
	t.Parallel()
	if MaxServers != 32 {
		t.Errorf("MaxServers = %d, want 32 (D115 cardinality contract)", MaxServers)
	}
}
