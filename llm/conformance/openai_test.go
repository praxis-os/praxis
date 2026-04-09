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

// TestConformance_OpenAI runs the shared conformance suite against the
// OpenAI provider backed by a local httptest server.
func TestConformance_OpenAI(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]json.RawMessage
		_ = json.NewDecoder(r.Body).Decode(&body)

		// Check if the request contains tools to decide response type.
		if _, hasTools := body["tools"]; hasTools {
			resp := map[string]any{
				"id":     "chatcmpl-tool",
				"object": "chat.completion",
				"model":  "gpt-4o",
				"choices": []map[string]any{{
					"index":         0,
					"finish_reason": "tool_calls",
					"message": map[string]any{
						"role": "assistant",
						"tool_calls": []map[string]any{{
							"id":   "call_01",
							"type": "function",
							"function": map[string]any{
								"name":      "get_weather",
								"arguments": `{"city":"Berlin"}`,
							},
						}},
					},
				}},
				"usage": map[string]any{
					"prompt_tokens":     15,
					"completion_tokens": 8,
					"total_tokens":      23,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		resp := map[string]any{
			"id":     "chatcmpl-123",
			"object": "chat.completion",
			"model":  "gpt-4o",
			"choices": []map[string]any{{
				"index":         0,
				"finish_reason": "stop",
				"message": map[string]any{
					"role":    "assistant",
					"content": "Hello from conformance test.",
				},
			}},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	p := openai.New("test-key", openai.WithBaseURL(srv.URL))

	suiteReq := func() llm.LLMRequest {
		return llm.LLMRequest{
			Model: "gpt-4o",
			Messages: []llm.Message{{
				Role:  llm.RoleUser,
				Parts: []llm.MessagePart{llm.TextPart("Say hello.")},
			}},
		}
	}

	toolReq := func() llm.LLMRequest {
		return llm.LLMRequest{
			Model: "gpt-4o",
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
