// SPDX-License-Identifier: Apache-2.0

// Package main demonstrates wiring an HTTP-based MCP server into a praxis
// orchestrator via the [mcp] sub-module, including bearer-token credential
// resolution.
//
// The example assumes a remote MCP server at the given URL that accepts
// bearer-token authentication. Replace the URL, CredentialRef, and
// Resolver with your own values to run this example.
//
// Usage:
//
//	ANTHROPIC_API_KEY=<key> MCP_TOKEN=<token> go run examples/mcp/http/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/praxis-os/praxis"
	"github.com/praxis-os/praxis/credentials"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/anthropic"
	"github.com/praxis-os/praxis/mcp"
	"github.com/praxis-os/praxis/orchestrator"
)

// envResolver is a minimal [credentials.Resolver] that reads tokens
// from environment variables. Production deployments should use a
// KMS-backed resolver; this is a demo.
type envResolver struct{}

func (envResolver) Fetch(_ context.Context, ref string) (credentials.Credential, error) {
	token := os.Getenv("MCP_TOKEN")
	if token == "" {
		return credentials.Credential{}, fmt.Errorf("MCP_TOKEN not set for ref %q", ref)
	}
	return credentials.Credential{
		Value: []byte(token),
	}, nil
}

func main() {
	ctx := context.Background()

	// 1. Build the MCP Invoker fronting one HTTP server with
	//    bearer-token authentication.
	servers := []mcp.Server{
		{
			LogicalName:   "remote-tools",
			CredentialRef: "mcp-bearer",
			Transport: mcp.TransportHTTP{
				URL: "https://mcp.example.com/v1",
				Header: map[string]string{
					"X-Custom-Header": "praxis-mcp-example",
				},
			},
		},
	}

	inv, err := mcp.New(ctx, servers,
		mcp.WithResolver(envResolver{}),
		mcp.WithMaxResponseBytes(16*1024*1024), // 16 MiB (default)
	)
	if err != nil {
		log.Fatalf("mcp.New: %v", err)
	}
	defer inv.Close()

	// 2. Feed the MCP tool definitions to the LLM request.
	defs := inv.Definitions()
	fmt.Printf("MCP adapter fronts %d tools:\n", len(defs))
	for _, d := range defs {
		fmt.Printf("  %s — %s\n", d.Name, d.Description)
	}

	// 3. Build the orchestrator with the MCP Invoker.
	provider := anthropic.New(os.Getenv("ANTHROPIC_API_KEY"))
	orch, err := orchestrator.New(provider,
		orchestrator.WithToolInvoker(inv),
	)
	if err != nil {
		log.Fatalf("orchestrator.New: %v", err)
	}

	// 4. Run an invocation.
	result, err := orch.Invoke(ctx, praxis.InvocationRequest{
		Messages: []llm.Message{{
			Role:  llm.RoleUser,
			Parts: []llm.MessagePart{llm.TextPart("What tools do you have?")},
		}},
		Tools: defs,
	})
	if err != nil {
		log.Fatalf("orchestrator.Invoke: %v", err)
	}

	fmt.Printf("\nFinal state: %s\n", result.FinalState)
}
