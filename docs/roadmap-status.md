# Roadmap Status

**Last updated:** 2026-04-10
**Target:** `praxis v1.0.0` — stable public Go API for enterprise agent orchestration

## Phase Status

| # | Phase | Status | Artifacts |
|---|-------|--------|-----------|
| 0 | Seed Context | locked | 1 (`docs/PRAXIS-SEED-CONTEXT.md`) |
| 1 | API Scope and Positioning | **approved** | 7 numbered + `REVIEW.md` |
| 2 | Core Runtime Design | **approved** | 6 numbered + `REVIEW.md` |
| 3 | Interface Contracts | **approved** | 11 numbered + `REVIEW.md` (+ go-architect note) |
| 4 | Observability and Error Model | **approved** | 5 numbered + `REVIEW.md` (+ go-architect note) |
| 5 | Security and Trust Boundaries | **approved** | 5 numbered + `REVIEW.md` (+ go-architect note) |
| 6 | Release, Versioning and Community Governance | **approved** | 6 numbered + `REVIEW.md` |
| 7 | MCP Integration | **approved** | 5 numbered + `research-solutions.md` + `REVIEW.md` |
| 8 | Skills Integration | **approved** | 5 numbered + `research-solutions.md` + `REVIEW.md` |

**All 8 planning phases are `approved`.** Implementation is unblocked.

## Locked Decisions

**Total:** ~133 decisions adopted across **D01–D134** (contiguous
range; a small number of reserved/released slots within individual
phases, per each phase's decision log).

Per-phase allocation:

| Phase | Range | Notes |
|---|---|---|
| 1 | D01–D14 | D15 released to Phase 2 |
| 2 | D15–D30 | D29, D30 reserved and released unused |
| 3 | D31–D52 | 22 decisions on the frozen v1.0 interface surface |
| 4 | D53–D66 | 14 decisions on observability + error taxonomy |
| 5 | D67–D80 | 14 decisions on credential, identity, trust boundaries |
| 6 | D81–D105 | 25 decisions on release + governance |
| 7 | D106–D121 | 16 decisions on `praxis/mcp` sub-module |
| 8 | D122–D134 | 13 decisions on `praxis/skills` sub-module |

## Completed Work

- **Phase 1** — positioning vs existing Go libraries, design
  principles, v1.0 freeze surface (14 interfaces frozen),
  non-goals, seed-question resolutions, composition patterns.
  "No plugins in v1" rule (D09) established as a hard gate.
- **Phase 2** — invocation state machine (14 states, 5 terminal),
  streaming channel protocol, cancellation semantics, concurrency
  model (one goroutine per invocation, sole-producer), 21 property
  invariants, zero-wiring streaming event set.
- **Phase 3** — all public v1.0 interfaces locked as concrete
  signatures: `AgentOrchestrator`, `llm.Provider`, `tools.Invoker`,
  `hooks.PolicyHook` / `PreLLMFilter` / `PostToolFilter`,
  `budget.Guard` / `PriceProvider`, `errors.TypedError` /
  `Classifier`, `telemetry.LifecycleEventEmitter` /
  `AttributeEnricher`, `credentials.Resolver`, `identity.Signer`.
  Defaults (`NullInvoker`, `NullResolver`, etc.) and construction
  pattern specified.
- **Phase 4** — OTel span tree, Prometheus metrics, slog
  redaction, typed error taxonomy, error-to-event mapping,
  `AttributeEnricher` contract, cardinality boundary (D60).
- **Phase 5** — credential zero-on-close, Ed25519 identity signing,
  trust-boundary classification, untrusted-output contract (D77,
  D78), stdlib-favoured posture (D73), filter-hook trust model.
- **Phase 6** — semver policy, v0→v1.0 stability commitment, v2+
  module path rules, release-please pipeline, CI pipeline (lint,
  test, coverage, benchmarks, banned-identifier grep, govulncheck,
  codeql), contribution model, RFC process, code of conduct.
- **Phase 7** — `github.com/praxis-go/praxis/mcp` separately
  versioned sub-module. Transport priority (stdio + Streamable
  HTTP), tool namespacing (`{LogicalName}__{mcpToolName}`),
  credential flow for long-lived sessions, trust-boundary
  classification of the transport edge, stdio hardening, minimal
  public API surface. Official `modelcontextprotocol/go-sdk` as
  reuse target (conditional on implementation-phase transitive-dep
  audit).
- **Phase 8** — `github.com/praxis-go/praxis/skills` separately
  versioned sub-module. Canonical `SKILL.md` shape anchored to
  agentskills.io spec intersection (`name`, `description` required;
  `license`, `compatibility`, `metadata`, `allowed-tools` optional).
  Permissive-preserve policy for unknown fields via
  `Skill.Extensions() map[string]any`. `Open(fs.FS, root)` +
  `Load(path)` loader, `skills.WithSkill(s)` orchestrator option,
  panic-on-duplicate-name collision preserving the frozen
  `NewOrchestrator` single-return signature. `praxis/skills` does
  NOT import `praxis/mcp`; callers compose both explicitly.
  11-item binding non-goals catalogue. Zero amendments to Phase 3
  frozen surface.

## Open Decisions

None at the planning level. All decisions in D01–D134 are locked.
Three items are explicitly deferred to the implementation phase
by Phase 8's reviewer:

- YAML parser final choice and govulncheck gate (OPEN-02 residual).
  Primary candidate: `gopkg.in/yaml.v3`.
- Case-insensitive filesystem behaviour of `skills.Open` on
  macOS/APFS and Windows NTFS.
- `skills.ComposedInstructions` helper behaviour on non-skill
  options in the variadic list.

One Phase 7 item remains as an implementation-phase gate:

- Transitive-dependency audit of `modelcontextprotocol/go-sdk` per
  D107 precondition 3.

## Risks / Blockers

None blocking. All planning-phase risks have been mitigated or
converted to implementation-phase gates. The v1.0.0 freeze is
**unblocked on every axis**:

- Core runtime: frozen via Phase 2 / Phase 3.
- Observability and errors: frozen via Phase 4.
- Security: frozen via Phase 5.
- Release and governance: frozen via Phase 6.
- **MCP extensibility:** frozen via Phase 7 (`praxis/mcp`
  sub-module at independent semver line).
- **Skills extensibility:** frozen via Phase 8 (`praxis/skills`
  sub-module at independent semver line).

## Decoupling Contract Health

**PASS.** Banned-identifier grep across all phase directories
(`custos`, `reef`, `governance_event`, hardcoded
`org.id`/`agent.id`/`user.id`/`tenant.id`, milestone codes like
`M1.5`, consumer-specific paths) returns zero matches on every
phase artifact set. The `research-solutions.md` files in Phases 7
and 8 reference external ecosystem consumers by brand as surveyed
facts, which is explicitly permitted by the decoupling contract
for research deliverables.

## Next Step

**Transition from planning to implementation.** Begin the v0.1.0
milestone — first working invocation with Anthropic provider, no
hooks, no filters, no budget — following
`docs/phase-6-release-governance/06-release-milestones.md`. The
`golang-pro` implementation subagent takes over from the design
subagents.

## Overall Status

- **v0.1.0 (first working invocation):** **UNBLOCKED.** All
  interface contracts and core-runtime design are frozen; the
  Anthropic provider adapter and zero-wiring smoke test can start
  immediately.
- **v0.5.0 (feature complete):** **UNBLOCKED.** All hooks, filters,
  budget, telemetry, credentials, identity, and observability
  contracts are frozen at v1.0 tiers. `praxis/mcp` and
  `praxis/skills` sub-modules can ship alongside the core module
  independently.
- **v1.0.0 (API freeze):** **UNBLOCKED on design.** Freeze
  commitment follows the first production consumer per Phase 1
  D04. Implementation must not drift from the Phase 3 frozen
  signatures.
