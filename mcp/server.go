// SPDX-License-Identifier: Apache-2.0

package mcp

// CredentialRef is the caller-supplied name of a credential that the
// adapter will resolve through the configured resolver when opening a
// session to an MCP server.
//
// CredentialRef is a plain string alias, not a distinct type. The
// core `credentials.Resolver.Fetch(ctx, name string)` signature is
// frozen at v1.0, so there is no richer type to wrap. The alias
// exists so that declarations of [Server.CredentialRef] read as a
// credential reference rather than an arbitrary string, and so that
// any future evolution of the concept (e.g., promoting it to a
// struct with scope metadata) is a purely local source-level change
// inside this package.
//
// An empty CredentialRef means the session opens unauthenticated.
type CredentialRef = string

// Server describes a single MCP server the adapter will connect to.
//
// Server values are consumed by [New] and retained internally for the
// lifetime of the returned [Invoker]. Each Server's transport opens
// at most one MCP session; parallel tool dispatch to the same server
// is serialised inside the adapter if the underlying MCP client is
// not safe for concurrent use.
//
// Server is a pure value type with no hidden state. Zero values are
// rejected by the validator invoked inside [New].
//
// Stability: stable-v0.x-candidate. Frozen at praxis/mcp v1.0.0.
type Server struct {
	// LogicalName is the caller-chosen identifier for this server.
	//
	// Used as the left half of the namespaced tool name exposed to
	// the LLM (see D111): an MCP tool `list_issues` behind a server
	// with LogicalName `"github"` is surfaced as `github__list_issues`.
	//
	// Validation rules enforced at [New] time (see validateServers):
	//
	//   - Must be non-empty.
	//   - Length must be in [1, 64].
	//   - Must match `^[a-zA-Z0-9_-]+$`.
	//   - Must not contain the substring `__` (reserved as the
	//     namespace delimiter).
	//   - Must be unique across the Server slice passed to [New];
	//     duplicates are a construction error.
	LogicalName string

	// Transport selects the MCP transport binding for this server.
	//
	// Transport is a sealed interface; only the concrete types
	// declared in this package ([TransportStdio], [TransportHTTP])
	// are valid values. A nil Transport is rejected at [New] time.
	Transport Transport

	// CredentialRef, if non-empty, names a credential that the
	// adapter will resolve via the configured resolver and use to
	// authenticate the MCP session.
	//
	// Interpretation per transport:
	//
	//   - [TransportStdio]: the resolved credential bytes are placed
	//     into the child process environment under the variable name
	//     given by [TransportStdio.CredentialEnv] before launch.
	//   - [TransportHTTP]: the resolved credential is injected as a
	//     `Bearer` token in the Authorization header on every request.
	//
	// An empty CredentialRef means the session opens unauthenticated.
	// Full credential lifecycle contract lives in the Phase 7
	// security-and-credentials document (Phase 7 §4).
	CredentialRef CredentialRef
}
