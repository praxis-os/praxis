// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"testing"
	"time"
)

// BenchmarkGenerateUUIDv7 measures the allocation cost of UUIDv7 generation.
//
// Run:
//
//	go test -run '^$' -bench BenchmarkGenerateUUIDv7 -benchmem -count=6 ./identity/
func BenchmarkGenerateUUIDv7(b *testing.B) {
	now := time.Now()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if _, err := generateUUIDv7(now); err != nil {
			b.Fatalf("generateUUIDv7: %v", err)
		}
	}
}
