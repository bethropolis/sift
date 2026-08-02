package rank

import (
	"os"
	"path/filepath"
	"testing"
)

// repoRoot locates this module's root (a git repository) so the git-based
// benchmarks have real history to read.
func repoRoot(b *testing.B) string {
	b.Helper()
	dir, err := os.Getwd()
	if err != nil {
		b.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			b.Skip("not in a git repository")
		}
		dir = parent
	}
}

// BenchmarkChangesFor measures the parallel status/diff/log ranking path
// against this repository's real git history.
func BenchmarkChangesFor(b *testing.B) {
	g := New(repoRoot(b))
	if !g.Available() {
		b.Skip("not in a git repository")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = g.ChangesFor("HEAD")
	}
}
