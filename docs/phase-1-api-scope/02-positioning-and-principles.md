# Phase 1 — Positioning and Principles

This document is the README-ready source of truth for what praxis *is*, what
it *is not*, who it is for, and the principles that govern its interface
design. It refines seed §1–§3 and is the canonical text that README.md and
godoc should quote when praxis first ships.

---

## 1. Positioning statement *(D01)*

`praxis` is a production-grade Go library for orchestrating LLM agent
invocations with enterprise guardrails built into the invocation kernel
rather than composed on top of it. It owns a single agent call from request
to terminal state, enforcing policy, cost, security, and observability
contracts by construction through a typed state machine and stable Go
interfaces. It is provider-agnostic, ships Anthropic and OpenAI adapters,
and delegates every consumer-specific concern — policy wiring, credential
management, identity attribution, tool routing — to caller-supplied
implementations of small, stable interfaces. `praxis` is not a general LLM
application framework, not a prompt template engine, not a multi-agent
coordination protocol, and not a replacement for direct SDK use when
auditability and cost control are not requirements.

---

## 2. Design principles *(D02)*

The v1.0 library is governed by eight principles. Principles 1–7 are carried
forward from seed §3 unchanged in substance. Principle 8 is added in Phase 1
because the seed implies it in §5 but does not state it at governance level,
and the smoke-test promise is load-bearing for DX.

1. **Generic first, opinionated where correctness demands it.** The state
   machine, error taxonomy, and budget dimensions are fixed because they
   carry correctness load. Every consumer-specific concern sits behind an
   interface.
2. **Compiler-enforced decoupling.** No consumer-specific identifier appears
   in framework code. CI greps for banned strings. The framework does not
   know the name of any consumer.
3. **Interfaces at every security seam.** Policy, credentials, identity
   signing, telemetry attribution, and price lookup are interfaces with null
   default implementations. Concrete wiring is the caller's responsibility.
4. **Typed errors, no `interface{}` payloads.** Every framework error
   implements a stable typed contract compatible with `errors.Is` and
   `errors.As`.
5. **Mandatory observability.** Every state transition emits a span and a
   lifecycle event. Silent paths are a bug.
6. **No plugins in v1.** Extension is by Go interface implementation at
   build time only. No `plugin.Open`, no WASM host, no reflection registry.
7. **Backward compatibility is a v1.0 commitment, not a v0 one.** Until v1.0
   the API may break on any minor tag. After v1.0 the interface surface is
   frozen and v2 requires a module path bump.
8. **Zero-wiring smoke path.** Every interface ships a null or noop default
   implementation. An `AgentOrchestrator` must be constructible and callable
   with zero caller-supplied wiring beyond an `llm.Provider`. (See D12.)

---

## 3. Target consumer archetype *(D03)*

The archetype consumer is a **platform engineering or ML infrastructure
team inside an organization that already runs LLM-backed features in
production**. Concretely:

- The team owns a shared internal platform on which product teams build
  LLM-powered features. They are not building the product; they are
  building the substrate.
- They have already shipped LLM calls to production using a direct SDK or a
  thin in-house wrapper. The prototype phase is behind them.
- Their pain is not "how do I call an LLM" — their pain is auditability
  (who called what, with which policy decision, with which budget outcome),
  cost control (per-tenant, per-model, per-feature accounting), and
  security posture (credential handling, untrusted tool output, identity
  attribution across the call graph).
- They have a compliance, security, or finance stakeholder asking questions
  that the current ad-hoc integration cannot answer without custom work on
  every call site.
- They write Go.

This archetype drives every decision in Phase 1. The interface surface is
small because the archetype team will implement several of the interfaces
themselves to wire praxis into their existing observability, policy, and
credential systems. The state machine is opinionated because the archetype
team wants correctness load-bearing behavior to be enforced by the framework
rather than reviewed on every PR. The zero-wiring smoke path exists because
even the archetype team still needs a 30-line example to evaluate praxis
before committing to integration work.

---

## 4. Anti-persona *(D03)*

Three explicit anti-personas. Users in any of these categories are not the
praxis audience; the README must name them and redirect them.

### 4.1 First-time LLM integration

Developers integrating an LLM call into an application for the first time,
who want the simplest possible path from "nothing" to "working API call."
**Redirect:** use `anthropic-sdk-go` or `openai-go` directly. praxis's value
proposition starts when ad-hoc glue stops scaling; before that point it is
overhead.

### 4.2 Multi-agent mesh builders

Teams building systems where multiple agents communicate with each other
over a protocol (agent-to-agent delegation, role-based agent teams, mesh
coordination). **Redirect:** use Google ADK for Go, which is purpose-built
for that scope. praxis handles exactly one agent call at a time; chaining
across agents is the caller's concern.

### 4.3 Python-LangChain ergonomic expectations

Users expecting a Python-LangChain-style API — composable chains, graph
primitives, prompt templates, vector store integration, tool libraries,
memory abstractions. **Redirect:** use LangChainGo or Eino. praxis is
deliberately narrower and will never ship a chain or graph composition
model. Users arriving with LangChain habits will find the interface surface
sparse and the non-goals list long.

---

## 5. Positioning gaps praxis will not close *(D11)*

Relative to LangChainGo, Google ADK for Go, Eino, and direct SDK use, praxis
will explicitly not try to close the following gaps. Each is framed as a
"will not X; use Y" pair so the README can redirect users cleanly.

1. **Prompt templating and management.** praxis will not provide a prompt
   template engine, prompt versioning system, or prompt management API.
   Users wanting prompt management should use a dedicated library or
   maintain prompts in their own codebase. The invocation kernel accepts
   fully formed messages; prompt construction is the caller's concern.

2. **Multi-agent orchestration.** praxis will not provide agent-to-agent
   coordination, agent mesh primitives, or role-based agent teams. Users
   wanting agent mesh coordination should use Google ADK for Go. Each
   `Invoke` call is one agent, one invocation; chaining is the caller's
   concern.

3. **RAG and vector store primitives.** praxis will not ship a vector store
   interface, embedding management, or retrieval-augmented generation
   helpers. Users wanting RAG should integrate a dedicated store (Weaviate,
   Qdrant, pgvector, etc.) independently and inject retrieved context into
   the messages passed to `Invoke`.

4. **Transport binding.** praxis will not provide an HTTP handler, gRPC
   service, Server-Sent Events server, or any other transport layer. Users
   exposing agent calls over a network should write their own handler using
   the `InvokeStream` channel output. The 16-event channel is the framework
   boundary.

5. **Bundled price tables.** praxis will not ship a built-in price table for
   any commercial LLM provider. Users must supply their own pricing data via
   `budget.StaticPriceProvider` or a custom `budget.PriceProvider`. The
   framework owns the accounting mechanism; the caller owns the commercial
   relationship.

6. **Graph or chain composition model.** praxis will not ship a
   Python-style chain, graph, or DAG composition system. Users coming from
   LangChain expecting composable pipelines should use LangChainGo or Eino.
   The state machine is a fixed kernel, not a composable graph.

7. **Plugin or extension registry.** praxis will not ship a plugin system,
   extension registry, or any runtime-loaded code path. Users wanting
   dynamic extensibility should compose at the Go binary level by injecting
   custom implementations of the provided interfaces at construction time.
   Build-time composition is the only extension path. (See D09.)

---

## 6. What praxis will do *(complement of §5)*

For completeness, and so the README can contrast clearly: praxis commits to
owning the **invocation kernel**. The things it will do in v1.0 are exactly:

- Drive a typed state machine through a single agent invocation, start to
  terminal.
- Enforce policy hooks at four named lifecycle phases.
- Run pre-LLM and post-tool filter chains.
- Enforce a four-dimensional budget (wall-clock, tokens, tool calls, cost
  estimate) with explicit terminal states on breach.
- Classify errors into a typed taxonomy and drive differentiated retry
  policy.
- Emit OpenTelemetry spans and lifecycle events on every state transition.
- Resolve credentials per tool call with memory-zeroed lifecycle.
- Optionally sign tool calls with an Ed25519 JWT for identity assertion.
- Ship provider adapters for Anthropic and OpenAI (the latter also covering
  Azure OpenAI on a best-effort basis — see D14).

Everything else is a caller concern.
