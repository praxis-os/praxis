// SPDX-License-Identifier: Apache-2.0

package conformance_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/conformance"
	"github.com/praxis-os/praxis/llm/openai"
)

// TestConformance_Groq runs the shared conformance suite against the
// OpenAI-compatible provider configured as Groq, backed by a local httptest
// server.
func TestConformance_Groq(t *testing.T) {
	srv := httptest.NewServer(openaiCompatHandler(t))
	defer srv.Close()

	p := openai.New("test-key",
		openai.WithBaseURL(srv.URL),
		openai.WithName("groq"),
		openai.WithDefaultModel("llama-3.3-70b-versatile"),
	)

	suiteReq := func() llm.LLMRequest {
		return llm.LLMRequest{
			Model: "llama-3.3-70b-versatile",
			Messages: []llm.Message{{
				Role:  llm.RoleUser,
				Parts: []llm.MessagePart{llm.TextPart("Say hello.")},
			}},
		}
	}

	toolReq := func() llm.LLMRequest {
		return llm.LLMRequest{
			Model: "llama-3.3-70b-versatile",
			Messages: []llm.Message{{
				Role:  llm.RoleUser,
				Parts: []llm.MessagePart{llm.TextPart("What is the weather in Berlin?")},
			}},
			Tools: []llm.ToolDefinition{{
				Name:        "get_weather",
				Description: "Get weather for a city",
				InputSchema: []byte(`{"type":"object","properties":{"city":{"type":"string"}}}`),
			}},
		}
	}

	conformance.RunSuite(t, p, suiteReq, toolReq)
}

// openaiCompatHandler returns an http.Handler that responds with
// OpenAI-compatible chat completion responses for both text and tool-use
// requests. Shared by Groq, OpenRouter, and Ollama conformance tests.
func openaiCompatHandler(t *testing.T) http.Handler {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]json.RawMessage
		_ = json.NewDecoder(r.Body).Decode(&body)

		if _, hasTools := body["tools"]; hasTools {
			resp := map[string]any{
				"id": "chatcmpl-tool", "object": "chat.completion",
				"choices": []map[string]any{{
					"index": 0, "finish_reason": "tool_calls",
					"message": map[string]any{
						"role": "assistant",
						"tool_calls": []map[string]any{{
							"id": "call_01", "type": "function",
							"function": map[string]any{
								"name":      "get_weather",
								"arguments": `{"city":"Berlin"}`,
							},
						}},
					},
				}},
				"usage": map[string]any{"prompt_tokens": 15, "completion_tokens": 8},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		resp := map[string]any{
			"id": "chatcmpl-123", "object": "chat.completion",
			"choices": []map[string]any{{
				"index": 0, "finish_reason": "stop",
				"message": map[string]any{"role": "assistant", "content": "Hello from conformance test."},
			}},
			"usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 5},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	return mux
}
