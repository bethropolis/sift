package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCheckProjectConfigRejectsSymlinkEscape verifies a repo-local symlink that
// points at a host file is treated as escaping the project root, and that the
// project-relative reader refuses it too.
func TestCheckProjectConfigRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.md")
	if err := os.WriteFile(outside, []byte("host secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "leak.md")); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	cfg := "[sift]\nprompt_file = \"leak.md\"\nui_theme_file = \"leak.md\"\n"
	if err := os.WriteFile(filepath.Join(root, ".sift.toml"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	issues := CheckProjectConfig(root)
	if len(issues) < 2 {
		t.Fatalf("issues = %v, want prompt_file and ui_theme_file escapes", issues)
	}

	if content, err := readPromptFileWithRoot(root, "leak.md"); err == nil {
		t.Fatalf("readPromptFileWithRoot returned host content: %q", content)
	}
}

// TestEscapesRootOutputThroughSymlinkDir covers paths that do not exist yet:
// the deepest existing ancestor is resolved, so an output written through a
// symlinked directory is rejected.
func TestEscapesRootOutputThroughSymlinkDir(t *testing.T) {
	root := t.TempDir()
	outsideDir := t.TempDir()
	if err := os.Symlink(outsideDir, filepath.Join(root, "outdir")); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	if !escapesRoot(root, "outdir/new.md") {
		t.Error("output through symlinked directory was not rejected")
	}
	if escapesRoot(root, "new.md") {
		t.Error("plain new output path was rejected")
	}
	if escapesRoot(root, "nested/new.md") {
		t.Error("nonexistent nested output path was rejected")
	}
}

// TestReadPromptFileInRootAllowsContained confirms the contained reader still
// reads ordinary project files.
func TestReadPromptFileInRootAllowsContained(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "prompt.md"), []byte("review this\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readPromptFileWithRoot(root, "prompt.md")
	if err != nil {
		t.Fatalf("readPromptFileWithRoot: %v", err)
	}
	if got != "review this\n" {
		t.Errorf("content = %q", got)
	}
}
