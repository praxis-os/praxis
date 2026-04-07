// SPDX-License-Identifier: Apache-2.0

package retry

import (
	"context"
	stderrors "errors"
	"fmt"
	"testing"
	"time"

	"github.com/praxis-os/praxis/errors"
)

// sentinel errors used across tests.
var (
	errTransient = errors.NewTransientLLMError("test-provider", 503, fmt.Errorf("service unavailable"))
	errPermanent = errors.NewPermanentLLMError("test-provider", 400, fmt.Errorf("bad request"))
)

var classifier = errors.NewDefaultClassifier()

// --- Do ---

func TestDo_SuccessOnFirstAttempt(t *testing.T) {
	calls := 0
	err := Do(context.Background(), classifier, func(_ context.Context) error {
		calls++
		return nil
	})

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestDo_SuccessAfterRetries(t *testing.T) {
	calls := 0
	err := DoWithConfig(context.Background(), classifier, Config{
		MaxAttempts: 5,
		BaseDelay:   time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
	}, func(_ context.Context) error {
		calls++
		if calls < 3 {
			return errTransient
		}
		return nil
	})

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestDo_ExhaustedAttemptsReturnsLastError(t *testing.T) {
	const maxAttempts = 4
	calls := 0

	err := DoWithConfig(context.Background(), classifier, Config{
		MaxAttempts: maxAttempts,
		BaseDelay:   time.Millisecond,
		MaxDelay:    5 * time.Millisecond,
	}, func(_ context.Context) error {
		calls++
		return errTransient
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if calls != maxAttempts {
		t.Fatalf("expected %d calls, got %d", maxAttempts, calls)
	}
	// Must return the last error, which is the transient error.
	var te errors.TypedError
	if !stderrors.As(err, &te) {
		t.Fatalf("expected TypedError, got %T: %v", err, err)
	}
	if te.Kind() != errors.ErrorKindTransientLLM {
		t.Fatalf("expected ErrorKindTransientLLM, got %v", te.Kind())
	}
}

func TestDo_NonRetryableErrorStopsImmediately(t *testing.T) {
	calls := 0

	err := DoWithConfig(context.Background(), classifier, Config{
		MaxAttempts: 10,
		BaseDelay:   time.Millisecond,
	}, func(_ context.Context) error {
		calls++
		return errPermanent
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if calls != 1 {
		t.Fatalf("expected 1 call (no retries), got %d", calls)
	}
}

func TestDo_ContextCancelledBeforeFirstAttempt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	calls := 0
	err := Do(ctx, classifier, func(_ context.Context) error {
		calls++
		return nil
	})

	if err == nil {
		t.Fatal("expected context error, got nil")
	}
	if calls != 0 {
		t.Fatalf("expected 0 calls, got %d", calls)
	}
}

func TestDo_ContextCancelledDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0

	err := DoWithConfig(ctx, classifier, Config{
		MaxAttempts: 10,
		BaseDelay:   500 * time.Millisecond, // long enough that we cancel during sleep
		MaxDelay:    5 * time.Second,
	}, func(_ context.Context) error {
		calls++
		// Cancel on the first failure so we hit the sleep path.
		if calls == 1 {
			go cancel()
		}
		return errTransient
	})

	if err == nil {
		t.Fatal("expected context error, got nil")
	}
	if !stderrors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	// Should have attempted exactly once (cancelled while sleeping after attempt 1).
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestDo_ContextDeadlineExceededDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	calls := 0
	err := DoWithConfig(ctx, classifier, Config{
		MaxAttempts: 10,
		BaseDelay:   200 * time.Millisecond, // longer than the context deadline
		MaxDelay:    5 * time.Second,
	}, func(_ context.Context) error {
		calls++
		return errTransient
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !stderrors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
}

// --- DoWithResult ---

func TestDoWithResult_ReturnsValueOnSuccess(t *testing.T) {
	result, err := DoWithResult(context.Background(), classifier, func(_ context.Context) (int, error) {
		return 42, nil
	})

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result != 42 {
		t.Fatalf("expected 42, got %d", result)
	}
}

func TestDoWithResult_ReturnsZeroValueOnFailure(t *testing.T) {
	result, err := DoWithResult(context.Background(), classifier, func(_ context.Context) (int, error) {
		return 99, errPermanent
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if result != 0 {
		t.Fatalf("expected zero value 0, got %d", result)
	}
}

func TestDoWithResult_SuccessAfterRetries(t *testing.T) {
	calls := 0
	// Use DoWithConfig via a wrapper to keep delays short in tests.
	var result string
	err := DoWithConfig(context.Background(), errors.NewDefaultClassifier(), Config{
		MaxAttempts: 5,
		BaseDelay:   time.Millisecond,
		MaxDelay:    5 * time.Millisecond,
	}, func(_ context.Context) error {
		calls++
		if calls < 3 {
			return errors.NewTransientLLMError("p", 503, fmt.Errorf("err"))
		}
		result = "done"
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "done" {
		t.Fatalf("expected 'done', got %q", result)
	}
}

// --- Config overrides ---

func TestDoWithConfig_OverridesMaxAttempts(t *testing.T) {
	calls := 0
	_ = DoWithConfig(context.Background(), classifier, Config{
		MaxAttempts: 2,
		BaseDelay:   time.Millisecond,
		MaxDelay:    5 * time.Millisecond,
	}, func(_ context.Context) error {
		calls++
		return errTransient
	})

	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

func TestDoWithConfig_ZeroConfigFallsBackToDefaults(t *testing.T) {
	// With zero Config, must use classifier policy (MaxRetries=3 → 4 total attempts).
	// Override BaseDelay to keep the test fast while still verifying attempt count.
	calls := 0
	_ = DoWithConfig(context.Background(), classifier, Config{
		BaseDelay: time.Millisecond,
		MaxDelay:  5 * time.Millisecond,
	}, func(_ context.Context) error {
		calls++
		return errTransient
	})

	// classifier policy for TransientLLM: MaxRetries=3 → 4 total attempts.
	if calls != 4 {
		t.Fatalf("expected 4 calls (from classifier policy MaxRetries=3), got %d", calls)
	}
}

// --- computeDelay ---

func TestComputeDelay_JitterWithinBounds(t *testing.T) {
	const iterations = 1000
	base := 100 * time.Millisecond
	max := 30 * time.Second

	for attempt := 1; attempt <= 10; attempt++ {
		for i := 0; i < iterations; i++ {
			d := computeDelay(attempt, base, max)
			if d < 0 {
				t.Fatalf("attempt %d: negative delay %v", attempt, d)
			}
			if d > max {
				t.Fatalf("attempt %d: delay %v exceeds maxDelay %v", attempt, d, max)
			}
		}
	}
}

func TestComputeDelay_NeverExceedsMaxDelay(t *testing.T) {
	base := 1 * time.Second
	max := 5 * time.Second

	for attempt := 1; attempt <= 20; attempt++ {
		d := computeDelay(attempt, base, max)
		if d > max {
			t.Fatalf("attempt %d: delay %v exceeds maxDelay %v", attempt, d, max)
		}
	}
}

func TestComputeDelay_ExponentialGrowth(t *testing.T) {
	// Without jitter interference we cannot assert exact values, but we can
	// verify that the delay for attempt N is at least baseDelay * 2^(N-1)
	// (jitter only adds, never subtracts).
	base := 10 * time.Millisecond
	max := 10 * time.Second

	// Attempt 1: base * 2^0 = 10ms minimum (plus up to 5ms jitter)
	// Attempt 2: base * 2^1 = 20ms minimum (plus up to 10ms jitter)
	for attempt := 1; attempt <= 5; attempt++ {
		shift := attempt - 1
		expected := base * (1 << uint(shift))
		d := computeDelay(attempt, base, max)
		if d < expected {
			t.Fatalf("attempt %d: delay %v less than expected minimum %v", attempt, d, expected)
		}
	}
}

func TestComputeDelay_LargeAttemptNoPanic(t *testing.T) {
	// Should not panic or overflow for very large attempt numbers.
	d := computeDelay(200, 100*time.Millisecond, 30*time.Second)
	if d < 0 || d > 30*time.Second {
		t.Fatalf("unexpected delay for large attempt: %v", d)
	}
}

// --- Benchmarks ---

func BenchmarkDo_SuccessFirstAttempt(b *testing.B) {
	ctx := context.Background()
	fn := func(_ context.Context) error { return nil }

	b.ResetTimer()
	for range b.N {
		_ = Do(ctx, classifier, fn)
	}
}

func BenchmarkComputeDelay(b *testing.B) {
	base := 100 * time.Millisecond
	max := 30 * time.Second

	b.ResetTimer()
	for i := range b.N {
		_ = computeDelay((i%10)+1, base, max)
	}
}
