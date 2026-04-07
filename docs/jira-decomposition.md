# Jira Decomposition — praxis v1.0.0

**Project key:** PRAX
**Board:** francescofioredev.atlassian.net
**Source:** `docs/phase-6-release-governance/06-release-milestones.md` (exit criteria) + 103 adopted decisions (D01–D105) across 6 approved design phases.
**Totals:** 4 Epics, 28 Stories, 92 Tasks

---

## Epic 1: PRAX — v0.1.0 First Invocation

**Type:** Epic
**Title:** v0.1.0 — First Invocation
**Description:** A consumable Go module that completes a single synchronous LLM invocation with zero caller-supplied wiring beyond an `llm.Provider`. All null/noop defaults wired, CI pipeline operational, documentation and governance scaffolding in place.
**Priority:** Highest
**Labels:** `milestone:v0.1.0`

---

### S1: Module Init & Build Setup

**Type:** Story
**Title:** Module initialisation and build tooling
**Description:** Create the Go module, build tooling, CI pipeline, branch protection, and release automation. Unblocked only after D10 (module path) is resolved.
**Priority:** Highest
**Dependencies:** D10 resolution (external blocker)
**Labels:** `milestone:v0.1.0`, `area:build`

#### T1.1: Create go.mod with final module path

**Type:** Task
**Title:** Create `go.mod` with confirmed module path
**Description:** Initialise the Go module with the final module path once D10 is resolved. Must not use `MODULE_PATH_TBD`. Minimum Go version: 1.23.
**Acceptance criteria:**
- `go.mod` exists with the confirmed module path
- `go build ./...` succeeds
**Decision refs:** D10, D89
**Priority:** Highest
**Dependencies:** D10 resolution
**Labels:** `milestone:v0.1.0`, `area:build`

#### T1.2: Makefile with standard targets

**Type:** Task
**Title:** Create Makefile with test, lint, bench, coverage, and banned-grep targets
**Description:** Provide a Makefile with targets: `test`, `lint`, `bench`, `coverage`, `banned-grep`, `spdx-check`, and a composite `check` target that runs all quality gates.
**Acceptance criteria:**
- `make check` runs lint, test, banned-grep, spdx-check
- `make bench` runs benchmarks
- `make coverage` reports line coverage
**Priority:** High
**Dependencies:** T1.1
**Labels:** `milestone:v0.1.0`, `area:build`

#### T1.3: CI pipeline

**Type:** Task
**Title:** Configure CI pipeline with 8 on-PR jobs and supporting schedules
**Description:** Set up GitHub Actions CI per D85:
- **6 required PR checks:** lint, test, commitsar (D83), banned-grep, spdx-check, dco
- **2 informational PR checks:** bench, govulncheck
- **2 nightly jobs:** property-tests (D87), conformance (D88) — initially stubs until v0.3.0
- **1 post-merge job:** bench-baseline (D105)
- **1 release job:** release-please
- **CodeQL:** weekly + on-PR
**Acceptance criteria:**
- All 8 on-PR jobs trigger on pull requests to `main`
- 6 required checks are enforced as merge blockers
- Nightly job stubs exist (will be populated in v0.3.0)
**Decision refs:** D83, D85, D86, D94
**Priority:** High
**Dependencies:** T1.1
**Labels:** `milestone:v0.1.0`, `area:ci`

#### T1.4: Branch protection on main

**Type:** Task
**Title:** Configure branch protection rules on `main`
**Description:** Enable branch protection per D94: squash merge only, 6 required status checks (lint, test, commitsar, banned-grep, spdx-check, dco).
**Acceptance criteria:**
- Direct pushes to `main` blocked
- PRs require all 6 required checks to pass
- Squash merge enforced
**Decision refs:** D94
**Priority:** High
**Dependencies:** T1.3
**Labels:** `milestone:v0.1.0`, `area:ci`

#### T1.5: release-please configuration

**Type:** Task
**Title:** Configure release-please with `release-type: go`
**Description:** Set up release-please per D84: `release-type: go`, changelog sections mapped to keep-a-changelog convention, `version.go` as extra-file for version bumps.
**Acceptance criteria:**
- `.release-please-manifest.json` and `release-please-config.json` exist
- `internal/version/version.go` is listed as an extra-file
- Changelog sections follow keep-a-changelog convention
**Decision refs:** D84
**Priority:** Medium
**Dependencies:** T1.1
**Labels:** `milestone:v0.1.0`, `area:build`

---

### S2: State Machine (14 states)

**Type:** Story
**Title:** Invocation state machine with 14 states and property-based tests
**Description:** Implement the core state machine with 9 non-terminal + 5 terminal states, the full transition allow-list, and property-based invariant tests.
**Priority:** Highest
**Dependencies:** T1.1
**Labels:** `milestone:v0.1.0`, `area:state-machine`

#### T2.1: Define State type and 14 constants

**Type:** Task
**Title:** Define `State` type with 14 state constants
**Description:** Create the `State` type (typed string or int) with 9 non-terminal states (`Initializing`, `PreparingLLMInput`, `CallingLLM`, `ProcessingLLMOutput`, `ToolDecision`, `ExecutingTool`, `ProcessingToolOutput`, `Evaluating`, `Finalizing`) and 5 terminal states (`Completed`, `Failed`, `Cancelled`, `BudgetExceeded`, `ApprovalRequired`).
**Acceptance criteria:**
- All 14 state constants defined and exported
- Terminal states identifiable via method or predicate
**Decision refs:** D15
**Priority:** Highest
**Dependencies:** T1.1
**Labels:** `milestone:v0.1.0`, `area:state-machine`

#### T2.2: Transition allow-list table

**Type:** Task
**Title:** Implement transition allow-list with full adjacency table
**Description:** Implement the allowed state transitions as a lookup table per D16. Only transitions in the allow-list are permitted; all others return an error.
**Acceptance criteria:**
- Transition function rejects disallowed transitions with a typed error
- Allowed transitions match D16 adjacency table exactly
**Decision refs:** D16
**Priority:** Highest
**Dependencies:** T2.1
**Labels:** `milestone:v0.1.0`, `area:state-machine`

#### T2.3: Machine interface and implementation

**Type:** Task
**Title:** Implement state `Machine` with thread-safe transitions
**Description:** Implement the state machine that tracks current state and enforces transitions. Must be safe for concurrent read access (single-writer via sole-producer rule D24).
**Acceptance criteria:**
- Current state queryable
- Transitions atomic and validated against allow-list
- Immutable once terminal state reached
**Decision refs:** D15, D16, D24
**Priority:** Highest
**Dependencies:** T2.2
**Labels:** `milestone:v0.1.0`, `area:state-machine`

#### T2.4: Property-based tests (10k iterations)

**Type:** Task
**Title:** Property-based state machine tests at 10k iterations
**Description:** Implement property-based tests covering the 21 invariants from D28. Must run at 10k iterations in CI (PR checks). Nightly escalation to 100k iterations will be configured in v0.3.0.
**Acceptance criteria:**
- 21 invariants tested via property-based framework
- CI runs 10k iterations per invariant
- All tests pass
**Decision refs:** D28, D87
**Priority:** High
**Dependencies:** T2.3
**Labels:** `milestone:v0.1.0`, `area:state-machine`, `area:testing`

#### T2.5: 21 state machine invariants

**Type:** Task
**Title:** Verify all 21 state machine invariants from Phase 2
**Description:** Ensure the implementation satisfies all 21 property-based invariants documented in D28 and `docs/phase-2-core-runtime/06-state-machine-invariants.md`.
**Acceptance criteria:**
- Each of the 21 invariants has a corresponding test assertion
- All assertions pass
**Decision refs:** D28
**Priority:** High
**Dependencies:** T2.3
**Labels:** `milestone:v0.1.0`, `area:state-machine`, `area:testing`

---

### S3: Orchestrator.Invoke (sync)

**Type:** Story
**Title:** Synchronous `Invoke` path through the orchestrator
**Description:** Implement the `AgentOrchestrator` facade with the synchronous `Invoke` method, the invocation loop driver, request/result types, and construction-time validation.
**Priority:** Highest
**Dependencies:** S2, S4, S5, S6
**Labels:** `milestone:v0.1.0`, `area:orchestrator`

#### T3.1: InvocationRequest and InvocationResult structs

**Type:** Task
**Title:** Define `InvocationRequest` and `InvocationResult` types
**Description:** Define the request and result structs for the `Invoke` method per Phase 3 interface contracts.
**Acceptance criteria:**
- `InvocationRequest` includes required fields (messages, context)
- `InvocationResult` includes response content, final state, token usage, error
**Priority:** High
**Dependencies:** T2.1
**Labels:** `milestone:v0.1.0`, `area:orchestrator`

#### T3.2: orchestrator.New constructor

**Type:** Task
**Title:** Implement `orchestrator.New(provider, ...Option)` constructor
**Description:** Implement the constructor that accepts only an `llm.Provider` plus optional `With*` functions. Must satisfy D12 zero-wiring promise: constructible with only a provider.
**Acceptance criteria:**
- `orchestrator.New(provider)` succeeds with all defaults wired
- `With*` options override defaults when provided
- Invalid options return construction-time errors
**Decision refs:** D12
**Priority:** High
**Dependencies:** T3.1, S6
**Labels:** `milestone:v0.1.0`, `area:orchestrator`

#### T3.3: Invocation loop driver

**Type:** Task
**Title:** Implement internal invocation loop driver
**Description:** Implement the `internal/loop` package that drives the state machine through the invocation cycle: prepare input → call LLM → process output → tool decision → execute tool → loop or finalize.
**Acceptance criteria:**
- Loop drives state machine through complete invocation
- Single-turn (no tool calls) completes successfully
- Multi-turn (with tool calls) loops correctly
**Decision refs:** D24
**Priority:** High
**Dependencies:** T2.3, T3.2
**Labels:** `milestone:v0.1.0`, `area:orchestrator`

#### T3.4: With* option functions and construction-time validation

**Type:** Task
**Title:** Implement `With*` option functions with construction-time validation
**Description:** Implement all `With*` option functions for the orchestrator constructor. All validation must happen at construction time, not at `Invoke` time.
**Acceptance criteria:**
- Each option validates its argument at construction time
- Invalid arguments return descriptive errors
- Options compose correctly (last-wins for single-value options)
**Decision refs:** D12
**Priority:** Medium
**Dependencies:** T3.2
**Labels:** `milestone:v0.1.0`, `area:orchestrator`

#### T3.5: Sync invocation e2e unit tests

**Type:** Task
**Title:** End-to-end unit tests for synchronous `Invoke`
**Description:** Write e2e unit tests using the mock provider that exercise the full sync invocation path: single-turn, multi-turn with tools, error paths.
**Acceptance criteria:**
- Single-turn invocation test passes
- Multi-turn with mock tool calls passes
- Error propagation tested (transient, permanent, tool errors)
**Priority:** High
**Dependencies:** T3.3, T5.4
**Labels:** `milestone:v0.1.0`, `area:orchestrator`, `area:testing`

---

### S4: Error Taxonomy and Classifier

**Type:** Story
**Title:** Typed error taxonomy with 8 concrete types and retry classifier
**Description:** Implement the `TypedError` interface, all 8 concrete error types, the `Classifier`, and the retry policy with exponential backoff and jitter.
**Priority:** Highest
**Dependencies:** T1.1
**Labels:** `milestone:v0.1.0`, `area:errors`

#### T4.1: TypedError interface

**Type:** Task
**Title:** Define `TypedError` interface with `Kind`, `HTTPStatusCode`, `Unwrap`
**Description:** Define the `errors.TypedError` interface with methods: `Kind() ErrorKind`, `HTTPStatusCode() int`, `Unwrap() error`, plus `Error() string`.
**Acceptance criteria:**
- Interface defined and exported
- `ErrorKind` type defined with constants for all 8 kinds
**Priority:** Highest
**Dependencies:** T1.1
**Labels:** `milestone:v0.1.0`, `area:errors`

#### T4.2: 8 concrete error types

**Type:** Task
**Title:** Implement 8 concrete `TypedError` types
**Description:** Implement: `TransientError`, `PermanentError`, `ToolError`, `PolicyDeniedError`, `BudgetExceededError`, `CancellationError`, `SystemError`, `ApprovalRequiredError`.
**Acceptance criteria:**
- Each type satisfies `TypedError` interface
- Each type has correct `Kind()` and `HTTPStatusCode()`
- `ApprovalRequiredError` handles terminal state semantics (D07, D17)
**Decision refs:** D07, D17
**Priority:** Highest
**Dependencies:** T4.1
**Labels:** `milestone:v0.1.0`, `area:errors`

#### T4.3: Classifier with retry policy

**Type:** Task
**Title:** Implement error `Classifier` with retry policy
**Description:** Implement the `Classifier` that categorizes errors and determines retry policy: transient LLM errors retry 3x with exponential backoff and jitter; all others never retry.
**Acceptance criteria:**
- Classifier correctly categorizes all 8 error types
- Transient errors get 3 retries with backoff + jitter
- Non-transient errors get 0 retries
**Priority:** High
**Dependencies:** T4.2
**Labels:** `milestone:v0.1.0`, `area:errors`

#### T4.4: internal/retry with backoff and jitter

**Type:** Task
**Title:** Implement `internal/retry` package with exponential backoff and jitter
**Description:** Implement the retry utility used by the classifier: exponential backoff with jitter, configurable max attempts.
**Acceptance criteria:**
- Exponential backoff with randomised jitter
- Respects context cancellation
- Table-driven tests with deterministic jitter (seeded RNG)
**Priority:** High
**Dependencies:** T1.1
**Labels:** `milestone:v0.1.0`, `area:errors`

---

### S5: Anthropic Provider

**Type:** Story
**Title:** Anthropic LLM provider adapter
**Description:** Define the `llm.Provider` interface, core LLM types, implement the Anthropic adapter, and provide a mock provider for testing.
**Priority:** Highest
**Dependencies:** T1.1
**Labels:** `milestone:v0.1.0`, `area:llm`

#### T5.1: llm.Provider interface

**Type:** Task
**Title:** Define `llm.Provider` interface
**Description:** Define the `llm.Provider` interface with the method surface from Phase 3 contracts.
**Acceptance criteria:**
- Interface exported with complete method signatures
- Godoc on all methods
**Decision refs:** D31
**Priority:** Highest
**Dependencies:** T1.1
**Labels:** `milestone:v0.1.0`, `area:llm`

#### T5.2: LLM message and tool types

**Type:** Task
**Title:** Define `llm.Message`, `MessagePart`, `ToolDefinition` types
**Description:** Define the core LLM types: `Message`, `MessagePart` (text, tool call, tool result), `ToolDefinition`, `ToolCall`, `ToolResult`.
**Acceptance criteria:**
- All types defined and exported
- `ToolCall` carries `tool_name` per D06
**Decision refs:** D06
**Priority:** Highest
**Dependencies:** T1.1
**Labels:** `milestone:v0.1.0`, `area:llm`

#### T5.3: Anthropic provider implementation

**Type:** Task
**Title:** Implement `anthropic.Provider`
**Description:** Implement the Anthropic Messages API adapter satisfying `llm.Provider`. Must handle message formatting, tool definitions, and error mapping to `TypedError`.
**Acceptance criteria:**
- Implements `llm.Provider` interface completely
- Maps Anthropic API errors to appropriate `TypedError` types
- Supports tool use responses
**Priority:** Highest
**Dependencies:** T5.1, T5.2, T4.2
**Labels:** `milestone:v0.1.0`, `area:llm`

#### T5.4: Mock provider for testing

**Type:** Task
**Title:** Implement `llm/mock.Provider` for tests
**Description:** Implement a configurable mock provider for unit and integration testing. Must support scripted responses, tool calls, and error injection.
**Acceptance criteria:**
- Satisfies `llm.Provider` interface
- Configurable response sequences
- Error injection support
**Priority:** High
**Dependencies:** T5.1
**Labels:** `milestone:v0.1.0`, `area:llm`, `area:testing`

#### T5.5: Smoke test with real API

**Type:** Task
**Title:** Anthropic provider smoke test with real API
**Description:** Write a smoke test (skipped in CI without API key) that performs a real Anthropic API call and validates the response.
**Acceptance criteria:**
- Test completes a real invocation when `ANTHROPIC_API_KEY` is set
- Test is skipped gracefully when the key is absent
- Validates response structure
**Priority:** Medium
**Dependencies:** T5.3
**Labels:** `milestone:v0.1.0`, `area:llm`, `area:testing`

---

### S6: Null/Noop Defaults (10 impls)

**Type:** Story
**Title:** All 10 null/noop default implementations
**Description:** Implement all null/noop defaults so the orchestrator is constructible with only an `llm.Provider` (D12 zero-wiring promise).
**Priority:** High
**Dependencies:** T1.1
**Labels:** `milestone:v0.1.0`, `area:defaults`

#### T6.1: tools.NullInvoker

**Type:** Task
**Title:** Implement `tools.NullInvoker`
**Description:** Null tool invoker that returns `StatusNotImplemented` for any tool call.
**Acceptance criteria:**
- Satisfies `tools.Invoker` interface
- Returns `StatusNotImplemented` error for any call
**Priority:** High
**Labels:** `milestone:v0.1.0`, `area:defaults`

#### T6.2: hooks.AllowAllPolicyHook

**Type:** Task
**Title:** Implement `hooks.AllowAllPolicyHook`
**Description:** Default policy hook that returns `Allow` for all evaluation phases.
**Acceptance criteria:**
- Satisfies `hooks.PolicyHook` interface
- Returns `Allow` for all 4 phases
**Priority:** High
**Labels:** `milestone:v0.1.0`, `area:defaults`

#### T6.3: hooks.NoOpPreLLMFilter and NoOpPostToolFilter

**Type:** Task
**Title:** Implement `hooks.NoOpPreLLMFilter` and `hooks.NoOpPostToolFilter`
**Description:** No-op filter implementations that pass all content through unmodified.
**Acceptance criteria:**
- Each satisfies its respective interface
- Returns `Pass` decision for all content
**Priority:** High
**Labels:** `milestone:v0.1.0`, `area:defaults`

#### T6.4: budget.NullGuard and NullPriceProvider

**Type:** Task
**Title:** Implement `budget.NullGuard` and `budget.NullPriceProvider`
**Description:** `NullGuard` never breaches any budget dimension. `NullPriceProvider` returns 0 for all cost queries.
**Acceptance criteria:**
- `NullGuard` satisfies `budget.Guard`, never returns budget exceeded
- `NullPriceProvider` satisfies `budget.PriceProvider`, returns 0
**Priority:** High
**Labels:** `milestone:v0.1.0`, `area:defaults`

#### T6.5: telemetry.NullEmitter and NullEnricher

**Type:** Task
**Title:** Implement `telemetry.NullEmitter` and `telemetry.NullEnricher`
**Description:** `NullEmitter` discards all lifecycle events. `NullEnricher` returns empty attributes.
**Acceptance criteria:**
- `NullEmitter` satisfies `telemetry.Emitter`, no-ops on all events
- `NullEnricher` satisfies `telemetry.AttributeEnricher`, returns empty map
**Priority:** High
**Labels:** `milestone:v0.1.0`, `area:defaults`

#### T6.6: credentials.NullResolver

**Type:** Task
**Title:** Implement `credentials.NullResolver`
**Description:** Null credential resolver that returns an error for any credential reference.
**Acceptance criteria:**
- Satisfies `credentials.Resolver` interface
- Returns descriptive error for any `Fetch` call
**Priority:** High
**Labels:** `milestone:v0.1.0`, `area:defaults`

#### T6.7: identity.NullSigner

**Type:** Task
**Title:** Implement `identity.NullSigner`
**Description:** Null identity signer that returns an empty string (no signing).
**Acceptance criteria:**
- Satisfies `identity.Signer` interface
- `Sign` returns empty string and nil error
**Priority:** High
**Labels:** `milestone:v0.1.0`, `area:defaults`

---

### S7: Documentation v0.1

**Type:** Story
**Title:** v0.1.0 documentation and governance files
**Description:** Create all required documentation and governance scaffolding for the first public release.
**Priority:** High
**Dependencies:** S3
**Labels:** `milestone:v0.1.0`, `area:docs`

#### T7.1: README.md

**Type:** Task
**Title:** Create README.md with hello-world example
**Description:** Write README with: one-line description, prerequisites (Go 1.23+, API key), `go get` command, hello-world example (target: 25 lines), error handling note, "where to go next" links, anti-persona redirect.
**Acceptance criteria:**
- Hello-world example is under 25 lines
- `go get` command uses the confirmed module path
- Anti-persona redirect included
**Priority:** High
**Labels:** `milestone:v0.1.0`, `area:docs`

#### T7.2: examples/minimal/ runnable

**Type:** Task
**Title:** Create `examples/minimal/` runnable hello-world
**Description:** The hello-world example from the README as a standalone runnable `main.go`.
**Acceptance criteria:**
- `go run examples/minimal/main.go` compiles (execution requires API key)
- Matches the README example
**Priority:** High
**Dependencies:** S3
**Labels:** `milestone:v0.1.0`, `area:docs`, `area:examples`

#### T7.3: CONTRIBUTING.md

**Type:** Task
**Title:** Create CONTRIBUTING.md
**Description:** Per `docs/phase-6-release-governance/05-contribution-and-governance.md` §3: PR process, commit conventions, DCO requirement, review policy.
**Acceptance criteria:**
- Documents PR review policy (D93)
- Documents conventional commit requirement (D83)
- Documents DCO sign-off requirement (D92)
**Decision refs:** D83, D92, D93
**Priority:** Medium
**Labels:** `milestone:v0.1.0`, `area:docs`

#### T7.4: SECURITY.md

**Type:** Task
**Title:** Create SECURITY.md
**Description:** Per `docs/phase-6-release-governance/05-contribution-and-governance.md` §8: vulnerability reporting via GitHub private reporting, 90-day disclosure timeline.
**Acceptance criteria:**
- Documents private vulnerability reporting process
- 90-day disclosure timeline stated
**Decision refs:** D96
**Priority:** Medium
**Labels:** `milestone:v0.1.0`, `area:docs`

#### T7.5: CODE_OF_CONDUCT.md and DCO file

**Type:** Task
**Title:** Create CODE_OF_CONDUCT.md (Contributor Covenant 2.1) and DCO file
**Description:** Add Contributor Covenant 2.1 as code of conduct. Add DCO file (Developer Certificate of Origin v1.1 text). Configure probot/dco as required check per D92.
**Acceptance criteria:**
- CODE_OF_CONDUCT.md uses Contributor Covenant 2.1
- DCO file contains Developer Certificate of Origin v1.1
- probot/dco installed and configured
**Decision refs:** D92
**Priority:** Medium
**Labels:** `milestone:v0.1.0`, `area:docs`

#### T7.6: SPDX headers on all .go files

**Type:** Task
**Title:** Add SPDX `Apache-2.0` headers to all `.go` files
**Description:** Every `.go` file must have the SPDX license identifier header. CI check (`spdx-check`) enforces this.
**Acceptance criteria:**
- All `.go` files have `// SPDX-License-Identifier: Apache-2.0` header
- `make spdx-check` passes
**Decision refs:** D97
**Priority:** Medium
**Dependencies:** T1.2
**Labels:** `milestone:v0.1.0`, `area:build`

#### T7.7: LICENSE file

**Type:** Task
**Title:** Add Apache 2.0 LICENSE file
**Description:** Add the standard Apache License 2.0 text as the LICENSE file.
**Acceptance criteria:**
- LICENSE file contains the full Apache 2.0 text
**Priority:** Medium
**Labels:** `milestone:v0.1.0`, `area:docs`

---

### S8: Quality Gate v0.1

**Type:** Story
**Title:** v0.1.0 quality gate verification
**Description:** Verify all v0.1.0 quality exit criteria are met before tagging.
**Priority:** Highest
**Dependencies:** S1–S7
**Labels:** `milestone:v0.1.0`, `area:quality`

#### T8.1: 85% line coverage

**Type:** Task
**Title:** Achieve and verify 85% line coverage on all packages
**Description:** All packages including `internal/` must meet the 85% line coverage gate per D86.
**Acceptance criteria:**
- `go test -coverprofile` reports >= 85% on every package
- `make coverage` confirms the threshold
**Decision refs:** D86
**Priority:** High
**Labels:** `milestone:v0.1.0`, `area:quality`, `area:testing`

#### T8.2: make check passes

**Type:** Task
**Title:** Full `make check` pass (lint, test, banned-grep, spdx-check)
**Description:** The composite `make check` target must pass all quality gates.
**Acceptance criteria:**
- `make check` exits 0
- Includes: lint, test, banned-grep, spdx-check
**Priority:** High
**Dependencies:** T1.2
**Labels:** `milestone:v0.1.0`, `area:quality`

#### T8.3: CHANGELOG.md via release-please

**Type:** Task
**Title:** Verify CHANGELOG.md generation via release-please
**Description:** Ensure release-please generates a correct CHANGELOG.md from conventional commits.
**Acceptance criteria:**
- CHANGELOG.md generated with correct version and date
- Entries match conventional commit categories
**Decision refs:** D84
**Priority:** Medium
**Dependencies:** T1.5
**Labels:** `milestone:v0.1.0`, `area:build`

---

## Epic 2: PRAX — v0.3.0 Interfaces Stable

**Type:** Epic
**Title:** v0.3.0 — Interfaces Stable, Primitives Functional
**Description:** All 14 public interfaces at their v1.0-candidate shape with hooks, filters, budget, streaming, OpenAI adapter, and OTel telemetry functional. All v0.1.0 criteria remain satisfied.
**Priority:** High
**Dependencies:** Epic 1 (v0.1.0)
**Labels:** `milestone:v0.3.0`

---

### S9: InvokeStream (streaming)

**Type:** Story
**Title:** Streaming invocation via `InvokeStream`
**Description:** Implement the streaming invocation path with channel-based event delivery, backpressure handling, and the full 21-event vocabulary.
**Priority:** Highest
**Dependencies:** S3
**Labels:** `milestone:v0.3.0`, `area:streaming`

#### T9.1: InvocationEvent type (21 event types)

**Type:** Task
**Title:** Define `InvocationEvent` type with all 21 event types
**Description:** Define the `InvocationEvent` struct and all 21 `EventType` constants. D18 defined the original 19 types; D52b expanded to 21 (adding `EventTypePIIRedacted` and `EventTypePromptInjectionSuspected`). D31 defines the typed string representation.
**Acceptance criteria:**
- All 21 event types defined as constants
- `InvocationEvent` struct includes all fields per D65 amendments (6 new fields)
**Decision refs:** D18, D31, D52b, D65
**Priority:** Highest
**Labels:** `milestone:v0.3.0`, `area:streaming`

#### T9.2: InvokeStream with 16-event buffer channel

**Type:** Task
**Title:** Implement `InvokeStream` with 16-event buffered channel
**Description:** Implement the streaming invocation method that returns a channel of `InvocationEvent` with a 16-event buffer.
**Acceptance criteria:**
- Returns `<-chan InvocationEvent`
- Channel buffer size is 16
- Terminal event always sent before channel close
**Decision refs:** D19
**Priority:** Highest
**Dependencies:** T9.1
**Labels:** `milestone:v0.3.0`, `area:streaming`

#### T9.3: sync.Once channel close protocol

**Type:** Task
**Title:** Implement `sync.Once`-guarded channel close
**Description:** Ensure the event channel is closed exactly once using `sync.Once`, preventing double-close panics.
**Acceptance criteria:**
- Channel close is `sync.Once`-guarded
- No panic possible from double close
- Test verifies concurrent close safety
**Decision refs:** D19
**Priority:** High
**Dependencies:** T9.2
**Labels:** `milestone:v0.3.0`, `area:streaming`

#### T9.4: Backpressure handling

**Type:** Task
**Title:** Implement backpressure via `select + ctx.Done()`
**Description:** Event sends use `select` with `ctx.Done()` to handle backpressure. If the consumer is slow and the buffer is full, context cancellation takes precedence.
**Acceptance criteria:**
- Slow consumer does not block the invocation indefinitely
- Context cancellation is respected during backpressure
**Decision refs:** D20
**Priority:** High
**Dependencies:** T9.2
**Labels:** `milestone:v0.3.0`, `area:streaming`

---

### S10: Cancellation Semantics

**Type:** Story
**Title:** Soft and hard cancellation with terminal event emission
**Description:** Implement the cancellation precedence matrix: soft cancel with grace window, hard cancel on deadline/budget breach, and guaranteed terminal event emission.
**Priority:** High
**Dependencies:** S2
**Labels:** `milestone:v0.3.0`, `area:cancellation`

#### T10.1: Soft cancel with 500ms grace

**Type:** Task
**Title:** Implement soft cancel with 500ms grace window
**Description:** Soft cancellation allows in-flight operations to complete within a 500ms grace window before forcing termination.
**Acceptance criteria:**
- Soft cancel waits up to 500ms for current operation
- State transitions to `Cancelled` after grace window
**Decision refs:** D21
**Priority:** High
**Labels:** `milestone:v0.3.0`, `area:cancellation`

#### T10.2: Hard cancel on deadline/budget breach

**Type:** Task
**Title:** Implement hard cancel on deadline or budget breach
**Description:** Hard cancel immediately terminates on context deadline or budget breach, bypassing the grace window.
**Acceptance criteria:**
- Budget breach triggers immediate `BudgetExceeded` terminal state
- Context deadline triggers immediate `Cancelled` terminal state
**Decision refs:** D21
**Priority:** High
**Labels:** `milestone:v0.3.0`, `area:cancellation`

#### T10.3: Terminal event emission on detached context

**Type:** Task
**Title:** Guarantee terminal event emission with 5s detached context
**Description:** Even after cancellation, the terminal lifecycle event must be emitted. Use a 5-second detached context to ensure telemetry emission completes.
**Acceptance criteria:**
- Terminal event always emitted, even on hard cancel
- Detached context has 5-second deadline
- If emission fails after 5s, event is dropped (not retried)
**Decision refs:** D22
**Priority:** High
**Dependencies:** T10.1, T10.2
**Labels:** `milestone:v0.3.0`, `area:cancellation`, `area:telemetry`

#### T10.4: internal/ctxutil.DetachedWithSpan

**Type:** Task
**Title:** Implement `internal/ctxutil.DetachedWithSpan`
**Description:** Utility that creates a detached context carrying the OTel span but not the parent's cancellation. Per D54, carries the full `trace.Span` for terminal attribute writes.
**Acceptance criteria:**
- Returned context is not cancelled when parent is
- OTel span is preserved for attribute writes
- Deadline is independently configurable
**Decision refs:** D22, D54
**Priority:** High
**Labels:** `milestone:v0.3.0`, `area:cancellation`

---

### S11: PolicyHook Chain

**Type:** Story
**Title:** PolicyHook evaluation at 4 lifecycle phases
**Description:** Implement the PolicyHook chain with evaluation at all 4 phases and the `ApprovalRequired` terminal state handling.
**Priority:** High
**Dependencies:** S3
**Labels:** `milestone:v0.3.0`, `area:hooks`

#### T11.1: hooks.Phase (4 phases)

**Type:** Task
**Title:** Define `hooks.Phase` type with 4 evaluation phases
**Description:** Define the 4 policy evaluation phases: `PreInvocation`, `PreLLMInput`, `PostToolOutput`, `PostInvocation`.
**Acceptance criteria:**
- 4 phase constants defined and exported
**Priority:** High
**Labels:** `milestone:v0.3.0`, `area:hooks`

#### T11.2: hooks.Decision type

**Type:** Task
**Title:** Define `hooks.Decision` type with 4 outcomes
**Description:** Define the 4 hook decision outcomes: `Allow`, `Deny`, `RequireApproval`, `Log`.
**Acceptance criteria:**
- 4 decision constants defined and exported
**Priority:** High
**Labels:** `milestone:v0.3.0`, `area:hooks`

#### T11.3: PolicyHook chain execution

**Type:** Task
**Title:** Implement PolicyHook chain evaluation logic
**Description:** Implement the chain execution that evaluates multiple PolicyHooks in order at each phase. First `Deny` or `RequireApproval` short-circuits.
**Acceptance criteria:**
- Multiple hooks execute in registration order
- `Deny` short-circuits with `PolicyDeniedError`
- `RequireApproval` short-circuits to terminal state
- `Log` is recorded but does not alter flow
**Priority:** High
**Dependencies:** T11.1, T11.2
**Labels:** `milestone:v0.3.0`, `area:hooks`

#### T11.4: ApprovalRequired terminal state handling

**Type:** Task
**Title:** Handle `ApprovalRequired` terminal state from hooks
**Description:** When a PolicyHook returns `RequireApproval`, the invocation transitions to the `ApprovalRequired` terminal state with an `ApprovalRequiredError`.
**Acceptance criteria:**
- State machine transitions to `ApprovalRequired`
- `ApprovalRequiredError` returned/emitted
- Invocation terminates cleanly
**Decision refs:** D07, D17
**Priority:** High
**Dependencies:** T11.3
**Labels:** `milestone:v0.3.0`, `area:hooks`

---

### S12: Filter Chains

**Type:** Story
**Title:** PreLLM and PostTool filter chains
**Description:** Implement the PreLLMFilter and PostToolFilter chains with `Pass`, `Redact`, `Log`, `Block` decisions.
**Priority:** High
**Dependencies:** S3
**Labels:** `milestone:v0.3.0`, `area:filters`

#### T12.1: PreLLMFilter chain

**Type:** Task
**Title:** Implement `PreLLMFilter` chain with 4 filter decisions
**Description:** Filter chain that processes content before sending to the LLM. Supports `Pass`, `Redact`, `Log`, `Block` decisions.
**Acceptance criteria:**
- Multiple filters execute in order
- `Block` short-circuits and prevents LLM call
- `Redact` modifies content in place
- `Log` records but passes through
**Priority:** High
**Labels:** `milestone:v0.3.0`, `area:filters`

#### T12.2: PostToolFilter chain

**Type:** Task
**Title:** Implement `PostToolFilter` chain with 4 filter decisions
**Description:** Filter chain that processes tool output before returning to the orchestrator. Same decision types as PreLLMFilter.
**Acceptance criteria:**
- Multiple filters execute in order
- `Block` prevents tool output from reaching the LLM
- `Redact` sanitises tool output
**Priority:** High
**Labels:** `milestone:v0.3.0`, `area:filters`

#### T12.3: FilterDecision type

**Type:** Task
**Title:** Define `FilterDecision` type
**Description:** Define the shared `FilterDecision` type used by both filter chains: `Pass`, `Redact`, `Log`, `Block`.
**Acceptance criteria:**
- 4 decision constants defined and exported
- Shared between PreLLMFilter and PostToolFilter
**Priority:** High
**Labels:** `milestone:v0.3.0`, `area:filters`

---

### S13: Budget Guard (4 dimensions)

**Type:** Story
**Title:** Budget enforcement across 4 dimensions
**Description:** Implement the full budget guard enforcing wall-clock duration, total tokens, tool call count, and cost estimate in micro-dollars, with per-invocation price snapshots.
**Priority:** High
**Dependencies:** S3
**Labels:** `milestone:v0.3.0`, `area:budget`

#### T13.1: budget.Guard implementation

**Type:** Task
**Title:** Implement `budget.Guard` with 4-dimension enforcement
**Description:** Implement the budget guard that checks all 4 dimensions on each relevant state transition.
**Acceptance criteria:**
- Satisfies `budget.Guard` interface
- Checks all 4 dimensions
- Returns `BudgetExceededError` identifying the offending dimension
**Priority:** High
**Labels:** `milestone:v0.3.0`, `area:budget`

#### T13.2: Wall-clock duration enforcement

**Type:** Task
**Title:** Implement wall-clock duration budget enforcement
**Description:** Wall-clock timer starts at `Initializing` entry and stops at terminal state per D25.
**Acceptance criteria:**
- Timer starts at `Initializing`
- Breach triggers `BudgetExceeded` terminal state
**Decision refs:** D25
**Priority:** High
**Dependencies:** T13.1
**Labels:** `milestone:v0.3.0`, `area:budget`

#### T13.3: Token count enforcement

**Type:** Task
**Title:** Implement total token count budget enforcement
**Description:** Track cumulative token usage (input + output) across LLM calls within an invocation.
**Acceptance criteria:**
- Cumulative tokens tracked across multi-turn
- Breach triggers `BudgetExceeded` with token dimension identified
**Priority:** High
**Dependencies:** T13.1
**Labels:** `milestone:v0.3.0`, `area:budget`

#### T13.4: Tool call count enforcement

**Type:** Task
**Title:** Implement tool call count budget enforcement
**Description:** Track the number of tool calls within an invocation.
**Acceptance criteria:**
- Tool call count incremented on each tool execution
- Breach triggers `BudgetExceeded` with tool-call dimension identified
**Priority:** High
**Dependencies:** T13.1
**Labels:** `milestone:v0.3.0`, `area:budget`

#### T13.5: Cost estimate enforcement (micro-dollars)

**Type:** Task
**Title:** Implement cost estimate budget enforcement in micro-dollars
**Description:** Track cumulative cost using the `PriceProvider` snapshot. Cost is calculated in micro-dollars for precision.
**Acceptance criteria:**
- Cost calculated from token usage × price snapshot
- Breach triggers `BudgetExceeded` with cost dimension identified
**Priority:** High
**Dependencies:** T13.1, T13.6
**Labels:** `milestone:v0.3.0`, `area:budget`

#### T13.6: PriceProvider per-invocation snapshot

**Type:** Task
**Title:** Implement `PriceProvider` per-invocation snapshot at Initializing
**Description:** Snapshot the price provider rates at `Initializing` entry per D26, so pricing is consistent throughout the invocation.
**Acceptance criteria:**
- Prices captured once at `Initializing`
- Snapshot used for all cost calculations within the invocation
**Decision refs:** D08, D26
**Priority:** High
**Dependencies:** T13.1
**Labels:** `milestone:v0.3.0`, `area:budget`

#### T13.7: BudgetExceeded terminal state

**Type:** Task
**Title:** Wire `BudgetExceeded` terminal state into the invocation loop
**Description:** When any budget dimension is breached, the state machine transitions to `BudgetExceeded` and the invocation terminates with a `BudgetExceededError`.
**Acceptance criteria:**
- State transitions to `BudgetExceeded`
- Error identifies the breached dimension
- Hard cancel semantics (no grace window)
**Priority:** High
**Dependencies:** T13.1
**Labels:** `milestone:v0.3.0`, `area:budget`

---

### S14: OTel Telemetry

**Type:** Story
**Title:** OpenTelemetry span tree, lifecycle events, metrics recorder
**Description:** Implement the OTel emitter with the span tree, all 21 invocation events, the AttributeEnricher flow, and the MetricsRecorder interface.
**Priority:** High
**Dependencies:** S3
**Labels:** `milestone:v0.3.0`, `area:telemetry`

#### T14.1: telemetry.OTelEmitter (1 root + 6 child spans)

**Type:** Task
**Title:** Implement `telemetry.OTelEmitter` with OTel span tree
**Description:** Default emitter that creates 1 root span (`praxis.invocation`) and 6 child spans for I/O-bound phases. No span for `ToolDecision` (sub-microsecond CPU work).
**Acceptance criteria:**
- Root span created at invocation start
- 6 child spans for I/O phases
- `ApprovalRequired` maps to `StatusOK`
- Spans carry standard OTel attributes
**Decision refs:** D53
**Priority:** High
**Labels:** `milestone:v0.3.0`, `area:telemetry`

#### T14.2: 21 InvocationEvent emission

**Type:** Task
**Title:** Emit all 21 `InvocationEvent` types through the emitter
**Description:** Wire all 21 event types through the emitter at the correct points in the invocation lifecycle.
**Acceptance criteria:**
- All 21 event types emitted at their documented lifecycle points
- Events carry correct metadata (state, timestamp, error if terminal)
**Decision refs:** D18, D52b
**Priority:** High
**Dependencies:** T14.1, T9.1
**Labels:** `milestone:v0.3.0`, `area:telemetry`

#### T14.3: AttributeEnricher flow

**Type:** Task
**Title:** Implement `AttributeEnricher` flow at Initializing
**Description:** Call `AttributeEnricher.Enrich` once at `Initializing` (after root span opened). Attributes are added to spans and `InvocationEvent.EnricherAttributes`, never to metric labels (hard cardinality boundary).
**Acceptance criteria:**
- Enricher called once at `Initializing`
- Attributes appear on spans and events
- Attributes never appear as metric labels
**Decision refs:** D60
**Priority:** High
**Dependencies:** T14.1
**Labels:** `milestone:v0.3.0`, `area:telemetry`

#### T14.4: MetricsRecorder interface and NullMetricsRecorder

**Type:** Task
**Title:** Implement `MetricsRecorder` interface with null default and Prometheus constructor
**Description:** Define the `MetricsRecorder` interface, implement `NullMetricsRecorder` default, and provide `NewPrometheusRecorder` constructor. Add `WithMetricsRecorder` option to the orchestrator.
**Acceptance criteria:**
- `MetricsRecorder` interface defined
- `NullMetricsRecorder` satisfies interface (no-ops)
- `NewPrometheusRecorder` constructor exists
- `WithMetricsRecorder` option wires the recorder
**Decision refs:** D57, D65
**Priority:** High
**Labels:** `milestone:v0.3.0`, `area:telemetry`

---

### S15: OpenAI Provider

**Type:** Story
**Title:** OpenAI LLM provider adapter with Azure support
**Description:** Implement the OpenAI adapter and validate it against the shared conformance suite. Include Azure OpenAI support via base-URL configuration.
**Priority:** High
**Dependencies:** S5
**Labels:** `milestone:v0.3.0`, `area:llm`

#### T15.1: openai.Provider implementation

**Type:** Task
**Title:** Implement `openai.Provider`
**Description:** Implement the OpenAI Chat Completions API adapter satisfying `llm.Provider`.
**Acceptance criteria:**
- Implements `llm.Provider` interface completely
- Maps OpenAI API errors to appropriate `TypedError` types
- Supports tool use (function calling)
**Priority:** High
**Labels:** `milestone:v0.3.0`, `area:llm`

#### T15.2: Azure OpenAI via base-URL

**Type:** Task
**Title:** Support Azure OpenAI via base-URL configuration
**Description:** Azure OpenAI compatibility via configurable base URL. Best-effort parity per D14.
**Acceptance criteria:**
- Base URL configurable via option
- Azure OpenAI deployments callable with correct auth headers
**Decision refs:** D14
**Priority:** Medium
**Dependencies:** T15.1
**Labels:** `milestone:v0.3.0`, `area:llm`

#### T15.3: Shared conformance suite

**Type:** Task
**Title:** Provider conformance suite shared between Anthropic and OpenAI
**Description:** A conformance test suite that both providers must pass, ensuring consistent behaviour across adapters.
**Acceptance criteria:**
- Same test cases run against both providers
- Both pass the full suite
- Suite is extensible for future providers
**Decision refs:** D88
**Priority:** High
**Dependencies:** T15.1, T5.3
**Labels:** `milestone:v0.3.0`, `area:llm`, `area:testing`

---

### S16: Quality Gate v0.3

**Type:** Story
**Title:** v0.3.0 quality gate verification
**Description:** Verify all v0.3.0 quality and interface stability exit criteria.
**Priority:** High
**Dependencies:** S9–S15
**Labels:** `milestone:v0.3.0`, `area:quality`

#### T16.1: All 14 interfaces at Phase 3 method surfaces

**Type:** Task
**Title:** Verify all 14 public interfaces match Phase 3 method surfaces
**Description:** Audit all 14 public interfaces against their Phase 3 contract definitions (D31–D52).
**Acceptance criteria:**
- Each interface matches its Phase 3 method surface exactly
- No missing or extra methods
**Decision refs:** D31–D52
**Priority:** High
**Labels:** `milestone:v0.3.0`, `area:quality`

#### T16.2: Property tests 10k CI + 100k nightly

**Type:** Task
**Title:** Configure property tests: 10k in CI, 100k nightly
**Description:** Property-based state machine tests run at 10k iterations in PR CI and 100k iterations in nightly jobs. Auto issue creation on nightly failure.
**Acceptance criteria:**
- PR CI runs 10k iterations
- Nightly job runs 100k iterations
- Auto issue created on nightly failure
**Decision refs:** D87
**Priority:** High
**Labels:** `milestone:v0.3.0`, `area:testing`, `area:ci`

#### T16.3: 85% coverage maintained

**Type:** Task
**Title:** Verify 85% line coverage maintained across all packages
**Description:** Ensure coverage has not regressed below 85% with the addition of v0.3.0 code.
**Acceptance criteria:**
- All packages including `internal/` at >= 85%
**Decision refs:** D86
**Priority:** High
**Labels:** `milestone:v0.3.0`, `area:quality`

#### T16.4: Nightly conformance suite

**Type:** Task
**Title:** Configure nightly LLM conformance suite
**Description:** Nightly CI job running the LLM conformance suite against Anthropic and OpenAI. Budget-capped at $0.50/run. Auto issue on failure.
**Acceptance criteria:**
- Nightly job runs conformance suite
- Budget cap enforced
- Auto issue on failure
**Decision refs:** D88
**Priority:** Medium
**Labels:** `milestone:v0.3.0`, `area:testing`, `area:ci`

---

### S17: Examples v0.3

**Type:** Story
**Title:** v0.3.0 example programs
**Description:** Runnable examples demonstrating tools, policy hooks, filters, and streaming.
**Priority:** Medium
**Dependencies:** S9, S11, S12
**Labels:** `milestone:v0.3.0`, `area:examples`

#### T17.1: examples/tools/

**Type:** Task
**Title:** Create `examples/tools/` — tool invocation example
**Acceptance criteria:**
- Demonstrates tool definition and invocation
- Compiles and runs with API key
**Priority:** Medium
**Labels:** `milestone:v0.3.0`, `area:examples`

#### T17.2: examples/policy/

**Type:** Task
**Title:** Create `examples/policy/` — custom PolicyHook example
**Acceptance criteria:**
- Demonstrates custom PolicyHook implementation
- Shows Allow, Deny, and Log decisions
**Priority:** Medium
**Labels:** `milestone:v0.3.0`, `area:examples`

#### T17.3: examples/filters/

**Type:** Task
**Title:** Create `examples/filters/` — PreLLM and PostTool filter example
**Acceptance criteria:**
- Demonstrates both filter types
- Shows Redact and Block decisions
**Priority:** Medium
**Labels:** `milestone:v0.3.0`, `area:examples`

#### T17.4: examples/streaming/

**Type:** Task
**Title:** Create `examples/streaming/` — InvokeStream with channel draining
**Acceptance criteria:**
- Demonstrates `InvokeStream` usage
- Shows proper channel draining pattern
- Handles cancellation gracefully
**Priority:** Medium
**Labels:** `milestone:v0.3.0`, `area:examples`

---

## Epic 3: PRAX — v0.5.0 Feature Complete

**Type:** Epic
**Title:** v0.5.0 — Feature Complete
**Description:** Production-ready quality with all features implemented: identity signing, credentials management, security hardening, full observability, benchmarks, and conformance. Ready for the first production consumer. All v0.3.0 criteria remain satisfied.
**Priority:** High
**Dependencies:** Epic 2 (v0.3.0)
**Labels:** `milestone:v0.5.0`

---

### S18: Identity Signing (Ed25519)

**Type:** Story
**Title:** Ed25519 identity signing with JWT
**Description:** Implement the Ed25519 reference signer, JWT construction with mandatory and custom claims, configurable token lifetime, and identity chaining.
**Priority:** Highest
**Dependencies:** S3
**Labels:** `milestone:v0.5.0`, `area:identity`

#### T18.1: identity.Ed25519Signer

**Type:** Task
**Title:** Implement `identity.NewEd25519Signer`
**Description:** `NewEd25519Signer(key ed25519.PrivateKey, ...SignerOption) (Signer, error)`. Stdlib-only: `crypto/ed25519`, `encoding/json`, `encoding/base64`, `crypto/rand`. JOSE header: `{"alg":"EdDSA","typ":"JWT"}`.
**Acceptance criteria:**
- Constructor validates key and options
- Signs tokens with EdDSA algorithm
- Stdlib-only implementation (no third-party JWT library)
**Decision refs:** D73
**Priority:** Highest
**Labels:** `milestone:v0.5.0`, `area:identity`

#### T18.2: JWT with 5 registered + 2 custom claims

**Type:** Task
**Title:** Implement JWT with mandatory registered and custom claims
**Description:** 5 mandatory registered claims: `iss`, `sub`, `exp`, `iat`, `jti`. 2 mandatory custom claims: `praxis.invocation_id`, `praxis.tool_name`. `iss` defaults to `"praxis"`.
**Acceptance criteria:**
- All 7 mandatory claims present in every token
- `iss` overridable via `WithIssuer`
- Mandatory claims win on collision with extra claims
**Decision refs:** D70, D71
**Priority:** Highest
**Dependencies:** T18.1
**Labels:** `milestone:v0.5.0`, `area:identity`

#### T18.3: Token lifetime [5s, 300s]

**Type:** Task
**Title:** Configurable token lifetime with [5s, 300s] range
**Description:** Default lifetime 60s, configurable via `WithTokenLifetime`. Out-of-range values rejected at construction time.
**Acceptance criteria:**
- Default lifetime is 60 seconds
- Range [5s, 300s] enforced at construction
- Out-of-range returns error (not panic)
**Decision refs:** D72
**Priority:** High
**Dependencies:** T18.1
**Labels:** `milestone:v0.5.0`, `area:identity`

#### T18.4: With* signer options

**Type:** Task
**Title:** Implement `WithIssuer`, `WithTokenLifetime`, `WithKeyID`, `WithExtraClaims`
**Description:** Construction options for the Ed25519 signer.
**Acceptance criteria:**
- `WithIssuer` overrides default `"praxis"` issuer
- `WithTokenLifetime` sets custom lifetime within range
- `WithKeyID` sets the `kid` header for verifier key selection (D74)
- `WithExtraClaims(map[string]any)` adds static caller claims
**Decision refs:** D70, D71, D72, D74
**Priority:** High
**Dependencies:** T18.1
**Labels:** `milestone:v0.5.0`, `area:identity`

#### T18.5: internal/jwt stdlib-only package

**Type:** Task
**Title:** Implement `internal/jwt` package with stdlib-only JWT construction
**Description:** Internal package for JWT construction using only Go standard library. Houses claim constants and encoding logic.
**Acceptance criteria:**
- No third-party dependencies
- JWT encoding/signing correct per RFC 7519
- Claim constants defined here per D101
**Decision refs:** D99, D101
**Priority:** High
**Labels:** `milestone:v0.5.0`, `area:identity`

#### T18.6: Identity chaining via praxis.parent_token

**Type:** Task
**Title:** Implement identity chaining via `praxis.parent_token` claim
**Description:** When an orchestrator is nested, the outer JWT is carried as the `praxis.parent_token` payload claim. Documentation recommends max 3 levels of chain depth (not enforced).
**Acceptance criteria:**
- Nested orchestrator passes parent JWT as `praxis.parent_token`
- Chain depth is documented but not enforced
**Decision refs:** D75
**Priority:** Medium
**Dependencies:** T18.2
**Labels:** `milestone:v0.5.0`, `area:identity`

---

### S19: Credentials Management

**Type:** Story
**Title:** Credential resolver with soft-cancel and secure zeroing
**Description:** Implement credential resolution with soft-cancel context, secure byte zeroing, and `runtime.KeepAlive`-fenced `Close()`.
**Priority:** High
**Labels:** `milestone:v0.5.0`, `area:credentials`

#### T19.1: credentials.Resolver.Fetch with soft-cancel

**Type:** Task
**Title:** Implement `Resolver.Fetch` with soft-cancel context
**Description:** `Fetch` uses `context.WithoutCancel` + 500ms `context.WithTimeout` so credential resolution is not hard-cancelled during graceful shutdown.
**Acceptance criteria:**
- Fetch continues for up to 500ms after parent cancellation
- Returns error after 500ms timeout
**Decision refs:** D69
**Priority:** High
**Labels:** `milestone:v0.5.0`, `area:credentials`

#### T19.2: credentials.ZeroBytes utility

**Type:** Task
**Title:** Implement `credentials.ZeroBytes` exported utility
**Description:** Exported helper that zeroes a byte slice. Centralises the zeroing pattern for third-party `Credential` implementations.
**Acceptance criteria:**
- `ZeroBytes([]byte)` overwrites all bytes with 0
- Exported for use by third-party implementors
**Decision refs:** D68
**Priority:** High
**Labels:** `milestone:v0.5.0`, `area:credentials`

#### T19.3: Credential.Close() with runtime.KeepAlive

**Type:** Task
**Title:** Implement `Credential.Close()` with `runtime.KeepAlive`-fenced zeroing
**Description:** `Close()` zeroes credential bytes with a `runtime.KeepAlive` fence to prevent dead-store elision by the Go compiler.
**Acceptance criteria:**
- Byte slice overwritten on `Close()`
- `runtime.KeepAlive` prevents compiler optimisation
- No credential material remains in memory after `Close()`
**Decision refs:** D67
**Priority:** High
**Labels:** `milestone:v0.5.0`, `area:credentials`

---

### S20: Security Hardening

**Type:** Story
**Title:** Panic recovery, trust boundaries, and security invariant tests
**Description:** Add panic recovery on all hook/filter call sites, implement trust boundary logging levels, and verify all 26 security invariants.
**Priority:** High
**Dependencies:** S11, S12
**Labels:** `milestone:v0.5.0`, `area:security`

#### T20.1: Panic recovery on hook/filter call sites

**Type:** Task
**Title:** Add deferred `recover()` on all hook and filter call sites
**Description:** Every hook and filter invocation must be wrapped in deferred panic recovery to prevent caller panics from crashing the orchestrator.
**Acceptance criteria:**
- All hook call sites have deferred `recover()`
- All filter call sites have deferred `recover()`
- Recovered panics are logged and converted to appropriate errors
**Decision refs:** D78
**Priority:** High
**Labels:** `milestone:v0.5.0`, `area:security`

#### T20.2: PostToolFilter ERROR, PreLLMFilter WARN

**Type:** Task
**Title:** Implement trust boundary logging levels for filters
**Description:** PostToolFilter errors log at ERROR (trust-boundary-crossing). PreLLMFilter errors log at WARN (trust-boundary-internal).
**Acceptance criteria:**
- PostToolFilter errors/panics logged at ERROR level
- PreLLMFilter errors/panics logged at WARN level
**Decision refs:** D78
**Priority:** Medium
**Dependencies:** T20.1
**Labels:** `milestone:v0.5.0`, `area:security`

#### T20.3: 26 security invariant tests

**Type:** Task
**Title:** Test all 26 security invariants from D80
**Description:** Implement tests for all 26 security invariants across 4 categories: C1–C8 (credential isolation), I1–I6 (identity signing), T1–T7 (trust boundaries), O1–O5 (observability safety).
**Acceptance criteria:**
- Each of the 26 invariants has a dedicated test
- All tests pass
- Traceability to D80 categories documented
**Decision refs:** D80
**Priority:** High
**Labels:** `milestone:v0.5.0`, `area:security`, `area:testing`

---

### S21: Full Observability

**Type:** Story
**Title:** Prometheus metrics, slog redaction, filter-to-event and error-to-event mapping
**Description:** Complete the observability surface: 10 Prometheus metrics, RedactingHandler, FilterDecision event mapping, and error-to-event 1:1 mapping.
**Priority:** High
**Dependencies:** S14
**Labels:** `milestone:v0.5.0`, `area:telemetry`

#### T21.1: telemetry/slog RedactingHandler

**Type:** Task
**Title:** Implement `telemetry/slog.RedactingHandler`
**Description:** slog handler that redacts sensitive attributes. Deny-list covers credentials, raw content, PII, `praxis.signed_identity`, and `_jwt` suffix per D79.
**Acceptance criteria:**
- Deny-list includes all items from D58 and D79
- Redacted values replaced with `[REDACTED]`
- Never-log list prevents sensitive data from reaching any slog output
**Decision refs:** D58, D79
**Priority:** High
**Labels:** `milestone:v0.5.0`, `area:telemetry`

#### T21.2: 10 Prometheus metrics

**Type:** Task
**Title:** Implement 10 Prometheus metrics with bounded cardinality
**Description:** Implement all 10 metrics with `praxis_` prefix. All labels bounded. Hard cardinality boundary: enricher attributes go to spans only, never metric labels. ~1,032 worst-case time series.
**Acceptance criteria:**
- All 10 metrics from D57 implemented
- Labels bounded per specification
- No enricher attributes in metric labels
**Decision refs:** D57
**Priority:** High
**Labels:** `milestone:v0.5.0`, `area:telemetry`

#### T21.3: FilterDecision to content-analysis events

**Type:** Task
**Title:** Map `FilterDecision` to content-analysis events
**Description:** Implement the mapping from filter decisions to content-analysis events per D59. Emission occurs before the enclosing state-transition event.
**Acceptance criteria:**
- Filter decisions trigger content-analysis events
- Events emitted before state-transition events
- Reason-driven trigger logic implemented
**Decision refs:** D59
**Priority:** Medium
**Labels:** `milestone:v0.5.0`, `area:telemetry`

#### T21.4: Error-to-event 1:1 mapping

**Type:** Task
**Title:** Implement 1:1 ErrorKind to terminal EventType mapping
**Description:** Each `ErrorKind` maps to exactly one terminal `EventType`. First-error-wins arbitration via state machine immutability.
**Acceptance criteria:**
- 1:1 mapping implemented for all error kinds
- First error wins (no overwriting terminal events)
**Decision refs:** D61
**Priority:** Medium
**Labels:** `milestone:v0.5.0`, `area:telemetry`

---

### S22: Error Model Refinements

**Type:** Story
**Title:** Error model refinements for v0.5.0
**Description:** Finalise error model details: BudgetExceeded godoc, classifier identity rule, and VerdictLog emission.
**Priority:** Medium
**Dependencies:** S4, S13
**Labels:** `milestone:v0.5.0`, `area:errors`

#### T22.1: BudgetExceededError godoc token-overshoot

**Type:** Task
**Title:** Add token-overshoot caveat to `BudgetExceededError` godoc
**Description:** Godoc for `BudgetExceededError` must document the token-overshoot caveat: actual tokens may exceed the budget slightly because checks happen between LLM calls, not mid-stream.
**Acceptance criteria:**
- Godoc clearly states the overshoot caveat
**Decision refs:** D62
**Priority:** Medium
**Labels:** `milestone:v0.5.0`, `area:errors`

#### T22.2: Classifier identity rule via errors.As

**Type:** Task
**Title:** Implement classifier identity rule using `errors.As`
**Description:** Classifier must use `errors.As` as the primary identification mechanism before falling back to other heuristics.
**Acceptance criteria:**
- `errors.As` tried first for classification
- Fallback heuristics documented
- Four worked examples from D63 pass as tests
**Decision refs:** D63
**Priority:** Medium
**Labels:** `milestone:v0.5.0`, `area:errors`

#### T22.3: VerdictLog AuditNote

**Type:** Task
**Title:** Implement VerdictLog emission via `AuditNote` field
**Description:** Hook-completion events carry an `AuditNote` field for verdict logging. No new `EventType` constant needed.
**Acceptance criteria:**
- `AuditNote` field present on hook-completion events
- Verdict information accessible via the field
**Decision refs:** D64
**Priority:** Low
**Labels:** `milestone:v0.5.0`, `area:errors`

---

### S23: Benchmarks and Conformance

**Type:** Story
**Title:** Benchmark baselines and conformance suite green
**Description:** Establish performance benchmarks and ensure the conformance suite passes for both providers.
**Priority:** High
**Dependencies:** S3, S15
**Labels:** `milestone:v0.5.0`, `area:benchmarks`

#### T23.1: Orchestrator overhead < 15ms

**Type:** Task
**Title:** Benchmark and verify orchestrator overhead under 15ms
**Description:** Benchmark the orchestrator overhead per invocation (LLM time excluded). Target: under 15ms.
**Acceptance criteria:**
- Benchmark exists and passes
- Overhead consistently under 15ms
**Priority:** High
**Labels:** `milestone:v0.5.0`, `area:benchmarks`

#### T23.2: State machine 1M transitions/sec/core

**Type:** Task
**Title:** Benchmark state machine at 1M transitions/sec/core
**Description:** Benchmark the state machine transition throughput. Target: 1M transitions per second per core.
**Acceptance criteria:**
- Benchmark exists and passes
- Throughput consistently at or above 1M/sec/core
**Priority:** High
**Labels:** `milestone:v0.5.0`, `area:benchmarks`

#### T23.3: benchstat in PR CI

**Type:** Task
**Title:** Configure benchstat comparison in PR CI
**Description:** PR CI compares benchmark results against the post-merge baseline cache using benchstat per D105.
**Acceptance criteria:**
- benchstat comparison runs on PRs
- Regressions flagged in PR comments
**Decision refs:** D105
**Priority:** Medium
**Labels:** `milestone:v0.5.0`, `area:benchmarks`, `area:ci`

#### T23.4: Conformance suite green for both providers

**Type:** Task
**Title:** LLM conformance suite green for Anthropic and OpenAI
**Description:** The shared conformance suite must pass for both Anthropic and OpenAI adapters.
**Acceptance criteria:**
- Both providers pass the full conformance suite
- Suite runs nightly in CI
**Decision refs:** D88
**Priority:** High
**Dependencies:** T15.3
**Labels:** `milestone:v0.5.0`, `area:llm`, `area:testing`

---

### S24: Quality Gate v0.5

**Type:** Story
**Title:** v0.5.0 quality gate verification
**Description:** Verify all v0.5.0 quality exit criteria including coverage, CI jobs, godoc, and interface integration tests.
**Priority:** High
**Dependencies:** S18–S23
**Labels:** `milestone:v0.5.0`, `area:quality`

#### T24.1: 85% coverage maintained

**Type:** Task
**Title:** Verify 85% line coverage maintained
**Acceptance criteria:**
- All packages at >= 85%
**Decision refs:** D86
**Priority:** High
**Labels:** `milestone:v0.5.0`, `area:quality`

#### T24.2: All CI jobs operational

**Type:** Task
**Title:** Verify all CI jobs operational (7 PR + 2 nightly + CodeQL weekly)
**Description:** All CI jobs from D85 must be operational and passing.
**Acceptance criteria:**
- 8 on-PR jobs running (6 required + 2 informational)
- 2 nightly jobs running (property-tests, conformance)
- CodeQL running weekly + on-PR
**Decision refs:** D85
**Priority:** High
**Labels:** `milestone:v0.5.0`, `area:ci`

#### T24.3: Godoc on every exported symbol

**Type:** Task
**Title:** Verify godoc on every exported symbol
**Description:** Every exported type, function, method, and constant must have godoc.
**Acceptance criteria:**
- `go vet` or linter reports no missing godoc on exported symbols
**Priority:** Medium
**Labels:** `milestone:v0.5.0`, `area:quality`

#### T24.4: Integration tests for all 14 public interfaces

**Type:** Task
**Title:** Integration tests exercising all 14 public interfaces
**Description:** Each of the 14 public interfaces must be exercised by at least one integration test: `orchestrator.AgentOrchestrator`, `llm.Provider`, `tools.Invoker`, `hooks.PolicyHook`, `hooks.PreLLMFilter`, `hooks.PostToolFilter`, `budget.Guard`, `budget.PriceProvider`, `telemetry.Emitter`, `telemetry.AttributeEnricher`, `telemetry.MetricsRecorder`, `credentials.Resolver`, `identity.Signer`, `errors.Classifier`.
**Acceptance criteria:**
- Each of the 14 interfaces has at least one integration test
- Tests use non-null implementations (not the noop defaults)
**Priority:** High
**Labels:** `milestone:v0.5.0`, `area:quality`, `area:testing`

---

### S25: Examples v0.5

**Type:** Story
**Title:** v0.5.0 example programs
**Description:** Runnable examples for identity signing and credential management.
**Priority:** Medium
**Dependencies:** S18, S19
**Labels:** `milestone:v0.5.0`, `area:examples`

#### T25.1: examples/identity/

**Type:** Task
**Title:** Create `examples/identity/` — Ed25519Signer usage
**Acceptance criteria:**
- Demonstrates `NewEd25519Signer` with key generation
- Shows token signing and claim inspection
**Priority:** Medium
**Labels:** `milestone:v0.5.0`, `area:examples`

#### T25.2: examples/credentials/

**Type:** Task
**Title:** Create `examples/credentials/` — custom Resolver with non-null implementation
**Description:** Example demonstrating a custom `credentials.Resolver` implementation (non-null reference implementation).
**Acceptance criteria:**
- Demonstrates a non-null Resolver implementation
- Shows `Fetch`, `Close()`, and `ZeroBytes` usage
**Priority:** Medium
**Labels:** `milestone:v0.5.0`, `area:examples`

---

## Epic 4: PRAX — v1.0.0 API Freeze

**Type:** Epic
**Title:** v1.0.0 — API Freeze
**Description:** The stability commitment. Interface surface frozen, semver contract in effect, first production consumer validated. All v0.5.0 criteria remain satisfied.
**Priority:** High
**Dependencies:** Epic 3 (v0.5.0)
**Labels:** `milestone:v1.0.0`

---

### S26: Production Consumer Gate

**Type:** Story
**Title:** Production consumer attestation gate
**Description:** v1.0.0 requires a production consumer to have shipped against a v0.5.x tag in production (serving real traffic, not staging). Maintainer attestation recorded in release notes.
**Priority:** Highest
**Dependencies:** Epic 3 (v0.5.0)
**Labels:** `milestone:v1.0.0`, `area:governance`

#### T26.1: Consumer attestation in release notes

**Type:** Task
**Title:** Record production consumer attestation in v1.0.0 release notes
**Description:** Dedicated release notes section documenting: consumer identity, version used, date of production deployment.
**Acceptance criteria:**
- Release notes contain consumer attestation section
- Consumer identity, version, and deployment date recorded
**Decision refs:** D91
**Priority:** Highest
**Labels:** `milestone:v1.0.0`, `area:governance`

---

### S27: API Freeze

**Type:** Story
**Title:** Freeze all 14 interfaces at frozen-v1.0
**Description:** Confirm all interfaces at `frozen-v1.0`, remove any remaining `stable-v0.x-candidate` interfaces, and set the version.
**Priority:** Highest
**Dependencies:** S26
**Labels:** `milestone:v1.0.0`, `area:api`

#### T27.1: 14 interfaces frozen-v1.0

**Type:** Task
**Title:** Confirm all 14 interfaces at `frozen-v1.0`
**Description:** Verify all 14 public interfaces are at `frozen-v1.0` stability tier. No `stable-v0.x-candidate` interfaces remaining.
**Acceptance criteria:**
- All 14 interfaces audited against Phase 3 contracts
- All at `frozen-v1.0` (none at `stable-v0.x-candidate`)
**Decision refs:** D04, D76
**Priority:** Highest
**Labels:** `milestone:v1.0.0`, `area:api`

#### T27.2: internal/version/version.go = 1.0.0

**Type:** Task
**Title:** Set `internal/version/version.go` to `1.0.0`
**Description:** Update the version constant to `1.0.0` for the release.
**Acceptance criteria:**
- `version.go` reads `1.0.0`
**Priority:** High
**Dependencies:** T27.1
**Labels:** `milestone:v1.0.0`, `area:build`

#### T27.3: No stable-v0.x-candidate remaining

**Type:** Task
**Title:** Verify no `stable-v0.x-candidate` interfaces remain
**Description:** Audit the codebase for any interfaces still at `stable-v0.x-candidate`. All must be promoted to `frozen-v1.0` or explicitly deferred to `post-v1`.
**Acceptance criteria:**
- Grep for `stable-v0.x-candidate` returns zero matches
- Any deferred interfaces documented
**Priority:** High
**Labels:** `milestone:v1.0.0`, `area:api`

---

### S28: Governance and Docs v1.0

**Type:** Story
**Title:** v1.0.0 governance, documentation, and CI hardening
**Description:** Final documentation updates, CI hardening, and governance setup for the v1.0.0 release.
**Priority:** High
**Dependencies:** S27
**Labels:** `milestone:v1.0.0`, `area:governance`, `area:docs`

#### T28.1: SECURITY.md with OI-1, OI-2

**Type:** Task
**Title:** Update SECURITY.md with OI-1 and OI-2 known limitations
**Description:** Document the two known security limitations: OI-1 (private key in-memory lifetime) and OI-2 (enricher attribute log-injection vector).
**Acceptance criteria:**
- OI-1 and OI-2 documented as known limitations
- Mitigation guidance provided
**Decision refs:** D96
**Priority:** High
**Labels:** `milestone:v1.0.0`, `area:docs`, `area:security`

#### T28.2: RFC process (Discussion category)

**Type:** Task
**Title:** Create "RFC" Discussion category on GitHub
**Description:** Set up the RFC process via GitHub Discussions per D95. Active post-v1.0 only.
**Acceptance criteria:**
- "RFC" Discussion category created
- Process documented in CONTRIBUTING.md
**Decision refs:** D95
**Priority:** Medium
**Labels:** `milestone:v1.0.0`, `area:governance`

#### T28.3: CONTRIBUTING.md v1.0 review requirements

**Type:** Task
**Title:** Update CONTRIBUTING.md with v1.0 review requirements
**Description:** Update review policy: 2 approvals required for frozen interface changes post-v1.0 (up from 1 during v0.x).
**Acceptance criteria:**
- v1.0 review requirements documented
- Frozen interface change process described
**Decision refs:** D93
**Priority:** Medium
**Labels:** `milestone:v1.0.0`, `area:docs`

#### T28.4: govulncheck promoted to required

**Type:** Task
**Title:** Promote govulncheck from informational to required check
**Description:** govulncheck was informational during v0.x. Promote to required PR check for v1.0.0.
**Acceptance criteria:**
- govulncheck is a required (blocking) PR check
- CI config updated
**Decision refs:** D85
**Priority:** High
**Labels:** `milestone:v1.0.0`, `area:ci`

#### T28.5: Nightly conformance suite stable (30 days)

**Type:** Task
**Title:** Verify nightly conformance suite stable for 30 consecutive days
**Description:** No flaky failures in the nightly conformance suite for 30 days before v1.0.0 tag.
**Acceptance criteria:**
- 30 consecutive days of clean nightly runs
- No auto-created issues from conformance failures
**Decision refs:** D88
**Priority:** High
**Labels:** `milestone:v1.0.0`, `area:testing`

#### T28.6: CHANGELOG v0.x to v1.0

**Type:** Task
**Title:** Finalise CHANGELOG.md covering v0.x to v1.0 journey
**Description:** Ensure the CHANGELOG covers all notable changes from v0.1.0 through v1.0.0.
**Acceptance criteria:**
- CHANGELOG entries for all v0.x releases
- v1.0.0 entry summarises the stability commitment
**Decision refs:** D84
**Priority:** Medium
**Labels:** `milestone:v1.0.0`, `area:docs`

#### T28.7: README with stability commitment

**Type:** Task
**Title:** Update README with stability commitment and versioning policy link
**Description:** Add a stability commitment section to the README and link to `docs/phase-6-release-governance/04-versioning-policy.md`.
**Acceptance criteria:**
- Stability commitment section in README
- Link to versioning policy document
- Semver contract explained
**Priority:** Medium
**Labels:** `milestone:v1.0.0`, `area:docs`

---

## Appendix: Ticket Totals

| Level | Count |
|-------|-------|
| Epics | 4 |
| Stories | 28 |
| Tasks | 92 |
| **Total tickets** | **124** |

## Appendix: Label Taxonomy

| Label | Description |
|-------|-------------|
| `milestone:v0.1.0` | First Invocation |
| `milestone:v0.3.0` | Interfaces Stable |
| `milestone:v0.5.0` | Feature Complete |
| `milestone:v1.0.0` | API Freeze |
| `area:build` | Module, Makefile, CI, release tooling |
| `area:state-machine` | Invocation state machine |
| `area:orchestrator` | AgentOrchestrator facade and invocation loop |
| `area:errors` | Error taxonomy, classifier, retry |
| `area:llm` | LLM provider interfaces and adapters |
| `area:defaults` | Null/noop default implementations |
| `area:docs` | Documentation, README, CHANGELOG |
| `area:examples` | Example programs |
| `area:streaming` | InvokeStream and event channel |
| `area:cancellation` | Soft/hard cancel, detached context |
| `area:hooks` | PolicyHook chain |
| `area:filters` | PreLLM/PostTool filter chains |
| `area:budget` | Budget guard, 4 dimensions |
| `area:telemetry` | OTel, metrics, slog, events |
| `area:identity` | Ed25519 signing, JWT, identity chaining |
| `area:credentials` | Credential resolver, zeroing |
| `area:security` | Trust boundaries, panic recovery, invariants |
| `area:benchmarks` | Performance benchmarks |
| `area:testing` | Tests, conformance suite, property tests |
| `area:ci` | CI pipeline configuration |
| `area:quality` | Quality gates, coverage |
| `area:governance` | Contribution model, RFC, DCO |
| `area:api` | API freeze, interface stability |

## Appendix: Critical Path

```
D10 resolution → S1 (Module Init) → S2 (State Machine) + S4 (Errors) + S5 (Anthropic) + S6 (Defaults)
    → S3 (Orchestrator.Invoke) → S7 (Docs) + S8 (Quality Gate) → v0.1.0 tag
    → S9–S15 (Streaming, Hooks, Filters, Budget, Telemetry, OpenAI) → S16–S17 (Quality + Examples) → v0.3.0 tag
    → S18–S23 (Identity, Credentials, Security, Observability, Errors, Benchmarks) → S24–S25 (Quality + Examples) → v0.5.0 tag
    → Production consumer ships → S26–S28 (Consumer Gate, API Freeze, Governance) → v1.0.0 tag
```
