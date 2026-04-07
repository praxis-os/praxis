// SPDX-License-Identifier: Apache-2.0

// Package hooks defines the policy hook and filter interfaces with their
// null/pass-through implementations.
//
// Three hook points are provided:
//
//   - [PolicyHook] — evaluated at defined lifecycle phases before any
//     potentially side-effectful operation. A Deny decision halts the
//     invocation.
//   - [PreLLMFilter] — inspects or mutates an [llm.LLMRequest] before it
//     is sent to the provider.
//   - [PostToolFilter] — inspects or mutates an [llm.LLMToolResult] before
//     it is returned to the LLM in the next turn.
//
// Null/pass-through implementations are provided for all three:
// [AllowAllPolicyHook], [NoOpPreLLMFilter], and [NoOpPostToolFilter].
//
// Stability: frozen-v1.0.
package hooks
