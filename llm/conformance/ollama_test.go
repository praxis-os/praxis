// SPDX-License-Identifier: Apache-2.0

package conformance_test

import (
	"net/http/httptest"
	"testing"

	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/conformance"
	"github.com/praxis-os/praxis/llm/openai"
)

// TestConformance_Ollama runs the shared conformance suite against the
// OpenAI-compatible provider configured as Ollama (no auth), backed by a
// local httptest server.
func TestConformance_Ollama(t *testing.T) {
	srv := httptest.NewServer(openaiCompatHandler(t))
	defer srv.Close()

	p := openai.New("", // no API key for Ollama
		openai.WithBaseURL(srv.URL),
		openai.WithName("ollama"),
		openai.WithDefaultModel("llama3.2"),
	)

	suiteReq := func() llm.LLMRequest {
		return llm.LLMRequest{
			Model: "llama3.2",
			Messages: []llm.Message{{
				Role:  llm.RoleUser,
				Parts: []llm.MessagePart{llm.TextPart("Say hello.")},
			}},
		}
	}

	// Ollama tool support varies by model; include tool request to verify
	// the protocol works even if real models may not support it.
	toolReq := func() llm.LLMRequest {
		return llm.LLMRequest{
			Model: "llama3.2",
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
