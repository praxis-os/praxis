// SPDX-License-Identifier: Apache-2.0

package conformance_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/conformance"
	"github.com/praxis-os/praxis/llm/gemini"
)

// TestConformance_Gemini runs the shared conformance suite against the
// Gemini provider backed by a local httptest server.
func TestConformance_Gemini(t *testing.T) {
	mux := http.NewServeMux()

	// Gemini endpoint pattern: /v1beta/models/{model}:generateContent
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]json.RawMessage
		_ = json.NewDecoder(r.Body).Decode(&body)

		// Check if the request contains tools to decide response type.
		if _, hasTools := body["tools"]; hasTools {
			resp := map[string]any{
				"candidates": []map[string]any{{
					"content": map[string]any{
						"role": "model",
						"parts": []map[string]any{{
							"functionCall": map[string]any{
								"name": "get_weather",
								"args": map[string]any{"city": "Berlin"},
							},
						}},
					},
					"finishReason": "STOP",
				}},
				"usageMetadata": map[string]any{
					"promptTokenCount":     15,
					"candidatesTokenCount": 8,
					"totalTokenCount":      23,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		resp := map[string]any{
			"candidates": []map[string]any{{
				"content": map[string]any{
					"role": "model",
					"parts": []map[string]any{
						{"text": "Hello from conformance test."},
					},
				},
				"finishReason": "STOP",
			}},
			"usageMetadata": map[string]any{
				"promptTokenCount":     10,
				"candidatesTokenCount": 5,
				"totalTokenCount":      15,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	p := gemini.New("test-key", gemini.WithBaseURL(srv.URL))

	suiteReq := func() llm.LLMRequest {
		return llm.LLMRequest{
			Model: "gemini-2.0-flash",
			Messages: []llm.Message{{
				Role:  llm.RoleUser,
				Parts: []llm.MessagePart{llm.TextPart("Say hello.")},
			}},
		}
	}

	toolReq := func() llm.LLMRequest {
		return llm.LLMRequest{
			Model: "gemini-2.0-flash",
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
