// SPDX-License-Identifier: Apache-2.0

package openai

import (
	"encoding/json"
	"fmt"

	"github.com/praxis-os/praxis/llm"
)

// apiRequest is the JSON body sent to POST /v1/chat/completions.
type apiRequest struct {
	Model       string       `json:"model"`
	Messages    []apiMessage `json:"messages"`
	Tools       []apiTool    `json:"tools,omitempty"`
	MaxTokens   int          `json:"max_tokens,omitempty"`
	Temperature *float64     `json:"temperature,omitempty"`
}

// apiMessage is a single turn in the OpenAI messages array.
//
// The Role field determines which optional fields are populated:
//   - "system"    — Content only.
//   - "user"      — Content only (may be string or array; always string here).
//   - "assistant" — Content and/or ToolCalls.
//   - "tool"      — Content and ToolCallID.
type apiMessage struct {
	Role       string        `json:"role"`
	Content    string        `json:"content,omitempty"`
	ToolCalls  []apiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
}

// apiToolCall is a tool invocation block returned by the assistant.
type apiToolCall struct {
	ID       string              `json:"id"`
	Type     string              `json:"type"` // always "function"
	Function apiToolCallFunction `json:"function"`
}

// apiToolCallFunction carries the name and JSON-encoded arguments for a tool call.
type apiToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// apiTool describes a tool available to the model.
type apiTool struct {
	Type     string      `json:"type"` // always "function"
	Function apiFunction `json:"function"`
}

// apiFunction is the function specification within an [apiTool].
type apiFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"`
}

// apiResponse is the JSON body returned by POST /v1/chat/completions.
type apiResponse struct {
	ID      string      `json:"id"`
	Object  string      `json:"object"`
	Model   string      `json:"model"`
	Choices []apiChoice `json:"choices"`
	Usage   apiUsage    `json:"usage"`
}

// apiChoice is one completion candidate in the response.
// praxis always uses n=1 so exactly one choice is expected.
type apiChoice struct {
	Index        int        `json:"index"`
	Message      apiMessage `json:"message"`
	FinishReason string     `json:"finish_reason"`
}

// apiUsage carries token consumption from the OpenAI response.
type apiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// apiError is the JSON body returned by the OpenAI API on 4xx/5xx responses.
type apiError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    any    `json:"code"` // string or null
	} `json:"error"`
}

// toAPIRequest converts an [llm.LLMRequest] into an [apiRequest] ready for
// JSON serialisation and dispatch to the OpenAI Chat Completions API.
func toAPIRequest(req llm.LLMRequest, defaultModel string) (apiRequest, error) {
	model := req.Model
	if model == "" {
		model = defaultModel
	}

	ar := apiRequest{
		Model:     model,
		MaxTokens: req.MaxTokens,
	}

	if req.Temperature != 0 {
		t := req.Temperature
		ar.Temperature = &t
	}

	// Convert tools.
	for _, td := range req.Tools {
		schema := json.RawMessage(td.InputSchema)
		if len(schema) == 0 {
			// Provide a minimal empty object schema so the API accepts the tool.
			schema = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		ar.Tools = append(ar.Tools, apiTool{
			Type: "function",
			Function: apiFunction{
				Name:        td.Name,
				Description: td.Description,
				Parameters:  schema,
			},
		})
	}

	// Prepend system prompt as a dedicated system message when present.
	if req.SystemPrompt != "" {
		ar.Messages = append(ar.Messages, apiMessage{
			Role:    "system",
			Content: req.SystemPrompt,
		})
	}

	// Convert conversation messages.
	for _, msg := range req.Messages {
		if msg.Role == llm.RoleSystem {
			// System messages are handled via the top-level system prompt above;
			// skip any system-role messages in the history to avoid duplication.
			continue
		}

		msgs, err := toAPIMessages(msg)
		if err != nil {
			return apiRequest{}, err
		}
		ar.Messages = append(ar.Messages, msgs...)
	}

	return ar, nil
}

// toAPIMessages converts a single [llm.Message] into one or more [apiMessage]
// values. Tool-result parts each require a separate "tool" role message in the
// OpenAI protocol, so a single llm.Message may expand to multiple API messages.
func toAPIMessages(msg llm.Message) ([]apiMessage, error) {
	switch msg.Role {
	case llm.RoleUser:
		return toUserMessages(msg)
	case llm.RoleAssistant:
		return toAssistantMessages(msg)
	case llm.RoleTool:
		return toToolResultMessages(msg)
	default:
		return nil, fmt.Errorf("openai: unsupported role %q", msg.Role)
	}
}

// toUserMessages converts a user-role [llm.Message] into a single "user"
// [apiMessage] by concatenating all text parts.
func toUserMessages(msg llm.Message) ([]apiMessage, error) {
	var text string
	for _, part := range msg.Parts {
		switch part.Type {
		case llm.PartTypeText:
			text += part.Text
		case llm.PartTypeImageURL:
			return nil, fmt.Errorf("openai: PartTypeImageURL is not supported")
		default:
			return nil, fmt.Errorf("openai: unexpected part type %q in user message", part.Type)
		}
	}
	return []apiMessage{{Role: "user", Content: text}}, nil
}

// toAssistantMessages converts an assistant-role [llm.Message] into a single
// "assistant" [apiMessage] carrying optional text content and tool calls.
func toAssistantMessages(msg llm.Message) ([]apiMessage, error) {
	am := apiMessage{Role: "assistant"}
	for _, part := range msg.Parts {
		switch part.Type {
		case llm.PartTypeText:
			am.Content += part.Text
		case llm.PartTypeToolCall:
			if part.ToolCall == nil {
				return nil, fmt.Errorf("openai: PartTypeToolCall with nil ToolCall")
			}
			args := string(part.ToolCall.ArgumentsJSON)
			if args == "" {
				args = "{}"
			}
			am.ToolCalls = append(am.ToolCalls, apiToolCall{
				ID:   part.ToolCall.CallID,
				Type: "function",
				Function: apiToolCallFunction{
					Name:      part.ToolCall.Name,
					Arguments: args,
				},
			})
		default:
			return nil, fmt.Errorf("openai: unexpected part type %q in assistant message", part.Type)
		}
	}
	return []apiMessage{am}, nil
}

// toToolResultMessages converts a tool-role [llm.Message] into one "tool"
// [apiMessage] per ToolResult part. OpenAI requires one tool message per
// tool_call_id.
func toToolResultMessages(msg llm.Message) ([]apiMessage, error) {
	var out []apiMessage
	for _, part := range msg.Parts {
		if part.Type != llm.PartTypeToolResult {
			return nil, fmt.Errorf("openai: unexpected part type %q in tool message", part.Type)
		}
		if part.ToolResult == nil {
			return nil, fmt.Errorf("openai: PartTypeToolResult with nil ToolResult")
		}
		out = append(out, apiMessage{
			Role:       "tool",
			Content:    part.ToolResult.Content,
			ToolCallID: part.ToolResult.CallID,
		})
	}
	return out, nil
}

// fromAPIResponse converts an [apiResponse] into an [llm.LLMResponse].
// It uses the first choice when the response contains multiple (n>1 is not
// used by praxis, but be defensive).
func fromAPIResponse(ar apiResponse) llm.LLMResponse {
	msg := llm.Message{Role: llm.RoleAssistant}

	if len(ar.Choices) > 0 {
		choice := ar.Choices[0]
		am := choice.Message

		if am.Content != "" {
			msg.Parts = append(msg.Parts, llm.TextPart(am.Content))
		}

		for i := range am.ToolCalls {
			tc := &am.ToolCalls[i]
			msg.Parts = append(msg.Parts, llm.ToolCallPart(&llm.LLMToolCall{
				CallID:        tc.ID,
				Name:          tc.Function.Name,
				ArgumentsJSON: []byte(tc.Function.Arguments),
			}))
		}
	}

	var stopReason llm.StopReason
	if len(ar.Choices) > 0 {
		stopReason = fromAPIFinishReason(ar.Choices[0].FinishReason)
	}

	return llm.LLMResponse{
		Message:    msg,
		StopReason: stopReason,
		Usage: llm.TokenUsage{
			InputTokens:  int64(ar.Usage.PromptTokens),
			OutputTokens: int64(ar.Usage.CompletionTokens),
			// OpenAI does not report cached input tokens in the base usage object.
		},
	}
}

// fromAPIFinishReason maps an OpenAI finish_reason string to [llm.StopReason].
func fromAPIFinishReason(reason string) llm.StopReason {
	switch reason {
	case "stop":
		return llm.StopReasonEndTurn
	case "tool_calls":
		return llm.StopReasonToolUse
	case "length":
		return llm.StopReasonMaxTokens
	case "content_filter":
		// Content filtered responses are treated as a permanent stop;
		// map to EndTurn so callers receive a completed (if empty) response.
		return llm.StopReasonEndTurn
	default:
		// Forward-compatible: unknown reasons map to EndTurn.
		return llm.StopReasonEndTurn
	}
}
