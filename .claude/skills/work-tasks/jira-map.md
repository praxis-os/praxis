# Task ID to Jira Key Mapping

## Epics
| ID | Key | Title |
|----|-----|-------|
| E1 | PRAX-5 | v0.1.0 — First Invocation |
| E2 | PRAX-6 | v0.3.0 — Interfaces Stable |
| E3 | PRAX-7 | v0.5.0 — Feature Complete |
| E4 | PRAX-8 | v1.0.0 — API Freeze |

## Stories
| ID | Key | Epic | Title |
|----|-----|------|-------|
| S1 | PRAX-9 | PRAX-5 | Module Init & Build Setup |
| S2 | PRAX-10 | PRAX-5 | State Machine (14 states) |
| S3 | PRAX-11 | PRAX-5 | Orchestrator.Invoke (sync) |
| S4 | PRAX-12 | PRAX-5 | Error Taxonomy & Classifier |
| S5 | PRAX-13 | PRAX-5 | Anthropic Provider |
| S6 | PRAX-14 | PRAX-5 | Null/Noop Defaults |
| S7 | PRAX-15 | PRAX-5 | Documentation v0.1 |
| S8 | PRAX-16 | PRAX-5 | Quality Gate v0.1 |
| S9 | PRAX-17 | PRAX-6 | Streaming (InvokeStream) |
| S10 | PRAX-18 | PRAX-6 | Cancellation Semantics |
| S11 | PRAX-19 | PRAX-6 | PolicyHook Chain |
| S12 | PRAX-20 | PRAX-6 | Filter Chains |
| S13 | PRAX-21 | PRAX-6 | Budget Guard |
| S14 | PRAX-22 | PRAX-6 | OTel Telemetry |
| S15 | PRAX-23 | PRAX-6 | OpenAI Provider |
| S16 | PRAX-24 | PRAX-6 | Quality Gate v0.3 |
| S17 | PRAX-25 | PRAX-6 | Examples v0.3 |
| S18 | PRAX-26 | PRAX-7 | Identity Signing (Ed25519) |
| S19 | PRAX-27 | PRAX-7 | Credentials Management |
| S20 | PRAX-28 | PRAX-7 | Security Hardening |
| S21 | PRAX-29 | PRAX-7 | Full Observability |
| S22 | PRAX-30 | PRAX-7 | Error Model Refinements |
| S23 | PRAX-31 | PRAX-7 | Benchmarks & Conformance |
| S24 | PRAX-32 | PRAX-7 | Quality Gate v0.5 |
| S25 | PRAX-33 | PRAX-7 | Examples v0.5 |
| S26 | PRAX-34 | PRAX-8 | Production Consumer Gate |
| S27 | PRAX-35 | PRAX-8 | API Freeze |
| S28 | PRAX-36 | PRAX-8 | Governance & Docs v1.0 |

## Subtasks — v0.1.0
| ID | Key | Story | Title | Status | PR |
|----|-----|-------|-------|--------|----|
| T1.1 | PRAX-37 | PRAX-9 | Create go.mod with confirmed module path | Done | merged |
| T1.2 | PRAX-38 | PRAX-9 | Create Makefile with standard targets | Done | merged |
| T1.3 | PRAX-39 | PRAX-9 | Configure CI pipeline | To Do | — |
| T1.4 | PRAX-40 | PRAX-9 | Configure branch protection on main | To Do | — |
| T1.5 | PRAX-41 | PRAX-9 | Configure release-please | To Do | — |
| T2.1 | PRAX-42 | PRAX-10 | Define State type with 14 constants | Done | [#2](https://github.com/praxis-os/praxis/pull/2) merged |
| T2.2 | PRAX-43 | PRAX-10 | Transition allow-list table | Done | included in T2.1 |
| T2.3 | PRAX-44 | PRAX-10 | Machine interface and implementation | Done | [#3](https://github.com/praxis-os/praxis/pull/3) |
| T2.4 | PRAX-45 | PRAX-10 | Property-based tests (10k iterations) | To Do | — |
| T2.5 | PRAX-46 | PRAX-10 | 21 state machine invariants | To Do | — |
| T3.1 | PRAX-47 | PRAX-11 | InvocationRequest and InvocationResult types | To Do | — |
| T3.2 | PRAX-48 | PRAX-11 | orchestrator.New constructor | To Do | — |
| T3.3 | PRAX-49 | PRAX-11 | Invocation loop driver | To Do | — |
| T3.4 | PRAX-50 | PRAX-11 | With* option functions | To Do | — |
| T3.5 | PRAX-51 | PRAX-11 | Sync invocation e2e tests | To Do | — |
| T4.1 | PRAX-52 | PRAX-12 | TypedError interface | Done | [#4](https://github.com/praxis-os/praxis/pull/4) |
| T4.2 | PRAX-53 | PRAX-12 | 8 concrete error types | To Do | — |
| T4.3 | PRAX-54 | PRAX-12 | Classifier with retry policy | To Do | — |
| T4.4 | PRAX-55 | PRAX-12 | internal/retry with backoff and jitter | To Do | — |
| T5.1 | PRAX-56 | PRAX-13 | llm.Provider interface | To Do | — |
| T5.2 | PRAX-57 | PRAX-13 | LLM message and tool types | To Do | — |
| T5.3 | PRAX-58 | PRAX-13 | Anthropic provider implementation | To Do | — |
| T5.4 | PRAX-59 | PRAX-13 | Mock provider for testing | To Do | — |
| T5.5 | PRAX-60 | PRAX-13 | Anthropic provider smoke test | To Do | — |
| T6.1 | PRAX-61 | PRAX-14 | tools.NullInvoker | To Do | — |
| T6.2 | PRAX-62 | PRAX-14 | hooks.AllowAllPolicyHook | To Do | — |
| T6.3 | PRAX-63 | PRAX-14 | NoOpPreLLMFilter and NoOpPostToolFilter | To Do | — |
| T6.4 | PRAX-64 | PRAX-14 | budget.NullGuard and NullPriceProvider | To Do | — |
| T6.5 | PRAX-65 | PRAX-14 | telemetry.NullEmitter and NullEnricher | To Do | — |
| T6.6 | PRAX-66 | PRAX-14 | credentials.NullResolver | To Do | — |
| T6.7 | PRAX-67 | PRAX-14 | identity.NullSigner | To Do | — |
| T7.1 | PRAX-68 | PRAX-15 | README.md | To Do | — |
| T7.2 | PRAX-69 | PRAX-15 | examples/minimal/ runnable | To Do | — |
| T7.3 | PRAX-70 | PRAX-15 | CONTRIBUTING.md | To Do | — |
| T7.4 | PRAX-71 | PRAX-15 | SECURITY.md | To Do | — |
| T7.5 | PRAX-72 | PRAX-15 | CODE_OF_CONDUCT.md and DCO | To Do | — |
| T7.6 | PRAX-73 | PRAX-15 | SPDX headers on all .go files | To Do | — |
| T7.7 | PRAX-74 | PRAX-15 | LICENSE file | To Do | — |
| T8.1 | PRAX-75 | PRAX-16 | 85% line coverage | To Do | — |
| T8.2 | PRAX-76 | PRAX-16 | make check passes | To Do | — |
| T8.3 | PRAX-77 | PRAX-16 | CHANGELOG.md via release-please | To Do | — |

## Subtasks — v0.3.0
| ID | Key | Story | Title |
|----|-----|-------|-------|
| T9.1 | PRAX-78 | PRAX-17 | InvocationEvent type (21 event types) |
| T9.2 | PRAX-79 | PRAX-17 | InvokeStream with 16-event buffer channel |
| T9.3 | PRAX-80 | PRAX-17 | sync.Once channel close protocol |
| T9.4 | PRAX-81 | PRAX-17 | Backpressure handling |
| T10.1 | PRAX-82 | PRAX-18 | Soft cancel with 500ms grace |
| T10.2 | PRAX-83 | PRAX-18 | Hard cancel on deadline/budget breach |
| T10.3 | PRAX-84 | PRAX-18 | Terminal event emission (detached context) |
| T10.4 | PRAX-85 | PRAX-18 | internal/ctxutil.DetachedWithSpan |
| T11.1 | PRAX-86 | PRAX-19 | hooks.Phase (4 phases) |
| T11.2 | PRAX-87 | PRAX-19 | hooks.Decision type |
| T11.3 | PRAX-88 | PRAX-19 | PolicyHook chain execution |
| T11.4 | PRAX-89 | PRAX-19 | ApprovalRequired terminal state |
| T12.1 | PRAX-90 | PRAX-20 | PreLLMFilter chain |
| T12.2 | PRAX-91 | PRAX-20 | PostToolFilter chain |
| T12.3 | PRAX-92 | PRAX-20 | FilterDecision type |
| T13.1 | PRAX-93 | PRAX-21 | budget.Guard implementation |
| T13.2 | PRAX-94 | PRAX-21 | Wall-clock duration enforcement |
| T13.3 | PRAX-95 | PRAX-21 | Token count enforcement |
| T13.4 | PRAX-96 | PRAX-21 | Tool call count enforcement |
| T13.5 | PRAX-97 | PRAX-21 | Cost estimate enforcement (micro-dollars) |
| T13.6 | PRAX-98 | PRAX-21 | PriceProvider per-invocation snapshot |
| T13.7 | PRAX-99 | PRAX-21 | BudgetExceeded terminal state |
| T14.1 | PRAX-100 | PRAX-22 | telemetry.OTelEmitter (span tree) |
| T14.2 | PRAX-101 | PRAX-22 | 21 InvocationEvent emission |
| T14.3 | PRAX-102 | PRAX-22 | AttributeEnricher flow |
| T14.4 | PRAX-103 | PRAX-22 | MetricsRecorder interface |
| T15.1 | PRAX-104 | PRAX-23 | openai.Provider implementation |
| T15.2 | PRAX-105 | PRAX-23 | Azure OpenAI via base-URL |
| T15.3 | PRAX-106 | PRAX-23 | Shared conformance suite |
| T16.1 | PRAX-107 | PRAX-24 | 14 interfaces at Phase 3 surfaces |
| T16.2 | PRAX-108 | PRAX-24 | Property tests 10k CI + 100k nightly |
| T16.3 | PRAX-109 | PRAX-24 | 85% coverage maintained |
| T16.4 | PRAX-110 | PRAX-24 | Nightly conformance suite |
| T17.1 | PRAX-111 | PRAX-25 | examples/tools/ |
| T17.2 | PRAX-112 | PRAX-25 | examples/policy/ |
| T17.3 | PRAX-113 | PRAX-25 | examples/filters/ |
| T17.4 | PRAX-114 | PRAX-25 | examples/streaming/ |

## Subtasks — v0.5.0
| ID | Key | Story | Title |
|----|-----|-------|-------|
| T18.1 | PRAX-115 | PRAX-26 | identity.NewEd25519Signer |
| T18.2 | PRAX-116 | PRAX-26 | JWT with 5+2 claims |
| T18.3 | PRAX-117 | PRAX-26 | Token lifetime [5s, 300s] |
| T18.4 | PRAX-118 | PRAX-26 | With* signer options |
| T18.5 | PRAX-119 | PRAX-26 | internal/jwt stdlib-only |
| T18.6 | PRAX-120 | PRAX-26 | Identity chaining (parent_token) |
| T19.1 | PRAX-121 | PRAX-27 | Resolver.Fetch with soft-cancel |
| T19.2 | PRAX-122 | PRAX-27 | credentials.ZeroBytes |
| T19.3 | PRAX-123 | PRAX-27 | Credential.Close() with KeepAlive |
| T20.1 | PRAX-124 | PRAX-28 | Panic recovery on hook/filter sites |
| T20.2 | PRAX-125 | PRAX-28 | Trust boundary logging levels |
| T20.3 | PRAX-126 | PRAX-28 | 26 security invariant tests |
| T21.1 | PRAX-127 | PRAX-29 | slog RedactingHandler |
| T21.2 | PRAX-128 | PRAX-29 | 10 Prometheus metrics |
| T21.3 | PRAX-129 | PRAX-29 | FilterDecision to events |
| T21.4 | PRAX-130 | PRAX-29 | Error-to-event 1:1 mapping |
| T22.1 | PRAX-131 | PRAX-30 | BudgetExceeded godoc caveat |
| T22.2 | PRAX-132 | PRAX-30 | Classifier identity rule (errors.As) |
| T22.3 | PRAX-133 | PRAX-30 | VerdictLog AuditNote |
| T23.1 | PRAX-134 | PRAX-31 | Orchestrator overhead < 15ms |
| T23.2 | PRAX-135 | PRAX-31 | State machine 1M transitions/sec |
| T23.3 | PRAX-136 | PRAX-31 | benchstat in PR CI |
| T23.4 | PRAX-137 | PRAX-31 | Conformance suite green |
| T24.1 | PRAX-138 | PRAX-32 | 85% coverage maintained |
| T24.2 | PRAX-139 | PRAX-32 | All CI jobs operational |
| T24.3 | PRAX-140 | PRAX-32 | Godoc on every exported symbol |
| T24.4 | PRAX-141 | PRAX-32 | Integration tests for 14 interfaces |
| T25.1 | PRAX-142 | PRAX-33 | examples/identity/ |
| T25.2 | PRAX-143 | PRAX-33 | examples/credentials/ |

## Subtasks — v1.0.0
| ID | Key | Story | Title |
|----|-----|-------|-------|
| T26.1 | PRAX-144 | PRAX-34 | Consumer attestation in release notes |
| T27.1 | PRAX-145 | PRAX-35 | 14 interfaces frozen-v1.0 |
| T27.2 | PRAX-146 | PRAX-35 | version.go = 1.0.0 |
| T27.3 | PRAX-147 | PRAX-35 | No stable-v0.x-candidate remaining |
| T28.1 | PRAX-148 | PRAX-36 | SECURITY.md with OI-1, OI-2 |
| T28.2 | PRAX-149 | PRAX-36 | RFC Discussion category |
| T28.3 | PRAX-150 | PRAX-36 | CONTRIBUTING.md v1.0 review reqs |
| T28.4 | PRAX-151 | PRAX-36 | govulncheck promoted to required |
| T28.5 | PRAX-152 | PRAX-36 | Nightly conformance 30 days stable |
| T28.6 | PRAX-153 | PRAX-36 | CHANGELOG v0.x to v1.0 |
| T28.7 | PRAX-154 | PRAX-36 | README stability commitment |
