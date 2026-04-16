// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"testing"

	"github.com/praxis-os/praxis/credentials"
	"github.com/praxis-os/praxis/telemetry"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// TestDefaultConfig pins the zero-wiring default shape the adapter
// guarantees when [New] is called with no options. Every field must
// be non-nil after defaultConfig so later code paths never need to
// nil-check before dispatching to the corresponding dependency.
func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	c := defaultConfig()

	if c.resolver == nil {
		t.Error("defaultConfig: resolver is nil")
	}
	if _, ok := c.resolver.(credentials.NullResolver); !ok {
		t.Errorf("defaultConfig: resolver type = %T, want credentials.NullResolver", c.resolver)
	}

	if c.metricsRecorder == nil {
		t.Error("defaultConfig: metricsRecorder is nil")
	}
	if _, ok := c.metricsRecorder.(telemetry.NoopMetricsRecorder); !ok {
		t.Errorf("defaultConfig: metricsRecorder type = %T, want telemetry.NoopMetricsRecorder", c.metricsRecorder)
	}

	if c.tracerProvider == nil {
		t.Error("defaultConfig: tracerProvider is nil")
	}

	if c.maxResponseBytes != DefaultMaxResponseBytes {
		t.Errorf("defaultConfig: maxResponseBytes = %d, want %d",
			c.maxResponseBytes, DefaultMaxResponseBytes)
	}
	if DefaultMaxResponseBytes != 16*1024*1024 {
		t.Errorf("DefaultMaxResponseBytes constant drifted: got %d, want %d",
			DefaultMaxResponseBytes, 16*1024*1024)
	}
}

// customResolver is a zero-cost [credentials.Resolver] double used to
// assert that [WithResolver] actually overrides the default.
type customResolver struct{}

func (customResolver) Fetch(_ context.Context, _ string) (credentials.Credential, error) {
	return credentials.Credential{}, nil
}

// customRecorder is a zero-cost [telemetry.MetricsRecorder] double
// used to assert that [WithMetricsRecorder] actually overrides the
// default. It satisfies the core interface by embedding the noop
// recorder; methods are inherited unchanged.
type customRecorder struct {
	telemetry.NoopMetricsRecorder
	tag string
}

func TestWithResolver(t *testing.T) {
	t.Parallel()

	t.Run("custom resolver overrides default", func(t *testing.T) {
		t.Parallel()

		c := defaultConfig()
		WithResolver(customResolver{})(&c)
		if _, ok := c.resolver.(customResolver); !ok {
			t.Errorf("resolver type = %T, want customResolver", c.resolver)
		}
	})

	t.Run("nil resolver preserves default", func(t *testing.T) {
		t.Parallel()

		c := defaultConfig()
		WithResolver(nil)(&c)
		if _, ok := c.resolver.(credentials.NullResolver); !ok {
			t.Errorf("nil arg: resolver type = %T, want credentials.NullResolver", c.resolver)
		}
	})
}

func TestWithMetricsRecorder(t *testing.T) {
	t.Parallel()

	t.Run("custom recorder overrides default", func(t *testing.T) {
		t.Parallel()

		rec := customRecorder{tag: "probe"}
		c := defaultConfig()
		WithMetricsRecorder(rec)(&c)
		got, ok := c.metricsRecorder.(customRecorder)
		if !ok {
			t.Fatalf("recorder type = %T, want customRecorder", c.metricsRecorder)
		}
		if got.tag != "probe" {
			t.Errorf("recorder tag = %q, want %q", got.tag, "probe")
		}
	})

	t.Run("nil recorder preserves default", func(t *testing.T) {
		t.Parallel()

		c := defaultConfig()
		WithMetricsRecorder(nil)(&c)
		if _, ok := c.metricsRecorder.(telemetry.NoopMetricsRecorder); !ok {
			t.Errorf("nil arg: recorder type = %T, want telemetry.NoopMetricsRecorder", c.metricsRecorder)
		}
	})
}

func TestWithTracerProvider(t *testing.T) {
	t.Parallel()

	t.Run("custom provider overrides default", func(t *testing.T) {
		t.Parallel()

		// noop.NewTracerProvider returns a concrete type distinct
		// from the otel.GetTracerProvider default. This gives the
		// override assertion a type it can distinguish from the
		// installed default via a type assertion against the
		// no-op concrete type.
		custom := noop.NewTracerProvider()
		c := defaultConfig()
		WithTracerProvider(custom)(&c)
		if c.tracerProvider == nil {
			t.Fatal("custom provider: tracerProvider is nil after option")
		}
		if _, ok := c.tracerProvider.(noop.TracerProvider); !ok {
			t.Errorf("custom provider: type = %T, want noop.TracerProvider", c.tracerProvider)
		}
	})

	t.Run("nil provider preserves default", func(t *testing.T) {
		t.Parallel()

		c := defaultConfig()
		before := c.tracerProvider
		WithTracerProvider(nil)(&c)
		if c.tracerProvider != before {
			t.Error("nil arg: tracerProvider should not change")
		}
	})
}

func TestWithMaxResponseBytes(t *testing.T) {
	t.Parallel()

	t.Run("positive value overrides default", func(t *testing.T) {
		t.Parallel()

		c := defaultConfig()
		WithMaxResponseBytes(1024)(&c)
		if c.maxResponseBytes != 1024 {
			t.Errorf("maxResponseBytes = %d, want 1024", c.maxResponseBytes)
		}
	})

	t.Run("zero is ignored, default preserved", func(t *testing.T) {
		t.Parallel()

		c := defaultConfig()
		WithMaxResponseBytes(0)(&c)
		if c.maxResponseBytes != DefaultMaxResponseBytes {
			t.Errorf("zero arg: maxResponseBytes = %d, want %d (default preserved)",
				c.maxResponseBytes, DefaultMaxResponseBytes)
		}
	})

	t.Run("negative is ignored, default preserved", func(t *testing.T) {
		t.Parallel()

		c := defaultConfig()
		WithMaxResponseBytes(-1)(&c)
		if c.maxResponseBytes != DefaultMaxResponseBytes {
			t.Errorf("negative arg: maxResponseBytes = %d, want %d (default preserved)",
				c.maxResponseBytes, DefaultMaxResponseBytes)
		}
	})
}

// TestOptionIsFunctionType pins the Option concrete shape — a
// function taking a *config. If the type is ever promoted to a
// struct or an interface, this test fails to compile.
func TestOptionIsFunctionType(t *testing.T) {
	t.Parallel()

	// Compile-time check: an Option value is callable with a *config.
	var opt Option = func(_ *config) {}
	var c config
	opt(&c)

	// Compile-time check: the return type of WithMaxResponseBytes is
	// assignable to Option.
	var _ = WithMaxResponseBytes(1)
	var _ = WithResolver(nil)
	var _ = WithMetricsRecorder(nil)
	var _ = WithTracerProvider(trace.TracerProvider(nil))
}
