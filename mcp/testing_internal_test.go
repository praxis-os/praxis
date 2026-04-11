// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/praxis-os/praxis/mcp/internal/client"
)

// This file declares test-only seams inside the mcp package. It
// is compiled under `go test` only (_test.go suffix) and never
// ships in production binaries. Every identifier here is
// intentionally unexported: external callers cannot reach these
// hooks, so production wiring cannot drift into them by accident.

// withSessionOpener is a package-internal Option that installs a
// substitute [sessionOpener] on the config the [New] constructor
// consumes. It is the single seam through which white-box tests
// bypass the real SDK-backed session-opening path in
// [openSessions].
//
// Production code MUST NOT depend on the existence of this
// option: it lives in a _test.go file and the compiler excludes
// it from non-test builds. External packages cannot reference it
// because both the option and the sessionOpener type are
// unexported.
func withSessionOpener(f sessionOpener) Option {
	return func(c *config) {
		c.opener = f
	}
}

// nullSessionOpener is a [sessionOpener] that returns a slice of
// nil [*sdkmcp.ClientSession] pointers, one per input server. It
// is used by tests that exercise the validation, option-binding,
// and Close paths but do not need real MCP sessions to assert
// their invariants.
//
// The returned slice length matches `len(servers)` so the
// len-alignment invariant in [buildRouter] holds. The nil entries
// are tolerated by [buildRouter] (no ListTools call on a nil
// session), by [closeSessions] (nil entries are skipped), and by
// [invoker.Invoke] (a nil-session dispatch surfaces as a typed
// internal-invariant error). Tests that need real sessions use
// [inMemorySessionOpener] instead.
func nullSessionOpener(_ context.Context, _ config, servers []Server) ([]*sdkmcp.ClientSession, error) {
	return make([]*sdkmcp.ClientSession, len(servers)), nil
}

// inMemorySessionOpener returns a [sessionOpener] that, for each
// server in the input slice, sets up an in-process MCP server via
// [sdkmcp.NewServer] + [sdkmcp.NewInMemoryTransports] and connects
// the praxis-owned [client.NewClient] wrapper to it. Every
// returned session is backed by the SDK's own in-memory transport
// pair, so the full MCP initialize handshake runs end-to-end
// without spawning child processes or opening network sockets.
//
// The opener is used by white-box session-lifecycle tests that
// need to exercise [closeSessions] and the partial-failure
// rollback semantics of [openSessions] against sessions that
// actually respond to the MCP protocol.
//
// Each returned session owns a background goroutine inside the
// SDK. Tests MUST call the invoker's Close method to drain them;
// otherwise the Go test runner's leak detector (in -race mode)
// may report goroutines left over at the end of the test.
func inMemorySessionOpener(_ context.Context, _ config, servers []Server) ([]*sdkmcp.ClientSession, error) {
	sessions := make([]*sdkmcp.ClientSession, 0, len(servers))
	for range servers {
		t1, t2 := sdkmcp.NewInMemoryTransports()

		// Run a fresh in-process server on the t1 side.
		srv := sdkmcp.NewServer(
			&sdkmcp.Implementation{Name: "praxis-mcp-test", Version: "0"},
			nil,
		)
		// Use a background context so the session outlives the
		// current test's parent context cancellation; closure of
		// the session on teardown is driven by the invoker's
		// Close call, not by context cancellation here.
		if _, err := srv.Connect(context.Background(), t1, nil); err != nil {
			return nil, err
		}

		c := client.NewClient()
		sess, err := c.Connect(context.Background(), t2, nil)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, sess)
	}
	return sessions, nil
}
