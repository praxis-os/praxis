// SPDX-License-Identifier: Apache-2.0

package state_test

import (
	"testing"
	"time"

	. "github.com/praxis-os/praxis/state"
)

// BenchmarkSingleTransition benchmarks a single legal state transition
// (Created → Initializing) in isolation.
func BenchmarkSingleTransition(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := NewMachine()
		_ = m.Transition(Initializing)
	}
}

// BenchmarkHappyPathCycle benchmarks a complete happy-path invocation cycle:
// Created → Initializing → PreHook → LLMCall → ToolDecision → PostHook → Completed.
// Each iteration includes 6 transitions.
func BenchmarkHappyPathCycle(b *testing.B) {
	steps := []State{Initializing, PreHook, LLMCall, ToolDecision, PostHook, Completed}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := NewMachine()
		for _, s := range steps {
			_ = m.Transition(s)
		}
	}
}

// BenchmarkToolLoopCycle benchmarks a multi-turn invocation with a tool loop:
// Created → Initializing → PreHook → LLMCall → ToolDecision → ToolCall →
// PostToolFilter → LLMContinuation → ToolDecision → PostHook → Completed.
// Each iteration includes 10 transitions.
func BenchmarkToolLoopCycle(b *testing.B) {
	steps := []State{
		Initializing, PreHook, LLMCall, ToolDecision,
		ToolCall, PostToolFilter, LLMContinuation, ToolDecision,
		PostHook, Completed,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := NewMachine()
		for _, s := range steps {
			_ = m.Transition(s)
		}
	}
}

// TestStateMachineThroughput verifies the state machine sustains at least
// 1 million transitions per second per core (T23.2 / PRAX-135).
//
// This is a non-benchmark assertion: it runs a timed loop and checks that
// the achieved throughput meets the target.
func TestStateMachineThroughput(t *testing.T) {
	const (
		targetTransitionsPerSec = 1_000_000
		runDuration             = 500 * time.Millisecond
	)

	steps := []State{Initializing, PreHook, LLMCall, ToolDecision, PostHook, Completed}
	transitionsPerCycle := len(steps)

	deadline := time.Now().Add(runDuration)
	var cycles int64
	for time.Now().Before(deadline) {
		m := NewMachine()
		for _, s := range steps {
			_ = m.Transition(s)
		}
		cycles++
	}

	elapsed := time.Since(deadline.Add(-runDuration))
	totalTransitions := cycles * int64(transitionsPerCycle)
	throughput := float64(totalTransitions) / elapsed.Seconds()

	t.Logf("state machine throughput: %.0f transitions/sec (%.0f cycles/sec, n=%d cycles)",
		throughput, float64(cycles)/elapsed.Seconds(), cycles)

	if throughput < float64(targetTransitionsPerSec) {
		t.Errorf("throughput %.0f transitions/sec is below 1M/sec/core target", throughput)
	}
}
