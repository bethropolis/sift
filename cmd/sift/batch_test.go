package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTargetRepo(t *testing.T, toml string) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".sift.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestDumpAllTargets(t *testing.T) {
	root := writeTargetRepo(t, `
[[targets]]
name = "markdown"
output = "codebase.md"

[[targets]]
name = "xml"
output = "codebase.xml"
style = "xml"
`)
	text, err := runSift(t, "dump", "--all-targets", "--quiet", root)
	if err != nil {
		t.Fatal(err)
	}
	_ = text
	for _, name := range []string{"codebase.md", "codebase.xml"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
}

func TestDumpAllTargetsRejectsConflicts(t *testing.T) {
	root := writeTargetRepo(t, `
[[targets]]
name = "a"
output = "same.md"

[[targets]]
name = "b"
output = "same.md"
`)
	if _, err := runSift(t, "dump", "--all-targets", root); err == nil || !strings.Contains(err.Error(), "same output") {
		t.Fatalf("collision error = %v", err)
	}

	empty := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := os.WriteFile(filepath.Join(empty, ".sift.toml"), []byte("[sift]\nstyle = \"plain\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runSift(t, "dump", "--all-targets", empty); err == nil || !strings.Contains(err.Error(), "no [[targets]]") {
		t.Fatalf("empty targets error = %v", err)
	}

	if _, err := runSift(t, "dump", "--all-targets", "--target", "a", root); err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("conflict error = %v", err)
	}
}

func TestDumpMultipleRootsNamespacesDuplicateBasenames(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	rootA := filepath.Join(base, "a", "svc")
	rootB := filepath.Join(base, "b", "svc")
	for _, root := range []string{rootA, rootB} {
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out := filepath.Join(base, "out.md")
	if _, err := runSift(t, "dump", "--quiet", "--style", "plain", "--output", out, rootA, rootB); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "svc/main.go") || !strings.Contains(text, "b-svc/main.go") {
		t.Fatalf("roots were not namespaced:\n%s", text)
	}
}
