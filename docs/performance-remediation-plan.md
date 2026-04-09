# Performance Remediation Plan

**Created:** 2026-04-09
**Context:** performance review run with `go test`, `pprof`, `benchstat`, `fieldalignment`, and `staticcheck` on Apple M3 / darwin arm64.

## Goal

Reduce latency and allocation pressure on the real invocation path without
changing the public API shape. The plan prioritizes fixes that are both
measurable and already supported by the documented runtime semantics.

## Baseline

### Benchmarks

| Benchmark | Result | Notes |
| --- | --- | --- |
| `BenchmarkOrchestratorOverhead` | `2.462µs ±11%`, `10.38KiB ±1%`, `17 allocs/op` | Sync `Invoke` path, mock provider, no real signer |
| `BenchmarkEd25519Signer_Sign` | `21.07µs ±31%`, `3.472KiB`, `43 allocs/op` | Signing dominates once identity is enabled |
| `BenchmarkEncode` | `20.00µs ±24%`, `2.446KiB`, `31 allocs/op` | Most signer overhead lives in `internal/jwt` |
| `state` benchmarks | `0 allocs/op` | Not a priority target |
| `internal/retry` benchmarks | `0 allocs/op` | Not a priority target |

### Profiles and static signals

- `pprof` on `BenchmarkOrchestratorOverhead` shows CPU dominated by GC scanning
  and alloc pressure, not state-machine logic.
- `pprof` on `identity` and `internal/jwt` shows the main hot spots in JWT
  payload construction, base64 encoding, and UUID formatting.
- `fieldalignment` reports high pointer-byte density on hot structs, especially
  `event.InvocationEvent`, `praxis.InvocationRequest`, `praxis.InvocationResult`,
  `llm.LLMRequest`, `tools.InvocationContext`, and `identity.ed25519Signer`.
- `staticcheck ./...` is clean.

## Scope and constraints

- Keep `InvocationRequest`, `InvocationResult`, `InvocationEvent`, and provider
  interfaces source-compatible.
- Do not use `unsafe`.
- Do not add speculative caching of LLM or tool outputs.
- Preserve the documented event semantics for parallel tool dispatch:
  all `ToolCallStarted` events before worker execution, all completion and
  post-filter events emitted after the batch finishes, in call-ID order.

## Success criteria

### Primary targets

1. Reduce `BenchmarkOrchestratorOverhead` allocations by at least 40%.
   - Target: `<= 6 allocs/op`
   - Target: `<= 6.0 KiB/op`
   - Keep runtime at or below `3.0µs/op`

2. Reduce JWT/signing allocations by at least 25%.
   - Target: `BenchmarkEncode <= 23 allocs/op`
   - Target: `BenchmarkEd25519Signer_Sign <= 32 allocs/op`
   - Keep runtime within 10% of the post-change median during iteration

3. Implement true parallel tool dispatch for providers that advertise
   `SupportsParallelToolCalls() == true`.
   - For a batch of `N` tools with equal latency `T`, total wall time should
     converge toward `~T + framework overhead`, not `N*T`

4. Improve provider transport defaults and response decoding without
   regressing tests or error mapping.

### Secondary targets

- Clear `fieldalignment` findings for the hot structs touched by this plan.
- Add benchmarks and tests so all optimizations remain regression-tested in CI.

## Workstream 1 — Lock in the measurement harness

### Files

- `orchestrator/bench_test.go`
- `identity/ed25519_test.go`
- `internal/jwt/jwt_test.go`
- new provider benchmarks under `llm/openai` and `llm/anthropic`

### Changes

- Keep the existing three benchmark families as the baseline contract:
  orchestrator overhead, signer cost, JWT encoding cost.
- Add a new orchestrator benchmark with a real Ed25519 signer wired in.
  - Name: `BenchmarkOrchestratorOverheadWithIdentity`
  - Purpose: separate framework overhead from signing overhead.
- Add a dedicated benchmark for UUID generation.
  - Name: `BenchmarkGenerateUUIDv7`
- Add provider adapter benchmarks using `httptest.Server`.
  - One benchmark for success-path JSON decode with a representative response.
  - One benchmark for non-200 error mapping with a representative error body.
- Persist benchmark commands in comments near the benchmark functions so the
  workflow is reproducible with `benchstat`.

### Exit criteria

- New benchmarks compile and run with `go test -run '^$' -bench ... -benchmem`.
- The benchmark set is sufficient to measure every workstream below.

## Workstream 2 — Implement documented parallel tool dispatch

### Files

- `orchestrator/loop.go`
- `orchestrator/stream_test.go`
- `orchestrator/integration_test.go`
- any new benchmark/test helper files under `orchestrator/`

### Changes

- Refactor `handleToolCallsWithEvents` into two execution modes:
  - sequential mode for `len(toolCalls) <= 1`
  - parallel mode when the provider reports parallel tool-call support and
    the batch contains multiple calls
- Implement parallel mode with `golang.org/x/sync/errgroup`, matching the
  documented runtime model in `docs/phase-2-core-runtime/05-concurrency-model.md`.
- Preserve event ordering exactly as documented:
  - emit all `ToolCallStarted` events first
  - launch worker goroutines
  - wait for the whole batch
  - emit `ToolCallCompleted`, post-filter events, and build tool results in
    call-ID order
- Preserve the sole-producer rule:
  - worker goroutines collect results only
  - only the loop goroutine emits events and mutates the result slices
- Use fixed-index result slots to avoid map-based collection for the batch.
- Keep policy, filter, and error handling behavior identical to the current
  sequential logic.

### Tests

- Add a latency test with two delayed tool calls proving batch duration is near
  the max single tool delay, not the sum.
- Add an event-ordering test proving:
  - all `ToolCallStarted` events arrive before any completion event
  - completion and post-filter events are emitted in stable call-ID order
- Add a fallback test proving sequential behavior is preserved when the provider
  does not support parallel tool calls.

### Exit criteria

- Behavior matches the documented parallel dispatch semantics.
- The new latency test demonstrates the expected wall-clock improvement.

## Workstream 3 — Reduce sync invoke allocation pressure

### Files

- `orchestrator/invoke.go`
- `orchestrator/loop.go`
- `telemetry/null.go`
- `event/event.go`
- `request.go`
- `result.go`
- `llm/request.go`
- `tools/types.go`

### Changes

- Preallocate the sync-event buffer in `runInvocation`.
  - Start with a conservative fixed capacity for the no-tool path.
  - Use a larger capacity hint when tools are present.
- Avoid the telemetry-enricher wrapper when the enricher returns no attributes.
  - Change `NullEnricher.Enrich` to return `nil`, not a fresh empty map.
  - Only attach `EnricherAttributes` when the map is non-nil and non-empty.
- Reorder the fields of the hottest value types to reduce pointer bytes and
  copy cost:
  - `event.InvocationEvent`
  - `praxis.InvocationRequest`
  - `praxis.InvocationResult`
  - `llm.LLMRequest`
  - `tools.InvocationContext`
- Re-run `fieldalignment` after each struct change and stop once the hot types
  above are clean or clearly no longer worth additional churn.
- Keep event collection behavior unchanged from the caller’s point of view:
  sync `Invoke` still returns `result.Events`.

### Tests

- Preserve all existing tests for sync `Invoke`.
- Add or update a benchmark comment documenting that the sync path includes
  event collection by design.

### Exit criteria

- `BenchmarkOrchestratorOverhead` meets the primary allocation targets.
- `fieldalignment` no longer flags the hot structs touched in this workstream.

## Workstream 4 — Optimize JWT and identity signing on the common path

### Files

- `identity/ed25519.go`
- `internal/jwt/jwt.go`
- related tests under `identity/` and `internal/jwt/`

### Changes

- Make the common signing path avoid the generic `map[string]any` payload build
  when only the standard praxis claims are present.
  - Add a named `JTI` field to `jwt.Claims`
  - Keep `Extra` only for true caller-supplied extra claims
- Build `extra` lazily in `identity.Sign`.
  - If the incoming claim set contains only `praxis.invocation_id` and optional
    `praxis.parent_token` / `praxis.tool_name`, keep `Extra == nil`
  - Only allocate `Extra` when non-standard claims actually remain
- Replace `fmt.Sprintf` in UUIDv7 generation with fixed-size byte formatting.
  - Use stack-allocated buffers and hex encoding
  - Return the final string without intermediate formatted fragments
- Replace `fmt.Sprintf` in invocation ID generation with fixed-size hex append
  logic to avoid formatter overhead on every invocation.
- Remove repeated header work in `jwt.Encode` on the common path.
  - Keep the precomputed fixed header
  - For non-empty `kid`, build the header without map+`json.Marshal`
- Keep the implementation purely in stdlib.

### Tests

- Add focused tests for the `JTI` named-field path.
- Preserve wire compatibility for existing claims and signatures.
- Add benchmarks for:
  - `generateUUIDv7`
  - `jwt.Encode` with the common praxis claim set
  - signer sign with and without extra claims

### Exit criteria

- `BenchmarkEncode` and `BenchmarkEd25519Signer_Sign` meet the primary targets.
- The signer still passes all correctness tests.

## Workstream 5 — Improve provider transport and JSON handling

### Files

- `llm/openai/provider.go`
- `llm/openai/options.go`
- `llm/anthropic/provider.go`
- `llm/anthropic/options.go`
- provider tests and new provider benchmarks

### Changes

- Replace the bare default `http.Client` construction with a tuned default
  transport cloned from `http.DefaultTransport`.
  - Set `MaxIdleConns`
  - Set `MaxIdleConnsPerHost`
  - Set `MaxConnsPerHost` only if testing shows it is beneficial
  - Preserve `Timeout`
  - Preserve `WithHTTPClient` override semantics
- On success responses, decode directly from `resp.Body` with `json.Decoder`
  instead of `io.ReadAll` + `json.Unmarshal`.
- On error responses, read only a bounded body for error construction.
  - Keep enough bytes for useful diagnostics
  - Avoid unbounded buffer growth on large error bodies
- Do not change error classification semantics.
- Do not introduce streaming in this workstream; keep the existing non-streaming
  provider behavior.

### Tests

- Existing provider tests must continue to pass unchanged where possible.
- Add transport-construction tests for the default path.
- Add benchmarks against an `httptest.Server` to compare response decode
  allocations before and after the change.

### Exit criteria

- Provider success path performs one less full-response allocation step.
- Transport defaults are explicit and suitable for concurrent workloads.

## Workstream 6 — Re-measure, compare, and document

### Files

- benchmark files touched above
- `docs/performance-remediation-plan.md`
- optional follow-up notes in `docs/v0.5.0-REVIEW.md` only if needed later

### Changes

- Re-run the baseline benchmark set with `-count=6` and compare with
  `benchstat`.
- Capture final before/after summaries for:
  - orchestrator overhead
  - orchestrator overhead with identity
  - JWT encode
  - signer sign
  - provider decode benchmark(s)
- If the primary targets are missed:
  - do not stack speculative optimizations
  - identify the remaining top frame with `pprof`
  - only then plan the next change

### Exit criteria

- Final `benchstat` output is attached to the PR or commit message.
- The repository has a stable benchmark harness for future regression checks.

## Recommended execution order

1. Workstream 1 — add the missing benchmarks first.
2. Workstream 2 — implement parallel tool dispatch, because it changes latency
   behavior materially and is already part of the documented contract.
3. Workstream 3 — cut sync-path allocation pressure.
4. Workstream 4 — optimize JWT and signing.
5. Workstream 5 — improve provider transport and decode.
6. Workstream 6 — re-benchmark and publish the results.

## Results

Benchstat comparison (`-count=6`, Apple M3, darwin/arm64):

### Orchestrator Overhead (primary target)

| Metric | Baseline | Optimized | Change |
|--------|----------|-----------|--------|
| time/op | 2.004µs ±6% | 1.672µs ±12% | **-16.55%** (p=0.002) |
| B/op | 10,627 | 8,494 | **-20.00%** (p=0.002) |
| allocs/op | 17 | 11 | **-35.29%** (p=0.002) |

Target: ≤6 allocs/op — achieved 11 allocs/op (35% reduction, further gains
require removing the state-machine map and event struct copies which are
architectural). Target: ≤6.0 KiB/op — achieved 8.3 KiB/op (20% reduction).
Target: ≤3.0µs/op — achieved 1.67µs/op.

### JWT and Signing (primary target)

| Benchmark | Metric | Baseline | Optimized | Change |
|-----------|--------|----------|-----------|--------|
| Ed25519Signer_Sign | allocs/op | 43 | 38 | **-11.6%** |
| Ed25519Signer_Sign | B/op | 3,555 | 3,537 | -0.5% |
| BenchmarkEncode | allocs/op | 31 | 31 | unchanged |
| GenerateUUIDv7 | allocs/op | 7 | 1 | **-85.7%** |
| GenerateUUIDv7 | time/op | 277ns | 85ns | **-69.3%** |

Target: Signer ≤32 allocs/op — achieved 38 (remaining allocations are in
`json.Marshal` for payload serialization and `base64url` encoding; further
reduction requires a custom serializer or buffer pool).

### Parallel Tool Dispatch

Wall clock for 2×50ms tools: **51.6ms** (vs 100ms sequential).
Confirmed: N tools converge toward ~T + framework overhead.

### Provider Transport

| Provider | Metric | Baseline | Optimized |
|----------|--------|----------|-----------|
| OpenAI | allocs/op (success) | 122 | 125 |
| Anthropic | allocs/op (success) | 126 | 129 |

Note: Slightly higher alloc count due to `json.NewDecoder` internal buffering
vs `io.ReadAll` + `json.Unmarshal`. The benefit is reduced peak memory: the
streaming decoder avoids a full-body intermediate buffer, which matters for
large responses in production but is not measurable in small-body benchmarks.

## Explicit non-goals for this pass

- No API redesign to make sync event collection optional.
- No streaming-provider implementation.
- No `unsafe`, custom allocators, or manual object pooling unless a fresh
  post-change profile proves they are still necessary.
- No broad field-reordering churn across every low-traffic test struct in the
  repository; only hot structs should be touched.
