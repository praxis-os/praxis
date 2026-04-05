# Phase 1 — v1.0 Freeze Surface

**Related decisions:** D04 (tier assignment), D13 (tiering policy).

This document enumerates every interface from seed §5 and assigns it to
exactly one of the three stability tiers defined in D13. It is the
authoritative reference for Phase 3 (Interface Contracts) and for the v1.0
release checklist.

---

## Tier definitions *(D13)*

- **`frozen-v1.0`** — The interface shape is frozen at v1.0 as a
  semver-level commitment to downstream consumers: any breaking change
  requires a v2 module path (`github.com/praxis-go/praxis/v2`). Adding a
  method is a breaking change and must be delivered as a new interface
  that embeds the original (e.g., `llm.ProviderV2 { llm.Provider;
  NewMethod(...) }`), shipped alongside. This is a technical contract with
  library users and is distinct from the methodological status of phase
  decisions (which remain amendable per the decision-log amendment
  protocol).
- **`stable-v0.x-candidate`** — The interface is intended to reach
  `frozen-v1.0` before v1.0.0 ships but is held back by a named gating
  condition (a later phase, an open decision, or a dependency on a
  downstream phase's output). Shape may still move during v0.x.
- **`post-v1`** — Explicitly not part of the v1.0 freeze. Ships experimental;
  may change on any minor tag post-v1. Provided as a signal that the
  surface may expand, not a commitment.

No interface in Phase 1 lands in `post-v1`. The tier exists for later phases.

---

## Per-interface tier assignment *(D04)*

### `orchestrator.AgentOrchestrator` — **frozen-v1.0**

Primary consumer touch point. Every caller binds to `Invoke` and
`InvokeStream`. Shape changes post-v1.0 break every caller. No plausible
method expansion before v1.0.

### `llm.Provider` — **frozen-v1.0**

Implemented by every adapter author (the shipped Anthropic and OpenAI
adapters, plus third-party adapters). Freezing this is the precondition for
a stable adapter ecosystem and for the validity of the shared
`llm/conformance/` test suite.

### `tools.Invoker` — **frozen-v1.0**

The single seam between the orchestration kernel and all tool execution
infrastructure. Instability here propagates to every caller's integration
layer. D06 (tool-name placement on `ToolCall`, not `InvocationContext`) is
a precondition for this freeze and is resolved in Phase 1.

### `hooks.PolicyHook` — **frozen-v1.0**

Policy evaluation is a compliance boundary. Callers will build policy
engines that implement this interface; shape drift after v1.0 breaks their
compliance guarantees. The four lifecycle phases are owned by the
orchestrator and defined in Phase 2; the interface shape is stable
independent of the phase set.

### `hooks.PreLLMFilter` — **frozen-v1.0**

Security and data-handling filters sit here. Callers implement these for
PII handling, prompt injection detection, and content moderation; they
cannot absorb breaking changes post-freeze. The interface is small (one
method) and has no plausible expansion need in v1.x.

### `hooks.PostToolFilter` — **frozen-v1.0**

Same reasoning as `PreLLMFilter`. Tool outputs are treated as untrusted by
contract, which is a security commitment the filter interface enforces; the
shape is minimal and stable.

### `budget.Guard` — **frozen-v1.0**

Called on every state transition in the hot path. The four budget
dimensions (wall-clock duration, total tokens, tool call count, cost
estimate in micro-dollars) are correctness-load-bearing and unlikely to
grow in v1.x. Any new dimension would be added as a new field on the
budget struct, not as a new method on `Guard`.

### `budget.PriceProvider` — **stable-v0.x-candidate**

**Gating condition:** resolution of D08 (hot-reload semantics).
**Resolution in Phase 1:** D08 adopts per-invocation snapshot. With
snapshot semantics, `PriceProvider`'s method surface (lookup by
`(provider, model, direction)`) is mechanically stable — no re-read, no
change notification, no live update contract. This interface is therefore
expected to promote to `frozen-v1.0` at Phase 3 sign-off, subject to the
final method signature review. Held back in Phase 1 because the decision
log must not prejudge Phase 3's signature choice.

### `errors.TypedError` — **frozen-v1.0**

Referenced by every layer. The minimum method set (`Kind()`,
`HTTPStatusCode()`, `Unwrap()`) is the stable floor. Any extension is
additive via type embedding on concrete error types, not modification of
the interface. Freezing early removes the largest potential source of
cross-layer churn.

### `errors.Classifier` — **frozen-v1.0**

Drives retry policy. Callers who implement custom classifiers (e.g., to
route domain-specific errors into the taxonomy) cannot absorb interface
changes. Single-method interface, stable by construction.

### `telemetry.LifecycleEventEmitter` — **frozen-v1.0**

Callers implement this to connect praxis lifecycle events to their
observability backend. The event vocabulary (the set of framework-defined
event constants) is defined by the framework and is extended by adding new
event types, not by changing the emitter interface. Shape is stable.

### `telemetry.AttributeEnricher` — **frozen-v1.0**

The decoupling contract's enforcement point for caller-contributed identity
attribution. If this interface changes, every caller's attribution adapter
breaks. The method surface is minimal (return a set of caller-contributed
attributes) and has no plausible expansion need.

### `credentials.Resolver` — **frozen-v1.0**

Security boundary. `Fetch` and `Credential.Close()` are the full surface
and neither has a plausible reason to change shape. The zero-on-close and
no-caching contracts are property-level commitments that live alongside
the interface, not in the interface signature.

### `identity.Signer` — **stable-v0.x-candidate**

**Gating condition:** Phase 5 (Security and Trust Boundaries).
**Reason.** The concept is adopted (optional per-tool-call identity
assertion via short-lived Ed25519 JWT), but the exact JWT claim set, key
algorithm constraints, whether the framework validates the returned token,
and key rotation expectations are open questions owned by Phase 5.
Committing a method signature before Phase 5 completes would prejudge
those questions. The interface is expected to promote to `frozen-v1.0`
after Phase 5 sign-off, well before v0.5.0.

---

## Tier summary

| Tier | Count | Interfaces |
|---|---|---|
| `frozen-v1.0` | 12 | `AgentOrchestrator`, `llm.Provider`, `tools.Invoker`, `hooks.PolicyHook`, `hooks.PreLLMFilter`, `hooks.PostToolFilter`, `budget.Guard`, `errors.TypedError`, `errors.Classifier`, `telemetry.LifecycleEventEmitter`, `telemetry.AttributeEnricher`, `credentials.Resolver` |
| `stable-v0.x-candidate` | 2 | `budget.PriceProvider` (gated on D08 → promotes Phase 3), `identity.Signer` (gated on Phase 5 → promotes Phase 5) |
| `post-v1` | 0 | — |

## Promotion schedule

- **`budget.PriceProvider`** → expected to promote to `frozen-v1.0` at
  Phase 3 sign-off, barring Phase 3 discovering a method-signature issue.
  D08 has already resolved the semantic question (per-invocation snapshot),
  so the remaining risk is narrowly scoped to the signature itself.
- **`identity.Signer`** → `frozen-v1.0` at Phase 5 sign-off, subject to
  finalization of JWT claim set and key lifecycle.

Both promotions must land before v0.5.0 is tagged. If either slips, the
affected interface ships v0.5.0 as `stable-v0.x-candidate` and v1.0.0 is
delayed rather than tagged with an unfrozen candidate — this is a hard
rule, not a guideline.

## Freeze rationale, overall

Twelve of fourteen interfaces reach `frozen-v1.0` in Phase 1. This is
deliberately aggressive: the seed surface was designed for freeze from the
outset, method counts are small (one to three methods per interface), and
each interface has few plausible expansion vectors. Premature freezing is
a real risk for bloated interfaces that cannot absorb additive changes;
praxis's interfaces are not bloated, so the usual ergonomic hedge against
freezing ("add a catch-all method now so we can extend later") does not
apply. The two held back are held back for phase-ordering reasons, not
design doubt — both are expected to freeze well before v1.0.0.
