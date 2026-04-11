# Review: Phase 8 — Skills Integration

## Overall Assessment

Phase 8 delivers a coherent, sourced, and internally consistent
design for `github.com/praxis-os/praxis/skills`, a
separately-versioned Go sub-module that loads installable
`SKILL.md` skill bundles. After three adversarial reviewer passes
(BLOCK → FAIL → PASS with non-blocking observations, both of
which were addressed in-place), the artifact set is free of
critical contradictions, makes zero amendments to the Phase 3
frozen-v1.0 interface surface, does not re-open D09, and anchors
every load-bearing decision to the solution-researcher's sourced
ecosystem survey rather than to speculation.

## Critical Issues

None. All critical findings from the three reviewer passes have
been resolved at their primary locations with no re-introduction
found during consistency walks.

## Important Weaknesses

1. **Implementation-phase govulncheck gate on the YAML parser is
   deferred, not closed.** `02-scope-and-positioning.md §2.1`
   resolves OPEN-02 partially: `gopkg.in/yaml.v3` is recommended
   as the primary candidate with `go.yaml.in/yaml/v3` as fallback,
   but the final govulncheck + `go mod graph` audit is explicitly
   deferred to the implementation phase. This is acceptable as a
   design-phase artifact, but the implementation phase must close
   this gate before the first `praxis/skills` tag is cut.
   Recommended fix: add an implementation-phase precondition item
   to the Phase 8 → implementation handoff checklist.

2. **`AllowedTools()` stability tier is implicit, not explicit.**
   The D134 stability-tier table in `02-scope-and-positioning.md §4`
   lists `skills.Skill` as a single row, inheriting
   `stable-v0.x-candidate`. But `allowed-tools` is flagged by the
   agentskills.io spec itself as an experimental field. A future
   spec revision that changes its semantics would force
   `AllowedTools()` to move — which is exactly what the
   `stable-v0.x-candidate` tier allows, but it deserves an
   explicit note. Recommended fix: add a parenthetical to the
   `skills.Skill` row of the tier table noting that `AllowedTools()`
   specifically tracks the agentskills.io experimental field and
   is the most likely accessor to move during v0.x.

3. **`WarnEmptyInstructions` has no worked example of caller
   handling.** The `04-dx-and-errors.md` strict-mode wiring pattern
   (§1.2) mentions treating warnings as errors but does not
   specifically show a caller reacting to `WarnEmptyInstructions`.
   This is an edge case (bundle with only frontmatter, no body) but
   it is catchable. Recommended fix: add a one-line note in §1.2
   that empty-instruction bundles are valid but unusual and the
   strict-mode pattern will reject them.

## Open Questions

1. **How does a caller distinguish a `Skill` that contributes
   instructions only (the common case) from a future `Skill` that
   contributes additional metadata?** Today, the distinction is
   invisible to the caller — both load the same way. If a future
   minor version adds typed accessors for new fields, callers
   iterating extension bags may need guidance on how to migrate.
   Not a blocker; worth capturing as an implementation-phase note.

2. **What is the behaviour of `skills.Load` / `skills.Open` when
   the bundle root contains multiple files named `SKILL.md` with
   different casings on a case-insensitive filesystem?** The
   design explicitly says case-insensitive filesystem behaviour
   is undefined and depends on the `fs.FS` implementation. This is
   defensible for design-phase scope, but implementation phase
   should document the observed behaviour on macOS/APFS and
   Windows NTFS explicitly.

3. **Does the `skills.ComposedInstructions` debug helper
   (§3.1 of `03-integration-model.md`) need to handle the case
   where the option list contains non-skill options?** The
   signature is `ComposedInstructions(opts ...praxis.Option) string`
   and the semantics say it walks the list and extracts skill
   options. The behaviour on non-skill options is not stated
   explicitly. Minor documentation gap for the implementation phase.

## Decoupling Contract Check

**PASS.**

Banned-identifier grep across all Phase 8 artifacts
(`docs/phase-8-skills-integration/*.md`) for `custos`, `reef`,
`governance_event`, `\borg\.id\b`, `\bagent\.id\b`, `\buser\.id\b`,
`\btenant\.id\b`, `M1\.5`, `apps/server`, `internal/policy`
returned zero matches (one self-reference to the grep pattern
itself in `REVIEW.md` §"Final verification" of the reviewer
output does not count — it is the regex, not an instance).

No consumer brand names appear in source code, design rationale,
or decision rationale. The research file
(`research-solutions.md`) references ecosystem consumers
(Anthropic, Claude Code, OpenAI Codex, Gemini CLI, skills.sh,
Antigravity, etc.) because it is surveying external facts, which
is explicitly permitted by the decoupling contract for the
research deliverable. None of those references leak into the
design artifacts.

## Recommendations

- **Promote Phase 8 to `approved`** and update
  `docs/roadmap-status.md` to reflect the new status. D122–D134
  are all decided; the decision range is closed.
- **Close the implementation-phase handoff checklist** with the
  three deferred items surfaced by this review: (1) YAML parser
  govulncheck gate (OPEN-02 residual), (2) case-insensitive
  filesystem behaviour documentation, (3) `ComposedInstructions`
  behaviour on non-skill options.
- **Add `praxis/skills` to the release-please manifest** as a
  second sub-module alongside `praxis/mcp`, following the D121
  precedent from Phase 7. This is flagged in
  `02-scope-and-positioning.md §4` under "Release pipeline note"
  as a Phase 6 observation; the implementation phase must execute
  the manifest update.
- **Acknowledge v1.0.0 freeze unblocked on the MCP / skills
  axis.** With Phase 7 already `approved` and Phase 8 now ready
  for `approved`, the two blocking phases for the v1.0.0 freeze
  on the extensibility surface are closed. The seed roadmap can
  progress to implementation.
- **Preserve the sub-module precedent.** Phase 7 and Phase 8 both
  landed on `praxis/<name>` sub-modules with independent semver
  lines. Any future "extensibility axis" phase (e.g., a future
  provider adapter, a future observability backend) should be
  expected to follow the same pattern unless there is explicit
  reason to deviate.

## Verdict: READY

All critical and important issues from three reviewer passes are
resolved, the six artifacts plus research are internally consistent,
the decoupling contract is clean, zero Phase 3 frozen-interface
amendments are implied, and Phase 8 can transition to `approved`
with confidence that the v1.0.0 freeze is unblocked on the skills
axis.
