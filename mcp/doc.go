// SPDX-License-Identifier: Apache-2.0

// Package mcp provides a Model Context Protocol adapter for praxis.
//
// The mcp sub-module adapts published MCP servers into the praxis
// [tools.Invoker] surface so that agents driven by [praxis.AgentOrchestrator]
// can invoke MCP-exposed tools without any runtime plugin loading.
// Integration is build-time only: callers construct an [Invoker] from a
// pinned list of [Server] specifications and pass it to the orchestrator
// via the standard tools option.
//
// The sub-module ships independently from praxis core at its own semver
// line under the tag prefix `mcp/vX.Y.Z`; see the module's CHANGELOG.md
// for release history. Public types are at `stable-v0.x-candidate` until
// `mcp/v1.0.0`.
//
// Scope (Phase 7, D106–D121):
//
//   - Transports: stdio and Streamable HTTP (D108).
//   - Tool namespacing: `{LogicalName}__{mcpToolName}` (D111).
//   - Budget participation via the existing `tool_calls` and `wall_clock`
//     dimensions — no new budget dimension, no double-counting (D112).
//   - Error translation to the core `errors.ErrorKindTool` taxonomy (D113).
//   - Trust boundary: the MCP transport edge is classified untrusted;
//     `PostToolFilter` runs on every response before it is treated as
//     trusted output (Phase 5 D77/D78, Phase 7 D116).
//   - No runtime plugin discovery (D09 / D109): all servers are pinned
//     at construction time.
//
// See the Phase 7 design documents under `docs/phase-7-mcp-integration/`
// in the praxis repository for the full decision log.
package mcp
