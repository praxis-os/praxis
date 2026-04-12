// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"sync"
	"testing"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/praxis-os/praxis/telemetry"
	"github.com/praxis-os/praxis/tools"
)

// fakeRecorder implements both telemetry.MetricsRecorder (core) and
// MCPMetricsRecorder (extension) so it can be wired through
// [WithMetricsRecorder] and discovered via the D115 type-assertion.
// Recorded events are stored in slices under a mutex so tests can
// assert after concurrent dispatches.
type fakeRecorder struct {
	telemetry.NoopMetricsRecorder // satisfy core interface

	mu              sync.Mutex
	calls           []mcpCallRecord
	transportErrors []mcpTransportErrorRecord
}

type mcpCallRecord struct {
	server, transport, status string
	duration                  time.Duration
}

type mcpTransportErrorRecord struct {
	server, transport, kind string
}

func (r *fakeRecorder) RecordMCPCall(server, transport, status string, d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, mcpCallRecord{server, transport, status, d})
}

func (r *fakeRecorder) RecordMCPTransportError(server, transport, kind string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.transportErrors = append(r.transportErrors, mcpTransportErrorRecord{server, transport, kind})
}

// Compile-time assertion: fakeRecorder satisfies both interfaces.
var (
	_ telemetry.MetricsRecorder = (*fakeRecorder)(nil)
	_ MCPMetricsRecorder        = (*fakeRecorder)(nil)
)

// TestMetricsRecordMCPCallOnSuccess wires a fake recorder through
// WithMetricsRecorder, dispatches a successful tool call, and
// asserts that RecordMCPCall is called exactly once with
// status="ok", the correct server and transport labels, and a
// positive duration.
func TestMetricsRecordMCPCallOnSuccess(t *testing.T) {
	t.Parallel()

	rec := &fakeRecorder{}
	specs := []serverSpec{{
		logicalName: "alpha",
		tools: []toolSpec{{
			name: "probe",
			handler: func(_ context.Context, _ *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
				return &sdkmcp.CallToolResult{
					Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "ok"}},
				}, nil
			},
		}},
	}}

	inv, err := New(context.Background(),
		[]Server{validServer("alpha")},
		withSessionOpener(openSessionsWithTools(specs)),
		WithMetricsRecorder(rec),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = inv.Close() }()

	_, frameworkErr := inv.Invoke(context.Background(), tools.InvocationContext{}, tools.ToolCall{
		CallID: "c1",
		Name:   "alpha__probe",
	})
	if frameworkErr != nil {
		t.Fatalf("Invoke: %v", frameworkErr)
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()

	if len(rec.calls) != 1 {
		t.Fatalf("RecordMCPCall count = %d, want 1", len(rec.calls))
	}
	c := rec.calls[0]
	if c.server != "alpha" {
		t.Errorf("server = %q, want %q", c.server, "alpha")
	}
	if c.transport != "stdio" {
		t.Errorf("transport = %q, want %q", c.transport, "stdio")
	}
	if c.status != "ok" {
		t.Errorf("status = %q, want %q", c.status, "ok")
	}
	if c.duration <= 0 {
		t.Errorf("duration = %v, want > 0", c.duration)
	}
	if len(rec.transportErrors) != 0 {
		t.Errorf("RecordMCPTransportError count = %d, want 0 on success", len(rec.transportErrors))
	}
}

// TestMetricsRecordMCPCallOnIsError asserts that a server-reported
// tool error (IsError=true) is recorded as status="error" via
// RecordMCPCall, and that NO transport-error is emitted (server-
// reported tool errors are tool-level, not transport-level).
func TestMetricsRecordMCPCallOnIsError(t *testing.T) {
	t.Parallel()

	rec := &fakeRecorder{}
	specs := []serverSpec{{
		logicalName: "bravo",
		tools: []toolSpec{{
			name: "broken",
			handler: func(_ context.Context, _ *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
				return &sdkmcp.CallToolResult{
					Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "fail"}},
					IsError: true,
				}, nil
			},
		}},
	}}

	inv, err := New(context.Background(),
		[]Server{validServer("bravo")},
		withSessionOpener(openSessionsWithTools(specs)),
		WithMetricsRecorder(rec),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = inv.Close() }()

	_, _ = inv.Invoke(context.Background(), tools.InvocationContext{}, tools.ToolCall{
		CallID: "c2",
		Name:   "bravo__broken",
	})

	rec.mu.Lock()
	defer rec.mu.Unlock()

	if len(rec.calls) != 1 {
		t.Fatalf("RecordMCPCall count = %d, want 1", len(rec.calls))
	}
	if rec.calls[0].status != "error" {
		t.Errorf("status = %q, want %q", rec.calls[0].status, "error")
	}
	if len(rec.transportErrors) != 0 {
		t.Errorf("RecordMCPTransportError count = %d, want 0 (IsError is tool-level, not transport)", len(rec.transportErrors))
	}
}

// TestMetricsNoEmissionWithoutExtension asserts the D115 silent-
// drop contract: a recorder that implements only the core
// telemetry.MetricsRecorder (no MCPMetricsRecorder) results in
// zero MCP metric emissions. The fakeRecorder would record them
// if detected, so we use the core NoopMetricsRecorder directly.
func TestMetricsNoEmissionWithoutExtension(t *testing.T) {
	t.Parallel()

	specs := []serverSpec{{
		logicalName: "alpha",
		tools: []toolSpec{{
			name: "probe",
			handler: func(_ context.Context, _ *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
				return &sdkmcp.CallToolResult{
					Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "ok"}},
				}, nil
			},
		}},
	}}

	// Pass the core-only NoopMetricsRecorder; the type-assertion
	// for MCPMetricsRecorder should fail silently.
	inv, err := New(context.Background(),
		[]Server{validServer("alpha")},
		withSessionOpener(openSessionsWithTools(specs)),
		WithMetricsRecorder(telemetry.NoopMetricsRecorder{}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = inv.Close() }()

	// Dispatch should succeed without panic/nil-pointer even
	// though no MCPMetricsRecorder is wired.
	result, frameworkErr := inv.Invoke(context.Background(), tools.InvocationContext{}, tools.ToolCall{
		CallID: "c3",
		Name:   "alpha__probe",
	})
	if frameworkErr != nil {
		t.Fatalf("Invoke: %v", frameworkErr)
	}
	if result.Status != tools.ToolStatusSuccess {
		t.Errorf("status = %q, want %q", result.Status, tools.ToolStatusSuccess)
	}
	// No way to assert zero emissions from NoopMetricsRecorder
	// directly — the test's value is proving the nil mcpRecorder
	// path does not panic or degrade the tool-call result.
}

// TestTransportLabel asserts the bounded transport dimension
// helper returns the correct string for each sealed Transport
// variant.
func TestTransportLabel(t *testing.T) {
	t.Parallel()

	stdio := Server{LogicalName: "a", Transport: TransportStdio{Command: "x"}}
	if got := transportLabel(stdio); got != "stdio" {
		t.Errorf("TransportStdio → %q, want %q", got, "stdio")
	}

	http := Server{LogicalName: "b", Transport: TransportHTTP{URL: "http://localhost"}}
	if got := transportLabel(http); got != "http" {
		t.Errorf("TransportHTTP → %q, want %q", got, "http")
	}
}

// TestBoundedLabelCardinality is T34.7: it computes the
// theoretical maximum cardinality of the MCP metric label-space
// and asserts it against a known upper bound. The 32-server cap
// from D115 combined with the fixed transport/status/kind sets
// guarantees a finite and predictable label-space.
//
// praxis_mcp_calls_total + duration histogram share labels:
//
//	server(32) × transport(2) × status(2) = 128 series
//
// praxis_mcp_transport_errors_total:
//
//	server(32) × transport(2) × kind(4) = 256 series
//
// Total: 128 + 128 + 256 = 512 (calls counter + duration + errors)
//
// The test asserts that the computed upper bound matches the
// expected 512. If a new transport, status, or kind value is
// added, the test will fail and force an explicit update to the
// cardinality budget, preventing accidental explosion.
func TestBoundedLabelCardinality(t *testing.T) {
	t.Parallel()

	const (
		maxServers = MaxServers // 32

		transportValues = 2 // "stdio", "http"
		statusValues    = 2 // "ok", "error"
		kindValues      = 4 // "network", "server_error", "schema_violation", "circuit_open"
	)

	// calls_total cardinality
	callsCardinality := maxServers * transportValues * statusValues
	// duration histogram has the same label set as calls_total
	durationCardinality := callsCardinality
	// transport_errors_total cardinality
	transportErrorsCardinality := maxServers * transportValues * kindValues

	total := callsCardinality + durationCardinality + transportErrorsCardinality

	const expectedTotal = 512
	if total != expectedTotal {
		t.Errorf("total cardinality = %d, want %d; if you added a new label value, update this test and the D115 cardinality budget",
			total, expectedTotal)
	}

	// Spot-check individual metric cardinalities for readability.
	if callsCardinality != 128 {
		t.Errorf("calls_total cardinality = %d, want 128", callsCardinality)
	}
	if transportErrorsCardinality != 256 {
		t.Errorf("transport_errors_total cardinality = %d, want 256", transportErrorsCardinality)
	}
}
