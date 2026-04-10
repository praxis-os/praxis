# Phase 8 — Non-Goals

**Decision:** D133 (draft)
**Cross-references:** Phase 1 D09 (no plugins in v1.0), Phase 1
Non-goal 7, Phase 7 D109 (D09 re-confirmation), Phase 7 D120
(non-goals catalogue precedent), Phase 8 D122 (positioning),
D124 (loader), D131 (cross-module composition).

This document catalogues things that `praxis/skills v1.0.0`
explicitly **does not** do. The list is binding: implementing any
of these items requires a new RFC and, for the items that touch
D09, explicit re-opening of the Phase 1 non-goals, which is
prohibited without a major version bump.

This catalogue mirrors the structure of Phase 7 D120
(`docs/phase-7-mcp-integration/05-non-goals.md`). Each non-goal
states what is not shipped, why, the D09 / Phase 7 / Phase 1
connection if any, and the explicit deferral posture.

---

## 1. No skill registry, marketplace, or directory

**What is not shipped.** praxis does not talk to skills.sh, the
agentskills.io registry, any GitHub index, any npm-style package
server, or any other skill registry. The sub-module contains no
HTTP client for registry APIs, no known registry URLs in source
code, and no configuration surface for adding registry endpoints.

**Why.** A registry client inside the library would:

1. Require network I/O at library construction time, violating
   the build-time-only composition principle of D09 / Phase 1
   Non-goal 7.
2. Introduce an authentication boundary that praxis does not
   own (who the caller is to the registry, what credentials are
   used, how failures are retried) — this is a consumer platform
   concern, not a library concern.
3. Create a supply-chain vector that is hard to audit: which
   registry was queried? at what point in the build? was the
   result cached? was TOFU (trust-on-first-use) applied?

**D09 connection.** This is the primary D09 boundary. A registry
client turns skill loading into dynamic capability acquisition,
which is exactly the "no plugins in v1" rule. Violating it would
re-open D09 and force a v2 module path.

**Caller workaround.** Consumers who want registry integration
write it themselves: fetch the bundle with their preferred
registry client, unpack it to a filesystem path, then call
`skills.Load(path)`. The library boundary stays at "I load from
a filesystem path you already have."

---

## 2. No downloading, fetching, or network access

**What is not shipped.** `skills.Load` and `skills.Open` make no
HTTP calls, no DNS lookups, no TCP dials, and no file descriptor
operations outside the provided `fs.FS`. The loader does not
follow `http://` or `https://` URLs declared inside a bundle's
frontmatter or body. It does not resolve `git+ssh://` references.
It does not expand `${HOME}` or any environment variable.

**Why.** Network I/O inside a library constructor is a surprise
that breaks:

- Reproducible builds (`go build` should not talk to the network).
- Hermetic tests (`skills.Open(fstest.MapFS{...}, ...)` should
  produce identical results across machines).
- Security auditing (every network hop is a trust boundary;
  hiding them inside a library constructor prevents audit).
- Offline development environments.

**D09 connection.** Auto-downloading a bundle from a URL is
registry-by-another-name. Same rule applies.

**Caller workaround.** If a bundle should come from a URL, the
caller downloads it first (into a temporary directory or a
persistent cache), then points `skills.Load` at the local path.

---

## 3. No runtime discovery or dynamic registration

**What is not shipped.** praxis does not scan the filesystem for
`SKILL.md` files. There is no `skills.Discover(root)` that walks
a directory tree, no default "look in `~/.praxis/skills/`" fallback,
no automatic inclusion of bundles based on their location, and no
"enable all skills in this directory" option on the orchestrator.

**Why.** Filesystem scanning is how platform tools like Claude
Code and Gemini CLI discover their skills, because they are
interactive agents that run in a user shell and can reasonably
treat the shell's filesystem as their configuration space. praxis
is a **library**. Its consumers are other libraries or services,
which have their own discovery conventions and cannot assume the
shape of the filesystem their process happens to run on.

Runtime discovery also re-opens D09 because it lets the set of
loaded skills depend on the state of the filesystem at
orchestrator-construction time — a form of dynamic plugin loading.

**D09 connection.** Direct. Discovery = plugins = rejected.

**Caller workaround.** The caller implements their own discovery
logic (if they need any) and calls `skills.Load` on each bundle
they choose to enable. The caller is the authority on which
bundles exist and which should be active.

---

## 4. No hot reloading, file watching, or runtime updates

**What is not shipped.** Once a `*Skill` is returned from `Load`
or `Open`, its contents are frozen for the lifetime of the value.
The loader does not re-read `SKILL.md` on every invocation. It
does not watch the bundle directory for changes. There is no
`Skill.Reload()` method. Modifying the bundle on disk after load
has no effect on the running orchestrator until the caller
explicitly re-loads and re-composes.

**Why.** Hot reloading creates invalidation races (what happens to
an in-flight invocation when the bundle changes mid-run?), breaks
determinism (two runs of the same program can behave differently
based on filesystem timing), and is incompatible with the Phase 3
frozen semantics of `AgentOrchestrator`, which assumes stable
configuration across invocations.

**D09 connection.** Indirect. Hot reloading is a form of runtime
dynamic registration — the same rule.

**Caller workaround.** The caller builds a new orchestrator with
the updated bundle. This is explicit, visible, and consistent
with how any other orchestrator configuration change is handled.

---

## 5. No authoring tooling, validators, or scaffolding

**What is not shipped.** `praxis/skills` is a **consumer** of
skill bundles, not an **author** of them. The sub-module provides
no CLI to create a new skill, no template generator, no linter,
no "check my `SKILL.md` for spec compliance" tool, and no
migration helper for upgrading bundles to newer spec versions.

**Why.** Authoring tooling is a separate problem domain with its
own user interface surface (CLI? GUI? editor plugin?) and its own
maintenance cadence (tied to the evolving agentskills.io spec,
not to the praxis release line). It also has no overlap with the
consumer use case that justifies praxis's first-class support.

**Precedent.** The spec itself ships a `skills-ref` reference
library marked "for demonstration purposes only"; the
agentskills.io ecosystem treats authoring as a separate concern.
praxis follows the same split.

**Caller workaround.** Use authoring tools from the ecosystem:
the official `anthropics/skills` templates, the community
`skill-creator` tools, IDE plugins, or direct editing. praxis
loads whatever they produce as long as it parses.

---

## 6. No sandboxing, script execution, or side-effect management

**What is not shipped.** `praxis/skills` does not execute any
script bundled with a skill. It does not provide a sandbox for
running bundled code. It does not chroot, drop privileges,
restrict syscalls, or apply any OS-level isolation. It does not
expose a "run this script safely" helper.

**Why.** Script execution inside a skill is a caller concern,
mediated through the `tools.Invoker` seam. If a bundle references
a local script that the model chooses to invoke, the `Invoker`
implementation on the caller's side decides:

1. Whether to execute it at all (policy).
2. How to isolate it if executed (sandbox, container, separate
   process with reduced privileges).
3. How to surface the output back to the LLM (trust classification
   per Phase 5 D77, D78).

praxis cannot make these decisions for the caller because the
right answer depends entirely on the deployment context
(interactive CLI vs. hosted API vs. batch job) and on the caller's
policy framework. Attempting to provide a one-size-fits-all
sandbox would produce a solution too permissive for hosted
environments and too restrictive for local development.

**Phase 5 connection.** This posture mirrors Phase 5 D77 (untrusted
output contract): praxis provides the classification and the
seam; the caller applies the policy.

**Caller workaround.** Use the Phase 7 `praxis/mcp` sub-module
or the caller's own `tools.Invoker` implementation to handle
script execution under whatever isolation policy the deployment
requires.

---

## 7. No `mcp_servers` field in the recognised frontmatter intersection

**What is not shipped.** `praxis/skills v1.0.0` does not recognise
`mcp_servers` as a frontmatter field. There is no `Skill.MCPServers()`
accessor and no `MCPServerSpec` value type in the v1.0.0 public
surface. If a bundle declares `mcp_servers` in its frontmatter
(which no surveyed consumer currently does — see
[research-solutions.md §3](research-solutions.md)), the loader
preserves the raw value in `Skill.Extensions()` under the key
`mcp_servers` as an untyped `any` (whatever the YAML decoder
produced) and emits a `SkillWarning{Kind: WarnExtensionField}`.

**Why.** The research showed zero consumers declaring MCP server
dependencies in SKILL.md frontmatter as of 2026-Q2. Codex uses a
separate `agents/openai.yaml` sidecar; Anthropic's official bundles
express MCP dependencies in instruction text only. Defining a
typed schema inside praxis now would freeze a shape the ecosystem
has not yet agreed on — a textbook case of premature intersection.
Shipping a typed accessor that always returns `nil` would also be
confusing API surface for v1.0.0.

**How this unblocks future work.** When the ecosystem converges
on a shape, a future minor version of `praxis/skills` can:

1. Add a typed `MCPServerSpec` value type (additive).
2. Add a typed `Skill.MCPServers() []MCPServerSpec` accessor (additive).
3. Move `mcp_servers` from the extension bag into the typed
   accessor (a documented behaviour change, but since today the
   data is in `Extensions()`, callers who already wrote code
   against `Extensions()["mcp_servers"]` get a transition window
   via a deprecation warning).

None of these changes break v1.0.0 callers, because v1.0.0
documents the extension-bag path as the only way to read this
field.

**Caller workaround in v1.0.0.** Callers who know a specific
bundle family carries MCP server data under a known key walk
`Skill.Extensions()` themselves:

```go
if raw, ok := sk.Extensions()["mcp_servers"]; ok {
    // raw is `any`; type-assert based on the bundle family's
    // convention. Example: a list of mappings.
    if servers, ok := raw.([]any); ok {
        for _, s := range servers {
            // interpret each entry according to the caller's
            // own knowledge of the bundle family
        }
    }
}
```

---

## 8. No signature verification or provenance attestation

**What is not shipped.** `praxis/skills v1.0.0` does not verify
signatures on skill bundles, does not check provenance
attestations (sigstore, SLSA, in-toto), and does not consult any
transparency log. It loads what the filesystem provides.

**Why.** Provenance verification is an orthogonal concern that
belongs outside the loader. The caller has better context about
which trust anchors to use, which attestation formats apply to
their deployment, and what failure mode is acceptable (warn,
reject, quarantine). Bundling a specific verification scheme
inside praxis would either force a choice on all consumers
(wrong — inflexible) or expose a pluggable verifier interface
(wrong — re-opens D09).

**Phase 5 connection.** Identity and signing for praxis's own
artifacts are handled by Phase 5 D70 (Ed25519 `identity.Signer`
for agent identity). That decision is about praxis-authored
JWTs, not about verifying third-party skill bundles.

**Caller workaround.** Verify bundles before passing their path
to `skills.Load`. The verification step runs in the caller's
code, outside the praxis library boundary.

---

## 9. No automatic credential resolution for skill-referenced MCP servers

**What is not shipped.** `praxis/skills` does not resolve
credentials. The loader does not call `credentials.Resolver`. It
does not know which MCP servers a skill's instruction text refers
to (per non-goal 7, MCP server lists are not in the recognised
frontmatter intersection). It does not pass credential values
between modules.

**Why.** Credential resolution is a Phase 5 concern (D67, D68
zero-on-close) that belongs in the caller's trust boundary. Even
if a future minor version adds typed MCP server declarations to
the recognised set, the credential flow remains entirely on the
caller's side: `praxis/skills` would surface a credential
reference name (an opaque string), and the caller would resolve
it via their `credentials.Resolver` before passing the resolved
value to `praxis/mcp`. The loader never sees credential values.

**Phase 5 / Phase 7 connection.** Phase 7 D117 specified the
credential flow for long-lived MCP sessions, with an accepted
Phase 5 §3.2 goroutine-scope deviation for HTTP transport. That
decision is the single source of truth. Phase 8 inherits it by
reference; it does not introduce a parallel path.

**Caller workaround in v1.0.0.** Callers configure MCP servers
(including their credentials) entirely outside `praxis/skills`,
in their normal `praxis/mcp.New` setup. The skill bundle
contributes only instruction text; the LLM dispatches to
MCP-exposed tools through the caller-configured invoker. See
[04-dx-and-errors.md §1.4](04-dx-and-errors.md) for the worked
wiring example.

---

## 10. No consumer brand awareness

**What is not shipped.** `praxis/skills` does not ship any code,
documentation, constant, or test fixture that references a
specific commercial consumer by name. It does not special-case
Claude Code, Codex, Gemini CLI, skills.sh, Antigravity, or any
other named tool. The intersection field set (D123 §3.1 / §3.2)
is justified by the agentskills.io spec and sourced research, not
by "what Claude Code does."

**Why.** The decoupling contract (Phase 1, seed §6.1) prohibits
identifiers, assumptions, or structure specific to any single
consumer in praxis code and phase artifacts. The research
document (`research-solutions.md`) references consumers because
it is surveying the ecosystem as external facts. The production
sub-module does not.

**Phase 1 connection.** Direct. The banned-identifier grep
enforces this at CI time. Any accidental leakage of a consumer
name into `praxis/skills` source code fails the build.

**Caller workaround.** N/A — this is a source-code hygiene rule,
not a feature the caller can opt into.

---

## 11. D09 / Non-goal 7 re-confirmation (mirror of Phase 7 D109)

This section is the explicit Phase 8 re-confirmation that D09
remains closed and is **not** re-opened by any Phase 8 decision.

- Skill loading is **build-time composition**: the caller provides
  a filesystem path at construction time, the library reads it
  once, and the resulting `*Skill` is immutable.
- The caller decides which bundles exist, which are enabled, and
  which `tools.Invoker` handles their tool calls. praxis does not
  discover or select bundles automatically.
- No network I/O, no registry talk, no hot reloading, no dynamic
  registration, no sandbox, no authoring tooling.
- The Phase 1 amendment protocol is not invoked. D09 stands.

Phase 7 made the equivalent statement in D109 for MCP. Phase 8
makes it here for skills. Both phases add capability to the
library without altering the D09 boundary.

---

## D133 decision summary

Non-goals for `praxis/skills v1.0.0`, binding:

| # | Non-goal | D09 link | Caller path |
|---|---|---|---|
| 1 | No registry, marketplace, or directory | Direct | Download outside praxis, pass path |
| 2 | No downloading, fetching, network access | Direct | Download outside praxis, pass path |
| 3 | No runtime discovery / dynamic registration | Direct | Caller enumerates bundles |
| 4 | No hot reloading, file watching, runtime updates | Indirect | Rebuild orchestrator |
| 5 | No authoring tooling, validators, scaffolding | n/a | Use ecosystem tools |
| 6 | No sandboxing or script execution | n/a | Caller-owned `tools.Invoker` |
| 7 | No `mcp_servers` field in recognised frontmatter | n/a | `Skill.Extensions()` passthrough |
| 8 | No signature / provenance verification | n/a | Verify before `Load` |
| 9 | No automatic credential injection for MCP | Phase 5/7 | `credentials.Resolver` explicit |
| 10 | No consumer brand awareness | Phase 1 | N/A — code hygiene rule |
| 11 | D09 / Non-goal 7 re-confirmation (binding statement) | Direct | N/A — is the rule |

All 11 non-goals are binding for v1.0.0. Changes after the v1.0.0
tag require the Phase 1 amendment protocol. Re-opening any D09-linked
item (1, 2, 3, 4, 11) requires a v2 module path, not an amendment.
