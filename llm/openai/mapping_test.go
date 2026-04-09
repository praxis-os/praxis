// SPDX-License-Identifier: Apache-2.0

package openai

import (
	"testing"

	"github.com/praxis-os/praxis/llm"
)

func TestToAssistantMessages(t *testing.T) {
	tests := []struct {
		name      string
		msg       llm.Message
		wantCount int
		wantErr   bool
		check     func(t *testing.T, msgs []apiMessage)
	}{
		{
			name: "text_only",
			msg: llm.Message{
				Role:  llm.RoleAssistant,
				Parts: []llm.MessagePart{llm.TextPart("hello world")},
			},
			wantCount: 1,
			check: func(t *testing.T, msgs []apiMessage) {
				if msgs[0].Role != "assistant" {
					t.Errorf("Role: want assistant, got %q", msgs[0].Role)
				}
				if msgs[0].Content != "hello world" {
					t.Errorf("Content: want 'hello world', got %q", msgs[0].Content)
				}
				if len(msgs[0].ToolCalls) != 0 {
					t.Errorf("ToolCalls: want 0, got %d", len(msgs[0].ToolCalls))
				}
			},
		},
		{
			name: "tool_call",
			msg: llm.Message{
				Role: llm.RoleAssistant,
				Parts: []llm.MessagePart{
					llm.ToolCallPart(&llm.LLMToolCall{
						CallID:        "c1",
						Name:          "search",
						ArgumentsJSON: []byte(`{"q":"test"}`),
					}),
				},
			},
			wantCount: 1,
			check: func(t *testing.T, msgs []apiMessage) {
				if len(msgs[0].ToolCalls) != 1 {
					t.Fatalf("ToolCalls: want 1, got %d", len(msgs[0].ToolCalls))
				}
				tc := msgs[0].ToolCalls[0]
				if tc.ID != "c1" {
					t.Errorf("ID: want c1, got %q", tc.ID)
				}
				if tc.Type != "function" {
					t.Errorf("Type: want function, got %q", tc.Type)
				}
				if tc.Function.Name != "search" {
					t.Errorf("Name: want search, got %q", tc.Function.Name)
				}
				if tc.Function.Arguments != `{"q":"test"}` {
					t.Errorf("Arguments: want {\"q\":\"test\"}, got %q", tc.Function.Arguments)
				}
			},
		},
		{
			name: "text_and_tool_calls",
			msg: llm.Message{
				Role: llm.RoleAssistant,
				Parts: []llm.MessagePart{
					llm.TextPart("I'll search for that."),
					llm.ToolCallPart(&llm.LLMToolCall{
						CallID:        "c2",
						Name:          "lookup",
						ArgumentsJSON: []byte(`{}`),
					}),
				},
			},
			wantCount: 1,
			check: func(t *testing.T, msgs []apiMessage) {
				if msgs[0].Content != "I'll search for that." {
					t.Errorf("Content: got %q", msgs[0].Content)
				}
				if len(msgs[0].ToolCalls) != 1 {
					t.Errorf("ToolCalls: want 1, got %d", len(msgs[0].ToolCalls))
				}
			},
		},
		{
			name: "nil_tool_call_error",
			msg: llm.Message{
				Role: llm.RoleAssistant,
				Parts: []llm.MessagePart{
					{Type: llm.PartTypeToolCall, ToolCall: nil},
				},
			},
			wantErr: true,
		},
		{
			name: "empty_arguments_defaults_to_braces",
			msg: llm.Message{
				Role: llm.RoleAssistant,
				Parts: []llm.MessagePart{
					llm.ToolCallPart(&llm.LLMToolCall{
						CallID:        "c3",
						Name:          "noop",
						ArgumentsJSON: nil,
					}),
				},
			},
			wantCount: 1,
			check: func(t *testing.T, msgs []apiMessage) {
				if msgs[0].ToolCalls[0].Function.Arguments != "{}" {
					t.Errorf("Arguments: want '{}', got %q", msgs[0].ToolCalls[0].Function.Arguments)
				}
			},
		},
		{
			name: "unexpected_part_type_error",
			msg: llm.Message{
				Role: llm.RoleAssistant,
				Parts: []llm.MessagePart{
					{Type: llm.PartTypeImageURL},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msgs, err := toAssistantMessages(tt.msg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error: got %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if len(msgs) != tt.wantCount {
				t.Fatalf("message count: want %d, got %d", tt.wantCount, len(msgs))
			}
			if tt.check != nil {
				tt.check(t, msgs)
			}
		})
	}
}

func TestToToolResultMessages(t *testing.T) {
	tests := []struct {
		name      string
		msg       llm.Message
		wantCount int
		wantErr   bool
		check     func(t *testing.T, msgs []apiMessage)
	}{
		{
			name: "single_result",
			msg: llm.Message{
				Role: llm.RoleTool,
				Parts: []llm.MessagePart{
					llm.ToolResultPart(&llm.LLMToolResult{CallID: "c1", Content: "result data"}),
				},
			},
			wantCount: 1,
			check: func(t *testing.T, msgs []apiMessage) {
				if msgs[0].Role != "tool" {
					t.Errorf("Role: want tool, got %q", msgs[0].Role)
				}
				if msgs[0].Content != "result data" {
					t.Errorf("Content: want 'result data', got %q", msgs[0].Content)
				}
				if msgs[0].ToolCallID != "c1" {
					t.Errorf("ToolCallID: want c1, got %q", msgs[0].ToolCallID)
				}
			},
		},
		{
			name: "multiple_results",
			msg: llm.Message{
				Role: llm.RoleTool,
				Parts: []llm.MessagePart{
					llm.ToolResultPart(&llm.LLMToolResult{CallID: "c1", Content: "a"}),
					llm.ToolResultPart(&llm.LLMToolResult{CallID: "c2", Content: "b"}),
					llm.ToolResultPart(&llm.LLMToolResult{CallID: "c3", Content: "c"}),
				},
			},
			wantCount: 3,
			check: func(t *testing.T, msgs []apiMessage) {
				for i, want := range []string{"c1", "c2", "c3"} {
					if msgs[i].ToolCallID != want {
						t.Errorf("msgs[%d].ToolCallID: want %q, got %q", i, want, msgs[i].ToolCallID)
					}
				}
			},
		},
		{
			name: "nil_tool_result_error",
			msg: llm.Message{
				Role: llm.RoleTool,
				Parts: []llm.MessagePart{
					{Type: llm.PartTypeToolResult, ToolResult: nil},
				},
			},
			wantErr: true,
		},
		{
			name: "wrong_part_type_error",
			msg: llm.Message{
				Role: llm.RoleTool,
				Parts: []llm.MessagePart{
					{Type: llm.PartTypeText, Text: "unexpected"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msgs, err := toToolResultMessages(tt.msg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error: got %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if len(msgs) != tt.wantCount {
				t.Fatalf("message count: want %d, got %d", tt.wantCount, len(msgs))
			}
			if tt.check != nil {
				tt.check(t, msgs)
			}
		})
	}
}

func TestFromAPIFinishReason(t *testing.T) {
	tests := []struct {
		reason string
		want   llm.StopReason
	}{
		{"stop", llm.StopReasonEndTurn},
		{"tool_calls", llm.StopReasonToolUse},
		{"length", llm.StopReasonMaxTokens},
		{"content_filter", llm.StopReasonEndTurn},
		{"unknown_reason", llm.StopReasonEndTurn},
		{"", llm.StopReasonEndTurn},
	}

	for _, tt := range tests {
		t.Run(tt.reason, func(t *testing.T) {
			got := fromAPIFinishReason(tt.reason)
			if got != tt.want {
				t.Errorf("fromAPIFinishReason(%q): want %v, got %v", tt.reason, tt.want, got)
			}
		})
	}
}

func TestToAPIMessages_Dispatch(t *testing.T) {
	tests := []struct {
		name    string
		msg     llm.Message
		wantErr bool
		check   func(t *testing.T, msgs []apiMessage)
	}{
		{
			name: "user_role",
			msg: llm.Message{
				Role:  llm.RoleUser,
				Parts: []llm.MessagePart{llm.TextPart("hello")},
			},
			check: func(t *testing.T, msgs []apiMessage) {
				if msgs[0].Role != "user" {
					t.Errorf("Role: want user, got %q", msgs[0].Role)
				}
			},
		},
		{
			name: "assistant_role",
			msg: llm.Message{
				Role:  llm.RoleAssistant,
				Parts: []llm.MessagePart{llm.TextPart("hi")},
			},
			check: func(t *testing.T, msgs []apiMessage) {
				if msgs[0].Role != "assistant" {
					t.Errorf("Role: want assistant, got %q", msgs[0].Role)
				}
			},
		},
		{
			name: "tool_role",
			msg: llm.Message{
				Role: llm.RoleTool,
				Parts: []llm.MessagePart{
					llm.ToolResultPart(&llm.LLMToolResult{CallID: "c1", Content: "ok"}),
				},
			},
			check: func(t *testing.T, msgs []apiMessage) {
				if msgs[0].Role != "tool" {
					t.Errorf("Role: want tool, got %q", msgs[0].Role)
				}
			},
		},
		{
			name: "unsupported_role",
			msg: llm.Message{
				Role:  llm.RoleSystem,
				Parts: []llm.MessagePart{llm.TextPart("sys")},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msgs, err := toAPIMessages(tt.msg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error: got %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if tt.check != nil {
				tt.check(t, msgs)
			}
		})
	}
}

func TestToAPIRequest_Variants(t *testing.T) {
	t.Run("default_model_fallback", func(t *testing.T) {
		req := llm.LLMRequest{
			Model:    "",
			Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}}},
		}
		ar, err := toAPIRequest(req, "gpt-4o-mini")
		if err != nil {
			t.Fatalf("toAPIRequest: %v", err)
		}
		if ar.Model != "gpt-4o-mini" {
			t.Errorf("Model: want gpt-4o-mini, got %q", ar.Model)
		}
	})

	t.Run("explicit_model_takes_precedence", func(t *testing.T) {
		req := llm.LLMRequest{
			Model:    "gpt-4",
			Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}}},
		}
		ar, err := toAPIRequest(req, "gpt-4o-mini")
		if err != nil {
			t.Fatalf("toAPIRequest: %v", err)
		}
		if ar.Model != "gpt-4" {
			t.Errorf("Model: want gpt-4, got %q", ar.Model)
		}
	})

	t.Run("system_prompt_prepended", func(t *testing.T) {
		req := llm.LLMRequest{
			Model:        "m",
			SystemPrompt: "You are helpful.",
			Messages:     []llm.Message{{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}}},
		}
		ar, err := toAPIRequest(req, "")
		if err != nil {
			t.Fatalf("toAPIRequest: %v", err)
		}
		if len(ar.Messages) < 2 {
			t.Fatalf("Messages: want >=2, got %d", len(ar.Messages))
		}
		if ar.Messages[0].Role != "system" || ar.Messages[0].Content != "You are helpful." {
			t.Errorf("first message: want system/'You are helpful.', got %q/%q", ar.Messages[0].Role, ar.Messages[0].Content)
		}
	})

	t.Run("tool_with_empty_schema", func(t *testing.T) {
		req := llm.LLMRequest{
			Model:    "m",
			Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}}},
			Tools: []llm.ToolDefinition{{
				Name:        "noop",
				Description: "does nothing",
				InputSchema: nil,
			}},
		}
		ar, err := toAPIRequest(req, "")
		if err != nil {
			t.Fatalf("toAPIRequest: %v", err)
		}
		if len(ar.Tools) != 1 {
			t.Fatalf("Tools: want 1, got %d", len(ar.Tools))
		}
		if string(ar.Tools[0].Function.Parameters) != `{"type":"object","properties":{}}` {
			t.Errorf("Parameters: want minimal schema, got %q", string(ar.Tools[0].Function.Parameters))
		}
	})

	t.Run("system_role_skipped_in_history", func(t *testing.T) {
		req := llm.LLMRequest{
			Model: "m",
			Messages: []llm.Message{
				{Role: llm.RoleSystem, Parts: []llm.MessagePart{llm.TextPart("sys")}},
				{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}},
			},
		}
		ar, err := toAPIRequest(req, "")
		if err != nil {
			t.Fatalf("toAPIRequest: %v", err)
		}
		// System-role messages should be skipped; only user message present.
		if len(ar.Messages) != 1 {
			t.Fatalf("Messages: want 1, got %d", len(ar.Messages))
		}
		if ar.Messages[0].Role != "user" {
			t.Errorf("Message role: want user, got %q", ar.Messages[0].Role)
		}
	})

	t.Run("temperature_set", func(t *testing.T) {
		req := llm.LLMRequest{
			Model:       "m",
			Temperature: 0.7,
			Messages:    []llm.Message{{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}}},
		}
		ar, err := toAPIRequest(req, "")
		if err != nil {
			t.Fatalf("toAPIRequest: %v", err)
		}
		if ar.Temperature == nil || *ar.Temperature != 0.7 {
			t.Errorf("Temperature: want 0.7, got %v", ar.Temperature)
		}
	})

	t.Run("temperature_zero_omitted", func(t *testing.T) {
		req := llm.LLMRequest{
			Model:       "m",
			Temperature: 0,
			Messages:    []llm.Message{{Role: llm.RoleUser, Parts: []llm.MessagePart{llm.TextPart("hi")}}},
		}
		ar, err := toAPIRequest(req, "")
		if err != nil {
			t.Fatalf("toAPIRequest: %v", err)
		}
		if ar.Temperature != nil {
			t.Errorf("Temperature: want nil, got %v", *ar.Temperature)
		}
	})
}
