package app

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/bethropolis/sift/internal/format"
)

// BenchmarkComputeFanIn exercises the single-pass import resolution used by
// ranking and dependency expansion.
func BenchmarkComputeFanIn(b *testing.B) {
	dir := b.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/bench\n"), 0o644); err != nil {
		b.Fatal(err)
	}

	files := make([]format.FileEntry, 0, 201)
	files = append(files, format.FileEntry{
		Path:    "core/lib.go",
		Content: []byte("package core\n"),
	})
	for i := 0; i < 200; i++ {
		path := fmt.Sprintf("pkg%d/file.go", i)
		body := fmt.Sprintf("package pkg%d\n\nimport \"example.com/bench/core\"\n", i)
		files = append(files, format.FileEntry{Path: path, Content: []byte(body)})
	}

	r := NewRanker(dir)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.computeFanIn(files)
	}
}
