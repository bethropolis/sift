package app

import (
	"strings"
	"testing"

	"github.com/bethropolis/sift/internal/config"
)

// TestCollectSignaturesModeCompresses confirms the blocking collect path stores
// the signature summary in FileEntry.Content when the global mode is
// signatures, so rendered dumps carry the compressed text rather than the raw
// source with compressed token counts.
func TestCollectSignaturesModeCompresses(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "main.go", `package main

// main runs the program.
func main() {
	done := false
	for !done {
		done = true
	}
	println("hi")
}
`)

	cfg := config.New()
	cfg.RootDir = root
	cfg.Quiet = true
	cfg.OutputFile = "-"
	cfg.Mode = "signatures"
	a, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	files, _, err := a.Collect()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("got %d files, want 1", len(files))
	}
	f := files[0]
	if !f.IsCompressed {
		t.Errorf("IsCompressed = false, want true in signatures mode")
	}
	if !strings.Contains(string(f.Content), "func main() { /* ... */ }") {
		t.Errorf("compressed content missing signature placeholder:\n%s", f.Content)
	}
	if strings.Contains(string(f.Content), "for !done") {
		t.Errorf("raw body leaked into compressed content:\n%s", f.Content)
	}
}

// TestCollectFullModeKeepsRaw confirms full mode leaves content untouched.
func TestCollectFullModeKeepsRaw(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "main.go", "package main\n\nfunc main() {\n\tprintln(\"hi\")\n}\n")

	cfg := config.New()
	cfg.RootDir = root
	cfg.Quiet = true
	cfg.OutputFile = "-"
	cfg.Mode = "full"
	a, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	files, _, err := a.Collect()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("got %d files, want 1", len(files))
	}
	f := files[0]
	if f.IsCompressed {
		t.Error("IsCompressed = true in full mode")
	}
	if !strings.Contains(string(f.Content), "println") {
		t.Errorf("full content missing body:\n%s", f.Content)
	}
}
