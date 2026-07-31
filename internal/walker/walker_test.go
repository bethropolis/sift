package walker

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	"github.com/bethropolis/dir-dumper/internal/ignore"
)

func writeTestTree(t *testing.T, root string) {
	t.Helper()
	files := map[string]string{
		"a.txt":           "alpha",
		"b.txt":           "bravo",
		"sub/c.md":        "charlie",
		".secret.txt":     "hidden",
		"sub/.inner/d.go": "delta",
	}
	for path, content := range files {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func collectWalk(t *testing.T, root string, opts ...Option) []string {
	t.Helper()
	matcher, err := ignore.New(root)
	if err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var files []string
	walkFn := func(relativePath string, content []byte, err error) error {
		mu.Lock()
		defer mu.Unlock()
		files = append(files, relativePath)
		return nil
	}

	if _, err := Walk(root, matcher, walkFn, opts...); err != nil {
		t.Fatalf("Walk returned error: %v", err)
	}
	sort.Strings(files)
	return files
}

func TestWalkSequential(t *testing.T) {
	root := t.TempDir()
	writeTestTree(t, root)

	got := collectWalk(t, root)
	want := []string{"a.txt", "b.txt", "sub/c.md"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got %v, want %v", got, want)
		}
	}
}

func TestWalkConcurrentMatchesSequential(t *testing.T) {
	root := t.TempDir()
	writeTestTree(t, root)

	sequential := collectWalk(t, root)
	concurrent := collectWalk(t, root,
		WithConcurrency(true),
		WithMaxWorkers(4),
	)

	if len(sequential) != len(concurrent) {
		t.Fatalf("sequential %v != concurrent %v", sequential, concurrent)
	}
	for i := range sequential {
		if sequential[i] != concurrent[i] {
			t.Errorf("sequential[%d]=%q, concurrent[%d]=%q", i, sequential[i], i, concurrent[i])
		}
	}
}

func TestWalkExtensionFilter(t *testing.T) {
	root := t.TempDir()
	writeTestTree(t, root)

	got := collectWalk(t, root, WithExtensions([]string{"txt"}))
	want := []string{"a.txt", "b.txt"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got %v, want %v", got, want)
		}
	}
}

func TestWalkContextCancellation(t *testing.T) {
	root := t.TempDir()
	writeTestTree(t, root)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	matcher, err := ignore.New(root)
	if err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	count := 0
	walkFn := func(relativePath string, content []byte, err error) error {
		mu.Lock()
		defer mu.Unlock()
		count++
		return nil
	}

	if _, err := Walk(root, matcher, walkFn, WithConcurrency(true), WithContext(ctx)); err == nil {
		t.Error("expected error from cancelled context, got nil")
	}
	if count != 0 {
		t.Errorf("processed %d files with cancelled context, want 0", count)
	}
}
