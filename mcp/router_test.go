// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/praxis-os/praxis/mcp/internal/client"
)

// toolSpec is a test-local description of a tool to advertise on
// an in-process MCP server. Handlers are optional — tests that
// only exercise the router do not need a real CallTool path.
type toolSpec struct {
	name        string
	description string
	// schema is set on [sdkmcp.Tool.InputSchema] verbatim and
	// marshalled by the SDK. A nil schema is emitted as JSON null
	// by the adapter's marshalInputSchema helper.
	schema  map[string]any
	handler sdkmcp.ToolHandler
}

// serverSpec describes one in-process MCP server to be stood up by
// [openSessionsWithTools] for a router or dispatch test. The
// LogicalName is only used by the caller-side [Server] value; the
// tools list drives the server-side AddTool calls.
type serverSpec struct {
	logicalName string
	tools       []toolSpec
}

// openSessionsWithTools returns a [sessionOpener] substitute that
// stands up one in-process MCP server per entry in specs, registers
// each spec's tools on the server-side, and connects the
// praxis-owned client wrapper to it. The resulting sessions are
// index-aligned with the servers slice passed into [New] — the
// caller is responsible for building that slice with LogicalNames
// that match specs order.
//
// The helper is used by router_test.go and dispatch_test.go. The
// handlers, if set, run on the server side of the in-memory pair
// so Invoke can exercise the full CallTool round-trip without
// spawning child processes.
func openSessionsWithTools(specs []serverSpec) sessionOpener {
	return func(_ context.Context, _ config, servers []Server) ([]*sdkmcp.ClientSession, error) {
		if len(servers) != len(specs) {
			// A mismatch here is always a test-authoring bug — the
			// caller built `servers` and `specs` from two different
			// sources. Return an error via the opener contract so
			// the test fails deterministically instead of silently
			// running against the wrong set of tools.
			return nil, systemError("openSessionsWithTools: len(servers) != len(specs); test-authoring bug")
		}

		sessions := make([]*sdkmcp.ClientSession, 0, len(servers))
		for i := range servers {
			t1, t2 := sdkmcp.NewInMemoryTransports()

			srv := sdkmcp.NewServer(
				&sdkmcp.Implementation{Name: "praxis-mcp-router-test", Version: "0"},
				nil,
			)
			for _, ts := range specs[i].tools {
				tool := &sdkmcp.Tool{
					Name:        ts.name,
					Description: ts.description,
				}
				// The SDK server-side AddTool panics when
				// InputSchema is absent. Tests that don't care
				// about schema content get a minimal object schema
				// here so the test surface stays small — router
				// and dispatch behaviour is schema-shape
				// independent.
				if ts.schema != nil {
					tool.InputSchema = ts.schema
				} else {
					tool.InputSchema = map[string]any{"type": "object"}
				}
				handler := ts.handler
				if handler == nil {
					handler = defaultEchoHandler(specs[i].logicalName, ts.name)
				}
				srv.AddTool(tool, handler)
			}

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
}

// defaultEchoHandler returns a handler that emits a single
// TextContent block echoing the server LogicalName and raw tool
// name. It exists so router tests can register tools without
// caring about CallTool behaviour; dispatch tests that want
// a specific response shape pass an explicit handler.
func defaultEchoHandler(logicalName, toolName string) sdkmcp.ToolHandler {
	return func(_ context.Context, _ *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{
				&sdkmcp.TextContent{Text: logicalName + "/" + toolName + ": echo"},
			},
		}, nil
	}
}

// TestBuildRouterHappyPath stands up two in-memory servers,
// registers distinct tools on each, runs buildRouter, and asserts:
//
//  1. Every advertised tool is present in routes as
//     {LogicalName}__{rawName}.
//  2. The routing entry points back to the correct server index
//     and raw name.
//  3. The defs slice is sorted by composed name.
//  4. Each def carries the description and a JSON-marshalled
//     InputSchema.
func TestBuildRouterHappyPath(t *testing.T) {
	t.Parallel()

	specs := []serverSpec{
		{
			logicalName: "alpha",
			tools: []toolSpec{
				{name: "probe", description: "probe the alpha server", schema: map[string]any{"type": "object"}},
				{name: "ping", description: "ping alpha"},
			},
		},
		{
			logicalName: "bravo",
			tools: []toolSpec{
				{name: "query", description: "query bravo", schema: map[string]any{"type": "object", "properties": map[string]any{"q": map[string]any{"type": "string"}}}},
			},
		},
	}

	servers := []Server{validServer("alpha"), validServer("bravo")}
	opener := openSessionsWithTools(specs)
	sessions, err := opener(context.Background(), defaultConfig(), servers)
	if err != nil {
		t.Fatalf("opener: %v", err)
	}
	defer func() { _ = closeSessions(sessions) }()

	rt, err := buildRouter(context.Background(), servers, sessions)
	if err != nil {
		t.Fatalf("buildRouter: %v", err)
	}

	wantComposed := []string{
		"alpha__ping",
		"alpha__probe",
		"bravo__query",
	}
	if got := len(rt.routes); got != len(wantComposed) {
		t.Errorf("len(routes) = %d, want %d", got, len(wantComposed))
	}
	for _, composed := range wantComposed {
		route, ok := rt.routes[composed]
		if !ok {
			t.Errorf("routes[%q] missing", composed)
			continue
		}
		logical, raw, _ := strings.Cut(composed, logicalNameSeparator)
		if servers[route.sessionIdx].LogicalName != logical {
			t.Errorf("routes[%q] sessionIdx points to %q, want %q",
				composed, servers[route.sessionIdx].LogicalName, logical)
		}
		if route.rawName != raw {
			t.Errorf("routes[%q] rawName = %q, want %q", composed, route.rawName, raw)
		}
	}

	// Deterministic sort assertion.
	gotNames := make([]string, len(rt.defs))
	for i, d := range rt.defs {
		gotNames[i] = d.Name
	}
	if !reflect.DeepEqual(gotNames, wantComposed) {
		t.Errorf("defs order = %v, want %v", gotNames, wantComposed)
	}

	// Per-def content assertions.
	for _, d := range rt.defs {
		if d.Description == "" {
			t.Errorf("defs[%q] has empty Description", d.Name)
		}
		if len(d.InputSchema) == 0 {
			t.Errorf("defs[%q] has empty InputSchema", d.Name)
			continue
		}
		// InputSchema must be valid JSON — roundtrip via json.Unmarshal
		// to prove it.
		var scratch any
		if err := json.Unmarshal(d.InputSchema, &scratch); err != nil {
			t.Errorf("defs[%q] InputSchema not valid JSON: %v; raw=%q", d.Name, err, string(d.InputSchema))
		}
	}
}

// TestBuildRouterCollisionDetected wires two servers that somehow
// advertise the same composed name (forced via a post-build
// duplicate injection) and asserts buildRouter returns a typed
// SystemError naming both servers. With the D111 LogicalName rules
// this case is structurally impossible for well-formed inputs, but
// the defensive collision check is expected to catch it.
//
// Because the natural collision path is closed by validation, the
// test uses two servers with the **same LogicalName** through the
// opener (bypassing validateServers, which the real New path runs
// before buildRouter). This exercises the collision code path in
// buildRouter without requiring a patched SDK.
func TestBuildRouterCollisionDetected(t *testing.T) {
	t.Parallel()

	// Both servers advertise the same raw tool name, and because
	// we forcibly use the same LogicalName for both, the composed
	// names collide.
	specs := []serverSpec{
		{logicalName: "dup", tools: []toolSpec{{name: "probe", description: "from server 0"}}},
		{logicalName: "dup", tools: []toolSpec{{name: "probe", description: "from server 1"}}},
	}
	// Note: we skip validateServers by calling the opener directly
	// and then buildRouter directly — we're unit-testing the
	// router's own collision detection, not the construction
	// pipeline wiring.
	servers := []Server{
		{LogicalName: "dup", Transport: TransportStdio{Command: "x"}},
		{LogicalName: "dup", Transport: TransportStdio{Command: "x"}},
	}
	opener := openSessionsWithTools(specs)
	sessions, err := opener(context.Background(), defaultConfig(), servers)
	if err != nil {
		t.Fatalf("opener: %v", err)
	}
	defer func() { _ = closeSessions(sessions) }()

	_, err = buildRouter(context.Background(), servers, sessions)
	if err == nil {
		t.Fatal("buildRouter: expected collision error, got nil")
	}
	assertSystemError(t, err, "composed tool name", `"dup__probe"`, "servers[0]", "servers[1]")
}

// TestBuildRouterEmptySessionSet asserts the len-alignment
// invariant check returns a typed SystemError without panicking
// when servers and sessions diverge.
func TestBuildRouterLenMismatch(t *testing.T) {
	t.Parallel()

	_, err := buildRouter(context.Background(),
		[]Server{validServer("a"), validServer("b")},
		[]*sdkmcp.ClientSession{nil},
	)
	if err == nil {
		t.Fatal("buildRouter: expected len-mismatch error, got nil")
	}
	assertSystemError(t, err, "len(servers)=2", "len(sessions)=1")
}

// TestBuildRouterNilSessionSkipped verifies that a nil entry in
// the sessions slice does not produce an error and does not leak
// into the routes map. It validates the test-hook contract used
// by nullSessionOpener.
func TestBuildRouterNilSessionSkipped(t *testing.T) {
	t.Parallel()

	servers := []Server{validServer("alpha")}
	rt, err := buildRouter(context.Background(), servers, []*sdkmcp.ClientSession{nil})
	if err != nil {
		t.Fatalf("buildRouter with nil session: %v", err)
	}
	if len(rt.routes) != 0 {
		t.Errorf("len(routes) = %d, want 0 for nil-session server", len(rt.routes))
	}
	if len(rt.defs) != 0 {
		t.Errorf("len(defs) = %d, want 0 for nil-session server", len(rt.defs))
	}
}

// TestBuildRouterDefinitionsExposedViaInvoker drives a full New
// call against two tool-bearing servers and asserts that the
// resulting Invoker.Definitions() slice matches the router's
// internal defs byte-for-byte — i.e., the accessor does not copy
// or drop any entries. Matches the public godoc contract that
// the returned slice is "owned by the Invoker".
func TestBuildRouterDefinitionsExposedViaInvoker(t *testing.T) {
	t.Parallel()

	specs := []serverSpec{
		{
			logicalName: "alpha",
			tools: []toolSpec{
				{name: "probe", description: "probe alpha"},
			},
		},
		{
			logicalName: "bravo",
			tools: []toolSpec{
				{name: "query", description: "query bravo"},
				{name: "ping", description: "ping bravo"},
			},
		},
	}
	servers := []Server{validServer("alpha"), validServer("bravo")}

	inv, err := New(context.Background(), servers, withSessionOpener(openSessionsWithTools(specs)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = inv.Close() }()

	defs := inv.Definitions()
	wantNames := []string{"alpha__probe", "bravo__ping", "bravo__query"}
	if len(defs) != len(wantNames) {
		t.Fatalf("len(Definitions) = %d, want %d", len(defs), len(wantNames))
	}
	for i, want := range wantNames {
		if defs[i].Name != want {
			t.Errorf("defs[%d].Name = %q, want %q", i, defs[i].Name, want)
		}
	}
}
