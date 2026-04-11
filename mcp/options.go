// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"github.com/praxis-os/praxis/credentials"
	"github.com/praxis-os/praxis/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// DefaultMaxResponseBytes is the default upper bound on the size of a
// buffered MCP tool response. Callers override this via
// [WithMaxResponseBytes].
//
// The value is 16 MiB, matching the D112 amendment rationale: large
// enough that well-behaved MCP servers almost never trip it, small
// enough to cap adversarial resource consumption at a tolerable level
// on typical deployments.
const DefaultMaxResponseBytes int64 = 16 * 1024 * 1024

// config holds the resolved configuration for an [Invoker]. It is
// populated by [Option]s at [New] time and then pinned for the
// lifetime of the returned Invoker. config is intentionally
// unexported: the public surface is the [Option] constructors
// above, not the struct itself.
type config struct {
	resolver         credentials.Resolver
	metricsRecorder  telemetry.MetricsRecorder
	tracerProvider   trace.TracerProvider
	maxResponseBytes int64
}

// defaultConfig returns a [config] pre-populated with zero-wiring
// defaults. Every field is non-nil after this call, so later code
// paths never need to nil-check before dispatching to the
// corresponding dependency.
func defaultConfig() config {
	return config{
		resolver:         credentials.NullResolver{},
		metricsRecorder:  telemetry.NoopMetricsRecorder{},
		tracerProvider:   otel.GetTracerProvider(),
		maxResponseBytes: DefaultMaxResponseBytes,
	}
}

// Option configures the [Invoker] returned by [New].
//
// Options are functional: each value mutates a private [config]
// struct inside [New] before the Invoker is constructed. The
// [config] itself is intentionally unexported — callers interact
// with the configuration exclusively through the `With*` constructors
// below.
//
// Stability: stable-v0.x-candidate. The Option type and its concrete
// constructors freeze at praxis/mcp v1.0.0. Adding new With*
// constructors between v0.7.0 and v1.0.0 is a minor version bump of
// praxis/mcp; removing or renaming existing ones is a breaking change.
type Option func(*config)

// WithResolver injects the [credentials.Resolver] the adapter uses to
// fetch each [Server]'s credential when opening a session.
//
// A nil resolver argument is ignored: the adapter falls back to the
// built-in [credentials.NullResolver], which errors on every Fetch
// call. The null default is acceptable only for deployments where
// every Server has an empty [Server.CredentialRef]; mixing
// credential-bearing servers with the null resolver causes session
// opening to fail with a typed error at [New] time.
//
// Typical wiring: callers pass the same resolver they configure on
// their [praxis.AgentOrchestrator] so that credential scopes remain
// consistent across every tool-call surface.
func WithResolver(r credentials.Resolver) Option {
	return func(c *config) {
		if r != nil {
			c.resolver = r
		}
	}
}

// WithMetricsRecorder injects the [telemetry.MetricsRecorder] used by
// the adapter to record MCP-specific metrics.
//
// The adapter type-asserts the passed recorder against an optional
// `mcp.MetricsRecorder` extension interface (D115) at runtime. If the
// assertion succeeds, the adapter uses the extension methods to
// record MCP-specific metrics (`praxis_mcp_calls_total`,
// `praxis_mcp_call_duration_seconds`, `praxis_mcp_transport_errors_total`).
// If the assertion fails, MCP-specific metrics are dropped silently —
// the core metrics recorded by the orchestrator through the core
// [telemetry.MetricsRecorder] are unaffected.
//
// This is NOT the D100 MetricsRecorderV2 pattern: D115 deliberately
// uses a separate-interface + type-assertion pattern because the MCP
// metric methods live in a different Go module than the core
// interface and embedding across module boundaries would require the
// core to know about the extension.
//
// A nil recorder argument is ignored: the adapter falls back to the
// built-in [telemetry.NoopMetricsRecorder].
//
// Typical wiring: callers pass the same recorder they configure on
// their [praxis.AgentOrchestrator] so the MCP metric series appear
// alongside the core ones in the same Prometheus registry.
func WithMetricsRecorder(r telemetry.MetricsRecorder) Option {
	return func(c *config) {
		if r != nil {
			c.metricsRecorder = r
		}
	}
}

// WithTracerProvider injects the OpenTelemetry [trace.TracerProvider]
// the adapter uses to open the `praxis.mcp.toolcall` child span on
// every [tools.Invoker.Invoke] call it processes.
//
// A nil TracerProvider argument is ignored: the adapter falls back
// to [otel.GetTracerProvider], which is the process-global default
// and is safe to call even when no tracing pipeline is configured
// (it returns a no-op implementation in that case).
//
// Typical wiring: callers pass the same TracerProvider they attach
// to the core orchestrator via its own tracing wiring so that the
// `praxis.toolcall` → `praxis.mcp.toolcall` span hierarchy is a
// single unbroken chain in the collector.
func WithTracerProvider(tp trace.TracerProvider) Option {
	return func(c *config) {
		if tp != nil {
			c.tracerProvider = tp
		}
	}
}

// WithMaxResponseBytes caps the maximum size of a buffered MCP tool
// response before the adapter returns it to the orchestrator.
//
// If a response exceeds the cap the adapter rejects it with a typed
// error of kind [errors.ErrorKindTool] and sub-kind
// `ToolSubKindServerError` and records a transport-error metric
// with kind `"schema"` (the oversize response is considered a
// protocol-level failure). The partially-buffered bytes are dropped;
// the adapter does not stream partial results.
//
// A non-positive argument is ignored: the adapter falls back to
// [DefaultMaxResponseBytes] (16 MiB). This prevents callers from
// silently configuring a zero-byte cap that would reject every
// response, which is never the intended behaviour.
//
// Rationale: the cap is a resource-consumption guard, not a budget
// dimension. Phase 7 rejected adding a bytes-over-wire budget
// dimension (see Phase 7 §4 / D112 rationale); the response-size cap
// is the minimal mitigation for a runaway MCP server that returns
// gigabytes of tool output.
func WithMaxResponseBytes(n int64) Option {
	return func(c *config) {
		if n > 0 {
			c.maxResponseBytes = n
		}
	}
}
