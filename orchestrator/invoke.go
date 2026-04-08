// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"

	"github.com/praxis-os/praxis"
	"github.com/praxis-os/praxis/errors"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/state"
	"github.com/praxis-os/praxis/tools"
)

// runInvocation is the v0.3.0 invocation loop driver.
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
func runInvocation(
	ctx context.Context,
	o *Orchestrator,
	model string,
	maxTurns int,
	req praxis.InvocationRequest,
) (*praxis.InvocationResult, error) {
	machine := state.NewMachine()

	// Step 1: Created → Initializing
	if err := machine.Transition(state.Initializing); err != nil {
		return failResult(machine, errors.NewSystemError("transition to Initializing failed", err))
	}

	// Step 2: Initializing → PreHook (no-op for v0.3.0 Wave 1)
	if err := machine.Transition(state.PreHook); err != nil {
		return failResult(machine, errors.NewSystemError("transition to PreHook failed", err))
	}

	// Conversation history: start from the request messages, then grow as
	// assistant messages and tool results are appended each iteration.
	messages := make([]llm.Message, len(req.Messages))
	copy(messages, req.Messages)

	iterations := 0
	firstCall := true

	// Main LLM call loop.
	for iterations < maxTurns {
		if firstCall {
			if err := machine.Transition(state.LLMCall); err != nil {
				return failResult(machine, errors.NewSystemError("transition to LLMCall failed", err))
			}
			firstCall = false
		}

		// Check context cancellation before making the LLM call.
		if result, cancelled, err := checkCancellation(ctx, o, machine); cancelled {
			return result, err
		}

		// Build and dispatch the LLM request.
		llmReq := llm.LLMRequest{
			Messages:     messages,
			Model:        model,
			Tools:        req.Tools,
			SystemPrompt: req.SystemPrompt,
		}

		resp, providerErr := o.provider.Complete(ctx, llmReq)
		if providerErr != nil {
			return handleProviderError(ctx, o, machine, providerErr)
		}

		iterations++

		// Transition to ToolDecision (legal from both LLMCall and LLMContinuation).
		if err := machine.Transition(state.ToolDecision); err != nil {
			return failResult(machine, errors.NewSystemError("transition to ToolDecision failed", err))
		}

		// Append the assistant's response to the conversation.
		messages = append(messages, resp.Message)

		// Inspect stop reason and act accordingly.
		done, newMessages, result, err := handleStopReason(ctx, o, machine, resp, messages, iterations)
		if done {
			return result, err
		}
		messages = newMessages
	}

	// Max turns exhausted.
	sysErr := errors.NewSystemError(
		fmt.Sprintf("max turns (%d) exceeded", maxTurns),
		nil,
	)
	_ = machine.Transition(state.Failed)
	return &praxis.InvocationResult{
		FinalState: machine.State(),
	}, sysErr
}

// checkCancellation checks whether ctx is already cancelled and, if so,
// transitions the machine to Cancelled and returns a populated result.
func checkCancellation(
	ctx context.Context,
	o *Orchestrator,
	machine *state.Machine,
) (*praxis.InvocationResult, bool, error) {
	if ctx.Err() == nil {
		return nil, false, nil
	}
	typed := o.classifier.Classify(ctx.Err())
	_ = machine.Transition(state.Cancelled)
	return &praxis.InvocationResult{
		FinalState: machine.State(),
	}, true, typed
}

// handleProviderError maps a provider error to the appropriate terminal state.
func handleProviderError(
	ctx context.Context,
	o *Orchestrator,
	machine *state.Machine,
	providerErr error,
) (*praxis.InvocationResult, error) {
	if ctx.Err() != nil {
		typed := o.classifier.Classify(ctx.Err())
		_ = machine.Transition(state.Cancelled)
		return &praxis.InvocationResult{
			FinalState: machine.State(),
		}, typed
	}
	typed := o.classifier.Classify(providerErr)
	_ = machine.Transition(state.Failed)
	return &praxis.InvocationResult{
		FinalState: machine.State(),
	}, typed
}

// handleStopReason processes the LLM response's stop reason.
func handleStopReason(
	ctx context.Context,
	o *Orchestrator,
	machine *state.Machine,
	resp llm.LLMResponse,
	messages []llm.Message,
	iterations int,
) (done bool, newMessages []llm.Message, result *praxis.InvocationResult, err error) {
	switch resp.StopReason {
	case llm.StopReasonEndTurn, llm.StopReasonMaxTokens, llm.StopReasonStopSequence:
		result, err = completeInvocation(machine, resp)
		return true, nil, result, err

	case llm.StopReasonToolUse:
		toolResultMsg, toolErr := handleToolCalls(ctx, o, resp.Message)
		if toolErr != nil {
			_ = machine.Transition(state.Failed)
			return true, nil, &praxis.InvocationResult{
				FinalState: machine.State(),
			}, toolErr
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
		return false, messages, nil, nil

	default:
		result, err = completeInvocation(machine, resp)
		return true, nil, result, err
	}
}

// completeInvocation transitions the machine through PostHook → Completed.
func completeInvocation(
	machine *state.Machine,
	resp llm.LLMResponse,
) (*praxis.InvocationResult, error) {
	if err := machine.Transition(state.PostHook); err != nil {
		return failResult(machine, errors.NewSystemError("transition to PostHook failed", err))
	}
	if err := machine.Transition(state.Completed); err != nil {
		return failResult(machine, errors.NewSystemError("transition to Completed failed", err))
	}
	msg := resp.Message
	return &praxis.InvocationResult{
		Response:   &msg,
		FinalState: state.Completed,
	}, nil
}

// handleToolCalls extracts tool calls from the assistant message, dispatches
// each via the orchestrator's tool invoker, and returns a user message
// containing the collected tool results.
func handleToolCalls(ctx context.Context, o *Orchestrator, msg llm.Message) (llm.Message, error) {
	var resultParts []llm.MessagePart

	// Build a minimal InvocationContext for tool dispatch.
	// Full context propagation (budget, span, identity) comes in later waves.
	ictx := tools.InvocationContext{}

	for _, part := range msg.Parts {
		if part.Type != llm.PartTypeToolCall || part.ToolCall == nil {
			continue
		}
		tc := part.ToolCall

		call := tools.ToolCall{
			CallID:        tc.CallID,
			Name:          tc.Name,
			ArgumentsJSON: tc.ArgumentsJSON,
		}

		result, err := o.toolInvoker.Invoke(ctx, ictx, call)
		if err != nil {
			return llm.Message{}, errors.NewSystemError(
				fmt.Sprintf("tool invoker failure for call %q", tc.CallID), err,
			)
		}

		resultParts = append(resultParts, llm.ToolResultPart(&llm.LLMToolResult{
			CallID:  result.CallID,
			Content: result.Content,
			IsError: result.Status == tools.ToolStatusError || result.Status == tools.ToolStatusNotImplemented,
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
func failResult(machine *state.Machine, err error) (*praxis.InvocationResult, error) {
	if !machine.State().IsTerminal() {
		_ = machine.Transition(state.Failed)
	}
	return &praxis.InvocationResult{
		FinalState: machine.State(),
	}, err
}
