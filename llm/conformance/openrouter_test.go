// SPDX-License-Identifier: Apache-2.0

package conformance_test

import (
	"net/http/httptest"
	"testing"

	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/conformance"
	"github.com/praxis-os/praxis/llm/openai"
)

// TestConformance_OpenRouter runs the shared conformance suite against the
// OpenAI-compatible provider configured as OpenRouter, backed by a local
// httptest server.
func TestConformance_OpenRouter(t *testing.T) {
	srv := httptest.NewServer(openaiCompatHandler(t))
	defer srv.Close()

	p := openai.New("test-key",
		openai.WithBaseURL(srv.URL),
		openai.WithName("openrouter"),
		openai.WithDefaultModel("anthropic/claude-sonnet-4-20250514"),
		openai.WithExtraHeaders(map[string]string{
			"HTTP-Referer": "https://test.example.com",
			"X-Title":      "Conformance Test",
		}),
	)

	suiteReq := func() llm.LLMRequest {
		return llm.LLMRequest{
			Model: "anthropic/claude-sonnet-4-20250514",
			Messages: []llm.Message{{
				Role:  llm.RoleUser,
				Parts: []llm.MessagePart{llm.TextPart("Say hello.")},
			}},
		}
	}

	toolReq := func() llm.LLMRequest {
		return llm.LLMRequest{
			Model: "anthropic/claude-sonnet-4-20250514",
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
