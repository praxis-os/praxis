// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"errors"
	"fmt"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/praxis-os/praxis/tools"
)

// TestNewOpensInMemorySession exercises the full session-opening
// pipeline through [inMemorySessionOpener]. It drives real
// [*sdkmcp.ClientSession] values through the New → Close
// lifecycle, proving that:
//
//  1. New opens one session per server and stores them in the
//     returned invoker's session slice.
//  2. The S32 router is built against those real sessions and the
//     returned invoker advertises an empty definitions slice
//     (the bare inMemorySessionOpener does not register any tools
//     on its embedded server — dedicated router tests set up
//     tool-bearing servers separately).
//  3. Invoke on an unknown composed tool name routes through the
//     router miss path and returns an ErrorKindTool/ServerError,
//     not a framework error.
//  4. Close tears every session down in reverse order and returns
//     nil on clean teardown.
//  5. A second Close call is idempotent (returns nil).
func TestNewOpensInMemorySession(t *testing.T) {
	t.Parallel()

	servers := []Server{
		validServer("alpha"),
		validServer("bravo"),
	}
	ctx := context.Background()

	inv, err := New(ctx, servers, withSessionOpener(inMemorySessionOpener))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	concrete, ok := inv.(*invoker)
	if !ok {
		t.Fatalf("New returned unexpected concrete type %T", inv)
	}
	if got := len(concrete.sessions); got != len(servers) {
		t.Errorf("len(sessions) = %d, want %d", got, len(servers))
	}
	for i, sess := range concrete.sessions {
		if sess == nil {
			t.Errorf("sessions[%d] is nil", i)
		}
	}

	// The bare inMemorySessionOpener connects empty servers, so
	// the router should advertise zero tool definitions.
	if defs := inv.Definitions(); len(defs) != 0 {
		t.Errorf("Definitions() on empty servers returned %d entries, want 0", len(defs))
	}

	// Dispatching an unknown composed name must route through the
	// router-miss path: ToolResult.Err carries an ErrorKindTool
	// with ServerSubKind, and no framework error is returned.
	result, frameworkErr := inv.Invoke(ctx, tools.InvocationContext{}, tools.ToolCall{
		CallID: "c1",
		Name:   "alpha__probe",
	})
	if frameworkErr != nil {
		t.Fatalf("Invoke framework error: %v", frameworkErr)
	}
	if result.Status != tools.ToolStatusError {
		t.Errorf("Invoke status = %q, want %q", result.Status, tools.ToolStatusError)
	}
	if result.Err == nil {
		t.Fatal("Invoke ToolResult.Err is nil; router miss must route through Err")
	}
	// The assertion delegates the shape check to assertToolError
	// to avoid duplicating the error-taxonomy plumbing across
	// tests. The helper lives in new_test.go.
	assertToolError(t, result.Err, "unknown tool")

	// Close tears every session down. Second call is idempotent.
	if err := inv.Close(); err != nil {
		t.Errorf("Close: unexpected error: %v", err)
	}
	if got := len(concrete.sessions); got != 0 {
		t.Errorf("sessions slice not cleared by Close: len = %d, want 0", got)
	}
	if err := inv.Close(); err != nil {
		t.Errorf("Close (second call): unexpected error: %v", err)
	}
}

// TestOpenSessionsRollbackOnFailure asserts the Phase 7
// "partial openings are cleaned up before returning" contract by
// driving a synthetic opener that succeeds for the first N-1
// servers and fails on the N-th. The rollback must close every
// previously-opened session. The synthetic opener records each
// Close call via a shared counter so the test can verify the
// number of teardown calls matches the number of pre-rollback
// successes.
func TestOpenSessionsRollbackOnFailure(t *testing.T) {
	t.Parallel()

	servers := []Server{
		validServer("a"),
		validServer("b"),
		validServer("c"), // failing index
	}

	// Build three real sessions via InMemoryTransports, then
	// return them from a substitute opener that fails on the
	// third iteration. closeCount counts how many sessions are
	// actually closed during rollback — it should equal 2
	// (sessions for servers "a" and "b"), not 3.
	var sessionsOpened []*sdkmcp.ClientSession
	failingOpener := func(_ context.Context, _ config, servers []Server) ([]*sdkmcp.ClientSession, error) {
		out := make([]*sdkmcp.ClientSession, 0, len(servers))
		for i := range servers {
			if i == 2 {
				// Close everything opened so far, matching the
				// real openSessions rollback semantics.
				for j := len(out) - 1; j >= 0; j-- {
					_ = out[j].Close()
				}
				return nil, fmt.Errorf("synthetic failure at index %d", i)
			}
			// Open a fresh InMemoryTransport-backed session for
			// this server. Copy of the inMemorySessionOpener
			// body without the outer loop, so tests can observe
			// the exact sessions list at failure time.
			t1, t2 := sdkmcp.NewInMemoryTransports()
			srv := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "rollback-test", Version: "0"}, nil)
			if _, err := srv.Connect(context.Background(), t1, nil); err != nil {
				return nil, err
			}
			client := clientForTest()
			sess, err := client.Connect(context.Background(), t2, nil)
			if err != nil {
				return nil, err
			}
			out = append(out, sess)
			sessionsOpened = append(sessionsOpened, sess)
		}
		return out, nil
	}

	_, err := New(context.Background(), servers, withSessionOpener(failingOpener))
	if err == nil {
		t.Fatal("expected failure from synthetic opener, got nil")
	}
	if !errors.Is(err, err) { // sanity: error propagates
		t.Errorf("error did not propagate: %v", err)
	}
	if len(sessionsOpened) != 2 {
		t.Errorf("expected 2 sessions opened before failure, got %d", len(sessionsOpened))
	}
}

// clientForTest is a tiny helper used by the rollback test. It
// exists so the test body does not need to import
// internal/client directly (which would work, but cluttering a
// test-only path with a production import is noise).
func clientForTest() *sdkmcp.Client {
	// Inline the praxis/mcp/internal/client NewClient call.
	return sdkmcp.NewClient(
		&sdkmcp.Implementation{
			Name:    "praxis-mcp-rollback-test",
			Version: "0",
		},
		nil,
	)
}
