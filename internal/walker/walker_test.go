package walker

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	"github.com/bethropolis/sift/internal/ignore"
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

func TestWalkBinaryFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write a file with a NUL byte in the first 512 bytes.
	if err := os.WriteFile(filepath.Join(root, "data.bin"), []byte{0x00, 0x01, 0x02}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ok.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Binary files are skipped by default.
	got := collectWalk(t, root)
	if len(got) != 1 || got[0] != "ok.txt" {
		t.Errorf("got %v, want [ok.txt]", got)
	}

	// With WithIncludeBinary they are included.
	got = collectWalk(t, root, WithIncludeBinary(true))
	if len(got) != 2 {
		t.Errorf("got %v, want [data.bin ok.txt]", got)
	}
}

func TestIsBinary(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
		want    bool
	}{
		{name: "empty", content: nil, want: false},
		{name: "text", content: []byte("hello world"), want: false},
		{name: "nul at start", content: []byte{0x00, 0x01}, want: true},
		{name: "nul within first 512", content: append(bytes.Repeat([]byte("a"), 100), 0x00), want: true},
		{name: "nul beyond 512 bytes not detected", content: append(bytes.Repeat([]byte("a"), 600), 0x00), want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isBinary(tc.content); got != tc.want {
				t.Errorf("isBinary(%q) = %v, want %v", tc.content, got, tc.want)
			}
		})
	}
}
