// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"time"

	"github.com/praxis-os/praxis"
	"github.com/praxis-os/praxis/budget"
	"github.com/praxis-os/praxis/errors"
	"github.com/praxis-os/praxis/event"
	"github.com/praxis-os/praxis/hooks"
	"github.com/praxis-os/praxis/internal/jwt"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/state"
	"github.com/praxis-os/praxis/telemetry"
	"github.com/praxis-os/praxis/tools"
)

// gracePeriod is the soft-cancel grace window (D21).
const gracePeriod = 500 * time.Millisecond

// generateInvocationID returns a random hex invocation identifier.
func generateInvocationID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("inv-%x", b[:])
}

// eventSink abstracts where lifecycle events are delivered.
type eventSink func(ctx context.Context, e event.InvocationEvent)

// runLoop is the shared state-machine driver for both Invoke and InvokeStream.
func (o *Orchestrator) runLoop(
	ctx context.Context,
	req praxis.InvocationRequest,
	model string,
	maxTurns int,
	sink eventSink,
) (result *praxis.InvocationResult) {
	machine := state.NewMachine()
	now := time.Now

	emit := func(t event.EventType, s state.State) {
		sink(ctx, event.InvocationEvent{Type: t, State: s, At: now()})
	}
	emitTerminal := func(t event.EventType, s state.State, err error) {
		sink(ctx, event.InvocationEvent{Type: t, State: s, At: now(), Err: err})
	}

	// Start budget wall clock for concrete BudgetGuard.
	if bg, ok := o.budgetGuard.(*budget.BudgetGuard); ok {
		bg.Start(time.Now())
	}

	// Generate invocation ID and track signed identity for result enrichment.
	// The deferred func sets these on every result regardless of exit path.
	invocationID := generateInvocationID()
	var signedIdentity string
	defer func() {
		if result != nil {
			result.InvocationID = invocationID
			result.SignedIdentity = signedIdentity
		}
	}()

	// Step 1: Created → Initializing
	if err := machine.Transition(state.Initializing); err != nil {
		return o.failLoop(ctx, machine, sink, errors.NewSystemError("transition to Initializing failed", err))
	}
	emit(event.EventTypeInvocationStarted, state.Initializing)

	// Enrich telemetry attributes (D60). Called once at Initializing; the
	// returned map is attached to every subsequent event. The first event
	// (InvocationStarted) has nil EnricherAttributes by design.
	enricherAttrs := o.attributeEnricher.Enrich(ctx)
	originalSink := sink
	sink = func(ctx context.Context, e event.InvocationEvent) {
		e.EnricherAttributes = enricherAttrs
		originalSink(ctx, e)
	}

	// Sign identity token at Initializing (D73). Claims include the
	// invocation ID and, for nested orchestrators, the parent token (D75).
	signClaims := map[string]any{
		jwt.ClaimInvocationID: invocationID,
	}
	if req.ParentToken != "" {
		signClaims[jwt.ClaimParentToken] = req.ParentToken
	}
	signedIdentity, signErr := o.identitySigner.Sign(ctx, signClaims)
	if signErr != nil {
		return o.failLoop(ctx, machine, sink, errors.NewSystemError("identity signing failed", signErr))
	}

	// Step 2: Initializing → PreHook
	if err := machine.Transition(state.PreHook); err != nil {
		return o.failLoop(ctx, machine, sink, errors.NewSystemError("transition to PreHook failed", err))
	}
	emit(event.EventTypeInitialized, state.PreHook)
	emit(event.EventTypePreHookStarted, state.PreHook)

	// Pre-invocation policy hook evaluation.
	preResult, preAuditNote, _ := o.evaluatePolicy(ctx, machine, sink, emit, emitTerminal, hooks.PhasePreInvocation, hooks.PolicyInput{
		Model:        model,
		SystemPrompt: req.SystemPrompt,
		Messages:     req.Messages,
		Metadata:     req.Metadata,
	})
	if preResult != nil {
		return preResult
	}

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
			sink(ctx, event.InvocationEvent{
				Type:      event.EventTypePreHookCompleted,
				State:     state.LLMCall,
				At:        now(),
				AuditNote: preAuditNote,
			})
			firstCall = false
		}

		var result *praxis.InvocationResult
		messages, iterations, result = o.runIteration(ctx, req, model, messages, iterations, machine, sink, emit, emitTerminal, invocationID, signedIdentity)
		if result != nil {
			return result
		}
	}

	// Max turns exhausted.
	sysErr := errors.NewSystemError(fmt.Sprintf("max turns (%d) exceeded", maxTurns), nil)
	_ = machine.Transition(state.Failed)
	emitTerminal(event.EventTypeInvocationFailed, state.Failed, sysErr)
	return &praxis.InvocationResult{FinalState: state.Failed}
}

// runIteration executes a single LLM call iteration within the main loop.
// Returns updated messages, iteration count, and a non-nil result if the loop should terminate.
func (o *Orchestrator) runIteration(
	ctx context.Context,
	req praxis.InvocationRequest,
	model string,
	messages []llm.Message,
	iterations int,
	machine *state.Machine,
	sink eventSink,
	emit func(event.EventType, state.State),
	emitTerminal func(event.EventType, state.State, error),
	invocationID string,
	signedIdentity string,
) ([]llm.Message, int, *praxis.InvocationResult) {
	emit(event.EventTypeLLMCallStarted, machine.State())

	// Check context cancellation before LLM call (D21 §2.3).
	if result := o.checkCancel(ctx, machine, sink, emitTerminal); result != nil {
		return messages, iterations, result
	}

	// Per-turn policy evaluation before each LLM call (PhasePreLLMInput).
	preLLMResult, _, _ := o.evaluatePolicy(ctx, machine, sink, emit, emitTerminal, hooks.PhasePreLLMInput, hooks.PolicyInput{
		InvocationID: invocationID,
		Model:        model,
		SystemPrompt: req.SystemPrompt,
		Messages:     messages,
		Metadata:     req.Metadata,
	})
	if preLLMResult != nil {
		return messages, iterations, preLLMResult
	}

	// Pre-LLM filter chain.
	filtered, filterResult := o.applyPreLLMFilter(ctx, machine, sink, emitTerminal, messages)
	if filterResult != nil {
		return messages, iterations, filterResult
	}
	messages = filtered

	// Build and dispatch LLM request.
	llmReq := llm.LLMRequest{
		Messages:     messages,
		Model:        model,
		Tools:        req.Tools,
		SystemPrompt: req.SystemPrompt,
	}

	resp, providerErr := o.provider.Complete(ctx, llmReq)
	if providerErr != nil {
		return messages, iterations, o.handleProviderError(ctx, machine, sink, emitTerminal, providerErr)
	}

	emit(event.EventTypeLLMCallCompleted, machine.State())
	iterations++

	// Record token usage from LLM response.
	_ = o.budgetGuard.RecordTokens(ctx, resp.Usage.InputTokens, resp.Usage.OutputTokens)

	// Transition to ToolDecision.
	if err := machine.Transition(state.ToolDecision); err != nil {
		return messages, iterations, o.failLoop(ctx, machine, sink, errors.NewSystemError("transition to ToolDecision failed", err))
	}
	emit(event.EventTypeToolDecisionStarted, state.ToolDecision)

	// Budget check at ToolDecision boundary (D21: budget breach > cancel).
	if result := o.checkBudget(ctx, machine, sink); result != nil {
		return messages, iterations, result
	}

	messages = append(messages, resp.Message)

	if resp.StopReason == llm.StopReasonToolUse {
		toolResultMsg, lastToolResult, toolErr := o.handleToolCallsWithEvents(ctx, resp.Message, machine, sink, invocationID, signedIdentity)
		if toolErr != nil {
			_ = machine.Transition(state.Failed)
			emitTerminal(event.EventTypeInvocationFailed, state.Failed, toolErr)
			return messages, iterations, &praxis.InvocationResult{FinalState: state.Failed}
		}

		// Post-tool-output policy evaluation (PhasePostToolOutput).
		postToolResult, _, _ := o.evaluatePolicy(ctx, machine, sink, emit, emitTerminal, hooks.PhasePostToolOutput, hooks.PolicyInput{
			InvocationID: invocationID,
			Model:        model,
			SystemPrompt: req.SystemPrompt,
			Messages:     messages,
			ToolResult:   lastToolResult,
			Metadata:     req.Metadata,
		})
		if postToolResult != nil {
			return messages, iterations, postToolResult
		}

		messages = append(messages, toolResultMsg)

		if err := machine.Transition(state.LLMContinuation); err != nil {
			return messages, iterations, o.failLoop(ctx, machine, sink, errors.NewSystemError("transition to LLMContinuation failed", err))
		}
		emit(event.EventTypeLLMContinuationStarted, state.LLMContinuation)
		return messages, iterations, nil
	}

	// EndTurn, MaxTokens, StopSequence, or unknown → complete.
	return messages, iterations, o.completeLoop(ctx, machine, sink, emit, emitTerminal, req, model, resp.Message)
}

// checkCancel inspects the context for cancellation and returns a terminal
// result if the context is done. Soft cancel (context.Canceled) gets a 500ms
// grace period; hard cancel (DeadlineExceeded) is immediate (D21).
func (o *Orchestrator) checkCancel(
	ctx context.Context,
	machine *state.Machine,
	_ eventSink,
	emitTerminal func(event.EventType, state.State, error),
) *praxis.InvocationResult {
	if ctx.Err() == nil {
		return nil
	}

	cancelErr := ctx.Err()

	// Soft cancel: give in-flight work a grace period.
	if cancelErr == context.Canceled {
		graceCtx, graceCancel := context.WithTimeout(context.WithoutCancel(ctx), gracePeriod)
		defer graceCancel()
		// Wait for grace period or until it expires.
		<-graceCtx.Done()
	}
	// Hard cancel (DeadlineExceeded) or grace period expired: terminate.

	kind := errors.CancellationKindSoft
	if cancelErr == context.DeadlineExceeded {
		kind = errors.CancellationKindHard
	}
	typed := errors.NewCancellationError(kind, cancelErr)
	_ = machine.Transition(state.Cancelled)
	emitTerminal(event.EventTypeInvocationCancelled, state.Cancelled, typed)
	return &praxis.InvocationResult{FinalState: state.Cancelled}
}

// evaluatePolicy runs the PolicyHook at the given phase and handles the
// 5 verdicts. Returns (nil, auditNote, continueLoop) if the invocation should
// proceed. continueLoop is true only when VerdictContinue is returned at
// PhasePostInvocation, signalling the orchestrator should force another LLM turn.
func (o *Orchestrator) evaluatePolicy(
	ctx context.Context,
	machine *state.Machine,
	sink eventSink,
	_ func(event.EventType, state.State),
	emitTerminal func(event.EventType, state.State, error),
	phase hooks.Phase,
	input hooks.PolicyInput,
) (*praxis.InvocationResult, string, bool) {
	var decision hooks.Decision
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic in policy hook at %s: %v", phase, r)
			}
		}()
		decision, err = o.policyHook.Evaluate(ctx, phase, input)
	}()
	if err != nil {
		return o.failLoop(ctx, machine, sink, errors.NewSystemError(
			fmt.Sprintf("policy hook error at %s", phase), err)), "", false
	}

	switch decision.Verdict {
	case hooks.VerdictAllow:
		return nil, "", false

	case hooks.VerdictLog:
		slog.InfoContext(ctx, "policy hook log",
			"phase", string(phase),
			"reason", decision.Reason)
		return nil, decision.Reason, false

	case hooks.VerdictContinue:
		slog.InfoContext(ctx, "policy hook continue",
			"phase", string(phase),
			"reason", decision.Reason)
		return nil, decision.Reason, phase == hooks.PhasePostInvocation

	case hooks.VerdictDeny:
		policyErr := errors.NewPolicyDeniedError(string(phase), decision.Reason)
		_ = machine.Transition(state.Failed)
		emitTerminal(event.EventTypeInvocationFailed, state.Failed, policyErr)
		return &praxis.InvocationResult{FinalState: state.Failed}, "", false

	case hooks.VerdictRequireApproval:
		snapshot := errors.ApprovalSnapshot{
			Messages:         input.Messages,
			Model:            input.Model,
			SystemPrompt:     input.SystemPrompt,
			ApprovalMetadata: decision.Metadata,
			RequestMetadata:  input.Metadata,
		}
		_ = machine.Transition(state.ApprovalRequired)
		sink(ctx, event.InvocationEvent{
			Type:             event.EventTypeApprovalRequired,
			State:            state.ApprovalRequired,
			At:               time.Now(),
			Err:              errors.NewApprovalRequiredError(snapshot),
			ApprovalSnapshot: &snapshot,
		})
		return &praxis.InvocationResult{FinalState: state.ApprovalRequired}, "", false

	default:
		return o.failLoop(ctx, machine, sink, errors.NewSystemError(
			fmt.Sprintf("unknown policy verdict %q at %s", decision.Verdict, phase), nil)), "", false
	}
}

// applyPreLLMFilter runs the PreLLMFilter chain and returns filtered messages.
// Returns a non-nil result if the filter blocked the invocation.
func (o *Orchestrator) applyPreLLMFilter(
	ctx context.Context,
	machine *state.Machine,
	sink eventSink,
	emitTerminal func(event.EventType, state.State, error),
	messages []llm.Message,
) ([]llm.Message, *praxis.InvocationResult) {
	var filtered []llm.Message
	var decisions []hooks.FilterDecision
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic in pre-LLM filter: %v", r)
				o.logger.LogAttrs(ctx, slog.LevelWarn, "pre-LLM filter panic recovered",
					slog.Any("panic", r),
				)
			}
		}()
		filtered, decisions, err = o.preLLMFilter.Filter(ctx, messages)
	}()
	if err != nil {
		o.logger.LogAttrs(ctx, slog.LevelWarn, "pre-LLM filter error (trust-boundary-internal)",
			slog.String("error", err.Error()),
		)
		result := o.failLoop(ctx, machine, sink, errors.NewSystemError("pre-LLM filter error", err))
		return nil, result
	}

	for _, d := range decisions {
		// Emit content-analysis events before any state transition (D59 §2.3).
		for _, evtType := range telemetry.ClassifyFilterDecision(d) {
			sink(ctx, event.InvocationEvent{
				Type:         evtType,
				State:        machine.State(),
				At:           time.Now(),
				FilterPhase:  "pre_llm",
				FilterField:  d.Field,
				FilterReason: d.Reason,
				FilterAction: string(d.Action),
			})
		}
		if d.Action == hooks.FilterActionBlock {
			sysErr := errors.NewPolicyDeniedError("pre_llm_input", d.Reason)
			_ = machine.Transition(state.Failed)
			emitTerminal(event.EventTypeInvocationFailed, state.Failed, sysErr)
			return nil, &praxis.InvocationResult{FinalState: state.Failed}
		}
	}

	return filtered, nil
}

// checkBudget evaluates budget limits and returns a terminal result on breach.
func (o *Orchestrator) checkBudget(
	ctx context.Context,
	machine *state.Machine,
	sink eventSink,
) *praxis.InvocationResult {
	snap, budgetErr := o.budgetGuard.Check(ctx)
	if budgetErr == nil {
		return nil
	}
	_ = machine.Transition(state.BudgetExceeded)
	sink(ctx, event.InvocationEvent{
		Type: event.EventTypeBudgetExceeded, State: state.BudgetExceeded,
		At: time.Now(), Err: budgetErr, BudgetSnapshot: snap,
	})
	return &praxis.InvocationResult{FinalState: state.BudgetExceeded, BudgetSnapshot: snap}
}

// handleProviderError maps a provider error to the appropriate terminal state.
func (o *Orchestrator) handleProviderError(
	ctx context.Context,
	machine *state.Machine,
	_ eventSink,
	emitTerminal func(event.EventType, state.State, error),
	providerErr error,
) *praxis.InvocationResult {
	if ctx.Err() != nil {
		kind := errors.CancellationKindSoft
		if ctx.Err() == context.DeadlineExceeded {
			kind = errors.CancellationKindHard
		}
		typed := errors.NewCancellationError(kind, ctx.Err())
		_ = machine.Transition(state.Cancelled)
		emitTerminal(event.EventTypeInvocationCancelled, state.Cancelled, typed)
		return &praxis.InvocationResult{FinalState: state.Cancelled}
	}
	typed := o.classifier.Classify(providerErr)
	_ = machine.Transition(state.Failed)
	emitTerminal(event.EventTypeInvocationFailed, state.Failed, typed)
	return &praxis.InvocationResult{FinalState: state.Failed}
}

// completeLoop transitions through PostHook → Completed with post-invocation
// policy evaluation.
func (o *Orchestrator) completeLoop(
	ctx context.Context,
	machine *state.Machine,
	sink eventSink,
	emit func(event.EventType, state.State),
	emitTerminal func(event.EventType, state.State, error),
	req praxis.InvocationRequest,
	model string,
	msg llm.Message,
) *praxis.InvocationResult {
	if err := machine.Transition(state.PostHook); err != nil {
		return o.failLoop(ctx, machine, sink, errors.NewSystemError("transition to PostHook failed", err))
	}
	emit(event.EventTypePostHookStarted, state.PostHook)

	// Post-invocation policy hook evaluation.
	resp := llm.LLMResponse{Message: msg}
	postResult, postAuditNote, continueLoop := o.evaluatePolicy(ctx, machine, sink, emit, emitTerminal, hooks.PhasePostInvocation, hooks.PolicyInput{
		Model:        model,
		SystemPrompt: req.SystemPrompt,
		Messages:     req.Messages,
		LLMResponse:  &resp,
		Metadata:     req.Metadata,
	})
	if postResult != nil {
		return postResult
	}

	// VerdictContinue at PostInvocation: force another LLM turn.
	// Transition to LLMCall; runIteration will emit the LLMCallStarted event.
	if continueLoop {
		if err := machine.Transition(state.LLMCall); err != nil {
			return o.failLoop(ctx, machine, sink, errors.NewSystemError("transition to LLMCall for continue failed", err))
		}
		return nil // loop continues
	}

	if err := machine.Transition(state.Completed); err != nil {
		return o.failLoop(ctx, machine, sink, errors.NewSystemError("transition to Completed failed", err))
	}
	sink(ctx, event.InvocationEvent{
		Type:      event.EventTypePostHookCompleted,
		State:     state.Completed,
		At:        time.Now(),
		AuditNote: postAuditNote,
	})

	emitTerminal(event.EventTypeInvocationCompleted, state.Completed, nil)
	return &praxis.InvocationResult{
		Response:   &msg,
		FinalState: state.Completed,
	}
}

// handleToolCallsWithEvents dispatches tool calls with event emission
// and post-tool filter chain. Returns the assembled tool-result message,
// the last filtered tool result (for PhasePostToolOutput policy input),
// and any error.
func (o *Orchestrator) handleToolCallsWithEvents(
	ctx context.Context,
	msg llm.Message,
	machine *state.Machine,
	sink eventSink,
	invocationID string,
	signedIdentity string,
) (llm.Message, *tools.ToolResult, error) {
	var resultParts []llm.MessagePart
	var toolResults []tools.ToolResult
	var lastToolResult *tools.ToolResult
	ictx := tools.InvocationContext{
		InvocationID:  invocationID,
		SignedIdentity: signedIdentity,
	}

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
		return llm.Message{}, nil, errors.NewSystemError(
			"StopReasonToolUse with no tool call parts in assistant message", nil)
	}

	if err := machine.Transition(state.ToolCall); err != nil {
		return llm.Message{}, nil, errors.NewSystemError("transition to ToolCall failed", err)
	}

	for _, call := range toolCalls {
		// Pre-tool filter: inspect/modify/block before execution.
		var preToolDecisions []hooks.FilterDecision
		var preToolErr error
		func() {
			defer func() {
				if r := recover(); r != nil {
					preToolErr = fmt.Errorf("panic in pre-tool filter: %v", r)
					o.logger.LogAttrs(ctx, slog.LevelWarn, "pre-tool filter panic recovered",
						slog.Any("panic", r),
					)
				}
			}()
			call, preToolDecisions, preToolErr = o.preToolFilter.Filter(ctx, call)
		}()
		if preToolErr != nil {
			o.logger.LogAttrs(ctx, slog.LevelWarn, "pre-tool filter error (trust-boundary-internal)",
				slog.String("error", preToolErr.Error()),
			)
			return llm.Message{}, nil, errors.NewSystemError("pre-tool filter error", preToolErr)
		}
		for _, d := range preToolDecisions {
			for _, evtType := range telemetry.ClassifyFilterDecision(d) {
				sink(ctx, event.InvocationEvent{
					Type:         evtType,
					State:        state.ToolCall,
					At:           time.Now(),
					FilterPhase:  "pre_tool",
					FilterField:  d.Field,
					FilterReason: d.Reason,
					FilterAction: string(d.Action),
				})
			}
			if d.Action == hooks.FilterActionBlock {
				return llm.Message{}, nil, errors.NewPolicyDeniedError("pre_tool_input", d.Reason)
			}
		}

		sink(ctx, event.InvocationEvent{
			Type: event.EventTypeToolCallStarted, State: state.ToolCall,
			At: time.Now(), ToolCallID: call.CallID, ToolName: call.Name,
		})

		_ = o.budgetGuard.RecordToolCall(ctx)

		result, err := o.toolInvoker.Invoke(ctx, ictx, call)
		if err != nil {
			return llm.Message{}, nil, errors.NewSystemError(
				fmt.Sprintf("tool invoker failure for call %q", call.CallID), err)
		}

		sink(ctx, event.InvocationEvent{
			Type: event.EventTypeToolCallCompleted, State: state.ToolCall,
			At: time.Now(), ToolCallID: call.CallID, ToolName: call.Name,
		})

		toolResults = append(toolResults, result)
	}

	// PostToolFilter state — apply filter to each tool result.
	if err := machine.Transition(state.PostToolFilter); err != nil {
		return llm.Message{}, nil, errors.NewSystemError("transition to PostToolFilter failed", err)
	}
	sink(ctx, event.InvocationEvent{
		Type: event.EventTypePostToolFilterStarted, State: state.PostToolFilter, At: time.Now(),
	})

	for _, tr := range toolResults {
		var filtered tools.ToolResult
		var decisions []hooks.FilterDecision
		var filterErr error
		func() {
			defer func() {
				if r := recover(); r != nil {
					filterErr = fmt.Errorf("panic in post-tool filter: %v", r)
					o.logger.LogAttrs(ctx, slog.LevelError, "post-tool filter panic recovered (trust-boundary-crossing)",
						slog.Any("panic", r),
					)
				}
			}()
			filtered, decisions, filterErr = o.postToolFilter.Filter(ctx, tr)
		}()
		if filterErr != nil {
			o.logger.LogAttrs(ctx, slog.LevelError, "post-tool filter error (trust-boundary-crossing)",
				slog.String("error", filterErr.Error()),
			)
			return llm.Message{}, nil, errors.NewSystemError("post-tool filter error", filterErr)
		}
		for _, d := range decisions {
			// Emit content-analysis events before any state transition (D59 §2.3).
			for _, evtType := range telemetry.ClassifyFilterDecision(d) {
				sink(ctx, event.InvocationEvent{
					Type:         evtType,
					State:        state.PostToolFilter,
					At:           time.Now(),
					FilterPhase:  "post_tool",
					FilterField:  d.Field,
					FilterReason: d.Reason,
					FilterAction: string(d.Action),
				})
			}
			if d.Action == hooks.FilterActionBlock {
				return llm.Message{}, nil, errors.NewPolicyDeniedError("post_tool_output", d.Reason)
			}
		}
		lastToolResult = &filtered
		resultParts = append(resultParts, llm.ToolResultPart(&llm.LLMToolResult{
			CallID:  filtered.CallID,
			Content: filtered.Content,
			IsError: filtered.Status == tools.ToolStatusError || filtered.Status == tools.ToolStatusNotImplemented,
		}))
	}

	sink(ctx, event.InvocationEvent{
		Type: event.EventTypePostToolFilterCompleted, State: state.PostToolFilter, At: time.Now(),
	})

	return llm.Message{Role: llm.RoleUser, Parts: resultParts}, lastToolResult, nil
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
		Type: event.EventTypeInvocationFailed, State: machine.State(), At: time.Now(), Err: err,
	})
	return &praxis.InvocationResult{FinalState: machine.State()}
}
