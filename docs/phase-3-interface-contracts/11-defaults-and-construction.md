# Phase 3 — Defaults and Construction

**Related decisions:** D12 (zero-wiring smoke path), D37 (constructor),
D48 (no Close)
**Packages:** all sub-packages that ship null/default implementations

---

## Overview

Every interface shipped in `praxis` v1.0 has a named null or default
implementation. This document catalogs them, states their behavior, and
demonstrates the zero-wiring smoke path guaranteed by D12.

---

## Null / default implementation catalog

| Interface | Default implementation | Package | Behavior |
|---|---|---|---|
| `tools.Invoker` | `tools.NullInvoker` | `tools` | Returns `ToolStatusNotImplemented` for every call |
| `hooks.PolicyHook` | `hooks.AllowAllPolicyHook` | `hooks` | Returns `VerdictAllow` for every evaluation |
| `hooks.PreLLMFilter` | `hooks.PassThroughPreLLMFilter` | `hooks` | Returns input messages unchanged; no decisions |
| `hooks.PostToolFilter` | `hooks.PassThroughPostToolFilter` | `hooks` | Returns input ToolResult unchanged; no decisions |
| `budget.Guard` | `budget.NullGuard` | `budget` | Records nothing; never signals breach; returns zero BudgetSnapshot |
| `budget.PriceProvider` | `budget.NullPriceProvider` | `budget` | Returns 0 micro-dollars for all token prices |
| `telemetry.LifecycleEventEmitter` | `telemetry.NullEmitter` | `telemetry` | Discards all events |
| `telemetry.AttributeEnricher` | `telemetry.NullEnricher` | `telemetry` | Returns empty map |
| `credentials.Resolver` | `credentials.NullResolver` | `credentials` | Returns an error for every Fetch (no store configured) |
| `identity.Signer` | `identity.NullSigner` | `identity` | Returns empty string for every Sign call |
| `errors.Classifier` | `errors.DefaultClassifier` | `errors` | Heuristic classification with TypedError identity rule (D44) |
| `llm.Provider` (test/mock) | `llm/mock.EchoProvider` | `llm/mock` | Returns caller-configured response; no network |

All null implementations are exported package-level `var` values of their
respective interface type, assigned at package init. They are stateless and
safe for concurrent use.

---

## Zero-wiring construction path (D12)

The minimum viable `Orchestrator` requires only an `llm.Provider`:

```go
import (
    "context"
    "MODULE_PATH_TBD"
    "MODULE_PATH_TBD/llm/anthropic"
)

provider := anthropic.New(os.Getenv("ANTHROPIC_API_KEY"))

orch := praxis.NewOrchestrator(provider)

result, err := orch.Invoke(context.Background(), praxis.InvocationRequest{
    Messages: []llm.Message{
        {Role: llm.RoleUser, Parts: []llm.MessagePart{{Type: llm.PartTypeText, Text: "Hello"}}},
    },
    Model: "claude-opus-4-5",
})
```

This compiles and runs. The default wiring is:

- `tools.NullInvoker` — no tools available; LLM will not attempt tool use.
- `hooks.AllowAllPolicyHook` — all policy evaluations return Allow.
- `hooks.PassThroughPreLLMFilter` — no input filtering.
- `hooks.PassThroughPostToolFilter` — no output filtering.
- `budget.NullGuard` — no budget enforcement.
- `budget.NullPriceProvider` — zero cost accounting.
- `telemetry.NullEmitter` — events discarded.
- `telemetry.NullEnricher` — no attribute enrichment.
- `credentials.NullResolver` — credential fetches would error, but NullInvoker never requests credentials.
- `identity.NullSigner` — no identity signing.
- `errors.DefaultClassifier` — heuristic classification.

`InvokeStream` works equivalently and emits the 10-event zero-wiring sequence
(D27) on the returned channel.

---

## Full wiring example

A production-grade orchestrator wires all dependencies:

```go
orch := praxis.NewOrchestrator(
    provider,
    praxis.WithToolInvoker(myToolInvoker),
    praxis.WithPolicyHook(myPolicyHook),
    praxis.WithPreLLMFilter(myPIIFilter),
    praxis.WithPostToolFilter(myInjectionFilter),
    praxis.WithBudgetGuard(budget.NewBudgetGuard(budget.Config{
        MaxWallClock:        30 * time.Second,
        MaxOutputTokens:     4096,
        MaxToolCalls:        10,
        MaxCostMicrodollars: 50_000, // $0.05
    })),
    praxis.WithPriceProvider(budget.NewStaticPriceProvider(myPriceTable)),
    praxis.WithLifecycleEventEmitter(telemetry.NewOTelEmitter(tracer)),
    praxis.WithAttributeEnricher(myTenantEnricher),
    praxis.WithCredentialResolver(myVaultResolver),
    praxis.WithIdentitySigner(myEd25519Signer),
)
```

Options are applied in order; later options for the same dependency override
earlier ones. Repeated `WithPolicyHook` calls replace the previously set hook.

---

## Composing multiple filters or hooks

The orchestrator accepts one `PolicyHook`, one `PreLLMFilter`, and one
`PostToolFilter`. Callers who need to chain multiple implementations compose
them before injection:

```go
// Chain two PreLLMFilters in sequence.
// Neither ChainPreLLMFilters nor individual filter types are praxis API;
// this is a caller-level composition pattern.
combined := hooks.ChainPreLLMFilters(piiFilter, injectionFilter)
orch := praxis.NewOrchestrator(provider, praxis.WithPreLLMFilter(combined))
```

`hooks.ChainPreLLMFilters` is a convenience constructor shipped in the `hooks`
package. It runs each filter in sequence; the first `FilterActionBlock` decision
halts the chain and returns immediately. The caller may also write their own
chain runner.

---

## `NullInvoker` specification

```go
// NullInvoker is the default tools.Invoker.
// It returns ToolResult{Status: ToolStatusNotImplemented, CallID: call.CallID}
// for every Invoke call.
//
// NullInvoker is safe for concurrent use.
//
// Typical use: zero-wiring construction (D12), or as a fallback in
// custom invokers for unrecognized tool names:
//
//   func (iv *MyInvoker) Invoke(ctx context.Context, ictx tools.InvocationContext, call tools.ToolCall) (tools.ToolResult, error) {
//       switch call.Name {
//       case "my_tool":
//           return iv.runMyTool(ctx, call)
//       default:
//           return tools.NullInvoker.Invoke(ctx, ictx, call)
//       }
//   }
var NullInvoker tools.Invoker = nullInvoker{}
```

---

## `AllowAllPolicyHook` specification

```go
// AllowAllPolicyHook is the default hooks.PolicyHook.
// It returns hooks.Allow() for every Evaluate call at every phase.
//
// AllowAllPolicyHook is safe for concurrent use.
//
// When AllowAllPolicyHook is active, the pre-hook and post-hook events
// (EventTypePreHookStarted, EventTypePreHookCompleted,
// EventTypePostHookStarted, EventTypePostHookCompleted) are still emitted
// — the event sequence is structurally determined by the state machine
// traversal, not by whether a hook implementation is non-null (D27).
var AllowAllPolicyHook hooks.PolicyHook = allowAllHook{}
```

---

## `NullResolver` specification

```go
// NullResolver is the default credentials.Resolver.
// It returns a *errors.SystemError for every Fetch call, indicating that
// no credential store is configured.
//
// NullResolver is appropriate for zero-wiring construction when:
//   (a) tools.NullInvoker is active (it never calls Fetch), or
//   (b) the custom Invoker does not require credential fetching.
//
// NullResolver is safe for concurrent use.
var NullResolver credentials.Resolver = nullResolver{}
```

---

## `StaticPriceProvider` specification

```go
// StaticPriceProvider is a budget.PriceProvider backed by a caller-supplied
// price table. It is the standard production-ready PriceProvider.
//
// Callers supply a map[budget.PriceKey]int64 where values are micro-dollars
// per token (millionths of a USD). Unknown keys return 0, nil.
//
// StaticPriceProvider is safe for concurrent use; the table is read-only
// after construction.
//
// Example:
//
//   pp := budget.NewStaticPriceProvider(map[budget.PriceKey]int64{
//       {Provider: "anthropic", Model: "claude-opus-4-5", Direction: budget.TokenDirectionInput}:  15,  // $0.000015 per input token
//       {Provider: "anthropic", Model: "claude-opus-4-5", Direction: budget.TokenDirectionOutput}: 75,  // $0.000075 per output token
//   })
func NewStaticPriceProvider(table map[budget.PriceKey]int64) budget.PriceProvider
```

---

## Testability

Every interface shipped in `praxis` is testable without a real LLM or
external service:

- Use `llm/mock.EchoProvider` to return deterministic LLM responses.
- Use `tools.InvokerFunc` for single-function inline invokers.
- Use the null implementations for any wiring not under test.
- Use the `state.Machine` directly in property-based tests.
- Use the conformance suite in `llm/conformance` to validate custom adapters.

Mock-friendliness is not incidental — every interface has five or fewer
methods (mostly one or two), making manual test doubles trivial to write
without code generation.
