// SPDX-License-Identifier: Apache-2.0

package gemini

import (
	"encoding/json"
	"fmt"

	"github.com/praxis-os/praxis/llm"
)

// --- Gemini API request types ---

type geminiRequest struct {
	SystemInstruction *geminiContent     `json:"systemInstruction,omitempty"`
	Contents          []geminiContent    `json:"contents"`
	Tools             []geminiToolConfig `json:"tools,omitempty"`
	GenerationConfig  *geminiGenConfig   `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text             string                `json:"text,omitempty"`
	FunctionCall     *geminiFunctionCall   `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResp   `json:"functionResponse,omitempty"`
}

type geminiFunctionCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

type geminiFunctionResp struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type geminiToolConfig struct {
	FunctionDeclarations []geminiFuncDecl `json:"functionDeclarations"`
}

type geminiFuncDecl struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type geminiGenConfig struct {
	MaxOutputTokens *int     `json:"maxOutputTokens,omitempty"`
	Temperature     *float64 `json:"temperature,omitempty"`
}

// --- Gemini API response types ---

type geminiResponse struct {
	Candidates    []geminiCandidate `json:"candidates"`
	UsageMetadata *geminiUsage      `json:"usageMetadata,omitempty"`
}

type geminiCandidate struct {
	Content      geminiContent `json:"content"`
	FinishReason string        `json:"finishReason"`
}

type geminiUsage struct {
	PromptTokenCount     int64 `json:"promptTokenCount"`
	CandidatesTokenCount int64 `json:"candidatesTokenCount"`
	TotalTokenCount      int64 `json:"totalTokenCount"`
}

// --- Gemini API error types ---

type geminiErrorEnvelope struct {
	Error geminiErrorDetail `json:"error"`
}

type geminiErrorDetail struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

// --- Conversion: praxis → Gemini ---

func toAPIRequest(req llm.LLMRequest, defaultModel string) (geminiRequest, string) {
	model := req.Model
	if model == "" {
		model = defaultModel
	}

	apiReq := geminiRequest{}

	// System instruction.
	if req.SystemPrompt != "" {
		apiReq.SystemInstruction = &geminiContent{
			Parts: []geminiPart{{Text: req.SystemPrompt}},
		}
	}

	// Messages.
	apiReq.Contents = toAPIContents(req.Messages)

	// Tools.
	if len(req.Tools) > 0 {
		decls := make([]geminiFuncDecl, len(req.Tools))
		for i, tool := range req.Tools {
			decls[i] = geminiFuncDecl{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			}
		}
		apiReq.Tools = []geminiToolConfig{{FunctionDeclarations: decls}}
	}

	// Generation config.
	if req.MaxTokens > 0 || req.Temperature > 0 {
		gc := &geminiGenConfig{}
		if req.MaxTokens > 0 {
			mt := req.MaxTokens
			gc.MaxOutputTokens = &mt
		}
		if req.Temperature > 0 {
			t := req.Temperature
			gc.Temperature = &t
		}
		apiReq.GenerationConfig = gc
	}

	return apiReq, model
}

func toAPIContents(messages []llm.Message) []geminiContent {
	contents := make([]geminiContent, 0, len(messages))
	for _, msg := range messages {
		// Skip system messages — handled via systemInstruction.
		if msg.Role == llm.RoleSystem {
			continue
		}

		content := geminiContent{
			Role:  toGeminiRole(msg.Role),
			Parts: make([]geminiPart, 0, len(msg.Parts)),
		}

		for _, part := range msg.Parts {
			switch part.Type {
			case llm.PartTypeText:
				content.Parts = append(content.Parts, geminiPart{Text: part.Text})

			case llm.PartTypeToolCall:
				if part.ToolCall != nil {
					content.Parts = append(content.Parts, geminiPart{
						FunctionCall: &geminiFunctionCall{
							Name: part.ToolCall.Name,
							Args: part.ToolCall.ArgumentsJSON,
						},
					})
				}

			case llm.PartTypeToolResult:
				if part.ToolResult != nil {
					// Gemini expects functionResponse with a JSON object body.
					respMap := map[string]any{"result": part.ToolResult.Content}
					if part.ToolResult.IsError {
						respMap["error"] = true
					}
					content.Parts = append(content.Parts, geminiPart{
						FunctionResponse: &geminiFunctionResp{
							Name:     toolNameFromCallID(part.ToolResult.CallID, messages),
							Response: respMap,
						},
					})
				}
			}
		}

		if len(content.Parts) > 0 {
			contents = append(contents, content)
		}
	}
	return contents
}

// toolNameFromCallID searches messages for a tool call with the given call ID
// and returns the tool name. Gemini correlates tool results by function name,
// not call ID.
func toolNameFromCallID(callID string, messages []llm.Message) string {
	for _, msg := range messages {
		for _, part := range msg.Parts {
			if part.Type == llm.PartTypeToolCall && part.ToolCall != nil && part.ToolCall.CallID == callID {
				return part.ToolCall.Name
			}
		}
	}
	// Fallback: use callID as name if not found.
	return callID
}

func toGeminiRole(role llm.Role) string {
	switch role {
	case llm.RoleUser, llm.RoleTool:
		return "user"
	case llm.RoleAssistant:
		return "model"
	default:
		return "user"
	}
}

// --- Conversion: Gemini → praxis ---

func fromAPIResponse(resp geminiResponse) llm.LLMResponse {
	result := llm.LLMResponse{}

	// Usage.
	if resp.UsageMetadata != nil {
		result.Usage = llm.TokenUsage{
			InputTokens:  resp.UsageMetadata.PromptTokenCount,
			OutputTokens: resp.UsageMetadata.CandidatesTokenCount,
		}
	}

	if len(resp.Candidates) == 0 {
		result.StopReason = llm.StopReasonEndTurn
		return result
	}

	candidate := resp.Candidates[0]
	result.StopReason = fromGeminiFinishReason(candidate.FinishReason)
	result.Message = fromGeminiContent(candidate.Content)

	return result
}

func fromGeminiContent(content geminiContent) llm.Message {
	msg := llm.Message{
		Role:  llm.RoleAssistant,
		Parts: make([]llm.MessagePart, 0, len(content.Parts)),
	}

	for i, part := range content.Parts {
		if part.Text != "" {
			msg.Parts = append(msg.Parts, llm.TextPart(part.Text))
		}
		if part.FunctionCall != nil {
			// Gemini does not use call IDs. Synthesize one for praxis
			// multi-tool correlation.
			callID := fmt.Sprintf("gemini-%s-%d", part.FunctionCall.Name, i)
			msg.Parts = append(msg.Parts, llm.MessagePart{
				Type: llm.PartTypeToolCall,
				ToolCall: &llm.LLMToolCall{
					CallID:        callID,
					Name:          part.FunctionCall.Name,
					ArgumentsJSON: part.FunctionCall.Args,
				},
			})
		}
	}

	return msg
}

func fromGeminiFinishReason(reason string) llm.StopReason {
	switch reason {
	case "STOP":
		return llm.StopReasonEndTurn
	case "MAX_TOKENS":
		return llm.StopReasonMaxTokens
	case "SAFETY", "RECITATION", "OTHER":
		return llm.StopReasonEndTurn
	default:
		// If any function call is present, treat as tool use.
		return llm.StopReasonEndTurn
	}
}
