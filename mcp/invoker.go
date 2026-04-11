// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"io"

	"github.com/praxis-os/praxis/tools"
)

// Invoker is the public handle returned by [New]. It is the sole
// exported interface the caller interacts with after construction.
//
// Invoker extends the Phase 3 frozen [tools.Invoker] interface with
// an [io.Closer] contract, so the value returned by [New] plugs
// directly into the orchestrator's tool-dispatch surface while also
// carrying a well-defined shutdown path for the MCP sessions it owns.
//
// An Invoker is safe for concurrent use by multiple goroutines. The
// orchestrator routes parallel tool-call dispatches through
// [tools.Invoker.Invoke], and the adapter serialises access to any
// single MCP session internally when the underlying client is not
// documented as concurrency-safe. Calls to different servers proceed
// concurrently.
//
// # Close semantics
//
// Calling [io.Closer.Close] tears down every open MCP session owned
// by this Invoker:
//
//   - Stdio sessions have their child process terminated (SIGTERM,
//     then SIGKILL on a fixed grace window).
//   - HTTP sessions have their transport closed and in-flight
//     requests cancelled via context.
//
// Close is idempotent: calling it more than once returns nil on
// every call after the first. Close MUST be called when the Invoker
// is no longer needed; a leaked stdio Invoker leaves a zombie child
// process attached to the parent's process group.
//
// Close does NOT need to be called from the same goroutine as the
// last [tools.Invoker.Invoke] call: the adapter coordinates shutdown
// via its internal locks.
//
// # Stability
//
// The embedded [tools.Invoker] interface is `frozen-v1.0` (Phase 3
// D31). The overall Invoker interface declared here is
// `stable-v0.x-candidate` — it freezes at praxis/mcp v1.0.0. Adding
// new methods to this interface between v0.7.0 and v1.0.0 is
// permitted under the sub-module's independent semver line; removing
// or changing existing methods is not.
type Invoker interface {
	// tools.Invoker: see the core package for the frozen contract.
	// The adapter routes every tool call through this method,
	// mapping namespaced tool names (`{LogicalName}__{mcpToolName}`)
	// back to the originating server before dispatch.
	tools.Invoker

	// io.Closer: tears down every MCP session owned by this Invoker.
	io.Closer
}
