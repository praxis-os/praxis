// SPDX-License-Identifier: Apache-2.0

// Package invocation defines the request and result types used by the
// praxis orchestrator's Invoke method.
//
// [InvocationRequest] carries everything the orchestrator needs to execute a
// single agent invocation: the conversation history, target model, optional
// tool definitions, iteration cap, and caller-supplied metadata.
//
// [InvocationResult] is the value returned after the invocation reaches a
// terminal state. It aggregates the final LLM response, the terminal state
// reached, total iteration count, and cumulative token usage across all
// LLM round-trips.
//
// These types are plain data structs with no behaviour. The orchestrator
// constructs and consumes them; callers create [InvocationRequest] values
// and read [InvocationResult] values.
package invocation
