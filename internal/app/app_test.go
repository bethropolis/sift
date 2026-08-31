package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/format"
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

// TestReadEntryRejectsOversize verifies the picker fallback enforces the size
// cap (the picker 10 MB default, or an explicit --max-size) before reading a
// file into memory, mirroring the walker's pre-read policy skip.
func TestReadEntryRejectsOversize(t *testing.T) {
	root := t.TempDir()
	cfg := config.New()
	cfg.RootDir = root
	cfg.Quiet = true
	cfg.OutputFile = "-"
	cfg.MaxFileSizeMB = 1 // 1 MB cap
	a, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	writeFile(t, root, "big.go", "")
	big := filepath.Join(root, "big.go")
	if err := os.WriteFile(big, make([]byte, 2<<20), 0o644); err != nil { // 2 MB
		t.Fatal(err)
	}

	if _, err := a.ReadEntry("big.go"); err == nil {
		t.Error("ReadEntry accepted an oversized file")
	} else if !errors.Is(err, ErrFileSkipped) {
		t.Errorf("oversize err = %v, want ErrFileSkipped sentinel", err)
	}
}

// TestReadEntryRejectsBinary verifies the picker fallback refuses binary files
// (skip-before-read, like the walker) unless --include-binary is set.
func TestReadEntryRejectsBinary(t *testing.T) {
	root := t.TempDir()
	cfg := config.New()
	cfg.RootDir = root
	cfg.Quiet = true
	cfg.OutputFile = "-"
	a, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	writeFile(t, root, "image.png", "\x89PNG\r\n\x1a\nwhatever")

	if _, err := a.ReadEntry("image.png"); err == nil {
		t.Error("ReadEntry accepted a binary file")
	} else if !errors.Is(err, ErrFileSkipped) {
		t.Errorf("binary err = %v, want ErrFileSkipped sentinel", err)
	}

	// With --include-binary the file is allowed through to the processor.
	cfg.IncludeBinary = true
	entry, err := a.ReadEntry("image.png")
	if err != nil {
		t.Errorf("ReadEntry rejected binary with --include-binary: %v", err)
	} else if entry.Path != "image.png" {
		t.Errorf("entry.Path = %q, want image.png", entry.Path)
	}
}

// TestRenderFinalToBuffer verifies the picker's non-destructive render path:
// RenderFinalToBuffer returns the rendered document as bytes without writing to
// (or truncating) the output destination, so a caller can replace the file only
// after a render fully succeeds.
func TestRenderFinalToBuffer(t *testing.T) {
	root := t.TempDir()
	cfg := config.New()
	cfg.RootDir = root
	cfg.Quiet = true
	cfg.Style = "markdown"
	cfg.OutputFile = filepath.Join(root, "out.md")
	a, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	buf, err := a.RenderFinalToBuffer([]format.FileEntry{
		{Path: "main.go", Content: []byte("package main\nfunc main() {}\n"), Tokens: 5},
	}, "do the thing")
	if err != nil {
		t.Fatalf("RenderFinalToBuffer: %v", err)
	}
	if !strings.Contains(string(buf), "file: main.go") || !strings.Contains(string(buf), "do the thing") {
		t.Errorf("buffer missing expected content:\n%s", buf)
	}
	// The output file must remain untouched (empty - not yet written to).
	data, err := os.ReadFile(cfg.OutputFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 0 {
		t.Errorf("output file written by RenderFinalToBuffer: %q, want untouched", data)
	}
}
