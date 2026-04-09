// SPDX-License-Identifier: Apache-2.0

package slogredact_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	slogredact "github.com/praxis-os/praxis/telemetry/slog"
)

const redacted = "[REDACTED]"

// ---- helpers ----------------------------------------------------------------

// captureHandler records every [slog.Record] passed to Handle so tests can
// inspect the sanitised output without depending on a text/JSON format.
type captureHandler struct {
	enabled bool
	records []slog.Record
	attrs   []slog.Attr
	groups  []string
}

func newCapture() *captureHandler { return &captureHandler{enabled: true} }

func (c *captureHandler) Enabled(_ context.Context, _ slog.Level) bool { return c.enabled }

func (c *captureHandler) Handle(_ context.Context, r slog.Record) error {
	c.records = append(c.records, r)
	return nil
}

func (c *captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	cp := *c
	cp.attrs = append(append([]slog.Attr{}, c.attrs...), attrs...)
	return &cp
}

func (c *captureHandler) WithGroup(name string) slog.Handler {
	cp := *c
	cp.groups = append(append([]string{}, c.groups...), name)
	return &cp
}

// attrsFromRecord collects all top-level attributes of r into a map for easy
// assertion.
func attrsFromRecord(r slog.Record) map[string]slog.Value {
	m := make(map[string]slog.Value)
	r.Attrs(func(a slog.Attr) bool {
		m[a.Key] = a.Value
		return true
	})
	return m
}

// ---- Enabled ----------------------------------------------------------------

func TestRedactingHandler_Enabled(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		level   slog.Level
		want    bool
	}{
		{"inner enabled debug", true, slog.LevelDebug, true},
		{"inner enabled error", true, slog.LevelError, true},
		{"inner disabled", false, slog.LevelInfo, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			inner := newCapture()
			inner.enabled = tc.enabled
			h := slogredact.NewRedactingHandler(inner)

			if got := h.Enabled(context.Background(), tc.level); got != tc.want {
				t.Errorf("Enabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

// ---- Default deny list redaction --------------------------------------------

func TestRedactingHandler_DefaultDenyList(t *testing.T) {
	tests := []struct {
		key       string
		wantRedact bool
	}{
		// Exact matches.
		{"token", true},
		{"key", true},
		{"secret", true},
		{"password", true},
		{"credential", true},
		{"authorization", true},
		// Substrings.
		{"api_token", true},
		{"access_key", true},
		{"client_secret", true},
		{"db_password", true},
		{"credentials", true},
		{"Authorization", true},  // capital A
		{"X-Authorization", true},
		// Safe keys.
		{"user_id", false},
		{"request_id", false},
		{"duration_ms", false},
		{"model", false},
		{"status", false},
	}

	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			inner := newCapture()
			h := slogredact.NewRedactingHandler(inner)
			logger := slog.New(h)

			logger.Info("msg", tc.key, "sensitive-value")

			if len(inner.records) != 1 {
				t.Fatalf("expected 1 record, got %d", len(inner.records))
			}

			attrs := attrsFromRecord(inner.records[0])
			val, ok := attrs[tc.key]
			if !ok {
				t.Fatalf("attribute %q not found in record", tc.key)
			}

			if tc.wantRedact {
				if got := val.String(); got != redacted {
					t.Errorf("key %q: got value %q, want [REDACTED]", tc.key, got)
				}
			} else {
				if got := val.String(); got == redacted {
					t.Errorf("key %q: value was unexpectedly redacted", tc.key)
				}
			}
		})
	}
}

// ---- Case-insensitive matching ----------------------------------------------

func TestRedactingHandler_CaseInsensitive(t *testing.T) {
	cases := []string{
		"TOKEN", "Token", "ToKeN",
		"PASSWORD", "Password",
		"SECRET", "Secret",
		"KEY", "Key",
		"CREDENTIAL", "Credential",
		"AUTHORIZATION", "Authorization",
	}

	for _, key := range cases {
		t.Run(key, func(t *testing.T) {
			inner := newCapture()
			h := slogredact.NewRedactingHandler(inner)
			slog.New(h).Info("msg", key, "v")

			attrs := attrsFromRecord(inner.records[0])
			if attrs[key].String() != redacted {
				t.Errorf("key %q: expected redaction", key)
			}
		})
	}
}

// ---- WithDenyList (replace) -------------------------------------------------

func TestRedactingHandler_WithDenyList(t *testing.T) {
	inner := newCapture()
	h := slogredact.NewRedactingHandler(inner,
		slogredact.WithDenyList("ssn", "cvv"),
	)
	logger := slog.New(h)

	// Old default keys must NOT be redacted anymore.
	logger.Info("msg", "password", "p@ss", "ssn", "123-45-6789", "cvv", "999")

	attrs := attrsFromRecord(inner.records[0])

	// "password" is no longer in the deny list.
	if v := attrs["password"].String(); v == redacted {
		t.Errorf("password: expected plain value after deny list replacement, got redacted")
	}

	// "ssn" and "cvv" are in the new list.
	for _, k := range []string{"ssn", "cvv"} {
		if v := attrs[k].String(); v != redacted {
			t.Errorf("%q: expected [REDACTED], got %q", k, v)
		}
	}
}

// ---- WithAdditionalDenyKeys (extend) ----------------------------------------

func TestRedactingHandler_WithAdditionalDenyKeys(t *testing.T) {
	inner := newCapture()
	h := slogredact.NewRedactingHandler(inner,
		slogredact.WithAdditionalDenyKeys("ssn", "cvv"),
	)
	logger := slog.New(h)

	logger.Info("msg",
		"password", "p@ss",
		"ssn", "123-45-6789",
		"cvv", "999",
		"user_id", "u-1",
	)

	attrs := attrsFromRecord(inner.records[0])

	// Default keys still redacted.
	if attrs["password"].String() != redacted {
		t.Error("password: expected redaction from default list")
	}
	// Additional keys also redacted.
	for _, k := range []string{"ssn", "cvv"} {
		if attrs[k].String() != redacted {
			t.Errorf("%q: expected [REDACTED]", k)
		}
	}
	// Safe keys untouched.
	if v := attrs["user_id"].String(); v == redacted {
		t.Error("user_id: should not be redacted")
	}
}

// ---- WithRedactedValue ------------------------------------------------------

func TestRedactingHandler_WithRedactedValue(t *testing.T) {
	inner := newCapture()
	h := slogredact.NewRedactingHandler(inner,
		slogredact.WithRedactedValue("<REMOVED>"),
	)
	slog.New(h).Info("msg", "api_token", "abc123")

	attrs := attrsFromRecord(inner.records[0])
	if got := attrs["api_token"].String(); got != "<REMOVED>" {
		t.Errorf("got %q, want <REMOVED>", got)
	}
}

// ---- WithAttrs redacts at binding time --------------------------------------

func TestRedactingHandler_WithAttrs(t *testing.T) {
	inner := newCapture()
	h := slogredact.NewRedactingHandler(inner)

	// Bind a sensitive attribute via WithAttrs.
	bound := h.WithAttrs([]slog.Attr{
		slog.String("api_key", "should-be-redacted"),
		slog.String("user_id", "u-42"),
	})

	// WithAttrs must return a *RedactingHandler (not the bare inner type).
	if _, ok := bound.(*slogredact.RedactingHandler); !ok {
		t.Fatal("WithAttrs did not return *RedactingHandler")
	}

	// The captureHandler.WithAttrs returns a copy; records emitted through
	// `bound` land in that copy's slice, not in `inner`. Verify via the
	// JSON-handler path instead (see TestRedactingHandler_WithAttrs_ValuesRedacted).
	// Here we only assert the return-type contract and that Handle succeeds.
	ctx := context.Background()
	if err := bound.Handle(ctx, slog.NewRecord(time.Now(), slog.LevelInfo, "test", 0)); err != nil {
		t.Fatalf("Handle returned unexpected error: %v", err)
	}
}

// TestRedactingHandler_WithAttrs_ValuesRedacted verifies redaction happens on
// pre-bound attrs by routing output through a JSON handler into a buffer.
//
// This is the authoritative assertion for WithAttrs behaviour. The sibling
// TestRedactingHandler_WithAttrs only checks the return type.
func TestRedactingHandler_WithAttrs_ValuesRedacted(t *testing.T) {
	var buf bytes.Buffer
	jsonHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	h := slogredact.NewRedactingHandler(jsonHandler)

	bound := slog.New(h.WithAttrs([]slog.Attr{
		slog.String("api_key", "super-secret"),
		slog.String("user_id", "u-1"),
	}))
	bound.Info("hello")

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("json.Unmarshal: %v (raw: %s)", err, buf.String())
	}

	if got, ok := record["api_key"]; !ok {
		t.Error("api_key missing from output")
	} else if got != redacted {
		t.Errorf("api_key = %q, want [REDACTED]", got)
	}

	if got, ok := record["user_id"]; !ok {
		t.Error("user_id missing from output")
	} else if got == redacted {
		t.Errorf("user_id was unexpectedly redacted")
	}
}

// ---- WithGroup delegates correctly -----------------------------------------

func TestRedactingHandler_WithGroup(t *testing.T) {
	inner := newCapture()
	h := slogredact.NewRedactingHandler(inner)

	grouped := h.WithGroup("request")

	// WithGroup must return a *RedactingHandler.
	if _, ok := grouped.(*slogredact.RedactingHandler); !ok {
		t.Fatal("WithGroup did not return *RedactingHandler")
	}

	// captureHandler.WithGroup returns a copy so `inner` is not mutated; the
	// group-nesting behaviour is verified end-to-end in
	// TestRedactingHandler_WithGroup_RedactsAttrs. Here we just confirm Handle
	// succeeds through the grouped handler without error.
	ctx := context.Background()
	if err := grouped.Handle(ctx, slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)); err != nil {
		t.Fatalf("Handle returned unexpected error: %v", err)
	}
}

// TestRedactingHandler_WithGroup_RedactsAttrs verifies that attributes logged
// inside a group are still redacted.
func TestRedactingHandler_WithGroup_RedactsAttrs(t *testing.T) {
	var buf bytes.Buffer
	jsonHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	h := slogredact.NewRedactingHandler(jsonHandler)

	logger := slog.New(h.WithGroup("http"))
	logger.Info("request", "Authorization", "Bearer secret-token", "path", "/api/v1")

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("json.Unmarshal: %v (raw: %s)", err, buf.String())
	}

	httpGroup, ok := record["http"].(map[string]any)
	if !ok {
		t.Fatalf("http group missing or wrong type in %v", record)
	}

	if got, ok := httpGroup["Authorization"]; !ok {
		t.Error("Authorization missing from http group")
	} else if got != redacted {
		t.Errorf("Authorization = %q, want [REDACTED]", got)
	}

	if got, ok := httpGroup["path"]; !ok {
		t.Error("path missing from http group")
	} else if got == redacted {
		t.Errorf("path was unexpectedly redacted")
	}
}

// ---- Group attribute (slog.Group) inside a record ---------------------------

func TestRedactingHandler_GroupAttrInRecord(t *testing.T) {
	var buf bytes.Buffer
	jsonHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	h := slogredact.NewRedactingHandler(jsonHandler)

	slog.New(h).Info("msg",
		slog.Group("db",
			slog.String("password", "hunter2"),
			slog.String("host", "localhost"),
		),
	)

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("json.Unmarshal: %v (raw: %s)", err, buf.String())
	}

	dbGroup, ok := record["db"].(map[string]any)
	if !ok {
		t.Fatalf("db group missing or wrong type in %v", record)
	}

	if got := dbGroup["password"]; got != redacted {
		t.Errorf("db.password = %q, want [REDACTED]", got)
	}
	if got := dbGroup["host"]; got == redacted {
		t.Errorf("db.host was unexpectedly redacted")
	}
}

// ---- Nil inner handler panics -----------------------------------------------

func TestNewRedactingHandler_NilPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil inner handler, got none")
		}
	}()
	slogredact.NewRedactingHandler(nil) //nolint:staticcheck
}

// ---- Non-denied keys pass through unchanged ---------------------------------

func TestRedactingHandler_NonDeniedPassThrough(t *testing.T) {
	inner := newCapture()
	h := slogredact.NewRedactingHandler(inner)

	slog.New(h).Info("msg",
		"user_id", "u-1",
		"request_id", "r-999",
		"latency_ms", 42,
		"model", "claude-3",
	)

	attrs := attrsFromRecord(inner.records[0])
	for k, v := range attrs {
		if v.String() == redacted {
			t.Errorf("key %q was unexpectedly redacted", k)
		}
	}
}

// ---- compile-time interface check -------------------------------------------

var _ slog.Handler = (*slogredact.RedactingHandler)(nil)
