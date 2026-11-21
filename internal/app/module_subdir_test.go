package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bethropolis/sift/internal/format"
)

// TestComputeFanInSubdirScan verifies Go imports are re-based when the scan
// root is below the module root: git/walker paths are root-relative, but Go
// package imports are module-relative.
func TestComputeFanInSubdirScan(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, repo, "go.mod", "module example.com/repo\n")
	writeFile(t, repo, "internal/app/app.go", "package app\n")
	writeFile(t, repo, "internal/lang/lang.go", "package lang\n")

	r := NewRanker(filepath.Join(repo, "internal"))
	files := []format.FileEntry{
		{Path: "app/app.go", Content: []byte("package app\n\nimport \"example.com/repo/internal/lang\"\n")},
		{Path: "lang/lang.go", Content: []byte("package lang\n")},
	}
	fanIn, graph := r.computeFanIn(files)

	if got := fanIn["lang/lang.go"]; got != 1 {
		t.Errorf("fanIn[lang/lang.go] = %d, want 1", got)
	}
	if got := fanIn["app/app.go"]; got != 0 {
		t.Errorf("fanIn[app/app.go] = %d, want 0", got)
	}
	targets := graph["app/app.go"]
	if len(targets) != 1 || targets[0] != "lang" {
		t.Errorf("graph[app/app.go] = %v, want [lang]", targets)
	}
}

// TestComputeFanInNestedModule verifies a file under a nested module resolves
// imports against that module and maps them back onto scan-root paths.
func TestComputeFanInNestedModule(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, repo, "go.mod", "module example.com/repo\n")
	writeFile(t, repo, "services/auth/go.mod", "module example.com/auth\n")
	writeFile(t, repo, "services/auth/cmd/main.go", "package main\n")
	writeFile(t, repo, "services/auth/pkg/x/x.go", "package x\n")

	r := NewRanker(repo)
	files := []format.FileEntry{
		{Path: "services/auth/go.mod", Content: []byte("module example.com/auth\n")},
		{Path: "services/auth/cmd/main.go", Content: []byte("package main\n\nimport \"example.com/auth/pkg/x\"\n")},
		{Path: "services/auth/pkg/x/x.go", Content: []byte("package x\n")},
	}
	fanIn, graph := r.computeFanIn(files)

	if got := fanIn["services/auth/pkg/x/x.go"]; got != 1 {
		t.Errorf("fanIn[x.go] = %d, want 1", got)
	}
	targets := graph["services/auth/cmd/main.go"]
	if len(targets) != 1 || targets[0] != "services/auth/pkg/x" {
		t.Errorf("graph main.go = %v, want [services/auth/pkg/x]", targets)
	}
}

// TestComputeFanInRootPackage verifies an import of the module's own root
// package resolves to the scan root and contributes centrality to root files,
// without inflating nested packages.
func TestComputeFanInRootPackage(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, repo, "go.mod", "module example.com/repo\n")

	r := NewRanker(repo)
	files := []format.FileEntry{
		{Path: "main.go", Content: []byte("package main\n\nimport \"example.com/repo\"\n")},
		{Path: "root.go", Content: []byte("package root\n")},
		{Path: "sub/deep.go", Content: []byte("package sub\n")},
	}
	fanIn, graph := r.computeFanIn(files)

	if got := fanIn["root.go"]; got != 1 {
		t.Errorf("fanIn[root.go] = %d, want 1", got)
	}
	if got := fanIn["sub/deep.go"]; got != 0 {
		t.Errorf("fanIn[sub/deep.go] = %d, want 0", got)
	}
	targets := graph["main.go"]
	if len(targets) != 1 || targets[0] != "." {
		t.Errorf("graph[main.go] = %v, want [.]", targets)
	}
}

func TestFindEnclosingModuleStopsAtGitRoot(t *testing.T) {
	parent := t.TempDir()
	writeFile(t, parent, "go.mod", "module example.com/parent\n")
	nested := filepath.Join(parent, "nested")
	if err := os.MkdirAll(filepath.Join(nested, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if dir, mod := findEnclosingModule(nested); dir != "" || mod != "" {
		t.Fatalf("findEnclosingModule = (%q, %q), want no module across the git boundary", dir, mod)
	}
}
