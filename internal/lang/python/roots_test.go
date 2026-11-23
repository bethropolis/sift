package langpy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSourceRootsCannotEscapeRoot(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "repo")
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "pyproject.toml"), []byte(`[tool.setuptools.packages.find]
where = ["src", "../outside"]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(base, "outside"), 0o755); err != nil {
		t.Fatal(err)
	}

	roots := pythonSourceRoots(root)
	want := map[string]bool{"src": true}
	if len(roots) != len(want) {
		t.Fatalf("source roots = %v, want %v", roots, want)
	}
	for _, got := range roots {
		if !want[got] {
			t.Fatalf("unexpected source root %q (all: %v)", got, roots)
		}
	}
}

func TestSrcLayoutAbsoluteImportResolves(t *testing.T) {
	root := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("src/pkg/mod.py", "VALUE = 1\n")
	write("src/pkg/app.py", "from pkg.mod import VALUE\n")

	content := []byte("from pkg.mod import VALUE\n")
	exists := func(p string) bool {
		_, err := os.Stat(filepath.Join(root, filepath.FromSlash(p)))
		return err == nil
	}
	got := pyDriver{}.ResolveImportsInRoot("src/pkg/app.py", root, "", content, exists)
	found := false
	for _, p := range got {
		if p == "src/pkg/mod.py" {
			found = true
		}
	}
	if !found {
		t.Fatalf("src layout import not resolved: %v", got)
	}
}
