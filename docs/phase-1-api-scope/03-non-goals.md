# Phase 1 — Non-goals for v1.x

This document enumerates what praxis will **not** be in v1.x. Each non-goal
is a positive commitment: it names the interface that would have to exist if
the non-goal were reversed, so that future reviewers can reject feature
requests on charter grounds without per-issue re-litigation.

Non-goals are binding for v1.x. Revisiting any of them requires a new
planning phase and an explicit amendment to this document.

---

## Non-goal 1 — No prompt template engine

**Commitment.** praxis will not provide a prompt template engine, a prompt
versioning system, a prompt management API, or any built-in support for
prompt composition beyond accepting fully formed `Message` values on the
`Invoke` path.

**Interface that would exist if reversed.** `prompts.Template`,
`prompts.Registry`, `prompts.Renderer`.

**Rationale.** Prompt construction is a product concern, not an invocation
kernel concern. Teams vary wildly in how they manage prompts (in-code,
in-YAML, in a database, in a dedicated service). Any choice praxis made
would be wrong for most of them. The invocation kernel takes messages as
input; constructing those messages is where product teams want their own
control.

---

## Non-goal 2 — No multi-agent orchestration

**Commitment.** praxis will not provide agent-to-agent coordination, a
multi-agent protocol, role-based agent teams, shared memory across agents,
or any form of mesh orchestration. Each `Invoke` call is a single agent
invocation.

**Interface that would exist if reversed.** `agents.Team`,
`agents.Coordinator`, `agents.Router`, a protocol interface for agent
message passing.

**Rationale.** Multi-agent orchestration is a distinct problem with its own
libraries (Google ADK for Go is the Go-ecosystem canonical choice). Scoping
praxis to the single-invocation kernel keeps the state machine finite and
the contract freezable. Chaining multiple praxis invocations to build a
multi-agent system is entirely possible at the caller level; praxis just
does not participate in the coordination.

---

## Non-goal 3 — No RAG, vector stores, or embedding management

**Commitment.** praxis will not ship a vector store interface, an embedding
client, a retrieval-augmented generation helper, a document loader, or any
memory abstraction beyond the message history the caller passes to
`Invoke`.

**Interface that would exist if reversed.** `rag.Store`, `rag.Embedder`,
`rag.Retriever`, `memory.Store`.

**Rationale.** The vector-store ecosystem is fragmented and moves faster
than praxis's freeze cycle. A v1.0 interface would either lock in yesterday's
assumptions or force an abstraction so generic it gives no leverage. Users
who need RAG integrate a dedicated store independently and pass retrieved
context into the `Invoke` call as part of the message list.

---

## Non-goal 4 — No transport binding (HTTP, gRPC, SSE server)

**Commitment.** praxis will not ship an HTTP handler, a gRPC service
definition, a Server-Sent Events server implementation, or any other
transport layer for exposing agent calls over a network.

**Interface that would exist if reversed.** `transport.Handler`,
`transport.SSEWriter`, a mux-level binding.

**Rationale.** The transport layer is a framework choice
(net/http, chi, echo, gRPC, Connect, custom). Any binding praxis shipped
would be wrong for most users and would drag transport-framework
dependencies into the core module. The `InvokeStream` channel output is
the framework boundary; bridging that channel to SSE or WebSocket is a
30-line example in the documentation, not a framework feature.

---

## Non-goal 5 — No bundled price tables for commercial providers

**Commitment.** praxis will not ship a built-in price table for Anthropic,
OpenAI, Azure OpenAI, or any other commercial LLM provider. The only
`budget.PriceProvider` implementations shipped are `NullPriceProvider`
(always zero) and `StaticPriceProvider` (loads from a caller-supplied
table).

**Interface that would exist if reversed.** `pricing.AnthropicTable`,
`pricing.OpenAITable`, or a bundled `pricing.EmbeddedProvider` pre-loaded
with rates.

**Rationale.** Pricing is a commercial relationship between the caller and
the vendor. Rates differ by contract, region, volume discount, and time.
Bundling rates in the library would be stale the day after a price change
and would give users a false sense of accuracy. The framework owns the
accounting mechanism; the caller owns the numbers.

---

## Non-goal 6 — No graph or chain composition model

**Commitment.** praxis will not ship a graph, DAG, chain, pipeline, or
any other declarative composition model for LLM calls. The invocation
state machine is fixed, not composable.

**Interface that would exist if reversed.** `graph.Node`, `graph.Edge`,
`graph.Runner`, `chain.Step`.

**Rationale.** Graph composition is exactly the scope LangChainGo and Eino
address. Replicating their model would be redundant and would inflate the
interface surface past any credible v1.0 freeze. praxis's state machine is
deliberately a single-purpose invocation kernel; callers who need graph
composition can either use an alternative framework or build their own
graph on top of praxis invocations.

---

## Non-goal 7 — No plugin or runtime extension registry

**Commitment.** praxis will not ship a plugin system, extension registry,
dynamic loader, `plugin.Open` integration, WASM host, or reflection-based
extensibility mechanism. Extension in v1.x is exclusively by Go interface
implementation at build time. (Formal decision: D09.)

**Interface that would exist if reversed.** `plugins.Loader`,
`plugins.Registry`, a WASM host contract.

**Rationale.** Plugin systems introduce API surface that cannot be frozen
in the Go sense: the host-plugin ABI, the lifecycle contract, the failure
modes of dynamically loaded code, and the security posture of third-party
binaries all become part of the stability commitment. Build-time interface
composition gives users everything a plugin system would without the
stability cost. See D09 for full rationale and forward record.

---

## Non-goal audit

Every phase-3 interface proposal must be checked against this list. If a
proposed interface would implement any of the interfaces-if-reversed named
above, that is a non-goal violation and the proposal is rejected unless
this document is amended first.

The list is expected to grow — not shrink — as design phases progress. New
non-goals are welcome; reversing an existing non-goal requires a documented
amendment with its own decision ID.
