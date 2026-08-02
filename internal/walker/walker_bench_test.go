package walker

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/bethropolis/sift/internal/ignore"
)

// buildBenchTree writes nFiles small files across dirs subdirs, plus an
// ignored node_modules-like subtree so prune behavior is exercised. It
// returns the tree root.
func buildBenchTree(b *testing.B, dirs, filesPerDir int) string {
	b.Helper()
	base := b.TempDir()
	var wg sync.WaitGroup
	for d := 0; d < dirs; d++ {
		dir := filepath.Join(base, "d"+string(rune('a'+d%26))+string(rune('0'+d/26)))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			b.Fatal(err)
		}
		wg.Add(1)
		go func(dir string) {
			defer wg.Done()
			for f := 0; f < filesPerDir; f++ {
				p := filepath.Join(dir, "f"+string(rune('a'+f%26))+string(rune('0'+f/26))+".txt")
				if err := os.WriteFile(p, []byte("package bench\nfunc Foo() int { return 1 }\n"), 0o644); err != nil {
					b.Error(err)
					return
				}
			}
		}(dir)
	}
	wg.Wait()
	if err := os.MkdirAll(filepath.Join(base, "node_modules", "pkg"), 0o755); err != nil {
		b.Fatal(err)
	}
	for f := 0; f < filesPerDir; f++ {
		if err := os.WriteFile(filepath.Join(base, "node_modules", "pkg", "x"+string(rune('a'+f%26))+".js"), []byte("x"), 0o644); err != nil {
			b.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(base, ".gitignore"), []byte("node_modules/\n"), 0o644); err != nil {
		b.Fatal(err)
	}
	return base
}

func benchWalk(b *testing.B, dirs, filesPerDir int, concurrent bool) {
	root := buildBenchTree(b, dirs, filesPerDir)
	matcher, err := ignore.New(root)
	if err != nil {
		b.Fatal(err)
	}

	opts := []Option{WithMaxWorkers(8)}
	if concurrent {
		opts = append(opts, WithConcurrency(true))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var mu sync.Mutex
		var n int
		walkFn := func(relativePath string, content []byte, err error) error {
			mu.Lock()
			n++
			mu.Unlock()
			return nil
		}
		if _, err := Walk(root, matcher, walkFn, opts...); err != nil {
			b.Fatal(err)
		}
		if n == 0 {
			b.Fatal("walked no files")
		}
	}
}

// BenchmarkWalkSmall measures the full walk + read path on a ~200 file tree.
func BenchmarkWalkSmall(b *testing.B) {
	benchWalk(b, 10, 20, false)
}

// BenchmarkWalkSmallConcurrent is BenchmarkWalkSmall with workers enabled.
func BenchmarkWalkSmallConcurrent(b *testing.B) {
	benchWalk(b, 10, 20, true)
}

// BenchmarkWalkLarge measures the full walk + read path on a ~10k file tree.
func BenchmarkWalkLarge(b *testing.B) {
	benchWalk(b, 100, 100, true)
}

// BenchmarkCollectLargeIgnored simulates a big tree dominated by ignored
// heavy directories (node_modules), the worst case for traversal overhead.
func BenchmarkCollectLargeIgnored(b *testing.B) {
	root := b.TempDir()
	for d := 0; d < 50; d++ {
		dir := filepath.Join(root, "node_modules", "pkg"+string(rune('0'+d%10))+string(rune('0'+d/10)))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			b.Fatal(err)
		}
		for f := 0; f < 100; f++ {
			if err := os.WriteFile(filepath.Join(dir, "f"+string(rune('0'+f%10))+".js"), []byte("x"), 0o644); err != nil {
				b.Fatal(err)
			}
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("node_modules/\n"), 0o644); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0o644); err != nil {
		b.Fatal(err)
	}

	matcher, err := ignore.New(root)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var mu sync.Mutex
		var n int
		walkFn := func(relativePath string, content []byte, err error) error {
			mu.Lock()
			n++
			mu.Unlock()
			return nil
		}
		if _, err := Walk(root, matcher, walkFn, WithConcurrency(true), WithMaxWorkers(8)); err != nil {
			b.Fatal(err)
		}
	}
}
