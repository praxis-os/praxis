# Phase 8: Skills Integration

> **Status:** scaffolded — awaiting `plan-phase` skill run.
> This file is a scoping stub. It was created before the phase was formally
> activated to record the initial intent, preliminary scope, and open
> questions that Phase 8 must resolve. When `plan-phase` is invoked on this
> phase, this file will be overwritten with the full planning artifact
> following the same structure as Phases 1–6.
>
> **Dependency:** Phase 8 depends on Phase 7 (MCP Integration). Phase 8
> can only transition to `under-review` after Phase 7 is `approved`,
> because several Phase 8 questions depend on the tool-namespacing,
> credential-flow, and error-taxonomy decisions made in Phase 7.

## Goal

Decide whether and how praxis supports the concept of **"skills"** (in the
sense used by Anthropic and similar provider-side tool/prompt bundles)
at v1.0.0. Specify whether skills are an explicit typed primitive in the
library, a convention over the existing `LLMProvider` → `ToolCall` →
`Invoker` flow, or an explicit non-goal. Clarify the impact on the frozen
v1.0 interface surface, on observability, and on the developer experience.

## Motivation

Neither the seed context nor Phases 1–6 mention a "skill" concept as a
praxis library feature. (The term `skill` in the repository refers
exclusively to the Claude Code planning harness at `.claude/skills/`,
which is unrelated.) Provider-side skills are now a visible part of the
LLM ecosystem, and consumers will ask how to use them with praxis.
Before freezing v1.0.0, the project needs an explicit answer: either
skills are a first-class concept (with type support, observability, and
documentation), a convention on top of existing primitives, or an
explicit non-goal.

## Preliminary Scope

**In scope (to be refined by `plan-phase`):**

- Positioning: is "skill" a typed praxis concept in v1.0, a documented
  convention, or a non-goal?
- Definition: what exactly is a "skill" from praxis's point of view — a
  bundle of tool declarations, a prompt template, both, or something
  else?
- Relationship to `LLMProvider`: how a provider signals which skills are
  active, how skill metadata is passed into the request.
- Relationship to `tools.Invoker`: whether skill-originated tool calls
  flow through the same `Invoker` seam or require a new layer.
- Built-in vs. custom skills: how the library distinguishes between
  provider-managed skills (server-side) and consumer-authored skills
  (client-side).
- Budget participation: how a skill that consumes multiple tools in a
  single turn is accounted against `Budget` without double-counting.
- Observability: whether skill activation produces new events, span
  attributes, or metric labels. Must respect Phase 4 D60 cardinality
  boundary.
- DX: how a consumer declares, configures, and enables a skill in their
  praxis setup. Error messages, examples, guide structure.
- Interaction with Phase 7 decisions: when a skill is implemented as an
  MCP server, does the Phase 7 adapter cover it automatically, or is a
  specialised path needed?
- Impact on the frozen v1.0 interface surface: whether a new interface
  or type is needed, and if so, whether it ships at `frozen-v1.0` or as
  a `stable-v0.x-candidate` per D13's three-tier policy.

**Out of scope:**

- Implementing any specific provider's skill format — that is a provider
  adapter concern, not a library concern.
- A general prompt-template system — prompts are the caller's
  responsibility (seed principle).
- A skill registry or marketplace — would violate D09 / Non-goal 7.
- Runtime skill discovery or dynamic loading — would violate D09.
- Implementation code (post-Phase 6).

## Key Questions

1. Is a "skill" a praxis type, or just a convention over existing
   primitives (`LLMProvider` + `[]ToolCall` + `Invoker`)?
2. If a type, which package does it live in (`llm`, `skills`, other)?
3. What metadata does a skill need to carry that cannot be expressed
   today (name, version, tool list, prompt fragment, resource references)?
4. How does `LLMProvider` signal which skills are enabled in a request —
   a new field on the request struct (requiring a Phase 3 amendment), a
   metadata map, or nothing at all?
5. How are skill-originated tool calls distinguished from regular tool
   calls in the event stream (if at all)?
6. How does a consumer declare a skill at construction time? Builder
   option, provider-specific config, or `Invoker` composition?
7. Does `InvocationEvent` need new event types for skill lifecycle (e.g.,
   `SkillActivated`, `SkillCompleted`), or is the existing vocabulary
   sufficient?
8. Does an active skill change budget accounting rules (e.g., per-skill
   token or wall-clock sub-budgets)?
9. When a skill is backed by an MCP server (Phase 7), is the caller
   integration identical, or does Phase 8 define a higher-level shortcut?
10. Is the built-in / custom distinction visible in the public API, or
    entirely opaque to praxis?
11. Does this phase require any amendment to the Phase 1 non-goals list
    or the Phase 3 interface contracts?
12. Does the "skill" terminology itself risk colliding with Claude Code's
    `.claude/skills/` planning harness in user documentation?

## Decisions Required

Decision IDs will be allocated contiguously starting **immediately after
Phase 7's last decision ID** when the phase is activated by `plan-phase`.
The list below is indicative of the decisions expected to come out of
this phase; the actual allocation is recorded in `01-decisions-log.md`.

- Positioning of skills in v1.0 (first-class type, convention, non-goal).
- Definition and metadata shape (if first-class).
- Relationship to `LLMProvider` request surface.
- Relationship to `tools.Invoker` dispatch.
- Built-in vs. custom skill distinction (visible or opaque).
- Budget accounting model for multi-tool skill turns.
- Observability additions (events, attributes, metrics).
- Phase 7 ↔ Phase 8 interaction model (MCP-backed skills).
- DX: declaration surface, error messages, examples.
- Impact on frozen v1.0 interface surface and stability tier placement.

## Assumptions

- **The `tools.Invoker` seam remains canonical.** Whatever Phase 8
  decides, tool dispatch continues to flow through `Invoker`. A skill
  does not bypass the invoker.
- **D09 is not re-opened.** No dynamic skill registration, no plugin
  loading. Any skill support must be build-time composition.
- **Phase 3 interface additions, not changes.** If Phase 8 needs a new
  interface or type, it adds it alongside the existing surface; it does
  not break frozen signatures.
- **Phase 4 cardinality rules apply.** Any new metric labels for skill
  names or types must respect the hard cardinality boundary (D60).
- **Phase 7 delivered the MCP integration model before Phase 8 starts.**
  Phase 8 can reference Phase 7 decisions; Phase 7 cannot reference
  Phase 8 decisions.
- **Terminology disambiguation is handled in documentation.** The word
  "skill" in the praxis library context refers to the provider-side
  concept, not to the Claude Code planning harness.

## Risks

- **R1 — Conceptual overreach.** Defining "skill" too broadly produces a
  generic abstraction that duplicates `tools.Invoker` without adding
  value. Must be bounded by a real consumer need.
- **R2 — Frozen-interface pressure.** A first-class `Skill` type may
  require amending Phase 3 frozen interfaces. The three-tier stability
  policy (D13) must be respected; any frozen-interface amendment needs
  explicit justification per the amendment protocol.
- **R3 — Double accounting in budget.** If a skill turn fires N tool
  calls, naive accounting may double-count tokens or credits.
- **R4 — Cardinality from skill-name labels.** Consumer-authored skill
  names are unbounded; they cannot become metric labels without a
  mitigation.
- **R5 — Terminology collision.** The word "skill" already means
  something specific in Claude Code (`.claude/skills/`). Poor phrasing
  in docs and error messages could confuse new users.
- **R6 — Provider-specific leakage.** Any provider's skill format
  leaking into the praxis public API would violate the decoupling
  contract (seed §6.1).

## Deliverables

Once the phase is activated, the expected deliverables are:

- `00-plan.md` — this file, rewritten as the full plan.
- `01-decisions-log.md` — the Phase 8 decision range.
- `02-scope-and-positioning.md` — first-class vs. convention vs.
  non-goal decision with rationale.
- `03-integration-model.md` — how skills map onto `LLMProvider` +
  `tools.Invoker`, including budget and observability flows.
- `04-non-goals.md` — things Phase 8 explicitly declines to support.
- `REVIEW.md` — reviewer + `review-phase` verdict.

## Recommended Subagents

1. **solution-researcher** — survey how "skills" are exposed by major
   LLM providers today, document the differences between provider-side
   and client-side models, identify prior art in Go orchestration
   libraries.
2. **api-designer** — decide whether a new typed primitive is necessary
   and, if so, where it belongs in the package layout; evaluate impact
   on frozen interfaces and stability tier placement.
3. **dx-designer** — consumer declaration surface, error messages,
   example code, documentation story, terminology disambiguation from
   Claude Code harness skills.
4. **observability-architect** — event additions, span attributes,
   metric labels, cardinality analysis for skill-name labels.
5. **reviewer** (fixed) — phase closure per standard loop.

## Exit Criteria

1. A clear positioning decision for skills in v1.0 (first-class /
   convention / non-goal).
2. If first-class: the typed primitive is fully specified, its package
   location is decided, and its stability tier is assigned per D13.
3. If convention-only: a reference integration guide exists with a
   complete worked example of how a consumer declares a skill on top
   of `LLMProvider` + `Invoker`.
4. All decisions in `01-decisions-log.md` with rationale.
5. Explicit statement of how Phase 8 interacts with Phase 7 (especially
   for MCP-backed skills).
6. Explicit confirmation that D09 is not re-opened.
7. Banned-identifier grep clean on all Phase 8 artifacts.
8. Reviewer subagent PASS.
9. `REVIEW.md` verdict: READY.
10. The v1.0.0 freeze is unblocked on the MCP/skills axis, i.e. both
    Phase 7 and Phase 8 are `approved`.
