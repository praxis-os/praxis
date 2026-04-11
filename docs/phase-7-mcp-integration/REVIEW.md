# Review: Phase 7 — MCP Integration

## Overall Assessment

Phase 7 delivers a well-bounded first-class MCP integration for praxis,
shipped as a separately-versioned sub-module at
`github.com/praxis-os/praxis/mcp`. Sixteen decisions (D106–D121) cover
positioning, public API, namespacing, budget participation, error
translation, content flattening, observability, credential flow, trust
boundary, stdio hardening, non-goals, and the required Phase 6
release-pipeline amendment. After one reviewer pass that surfaced ten
important weaknesses (no critical issues), every finding has been
addressed in-place through amendments dated 2026-04-10. A verified
solution-research pass landing shortly after the initial draft
confirmed the SDK choice, closed two of the three D107 preconditions
(license ✓, stability statement ✓), replaced `research-solutions.md`
with a sourced version, and contributed a testability simplification
(the official SDK ships `InMemoryTransport`, removing the need for a
praxis-owned fake transport) and an explicit `tools/list_changed`
posture (ignored in v1.0.0 per Non-goal 7.3, matching the ecosystem
baseline). The decoupling contract is clean, no frozen-v1.0 interface
is modified, D09 / Non-goal 7 is preserved, and the readiness
criteria for Phase 8 inputs are satisfied.

## Critical Issues

None.

The initial reviewer pass found zero CRITICAL issues. Ten IMPORTANT
weaknesses were raised and are all resolved; summary below under
**Resolved findings from first reviewer pass**.

## Important Weaknesses

None remaining. See **Resolved findings from first reviewer pass**
below for the full trail.

## Resolved findings from first reviewer pass

All ten items from the first reviewer pass have been addressed. The
resolution for each:

1. **Release-pipeline gap.** `D106` originally claimed Phase 6's
   release-please configuration "already accommodates" multi-module
   releases. This was factually wrong: D84's manifest contains a
   single `.` package entry. **Resolved** by amending D106 with a
   "Release-pipeline dependency" block that makes D106 conditional
   on D121, and by adding **D121 — Phase 6 release-pipeline amendment
   for the `praxis/mcp` sub-module**, which records the exact
   `packages` / `manifest` diff required before any `praxis/mcp`
   tag is cut, plus the `feat(mcp):` / `fix(mcp):` commit-scope
   convention.
   (`01-decisions-log.md` D106 amendment + D121; `02-scope-and-positioning.md` §9 unaffected prose.)

2. **Credential goroutine-scope isolation breach (HTTP transport).**
   `04-security-and-credentials.md` §2.2 originally conflated the
   D67 zeroing boundary with the Phase 5 §3.2 goroutine-scope
   isolation invariant. **Resolved** by adding §2.2.1 "Accepted
   deviation from Phase 5 §3.2 (HTTP transport only)" to
   `04-security-and-credentials.md`, and by amending D117 with an
   "Accepted deviation from Phase 5 §3.2" block. The breach is
   explicitly acknowledged, explained (HTTP connection reuse is a
   non-negotiable), scoped to HTTP only (stdio is unaffected), and
   classified at the same "acceptable risk" tier as D67 §4.3.

3. **Misleading D100 reference for `mcp.MetricsRecorder`.**
   `03-integration-model.md` §7.2 described the extension pattern
   as "consistent with the `MetricsRecorderV2` extension pattern in
   D100". That claim was wrong: D100 uses embedding;
   `mcp.MetricsRecorder` is a standalone interface in a different
   package. **Resolved** by rewriting the godoc block in
   `03-integration-model.md` §7.2 to explicitly disown the D100
   embedding pattern, by adding a "Detection mechanism" amendment
   block under D115 that specifies the exact type-assertion
   mechanism plus the silent-drop fallback behaviour, and by
   updating the D115 one-liner in the integration-model summary.

4. **Incomplete error taxonomy.** D113's table missed OAuth 401/403
   on an established session, HTTP 429, single handshake timeout,
   and TLS handshake failure. **Resolved** by extending the D113
   mapping table with five new rows (including the
   `MaxResponseBytes` overrun from the D112 amendment), plus
   explanatory amendment blocks for the 401/403 → `CircuitOpen` and
   429 → `Network` choices.

5. **Empty-content edge case for `PostToolFilter`.** D114's
   text-only flattening created a new valid
   `Status == ToolStatusSuccess && Content == ""` combination that
   the Phase 3 `ToolResult` documentation does not anticipate.
   **Resolved** by adding a "Contract note for `PostToolFilter`
   implementors" amendment block to D114 that explicitly calls out
   the new combination, warns filter authors not to treat it as a
   framework bug, and notes that the adapter godoc documents it.

6. **Sealed `Transport` testability gap.** The original
   `03-integration-model.md` specified a sealed `Transport`
   interface but had no story for unit-testing the credential
   lifecycle, error mapping, or content flattening paths without
   live child processes or HTTP servers. **Resolved** by adding §8a
   "Testability" to `03-integration-model.md`, which specifies an
   `internal/transport/fake/` sub-package that provides an
   in-memory MCP session double. The fake is kept behind `internal/`
   to preserve the sealed-transport guarantee for consumers while
   satisfying the module's own 85 % coverage gate.

7. **`__` namespacing silent-dispatch hazard.** The original
   `LogicalName` validation regex (`[a-zA-Z0-9_-]{1,64}`) permitted
   `__` inside `LogicalName`, creating a scenario where the
   leftmost-split recovery routed to the wrong server silently.
   **Resolved** by amending D111 with an explicit "`LogicalName`
   validation" block that prohibits two consecutive underscores
   anywhere in `LogicalName` (hard validation at `New` time) and
   documents that the prohibition preserves correctness of the
   leftmost-split recovery for server-defined tool names that
   contain `__`.

8. **No testability story for zero-on-close after session-open.**
   Same root cause as finding 6. **Resolved** by the same
   `internal/transport/fake/` mechanism, which allows the
   credential-lifecycle unit tests to assert buffer zeroing without
   a live MCP session.

9. **stdio hardening omitted SIGPIPE and child resource limits.**
   **Resolved** by adding §4.4 "SIGPIPE handling" and §4.5 "Child
   resource constraints" to `04-security-and-credentials.md`, and
   by amending D119 with two new requirements: (6) `EPIPE` / SIGPIPE
   handling converted to `ToolSubKindNetwork`, and (7) a documented
   operator obligation to supervise untrusted MCP binaries under
   external resource limiters (systemd, launchd, container) in
   v1.0.0, with a v1.x backlog item for a cross-platform
   `WithChildRLimits` option.

10. **Budget gap: unbounded streaming response size.** D112
    originally claimed `wall_clock` + `tool_calls` were sufficient.
    **Resolved** by amending D112 with a "Known gap: streaming
    response size" block that mandates a `MaxResponseBytes`
    adapter-local resource guard (default 16 MiB), maps the overrun
    to `ToolSubKindServerError` (new D113 row), and adds a
    `WithMaxResponseBytes` option to the D110 public API surface.
    Explicitly classified as an adapter-local resource guard
    operating outside the `budget.Guard` model (no new budget
    dimension — the v1.0 `budget.Guard` freeze is preserved).

## Open Questions

The following items carry forward. The list was significantly
narrowed by a verified-research pass that landed after the initial
draft (`research-solutions.md` was replaced with the researcher's
sourced output on 2026-04-10). Items 1a and 1b below are **closed**
by the verified research; items 1c onward remain open.

1. **D107 preconditions — status update.**
   - **1a. License verification — CLOSED.** Confirmed Apache-2.0
     for new contributions (older MIT being migrated). Compatible
     with praxis's Apache-2.0.
   - **1b. Stability statement — CLOSED.** v1.0.0 (Sep 30, 2025)
     explicit "no breaking API changes" commitment, sustained
     across 22 tagged releases to v1.5.0 (Apr 7, 2026).
   - **1c. Transitive dep audit — STILL OPEN.** The header count
     (9 direct/indirect) is manageable, but `golang.org/x/oauth2`
     and `golang.org/x/tools` in the SDK's go.mod may pull in
     heavy transitive deps (grpc, large net stack). Must be
     resolved with `go mod graph` before the first `praxis/mcp`
     `go.mod` commit. Failure mode: fall back to pattern-only
     reference implementation per `research-solutions.md` §5.3
     — NOT to a different SDK (both community alternatives are
     disqualified: `mark3labs/mcp-go` pre-v1.0, `metoro-io/mcp-golang`
     abandoned).

2. **SDK concurrency guarantee.** `research-solutions.md` §6 Open
   Question 6 asks whether the official SDK's client is safe for
   concurrent `tools/call` invocations on a single session. The
   adapter's per-server serialisation fallback is specified in
   03 §8, but the actual serialisation cost under Phase 2 D24
   parallel-dispatch conditions is unknown until benchmarked. This
   is an implementation-phase obligation.

3. **Session survival across soft-cancel mid-call.**
   `04-security-and-credentials.md` §2.3 covers the first-call
   session-open soft-cancel path, but does not specify whether a
   subsequent call on an already-open session, interrupted by
   soft-cancel, leaves the session recoverable or forces a
   teardown. This is a v1.0 implementation-phase decision (likely:
   tear down and let the next call re-open, matching the
   circuit-open cool-down posture), but the artifact does not
   commit.

4. **`WithMetadataHeaders` as a future v1.x option.**
   `03-integration-model.md` §9 references this as a "future
   option" but D110's minimal-surface commitment does not
   explicitly address whether new options may be added
   non-breakingly in v1.x. Functional options ARE the standard
   Go pattern for non-breaking v1.x additions, and the underlying
   Go type system permits it, so this is a documentation gap
   rather than an architectural gap — but Phase 7 could usefully
   state the rule explicitly ("new `WithX` options may be added in
   v1.x minor releases; existing options are frozen").

5. **Phase 8 readiness.** Phase 8 (Skills Integration) consumes
   Phase 7's namespacing convention, credential flow, error
   taxonomy, and observability extension pattern. With the
   amendments above, all four inputs are now specified. The
   `mcp.MetricsRecorder` type-assertion pattern (D115) is
   particularly important: Phase 8 will likely need to replicate
   this pattern for skills-specific metrics, and the
   standalone-interface-plus-type-assertion idiom is now
   precedented in the praxis codebase.

## Decoupling Contract Check

**PASS.**

Banned-identifier grep over `docs/phase-7-mcp-integration/` reports:

| File:line | Match | Classification |
|---|---|---|
| `03-integration-model.md` §9 (line ~598) | `org.id`, `agent.id`, `user.id`, `tenant.id` | **Meta-mention** — inside §9 "Decoupling contract compliance" as part of a compliance reminder listing the banned identifiers. This matches the pattern used in Phase 2, Phase 3, Phase 4, Phase 5, and Phase 6 REVIEW.md files. Permitted. |
| `REVIEW.md` (this file) | `org.id`, `agent.id`, `user.id`, `tenant.id`; `custos`, `reef`, `governance_event` | **Meta-mention** — this compliance-check section itself. Permitted by convention. |

No hits for `custos`, `reef`, or `governance_event` as hardcoded uses
anywhere in any Phase 7 file. No consumer-specific file paths. No
hardcoded framework attribute keys. No decision IDs from external
repositories. No milestone codes from other projects.

## Hard Checks

- **Banned identifier grep: PASS** (meta-mentions only).
- **Frozen-v1.0 interface modification: PASS.** Phase 7 introduces no
  amendments to `tools.Invoker`, `ToolCall`, `ToolResult`,
  `InvocationContext`, `credentials.Resolver`, `identity.Signer`,
  `budget.Guard`, `errors.Classifier`, `telemetry.LifecycleEventEmitter`,
  or any other frozen-v1.0 interface.
- **D09 / Non-goal 7 compliance: PASS.** D109 re-confirms the
  commitment. No runtime discovery (Non-goal 7.1), no runtime tool
  registration (Non-goal 7.3), no dynamic SDK version switching
  (Non-goal 7.10), no plugin loading, no reflection-based extension.
  Adapter configuration is fully build-time.
- **Phase 4 cardinality compliance: PASS.** D115 adds three MCP-
  specific metrics with bounded labels. The 32-server hard cap at
  `New` time enforces the `server` label's bound. The worst-case
  time-series count is ~704, well within the ~1,032 ceiling
  documented in Phase 4 §6.
- **Phase 5 credential lifecycle compliance: CONDITIONAL PASS.** D67
  zero-on-close is preserved up to the SDK boundary. D69 soft-cancel
  rules are inherited verbatim for first-call fetch. D77, D78, D79
  are preserved unchanged. The Phase 5 §3.2 goroutine-scope
  isolation invariant is explicitly breached for HTTP transport and
  recorded as an accepted deviation in D117 — this is tier-matched
  to D67 §4.3 and documented in adapter godoc.
- **Default implementations: PASS.** The `Invoker` returned by
  `New` is itself the working default; the internal fake transport
  provides the testing default; the `mcp.MetricsRecorder` fallback
  path is specified as silent drop-to-null when the passed recorder
  does not implement the extension interface.
- **Composability: PASS.** `mcp.Invoker` satisfies the frozen
  `tools.Invoker` contract and composes through the standard
  `Orchestrator` wiring. Consumers who need to wrap it (for
  `SignedIdentity` forwarding, custom namespacing, MCP-specific
  policy) build a standard wrapping `Invoker`. No adapter-layer
  hooks are introduced.
- **Testability: PASS.** The `internal/transport/fake/` pattern
  gives the module a testable foundation that does not require live
  child processes or live HTTP servers; the D86 85 % coverage gate
  is achievable without integration infrastructure in CI.
- **Seed context consistency: PASS.** The sub-module at
  `mcp/` is consistent with seed §7's repository layout (which does
  not preclude additional `go.mod` files). The seed's "no plugins"
  principle (§3.6) is preserved. The decoupling contract (§6) is
  preserved. No seed edit is required.

## Recommendations

Phase 7 is ready for approval. The recommendations below are forward-
looking items that belong to the implementation phase or to Phase 8,
not blockers for Phase 7 sign-off:

- **Apply D121 before any `praxis/mcp` commit lands.** The
  release-please configuration diff specified in D121 must be applied
  to the repository's `.github/release-please-config.json` and
  `.github/release-please-manifest.json` files in a dedicated commit
  that cites D121, **before** the first `praxis/mcp` code commit.
  Otherwise the first tag will not cut correctly.

- **Resolve `research-solutions.md` `[verify]` markers during the
  implementation-phase handoff.** Before the `praxis/mcp` `go.mod` is
  committed, verify: official SDK license (must be Apache-2.0 or
  compatible), v1 stability statement presence, and the actual
  transitive-dependency footprint. If any of the three fails, raise a
  Phase 7 amendment.

- **Benchmark the SDK concurrency path in the implementation phase.**
  If the official SDK is not safe for concurrent `tools/call` on a
  single session, the per-server mutex in the adapter will serialise
  same-server tool calls, partially defeating Phase 2 D24 parallel
  dispatch. This is a known risk — benchmark and document the actual
  cost before cutting `praxis/mcp v0.1.0`.

- **State the "new `WithX` options are non-breaking in v1.x" rule
  explicitly** in D110's prose or in the module godoc, so that future
  v1.x additions (`WithMetadataHeaders`, `WithChildRLimits`, custom
  `http.Client`) can land without re-opening the freeze-surface
  commitment.

- **Coordinate Phase 8 (Skills Integration) on the
  `mcp.MetricsRecorder` type-assertion pattern.** Phase 8 will likely
  need the same idiom for skills-specific telemetry; using the same
  shape twice establishes it as the praxis-canonical extension
  pattern for sub-module metrics.

- **Document the PostToolFilter empty-content edge case in the core
  Phase 3 `ToolResult` godoc** when the `praxis/mcp` module is
  landed. This is a Phase 3 doc amendment, not a Phase 3 decision
  amendment — the comment text under `ToolResult.Content` should be
  extended to mention that `Status == ToolStatusSuccess && Content
  == ""` is valid for tools that return only non-text content.

## Verdict: READY

Phase 7 is ready for approval. The ten important weaknesses from the
first reviewer pass are each addressed by a specific amendment recorded
in `01-decisions-log.md`, the decoupling contract is clean, the frozen
v1.0 interface surface is unmodified, D09 is preserved, cardinality is
bounded, the credential-lifecycle deviation is explicitly acknowledged,
and the inputs Phase 8 needs (namespacing, credential flow, error
taxonomy, metrics extension pattern) are all specified.
