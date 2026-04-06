# Phase 4 — Prometheus Metrics

**Decisions:** D57, D60
**Cross-references:** `02-span-tree.md` (AttributeEnricher boundary),
`05-error-event-mapping.md` (error_kind label values)

---

## 1. Design principles

**Cardinality first.** Every label must have a statically bounded value set.
No metric label may have unbounded cardinality (invocation IDs, user IDs,
request IDs). These go on OTel spans only.

**AttributeEnricher attributes never appear on metric labels.** Enricher
attributes (tenant ID, org ID, user ID, agent ID) go to spans exclusively
(D60). This is a hard boundary enforced by framework design: the metric
registration code does not call `AttributeEnricher.Enrich` and has no access
to enricher output. Callers who need per-tenant metrics aggregate from spans or
implement a `LifecycleEventEmitter` that feeds their own metric pipeline.

**Small, focused metric set.** The goal is an actionable default dashboard, not
exhaustive instrumentation. 10 metrics, each answering a specific operational
question.

**`praxis_` prefix on all metric names.** This namespace prefix avoids
collisions with application metrics in the same Prometheus registry.

**No-op meter compatibility.** When no Prometheus registry is wired, the
framework uses `prometheus.NewRegistry()` internally or accepts a caller-provided
registry. The metric set can also be driven by an OTel meter; in that case,
the same label taxonomy applies. If no meter is configured, metric calls are
dropped silently (OTel no-op meter semantics).

---

## 2. Metric table

### 2.1 Invocation metrics

| Metric name | Type | Labels | Cardinality estimate | Question answered |
|---|---|---|---|---|
| `praxis_invocations_total` | Counter | `provider`, `model`, `terminal_state` | ~5 × 10 × 5 = 250 | How many invocations complete, and in what terminal state? |
| `praxis_invocation_duration_seconds` | Histogram | `provider`, `model`, `terminal_state` | same as above | How long do invocations take, broken down by outcome? |

**`terminal_state` label values:** `completed`, `failed`, `cancelled`,
`budget_exceeded`, `approval_required` — bounded (5 values).

**`provider` label values:** one value per registered `llm.Provider.Name()`
— bounded in practice (expected: `anthropic`, `openai`, `gemini`, ...; ~10
values in a typical deployment).

**`model` label values:** one value per model identifier — bounded in practice
(expected: ~5–20 values per provider; total ~50 across providers at any given
deployment). If a caller runs an unbounded number of model variants, this label
approaches the cardinality limit. Callers may truncate model strings to a base
identifier in their adapter code; the framework uses whatever `Provider.Name()`
and `InvocationRequest.Model` return.

**Cardinality cap note:** `provider × model` is the highest-risk label pair.
The framework accepts that `model` can grow; it documents the risk and allows
callers to control it through their `LLMProvider` adapter's `Name()` method.

### 2.2 LLM call metrics

| Metric name | Type | Labels | Cardinality estimate | Question answered |
|---|---|---|---|---|
| `praxis_llm_calls_total` | Counter | `provider`, `model`, `status` | ~5 × 10 × 3 = 150 | How many LLM calls are made, and do they succeed? |
| `praxis_llm_call_duration_seconds` | Histogram | `provider`, `model`, `status` | same | How long do LLM calls take? |
| `praxis_llm_tokens_total` | Counter | `provider`, `model`, `direction` | ~5 × 10 × 2 = 100 | How many tokens are consumed (input vs. output)? |

**`status` label values for LLM calls:** `ok`, `transient_error`,
`permanent_error` — bounded (3 values). Maps from `ErrorKind`: `transient_llm`
→ `transient_error`, `permanent_llm` → `permanent_error`, success → `ok`.

**`direction` label values:** `input`, `output` — bounded (2 values,
mirrors `budget.TokenDirection`).

### 2.3 Tool call metrics

| Metric name | Type | Labels | Cardinality estimate | Question answered |
|---|---|---|---|---|
| `praxis_tool_calls_total` | Counter | `tool_name`, `status` | ~20 × 3 = 60 | Which tools are called and do they succeed? |
| `praxis_tool_call_duration_seconds` | Histogram | `tool_name`, `status` | same | How long do tool calls take? |

**`tool_name` label values:** bounded per deployment (the tool set registered
with the orchestrator is finite). Expected: ~5–20 tool names per deployment.

**`status` label values for tool calls:** `ok`, `error`, `denied` — bounded
(3 values, mirrors `praxis.tool_status` span attribute).

**Cardinality note:** `tool_name` is the only label that depends on
caller-defined values. Framework code uses the tool name as reported by
`tools.Invoker` via the `InvocationEvent.ToolName` field. Callers must not
register tools with dynamic or unbounded names (e.g., per-user tool IDs). This
constraint is documented in the `tools.Invoker` godoc.

### 2.4 Budget metrics

| Metric name | Type | Labels | Cardinality estimate | Question answered |
|---|---|---|---|---|
| `praxis_budget_exceeded_total` | Counter | `dimension` | 4 | How often and on which dimension are budgets exceeded? |

**`dimension` label values:** `wall_clock`, `tokens`, `tool_calls`, `cost` —
bounded (4 values, mirrors `budget.BudgetDimension`).

### 2.5 Error metrics

| Metric name | Type | Labels | Cardinality estimate | Question answered |
|---|---|---|---|---|
| `praxis_errors_total` | Counter | `error_kind` | 8 | How many errors of each kind occur? |

**`error_kind` label values:** the 8 `ErrorKind` string values:
`transient_llm`, `permanent_llm`, `tool`, `policy_denied`, `budget_exceeded`,
`cancellation`, `system`, `approval_required` — bounded (8 values).

**Note:** `approval_required` appears here because `ApprovalRequiredError` is
returned on the error path from `AgentOrchestrator.Invoke`. It is semantically
not a failure, but it is still an `ErrorKind`. Callers who want to distinguish
error rates from approval rates can filter `error_kind != "approval_required"`.

---

## 3. Histogram bucket boundaries

All duration histograms use seconds as the unit, conforming to Prometheus and
OTel conventions.

### 3.1 `praxis_invocation_duration_seconds`

Invocations include multiple LLM calls, tool calls, and policy evaluations.
Expected range: tens of milliseconds to tens of minutes.

```
Buckets: [0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0, 60.0, 120.0, 300.0, +Inf]
```

Rationale: lower bound at 100ms (sub-100ms invocations are rare in practice
given LLM latency). Upper bound at 300s covers long multi-turn invocations with
slow tool calls.

### 3.2 `praxis_llm_call_duration_seconds`

Individual LLM call round-trip time (from request dispatch to response receipt).
Expected range: hundreds of milliseconds to tens of seconds.

```
Buckets: [0.1, 0.25, 0.5, 1.0, 2.0, 5.0, 10.0, 20.0, 30.0, 60.0, +Inf]
```

Rationale: 100ms is a realistic lower bound even for fast models. 60s covers
slow completions and network-degraded calls.

### 3.3 `praxis_tool_call_duration_seconds`

Individual tool invocation round-trip time. Tool latency is highly variable
(in-process tools: microseconds; remote APIs: seconds).

```
Buckets: [0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0, 5.0, 10.0, 30.0, +Inf]
```

Rationale: starts at 1ms to capture fast in-process tools; extends to 30s for
slow external APIs. The wide range reflects tool diversity.

---

## 4. Relationship between `AttributeEnricher` and metric labels

This relationship has a hard boundary:

**AttributeEnricher → OTel spans (unlimited).**
Enricher attributes attach to every span in the trace. Unbounded-cardinality
attributes (per-user IDs, per-request IDs) are acceptable because tracing
backends handle cardinality through sampling and retention policies, not through
a fixed label count.

**AttributeEnricher → Prometheus metric labels (never).**
Enricher attributes are excluded from all metric labels. The Prometheus label
sets defined in §2 are closed and framework-defined. Adding enricher attributes
to Prometheus labels would allow callers with high-cardinality attributes to
cause Prometheus OOM or query-time degradation.

**The escape hatch:** callers who need per-tenant or per-user Prometheus
metrics implement a `LifecycleEventEmitter` that reads
`InvocationEvent.EnricherAttributes` and updates their own Prometheus counters.
The framework does not constrain what `LifecycleEventEmitter` implementations
do with the enricher attributes — the cardinality responsibility is the
caller's.

---

## 5. Metric registration and `MetricsRecorder` interface (D57, D65)

### 5.1 `MetricsRecorder` interface

The orchestrator records metric observations through the `MetricsRecorder`
interface. This interface is intentionally narrow: each method accepts only
framework-defined, bounded-cardinality parameters. There is no
`map[string]string` parameter on any method — this structurally enforces the
cardinality boundary between enricher attributes (spans only) and metric labels
(framework-defined only).

```go
// MetricsRecorder records metric observations for an invocation.
//
// The orchestrator calls the appropriate method at each metric-relevant
// state transition. Implementations must be safe for concurrent use.
//
// MetricsRecorder methods accept only framework-defined, bounded-cardinality
// values as parameters. AttributeEnricher attributes are intentionally
// excluded from all method signatures to prevent cardinality explosion
// in Prometheus (D57, D60).
//
// Stability: frozen-v1.0.
// Package: MODULE_PATH_TBD/telemetry
type MetricsRecorder interface {
    // RecordInvocation records a completed invocation.
    // Called once at terminal state entry.
    RecordInvocation(provider, model, terminalState string, duration time.Duration)

    // RecordLLMCall records a single LLM call.
    // Called after each LLM provider call returns.
    // status is "ok", "transient_error", or "permanent_error".
    RecordLLMCall(provider, model, status string, duration time.Duration)

    // RecordLLMTokens records token consumption from a single LLM call.
    // direction is "input" or "output".
    RecordLLMTokens(provider, model, direction string, count int64)

    // RecordToolCall records a single tool invocation.
    // status is "ok", "error", or "denied".
    RecordToolCall(toolName, status string, duration time.Duration)

    // RecordBudgetExceeded records a budget breach.
    // dimension is one of the BudgetDimension values.
    RecordBudgetExceeded(dimension string)

    // RecordError records an error by kind.
    // errorKind is one of the ErrorKind string values.
    RecordError(errorKind string)
}
```

### 5.2 Default implementation

```go
// NullMetricsRecorder is the default MetricsRecorder implementation.
// It discards all metric observations without error.
// Safe for concurrent use.
//
// Package: MODULE_PATH_TBD/telemetry
var NullMetricsRecorder MetricsRecorder = nullMetricsRecorder{}
```

### 5.3 Prometheus-backed implementation

The framework ships a Prometheus-backed `MetricsRecorder` constructed via
`NewPrometheusRecorder`:

```go
// NewPrometheusRecorder registers all 10 praxis metrics with the given
// Prometheus registerer and returns a MetricsRecorder backed by them.
//
// Callers who use the default Prometheus global registry:
//
//   recorder := telemetry.NewPrometheusRecorder(prometheus.DefaultRegisterer)
//
// Callers with isolated registries:
//
//   reg := prometheus.NewRegistry()
//   recorder := telemetry.NewPrometheusRecorder(reg)
//
// The returned recorder is safe for concurrent use. The registerer must
// not be nil; NewPrometheusRecorder panics on nil.
//
// Package: MODULE_PATH_TBD/telemetry
func NewPrometheusRecorder(reg prometheus.Registerer) MetricsRecorder
```

If `NewPrometheusRecorder` is never called, no Prometheus metrics are
registered. The framework does not self-register with
`prometheus.DefaultRegisterer` — that would be inappropriate for a library.

### 5.4 Orchestrator wiring

The `AgentOrchestrator` construction path accepts a `MetricsRecorder` via a
new option function:

```go
// WithMetricsRecorder injects the telemetry.MetricsRecorder.
// Default: telemetry.NullMetricsRecorder (discards all metric observations).
func WithMetricsRecorder(recorder telemetry.MetricsRecorder) Option
```

This option is added to the Phase 3 option inventory as a Phase 4 amendment
(D65).

---

## 6. Cardinality budget summary

| Label | Estimated distinct values | Bound? |
|---|---|---|
| `provider` | ~5–10 | Yes (LLM provider names) |
| `model` | ~10–50 | Yes (bounded by deployed model set) |
| `terminal_state` | 5 | Yes (fixed enum) |
| `status` (LLM) | 3 | Yes (ok, transient_error, permanent_error) |
| `direction` | 2 | Yes (input, output) |
| `tool_name` | ~5–20 | Yes (bounded by registered tool set) |
| `status` (tool) | 3 | Yes (ok, error, denied) |
| `dimension` | 4 | Yes (fixed enum) |
| `error_kind` | 8 | Yes (fixed enum) |

**Total metric time series upper bound** (worst case, all metrics, all label
combinations):

- Invocation metrics: 10 × 5 × 5 = 250 series per metric × 2 = 500
- LLM metrics: 10 × 5 × 3 = 150 (calls/duration) × 2 + 10 × 5 × 2 = 100 (tokens) = 400
- Tool metrics: 20 × 3 = 60 per metric × 2 = 120
- Budget metrics: 4
- Error metrics: 8

**Total: ~1,032 time series** — well within Prometheus capacity for any
deployment. This budget leaves ample room for the framework to add future
metrics without approaching cardinality limits.
