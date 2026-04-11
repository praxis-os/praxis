# Phase 1 — Decisions Log

**Phase:** API Scope and Positioning
**Range owned:** D01–D15 (contiguous)
**Status:** decided, pending reviewer confirmation
**Starting baseline:** `docs/PRAXIS-SEED-CONTEXT.md`

Each decision below carries: title, status, rationale, alternatives considered,
and a one-line summary for the `roadmap-status` skill.

---

## Amendment protocol

Decisions recorded in this log are **adopted working positions**, not
immutable commitments. They represent the best answer available with the
evidence at hand and are expected to hold through the phases that depend on
them — but they remain open to amendment when a later phase, external
change, or new evidence gives a concrete reason to revisit.

**When amendment is appropriate.**

- A later phase discovers a contradiction, a missed case, or an
  incompatibility with a downstream design constraint.
- New external evidence emerges (ecosystem change, vendor behavior change,
  consumer feedback, a competitor shipping something relevant).
- An assumption flagged as weak in a phase's `00-plan.md` is invalidated by
  observation.
- The reviewer subagent or `review-phase` skill surfaces a concern that
  cannot be resolved without revisiting the prior decision.

**How amendments are recorded.**

1. The phase that discovers the need opens a **new decision ID** in its own
   phase's `01-decisions-log.md` (not a retroactive edit to the original
   entry). The new decision carries a `Supersedes:` field naming the
   original decision ID.
2. The original entry is annotated with a `Superseded by: DNN (Phase N)`
   line. Its text is left intact as the historical record of what was
   believed at the time.
3. The `roadmap-status.md` adopted-decisions list is updated to reflect the
   new current position.
4. If the amendment touches the seed document
   (`docs/PRAXIS-SEED-CONTEXT.md`), the seed itself is **not edited**. The
   amendment record in the decision log is the authoritative source of
   truth; the seed remains the starting baseline as-of extraction.

**What is not amendment territory.**

- The `frozen-v1.0` API stability tier in
  [`04-v1-freeze-surface.md`](04-v1-freeze-surface.md) is a different
  concept. It is a semver-level commitment to downstream consumers that
  interface *shapes* do not change without a v2 module path bump. That is a
  technical contract with users, not a methodological stance of the design
  team. Amendments to the methodology (this log) do not imply amendments
  to the v1.0 stability commitment (the tier).
- The decoupling contract in seed §6 is binding and is not subject to
  amendment — it is a correctness invariant, enforced by CI.

The amendment protocol exists so that being decisive in Phase 1 does not
mean being inflexible through Phases 2–6. Decisiveness now, flexibility
with justification later.

---

## D01 — Positioning statement

**Status:** decided
**Summary:** praxis is a production-grade Go invocation kernel for a single
agent call, with enterprise guardrails built in by construction.

**Decision.**
`praxis` is a production-grade Go library for orchestrating LLM agent
invocations with enterprise guardrails built into the invocation kernel rather
than composed on top of it. It owns a single agent call from request to
terminal state, enforcing policy, cost, security, and observability contracts
by construction through a typed state machine and stable Go interfaces. It is
provider-agnostic, ships Anthropic and OpenAI adapters, and delegates every
consumer-specific concern — policy wiring, credential management, identity
attribution, tool routing — to caller-supplied implementations of small,
stable interfaces. `praxis` is not a general LLM application framework, not a
prompt template engine, not a multi-agent coordination protocol, and not a
replacement for direct SDK use when auditability and cost control are not
requirements.

**Rationale.** Sharpens seed §1 into a README-ready paragraph that is narrow
enough to reject scope creep on sight. Phrased so that adding a prompt engine,
RAG store, or multi-agent router is rejectable on charter grounds.

**Alternatives considered.** (a) Seed §1 verbatim — kept the "enterprise
guardrails" hook but read as governance-platform marketing rather than a
library charter. (b) "Agent orchestration library for Go" — too broad,
indistinguishable from LangChainGo/Eino framing.

---

## D02 — Design principles

**Status:** decided
**Summary:** Seven seed principles kept, one principle added: zero-wiring
smoke path. Final count: eight.

**Decision.** The principles from seed §3 are kept verbatim in substance
(principles 1–7). A new principle is added:

> **P8 — Zero-wiring smoke path.** Every interface ships a null or noop
> default implementation. An `AgentOrchestrator` must be constructible and
> callable with zero caller-supplied wiring beyond an `llm.Provider`.

**Rationale.** The seed implies P8 in §5 ("each ships with a null or minimal
default implementation") but does not state it as a governing principle. Left
at implementation-detail status, it is the first thing eroded under velocity
pressure — and the 30-line README example depends on it. Promoting it to a
named principle makes it binding and drives Phase 3 constructor shape (see D12).

**Alternatives considered.** (a) No new principle — rejected because the
existing seven leave the smoke path implicit and therefore negotiable. (b) A
principle about "idiomatic Go" — rejected as too vague; the golangci-lint and
gofmt gates in the development process already enforce this.

---

## D03 — Target consumer archetype and anti-persona

**Status:** decided
**Summary:** Target = platform/infra teams with LLM calls already in
production who need audit/cost/security by construction. Anti-persona = first
LLM integration, multi-agent mesh builders, Python-LangChain ergonomics
expectations.

**Decision.** See `02-positioning-and-principles.md` §3 for the full profile.
Target archetype: platform engineering or ML infrastructure teams inside
organizations that already run LLM-backed features in production and whose
pain is auditability, cost control, and security boundaries rather than "our
first LLM integration". Anti-persona explicitly named: (a) developers
prototyping an LLM feature for the first time — use a direct SDK or
LangChainGo; (b) teams building agent-to-agent mesh systems — use Google ADK
for Go; (c) teams wanting Python-LangChain-style composable chains or graphs
— use LangChainGo or Eino.

**Rationale.** Phase 1 without an anti-persona leaks into bug reports and
feature requests from users who are not the target audience, and each rejection
is re-litigated case by case. Naming the anti-persona up front gives the
README and the issue tracker a clean redirect.

**Alternatives considered.** (a) No anti-persona — rejected, see R5 in
`00-plan.md`. (b) Target "all Go developers doing LLM work" — rejected as too
broad to justify the narrow interface surface.

---

## D04 — v1.0 freeze surface (which interfaces are frozen at v1.0)

**Status:** decided
**Summary:** Twelve of fourteen seed §5 interfaces reach `frozen-v1.0`. Two
(`budget.PriceProvider`, `identity.Signer`) ship as `stable-v0.x-candidate`
pending later-phase resolution.

**Decision.** Full per-interface tiering in
[`04-v1-freeze-surface.md`](04-v1-freeze-surface.md). Summary:

| Interface | Tier |
|---|---|
| `orchestrator.AgentOrchestrator` | frozen-v1.0 |
| `llm.Provider` | frozen-v1.0 |
| `tools.Invoker` | frozen-v1.0 |
| `hooks.PolicyHook` | frozen-v1.0 |
| `hooks.PreLLMFilter` | frozen-v1.0 |
| `hooks.PostToolFilter` | frozen-v1.0 |
| `budget.Guard` | frozen-v1.0 |
| `budget.PriceProvider` | stable-v0.x-candidate (gated by D08; promotion schedule in `04-v1-freeze-surface.md`) |
| `errors.TypedError` | frozen-v1.0 |
| `errors.Classifier` | frozen-v1.0 |
| `telemetry.LifecycleEventEmitter` | frozen-v1.0 |
| `telemetry.AttributeEnricher` | frozen-v1.0 |
| `credentials.Resolver` | frozen-v1.0 |
| `identity.Signer` | stable-v0.x-candidate (gated by Phase 5) |

**Rationale.** The seed interfaces are small (1–5 methods each), designed for
freeze from the outset, and have few plausible expansion vectors. The two
held back are both gated on unresolved phase work, not on design doubt.

**Alternatives considered.** (a) Freeze all 14 — rejected because
`PriceProvider` shape depends on D08 and `Signer` details depend on Phase 5
output. (b) Ship fewer than 12 as frozen — rejected as unnecessarily
conservative; the interfaces are deliberately minimal.

---

## D05 — Non-goals for v1.x

**Status:** decided
**Summary:** Seven explicit non-goals cataloged in `03-non-goals.md`; each
references the interface that would have to exist if the non-goal were
reversed.

**Decision.** See [`03-non-goals.md`](03-non-goals.md). Headline items: no
prompt template engine, no multi-agent orchestration, no vector store or RAG
primitives, no HTTP/gRPC transport binding, no bundled price tables for any
commercial LLM provider, no graph/chain composition model, no plugin or
runtime extension registry (see D09).

**Rationale.** Without an enumerated non-goals list, Phase 3 interface design
drifts into framework territory. The list gives reviewers a concrete rejection
reference for any proposed interface that does not fit the invocation-kernel
scope.

**Alternatives considered.** (a) No non-goals list, rely on positioning
statement — rejected; the positioning statement is too terse for specific
rejection arguments.

---

## D06 — `tools.Invoker` tool-name placement *(resolves seed §13.1)*

**Status:** decided
**Summary:** Tool name lives on the `ToolCall` struct, not on
`InvocationContext`.

**Decision.** The tool name is a field of the `ToolCall` struct passed to
`tools.Invoker.Invoke`. `InvocationContext` carries invocation-level state
(budget, span, policy decision carry-through) only.

**Rationale.** Parallel tool-call execution is the decisive argument. When
the LLM returns multiple tool calls in a single response, the orchestrator
dispatches them concurrently (subject to
`llm.Provider.SupportsParallelToolCalls`). Each concurrent dispatch is an
independent `Invoke` call. If the tool name lived on `InvocationContext`, the
context would need to be cloned per call (overhead, mutation risk) or shared
(race by construction). Placing the name on `ToolCall` makes each `Invoke`
call self-contained: tool name, arguments, and call id travel together as an
immutable unit. Cohesion also improves: the invoker routes purely on the
struct it receives, and tests can construct a minimal `ToolCall` without
building a full `InvocationContext`.

**Alternatives considered.** (a) Tool name on `InvocationContext` — rejected
because it conflates invocation-level state with per-tool-call routing keys
that live at different granularities, and introduces a race under parallel
dispatch.

---

## D07 — `requires_approval` hook-result semantics *(resolves seed §13.2)*

**Status:** decided
**Summary:** Orchestrator returns a structured "approval required" decision
to the caller and transitions to a terminal `ApprovalRequired`-style state;
the caller handles persistence and resume out-of-process.

**Decision.** When a `hooks.PolicyHook` returns a decision indicating human
approval is required, the orchestrator treats the condition as terminal for
that invocation. It emits a terminal lifecycle event, closes the
`InvokeStream` channel cleanly, and returns a typed error (or a terminal
decision on the non-streaming path) that is `errors.As`-compatible with an
`ApprovalRequiredError`. The caller is responsible for persisting whatever
context it needs to resume, for collecting the human decision, and for
re-invoking the orchestrator with a fresh context when approval is granted.
Each resume is a new invocation with its own state machine, span tree, and
budget accounting; the original invocation's consumed budget is terminal and
does not roll over automatically (the caller can choose to carry budget
forward in its own bookkeeping).

**Rationale.** The in-process stall alternative (seed §13.2 option a) is
incompatible with cancellation, streaming, and budget contracts. An
indefinitely stalled goroutine holds an open `<-chan InvocationEvent`
channel; the 16-event buffer fills if the caller drains lazily; budget
wall-clock continues to tick during a stall; `ctx.Done()` semantics become
ambiguous (abandon the approval request or honor it?). The returned-to-caller
model sidesteps all of these and aligns with prior art in the ecosystem
(LangGraph `interrupt()`, CrewAI `HumanFeedbackPending`) — both of which
delegate persistence and resume to the caller and do not hold an in-process
wait. Temporal's durable `workflow.WaitCondition()` pattern achieves in-flight
durability but requires adopting Temporal as infrastructure, which is outside
the praxis scope. Caller-owned resume keeps praxis free of persistence
dependencies.

**Alternatives considered.** (a) In-process stall with a new
`ApprovalPending` non-terminal state and a resume signal (seed §13.2 option
a) — rejected; see above. (b) Temporal-style durable wait — rejected;
pushes infrastructure dependency into a library scope.

**Implication for Phase 2.** `ApprovalRequired` is a terminal state sibling
to `Failed`, `Cancelled`, and `BudgetExceeded`, bringing the terminal count
to five (or expressed as a typed error leaving the state machine at 13 states
with four terminals — the exact representation is a Phase 2 call).

**Implication for the error taxonomy (amends seed §5).** The seed §5
`errors.TypedError` section declares "seven concrete types ship". D07
introduces an eighth concrete type, `ApprovalRequiredError`, which
implements `errors.TypedError` and is `errors.As`-compatible. It is
classified by `errors.Classifier` as **non-retryable** (terminal,
policy-class): it shares the retry posture of `PolicyDeniedError`. Phase 4
(Observability and Error Model) inherits this amendment and must reflect
the eight-type count in its taxonomy artifact. The seed document itself is
not edited; this decision log is the authoritative amendment record per
Phase 1's handoff contract.

**Phase 2 handoff note — state count reconciliation.** The seed §4.2 table
enumerates 13 numbered states (four terminal), while seed §1 and §8 refer
to an "11-state machine". Phase 2 must reconcile: either the two
cancellation-adjacent states (`Cancelled`, `BudgetExceeded`) are counted
as terminals separate from the 11 main states (yielding 11 + 2 =
13 total), or the count is off by two. With D07 adding
`ApprovalRequired`, Phase 2 will need to pick a coherent enumeration and
update the seed reference count via amendment note, not a seed edit.

---

## D08 — `budget.PriceProvider` hot-reload policy *(resolves seed §13.3)*

**Status:** decided
**Summary:** Per-invocation snapshot. `PriceProvider` is consulted at
invocation start for each `(provider, model, direction)` tuple needed, and
the result is cached in `InvocationContext` for the life of the invocation.
Mid-process updates affect new invocations only. In-flight invocations are
never re-priced.

**Decision.** At the start of each invocation, the orchestrator resolves the
price entries required for that invocation's configured model and caches
them in the `InvocationContext`. All token accounting within that invocation
uses the cached prices. A subsequent invocation sees whatever the
`PriceProvider` returns at its own start time. There is no live re-lookup
during an invocation, and no mechanism for the framework to notify an
in-flight invocation that prices have changed.

**Rationale.** Per-invocation snapshot is the audit-safer default. The
invocation's final `cost_estimate_micros` matches the prices in effect at its
start — a property that is load-bearing for audit-trail reproducibility and
for the `BudgetExceeded` classification (the caller needs to know which price
caused the breach). Prior art supports both patterns, but the split in the
research shows:

- Temporal's durable-workflow pattern snapshots config at workflow start for
  the same audit-stability reason.
- Cloud billing systems re-price at accounting time, but they have no
  in-flight invocation concept to worry about — they are post-hoc.
- `golang.org/x/time/rate.SetLimit` supports live updates, but a rate limit
  is a policy, not a cost-accounting record.

Snapshot also lets `budget.PriceProvider` promote to `frozen-v1.0`
mechanically — see D04. A live-lookup model would require a change
notification or re-read contract that would enlarge the interface surface
and delay its freeze.

**Alternatives considered.** (a) Live lookup on every token accounting call
— rejected; undermines audit reproducibility, enlarges the interface, and
creates race hazards on token accounting. (b) Live lookup with an explicit
"price changed mid-invocation" lifecycle event — rejected; adds a new event
type and a resume-or-abort caller decision that is disproportionate to the
operational frequency of price changes.

**Implication.** Promotes `budget.PriceProvider` from
`stable-v0.x-candidate` to `frozen-v1.0` at the Phase 3 close.

---

## D09 — Re-confirmation of "no plugins in v1" *(resolves seed §13.4)*

**Status:** decided
**Summary:** Re-confirmed. Extension in v1.x is exclusively by Go interface
implementation at build time. No `plugin.Open`, no WASM host, no reflection
registry, no RPC-based extensibility layer.

**Decision.** The framework ships no plugin, extension, or dynamic loading
mechanism in v1.x. Every extension point is a Go interface defined in the
stable surface (seed §5). Consumers extend the library by implementing those
interfaces in their own packages and injecting them at construction time.

**Rationale.** A plugin system introduces an API surface that cannot be
frozen in the Go sense: the host-plugin ABI, the lifecycle contract, the
failure modes of dynamically loaded code, and the security posture of
third-party binaries all become part of the stability commitment.
`plugin.Open` has well-known limitations (no Windows support, strict
version-match requirements, process-level failure domains). WASM hosts add a
runtime dependency. Reflection registries defer type safety to runtime. None
of these are compatible with the v1.0 freeze promise or the "compiler-enforced
decoupling" principle (D02 P2). Build-time composition gives consumers
everything a plugin system would, without the stability cost.

**Alternatives considered.** (a) Ship `plugin.Open`-based loader in v1.x —
rejected; the compatibility and cross-platform costs exceed any benefit. (b)
Defer the decision — rejected; the seed correctly identifies this as a
decision that future RFCs should be able to point at.

**Forward record.** Any future RFC proposing a plugin system in v2+ must
address this decision explicitly.

---

## D10 — Project name and module path

**Status:** decided (conditional, pending pre-v0.1.0 verification)
**Summary:** Project name `praxis`, module path `github.com/praxis-os/praxis`.
Conditional on pre-v0.1.0 brand/trademark review vs `usepraxis.app` and on
acquiring or confirming the currently empty `praxis-go` GitHub org.

**Decision.** The project ships under the name `praxis` with module path
`github.com/praxis-os/praxis`, subject to two preconditions that must be
satisfied before the first public commit (v0.1.0):

1. **GitHub org.** The `github.com/praxis-go` organization exists but is
   empty and has undisclosed ownership. Before v0.1.0 the maintainers must
   either (a) confirm ownership of that org, (b) reach GitHub support to
   acquire it if dormant, or (c) adopt an alternative org slug (e.g.,
   `praxis-kernel`, `praxis-lib`).
2. **Brand overlap review.** A commercial product at `usepraxis.app`
   operates under the name "Praxis" in the runtime-governance / AI-agent
   governance space — a semantically adjacent category. Before v0.1.0 the
   maintainers must confirm no trademark conflict and assess SEO /
   documentation-confusion risk. If the risk is unacceptable, the fallback
   names recorded in this decision are `praxis-kernel` and `invokekit`;
   the rename is a one-time cost paid before v0.1.0 per seed risk §14.3.

**Rationale.** The seed §14.3 risk is real: an independent researcher scan
identified one adjacent commercial brand (`usepraxis.app` — runtime
governance infrastructure for AI agents) and one empty GitHub org blocking
the canonical path. Neither is an immediate legal blocker, but both are
serious enough to require review before public release. Adopting the name
conditionally allows Phase 2 and Phase 3 to proceed without a dependency on
the outcome, while preventing the name from being written into v0.1.0
artifacts if review fails.

**Alternatives considered.** (a) Rename to `invokekit` pre-emptively —
rejected; the semantic fit of "praxis" (practice / applied action) to the
library's posture is strong, and the brand overlap is not a legal blocker.
(b) Defer the naming decision to Phase 6 — rejected; every phase artifact
going forward will embed the name in prose and cross-references.

**Tripwire for Phase 3.** The two preconditions above (GitHub org,
brand/trademark review) must be resolved before Phase 3 artifacts embed
the module path into godoc — every frozen interface in Phase 3 will be
namespaced under the module path, and a post-Phase-3 rename would
invalidate those godoc references. If the preconditions are unresolved at
Phase 3 start, Phase 3 MUST treat the module path as a placeholder token
(`MODULE_PATH_TBD`) and block its own sign-off on D10 resolution. This is
the concrete deadline that prevents the conditional status from leaking
into v0.1.0 artifacts.

---

## D11 — Positioning gaps praxis will not close

**Status:** decided
**Summary:** Seven explicit gaps vs LangChainGo, Google ADK for Go, Eino,
and direct SDK use — each framed as "praxis will not X; users wanting X
should use Y."

**Decision.** Full text in
[`02-positioning-and-principles.md`](02-positioning-and-principles.md) §5.
Headline entries: no prompt template engine, no multi-agent orchestration,
no RAG / vector store, no transport binding, no bundled price tables, no
graph / chain composition model, no plugin registry.

**Rationale.** The positioning table in the seed §2 describes what the
alternatives *do* but does not enumerate what praxis *will not try to do* in
response. Without the enumeration, the comparison invites feature-parity
requests. Section 5 of `02-positioning-and-principles.md` makes the
non-compete explicit so that reviewers can reject feature requests on
positioning grounds without a per-issue argument.

**Alternatives considered.** None — this decision is a catalog, not a
choice.

---

## D12 — Zero-wiring smoke-test promise

**Status:** decided
**Summary:** Yes. An `AgentOrchestrator` must be constructible and `Invoke`
callable with zero caller-supplied wiring beyond an `llm.Provider`.

**Decision.** The `AgentOrchestrator` constructor takes `llm.Provider` as
the single required dependency. All other dependencies — tool invoker,
policy hook, pre-LLM filter, post-tool filter, budget guard, price provider,
lifecycle event emitter, attribute enricher, credential resolver, identity
signer, classifier — are injected via functional options and default to
null / allow-all / noop implementations the library ships under names like
`tools.NullInvoker`, `hooks.AllowAllPolicyHook`, `telemetry.NullEmitter`,
and so on. The zero-option form (only `llm.Provider` supplied) must produce
a fully functional orchestrator that can complete an `Invoke` call
successfully against a live LLM with no tools and no hooks.

**Rationale.** The 30-line README example is the library's most-read code.
If construction requires wiring five interfaces before a first call, the
library fails its first impression for every evaluator. This decision shapes
Phase 3's constructor signature and is a direct consequence of principle P8
(D02).

**Alternatives considered.** (a) Require an explicit options argument with
no defaults — rejected; kills the smoke path and contradicts P8. (b)
Provide a separate `orchestrator.NewForSmokeTest()` helper — rejected;
duplicates the constructor surface and invites drift between the "easy" and
"real" paths.

---

## D13 — Interface stability tiering policy

**Status:** decided
**Summary:** Three tiers — `frozen-v1.0`, `stable-v0.x-candidate`,
`post-v1`. The authoritative per-interface tier table is in D04 and
`04-v1-freeze-surface.md`.

**Decision.** The praxis interface surface is classified into three tiers:

- **`frozen-v1.0`**: interface shape is frozen at v1.0 as a semver-level
  commitment to downstream consumers. Any breaking change requires a v2
  module path. Adding a method requires a new embedded interface (e.g.,
  `llm.ProviderV2 { llm.Provider; NewMethod(...) }`).
- **`stable-v0.x-candidate`**: shape can still move during v0.x. Intended to
  reach `frozen-v1.0` before v1.0.0 but explicitly not committed yet, with a
  named gating condition.
- **`post-v1`**: explicitly not part of the v1.0 freeze. Ships experimental
  and may change on any minor tag post-v1.

**Rationale.** Without a named tiering policy, every interface would need
per-interface relitigation in Phase 3. The three-tier model maps cleanly
onto the "frozen / candidate / experimental" distinctions other mature Go
libraries use, and the `post-v1` tier gives future phases a clean place to
land new interfaces without forcing them through the freeze.

**Alternatives considered.** (a) Two tiers (frozen vs not) — rejected;
collapses "about to freeze" and "will never freeze in v1" into one class.
(b) Five-tier scheme — rejected as over-engineered.

---

## D14 — Azure OpenAI parity commitment

**Status:** decided
**Summary:** Best-effort. Azure OpenAI is tested via `openai.Provider` with
base-URL configuration but is not a v1.0 parity guarantee.

**Decision.** `openai.Provider` supports Azure OpenAI via base-URL
configuration. The shared `llm/conformance/` suite runs against Azure in CI
when credentials are available. The v1.0 stability commitment covers the
`llm.Provider` interface shape only; it does not commit to feature parity
between OpenAI direct and Azure OpenAI. Capability differences
(structured outputs, parallel tool calls, reasoning models, model
availability) are documented in the `openai/` adapter's godoc and release
notes as a compatibility matrix maintained on a best-effort basis.

**Rationale.** Azure OpenAI's feature parity with OpenAI lags by weeks to
months on new capabilities. Committing to parity at v1.0 would bind the
project to a dependency over which it has no control. Best-effort with an
explicit documented matrix gives Azure users a reasonable path without
overcommitting.

**Alternatives considered.** (a) Full parity commitment — rejected; binds
to an external roadmap. (b) Ship a separate `azureopenai.Provider` adapter
— rejected for v1.0; unnecessary interface duplication when base-URL
configuration is sufficient for the common case. Reconsiderable post-v1 if
divergence grows.

---

## D15 — *Released.*

**Status:** released
**Summary:** D15 was held as a reviewer reserve in `00-plan.md` and is not
needed. The next phase (Core Runtime Design) will allocate D15+.

---

## Adopted decisions summary (for `roadmap-status`)

The entries below reflect the **current working position** for each
decision. Per the Amendment protocol above, any of these may be revisited
in a later phase if new evidence or downstream constraints justify it.


| ID | Title | Status |
|---|---|---|
| D01 | Positioning statement | decided |
| D02 | Design principles (8, incl. zero-wiring smoke path) | decided |
| D03 | Target consumer archetype + anti-persona | decided |
| D04 | v1.0 freeze surface (12/14 frozen) | decided |
| D05 | Non-goals for v1.x | decided |
| D06 | `tools.Invoker` tool-name on `ToolCall` | decided |
| D07 | `requires_approval` returned-to-caller semantics | decided |
| D08 | `PriceProvider` per-invocation snapshot | decided |
| D09 | No plugins in v1 (re-confirmed) | decided |
| D10 | Name `praxis` / module `github.com/praxis-os/praxis` | decided (conditional) |
| D11 | Positioning gaps not closed | decided |
| D12 | Zero-wiring smoke-test promise | decided |
| D13 | Three-tier interface stability policy | decided |
| D14 | Azure OpenAI best-effort parity | decided |
| D15 | *released* | — |

Next phase (Phase 2 — Core Runtime Design) opens D15 onwards.
