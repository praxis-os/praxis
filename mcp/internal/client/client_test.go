// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"sync"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestNewClientShape asserts that the [NewClient] wrapper returns a
// non-nil [sdkmcp.Client] configured with the praxis adapter's
// identity. It is a pure smoke test: the returned client is not
// connected to any transport.
func TestNewClientShape(t *testing.T) {
	t.Parallel()

	c := NewClient()
	if c == nil {
		t.Fatal("NewClient returned nil")
	}
	// sdkmcp.Client does not expose its Implementation back through
	// an accessor, so the best this test can do without reflection
	// is assert non-nil. The identity string is checked indirectly
	// by TestSDKInMemorySessionLifecycle, which succeeds only if
	// the client can run the full initialize handshake.
}

// TestSDKInMemorySessionLifecycle exercises the end-to-end client
// ↔ server handshake over [sdkmcp.NewInMemoryTransports] so that
// this package's CI proves, on every run, that:
//
//  1. The upstream SDK's in-memory transport pair is reachable from
//     inside praxis/mcp. If a future SDK refactor renames or
//     removes NewInMemoryTransports, this test fails the build
//     immediately rather than at S31 PR-B integration time.
//  2. The praxis [NewClient] wrapper produces a functional
//     [sdkmcp.Client] that completes the MCP `initialize` handshake
//     against a fresh [sdkmcp.Server]. The server's
//     InitializedHandler firing is the wire-level proof.
//  3. Both sides close cleanly: the client closes its session, the
//     server waits for its session to end and returns without
//     error.
//
// This is the T31.6 "use InMemoryTransport as adapter test
// substrate" requirement, scoped to PR-A. The Phase 7
// 03-integration-model.md §8a explicitly prefers using the SDK's
// InMemoryTransport over a praxis-owned fake for the happy-path,
// error-mapping, and content-flattening tests.
//
// PR-B (stdio + HTTP transport implementations) will add further
// tests that use InMemoryTransport to exercise the adapter's
// production dispatch path; this test exists to pin the substrate
// itself so those later tests have a stable known-good baseline.
func TestSDKInMemorySessionLifecycle(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// Use a sync.WaitGroup so the assertion on InitializedHandler
	// firing is deterministic — the handler runs on a separate
	// goroutine inside the SDK.
	var (
		initWG        sync.WaitGroup
		initHandlerOK bool
	)
	initWG.Add(1)

	client := NewClient()
	server := sdkmcp.NewServer(
		&sdkmcp.Implementation{Name: "praxis-mcp-test-server", Version: "test-0"},
		&sdkmcp.ServerOptions{
			InitializedHandler: func(_ context.Context, _ *sdkmcp.InitializedRequest) {
				initHandlerOK = true
				initWG.Done()
			},
		},
	)

	// Wire the server first so it is ready to receive the client's
	// `initialize` message as soon as the client connects. This
	// ordering matches the SDK example lifecycle documented at
	// github.com/modelcontextprotocol/go-sdk@v1.5.0 design/design.md.
	t1, t2 := sdkmcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, t1, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	clientSession, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}

	// Wait for the server's InitializedHandler to fire. If the
	// handshake failed (or the SDK's event plumbing regressed),
	// this blocks forever and the go test timeout catches it.
	initWG.Wait()
	if !initHandlerOK {
		t.Fatal("InitializedHandler was not invoked by the server session")
	}

	// Clean shutdown. The client closes its session; the server
	// then waits for its own session to drain. Any non-nil error
	// on either side is a regression.
	if err := clientSession.Close(); err != nil {
		t.Errorf("clientSession.Close: %v", err)
	}
	if err := serverSession.Wait(); err != nil {
		t.Errorf("serverSession.Wait: %v", err)
	}
}
