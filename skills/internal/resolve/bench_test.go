// SPDX-License-Identifier: Apache-2.0

package resolve_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/praxis-os/praxis/skills/internal/resolve"
)

// BenchmarkResolvePath_Directory measures the directory→(dir, dir/SKILL.md)
// resolution path.
func BenchmarkResolvePath_Directory(b *testing.B) {
	dir := b.TempDir()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _, err := resolve.ResolvePath(dir)
		if err != nil {
			b.Fatalf("ResolvePath: %v", err)
		}
	}
}

// BenchmarkResolvePath_File measures the file→(dir, file) resolution path.
func BenchmarkResolvePath_File(b *testing.B) {
	dir := b.TempDir()
	file := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(file, []byte("content"), 0644); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := resolve.ResolvePath(file)
		if err != nil {
			b.Fatalf("ResolvePath: %v", err)
		}
	}
}
