# Roadmap Status

**Last updated:** 2026-04-10
**Target:** `praxis v1.0.0` — stable public Go API for enterprise agent orchestration

## Phase Status

| # | Phase | Status | Artifacts |
|---|-------|--------|-----------|
| 0 | Seed Context | starting baseline (amendable via decision-log amendment) | 1 (`docs/PRAXIS-SEED-CONTEXT.md`) |
| 1 | API Scope and Positioning | **approved** | 8 (incl. `REVIEW.md`) |
| 2 | Core Runtime Design | **approved** | 8 (incl. `REVIEW.md`) |
| 3 | Interface Contracts | **approved** | 14 (incl. `REVIEW.md`) |
| 4 | Observability and Error Model | **approved** | 9 (incl. `REVIEW.md`) |
| 5 | Security and Trust Boundaries | **approved** | 8 (incl. `REVIEW.md`) |
| 6 | Release, Versioning and Community Governance | **approved** | 8 (incl. `REVIEW.md`) |
| 7 | MCP Integration | **approved** | 8 (`00-plan.md`, `01-decisions-log.md`, `02-scope-and-positioning.md`, `03-integration-model.md`, `04-security-and-credentials.md`, `05-non-goals.md`, `research-solutions.md`, `REVIEW.md`) |
| 8 | Skills Integration | **not-started** (scaffolded) | 2 (`00-plan.md` *stub*, `01-decisions-log.md` *stub*) |

## Adopted Decisions

**119 decisions adopted** across Phases 1–7.

| Phase | Range | Adopted | Notes |
|---|---|---|---|
| 1 | D01–D15 | D01–D14 | D15 released unused |
| 2 | D15–D30 | D15–D28 | D29, D30 released unused |
| 3 | D31–D52 | D31–D52 | full range |
| 4 | D53–D66 | D53–D66 | full range |
| 5 | D67–D80 | D67–D80 | full range |
| 6 | D81–D105 | D81–D105 | full range |
| 7 | D106–D121 | D106–D121 | full range (D121 added during reviewer pass for the release-pipeline amendment obligation) |

Phase 8 (Skills Integration) reserves its first decision ID at **D122**
when the phase is activated by `plan-phase`. All decision ranges are
contiguous and non-overlapping.

## Completed Work

**Phase 1 — API Scope and Positioning.** Charter, positioning, design
principles, target consumer archetype and anti-persona, v1.0 freeze
surface (twelve interfaces frozen at v1.0, two held as
`stable-v0.x-candidate`), seven non-goals, "no plugins in v1"
re-confirmed as D09.

**Phase 2 — Core Runtime Design.** 13-state typed invocation machine
with property-based test harness, 16-event streaming channel,
two-flavor cancellation model, four-layer context model,
approval-required terminal state added alongside the existing
`Completed/Failed/Cancelled/BudgetExceeded` terminals.

**Phase 3 — Interface Contracts.** All original seed §5 interfaces at
`frozen-v1.0` (including `credentials.Resolver`, `tools.Invoker`,
`llm.Provider`, `hooks.*`, `budget.Guard`/`budget.PriceProvider`,
`errors.TypedError`/`errors.Classifier`,
`telemetry.LifecycleEventEmitter`/`telemetry.AttributeEnricher`,
`identity.Signer`) plus `MetricsRecorder`, plus `InvocationContext`
specified in detail. Canonical package layout committed.

**Phase 4 — Observability and Error Model.** OTel span tree (one root
+ six child spans, `praxis.*` namespace), 10 Prometheus metrics with
hard cardinality bounds, `AttributeEnricher` vs metric-label
boundary, `slog` redaction handler with static deny-list, eight-type
typed error taxonomy + event mapping, filter-phase event mapping,
`DetachedWithSpan` helper for terminal-event emission under Layer 4
context.

**Phase 5 — Security and Trust Boundaries.** Zero-on-close credential
lifecycle with `runtime.KeepAlive` pattern and `credentials.ZeroBytes`
utility, `context.WithoutCancel` + 500 ms deadline soft-cancel
credential fetch path, Ed25519 JWT signing with three praxis-specific
claims, trust-boundary classification of filter positions (`PreLLMFilter`
internal vs `PostToolFilter` crossing), untrusted tool output contract,
`RedactingHandler` deny-list extensions, five load-bearing security
invariants.

**Phase 6 — Release, Versioning and Community Governance.** Linear-
history merge policy (D81), commitsar conventional-commit validator
(D82), trunk-based development (D83), release-please configuration
with `go` release type and `version.go` extra-file (D84), CI pipeline
with banned-grep + spdx-check + dco + govulncheck (D85), 85 % coverage
gate across all packages (D86), SPDX license headers, governance model
codified, CODEOWNERS, CHANGELOG policy, contribution flow, v0.1.0
through v1.0.0 milestone checklists (D96), plus Phase 4 back-annotation
decisions D99–D105.

**Phase 7 — MCP Integration.** MCP ships as a separately-versioned Go
sub-module at `github.com/praxis-go/praxis/mcp` (D106) reusing the
official `modelcontextprotocol/go-sdk` (D107, conditional on license
and stability verification). v1.0.0 transport set: stdio + Streamable
HTTP (D108). D09 / Non-goal 7 re-confirmed (D109). Public API
(D110): `Server`, sealed `Transport` + `TransportStdio` +
`TransportHTTP`, `Option`, `WithResolver` / `WithMetricsRecorder` /
`WithTracerProvider` / `WithMaxResponseBytes`, `New`, `Invoker io.Closer`.
Tool namespacing `{LogicalName}__{mcpToolName}` with `__` prohibited
inside `LogicalName` (D111). Budget participation via existing
`wall_clock` + `tool_calls` dimensions only, with a 16 MiB
`MaxResponseBytes` adapter-local resource guard (D112). Error
translation maps all MCP failures to `ErrorKindTool` sub-kinds
including OAuth 401/403, HTTP 429, handshake timeout, TLS failure
(D113). Content flattening to text-only newline-joined output (D114).
MCP-specific metrics via a standalone `mcp.MetricsRecorder` interface
detected by type assertion (not D100 embedding), with 32-server cap
(D115). MCP transport edge classified as a Phase 5 trust boundary
handled by existing `PostToolFilter` contract (D116). Credential flow:
first-call fetch + session-reuse with explicit accepted deviation from
the Phase 5 §3.2 goroutine-scope isolation invariant for HTTP transport
(D117). `SignedIdentity` not forwarded to MCP servers in v1.0.0 (D118).
stdio transport hardening: absolute command resolution, env-var buffer
zeroing, pipe redirection, process-group isolation, `EPIPE`/SIGPIPE
handling, operator obligation for resource limits (D119). Ten non-goals
binding for `praxis/mcp v1.0.0` (D120). Phase 6 release-pipeline
amendment obligation recorded as a new decision so the
"independently-versioned sub-module" claim in D106 has a concrete
pipeline foundation (D121).

## Open Decisions

None blocking at the Phase 7 level. The following items are explicit
forward-carries that belong to later work:

- **Phase 7 implementation-phase preconditions (D107).** Before the
  first `praxis/mcp` `go.mod` commit, verify (a) official SDK license
  is Apache-2.0-compatible, (b) transitive dependency footprint is
  acceptable, (c) written v1 stability statement exists. Failure on
  any precondition re-opens D107.
- **D121 pipeline diff must be applied to the repository** before any
  `praxis/mcp` tag is cut. This is an implementation-phase obligation,
  not a planning-phase blocker.
- **Phase 8 (Skills Integration)** is still `not-started`. Activation
  is gated on Phase 7 approval, which is now in place. Next working
  loop: run `plan-phase` on Phase 8.

## Risks / Blockers

- **R1 — `research-solutions.md` `[verify]` markers.** Phase 7's
  research artifact was produced under time pressure after the
  solution-researcher subagent stalled. Multiple factual claims about
  the official Go MCP SDK (exact license, transitive-dep count,
  written stability statement) are `[verify]`-tagged and must be
  independently verified before the first `praxis/mcp go.mod` commit.
  D107 records the preconditions; the `REVIEW.md` recommendations
  restate them.
- **R2 — SDK concurrency guarantee is unverified.** If the official
  MCP Go SDK is not safe for concurrent `tools/call` on a single
  session, the adapter's per-server mutex fallback will serialise
  same-server tool calls, partially defeating Phase 2 D24 parallel
  dispatch. Benchmarking obligation recorded in `REVIEW.md`.
- **R3 — HTTP-transport credential goroutine-scope invariant is
  breached.** Phase 5 §3.2's structural invariant does not hold for
  HTTP transport in the MCP adapter; this is an **accepted deviation**
  (D117), not a blocker, and is classified at the same tier as D67
  §4.3. Consumers with strict isolation requirements use stdio
  transport exclusively.
- **R4 — Pattern precedent risk.** Phase 7 establishes the
  "standalone optional interface + type assertion" pattern for
  sub-module metrics (D115). Phase 8 should use the same pattern for
  consistency. If Phase 8 diverges, the praxis sub-module extension
  story becomes fragmented.

No blockers prevent Phase 8 from starting. No blockers prevent v0.1.0
or v0.3.0 implementation work.

## Decoupling Contract Health

**PASS across all approved phases.** The banned-identifier grep is
clean over Phases 1–7 modulo meta-mentions (permitted quotations of
banned identifiers inside compliance-check sections). No hardcoded
consumer-brand names, no `governance_event`, no hardcoded framework
identity attributes, no external-repo decision IDs or milestone codes,
no consumer-specific file paths. Phase 7's `03-integration-model.md`
§9 "Decoupling contract compliance" section contains a meta-mention
consistent with the pattern used in prior phases' review files.

## Next Step

Run `plan-phase` on **Phase 8 — Skills Integration** to begin the
provider-side skills analysis. Phase 8 inherits Phase 7's namespacing
convention, credential flow, error taxonomy, and
`MetricsRecorder`-style extension pattern as direct inputs.

## Overall Status

**v0.1.0 (first working invocation) and v0.3.0 (feature complete at
v1.0-candidate interface shape)** are unblocked by planning — Phases
1–6 are approved and specify everything the implementation needs to
ship Anthropic + OpenAI providers, the full state machine, hooks,
filters, budget, telemetry, and the release/CI pipeline.

**v0.5.0 (production-consumer ready)** additionally requires
Phases 7 and 8. Phase 7 is now **approved**, Phase 8 remains
`not-started`; v0.5.0 is still on track but **the v1.0.0 freeze
remains gated on Phase 8** per the charter's "all 8 planning phases
approved before implementation begins" commitment. Phase 7's work
gives Phase 8 its four required inputs.
