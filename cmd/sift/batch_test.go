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

func TestDumpMultipleRootsRelativeOutputAvoidsSelfIngest(t *testing.T) {
	base := t.TempDir()
	cwd := t.TempDir()
	t.Chdir(cwd)
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
	out := filepath.Join(cwd, "out.md")
	if _, err := runSift(t, "dump", "--quiet", "--style", "plain", "--output", "out.md", rootA, rootB); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{rootA, rootB} {
		if _, err := os.Stat(filepath.Join(root, "out.md")); !os.IsNotExist(err) {
			t.Fatalf("multi-root output landed inside a root: %v", err)
		}
	}
	if _, err := runSift(t, "dump", "--quiet", "--style", "plain", "--output", "out.md", rootA, rootB); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("second multi-root dump changed output:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

func TestDumpMultipleRootsPrefixedArchitectureGraph(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, name := range []string{"a", "b"} {
		root := filepath.Join(base, name)
		if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
			t.Fatal(err)
		}
		module := "example.com/" + name
		main := "package main\n\nimport \"" + module + "/pkg\"\n\nfunc main() { pkg.Hi() }\n"
		if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module "+module+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(main), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "pkg", "lib.go"), []byte("package pkg\n\nfunc Hi() {}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out := filepath.Join(base, "graph.txt")
	if _, err := runSift(t, "dump", "--quiet", "--style", "plain", "--include-graph", "--output", out, filepath.Join(base, "a"), filepath.Join(base, "b")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"a -> a/pkg", "b -> b/pkg"} {
		if !strings.Contains(text, want) {
			t.Fatalf("multi-root graph missing %q:\n%s", want, text)
		}
	}
}

func TestDumpAllTargetsStdoutClashNamesDash(t *testing.T) {
	root := writeTargetRepo(t, `
[[targets]]
name = "a"
output = "-"

[[targets]]
name = "b"
output = "-"
`)
	if _, err := runSift(t, "dump", "--all-targets", root); err == nil || !strings.Contains(err.Error(), "same output -") {
		t.Fatalf("stdout clash error = %v, want 'same output -'", err)
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
