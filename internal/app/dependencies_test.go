package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bethropolis/sift/internal/format"
	"github.com/bethropolis/sift/internal/selection"
)

func depFiles() []format.FileEntry {
	return []format.FileEntry{
		{Path: "a.go", Content: []byte("package a\n")},
		{Path: "b.go", Content: []byte("package b\n")},
		{Path: "sub/c.go", Content: []byte("package sub\n")},
		{Path: "sub/d.go", Content: []byte("package sub\n")},
	}
}

func TestExpandDependenciesDepth(t *testing.T) {
	files := depFiles()
	graph := map[string][]string{
		"a.go":     {"b.go"},
		"b.go":     {"sub/c.go"},
		"sub/c.go": {"sub/d.go"},
	}
	selected := []format.FileEntry{files[0]}

	depth1 := ExpandDependencies(selected, files, graph, 1)
	if len(depth1) != 1 || depth1[0].Path != "b.go" {
		t.Fatalf("depth 1 = %v, want [b.go]", pathsOf(depth1))
	}
	depth2 := ExpandDependencies(selected, files, graph, 2)
	if len(depth2) != 2 {
		t.Fatalf("depth 2 = %v, want [b.go sub/c.go]", pathsOf(depth2))
	}
	unlimited := ExpandDependencies(selected, files, graph, -1)
	if len(unlimited) != 3 {
		t.Fatalf("unlimited = %v, want 3 files", pathsOf(unlimited))
	}
	if got := ExpandDependencies(selected, files, graph, 0); len(got) != 0 {
		t.Fatalf("depth 0 = %v, want []", pathsOf(got))
	}
}

func TestExpandDependenciesCycleAndDirs(t *testing.T) {
	files := depFiles()
	graph := map[string][]string{
		"a.go": {"b.go"},
		"b.go": {"a.go", "sub"}, // cycle back + package dir
	}
	deps := ExpandDependencies([]format.FileEntry{files[0]}, files, graph, -1)
	got := map[string]bool{}
	for _, d := range deps {
		got[d.Path] = true
	}
	// b.go directly; sub/ expands to both files beneath it; a.go (the seed)
	// is never re-added despite the cycle.
	if !got["b.go"] || !got["sub/c.go"] || !got["sub/d.go"] || got["a.go"] || len(deps) != 3 {
		t.Fatalf("cycle+dir expansion = %v, want [b.go sub/c.go sub/d.go]", pathsOf(deps))
	}
}

func pathsOf(files []format.FileEntry) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, f.Path)
	}
	return out
}

func TestSelectWithDependencies(t *testing.T) {
	mkCandidates := func() []selection.Candidate {
		return []selection.Candidate{
			{File: format.FileEntry{Path: "a.go", Content: []byte("package a\n"), TokensFull: 10, TokensSig: 4}, PreferredMode: "full"},
		}
	}
	files := []format.FileEntry{
		{Path: "a.go", Content: []byte("package a\n"), TokensFull: 10, TokensSig: 4},
		{Path: "b.go", Content: []byte("package b\n"), TokensFull: 50, TokensSig: 5, SigContent: []byte("package b\n")},
	}
	graph := map[string][]string{"a.go": {"b.go"}}
	tuning := selection.Tuning{}

	t.Run("dep added as signatures within budget", func(t *testing.T) {
		res := SelectWithDependencies(DependencyRequest{
			Candidates: mkCandidates(), Files: files, Graph: graph,
			Budget: 20, Tuning: tuning, MaxDepth: 2,
		})
		if len(res.Selected) != 2 {
			t.Fatalf("selected = %v, want [a.go b.go]", pathsOf(res.Selected))
		}
		if res.Selected[1].Path != "b.go" || !res.Selected[1].IsCompressed {
			t.Errorf("dep not signature-compressed: %+v", res.Selected[1])
		}
		if res.UsedTokens > 20 {
			t.Errorf("UsedTokens = %d, exceeds budget 20", res.UsedTokens)
		}
	})

	t.Run("dep competes in the same optimization, not on crumbs", func(t *testing.T) {
		// Budget 15: the first pass spends 10 on a.go and has no room to
		// add b.go's 5-token signature afterwards — the old remaining-
		// budget design would drop it. Re-running the optimizer over the
		// union fits both (10 + 5).
		res := SelectWithDependencies(DependencyRequest{
			Candidates: mkCandidates(), Files: files, Graph: graph,
			Budget: 15, Tuning: tuning, MaxDepth: 2,
		})
		if len(res.Selected) != 2 {
			t.Fatalf("selected = %v, want [a.go b.go]", pathsOf(res.Selected))
		}
		if res.UsedTokens > 15 {
			t.Errorf("UsedTokens = %d, exceeds budget 15", res.UsedTokens)
		}
	})
}

func TestExpandChosenBudgetBound(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/t\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	files := []format.FileEntry{
		{Path: "a.go", Content: []byte("package a\nimport \"example.com/t/dep\"\n"), TokensFull: 10, Tokens: 10},
		{Path: "dep/dep.go", Content: []byte("package dep\n"), TokensFull: 50, TokensSig: 5, SigContent: []byte("package dep\n")},
	}

	// Budget 12: a.go spends 10, dep's 5-signature form doesn't fit.
	out := ExpandChosen(files, files[:1], dir, 12, 2)
	if len(out) != 1 {
		t.Fatalf("tight budget ExpandChosen = %v, want [a.go]", pathsOf(out))
	}

	// Budget 20: dep joins as signature-compressed context.
	out = ExpandChosen(files, files[:1], dir, 20, 2)
	if len(out) != 2 || out[1].Path != "dep/dep.go" || !out[1].IsCompressed {
		t.Fatalf("roomy budget ExpandChosen = %v, want [a.go dep/dep.go compressed]", pathsOf(out))
	}
}
