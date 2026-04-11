# Phase 8: Skills Integration

> **Status:** `approved` — activated 2026-04-10, approved 2026-04-10
> after three reviewer passes (BLOCK → FAIL → PASS with non-blocking
> observations, both addressed in-place) and `review-phase` verdict
> **READY**. Decision range **D122–D134** (13 decisions,
> contiguous, all decided). Phase 7 `approved`; Phase 8 `approved`;
> the v1.0.0 freeze is now unblocked on the MCP / skills axis.
> See `docs/roadmap-status.md` for the phase table.

## Goal

Enable agents orchestrated by `praxis` to **load and use installable
skill bundles** — i.e., directories shaped like the `SKILL.md`
convention used by Claude Code, Codex, and distributed via registries
such as skills.sh — without violating the "no plugins in v1"
commitment (Phase 1 D09, Non-goal 7), without disturbing the Phase 3
`frozen-v1.0` interface surface, and without smuggling provider-specific
shapes into the public API.

## Motivation

Skill bundles are now a de facto unit of reusable agent capability
across the LLM tooling ecosystem. A "skill" in this sense is a
directory containing at minimum a `SKILL.md` descriptor (name,
description, optional metadata, instructions the agent should follow
when the skill is active) and optionally supporting assets: tool
declarations, local scripts, reference resources, or pointers to MCP
servers. Agents built on top of Claude Code, Codex, and similar tools
already install and consume these bundles routinely.

A praxis consumer who wants their orchestrated agent to benefit from
an existing skill bundle has today no library-supported path:
they must hand-wire the skill's instructions into their system prompt,
register its tools through `tools.Invoker` manually, and track
lifecycle themselves. Before the v1.0.0 freeze, `praxis` needs an
explicit answer: either the library provides a supported loader + a
typed composition surface for skill bundles, or it commits to
"convention only" with a single binding reference pattern and a worked
example, or it declares the whole thing a non-goal and documents the
manual path. Leaving this unresolved at freeze would push every
consumer that wants skill-bundle support to invent their own wiring
and would risk either a breaking change in v1.x or a forced v2.

## Scope

Refines `docs/PRAXIS-SEED-CONTEXT.md` §3 *Design principles*, §4.3
*Tools and filters*, §5 *Interface surface*, §6 *Decoupling contract*,
and §13 *Open questions*. Depends on Phase 7 (`approved`) for the
MCP-backed skill path.

**In scope:**

- **Positioning** in v1.0: first-class loader + typed bundle value, a
  documented convention with a single reference pattern, or an
  explicit non-goal.
- **Bundle format recognised by praxis**: which `SKILL.md` shape is
  considered canonical (YAML frontmatter fields: `name`,
  `description`, `version`, optional tool declarations, optional MCP
  server pointers, optional resource paths), and which fields are
  required vs. optional vs. ignored. The format must be a
  provider-neutral intersection of what skills.sh-style bundles
  actually carry today — not a new format invented by praxis.
- **Loader surface**: if first-class, the caller-facing entry point
  for loading a bundle from a filesystem path the caller provides
  (e.g., `skills.Load(path)` or `skills.Open(fs, path)`), what it
  returns, and how it fails.
- **Composition with `AgentOrchestrator`**: how a loaded bundle is
  wired into an invocation — as an additive system-prompt fragment,
  as a set of tool registrations on `tools.Invoker`, or both. The
  composition must be build-time, explicit, and caller-initiated.
- **Tool namespacing inside a bundle**: the convention by which a
  skill's tool names are exposed to the LLM and routed by
  `tools.Invoker`, and how that interacts with Phase 7 D111 when a
  bundle's tools come from an MCP server.
- **Instruction injection into the LLM request**: how a skill's
  `SKILL.md` instructions reach the LLM — new field on the request,
  provider-opaque metadata, or an additive system-prompt fragment
  produced at composition time — and whether this requires any
  change to the Phase 3 `frozen-v1.0` `LLMProvider` surface.
- **Multi-skill composition**: rules when a caller loads and enables
  two or more bundles in the same invocation. Name-collision
  handling, instruction ordering, and tool-name resolution must
  all be defined.
- **Budget participation**: skill-originated tool calls participate
  in the existing `Budget` via Phase 7 D112 verbatim. No new budget
  dimension, no double-counting, no per-skill sub-budgets.
- **Observability**: whether skill activation produces new events,
  span attributes, or metric labels, bounded by the Phase 4 D60
  cardinality boundary (skill names are caller-chosen strings and
  are therefore unbounded by default).
- **Interaction with Phase 7 (MCP)**: a skill bundle that declares
  one or more MCP servers as dependencies must flow through the
  Phase 7 `praxis/mcp` adapter. Phase 8 defines the handoff, not a
  second MCP path.
- **DX**: consumer declaration surface, error messages for
  malformed bundles / missing files / invalid frontmatter,
  worked example wiring a bundle into an orchestrator, and the
  documentation story.
- **Impact on the Phase 3 frozen interface surface**: if any new
  type ships, its stability tier under D13's three-tier policy.

**Out of scope:**

- **Registry / marketplace / discovery mechanics.** praxis does not
  talk to skills.sh or any other registry. Downloading, caching,
  verifying, or updating bundles is the caller's responsibility.
  praxis only loads from a filesystem path the caller already
  provides. *(This is the hard wall against R7 and the explicit
  re-confirmation of D09.)*
- **Runtime skill loading or hot reloading.** Bundles are loaded
  once, at caller-controlled construction time. No dynamic
  registration, no file-watcher reloading.
- **Authoring tooling for skill bundles.** praxis consumes bundles;
  it does not provide tools to create or edit them.
- **Sandboxing the execution of skill-declared scripts.** If a
  bundle contains local scripts, invoking them (or not) is the
  caller's choice via the existing `tools.Invoker` seam. praxis
  does not provide a sandbox.
- **A new prompt-template system.** Instruction injection reuses the
  existing system-prompt composition path. praxis still does not
  own the prompt (seed §3).
- Implementing any specific provider's skill format variant that
  sits *outside* the SKILL.md intersection. Variant-specific fields
  are ignored by the loader with a documented policy.
- Implementation code (design harness; post-Phase 6).
- Changing the frozen semantics of `tools.Invoker`, `ToolCall`,
  `ToolResult`, `LLMProvider`, `AgentOrchestrator`, or any other
  Phase 3 `frozen-v1.0` interface. Phase 8 may add alongside;
  it may not rewrite.

## Key Questions

1. Does a convergent `SKILL.md` format exist across the ecosystem
   today (Anthropic, Claude Code, Codex, skills.sh) that praxis can
   anchor on, or is there enough fragmentation that praxis must pick
   a documented intersection and explicitly declare which fields it
   ignores?
2. Is the loader a first-class typed primitive
   (`skills.Load(path) (Skill, error)` returning a typed value that
   is then passed to the orchestrator), a documented convention
   (example code the caller copies), or a non-goal (document the
   manual wiring path)?
3. If first-class, which package owns it — a new `skills` package,
   or a sub-package of an existing one (`llm/skills`, `tools/skills`,
   `praxis/skills`)? The package location implicitly signals where
   the concept belongs in the mental model.
4. How does a loaded `Skill` compose into an invocation — a
   dedicated `WithSkill(s)` option on the orchestrator, a slice
   passed at `Invoke` time, or by the caller manually pulling
   `s.Tools()` and `s.SystemPrompt()` into their own wiring?
5. Does skill-instruction injection require a new field on the
   frozen `LLMProvider` request type, or can it be composed entirely
   at the caller / orchestrator layer via the existing system-prompt
   path without touching frozen signatures?
6. What is the tool-namespacing convention inside a skill, and how
   does it compose with Phase 7 D111 (`{LogicalName}__{mcpToolName}`)
   when the skill's tools are sourced from an MCP server? Is the
   final name `{skillName}__{toolName}`, `{skillName}__{logical}__{mcp}`,
   or flattened?
7. How are multi-skill conflicts resolved? If two bundles declare a
   tool with the same name, does construction fail, does the later
   one win, or is the caller required to namespace them?
8. Does `InvocationEvent` need new event types for skill lifecycle
   (`SkillActivated`, `SkillCompleted`), or does the existing
   tool-call vocabulary carry enough signal? If new events, are
   they emitted per skill per turn or once per invocation?
9. Does an active skill change budget accounting rules? The default
   should be "no — skills participate via D112 verbatim". Is there
   any concrete case that breaks this default?
10. Which skill-level labels are cardinality-safe as metric labels
    under Phase 4 D60? Caller-chosen skill names are unbounded;
    mitigation options include bounded allow-list, hashing, or
    bucketing (mirror of Phase 7 D115).
11. When a skill bundle declares an MCP server as a dependency,
    does the Phase 8 loader wire it through the Phase 7 adapter
    automatically, or does the caller compose `praxis/mcp` and
    `praxis/skills` themselves at construction time? The former is
    more ergonomic; the latter keeps the package boundary clean.
12. What happens when a bundle references local scripts or
    resources (relative filesystem paths)? Does the loader resolve
    them, or does it surface them as raw paths for the caller's
    `tools.Invoker` to handle?
13. Which `SKILL.md` frontmatter fields are required, which are
    optional, and which are ignored with a warning? What does the
    loader do when it encounters an unknown field?
14. Does this phase require an amendment to any Phase 1 non-goal or
    to any Phase 3 frozen-interface signature, and if so, is the
    amendment protocol followed?

## Decisions Required

Decision IDs allocated contiguously from **D122**. Indicative shape,
final count set when the phase reaches `under-review`.

1. **D122** — Positioning of skill-bundle support in v1.0:
   first-class loader + typed value, documented convention, or
   explicit non-goal. Load-bearing; every downstream decision is
   conditional.
2. **D123** — Canonical `SKILL.md` shape recognised by praxis:
   required frontmatter fields, optional fields, ignored fields,
   unknown-field policy. Must be a defensible intersection of the
   ecosystem as of 2026-Q2, not a praxis-invented format.
3. **D124** — Loader surface: constructor signature, what it
   returns, how it fails (typed errors via the Phase 4 taxonomy),
   whether it reads from `os` or from an `fs.FS`.
4. **D125** — Composition surface: how a loaded `Skill` is wired
   into the orchestrator (builder option, per-invocation slice, or
   caller-driven pull). This decision must demonstrate no change to
   the Phase 3 `frozen-v1.0` `AgentOrchestrator` / `LLMProvider`
   signatures, or justify any amendment under the Phase 1 protocol.
5. **D126** — Tool namespacing convention inside a skill, and
   explicit composition rule with Phase 7 D111 for MCP-sourced
   tools inside a skill.
6. **D127** — Multi-skill conflict resolution: collision policy,
   instruction ordering, deterministic name resolution.
7. **D128** — Instruction injection path: how a skill's `SKILL.md`
   instructions reach the LLM. Explicit confirmation that this path
   does not modify the frozen request surface (or the amendment
   protocol is invoked).
8. **D129** — Budget participation: explicit re-use of Phase 7 D112
   (`tool_calls` + `wall_clock` via `tools.Invoker`) and an
   explicit "no double counting, no per-skill sub-budgets" rule.
9. **D130** — Observability additions: event types (if any), span
   attributes, metric labels, cardinality mitigation for
   unbounded skill names (mirror of Phase 7 D115).
10. **D131** — Phase 7 ↔ Phase 8 interaction for MCP-backed skill
    dependencies: whether the loader transparently wires
    `praxis/mcp`, or the caller composes both packages.
11. **D132** — DX and error-message catalogue for malformed /
    missing / invalid bundles; the single worked example that
    ships in the guide.
12. **D133** — Non-goals catalogue (binding for v1.0.0), including
    the explicit D09 re-confirmation: no registry talk, no
    downloading, no runtime discovery, no hot reloading, no
    sandboxing.
13. **D134** — Impact on frozen v1.0 interface surface and
    stability tier placement of any new type, per D13.

The phase may contract below this list if positioning is "non-goal"
— in that case D123–D132 collapse into the non-goals catalogue plus
D131 (Phase 7 interaction note).

## Assumptions

- **The `tools.Invoker` seam remains canonical.** Every tool
  exposed by a skill bundle flows through `tools.Invoker`. A skill
  cannot bypass the invoker. (Seed §4.3; Phase 3 `frozen-v1.0`.)
- **D09 is not re-opened.** Loading a bundle from a caller-provided
  filesystem path at construction time is **build-time composition**
  — the caller explicitly chooses which bundles to enable before
  the orchestrator starts. Runtime discovery, hot reloading,
  registry talk, and download mechanics remain banned.
- **Phase 3 interface additions, not changes.** Any new type ships
  alongside the existing surface. Any pressure to amend a frozen
  signature (for example, a new field on the `LLMProvider` request
  type to carry skill metadata) must be justified via the Phase 1
  amendment protocol — D128 and D134 own this decision.
- **Phase 4 cardinality rules apply.** Skill names are
  caller-chosen strings and are unbounded by default; they cannot
  become raw metric labels without a D60-compliant mitigation.
- **Phase 5 trust-boundary classification applies.** If a skill
  depends on an MCP server, the Phase 5 trust-boundary model
  applies verbatim via Phase 7 D116. Tool output returned via a
  skill-declared MCP server is untrusted by the same rules as
  direct MCP calls.
- **Phase 7 outputs are stable inputs.** Phase 8 references Phase 7
  D106 / D109 / D111 / D112 / D113 / D115 / D116 / D117 / D120.
  Phase 7 cannot reference Phase 8.
- **No convergent "skill spec RFC" has been ratified across the
  ecosystem as of 2026-04-10.** This is a weak assumption that
  solution-researcher must verify. If a de jure spec does exist,
  D123 anchors to it verbatim. If not, D123 anchors to the
  intersection of observable bundles in skills.sh and the
  Anthropic / Claude Code / Codex documented shapes.
- **The Claude Code `.claude/skills/` planning harness and the
  praxis "skill" concept are the same concept, not a collision.**
  The original terminology-disambiguation risk dissolves: bundles
  are bundles. Documentation should cross-reference rather than
  disambiguate.
- **Skill bundles typically carry instructions, tool declarations,
  and optional MCP pointers.** This is a working assumption from
  the existing ecosystem; solution-researcher must confirm the
  real shape of `SKILL.md` in the wild and report the minimum
  field set that is genuinely portable across tools.

## Risks

**Critical risks** (block v1.0 or violate the decoupling contract):

- **R1 — Format fragmentation.** If the `SKILL.md` shape varies
  meaningfully across ecosystem tools, any intersection praxis
  picks will silently reject fields that matter for some consumers
  and will force caller-side translation. Mitigation:
  solution-researcher produces a sourced matrix of the real
  frontmatter shapes observed in the wild, and D123 anchors to the
  intersection with an explicit "ignored fields" list.
- **R2 — Frozen-interface pressure.** Instruction injection may
  push toward a new field on the frozen `LLMProvider` request
  type. The D13 three-tier stability policy applies, and any
  frozen-interface amendment must be justified under the Phase 1
  amendment protocol. Mitigation: prefer composition at the
  orchestrator layer (system-prompt fragment produced at
  build-time, no request-type change).
- **R6 — Provider-specific leakage.** Any field in a particular
  provider's skill format that only makes sense for that provider
  must stay *outside* the praxis public API. Mitigation:
  api-designer rejects any shape that leaks provider-specific
  semantics; D123 commits to the intersection only.
- **R7 — D09 re-opening pressure.** A successful skill loader will
  attract requests for registry support, hot reloading, and
  runtime discovery. Mitigation: D133 catalogues these as binding
  non-goals up-front (mirror of Phase 7 D120).
- **R9 — Path resolution and supply-chain exposure.** A skill
  bundle may reference local scripts or resources by relative
  path. Naively resolving these, or silently fetching remote
  references, would re-open D09 and create a supply-chain risk.
  Mitigation: D124 specifies that the loader treats the bundle
  directory as a closed unit (no path escape, no network fetches,
  no implicit execution). Script execution remains the caller's
  choice via `tools.Invoker`.

**Secondary risks** (quality / DX, do not block v1.0):

- **R3 — Double accounting in budget.** If a "skill activation"
  event is emitted separately from the underlying tool calls,
  naive accounting could double-count. Mitigation: D129 explicitly
  forbids double-counting and re-uses D112 verbatim.
- **R4 — Cardinality from skill-name labels.** Mirror of Phase 7
  R5; D130 applies the D60 mitigation.
- **R5 — Conflict resolution surprise.** If two skills declare a
  tool with the same name, the orchestrator must fail loudly, not
  silently shadow. D127 locks the default policy as "construction
  error" unless the caller opts into a namespacing strategy.
- **R8 — Instruction ordering drift.** When multiple bundles
  contribute instruction fragments to the system prompt, the
  resulting prompt must be deterministic and caller-observable.
  Non-determinism here produces bug reports that are painful to
  reproduce.

## Deliverables

Numbered order defines reading order. `REVIEW.md` is unnumbered.

- `00-plan.md` — this file (Phase 8 plan + working-loop tracker).
- `01-decisions-log.md` — Phase 8 decision range (from **D122**),
  adopted decisions with rationale, alternatives considered,
  cross-references to Phase 1/3/4/5/7 dependencies.
- `02-scope-and-positioning.md` — the positioning call (D122) with
  rationale grounded in the solution-researcher output; the
  canonical `SKILL.md` shape (D123) with the intersection matrix;
  the explicit "what praxis recognises and what it ignores" table.
- `03-integration-model.md` — loader surface, composition surface,
  instruction-injection path, tool namespacing, multi-skill
  conflict resolution, budget flow, observability flow, Phase 7
  MCP interaction for skill-declared MCP servers. Contains the
  single worked example ("load this bundle, pass it to the
  orchestrator, invoke"). Conditional on D122 positioning.
- `04-dx-and-errors.md` — consumer declaration surface, typed
  error catalogue for malformed / missing / invalid bundles,
  worked example wiring, documentation story, cross-reference
  (not disambiguation) with the Claude Code `.claude/skills/`
  planning harness. Always produced.
- `05-non-goals.md` — binding non-goals catalogue for `praxis v1.0.0`:
  no registry talk, no downloading, no runtime discovery, no hot
  reloading, no authoring tooling, no sandboxing, explicit D09
  re-confirmation. Mirrors Phase 7 D120. Always produced.
- `research-solutions.md` — solution-researcher output: sourced
  survey of `SKILL.md` bundles in the wild (skills.sh, Anthropic
  docs, Claude Code repo layout, Codex repo layout, notable
  community bundles), intersection matrix of frontmatter fields,
  prior art in Go / Python / TS orchestration libraries for
  bundle loading, evidence for / against a first-class type.
- `REVIEW.md` — reviewer subagent + `review-phase` verdict.

## Recommended Subagents

1. **solution-researcher** — the load-bearing question is whether a
   convergent `SKILL.md` shape exists across the ecosystem as of
   2026-Q2. Without a sourced survey the D122 positioning call and
   the D123 format decision are vibes-based and would fail reviewer
   challenge on R1 and R6. The researcher must produce a field-level
   matrix from real bundles (skills.sh entries, Anthropic skills
   documentation, Claude Code and Codex repository examples),
   document the consumer loader mechanics of at least Claude Code
   and one other tool, and surface any de jure spec work in
   progress. Output: `research-solutions.md`.
2. **api-designer** — given the research output, decide whether a
   first-class loader + typed `Skill` value can be added without
   disturbing the Phase 3 `frozen-v1.0` surface; specify the loader
   signature, the composition surface, and the stability tier
   placement under D13. This is an API-surface judgement call.
   Output: the typed content for `02-scope-and-positioning.md` and
   `03-integration-model.md`.

`dx-designer` and `observability-architect` contribute inline
(error-message catalogue and cardinality mitigation respectively); a
full subagent pass is not justified unless the reviewer flags
insufficient depth in a second loop.

## Exit Criteria

1. A clear positioning decision for skill-bundle support in v1.0
   (first-class / convention / non-goal) locked in
   `01-decisions-log.md` as D122 with full rationale, alternatives
   considered, and direct reference to the solution-researcher
   output.
2. If first-class: the loader surface (D124), the composition
   surface (D125), the tool namespacing rule (D126), and the
   instruction-injection path (D128) are fully specified;
   `03-integration-model.md` contains a complete worked example
   wiring a real-world bundle into an orchestrator.
3. If convention-only: `03-integration-model.md` contains a single
   binding reference pattern with a complete worked example of
   how a consumer loads a `SKILL.md` bundle themselves and wires
   its tools + instructions on top of `LLMProvider` + `Invoker`.
   A menu of patterns is not acceptable.
4. If non-goal: `05-non-goals.md` explicitly catalogues the
   decision with rationale, and `04-dx-and-errors.md` documents the
   manual wiring path regardless.
5. `02-scope-and-positioning.md` contains the ecosystem
   intersection matrix (D123) with a sourced citation per field
   from the solution-researcher output.
6. All Phase 8 decisions (D122 onwards) recorded in
   `01-decisions-log.md` with rationale, alternatives, and
   cross-references to Phase 1 / 3 / 4 / 5 / 7 dependencies.
7. Explicit Phase 7 ↔ Phase 8 interaction statement (D131) for
   MCP-backed skill dependencies in `03-integration-model.md`.
8. Explicit confirmation in `01-decisions-log.md` that D09 /
   Non-goal 7 is not re-opened — the loader works from
   caller-provided filesystem paths only, with no registry talk
   (mirror of Phase 7 D109).
9. If any amendment to a Phase 3 `frozen-v1.0` signature is
   required, the amendment follows the Phase 1 amendment
   protocol and is explicitly justified in D134.
10. Binding non-goals catalogue (D133) in `05-non-goals.md`:
    no registry, no downloading, no runtime discovery, no hot
    reloading, no authoring tooling, no sandboxing.
11. Banned-identifier grep clean on all Phase 8 artifacts
    (decoupling contract).
12. Reviewer subagent verdict: PASS.
13. `review-phase` skill verdict: READY.
14. `REVIEW.md` written with the verdict, no critical issues
    outstanding.
15. Phase 8 transitions to `approved` and `docs/roadmap-status.md`
    is updated. The v1.0.0 freeze is then unblocked on the MCP /
    skills axis (Phase 7 already `approved`; Phase 8 `approved`).
