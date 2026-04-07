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
	o *Orchestrator,
	model string,
	maxIterations int,
	req invocation.InvocationRequest,
) (invocation.InvocationResult, error) {
	machine := state.NewMachine()

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
		if result, cancelled, err := checkCancellation(ctx, o, machine, iterations, usage); cancelled {
			return result, err
		}

		// Build and dispatch the LLM request.
		llmReq := llm.LLMRequest{
			Messages: messages,
			Model:    model,
			Tools:    req.Tools,
		}

		resp, providerErr := o.provider.Complete(ctx, llmReq)
		if providerErr != nil {
			return handleProviderError(ctx, o, machine, iterations, usage, providerErr)
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

		// Inspect stop reason and act accordingly.
		done, newMessages, result, err := handleStopReason(ctx, o, machine, resp, messages, iterations, usage)
		if done {
			return result, err
		}
		messages = newMessages
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

// checkCancellation checks whether ctx is already cancelled and, if so,
// transitions the machine to Cancelled and returns a populated result.
// The cancelled return value reports whether the caller should return immediately.
func checkCancellation(
	ctx context.Context,
	o *Orchestrator,
	machine *state.Machine,
	iterations int,
	usage invocation.TokenUsage,
) (invocation.InvocationResult, bool, error) {
	if ctx.Err() == nil {
		return invocation.InvocationResult{}, false, nil
	}
	typed := o.classifier.Classify(ctx.Err())
	_ = machine.Transition(state.Cancelled)
	return invocation.InvocationResult{
		FinalState: machine.State(),
		Iterations: iterations,
		TokenUsage: usage,
		Error:      typed,
	}, true, typed
}

// handleProviderError maps a provider error to the appropriate terminal state
// (Cancelled when the context is done, Failed otherwise) and returns the result.
func handleProviderError(
	ctx context.Context,
	o *Orchestrator,
	machine *state.Machine,
	iterations int,
	usage invocation.TokenUsage,
	providerErr error,
) (invocation.InvocationResult, error) {
	if ctx.Err() != nil {
		typed := o.classifier.Classify(ctx.Err())
		_ = machine.Transition(state.Cancelled)
		return invocation.InvocationResult{
			FinalState: machine.State(),
			Iterations: iterations,
			TokenUsage: usage,
			Error:      typed,
		}, typed
	}
	typed := o.classifier.Classify(providerErr)
	_ = machine.Transition(state.Failed)
	return invocation.InvocationResult{
		FinalState: machine.State(),
		Iterations: iterations,
		TokenUsage: usage,
		Error:      typed,
	}, typed
}

// handleStopReason processes the LLM response's stop reason.
// It returns done=true when the loop should exit, together with the final
// result and error. When done=false, newMessages contains the updated
// conversation to continue with.
func handleStopReason(
	ctx context.Context,
	o *Orchestrator,
	machine *state.Machine,
	resp llm.LLMResponse,
	messages []llm.Message,
	iterations int,
	usage invocation.TokenUsage,
) (done bool, newMessages []llm.Message, result invocation.InvocationResult, err error) {
	switch resp.StopReason {
	case llm.StopReasonEndTurn, llm.StopReasonMaxTokens, llm.StopReasonStopSequence:
		result, err = completeInvocation(machine, resp, iterations, usage)
		return true, nil, result, err

	case llm.StopReasonToolUse:
		toolResultMsg, toolErr := handleToolCalls(ctx, o, resp.Message)
		if toolErr != nil {
			_ = machine.Transition(state.Failed)
			r := invocation.InvocationResult{
				FinalState: machine.State(),
				Iterations: iterations,
				TokenUsage: usage,
				Error:      toolErr,
			}
			return true, nil, r, toolErr
		}

		if tErr := machine.Transition(state.ToolCall); tErr != nil {
			r, e := failResult(machine, errors.NewSystemError("transition to ToolCall failed", tErr))
			return true, nil, r, e
		}
		messages = append(messages, toolResultMsg)
		if tErr := machine.Transition(state.PostToolFilter); tErr != nil {
			r, e := failResult(machine, errors.NewSystemError("transition to PostToolFilter failed", tErr))
			return true, nil, r, e
		}
		if tErr := machine.Transition(state.LLMContinuation); tErr != nil {
			r, e := failResult(machine, errors.NewSystemError("transition to LLMContinuation failed", tErr))
			return true, nil, r, e
		}
		return false, messages, invocation.InvocationResult{}, nil

	default:
		result, err = completeInvocation(machine, resp, iterations, usage)
		return true, nil, result, err
	}
}

// completeInvocation transitions the machine through PostHook → Completed and
// returns the final successful result.
func completeInvocation(
	machine *state.Machine,
	resp llm.LLMResponse,
	iterations int,
	usage invocation.TokenUsage,
) (invocation.InvocationResult, error) {
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

// handleToolCalls extracts tool calls from the assistant message, dispatches
// each via the orchestrator's tool invoker, and returns a user message
// containing the collected tool results.
func handleToolCalls(ctx context.Context, o *Orchestrator, msg llm.Message) (llm.Message, error) {
	var resultParts []llm.MessagePart

	for _, part := range msg.Parts {
		if part.Type != llm.PartTypeToolCall || part.ToolCall == nil {
			continue
		}
		tc := part.ToolCall

		result, err := o.toolInvoker.Invoke(ctx, *tc)
		if err != nil {
			// Framework-level invoker failure — treat as system error.
			return llm.Message{}, errors.NewSystemError(
				fmt.Sprintf("tool invoker failure for call %q", tc.CallID), err,
			)
		}

		resultParts = append(resultParts, llm.ToolResultPart(&llm.LLMToolResult{
			CallID:  result.CallID,
			Content: result.Content,
			IsError: result.IsError,
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
