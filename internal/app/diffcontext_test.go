package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/format"
)

func TestDiffNeighborFilesWalksBothDirections(t *testing.T) {
	root := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.ts", "import { b } from './b';\nexport const a = 1;\n")
	write("b.ts", "import { c } from './c';\nexport const b = 2;\n")
	write("c.ts", "export const c = 3;\n")
	write("d.ts", "import { b } from './b';\nexport const d = 4;\n")

	files := []format.FileEntry{
		{Path: "a.ts", Content: []byte("import { b } from './b';\nexport const a = 1;\n"), SigContent: []byte("export const a = 1;\n"), TokensFull: 10, TokensSig: 4},
		{Path: "b.ts", Content: []byte("import { c } from './c';\nexport const b = 2;\n"), SigContent: []byte("export const b = 2;\n"), TokensFull: 10, TokensSig: 4},
		{Path: "c.ts", Content: []byte("export const c = 3;\n"), SigContent: []byte("export const c = 3;\n"), TokensFull: 5, TokensSig: 5},
		{Path: "d.ts", Content: []byte("import { b } from './b';\nexport const d = 4;\n"), SigContent: []byte("export const d = 4;\n"), TokensFull: 10, TokensSig: 4},
	}

	cfg := config.New()
	cfg.RootDir = root
	cfg.SecretScan = false
	got := DiffNeighborFiles(cfg, files, []string{"b.ts"}, 1)
	var paths []string
	for _, f := range got {
		paths = append(paths, f.Path)
		if !f.IsCompressed {
			t.Errorf("%s not rendered as signatures", f.Path)
		}
	}
	want := []string{"a.ts", "c.ts", "d.ts"}
	if len(paths) != len(want) {
		t.Fatalf("neighbors = %v, want %v", paths, want)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("neighbors = %v, want %v", paths, want)
		}
	}
}
