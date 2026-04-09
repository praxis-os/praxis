// SPDX-License-Identifier: Apache-2.0

// Package slogredact provides a [log/slog.Handler] wrapper that redacts
// sensitive attribute values before forwarding log records to an inner handler.
//
// # Usage
//
//	inner := slog.NewJSONHandler(os.Stderr, nil)
//	h := slogredact.NewRedactingHandler(inner)
//	logger := slog.New(h)
//
// Any attribute whose key contains a substring from the deny list
// (case-insensitive) has its value replaced with the redaction placeholder
// before the record reaches the inner handler. The default deny list covers
// common secret-bearing key names: token, key, secret, password, credential,
// authorization.
//
// # Customisation
//
// Replace the deny list entirely:
//
//	h := slogredact.NewRedactingHandler(inner,
//	    slogredact.WithDenyList("api_token", "bearer"),
//	)
//
// Extend the default deny list:
//
//	h := slogredact.NewRedactingHandler(inner,
//	    slogredact.WithAdditionalDenyKeys("ssn", "cvv"),
//	)
//
// Customise the placeholder:
//
//	h := slogredact.NewRedactingHandler(inner,
//	    slogredact.WithRedactedValue("<REMOVED>"),
//	)
//
// Stability: stable-v0.5.
package slogredact

import (
	"context"
	"log/slog"
	"strings"
)

// defaultDenySubstrings is the set of case-insensitive substrings that trigger
// redaction when found inside an attribute key.
var defaultDenySubstrings = []string{
	"token",
	"key",
	"secret",
	"password",
	"credential",
	"authorization",
}

// defaultRedactedValue is the placeholder substituted for sensitive values.
const defaultRedactedValue = "[REDACTED]"

// RedactingHandler wraps an inner [slog.Handler] and redacts attribute values
// whose keys match any entry in the configured deny list.
//
// Matching is case-insensitive substring matching: a key is considered
// sensitive if it contains any deny-list substring. For example, the key
// "API_TOKEN" matches the deny entry "token".
//
// RedactingHandler is safe for concurrent use. It holds no mutable state after
// construction; all configuration is captured at [NewRedactingHandler] time.
type RedactingHandler struct {
	inner        slog.Handler
	denyList     []string // lower-cased substrings
	redactedWith string
}

// Option is a functional option for [NewRedactingHandler].
type Option func(*RedactingHandler)

// WithDenyList replaces the default deny list with the provided substrings.
// Matching is case-insensitive, so callers need not normalise the case of the
// supplied values.
func WithDenyList(keys ...string) Option {
	return func(h *RedactingHandler) {
		lower := make([]string, len(keys))
		for i, k := range keys {
			lower[i] = strings.ToLower(k)
		}
		h.denyList = lower
	}
}

// WithAdditionalDenyKeys appends the provided substrings to the default deny
// list without discarding the defaults. Matching is case-insensitive.
func WithAdditionalDenyKeys(keys ...string) Option {
	return func(h *RedactingHandler) {
		for _, k := range keys {
			h.denyList = append(h.denyList, strings.ToLower(k))
		}
	}
}

// WithRedactedValue overrides the placeholder string that replaces a sensitive
// attribute's value. The default placeholder is "[REDACTED]".
func WithRedactedValue(v string) Option {
	return func(h *RedactingHandler) {
		h.redactedWith = v
	}
}

// NewRedactingHandler constructs a [RedactingHandler] that wraps inner and
// applies the given options.
//
// If inner is nil, NewRedactingHandler panics; callers must supply a valid
// handler (use [slog.DiscardHandler] to silence output while still exercising
// the redaction logic).
func NewRedactingHandler(inner slog.Handler, opts ...Option) *RedactingHandler {
	if inner == nil {
		panic("slogredact: inner handler must not be nil")
	}

	// Build default deny list (already lower-cased).
	deny := make([]string, len(defaultDenySubstrings))
	copy(deny, defaultDenySubstrings)

	h := &RedactingHandler{
		inner:        inner,
		denyList:     deny,
		redactedWith: defaultRedactedValue,
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

// Enabled reports whether the inner handler is enabled for the given level.
// RedactingHandler adds no level filtering of its own.
func (h *RedactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle redacts sensitive attributes in r, then forwards the sanitised record
// to the inner handler.
//
// The original record is not modified; Handle operates on a shallow clone whose
// attribute set is rebuilt with sensitive values replaced by the redaction
// placeholder.
func (h *RedactingHandler) Handle(ctx context.Context, r slog.Record) error {
	// Clone the record to avoid mutating the caller's copy. The clone carries
	// the same time, level, message, and PC, but its attribute list is empty so
	// we can rebuild it selectively.
	clean := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)

	r.Attrs(func(a slog.Attr) bool {
		clean.AddAttrs(h.redactAttr(a))
		return true
	})

	return h.inner.Handle(ctx, clean)
}

// WithAttrs returns a new handler whose pre-bound attribute set is the redacted
// form of attrs, forwarded to the inner handler via its own WithAttrs.
//
// Redaction at binding time means sensitive values never reach the inner handler
// even when they are supplied as pre-bound context attributes.
func (h *RedactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		redacted[i] = h.redactAttr(a)
	}

	return &RedactingHandler{
		inner:        h.inner.WithAttrs(redacted),
		denyList:     h.denyList,
		redactedWith: h.redactedWith,
	}
}

// WithGroup returns a new handler that wraps the inner handler's WithGroup
// result. Group nesting is delegated entirely to the inner handler; the
// RedactingHandler layer is preserved so that subsequent attribute operations
// continue to apply redaction.
func (h *RedactingHandler) WithGroup(name string) slog.Handler {
	return &RedactingHandler{
		inner:        h.inner.WithGroup(name),
		denyList:     h.denyList,
		redactedWith: h.redactedWith,
	}
}

// redactAttr returns a copy of a with its value replaced by the redaction
// placeholder when the key matches the deny list, or an equivalent copy for
// group attributes (recursing into the group's attributes).
func (h *RedactingHandler) redactAttr(a slog.Attr) slog.Attr {
	// Resolve any LogValuer before inspecting the kind.
	a.Value = a.Value.Resolve()

	if a.Value.Kind() == slog.KindGroup {
		// For group attributes, recurse into the group's children. The group
		// key itself is never redacted (it is structural, not a secret value).
		groupAttrs := a.Value.Group()
		redacted := make([]any, 0, len(groupAttrs))
		for _, child := range groupAttrs {
			redacted = append(redacted, h.redactAttr(child))
		}
		return slog.Group(a.Key, redacted...)
	}

	if h.isDenied(a.Key) {
		return slog.String(a.Key, h.redactedWith)
	}

	return a
}

// isDenied reports whether key contains any substring from the deny list
// (case-insensitive).
func (h *RedactingHandler) isDenied(key string) bool {
	lower := strings.ToLower(key)
	for _, sub := range h.denyList {
		if strings.Contains(lower, sub) {
			return true
		}
	}
	return false
}
