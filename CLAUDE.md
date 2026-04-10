# praxis — Enterprise Agent Orchestration for Go

## Project Context

`praxis` is a production-grade Go library for orchestrating LLM agents with
enterprise-grade guardrails built in (not bolted on): typed state machine,
multi-provider LLM, policy hooks, input/output filter chains, multi-dimensional
budget enforcement, typed error taxonomy, OpenTelemetry observability, and optional
agent identity signing.

**License:** Apache 2.0
**Module:** `github.com/praxis-go/praxis` (to be confirmed at first public commit)
**Minimum Go version:** 1.23+
**Target:** `v1.0.0` — stable public Go API that enterprise teams reach for when
"call an LLM in a loop" is not enough.

**Foundational context:** `docs/PRAXIS-SEED-CONTEXT.md` is the frozen baseline for
the entire project. It contains the vision, design principles, interface surface
summary, decoupling contract, and initial roadmap. All planning phases refine this
document — none override it. Read it first.

## Planning Harness

The project uses a lean planning harness inspired by the one used at the project's
origin, adapted for an OSS Go library context.

**Skills (3):** plan-phase, review-phase, roadmap-status (in `.claude/skills/`)
**Subagents (7):** api-designer, go-architect, security-architect,
observability-architect, dx-designer, reviewer, solution-researcher
(in `.claude/agents/`)

Plus `golang-pro` (pre-existing general-purpose Go implementation agent, used during
implementation phases after Phase 6 is approved).

### Working Loop (per phase)

1. Run `plan-phase`
2. Invoke 1–2 relevant subagents (api-designer, go-architect, security-architect,
   observability-architect, dx-designer, or solution-researcher)
3. Invoke `reviewer`
4. Run `review-phase`
5. Run `roadmap-status`

### Decoupling Contract (hard rule)

praxis is a generic library. It must never contain identifiers, assumptions, or
structure specific to any single consumer. Enforced in every phase by the `reviewer`
agent and the `review-phase` skill via a banned-identifier grep.

**Banned anywhere in code or phase artifacts (except explicit consumer-disclosure
sections):**

- `custos`, `reef`, or any other consumer brand name
- `governance_event` (use `lifecycle_event`)
- `org.id`, `agent.id`, `user.id`, `tenant.id` as **hardcoded** framework attributes.
  These attributes exist only as outputs of a caller-provided `AttributeEnricher`
- Milestone codes or decision IDs from other repositories (e.g., `M1.5`, `D47`, `G3`)
- Consumer-specific file paths (`apps/server/...`, `internal/policy/...`)

**Permitted in dedicated sections only:**

- `docs/PRAXIS-SEED-CONTEXT.md` §11 "Primary consumer disclosure" and §12 "Origin" —
  may reference the project's origin at Custos
- New equivalent sections in CHANGELOG or README if transparency about the first
  production consumer is needed

### Planning Phases

The design of `praxis v1.0` is broken into 8 planning phases:

1. **API Scope and Positioning** — what praxis is, positioning vs existing Go
   libraries, design principles, target consumers, what v1.0 commits to, what is
   explicitly non-goal.
2. **Core Runtime Design** — invocation state machine, lifecycle, streaming transport,
   cancellation semantics, context propagation, concurrency model.
3. **Interface Contracts** — all public v1.0 interfaces (AgentOrchestrator, LLMProvider,
   tools.Invoker, hooks.PolicyHook, hooks.PreLLMFilter, hooks.PostToolFilter,
   budget.Guard, budget.PriceProvider, errors.TypedError, errors.Classifier,
   telemetry.LifecycleEventEmitter, telemetry.AttributeEnricher, credentials.Resolver,
   identity.Signer), their method surfaces, default/null implementations, composability
   rules.
4. **Observability and Error Model** — OTel span structure, Prometheus metrics, slog
   redaction, typed error taxonomy, error-to-event mapping, AttributeEnricher contract,
   cardinality constraints.
5. **Security and Trust Boundaries** — credential fetch semantics, zero-on-close,
   identity signing (Ed25519 JWT), untrusted tool output handling, filter hook trust
   model, key lifecycle.
6. **Release, Versioning and Community Governance** — semver policy, v0 to v1.0
   stability commitment, deprecation windows, v2+ module path rules, release process
   (conventional commits + release-please), CI pipeline (lint, test, coverage,
   benchmarks, banned-identifier grep, govulncheck, codeql), contribution model,
   code of conduct, RFC process.
7. **MCP Integration** — whether and how praxis supports the Model Context
   Protocol at v1.0.0 (in-tree adapter vs. pattern-only), integration model via
   `tools.Invoker`, credential flow, transport priority, observability extensions,
   trust-boundary classification, compatibility with the "no plugins in v1"
   commitment (D09). Added after Phase 6 was approved; blocks v1.0.0 freeze.
8. **Skills Integration** — whether and how praxis supports the provider-side
   "skills" concept at v1.0.0 (first-class type vs. convention vs. non-goal),
   relationship to `LLMProvider` and `tools.Invoker`, budget participation,
   observability additions, DX and terminology disambiguation. Depends on
   Phase 7; blocks v1.0.0 freeze.

Implementation begins only after **all 8 planning phases are approved**. Release targets:

- **v0.1.0** — first working invocation with Anthropic provider, no hooks, no filters,
  no budget. First consumable tag.
- **v0.3.0** — all v1.0 interfaces stable, hooks + filters + budget + telemetry
  functional, OpenAI adapter shipped.
- **v0.5.0** — feature complete, ≥85% coverage, benchmarks green, ready for first
  production consumer.
- **v1.0.0** — API freeze committed after the first production consumer ships.
  Breaking changes after this point require a `v2` module path.

## Conventions

### File Naming

All phase output files use a two-digit numbered prefix for reading order:
`NN-filename.md` (e.g., `01-decisions-log.md`, `02-state-machine.md`).
`REVIEW.md` is the only unnumbered file — always read last.

### Phase Directories

`docs/phase-<N>-<slug>/` — for example:

- `docs/phase-1-api-scope/`
- `docs/phase-2-core-runtime/`
- `docs/phase-3-interface-contracts/`
- `docs/phase-4-observability-errors/`
- `docs/phase-5-security-trust/`
- `docs/phase-6-release-governance/`
- `docs/phase-7-mcp-integration/`
- `docs/phase-8-skills-integration/`

### Phase States

`not-started` | `in-progress` | `under-review` | `blocked` | `approved`

### Decision IDs

Decisions use a `DNN` identifier scheme, unique across phases, allocated sequentially
as they are made (D01, D02, D03, ...). Each phase's `01-decisions-log.md` owns a
contiguous range. The `roadmap-status` skill tracks the current allocation.

### Go Development

Once implementation begins, follow the checklist in `.claude/agents/golang-pro.md`:
idiomatic Go, `gofmt` + `golangci-lint` compliance, context propagation everywhere,
typed errors with wrapping, table-driven tests, benchmarks on hot paths, race-free
code, godoc on every exported symbol.

### Current Status

Phases 1–6 are `approved` with a clean decoupling contract and 103 adopted
decisions (D01–D105). All 14 originally planned public interfaces are at
`frozen-v1.0`.

Two additional phases have been scaffolded and block the `v1.0.0` freeze:

- **Phase 7 — MCP Integration** (`not-started`) — decides whether and how
  praxis supports the Model Context Protocol at v1.0.0. Decision range
  reserved from `D106`.
- **Phase 8 — Skills Integration** (`not-started`) — decides whether and how
  praxis supports the provider-side "skills" concept at v1.0.0. Decision
  range starts immediately after Phase 7 closes.

Next step: run `plan-phase` on Phase 7 "MCP Integration". Phase 8 activation
is gated on Phase 7 approval.
