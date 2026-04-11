# Phase 8 — Research: SKILL.md Ecosystem Survey

> **Scope.** Pre-design scan for Phase 8 "Skills Integration". Surveys
> the Agent Skills / SKILL.md ecosystem as of 2026-Q2, documents the
> canonical frontmatter field set, inspects real bundles from the
> wild, records loader mechanics for two major consumers, documents
> prior art in Go / Python / Java orchestration libraries, confirms
> the spec/RFC status, reports the supply-chain security posture of
> existing implementations, and gives a verdict on the working
> hypothesis (GO / PIVOT / DEFER).
>
> **Method.** All claims are sourced from public URLs fetched on
> 2026-04-10. Unverified claims are marked `[unverified]`.

---

## 1. Need

praxis needs to decide whether and how to support installable skill
bundles — directories shaped like the `SKILL.md` convention — before
the v1.0.0 freeze. Without a sourced survey, D122 (first-class
loader, convention-only, or non-goal) and D123 (frontmatter field
intersection) are vibes-based and would fail reviewer challenge on
R1 (format fragmentation) and R6 (provider-specific leakage). This
research provides the field-level evidence needed to anchor both
decisions.

---

## 2. Ecosystem Survey

### 2.1 Agent Skills Open Standard (`agentskills.io`)

**What it is.** The Agent Skills format was developed by Anthropic,
released as an open standard in December 2025, and adopted by a
large number of agent products. The canonical specification lives at
`https://agentskills.io/specification`. The governance repository is
`https://github.com/agentskills/agentskills` (created
December 16, 2025).

**Adopters as of 2026-Q2 (verified via agentskills.io overview
page).** 30+ named tools including: Claude Code, OpenAI Codex,
Gemini CLI, GitHub Copilot, VS Code, Cursor, Amp, Roo Code, JetBrains
Junie, OpenHands, OpenCode, Letta, Goose (Block), Spring AI, Emdash,
Firebender, Snowflake Cortex Code, Factory, Mux (Coder), TRAE
(ByteDance), Qodo, Kiro, Databricks Genie Code, Mistral Vibe,
VT Code, Laravel Boost, Autohand Code CLI, Agentman, Ona, pi,
Command Code.

**Open development status.** No formal RFC process, no ISO/IETF
governance track, no versioned spec release (no "v1.0" tag on the
spec). The spec evolves via pull request on GitHub and is open to
community contribution via Discord. The spec is maintained as an
evolving living document.

**Key conclusion.** This is an already-ratified open standard with
30+ production adopters — not a draft RFC in active ratification.
The reversal condition for DEFER does not apply.

Sources: `https://agentskills.io`,
`https://agentskills.io/specification`,
`https://github.com/agentskills/agentskills`

---

### 2.2 Anthropic Official Skills Repository (`anthropics/skills`)

**Repository.** `https://github.com/anthropics/skills`. Contains 17
bundled skills under `skills/` as of 2026-Q2. The
`spec/agent-skills-spec.md` file contains only a redirect to
`agentskills.io/specification` (87 bytes; the spec was moved
externally in December 2025).

**Licensing.** Most bundled skills are Apache 2.0. Document-format
skills (`docx`, `pdf`, `pptx`, `xlsx`) are source-available with a
separate license.

**Frontmatter observed in real bundles (raw fetches verified):**

`skills/mcp-builder/SKILL.md`:
```yaml
name: mcp-builder
description: Guide for creating high-quality MCP (Model Context Protocol) servers...
license: Complete terms in LICENSE.txt
```

`skills/algorithmic-art/SKILL.md`:
```yaml
name: algorithmic-art
description: Creating algorithmic art using p5.js with seeded randomness...
license: Complete terms in LICENSE.txt
```

`skills/frontend-design/SKILL.md`:
```yaml
name: frontend-design
description: Create distinctive, production-grade frontend interfaces with high design quality.
license: Complete terms in LICENSE.txt
```

`skills/webapp-testing/SKILL.md`:
```yaml
name: webapp-testing
description: Toolkit for interacting with and testing local web applications using Playwright...
license: Complete terms in LICENSE.txt
```

`skills/skill-creator/SKILL.md`: `name`, `description` confirmed.
No additional frontmatter fields.

**Observation.** All 4 inspected official Anthropic bundles use only
`name`, `description`, and `license`. No bundle in the official
repository uses `compatibility`, `metadata`, `allowed-tools`,
`version`, or any MCP server declaration in frontmatter. MCP usage
is expressed in the SKILL.md body as human-readable instructions —
not as machine-parseable frontmatter.

Sources: `https://github.com/anthropics/skills`, raw file fetches at
`https://raw.githubusercontent.com/anthropics/skills/main/skills/{name}/SKILL.md`

---

### 2.3 Claude Code (`.claude/skills/` convention)

**Disk layout.** Claude Code loads skills from four scopes:

| Scope | Path |
|---|---|
| Enterprise | Managed settings path |
| Personal | `~/.claude/skills/<skill-name>/SKILL.md` |
| Project | `.claude/skills/<skill-name>/SKILL.md` |
| Plugin | `<plugin>/skills/<skill-name>/SKILL.md` |

Cross-client interoperability path `.agents/skills/` is also
scanned. Nested `.claude/skills/` directories inside subdirectories
are auto-discovered for monorepo support. `--add-dir` directories
have their `.claude/skills/` loaded automatically.

**Bundle directory structure:**
```
my-skill/
├── SKILL.md           # Required entrypoint
├── scripts/           # Optional executable code
├── references/        # Optional documentation
└── assets/            # Optional templates/resources
```

**Frontmatter fields recognised by Claude Code (complete list from
official docs):**

| Field | Required? |
|---|---|
| `name` | No (uses directory name if absent) |
| `description` | Recommended (uses first paragraph of body if absent) |
| `argument-hint` | No |
| `disable-model-invocation` | No |
| `user-invocable` | No |
| `allowed-tools` | No |
| `model` | No |
| `effort` | No |
| `context` | No (`fork` = subagent) |
| `agent` | No |
| `hooks` | No |
| `paths` | No |
| `shell` | No |

**Loading mechanics.** At session start, Claude Code scans all
active skill directories, parses YAML frontmatter of each SKILL.md,
and builds a catalog (name + description, ~50–100 tokens per
skill). The catalog enters the context. When the model or user
invokes a skill, the rendered SKILL.md body (after `$ARGUMENTS`,
`${CLAUDE_SESSION_ID}`, `${CLAUDE_SKILL_DIR}` substitution) enters
the conversation as a single message and persists for the session.
Claude Code does not re-read the SKILL.md on subsequent turns. Upon
compaction, skills are re-attached within a 25,000-token combined
budget (up to 5,000 tokens per skill, most-recent-first).

**Tool exposure.** `allowed-tools` grants pre-approval for existing
tools (Bash, Read, etc.) when the skill is active. It does not
declare new tools. No new tools are introduced via SKILL.md
frontmatter.

**MCP wiring.** MCP servers are not declared in SKILL.md frontmatter.
A skill's body may instruct the model to use MCP-exposed tools, but
the SKILL.md format carries no machine-parseable `mcp_servers`
field. The MCP connection is a session-level concern, not a
bundle-level concern.

**Supply-chain protections.** As of v2.1.38, writes to
`.claude/skills` are blocked in sandbox mode.
`disableSkillShellExecution: true` disables inline `!cmd`
substitution org-wide. Path traversal is prevented by the sandbox
and `containsPathTraversal` validation.

**Validator fragmentation note.** Several GitHub issues report that
Claude Code's built-in SKILL.md validator rejects valid
agentskills.io spec fields (`allowed-tools` reported as unsupported
despite being valid per docs; extension fields rejected). The
runtime still loads such skills, but validator errors create
friction. This is an ongoing reconciliation issue between the
agentskills.io spec and Claude Code's extension-heavy
implementation.

Sources: `https://code.claude.com/docs/en/skills`,
`https://github.com/anthropics/claude-code/issues/26795`,
`https://github.com/microsoft/vscode/issues/294520`,
`https://deepwiki.com/ai-ml-architect/claude-code/4.3-sandbox-and-path-validation`

---

### 2.4 OpenAI Codex

**Disk layout.** Codex scans:

| Scope | Path |
|---|---|
| REPO (local) | `.agents/skills` (current directory) |
| REPO (root) | `$REPO_ROOT/.agents/skills` |
| USER | `$HOME/.agents/skills` |
| ADMIN | `/etc/codex/skills` |
| SYSTEM | Bundled with Codex |

**Frontmatter fields.** Codex uses only `name` and `description`
from SKILL.md frontmatter. These are documented as "the only fields
that Codex reads to determine when the skill gets used." Tool and
MCP dependencies are declared in a separate `agents/openai.yaml`
sidecar file (not part of SKILL.md), carrying `display_name`,
`short_description`, `default_prompt`, and tool dependency
declarations (including MCP tool references via `type: mcp`).

**Key deviation.** Codex explicitly separates triggering metadata
(SKILL.md: `name`, `description`) from UI and dependency metadata
(`agents/openai.yaml`). This is a notable deviation from the
agentskills.io spec, which uses SKILL.md frontmatter for all
metadata. Codex does not put MCP declarations into SKILL.md
frontmatter.

**Loading mechanics.** Progressive disclosure: metadata only at
startup, full SKILL.md body loaded when the skill is selected. The
model's decision is based on description matching.

Sources: `https://developers.openai.com/codex/skills`,
`https://github.com/openai/skills/blob/main/skills/.system/skill-creator/SKILL.md`

---

### 2.5 Gemini CLI

**Disk layout.** Gemini CLI scans `.gemini/skills/`,
`~/.gemini/skills/`, and the cross-client `.agents/skills/` paths
in the same tiers. Extension-bundled skills are also discovered.

**Frontmatter fields.** Gemini CLI reads `name` and `description`.
No other frontmatter fields are documented as recognised.

**Loading mechanics.** Progressive disclosure. The model calls
`activate_skill(name)`. After user approval, the SKILL.md body and
folder structure are added to the conversation history. The skill's
directory is added to the agent's allowed file paths, granting it
permission to read bundled assets.

Source: `https://geminicli.com/docs/cli/skills/`

---

### 2.6 Google ADK (Python — prior art for the typed `Skill` value pattern)

**Skills support.** Google ADK Python (v1.25.0) has a first-class
`SkillToolset` and `load_skill_from_dir(path: Path) -> Skill` API.
The `SkillToolset` auto-generates three tools: `list_skills`,
`load_skill(name)`, and `load_skill_resource(skill_name, path)` —
mirroring the three tiers of progressive disclosure.

**Go ADK status.** The Go ADK (`google.golang.org/adk`) does **not**
have a `load_skill_from_dir` equivalent. The Go ADK takes a
code-first approach; tools are registered programmatically. No
skill-bundle loader exists in the Go package as of research date.
This is a confirmed gap.

Sources: `https://adk.dev/skills/`,
`https://pkg.go.dev/google.golang.org/adk`

---

### 2.7 Community Registries

**SkillsMP** (`skillsmp.com`). Largest community registry as of
2026-Q2. A community-run aggregator that crawls GitHub for SKILL.md
files. Not affiliated with Anthropic. All indexed skills follow the
agentskills.io standard. The registry reported 351,000+ indexed
skills as of March 2026 (direct fetch returned HTTP 403 — size data
sourced from secondary search results, `[unverified]` for exact
current count).

**Antigravity Awesome Skills**
(`github.com/sickn33/antigravity-awesome-skills`). 1,397+ skills for
Claude Code, Cursor, Codex, Gemini CLI. Installer:
`npx antigravity-awesome-skills`. Supports `--category`, `--risk`,
`--tags` filters (from a separate `skills_index.json`, not SKILL.md
frontmatter).

**Community bundle frontmatter observed (raw fetch):**

`sickn33/antigravity-awesome-skills/skills/frontend-design/SKILL.md`:
```yaml
name: frontend-design
description: "You are a frontend designer-engineer, not a layout generator."
risk: unknown
source: community
date_added: "2026-02-27"
```

**Observation.** This bundle carries non-spec fields (`risk`,
`source`, `date_added`) added by the aggregator as registry metadata
— not by the skill author. A lenient parser correctly ignores these.
A strict parser would reject this bundle. The agentskills.io client
guide explicitly recommends lenient parsing (warn on unknown fields,
do not reject).

Sources: `https://github.com/sickn33/antigravity-awesome-skills`,
raw fetch of `skills/frontend-design/SKILL.md`

---

### 2.8 Claude Code Plugin Skills (unofficial `version` field)

Claude Code plugin bundles use an extra `version` frontmatter field
not in the agentskills.io spec. Observed in
`anthropics/claude-code/plugins/plugin-dev/skills/skill-development/SKILL.md`:

```yaml
name: Skill Development
description: This skill should be used when the user wants to "create a skill"...
version: 0.1.0
```

The `version` field is a Claude Code plugin convention, not a
spec-mandated field. It appears only in plugin bundles and is not
used by any other surveyed consumer.

Source: `https://github.com/anthropics/claude-code/blob/main/plugins/plugin-dev/skills/skill-development/SKILL.md`

---

## 3. Frontmatter Field Matrix

Rows = field names. Columns = consumers. Based on real bundles
observed and official documentation, not documentation promises.

| Field | agentskills.io spec | Anthropic skills repo (4 bundles) | Claude Code | OpenAI Codex | Gemini CLI | Google ADK Python | LangChain DeepAgents | Community bundles (observed) |
|---|---|---|---|---|---|---|---|---|
| `name` | **required** | required | optional (dir fallback) | required | required | required | required | required |
| `description` | **required** | required | recommended | required | required | required | required | required |
| `license` | optional | **optional** (seen in all 4) | not parsed | not parsed | not parsed | optional | optional | not present |
| `compatibility` | optional | not present | not parsed | not parsed | not parsed | optional | optional | not present |
| `metadata` | optional (map) | not present | not parsed | not parsed | not parsed | optional | optional | not present |
| `allowed-tools` | optional (experimental) | not present | **optional (extension)** | not parsed | not parsed | not parsed | optional | not present |
| `version` | not present | not present | not present | not present | not present | not present | not present | **extension** (plugin bundles only) |
| `model` | not present | not present | optional (extension) | not present | not present | not present | not present | not present |
| `effort` | not present | not present | optional (extension) | not present | not present | not present | not present | not present |
| `context` | not present | not present | optional (extension) | not present | not present | not present | not present | not present |
| `agent` | not present | not present | optional (extension) | not present | not present | not present | not present | not present |
| `hooks` | not present | not present | optional (extension) | not present | not present | not present | not present | not present |
| `paths` | not present | not present | optional (extension) | not present | not present | not present | not present | not present |
| `shell` | not present | not present | optional (extension) | not present | not present | not present | not present | not present |
| `argument-hint` | not present | not present | optional (extension) | not present | not present | not present | not present | not present |
| `disable-model-invocation` | not present | not present | optional (extension) | not present | not present | not present | not present | not present |
| `user-invocable` | not present | not present | optional (extension) | not present | not present | not present | not present | not present |
| MCP server field | not present | not present | not present | `agents/openai.yaml` (separate file) | not present | not present | not present | not present |
| Tool declaration | not present | not present | not present | `agents/openai.yaml` (separate file) | not present | not present | not present | not present |
| `risk` | not present | not present | not present | not present | not present | not present | not present | **extension** (aggregator metadata) |
| `source` | not present | not present | not present | not present | not present | not present | not present | **extension** (aggregator metadata) |
| `date_added` | not present | not present | not present | not present | not present | not present | not present | **extension** (aggregator metadata) |

**Key findings from the matrix:**

1. `name` and `description` are the **universal minimum** — required
   everywhere, uncontested.
2. `license`, `compatibility`, and `metadata` are **spec-defined
   optional fields** parsed by ADK and DeepAgents but not by Claude
   Code, Codex, or Gemini CLI in practice.
3. Claude Code adds **12+ consumer-specific extension fields** that
   are not portable. No other consumer parses them.
4. MCP server dependencies are **not declared in SKILL.md
   frontmatter by any surveyed consumer**. Codex uses a separate
   `agents/openai.yaml` sidecar. Anthropic official bundles express
   MCP usage in instruction text only.
5. Tool declarations in frontmatter: **not present in any
   consumer**. `allowed-tools` pre-approves existing tools; it does
   not declare new ones.
6. The `version` field appears only as an unofficial Claude Code
   plugin extension.
7. Community aggregators add their own non-spec fields as registry
   metadata. These will appear in bundles sourced from aggregators.

---

## 4. Loader Mechanics

### 4.1 Claude Code — detailed mechanics

**Discovery.** Walks active skill directories. Each subdirectory
containing a `SKILL.md` file is a skill. Priority:
enterprise > personal > project. Plugin skills use a
`plugin-name:skill-name` namespace.

**Frontmatter parsing.** Reads YAML between `---` delimiters.
Extracts `name` (with directory fallback), `description` (with
first-paragraph fallback), and all other supported fields. String
substitution variables (`$ARGUMENTS`, `${CLAUDE_SESSION_ID}`,
`${CLAUDE_SKILL_DIR}`) are replaced at activation time, not at parse
time.

**Instruction injection.** The skill catalog is injected into
context so the model knows what skills are available. When the model
or user invokes a skill, the rendered SKILL.md body enters the
conversation as a single message. It persists for the session.
Claude Code does not re-read the file on subsequent turns.

**Tool exposure.** `allowed-tools` pre-approves existing tools.
Example: `allowed-tools: Bash(git add *) Bash(git commit *) Read`.
No new tool declarations in frontmatter.

**Relative file references.** `${CLAUDE_SKILL_DIR}` resolves to the
skill's directory. Relative paths in the body are resolved by the
model's tool call against this base path. No server-side resolution
happens at parse time.

**MCP wiring.** Not handled at the SKILL.md layer. MCP connections
must be established separately at the session level.

Source: `https://code.claude.com/docs/en/skills`

### 4.2 Google ADK Python — detailed mechanics

**Discovery.** Caller-provided: `load_skill_from_dir(path: Path)` or
`SkillToolset(skills=[loaded_skill, ...])`. No automatic directory
scanning — the caller controls which bundles are loaded.

**Frontmatter parsing.** `load_skill_from_dir` reads SKILL.md,
parses YAML frontmatter into a `Frontmatter` object (`name`,
`description`), reads the body into an `instructions` string.
Returns a typed `Skill` object with `.frontmatter`, `.instructions`,
and `.resources` attributes.

**Instruction injection.** The `SkillToolset` exposes three tools:
`list_skills` (returns catalog), `load_skill(name)` (returns full
SKILL.md body), `load_skill_resource(skill_name, path)` (reads a
specific file from `references/` or `assets/`). The model calls
these tools to progressively disclose skill content.

**Relative file references.** `load_skill_resource` resolves paths
relative to the skill's root directory. Only files within the
skill's directory tree are accessible (path traversal prevention
enforced by the API).

**MCP wiring.** Not handled by the skill loader. Skills provide
instructions; tools (including MCP-sourced tools) are registered
separately via the ADK tool registration API.

Source: `https://adk.dev/skills/`

### 4.3 agentskills.io reference guide — generalised recommended pattern

The agentskills.io client implementation guide documents the
universal recommended pattern:

1. **Scan** skill directories (client-specific path +
   `.agents/skills/` for cross-client compatibility).
2. **Parse** YAML frontmatter: extract `name`, `description`, store
   `location` (absolute path to `SKILL.md`).
3. **Handle malformed YAML leniently**: warn on unknown fields;
   skip only if `description` is entirely absent or YAML is
   completely unparseable.
4. **Disclose** the catalog as `<available_skills>` XML (or
   equivalent) in the system prompt or tool description.
5. **Activate** on model or user request by loading full SKILL.md
   body (via file-read tool or dedicated `activate_skill` tool).
6. **Resolve** relative file references against the skill's
   directory.
7. **Protect** skill content from context compaction.

Source: `https://agentskills.io/client-implementation/adding-skills-support`

---

## 5. Prior Art in Go / Python / Java Orchestration Libraries

### 5.1 Go ecosystem — current state

**`github.com/niwoerner/go-agentskills` (v0.1.0, December 22, 2025)**

- A Go port of the Python `skills-ref` reference validator.
  Apache 2.0 license.
- Exports: `FindSkillMd`, `ParseFrontmatter`, `ReadProperties`,
  `Validate`, `ValidateMetadata`, `ToPrompt`.
- Typed `SkillProperties` struct: `Name`, `Description`,
  `License *string`, `Compatibility *string`, `AllowedTools *string`,
  `Metadata map[string]string`.
- Validates `name` format, `description` length, `compatibility`
  length.
- **Critical caveat.** README states: "This library was purely
  generated by a LLM and aims to mirror the python implementation."
  Pre-v1.0, no stability guarantee, explicitly experimental.
- Source: `https://pkg.go.dev/github.com/niwoerner/go-agentskills/pkg/agentskills`

**Google ADK for Go (`google.golang.org/adk`)**

- Does **not** have a skill-bundle loader. Code-first approach only.
  No `load_skill_from_dir` equivalent. Skills-in-the-agentskills.io-sense
  are not supported.
- Source: `https://pkg.go.dev/google.golang.org/adk`

**Eino (ByteDance/CloudWeGo)**

- Eino's ADK has middleware components; it contains a
  `Middleware_Skill.md` which is a SKILL.md skill for developing
  Eino middleware, but Eino does not expose a skill-bundle loader
  API.
- Source: `https://github.com/cloudwego/eino/tree/main/adk`

**Conclusion.** No production-grade Go library ships a SKILL.md
bundle loader with v1.0 stability. There is a genuine Go ecosystem
gap.

### 5.2 Python ecosystem

**Google ADK Python** — `load_skill_from_dir(path)` → typed `Skill`.
`SkillToolset`. Production, v1.25.0. Best prior art for the typed
`Skill` value + loader pattern.

**LangChain DeepAgents** — `create_deep_agent(skills=["/path/..."])`.
Recognises full agentskills.io field set. Catalog built at startup;
progressive disclosure on invocation.

**agentskills.io `skills-ref`** — Reference Python library.
`validate()`, `read_properties()`, `to_prompt()`. Apache 2.0. Marked
"for demonstration purposes only."

**`github.com/gotalab/skillport`** — Python MCP server for skill
management. MIT. Status: "Work in progress."

### 5.3 Java ecosystem

**Spring AI `SkillsTool`** — `SkillsTool` scans directories, parses
SKILL.md frontmatter, builds a registry. `AgentSkill.builder()` API
for programmatic construction. Loads from filesystem, classpath
JARs, URL resources. Source:
`https://spring.io/blog/2026/01/13/spring-ai-generic-agent-skills/`

### 5.4 TypeScript / JavaScript ecosystem

No production-grade TypeScript orchestration library with a SKILL.md
bundle loader was found in research (unable to verify as of
2026-04-10). No relevant TypeScript SDK shipped this feature in the
research window.

### 5.5 Summary

The typed `Skill` value + filesystem loader pattern is established
in Python (Google ADK, LangChain DeepAgents) and Java (Spring AI).
No Go library ships a production-grade equivalent. The only Go
implementation (`go-agentskills`) is LLM-generated, pre-v1.0, and
explicitly experimental. `praxis/skills` would be the first
production-quality Go implementation of a SKILL.md bundle loader.

---

## 6. Spec / RFC Status

**Verdict: no draft RFC, no active ratification process.**

The Agent Skills format is an open standard maintained by Anthropic
at `agentskills.io`, open to community contribution via GitHub pull
request. It was released December 2025 and has been adopted by 30+
tools without a formal ratification process. There is no versioned
spec release, no IETF working group, no ISO track, and no competing
cross-ecosystem standards body process underway.

The reversal condition for DEFER — "an imminent RFC in active
ratification with a stable draft expected within six months" — does
**not** apply. The format is already stable and widely deployed.

Sources: `https://agentskills.io`,
`https://github.com/agentskills/agentskills` (created 2025-12-16),
`https://simonwillison.net/2025/Dec/19/agent-skills/`

---

## 7. Security and Supply-Chain Considerations

### 7.1 Observed practice in existing consumers

**Path traversal.** The agentskills.io spec recommends clients
validate that resolved paths stay within the skill's directory tree.
Claude Code implements this via its sandbox `containsPathTraversal`
checks. Google ADK's `load_skill_resource` enforces bundle-root
containment. The community reference library (`skills-ref`) does
not implement path traversal prevention (it is demonstration-only).
Observed practice: path containment is the consumer's
responsibility; the spec recommends it but does not mandate a
specific implementation.

**Script execution.** Claude Code does not execute scripts from
skill bundles automatically. Script execution is subject to the
normal permission system. Inline `!cmd` substitution can be
disabled org-wide via `disableSkillShellExecution: true`. Script
execution is a caller-initiated action via the tool permission
model, not an automatic capability of skill loading. Claude Code
v2.1.38 blocks writes to `.claude/skills` in sandbox mode.

**Remote URL references.** The agentskills.io spec does not define
a mechanism for skills to declare remote URL dependencies. Claude
Code's sandbox blocks unexpected network access. There is no
observed practice of skill loaders automatically fetching remote
URLs at load time.

**Supply-chain risk is real and documented.** Snyk scanned 3,984
published skills and found 1,467 (36.8%) with malicious payloads
(credential theft, backdoors, data exfiltration). The ClawHavoc
campaign used trojanised skills with legitimate-appearing names.
Attack vectors:

- Malicious scripts bundled with skills (require manual approval to
  execute)
- Prompt injection payloads in SKILL.md body instructions
- Supply-chain attacks via unpinned dependencies in bundled scripts
- Social engineering via legitimate-looking skill names
- Aggregator-injected metadata fields used as vectors

**Observed consumer security posture (not hypothetical):**

- "Treat third-party skills as trusted code; read them before
  enabling" — widely recommended; acknowledged as insufficient at
  scale.
- Version pinning for bundled script dependencies — recommended by
  SafeDep.
- Project-level skills from untrusted repositories should be gated
  on a trust check — recommended by agentskills.io client guide.
- No widely deployed signature/attestation mechanism exists for
  skill bundles as of research date.

### 7.2 Implications for `praxis/skills`

The Phase 8 plan's R9 mitigation is well-founded by observed
practice. The loader must:

- Treat the bundle directory as a closed unit (no path escape).
- Make no network calls.
- Not automatically execute scripts.
- Surface raw paths for the caller's `tools.Invoker` to handle
  (script execution remains the caller's choice).

This is the pattern followed by the most security-conscious
existing consumers (Claude Code sandbox, Google ADK's
`load_skill_resource`). praxis inherits this posture.

Sources: `https://safedep.io/agent-skills-threat-model/`,
`https://snyk.io/articles/skill-md-shell-access/`,
`https://developers.redhat.com/articles/2026/03/10/agent-skills-explore-security-threats-and-controls`,
`https://www.anthropic.com/engineering/claude-code-sandboxing`

---

## 8. Comparison Matrix

| Criterion | Claude Code | OpenAI Codex | Gemini CLI | Google ADK (Python) | agentskills.io spec |
|---|---|---|---|---|---|
| Spec conformance | Superset (spec + 12+ extensions) | Subset (name, description only; deps in sidecar) | Conformant (name, description) | Conformant | Canonical |
| `name` required | No (dir fallback) | Yes | Yes | Yes | Yes |
| `description` required | Recommended (body fallback) | Yes | Yes | Yes | Yes |
| `license` parsed | No | No | No | Yes | Optional |
| `compatibility` parsed | No | No | No | Yes | Optional |
| `metadata` parsed | No | No | No | Yes | Optional |
| `allowed-tools` parsed | Yes (extension) | No | No | No | Experimental |
| MCP in frontmatter | No | No (sidecar) | No | No | No |
| Tool declaration in frontmatter | No | No (sidecar) | No | No | No |
| Typed loader API | No (internal) | No (internal) | No (internal) | Yes (`load_skill_from_dir`) | No |
| Go loader | No | No | No | No (Go ADK code-first only) | No |
| Path containment | Yes (sandbox) | `[unverified]` | `[unverified]` | Yes (API enforced) | Recommended |
| Auto-script execution | No (tool permission required) | `[unverified]` | `[unverified]` | No | No |

---

## 9. Verdict on the Working Hypothesis

**The working hypothesis.** praxis ships a first-class loader and
typed `Skill` value as a separately-versioned sub-module at
`github.com/praxis-os/praxis/skills`, mirroring Phase 7's
`github.com/praxis-os/praxis/mcp`. Loading is build-time only from
a caller-provided filesystem path. No registry talk, no download,
no runtime discovery, no hot-reload.

### Evidence summary

**Convergence is sufficient for a meaningful intersection.** The
agentskills.io specification provides a concrete, stable field set.
The portable minimum across all surveyed consumers is: `name`
(required), `description` (required), Markdown body as instruction
text (universal), and the three-level directory layout (`scripts/`,
`references/`, `assets/`, universal). Extension fields (Claude
Code's 12+ extras; Codex's `agents/openai.yaml`; aggregator
metadata) exist at the consumer-specific layer and are not part of
the intersection.

**MCP server declarations are not in frontmatter anywhere.** The
working hypothesis's assumption that `mcp_servers` might appear as
a parseable frontmatter field is not supported by any surveyed
consumer. The Phase 8 D131 design for MCP-backed skill dependencies
should assume the caller composes `praxis/mcp` and `praxis/skills`
themselves; the loader surfaces raw instructions only, not
machine-parseable MCP server specs.

**No imminent competing RFC.** The DEFER condition does not apply.

**There is a genuine Go library gap.** No production-grade Go
library ships a SKILL.md bundle loader. Python (Google ADK) and
Java (Spring AI) do. `praxis/skills` would be the first
production-quality Go implementation.

**Format fragmentation is contained.** The format is not fragmented
at the `name` + `description` + body level. Fragmentation exists
only in the optional fields layer and the consumer-specific
extension layer — all of which the loader should ignore with a
warning (lenient-ignore policy recommended by agentskills.io and
justified by observed community bundles carrying non-spec fields).

### Verdict: GO (sub-module)

The convergence across consumers is sufficient to justify shipping
a loader in a `praxis/skills` sub-module now. The agentskills.io
spec defines a stable, widely-adopted field set with 30+ production
adopters. The portable intersection (`name`, `description`,
Markdown body as instructions, three-level directory layout) is
clear, uncontested, and validated by real bundle inspection. No
competing RFC is in flight. The Go ecosystem has a genuine gap that
`praxis/skills` would fill. The sub-module pattern (same repo,
separate `go.mod`, zero cost to non-skills consumers) is proven by
Phase 7.

**D123 anchor — canonical `SKILL.md` shape for `praxis/skills`:**

| Field | Status in praxis/skills v1.0 | Evidence |
|---|---|---|
| `name` | Required | Universal across all consumers and spec |
| `description` | Required | Universal across all consumers and spec |
| `license` | Optional (parsed, surfaced on `Skill`) | Spec-defined; seen in all 4 inspected Anthropic official bundles |
| `compatibility` | Optional (parsed, surfaced on `Skill`) | Spec-defined; parsed by ADK, DeepAgents |
| `metadata` | Optional (map, parsed, surfaced on `Skill`) | Spec-defined; parsed by ADK, DeepAgents |
| `allowed-tools` | Optional (experimental; parsed as `[]string`, surfaced on `Skill`) | Spec-defined (experimental); Claude Code extension; no enforcement in praxis |
| All other fields | Ignored with `SkillWarning` | Consumer-specific extensions not in spec intersection |

**What is explicitly not in the v1.0.0 loader:**

- No `mcp_servers` frontmatter field (not present in any surveyed
  consumer).
- No `version` frontmatter field (unofficial Claude Code plugin
  extension only).
- No tool declaration in frontmatter (no consumer uses this
  pattern).
- No runtime MCP wiring by the loader (callers compose `praxis/mcp`
  separately).
- No `model`, `effort`, `context`, `agent`, `hooks`, `paths`,
  `shell`, `argument-hint`, `disable-model-invocation`,
  `user-invocable` (Claude Code-specific, not portable).

---

## 10. What to Explore Further

The following items require deeper investigation before
implementation (informing OPEN items in `02-scope-and-positioning.md`):

1. **YAML parser selection.** The current `go.mod` carries
   `go.yaml.in/yaml/v2` as an indirect dependency.
   `gopkg.in/yaml.v3` (BSD-3-Clause, Canonical) is the de facto Go
   standard and the recommended choice for `praxis/skills`. Confirm
   it stays within the Phase 5 D73 stdlib-favoured posture and
   passes govulncheck. The alternative is to use `go.yaml.in/yaml/v3`
   (the successor to yaml.v2 maintained by the same team).

2. **`allowed-tools` enforcement posture.** The spec marks it
   experimental; Claude Code is the only consumer that parses it.
   The praxis loader should parse and surface it as a `[]string` on
   `Skill`, but make no enforcement decisions. Enforcement is the
   caller's choice via their `tools.Invoker` configuration. This
   must be stated explicitly in D124 to prevent callers from
   assuming praxis enforces it.

3. **`fs.FS` vs `os.DirFS` in the loader signature.** `fs.FS`
   (stdlib, Go 1.16+) enables testing with `embed.FS` and
   `fstest.MapFS`, mirrors the `InMemoryTransport` testability
   benefit from Phase 7, and is the idiomatic abstraction. Confirm
   `skills.Open(fsys fs.FS, path string)` as the primary entry
   point alongside `skills.Load(path string)` (which calls
   `skills.Open(os.DirFS("."), path)` internally).

4. **Tool namespacing convention.** No ecosystem consumer
   namespaces tools with a skill prefix. D126 must design the
   convention from scratch without a reference point. The
   api-designer owns this; the research confirms the blank slate.

5. **Codex `agents/openai.yaml` sidecar.** If any praxis consumer
   distributes skill bundles that include `agents/openai.yaml`, the
   loader must silently ignore the file (it is not a SKILL.md
   concern). D124 should explicitly state that non-SKILL.md files in
   the bundle directory are not read by the loader.

6. **Progressive disclosure implementation model.** The
   agentskills.io guide recommends two patterns: file-read
   activation (model uses file-read tool) and dedicated-tool
   activation (harness provides `activate_skill` tool). For praxis,
   instruction injection at build time (system-prompt composition)
   is the simpler and more secure choice — it does not require the
   orchestrator to expose a file-read tool. Confirm this is
   compatible with D128 (no change to frozen `LLMRequest` type).

7. **`metadata` map key naming for MCP server conventions.** Some
   community bundles informally use `metadata.mcp-server` to
   declare MCP dependencies. This is not spec-defined and should
   not be parsed as machine-readable MCP configuration in v1.0.0.
   The loader should surface it as part of the opaque
   `Metadata map[string]string` value only.

---

## 11. Sources

All URLs fetched and verified on 2026-04-10. `[unverified]` marks
claims not directly confirmed by a fetch.

- Agent Skills specification — `https://agentskills.io/specification`
- Agent Skills overview — `https://agentskills.io`
- Agent Skills client implementation guide — `https://agentskills.io/client-implementation/adding-skills-support`
- `agentskills/agentskills` GitHub repository — `https://github.com/agentskills/agentskills`
- `anthropics/skills` GitHub repository — `https://github.com/anthropics/skills`
- Anthropic skills: mcp-builder SKILL.md (raw) — `https://raw.githubusercontent.com/anthropics/skills/main/skills/mcp-builder/SKILL.md`
- Anthropic skills: algorithmic-art SKILL.md (raw) — `https://raw.githubusercontent.com/anthropics/skills/main/skills/algorithmic-art/SKILL.md`
- Anthropic skills: frontend-design SKILL.md (raw) — `https://raw.githubusercontent.com/anthropics/skills/main/skills/frontend-design/SKILL.md`
- Anthropic skills: webapp-testing SKILL.md (raw) — `https://raw.githubusercontent.com/anthropics/skills/main/skills/webapp-testing/SKILL.md`
- Claude Code skills documentation — `https://code.claude.com/docs/en/skills`
- Claude Code plugin skills SKILL.md — `https://github.com/anthropics/claude-code/blob/main/plugins/plugin-dev/skills/skill-development/SKILL.md`
- Claude Code `allowed-tools` validator bug — `https://github.com/anthropics/claude-code/issues/26795`
- VS Code agent skills validator issue — `https://github.com/microsoft/vscode/issues/294520`
- OpenAI Codex skills — `https://developers.openai.com/codex/skills`
- OpenAI skills repo SKILL.md — `https://github.com/openai/skills/blob/main/skills/.system/skill-creator/SKILL.md`
- Gemini CLI skills — `https://geminicli.com/docs/cli/skills/`
- Google ADK skills — `https://adk.dev/skills/`
- Google ADK Go package — `https://pkg.go.dev/google.golang.org/adk`
- LangChain DeepAgents skills — `https://docs.langchain.com/oss/python/deepagents/skills`
- Spring AI agent skills — `https://spring.io/blog/2026/01/13/spring-ai-generic-agent-skills/`
- `go-agentskills` package — `https://pkg.go.dev/github.com/niwoerner/go-agentskills/pkg/agentskills`
- `go-agentskills` GitHub — `https://github.com/niwoerner/go-agentskills`
- `skillport` GitHub — `https://github.com/gotalab/skillport`
- DeepWiki SKILL.md format specification — `https://deepwiki.com/anthropics/skills/2.2-skill.md-format-specification`
- Anthropic Agent Skills API docs — `https://platform.claude.com/docs/en/agents-and-tools/agent-skills/overview`
- agentskills.io `skills-ref` reference library — `https://github.com/agentskills/agentskills/tree/main/skills-ref`
- SkillsMP marketplace — `https://skillsmp.com` (HTTP 403 on direct fetch; size data from secondary sources `[unverified]`)
- Antigravity Awesome Skills — `https://github.com/sickn33/antigravity-awesome-skills`
- Antigravity frontend-design SKILL.md (raw) — `https://raw.githubusercontent.com/sickn33/antigravity-awesome-skills/main/skills/frontend-design/SKILL.md`
- Vercel Agent Skills FAQ — `https://vercel.com/blog/agent-skills-explained-an-faq`
- Simon Willison on Agent Skills — `https://simonwillison.net/2025/Dec/19/agent-skills/`
- SafeDep threat model — `https://safedep.io/agent-skills-threat-model/`
- Snyk skill shell access — `https://snyk.io/articles/skill-md-shell-access/`
- Red Hat security controls — `https://developers.redhat.com/articles/2026/03/10/agent-skills-explore-security-threats-and-controls`
- Claude Code sandboxing — `https://www.anthropic.com/engineering/claude-code-sandboxing`
- Claude Code permissions — `https://code.claude.com/docs/en/permissions`
- ToolHive agent skills — `https://docs.stacklok.com/toolhive/updates/2026/04/06/updates`

---

**Document status.** Solution-researcher output, 2026-04-10. All
sources verified by direct URL fetch except where marked
`[unverified]`.
