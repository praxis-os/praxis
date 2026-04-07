// SPDX-License-Identifier: Apache-2.0

// Package orchestrator is the primary entry point for the praxis agent
// orchestration runtime.
//
// Use [New] to create an [Orchestrator] configured with an [llm.Provider]
// and optional settings such as a default model and iteration limit.
// Call [Orchestrator.Invoke] to run a single agent invocation through the
// full state machine lifecycle.
//
// Example:
//
//	orch, err := orchestrator.New(myProvider,
//	    orchestrator.WithDefaultModel("claude-3-5-sonnet-20241022"),
//	    orchestrator.WithMaxIterations(20),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	result, err := orch.Invoke(ctx, req)
package orchestrator
