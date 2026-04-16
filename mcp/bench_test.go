// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"fmt"
	"testing"
)

// BenchmarkRouterLookup measures the map-lookup cost for varying numbers of
// tools across different server counts. This is the hot path on every Invoke.
func BenchmarkRouterLookup(b *testing.B) {
	for _, serverCount := range []int{1, 8, 64} {
		b.Run(fmt.Sprintf("servers=%d", serverCount), func(b *testing.B) {
			specs := make([]serverSpec, serverCount)
			servers := make([]Server, serverCount)
			for i := range serverCount {
				name := fmt.Sprintf("server%d", i)
				specs[i] = serverSpec{
					logicalName: name,
					tools: []toolSpec{
						{name: "tool-a", description: "bench tool a"},
						{name: "tool-b", description: "bench tool b"},
					},
				}
				servers[i] = validServer(name)
			}

			opener := openSessionsWithTools(specs)
			sessions, err := opener(context.Background(), defaultConfig(), servers)
			if err != nil {
				b.Fatalf("opener: %v", err)
			}
			defer func() { _ = closeSessions(sessions) }()

			rt, err := buildRouter(context.Background(), servers, sessions)
			if err != nil {
				b.Fatalf("buildRouter: %v", err)
			}

			// Pick a tool name from the middle of the set to avoid any
			// edge-case advantage for first/last entries.
			mid := serverCount / 2
			target := fmt.Sprintf("server%d__tool-a", mid)

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, ok := rt.lookup(target)
				if !ok {
					b.Fatalf("lookup(%q) returned false", target)
				}
			}
		})
	}
}

// BenchmarkBuildRouter measures the one-time router construction cost
// (tool enumeration + sorting) for varying server counts.
func BenchmarkBuildRouter(b *testing.B) {
	for _, serverCount := range []int{1, 8, 32} {
		b.Run(fmt.Sprintf("servers=%d", serverCount), func(b *testing.B) {
			specs := make([]serverSpec, serverCount)
			servers := make([]Server, serverCount)
			for i := range serverCount {
				name := fmt.Sprintf("server%d", i)
				specs[i] = serverSpec{
					logicalName: name,
					tools: []toolSpec{
						{name: "tool-a", description: "a"},
						{name: "tool-b", description: "b"},
						{name: "tool-c", description: "c"},
						{name: "tool-d", description: "d"},
					},
				}
				servers[i] = validServer(name)
			}

			opener := openSessionsWithTools(specs)
			sessions, err := opener(context.Background(), defaultConfig(), servers)
			if err != nil {
				b.Fatalf("opener: %v", err)
			}
			defer func() { _ = closeSessions(sessions) }()

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := buildRouter(context.Background(), servers, sessions)
				if err != nil {
					b.Fatalf("buildRouter: %v", err)
				}
			}
		})
	}
}
