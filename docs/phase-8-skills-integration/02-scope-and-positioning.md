# Phase 8 — Scope and Positioning

**Decisions:** D122 (draft), D123 (draft), D134 (draft)
**Cross-references:** Phase 7 D106 (MCP sub-module framework),
Phase 7 D110 (package boundary posture),
Phase 1 D09 / Non-goal 7 (no plugins),
Phase 1 D13 (three-tier stability policy),
Phase 3 frozen-v1.0 interface surface,
Phase 4 D60 (cardinality boundary).

---

## 1. Positioning decision (D122 draft)

### Recommendation

praxis ships **first-class support** for skill bundles as an
**officially supported, separately-versioned Go sub-module** at
`github.com/praxis-os/praxis/skills`.

This mirrors the Phase 7 D106 framework precisely:

- The core module `github.com/praxis-os/praxis` ships **no** skill-loading
  code and **no** YAML-parsing dependency. Zero-skills consumers pay
  zero transitive cost.
- A second Go module, `github.com/praxis-os/praxis/skills`, lives in the
  same repository under a `skills/` directory with its own `go.mod`. It
  provides a typed `Skill` value and a filesystem loader, with composition
  utilities that wire loaded bundles into the core orchestrator via the
  frozen `tools.Invoker` and system-prompt paths.
- The module is maintained in the same repository, under the same license
  (Apache 2.0), by the same maintainer set, subject to the same CI
  pipeline (lint, test, banned-identifier grep, SPDX check, coverage gate,
  govulncheck).
- "First-class" means exactly what D106 §2 means — same repo, same CI,
  same code-of-conduct, documented from the top-level README, and subject
  to its own v1.0.0 stability commitment on an independent semver line.

### Rationale

The three candidate positionings are: (a) non-goal with only documented
manual wiring, (b) convention-only with a single reference pattern in
docs but no shipped code, and (c) first-class sub-module.

Option (a) is rejected because the skill-bundle ecosystem is becoming a
de-facto unit of agent capability distribution. Every praxis consumer who
uses skill bundles today must re-solve the same problems: frontmatter
parsing, path validation, instruction injection, tool namespacing, and
MCP server handoff. Leaving this as consumer work fragments the ecosystem
and re-invites the supply-chain risks (path traversal, YAML injection,
symbol collision) that a framework-provided loader can audit once and
amortise across all consumers.

Option (b) is rejected for the same reason it was rejected for MCP in
D106: a documented convention without a shipped reference implementation
will inevitably fork across consumers, producing incompatible naming
conventions, inconsistent error semantics, and unaudited path-resolution
logic. The first-consumer obligation argument from D106 §6 applies here
equally: praxis's first production consumer will need skill-bundle support,
and moving the work outside the framework simply moves the audit burden to
the consumer layer, where it is less reviewable.

Option (c) is chosen. The sub-module pattern pays all the costs of
option (a)'s zero-cost opt-out (non-skills consumers do not import the
module and do not carry the YAML parser or bundle-walking code) while
providing the correctness benefits of an audited reference implementation.
The pattern is proven by Phase 7; Phase 8 inherits the same rationale
verbatim.

### Conditions for reversal

The recommendation flips to **convention-only** if the solution-researcher
verdict is PIVOT, which is defined as: the ecosystem survey finds no
convergent `SKILL.md` frontmatter shape across two or more major tools
(Anthropic, Claude Code, Codex, skills.sh), such that D123 cannot produce
a meaningful intersection without praxis inventing its own format. In that
case, shipping a loader that parses an internally-defined format would
impose praxis-specific structure on a fragmented ecosystem, which violates
the "generic first" principle (seed §3.1). The fallback is one canonical
worked example in `examples/skills/` showing the manual wiring path.

The recommendation flips to **non-goal** if the solution-researcher verdict
is DEFER, which is defined as: a de jure skills specification RFC is in
active ratification across the major ecosystem stakeholders, with a stable
draft expected within six months, and praxis anchoring to the pre-ratification
draft would create a breaking change at v1.x. In that case D122 is recorded
as "deferred to v1.1.0" and D123–D132 are collapsed into the non-goals
catalogue.

**OPEN-01 resolved.** Solution-researcher verdict:
**GO (sub-module)** — see `research-solutions.md §9`. Neither the
PIVOT condition (format fragmentation at the intersection level) nor
the DEFER condition (imminent competing RFC in ratification) applies.
The reversal conditions stated above are recorded for completeness
and for future re-evaluation if the ecosystem state changes; they
are not blocking D122.

---

## 2. Package boundary and module layout

### Binding rule

The core module `github.com/praxis-os/praxis` **must not** import
`github.com/praxis-os/praxis/skills` or any code from the `skills/`
directory. The direction of the module dependency is strictly one-way:

```
praxis/skills → praxis (core)
praxis/skills → [YAML parser, stdlib]
praxis (core) → [nothing from skills/]
```

This mirrors D110 verbatim. The rationale is identical:

1. Dependency cleanliness. YAML parsing and bundle-walking code are
   non-trivial transitive dependencies. Consumers who orchestrate agents
   without skill bundles (the majority of early adopters) must not carry
   that cost.
2. Auditability isolation. The skill loader is a filesystem-reading,
   user-content-parsing component. Keeping it out of the core module
   means a security audit of the core module does not need to cover
   YAML deserialization attack surfaces.
3. Independent versioning. The `praxis/skills` sub-module can evolve its
   frontmatter field set as the ecosystem stabilises without triggering
   a core module version bump.

### Repository layout (additive to Phase 7)

```
praxis/                            # core module (github.com/praxis-os/praxis)
│   go.mod                         # unchanged by Phase 8
│   ...
├── mcp/                           # Phase 7 sub-module (unchanged)
│   ...
└── skills/                        # Phase 8 sub-module
    │   go.mod                     # module github.com/praxis-os/praxis/skills
    │   go.sum
    │   doc.go                     # package-level documentation
    │   skill.go                   # Skill value type and read-only accessors
    │   load.go                    # Load / Open entry points
    │   compose.go                 # WithSkill orchestrator option
    │   namespace.go               # tool-name namespacing helpers
    │   errors.go                  # loader error → TypedError mapping
    │   observability.go           # span + metric helpers
    │   examples_test.go           # godoc examples
    │   internal/
    │       frontmatter/           # YAML frontmatter parse + validation
    │       resolve/               # path resolution, traversal prevention
    │       normalize/             # tool-declaration normalization
```

The `skills/go.mod` requires the parent module for the frozen interfaces:

```go
require (
    github.com/praxis-os/praxis v0.5.x  // or v1.0.0 once released
    // YAML parser — see §2.1 below
)
```

The core module never requires `praxis/skills`. The core orchestrator
has no awareness of bundles at the runtime level; all bundle wiring is
completed at construction time before any invocation starts.

### 2.1 YAML parser recommendation

**OPEN-02 partially resolved.** The research recommends
`gopkg.in/yaml.v3` (BSD-3-Clause, maintained by Canonical) as the
primary candidate — de-facto standard in the Go ecosystem, already
present in the praxis `go.sum` as an indirect dependency, no
transitive web-stack pull. The alternative `go.yaml.in/yaml/v3`
(successor to yaml.v2 maintained by the same team) is a fallback if
yaml.v3 falls out of maintenance. Both satisfy the Phase 5 D73
stdlib-favoured posture. **The remaining implementation-phase gate**
is a govulncheck + `go mod graph` audit before the first
`praxis/skills` tag is cut — not a design-phase blocker.

---

## 3. Canonical SKILL.md shape (D123 draft)

### Status

The canonical field set below is **anchored to the research intersection
matrix** in `research-solutions.md §3`. Every field listed here has
either (a) universal presence across the surveyed ecosystem or (b)
explicit agentskills.io spec membership with observed parsing by at
least two major consumers. All previously OPEN annotations in this
section are closed by the research verdict (GO sub-module).

**OPEN-03 resolved** (was: confidence tiers as working assumptions).
Resolved by research §3 field matrix and §9 D123 anchor table.

**OPEN-04 resolved** (was: `resources` presence and semantics).
Resolved by research §2.3 and §3 matrix: the three-level directory
layout (`scripts/`, `references/`, `assets/`) is universally supported,
but the `resources` frontmatter field is not a spec field — resource
resolution happens against the bundle directory directly (via
instruction-body references like `${SKILL_DIR}/references/...`), not
via a declarative frontmatter list. `resources` is dropped from the
recognised set.

### 3.1 Required frontmatter fields

| Field | Type | Notes |
|---|---|---|
| `name` | string | Unique within a caller's skill set. Must match `^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$` (same LLM-safe constraint as tool names). Universal: required by the agentskills.io spec; observed in every inspected consumer. |
| `description` | string | Human-readable summary used by callers when building a skill catalogue for the LLM. Universal: required by the agentskills.io spec; observed in every inspected consumer. |

A `SKILL.md` file missing `name` or `description` is a loader error
(`SkillSubKindInvalidField`). These two fields are the universal
minimum confirmed by the research §3 matrix across agentskills.io
spec, Anthropic official bundles, Claude Code, OpenAI Codex, Gemini
CLI, Google ADK Python, and LangChain DeepAgents.

**Why `version` is NOT a recognised field.** The research matrix
(§3, row `version`) shows that `version` is not present in the
agentskills.io spec, not present in any surveyed consumer, and
appears only as an unofficial Claude Code plugin extension. Putting
`version` in the recognised set would contradict the research.
Bundles that declare `version` (e.g., Claude Code plugin bundles)
will have the value preserved under `Skill.Extensions()["version"]`
per the permissive-preserve policy (§3.4), and a
`SkillWarning{Kind: WarnExtensionField, Field: "version"}` will be
emitted.

### 3.2 Optional frontmatter fields (parsed and surfaced by typed accessors)

| Field | Type | Notes |
|---|---|---|
| `license` | string | SPDX-style licence identifier or free-text licence statement. Defined by the agentskills.io spec as an optional field; observed in every Anthropic official bundle inspected. Surfaced via `Skill.License() string`. |
| `compatibility` | string | Free-text compatibility statement (e.g., target model family, minimum agent version). Defined by the agentskills.io spec; parsed by Google ADK Python and LangChain DeepAgents; informational only (praxis does not enforce it). Surfaced via `Skill.Compatibility() string`. |
| `metadata` | `map[string]any` | Author-defined metadata bag. Defined by the agentskills.io spec; parsed by Google ADK Python and LangChain DeepAgents. praxis preserves the full map verbatim (nested scalars, sequences, and mappings all pass through). Surfaced via `Skill.Metadata() map[string]any`. |
| `allowed-tools` | `[]string` | Experimental agentskills.io field parsed by Claude Code as a pre-approval list for existing tools. praxis parses it and surfaces it as `Skill.AllowedTools() []string` but **does not enforce it** — enforcement is a caller concern via `tools.Invoker` configuration. The YAML key uses a hyphen, not an underscore (matches spec and Claude Code). |

None of the optional fields are required. A bundle that contains only
`name` + `description` + Markdown body is valid.

**Why `tools` and `mcp_servers` are NOT recognised fields.** The
research matrix (§3) is unambiguous: "Tool declarations in
frontmatter: not present in any consumer" and "MCP server field:
not present" (for all consumers except Codex, which uses a separate
`agents/openai.yaml` sidecar file that is not `SKILL.md`). Defining
typed parsing for these fields in v1.0.0 would freeze a shape that
no ecosystem consumer has ratified. Both fields are explicitly
excluded from the recognised set and are handled via the
permissive-preserve policy (§3.4) if they happen to appear. See
also `05-non-goals.md §7`.

### 3.3 Instructions source

A skill's instruction text is the **non-frontmatter Markdown body**
of `SKILL.md` in its entirety. There is no separate `instructions`
frontmatter field (it is not a spec field and no surveyed consumer
uses it).

The loader captures the body verbatim — whitespace, Markdown
formatting, embedded code blocks, and any `${...}` template-variable
placeholders are preserved as-is and passed to the LLM unchanged.
praxis does not perform template-variable substitution (see
`04-dx-and-errors.md §3.3` for the rationale and the
`${SKILL_DIR}` posture).

If the body is empty (SKILL.md contains only frontmatter), the
loader emits a `SkillWarning{Kind: WarnEmptyInstructions}` and the
resulting `Skill.Instructions()` returns `""`. This is a valid but
unusual configuration; strict callers can promote the warning to
an error.

### 3.4 Unknown-field policy

The loader applies **permissive-preserve with a typed warning**.

When the YAML frontmatter contains a field not in the recognised set
(§3.1 + §3.2), the loader:

1. Does not return an error.
2. **Preserves** the field's raw value in the `Skill.Extensions()` map
   under its exact frontmatter key. The value is whatever the YAML
   decoder produced: scalar (`string`, `int64`, `float64`, `bool`),
   sequence (`[]any`), or nested mapping (`map[string]any`).
3. Returns a `[]SkillWarning` alongside the `*Skill` value. Each warning
   carries the field name and a message of the form
   `"unrecognised frontmatter field %q: preserved in Extensions()"`,
   so callers are explicitly notified that an extension field was
   encountered.
4. Does not populate any typed accessor (`Name`, `Description`, ...)
   with an extension field's value. The separation between the typed
   intersection and the extension bag is strict.

**Rationale for permissive-preserve over both strict-reject and
permissive-ignore:**

- **No data loss.** Skill bundles in the wild carry non-spec fields that
  the bundle author intentionally placed there (Anthropic-specific tool
  metadata, Claude Code plugin `version`, aggregator-injected `risk` /
  `source` / `date_added`, future agentskills.io spec additions). A
  loader that discarded those fields would force the caller to fall
  back to raw YAML parsing whenever they need one.
- **Typed surface for the intersection, untyped bag for the rest.**
  The recognised fields (§3.1 + §3.2) flow into typed accessors. Everything
  else flows into `Extensions() map[string]any`. The caller reads typed
  values when they are in the portable intersection and type-asserts
  individual extension values when they need provider-specific data.
- **Forward compatibility.** When agentskills.io promotes a new field
  to the spec, existing bundles that already use it keep working on
  `praxis/skills` without any upgrade — the field appears in
  `Extensions()` until praxis's next release adds a typed getter for
  it. No breaking change, no bundle incompatibility during the transition
  window.
- **Strict mode is trivially composable.** Callers who want strict
  validation check `len(warnings) == 0` and/or
  `len(sk.Extensions()) == 0` and fail. The inverse (building
  permissive on top of a strict loader) is impossible without
  re-implementing the parser.
- **Extends the agentskills.io client implementation guide.** The
  guide explicitly recommends lenient parsing: warn on unknown
  fields, do not reject. praxis goes one step further by
  *preserving* the values rather than discarding them — a
  deliberate extension justified by the "no data loss" principle
  above. The guide does not mandate preservation; praxis adds it
  because the typed-intersection / untyped-bag split makes
  preservation cheap and gives caller policy code something to
  read.
- **Decoupling contract preserved.** praxis does not encode knowledge of
  any specific provider's extension fields. It preserves them verbatim
  as opaque values; the caller decides what to do.

**Strict-reject alternative rejected because:** it would require praxis to
maintain an exhaustive allow-list of every valid provider extension field
across the ecosystem, which inevitably lags behind spec evolution and
forces consumers to patch their bundles to conform to praxis's subset.
Incompatible with observed real-world bundles (see research-solutions.md
§3 matrix).

**Permissive-ignore (discard with warning) alternative rejected
because:** while warning-on-unknown is sufficient for the caller to detect
that something was skipped, it destroys the field's value. A caller who
wants to apply a policy based on an extension field (e.g., "reject any
bundle with `risk: unknown`") has no way to recover the data. Preserving
into `Extensions()` gives callers full agency without imposing praxis's
intersection as a hard constraint on upstream bundle authors.

---

## 4. Stability tier placement (D134 draft)

### Core module (unchanged)

The core `github.com/praxis-os/praxis` module's public surface remains
entirely at `frozen-v1.0`. Phase 8 adds **no types and no functions** to
the core module. This is the binding constraint.

### `praxis/skills` sub-module

The `praxis/skills` sub-module's public types are assigned to the
**`stable-v0.x-candidate`** tier under D13, not `frozen-v1.0`, at the
point when the core module reaches v1.0.0.

Concretely:

| Symbol | Tier at core v1.0.0 | Expected freeze |
|---|---|---|
| `skills.Skill` (value type) | `stable-v0.x-candidate` | `praxis/skills v1.0.0` |
| `skills.Load` | `stable-v0.x-candidate` | `praxis/skills v1.0.0` |
| `skills.Open` | `stable-v0.x-candidate` | `praxis/skills v1.0.0` |
| `skills.WithSkill` (orchestrator option) | `stable-v0.x-candidate` | `praxis/skills v1.0.0` |
| `skills.SkillWarning` + `WarnKind` constants | `stable-v0.x-candidate` | `praxis/skills v1.0.0` |
| `skills.LoadError` + `SkillSubKind` constants | `stable-v0.x-candidate` | `praxis/skills v1.0.0` |
| `skills.MetricsRecorder` (optional interface) | `experimental` | May move to `stable` at v1.0 |
| Internal loader types | Not exported | n/a |

**Removed from the surface vs earlier draft:** `ToolDeclaration`
and `MCPServerSpec` were in an earlier draft that assumed `tools`
and `mcp_servers` were frontmatter fields. The research intersection
matrix (§3.1, §3.2) shows neither is a spec field, so both types
are dropped from `praxis/skills v1.0.0`. When the ecosystem
converges on a machine-readable declaration shape for tool or MCP
dependencies in frontmatter, these types can be added in a future
minor version without breaking existing callers (who will then
find non-nil data via `Skill.Tools()` / `Skill.MCPServers()` if
those accessors are added at that point).

**Justification.** The `praxis/skills` module is a separately-versioned
sub-module with its own semver line. Its stability commitment is
independent of the core module's freeze. This is the same posture adopted
by Phase 7 (D121): `praxis/mcp v1.0.0` freezes its surface independently,
at a time potentially different from the core v1.0.0 tag.

The `stable-v0.x-candidate` tier is correct here because:

1. The agentskills.io spec itself is a living document with no
   versioned release tag (research §2.1). New optional fields may be
   promoted to the recognised set as the spec evolves; this would
   require new typed accessors on `Skill`, which are additive but
   still represent surface movement.
2. The `allowed-tools` field is marked experimental by the
   agentskills.io spec itself (research §3); its parsing behaviour
   may need to change as the experimental status resolves.
3. The `experimental` tier for `skills.MetricsRecorder` reflects
   that the optional-interface + type-assertion pattern (D115) is
   relatively new in the codebase and may be superseded by a
   framework-wide metrics extension mechanism before
   `praxis/skills v1.0.0` tags.

Additionally, D131 is now fully resolved (no typed cross-module
dependency on `praxis/mcp`), so the earlier concern about
`MCPServerSpec` shape mobility no longer applies — the type has
been dropped from the surface.

**Release pipeline note.** Consistent with D121's framework, the Phase 6
release-please manifest must be updated to add `skills/` as a second
extension module alongside `mcp/`. This is a Phase 6 observation, not an
amendment. The same D92/D93 multi-module release mechanics apply.

---

## 5. Relationship to the frozen Phase 3 surface

The table below covers every relevant frozen interface. The answer is
"not touched" for all rows. If any row needed to be "touched", this
would constitute an amendment requiring explicit justification under the
Phase 1 amendment protocol, a loud flag in D134, and a corresponding change
to the Phase 3 decisions log.

| Interface | Package | Touched? | Rationale |
|---|---|---|---|
| `AgentOrchestrator` | `orchestrator` | **Not touched** | Skill wiring is build-time only; callers use `skills.WithSkill(s)` which is a new constructor option shipped in `praxis/skills`, not a new method on the orchestrator interface. The orchestrator's `Invoke` and `InvokeStream` signatures are unchanged. The frozen constructor `praxis.NewOrchestrator(provider, opts...) *Orchestrator` is unchanged — single return, no error. Multi-skill tool-name collision is reported via **panic at construction time** (see `03-integration-model.md §1.4` and §5.1), not via a new error return, to preserve the frozen signature. |
| `LLMProvider` | `llm` | **Not touched** | Instruction injection from skill bundles is composed at orchestrator-construction time as an additive system-prompt fragment. No new field is added to `LLMRequest`, `LLMResponse`, or any other type in the `llm` package. The LLM provider never learns it is serving a skill-augmented invocation. |
| `tools.Invoker` | `tools` | **Not touched** | Skill-declared tools flow through `tools.Invoker` via the existing `Invoke(ctx, invCtx, call)` contract. The `ToolCall`, `ToolResult`, and `InvocationContext` shapes are preserved verbatim. |
| `budget.Guard` | `budget` | **Not touched** | Skill-originated tool calls participate via the existing `wall_clock` and `tool_calls` dimensions (re-use of D112). No new budget dimension. No per-skill budget ledger. |
| `budget.PriceProvider` | `budget` | **Not touched** | Skill bundles have no pricing dimension in v1.0.0. The `PriceProvider` contract is unchanged. |
| `hooks.PolicyHook` | `hooks` | **Not touched** | Skill-contributed tools are subject to the same policy hook chain as any other tool call. No new hook phase for "skill activation". |
| `hooks.PreLLMFilter` | `hooks` | **Not touched** | The system-prompt fragment contributed by a skill is composed before the first `LLMCall` state. Pre-LLM filters run over the composed message list including the skill-contributed fragment. No change to the filter interface. |
| `hooks.PostToolFilter` | `hooks` | **Not touched** | Tool outputs from skill-declared tools pass through the post-tool filter chain identically to any other tool output. |
| `telemetry.LifecycleEventEmitter` | `telemetry` | **Not touched** | No new event types are added to the frozen `LifecycleEventEmitter` interface. Skill-level observability signals are carried by optional extension points in `praxis/skills` (see `03-integration-model.md §7`). |
| `telemetry.AttributeEnricher` | `telemetry` | **Not touched** | Skill names are not injected as framework-level span attributes. Callers who want skill-name attribution implement their own `AttributeEnricher`. |
| `errors.TypedError` | `errors` | **Not touched** | Loader errors are mapped into the existing taxonomy using new sub-kind strings. The `TypedError` interface, its `Kind()` method, and all seven concrete error types are unchanged. |
| `errors.Classifier` | `errors` | **Not touched** | The classifier is not modified. Loader-originated errors are constructed with the existing `ErrorKindTool` and `ErrorKindSystem` kinds; the classifier already handles both. |
| `credentials.Resolver` | `credentials` | **Not touched** | The skill loader does not resolve credentials and does not interact with the `credentials.Resolver` interface in any way. Credential resolution for skill-referenced MCP servers (when applicable) flows entirely through the caller's `praxis/mcp` configuration, identical to a non-skill MCP setup. See `05-non-goals.md §9`. |
| `identity.Signer` | `identity` | **Not touched** | Skill-contributed tool calls pass through the same identity signing path as any other tool call. The `Signer` interface is unchanged. |

**Conclusion:** Phase 8 makes zero amendments to the Phase 3 frozen surface.
The sub-module architecture (borrowed from Phase 7) is precisely the
mechanism that enables this: all new types live in `praxis/skills`, which
depends on the core module but is never depended on by it.

---

**Next:** `03-integration-model.md` specifies the loader surface, composition
surface, tool namespacing, multi-skill conflict resolution, budget flow,
observability, and the Phase 7 MCP interaction for skill-declared MCP servers.
