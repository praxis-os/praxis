// SPDX-License-Identifier: Apache-2.0

package mcp

import "time"

// MCPMetricsRecorder is the optional extension interface the adapter
// uses to emit MCP-specific metrics. It is a **standalone interface**
// in the `mcp` package, deliberately NOT embedding the core
// [telemetry.MetricsRecorder]: callers who want both core and MCP
// metrics pass a single recorder value that implements both
// interfaces (or an adapter type that wraps a core recorder and
// adds the MCP methods). Callers who pass a recorder implementing
// only [telemetry.MetricsRecorder] silently drop MCP-specific
// metrics; the core metrics recorded by the orchestrator through
// the core interface are unaffected.
//
// The adapter detects the extension at construction time via a type
// assertion on the value passed to [WithMetricsRecorder]:
//
//	coreRec := opt.metricsRecorder // telemetry.MetricsRecorder
//	mcpRec, hasMCP := coreRec.(MCPMetricsRecorder)
//
// If the assertion succeeds, the adapter stores `mcpRec` and calls
// its methods on every [tools.Invoker.Invoke] call. If it fails,
// MCP metric emission is a no-op for the Invoker's lifetime.
//
// # Metrics emitted
//
// | Method | Prometheus metric | Labels |
// |---|---|---|
// | [RecordMCPCall] | `praxis_mcp_calls_total` (counter) + `praxis_mcp_call_duration_seconds` (histogram) | `server`, `transport`, `status` |
// | [RecordMCPTransportError] | `praxis_mcp_transport_errors_total` (counter) | `server`, `transport`, `kind` |
//
// Label cardinality is bounded by construction:
//   - `server`: ≤ 32 (enforced by [MaxServers] at [New] time)
//   - `transport`: `"stdio"` | `"http"` (fixed 2-value set)
//   - `status`: `"ok"` | `"error"` (fixed 2-value set; `"denied"`
//     is reserved for future budget-denial, not emitted in v0.7.0)
//   - `kind`: `"network"` | `"server_error"` | `"schema_violation"` |
//     `"circuit_open"` (fixed set, matching [praxiserrors.ToolSubKind])
//
// See D115 for the full rationale and the cardinality proof.
//
// # Stability
//
// This interface is `stable-v0.x-candidate`: the method set
// freezes at praxis/mcp v1.0.0. Adding new methods between
// v0.7.0 and v1.0.0 is a minor-version bump; removing or
// changing existing methods is a breaking change.
type MCPMetricsRecorder interface {
	// RecordMCPCall records a single MCP tool-call outcome. The
	// adapter calls this once per [tools.Invoker.Invoke] dispatch
	// that reaches the CallTool stage (i.e., past routing, past
	// argument parsing). The `status` label is `"ok"` for a
	// successful tool response and `"error"` for an errored one
	// (either IsError=true server-reported or SDK-returned error).
	//
	// The `duration` is the wall-clock elapsed time between the
	// `session.CallTool` send and its return. It does NOT include
	// routing, argument parsing, or content flattening time —
	// those are negligible compared to the network round-trip and
	// their inclusion would skew the histogram distribution.
	RecordMCPCall(server, transport, status string, duration time.Duration)

	// RecordMCPTransportError records a transport-layer failure.
	// Called once per CallTool error that [classifyCallToolError]
	// maps to a transport-originated sub-kind (Network,
	// CircuitOpen, SchemaViolation). It is NOT called for
	// server-reported tool errors (IsError=true), which are
	// counted only via [RecordMCPCall] with status "error".
	//
	// The `kind` label is the string form of the
	// [praxiserrors.ToolSubKind] value (e.g., `"network"`,
	// `"circuit_open"`, `"schema_violation"`).
	RecordMCPTransportError(server, transport, kind string)
}

// transportLabel returns the bounded transport label string for
// a [Server] value, used as the `transport` dimension in every
// MCP metric emission. The function type-switches on
// [Server.Transport] — the sealed interface guarantees exactly
// two variants.
func transportLabel(s Server) string {
	switch s.Transport.(type) {
	case TransportStdio:
		return "stdio"
	case TransportHTTP:
		return "http"
	default:
		// Unreachable: the sealed Transport interface admits only
		// the two concrete types above. If a future refactor adds
		// a third variant, this default arm points at the exact
		// place that needs a new label string.
		return "unknown"
	}
}
