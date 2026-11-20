package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/format"
)

// TestRenderReplacesOutputFile pins the watch/repeat-render behavior: each
// render must produce one complete document, never append to the previous one.
func TestRenderReplacesOutputFile(t *testing.T) {
	for _, style := range []string{"markdown", "xml", "json"} {
		t.Run(style, func(t *testing.T) {
			dir := t.TempDir()
			out := filepath.Join(dir, "out.txt")
			cfg := config.New()
			cfg.RootDir = dir
			cfg.Style = style
			cfg.OutputFile = out
			cfg.OutputFileExplicit = true
			cfg.SecretScan = false

			application, err := New(cfg)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			defer application.Close()

			files := []format.FileEntry{{
				Path:    "only.go",
				Content: []byte("package only\n"),
				Tokens:  3,
			}}
			if err := application.Render(files, nil, 0, nil); err != nil {
				t.Fatalf("first render: %v", err)
			}
			first, err := os.ReadFile(out)
			if err != nil {
				t.Fatalf("read output: %v", err)
			}
			for i := 0; i < 2; i++ {
				if err := application.Render(files, nil, 0, nil); err != nil {
					t.Fatalf("render %d: %v", i+2, err)
				}
				data, err := os.ReadFile(out)
				if err != nil {
					t.Fatalf("read output: %v", err)
				}
				if !bytes.Equal(data, first) {
					t.Fatalf("render %d changed the document instead of replacing it:\n--- before ---\n%s\n--- after ---\n%s", i+2, first, data)
				}
			}
			switch style {
			case "json":
				if !bytes.HasPrefix(bytes.TrimSpace(first), []byte("[")) {
					t.Fatalf("output is not one JSON document:\n%s", first)
				}
			case "xml":
				if n := strings.Count(string(first), "<repository>"); n != 1 {
					t.Fatalf("xml envelope repeated %d times:\n%s", n, first)
				}
			}
			if style != "json" {
				if n := strings.Count(string(first), "package only"); n != 1 {
					t.Fatalf("file content appears %d times, want 1:\n%s", n, first)
				}
			}
		})
	}
}

// TestRenderMetricsMatchRedactedOutput verifies the reported counts come from
// the files actually written, including secret redaction, rather than the
// pre-redaction collection.
func TestRenderMetricsMatchRedactedOutput(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out.md")
	cfg := config.New()
	cfg.RootDir = dir
	cfg.Style = "markdown"
	cfg.OutputFile = out
	cfg.OutputFileExplicit = true
	cfg.SecretScan = true

	application, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer application.Close()

	files := []format.FileEntry{{
		Path:    "config.go",
		Content: []byte("package config\n\nconst token = \"ghp_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\"\n"),
		Tokens:  1000, // deliberately wrong; the recount must replace it
	}}
	metrics, err := application.RenderWithMetrics(files, nil, 0, nil)
	if err != nil {
		t.Fatalf("RenderWithMetrics: %v", err)
	}
	if metrics.Files != 1 {
		t.Errorf("Files = %d, want 1", metrics.Files)
	}
	if metrics.Secrets != 1 {
		t.Errorf("Secrets = %d, want 1", metrics.Secrets)
	}
	if metrics.Tokens == 1000 || metrics.Tokens <= 0 {
		t.Errorf("Tokens = %d, want the recounted post-redaction value", metrics.Tokens)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !bytes.Contains(data, []byte("[REDACTED_SECRET:")) {
		t.Errorf("output does not contain the redaction marker:\n%s", data)
	}
	if bytes.Contains(data, []byte("ghp_AAAA")) {
		t.Errorf("github token leaked into output:\n%s", data)
	}
}
