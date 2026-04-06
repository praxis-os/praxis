# Review: Phase 3 — Interface Contracts

## Overall Assessment

Phase 3 delivers a complete, internally consistent API surface for praxis v1.0.
The 14 interfaces are fully specified with Go method signatures, concurrency
contracts, and null defaults. The independent reviewer subagent surfaced two
structural blockers (root-vs-sub-package contradiction, OPEN-1 import cycle)
and seven important findings. All have been addressed in-place via D51
(package layout resolution and OPEN-1 fix) and D52 (seed amendments for
interface-to-struct change, event vocabulary extension, and PriceProvider
promotion). The decoupling contract grep returns zero matches. The phase is
ready to close.

## Critical Issues

None remaining.

The reviewer subagent raised two blockers in its pass; both were resolved via
D51 and D52:

1. **Root-vs-sub-package contradiction (resolved by D51).** D32 placed the
   facade in the root `praxis` package; the go-architect analysis showed this
   creates a `praxis ↔ hooks` import cycle. D51 adopts the go-architect's
   layout: root holds types, `orchestrator/` holds the facade and constructor.

2. **OPEN-1 import cycle (resolved by D51).** `hooks.PolicyInput` originally
   carried a `praxis.InvocationRequest`, creating a cycle. D51 replaces this
   with projection fields (`Model`, `SystemPrompt`, `Messages`, `Metadata`)
   that reference only `llm` and `tools` types — both are import-safe.

## Important Findings (all addressed)

3. **Option function name inconsistency.** The go-architect document used
   different option names (`WithLifecycleEmitter`, `WithInvoker`) from the
   canonical list in `02-orchestrator-api.md`. **Fix:** A reconciliation note
   was added to the go-architect document; `02-orchestrator-api.md` is
   authoritative. `WithInvocationIDFunc` was added to the canonical list.

4. **`CancellationError.Kind_` non-idiomatic naming.** The exported `Kind_`
   field with a trailing underscore was non-idiomatic Go. **Fix:** Replaced
   with an unexported `cancelKind` field and a `CancelKind()` accessor method
   in `07-errors-and-classifier.md`.

5. **`EventTypePIIRedacted` and `EventTypePromptInjectionSuspected` missing.**
   Seed §5 lists these as framework-defined event constants but D18's 19-event
   set omitted them. **Fix:** D52b extends the vocabulary to 21 constants,
   adding `EventTypePIIRedacted` and `EventTypePromptInjectionSuspected` with
   `filter.*` namespace. Phase 4 owns the emission semantics.

6. **`budget.Guard.Check` calling-point ambiguity.** The godoc said "called at
   LLMCall" without specifying before or after the LLM call. **Fix:** Clarified
   in `05-budget-interfaces.md` that Check fires after the LLM response is
   received and tokens have been recorded (at the LLMCall → ToolDecision
   boundary). Token overshoot (C3) is an explicit consequence.

7. **Zero-wiring example used wrong Message fields.** The go-architect document
   used `llm.Message{Role: "user", Content: "Hello"}` and
   `result.FinalMessage()`, neither of which exist. **Fix:** Updated to use
   `llm.Message{Role: llm.RoleUser, Parts: ...}` and `result.Response.Parts[0].Text`.

8. **BudgetSnapshot field names inconsistent.** The go-architect document used
   different field names from the canonical definition in `05-budget-interfaces.md`.
   **Fix:** Reconciliation note added; `05-budget-interfaces.md` is authoritative.

9. **`AgentOrchestrator` interface → concrete struct without amendment.**
   Seed §5 implies an interface; D37 adopted a concrete struct. **Fix:** D52a
   records the formal amendment and documents the mockability story (consumer-
   defined narrow interfaces).

## Minor Findings

10. **`ApprovalRequiredError` private `err` field inconsistency.** D39 in the
    decisions log included a private `err` field; `07-errors-and-classifier.md`
    omitted it. **Resolution:** The canonical definition in
    `07-errors-and-classifier.md` is authoritative. `ApprovalRequired` is not a
    failure; `Unwrap()` returns nil. D39's prose `err` field is editorial; the
    canonical struct is the source of truth.

11. **`tools → praxis` spurious edge in go-architect DAG.** The Mermaid graph
    showed a `tools --> praxis` edge that the §5.2 text did not justify.
    **Resolution:** The reconciliation note in the go-architect document marks
    its details as advisory; the authoritative DAG is in D51.

## Phase 2 Compatibility

No Phase 2 decision (D15–D28) is contradicted.

- D15 (14 states): `state.State` in `10-state-types.md` has exactly 14 states
  with the D15 ordering (non-terminals first, terminals last).
- D16 (adjacency table): `Transitions(s)` in `10-state-types.md` encodes the
  exact D16 table.
- D17 (ApprovalRequired terminal): `ApprovalRequiredError` in
  `07-errors-and-classifier.md` implements `TypedError`. `Decision` in
  `04-hooks-and-filters.md` carries `VerdictRequireApproval`. The three D17
  constraints (value-variant, opaque metadata, minimum variants) are satisfied.
- D18 (event types): All 19 transition events are present. D52b extends the
  vocabulary to 21 with two content-analysis events from seed §5. This is an
  additive amendment, not a contradiction.
- D22 (terminal lifecycle emission): `LifecycleEventEmitter.Emit` takes
  `context.Context`; the detached emission context (Layer 4) is carried by the
  orchestrator. Interface shape supports D22.
- D24 (parallel dispatch): `tools.InvocationContext` is a read-only struct;
  concurrent `Invoke` calls sharing the same instance is safe.
- D26 (price snapshot): `PriceProvider.PriceForToken` signature supports
  per-invocation snapshot.
- D27 (zero-wiring path): Null defaults still produce all bracket events; the
  10-event canonical sequence is unaffected.

## Composability Checks

### CP3 — Budget Guard composition

`budget.Guard` in `05-budget-interfaces.md` satisfies CP3:

1. No `Start(invocationID)` or `Stop()` method — the Guard makes no assumption
   about being bound to one invocation.
2. `RecordXxx` methods accumulate; `Check` reads against the combined total.
3. Multiple concurrent invocations can share one `Guard` instance.
4. The default `BudgetGuard` uses atomic accumulators.

### CP5 — Typed-error propagation through ToolResult

`tools.ToolResult.Err error` satisfies the shape half of CP5:

1. `ToolResult` carries an `error` field that may hold any `TypedError` from a
   nested invocation.
2. `errors.DefaultClassifier` checks `errors.As(err, &typed)` before heuristic
   reclassification. A `BudgetExceededError` surfaced through `ToolResult.Err`
   is returned with its original `Kind()` preserved.

## Stability Tier Summary

| Interface | Tier | File |
|---|---|---|
| `*Orchestrator` (facade, constructor, options) | `frozen-v1.0` | `02-orchestrator-api.md` |
| `llm.Provider` | `frozen-v1.0` | `03-llm-provider.md` |
| `tools.Invoker` | `frozen-v1.0` | `06-tools-and-invocation-context.md` |
| `hooks.PolicyHook` | `frozen-v1.0` | `04-hooks-and-filters.md` |
| `hooks.PreLLMFilter` | `frozen-v1.0` | `04-hooks-and-filters.md` |
| `hooks.PostToolFilter` | `frozen-v1.0` | `04-hooks-and-filters.md` |
| `budget.Guard` | `frozen-v1.0` | `05-budget-interfaces.md` |
| `budget.PriceProvider` | **promoted to `frozen-v1.0`** (D47) | `05-budget-interfaces.md` |
| `errors.TypedError` | `frozen-v1.0` | `07-errors-and-classifier.md` |
| `errors.Classifier` | `frozen-v1.0` | `07-errors-and-classifier.md` |
| `telemetry.LifecycleEventEmitter` | `frozen-v1.0` | `08-telemetry-interfaces.md` |
| `telemetry.AttributeEnricher` | `frozen-v1.0` | `08-telemetry-interfaces.md` |
| `credentials.Resolver` | `frozen-v1.0` | `09-credentials-and-identity.md` |
| `identity.Signer` | `stable-v0.x-candidate` | `09-credentials-and-identity.md` |

13 of 14 interfaces at `frozen-v1.0`. The 14th (`identity.Signer`) remains
`stable-v0.x-candidate` pending Phase 5 JWT claim set specification.

## Decoupling Contract Compliance

**PASS.** A case-insensitive word-bounded grep across all Phase 3 artifacts
returns zero matches for banned identifiers as actual identifiers:

- `custos` — not present (negation-mentions in this file only)
- `reef` — not present
- `governance_event` — not present
- `org.id`, `agent.id`, `user.id`, `tenant.id` as hardcoded attributes — not
  present (attribution is via `AttributeEnricher` and `Metadata` maps only)
- Consumer brand names — not present
- Cross-repository milestone codes — not present

## D10 Module-Path Compliance

All Phase 3 artifacts use `MODULE_PATH_TBD` as a placeholder per the D10
tripwire. No concrete module path is embedded. Phase 3 does not block on D10.

## Phase 4 and Phase 5 Obligations

**Phase 4 obligations:**
- Span tree design must preserve CP1 (nested `Invoke` creates child span).
- `parent_invocation_id` attribute on lifecycle events (CP2).
- `internal/ctxutil.DetachedWithSpan` span re-attachment (C1).
- `DefaultClassifier` precedence rules for propagated typed errors (CP5).
- `BudgetExceededError` token-overshoot documentation (C3).
- Emission semantics for `EventTypePIIRedacted` and
  `EventTypePromptInjectionSuspected` (D52b).
- `FilterDecision.Action` → lifecycle event mapping.

**Phase 5 obligations:**
- `Credential.Close()` zeroing mechanics.
- Soft-cancel `context.WithoutCancel` contract for `Resolver.Fetch` (C4).
- `identity.Signer` JWT claim set; promotion to `frozen-v1.0`.
- CP6: outer identity readable from `InvocationContext.SignedIdentity`.

## Verdict: READY

Phase 3 delivers 22 adopted decisions (D31–D52), complete Go method signatures
for all 14 interfaces, null defaults for every interface, and a cycle-free
package dependency graph. Both structural blockers from the reviewer pass have
been resolved with formal decisions. The decoupling contract is clean. The
phase is ready to close.
