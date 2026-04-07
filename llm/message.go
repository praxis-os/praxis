// SPDX-License-Identifier: Apache-2.0

package llm

// Role identifies the author of a [Message].
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
	RoleTool      Role = "tool"
)

// String returns the string representation of the Role.
func (r Role) String() string { return string(r) }

// Message is a single turn in the conversation history.
// It contains one or more parts representing the turn content.
type Message struct {
	Role  Role
	Parts []MessagePart
}

// PartType distinguishes message content kinds.
type PartType string

const (
	PartTypeText       PartType = "text"
	PartTypeToolCall   PartType = "tool_call"
	PartTypeToolResult PartType = "tool_result"
	PartTypeImageURL   PartType = "image_url"
)

// MessagePart is a single content element within a [Message].
// Exactly one of the content fields is non-zero, determined by Type.
type MessagePart struct {
	// Type identifies which content field is populated.
	Type PartType

	// Text is the text content. Non-empty when Type == PartTypeText.
	Text string

	// ToolCall describes a tool invocation requested by the LLM.
	// Non-nil when Type == PartTypeToolCall.
	ToolCall *LLMToolCall

	// ToolResult carries the output of a tool invocation.
	// Non-nil when Type == PartTypeToolResult.
	ToolResult *LLMToolResult

	// ImageURL is a URL pointing to an image.
	// Non-empty when Type == PartTypeImageURL.
	ImageURL string
}

// TextPart is a convenience constructor for a text MessagePart.
func TextPart(text string) MessagePart {
	return MessagePart{Type: PartTypeText, Text: text}
}

// ToolCallPart is a convenience constructor for a tool-call MessagePart.
func ToolCallPart(call *LLMToolCall) MessagePart {
	return MessagePart{Type: PartTypeToolCall, ToolCall: call}
}

// ToolResultPart is a convenience constructor for a tool-result MessagePart.
func ToolResultPart(result *LLMToolResult) MessagePart {
	return MessagePart{Type: PartTypeToolResult, ToolResult: result}
}
