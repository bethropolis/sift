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

func writeTargetConfigRepo(t *testing.T) (root, module string) {
	t.Helper()
	root = t.TempDir()
	module = filepath.Join(root, "services", "api")
	if err := os.MkdirAll(module, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		filepath.Join(module, "api.go"):     "package api\n",
		filepath.Join(module, "README.md"):  "api docs\n",
		filepath.Join(module, "PROMPT.md"):  "API module prompt.\n",
		filepath.Join(module, ".sift.toml"): "[sift]\nextensions = [\"go\"]\nprompt_file = \"PROMPT.md\"\n",
		filepath.Join(root, "main.go"):      "package main\n\nfunc main() {}\n",
		filepath.Join(root, ".sift.toml"): `[sift]
roots = ["."]
output = "root-only.md"
extensions = ["md"]

[[targets]]
name = "api"
config = "services/api/.sift.toml"
output = "api-context.txt"
style = "plain"
`,
	}
	for path, body := range files {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	return root, module
}

func assertTargetConfigOutput(t *testing.T, root, module string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "api-context.txt"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "api.go") || !strings.Contains(text, "package api") {
		t.Fatalf("module output did not include api.go:\n%s", text)
	}
	if !strings.Contains(text, "API module prompt.") {
		t.Fatalf("module prompt_file did not resolve from the module config:\n%s", text)
	}
	if strings.Contains(text, "README") {
		t.Fatalf("module config extensions did not apply:\n%s", text)
	}
	if _, err := os.Stat(filepath.Join(module, "api-context.txt")); !os.IsNotExist(err) {
		t.Fatalf("target output landed in the module: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "root-only.md")); !os.IsNotExist(err) {
		t.Fatalf("outer [sift] output leaked into target: %v", err)
	}
}

func TestDumpTargetConfigUsesModuleConfigAndRootOutput(t *testing.T) {
	root, module := writeTargetConfigRepo(t)
	if _, err := runSift(t, "dump", "--target", "api", "--quiet", root); err != nil {
		t.Fatal(err)
	}
	assertTargetConfigOutput(t, root, module)
}

func TestDumpAllTargetsTargetConfig(t *testing.T) {
	root, module := writeTargetConfigRepo(t)
	if _, err := runSift(t, "dump", "--all-targets", "--quiet", root); err != nil {
		t.Fatal(err)
	}
	assertTargetConfigOutput(t, root, module)
}

func TestDumpTargetConfigProfileAndModuleRoots(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	module := filepath.Join(root, "services", "api")
	if err := os.MkdirAll(filepath.Join(module, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		filepath.Join(module, "src", "api.go"): "package api\n",
		filepath.Join(module, "README.md"):     "api docs\n",
		filepath.Join(module, ".sift.toml"): `[sift]
extensions = ["md"]

[profiles.go_only]
extensions = ["go"]
roots = ["src"]
`,
		filepath.Join(root, ".sift.toml"): `[[targets]]
name = "api"
config = "services/api/.sift.toml"
profile = "go_only"
output = "api-context.txt"
style = "plain"
`,
	}
	for path, body := range files {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := runSift(t, "dump", "--target", "api", "--quiet", root); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "api-context.txt"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "api.go") || strings.Contains(text, "README") {
		t.Fatalf("target profile or module roots did not apply:\n%s", text)
	}
}

func TestDumpAllTargetsTargetConfigMultipleRoots(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	module := filepath.Join(root, "services", "api")
	for _, dir := range []string{filepath.Join(module, "a"), filepath.Join(module, "b")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		filepath.Join(module, "a", "a.go"): "package a\n",
		filepath.Join(module, "b", "b.go"): "package b\n",
		filepath.Join(module, ".sift.toml"): `[sift]
roots = ["a", "b"]
`,
		filepath.Join(root, ".sift.toml"): `[[targets]]
name = "api"
config = "services/api/.sift.toml"
output = "api-context.txt"
style = "plain"
`,
	}
	for path, body := range files {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := runSift(t, "dump", "--all-targets", "--quiet", root); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "api-context.txt"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"a/a.go", "b/b.go"} {
		if !strings.Contains(text, want) {
			t.Fatalf("multi-root target config output missing %q:\n%s", want, text)
		}
	}
}

func TestDumpTargetConfigRelativeToConfigFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	module := filepath.Join(root, "services", "api")
	configs := filepath.Join(root, "configs")
	for _, dir := range []string{module, configs} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(module, "api.go"), []byte("package api\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(module, ".sift.toml"), []byte("[sift]\nextensions = [\"go\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rootConfig := filepath.Join(configs, "root.toml")
	if err := os.WriteFile(rootConfig, []byte(`[[targets]]
name = "api"
config = "../services/api/.sift.toml"
output = "api-context.txt"
style = "plain"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := runSift(t, "dump", root, "--config", rootConfig, "--target", "api", "--quiet"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "api-context.txt")); err != nil {
		t.Fatalf("target output not written at project root: %v", err)
	}
}

func TestDumpTargetConfigMissing(t *testing.T) {
	root := writeTargetRepo(t, `
[[targets]]
name = "api"
config = "missing.toml"
output = "api-context.txt"
`)
	_, err := runSift(t, "dump", "--target", "api", "--quiet", root)
	if err == nil || !strings.Contains(err.Error(), "missing.toml") {
		t.Fatalf("missing target config error = %v", err)
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
