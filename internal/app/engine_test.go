package app

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/bethropolis/sift/internal/config"
)

// testProject creates a small project: a go file, a markdown doc, and a file
// carrying a fake secret, plus a git repo marker is intentionally absent so
// ranking stays order-stable without forking git.
func testProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":    "module example.com/demo\n\ngo 1.21\n",
		"main.go":   "package main\n\nfunc main() {}\n",
		"README.md": "# demo\n",
		// Fake AWS key triggers the redactor without touching real secrets.
		"secret.go": "package main\n\nvar key = \"AKIAIOSFODNN7EXAMPLE\"\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func testConfig() *config.Config {
	cfg := config.New()
	cfg.Quiet = true
	cfg.OutputFile = "-"
	cfg.SmartFilter = false
	return cfg
}

// TestScanMatchesCollect pins the engine Scan to the CLI collect path: same
// files, same order, same tokens.
func TestScanMatchesCollect(t *testing.T) {
	root := testProject(t)
	ctx := context.Background()

	got, _, err := Scan(ctx, root, testConfig())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	cfg := testConfig()
	cfg.RootDir = root
	cliApp, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer cliApp.Close()
	want, _, err := cliApp.Collect()
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	if len(got) != len(want) {
		t.Fatalf("Scan returned %d files, Collect returned %d", len(got), len(want))
	}
	// Concurrent walks complete in arbitrary order; compare sorted.
	sort.Slice(got, func(i, j int) bool { return got[i].Path < got[j].Path })
	sort.Slice(want, func(i, j int) bool { return want[i].Path < want[j].Path })
	for i := range want {
		if got[i].Path != want[i].Path || got[i].Tokens != want[i].Tokens {
			t.Fatalf("file %d: Scan=%v/%d Collect=%v/%d", i, got[i].Path, got[i].Tokens, want[i].Path, want[i].Tokens)
		}
	}
}

// TestScanPickerMatchesCollectPicker pins ScanPicker the same way.
func TestScanPickerMatchesCollectPicker(t *testing.T) {
	root := testProject(t)
	ctx := context.Background()

	got, _, err := ScanPicker(ctx, root, testConfig())
	if err != nil {
		t.Fatalf("ScanPicker: %v", err)
	}

	cfg := testConfig()
	cfg.RootDir = root
	cliApp, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer cliApp.Close()
	want, _, err := cliApp.CollectPicker()
	if err != nil {
		t.Fatalf("CollectPicker: %v", err)
	}

	if len(got) != len(want) {
		t.Fatalf("ScanPicker returned %d files, CollectPicker returned %d", len(got), len(want))
	}
	// Concurrent walks complete in arbitrary order; compare sorted.
	sort.Slice(got, func(i, j int) bool { return got[i].Path < got[j].Path })
	sort.Slice(want, func(i, j int) bool { return want[i].Path < want[j].Path })
	for i := range want {
		if got[i].Path != want[i].Path || got[i].TokensFull != want[i].TokensFull {
			t.Fatalf("file %d: ScanPicker=%v/%d CollectPicker=%v/%d", i, got[i].Path, got[i].TokensFull, want[i].Path, want[i].TokensFull)
		}
	}
}

// TestRenderBufferMatchesRenderFinalToBuffer pins the engine render path.
func TestRenderBufferMatchesRenderFinalToBuffer(t *testing.T) {
	root := testProject(t)
	ctx := context.Background()
	cfg := testConfig()

	files, _, err := ScanPicker(ctx, root, cfg)
	if err != nil {
		t.Fatalf("ScanPicker: %v", err)
	}

	got, err := RenderBuffer(ctx, files, "test prompt", cfg)
	if err != nil {
		t.Fatalf("RenderBuffer: %v", err)
	}

	cliCfg := testConfig()
	cliCfg.RootDir = root
	cliApp, err := New(cliCfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer cliApp.Close()
	want, err := cliApp.RenderFinalToBuffer(files, "test prompt")
	if err != nil {
		t.Fatalf("RenderFinalToBuffer: %v", err)
	}

	if string(got) != string(want) {
		t.Fatalf("RenderBuffer output differs from RenderFinalToBuffer:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestScanCancelled ensures a dead context fails fast without walking.
func TestScanCancelled(t *testing.T) {
	root := testProject(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, _, err := Scan(ctx, root, testConfig()); err == nil {
		t.Fatal("Scan with cancelled context should fail")
	}
	if _, err := RenderBuffer(ctx, nil, "", testConfig()); err == nil {
		t.Fatal("RenderBuffer with cancelled context should fail")
	}
}

// TestReadPreviewRedacts ensures previews redact secrets, count post-
// redaction tokens, and report the language.
func TestReadPreviewRedacts(t *testing.T) {
	root := testProject(t)
	cfg := testConfig()

	content, tokens, redactions, language, truncated, err := ReadPreview(root, "secret.go", "full", 1<<20, cfg)
	if err != nil {
		t.Fatalf("ReadPreview: %v", err)
	}
	if truncated {
		t.Fatal("small file must not be truncated")
	}
	if language != "go" {
		t.Fatalf("language = %q, want go", language)
	}
	if redactions == 0 {
		t.Fatal("expected at least one redaction for the fake AWS key")
	}
	if tokens <= 0 {
		t.Fatal("expected positive post-redaction token count")
	}
	for _, leak := range [][]byte{[]byte("AKIAIOSFODNN7EXAMPLE")} {
		for i := 0; i+len(leak) <= len(content); i++ {
			match := true
			for j := range leak {
				if content[i+j] != leak[j] {
					match = false
					break
				}
			}
			if match {
				t.Fatal("preview leaked the raw secret")
			}
		}
	}
}

// TestReadPreviewTruncates checks the 1 MiB-style cap behavior.
func TestReadPreviewTruncates(t *testing.T) {
	root := testProject(t)
	cfg := testConfig()

	content, _, _, _, truncated, err := ReadPreview(root, "main.go", "full", 10, cfg)
	if err != nil {
		t.Fatalf("ReadPreview: %v", err)
	}
	if !truncated {
		t.Fatal("expected truncation at a 10-byte cap")
	}
	if len(content) != 10 {
		t.Fatalf("content length = %d, want 10", len(content))
	}
}

// TestReadPreviewRefusesForceSecrets ensures previews never run unredacted.
func TestReadPreviewRefusesForceSecrets(t *testing.T) {
	root := testProject(t)
	cfg := testConfig()
	cfg.ForceSecrets = true

	if _, _, _, _, _, err := ReadPreview(root, "main.go", "full", 1<<20, cfg); err == nil {
		t.Fatal("ReadPreview with ForceSecrets should be refused")
	}
}

// TestNewBufferedTouchesNothing ensures the buffered constructor creates no
// files and leaves stdout alone.
func TestNewBufferedTouchesNothing(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig()
	cfg.RootDir = dir
	cfg.OutputFile = "should-not-exist.md"

	a, err := NewBuffered(cfg)
	if err != nil {
		t.Fatalf("NewBuffered: %v", err)
	}
	defer a.Close()
	if a.OutputPath() != "" {
		t.Fatalf("buffered app has output path %q", a.OutputPath())
	}
	if _, err := os.Stat(dir + "/should-not-exist.md"); !os.IsNotExist(err) {
		t.Fatal("NewBuffered created the output file")
	}
}
