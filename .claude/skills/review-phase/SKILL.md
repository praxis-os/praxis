---
name: review-phase
description: >
  Critically review a praxis planning artifact, design document, or phase output to find
  flaws. Use after a phase has been planned or when any planning artifact needs hard
  review. Checks for logical gaps, weak assumptions, conflicting decisions, scope creep,
  decoupling contract violations, API stability risks, security blind spots, observability
  gaps. Produces a structured verdict: ready or not ready.
---

# Review Phase

Perform a hard, critical review of a praxis planning artifact or phase output.

## Input

Accept a phase output document, design artifact, or planning deliverable. Default input:
the latest phase directory under `docs/phase-<N>-<slug>/`.

## Review Focus

Evaluate against these dimensions:

1. **Logical gaps** — missing reasoning steps, unsupported conclusions
2. **Weak assumptions** — assumptions stated without validation or evidence from the Go
   ecosystem, prior art, or explicit trade-off analysis
3. **Conflicting decisions** — contradictions within or across phases, or with
   `docs/PRAXIS-SEED-CONTEXT.md`
4. **Scope creep** — inclusions that exceed the phase's stated goal or that fight the
   "generic first, opinionated where it matters" design principle
5. **Decoupling contract violations** — banned identifiers (`custos`, `reef`,
   `governance_event`, hardcoded `org.id`/`agent.id` as framework attributes, milestone
   codes like `M1.5`, decision IDs from other repositories), consumer-specific leakage,
   or framework logic that assumes a specific caller
6. **API stability risks** — interfaces that cannot be frozen at v1.0 without regret,
   missing deprecation windows, or signatures that couple to internal types
7. **Security blind spots** — credential handling, identity trust, untrusted tool output,
   key lifecycle, side-channel risks
8. **Observability gaps** — missing span attributes, metric cardinality risks, redaction
   holes, event emission gaps on failure paths
9. **Go idiom violations** — non-idiomatic interface shapes, missing context propagation,
   error wrapping anti-patterns, premature generics, unnecessary reflection
10. **Testability gaps** — designs that cannot be unit-tested, property-tested, or
    benchmarked against their stated targets

## Output Structure

Use this exact structure:

```
# Review: <phase or artifact name>

## Overall Assessment
2-3 sentences summarizing the quality and readiness of the artifact.

## Critical Issues
Numbered list. These block progress. Each item states the problem, why it matters, and
the specific file and location where it occurs.

## Important Weaknesses
Numbered list. These do not block but should be addressed. Each item states what is
weak and what would strengthen it.

## Open Questions
Numbered list. Questions that the artifact leaves unanswered.

## Decoupling Contract Check
Explicit PASS/FAIL on the banned-identifier check. If FAIL, list every violation with
file:line.

## Recommendations
Bullet list of specific actions to improve the artifact.

## Verdict: READY / NOT READY
One sentence justification.
```

## Output File

Write the review to a file named `REVIEW.md` inside the phase directory being reviewed.

- Phase 1 → `docs/phase-1-api-scope/REVIEW.md`
- Phase 2 → `docs/phase-2-core-runtime/REVIEW.md`
- Phase 3 → `docs/phase-3-interface-contracts/REVIEW.md`
- Phase 4 → `docs/phase-4-observability-errors/REVIEW.md`
- Phase 5 → `docs/phase-5-security-trust/REVIEW.md`
- Phase 6 → `docs/phase-6-release-governance/REVIEW.md`

If the file already exists, overwrite it with the new review.
After writing the file, print a one-line summary to the conversation: the verdict and
the file path.

## Review Style

- Be critical, not polite.
- Prefer identifying problems over rewriting the artifact.
- Separate critical blockers from lower-priority concerns.
- Point out what is missing, inconsistent, or unjustified.
- Treat the decoupling contract as a hard gate. A single banned-identifier leak is a
  CRITICAL issue, not a minor one.

## Guardrails

- Do not rewrite the whole artifact unless it is fundamentally broken.
- Do not approve weak plans — a NOT READY verdict is expected when warranted.
- Do not hide uncertainty behind diplomatic language.
- Do not add speculative complexity without clear justification.
- Do not approve a phase that ships an interface without explaining its default/null
  implementation, its composability, or its testability story.
