# Phase 8 — Integration Model

**Decisions:** D124 (draft), D125 (draft), D126 (draft), D127 (draft),
D128 (draft), D129 (draft), D130 (draft), D131 (draft)
**Cross-references:** Phase 7 D110 (API surface pattern), D111 (namespacing),
D112 (budget), D115 (metrics), D116 (trust boundary),
Phase 3 frozen-v1.0 surface, Phase 4 D60 (cardinality),
Phase 5 D73 (stdlib-favoured), D77 (untrusted output).

---

## 1. Public API surface of `praxis/skills` (D124, D125 drafts)

### 1.1 Loader signature (D124)

Two constructor entry points are shipped. The primary entry point is the
`fs.FS`-based form; the path-based form is a convenience wrapper:

```go
// Package skills provides a typed loader and composition surface for
// SKILL.md-format skill bundles.
//
// A skill bundle is a filesystem directory containing a SKILL.md descriptor
// (Markdown with YAML frontmatter) and optional supporting files. The loader
// reads the bundle at construction time only. No network I/O is performed.
// No scripts are executed. The bundle directory is treated as a closed unit:
// no path may escape the root.
//
// Package: github.com/praxis-go/praxis/skills
package skills

// Open loads a skill bundle from the given filesystem rooted at root.
//
// root must be a valid path within fsys. Open reads SKILL.md at
// fsys/root/SKILL.md, parses the frontmatter, validates required fields,
// and returns a Skill value together with any non-fatal warnings.
//
// Open never dials any network address. MCP server specs declared in
// the bundle directory are not parsed or interpreted by the loader
// beyond SKILL.md itself. Scripts, references, and assets are
// filesystem artefacts the caller's tools may read at their own
// discretion; praxis/skills does not enumerate them.
//
// Errors: returns a *LoadError wrapping an errors.TypedError if the
// bundle is missing, malformed, or invalid (see §2.5 for the typed error
// catalogue). Partial failures (e.g., one unresolvable resource path)
// are returned as entries in []SkillWarning with err == nil.
func Open(fsys fs.FS, root string) (*Skill, []SkillWarning, error)

// Load loads a skill bundle from the given filesystem path.
//
// Load is a convenience wrapper around Open that constructs an os.DirFS
// rooted at the directory containing path. Callers who need hermetic
// tests or custom VFS implementations should use Open directly.
//
// path must point to either:
//   - the bundle's SKILL.md file, or
//   - the bundle's root directory (SKILL.md is assumed to be directly inside).
func Load(path string) (*Skill, []SkillWarning, error)
```

**Why `Open` is primary, `Load` is the wrapper — rationale.**

The `fs.FS`-based form is preferred because it is directly testable without
a real filesystem (callers pass `fstest.MapFS` in unit tests), it is
hermetic (no OS-level path expansion, no `os.Getwd` dependency), and it
mirrors the approach Phase 7 adopted for `InMemoryTransport` testability.
`Load` exists as the ergonomic entry point for production use, where the
bundle root is an on-disk path the caller provides. Shipping both eliminates
the "testability vs. ergonomics" trade-off.

**Why not `skills.Load(path string) (*Skill, error)` (no warnings).**

The warning slice is necessary because the permissive-preserve policy (D123 §3.4)
surfaces unknown fields as warnings (and preserves their values in
`Skill.Extensions()`) rather than failing. A caller who wants strict validation
wraps the call:

```go
s, warnings, err := skills.Load(bundlePath)
if err != nil { /* fatal */ }
if len(warnings) > 0 { /* treat as error or log */ }
```

Removing warnings from the signature would require callers to provide
strict-mode options, which is more API surface. The pattern of `(value,
warnings, error)` is established precedent in the Go ecosystem (e.g.,
`go/parser.ParseFile` returns partial ASTs alongside errors).

### 1.2 `Skill` value type

`Skill` is an opaque struct with read-only accessor methods. All fields
are populated at `Open` / `Load` time; the type is immutable after
construction.

```go
// Skill is an immutable, loaded representation of a SKILL.md skill bundle.
//
// All accessor methods are safe for concurrent use. Skill values are
// obtained via Open or Load; they cannot be constructed directly by callers.
type Skill struct {
    // unexported fields
}

// Name returns the skill's name as declared in frontmatter.
// Satisfies ^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$.
func (s *Skill) Name() string

// Description returns the skill's human-readable description.
func (s *Skill) Description() string

// Instructions returns the instruction text the skill contributes
// to the agent's system prompt. The source is the non-frontmatter
// Markdown body of SKILL.md, captured verbatim with no template-
// variable substitution. Returns "" if the bundle's body is empty
// (in which case Load also emits a SkillWarning with Kind ==
// WarnEmptyInstructions).
//
// Note: there is no `instructions` frontmatter field. The body IS
// the instructions. See 02-scope-and-positioning.md §3.3.
func (s *Skill) Instructions() string

// (No Version() accessor: `version` is not a recognised
// frontmatter field per 02-scope-and-positioning.md §3.1. Bundles
// that declare `version` (e.g., Claude Code plugin bundles) have
// the value preserved in Extensions()["version"] under the
// permissive-preserve policy.)

// License returns the skill's declared license string, or "" if the
// frontmatter did not specify one. Informational only; praxis does not
// interpret the value.
func (s *Skill) License() string

// Compatibility returns the skill's declared compatibility statement,
// or "" if the frontmatter did not specify one. Informational only;
// praxis does not enforce compatibility ranges.
func (s *Skill) Compatibility() string

// Metadata returns the author-declared metadata map from the skill's
// frontmatter, or nil if the frontmatter did not specify one.
//
// The map is owned by the Skill. Callers MUST NOT mutate it; the
// behaviour under mutation is unspecified. Follows the same convention
// as Extensions() (see below).
//
// This is the agentskills.io-spec `metadata` field, distinct from the
// unknown-field bag exposed by Extensions(). A field is only in
// Metadata() if the bundle's frontmatter had a top-level `metadata`
// key whose value is a mapping.
func (s *Skill) Metadata() map[string]any

// AllowedTools returns the skill's declared allowed-tools list as a
// slice of tool name strings, or nil if the frontmatter did not
// specify one.
//
// AllowedTools is an agentskills.io-spec experimental field. praxis
// parses and surfaces it for caller introspection but does NOT enforce
// it at runtime — enforcement is a caller concern, applied via the
// caller's tools.Invoker configuration. Treating AllowedTools as an
// authoritative permission boundary is a caller policy decision, not
// a praxis guarantee.
func (s *Skill) AllowedTools() []string

// Extensions returns the frontmatter fields that are NOT part of the
// recognised SKILL.md intersection (see 02-scope-and-positioning.md
// §3.1 / §3.2: name, description, license, compatibility, metadata,
// allowed-tools) preserved verbatim from the YAML frontmatter.
//
// Keys are the exact frontmatter field names. Values are whatever the
// YAML decoder produced for that field: a scalar (string, int64,
// float64, bool), a sequence ([]any), or a nested mapping
// (map[string]any). Callers type-assert per field as needed.
//
// Extensions is distinct from Metadata(). The latter returns the
// agentskills.io-spec `metadata` field (which is a recognised optional
// field and thus NOT in Extensions). Extensions contains only fields
// the loader did not recognise at all: consumer-specific extensions
// like Claude Code's `model` / `effort` / `version`, aggregator
// metadata like `risk` / `source`, and any future spec field praxis
// has not yet promoted to a typed accessor.
//
// The returned map is owned by the Skill. Callers MUST NOT mutate it;
// the result of mutation is unspecified (may be visible across
// Extensions() calls on the same Skill value). This follows the same
// convention as http.Header and similar stdlib maps.
//
// The Extensions map is populated during Open / Load. Each extension
// field also produces a SkillWarning with Kind == WarnExtensionField,
// so callers who want strict validation can fail on len(warnings) > 0
// without walking the Extensions map.
//
// Returns nil if the bundle's frontmatter contained only recognised
// fields.
func (s *Skill) Extensions() map[string]any
```

**Why an opaque struct with accessors rather than a public struct.**

A public struct's fields cannot be removed or renamed without a breaking
change. An opaque struct with accessor methods allows the internal
representation to evolve as the ecosystem's frontmatter field set
stabilises, while keeping the public API surface frozen at the method
set. This is consistent with the "accept interfaces, return concrete types"
Go idiom: the concrete type's field layout is not part of the API contract,
only the method set is. Callers cannot construct a `Skill` directly —
they always go through `Load` or `Open` — so the unexported fields do not
limit composability.

### 1.3 Supporting types

`praxis/skills v1.0.0` does NOT ship `ToolDeclaration`,
`MCPServerSpec`, or `ResourceRef` value types. An earlier draft of
this document assumed that `tools`, `mcp_servers`, and `resources`
were frontmatter fields; the research intersection matrix
(`research-solutions.md §3`) shows that none of the three is a
spec field and none is parsed by any surveyed consumer. Shipping
typed values for non-existent fields would freeze a shape that has
no ecosystem justification.

When the ecosystem converges on a machine-readable declaration for
any of these concepts, the corresponding types can be added in a
future minor version as purely additive API.

The only supporting type shipped by `praxis/skills v1.0.0` is the
diagnostic `SkillWarning` + `WarnKind` family described below.

```go
// SkillWarning is a non-fatal diagnostic produced during bundle loading.
// Callers may treat warnings as errors if they require strict validation:
//
//   sk, warnings, err := skills.Load(path)
//   if err != nil { return err }
//   if len(warnings) > 0 { return fmt.Errorf("strict mode: %v", warnings) }
type SkillWarning struct {
    // Kind classifies the warning (see WarnKind constants).
    Kind WarnKind

    // Field is the frontmatter key that triggered the warning, or ""
    // for structural warnings (e.g., empty instructions body).
    Field string

    // Message is a human-readable description of the issue.
    Message string
}

// WarnKind classifies a SkillWarning.
type WarnKind string

const (
    // WarnExtensionField indicates that the frontmatter contained a
    // field not in the recognised SKILL.md intersection. The field's
    // value has been preserved in Skill.Extensions() under the same
    // key. See 02-scope-and-positioning.md §3.4.
    WarnExtensionField WarnKind = "extension_field"

    // WarnEmptyInstructions indicates that the bundle's
    // non-frontmatter Markdown body was empty, so the Skill
    // contributes no instruction text. The Skill still loads; its
    // Instructions() method returns "".
    WarnEmptyInstructions WarnKind = "empty_instructions"

    // (No WarnUnresolvableResource in v1.0.0 — resources are not a
    // recognised frontmatter field per 02-scope-and-positioning.md §3.2.
    // This slot is reserved for a future minor version if the
    // ecosystem adopts a declarative `resources` field.)
)
```

### 1.4 Composition entry point on `AgentOrchestrator` (D125)

The frozen Phase 3 constructor `praxis.NewOrchestrator(provider,
opts...) *Orchestrator` is unchanged (single return, no error).
Skill composition uses a new **functional option** `skills.WithSkill`
shipped in the `praxis/skills` sub-module. This is additive — it
does not touch the orchestrator's interface, the constructor
signature, or any core package.

```go
// WithSkill returns a praxis.Option that wires the given Skill into
// the orchestrator at construction time:
//
//   - Skill.Instructions() is appended to the system prompt fragment
//     list. Fragments are injected before the first LLMCall in the
//     order they were passed to WithSkill. See D128 for the instruction
//     injection path.
//
// WithSkill takes ONLY the *Skill value. It does not take a
// tools.Invoker parameter because praxis/skills v1.0.0 does not
// recognise tool declarations in SKILL.md frontmatter (see
// 02-scope-and-positioning.md §3.2 and research-solutions.md §3).
// If a bundle's instructions reference tools that must execute, the
// caller registers those tools via the orchestrator's existing
// tools.Invoker option — outside WithSkill. WithSkill's
// responsibility is limited to instruction composition.
//
// WithSkill panics at orchestrator-construction time if an
// instruction fragment produced by the Skill conflicts with a
// previously-composed fragment in a way the composition cannot
// resolve deterministically (see D127 §5.1). The panic preserves
// the frozen praxis.NewOrchestrator signature (single return, no
// error), which would otherwise have to be amended — a frozen-v1.0
// change rejected by D125 and 02-scope-and-positioning.md §5.
//
// Package: github.com/praxis-go/praxis/skills
func WithSkill(s *Skill) praxis.Option
```

**Concrete wire-up example:**

```go
bundle, _, err := skills.Load("/path/to/my-skill")
if err != nil {
    log.Fatal(err)
}

orch := praxis.NewOrchestrator(
    llmProvider,
    skills.WithSkill(bundle),
    // other options (tools.Invoker, hooks, budget, ...) as usual
)
```

**Why option (a) — `WithSkill` — over option (b) — caller manual pull.**

Option (b) (caller manually reads `s.Instructions()` and appends to
their own system prompt) is simpler in terms of API surface and is
what a "convention-only" positioning would produce. However, it
requires every caller to implement the same composition logic:
instruction ordering, fragment deduplication, deterministic
injection point. This is the kind of correctness-load that the
framework exists to absorb. `WithSkill` centralises that logic,
emits consistent observability, and enforces the ordering policy
(D127 §5.2) at construction time.

Option (a) is the ergonomic, correctness-enforcing choice without
touching any frozen interface — it is purely additive, living in
`praxis/skills`.

### 1.5 Typed errors for loader failures

`LoadError` is a concrete error type implementing the Phase 3
frozen `errors.TypedError` interface (Kind, HTTPStatusCode, Unwrap).
`SkillSubKind` is a named type, not `string`, so callers can branch
in a type-safe manner and `go vet` can analyse exhaustive switches.

```go
// SkillSubKind classifies the specific failure mode within a
// LoadError. Mirrors Phase 3's ToolSubKind pattern.
type SkillSubKind string

const (
    // SkillSubKindMissing: SKILL.md or bundle root not found / not readable.
    SkillSubKindMissing       SkillSubKind = "skill_bundle_missing"

    // SkillSubKindMalformedYAML: frontmatter fails to parse; YAML bomb;
    // alias cycle; malformed embedded JSON schema.
    SkillSubKindMalformedYAML SkillSubKind = "skill_bundle_malformed_yaml"

    // SkillSubKindInvalidField: required field missing / empty; validation
    // regex violation; file size > 256 KiB.
    SkillSubKindInvalidField  SkillSubKind = "skill_bundle_invalid_field"

    // SkillSubKindPathEscape: attempted read outside the bundle root
    // (traversal via .. or absolute path).
    SkillSubKindPathEscape    SkillSubKind = "skill_bundle_path_escape"
)

// LoadError is returned by Open and Load when the bundle cannot be
// loaded. It implements errors.TypedError.
type LoadError struct {
    // Bundle is the path or root that was being loaded.
    Bundle string

    // SubKind classifies the failure.
    SubKind SkillSubKind

    // Cause is the underlying error, if any. Unwrap returns it.
    Cause error
}

// Kind implements errors.TypedError. Path-level failures classify as
// ErrorKindSystem; content-level failures (malformed frontmatter,
// invalid field values) classify as ErrorKindTool so they flow through
// the existing Phase 4 tool-error routing. All loader errors are
// non-retryable regardless of Kind.
func (e *LoadError) Kind() errors.ErrorKind {
    switch e.SubKind {
    case SkillSubKindMissing, SkillSubKindPathEscape:
        return errors.ErrorKindSystem
    default: // SkillSubKindMalformedYAML, SkillSubKindInvalidField
        return errors.ErrorKindTool
    }
}

// HTTPStatusCode implements errors.TypedError. All loader errors are
// configuration errors (400-equivalent) rather than server errors.
func (e *LoadError) HTTPStatusCode() int { return 400 }

// Error implements the error interface.
func (e *LoadError) Error() string {
    if e.Cause != nil {
        return fmt.Sprintf("skills: %s: bundle=%s: %v", e.SubKind, e.Bundle, e.Cause)
    }
    return fmt.Sprintf("skills: %s: bundle=%s", e.SubKind, e.Bundle)
}

// Unwrap implements errors.TypedError for errors.Is / errors.As
// traversal.
func (e *LoadError) Unwrap() error { return e.Cause }
```

All loader errors are non-retryable. The classifier treats them as
terminal configuration errors, not transient I/O failures. There is
no `SkillSubKindNameCollision` — instruction-composition conflicts
during `WithSkill` happen at orchestrator-construction time and
**panic** (see §5.1), rather than returning a `LoadError`, to
preserve the frozen `NewOrchestrator` single-return signature.

---

## 2. Loader lifecycle (D124 continued)

From `Load(path)` or `Open(fsys, root)` to a returned `*Skill`:

### 2.1 Path validation

For `Load(path)`:

1. The path is resolved to an absolute path via `filepath.Abs`.
2. If path points to a file named `SKILL.md`, the bundle root is its
   parent directory. If path points to a directory, the bundle root is
   that directory.
3. The bundle root is verified to exist (`os.Stat`). Failure is a
   `SkillSubKindMissing` error.
4. An `os.DirFS` is constructed at the bundle root. All subsequent
   operations use this `fs.FS`.

For `Open(fsys, root)`:

1. `root` is cleaned with `path.Clean` and validated to not start with
   `..` or contain absolute path components. Any escape attempt is a
   `SkillSubKindPathEscape` error at this step.
2. The `SKILL.md` file is opened at `root/SKILL.md`. The loader
   requires the exact casing `SKILL.md`; behaviour on case-insensitive
   filesystems that silently match `skill.md` or `Skill.md` is
   undefined and depends on the underlying `fs.FS` implementation.

### 2.2 Frontmatter parse and validation

1. The `SKILL.md` file is read in full. Maximum size: 256 KiB. Files
   larger than this are rejected with `SkillSubKindInvalidField` (the
   field is `"file_size"`).
2. The frontmatter block (between the opening `---` and closing `---` or
   `...` YAML document markers) is extracted. If no frontmatter markers
   are present, the entire file is treated as the instructions body (no
   frontmatter, no metadata fields, minimal-valid bundle).
3. The frontmatter YAML is parsed into the internal descriptor struct.
   YAML anchors that would expand to more than 64 KiB of text are
   rejected (YAML bomb mitigation). YAML alias cycles are rejected.
4. Required fields (`name`, `description`) are validated as non-empty.
5. The `name` field is validated against `^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`.
6. Recognised optional fields (§2.3) are parsed and validated.
7. Each field not in the recognised set is preserved verbatim in the
   Skill's internal extensions map (exposed via `Skill.Extensions()`)
   **and** a corresponding `SkillWarning{Kind: WarnExtensionField, ...}`
   is appended to the returned `[]SkillWarning` slice. This is the
   permissive-preserve policy per D123 §3.4. Neither the typed
   accessors nor the tool / MCP / resource extraction paths read
   from the extensions map.

### 2.3 Optional-field normalisation

For the recognised optional fields (§02 §3.2), the loader:

1. If `license` is present, validates it is a string and stores it
   for `Skill.License()`. Non-string types (e.g., a mapping) produce
   `SkillSubKindInvalidField`.
2. If `compatibility` is present, same rules as `license`.
3. If `metadata` is present, validates it is a YAML mapping and stores
   it verbatim as `map[string]any` for `Skill.Metadata()`. Nested
   scalars, sequences, and mappings are preserved as the YAML decoder
   produced them. A non-mapping `metadata` value is a
   `SkillSubKindInvalidField` error.
4. If `allowed-tools` is present, validates it is a YAML sequence of
   strings and stores it as `[]string` for `Skill.AllowedTools()`.
   A non-sequence value, or a sequence containing non-strings, is a
   `SkillSubKindInvalidField` error. The YAML key uses a hyphen
   (`allowed-tools`), matching the agentskills.io spec and Claude Code.
5. Any other top-level key is preserved into `Skill.Extensions()`
   with a `WarnExtensionField` warning, per §2.2 point 7.

There is no tool declaration normalisation, no MCP server spec
extraction, and no resource list resolution in v1.0.0 — none of
these fields is in the recognised set (see §02 §3.2 "Why `tools`
and `mcp_servers` are NOT recognised fields" and §02 §3 OPEN-04
resolution for `resources`).

### 2.4 Failure modes and their typed error kinds

| Failure mode | `LoadError.Kind()` | SubKind | Retryable? |
|---|---|---|---|
| `SKILL.md` file not found / bundle root not readable | `ErrorKindSystem` | `SkillSubKindMissing` | No |
| Path escape attempt (traversal) | `ErrorKindSystem` | `SkillSubKindPathEscape` | No |
| YAML parse failure | `ErrorKindTool` | `SkillSubKindMalformedYAML` | No |
| YAML bomb / anchor overexpansion | `ErrorKindTool` | `SkillSubKindMalformedYAML` | No |
| Required field missing (`name`, `description`) | `ErrorKindTool` | `SkillSubKindInvalidField` | No |
| `name` regex violation | `ErrorKindTool` | `SkillSubKindInvalidField` | No |
| `metadata` not a mapping | `ErrorKindTool` | `SkillSubKindInvalidField` | No |
| `allowed-tools` not a sequence of strings | `ErrorKindTool` | `SkillSubKindInvalidField` | No |
| `license` / `compatibility` not a string | `ErrorKindTool` | `SkillSubKindInvalidField` | No |
| File too large (> 256 KiB) | `ErrorKindTool` | `SkillSubKindInvalidField` | No |

All loader errors are classified as non-retryable. Skill-bundle
loading failures are configuration errors, not transient I/O
failures. Instruction-composition conflicts during `WithSkill` do
not appear in this table — they **panic** at construction time to
preserve the frozen `NewOrchestrator` signature (see §1.5 and §5.1).

---

## 3. Composition with `AgentOrchestrator` (D125 continued, D128 draft)

### 3.1 Instruction injection path (D128)

**Commitment:** The instruction injection path does not modify any frozen
`LLMProvider`, `LLMRequest`, `LLMResponse`, or `Message` type. Zero
amendments to Phase 3 frozen signatures.

The instruction text contributed by a skill (`Skill.Instructions()`) is
injected as an additive fragment prepended to the system prompt at
orchestrator-construction time. The orchestrator constructs a final
system prompt by concatenating:

```
[Caller's own system prompt, if any]

--- Skills ---

[Fragment from skill 1, if Instructions() != ""]

[Fragment from skill 2, if Instructions() != ""]
```

The separator `--- Skills ---` and the per-fragment spacing are
deterministic. The exact separator string is an implementation detail
of the composition helper and is not exposed as a configurable parameter
in v1.0.0.

The final composed system prompt is a single string that the orchestrator
passes via the existing `LLMRequest` system-prompt field. No new field is
added. The LLM provider receives a string that happens to contain skill
instructions; it has no awareness that it came from a bundle.

**Why "at construction time" and not "at invocation time."**

If instruction injection happened at `Invoke` time (per-invocation), callers
could not inspect the final prompt at construction time, which violates the
"caller-observable final prompt for debugging" requirement in the plan §Scope.
Build-time composition means the final prompt is a static property of the
orchestrator instance, inspectable before any LLM call is made.

**Caller observability.** The composed system prompt is exposed via
a pure helper function in `praxis/skills`, not via a method on any
frozen orchestrator type:

```go
// ComposedInstructions returns the final instruction fragment that
// WithSkill would inject into the orchestrator's system prompt given
// the supplied options. It accepts the same options the caller would
// pass to praxis.NewOrchestrator and returns the concatenated fragment
// text (empty string if no skills.WithSkill options are present).
//
// ComposedInstructions does not build an orchestrator. It is intended
// for debugging and golden-file testing of prompt composition, so
// callers can validate the final text before paying the cost of an
// orchestrator + LLM provider round trip.
//
// Package: github.com/praxis-go/praxis/skills
func ComposedInstructions(opts ...praxis.Option) string
```

This function has zero contact with any frozen `AgentOrchestrator`
type. It walks the option list, extracts skill options, and renders
the fragment deterministically using the same logic `WithSkill`
uses at construction time. It is a debugging utility, not a
composition primitive.

### 3.2 No tool dispatch path in v1.0.0

Skill bundles do NOT declare new tools in frontmatter in the
recognised set (§02 §3.2; research §3 matrix). Any tools an agent
executes while a skill is active are tools the caller already
registered via the orchestrator's existing `tools.Invoker` option
— not tools declared by the bundle.

The skill bundle's role is instruction-only: the Markdown body of
`SKILL.md` tells the LLM how to use tools that already exist in
the caller's invoker configuration. `allowed-tools`, if present, is
a hint about which of those pre-existing tools the skill is
expected to use (and is available to the caller via
`Skill.AllowedTools()` for their own policy checks), but it does
NOT register new tools.

Consequently, there is no tool-dispatch routing layer inside
`praxis/skills`, no tool-name collision detection, and no
`tools.Invoker` parameter on `WithSkill`. These concerns may
return in a future minor version if the ecosystem converges on a
declarative tool-declaration shape in SKILL.md frontmatter.

### 3.3 Zero-modification guarantee

The guarantee is structural: `AgentOrchestrator.Invoke`,
`AgentOrchestrator.InvokeStream`, and `praxis.NewOrchestrator`
signatures are not touched. The frozen interface methods and
constructor receive the same types they have always received. All
skill-related wiring is resolved before the first `Invoke` call
is made.

---

## 4. Tool namespacing (D126 — v1.0.0 no-op)

**D126 v1.0.0 resolution: no namespacing rule is shipped because
skills do not declare tools in frontmatter.**

An earlier draft of this section proposed a
`{skillName}__{toolName}` convention, mirroring Phase 7 D111's
`{LogicalName}__{mcpToolName}` rule. That convention assumed that
SKILL.md frontmatter carried a machine-readable `tools` list,
which the research matrix (§3) shows is not the case in any
surveyed consumer.

The convention described here is **reserved** but not implemented
in v1.0.0. When the ecosystem converges on a declarative tool-
declaration shape, the reserved convention takes effect:

- Rule: `{skillName}__{toolName}` for non-MCP tools;
  `{skillName}__{mcpLogicalName}__{mcpToolName}` for MCP-backed
  tools (composing with Phase 7 D111).
- Delimiter: double underscore (`__`).
- `skillName` must match `^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$` and must
  not contain `__` (mirrors Phase 7 D111 amendment).
- Composed name must satisfy `^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$` and
  fit within the LLM-safe 64-character ceiling.
- Leftmost-split hazard closed by the `__` prohibition in
  `skillName`, same mechanism as Phase 7 D111 amendment.

Until that convergence happens, `praxis/skills` does not
participate in tool namespacing. Callers who pre-register tools
via their own `tools.Invoker` choose their own names.

---

## 5. Multi-skill conflict resolution (D127 draft)

### 5.1 Duplicate-name collision policy

**Default: panic loud at construction time. No silent shadowing.**

When two or more skills are passed to `WithSkill` calls during
orchestrator construction, and they declare the same `name`
(`Skill.Name()`), the `praxis.NewOrchestrator` call **panics** with
a descriptive message identifying both skill bundles by origin
path (when available) before any invocation can start.

The failure mode is a panic rather than an error return because
`praxis.NewOrchestrator` has a frozen single-return signature
(Phase 3). Introducing an error return would amend the frozen
interface, which D125 and §02 §5 explicitly forbid. A panic at
orchestrator-construction time is caught by the caller's process
bootstrap (or their own `defer recover()`), whereas a silent
shadowing would be undetectable until runtime manifest as subtle
behavioural bugs.

Rationale for fail-loud:

1. **Silent shadowing is a correctness hazard.** If skill A and
   skill B both name themselves `"code_reviewer"`, and skill B
   silently wins, debuggers and `Skill.Name()` introspection
   return ambiguous answers.
2. **Collisions are always bugs.** Two distinct skills with the
   same `name` are either (a) the same skill loaded twice by
   mistake or (b) two different skills with a name-space conflict
   the caller should resolve by renaming one.
3. **Enterprise deployments require auditability.** A production
   agent's configuration must be deterministic and fully auditable.
   Allowing silent shadowing would make the effective skill surface
   depend on `WithSkill` call order.

**There is no opt-in to silent shadowing in v1.0.0.** Callers who
need to override a skill's identity wrap the loaded `*Skill` with
their own helper before passing it to `WithSkill`.

### 5.2 Instruction fragment ordering

Instruction fragments from multiple skills are injected in the
exact order the `WithSkill` calls appear in the orchestrator
constructor. The order is deterministic, observable via the
composition helper's debug API, and is not altered by the
framework at any point.

```go
praxis.NewOrchestrator(llmProvider,
    skills.WithSkill(skillA),  // fragment A appears first
    skills.WithSkill(skillB),  // fragment B appears second
)
```

If both skills have non-empty instructions, the composed system
prompt contains both fragments in that order, separated by the
fixed separator defined in §3.1. If a skill's `Instructions()` is
empty, its fragment is omitted from the system prompt (no blank
section is inserted); a `WarnEmptyInstructions` was already emitted
at `Load` time.

### 5.3 Duplicate frontmatter fields across skills

Each skill's frontmatter fields are independent per skill. There is
no cross-skill merging of `license`, `compatibility`, `metadata`,
`allowed-tools`, or `Extensions`. Two skills each carrying a
`metadata` mapping produce two independent `Skill.Metadata()` maps,
accessible from the respective `*Skill` values.

---

## 6. Phase 7 and Phase 8 interaction (D131 draft)

### 6.1 The architectural question

Some skill bundles reference MCP-exposed tools in their instruction
text. The question is: when a caller wants to use such a bundle,
does `praxis/skills` participate in wiring the MCP server, or is
this entirely a caller concern outside the skills sub-module?

This determines whether `praxis/skills` imports `praxis/mcp` in its
`go.mod`. The consequences are:

| Choice | `praxis/skills` go.mod | Caller DX | Module independence |
|---|---|---|---|
| Auto-wire (skills imports mcp) | requires `praxis/mcp` | Ergonomic: one call wires everything | Coupled: skills callers pull mcp transitive deps |
| Explicit caller composition | no `praxis/mcp` dependency | Verbose: caller calls `mcp.New` separately | Decoupled: use skills without mcp |

### 6.2 Recommendation: explicit caller composition

**D131 recommends explicit caller composition. `praxis/skills`
does NOT import `praxis/mcp`.**

Rationale:

1. **Research confirms MCP server dependencies are NOT declared in
   SKILL.md frontmatter.** The research intersection matrix
   (`research-solutions.md §3`) shows that no surveyed consumer
   uses a machine-readable `mcp_servers` field in SKILL.md. Codex
   declares MCP dependencies in a separate `agents/openai.yaml`
   sidecar file that praxis/skills explicitly ignores. Anthropic's
   official bundles express MCP usage in instruction text only.
   There is therefore no machine-readable data for `praxis/skills`
   to hand off; the caller is the authority on which MCP servers
   are configured for the process.

2. **Not all skill bundles use MCP.** A bundle that contributes
   only instructions (the common case observed in the research)
   has no MCP footprint at all. If `praxis/skills` imported
   `praxis/mcp`, every caller who uses skills — even for
   instruction-only bundles — would pull the full MCP SDK
   transitive dependency graph. This contradicts the sub-module
   architecture's core premise: pay only for what you use.

3. **Module boundary clarity.** The Phase 7 D106 framework
   established that MCP support is opt-in. A skills caller who
   does not use MCP should not encounter any MCP import, not even
   transitively through `praxis/skills`.

4. **Auditable trust boundaries.** Opening an MCP session is a
   trust boundary (Phase 5 / Phase 7 D116). Hiding the MCP dial
   inside a skills helper would make the trust-boundary crossing
   invisible to auditors. Explicit caller composition keeps the
   dial visible where the caller can apply policy.

### 6.3 Cross-module handoff shape (when the caller opts in)

When a caller decides to compose skills and MCP together, they do
so entirely in their own code:

```go
// 1. Load the skill bundle. No MCP side effect.
sk, _, err := skills.Load("./bundles/code-reviewer")

// 2. If the caller knows this bundle expects an "eslint" MCP
//    server, they configure it independently via praxis/mcp.
mcpInvoker, err := mcp.New(ctx, []mcp.Server{
    {
        LogicalName: "eslint",
        Transport:   mcp.Stdio("eslint-mcp-server"),
    },
})

// 3. Both go into the orchestrator as independent options.
//    The skill contributes instructions; the mcp invoker handles
//    tool calls. The LLM sees both and glues them together via
//    the skill's instruction text.
orch := praxis.NewOrchestrator(llmProvider,
    skills.WithSkill(sk),
    praxis.WithToolInvoker(mcpInvoker),
)
```

The skill's instruction text can reference MCP-exposed tools by
name (e.g., "use the `eslint__run` tool to lint the file"). The
LLM dispatches to the matching tool through the caller-configured
invoker. `praxis/skills` never touches the MCP transport path.

### 6.4 Forward-compatibility note (post-v1.0.0)

`praxis/skills v1.0.0` does **not** ship a typed `MCPServerSpec`
value, a `Skill.MCPServers()` accessor, or any cross-module
helper. All three are explicit non-goals; see `05-non-goals.md §7`.

If a future agentskills.io spec revision standardises a machine-
readable `mcp_servers` frontmatter field, a later minor version of
`praxis/skills` can introduce a typed accessor (additive,
non-breaking) and an optional sub-package such as
`praxis/skills/mcpwire` that imports both `praxis/skills` and
`praxis/mcp` to provide a translation helper. Until that happens,
v1.0.0 callers read MCP server data — if any — from
`Skill.Extensions()["mcp_servers"]` as an opaque `any`, or (more
commonly) configure their MCP servers entirely outside the skill
loader as shown in §6.3.

---

## 7. Budget, observability, and error translation (D129, D130 drafts)

### 7.1 Budget participation (D129)

Skill-originated tool calls participate in `budget.Guard` via the existing
`wall_clock` and `tool_calls` dimensions, identically to any other
`tools.Invoker` dispatch. This is the D112 rule applied verbatim.

**No new budget dimension.** There is no "skill activation charge", no
"bundle load cost", and no per-skill sub-budget. The existing four
dimensions (`wall_clock`, `tool_calls`, `tokens`, `cost`) are unchanged.

**No double-counting.** The loader runs once at construction time, before
any invocation starts. It incurs no budget-measured cost. Tool calls
dispatched through a skill-wired invoker are counted once, at the
`tools.Invoker.Invoke` boundary. They are not counted separately as
"skill-originated" calls.

**Explicit no-per-skill-sub-budget rule.** praxis does not support budget
limits scoped to a single skill. Callers who need per-skill cost
accounting implement it in their `LifecycleEventEmitter`.

### 7.2 Observability additions (D130)

#### Spans

The composition helper adds **no new span types**. Skill bundles
do not declare tools in v1.0.0 (§3.2), so there is no
"skill-originated" tool call to attribute at the span level.
Tool calls dispatched while a skill is active generate the same
`praxis.toolcall` span (Phase 4 §3.3) as any other tool call,
with whatever name the caller's `tools.Invoker` registered.

No `SkillActivated` or `SkillLoaded` span is added. Skill loading
is a construction-time operation, not an invocation-time operation;
emitting a span (which implies a time interval inside an
invocation) for a purely build-time event would be misleading.

#### Events

No new event types are added to the frozen `LifecycleEventEmitter`
interface. The existing `EventToolCalled` and `EventToolError` events are
sufficient to observe skill-originated tool calls. A skill-augmented
invocation is not distinguishable from a non-skill invocation at the
lifecycle-event level; the distinction is in the tool names themselves.

#### Metrics (cardinality-bounded)

The `praxis/skills` module ships an optional `skills.MetricsRecorder`
interface, following the D115 type-assertion pattern exactly:

```go
// MetricsRecorder is an optional interface that callers may
// implement on their telemetry.MetricsRecorder to receive a
// callback for each skill the orchestrator wires in via
// skills.WithSkill at construction time.
//
// The composition helper detects this interface via type assertion
// on the MetricsRecorder passed to the orchestrator. If the
// assertion succeeds, skill-load callbacks are invoked. If it
// fails, skill callbacks are silently dropped — core orchestrator
// metrics are unaffected.
//
// Contract for skillName: this is the value of Skill.Name() at
// the time of WithSkill composition. The implementation MUST NOT
// use it as a metric label value (skill names are caller-chosen
// and therefore unbounded; using them as labels would violate
// Phase 4 D60). Acceptable uses: span attribute, slog field,
// non-cardinality-bearing trace event, or hashing into a bounded
// label space the caller already maintains. The framework's own
// `praxis_skills_loaded_total` counter exposes only the bounded
// `status` label.
//
// Package: github.com/praxis-go/praxis/skills
type MetricsRecorder interface {
    RecordSkillLoad(skillName, status string)
}
```

| Metric | Type | Labels | Cardinality |
|---|---|---|---|
| `praxis_skills_loaded_total` | Counter | `status` | 2 (`ok`, `error`) |

**`skillName` is not a metric label.** Skill names are
caller-chosen strings and are unbounded (D60 violation risk if
used as labels). Mirroring Phase 7 D115's approach: skill names
may appear in span attributes (bounded per orchestrator instance)
or other non-cardinality-bearing places at the implementation's
discretion, but **never** as a metric label value. The interface
contract documented above makes this rule binding on every
`MetricsRecorder` implementation.

The `status` label (`ok` or `error`) is the only label on
`praxis_skills_loaded_total`. This keeps the cardinality at exactly 2,
well within D60's boundary.

**Why no `praxis_skills_tool_calls_total`.** Skill-originated tool calls
are already counted by the core `praxis_tool_calls_total` metric, which
uses the namespaced tool name as its label. Adding a separate
`praxis_skills_tool_calls_total` would double-count in violation of D129
and add an N-valued label (tool names are unbounded) in violation of D60.

#### Cardinality analysis

Total new time series from Phase 8: 2 (`praxis_skills_loaded_total`
with `status=ok` and `status=error`). This is negligible within the
Phase 4 §6 cardinality budget.

### 7.3 Typed error sub-kinds (Phase 4 taxonomy mapping)

Loader errors are mapped into Phase 4 `errors.TypedError` kinds as
documented in §1.5 and the §2.4 failure-mode table. No amendment to
the `errors` package's exported sub-kind constants is required;
`SkillSubKind` is a new named type (§1.5) defined in the
`praxis/skills` sub-module.

Duplicate-skill-name collisions during `WithSkill` composition are
handled via **panic at construction time** (§5.1), not via a typed
error, to preserve the frozen `praxis.NewOrchestrator` signature.

---

## 8. Open questions resolution

All OPEN tags raised during the draft phase have been resolved. The
resolutions are recorded alongside the decisions they support.

| Tag | Element | Resolution |
|---|---|---|
| OPEN-01 | D122 reversal conditions | **Resolved** (`02-scope-and-positioning.md §1`). Solution-researcher verdict: GO (sub-module). Neither PIVOT nor DEFER applies. |
| OPEN-02 | YAML parser choice | **Partially resolved** (`02-scope-and-positioning.md §2.1`). Primary candidate: `gopkg.in/yaml.v3` (BSD-3-Clause). Fallback: `go.yaml.in/yaml/v3`. Final govulncheck gate deferred to implementation phase. |
| OPEN-03 | D123 field set | **Resolved** (`02-scope-and-positioning.md §3`). Field set anchored to research §3 matrix: required (`name`, `description`) + optional (`license`, `compatibility`, `metadata`, `allowed-tools`). All other fields via permissive-preserve into `Extensions()`. |
| OPEN-04 | `resources` field inclusion | **Resolved** (`02-scope-and-positioning.md §3 status`). Dropped from the recognised set. The three-level directory layout (`scripts/`, `references/`, `assets/`) is universal, but `resources` as a frontmatter list is not a spec field. |
| OPEN-05 | Composed system-prompt debug accessor | **Resolved by relocation.** The debug accessor is a method on the internal composition state held by `skills.WithSkill` (not on the frozen orchestrator type) and is exposed via a helper function `skills.ComposedInstructions(opts ...praxis.Option) string` that takes the same options the caller would pass to `NewOrchestrator` and returns the composed fragment without building an orchestrator. Zero touch on frozen interfaces. |

---

## 9. Decisions summary

| ID | Subject | Outcome |
|---|---|---|
| D124 | Loader surface | `Open(fsys fs.FS, root string) (*Skill, []SkillWarning, error)` primary; `Load(path string)` convenience wrapper; opaque `Skill` struct with read-only accessors; `LoadError` implementing the full `errors.TypedError` interface via explicit method stubs; `SkillSubKind` as a named type |
| D125 | Composition surface | `skills.WithSkill(s *Skill) praxis.Option`; build-time only; **panics** on duplicate-name collision to preserve the frozen `praxis.NewOrchestrator` single-return signature; no tools.Invoker parameter (skills do not declare new tools in frontmatter) |
| D126 | Tool namespacing | v1.0.0 no-op. Reserved convention (`{skillName}__{toolName}`) documented for future ecosystem convergence on a declarative `tools` frontmatter field. |
| D127 | Multi-skill conflict | Fail loud (panic) at construction time on duplicate `Skill.Name()`; deterministic instruction ordering by `WithSkill` call order; no merging of per-skill fields |
| D128 | Instruction injection | Build-time system-prompt fragment concatenation; no frozen `LLMRequest` field added; observable via the `skills.ComposedInstructions` helper |
| D129 | Budget participation | Re-use Phase 7 D112 verbatim; no new dimension; no per-skill sub-budget; no double-counting; loader runs at construction time incurring no budget cost |
| D130 | Observability | No new event types; no new spans; one bounded counter metric (`praxis_skills_loaded_total`) with a 2-valued `status` label; skill names are NEVER used as metric labels (mirrors Phase 7 D115) |
| D131 | Phase 7 ↔ Phase 8 | Explicit caller composition; `praxis/skills` does NOT import `praxis/mcp`; MCP server dependencies are not declared in SKILL.md frontmatter by any surveyed consumer; when a caller opts in, they compose both sub-modules independently in their own code |

Full decision rationale in `01-decisions-log.md` when adopted.

---

**Next:** `04-dx-and-errors.md` covers the consumer declaration surface,
typed error catalogue, worked example, and documentation story.
