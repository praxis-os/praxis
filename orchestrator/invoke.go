// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"

	"github.com/praxis-os/praxis/errors"
	"github.com/praxis-os/praxis/invocation"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/state"
)

// runInvocation is the v0.1.0 invocation loop driver.
//
// It drives the state machine through the happy path and tool-use loop.
// The first LLM call follows:
//
//	Created → Initializing → PreHook → LLMCall → [call] → ToolDecision
//
// If the response requires tool use, the loop continues as:
//
//	ToolDecision → ToolCall → PostToolFilter → LLMContinuation → [call] → ToolDecision
//
// On EndTurn/MaxTokens:
//
//	ToolDecision → PostHook → Completed
//
// For v0.1.0, PreHook, PostHook, PostToolFilter, and LLMContinuation are
// structural no-ops (no external hooks or filters are invoked). Tool calls
// return stub error results because no tool invoker is configured.
func runInvocation(
	ctx context.Context,
	provider llm.Provider,
	model string,
	maxIterations int,
	req invocation.InvocationRequest,
) (invocation.InvocationResult, error) {
	machine := state.NewMachine()
	classifier := errors.NewDefaultClassifier()

	// Step 1: Created → Initializing
	if err := machine.Transition(state.Initializing); err != nil {
		return failResult(machine, errors.NewSystemError("transition to Initializing failed", err))
	}

	// Step 2: Initializing → PreHook (no-op for v0.1.0)
	if err := machine.Transition(state.PreHook); err != nil {
		return failResult(machine, errors.NewSystemError("transition to PreHook failed", err))
	}

	// Conversation history: start from the request messages, then grow as
	// assistant messages and tool results are appended each iteration.
	messages := make([]llm.Message, len(req.Messages))
	copy(messages, req.Messages)

	var usage invocation.TokenUsage
	iterations := 0
	firstCall := true

	// Main LLM call loop.
	for iterations < maxIterations {
		if firstCall {
			if err := machine.Transition(state.LLMCall); err != nil {
				return failResult(machine, errors.NewSystemError("transition to LLMCall failed", err))
			}
			firstCall = false
		}

		// Check context cancellation before making the LLM call.
		if err := ctx.Err(); err != nil {
			typed := classifier.Classify(err)
			_ = machine.Transition(state.Cancelled)
			return invocation.InvocationResult{
				FinalState: machine.State(),
				Iterations: iterations,
				TokenUsage: usage,
				Error:      typed,
			}, typed
		}

		// Build and dispatch the LLM request.
		llmReq := llm.LLMRequest{
			Messages: messages,
			Model:    model,
			Tools:    req.Tools,
		}

		resp, providerErr := provider.Complete(ctx, llmReq)
		if providerErr != nil {
			if ctx.Err() != nil {
				typed := classifier.Classify(ctx.Err())
				_ = machine.Transition(state.Cancelled)
				return invocation.InvocationResult{
					FinalState: machine.State(),
					Iterations: iterations,
					TokenUsage: usage,
					Error:      typed,
				}, typed
			}
			typed := classifier.Classify(providerErr)
			_ = machine.Transition(state.Failed)
			return invocation.InvocationResult{
				FinalState: machine.State(),
				Iterations: iterations,
				TokenUsage: usage,
				Error:      typed,
			}, typed
		}

		// Accumulate token usage.
		usage.InputTokens += resp.Usage.InputTokens
		usage.OutputTokens += resp.Usage.OutputTokens
		usage.TotalTokens += resp.Usage.InputTokens + resp.Usage.OutputTokens

		iterations++

		// Transition to ToolDecision (legal from both LLMCall and LLMContinuation).
		if err := machine.Transition(state.ToolDecision); err != nil {
			return failResult(machine, errors.NewSystemError("transition to ToolDecision failed", err))
		}

		// Append the assistant's response to the conversation.
		messages = append(messages, resp.Message)

		// Inspect stop reason.
		switch resp.StopReason {
		case llm.StopReasonEndTurn, llm.StopReasonMaxTokens, llm.StopReasonStopSequence:
			if err := machine.Transition(state.PostHook); err != nil {
				return failResult(machine, errors.NewSystemError("transition to PostHook failed", err))
			}
			if err := machine.Transition(state.Completed); err != nil {
				return failResult(machine, errors.NewSystemError("transition to Completed failed", err))
			}
			return invocation.InvocationResult{
				Response:   resp,
				FinalState: state.Completed,
				Iterations: iterations,
				TokenUsage: usage,
			}, nil

		case llm.StopReasonToolUse:
			toolResultMsg, err := handleToolCalls(resp.Message)
			if err != nil {
				_ = machine.Transition(state.Failed)
				return invocation.InvocationResult{
					FinalState: machine.State(),
					Iterations: iterations,
					TokenUsage: usage,
					Error:      err,
				}, err
			}

			if tErr := machine.Transition(state.ToolCall); tErr != nil {
				return failResult(machine, errors.NewSystemError("transition to ToolCall failed", tErr))
			}

			messages = append(messages, toolResultMsg)

			if tErr := machine.Transition(state.PostToolFilter); tErr != nil {
				return failResult(machine, errors.NewSystemError("transition to PostToolFilter failed", tErr))
			}

			if tErr := machine.Transition(state.LLMContinuation); tErr != nil {
				return failResult(machine, errors.NewSystemError("transition to LLMContinuation failed", tErr))
			}

			continue

		default:
			if err := machine.Transition(state.PostHook); err != nil {
				return failResult(machine, errors.NewSystemError("transition to PostHook failed", err))
			}
			if err := machine.Transition(state.Completed); err != nil {
				return failResult(machine, errors.NewSystemError("transition to Completed failed", err))
			}
			return invocation.InvocationResult{
				Response:   resp,
				FinalState: state.Completed,
				Iterations: iterations,
				TokenUsage: usage,
			}, nil
		}
	}

	// Max iterations exhausted.
	sysErr := errors.NewSystemError(
		fmt.Sprintf("max iterations (%d) exceeded", maxIterations),
		nil,
	)
	_ = machine.Transition(state.Failed)
	return invocation.InvocationResult{
		FinalState: machine.State(),
		Iterations: iterations,
		TokenUsage: usage,
		Error:      sysErr,
	}, sysErr
}

// handleToolCalls extracts tool calls from the assistant message and returns
// a user message containing stub tool results.
func handleToolCalls(msg llm.Message) (llm.Message, error) {
	var resultParts []llm.MessagePart

	for _, part := range msg.Parts {
		if part.Type != llm.PartTypeToolCall || part.ToolCall == nil {
			continue
		}
		tc := part.ToolCall
		resultParts = append(resultParts, llm.ToolResultPart(&llm.LLMToolResult{
			CallID:  tc.CallID,
			Content: "no tool invoker configured",
			IsError: true,
		}))
	}

	if len(resultParts) == 0 {
		return llm.Message{}, errors.NewSystemError(
			"StopReasonToolUse with no tool call parts in assistant message", nil,
		)
	}

	return llm.Message{
		Role:  llm.RoleUser,
		Parts: resultParts,
	}, nil
}

// failResult transitions the machine to Failed (if not already terminal) and
// returns a failed InvocationResult.
func failResult(machine *state.Machine, err error) (invocation.InvocationResult, error) {
	if !machine.State().IsTerminal() {
		_ = machine.Transition(state.Failed)
	}
	return invocation.InvocationResult{
		FinalState: machine.State(),
		Error:      err,
	}, err
}
