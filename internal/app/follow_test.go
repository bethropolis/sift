package app

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseFollowDirection(t *testing.T) {
	for _, d := range []string{FollowDeps, FollowDependents, FollowBoth} {
		if got, err := ParseFollowDirection(d); err != nil || got != d {
			t.Fatalf("ParseFollowDirection(%q) = %q, %v", d, got, err)
		}
	}
	if _, err := ParseFollowDirection("sideways"); err == nil {
		t.Fatal("expected error for invalid direction")
	}
}

func TestInvertGraph(t *testing.T) {
	inv := InvertGraph(map[string][]string{
		"a.go": {"b.go", "c.go"},
		"b.go": {"c.go"},
	})
	if got := inv["c.go"]; !reflect.DeepEqual(got, []string{"a.go", "b.go"}) {
		t.Fatalf("importers of c.go = %v", got)
	}
	if got := inv["b.go"]; !reflect.DeepEqual(got, []string{"a.go"}) {
		t.Fatalf("importers of b.go = %v", got)
	}
	if _, ok := inv["a.go"]; ok {
		t.Fatalf("a.go has no importers, got %v", inv["a.go"])
	}
}

func TestWalkGraphDirections(t *testing.T) {
	graph := map[string][]string{
		"a.go": {"b.go"},
		"b.go": {"c.go"},
	}
	deps := WalkGraph(graph, []string{"b.go"}, -1)
	if hitPaths(deps) != "b.go c.go" {
		t.Fatalf("deps from b = %v", deps)
	}
	inv := InvertGraph(graph)
	back := WalkGraph(inv, []string{"b.go"}, -1)
	if hitPaths(back) != "b.go a.go" {
		t.Fatalf("dependents of b = %v", back)
	}
	both := WalkGraph(MergeGraphs(graph, inv), []string{"b.go"}, -1)
	if hitPaths(both) != "b.go a.go c.go" {
		t.Fatalf("both from b = %v", both)
	}
	for _, h := range both {
		switch h.Path {
		case "b.go":
			if h.Distance != 0 {
				t.Fatalf("seed distance = %d", h.Distance)
			}
		case "a.go":
			if h.Via != "b.go" {
				t.Fatalf("a.go via = %q", h.Via)
			}
		}
	}
}

func hitPaths(hits []FollowHit) string {
	out := ""
	for i, h := range hits {
		if i > 0 {
			out += " "
		}
		out += h.Path
	}
	return out
}

func TestWalkGraphCycleDiamondDepth(t *testing.T) {
	graph := map[string][]string{
		"a.go": {"b.go", "c.go"},
		"b.go": {"d.go"},
		"c.go": {"d.go"},
		"d.go": {"a.go"},
	}
	all := WalkGraph(graph, []string{"a.go"}, -1)
	if len(all) != 4 {
		t.Fatalf("diamond+cycle = %v, want 4 files", all)
	}
	byPath := map[string]FollowHit{}
	for _, h := range all {
		byPath[h.Path] = h
	}
	if byPath["d.go"].Distance != 2 {
		t.Fatalf("d.go distance = %d, want 2 (shortest wins)", byPath["d.go"].Distance)
	}
	if got := WalkGraph(graph, []string{"a.go"}, 0); len(got) != 1 || got[0].Path != "a.go" {
		t.Fatalf("depth 0 = %v, want seed only", got)
	}
	if got := WalkGraph(graph, []string{"a.go"}, 1); len(got) != 3 {
		t.Fatalf("depth 1 = %v, want seed + 2", got)
	}
}

func TestWalkGraphDirTargets(t *testing.T) {
	graph := map[string][]string{"a.go": {"pkg"}}
	expanded := ExpandGraphDirs(graph, []string{"a.go", "pkg/x.go", "pkg/y.go", "other.go"})
	hits := WalkGraph(expanded, []string{"a.go"}, -1)
	if hitPaths(hits) != "a.go pkg/x.go pkg/y.go" {
		t.Fatalf("dir expansion walk = %v", hits)
	}
	if got := ExpandGraphDirs(map[string][]string{"a.go": {"missing"}}, []string{"a.go"}); len(got["a.go"]) != 0 {
		t.Fatalf("dangling target kept: %v", got)
	}
}

func TestFollowModes(t *testing.T) {
	hits := []FollowHit{{Path: "s"}, {Path: "a", Distance: 1}, {Path: "b", Distance: 2}}
	modes := FollowModes(hits, 1)
	if modes["s"] != "full" || modes["a"] != "full" || modes["b"] != "signatures" {
		t.Fatalf("modes = %v", modes)
	}
	if modes := FollowModes(hits, -1); modes["b"] != "full" {
		t.Fatalf("unlimited full-depth = %v", modes)
	}
}

func TestResolveSeed(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "sub", "f.go")
	if err := os.WriteFile(target, []byte("package sub\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, seed := range []string{"sub/f.go", target} {
		rel, err := ResolveSeed(root, seed)
		if err != nil || rel != "sub/f.go" {
			t.Fatalf("ResolveSeed(%q) = %q, %v", seed, rel, err)
		}
	}
	for _, seed := range []string{"../escape.go", "/etc/hostname", "sub/missing.go"} {
		if _, err := ResolveSeed(root, seed); err == nil {
			t.Fatalf("ResolveSeed(%q) should fail", seed)
		}
	}
	if _, err := ResolveSeed(root, "sub"); err == nil {
		t.Fatal("directory seed should fail")
	}
}
