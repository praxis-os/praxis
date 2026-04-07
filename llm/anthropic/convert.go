// SPDX-License-Identifier: Apache-2.0

package anthropic

import (
	"encoding/json"
	"fmt"

	"github.com/praxis-os/praxis/llm"
)

// toAPIRequest converts an [llm.LLMRequest] into an [apiRequest] ready for
// JSON serialisation and dispatch to the Anthropic Messages API.
func toAPIRequest(req llm.LLMRequest, defaultModel string, defaultMaxTokens int) (apiRequest, error) {
	model := req.Model
	if model == "" {
		model = defaultModel
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = defaultMaxTokens
	}

	ar := apiRequest{
		Model:     model,
		MaxTokens: maxTokens,
		System:    req.SystemPrompt,
	}

	if req.Temperature != 0 {
		t := req.Temperature
		ar.Temperature = &t
	}

	// Convert tools.
	for _, td := range req.Tools {
		schema := td.InputSchema
		if len(schema) == 0 {
			// Provide a minimal empty object schema so the API accepts the tool.
			schema = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		ar.Tools = append(ar.Tools, apiTool{
			Name:        td.Name,
			Description: td.Description,
			InputSchema: schema,
		})
	}

	// Convert messages, skipping RoleSystem (handled via the top-level system field).
	for _, msg := range req.Messages {
		if msg.Role == llm.RoleSystem {
			continue
		}

		am, err := toAPIMessage(msg)
		if err != nil {
			return apiRequest{}, err
		}
		ar.Messages = append(ar.Messages, am)
	}

	return ar, nil
}

// toAPIMessage converts a single [llm.Message] to an [apiMessage].
func toAPIMessage(msg llm.Message) (apiMessage, error) {
	role, err := toAPIRole(msg.Role)
	if err != nil {
		return apiMessage{}, err
	}

	am := apiMessage{Role: role}
	for _, part := range msg.Parts {
		content, err := toAPIContent(part)
		if err != nil {
			return apiMessage{}, err
		}
		am.Content = append(am.Content, content)
	}
	return am, nil
}

// toAPIRole maps an [llm.Role] to the Anthropic API role string.
func toAPIRole(role llm.Role) (string, error) {
	switch role {
	case llm.RoleUser, llm.RoleTool:
		// Tool results are sent as "user" messages in the Anthropic protocol.
		return "user", nil
	case llm.RoleAssistant:
		return "assistant", nil
	default:
		return "", fmt.Errorf("anthropic: unsupported role %q", role)
	}
}

// toAPIContent converts a single [llm.MessagePart] to an [apiContent] block.
func toAPIContent(part llm.MessagePart) (apiContent, error) {
	switch part.Type {
	case llm.PartTypeText:
		return apiContent{Type: "text", Text: part.Text}, nil

	case llm.PartTypeToolCall:
		if part.ToolCall == nil {
			return apiContent{}, fmt.Errorf("anthropic: PartTypeToolCall with nil ToolCall")
		}
		input := json.RawMessage(part.ToolCall.ArgumentsJSON)
		if len(input) == 0 {
			input = json.RawMessage(`{}`)
		}
		return apiContent{
			Type:  "tool_use",
			ID:    part.ToolCall.CallID,
			Name:  part.ToolCall.Name,
			Input: input,
		}, nil

	case llm.PartTypeToolResult:
		if part.ToolResult == nil {
			return apiContent{}, fmt.Errorf("anthropic: PartTypeToolResult with nil ToolResult")
		}
		return apiContent{
			Type:              "tool_result",
			ToolUseID:         part.ToolResult.CallID,
			IsError:           part.ToolResult.IsError,
			ToolResultContent: part.ToolResult.Content,
		}, nil

	case llm.PartTypeImageURL:
		// Image URL content is not yet supported in this adapter.
		return apiContent{}, fmt.Errorf("anthropic: PartTypeImageURL is not supported")

	default:
		return apiContent{}, fmt.Errorf("anthropic: unknown part type %q", part.Type)
	}
}

// fromAPIResponse converts an [apiResponse] into an [llm.LLMResponse].
func fromAPIResponse(ar apiResponse) llm.LLMResponse {
	msg := llm.Message{Role: llm.RoleAssistant}

	for _, block := range ar.Content {
		switch block.Type {
		case "text":
			msg.Parts = append(msg.Parts, llm.TextPart(block.Text))
		case "tool_use":
			msg.Parts = append(msg.Parts, llm.ToolCallPart(&llm.LLMToolCall{
				CallID:        block.ID,
				Name:          block.Name,
				ArgumentsJSON: []byte(block.Input),
			}))
		}
		// Unknown block types are silently ignored to be forward-compatible.
	}

	return llm.LLMResponse{
		Message:    msg,
		StopReason: fromAPIStopReason(ar.StopReason),
		Usage: llm.TokenUsage{
			InputTokens:       int64(ar.Usage.InputTokens),
			OutputTokens:      int64(ar.Usage.OutputTokens),
			CachedInputTokens: int64(ar.Usage.CacheReadInputTokens),
		},
	}
}

// fromAPIStopReason maps an Anthropic stop_reason string to [llm.StopReason].
func fromAPIStopReason(reason string) llm.StopReason {
	switch reason {
	case "end_turn":
		return llm.StopReasonEndTurn
	case "tool_use":
		return llm.StopReasonToolUse
	case "max_tokens":
		return llm.StopReasonMaxTokens
	case "stop_sequence":
		return llm.StopReasonStopSequence
	default:
		// Future stop reasons from the API are mapped to EndTurn to be
		// forward-compatible without breaking callers.
		return llm.StopReasonEndTurn
	}
}
