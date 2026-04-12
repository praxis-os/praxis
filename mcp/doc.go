// SPDX-License-Identifier: Apache-2.0

// Package mcp provides a Model Context Protocol adapter for praxis.
//
// The mcp sub-module adapts published MCP servers into the praxis
// [tools.Invoker] surface so that agents driven by
// [praxis.AgentOrchestrator] can invoke MCP-exposed tools without
// any runtime plugin loading. Integration is build-time only:
// callers construct an [Invoker] from a pinned list of [Server]
// specifications at [New] time, and the orchestrator then dispatches
// tool calls through the standard tools option.
//
// The sub-module ships independently from praxis core at its own
// semver line under the tag prefix `mcp/vX.Y.Z`; see the module's
// CHANGELOG for release history. Public types are at
// `stable-v0.x-candidate` until `mcp/v1.0.0`.
//
// # Public surface
//
// The exported API is intentionally small (see D110):
//
//   - [Server] — a pure value type describing a single MCP server.
//   - [Transport] — a sealed interface; only [TransportStdio] and
//     [TransportHTTP] are valid concrete values. Consumer-supplied
//     transports are NOT permitted in v1.0.0.
//   - [Invoker] — the public handle returned by [New]; embeds
//     [tools.Invoker] (frozen-v1.0) and [io.Closer].
//   - [New] — the constructor. Validates the server list against
//     the T30.4 + T30.6 rules, applies options in order, and
//     returns an Invoker.
//   - [Option] and the four With* constructors: [WithResolver],
//     [WithMetricsRecorder], [WithTracerProvider],
//     [WithMaxResponseBytes].
//   - [MaxServers] and [DefaultMaxResponseBytes] — exported
//     constants documenting the contract pins.
//
// Everything else — session pooling, transport framing, SDK
// integration, span and metric emission — lives under the `internal/`
// tree and is not part of the public surface.
//
// # Minimal wiring example
//
// The following snippet constructs an Invoker fronting a single
// stdio-based MCP server and plugs it into a praxis orchestrator:
//
//	servers := []mcp.Server{
//	    {
//	        LogicalName: "github",
//	        Transport:   mcp.TransportStdio{Command: "mcp-github"},
//	    },
//	}
//	inv, err := mcp.New(ctx, servers,
//	    mcp.WithResolver(myResolver),
//	    mcp.WithMaxResponseBytes(8*1024*1024),
//	)
//	if err != nil {
//	    return fmt.Errorf("mcp.New: %w", err)
//	}
//	defer inv.Close()
//
//	orch := praxis.NewOrchestrator(
//	    provider,
//	    praxis.WithInvoker(inv),
//	)
//
// The `inv` value is a [tools.Invoker] plus an [io.Closer] and
// requires no praxis-owned glue: the orchestrator routes every
// namespaced tool call (`github__list_issues`, …) through it
// identically to any other tool invoker.
//
// # PostToolFilter empty-content contract (D114)
//
// Callers who install a [hooks.PostToolFilter] against an
// orchestrator configured with an MCP [Invoker] MUST treat
// `(Status == ToolStatusSuccess, Content == "")` as a valid
// successful outcome, not as an error or anomaly. The adapter's
// content flattening pass (D114) discards non-text content blocks
// (image, audio, resource references) and joins remaining text
// blocks with a newline separator; a response whose content array
// is purely non-text produces an empty string after flattening.
// The adapter deliberately preserves `ToolStatusSuccess` in this
// case because the MCP server completed the tool call without
// error — the text projection is simply empty.
//
// Filter implementations that scan [tools.ToolResult.Content] for
// prompt-injection markers or apply redaction MUST handle the
// empty-string input path and return a decision consistent with
// their policy. Treating empty content as a filter failure would
// spuriously reject tool calls against multi-modal MCP servers.
//
// This contract is specific to praxis/mcp and is NOT a core praxis
// commitment — the core orchestrator does not produce
// `ToolStatusSuccess + empty Content` from any of its built-in
// tools.Invoker implementations as of v0.5.0. Filter authors who
// wire MCP Invokers alongside core invokers must account for this
// combination in their filter logic.
//
// # Known limitation OI-MCP-1: residual credential bytes in Go strings
//
// The adapter enforces the Phase 5 D67 zero-on-close contract on
// every byte slice it owns that has ever held credential material:
// after a credential is delivered to an MCP session, the adapter
// calls [credentials.ZeroBytes] on its copy and calls
// [credentials.Credential.Close] on the Resolver-owned copy.
//
// There is one place where this zeroing contract cannot be
// enforced, by design of the Go language itself: the Go string
// value that the adapter hands to the SDK for stdio env variables
// and for HTTP `Authorization: Bearer` headers.
//
// The stdio path, in particular, must assign the credential
// material into [os/exec.Cmd.Env], which is a `[]string`. The
// assignment requires a `string(credentialBytes)` conversion, and
// Go strings are immutable — once the conversion has happened, the
// adapter cannot zero the bytes the string points at. The string
// lives in the Go runtime heap until the next garbage collection
// pass collects it, which is typically within a few seconds of the
// adapter dropping its last reference but is not observable from
// praxis code.
//
// The HTTP path has the same shape: the bearer token appears as
// the value in an [net/http.Header] map, which is also a
// `[]string`. The adapter's round-tripper holds the token only for
// the lifetime of the session, but a residual copy of the string
// lives in the heap until GC.
//
// **OI-MCP-1 is the stable identifier for this residual risk.** It
// is documented here rather than hidden because Phase 7 §4.2 and
// §4.3 explicitly call out this boundary as a known imperfect
// zeroing point that v1.0.0 accepts. The adapter's mitigation is to
// minimise the lifetime of credential strings: the `[]byte` source
// buffer is zeroed immediately after the string conversion, so the
// string is the only place the material exists in praxis process
// memory; the SDK's session teardown releases its own internal
// references; and a short GC cycle afterwards collects the
// residual string.
//
// Callers deploying praxis/mcp in environments where even a
// GC-bounded residual credential copy in Go heap is unacceptable
// MUST either avoid using credentialed MCP servers entirely or
// build their own adapter with a cgo-backed secret-memory allocator
// that bypasses Go's string semantics. No such allocator ships in
// v1.0.0.
//
// The full residual-risk discussion lives in Phase 7
// `docs/phase-7-mcp-integration/04-security-and-credentials.md §6`
// and the adapter-level zeroing implementation lives in
// `mcp/internal/transport/credentials.go`.
//
// # Trust boundary classification (D116)
//
// The MCP transport edge — the boundary between the praxis process
// and any external MCP server — is classified as a Phase 5
// untrusted-output boundary. This means:
//
//   - Every [tools.ToolResult.Content] produced by the MCP adapter
//     is untrusted by default and MUST pass through the caller's
//     [hooks.PostToolFilter] before being treated as trusted output
//     in the conversation history.
//   - The adapter does NOT introduce any new filter, hook, or trust
//     tier. The existing Phase 5 D77/D78 contracts apply verbatim
//     to MCP-sourced results.
//   - The adapter does NOT forward [tools.InvocationContext.SignedIdentity]
//     to any MCP server. See D118 for the rationale. The JWT is
//     never written to any MCP HTTP header, stdio environment
//     variable, or JSON-RPC frame.
//
// Callers who deploy MCP servers from untrusted sources MUST
// install a PostToolFilter that inspects the flattened Content for
// prompt-injection markers, just as they would for any other
// untrusted tool output. The adapter's content flattening (D114)
// produces a plain-text string that is friendly to text-pattern
// based injection detectors.
//
// # Known limitation OI-MCP-2: HTTP goroutine-scope isolation breach
//
// Phase 5 `02-credential-lifecycle.md` §3.2 states the credential
// goroutine-scope isolation invariant: the Credential value is used
// only within the goroutine that received it from Resolver.Fetch.
// The HTTP transport path in the MCP adapter breaches this invariant:
// the bearer-token string is handed to the underlying HTTP client
// library, which maintains connection pools and keep-alive goroutines
// that read the Authorization header during connection-reuse.
//
// The breach is structural and unavoidable — any HTTP client that
// supports connection reuse will read auth headers from a background
// goroutine. It is accepted as an architectural consequence of
// supporting HTTP-backed MCP sessions and is classified at the same
// "acceptable risk" tier as OI-MCP-1 (residual credential bytes in
// Go strings).
//
// **OI-MCP-2 is the stable identifier for this deviation.** It is
// documented in the D117 amendment (2026-04-10) in the Phase 7
// decisions log and referenced in SECURITY.md. Callers with strict
// goroutine-scope isolation requirements should use stdio transport
// or build a custom HTTP adapter with KMS-backed proxy tokens.
//
// # Phase 7 scope (D106–D121)
//
//   - Transports: stdio and Streamable HTTP (D108).
//   - Tool namespacing: `{LogicalName}__{mcpToolName}` (D111).
//   - Budget participation via the existing `tool_calls` and
//     `wall_clock` dimensions — no new budget dimension, no
//     double-counting (D112).
//   - Error translation to the core `errors.ErrorKindTool` taxonomy
//     (D113).
//   - Content flattening: text-only, newline-joined (D114).
//   - Trust boundary: the MCP transport edge is classified
//     untrusted; [hooks.PostToolFilter] runs on every response
//     before it is treated as trusted output (Phase 5 D77/D78,
//     Phase 7 D116).
//   - No runtime plugin discovery (D09 / D109): all servers are
//     pinned at construction time.
//
// See the Phase 7 design documents under
// `docs/phase-7-mcp-integration/` in the praxis repository for the
// full decision log.
package mcp
