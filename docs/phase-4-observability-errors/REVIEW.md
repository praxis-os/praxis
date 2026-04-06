# Review: Phase 4 — Observability and Error Model

## Overall Assessment

Phase 4 delivers a comprehensive observability contract covering OTel span
trees, Prometheus metrics, slog redaction, error-to-event mapping, and
content-analysis event semantics. All five forward-carried concerns (C1, C3,
CP1, CP2, CP5) are resolved with traceable decision IDs. The previous
reviewer round surfaced 2 blockers and 5 important findings; all have been
addressed in the current artifact set.

## Critical Issues

None. The two blockers from the prior review (B1: undefined `MetricsRecorder`
interface; B2: `DetachedWithSpan` signature contradiction) have been resolved:

- B1 is resolved in `03-metrics.md` §5.1–5.4 with a full `MetricsRecorder`
  interface definition (6 methods), a `NullMetricsRecorder` default, a
  `NewPrometheusRecorder` constructor accepting `prometheus.Registerer`, a
  `frozen-v1.0` stability commitment, and a `WithMetricsRecorder` option
  function recorded as a Phase 3 amendment in D65.
- B2 is resolved in D54 with an explicit acknowledgement of the go-architect
  recommendation. The signature now accepts `deadline time.Duration` (more
  idiomatic than the go-architect's `time.Time` suggestion) with documented
  rationale for why the deadline is parameterized rather than hardcoded.

## Important Weaknesses

1. **`DetachedWithSpan` signature uses `time.Duration` where go-architect
   suggested `time.Time`.** D54 adopts `time.Duration` (a timeout relative to
   now) rather than `time.Time` (an absolute deadline). This is actually more
   idiomatic Go — `context.WithTimeout` takes a `Duration`, not a `Time` — and
   the deviation is documented in D54. No action required, but worth noting for
   traceability.

2. **M2 from prior review (VerdictLog + VerdictDeny audit note gap) is not
   addressed.** When a VerdictLog is followed by a VerdictDeny in the hook
   chain, the AuditNote is captured on the span but not on any
   `InvocationEvent` — callers using only `LifecycleEventEmitter` (no tracing
   backend) lose the audit note. This is a minor observability gap documented
   in `06-filter-event-mapping.md` §4.4. Acceptable for v1.0 given the
   workaround (read the span attribute), but should be revisited if a
   span-free observability path becomes a first-class use case.

3. **M3 from prior review (`praxis_errors_total` includes `approval_required`)
   is documented but not resolved.** The metric name "errors total" counting
   non-error outcomes is semantically misleading. The documentation includes a
   filter recommendation (`error_kind != "approval_required"`), which is
   survivable but shifts burden to every consumer. A dedicated
   `praxis_approvals_total` counter would be cleaner. This is a minor naming
   concern, not a blocker.

## Open Questions

1. Should Phase 5 (Security and Trust) impose any additional constraints on
   what the `RedactingHandler` must redact beyond the deny-list in D58? The
   current deny-list covers credential suffixes and the `praxis.credential.*`
   prefix, but Phase 5 may surface additional sensitive field patterns.

2. The `MetricsRecorder` interface is `frozen-v1.0`. If a future Prometheus
   metric is added in v1.x, can it be recorded via the existing methods
   (unlikely — a new metric probably needs a new method)? The interface
   extension story (embedding) is documented in seed §9 but not specifically
   addressed for `MetricsRecorder`.

## Decoupling Contract Check

**PASS.** Case-insensitive grep for `custos`, `reef`, `governance.event`,
`org.id`, `agent.id`, `user.id`, `tenant.id` across all Phase 4 artifacts
returns two matches, both negation-mentions in compliance declarations:

- `00-plan.md` line 123: "`tenant.id`, `agent.id`) in span or metric names
  would violate seed §6.1."

No actual identifiers or hardcoded attribute keys leak anywhere. All span
attribute keys use the `praxis.*` prefix. No metric label embeds a
consumer-specific namespace.

## Recommendations

- Accept the minor naming concern on `praxis_errors_total` (M3) as a tracked
  issue for post-v1.0. The documented workaround is sufficient.
- Carry the VerdictLog+VerdictDeny audit note gap (M2) as a concern for the
  implementation phase. If a span-free observability path is requested,
  address it then.
- At implementation time, add a back-reference annotation to Phase 3
  `02-orchestrator-api.md` referencing D65 for the `InvocationEvent` field
  amendments and the `WithMetricsRecorder` option addition.

## Verdict: READY

All 14 decisions (D53–D66) are adopted with rationale and trade-offs. All five
forward-carried concerns are resolved. The decoupling contract is clean. The
21 EventType constants are preserved without additions. The MetricsRecorder
interface is fully defined with a default implementation. The span tree, metric
set, slog redaction, error mapping, filter mapping, and enricher flow are
coherent and internally consistent. Two minor naming/gap concerns (M2, M3) are
documented and survivable. Phase 5 may proceed.
