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
