// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/praxis-os/praxis/llm"
)

// toolRoute is the routing-table entry for a single composed tool
// name. It binds the composed public name back to the session that
// owns the tool and the raw MCP tool name to pass to
// [sdkmcp.ClientSession.CallTool].
type toolRoute struct {
	// sessionIdx is the index into [invoker.sessions] (and, by
	// construction, into [invoker.servers]) of the MCP server that
	// advertised this tool at handshake.
	sessionIdx int

	// rawName is the tool name as advertised by the MCP server. It
	// is the value passed verbatim as [sdkmcp.CallToolParams.Name].
	rawName string
}

// router is the adapter's tool-dispatch and description surface.
//
// The router is constructed once at [New] time by [buildRouter] and
// then referenced read-only for the rest of the [Invoker]'s
// lifetime. It carries two data structures built from the same
// source (`session.Tools`):
//
//   - routes: a composed-name → [toolRoute] map used by
//     [invoker.Invoke] to decode a namespaced tool name from the
//     LLM back into the `(session, raw name)` tuple needed for
//     SDK dispatch.
//   - defs: the cached slice of [llm.ToolDefinition] values the
//     caller threads into `llm.Request.Tools`, returned by
//     [invoker.Definitions] by reference.
//
// Both structures are sorted deterministically (by composed name)
// so that repeated runs against the same server set produce
// byte-identical output — a prerequisite for reproducible LLM
// test fixtures.
//
// The router is never mutated after construction. Callers hold it
// behind the invoker's existing locks; no additional synchronisation
// is required.
type router struct {
	routes map[string]toolRoute
	defs   []llm.ToolDefinition
}

// lookup returns the routing entry for a composed tool name, or
// (zero-value, false) if no such tool is known to this adapter.
// It is a thin helper used by [invoker.Invoke] so the error path
// stays localised.
func (r *router) lookup(composedName string) (toolRoute, bool) {
	if r == nil {
		return toolRoute{}, false
	}
	rt, ok := r.routes[composedName]
	return rt, ok
}

// buildRouter enumerates the tools advertised by each open MCP
// session and produces the routing table + description slice the
// adapter exposes for the rest of its lifetime.
//
// Inputs are index-aligned: `servers[i]` is the [Server] whose
// session is `sessions[i]`. The caller (New) guarantees this
// invariant. The error paths never mutate either slice — session
// teardown on failure is the caller's responsibility, matching the
// partial-openings rollback contract from
// `03-integration-model.md §2`.
//
// For every advertised tool `t` on session `i`, buildRouter:
//
//  1. Composes `composed := servers[i].LogicalName + "__" + t.Name`
//     per D111.
//  2. If `composed` is already in the routing table, returns a
//     typed [praxiserrors.SystemError] whose message names both
//     colliding servers (LogicalName + index) and the conflicting
//     composed tool name. See §3.4 of `03-integration-model.md`
//     for the "typed error, not panic" rationale.
//  3. Records `routes[composed] = {sessionIdx: i, rawName: t.Name}`.
//  4. Appends a single [llm.ToolDefinition] with the composed name
//     as `Name`, the server-advertised `Description`, and the
//     JSON-marshalled `InputSchema`. The schema is marshalled
//     exactly once per tool; the adapter never re-interprets it.
//
// On any error from `session.Tools` iteration or from JSON
// marshalling, buildRouter wraps the underlying error as a typed
// [praxiserrors.SystemError] (with the offending server index and
// LogicalName for quick triage) and returns immediately — further
// sessions are not enumerated.
//
// On success, the returned router's `defs` slice is sorted by
// composed name and the routing map is populated. The slices may be
// empty if every connected server advertised zero tools; that is
// a valid end state.
func buildRouter(ctx context.Context, servers []Server, sessions []*sdkmcp.ClientSession) (*router, error) {
	if len(servers) != len(sessions) {
		return nil, systemError(fmt.Sprintf(
			"buildRouter: internal invariant violated: len(servers)=%d != len(sessions)=%d",
			len(servers), len(sessions),
		))
	}

	routes := make(map[string]toolRoute)
	defs := make([]llm.ToolDefinition, 0)

	for i, s := range servers {
		session := sessions[i]
		if session == nil {
			// Test hooks (see testing_internal_test.go's
			// nullSessionOpener) return a slice of nil pointers when
			// they don't need real sessions to assert their invariants.
			// Production [openSessions] never returns nil entries —
			// a nil in a production slice is a framework bug that
			// [invoker.Invoke] surfaces as a typed internal-invariant
			// error at dispatch time. Here in buildRouter we simply
			// skip: no live session means no tools to enumerate and
			// no routes to record for this server.
			continue
		}

		for tool, err := range session.Tools(ctx, nil) {
			if err != nil {
				return nil, systemError(fmt.Sprintf(
					"buildRouter: servers[%d] (%q): list tools: %v",
					i, s.LogicalName, err,
				))
			}
			if tool == nil {
				// Defensive: the SDK iterator yields (nil, err) on
				// the error path; a (nil, nil) pair would be a bug
				// upstream but costs nothing to guard against.
				continue
			}

			composed := s.LogicalName + logicalNameSeparator + tool.Name

			if existing, ok := routes[composed]; ok {
				other := servers[existing.sessionIdx].LogicalName
				return nil, systemError(fmt.Sprintf(
					"buildRouter: composed tool name %q is advertised by both servers[%d] (%q) and servers[%d] (%q); tool names must be unique after composition (D111)",
					composed,
					existing.sessionIdx, other,
					i, s.LogicalName,
				))
			}

			schemaBytes, err := marshalInputSchema(tool.InputSchema)
			if err != nil {
				return nil, systemError(fmt.Sprintf(
					"buildRouter: servers[%d] (%q): marshal input schema for tool %q: %v",
					i, s.LogicalName, tool.Name, err,
				))
			}

			routes[composed] = toolRoute{sessionIdx: i, rawName: tool.Name}
			defs = append(defs, llm.ToolDefinition{
				Name:        composed,
				Description: tool.Description,
				InputSchema: schemaBytes,
			})
		}
	}

	// Deterministic order keeps test fixtures stable and produces
	// reproducible llm.Request.Tools orderings for identical server
	// sets. Sorting after full construction is O(N log N) in the
	// total tool count; on the per-server cap of 32 with a practical
	// tool-per-server budget this is inconsequential.
	sort.Slice(defs, func(a, b int) bool {
		return defs[a].Name < defs[b].Name
	})

	return &router{routes: routes, defs: defs}, nil
}

// marshalInputSchema produces the byte form of an MCP tool's input
// schema. The SDK delivers the schema as `any` (per
// [sdkmcp.Tool.InputSchema] godoc, a client-side `map[string]any`
// after default JSON unmarshalling), and the praxis
// [llm.ToolDefinition.InputSchema] field expects a raw JSON
// message.
//
// A nil schema is marshalled as the literal JSON value `null`, so
// consumers receive a well-formed JSON message rather than an
// empty byte slice. This keeps the downstream provider-mapping
// layer simple: a single `json.RawMessage(schema)` passthrough
// works for every tool.
func marshalInputSchema(schema any) ([]byte, error) {
	return json.Marshal(schema)
}
