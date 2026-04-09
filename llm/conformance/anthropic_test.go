// SPDX-License-Identifier: Apache-2.0

package conformance_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/anthropic"
	"github.com/praxis-os/praxis/llm/conformance"
)

// TestConformance_Anthropic runs the shared conformance suite against the
// Anthropic provider backed by a local httptest server.
func TestConformance_Anthropic(t *testing.T) {
	mux := http.NewServeMux()

	// Simple text response.
	mux.HandleFunc("/v1/messages", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]json.RawMessage
		_ = json.NewDecoder(r.Body).Decode(&body)

		// Check if the request contains tools to decide response type.
		if _, hasTools := body["tools"]; hasTools {
			resp := map[string]any{
				"id":    "msg_tool",
				"type":  "message",
				"role":  "assistant",
				"model": "claude-sonnet-4-20250514",
				"stop_reason": "tool_use",
				"content": []map[string]any{
					{
						"type": "tool_use",
						"id":   "toolu_01",
						"name": "get_weather",
						"input": map[string]any{
							"city": "Berlin",
						},
					},
				},
				"usage": map[string]any{
					"input_tokens":  15,
					"output_tokens": 8,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		resp := map[string]any{
			"id":          "msg_01test",
			"type":        "message",
			"role":        "assistant",
			"model":       "claude-sonnet-4-20250514",
			"stop_reason": "end_turn",
			"content": []map[string]any{
				{"type": "text", "text": "Hello from conformance test."},
			},
			"usage": map[string]any{
				"input_tokens":  10,
				"output_tokens": 5,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	p := anthropic.New("test-key",
		anthropic.WithBaseURL(srv.URL),
		anthropic.WithMaxTokens(64),
	)

	suiteReq := func() llm.LLMRequest {
		return llm.LLMRequest{
			Messages: []llm.Message{{
				Role:  llm.RoleUser,
				Parts: []llm.MessagePart{llm.TextPart("Say hello.")},
			}},
		}
	}

	toolReq := func() llm.LLMRequest {
		return llm.LLMRequest{
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
