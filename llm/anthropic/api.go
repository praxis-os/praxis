// SPDX-License-Identifier: Apache-2.0

package anthropic

import "encoding/json"

// apiRequest is the JSON body sent to POST /v1/messages.
type apiRequest struct {
	Model     string       `json:"model"`
	MaxTokens int          `json:"max_tokens"`
	System    string       `json:"system,omitempty"`
	Messages  []apiMessage `json:"messages"`
	Tools     []apiTool    `json:"tools,omitempty"`

	// Temperature is omitted when zero so the API uses its default.
	Temperature *float64 `json:"temperature,omitempty"`
}

// apiMessage is a single turn in the Anthropic messages array.
type apiMessage struct {
	Role    string        `json:"role"`
	Content []apiContent  `json:"content"`
}

// apiContent is a single content block within an apiMessage.
// The Type field discriminates the union; only the relevant fields are
// populated. For inbound tool_result blocks, ToolUseID + Content are used.
type apiContent struct {
	// Shared discriminator.
	Type string `json:"type"`

	// text block.
	Text string `json:"text,omitempty"`

	// tool_use block (LLM → caller).
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// tool_result block (caller → LLM).
	ToolUseID string `json:"tool_use_id,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`
	// Content inside a tool_result is a string for simplicity.
	ToolResultContent string `json:"content,omitempty"`
}

// apiTool describes a tool available to the model.
type apiTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// apiResponse is the JSON body returned by POST /v1/messages.
type apiResponse struct {
	ID           string       `json:"id"`
	Type         string       `json:"type"`
	Role         string       `json:"role"`
	Content      []apiContent `json:"content"`
	Model        string       `json:"model"`
	StopReason   string       `json:"stop_reason"`
	StopSequence *string      `json:"stop_sequence"`
	Usage        apiUsage     `json:"usage"`
}

// apiUsage carries token consumption from the Anthropic response.
type apiUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

// apiError is the JSON body returned by the Anthropic API on 4xx/5xx responses.
type apiError struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}
