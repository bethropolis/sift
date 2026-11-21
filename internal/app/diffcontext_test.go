package app

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/format"
)

func TestDiffNeighborFilesSetsTokensForUnsupportedNeighbors(t *testing.T) {
	root := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.ts", "import { b } from './b';\nexport const a = 1;\n")
	write("b.ts", "export const b = 2;\n")

	files := []format.FileEntry{
		{Path: "a.ts", Content: []byte("import { b } from './b';\nexport const a = 1;\n"), SigContent: []byte("export const a = 1;\n"), Tokens: -1, TokensFull: 10, TokensSig: 4},
		// No SigContent: unsupported language or a no-CGO build.
		{Path: "b.ts", Content: []byte("export const b = 2;\n"), Tokens: -1, TokensFull: 7},
	}
	cfg := config.New()
	cfg.RootDir = root
	got := DiffNeighborFiles(cfg, files, []string{"a.ts"}, 1)
	if len(got) != 1 || got[0].Path != "b.ts" {
		t.Fatalf("neighbors = %v, want [b.ts]", got)
	}
	if got[0].Tokens != 7 {
		t.Fatalf("neighbor Tokens = %d, want TokensFull 7", got[0].Tokens)
	}
}
func TestRenderPatchRedactsSecrets(t *testing.T) {
	cfg := config.New()
	cfg.RootDir = t.TempDir()
	cfg.Style = "markdown"
	cfg.OutputFile = "-"
	cfg.SecretScan = true
	application, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer application.Close()
	var out bytes.Buffer
	application.output = &out

	patch := "diff --git a/config.go b/config.go\n+const token = \"ghp_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\"\n"
	metrics, err := application.RenderPatchWithMetrics(nil, patch, nil, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if metrics.Secrets == 0 {
		t.Fatal("patch secret was not counted")
	}
	if bytes.Contains(out.Bytes(), []byte("ghp_AAAA")) {
		t.Fatalf("patch leaked a secret:\n%s", out.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("[REDACTED_SECRET:")) {
		t.Fatalf("patch was not redacted:\n%s", out.String())
	}
}
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
