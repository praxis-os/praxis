# Phase: Observability and Error Model

## Goal

Define the complete observability contract (OTel span tree, Prometheus metrics, slog
redaction) and the error-to-event mapping that makes every invocation fully observable
by construction, resolving all forward-carried concerns (C1, C3) and finalizing the
two content-analysis event types (D52b).

## Scope

**In scope** (refines seed §3.5, §4.5, §5, §6; builds on Phase 2 D22/D23/D25 and
Phase 3 D31/D42/D44/D52b):

- OTel span tree structure: root span, child spans per state, span attributes,
  span status mapping, and the C1 resolution for `internal/ctxutil.DetachedWithSpan`.
- CP1 nested span child-of semantics for composed orchestrators.
- CP2 `parent_invocation_id` propagation across nested invocations.
- Prometheus metric set: metric names, label taxonomy, cardinality constraints,
  histogram bucket boundaries.
- `slog` integration: structured log fields, redaction rules for credential and
  PII material, log-level mapping from event types.
- Typed error taxonomy completions: `BudgetExceededError` C3 token-overshoot
  documentation, error-to-terminal-event mapping rules.
- `FilterDecision` → event mapping: when `EventTypePIIRedacted` and
  `EventTypePromptInjectionSuspected` are emitted, what `InvocationEvent` fields
  are populated, correlation with `FilterDecision` values from D42.
- `AttributeEnricher` contract: when `Enrich` is called, how attributes flow to
  spans and events, cardinality expectations.
- CP5 `Classifier` precedence rules for propagated typed errors.
- `VerdictLog` lifecycle event emission semantics.

**Out of scope:**

- Concrete `telemetry.OTelEmitter` implementation code (implementation phase).
- Concrete `slog.Handler` implementation code (implementation phase).
- Security and credential handling (Phase 5).
- CI pipeline and release governance (Phase 6).
- Span sampling, tail-based sampling, or collector configuration — these are
  deployment concerns, not framework design.

## Key Questions

1. What is the exact span tree shape? How many child spans does a single invocation
   produce, and which state transitions create vs. close spans?
2. How does `internal/ctxutil.DetachedWithSpan` re-attach the span for terminal
   lifecycle emission (C1)? Does it carry a full `trace.Span` or only a
   `trace.SpanContext`?
3. What Prometheus metrics does `praxis` export, and what are the cardinality
   constraints on label values?
4. How do `FilterDecision` values from D42 map to the two content-analysis event
   types (`EventTypePIIRedacted`, `EventTypePromptInjectionSuspected`)? What
   triggers each, and what fields on `InvocationEvent` are populated?
5. What are the slog structured fields on each event emission, and which fields
   are redacted (never logged)?
6. How does CP1 (nested orchestrator span correlation) work in practice? Does the
   child orchestrator create a child span of the parent's span, or does it use
   span links?
7. How does the `AttributeEnricher` attribute set flow to both OTel spans and
   Prometheus metric labels, and where is the cardinality boundary enforced?
8. What is the CP5 classifier precedence order when a `TypedError` propagates
   through `ToolResult.Err` — does the classifier unwrap it, and does the error
   kind override the tool-error classification?
9. How is `BudgetExceededError` documented to account for C3 (token-dimension
   overshoot)?
10. What lifecycle event does `VerdictLog` emit, and is it an existing event type
    or a new one?

## Decisions Required

- **D53** — OTel span tree structure: root span per invocation, child spans per
  state-machine phase, span attributes, span status mapping.
- **D54** — C1 resolution: `DetachedWithSpan` implementation contract — full span
  vs. span context, re-attachment mechanics.
- **D55** — CP1 nested span child-of semantics: child span vs. span link for
  composed orchestrators.
- **D56** — CP2 `parent_invocation_id` propagation: attribute on child span,
  enricher responsibility, or framework-injected.
- **D57** — Prometheus metric set: metric names, types (counter/histogram/gauge),
  label taxonomy, cardinality cap.
- **D58** — slog integration contract: structured fields, redaction rules, log-level
  mapping per event type, `slog.Handler` redaction.
- **D59** — `FilterDecision` → content-analysis event mapping: trigger conditions,
  `InvocationEvent` field population, emission point in the loop.
- **D60** — `AttributeEnricher` → span/event flow: when attributes are collected,
  how they attach to OTel spans and lifecycle events, cardinality enforcement.
- **D61** — Error-to-event mapping: which `ErrorKind` maps to which terminal
  `EventType`, edge cases (multiple errors, classifier fallback).
- **D62** — C3 token-overshoot documentation: `BudgetExceededError` godoc
  amendment, overshoot visibility in `BudgetSnapshot`.
- **D63** — CP5 classifier precedence: identity rule enforcement, unwrap-before-
  classify contract, precedence order for mixed-error scenarios.
- **D64** — `VerdictLog` event emission: event type, fields, and emission point.

## Assumptions

- The OTel SDK dependency (`go.opentelemetry.io/otel` and
  `go.opentelemetry.io/otel/trace`) is already approved (Phase 3
  `go-architect-package-layout.md` §10).
- `github.com/prometheus/client_golang` is the Prometheus client library
  (listed in Phase 3 `go-architect-package-layout.md` §10).
- The `telemetry` package is the right home for the `OTelEmitter` default
  implementation and any slog integration helpers.
- Terminal lifecycle emission always runs under the Layer 4 detached context
  (D22/D23) — this is a hard invariant, not a design choice for Phase 4.
- The 21 `EventType` constants (D52b) are fixed; Phase 4 does not add new
  event types beyond what D52b established. (**Weak assumption** — D59 or D64
  may surface a need for one additional event type; if so, it must be recorded
  as a seed amendment.)
- The `AttributeEnricher.Enrich` is called exactly once per invocation at
  `Initializing` state (Phase 3 `08-telemetry-interfaces.md`).

## Risks

**Critical:**

- **Cardinality explosion in metrics.** If `AttributeEnricher` attributes flow
  directly to Prometheus labels, callers with high-cardinality attributes (per-user
  IDs, per-request IDs) can blow up Prometheus storage. Phase 4 must define a
  hard boundary between enricher attributes on spans (high cardinality OK) and
  enricher attributes on metrics (cardinality must be bounded).
- **Decoupling contract violation.** Any hardcoded attribute key (e.g.,
  `tenant.id`, `agent.id`) in span or metric names would violate seed §6.1. The
  span and metric design must use only framework-defined attribute keys; caller
  identity attribution flows exclusively through `AttributeEnricher`.

**Secondary:**

- **OTel SDK version coupling.** The framework's span tree design is coupled to
  OTel SDK conventions. A major OTel SDK breaking change could require a praxis
  minor version bump during v0.x.
- **slog redaction completeness.** It is difficult to guarantee that no sensitive
  material leaks into structured logs without a comprehensive redaction handler.
  Phase 4 defines the contract; implementation must enforce it.
- **C1 span re-attachment correctness.** If `DetachedWithSpan` re-attaches only
  a `SpanContext` (not a full `Span`), the terminal lifecycle event will appear
  in the trace but the emitter cannot add attributes to the span. This is a
  design trade-off that D54 must resolve explicitly.

## Deliverables

- `00-plan.md` — this document.
- `01-decisions-log.md` — decisions D53–D64 (range may extend if needed).
- `02-span-tree.md` — OTel span tree structure, span lifecycle, C1 resolution,
  CP1/CP2 nested span semantics.
- `03-metrics.md` — Prometheus metric set, label taxonomy, cardinality constraints,
  histogram bucket boundaries.
- `04-slog-redaction.md` — slog structured fields, redaction rules, log-level
  mapping, `slog.Handler` contract.
- `05-error-event-mapping.md` — error-to-terminal-event mapping, C3 token-overshoot
  documentation, CP5 classifier precedence.
- `06-filter-event-mapping.md` — `FilterDecision` → content-analysis event mapping,
  `VerdictLog` emission, `AttributeEnricher` flow contract.
- `REVIEW.md` — phase review verdict.

## Recommended Subagents

- **observability-architect** — designs the OTel span tree, Prometheus metric set,
  slog redaction contract, and cardinality constraints. This is the core domain of
  Phase 4.
- **go-architect** — validates that the span tree, metrics, and slog integration
  fit cleanly into the Phase 3 package layout without introducing import cycles
  or new dependencies beyond those already approved.

## Exit Criteria

1. All decisions D53–D64 (or extended range) are adopted with rationale and
   trade-offs documented.
2. The OTel span tree is fully specified: every span has a name, parent, lifecycle
   (start/end state), and attribute set.
3. C1 is resolved: `DetachedWithSpan` contract is defined with enough precision
   to implement.
4. C3 is resolved: `BudgetExceededError` godoc includes the token-overshoot
   caveat.
5. CP1 and CP2 are resolved: nested orchestrator span correlation is defined.
6. CP5 is resolved: classifier precedence for propagated typed errors is defined.
7. The two content-analysis event types (`EventTypePIIRedacted`,
   `EventTypePromptInjectionSuspected`) have defined trigger conditions and
   `InvocationEvent` field population rules.
8. Prometheus metrics have defined names, types, labels, and cardinality cap.
9. slog structured fields and redaction rules are defined.
10. `AttributeEnricher` attribute flow to spans and events is defined with a
    cardinality boundary between spans and metrics.
11. Reviewer subagent returns PASS with no BLOCKER findings.
12. `REVIEW.md` verdict is READY.
13. No banned-identifier leakage in any Phase 4 artifact.
