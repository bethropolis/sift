package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bethropolis/sift/internal/config"
)

// TestNewStdoutOutput verifies "--output -" writes to stdout with no output
// file and no construction error.
func TestNewStdoutOutput(t *testing.T) {
	cfg := config.New()
	cfg.RootDir = t.TempDir()
	cfg.OutputFile = "-"

	a, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	if a.OutputPath() != "" {
		t.Errorf("OutputPath() = %q, want empty for stdout", a.OutputPath())
	}
	if a.Output() != os.Stdout {
		t.Errorf("Output() = %T, want os.Stdout", a.Output())
	}
}

// TestNewOutputFileError verifies a construction error surfaces when the
// output file cannot be created, instead of exiting the process.
func TestNewOutputFileError(t *testing.T) {
	cfg := config.New()
	cfg.RootDir = t.TempDir()
	cfg.OutputFile = filepath.Join(t.TempDir(), "missing", "dir", "out.md")

	a, err := New(cfg)
	if err == nil {
		a.Close()
		t.Fatal("expected error for uncreatable output file")
	}
}

// TestNewOutputFileAndClose verifies the output file is created, written
// through, flushed on Close, and that Close is idempotent.
func TestNewOutputFileAndClose(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, "out.md")

	cfg := config.New()
	cfg.RootDir = root
	cfg.OutputFile = out

	a, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if a.OutputPath() != out {
		t.Errorf("OutputPath() = %q, want %q", a.OutputPath(), out)
	}
	if _, err := a.Output().Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	a.Close()

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Errorf("output file content = %q, want %q", data, "hello")
	}

	// Close must be safe to call more than once.
	a.Close()
}

// TestReadEntryRejectsEscapingPaths verifies the picker fallback refuses
// paths that would escape the scan root.
func TestReadEntryRejectsEscapingPaths(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "main.go", "package main\n")

	cfg := config.New()
	cfg.RootDir = root
	cfg.Quiet = true
	cfg.OutputFile = "-"
	a, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	if _, err := a.ReadEntry("../etc/passwd"); err == nil {
		t.Error("ReadEntry accepted a root-escaping path")
	}
}
