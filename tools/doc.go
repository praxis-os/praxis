// SPDX-License-Identifier: Apache-2.0

// Package tools defines the Invoker interface and its null implementation.
//
// An Invoker dispatches tool calls requested by the LLM. The orchestrator
// routes each [llm.LLMToolCall] through the configured Invoker and returns
// the resulting [llm.LLMToolResult] to the LLM in the next turn.
//
// [NullInvoker] is the default when no tool invoker is configured. It
// returns a safe error result for every call rather than panicking or
// blocking indefinitely.
//
// Stability: frozen-v1.0.
package tools
