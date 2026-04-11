---
name: plan-phase
description: >
  Turn a praxis design phase into a structured planning artifact. Use when starting or
  refining any of the 8 planning phases (API Scope and Positioning, Core Runtime,
  Interface Contracts, Observability and Errors, Security and Trust, Release and
  Community Governance, MCP Integration, Skills Integration). Produces objectives,
  key questions, decisions, assumptions, risks, deliverables, recommended subagents,
  and exit criteria.
---

# Plan Phase

Produce a concrete planning artifact for a single design phase of the `praxis` Go library.

## Input

Accept a phase name or topic. If ambiguous, ask for clarification before proceeding.

## Output Structure

Use this exact structure:

```
# Phase: <name>

## Goal
One sentence stating the phase objective — what the phase must produce to unblock v1.0.

## Scope
Bullet list of what is in scope and what is explicitly out of scope. Reference which
parts of `docs/PRAXIS-SEED-CONTEXT.md` this phase refines.

## Key Questions
Numbered list of open questions that must be answered to close this phase. Phrase them
as questions that have concrete answers, not wishlists.

## Decisions Required
Numbered list of decisions with brief context for each. Use `DNN` identifiers (D01, D02, ...)
that stay unique across phases.

## Assumptions
Bullet list of assumptions being made. Flag any that are weak or unvalidated.

## Risks
Bullet list. Separate critical risks (block v1.0 or break the decoupling contract) from
secondary risks.

## Deliverables
Bullet list of expected output files. All files use the `NN-filename.md` convention.
`REVIEW.md` is the only unnumbered file.

## Recommended Subagents
List 1-2 subagents from: api-designer, go-architect, security-architect,
observability-architect, dx-designer, solution-researcher.
Justify each recommendation in one sentence.
The reviewer subagent is always invoked — do not list it here.

## Exit Criteria
Numbered list of conditions that must be true for the phase to be considered complete.
Include: all decisions locked, reviewer PASS, REVIEW.md verdict READY, no banned-identifier
leakage in phase artifacts.
```

## Deliverable Naming Convention

All phase output files must be numbered with a two-digit prefix to enforce reading order.
Format: `NN-filename.md` (e.g., `01-decisions-log.md`, `02-state-machine.md`).

The `Deliverables` section of the planning artifact must list files in numbered order.
`REVIEW.md` is the only file without a number prefix — it is always the last file in the
phase directory.

## Guardrails

- Do not generate implementation code. This is a design harness.
- Do not invent technical detail unless necessary to frame a question or risk.
- Do not expand scope beyond what the phase requires. Refer ambiguous scope back to
  `docs/PRAXIS-SEED-CONTEXT.md` — it is the source of truth.
- Do not propose more than 2 specialist subagents unless truly justified.
- Identify ambiguity explicitly rather than papering over it.
- Respect the decoupling contract: no `custos`, `reef`, `governance_event`, hardcoded
  `org.id`/`agent.id` leakage. Phase artifacts must pass the banned-identifier grep.
- Prefer clarity over completeness theater. OSS libraries fail when over-designed.
- Focus on planning, not implementation.

## Phase Reference

1. **API Scope and Positioning** — what `praxis` is, positioning vs existing Go libraries,
   design principles, target consumers, what v1.0 commits to, what is explicitly non-goal.
2. **Core Runtime Design** — invocation state machine, lifecycle, streaming transport,
   cancellation semantics, context propagation, concurrency model.
3. **Interface Contracts** — all public v1.0 interfaces (AgentOrchestrator, LLMProvider,
   tools.Invoker, hooks.PolicyHook, hooks.PreLLMFilter, hooks.PostToolFilter, budget.Guard,
   budget.PriceProvider, errors.TypedError, errors.Classifier, telemetry.LifecycleEventEmitter,
   telemetry.AttributeEnricher, credentials.Resolver, identity.Signer), their method
   surfaces, default/null implementations, composability rules.
4. **Observability and Error Model** — OTel span structure, Prometheus metrics, slog
   redaction, typed error taxonomy, error-to-event mapping, `AttributeEnricher` contract,
   cardinality constraints.
5. **Security and Trust Boundaries** — credential fetch semantics, zero-on-close, identity
   signing (Ed25519 JWT), untrusted tool output handling, filter hook trust model, key
   lifecycle.
6. **Release, Versioning and Community Governance** — semver policy, v0 to v1.0 stability
   commitment, deprecation windows, v2+ module path rules, release process (conventional
   commits + release-please), CI pipeline (lint, test, coverage, benchmarks, banned-identifier
   grep, govulncheck, codeql), contribution model, code of conduct, RFC process.

## Reference inputs

Every plan-phase invocation should read:

- `docs/PRAXIS-SEED-CONTEXT.md` — the foundational context document. It contains the
  vision, design principles, interface surface summary, decoupling contract, and initial
  roadmap. Phases refine this document, they do not override it.
- Any previously approved phase in `docs/phase-<N>-<slug>/` — earlier decisions flow
  forward and must not be contradicted without an explicit amendment.
