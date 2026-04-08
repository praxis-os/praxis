// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/praxis-os/praxis"
	"github.com/praxis-os/praxis/errors"
	"github.com/praxis-os/praxis/event"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/state"
	"github.com/praxis-os/praxis/tools"
)

// eventSink abstracts where lifecycle events are delivered.
// On the sync path, events are collected into a slice.
// On the stream path, events are sent to a channel.
type eventSink func(ctx context.Context, e event.InvocationEvent)

// runLoop is the shared state-machine driver for both Invoke and InvokeStream.
//
// It drives the state machine through the happy path and tool-use loop,
// emitting lifecycle events via the provided sink at each transition.
func (o *Orchestrator) runLoop(
	ctx context.Context,
	req praxis.InvocationRequest,
	model string,
	maxTurns int,
	sink eventSink,
) *praxis.InvocationResult {
	machine := state.NewMachine()
	now := time.Now

	emit := func(t event.EventType, s state.State) {
		sink(ctx, event.InvocationEvent{
			Type:  t,
			State: s,
			At:    now(),
		})
	}

	emitTerminal := func(t event.EventType, s state.State, err error) {
		sink(ctx, event.InvocationEvent{
			Type:  t,
			State: s,
			At:    now(),
			Err:   err,
		})
	}

	// Step 1: Created → Initializing
	if err := machine.Transition(state.Initializing); err != nil {
		return o.failLoop(ctx, machine, sink, errors.NewSystemError("transition to Initializing failed", err))
	}
	emit(event.EventTypeInvocationStarted, state.Initializing)

	// Step 2: Initializing → PreHook
	if err := machine.Transition(state.PreHook); err != nil {
		return o.failLoop(ctx, machine, sink, errors.NewSystemError("transition to PreHook failed", err))
	}
	emit(event.EventTypeInitialized, state.PreHook)
	emit(event.EventTypePreHookStarted, state.PreHook)

	// Conversation history.
	messages := make([]llm.Message, len(req.Messages))
	copy(messages, req.Messages)

	iterations := 0
	firstCall := true

	for iterations < maxTurns {
		if firstCall {
			if err := machine.Transition(state.LLMCall); err != nil {
				return o.failLoop(ctx, machine, sink, errors.NewSystemError("transition to LLMCall failed", err))
			}
			emit(event.EventTypePreHookCompleted, state.LLMCall)
			firstCall = false
		}

		emit(event.EventTypeLLMCallStarted, machine.State())

		// Check context cancellation before LLM call.
		if ctx.Err() != nil {
			typed := o.classifier.Classify(ctx.Err())
			_ = machine.Transition(state.Cancelled)
			emitTerminal(event.EventTypeInvocationCancelled, state.Cancelled, typed)
			return &praxis.InvocationResult{FinalState: state.Cancelled}
		}

		// Build and dispatch LLM request.
		llmReq := llm.LLMRequest{
			Messages:     messages,
			Model:        model,
			Tools:        req.Tools,
			SystemPrompt: req.SystemPrompt,
		}

		resp, providerErr := o.provider.Complete(ctx, llmReq)
		if providerErr != nil {
			if ctx.Err() != nil {
				typed := o.classifier.Classify(ctx.Err())
				_ = machine.Transition(state.Cancelled)
				emitTerminal(event.EventTypeInvocationCancelled, state.Cancelled, typed)
				return &praxis.InvocationResult{FinalState: state.Cancelled}
			}
			typed := o.classifier.Classify(providerErr)
			_ = machine.Transition(state.Failed)
			emitTerminal(event.EventTypeInvocationFailed, state.Failed, typed)
			return &praxis.InvocationResult{FinalState: state.Failed}
		}

		emit(event.EventTypeLLMCallCompleted, machine.State())
		iterations++

		// Transition to ToolDecision.
		if err := machine.Transition(state.ToolDecision); err != nil {
			return o.failLoop(ctx, machine, sink, errors.NewSystemError("transition to ToolDecision failed", err))
		}
		emit(event.EventTypeToolDecisionStarted, state.ToolDecision)

		messages = append(messages, resp.Message)

		if resp.StopReason == llm.StopReasonToolUse {
			// Tool cycle: dispatch calls, filter, continue.
			toolResultMsg, toolErr := o.handleToolCallsWithEvents(ctx, resp.Message, machine, sink)
			if toolErr != nil {
				_ = machine.Transition(state.Failed)
				emitTerminal(event.EventTypeInvocationFailed, state.Failed, toolErr)
				return &praxis.InvocationResult{FinalState: state.Failed}
			}

			messages = append(messages, toolResultMsg)

			// PostToolFilter → LLMContinuation
			if err := machine.Transition(state.LLMContinuation); err != nil {
				return o.failLoop(ctx, machine, sink, errors.NewSystemError("transition to LLMContinuation failed", err))
			}
			emit(event.EventTypeLLMContinuationStarted, state.LLMContinuation)
			continue
		}

		// EndTurn, MaxTokens, StopSequence, or unknown → complete.
		return o.completeLoop(ctx, machine, sink, emit, emitTerminal, resp.Message)
	}

	// Max turns exhausted.
	sysErr := errors.NewSystemError(
		fmt.Sprintf("max turns (%d) exceeded", maxTurns),
		nil,
	)
	_ = machine.Transition(state.Failed)
	emitTerminal(event.EventTypeInvocationFailed, state.Failed, sysErr)
	return &praxis.InvocationResult{FinalState: state.Failed}
}

// completeLoop transitions through PostHook → Completed and emits events.
func (o *Orchestrator) completeLoop(
	ctx context.Context,
	machine *state.Machine,
	sink eventSink,
	emit func(event.EventType, state.State),
	emitTerminal func(event.EventType, state.State, error),
	msg llm.Message,
) *praxis.InvocationResult {
	if err := machine.Transition(state.PostHook); err != nil {
		return o.failLoop(ctx, machine, sink, errors.NewSystemError("transition to PostHook failed", err))
	}
	emit(event.EventTypePostHookStarted, state.PostHook)

	if err := machine.Transition(state.Completed); err != nil {
		return o.failLoop(ctx, machine, sink, errors.NewSystemError("transition to Completed failed", err))
	}
	emit(event.EventTypePostHookCompleted, state.Completed)

	emitTerminal(event.EventTypeInvocationCompleted, state.Completed, nil)
	return &praxis.InvocationResult{
		Response:   &msg,
		FinalState: state.Completed,
	}
}

// handleToolCallsWithEvents dispatches tool calls with event emission.
func (o *Orchestrator) handleToolCallsWithEvents(
	ctx context.Context,
	msg llm.Message,
	machine *state.Machine,
	sink eventSink,
) (llm.Message, error) {
	var resultParts []llm.MessagePart
	ictx := tools.InvocationContext{}

	// Collect tool calls.
	var toolCalls []tools.ToolCall
	for _, part := range msg.Parts {
		if part.Type != llm.PartTypeToolCall || part.ToolCall == nil {
			continue
		}
		tc := part.ToolCall
		toolCalls = append(toolCalls, tools.ToolCall{
			CallID:        tc.CallID,
			Name:          tc.Name,
			ArgumentsJSON: tc.ArgumentsJSON,
		})
	}

	if len(toolCalls) == 0 {
		return llm.Message{}, errors.NewSystemError(
			"StopReasonToolUse with no tool call parts in assistant message", nil,
		)
	}

	// Transition to ToolCall state.
	if err := machine.Transition(state.ToolCall); err != nil {
		return llm.Message{}, errors.NewSystemError("transition to ToolCall failed", err)
	}

	for _, call := range toolCalls {
		sink(ctx, event.InvocationEvent{
			Type:       event.EventTypeToolCallStarted,
			State:      state.ToolCall,
			At:         time.Now(),
			ToolCallID: call.CallID,
			ToolName:   call.Name,
		})

		result, err := o.toolInvoker.Invoke(ctx, ictx, call)
		if err != nil {
			return llm.Message{}, errors.NewSystemError(
				fmt.Sprintf("tool invoker failure for call %q", call.CallID), err,
			)
		}

		sink(ctx, event.InvocationEvent{
			Type:       event.EventTypeToolCallCompleted,
			State:      state.ToolCall,
			At:         time.Now(),
			ToolCallID: call.CallID,
			ToolName:   call.Name,
		})

		resultParts = append(resultParts, llm.ToolResultPart(&llm.LLMToolResult{
			CallID:  result.CallID,
			Content: result.Content,
			IsError: result.Status == tools.ToolStatusError || result.Status == tools.ToolStatusNotImplemented,
		}))
	}

	// Transition to PostToolFilter.
	if err := machine.Transition(state.PostToolFilter); err != nil {
		return llm.Message{}, errors.NewSystemError("transition to PostToolFilter failed", err)
	}
	sink(ctx, event.InvocationEvent{
		Type:  event.EventTypePostToolFilterStarted,
		State: state.PostToolFilter,
		At:    time.Now(),
	})
	sink(ctx, event.InvocationEvent{
		Type:  event.EventTypePostToolFilterCompleted,
		State: state.PostToolFilter,
		At:    time.Now(),
	})

	return llm.Message{
		Role:  llm.RoleUser,
		Parts: resultParts,
	}, nil
}

// failLoop transitions to Failed and emits the terminal event.
func (o *Orchestrator) failLoop(
	ctx context.Context,
	machine *state.Machine,
	sink eventSink,
	err error,
) *praxis.InvocationResult {
	if !machine.State().IsTerminal() {
		_ = machine.Transition(state.Failed)
	}
	sink(ctx, event.InvocationEvent{
		Type:  event.EventTypeInvocationFailed,
		State: machine.State(),
		At:    time.Now(),
		Err:   err,
	})
	return &praxis.InvocationResult{FinalState: machine.State()}
}
