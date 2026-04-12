// SPDX-License-Identifier: Apache-2.0

// Package main demonstrates wiring a stdio-based MCP server into a praxis
// orchestrator via the [mcp] sub-module.
//
// The example assumes a local MCP server binary called "mcp-github" that
// exposes GitHub-related tools (list_issues, create_issue, etc.) over
// the MCP stdio transport. Replace the Command and LogicalName with your
// own server to run this example.
//
// Usage:
//
//	ANTHROPIC_API_KEY=<key> go run examples/mcp/stdio/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/praxis-os/praxis"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/llm/anthropic"
	"github.com/praxis-os/praxis/mcp"
	"github.com/praxis-os/praxis/orchestrator"
)

func main() {
	ctx := context.Background()

	// 1. Build the MCP Invoker fronting one stdio server.
	servers := []mcp.Server{
		{
			LogicalName: "github",
			Transport: mcp.TransportStdio{
				Command: "mcp-github",
				// Args and Env are optional; the adapter resolves
				// the binary via exec.LookPath.
			},
		},
	}

	inv, err := mcp.New(ctx, servers,
		mcp.WithMaxResponseBytes(8*1024*1024), // 8 MiB cap
	)
	if err != nil {
		log.Fatalf("mcp.New: %v", err)
	}
	defer inv.Close()

	// 2. Feed the MCP tool definitions to the LLM request.
	//    inv.Definitions() returns []llm.ToolDefinition ready for
	//    praxis.InvocationRequest.Tools.
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

	// 4. Run an invocation. The orchestrator routes MCP-namespaced
	//    tool calls (e.g., "github__list_issues") through the MCP
	//    adapter automatically.
	result, err := orch.Invoke(ctx, praxis.InvocationRequest{
		Messages: []llm.Message{{
			Role:  llm.RoleUser,
			Parts: []llm.MessagePart{llm.TextPart("List open issues on praxis-os/praxis")},
		}},
		Tools: defs,
	})
	if err != nil {
		log.Fatalf("orchestrator.Invoke: %v", err)
	}

	fmt.Printf("\nFinal state: %s\n", result.FinalState)
}
