# Phase 8 — DX and Errors

**Decisions:** D132 (draft)
**Cross-references:** Phase 1 D05 (error model), Phase 4 D61 (typed
error taxonomy), Phase 7 D113 (error translation pattern),
Phase 8 D123 (permissive-preserve), D124 (loader),
D125 (composition), D127 (collision), D131 (cross-module handoff).

This document specifies the developer experience of `praxis/skills`:
the consumer declaration surface, the typed error catalogue, the
worked example wiring a bundle into an orchestrator end-to-end,
and the terminology cross-reference with Claude Code's
`.claude/skills/` planning harness.

---

## 1. Consumer declaration surface

A praxis consumer who wants to load and use a skill bundle goes
through three explicit steps:

1. **Call the loader.** `skills.Load(path)` or `skills.Open(fsys, root)`
   returns a `*Skill`, a `[]SkillWarning`, and an `error`.
2. **Inspect warnings.** Warnings are non-fatal diagnostics. The
   caller decides whether to log, reject, or act on extension
   fields surfaced in `Skill.Extensions()`.
3. **Wire the skill into the orchestrator.** Pass `*Skill` together
   with a `tools.Invoker` to `skills.WithSkill(...)`, which is
   applied as an orchestrator option at construction time.

There are no intermediate steps. There is no global registry, no
hidden side effect, and no network call anywhere on the path.

### 1.1 Minimum-viable wiring

```go
import (
    "context"
    "log"

    "github.com/praxis-go/praxis"
    "github.com/praxis-go/praxis/skills"
)

func main() {
    // 1. Load the bundle from a caller-controlled path.
    sk, warnings, err := skills.Load("./bundles/frontend-design")
    if err != nil {
        log.Fatalf("skill load failed: %v", err)
    }

    // 2. Inspect any warnings. Minimal wiring just logs them.
    for _, w := range warnings {
        log.Printf("skill warning: kind=%s field=%s msg=%s",
            w.Kind, w.Field, w.Message)
    }

    // 3. Build the orchestrator with the skill wired in.
    //    The skill contributes only instructions (the bundle's
    //    SKILL.md body). Tools the instructions refer to are
    //    registered through the caller's own tools.Invoker via
    //    the existing praxis.WithToolInvoker option — not via
    //    WithSkill, which takes no invoker parameter.
    orch := praxis.NewOrchestrator(
        llmProvider,
        skills.WithSkill(sk),
        praxis.WithToolInvoker(localInvoker),
    )

    // 4. Invoke as usual. The skill's instructions are already in
    //    the system prompt; the tools it references are routed
    //    through localInvoker like any other tool; nothing changes
    //    at invocation time.
    _, err = orch.Invoke(context.Background(), userMessage)
    if err != nil {
        log.Fatalf("invocation failed: %v", err)
    }
}
```

### 1.2 Strict-mode wiring

Callers who want to reject any bundle carrying non-spec extension
fields compose strict validation on top of the permissive-preserve
default:

```go
sk, warnings, err := skills.Load("./bundles/some-bundle")
if err != nil {
    return err
}
if len(warnings) > 0 {
    return fmt.Errorf(
        "strict mode: bundle %q carries %d non-spec field(s): %v",
        sk.Name(), len(warnings), warnings,
    )
}
```

This is trivially composable because permissive-preserve is the
default. The inverse (building permissive on top of strict) is not
expressible without re-implementing the parser.

### 1.3 Reading extension fields for caller policy

The permissive-preserve policy lets callers apply their own policy
to fields that praxis does not recognise. The extension bag is
`map[string]any` (D123 §3.4).

```go
sk, warnings, _ := skills.Load("./bundles/antigravity-frontend-design")

// The Antigravity aggregator adds `risk` as a registry metadata field.
// praxis does not interpret it; the caller does.
if risk, ok := sk.Extensions()["risk"].(string); ok {
    if risk == "unknown" || risk == "high" {
        return fmt.Errorf(
            "caller policy: refusing to load bundle %q with risk=%s",
            sk.Name(), risk,
        )
    }
}
```

The type assertion is idiomatic Go: the caller knows which
extension fields they care about and asserts the expected type.
Unknown-type or absent fields are safe because the type assertion's
ok-return catches both cases.

### 1.4 Wiring a skill whose instructions reference MCP tools (D131)

SKILL.md frontmatter does not carry a machine-readable MCP server
list in any surveyed consumer (see [research-solutions.md §3](research-solutions.md)
and `05-non-goals.md §7`). If a bundle's instruction text expects
MCP-exposed tools, the caller composes `praxis/skills` and
`praxis/mcp` **explicitly and independently**. `praxis/skills` does
not import `praxis/mcp`; the two sub-modules never touch each
other at the package level.

```go
import (
    "context"

    "github.com/praxis-go/praxis"
    "github.com/praxis-go/praxis/mcp"
    "github.com/praxis-go/praxis/skills"
)

func build(ctx context.Context, llm praxis.LLMProvider, bundlePath string) *praxis.Orchestrator {
    // 1. Load the skill bundle. No MCP side effect.
    sk, _, err := skills.Load(bundlePath)
    if err != nil {
        panic(err)
    }

    // 2. The caller knows this bundle's instructions reference an
    //    "eslint" MCP server. Configure it independently via
    //    praxis/mcp. The trust-boundary crossing is visible here.
    mcpInvoker, err := mcp.New(ctx, []mcp.Server{
        {
            LogicalName: "eslint",
            Transport:   mcp.Stdio("eslint-mcp-server"),
        },
    })
    if err != nil {
        panic(err)
    }

    // 3. Both flow into the orchestrator as independent options.
    //    The skill contributes instructions; the mcp invoker
    //    handles tool calls. The LLM glues them together via the
    //    skill's instruction text.
    return praxis.NewOrchestrator(
        llm,
        skills.WithSkill(sk),
        praxis.WithToolInvoker(mcpInvoker),
    )
}
```

**Why the caller owns the wiring.** If `praxis/skills` auto-wired
MCP, every skills consumer — including those whose bundles do not
use MCP — would pull the full MCP SDK transitive dependency graph.
Explicit composition costs the caller a handful of lines per
application bootstrap and keeps the trust-boundary dial auditable
in one place. This is the right trade-off for a security-critical
path. See `03-integration-model.md §6`.

### 1.5 Wiring multiple skills

Multiple skills compose through multiple `WithSkill` calls. Order
is preserved: instructions are concatenated in the order
`WithSkill` is invoked (D127 §5.2). If two bundles have the same
`Skill.Name()`, `praxis.NewOrchestrator` **panics** at construction
time (D127 §5.1) — not returns an error, because the frozen
`NewOrchestrator` signature has no error return.

```go
orch := praxis.NewOrchestrator(
    llmProvider,
    skills.WithSkill(frontendBundle),
    skills.WithSkill(testingBundle),
    skills.WithSkill(docsBundle),
)
// If any two bundles share the same Skill.Name(),
// praxis.NewOrchestrator panics with a message identifying both
// bundles. A panic at construction time is caught by the caller's
// process bootstrap or defer recover(); it never reaches runtime.
```

---

## 2. Typed error catalogue

All loader and composition errors implement `errors.TypedError`
(Phase 4 D61) and carry a stable `SubKind` string so callers can
branch on specific failure modes without parsing messages.

### 2.1 Loader errors (`skills.LoadError`)

Returned by `skills.Load` and `skills.Open`. See [03-integration-model.md §1.5](03-integration-model.md) for the type declaration.

| SubKind | When | TypedError Kind | Caller action |
|---|---|---|---|
| `skill_bundle_missing` | `SKILL.md` file not found; bundle root does not exist; bundle root not readable | `ErrorKindSystem` | Confirm path; treat as configuration error |
| `skill_bundle_malformed_yaml` | YAML fails to parse; YAML anchor bomb detected; YAML alias cycle | `ErrorKindTool` | Fix the bundle; do not retry |
| `skill_bundle_invalid_field` | Required field missing / empty (`name`, `description`); `name` regex violation; `metadata` not a mapping; `allowed-tools` not a sequence of strings; `license` / `compatibility` not a string; file size > 256 KiB | `ErrorKindTool` | Fix the bundle; do not retry |
| `skill_bundle_path_escape` | A `..` component in `root`; any attempt to read outside the `fs.FS` rooted at the bundle | `ErrorKindSystem` | Security signal; log at warn+ level; do not retry |

All loader errors are non-retryable. They are configuration errors,
not transient I/O failures.

Duplicate-name collisions during `skills.WithSkill` composition do
NOT appear in this table. They panic at orchestrator construction
time (see `03-integration-model.md §1.5` and §5.1), preserving the
frozen `praxis.NewOrchestrator` single-return signature.

### 2.2 Error message conventions

Loader error messages follow the format produced by
`LoadError.Error()`:

```
skills: <subkind>: bundle=<path>: <underlying cause>
```

(the `: <underlying cause>` suffix is omitted when `LoadError.Cause`
is nil).

Examples:

- `skills: skill_bundle_missing: bundle=./missing: stat ./missing: no such file or directory`
- `skills: skill_bundle_malformed_yaml: bundle=./broken: yaml: line 3: mapping values are not allowed in this context`
- `skills: skill_bundle_invalid_field: bundle=./no-name: name: required field missing`
- `skills: skill_bundle_path_escape: bundle=./escape: path "../../etc/passwd" escapes bundle root`

The `<subkind>` portion is stable and can be matched programmatically
via `errors.As` against `*skills.LoadError` followed by a
`SkillSubKind`-typed switch (see §2.4). The human-readable tail is
informational and may evolve across patch releases.

### 2.3 Warnings are not errors

`SkillWarning` is a distinct concept from `LoadError`. Warnings are
returned alongside a successfully loaded `*Skill`; errors prevent
loading entirely. The warning kinds
(`WarnExtensionField`, `WarnEmptyInstructions`) all leave the
`Skill` in a usable state.

The strict-mode composition pattern (§1.2) lets callers promote
any warning to an error without changing the loader's semantics.
This is the explicit design: warnings are signals, errors are
verdicts.

### 2.4 Error wrapping and `errors.Is` / `errors.As`

`LoadError.Unwrap()` returns the underlying cause (if any), so
standard error wrapping works:

```go
sk, _, err := skills.Load(path)
if err != nil {
    var loadErr *skills.LoadError
    if errors.As(err, &loadErr) {
        switch loadErr.SubKind {
        case skills.SkillSubKindPathEscape:
            securityLog.Warn("path escape attempt", "path", path)
            return fmt.Errorf("bundle rejected: %w", err)
        case skills.SkillSubKindMalformedYAML:
            return fmt.Errorf("please fix the bundle's YAML: %w", err)
        default:
            return err
        }
    }
    return err
}
```

---

## 3. Worked example: end-to-end

A complete, runnable shape showing every step from bundle layout
to invocation.

### 3.1 Bundle on disk

```
./bundles/code-reviewer/
├── SKILL.md
└── references/
    └── review-checklist.md
```

`./bundles/code-reviewer/SKILL.md`:

```markdown
---
name: code-reviewer
description: |
  Reviews staged changes for correctness, test coverage, and style.
  Should be invoked before creating a pull request.
license: Apache-2.0
metadata:
  author: team-platform
  maintainer: platform@example.com
---

You are a code reviewer. When invoked, read the currently staged
changes and produce a review organised by severity:

1. **Blocker** — bugs, regressions, security issues.
2. **Important** — missing tests, missing documentation,
   inconsistent style.
3. **Nitpick** — cosmetic suggestions.

Refer to `${SKILL_DIR}/references/review-checklist.md` for the
full checklist. Always end with a one-line "READY / BLOCK" verdict.
```

### 3.2 Caller code

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/praxis-go/praxis"
    "github.com/praxis-go/praxis/llm/anthropic"
    "github.com/praxis-go/praxis/skills"
    "github.com/praxis-go/praxis/tools"
)

func main() {
    ctx := context.Background()

    // 1. Construct an LLM provider (frozen Phase 3 surface).
    llm, err := anthropic.New(
        anthropic.WithAPIKey(os.Getenv("ANTHROPIC_API_KEY")),
        anthropic.WithModel("claude-sonnet-4-6"),
    )
    if err != nil {
        log.Fatal(err)
    }

    // 2. Load the skill bundle. This is a single filesystem read.
    //    No network, no script execution.
    sk, warnings, err := skills.Load("./bundles/code-reviewer")
    if err != nil {
        log.Fatalf("skill load: %v", err)
    }
    for _, w := range warnings {
        log.Printf("skill warning: %s", w.Message)
    }

    // 3. Read the recognised metadata field via its typed accessor.
    //    `metadata` is an agentskills.io-spec optional field (§02 §3.2),
    //    so it goes to Skill.Metadata(), not Extensions().
    if md := sk.Metadata(); md != nil {
        if maintainer, ok := md["maintainer"].(string); ok {
            log.Printf("skill maintainer: %s", maintainer)
        }
    }

    // 4. This bundle's instructions do not reference any tool, so
    //    tools.NullInvoker (the Phase 3 default) is sufficient.
    //    If the bundle referenced tools, the caller would configure
    //    them via praxis.WithToolInvoker here.

    // 5. Wire the orchestrator with the skill composed in. The
    //    Phase 3 NewOrchestrator signature is single-return.
    orch := praxis.NewOrchestrator(
        llm,
        skills.WithSkill(sk),
        // praxis.WithToolInvoker(tools.NullInvoker) is the default.
    )

    // 6. Invoke. The skill's instructions are already in the
    //    system prompt; the LLM has context for the task.
    resp, err := orch.Invoke(ctx, "review the staged changes")
    if err != nil {
        log.Fatalf("invoke: %v", err)
    }
    fmt.Println(resp.Content)
}
```

### 3.3 What happens under the hood

- `skills.Load` reads `SKILL.md` via `os.DirFS("./bundles/code-reviewer")`,
  parses the frontmatter, validates required fields (`name`,
  `description`), stores the instruction body, and returns a
  `*Skill` carrying the typed accessors for recognised fields
  (`License()`, `Metadata()`, etc.) and an empty `Extensions()`
  map (no non-spec fields were present in this bundle).
- `Skill.License()` returns `"Apache-2.0"` from the frontmatter.
- `Skill.Metadata()` returns `map[string]any{"author": "team-platform",
  "maintainer": "platform@example.com"}` — the full recognised
  `metadata` mapping. It does not appear in `Extensions()` because
  `metadata` is a recognised optional spec field (§02 §3.2).
- `skills.WithSkill` prepends the instruction body to the orchestrator's
  system prompt fragment list (D128). The fragment includes a
  deterministic separator for debuggability.
- At `orch.Invoke` time, the composed system prompt is sent to the
  LLM verbatim, including any literal `${SKILL_DIR}` tokens in the
  instruction text. **praxis does not perform template-variable
  substitution** — the LLM sees `${SKILL_DIR}` as literal text, and
  if the instructions depend on substitution, the caller rewrites
  the instruction text before wrapping it in a `Skill` or performs
  substitution in their own `tools.Invoker` when the LLM tool-calls
  for a file read.
- The trust boundary for script execution stays at the `tools.Invoker`
  seam. This bundle's instructions can reference files under
  `${SKILL_DIR}/references/`, but any actual file read is mediated
  by whatever read tool the caller's invoker exposes — not by
  praxis auto-loading files from the bundle.

---

## 4. Terminology cross-reference with `.claude/skills/`

The Claude Code planning harness at `.claude/skills/` and the
`praxis/skills` sub-module use the **same concept**, not different
ones. Both refer to filesystem directories containing a `SKILL.md`
descriptor following the agentskills.io convention. praxis
documentation treats the two as cross-references, not as
disambiguation targets.

### 4.1 What the Claude Code harness is

The `.claude/skills/` directory at the root of this repository
contains three skills used by Claude Code when working on the
praxis project itself:

- `plan-phase` — structures a design phase into the planning
  artifact template.
- `review-phase` — critically reviews a phase output.
- `roadmap-status` — produces the current phase status table.

These are authoring-time artifacts for the praxis project
maintainers. They have no runtime dependency on the `praxis/skills`
sub-module. A consumer of `praxis/skills` never interacts with
`.claude/skills/` unless they happen to also be a Claude Code user
contributing to praxis development.

### 4.2 Relationship to `praxis/skills`

The `praxis/skills` sub-module is a **runtime library** that loads
the same kind of bundle. If a consumer wanted to load the
`plan-phase` skill into their own praxis-based agent (an unusual
but valid case), they would use `skills.Load(".claude/skills/plan-phase")`
and receive a `*Skill` with the harness's instructions — praxis
does not care about the directory the bundle lives under, only
that a `SKILL.md` is present at the root.

### 4.3 Documentation guidance

In the praxis godoc and README, references to "skills" should:

1. Assume the reader knows what a `SKILL.md` bundle is (or link to
   `agentskills.io` on first mention).
2. Cross-reference Claude Code's `.claude/skills/` as an example
   of a real-world consumer that uses the same convention, **not**
   as a distinct concept that needs disambiguation.
3. Avoid framing the two as potentially confusable. They use the
   same format and the same mental model.

The pre-existing "terminology collision" concern from the early
Phase 8 scaffold (00-plan.md R5) dissolves once the concept is
recognised as a single ecosystem-wide convention rather than two
homonymous things.

---

## 5. Discovery, documentation, and examples

### 5.1 README and godoc

The top-level `README.md` of the praxis repository gets a short
"Skills" section pointing at the `github.com/praxis-go/praxis/skills`
sub-module godoc, mirroring the treatment of `praxis/mcp` after
Phase 7.

The `skills` package godoc opens with a one-paragraph explanation
of what a skill bundle is (with an `agentskills.io` link), followed
by the worked example from §3. The examples directory in the
sub-module ships a runnable version of §3 that points at a
bundled test fixture under `skills/testdata/`.

### 5.2 Doc examples for the stability policy

The `praxis/skills` godoc explicitly labels the module's stability
tier (D134 §4): `stable-v0.x-candidate` at the core v1.0.0 release,
with its own independent semver line. Callers pinning `praxis/skills`
see the tier in the first paragraph of the package doc, so there is
no surprise about signature mobility during v0.x.

### 5.3 No mandatory reading list

The worked example in §3 is the canonical entry point. Callers who
only want minimum-viable wiring need not read beyond §1.1. Callers
who want the full trust-boundary story are pointed to the Phase 8
non-goals catalogue (`05-non-goals.md`) and the Phase 7 security
appendix (`docs/phase-7-mcp-integration/04-security-and-credentials.md`).

---

## 6. D132 decision summary

| Element | Decision |
|---|---|
| Loader return shape | `(*Skill, []SkillWarning, error)` |
| Error type | `*skills.LoadError` implementing `errors.TypedError` (`Kind`, `HTTPStatusCode`, `Unwrap`, `Error`) |
| `SubKind` type | `SkillSubKind string` named type (mirrors Phase 3 `ToolSubKind`) |
| SubKinds | `SkillSubKindMissing`, `SkillSubKindMalformedYAML`, `SkillSubKindInvalidField`, `SkillSubKindPathEscape` |
| Error message format | `skills: <subkind>: bundle=<path>: <cause>` (cause suffix omitted when nil) |
| Warning kinds | `WarnExtensionField`, `WarnEmptyInstructions` |
| Composition entry point | `skills.WithSkill(s *Skill) praxis.Option` — no invoker parameter; panics at `praxis.NewOrchestrator` time on duplicate-name collision |
| Orchestrator constructor | `praxis.NewOrchestrator(provider, opts...) *Orchestrator` (frozen; unchanged; single-return) |
| MCP handoff style | Explicit caller composition (D131); `praxis/skills` does not import `praxis/mcp`; SKILL.md frontmatter does not carry machine-readable MCP server data in any surveyed consumer |
| Template-variable substitution | None. Instruction text is forwarded verbatim; `${SKILL_DIR}` and similar tokens are literal strings |
| Worked example location | §3 of this document + runnable form in `skills/examples/` |
| Terminology posture | Cross-reference with `.claude/skills/`, not disambiguation |

No open items in D132. The decision is ready for the reviewer pass.
