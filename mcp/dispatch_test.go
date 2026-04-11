// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	praxiserrors "github.com/praxis-os/praxis/errors"
	"github.com/praxis-os/praxis/tools"
)

// TestInvokeHappyPathEndToEnd drives a real tool call through the
// full S32 dispatch path: New opens an in-memory session with a
// handler-bearing tool, Invoke decodes the composed name, routes
// to the session, calls sdkmcp.ClientSession.CallTool, and
// flattens the text-content response. The test asserts that the
// returned ToolResult carries the handler's emitted text as
// Content and tools.ToolStatusSuccess as Status.
func TestInvokeHappyPathEndToEnd(t *testing.T) {
	t.Parallel()

	specs := []serverSpec{{
		logicalName: "alpha",
		tools: []toolSpec{{
			name:        "echo",
			description: "echo the input",
			handler: func(_ context.Context, _ *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
				return &sdkmcp.CallToolResult{
					Content: []sdkmcp.Content{
						&sdkmcp.TextContent{Text: "line one"},
						&sdkmcp.TextContent{Text: "line two"},
					},
				}, nil
			},
		}},
	}}

	inv, err := New(context.Background(),
		[]Server{validServer("alpha")},
		withSessionOpener(openSessionsWithTools(specs)),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = inv.Close() }()

	args, _ := json.Marshal(map[string]any{"msg": "hi"})
	result, frameworkErr := inv.Invoke(context.Background(), tools.InvocationContext{}, tools.ToolCall{
		CallID:        "c-success",
		Name:          "alpha__echo",
		ArgumentsJSON: args,
	})
	if frameworkErr != nil {
		t.Fatalf("Invoke framework error: %v", frameworkErr)
	}
	if result.Status != tools.ToolStatusSuccess {
		t.Errorf("status = %q, want %q; ToolResult.Err = %v", result.Status, tools.ToolStatusSuccess, result.Err)
	}
	// D114: text blocks joined with "\n\n" (paragraph break), not
	// single "\n". See flattenTextContent godoc.
	want := "line one\n\nline two"
	if result.Content != want {
		t.Errorf("Content = %q, want %q", result.Content, want)
	}
	if result.CallID != "c-success" {
		t.Errorf("CallID = %q, want %q", result.CallID, "c-success")
	}
	if result.Err != nil {
		t.Errorf("Err = %v, want nil on success", result.Err)
	}
}

// TestInvokeServerIsErrorRoutedAsToolError asserts the S32
// placeholder translation: when the MCP server returns
// IsError=true, the adapter yields a ToolStatusError + a typed
// ToolError (ToolSubKindServerError) + the flattened text
// content. This matches the MCP spec's "errors go in the Content
// field with IsError=true" contract and survives the S33 taxonomy
// rewrite (the flattening path stays, the error classification
// narrows).
func TestInvokeServerIsErrorRoutedAsToolError(t *testing.T) {
	t.Parallel()

	specs := []serverSpec{{
		logicalName: "alpha",
		tools: []toolSpec{{
			name:        "broken",
			description: "always errors",
			handler: func(_ context.Context, _ *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
				return &sdkmcp.CallToolResult{
					Content: []sdkmcp.Content{
						&sdkmcp.TextContent{Text: "tool-level failure payload"},
					},
					IsError: true,
				}, nil
			},
		}},
	}}

	inv, err := New(context.Background(),
		[]Server{validServer("alpha")},
		withSessionOpener(openSessionsWithTools(specs)),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = inv.Close() }()

	result, frameworkErr := inv.Invoke(context.Background(), tools.InvocationContext{}, tools.ToolCall{
		CallID: "c-iserror",
		Name:   "alpha__broken",
	})
	if frameworkErr != nil {
		t.Fatalf("Invoke framework error: %v", frameworkErr)
	}
	if result.Status != tools.ToolStatusError {
		t.Errorf("status = %q, want %q", result.Status, tools.ToolStatusError)
	}
	if result.Content != "tool-level failure payload" {
		t.Errorf("Content = %q, want the flattened error text", result.Content)
	}
	assertToolError(t, result.Err, "mcp server reported tool error")
}

// TestInvokeDropsNonTextContent asserts the S32 minimal flattening
// contract: non-text content blocks (image, audio, resource) are
// silently dropped, and the remaining text blocks are joined with
// '\n'. A tool that returns only non-text blocks yields a
// successful ToolResult with Content == "".
func TestInvokeDropsNonTextContent(t *testing.T) {
	t.Parallel()

	specs := []serverSpec{{
		logicalName: "alpha",
		tools: []toolSpec{
			{
				name: "mixed",
				handler: func(_ context.Context, _ *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
					return &sdkmcp.CallToolResult{
						Content: []sdkmcp.Content{
							&sdkmcp.TextContent{Text: "kept"},
							&sdkmcp.ImageContent{Data: []byte{1, 2, 3}, MIMEType: "image/png"},
							&sdkmcp.TextContent{Text: "also kept"},
						},
					}, nil
				},
			},
			{
				name: "only_image",
				handler: func(_ context.Context, _ *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
					return &sdkmcp.CallToolResult{
						Content: []sdkmcp.Content{
							&sdkmcp.ImageContent{Data: []byte{9}, MIMEType: "image/png"},
						},
					}, nil
				},
			},
		},
	}}

	inv, err := New(context.Background(),
		[]Server{validServer("alpha")},
		withSessionOpener(openSessionsWithTools(specs)),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = inv.Close() }()

	// Mixed tool: text blocks preserved, image dropped.
	mixed, frameworkErr := inv.Invoke(context.Background(), tools.InvocationContext{}, tools.ToolCall{
		CallID: "c-mixed",
		Name:   "alpha__mixed",
	})
	if frameworkErr != nil {
		t.Fatalf("mixed: framework error: %v", frameworkErr)
	}
	if mixed.Status != tools.ToolStatusSuccess {
		t.Errorf("mixed status = %q, want %q; Err=%v", mixed.Status, tools.ToolStatusSuccess, mixed.Err)
	}
	// D114: text blocks joined with "\n\n"; the dropped image
	// block does not introduce an extra separator.
	if mixed.Content != "kept\n\nalso kept" {
		t.Errorf("mixed Content = %q, want %q", mixed.Content, "kept\n\nalso kept")
	}

	// Only-image tool: empty content, still a success.
	only, frameworkErr := inv.Invoke(context.Background(), tools.InvocationContext{}, tools.ToolCall{
		CallID: "c-only-image",
		Name:   "alpha__only_image",
	})
	if frameworkErr != nil {
		t.Fatalf("only_image: framework error: %v", frameworkErr)
	}
	if only.Status != tools.ToolStatusSuccess {
		t.Errorf("only_image status = %q, want %q; Err=%v", only.Status, tools.ToolStatusSuccess, only.Err)
	}
	if only.Content != "" {
		t.Errorf("only_image Content = %q, want empty string", only.Content)
	}
}

// TestInvokeInvalidArgumentsJSONReturnsSchemaViolation asserts
// that malformed ArgumentsJSON surfaces as a typed ToolError with
// ToolSubKindSchemaViolation, without a framework error. The
// tool handler is never reached because json.Unmarshal fails at
// the adapter boundary.
func TestInvokeInvalidArgumentsJSONReturnsSchemaViolation(t *testing.T) {
	t.Parallel()

	specs := []serverSpec{{
		logicalName: "alpha",
		tools:       []toolSpec{{name: "probe"}},
	}}
	inv, err := New(context.Background(),
		[]Server{validServer("alpha")},
		withSessionOpener(openSessionsWithTools(specs)),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = inv.Close() }()

	result, frameworkErr := inv.Invoke(context.Background(), tools.InvocationContext{}, tools.ToolCall{
		CallID:        "c-badjson",
		Name:          "alpha__probe",
		ArgumentsJSON: []byte("{not valid json"),
	})
	if frameworkErr != nil {
		t.Fatalf("framework error: %v", frameworkErr)
	}
	if result.Status != tools.ToolStatusError {
		t.Errorf("status = %q, want %q", result.Status, tools.ToolStatusError)
	}
	if result.Err == nil {
		t.Fatal("Err is nil; bad JSON should produce a typed tool error")
	}
	var toolErr *praxiserrors.ToolError
	if !errors.As(result.Err, &toolErr) {
		t.Fatalf("Err is not a *ToolError: %T: %v", result.Err, result.Err)
	}
	if toolErr.SubKind != praxiserrors.ToolSubKindSchemaViolation {
		t.Errorf("SubKind = %q, want %q", toolErr.SubKind, praxiserrors.ToolSubKindSchemaViolation)
	}
}

// TestInvokeDropsAudioAndResource extends S32's image-only drop
// coverage to audio and resource-link blocks. D114 §decision
// lists image, audio, and resource as the three non-text
// variants to discard; S33's godoc formalises this contract and
// the test locks it in.
func TestInvokeDropsAudioAndResource(t *testing.T) {
	t.Parallel()

	specs := []serverSpec{{
		logicalName: "alpha",
		tools: []toolSpec{{
			name: "mixed_av_resource",
			handler: func(_ context.Context, _ *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
				return &sdkmcp.CallToolResult{
					Content: []sdkmcp.Content{
						&sdkmcp.TextContent{Text: "first"},
						&sdkmcp.AudioContent{Data: []byte{0x10, 0x20}, MIMEType: "audio/mpeg"},
						&sdkmcp.TextContent{Text: "second"},
						&sdkmcp.ResourceLink{URI: "https://example.com/r", Name: "r"},
						&sdkmcp.TextContent{Text: "third"},
					},
				}, nil
			},
		}},
	}}

	inv, err := New(context.Background(),
		[]Server{validServer("alpha")},
		withSessionOpener(openSessionsWithTools(specs)),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = inv.Close() }()

	result, frameworkErr := inv.Invoke(context.Background(), tools.InvocationContext{}, tools.ToolCall{
		CallID: "c-mixed-av",
		Name:   "alpha__mixed_av_resource",
	})
	if frameworkErr != nil {
		t.Fatalf("framework error: %v", frameworkErr)
	}
	if result.Status != tools.ToolStatusSuccess {
		t.Errorf("status = %q, want %q; Err=%v", result.Status, tools.ToolStatusSuccess, result.Err)
	}
	// Only text blocks survive; audio and resource-link are
	// dropped. Server-side order preserved.
	want := "first\n\nsecond\n\nthird"
	if result.Content != want {
		t.Errorf("Content = %q, want %q", result.Content, want)
	}
}

// TestInvokeOnlyNonTextYieldsEmptySuccess asserts the D114
// amendment 2026-04-10 "empty content is a valid success"
// contract in isolation. The tool returns only an audio block
// (a response shape the adapter treats as "successful but
// text-free"). The adapter must yield
// Status=Success && Content=="" so that PostToolFilter
// implementors following the amendment contract see it as a
// legitimate outcome.
func TestInvokeOnlyNonTextYieldsEmptySuccess(t *testing.T) {
	t.Parallel()

	specs := []serverSpec{{
		logicalName: "alpha",
		tools: []toolSpec{{
			name: "audio_only",
			handler: func(_ context.Context, _ *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
				return &sdkmcp.CallToolResult{
					Content: []sdkmcp.Content{
						&sdkmcp.AudioContent{Data: []byte{0x01, 0x02, 0x03}, MIMEType: "audio/wav"},
					},
				}, nil
			},
		}},
	}}

	inv, err := New(context.Background(),
		[]Server{validServer("alpha")},
		withSessionOpener(openSessionsWithTools(specs)),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = inv.Close() }()

	result, frameworkErr := inv.Invoke(context.Background(), tools.InvocationContext{}, tools.ToolCall{
		CallID: "c-audio-only",
		Name:   "alpha__audio_only",
	})
	if frameworkErr != nil {
		t.Fatalf("framework error: %v", frameworkErr)
	}
	if result.Status != tools.ToolStatusSuccess {
		t.Errorf("status = %q, want %q; Err=%v", result.Status, tools.ToolStatusSuccess, result.Err)
	}
	if result.Content != "" {
		t.Errorf("Content = %q, want empty string (D114 amendment 2026-04-10)", result.Content)
	}
	if result.Err != nil {
		t.Errorf("Err = %v, want nil on text-free success", result.Err)
	}
}

// TestInvokeMaxResponseBytesRejects exercises T33.7: a response
// whose estimated byte footprint exceeds the caller-configured
// cap is rejected with ErrorKindTool/ToolSubKindServerError. The
// error message carries the actual vs. configured counts so
// operators can triage with exit_status alone.
func TestInvokeMaxResponseBytesRejects(t *testing.T) {
	t.Parallel()

	// Return a text block large enough to exceed a 1 KiB cap.
	largeText := make([]byte, 2048)
	for i := range largeText {
		largeText[i] = 'x'
	}
	specs := []serverSpec{{
		logicalName: "alpha",
		tools: []toolSpec{{
			name: "huge",
			handler: func(_ context.Context, _ *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
				return &sdkmcp.CallToolResult{
					Content: []sdkmcp.Content{
						&sdkmcp.TextContent{Text: string(largeText)},
					},
				}, nil
			},
		}},
	}}

	inv, err := New(context.Background(),
		[]Server{validServer("alpha")},
		withSessionOpener(openSessionsWithTools(specs)),
		WithMaxResponseBytes(1024),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = inv.Close() }()

	result, frameworkErr := inv.Invoke(context.Background(), tools.InvocationContext{}, tools.ToolCall{
		CallID: "c-huge",
		Name:   "alpha__huge",
	})
	if frameworkErr != nil {
		t.Fatalf("framework error: %v", frameworkErr)
	}
	if result.Status != tools.ToolStatusError {
		t.Errorf("status = %q, want %q", result.Status, tools.ToolStatusError)
	}
	if result.Content != "" {
		t.Errorf("Content = %q, want empty (payload rejected)", result.Content)
	}
	// Assert the ServerError classification and that the message
	// carries both actual and cap counts.
	assertToolError(t, result.Err, "MaxResponseBytes", "actual=2048", "cap=1024")
}

// TestInvokeMaxResponseBytesAllowsUnderCap is the positive
// counterpart of TestInvokeMaxResponseBytesRejects: a response
// whose estimated footprint is below the configured cap flows
// through the happy path unmodified.
func TestInvokeMaxResponseBytesAllowsUnderCap(t *testing.T) {
	t.Parallel()

	specs := []serverSpec{{
		logicalName: "alpha",
		tools: []toolSpec{{
			name: "small",
			handler: func(_ context.Context, _ *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
				return &sdkmcp.CallToolResult{
					Content: []sdkmcp.Content{
						&sdkmcp.TextContent{Text: "tiny"},
					},
				}, nil
			},
		}},
	}}

	inv, err := New(context.Background(),
		[]Server{validServer("alpha")},
		withSessionOpener(openSessionsWithTools(specs)),
		WithMaxResponseBytes(1024),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = inv.Close() }()

	result, frameworkErr := inv.Invoke(context.Background(), tools.InvocationContext{}, tools.ToolCall{
		CallID: "c-small",
		Name:   "alpha__small",
	})
	if frameworkErr != nil {
		t.Fatalf("framework error: %v", frameworkErr)
	}
	if result.Status != tools.ToolStatusSuccess {
		t.Errorf("status = %q, want %q; Err=%v", result.Status, tools.ToolStatusSuccess, result.Err)
	}
	if result.Content != "tiny" {
		t.Errorf("Content = %q, want %q", result.Content, "tiny")
	}
}

// TestInvokePostCloseReturnsFrameworkError asserts that dispatching
// after Close observes the closed flag and returns a framework
// error, not a ToolResult-only error. Broken-invoker signalling is
// the orchestrator's cue to stop routing further calls.
func TestInvokePostCloseReturnsFrameworkError(t *testing.T) {
	t.Parallel()

	specs := []serverSpec{{
		logicalName: "alpha",
		tools:       []toolSpec{{name: "probe"}},
	}}
	inv, err := New(context.Background(),
		[]Server{validServer("alpha")},
		withSessionOpener(openSessionsWithTools(specs)),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := inv.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	_, frameworkErr := inv.Invoke(context.Background(), tools.InvocationContext{}, tools.ToolCall{
		CallID: "c-postclose",
		Name:   "alpha__probe",
	})
	if frameworkErr == nil {
		t.Fatal("expected framework error on post-Close Invoke, got nil")
	}
	assertSystemError(t, frameworkErr, "closed")
}
