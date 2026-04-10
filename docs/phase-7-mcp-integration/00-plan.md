# Phase 7: MCP Integration

> **Status:** `approved` — activated 2026-04-10, approved 2026-04-10
> after one reviewer pass + in-place amendments. Decision range
> **D106–D121** (16 decisions, contiguous). `REVIEW.md` verdict:
> **READY**. See `docs/roadmap-status.md` for overall phase table.
> Phase 8 (Skills Integration) is now unblocked and opens at D122.

## Goal

Decide whether and how praxis supports the **Model Context Protocol (MCP)**
at v1.0.0, without violating the "no plugins in v1" commitment
(Phase 1 D09, Non-goal 7). Specify the integration model (pattern-only vs.
in-tree adapter), the credential flow, the observability extensions, and
the impact on the frozen v1.0 interface surface.

## Motivation

MCP is not mentioned anywhere in the seed context or in Phases 1–6. Before
freezing v1.0.0, the project needs an explicit decision: either MCP is
supported (and how), or it is an explicit non-goal with a documented
workaround via the `tools.Invoker` seam. Leaving this unresolved at freeze
would force every consumer that wants MCP to invent their own integration,
and would risk a breaking change in v1.x or a forced v2.

## Preliminary Scope

**In scope (to be refined by `plan-phase`):**

- Positioning: is MCP a first-class integration target for v1.0, a
  documented pattern, or an explicit non-goal?
- Integration model: in-tree adapter package (e.g., `tools/mcp`) vs.
  external integration guide vs. both.
- Transport support priority: stdio, HTTP, SSE — which ship first, which
  are deferred.
- Credential flow: mapping `credentials.Resolver` onto MCP server
  authentication (env vars, headers, tokens).
- Tool namespacing: convention for `ToolCall.Name` when multiple MCP
  servers are bridged (e.g., `server/tool` prefixing) vs. consumer-managed.
- Error translation: mapping MCP error responses through
  `errors.Classifier` into the typed error taxonomy.
- Budget accounting: how MCP-backed tool calls participate in the
  `Budget` without violating D08 (no dynamic pricing updates).
- Untrusted output handling: extending the Phase 5 model (D77) to content
  returned by MCP servers.
- Observability: span structure extensions for MCP calls, metric labels,
  cardinality constraints. Must respect Phase 4 D60 hard cardinality
  boundary.
- Trust boundary: the praxis process ↔ MCP server transport as a trust
  edge. Propagation of `SignedIdentity` to MCP-backed tools.
- Dependency impact: if in-tree, what SDK is used, licence compatibility,
  impact on `go.mod` of v1.0, build-tag isolation.
- Compatibility with D09 / Non-goal 7: explicit confirmation that any
  in-tree bridge is **build-time interface composition**, not dynamic
  plugin loading.

**Out of scope:**

- A dynamic MCP server discovery mechanism (would violate D09).
- An MCP *server* implementation exposing praxis agents over MCP
  (different direction; separate future phase if demanded).
- Runtime tool registration (would violate D09).
- Implementation code (that belongs to the implementation phase, post-Phase 6).

## Key Questions

1. Is MCP support in-tree for v1.0, or deferred to post-v1.0 with
   documentation only?
2. If in-tree, which MCP Go SDK is used, and what is its licence?
3. What is the minimum transport set for v1.0 (stdio only, stdio+HTTP, all)?
4. How does `credentials.Resolver` map onto MCP server authentication
   without leaking the Phase 5 credential model?
5. Is there a standard namespacing convention for `ToolCall.Name` when an
   `Invoker` fronts multiple MCP servers, or is that left to the consumer?
6. How do MCP errors map onto `ErrorKind` (Phase 4 D61 taxonomy)?
7. Does an MCP-backed tool call count toward `Budget` the same way a local
   tool call does, or does it need a new budget dimension (transport
   latency, bytes over wire)?
8. Does `PostToolFilter` (Phase 5 D77, D78) need any changes to handle
   MCP-sourced content, or is the existing untrusted-output contract
   sufficient?
9. Does Phase 4 span structure need a new child span for the MCP transport
   hop, and if so, how is cardinality bounded?
10. Should `SignedIdentity` be forwarded to MCP servers (as a JWT claim,
    header, or not at all)?
11. If MCP is in-tree, is it a top-level package (`tools/mcp`) or an
    `internal/` helper consumed by a thin public wrapper?
12. Does this phase require any amendment to the Phase 1 non-goals list?

## Decisions Required

Decision IDs will be allocated contiguously starting at **D106** when the
phase is activated by `plan-phase`. The list below is indicative of the
decisions expected to come out of this phase; the actual allocation is
recorded in `01-decisions-log.md`.

- Positioning of MCP in v1.0 (first-class, pattern-only, non-goal).
- Integration model (in-tree package vs. guide).
- Transport priority list.
- Credential flow specification.
- Tool namespacing convention (or explicit deferral).
- Error translation mapping.
- Budget participation model.
- Untrusted-output contract extension (if any).
- Observability span/metric extensions.
- Trust-boundary model for the transport edge.
- Dependency impact and build-tag isolation rules.
- Compatibility confirmation with D09 / Non-goal 7.

## Assumptions

- **`tools.Invoker` is the only framework seam MCP should use.** The
  interface is `frozen-v1.0` and was explicitly designed as the "connector
  layer or consumer-specific tool gateway" (seed §6.2). Any MCP support
  lives behind or around this interface.
- **D09 is not re-opened by this phase.** Phase 7 must find a solution
  that is compatible with build-time composition only. Any proposal that
  requires a runtime plugin mechanism is out of scope and must be
  rejected.
- **No change to the Phase 3 interface surface.** Phase 7 may add new
  packages but must not modify `Invoker`, `ToolCall`, `ToolResult`, or
  `InvocationContext` signatures in ways that break the frozen contract.
- **Phase 4 cardinality rules apply to any new metrics.** Any MCP-specific
  metric labels must respect the hard cardinality boundary (D60).
- **Phase 5 trust-boundary classification applies.** An MCP transport edge
  is a trust boundary; the Phase 5 model must be extended, not replaced.

## Risks

- **R1 — Scope creep into a full MCP SDK.** Temptation to ship a complete
  MCP client inside praxis. Must be bounded by what a bridge adapter
  actually needs.
- **R2 — Hidden dependency bloat.** An in-tree MCP SDK pulls transitive
  dependencies that may conflict with the "stdlib-favoured" posture of
  Phase 5 (D73).
- **R3 — Tool-name collision across servers.** Without a standard
  namespacing convention, multi-server consumers will invent incompatible
  schemes.
- **R4 — Credential leakage through transport headers.** Naive mapping of
  `credentials.Resolver` output into MCP transport headers could
  circumvent the Phase 5 zero-on-close guarantee (D67, D68).
- **R5 — Unbounded cardinality from MCP server/tool labels.** If MCP
  server names or remote tool names become metric labels, the worst-case
  time-series count explodes past Phase 4 D57 budget.
- **R6 — D09 pressure.** A powerful adapter will attract requests for
  "runtime MCP server discovery", which Phase 7 must pre-emptively decline.

## Deliverables

Status as of 2026-04-10 (phase **approved**):

- `00-plan.md` — this file. Phase scoping + working-loop tracker.
- `01-decisions-log.md` — ✅ **approved.** D106–D121 adopted
  (16 decisions, contiguous).
- `02-scope-and-positioning.md` — ✅ **approved.** First-class shipped
  as a separately-versioned sub-module at
  `github.com/praxis-go/praxis/mcp`.
- `03-integration-model.md` — ✅ **approved.** Package layout, public
  API, namespacing, budget flow, error translation, content
  flattening, observability extensions, testability via the SDK's
  `InMemoryTransport`.
- `04-security-and-credentials.md` — ✅ **approved.** Trust-boundary
  classification, credential flow for long-lived sessions with the
  accepted Phase 5 §3.2 goroutine-scope deviation for HTTP transport,
  `SignedIdentity` propagation policy, stdio hardening including
  SIGPIPE.
- `05-non-goals.md` — ✅ **approved.** Ten non-goals catalogued,
  binding for `praxis/mcp v1.0.0`; Non-goal 7.3 includes the explicit
  `tools/list_changed` "ignore" posture.
- `research-solutions.md` — ✅ **approved (verified pass).** Replaced
  with the solution-researcher's sourced output; two of three D107
  preconditions closed (license ✓, stability statement ✓); only the
  transitive dep audit remains as an implementation-phase gate.
- `REVIEW.md` — ✅ **verdict: READY.** Ten important weaknesses from
  the reviewer pass addressed in-place via amendments dated
  2026-04-10; no critical issues.

## Recommended Subagents

1. **solution-researcher** — survey the Go MCP SDK ecosystem (maturity,
   licences, transport support, import footprint), document prior art of
   MCP-bridge patterns in other agent orchestrators.
2. **api-designer** — evaluate impact on the frozen v1.0 interface
   surface; decide whether a new typed primitive is needed or whether
   `tools.Invoker` alone is enough.
3. **dx-designer** — consumer wiring patterns, error messages, example
   code, documentation story.
4. **security-architect** — credential flow, trust boundary, untrusted
   output, `SignedIdentity` propagation.
5. **observability-architect** — span extensions, metric labels,
   cardinality analysis.
6. **go-architect** — package layout (`tools/mcp` vs. `internal/`),
   dependency isolation, build tags.
7. **reviewer** (fixed) — phase closure per standard loop.

## Exit Criteria

1. A clear positioning decision for MCP in v1.0 (first-class / pattern /
   non-goal).
2. If first-class: package layout, transport list, and `go.mod` impact
   are fully specified.
3. If pattern-only: a reference integration guide exists with a complete
   worked example of an `mcpInvoker` implementation.
4. All decisions in `01-decisions-log.md` with rationale.
5. Compatibility with D09 and Non-goal 7 explicitly confirmed in the
   decision log.
6. Banned-identifier grep clean on all Phase 7 artifacts.
7. Reviewer subagent PASS.
8. `REVIEW.md` verdict: READY.
9. Phase 8 (Skills Integration) has the inputs it needs from Phase 7
   (namespacing convention, credential flow, error taxonomy mapping).
