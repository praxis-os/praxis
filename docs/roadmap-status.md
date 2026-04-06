# Roadmap Status

**Last updated:** 2026-04-06
**Target:** `praxis v1.0.0` — stable public Go API for enterprise agent orchestration

## Phase Status

| # | Phase | Status | Artifacts |
|---|-------|--------|-----------|
| 0 | Seed Context | starting baseline (amendable via decision-log amendment) | 1 (`docs/PRAXIS-SEED-CONTEXT.md`) |
| 1 | API Scope and Positioning | **approved** | 7 (`00-plan.md`, `01-decisions-log.md`, `02-positioning-and-principles.md`, `03-non-goals.md`, `04-v1-freeze-surface.md`, `05-seed-question-resolutions.md`, `06-composition-patterns.md`, `REVIEW.md`) |
| 2 | Core Runtime Design | **approved** | 8 (`00-plan.md`, `01-decisions-log.md`, `02-state-machine.md`, `03-streaming-and-events.md`, `04-cancellation-and-context.md`, `05-concurrency-model.md`, `06-state-machine-invariants.md`, `REVIEW.md`) |
| 3 | Interface Contracts | **approved** | 14 (`00-plan.md`, `01-decisions-log.md`, `02-orchestrator-api.md`, `03-llm-provider.md`, `04-hooks-and-filters.md`, `05-budget-interfaces.md`, `06-tools-and-invocation-context.md`, `07-errors-and-classifier.md`, `08-telemetry-interfaces.md`, `09-credentials-and-identity.md`, `10-state-types.md`, `11-defaults-and-construction.md`, `go-architect-package-layout.md`, `REVIEW.md`) |
| 4 | Observability and Error Model | **approved** | 9 (`00-plan.md`, `01-decisions-log.md`, `02-span-tree.md`, `03-metrics.md`, `04-slog-redaction.md`, `05-error-event-mapping.md`, `06-filter-event-mapping.md`, `go-architect-validation.md`, `REVIEW.md`) |
| 5 | Security and Trust Boundaries | **approved** | 8 (`00-plan.md`, `01-decisions-log.md`, `02-credential-lifecycle.md`, `03-identity-signing.md`, `04-trust-boundaries.md`, `05-security-invariants.md`, `go-architect-validation.md`, `REVIEW.md`) |
| 6 | Release, Versioning and Community Governance | not-started | — |

## Adopted Decisions

**78 of 80 allocated decisions adopted** across Phases 1–5. Phase 1 owns
D01–D15 (D15 released unused); Phase 2 owns D15–D30 (D15–D28 adopted,
D29/D30 released unused); Phase 3 owns D31–D52 (D31–D52 adopted); Phase 4
owns D53–D66 (D53–D66 adopted); Phase 5 owns D67–D80 (D67–D80 adopted).
Next range opens at **D81** for Phase 6.

- **Phase 1 (approved):** D01–D14 adopted. D15 released.
- **Phase 2 (approved):** D15–D28 adopted. D29, D30 released.
- **Phase 3 (approved):** D31–D52 adopted. D51 resolved the package layout
  (facade in `orchestrator/` sub-package, types in root). D52 recorded
  three seed amendments (interface→struct, 21-event vocabulary, PriceProvider
  promotion).
- **Phase 4 (approved):** D53–D66 adopted. D53 defines the OTel span tree
  (1 root + 6 child spans). D54 resolves C1 (full `trace.Span` in
  `DetachedWithSpan`). D55 resolves CP1 (span links for nested orchestrators).
  D56 resolves CP2 (`parent_invocation_id` as span attribute). D57 defines
  10 Prometheus metrics with bounded cardinality. D58 places `RedactingHandler`
  in `telemetry/slog/`. D59 maps FilterDecision to content-analysis events.
  D60 defines AttributeEnricher flow with hard cardinality boundary. D61 maps
  ErrorKind to terminal EventType 1:1. D62 resolves C3 (token-overshoot
  godoc). D63 resolves CP5 (classifier identity rule). D64 defines VerdictLog
  emission via AuditNote field. D65 formally amends Phase 3 InvocationEvent
  (6 new fields) and adds WithMetricsRecorder option. D66 commits signal-term
  lists to frozen-v1.0.
- **Phase 5 (approved):** D67–D80 adopted. D67 mandates `runtime.KeepAlive`-
  fenced zeroing for `Credential.Close()`. D68 ships `credentials.ZeroBytes`
  utility. D69 resolves C4 (soft-cancel credential context via
  `context.WithoutCancel` + 500ms timeout). D70–D71 define mandatory JWT
  registered claims (iss, sub, exp, iat, jti) and custom claims
  (praxis.invocation_id, praxis.tool_name). D72 sets configurable token
  lifetime (60s default, [5s, 300s] range). D73 specifies `NewEd25519Signer`
  constructor (stdlib-only, EdDSA algorithm). D74 documents static key model
  with `kid` header support. D75 resolves CP6 (identity chaining via
  `praxis.parent_token` claim). D76 promotes `identity.Signer` to
  `frozen-v1.0`. D77 formalises untrusted tool output model. D78 classifies
  filter trust boundaries (PostToolFilter = boundary-crossing, PreLLMFilter =
  boundary-internal). D79 extends RedactingHandler deny-list with
  `praxis.signed_identity` and `_jwt` suffix. D80 enumerates 26 security
  invariants across 4 categories (C1–C8, I1–I6, T1–T7, O1–O5).
- **Phase 6:** no decisions allocated yet.

Adopted decisions remain **amendable** via the protocol recorded in
`docs/phase-1-api-scope/01-decisions-log.md#amendment-protocol`. The
three-tier stability policy (D13) governs interface freezes separately
from methodological-decision amendability.

## Completed Work

**Phase 1 — API Scope and Positioning (approved).** 14 decisions (D01–D14):
positioning statement, eight design principles (seven from seed + the
zero-wiring smoke path principle), target consumer archetype and three
anti-personas, v1.0 freeze surface (12/14 interfaces at `frozen-v1.0`,
two at `stable-v0.x-candidate`), seven non-goals, tool-name placement on
`ToolCall` (seed §13.1), `ApprovalRequiredError` terminal semantics (seed
§13.2), `PriceProvider` per-invocation snapshot policy (seed §13.3), "no
plugins in v1" re-confirmed (seed §13.4), conditional adoption of the name
`praxis` / module `github.com/praxis-go/praxis` with a Phase 3 tripwire,
positioning gaps catalogued, zero-wiring smoke-test promise, three-tier
stability policy, Azure OpenAI best-effort parity. All four seed §13 open
questions resolved. Decoupling grep clean. `REVIEW.md` verdict: **READY**.

**Phase 2 — Core Runtime Design (approved).** 14 decisions (D15–D28):

- **D15 — 14-state machine** (9 non-terminal + 5 terminal).
- **D16 — Transition allow-list** with full adjacency table.
- **D17 — `ApprovalRequired` is a distinct terminal state.**
- **D18 — 19 `InvocationEvent` types** (later expanded to 21 via D52b).
- **D19 — Streaming channel close protocol** (`sync.Once`-guarded).
- **D20 — Backpressure via `select + ctx.Done()`.**
- **D21 — Soft vs. hard cancel precedence matrix.**
- **D22 — Terminal lifecycle-emission invariant** (5-second detached context).
- **D23 — Four-layer context propagation.**
- **D24 — One goroutine per invocation, sole-producer rule.**
- **D25 — Budget wall-clock boundary** (starts at Initializing, stops at terminal).
- **D26 — PriceProvider snapshot at Initializing entry.**
- **D27 — Zero-wiring streaming event set** (10 events single-turn).
- **D28 — 21 property-based invariants.**
- D29, D30 released unused.
- Five forward-carried concerns documented (C1–C5).

**Phase 3 — Interface Contracts (approved).** 22 decisions (D31–D52):
complete Go interface definitions for all 14 public interfaces, type
placements, package layout, constructor pattern, null implementations,
composition properties (CP1–CP6). D51 resolved the package layout. D52
recorded three seed amendments.

**Phase 4 — Observability and Error Model (approved).** 14 decisions
(D53–D66):

- **D53 — OTel span tree:** 1 root span (`praxis.invocation`) + 6 child
  spans for I/O-bound phases. No span for `ToolDecision` (sub-microsecond
  CPU work). `ApprovalRequired` maps to `StatusOK`.
- **D54 — C1 resolved:** `DetachedWithSpan(span trace.Span, deadline
  time.Duration)` carries full span for terminal attribute writes.
- **D55 — CP1 resolved:** span links (not child spans) for nested
  orchestrators, avoiding lifetime mismatches and depth limits.
- **D56 — CP2 resolved:** `praxis.parent_invocation_id` as framework-
  injected span attribute.
- **D57 — 10 Prometheus metrics** with `praxis_` prefix, all labels
  bounded, ~1,032 worst-case time series. Hard cardinality boundary:
  enricher attributes → spans only, never metric labels.
- **D58 — slog integration:** `RedactingHandler` in `telemetry/slog/`
  sub-package. Never-log list covers credentials, raw content, PII.
- **D59 — FilterDecision → content-analysis event mapping:** reason-driven
  trigger logic, emission before enclosing state-transition events.
- **D60 — AttributeEnricher flow:** Enrich called once at Initializing
  (after root span opened), attributes to spans and
  `InvocationEvent.EnricherAttributes`, never to metric labels.
- **D61 — Error-to-event mapping:** 1:1 ErrorKind → terminal EventType.
  First-error-wins arbitration via state machine immutability.
- **D62 — C3 resolved:** BudgetExceededError godoc amended with
  token-overshoot caveat.
- **D63 — CP5 resolved:** classifier identity rule (`errors.As` first)
  with four worked examples.
- **D64 — VerdictLog emission:** AuditNote field on hook-completion events,
  no new EventType constant.
- **D65 — Phase 3 amendments:** 6 new InvocationEvent fields + 
  WithMetricsRecorder option, formally recorded.
- **D66 — Signal-term stability:** frozen-v1.0 commitment on PII and
  injection signal-term lists.

Phase 4 resolved all five forward-carried concerns: C1 (D54), C3 (D62),
CP1 (D55), CP2 (D56), CP5 (D63). Decoupling grep clean.
`REVIEW.md` verdict: **READY**.

**Phase 5 — Security and Trust Boundaries (approved).** 14 decisions
(D67–D80):

- **D67 — Credential zeroing:** `runtime.KeepAlive`-fenced byte-slice
  overwrite. Prevents dead-store elision by the Go compiler.
- **D68 — `credentials.ZeroBytes` utility:** exported helper centralises
  the zeroing pattern for third-party `Credential` implementations.
- **D69 — C4 resolved:** soft-cancel credential context uses
  `context.WithoutCancel` + 500ms `context.WithTimeout` so credential
  resolution is not hard-cancelled during graceful shutdown.
- **D70 — JWT registered claims:** 5 mandatory (`iss`, `sub`, `exp`,
  `iat`, `jti`), 2 optional (`aud`, `nbf` omitted). `iss` defaults to
  `"praxis"`; production callers must set via `WithIssuer`.
- **D71 — JWT custom claims:** `praxis.invocation_id` and
  `praxis.tool_name` mandatory. `WithExtraClaims(map[string]any)` for
  static caller claims; mandatory claims win on collision.
- **D72 — Token lifetime:** configurable, 60s default, [5s, 300s] range.
  Out-of-range values rejected at construction time with error.
- **D73 — Ed25519 reference impl:** `NewEd25519Signer(key, ...SignerOption)
  (Signer, error)`. Stdlib-only: `crypto/ed25519`, `encoding/json`,
  `encoding/base64`, `crypto/rand`. JOSE header: `{"alg":"EdDSA","typ":"JWT"}`.
- **D74 — Key lifecycle:** static key model in reference impl. `kid` header
  for verifier key selection. Rotation requires caller-implemented Signer.
- **D75 — CP6 resolved:** identity chaining via `praxis.parent_token`
  payload claim containing the outer JWT string. Chain depth: documentation
  recommendation of 3 levels, not enforced.
- **D76 — `identity.Signer` promoted to `frozen-v1.0`.** All gating
  conditions satisfied (D70–D75).
- **D77 — Untrusted tool output model:** `ToolResult.Content` untrusted by
  contract. Framework passes through `PostToolFilter`, honors Block, never
  inspects content for security patterns.
- **D78 — Filter trust boundaries:** `PostToolFilter` is trust-boundary-
  crossing (errors at ERROR). `PreLLMFilter` is trust-boundary-internal
  (errors at WARN). Panic recovery via deferred `recover()` on all
  hook/filter call sites.
- **D79 — RedactingHandler amendments:** added `praxis.signed_identity` and
  `_jwt` suffix to Phase 4 D58 deny-list.
- **D80 — Security invariants:** 26 invariants in 4 categories (C1–C8
  credential isolation, I1–I6 identity signing, T1–T7 trust boundaries,
  O1–O5 observability safety) with traceability matrix.

Phase 5 resolved C4 (D69) and CP6 (D75) — the last two forward-carried
concerns from Phase 2/3. All interfaces now at `frozen-v1.0`.
Two open issues documented for post-v1.0: OI-1 (private key in-memory
lifetime) and OI-2 (enricher attribute log-injection vector).
Decoupling grep clean. `REVIEW.md` verdict: **READY**.

## Open Decisions

No open decisions in Phases 1–5.

- **Phase 6 (Release and Governance) — open.** Must allocate: semver
  deprecation windows, release-please configuration, CI pipeline including
  banned-identifier grep, D10 tripwire enforcement, bus-factor mitigation
  (seed §14.1), first-production-consumer gating for v1.0.0. Additionally,
  Phase 6 must:
  - Record `internal/jwt` package addition (Phase 5 go-architect) in the
    canonical package layout.
  - Document OI-1 (private key lifetime) and OI-2 (enricher log-injection)
    in `SECURITY.md` as known limitations.
  - Confirm the `MetricsRecorder` interface extension story (Phase 4 OQ2).

## Risks / Blockers

- **D10 external dependency** (carried from Phase 1). Name/module-path
  resolution is conditional. Phase 3 used `MODULE_PATH_TBD` throughout;
  v0.1.0 tag is gated on resolution.
- **`MetricsRecorder` interface extension story** (Phase 4 REVIEW.md
  open question 2). If a new Prometheus metric is added in v1.x, a new
  `MetricsRecorderV2` interface may be needed. Not a blocker for v1.0 but
  worth tracking.
- **Bus factor** (seed §14.1). Flagged for Phase 6.
- **First production consumer dependency** (seed §8 v1.0.0 criterion).
  v1.0.0 tag is gated on a production consumer shipping against v0.5.x.
  Flagged for Phase 6.

## Decoupling Contract Health

**PASS.** A case-insensitive word-bounded grep against the seed §6.1
banned-identifier set returns zero matches across all Phase 1–5 artifacts
as actual identifiers. Occurrences are limited to negation-mentions inside
compliance declarations in plan and review files. Verified by the reviewer
subagent in each phase pass.

The decoupling contract is a correctness invariant, not an amendable
decision.

## Next Step

Invoke `plan-phase` on **Phase 6 — Release, Versioning and Community
Governance**. Phase 6 must specify: semver deprecation windows,
release-please configuration, CI pipeline (lint, test, coverage, benchmarks,
banned-identifier grep, govulncheck, codeql), D10 tripwire enforcement
(module path resolution), bus-factor mitigation, first-production-consumer
gating, contribution model, code of conduct, RFC process, and `SECURITY.md`
content including Phase 5 open issues OI-1 and OI-2.
Decision range opens at D81.

## Overall Status

Planning is on track: **5 of 6 design phases are approved** with a clean
decoupling contract and 78 adopted decisions; one design phase remains
before implementation begins. All forward-carried concerns (C1–C4, CP1–CP6)
are fully resolved. All 14 public interfaces are now at `frozen-v1.0`
(including `identity.Signer`, promoted in Phase 5 D76). **v0.1.0** (first
working invocation) requires Phase 6 approval plus D10 resolution.
**v0.5.0** (feature complete) requires all six phases approved. **v1.0.0**
(API freeze) further requires a production consumer to ship against a
v0.5.x tag — a dependency outside any design phase. Adopted decisions in
every phase remain amendable via the protocol recorded in each phase's
decision log.
