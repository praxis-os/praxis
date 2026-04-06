# Phase 3: Interface Contracts

## Goal

Define the complete Go method signatures, struct shapes, type representations,
default implementations, composability rules, and concurrency contracts for all
14 public v1.0 interfaces so that implementation (post-Phase 6) can proceed
without re-opening any interface question.

## Scope

### In scope

- Final Go method signatures for all 14 interfaces enumerated in seed SS5 and
  tiered in D04 / `04-v1-freeze-surface.md`.
- The `hooks.PolicyHook` `Decision` type shape per D17 constraints
  (`Allow`/`Deny`/`RequireApproval` minimum, opaque metadata forwarding,
  value-variant).
- The `InvocationEvent` struct with final Go types per Phase 2
  `03-streaming-and-events.md` SS6 (EventType representation, field types,
  package placement).
- The `BudgetSnapshot` struct (value-copyable, cheap to allocate) per D25/D27.
- The `state.State` Go representation per Phase 2 `02-state-machine.md` SS6
  advisory (typed integer, `iota`, `IsTerminal()` predicate, adjacency table
  exposure).
- The `ApprovalRequiredError` concrete type with conversation snapshot field
  per D07/D17.
- All eight concrete `errors.TypedError` types (seven from seed + D07's
  `ApprovalRequiredError`), their `Kind()` values, `HTTPStatusCode()` hints,
  and sub-kind structure where applicable (`ToolError` sub-kinds).
- `errors.Classifier` method signature and precedence rules for propagated
  typed errors (CP5 shape half from `06-composition-patterns.md`).
- `ToolResult` shape with enough structure to embed a `TypedError` for
  nested-invocation error propagation (CP5).
- `InvocationContext` struct: what it carries, what it does not carry,
  how it relates to D06 (tool name on `ToolCall`, not `InvocationContext`).
- `InvocationRequest` and `InvocationResult` struct shapes.
- `AgentOrchestrator` constructor signature: `llm.Provider` as sole required
  dependency, functional options for all others per D12.
- Null/default implementations for every interface: `tools.NullInvoker`,
  `hooks.AllowAllPolicyHook`, no-op filters, `budget.NullGuard`,
  `budget.NullPriceProvider`, `telemetry.NullEmitter`,
  `telemetry.NullEnricher`, `credentials.NullResolver`,
  `identity.NullSigner`, default `errors.Classifier`.
- Concurrency contract clause for every interface godoc per Phase 2
  `05-concurrency-model.md` SS3.2.
- `budget.Guard` method surface designed for composition-friendliness per
  CP3 (no assumption of one Guard per invocation).
- `budget.PriceProvider` final signature (expected to promote to
  `frozen-v1.0` at Phase 3 close per D08).
- `llm.Provider` agnostic request/response/message type shapes
  (`LLMRequest`, `LLMResponse`, `LLMToolCall`, `LLMToolResult`, `Message`,
  `MessagePart`, `Capabilities` struct).
- Package placement decisions: which types live in which packages per seed SS7.
- D10 tripwire: resolve module-path precondition or use `MODULE_PATH_TBD`
  placeholder.
- Whether resume after `ApprovalRequired` is a fresh `Invoke` or a dedicated
  method (D07 says fresh `Invoke`; Phase 3 confirms or amends).
- Invocation ID generation strategy (UUIDv7 or similar).

### Out of scope

- OTel span tree, Prometheus metrics, slog redaction (Phase 4).
- Span re-attachment in `internal/ctxutil.DetachedWithSpan` (Phase 4, C1).
- Error-to-event mapping and classifier precedence rules beyond the
  shape-level question (Phase 4).
- Token-dimension budget overshoot documentation (Phase 4, C3).
- Credential zero-on-close mechanics beyond the interface shape (Phase 5).
- `credentials.Resolver.Fetch` soft-cancel `context.WithoutCancel`
  requirement documentation (Phase 5, C4).
- JWT claim set, key lifecycle, `identity.Signer` promotion (Phase 5).
- Release process, CI pipeline, contribution model (Phase 6).
- Implementation code. This is a design phase.

### Composability check

Phase 3 owns the surface half of CP3 (budget Guard composition) and the
shape half of CP5 (ToolResult admits typed-error embedding). Both must be
verified in the review. See `docs/phase-1-api-scope/06-composition-patterns.md`.

### Forward-carried concerns consumed

- **C2** (parallel tool-call completion ordering): `InvocationEvent` godoc
  must state that per-tool completion ordering is not preserved under
  parallel dispatch.
- **C5** (`golang.org/x/sync/errgroup` dependency): recorded, no action
  needed in Phase 3.

## Key Questions

1. What is the Go representation of `EventType` -- typed string (`type
   EventType string`) or typed integer (`type EventType uint8` with `iota`)?
   Typed string is grep-friendly and JSON-serializable without a lookup
   table; typed integer is cheaper in hot paths. Which trade-off wins for
   a library whose events transit a channel?
2. Where does `InvocationEvent` live -- `praxis` root package or
   `orchestrator` sub-package? The seed SS7 shows `orchestrator/` as the
   public facade, but the event types are referenced by every consumer
   draining a stream. Import-cycle risk if it lives alongside
   `AgentOrchestrator`.
3. What is the exact shape of `hooks.PolicyHook.Decision`? D17 pins three
   constraints (value-variant, opaque metadata, `Allow`/`Deny`/`RequireApproval`
   minimum). Is it a struct with a `Verdict` enum + `Metadata map[string]any`,
   or a sealed-variant approach, or something else?
4. Does `budget.Guard` expose a `Check` method called by the loop, or does
   the loop call dimension-specific `AddTokens`/`AddToolCall`/`AddCost`
   methods and the Guard decides internally? The composition property (CP3)
   is easier with a self-contained `Check` that reads accumulated state.
5. What does the `tools.InvocationContext` carry? It must include at minimum:
   invocation ID, budget snapshot (read-only), OTel span context (for
   nested trace correlation, CP1), and the outer identity token if present
   (for identity chaining, CP6). What else, if anything?
6. Does `AgentOrchestrator` have a `Close() error` method? Phase 2
   `05-concurrency-model.md` SS5 notes no background goroutines, so `Close`
   may be unnecessary. But injected dependencies (LLM providers, credential
   resolvers) may need lifecycle hooks.
7. How is the conversation snapshot in `ApprovalRequiredError` represented?
   D17 requires "minimum conversation snapshot content specification". Is it
   the full `[]Message` history, a summary struct, or a caller-provided
   snapshot function?
8. What is the `BudgetSnapshot` shape? It must be value-copyable and cheap.
   Fields: consumed wall-clock, consumed tokens (input + output), tool-call
   count, estimated cost in micro-dollars, plus an `ExceededDimension` field
   for the terminal event (INV-20).
9. Is `llm.Provider.Stream` a separate method returning a
   `<-chan LLMStreamChunk`, or does the orchestrator handle streaming
   internally via `Complete` with a streaming option? Seed SS5 lists both
   `Complete` and `Stream` as separate methods.
10. What is the filter chain return shape? `hooks.PreLLMFilter` and
    `hooks.PostToolFilter` return `(filtered, decisions, err)` per seed SS5.
    What are the Go types for `filtered` and `decisions`?

## Decisions Required

Phase 3 allocates decision range D31-D50. Expected decisions:

| ID | Topic |
|---|---|
| D31 | `EventType` Go representation (typed string vs typed integer) |
| D32 | `InvocationEvent` package placement and struct shape |
| D33 | `hooks.PolicyHook.Decision` type shape |
| D34 | `budget.Guard` method surface (Check vs dimension-additive) |
| D35 | `BudgetSnapshot` struct shape |
| D36 | `tools.InvocationContext` field set |
| D37 | `AgentOrchestrator` constructor signature and functional options |
| D38 | `InvocationRequest` and `InvocationResult` shapes |
| D39 | `ApprovalRequiredError` and conversation snapshot shape |
| D40 | `tools.ToolResult` shape (including typed-error embedding for CP5) |
| D41 | `llm.Provider` method surface and agnostic type shapes |
| D42 | Filter chain return types (`PreLLMFilter`, `PostToolFilter`) |
| D43 | `state.State` Go type and adjacency table exposure |
| D44 | `errors.Classifier` method signature |
| D45 | `credentials.Resolver` and `Credential` interface shapes |
| D46 | `identity.Signer` method surface (stable-v0.x-candidate shape) |
| D47 | `budget.PriceProvider` final signature and `frozen-v1.0` promotion |
| D48 | `AgentOrchestrator.Close()` inclusion or exclusion |
| D49 | Invocation ID generation strategy |
| D50 | Resume-after-approval confirmation (fresh `Invoke` per D07, or amend) |

Reserve: D51-D52 if needed.

## Assumptions

- **D10 (module path) remains conditional.** Phase 3 will use
  `MODULE_PATH_TBD` in any godoc-facing references until the precondition
  is resolved. This is a Phase 1 tripwire, not a Phase 3 blocker.
- **Phase 2 decisions D15-D28 hold as-is.** Phase 3 consumes them without
  amendment unless a contradiction is discovered during interface design.
- **Go 1.23+ is the floor.** `context.WithoutCancel`, generics, and
  `log/slog` are available without version guards.
- **No generics on public interfaces in v1.0.** The seed does not use
  generics on any interface. Introducing generics would complicate the
  freeze commitment (generic type parameters are part of the API surface).
  This is a weak assumption -- if a concrete interface benefits materially
  from generics, it can be reconsidered.
- **`golang.org/x/sync/errgroup` is the only runtime dependency** beyond
  stdlib (C5). Phase 3 does not introduce additional dependencies.

## Risks

### Critical

- **Interface over-design.** 14 interfaces with full method signatures,
  structs, and composability rules is a large surface to freeze in one
  phase. Risk: analysis paralysis or premature commitment to shapes that
  Phase 4/5 will need to amend. Mitigation: focus on minimum viable
  signatures; leave Phase 4/5-owned semantics (span tree, JWT claims)
  as documented extension points rather than premature fields.
- **D10 module-path leak.** If godoc references embed the module path
  and D10 resolves differently, Phase 3 artifacts need a search-replace.
  Mitigation: strict `MODULE_PATH_TBD` discipline.

### Secondary

- **`budget.PriceProvider` promotion risk.** D08 resolved the semantic
  question (per-invocation snapshot), but Phase 3 must finalize the exact
  method signature. If the signature proves inadequate during Phase 4's
  cost-accounting design, promotion to `frozen-v1.0` slips.
- **Filter chain complexity.** The `PreLLMFilter` / `PostToolFilter` return
  type (`filtered, decisions, err`) is the least-specified interface in the
  seed. Getting the shape wrong here cascades into Phase 4's error-to-event
  mapping.
- **CP5 tension.** Embedding a `TypedError` inside `ToolResult` for
  nested-invocation propagation adds structure to `ToolResult` that purely
  local tool calls do not need. Risk of over-engineering the common case
  for the composition case.

## Deliverables

- `00-plan.md` -- this file.
- `01-decisions-log.md` -- D31-D50+ decisions with rationale.
- `02-orchestrator-api.md` -- `AgentOrchestrator` facade: constructor,
  `Invoke`, `InvokeStream`, `Close` (if included), functional options,
  `InvocationRequest`, `InvocationResult`.
- `03-llm-provider.md` -- `llm.Provider` interface, agnostic types
  (`LLMRequest`, `LLMResponse`, `Message`, `MessagePart`, `LLMToolCall`,
  `LLMToolResult`, `Capabilities`), adapter conformance contract.
- `04-hooks-and-filters.md` -- `hooks.PolicyHook`, `Decision` type,
  `PreLLMFilter`, `PostToolFilter`, filter chain return types, null
  defaults.
- `05-budget-interfaces.md` -- `budget.Guard`, `budget.PriceProvider`,
  `BudgetSnapshot`, composition rules (CP3).
- `06-tools-and-invocation-context.md` -- `tools.Invoker`, `ToolCall`,
  `ToolResult` (with CP5 error embedding), `InvocationContext`.
- `07-errors-and-classifier.md` -- `errors.TypedError`, eight concrete
  types, `ErrorKind` enum, `Classifier`, `ApprovalRequiredError` with
  conversation snapshot.
- `08-telemetry-interfaces.md` -- `telemetry.LifecycleEventEmitter`,
  `telemetry.AttributeEnricher`, `InvocationEvent`, `EventType`.
- `09-credentials-and-identity.md` -- `credentials.Resolver`, `Credential`,
  `identity.Signer`, null defaults.
- `10-state-types.md` -- `state.State`, `state.Machine` public surface,
  adjacency table, `IsTerminal()`.
- `11-defaults-and-construction.md` -- catalog of all null/default
  implementations, zero-wiring construction path, functional option
  inventory.
- `REVIEW.md` -- phase review (unnumbered, read last).

## Recommended Subagents

1. **api-designer** -- Phase 3 is the API surface definition phase. The
   api-designer subagent owns method signatures, composability rules,
   and the freeze-readiness assessment for all 14 interfaces.
2. **go-architect** -- Package placement decisions (where types live,
   import-cycle avoidance), internal vs exported boundaries, and the
   functional-option pattern for the constructor all require the
   go-architect's perspective.

## Exit Criteria

1. All decisions D31-D50 (plus any reserve used) are adopted with
   rationale.
2. Every v1.0 interface has a complete Go method signature (parameters,
   return types, documented contracts).
3. Every interface has a named null/default implementation.
4. `budget.PriceProvider` is promoted to `frozen-v1.0` or an explicit
   blocker is recorded.
5. `identity.Signer` has a `stable-v0.x-candidate` shape (promotion
   deferred to Phase 5).
6. CP3 (budget composition) and CP5 (typed-error propagation through
   ToolResult) are satisfied at the interface level.
7. C2 (parallel tool-call ordering) is documented in the `InvocationEvent`
   godoc spec.
8. The reviewer subagent returns PASS.
9. `REVIEW.md` verdict is READY.
10. Banned-identifier grep returns zero matches across all Phase 3
    artifacts (excluding negation-mentions in compliance declarations).
11. No Phase 2 decision (D15-D28) is contradicted without an explicit
    amendment recorded in Phase 3's decisions log.
