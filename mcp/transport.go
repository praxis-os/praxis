// SPDX-License-Identifier: Apache-2.0

package mcp

// Transport describes the wire-level binding that an MCP [Server] uses
// to communicate with its peer. It is a sealed interface: the only
// valid values are the concrete [TransportStdio] and [TransportHTTP]
// types declared in this package.
//
// The interface is sealed via the unexported [Transport.isMCPTransport]
// marker method. Consumers cannot supply their own Transport
// implementations in v1.0.0.
//
// # Rationale
//
// A sealed Transport interface is a deliberate scope restriction tied
// to Phase 7 D108 (stdio + Streamable HTTP only) and D110 (minimal
// public API surface). It carries three benefits:
//
//  1. The audit story is tight: every MCP session the adapter opens
//     is backed by one of two transport implementations that ship in
//     this package and are covered by the module's own test suite.
//     There is no consumer-written transport that could exfiltrate
//     credentials or mis-implement the JSON-RPC framing.
//
//  2. The trust-boundary classification (Phase 5) is exhaustive.
//     Adding a new transport (e.g., WebSocket) requires a
//     source-level change in this module with corresponding
//     updates to the security review, so the transport edge and
//     its trust semantics always evolve together.
//
//  3. Callers that need a different transport model build their own
//     [tools.Invoker] that wraps the public [Invoker] surface — they
//     are not blocked, just routed away from the adapter's internal
//     machinery.
//
// Adding a new transport type (e.g., TransportWebSocket) is a
// source-level change in this module, a new Phase 7 decision, and a
// minor version bump of praxis/mcp.
//
// Stability: stable-v0.x-candidate. Frozen at praxis/mcp v1.0.0.
type Transport interface {
	// isMCPTransport is an unexported sentinel method that seals the
	// interface. Only types declared in this package can satisfy it.
	// Implementations return no value; the method is invoked only by
	// compile-time interface satisfaction checks.
	isMCPTransport()
}

// TransportStdio launches the MCP server as a child process and
// communicates over its stdin/stdout pipes using the JSON-RPC
// framing defined by the MCP specification.
//
// TransportStdio is a pure value type. All fields are consumed at
// [New] time; mutating a TransportStdio value after passing it to New
// has no effect on the running session.
//
// Stability: stable-v0.x-candidate. Frozen at praxis/mcp v1.0.0.
type TransportStdio struct {
	// Command is the executable to launch. It must be either an
	// absolute path or a bare name resolvable via `exec.LookPath` at
	// [New] time. Relative paths are rejected.
	//
	// An empty Command is a construction error.
	Command string

	// Args are the command-line arguments passed to the child
	// process. A nil or empty slice is permitted — the child is then
	// invoked with Command as argv[0] only.
	//
	// The adapter passes Args through verbatim. Callers are
	// responsible for not embedding secret material in Args; see
	// [TransportStdio.CredentialEnv] for the supported credential
	// delivery channel.
	Args []string

	// Env is a fixed environment-variable map merged into the child
	// process environment at launch time. Framework-injected credential
	// values are merged on top of Env: if [Server.CredentialRef] is
	// non-empty, the resolved credential's bytes are placed in the
	// environment under the name [CredentialEnv], overwriting any
	// pre-existing entry with the same key.
	//
	// A nil Env is permitted and means "no extra environment".
	Env map[string]string

	// CredentialEnv is the name of the environment variable under
	// which the resolved credential's Value() bytes are placed before
	// the child process is launched.
	//
	// Ignored if [Server.CredentialRef] is empty.
	//
	// If [Server.CredentialRef] is non-empty and CredentialEnv is
	// empty, construction fails with a validation error: the adapter
	// refuses to silently drop credential material.
	CredentialEnv string
}

// isMCPTransport seals the [Transport] interface. TransportStdio is
// one of the two valid transport types.
func (TransportStdio) isMCPTransport() {}

// TransportHTTP connects to an MCP server over the Streamable HTTP
// transport, which the official SDK abstracts over Server-Sent Events
// as well.
//
// TransportHTTP is a pure value type. All fields are consumed at
// [New] time; mutating a TransportHTTP value after passing it to New
// has no effect on the running session.
//
// Stability: stable-v0.x-candidate. Frozen at praxis/mcp v1.0.0.
type TransportHTTP struct {
	// URL is the fully qualified MCP endpoint URL. An empty URL is a
	// construction error. The adapter parses URL with net/url at
	// [New] time and rejects malformed values.
	URL string

	// Header is a map of fixed HTTP headers set on every request
	// issued to this MCP server.
	//
	// If [Server.CredentialRef] is non-empty, the adapter adds an
	// `Authorization: Bearer <value>` header derived from the
	// resolved credential on every request. Callers MUST NOT
	// pre-set an Authorization key in Header; doing so is a
	// construction error (the adapter refuses to choose between
	// the caller-supplied and credential-derived headers).
	//
	// A nil Header is permitted and means "no extra headers".
	Header map[string]string
}

// isMCPTransport seals the [Transport] interface. TransportHTTP is
// one of the two valid transport types.
func (TransportHTTP) isMCPTransport() {}

// Compile-time interface-satisfaction checks for the sealed set.
var (
	_ Transport = TransportStdio{}
	_ Transport = TransportHTTP{}
)
