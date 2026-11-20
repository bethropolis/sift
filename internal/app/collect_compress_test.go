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
	requireCompression(t)
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

func TestCollectOutlineModeOmitsDocComments(t *testing.T) {
	requireCompression(t)
	root := t.TempDir()
	writeFile(t, root, "main.go", `package main

// main runs the program.
func main() {
	println("hi")
}
`)

	cfg := config.New()
	cfg.RootDir = root
	cfg.Quiet = true
	cfg.OutputFile = "-"
	cfg.Mode = "outline"
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
	content := string(files[0].Content)
	if strings.Contains(content, "main runs the program") {
		t.Errorf("outline kept doc comment:\n%s", content)
	}
	if !strings.Contains(content, "func main() { /* ... */ }") {
		t.Errorf("outline missing signature:\n%s", content)
	}
}

func TestCollectCollapseTestsCompressesOnlyTests(t *testing.T) {
	requireCompression(t)
	root := t.TempDir()
	writeFile(t, root, "main.go", `package main

func main() {
	println("impl")
}
`)
	writeFile(t, root, "main_test.go", `package main

func TestMain(t *testing.T) {
	println("test body")
}
`)

	cfg := config.New()
	cfg.RootDir = root
	cfg.Quiet = true
	cfg.OutputFile = "-"
	cfg.Mode = "full"
	cfg.CollapseTests = true
	a, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	files, _, err := a.Collect()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2", len(files))
	}
	for _, f := range files {
		isTest := strings.HasSuffix(f.Path, "_test.go")
		if isTest {
			if !f.IsCompressed || !strings.Contains(string(f.Content), "/* ... */") {
				t.Errorf("test file not collapsed:\n%s", f.Content)
			}
			if strings.Contains(string(f.Content), "test body") {
				t.Errorf("test body leaked:\n%s", f.Content)
			}
			continue
		}
		if f.IsCompressed {
			t.Errorf("non-test file compressed")
		}
		if !strings.Contains(string(f.Content), "impl") {
			t.Errorf("non-test body missing:\n%s", f.Content)
		}
	}
}

func TestCollectSignaturesSummarizesSQL(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "schema.sql", "CREATE TABLE t (id int);\nINSERT INTO t VALUES (1);\n")

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
	content := string(files[0].Content)
	if !strings.Contains(content, "CREATE TABLE t") || strings.Contains(content, "VALUES (1)") {
		t.Fatalf("SQL summary wrong:\n%s", content)
	}
}
