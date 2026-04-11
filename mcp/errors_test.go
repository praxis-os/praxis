// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"syscall"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	praxiserrors "github.com/praxis-os/praxis/errors"
)

// TestClassifyCallToolError exercises every branch of the D113
// translation table via synthetic errors. Each sub-test matches
// one row of the decision table; the branch ordering in the
// classifier is first-match-wins, so the tests are written in
// the same order the classifier evaluates them.
func TestClassifyCallToolError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		err     error
		wantSub praxiserrors.ToolSubKind
	}{
		{
			name:    "nil → empty sub-kind",
			err:     nil,
			wantSub: "",
		},
		{
			name:    "context.Canceled → Network",
			err:     context.Canceled,
			wantSub: praxiserrors.ToolSubKindNetwork,
		},
		{
			name:    "context.DeadlineExceeded → Network",
			err:     context.DeadlineExceeded,
			wantSub: praxiserrors.ToolSubKindNetwork,
		},
		{
			name:    "wrapped context.Canceled → Network",
			err:     fmt.Errorf("call-tool: %w", context.Canceled),
			wantSub: praxiserrors.ToolSubKindNetwork,
		},
		{
			name:    "sdkmcp.ErrConnectionClosed → Network",
			err:     sdkmcp.ErrConnectionClosed,
			wantSub: praxiserrors.ToolSubKindNetwork,
		},
		{
			name:    "wrapped sdkmcp.ErrConnectionClosed → Network",
			err:     fmt.Errorf("transport: %w", sdkmcp.ErrConnectionClosed),
			wantSub: praxiserrors.ToolSubKindNetwork,
		},
		{
			name:    "io.EOF → Network",
			err:     io.EOF,
			wantSub: praxiserrors.ToolSubKindNetwork,
		},
		{
			name:    "io.ErrUnexpectedEOF → Network",
			err:     io.ErrUnexpectedEOF,
			wantSub: praxiserrors.ToolSubKindNetwork,
		},
		{
			name:    "syscall.ECONNREFUSED → Network",
			err:     syscall.ECONNREFUSED,
			wantSub: praxiserrors.ToolSubKindNetwork,
		},
		{
			name:    "syscall.ECONNRESET → Network",
			err:     syscall.ECONNRESET,
			wantSub: praxiserrors.ToolSubKindNetwork,
		},
		{
			name:    "syscall.ETIMEDOUT → Network",
			err:     syscall.ETIMEDOUT,
			wantSub: praxiserrors.ToolSubKindNetwork,
		},
		{
			name:    "syscall.EPIPE → Network",
			err:     syscall.EPIPE,
			wantSub: praxiserrors.ToolSubKindNetwork,
		},
		{
			name:    "net.Error in chain → Network",
			err:     &net.OpError{Op: "read", Net: "tcp", Err: errors.New("synthetic")},
			wantSub: praxiserrors.ToolSubKindNetwork,
		},
		{
			name:    "sdkmcp.ErrSessionMissing → CircuitOpen",
			err:     sdkmcp.ErrSessionMissing,
			wantSub: praxiserrors.ToolSubKindCircuitOpen,
		},
		{
			name:    "wrapped sdkmcp.ErrSessionMissing → CircuitOpen",
			err:     fmt.Errorf("session: %w", sdkmcp.ErrSessionMissing),
			wantSub: praxiserrors.ToolSubKindCircuitOpen,
		},
		{
			name:    "HTTP 401 Unauthorized → CircuitOpen",
			err:     errors.New("POST /mcp: Unauthorized"),
			wantSub: praxiserrors.ToolSubKindCircuitOpen,
		},
		{
			name:    "HTTP 403 Forbidden → CircuitOpen",
			err:     errors.New("POST /mcp: Forbidden"),
			wantSub: praxiserrors.ToolSubKindCircuitOpen,
		},
		{
			name:    "HTTP 429 Too Many Requests → Network",
			err:     errors.New("POST /mcp: Too Many Requests"),
			wantSub: praxiserrors.ToolSubKindNetwork,
		},
		{
			name:    "word-boundary guard: AuthorizedHandler → ServerError",
			err:     errors.New("AuthorizedHandler: weird thing"),
			wantSub: praxiserrors.ToolSubKindServerError,
		},
		{
			name:    "TLS handshake error → Network",
			err:     errors.New("tls: failed to verify certificate"),
			wantSub: praxiserrors.ToolSubKindNetwork,
		},
		{
			name:    "X.509 error → Network",
			err:     errors.New("x509: certificate has expired or is not yet valid"),
			wantSub: praxiserrors.ToolSubKindNetwork,
		},
		{
			name:    "json.SyntaxError → SchemaViolation",
			err:     mustJSONSyntaxError(t),
			wantSub: praxiserrors.ToolSubKindSchemaViolation,
		},
		{
			name:    "json.UnmarshalTypeError → SchemaViolation",
			err:     mustJSONUnmarshalTypeError(t),
			wantSub: praxiserrors.ToolSubKindSchemaViolation,
		},
		{
			name:    "unknown error → ServerError (default)",
			err:     errors.New("jsonrpc2: internal error: server is confused"),
			wantSub: praxiserrors.ToolSubKindServerError,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := classifyCallToolError(tc.err)
			if got != tc.wantSub {
				t.Errorf("classifyCallToolError(%q) = %q, want %q",
					tc.err, got, tc.wantSub)
			}
		})
	}
}

// TestIsNetworkErrorDirect exercises isNetworkError in isolation
// so the per-sentinel matches can be verified without the
// first-match-wins shadowing that the top-level classifier
// imposes on later branches. The classifier covers the same set
// of sentinels via its own tests; this test is an extra layer of
// defence in depth.
func TestIsNetworkErrorDirect(t *testing.T) {
	t.Parallel()

	positive := []error{
		sdkmcp.ErrConnectionClosed,
		fmt.Errorf("wrap: %w", sdkmcp.ErrConnectionClosed),
		io.EOF,
		io.ErrUnexpectedEOF,
		syscall.ECONNREFUSED,
		syscall.ECONNRESET,
		syscall.ETIMEDOUT,
		syscall.EPIPE,
		&net.OpError{Op: "write", Net: "tcp", Err: errors.New("synthetic")},
	}
	for _, err := range positive {
		if !isNetworkError(err) {
			t.Errorf("isNetworkError(%v) = false, want true", err)
		}
	}

	negative := []error{
		errors.New("random"),
		sdkmcp.ErrSessionMissing,
		&json.SyntaxError{Offset: 0},
	}
	for _, err := range negative {
		if isNetworkError(err) {
			t.Errorf("isNetworkError(%v) = true, want false", err)
		}
	}
}

// TestClassifyHTTPStatusSignalBoundaries pins the word-boundary
// behaviour of the HTTP status sniffer. "Unauthorized" inside a
// longer word must not match; "Unauthorized" at a colon/space
// boundary must match; a punctuation-bounded match must match.
func TestClassifyHTTPStatusSignalBoundaries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		msg     string
		wantSub praxiserrors.ToolSubKind
		wantOK  bool
	}{
		{"POST /mcp: Unauthorized", praxiserrors.ToolSubKindCircuitOpen, true},
		{"AuthorizedHandler panicked", "", false},
		{"Unauthorized(): stack...", praxiserrors.ToolSubKindCircuitOpen, true},
		{"POST /mcp: Forbidden", praxiserrors.ToolSubKindCircuitOpen, true},
		{"POST /mcp: Too Many Requests", praxiserrors.ToolSubKindNetwork, true},
		{"TooManyRequestsHandler blew up", "", false},
		{"just a random message", "", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.msg, func(t *testing.T) {
			t.Parallel()
			sub, ok := classifyHTTPStatusSignal(errors.New(tc.msg))
			if ok != tc.wantOK {
				t.Errorf("ok = %v, want %v", ok, tc.wantOK)
			}
			if sub != tc.wantSub {
				t.Errorf("sub = %q, want %q", sub, tc.wantSub)
			}
		})
	}
}

// TestLooksLikeTLSError verifies the TLS/X.509 prefix sniffing
// predicate against common stdlib error shapes.
func TestLooksLikeTLSError(t *testing.T) {
	t.Parallel()

	positive := []error{
		errors.New("tls: handshake failure"),
		errors.New("x509: certificate signed by unknown authority"),
		errors.New("POST: tls: bad record mac"),
		&tls.RecordHeaderError{Msg: "tls: first record does not look like a TLS handshake"},
	}
	for _, err := range positive {
		if !looksLikeTLSError(err) {
			t.Errorf("looksLikeTLSError(%q) = false, want true", err)
		}
	}

	negative := []error{
		errors.New("random error"),
		errors.New("TLS with capital letters but no prefix"),
		errors.New("status: Unauthorized"),
	}
	for _, err := range negative {
		if looksLikeTLSError(err) {
			t.Errorf("looksLikeTLSError(%q) = true, want false", err)
		}
	}
}

// TestEstimateResponseBytes verifies the MaxResponseBytes guard
// measurement primitive. Coverage:
//
//   - Empty / nil slice → 0.
//   - Text blocks counted in bytes.
//   - Image blocks counted as len(Data) (already base64-decoded).
//   - Audio blocks counted as len(Data).
//   - Mixed slice: sums each contribution.
//   - Unknown block type: ignored (returns 0 for that block).
func TestEstimateResponseBytes(t *testing.T) {
	t.Parallel()

	if got := estimateResponseBytes(nil); got != 0 {
		t.Errorf("nil slice: got %d, want 0", got)
	}
	if got := estimateResponseBytes([]sdkmcp.Content{}); got != 0 {
		t.Errorf("empty slice: got %d, want 0", got)
	}

	text := &sdkmcp.TextContent{Text: "hello"}
	if got := estimateResponseBytes([]sdkmcp.Content{text}); got != 5 {
		t.Errorf("text %q: got %d, want 5", text.Text, got)
	}

	image := &sdkmcp.ImageContent{Data: make([]byte, 1024)}
	if got := estimateResponseBytes([]sdkmcp.Content{image}); got != 1024 {
		t.Errorf("image 1024B: got %d, want 1024", got)
	}

	audio := &sdkmcp.AudioContent{Data: make([]byte, 512)}
	if got := estimateResponseBytes([]sdkmcp.Content{audio}); got != 512 {
		t.Errorf("audio 512B: got %d, want 512", got)
	}

	mixed := []sdkmcp.Content{
		&sdkmcp.TextContent{Text: "abc"}, // 3
		&sdkmcp.ImageContent{Data: make([]byte, 100)},
		&sdkmcp.AudioContent{Data: make([]byte, 200)},
		&sdkmcp.TextContent{Text: "defgh"}, // 5
	}
	if got := estimateResponseBytes(mixed); got != 308 {
		t.Errorf("mixed: got %d, want 308 (3+100+200+5)", got)
	}

	// ResourceLink is deliberately NOT counted per godoc.
	resource := &sdkmcp.ResourceLink{
		URI:         "https://example.com/really-long-uri-but-still-metadata",
		Description: "a description that would be hundreds of bytes if we counted it",
	}
	if got := estimateResponseBytes([]sdkmcp.Content{resource}); got != 0 {
		t.Errorf("resource link: got %d, want 0 (metadata not counted)", got)
	}
}

// mustJSONSyntaxError produces a real *json.SyntaxError by
// running json.Unmarshal on a malformed payload. Constructing
// the error directly via a struct literal is awkward because
// SyntaxError's unexported `msg` field is the only way the
// `.Error()` method returns useful text; letting the stdlib
// produce the error gives us a safe value with all fields
// populated consistently.
func mustJSONSyntaxError(t *testing.T) error {
	t.Helper()
	var scratch any
	err := json.Unmarshal([]byte("{not valid"), &scratch)
	if err == nil {
		t.Fatal("json.Unmarshal did not return an error")
	}
	var synErr *json.SyntaxError
	if !errors.As(err, &synErr) {
		t.Fatalf("expected *json.SyntaxError, got %T: %v", err, err)
	}
	return err
}

// mustJSONUnmarshalTypeError produces a real
// *json.UnmarshalTypeError the same way — by driving
// json.Unmarshal into a type mismatch. A struct-literal
// construction with `Type: nil` panics inside `.Error()`
// because the stdlib dereferences Type unconditionally; going
// through json.Unmarshal guarantees every field is populated.
func mustJSONUnmarshalTypeError(t *testing.T) error {
	t.Helper()
	var target int
	err := json.Unmarshal([]byte(`"not a number"`), &target)
	if err == nil {
		t.Fatal("json.Unmarshal did not return an error")
	}
	var typeErr *json.UnmarshalTypeError
	if !errors.As(err, &typeErr) {
		t.Fatalf("expected *json.UnmarshalTypeError, got %T: %v", err, err)
	}
	return err
}
