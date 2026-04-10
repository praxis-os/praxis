# Phase 8 — Decisions Log

> **Status:** `in-progress` — range reserved, no decisions adopted yet.
>
> Phase 8 was activated on 2026-04-10 by `plan-phase`, after Phase 7
> closed at **D121**. This file holds the decision range reservation
> and will accumulate adopted decisions as the phase advances through
> the working loop (subagents → reviewer → `review-phase`).

## Decision Range Reservation

- **First decision ID:** **D122** (immediately after Phase 7's last
  decision, D121).
- **Last decision ID:** **D134**. The Phase 8 range is
  **D122–D134**, contiguous, 13 decisions.
- **Contiguity rule:** Phase 8 owns a contiguous range that begins
  immediately after Phase 7's range. No skips, no gaps.
- **Ordering rule:** Phase 8 may begin adopting decisions now that
  Phase 7 is `approved`. D122 (positioning) must be locked before any
  other Phase 8 decision, because every downstream decision is
  conditional on it.

## Dependencies on Prior Phases

Phase 8 references but does not modify the following decisions from
earlier phases. Any conflict with these must be resolved in favour of
the earlier decision unless an explicit amendment is justified under
the Phase 1 amendment protocol.

- **Phase 1 — D09** — "no plugins in v1" / Non-goal 7. Any Phase 8
  proposal that requires runtime skill loading, dynamic registration,
  or a skill registry is out of scope and must be rejected. Phase 8
  must include an explicit re-confirmation of D09 (mirror of Phase 7
  D109).
- **Phase 1 — D13** — three-tier stability policy
  (`frozen-v1.0` / `stable-v0.x-candidate` / `experimental`). Any new
  type or interface introduced by Phase 8 must be assigned to a tier
  and justified.
- **Phase 3** — all `frozen-v1.0` interface signatures
  (`AgentOrchestrator`, `LLMProvider`, `tools.Invoker`,
  `budget.Guard`, `telemetry.*`, `errors.*`, `hooks.*`,
  `credentials.Resolver`, `identity.Signer`). Phase 8 may add
  alongside; it may not rewrite. Any pressure to amend a frozen
  signature (e.g., a new field on the `LLMProvider` request type to
  carry skill metadata) requires explicit justification under the
  Phase 1 amendment protocol.
- **Phase 4 — D57 / D60 / D61** — observability contract, the hard
  cardinality boundary, and the typed error taxonomy. Any new metric
  label introduced by Phase 8 for skill names or skill types must
  respect D60 verbatim.
- **Phase 5 — D67 / D68 / D73 / D77 / D78** — credential
  zero-on-close, stdlib-favoured posture, untrusted-output contract,
  and filter trust-boundary classification. If a Phase 8 "skill"
  reaches across a transport edge, the Phase 5 trust-boundary model
  applies via Phase 7 D116; if a skill's instructions reference a
  local script the LLM chooses to invoke, D77/D78 govern how the
  caller's `tools.Invoker` classifies the script's output.
- **Phase 7 — D106 / D109 / D111 / D112 / D113 / D115 / D116 /
  D117 / D120** — MCP positioning, D09 re-confirmation, tool
  namespacing convention, budget participation, error translation,
  metrics, trust classification, credential flow, and the Phase 7
  non-goals catalogue. Phase 8 uses these as load-bearing inputs
  when deciding how an MCP-backed "skill" integrates (D129).

## Adopted Decisions

User-confirmed trade-offs (2026-04-10) and reviewer-passed
on third-pass review (2026-04-10): the Phase 8 decisions D122–D134
are decided. The summary positions are listed below; the full
rationale, alternatives considered, and worked specifications live
in `02-scope-and-positioning.md`, `03-integration-model.md`,
`04-dx-and-errors.md`, and `05-non-goals.md` per the Phase 7 log
structure precedent (`docs/phase-7-mcp-integration/01-decisions-log.md`).

User-confirmed positions on the two load-bearing trade-offs:

- **D123 — unknown-field policy:** permissive-**preserve** (not
  permissive-ignore). Unknown frontmatter fields are preserved
  verbatim in `Skill.Extensions() map[string]any` and reported as
  `SkillWarning{Kind: WarnExtensionField, ...}`. Strict validation
  is composable on top by checking `len(warnings) == 0`. See
  `02-scope-and-positioning.md` §3.4 for the full rationale.
- **D131 — cross-module composition:** `praxis/skills` does **not**
  import `praxis/mcp`. The research intersection matrix
  (`research-solutions.md §3`) shows that no surveyed consumer
  declares MCP server dependencies in SKILL.md frontmatter, so
  `praxis/skills v1.0.0` does **not** ship a typed `MCPServerSpec`
  value type or a `Skill.MCPServers()` accessor. When a caller
  wants a skill whose instruction text references MCP-exposed
  tools, the caller configures `praxis/mcp` independently and
  passes both sub-modules' options to `praxis.NewOrchestrator`
  (`skills.WithSkill` for the instructions, `praxis.WithToolInvoker`
  for the MCP invoker). This keeps `praxis/skills`'s transitive
  dependency footprint minimal for consumers whose bundles do not
  use MCP, and preserves the Phase 5 / Phase 7 trust-boundary
  auditability by keeping MCP dialling visible in caller code.
  See `03-integration-model.md §6` for the full rationale and
  `04-dx-and-errors.md §1.4` for the worked wiring example.

  D131 also defers a future, additive evolution path: if the
  ecosystem later converges on a machine-readable `mcp_servers`
  frontmatter shape, a future minor version can add a typed
  `Skill.MCPServers() []MCPServerSpec` accessor without breaking
  v1.0.0 callers — they would simply migrate from reading
  `Skill.Extensions()["mcp_servers"]` (today's path) to the typed
  accessor (future path). See `05-non-goals.md §7` for the
  forward-compatibility note.

## Summary Table

| ID   | Summary | Status |
|------|---------|--------|
| D122 | Positioning of skill-bundle support in v1.0: **first-class sub-module** at `github.com/praxis-go/praxis/skills` | **decided** |
| D123 | Canonical `SKILL.md` shape: required (`name`, `description`) + optional (`license`, `compatibility`, `metadata`, `allowed-tools`); permissive-preserve unknown-field policy via `Skill.Extensions() map[string]any` | **decided** |
| D124 | Loader surface: `Open(fsys fs.FS, root string) (*Skill, []SkillWarning, error)` primary + `Load(path string)` wrapper; `LoadError` implementing full `errors.TypedError`; `SkillSubKind` named type | **decided** |
| D125 | Composition surface: `skills.WithSkill(s *Skill) praxis.Option`; panic at orchestrator-construction time on duplicate-name collision; preserves frozen `NewOrchestrator` single-return signature; no `tools.Invoker` parameter | **decided** |
| D126 | Tool namespacing: **v1.0.0 no-op** (skills do not declare new tools in frontmatter); reserved convention `{skillName}__{toolName}` documented for future ecosystem convergence | **decided** |
| D127 | Multi-skill conflict resolution: fail-loud (panic) on duplicate `Skill.Name()`; deterministic instruction-fragment ordering by `WithSkill` call order | **decided** |
| D128 | Instruction injection via additive system-prompt fragment composed at construction time; zero change to frozen `LLMProvider` request surface | **decided** |
| D129 | Budget participation: verbatim re-use of Phase 7 D112 (`tool_calls` + `wall_clock`); no new dimension; no double-counting; no per-skill sub-budget | **decided** |
| D130 | Observability: no new event types or spans; one bounded counter `praxis_skills_loaded_total` (status label only); skill names NEVER as metric labels (mirror Phase 7 D115); optional `MetricsRecorder` interface via D115 type-assertion pattern | **decided** |
| D131 | `praxis/skills` does NOT import `praxis/mcp`; no `MCPServerSpec` value type or `Skill.MCPServers()` accessor in v1.0.0; callers compose both sub-modules explicitly when a bundle's instructions reference MCP-exposed tools | **decided** |
| D132 | DX + error catalogue: typed `LoadError` with stable subkinds; worked end-to-end example; cross-reference (not disambiguation) with Claude Code `.claude/skills/`; no template-variable substitution | **decided** |
| D133 | Non-goals catalogue (11 binding items): no registry, no download, no runtime discovery, no hot-reload, no authoring tooling, no sandboxing, no `mcp_servers` recognised field, no provenance verification, no automatic credential injection, no consumer brand awareness, explicit D09 re-confirmation | **decided** |
| D134 | Impact on frozen v1.0 surface: zero amendments to Phase 3 `frozen-v1.0` signatures; all new types live in `praxis/skills` sub-module at `stable-v0.x-candidate` tier (freeze at `praxis/skills v1.0.0`); `MetricsRecorder` at `experimental` | **decided** |

Legend: **decided** = full rationale present in the relevant
artifact, reviewer pass clean, no outstanding contradictions,
ready for implementation phase. The Phase 8 decision range
**D122–D134 (13 decisions, contiguous)** is closed; D135 is
available for any future phase.

## Amendment Protocol

Once decisions are adopted, amendments follow the protocol recorded in
`docs/phase-1-api-scope/01-decisions-log.md#amendment-protocol` — the
same protocol used by all Phase 1–7 decisions.
